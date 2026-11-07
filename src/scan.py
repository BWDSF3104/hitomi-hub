from __future__ import annotations

import argparse
import hashlib
import json
import re
import socket
import sqlite3
import subprocess
import sys
import time
import zipfile
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

SCAN_TARGETS = {"pdf", "cbz", "zip"}
IMAGE_EXTS = {"webp", "avif", "png", "jpg", "jpeg", "gif", "bmp"}
EXEC_EXT = {"exe", "dll", "scr", "jar", "js", "vbs", "ps1", "bat", "cmd", "com", "msi", "lnk"}

DEFAULT_LIMITS = {
    "max_entries": 2000,
    "max_uncompressed_bytes": 209715200,
    "max_ratio": 100,
    "max_scan_bytes": 2097152,
    "yr_timeout_sec": 10,
    "yr_cpu_limit": 50,
    "yr_skip_larger": 67108864,
    "yr_fast_scan": True,
}

STATUS_VALUES = {"pass", "hit", "not_evaluated", "external_removal", "scan_error"}
SCAN_CACHE_TTL_SECONDS = 30 * 24 * 3600
NEGATIVE_SCAN_CACHE_TTL_SECONDS = 2 * 3600

# 正典 data ファイル（schema.sql / rules/ / config.example.json）はリポジトリルート（完成フォルダ直下）
DATA_DIR = Path(__file__).resolve().parent.parent


def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def detect_magic(head: bytes) -> str:
    if head[:4] == b"PK\x03\x04" or head[:4] == b"PK\x05\x06" or head[:4] == b"PK\x07\x08":
        return "zip"
    if head[:4] == b"%PDF":
        return "pdf"
    if head[:2] == b"MZ":
        return "pe"
    if head[:4] == b"\x7fELF":
        return "elf"
    if head[:4] == b"RIFF" and head[8:12] == b"WEBP":
        return "webp"
    if head[:8] == b"\x89PNG\r\n\x1a\n":
        return "png"
    if head[:6] in (b"GIF87a", b"GIF89a"):
        return "gif"
    if head[:2] == b"BM":
        return "bmp"
    if head[:3] == b"\xff\xd8\xff":
        return "jpeg"
    if head[4:8] == b"ftyp":
        brand = head[8:12]
        if brand in (b"avif", b"avis"):
            return "avif"
        if brand in (b"heic", b"heix"):
            return "heif"
        return "iso-bmff"
    if head[:6] == b"7z\xbc\xaf\x27\x1c":
        return "7z"
    return "unknown"


def ext_of(path: Path) -> str:
    return path.suffix.lower().lstrip(".")


def is_pe(path: Path, head: bytes) -> bool:
    if head[:2] != b"MZ":
        return False
    if len(head) < 0x40:
        return False
    e_lfanew = int.from_bytes(head[0x3C:0x40], "little")
    if e_lfanew <= 0:
        return False
    if e_lfanew + 4 <= len(head):
        signature = head[e_lfanew:e_lfanew + 4]
    else:
        try:
            with path.open("rb") as fh:
                fh.seek(e_lfanew)
                signature = fh.read(4)
        except OSError:
            return False
    return signature == b"PE\x00\x00"


EXPECTED_MAGIC = {
    "webp": {"webp"},
    "avif": {"avif"},
    "png": {"png"},
    "jpg": {"jpeg"},
    "jpeg": {"jpeg"},
    "gif": {"gif"},
    "bmp": {"bmp"},
    "cbz": {"zip"},
    "zip": {"zip"},
    "pdf": {"pdf"},
}


def has_dot_dot_segment(name: str) -> bool:
    # path separator（/ \）を跨いだ `..` segment のみの判定。部分一致の `..` は hit しない。
    return any(seg == ".." for seg in re.split(r"[/\\]", name))


