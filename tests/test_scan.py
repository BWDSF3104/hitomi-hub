"""src/scan.py（hitomi-hub スキャン層）tests（tmp_path・VirusTotal は mock・外部 API なし）。

Run from project root:
    python -m pytest tests/test_hitomi_scan.py -v
"""

from __future__ import annotations

import json
import sqlite3
import sys
import time
import zipfile
from pathlib import Path
from types import SimpleNamespace

import pytest

SCRIPT_DIR = Path(__file__).resolve().parents[1] / "src"
DATA_DIR = Path(__file__).resolve().parents[1]

sys.path.insert(0, str(SCRIPT_DIR))

import scan  # noqa: E402

CFG = json.loads((DATA_DIR / "config.example.json").read_text(encoding="utf-8"))
PNG = b"\x89PNG\r\n\x1a\n" + b"\x00" * 32


def make_db(tmp_path: Path) -> sqlite3.Connection:
    conn = scan.open_db(tmp_path / "hitomi.db")
    scan.ensure_schema(conn, DATA_DIR / "schema.sql")
    return conn


def make_zip(path: Path, entries: dict) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        for name, data in entries.items():
            zf.writestr(name, data)
    return path


def test_clean_cbz_is_pass(tmp_path):
    cbz = make_zip(tmp_path / "Galleries" / "123" / "123.cbz", {"01.webp": PNG})
    ext, magic, findings, status = scan.scan_file(cbz, CFG)
    assert magic == "zip"
    assert findings == []
    assert status == "pass"


def test_zip_spoofed_entry_and_traversal(tmp_path):
    cbz = make_zip(tmp_path / "Galleries" / "123" / "123.cbz",
                   {"../evil.exe": b"MZ\x90\x00", "ok.webp": PNG})
    ext, magic, findings, status = scan.scan_file(cbz, CFG)
    assert status == "hit"
    assert any(f.startswith("path_traversal:") for f in findings)
    assert any(f.startswith("executable_entry:") for f in findings)


def test_zip_ratio_exceeded(tmp_path):
    cbz = make_zip(tmp_path / "Galleries" / "124" / "124.cbz", {"bomb.webp": b"\x00" * 2000000})
    _, _, findings, status = scan.scan_file(cbz, CFG)
    assert status == "hit"
    assert any(f.startswith("ratio_exceeded:") for f in findings)


def test_pdf_viewer_action_hit(tmp_path):
    path = tmp_path / "Galleries" / "125" / "125.pdf"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(b"%PDF-1.4\n1 0 obj\n<< /Launch << /S /URI /F (evil.exe) >> >>\n")
    ext, magic, findings, status = scan.scan_file(path, CFG)
    assert magic == "pdf"
    assert status == "hit"
    assert "pdf_object:/Launch" in findings


def test_magic_mismatch_and_mz_without_pe(tmp_path):
    path = tmp_path / "Galleries" / "126" / "image.png"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(b"MZ\x90\x00" + b"\x00" * 200)
    _, magic, findings, status = scan.scan_file(path, CFG)
    assert magic == "pe"
    assert status == "hit"
    assert "magic_mismatch:png_vs_pe" in findings
    assert "mz_without_pe_header" in findings


def test_plain_image_is_not_evaluated(tmp_path):
    path = tmp_path / "Galleries" / "127" / "001.png"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(PNG)
    _, magic, findings, status = scan.scan_file(path, CFG)
    assert magic == "png"
    assert status == "not_evaluated"


def test_bad_zip_is_scan_error(tmp_path):
    path = tmp_path / "Galleries" / "128" / "broken.cbz"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(b"PK\x03\x04garbage")
    _, _, findings, status = scan.scan_file(path, CFG)
    assert status == "scan_error"
    assert findings == ["bad_zip"]


def test_vt_hash_only_lookup_uses_mock(tmp_path):
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    calls = []

    def fetch(url, key):
        calls.append((url, key))
        return {"data": {"attributes": {"last_analysis_stats": {"malicious": 0, "suspicious": 0},
                                        "last_analysis_results": {"e": {"category": "undetected"}}}}}, 200

    result = scan.vt_lookup("a" * 64, cfg, {"window": [], "total": 0}, fetch=fetch)
    assert result["status"] == "pass"
    assert calls and calls[0][0].endswith("/files/" + "a" * 64)
    assert "upload" not in calls[0][0]


