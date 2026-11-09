"""src/scan.py（hitomi-hub スキャン層）tests（tmp_path・VirusTotal は mock・外部 API なし）。

Run from project root:
    python -m pytest tests/test_hitomi_scan.py -v
"""

from __future__ import annotations

import json
import socket
import sqlite3
import sys
import time
import urllib.error
import urllib.request
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


def test_detect_magic_7z_canonical_signature():
    # R-03: 正典 signature 6 bytes 37 7A BC AF 27 1C（7zFormat.txt・py7zr normative・実ファイル実測で一致）。fixture canonical_271c.bin と同バイト
    head = b"7z\xbc\xaf\x27\x1c" + b"\x00\x04" + b"\x00" * 56
    assert scan.detect_magic(head) == "7z"


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


def test_zip_dotdot_is_path_segment_only(tmp_path):
    legit = make_zip(tmp_path / "Galleries" / "123" / "legit.cbz", {"Vol...01.webp": PNG, "ok.webp": PNG})
    _, _, findings, status = scan.scan_file(legit, CFG)
    assert status == "pass"
    assert findings == []
    evil = make_zip(tmp_path / "Galleries" / "123" / "evil.cbz", {"../evil.exe": PNG})
    _, _, findings, status = scan.scan_file(evil, CFG)
    assert status == "hit"
    assert "path_traversal:../evil.exe" in findings
    assert not scan.has_dot_dot_segment("Vol...01.webp")
    assert scan.has_dot_dot_segment("../evil.exe")
    assert scan.has_dot_dot_segment("a/../b.webp")
    assert scan.has_dot_dot_segment("a\\..\\b.webp")
    assert scan.has_dot_dot_segment("foo/..")
    assert scan.has_dot_dot_segment("..")
    assert not scan.has_dot_dot_segment("..evil.webp")
    assert not scan.has_dot_dot_segment("a/...b.webp")


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


def make_pe_stub(path: Path, e_lfanew: int, pe_sig: bool) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    data = bytearray(b"\x00" * max(e_lfanew + 4, 64))
    data[0:2] = b"MZ"
    data[0x3C:0x40] = e_lfanew.to_bytes(4, "little")
    if pe_sig:
        data[e_lfanew:e_lfanew + 4] = b"PE\x00\x00"
    path.write_bytes(bytes(data))
    return path


def test_pe_signature_beyond_head_window_is_not_mz_hit(tmp_path):
    # D-04: e_lfanew >= 64（0x80）で seek 判定。合格条件: mz_without_pe_header が出ない
    path = make_pe_stub(tmp_path / "Galleries" / "102" / "b.bin", 0x80, True)
    _, magic, findings, status = scan.scan_file(path, CFG)
    assert magic == "pe"
    assert "mz_without_pe_header" not in findings
    assert findings == []
    assert status == "not_evaluated"


def test_pe_signature_within_head_window_regression(tmp_path):
    path = make_pe_stub(tmp_path / "Galleries" / "102" / "legacy.bin", 0x38, True)
    _, _, findings, status = scan.scan_file(path, CFG)
    assert findings == []
    assert status == "not_evaluated"


def test_mz_stub_claiming_offset_without_pe_still_hits(tmp_path):
    path = make_pe_stub(tmp_path / "Galleries" / "102" / "stub.bin", 0x80, False)
    _, _, findings, status = scan.scan_file(path, CFG)
    assert findings == ["mz_without_pe_header"]
    assert status == "hit"


def test_true_mz_only_still_hits_regression(tmp_path):
    path = make_pe_stub(tmp_path / "Galleries" / "102" / "mzonly.bin", 0, False)
    _, _, findings, status = scan.scan_file(path, CFG)
    assert findings == ["mz_without_pe_header"]
    assert status == "hit"


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
    legit = make_zip(tmp_path / "101.cbz", {"Vol...01.webp": PNG})
    rules = scan.run_yara(legit, DATA_DIR / "rules" / "structural.yar", str(yr), CFG["limits"])
    assert "Zip_Path_Traversal" not in rules
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