def scan_zip(path: Path, limits: dict) -> list:
    findings = []
    with zipfile.ZipFile(path) as zf:
        infos = zf.infolist()
        if len(infos) > limits["max_entries"]:
            findings.append("entry_count_exceeded:%d" % len(infos))
        for info in infos[: limits["max_entries"]]:
            name = info.filename
            if has_dot_dot_segment(name) or name.startswith("/") or name.startswith("\\") or re.match(r"^[A-Za-z]:", name):
                findings.append("path_traversal:%s" % name)
            entry_ext = name.rsplit(".", 1)[-1].lower() if "." in name else ""
            if entry_ext in EXEC_EXT:
                findings.append("executable_entry:%s" % name)
            if info.file_size > limits["max_uncompressed_bytes"]:
                findings.append("entry_size_exceeded:%s" % name)
            if info.compress_size and info.file_size and info.file_size / info.compress_size > limits["max_ratio"]:
                findings.append("ratio_exceeded:%s" % name)
    return findings


PDF_KEYS = (b"/Launch", b"/OpenAction", b"/AA", b"/JS", b"/JavaScript", b"/EmbeddedFile", b"/XFA")


def scan_pdf(path: Path, limits: dict) -> list:
    with path.open("rb") as fh:
        data = fh.read(limits["max_scan_bytes"])
    found = []
    for key in PDF_KEYS:
        if key in data:
            found.append("pdf_object:%s" % key.decode())
    return found


def scan_file(path: Path, cfg: dict) -> tuple:
    limits = cfg.get("limits", DEFAULT_LIMITS)
    ext = ext_of(path)
    size = path.stat().st_size
    with path.open("rb") as fh:
        head = fh.read(64)
    magic = detect_magic(head)
    findings = []
    expected = EXPECTED_MAGIC.get(ext)
    if expected and magic not in expected:
        findings.append("magic_mismatch:%s_vs_%s" % (ext, magic))
    if magic == "pe" and not is_pe(path, head):
        findings.append("mz_without_pe_header")
    if magic == "zip" and ext in SCAN_TARGETS:
        try:
            findings.extend(scan_zip(path, limits))
        except zipfile.BadZipFile:
            return ext, magic, ["bad_zip"], "scan_error"
    if magic == "pdf":
        try:
            findings.extend(scan_pdf(path, limits))
        except OSError as exc:
            return ext, magic, ["read_error:%s" % exc], "scan_error"
    if findings:
        return ext, magic, findings, "hit"
    if ext in SCAN_TARGETS:
        return ext, magic, findings, "pass"
    return ext, magic, findings, "not_evaluated"


def vt_lookup(sha256: str, cfg: dict, state: dict, fetch=None, conn=None) -> dict:
    vt = cfg.get("virustotal", {})
    if not vt.get("enabled"):
        return {"status": "skipped"}
    if conn is not None:
        cached = cache_lookup(conn, sha256)
        if cached.get("verdict"):
            return {
                "status": cached["verdict"],
                "positives": 0,
                "threat_name": cached.get("threat_name", ""),
                "detail": cached.get("detail", ""),
                "http_status": 200,
            }

    now = time.time()
    window = [t for t in state.get("window", []) if now - t < 60]
    state["window"] = window
    if len(window) >= int(vt.get("max_requests_per_minute", 4)):
        return {"status": "rate_limited"}
    if state.get("total", 0) >= int(vt.get("max_daily_requests", 500)):
        return {"status": "daily_quota_exhausted"}
    key = vt.get("api_key") or ""
    if not key:
        return {"status": "no_key"}
    url = "https://www.virustotal.com/api/v3/files/%s" % sha256
    if fetch is None:
        req = urllib.request.Request(url, headers={"x-apikey": key})
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                result = json.load(resp)
        except urllib.error.HTTPError as exc:
            result = ({}, exc.code)
        except (urllib.error.URLError, socket.timeout) as exc:
            # B-10 残件（2026-11-04 推奨承認 #3）: 非 HTTPError（URLError・timeout）も tuple 規約へ。
            # HTTPError は URLError の subclass なのでこの順が B-10 分岐の意味を維持する。http_status 0 = network 層
            result = ({}, 0)
    else:
        result = fetch(url, key)
    if isinstance(result, tuple):
        payload, http_status = result
    else:
        payload, http_status = result, 200

    state["window"].append(now)
    state["total"] = state.get("total", 0) + 1

    if http_status != 200:
        status = "not_evaluated"
        if http_status == 401:
            status = "auth_error"
        elif http_status == 429:
            status = "rate_limited"
        elif http_status == 404:
            status = "unknown_to_vt"
        elif http_status == 0:
            status = "network_error"
        if conn is not None and status == "unknown_to_vt":
            cache_store(conn, sha256, status, "virustotal", "", status)
        return {"status": status, "positives": 0, "threat_name": "", "detail": status, "http_status": http_status}

    attrs = payload.get("data", {})
    if not isinstance(attrs, dict):
        return {"status": "not_evaluated", "positives": 0, "threat_name": "", "detail": "", "http_status": http_status}
    attributes = attrs.get("attributes", {})
    if not isinstance(attributes, dict):
        return {"status": "not_evaluated", "positives": 0, "threat_name": "", "detail": "", "http_status": http_status}
    stats = attributes.get("last_analysis_stats", {})
    results = attributes.get("last_analysis_results", {})
    if not isinstance(stats, dict):
        stats = {}
    if not isinstance(results, dict):
        results = {}
    positives = int(stats.get("malicious", 0)) + int(stats.get("suspicious", 0))
    if not results and positives == 0:
        return {"status": "not_evaluated", "positives": 0, "threat_name": "", "detail": "", "http_status": http_status}
    verdict = "hit" if positives > 0 else "pass"
    names = []
    for engine, entry in results.items():
        if isinstance(entry, dict) and entry.get("category") in ("malicious", "suspicious"):
            engine_name = entry.get("engine_name") or engine
            result_name = entry.get("result") or ""
            names.append("%s: %s" % (engine_name, result_name) if result_name else engine_name)
    return {"status": verdict, "positives": positives, "threat_name": ";".join(sorted(names)[:5]),
            "detail": "positives=%d" % positives, "http_status": http_status}


