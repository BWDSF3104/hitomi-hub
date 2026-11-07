package main

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
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

// R-14: fast path（型 switch）は opaque 内容で At() 換算と byte 一致。alpha は stored 値
// （premultiply ではなく）を用い、非 opaque alpha がある page だけ alpha buffer を返す。
func TestExportPDFRawRGBFastPathPixelParity(t *testing.T) {
	refRGB := func(img image.Image) []byte {
		bounds := img.Bounds()
		out := make([]byte, 0, bounds.Dx()*bounds.Dy()*3)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				out = append(out, byte(r>>8), byte(g>>8), byte(b>>8))
			}
		}
		return out
	}
	rng := rand.New(rand.NewSource(1))

	w, h := 37, 23
	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range nrgba.Pix {
		nrgba.Pix[i] = uint8(rng.Intn(256))
	}
	for i := 3; i < len(nrgba.Pix); i += 4 {
		nrgba.Pix[i] = 255
	}
	rgb, alpha, nonOpaque := rawRGB(nrgba, nil, nil)
	if nonOpaque || alpha != nil {
		t.Fatalf("NRGBA opaque: got nonOpaque=%v alpha=%v", nonOpaque, alpha)
	}
	if !bytes.Equal(rgb, refRGB(nrgba)) {
		t.Fatalf("NRGBA opaque: fast path bytes != At() bytes")
	}

	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range rgba.Pix {
		rgba.Pix[i] = uint8(rng.Intn(256))
	}
	for i := 3; i < len(rgba.Pix); i += 4 {
		rgba.Pix[i] = 255
	}
	rgb, alpha, nonOpaque = rawRGB(rgba, nil, nil)
	if nonOpaque || alpha != nil {
		t.Fatalf("RGBA opaque: got nonOpaque=%v", nonOpaque)
	}
	if !bytes.Equal(rgb, refRGB(rgba)) {
		t.Fatalf("RGBA opaque: fast path bytes != At() bytes")
	}

	// 個別 ratio の添字（COffset）一致を odd size で確認。
	for _, ratio := range []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422,
		image.YCbCrSubsampleRatio420, image.YCbCrSubsampleRatio440,
		image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410,
	} {
		w, h := 37, 23
		ycc := image.NewYCbCr(image.Rect(0, 0, w, h), ratio)
		for i := range ycc.Y {
			ycc.Y[i] = uint8(rng.Intn(256))
		}
		for i := range ycc.Cb {
			ycc.Cb[i] = uint8(rng.Intn(256))
			ycc.Cr[i] = uint8(rng.Intn(256))
		}
		rgb, _, _ := rawRGB(ycc, nil, nil)
		if !bytes.Equal(rgb, refRGB(ycc)) {
			t.Fatalf("YCbCr %v: fast path bytes != At() bytes", ratio)
		}
	}

	// NYCbCrA（alpha あり）の stored 値 semantics: pixel (1,0) semi-transparent, (1,2) fully transparent。
	w, h = 5, 4
	a := image.NewNYCbCrA(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for i := range a.Y {
		a.Y[i] = 128
	}
	for i := range a.Cb {
		a.Cb[i] = 128
		a.Cr[i] = 128
	}
	for i := range a.A {
		a.A[i] = 255
	}
	a.A[a.YOffset(1, 0)] = 128
	a.A[a.YOffset(1, 2)] = 0
	rgb, alpha, nonOpaque = rawRGB(a, nil, nil)
	if !nonOpaque || len(alpha) != w*h {
		t.Fatalf("NYCbCrA alpha: nonOpaque=%v alpha len=%d", nonOpaque, len(alpha))
	}
	if alpha[1] != 128 || alpha[11] != 0 { // j = y*w + x: (1,0)=1, (1,2)=11
		t.Fatalf("NYCbCrA alpha stream: got %d/%d want 128/0", alpha[1], alpha[11])
	}
	// Y=128,Cb=128,Cr=128 → stored RGB (128,128,128)。At() premultiply なら alpha=0/128 で 0 に寄る
	// （(1,2) は At() 経由で 0 になるはず → fast path は stored 128 を保つ）
	if rgb[3] != 128 || rgb[33] != 128 { // pixel (1,0)=j1→rgb idx 3, (1,2)=j11→rgb idx 33
		t.Fatalf("NYCbCrA stored RGB: got %d/%d want 128/128", rgb[3], rgb[33])
	}
	if ref := refRGB(a); ref[33] == 128 {
		t.Fatalf("At() reference should premultiply (1,2) to 0 for this fixture (alpha=0)")
	}

	// NRGBA alpha（record の fixture 相当）: (0,1)=(40,70,80,0), (0,2)=(40,80,80,128)。
	nrgba2 := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	nrgba2.SetNRGBA(0, 1, color.NRGBA{40, 70, 80, 0})
	nrgba2.SetNRGBA(0, 2, color.NRGBA{40, 80, 80, 128})
	nrgba2.SetNRGBA(1, 3, color.NRGBA{50, 90, 85, 255})
	rgb, alpha, nonOpaque = rawRGB(nrgba2, nil, nil)
	if !nonOpaque {
		t.Fatal("NRGBA alpha: expected nonOpaque")
	}
	// j = y*w + x。pixel (0,1): stored (40,70,80)（At() premultiply なら 0 に寄る）
	if rgb[24] != 40 || rgb[25] != 70 || rgb[26] != 80 || alpha[8] != 0 {
		t.Fatalf("NRGBA pixel (0,1): rgb=%d,%d,%d alpha=%d want 40,70,80,0", rgb[24], rgb[25], rgb[26], alpha[8])
	}
	if rgb[48] != 40 || rgb[49] != 80 || rgb[50] != 80 || alpha[16] != 128 {
		t.Fatalf("NRGBA pixel (0,2): rgb=%d,%d,%d alpha=%d want 40,80,80,128", rgb[48], rgb[49], rgb[50], alpha[16])
	}
	if rgb[75] != 50 || rgb[76] != 90 || rgb[77] != 85 || alpha[25] != 255 {
		t.Fatalf("NRGBA pixel (1,3): rgb=%d,%d,%d alpha=%d want 50,90,85,255", rgb[75], rgb[76], rgb[77], alpha[25])
	}
}