def test_findings_unique_index_key_includes_path(tmp_path):
    conn = make_db(tmp_path)
    unique = [i for i in conn.execute("PRAGMA index_list(findings)").fetchall() if i[1] == "idx_findings_unique"]
    assert unique, "new DB must have unique index idx_findings_unique"
    assert unique[0][2] == 1
    key = [c[2] for c in conn.execute("PRAGMA index_info('idx_findings_unique')").fetchall()]
    assert key == ["path", "sha256", "rule", "detail"]
    conn.close()


def test_ensure_schema_adds_findings_resolved_at_without_deleting_rows(tmp_path):
    # R-17（B1・Go ensureFindingsResolvedAt 対称）: resolved_at 列追加は列追加のみ・
    # 既存行を削除しない（D-03 非破壊迁移規約）。既存 DB 形状（列なし）から ensure_schema で揃う
    conn = scan.open_db(tmp_path / "hitomi.db")
    conn.execute("""CREATE TABLE findings (
        id INTEGER PRIMARY KEY, path TEXT NOT NULL, sha256 TEXT NOT NULL,
        rule TEXT NOT NULL, detail TEXT, found_at TEXT NOT NULL)""")
    conn.execute("INSERT INTO findings (path, sha256, rule, detail, found_at) VALUES (?, ?, ?, ?, ?)",
                 ("Galleries/1/1.cbz", "a" * 64, "structural", "bad_zip", scan.now_iso()))
    conn.commit()
    scan.ensure_schema(conn, DATA_DIR / "schema.sql")
    have = [row[1] for row in conn.execute("PRAGMA table_info(findings)")]
    assert "resolved_at" in have, "findings must have resolved_at column after ensure_schema"
    row = conn.execute("SELECT path, sha256, rule, detail, resolved_at FROM findings").fetchone()
    assert row == ("Galleries/1/1.cbz", "a" * 64, "structural", "bad_zip", None), "existing row must survive"
    conn.close()


def test_rescan_content_change_marks_stale_findings_resolved(tmp_path):
    # R-17 合格条件固定（B1）: 同一 path で内容（sha256）が変わる再 scan で旧 sha256 行が
    # stale マーキング（resolved_at）され、現行判定（join view）と食い違わない。残存行は歴史として
    # 削除せず維持（D-03）。sha 一致の行は非更新・resolved_at 済み行の再マーキングもしない。
    # （UI 読み取り口なし・直接 SQL 照会の意味の実測固定。VT なし・structural のみ）
    conn = make_db(tmp_path)
    root = tmp_path
    state = {"window": [], "total": 0}

    def join_view(conn, unresolved_only=False):
        sql = ("SELECT COUNT(1) FROM findings f JOIN files fi ON fi.path = f.path AND fi.sha256 = f.sha256"
               + (" AND f.resolved_at IS NULL" if unresolved_only else ""))
        return conn.execute(sql).fetchone()[0]

    cbz = make_zip(root / "Galleries" / "71" / "71.cbz", {"../evil.exe": b"MZ\x90\x00"})
    assert scan.process_file(conn, cbz, root, CFG, state, {}) == "hit"
    rows = conn.execute("SELECT sha256, resolved_at FROM findings").fetchall()
    assert len(rows) == 2 and all(r[1] is None for r in rows)
    assert join_view(conn) == 2

    # 内容変化（clean zip → pass）。旧行はマーキング・join view 0（現判定と食い違わない）・findings は消えない
    make_zip(cbz, {"ok.webp": PNG})
    assert scan.process_file(conn, cbz, root, CFG, state, {}) == "pass"
    total, marked = conn.execute("SELECT COUNT(1), COUNT(resolved_at) FROM findings").fetchone()
    assert total == 2 and marked == 2, f"stale rows must be marked and kept: total {total} marked {marked}"
    assert join_view(conn) == 0

    # 同一 content 再実行: 既存マーキングは再マーキング（上書き）なし・新行なし（D-03 行数不変）
    resolved_before = conn.execute("SELECT resolved_at FROM findings ORDER BY rule, detail").fetchall()
    assert scan.process_file(conn, cbz, root, CFG, state, {}) == "pass"
    resolved_after = conn.execute("SELECT resolved_at FROM findings ORDER BY rule, detail").fetchall()
    assert resolved_after == resolved_before, "stale row must not be re-marked on same-content re-scan"

    # 内容変化・再 hit: 新行は unresolved・join view = 新行のみ（現判定と一致）
    make_zip(cbz, {"../bad.exe": b"MZ\x90\x00"})
    assert scan.process_file(conn, cbz, root, CFG, state, {}) == "hit"
    total, marked = conn.execute("SELECT COUNT(1), COUNT(resolved_at) FROM findings").fetchone()
    assert total == 4 and marked == 2, f"new findings must be unresolved, stale must remain marked: total {total} marked {marked}"
    assert join_view(conn, unresolved_only=True) == 2
    conn.close()