def test_vt_rate_limit_blocks_second_request(tmp_path):
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    state = {"window": [time.time() for _ in range(int(cfg["virustotal"]["max_requests_per_minute"]))], "total": 0}
    assert scan.vt_lookup("b" * 64, cfg, state, fetch=lambda url, key: {})["status"] == "rate_limited"
    assert scan.vt_lookup("b" * 64, cfg, {"window": [], "total": 500}, fetch=lambda url, key: {})["status"] == "daily_quota_exhausted"


def test_cache_prevents_lookup(tmp_path):
    conn = make_db(tmp_path)
    scan.cache_store(conn, "c" * 64, "hit", "virustotal", "Trojan:Win32/Test", "positives=3")
    cached = scan.cache_lookup(conn, "c" * 64)
    assert cached["verdict"] == "hit"
    assert cached["threat_name"] == "Trojan:Win32/Test"
    conn.close()


def test_parse_yara_output_matches_real_ndjson_shape():
    line = '{"path":"a/100.cbz","rules":[{"identifier":"Zip_Path_Traversal"},{"identifier":"Zip_Executable_Entry"}]}'
    assert scan.parse_yara_output(line) == ["Zip_Path_Traversal", "Zip_Executable_Entry"]
    assert scan.parse_yara_output('{"rule":"MZ_Header_At_Offset0","path":"a.png"}') == ["MZ_Header_At_Offset0"]
    assert scan.parse_yara_output("not json\n\n") == []


def test_run_yara_rules_on_fixture(tmp_path):
    yr = Path(__file__).resolve().parents[1] / "tools" / "yr.exe"
    if not yr.exists():
        pytest.skip("tools/yr.exe not present")
    cbz = make_zip(tmp_path / "100.cbz", {"../payload.exe": b"MZ\x90\x00", "bomb.webp": b"\x00" * 2000000})
    rules = scan.run_yara(cbz, DATA_DIR / "rules" / "structural.yar", str(yr), CFG["limits"])
    assert "Zip_Path_Traversal" in rules
    assert "Zip_Executable_Entry" in rules
    assert "Zip_Decompression_Ratio" in rules
    spoofed = tmp_path / "image.png"
    spoofed.write_bytes(b"MZ\x90\x00" + b"\x00" * 60)
    assert scan.run_yara(spoofed, DATA_DIR / "rules" / "structural.yar", str(yr), CFG["limits"]) == ["MZ_Header_At_Offset0"]
    pdf = tmp_path / "a.pdf"
    pdf.write_bytes(b"%PDF-1.4\n1 0 obj\n<< /OpenAction << /S /JavaScript /JS (x) >> >>\n")
    assert scan.run_yara(pdf, DATA_DIR / "rules" / "structural.yar", str(yr), CFG["limits"]) == ["PDF_Structural_Risk"]


def test_yara_gate_runs_for_all_scan_targets():
    # 構造検査が hit を捕えたファイルでも YARA 固有規則が hit できる（no-op 回避）
    cfg = dict(CFG)
    cfg["yr"] = "tools/yr.exe"
    assert scan.yara_gate("cbz", cfg) is True
    assert scan.yara_gate("pdf", cfg) is True
    assert scan.yara_gate("webp", cfg) is False
    cfg["yr"] = None
    assert scan.yara_gate("cbz", cfg) is False


