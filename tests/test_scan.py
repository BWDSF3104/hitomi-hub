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

import pytest

SCRIPT_DIR = Path(__file__).resolve().parents[1] / "src"

sys.path.insert(0, str(SCRIPT_DIR))

import scan  # noqa: E402

CFG = json.loads((SCRIPT_DIR / "config.example.json").read_text(encoding="utf-8"))
PNG = b"\x89PNG\r\n\x1a\n" + b"\x00" * 32


def make_db(tmp_path: Path) -> sqlite3.Connection:
    conn = scan.open_db(tmp_path / "hitomi.db")
    scan.ensure_schema(conn, SCRIPT_DIR / "schema.sql")
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
        return {"data": {"attributes": {"stats": {"malicious": 0, "suspicious": 0},
                                        "last_analysis_results": {}}}}

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