def test_same_content_different_paths_are_separate_findings(tmp_path):
    conn = make_db(tmp_path)
    scan.add_finding(conn, "Galleries/1/1.cbz", "a" * 64, "structural", "bad_zip")
    scan.add_finding(conn, "Galleries/2/2.cbz", "a" * 64, "structural", "bad_zip")
    conn.commit()
    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 2
    conn.close()


def test_second_scan_does_not_change_findings_count(tmp_path):
    conn = make_db(tmp_path)
    cbz = make_zip(tmp_path / "Galleries" / "60" / "60.cbz", {"../evil.exe": b"MZ\x90\x00"})
    state = {"window": [], "total": 0}
    summary = {}
    scan.process_file(conn, cbz, tmp_path, CFG, state, summary)
    first = conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0]
    assert first > 0
    scan.process_file(conn, cbz, tmp_path, CFG, state, summary)
    second = conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0]
    assert second == first, f"second scan must skip duplicates: {first} -> {second}"
    conn.close()


def test_run_yara_exit1_error_is_scan_error():
    rules, error = scan.run_yara_with_runner(Path("a.cbz"), Path("rules.yar"), "yr", CFG["limits"],
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


def test_process_file_yara_exit1_error_keeps_not_evaluated_and_not_finding(tmp_path):
    # B-07残件①（Go 正典表現への parity・`app/scan_test.go` の同名 Go テストと同形）:
    # YARA error で構造 pass の scan_status を scan_error へ上書きせず not_evaluated 維持。
    # export skip 条件（hit/scan_error）は変更なし。YARA error は not_evaluated なので
    # export される（Go 側 TestExportCBZExportsStructuralPassWithYaraErrorAsNotEvaluated と同一挙動。
    # Python 参照実装に export 経路はないため、status が skip 集合に属さないことで固定）
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
    assert status == "not_evaluated"
    assert summary.get("not_evaluated") == 1
    assert summary.get("scan_error", 0) == 0
    assert status not in ("hit", "scan_error")  # export skip 条件（hit/scan_error）で skip されない
    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 0
    source = conn.execute("SELECT scan_source FROM files").fetchone()[0]
    assert source == "structural+yara_error"
    conn.close()


def test_process_file_yara_execution_failure_keeps_not_evaluated(tmp_path):
    # 回帰対照（Go TestProcessFileYaraExecutionFailureKeepsNotEvaluated）: runner 例外も同表現
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "58" / "58.cbz", {"ok.webp": PNG})
    cfg = json.loads(json.dumps(CFG))
    cfg["yr"] = "tools/yr.exe"
    cfg["virustotal"]["enabled"] = False
    state = {"window": [], "total": 0}
    summary = {}

    def runner(cmd):
        raise OSError("missing yr")

    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=lambda url, key: {},
                               runner=runner, yr=cfg["yr"])
    assert status == "not_evaluated"
    assert summary.get("not_evaluated") == 1
    assert summary.get("scan_error", 0) == 0
    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 0
    assert conn.execute("SELECT scan_source FROM files").fetchone()[0] == "structural+yara_error"
    conn.close()


def test_process_file_structural_hit_keeps_hit_on_yara_error(tmp_path):
    # 回帰（Go TestProcessFileStructuralHitKeepsHitOnYaraError）: hit は hit のまま・findings 1
    conn = make_db(tmp_path)
    root = tmp_path
    # Go 同一 fixture（`../evil.txt`・path_traversal のみ）。`../evil.exe` は executable_entry も hit し findings 2 になる
    cbz = make_zip(root / "Galleries" / "59" / "59.cbz", {"../evil.txt": PNG})
    cfg = json.loads(json.dumps(CFG))
    cfg["yr"] = "tools/yr.exe"
    cfg["virustotal"]["enabled"] = False
    state = {"window": [], "total": 0}
    summary = {}

    runner = lambda cmd: SimpleNamespace(returncode=1, stdout="")
    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=lambda url, key: {},
                               runner=runner, yr=cfg["yr"])
    assert status == "hit"
    assert summary.get("hit") == 1
    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 1
    row = conn.execute("SELECT scan_status, scan_source FROM files").fetchone()
    assert row == ("hit", "structural+yara_error")
    conn.close()