def test_yara_only_rules_fire_on_fixture(tmp_path):
    yr = Path(__file__).resolve().parents[1] / "tools" / "yr.exe"
    if not yr.exists():
        pytest.skip("tools/yr.exe not present")
    rules_path = DATA_DIR / "rules" / "structural.yar"

    # 参照 entry（構造検査の exec ext に無い）→ YARA 側だけの検知
    cbz = make_zip(tmp_path / "100.cbz", {"startup.url": "[InternetShortcut]\nURL=https://example.com\n"})
    assert scan.run_yara(cbz, rules_path, str(yr), CFG["limits"]) == ["Zip_Reference_Entry"]

    # 単独 key の hit（構造検査と重複）は YARA の単独規則だけ
    single = tmp_path / "single.pdf"
    single.write_bytes(b"%PDF-1.4\n1 0 obj\n<< /OpenAction << /S /JavaScript /JS (x) >> >>\n")
    assert scan.run_yara(single, rules_path, str(yr), CFG["limits"]) == ["PDF_Structural_Risk"]

    # 複合条件（構造検査は key の有無しか見ない）→ YARA 固有規則が上書きしない
    combo = tmp_path / "combo.pdf"
    combo.write_bytes(b"%PDF-1.4\n1 0 obj\n<< /OpenAction << /S /Launch >> >>\n")
    assert "PDF_OpenAction_Launch" in scan.run_yara(combo, rules_path, str(yr), CFG["limits"])
    combo2 = tmp_path / "combo2.pdf"
    combo2.write_bytes(b"%PDF-1.4\n1 0 obj\n<< /AA << /OS << /JS (x) >> >> >>\n")
    assert "PDF_AA_Script" in scan.run_yara(combo2, rules_path, str(yr), CFG["limits"])



def test_yara_findings_recorded_for_already_hit_file(tmp_path, monkeypatch):
    # main ループは hit 判定後も YARA findings を engine 'yara' で記録する
    cfg = dict(CFG)
    cfg["yr"] = "tools/yr.exe"
    conn = make_db(tmp_path)
    cbz = make_zip(tmp_path / "Galleries" / "5" / "5.cbz", {"../evil.exe": b"MZ\x90\x00", "startup.url": "x"})
    monkeypatch.setattr(scan, "run_yara", lambda path, rules_path, yr, limits: ["Zip_Reference_Entry"])
    sha = scan.sha256_file(cbz)
    ext, magic, findings, status = scan.scan_file(cbz, cfg)
    assert status == "hit"
    scan.add_finding(conn, "Galleries/5/5.cbz", sha, "structural", findings[0])
    if scan.yara_gate(ext, cfg):
        for rule in scan.run_yara(cbz, DATA_DIR / "rules" / "structural.yar", cfg["yr"], cfg["limits"]):
            scan.add_finding(conn, "Galleries/5/5.cbz", sha, "yara", rule)
    rows = conn.execute("SELECT rule, detail FROM findings ORDER BY rule").fetchall()
    assert ("structural", findings[0]) in rows
    assert ("yara", "Zip_Reference_Entry") in rows
    conn.close()


def test_status_constraint_and_verify_missing(tmp_path):
    conn = make_db(tmp_path)
    with pytest.raises(sqlite3.IntegrityError):
        conn.execute(
            "INSERT INTO files (path, sha256, size, ext, magic, scan_status, scan_source, scan_time) "
            "VALUES (?, ?, 1, 'webp', 'zip', 'clean', 'structural', ?)", ("a/1.webp", "d" * 64, scan.now_iso())
        )
    gallery = tmp_path / "Galleries" / "9"
    gallery.mkdir(parents=True)
    (gallery / "001.webp").write_bytes(PNG)
    scan.record_file(conn, "Galleries/9/001.webp", "e" * 64, len(PNG), "webp", "webp", "not_evaluated", "structural")
    (gallery / "001.webp").unlink()
    assert scan.verify_missing(conn, tmp_path) == 1
    row = conn.execute("SELECT scan_status, missing_at_verify FROM files WHERE sha256 = ?", ("e" * 64,)).fetchone()
    assert row[0] == "external_removal"
    assert row[1] == 1
    conn.close()


def test_add_finding_ignores_duplicates(tmp_path):
    conn = make_db(tmp_path)
    scan.add_finding(conn, "Galleries/1/1.cbz", "a" * 64, "structural", "path_traversal:../evil.exe")
    scan.add_finding(conn, "Galleries/1/1.cbz", "a" * 64, "structural", "path_traversal:../evil.exe")
    conn.commit()
    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 1
    conn.close()


