package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func validGalleryID(id int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(id), Valid: true}
}

func exportTestCfg() Config {
	return Config{Limits: defaultLimits(), Hitomi: defaultHitomiConfig()}
}

func writeExportFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExportCBZUsesPageOrderAndSkipsHit(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	if err := recordDiscovered(db, 1, "Galleries/1"); err != nil {
		t.Fatal(err)
	}
	if err := recordPage(db, 1, 1, "aaaa"); err != nil {
		t.Fatal(err)
	}
	if err := recordPage(db, 1, 2, "bbbb"); err != nil {
		t.Fatal(err)
	}
	writeExportFile(t, root, "Galleries/1/aaaa.webp", []byte("webp-bytes"))
	writeExportFile(t, root, "Galleries/1/bbbb.webp", []byte("hit-bytes"))
	for _, row := range []struct {
		rel, status string
	}{
		{"Galleries/1/aaaa.webp", "not_evaluated"},
		{"Galleries/1/bbbb.webp", "hit"},
	} {
		if err := recordFile(db, filepath.Join("Galleries", "1", filepath.Base(row.rel)), validGalleryID(1), "0", 10, "webp", "webp", row.status, "structural"); err != nil {
			t.Fatal(err)
		}
	}
	result, err := exportCBZ(db, exportTestCfg(), root, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if result["order"] != "pageorder" || result["entries"] != 1 {
		t.Fatalf("export: %v", result)
	}
	if result["skipped"].(map[string]int)["hit"] != 1 {
		t.Fatalf("skipped: %v", result["skipped"])
	}
	file, err := os.Open(result["output"].(string))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := zip.NewReader(file, int64(zipStatSize(t, result["output"].(string))))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != "001_aaaa.webp" {
		t.Fatalf("zip entries: %v", namesOf(reader.File))
	}
	if reader.File[0].Method != 0 {
		t.Errorf("zip method: got %d want Store (0)", reader.File[0].Method)
	}
}

func zipStatSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func namesOf(files []*zip.File) []string {
	out := []string{}
	for _, f := range files {
		out = append(out, f.Name)
	}
	sort.Strings(out)
	return out
}

func TestExportCBZFallsBackToInsertionOrder(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	if err := recordDiscovered(db, 2, "Galleries/2"); err != nil {
		t.Fatal(err)
	}
	writeExportFile(t, root, "Galleries/2/cc.webp", []byte("first"))
	writeExportFile(t, root, "Galleries/2/dd.webp", []byte("second"))
	for _, name := range []string{"cc.webp", "dd.webp"} {
		if err := recordFile(db, filepath.Join("Galleries", "2", name), validGalleryID(2), "0", 6, "webp", "webp", "not_evaluated", "structural"); err != nil {
			t.Fatal(err)
		}
	}
	result, err := exportCBZ(db, exportTestCfg(), root, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if result["order"] != "insertion" || result["entries"] != 2 {
		t.Fatalf("export: %v", result)
	}
	file, err := os.Open(result["output"].(string))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := zip.NewReader(file, zipStatSize(t, result["output"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	got := namesOf(reader.File)
	if len(got) != 2 || got[0] != "001_cc.webp" || got[1] != "002_dd.webp" {
		t.Fatalf("zip entries: %v", got)
	}
}

func TestExportPDFWritesPagesAndSkipsUndecodable(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	if err := recordDiscovered(db, 3, "Galleries/3"); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	writeExportFile(t, root, "Galleries/3/pnghash.png", buf.Bytes())
	writeExportFile(t, root, "Galleries/3/avifhash.avif", []byte("not-an-avif"))
	for _, name := range []string{"pnghash.png", "avifhash.avif"} {
		if err := recordFile(db, filepath.Join("Galleries", "3", name), validGalleryID(3), "0", int64(len(buf.Bytes())), "png", "png", "not_evaluated", "structural"); err != nil {
			t.Fatal(err)
		}
	}
	if err := recordPage(db, 3, 1, "pnghash"); err != nil {
		t.Fatal(err)
	}
	if err := recordPage(db, 3, 2, "avifhash"); err != nil {
		t.Fatal(err)
	}
	result, err := exportPDF(db, exportTestCfg(), root, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if result["pages"] != 1 || result["skipped"].(map[string]int)["undecodable"] != 1 {
		t.Fatalf("export: %v", result)
	}
	body, err := os.ReadFile(result["output"].(string))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"%PDF-1.4", "/Type /Pages", "/Count 1", "/MediaBox [0 0 2 1]", "xref", "startxref", "%%EOF"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("pdf missing %q", want)
		}
	}
	if text == "" {
		t.Fatal("empty pdf")
	}
}

func TestExportRefusesOutputOutsideRoot(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	if err := recordDiscovered(db, 4, "Galleries/4"); err != nil {
		t.Fatal(err)
	}
	writeExportFile(t, root, "Galleries/4/hh.webp", []byte("data"))
	if err := recordFile(db, filepath.Join("Galleries", "4", "hh.webp"), validGalleryID(4), "0", 4, "webp", "webp", "not_evaluated", "structural"); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(root), "escape.cbz")
	if _, err := exportCBZ(db, exportTestCfg(), root, 4, outside); err == nil {
		t.Fatal("expected output-outside-root error")
	}
}