def test_process_file_bad_zip_scan_error_keeps_scan_error_without_yara_call(tmp_path):
    # 回帰（Go TestProcessFileBadZipScanErrorKeepsScanErrorWithoutYaraCall）: 構造 scan_error は不変・YARA 呼出 0
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = root / "Galleries" / "60" / "60.cbz"
    cbz.parent.mkdir(parents=True, exist_ok=True)
    cbz.write_bytes(b"PK\x03\x04garbage")
    cfg = json.loads(json.dumps(CFG))
    cfg["yr"] = "tools/yr.exe"
    cfg["virustotal"]["enabled"] = False
    state = {"window": [], "total": 0}
    summary = {}
    calls = []

    def runner(cmd):
        calls.append(cmd)
        return SimpleNamespace(returncode=0, stdout="")

    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=lambda url, key: {},
                               runner=runner, yr=cfg["yr"])
    assert status == "scan_error"
    assert summary.get("scan_error") == 1
    assert calls == []
    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 1
    row = conn.execute("SELECT scan_status, scan_source FROM files").fetchone()
    assert row == ("scan_error", "structural")
    conn.close()


def test_process_file_yara_success_no_rules_keeps_pass(tmp_path):
    # 回帰（Go TestProcessFileYaraSuccessNoRulesKeepsPass）: YARA 成功で rule なしは構造 pass 維持
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "61" / "61.cbz", {"ok.webp": PNG})
    cfg = json.loads(json.dumps(CFG))
    cfg["yr"] = "tools/yr.exe"
    cfg["virustotal"]["enabled"] = False
    state = {"window": [], "total": 0}
    summary = {}

    runner = lambda cmd: SimpleNamespace(returncode=0, stdout="")
    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=lambda url, key: {},
                               runner=runner, yr=cfg["yr"])
    assert status == "pass"
    assert summary.get("pass") == 1
    assert conn.execute("SELECT COUNT(1) FROM findings").fetchone()[0] == 0
    assert conn.execute("SELECT scan_source FROM files").fetchone()[0] == "structural"
    conn.close()


def test_process_file_yara_error_file_enters_vt_gate(tmp_path):
    # B-07残件②（decision 2026-11-02 #3 と整合・現状許容）: not_evaluated の YARA error ファイルは
    # VT gate（status not in hit/scan_error）に入る。VT gate・quota・TTL・negative cache の意味は不変
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "62" / "62.cbz", {"ok.webp": PNG})
    sha = scan.sha256_file(cbz)
    cfg = json.loads(json.dumps(CFG))
    cfg["yr"] = "tools/yr.exe"
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    state = {"window": [], "total": 0}
    summary = {}
    calls = []

    def fetch(url, key):
        calls.append(url)
        return {}, 404

    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=fetch,
                               runner=lambda cmd: SimpleNamespace(returncode=1, stdout=""), yr=cfg["yr"])
    assert status == "not_evaluated"
    assert len(calls) == 1  # YARA error で VT gate に入る（構造 scan_error と異なり gate 通過）
    assert calls[0].endswith("/files/" + sha)
    row = conn.execute("SELECT verdict, detail FROM scan_cache WHERE sha256 = ?", (sha,)).fetchone()
    assert row == ("unknown_to_vt", "unknown_to_vt")  # negative cache 意味不変
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


def test_vt_empty_results_with_stats_total_is_not_evaluated(tmp_path):
    # B-06 合格条件: stats.total > 0 なのに last_analysis_results が空 → not_evaluated（pass にしない）
    conn = make_db(tmp_path)
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"

    def fetch(url, key):
        return {"data": {"attributes": {"last_analysis_stats": {"malicious": 0, "suspicious": 0,
                                        "harmless": 60, "undetected": 40, "total": 100},
                                        "last_analysis_results": {}}}}, 200

    result = scan.vt_lookup("f" * 64, cfg, {"window": [], "total": 0}, fetch=fetch, conn=conn)
    assert result["status"] == "not_evaluated"
    assert result["positives"] == 0
    assert result["detail"] == ""
    assert scan.cache_lookup(conn, "f" * 64) == {}
    conn.close()