def test_run_yara_exit1_error_is_scan_error():
    rules, error = scan.run_yara_with_runner(Path("a.cbz"), Path("rules.yar"), "yr", scan.DEFAULT_LIMITS,
                                             runner=lambda cmd: SimpleNamespace(returncode=1, stdout='{"rules":[{"identifier":"Zip_Path_Traversal"}]}'))
    assert rules == []
    assert error == "yara_error:exit_1"


def test_scan_cache_stale_negative_ttl_not_used(tmp_path):
    conn = make_db(tmp_path)
    scan.cache_store(conn, "a" * 64, "unknown_to_vt", "virustotal", "", "unknown_to_vt")
    old = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(time.time() - scan.NEGATIVE_SCAN_CACHE_TTL_SECONDS - 1))
    conn.execute("UPDATE scan_cache SET cached_at = ? WHERE sha256 = ?", (old, "a" * 64))
    conn.commit()
    assert scan.cache_lookup(conn, "a" * 64) == {}
    conn.close()


def test_process_file_uses_transaction_after_external_lookup(tmp_path):
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "51" / "51.cbz", {"ok.webp": PNG})
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    cfg["yr"] = ""
    state = {"window": [], "total": 0}
    summary = {}
    calls = []

    def fetch(url, key):
        calls.append((url, key))
        count = conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0]
        if count != 0:
            raise AssertionError("external lookup must run before committed findings write")
        return {"data": {"attributes": {"last_analysis_stats": {"malicious": 1},
                                        "last_analysis_results": {"E": {"category": "malicious",
                                                                        "engine_name": "E",
                                                                        "result": "R"}}}}}, 200

    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=fetch)
    assert status == "hit"
    assert summary.get("hit") == 1
    assert calls and calls[0][0].endswith("/files/" + scan.sha256_file(cbz))
    row = conn.execute("SELECT rule, detail FROM findings WHERE rule = 'virustotal'").fetchone()
    assert row == ("virustotal", "E: R")
    assert conn.execute("SELECT COUNT(1) FROM scan_cache").fetchone()[0] == 1
    conn.close()


def test_process_file_negative_vt_cache_avoids_fetch(tmp_path):
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "52" / "52.cbz", {"ok.webp": PNG})
    sha = scan.sha256_file(cbz)
    scan.cache_store(conn, sha, "unknown_to_vt", "virustotal", "", "unknown_to_vt")
    conn.commit()

    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    cfg["yr"] = ""
    state = {"window": [], "total": 0}
    summary = {}

    def fetch(url, key):
        raise AssertionError("cached negative VT must not fetch")

    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=fetch)
    assert status == "pass"
    assert summary.get("pass") == 1
    assert summary.get("cached") == 1
    row = conn.execute("SELECT rule, detail FROM findings WHERE rule = 'virustotal'").fetchone()
    assert row == ("virustotal", "unknown_to_vt")
    source = conn.execute("SELECT scan_source FROM files").fetchone()[0]
    assert source == "structural+virustotal"
    conn.close()


def test_process_file_yara_exit1_error_is_scan_error(tmp_path):
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "53" / "53.cbz", {"ok.webp": PNG})
    cfg = json.loads(json.dumps(CFG))
    cfg["yr"] = "tools/yr.exe"
    cfg["virustotal"]["enabled"] = False
    state = {"window": [], "total": 0}
    summary = {}

    runner = lambda cmd: SimpleNamespace(returncode=1, stdout='{"rules":[{"identifier":"Zip_Path_Traversal"}]}')
    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=lambda url, key: {},
                               runner=runner, yr=cfg["yr"])
    assert status == "scan_error"
    assert summary.get("scan_error") == 1
    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 0
    source = conn.execute("SELECT scan_source FROM files").fetchone()[0]
    assert source == "structural+yara_error"
    conn.close()


def test_process_file_rolls_back_when_record_file_fails(tmp_path):
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "54" / "54.cbz", {"../evil.exe": b"MZ\x90\x00", "ok.webp": PNG})
    conn.execute("DROP TABLE files")

    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = False
    cfg["yr"] = ""
    state = {"window": [], "total": 0}
    summary = {}

    with pytest.raises(sqlite3.Error):
        scan.process_file(conn, cbz, root, cfg, state, summary, fetch=lambda url, key: {})

    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 0
    conn.close()