// R-14: alpha を保持した page は `/SMask`（DeviceGray・Flate）を追加 object として書く。
// opaque のみの page は byte 形式が従来と同一（既存テストで確認済み）。
func TestExportPDFWritesSMaskForNonOpaqueAlpha(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	if err := recordDiscovered(db, 7, "Galleries/7"); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	img.SetNRGBA(0, 1, color.NRGBA{40, 70, 80, 0})
	img.SetNRGBA(0, 2, color.NRGBA{40, 80, 80, 128})
	img.SetNRGBA(1, 3, color.NRGBA{50, 90, 85, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	writeExportFile(t, root, "Galleries/7/alphahash.png", buf.Bytes())
	if err := recordFile(db, filepath.Join("Galleries", "7", "alphahash.png"), validGalleryID(7), "alphahash", int64(buf.Len()), "png", "png", "not_evaluated", "structural"); err != nil {
		t.Fatal(err)
	}
	if err := recordPage(db, 7, 1, "alphahash"); err != nil {
		t.Fatal(err)
	}
	result, err := exportPDF(db, exportTestCfg(), root, 7, "")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(result["output"].(string))
	if err != nil {
		t.Fatal(err)
	}

	streamOf := func(objNum int) map[string]int {
		// find "N 0 obj", dict up to "stream\n", stream up to "\nendstream"
		start := bytes.Index(body, []byte(fmt.Sprintf("%d 0 obj\n", objNum)))
		if start < 0 {
			t.Fatalf("object %d not found", objNum)
		}
		sIdx := bytes.Index(body[start:], []byte("stream\n"))
		if sIdx < 0 {
			t.Fatalf("object %d has no stream", objNum)
		}
		sStart := start + sIdx + len("stream\n")
		eIdx := bytes.Index(body[sStart:], []byte("\nendstream\n"))
		if eIdx < 0 {
			t.Fatalf("object %d has no endstream", objNum)
		}
		return map[string]int{"dictStart": start, "streamStart": sStart, "streamLen": eIdx}
	}

	imageObj := streamOf(5)
	dict := string(body[imageObj["dictStart"] : imageObj["dictStart"]+bytes.Index(body[imageObj["dictStart"]:], []byte("stream\n"))])
	if !strings.Contains(dict, "/ColorSpace /DeviceRGB") {
		t.Fatalf("image dict missing DeviceRGB: %q", dict)
	}
	if !strings.Contains(dict, "/SMask 6 0 R") {
		t.Fatalf("image dict missing /SMask 6 0 R: %q", dict)
	}
	if !strings.Contains(dict, fmt.Sprintf("/Length %d", imageObj["streamLen"])) {
		t.Fatalf("image /Length mismatch: %q vs stream %d", dict, imageObj["streamLen"])
	}
	smaskObj := streamOf(6)
	sDict := string(body[smaskObj["dictStart"] : smaskObj["dictStart"]+bytes.Index(body[smaskObj["dictStart"]:], []byte("stream\n"))])
	if !strings.Contains(sDict, "/ColorSpace /DeviceGray") {
		t.Fatalf("smask dict missing DeviceGray: %q", sDict)
	}
	if !strings.Contains(sDict, fmt.Sprintf("/Length %d", smaskObj["streamLen"])) {
		t.Fatalf("smask /Length mismatch: %q vs stream %d", sDict, smaskObj["streamLen"])
	}

	zread := func(start, n int) []byte {
		r, err := zlib.NewReader(bytes.NewReader(body[start : start+n]))
		if err != nil {
			t.Fatal(err)
		}
		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 8*8*3 && len(out) != 8*8 {
			t.Fatalf("unexpected decompressed size %d", len(out))
		}
		return out
	}
	rgb := zread(imageObj["streamStart"], imageObj["streamLen"])
	alpha := zread(smaskObj["streamStart"], smaskObj["streamLen"])
	if len(rgb) != 8*8*3 || len(alpha) != 8*8 {
		t.Fatalf("decompressed sizes: rgb=%d alpha=%d", len(rgb), len(alpha))
	}
	for _, p := range []struct {
		x, y int
		rgb  [3]byte
		a    byte
	}{
		{0, 1, [3]byte{40, 70, 80}, 0},
		{0, 2, [3]byte{40, 80, 80}, 128},
		{1, 3, [3]byte{50, 90, 85}, 255},
	} {
		j := p.y*8 + p.x
		// fully transparent pixel must not be black in the RGB stream (black=True was the bug)
		if rgb[j*3] != p.rgb[0] || rgb[j*3+1] != p.rgb[1] || rgb[j*3+2] != p.rgb[2] {
			t.Errorf("pixel (%d,%d): rgb stream got %d,%d,%d want %d,%d,%d (black=%v)",
				p.x, p.y, rgb[j*3], rgb[j*3+1], rgb[j*3+2], p.rgb[0], p.rgb[1], p.rgb[2],
				rgb[j*3] == 0 && rgb[j*3+1] == 0 && rgb[j*3+2] == 0)
		}
		if alpha[j] != p.a {
			t.Errorf("pixel (%d,%d): alpha got %d want %d", p.x, p.y, alpha[j], p.a)
		}
	}

	// xref offset は file 実バイト上の object を指す（strict parser 回帰ガード）。
	xrefStart := bytes.LastIndex(body, []byte("xref\n0 "))
	if xrefStart < 0 {
		t.Fatal("xref not found")
	}
	sizeStart := xrefStart + len("xref\n0 ")
	sizeEnd := sizeStart + bytes.IndexByte(body[sizeStart:], '\n')
	if sizeEnd < 0 {
		t.Fatal("xref size line missing")
	}
	if string(body[sizeStart:sizeEnd]) != "7" {
		t.Fatalf("xref size: got %q want 7 (objects 0..6)", string(body[sizeStart:sizeEnd]))
	}
	xrefBlockStart := sizeEnd + 1 + 20 // skip the `f` entry line (object 0)
	for num := 1; num <= 6; num++ {
		line := body[xrefBlockStart : xrefBlockStart+20]
		offset, err := strconv.Atoi(string(line[:10]))
		if err != nil {
			t.Fatalf("xref entry %d parse: %v line %q", num, err, line)
		}
		if !bytes.HasPrefix(body[offset:], []byte(fmt.Sprintf("%d 0 obj\n", num))) {
			t.Fatalf("xref entry %d offset %d does not point to %d 0 obj", num, offset, num)
		}
		xrefBlockStart += 20
	}
}

// R-14: `.jpg` entry は decode せず保存済みバイトを DCTDecode stream へ embed する。
// invalid .jpg は undecodable skip 維持（skipped セマンティクス非触）。
func TestExportPDFEmbedsJpegAsDCTDecode(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	if err := recordDiscovered(db, 8, "Galleries/8"); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 5, 7))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	jpegBytes := buf.Bytes()
	writeExportFile(t, root, "Galleries/8/jpg1hash.jpg", jpegBytes)
	writeExportFile(t, root, "Galleries/8/jpg2hash.jpg", []byte("not-a-jpeg"))
	for _, name := range []string{"jpg1hash.jpg", "jpg2hash.jpg"} {
		if err := recordFile(db, filepath.Join("Galleries", "8", name), validGalleryID(8), "0", int64(len(name)), "jpg", "jpg", "not_evaluated", "structural"); err != nil {
			t.Fatal(err)
		}
	}
	if err := recordPage(db, 8, 1, "jpg1hash"); err != nil {
		t.Fatal(err)
	}
	if err := recordPage(db, 8, 2, "jpg2hash"); err != nil {
		t.Fatal(err)
	}
	result, err := exportPDF(db, exportTestCfg(), root, 8, "")
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
	start := bytes.Index(body, []byte("3 0 obj\n"))
	if start < 0 {
		t.Fatal("page object not found")
	}
	if !bytes.Contains(body[start:], []byte("/MediaBox [0 0 5 7]")) {
		t.Errorf("MediaBox missing jpeg dims (header parse): %q", string(body[start:start+120]))
	}
	objStart := bytes.Index(body, []byte("5 0 obj\n"))
	if objStart < 0 {
		t.Fatal("image object not found")
	}
	objEnd := objStart + bytes.Index(body[objStart:], []byte("endobj"))
	imageObj := body[objStart:objEnd]
	if !bytes.Contains(imageObj, []byte("/Filter /DCTDecode")) {
		t.Fatalf("image object is not DCTDecode: %q", string(imageObj))
	}
	dictEnd := objStart + bytes.Index(body[objStart:], []byte("stream\n"))
	if !bytes.Contains(imageObj, []byte(fmt.Sprintf("/Length %d", len(jpegBytes)))) {
		t.Fatalf("DCT /Length mismatch: %q want %d", string(imageObj), len(jpegBytes))
	}
	sStart := dictEnd + len("stream\n")
	if !bytes.Equal(body[sStart:sStart+len(jpegBytes)], jpegBytes) {
		t.Fatal("embedded stream bytes != stored jpg file bytes")
	}
	if !bytes.Contains(body[sStart+len(jpegBytes):sStart+len(jpegBytes)+len("\nendstream\n")], []byte("\nendstream\n")) {
		t.Fatal("endstream missing after DCT stream")
	}
	_ = objEnd
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