def test_process_file_vt_empty_results_does_not_store_pass(tmp_path):
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "55" / "55.cbz", {"ok.webp": PNG})
    sha = scan.sha256_file(cbz)
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    cfg["yr"] = ""
    state = {"window": [], "total": 0}
    summary = {}

    def fetch(url, key):
        return {"data": {"attributes": {"last_analysis_stats": {"malicious": 0, "suspicious": 0, "total": 100},
                                        "last_analysis_results": {}}}}, 200

    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=fetch)
    assert status == "pass"
    assert scan.cache_lookup(conn, sha) == {}
    assert conn.execute("SELECT COUNT(1) FROM scan_cache").fetchone()[0] == 0
    assert conn.execute("SELECT COUNT(1) FROM scan_cache WHERE verdict = 'pass'").fetchone()[0] == 0
    row = conn.execute("SELECT rule, detail FROM findings WHERE rule = 'virustotal'").fetchone()
    assert row == ("virustotal", "api_error:not_evaluated")
    conn.close()


def test_vt_positives_hit_and_cache_verdict_unchanged(tmp_path):
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "56" / "56.cbz", {"ok.webp": PNG})
    sha = scan.sha256_file(cbz)
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    cfg["yr"] = ""
    state = {"window": [], "total": 0}
    summary = {}

    def fetch(url, key):
        return {"data": {"attributes": {"last_analysis_stats": {"malicious": 2, "suspicious": 0, "total": 100},
                                        "last_analysis_results": {"E": {"category": "malicious",
                                                                        "engine_name": "E",
                                                                        "result": "R"}}}}}, 200

    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=fetch)
    assert status == "hit"
    assert scan.cache_lookup(conn, sha)["verdict"] == "hit"
    conn.close()


def test_vt_404_unknown_to_vt_path_and_negative_cache_unchanged(tmp_path):
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "57" / "57.cbz", {"ok.webp": PNG})
    sha = scan.sha256_file(cbz)
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    cfg["yr"] = ""
    state = {"window": [], "total": 0}
    summary = {}

    status = scan.process_file(conn, cbz, root, cfg, state, summary, fetch=lambda url, key: ({}, 404))
    assert status == "pass"
    row = conn.execute("SELECT verdict, detail FROM scan_cache WHERE sha256 = ?", (sha,)).fetchone()
    assert row == ("unknown_to_vt", "unknown_to_vt")
    conn.close()


def test_pass_verdict_cache_keeps_positive_ttl_semantics(tmp_path):
    conn = make_db(tmp_path)
    scan.cache_store(conn, "g" * 64, "pass", "virustotal", "", "positives=0")
    conn.commit()
    aged = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(time.time() - scan.NEGATIVE_SCAN_CACHE_TTL_SECONDS - 1))
    conn.execute("UPDATE scan_cache SET cached_at = ? WHERE sha256 = ?", (aged, "g" * 64))
    conn.commit()
    assert scan.cache_lookup(conn, "g" * 64)["verdict"] == "pass"
    assert scan.SCAN_CACHE_TTL_SECONDS == 30 * 24 * 3600
    assert scan.NEGATIVE_SCAN_CACHE_TTL_SECONDS == 2 * 3600
    conn.close()
    conn.close()


def test_vt_urlerror_on_fetch_none_path_converts_to_network_error(tmp_path, monkeypatch):
    # B-10 残件（2026-11-04 推奨承認 #3）: fetch=None path の URLError → tuple 規約 (payload, 0) → network_error
    conn = make_db(tmp_path)
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    state = {"window": [], "total": 0}

    def fake_urlopen(req, timeout=30):
        raise urllib.error.URLError("mock network failure")

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)
    result = scan.vt_lookup("a" * 64, cfg, state, conn=conn)
    assert result == {"status": "network_error", "positives": 0, "threat_name": "",
                      "detail": "network_error", "http_status": 0}
    assert state["total"] == 1
    assert len(state["window"]) == 1
    assert scan.cache_lookup(conn, "a" * 64) == {}
    conn.close()


