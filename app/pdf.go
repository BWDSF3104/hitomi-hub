package main

import (
	"bytes"
	"compress/zlib"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/webp"
)

// decodeStored は保存済み画像を decoder で読む。shard の保存形式は webp / avif のみで、
// PDF に embed できるのは decoder が持つ形式（webp・png・jpeg）。AVIF は pure-Go decoder が無い
func decodeStored(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	switch extOf(path) {
	case "webp":
		return webp.Decode(file)
	case "png":
		return png.Decode(file)
	case "jpg", "jpeg":
		return jpeg.Decode(file)
	default:
		return nil, fmt.Errorf("no decoder for %s", extOf(path))
	}
}

// rawRGB は img の raw RGB を buf（page 間で再利用）へ書き出す。A-11: page ごとの
// working set として 1 枚分の容量しか保持しない（append 再割当て・page ごとの copy を避ける）。
func rawRGB(img image.Image, buf []byte) []byte {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	n := w * h * 3
	if cap(buf) < n {
		buf = make([]byte, n)
	}
	buf = buf[:n]
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			buf[i] = byte(r >> 8)
			buf[i+1] = byte(g >> 8)
			buf[i+2] = byte(b >> 8)
			i += 3
		}
	}
	return buf
}

// zlibBytes は data を zlib へ圧縮し buf（page 間で再利用）へ書き出す。
func zlibBytes(data []byte, buf *bytes.Buffer, w *zlib.Writer) ([]byte, error) {
	buf.Reset()
	if buf.Cap() < len(data) {
		buf.Grow(len(data))
	}
	w.Reset(buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// exportPDF は decoder で読める画像を 1 ページ 1 画像の PDF にする（FlateDecode の raw RGB）。
// A-11: page ごとに decode→rawRGB→zlib→file 直接書き出し（streaming）。page ごとの結果を
// メモリに保持しない。ピークは 1 page 分の working set（decoder image バッファ + rawRGB + zlib 結果）
// ＋ header/metadata（xref offset map・kids）のみ。
// `/Pages`（object 2）は `/Kids` と `/Count` が全 page 数を決めてから書けるため物理的に末尾へ。
// PDF は object の物理順が非連続でも xref offset が正しければ有効（strict parser は xref 経由で読む）。
// decodable page が 0 件の場合、file を作成せずに error を返す既存動作を lazy create で維持する。
func exportPDF(db *sql.DB, cfg Config, root string, galleryID int, out string) (map[string]any, error) {
	dirName := orDefault(cfg.GalleryDir, "Galleries")
	format := orDefault(cfg.Hitomi.ImageFormat, "webp")
	entries, order, err := exportPages(db, galleryID, dirName, format)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("no files recorded for gallery " + strconv.Itoa(galleryID))
	}
	target := out
	if target == "" {
		target = filepath.Join(root, dirName, strconv.Itoa(galleryID)+".pdf")
	}
	if !withinRoot(root, target) {
		return nil, fmt.Errorf("output must stay inside root: %s", target)
	}

	// object numbers: 1 catalog, 2 pages, then per page page/content/image at 3+i*3
	var file *os.File
	defer func() {
		if file != nil {
			file.Close()
		}
	}()
	offsets := map[int]int{}
	kids := []string{}
	pageCount := 0
	pos := 0
	writePiece := func(b []byte) error {
		if _, err := file.Write(b); err != nil {
			return err
		}
		pos += len(b)
		return nil
	}
	writeObj := func(num int, body string, stream []byte) error {
		offsets[num] = pos
		if err := writePiece([]byte(fmt.Sprintf("%d 0 obj\n", num))); err != nil {
			return err
		}
		if err := writePiece([]byte(body)); err != nil {
			return err
		}
		if err := writePiece([]byte("\n")); err != nil {
			return err
		}
		if stream != nil {
			if err := writePiece([]byte("stream\n")); err != nil {
				return err
			}
			if err := writePiece(stream); err != nil {
				return err
			}
			if err := writePiece([]byte("\nendstream\n")); err != nil {
				return err
			}
		}
		return writePiece([]byte("endobj\n"))
	}
	open := func() error {
		if file != nil {
			return nil
		}
		f, err := os.Create(target)
		if err != nil {
			return err
		}
		file = f
		if err := writePiece([]byte("%PDF-1.4\n")); err != nil {
			return err
		}
		offsets[1] = pos
		return writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>", nil)
	}

	skipped := map[string]int{}
	rawBuf := []byte{}
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	for _, entry := range entries {
		if pageCount >= cfg.Limits.MaxEntries {
			skipped["entry_count_exceeded"]++
			continue
		}
		if entry.status == "hit" || entry.status == "scan_error" {
			skipped[entry.status]++
			continue
		}
		img, err := decodeStored(filepath.Join(root, entry.path))
		if err != nil {
			if os.IsNotExist(err) {
				skipped["missing"]++
			} else {
				skipped["undecodable"]++
			}
			continue
		}
		bounds := img.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		raw, err := zlibBytes(rawRGB(img, rawBuf), &zbuf, zw)
		if err != nil {
			return nil, err
		}
		if err := open(); err != nil {
			return nil, err
		}
		pageNum, contentNum, imageNum := 3+pageCount*3, 4+pageCount*3, 5+pageCount*3
		kids = append(kids, fmt.Sprintf("%d 0 R", pageNum))
		if err := writeObj(pageNum, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>",
			width, height, imageNum, contentNum), nil); err != nil {
			return nil, err
		}
		content := fmt.Sprintf("q %d 0 0 %d 0 0 cm /Im0 Do Q", width, height)
		if err := writeObj(contentNum, fmt.Sprintf("<< /Length %d >>", len(content)), []byte(content)); err != nil {
			return nil, err
		}
		if err := writeObj(imageNum, fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>",
			width, height, len(raw)), raw); err != nil {
			return nil, err
		}
		pageCount++
		img = nil
		raw = nil
	}
	if pageCount == 0 {
		return nil, errors.New("no decodable images for pdf export")
	}

	if err := writeObj(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), pageCount), nil); err != nil {
		return nil, err
	}
	maxNum := 3 + (pageCount-1)*3 + 2
	xrefPos := pos
	if err := writePiece([]byte(fmt.Sprintf("xref\n0 %d\n", maxNum+1))); err != nil {
		return nil, err
	}
	if err := writePiece([]byte("0000000000 65535 f \n")); err != nil {
		return nil, err
	}
	for num := 1; num <= maxNum; num++ {
		if err := writePiece([]byte(fmt.Sprintf("%010d 00000 n \n", offsets[num]))); err != nil {
			return nil, err
		}
	}
	if err := writePiece([]byte(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", maxNum+1, xrefPos))); err != nil {
		return nil, err
	}
	return map[string]any{"gallery_id": galleryID, "output": target, "order": order,
		"pages": pageCount, "skipped": skipped}, nil
}
