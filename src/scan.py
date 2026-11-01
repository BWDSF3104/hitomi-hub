from __future__ import annotations

import argparse
import hashlib
import json
import re
import sqlite3
import subprocess
import sys
import time
import zipfile
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
    if head[:6] == b"7z\xbc\xaf\x27\x38":
        return "7z"
    return "unknown"


def ext_of(path: Path) -> str:
    return path.suffix.lower().lstrip(".")


def is_pe(head: bytes) -> bool:
    if head[:2] != b"MZ":
        return False
    try:
        e_lfanew = int.from_bytes(head[0x3C:0x40], "little")
    except Exception:
        return False
    if e_lfanew <= 0 or e_lfanew + 4 > len(head):
        return False
    return head[e_lfanew:e_lfanew + 4] == b"PE\x00\x00"


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


def scan_zip(path: Path, limits: dict) -> list:
    findings = []
    with zipfile.ZipFile(path) as zf:
        infos = zf.infolist()
        if len(infos) > limits["max_entries"]:
            findings.append("entry_count_exceeded:%d" % len(infos))
        for info in infos[: limits["max_entries"]]:
            name = info.filename
            if ".." in name or name.startswith("/") or name.startswith("\\") or re.match(r"^[A-Za-z]:", name):
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
    if magic == "pe" and not is_pe(head):
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


def vt_lookup(sha256: str, cfg: dict, state: dict, fetch=None) -> dict:
    vt = cfg.get("virustotal", {})
    if not vt.get("enabled"):
        return {"status": "skipped"}
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
        with urllib.request.urlopen(req, timeout=30) as resp:
            payload = json.load(resp)
    else:
        payload = fetch(url, key)
    state["window"].append(now)
    state["total"] = state.get("total", 0) + 1
    attrs = payload.get("data", {}).get("attributes", {})
    stats = attrs.get("stats", {})
    positives = int(stats.get("malicious", 0)) + int(stats.get("suspicious", 0))
    verdict = "hit" if positives > 0 else "pass"
    names = [v.get("category", "") for v in attrs.get("last_analysis_results", {}).values()
            if v.get("category") in ("malicious", "suspicious")]
    return {"status": verdict, "positives": positives, "threat_name": ";".join(sorted(set(names))[:5])}


def open_db(db_path: Path) -> sqlite3.Connection:
    conn = sqlite3.connect(str(db_path))
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


def ensure_schema(conn: sqlite3.Connection, schema_path: Path) -> None:
    conn.executescript(schema_path.read_text(encoding="utf-8"))
    conn.commit()


def cache_lookup(conn: sqlite3.Connection, sha256: str) -> dict:
    row = conn.execute(
        "SELECT verdict, engine, threat_name, detail FROM scan_cache WHERE sha256 = ?", (sha256,)
    ).fetchone()
    return {"verdict": row[0], "engine": row[1], "threat_name": row[2], "detail": row[3]} if row else {}


def cache_store(conn: sqlite3.Connection, sha256: str, verdict: str, engine: str, threat_name: str, detail: str) -> None:
    conn.execute(
        "INSERT INTO scan_cache (sha256, verdict, engine, threat_name, detail, cached_at) VALUES (?, ?, ?, ?, ?, ?) "
        "ON CONFLICT(sha256) DO UPDATE SET verdict = excluded.verdict, engine = excluded.engine, "
        "threat_name = excluded.threat_name, detail = excluded.detail, cached_at = excluded.cached_at",
        (sha256, verdict, engine, threat_name, detail, now_iso()),
    )
    conn.commit()


def add_finding(conn: sqlite3.Connection, path: str, sha256: str, rule: str, detail: str) -> None:
    conn.execute(
        "INSERT INTO findings (path, sha256, rule, detail, found_at) VALUES (?, ?, ?, ?, ?)",
        (path, sha256, rule, detail, now_iso()),
    )
    conn.commit()


def record_file(conn: sqlite3.Connection, path: str, sha256: str, size: int, ext: str, magic: str,
                status: str, source: str) -> None:
    conn.execute(
        "INSERT INTO files (path, sha256, size, ext, magic, scan_status, scan_source, scan_time) "
        "VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(path) DO UPDATE SET scan_status = excluded.scan_status, "
        "scan_source = excluded.scan_source, scan_time = excluded.scan_time, sha256 = excluded.sha256, "
        "magic = excluded.magic",
        (path, sha256, size, ext, magic, status, source, now_iso()),
    )
    conn.commit()


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


def run_yara(path: Path, rules_path: Path, yr: str, limits: dict) -> list:
    cmd = [yr, "scan", str(rules_path), str(path), "--output-format", "ndjson",
           "--timeout", str(limits.get("yr_timeout_sec", 10)),
           "--cpu-limit", str(limits.get("yr_cpu_limit", 50)),
           "--skip-larger", str(limits.get("yr_skip_larger", 67108864))]
    if limits.get("yr_fast_scan"):
        cmd.append("--fast-scan")
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=limits.get("yr_timeout_sec", 10) + 15)
    except Exception as exc:
        return ["yara_error:%s" % exc]
    rules = parse_yara_output(proc.stdout)
    if proc.returncode not in (0, 1) and not rules:
        return ["yara_error:exit_%d" % proc.returncode]
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
        sha = sha256_file(path)
        ext, magic, findings, status = scan_file(path, cfg)
        rel = str(path.relative_to(root))
        for item in findings:
            add_finding(conn, rel, sha, "structural", item)
        if status == "scan_error":
            summary["scan_error"] += 1
            record_file(conn, rel, sha, path.stat().st_size, ext, magic, status, "structural")
            continue
        if yara_gate(ext, cfg):
            for rule in run_yara(path, rules_path, yr, cfg["limits"]):
                findings.append(rule)
                add_finding(conn, rel, sha, "yara", rule)
            if findings:
                status = "hit"
        if status != "hit" and cfg.get("virustotal", {}).get("enabled") and ext in SCAN_TARGETS:
            cached = cache_lookup(conn, sha)
            if cached:
                summary["cached"] += 1
                if cached["verdict"] == "hit":
                    status = "hit"
                    add_finding(conn, rel, sha, cached["engine"], cached["detail"] or cached["threat_name"] or "")
                else:
                    add_finding(conn, rel, sha, cached["engine"], "cache_hit")
            else:
                result = vt_lookup(sha, cfg, state)
                if result.get("status") in ("pass", "hit"):
                    cache_store(conn, sha, result["status"], "virustotal", result.get("threat_name", ""),
                                "positives=%d" % result.get("positives", 0))
                    if result["status"] == "hit":
                        status = "hit"
                        add_finding(conn, rel, sha, "virustotal", "positives=%d" % result.get("positives", 0))
        summary[status] = summary.get(status, 0) + 1
        record_file(conn, rel, sha, path.stat().st_size, ext, magic, status, "structural+virustotal" if cfg.get("virustotal", {}).get("enabled") else "structural")

    if args.verify:
        summary["external_removal"] = verify_missing(conn, root)

    print(json.dumps(summary, sort_keys=True))
    conn.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