def test_vt_socket_timeout_on_fetch_none_path_converts_to_network_error(tmp_path, monkeypatch):
    conn = make_db(tmp_path)
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    state = {"window": [], "total": 0}

    class TimeoutReader:
        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc_value, tb):
            return False

        def read(self, size=-1):
            raise socket.timeout("mock read timeout")

    def fake_urlopen(req, timeout=30):
        return TimeoutReader()

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)
    result = scan.vt_lookup("b" * 64, cfg, state, conn=conn)
    assert result["status"] == "network_error"
    assert result["http_status"] == 0
    assert result["detail"] == "network_error"
    assert scan.cache_lookup(conn, "b" * 64) == {}
    conn.close()


def test_process_file_network_error_uses_api_error_convention_without_negative_cache(tmp_path, monkeypatch):
    conn = make_db(tmp_path)
    root = tmp_path
    cbz = make_zip(root / "Galleries" / "58" / "58.cbz", {"ok.webp": PNG})
    sha = scan.sha256_file(cbz)
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    cfg["yr"] = ""
    state = {"window": [], "total": 0}
    summary = {}

    def fake_urlopen(req, timeout=30):
        raise urllib.error.URLError("mock network failure")

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)
    status = scan.process_file(conn, cbz, root, cfg, state, summary)
    assert status == "pass"
    row = conn.execute("SELECT rule, detail FROM findings WHERE rule = 'virustotal'").fetchone()
    assert row == ("virustotal", "api_error:network_error")
    # negative cache は unknown_to_vt のみ（network_error は権威ある「unknown」ではない）→ 行なし
    assert conn.execute("SELECT COUNT(1) FROM scan_cache").fetchone()[0] == 0
    conn.close()


def test_scan_loop_without_try_survives_network_error(tmp_path, monkeypatch):
    # main() 同形 loop（try なし）で exception が発生しないこと（crash しない）
    conn = make_db(tmp_path)
    root = tmp_path
    make_zip(root / "Galleries" / "59" / "59.cbz", {"ok.webp": PNG})
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    cfg["yr"] = ""
    state = {"window": [], "total": 0}
    summary = {"pass": 0, "hit": 0, "not_evaluated": 0, "scan_error": 0, "cached": 0}

    def fake_urlopen(req, timeout=30):
        raise urllib.error.URLError("mock network failure")

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)
    for path in sorted((root / "Galleries").rglob("*")):
        if not path.is_file():
            continue
        scan.process_file(conn, path, root, cfg, state, summary)
    assert summary["pass"] == 1
    assert state["total"] == 1
    conn.close()


def test_vt_httperror_branch_on_fetch_none_path_unchanged(tmp_path, monkeypatch):
    # 回帰: B-10 の HTTPError 分岐（fetch=None path）の意味不変。404→unknown_to_vt+negative cache・429→rate_limited・500→not_evaluated・401→auth_error
    conn = make_db(tmp_path)
    cfg = json.loads(json.dumps(CFG))
    cfg["virustotal"]["enabled"] = True
    cfg["virustotal"]["api_key"] = "testkey"
    state = {"window": [], "total": 0}
    codes = [404, 429, 500, 401]

    def fake_urlopen(req, timeout=30):
        raise urllib.error.HTTPError("https://www.virustotal.com/api/v3/files/x", codes[0], "mock", {}, None)

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)
    r404 = scan.vt_lookup("c" * 64, cfg, state, conn=conn)
    assert (r404["status"], r404["http_status"]) == ("unknown_to_vt", 404)
    row = conn.execute("SELECT verdict, detail FROM scan_cache WHERE sha256 = ?", ("c" * 64,)).fetchone()
    assert row == ("unknown_to_vt", "unknown_to_vt")
    codes[0] = 429
    r429 = scan.vt_lookup("d" * 64, cfg, state, conn=conn)
    assert (r429["status"], r429["http_status"]) == ("rate_limited", 429)
    codes[0] = 500
    r500 = scan.vt_lookup("e" * 64, cfg, state, conn=conn)
    assert (r500["status"], r500["http_status"]) == ("not_evaluated", 500)
    codes[0] = 401
    r401 = scan.vt_lookup("9" * 64, cfg, state, conn=conn)
    assert (r401["status"], r401["http_status"]) == ("auth_error", 401)
    assert state["total"] == 4
    conn.close()
