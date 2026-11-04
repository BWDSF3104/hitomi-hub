package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
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

func TestExportPDFPageObjectDictCountsAreBalanced(t *testing.T) {
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
	if err := recordFile(db, filepath.Join("Galleries", "3", "pnghash.png"), validGalleryID(3), "0", int64(len(buf.Bytes())), "png", "png", "not_evaluated", "structural"); err != nil {
		t.Fatal(err)
	}
	if err := recordPage(db, 3, 1, "pnghash"); err != nil {
		t.Fatal(err)
	}
	result, err := exportPDF(db, exportTestCfg(), root, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(result["output"].(string))
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(body, []byte("3 0 obj\n"))
	if start < 0 {
		t.Fatalf("page object not found in pdf")
	}
	end := bytes.Index(body[start:], []byte("endobj"))
	if end < 0 {
		t.Fatalf("page object end not found in pdf")
	}
	pageObj := body[start : start+end]
	open := bytes.Count(pageObj, []byte("<<"))
	close := bytes.Count(pageObj, []byte(">>"))
	if open != close {
		t.Fatalf("page object dict counts unbalanced: open=%d close=%d obj=%q", open, close, string(pageObj))
	}
	if open == 0 {
		t.Fatalf("page object has no dict markers: obj=%q", string(pageObj))
	}
	if !bytes.Contains(pageObj, []byte("/Type /Page")) {
		t.Errorf("page object must contain /Type /Page: got %q", string(pageObj))
	}
}

// A-11: streaming 化後の peak memory gate。mocked fixture（1300 page）で export 中の
// peak HeapAlloc delta を runtime.MemStats サンプリングで実測し、page 1 分の working set
// （decoder image 4/3P + rawRGB P + zlib 結果上限 P）＋ header/metadata margin に収まることを検証する。
// 全 page 保持（旧実装）なら 1300x360,000 = 468MB で不合格。xref offset の strict parser 回帰も同時に検証。
func TestExportPDFStreamedPeakMemoryFitsSinglePageWorkingSet(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	if err := recordDiscovered(db, 9, "Galleries/9"); err != nil {
		t.Fatal(err)
	}
	const pageCount = 1300
	w, h := 300, 400
	pageBytes := w * h * 3
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA64{uint16(x), uint16(y), 128, 65535})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	pngBytes := buf.Bytes()
	for i := 1; i <= pageCount; i++ {
		hash := fmt.Sprintf("%06d", i)
		writeExportFile(t, root, fmt.Sprintf("Galleries/9/%s.png", hash), pngBytes)
		if err := recordFile(db, filepath.Join("Galleries", "9", hash+".png"), validGalleryID(9), "0", int64(len(pngBytes)), "png", "png", "not_evaluated", "structural"); err != nil {
			t.Fatal(err)
		}
		if err := recordPage(db, 9, i, hash); err != nil {
			t.Fatal(err)
		}
	}

	// gate は 1 page 分の live working set の理論上界（実行中常時 live: decoder image 4/3P +
	// rawRGB buffer P + zlib 結果上限 P + zlib writer internal（32KB window + block buffer））
	// ＋ header/metadata margin（xref offset map・kids・entries）1MB ＋ GC noise margin 8MB
	// （sweep 前 garbage と live 分散による span 保持分。sampling goroutine の MemStats 割当は
	// loop 外へ移出して測定ノイズを最小化済み。実測 10 run で peakDelta 最大 6.3MB、gate に
	// headroom。全 page 保持の旧実装なら N 分（468MB）で不合格）。
	// peak は強制 GC 付きサンプリングで live heap を測る: page ごとの結果は即解放されるため
	// live は 1 page 分＋header/metadata に収まる。全 page 保持（旧実装）は live が N 分（468MB）へ
	// 成長するのでこの測定で不合格になる。加えて export 後（post-GC）の live delta が header/metadata
	// （1MB）以内であることで page ごとの保持がないことを証明する（garbage 影響なし、gate の sharp 側）。
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)

	done := make(chan struct{})
	stopped := make(chan struct{})
	var peak atomic.Uint64
	var ms runtime.MemStats // sampling 自体の per-iteration garbage を避ける（measurement noise 源）
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			default:
			}
			runtime.GC()
			runtime.ReadMemStats(&ms)
			v := ms.HeapAlloc
			if v > peak.Load() {
				peak.Store(v)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	result, err := exportPDF(db, exportTestCfg(), root, 9, "")
	close(done)
	<-stopped
	if err != nil {
		t.Fatal(err)
	}
	if result["pages"] != pageCount {
		t.Fatalf("pages: got %v want %d", result["pages"], pageCount)
	}

	peakDelta := int64(peak.Load()) - int64(base.HeapAlloc)
	runtime.GC()
	var live runtime.MemStats
	runtime.ReadMemStats(&live)
	liveDelta := int64(live.HeapAlloc) - int64(base.HeapAlloc)
	gate := int64(pageBytes)*3 + int64(pageBytes)/3 + 81920 + 1048576 + 8388608
	t.Logf("measured: base=%d peakDelta=%d gate=%d singlePage=%d liveDelta=%d", base.HeapAlloc, peakDelta, gate, pageBytes, liveDelta)
	if peakDelta > gate {
		t.Fatalf("peak memory delta: got %d bytes want <= %d (single page working set + header)", peakDelta, gate)
	}
	if liveDelta > 1048576 {
		t.Fatalf("live heap after export: got %d bytes want <= 1MB (header/metadata only)", liveDelta)
	}

	body, err := os.ReadFile(result["output"].(string))
	if err != nil {
		t.Fatal(err)
	}
	xrefStart := bytes.LastIndex(body, []byte("xref\n0 "))
	if xrefStart < 0 {
		t.Fatal("xref not found")
	}
	trailerStart := bytes.LastIndex(body, []byte("trailer\n<< /Size "))
	if trailerStart < 0 {
		t.Fatal("trailer not found")
	}
	sizeStart := trailerStart + len("trailer\n<< /Size ")
	sizeEnd := sizeStart + bytes.IndexByte(body[sizeStart:], ' ')
	size, err := strconv.Atoi(string(body[sizeStart:sizeEnd]))
	if err != nil {
		t.Fatalf("trailer size parse: %v", err)
	}
	if size != pageCount*3+3 {
		t.Fatalf("trailer size: got %d want %d", size, pageCount*3+3)
	}
	startxrefLine := bytes.LastIndex(body, []byte("startxref\n"))
	if startxrefLine < 0 {
		t.Fatal("startxref not found")
	}
	startxrefLineCopy := startxrefLine + len("startxref\n")
	xrefValEnd := bytes.IndexByte(body[startxrefLineCopy:], '\n')
	startxref, err := strconv.Atoi(string(bytes.TrimSpace(body[startxrefLineCopy : startxrefLineCopy+xrefValEnd])))
	if err != nil {
		t.Fatalf("startxref parse: %v", err)
	}
	if startxref != xrefStart {
		t.Fatalf("startxref: got %d want %d", startxref, xrefStart)
	}
	xrefBlockStart := xrefStart + len("xref\n0 "+strconv.Itoa(size)+"\n")
	lines := bytes.Split(body[xrefBlockStart:trailerStart], []byte("\n"))
	if len(lines) != size+1 {
		t.Fatalf("xref entries: got %d want %d (size %d + trailing newline)", len(lines), size+1, size)
	}
	for i := 1; i < size; i++ {
		line := lines[i]
		offset, err := strconv.Atoi(string(line[:10]))
		if err != nil {
			t.Fatalf("xref entry %d parse: %v line %q", i, err, line)
		}
		want := fmt.Sprintf("%d 0 obj\n", i)
		if !bytes.HasPrefix(body[offset:], []byte(want)) {
			t.Fatalf("xref entry %d offset %d: got %q want %q", i, offset, string(body[offset:min(offset+20, len(body))]), want)
		}
	}
	if !bytes.Contains(body, []byte(fmt.Sprintf("/Count %d", pageCount))) {
		t.Errorf("pdf missing /Count %d", pageCount)
	}
}