def open_db(db_path: Path) -> sqlite3.Connection:
    conn = sqlite3.connect(str(db_path))
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


def ensure_schema(conn: sqlite3.Connection, schema_path: Path) -> None:
    conn.executescript(schema_path.read_text(encoding="utf-8"))
    conn.commit()


def first_nonempty(*values) -> str:
    for value in values:
        if value:
            return value
    return ""


def cache_lookup(conn: sqlite3.Connection, sha256: str, ttl: int = SCAN_CACHE_TTL_SECONDS,
                 negative_ttl: int = NEGATIVE_SCAN_CACHE_TTL_SECONDS) -> dict:
    row = conn.execute(
        "SELECT verdict, engine, threat_name, detail, cached_at FROM scan_cache WHERE sha256 = ?", (sha256,)
    ).fetchone()
    if not row:
        return {}
    verdict, engine, threat_name, detail, cached_at = row
    if cached_at:
        effective_ttl = ttl
        if verdict == "unknown_to_vt" and negative_ttl < effective_ttl:
            effective_ttl = negative_ttl
        try:
            cached_time = datetime.strptime(cached_at, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
        except ValueError:
            cached_time = None
        if effective_ttl > 0 and cached_time is not None and time.time() - cached_time.timestamp() > effective_ttl:
            return {}
    return {"verdict": verdict, "engine": engine, "threat_name": threat_name, "detail": detail}


def cache_store(conn: sqlite3.Connection, sha256: str, verdict: str, engine: str, threat_name: str, detail: str) -> None:
    conn.execute(
        "INSERT INTO scan_cache (sha256, verdict, engine, threat_name, detail, cached_at) VALUES (?, ?, ?, ?, ?, ?) "
        "ON CONFLICT(sha256) DO UPDATE SET verdict = excluded.verdict, engine = excluded.engine, "
        "threat_name = excluded.threat_name, detail = excluded.detail, cached_at = excluded.cached_at",
        (sha256, verdict, engine, threat_name, detail, now_iso()),
    )


def add_finding(conn: sqlite3.Connection, path: str, sha256: str, rule: str, detail: str) -> None:
    conn.execute(
        "INSERT OR IGNORE INTO findings (path, sha256, rule, detail, found_at) VALUES (?, ?, ?, ?, ?)",
        (path, sha256, rule, detail, now_iso()),
    )


def record_file(conn: sqlite3.Connection, path: str, sha256: str, size: int, ext: str, magic: str,
                status: str, source: str) -> None:
    conn.execute(
        "INSERT INTO files (path, sha256, size, ext, magic, scan_status, scan_source, scan_time) "
        "VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(path) DO UPDATE SET scan_status = excluded.scan_status, "
        "scan_source = excluded.scan_source, scan_time = excluded.scan_time, sha256 = excluded.sha256, "
        "magic = excluded.magic",
        (path, sha256, size, ext, magic, status, source, now_iso()),
    )


def parse_yara_output(text: str) -> list:
    rules = []
    for line in text.splitlines():
        if not line.strip():
            continue
        try:
            obj = json.loads(line)
        except ValueError:
            continue
        if isinstance(obj.get("rules"), list):
            for item in obj["rules"]:
                if isinstance(item, dict) and item.get("identifier"):
                    rules.append(item["identifier"])
                elif isinstance(item, str):
                    rules.append(item)
        elif obj.get("rule"):
            rules.append(obj["rule"])
    return rules


def yara_gate(ext: str, cfg: dict) -> bool:
    # YARA は scan target の全ファイルで走らせる。構造検査が既に hit を捕えたファイルでも
    # YARA 固有規則（参照 entry・PDF の複合条件）が hit できる。二重走査は YARA 固有規則の対価。
    return bool(cfg.get("yr")) and ext in SCAN_TARGETS


def run_yara_with_runner(path: Path, rules_path: Path, yr: str, limits: dict, runner=None) -> tuple:
    cmd = [yr, "scan", str(rules_path), str(path), "--output-format", "ndjson",
           "--timeout", str(limits.get("yr_timeout_sec", 10)),
           "--cpu-limit", str(limits.get("yr_cpu_limit", 50)),
           "--skip-larger", str(limits.get("yr_skip_larger", 67108864))]
    if limits.get("yr_fast_scan"):
        cmd.append("--fast-scan")
    if runner is None:
        try:
            proc = subprocess.run(cmd, capture_output=True, text=True, timeout=limits.get("yr_timeout_sec", 10) + 15)
        except Exception as exc:
            return ([], "yara_error:%s" % exc)
    else:
        try:
            proc = runner(cmd)
        except Exception as exc:
            return ([], "yara_error:%s" % exc)
    exit_code = getattr(proc, "returncode", 0)
    if exit_code != 0:
        return ([], "yara_error:exit_%d" % exit_code)
    return (parse_yara_output(getattr(proc, "stdout", "")), "")


def run_yara(path: Path, rules_path: Path, yr: str, limits: dict) -> list:
    rules, _ = run_yara_with_runner(path, rules_path, yr, limits)
    return rules


def resolve_root(explicit: str) -> Path:
    if explicit:
        return Path(explicit).resolve()
    cur = Path.cwd().resolve()
    probe = cur
    while probe != probe.parent:
        if (probe / ".git").exists():
            return probe
        probe = probe.parent
    return cur


def load_config(cfg_path: Path) -> dict:
    cfg = json.loads(cfg_path.read_text(encoding="utf-8"))
    cfg.setdefault("limits", DEFAULT_LIMITS)
    for key, value in DEFAULT_LIMITS.items():
        cfg["limits"].setdefault(key, value)
    return cfg


def verify_missing(conn: sqlite3.Connection, root: Path) -> int:
    count = 0
    for row in conn.execute("SELECT path, scan_status FROM files WHERE missing_at_verify = 0"):
        if not (root / row[0]).exists():
            conn.execute("UPDATE files SET missing_at_verify = 1, scan_status = 'external_removal' WHERE path = ?", (row[0],))
            count += 1
    conn.commit()
    return count


def process_file(conn: sqlite3.Connection, path: Path, root: Path, cfg: dict, state: dict, summary: dict,
                 fetch=None, rules_path: Path = None, runner=None, yr: str = "") -> str:
    sha = sha256_file(path)
    ext, magic, findings, status = scan_file(path, cfg)
    rel = str(path.relative_to(root))
    yr_path = yr or cfg.get("yr")
    source = "structural"
    cached = False
    yara_findings = []
    vt_findings = []
    pending_cache = None

    if status != "scan_error" and yr_path and yara_gate(ext, cfg):
        rules, yara_error = run_yara_with_runner(path, rules_path or DATA_DIR / "rules" / "structural.yar", yr_path, cfg["limits"], runner)
        if yara_error:
            if status != "hit":
                # Go 正典表現（B-07・`bfdc83e`）: YARA 実行失敗は構造 pass を scan_error へ上書きしない。
                # 判定不能（not_evaluated）へ維持し、失敗は scan_source（structural+yara_error）で表現（retry 判定用）
                status = "not_evaluated"
            source = "structural+yara_error"
        elif rules:
            status = "hit"
            source = "structural+yara"
            for rule in rules:
                yara_findings.append(("yara", rule))

    if status not in ("hit", "scan_error") and cfg.get("virustotal", {}).get("enabled") and ext in SCAN_TARGETS:
        cached_entry = cache_lookup(conn, sha)
        if cached_entry.get("verdict"):
            source = "structural+virustotal"
            cached = True
            detail = first_nonempty(cached_entry.get("detail", ""), cached_entry.get("threat_name", ""))
            rule = cached_entry.get("engine", "virustotal")
            if cached_entry["verdict"] == "hit":
                status = "hit"
            vt_findings.append((rule, detail or "cache_hit"))
        else:
            result = vt_lookup(sha, cfg, state, fetch, conn)
            if result.get("status") in ("pass", "hit"):
                pending_cache = (sha, result["status"], "virustotal", result.get("threat_name", ""),
                                 result.get("detail", "") or "positives=%d" % result.get("positives", 0))
                source = "structural+virustotal"
                if result["status"] == "hit":
                    status = "hit"
                    vt_findings.append(("virustotal", first_nonempty(result.get("threat_name", ""), result.get("detail", ""))))
            elif result.get("status") == "unknown_to_vt":
                source = "structural+virustotal"
                vt_findings.append(("virustotal", first_nonempty(result.get("detail", ""), result.get("threat_name", ""))))
            elif result.get("status") != "skipped":
                vt_findings.append(("virustotal", "api_error:%s" % result.get("status", "")))

    with conn:
        for item in findings:
            add_finding(conn, rel, sha, "structural", item)
        for rule, detail in yara_findings:
            add_finding(conn, rel, sha, rule, detail)
        for rule, detail in vt_findings:
            add_finding(conn, rel, sha, rule, detail)
        if pending_cache:
            cache_store(conn, *pending_cache)
        record_file(conn, rel, sha, path.stat().st_size, ext, magic, status, source)

    summary[status] = summary.get(status, 0) + 1
    if cached:
        summary["cached"] = summary.get("cached", 0) + 1
    return status


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description="hitomi hub scan layer (structural + hash reputation)")
    parser.add_argument("--root", default="")
    parser.add_argument("--config", default="")
    parser.add_argument("--gallery-dir", default="")
    parser.add_argument("--no-vt", action="store_true")
    parser.add_argument("--verify", action="store_true", help="mark recorded files that no longer exist")
    args = parser.parse_args(argv)

    root = resolve_root(args.root)
    cfg_path = Path(args.config) if args.config else DATA_DIR / "config.example.json"
    cfg = load_config(cfg_path)
    if args.no_vt:
        cfg.setdefault("virustotal", {})["enabled"] = False
    if args.gallery_dir:
        cfg["gallery_dir"] = args.gallery_dir

    db_path = root / cfg.get("db", "hitomi.db")
    rules_path = DATA_DIR / "rules" / "structural.yar"
    conn = open_db(db_path)
    ensure_schema(conn, DATA_DIR / "schema.sql")

    gallery = root / cfg.get("gallery_dir", "Galleries")
    if not gallery.exists():
        print("gallery dir not found: %s" % gallery, file=sys.stderr)
        return 2

    yr = cfg.get("yr")
    state = {"window": [], "total": 0}
    summary = {"pass": 0, "hit": 0, "not_evaluated": 0, "scan_error": 0, "cached": 0}

    for path in sorted(gallery.rglob("*")):
        if not path.is_file():
            continue
        process_file(conn, path, root, cfg, state, summary, rules_path=rules_path)

    if args.verify:
        summary["external_removal"] = verify_missing(conn, root)

    print(json.dumps(summary, sort_keys=True))
    conn.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