func TestExportTraversalUsesSharedSegmentCriteria(t *testing.T) {
	for _, c := range []struct {
		name string
		want bool
	}{
		{"Vol...01.webp", false},
		{"..evil.webp", false},
		{"normal.webp", false},
		{"../evil.exe", true},
		{"a/../b.webp", true},
		{"a\\..\\b.webp", true},
		{"normal/01.webp", true},
	} {
		if got := containsTraversal(c.name); got != c.want {
			t.Errorf("containsTraversal(%q): got %v want %v", c.name, got, c.want)
		}
	}
	for _, name := range []string{"Vol...01.webp", "..evil.webp", "normal.webp", "../evil.exe", "a/../b.webp", "a\\..\\b.webp"} {
		if containsTraversal(name) != containsDotDotSegment(name) {
			t.Errorf("criterion drift: containsTraversal(%q) != containsDotDotSegment(%q)", name, name)
		}
	}
}

func TestExportCBZExportsLegitDotDotName(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	if err := recordDiscovered(db, 5, "Galleries/5"); err != nil {
		t.Fatal(err)
	}
	writeExportFile(t, root, "Galleries/5/Vol...01.webp", []byte("webp-bytes"))
	if err := recordFile(db, filepath.Join("Galleries", "5", "Vol...01.webp"), validGalleryID(5), "0", 10, "webp", "webp", "not_evaluated", "structural"); err != nil {
		t.Fatal(err)
	}
	result, err := exportCBZ(db, exportTestCfg(), root, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if result["entries"] != 1 {
		t.Fatalf("export: %v", result)
	}
	if result["skipped"].(map[string]int)["path_traversal"] != 0 {
		t.Fatalf("skipped: %v", result["skipped"])
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
	if len(reader.File) != 1 || reader.File[0].Name != "001_Vol...01.webp" {
		t.Fatalf("zip entries: %v", namesOf(reader.File))
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
