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

type pdfPage struct {
	index  int
	width  int
	height int
	rgb    []byte
}

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

func rawRGB(img image.Image) []byte {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := make([]byte, 0, w*h*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			out = append(out, byte(r>>8), byte(g>>8), byte(b>>8))
		}
	}
	return out
}

func zlibBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// exportPDF は decoder で読める画像を 1 ページ 1 画像の PDF にする（FlateDecode の raw RGB）
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

	pages := []pdfPage{}
	skipped := map[string]int{}
	for _, entry := range entries {
		if len(pages) >= cfg.Limits.MaxEntries {
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
		pages = append(pages, pdfPage{index: entry.index, width: bounds.Dx(), height: bounds.Dy(), rgb: rawRGB(img)})
	}
	if len(pages) == 0 {
		return nil, errors.New("no decodable images for pdf export")
	}

	file, err := os.Create(target)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// object numbers: 1 catalog, 2 pages, then per page page/content/image at 3+i*3
	type obj struct {
		num    int
		body   []byte
		stream []byte
	}
	objects := []obj{}
	writeObj := func(o obj) { objects = append(objects, o) }
	writeObj(obj{num: 1, body: []byte("<< /Type /Catalog /Pages 2 0 R >>")})

	kids := []string{}
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i*3))
	}
	writeObj(obj{num: 2, body: []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))})

	for i, page := range pages {
		pageNum, contentNum, imageNum := 3+i*3, 4+i*3, 5+i*3
		writeObj(obj{num: pageNum, body: []byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>",
			page.width, page.height, imageNum, contentNum))})
		content := []byte(fmt.Sprintf("q %d 0 0 %d 0 0 cm /Im0 Do Q", page.width, page.height))
		writeObj(obj{num: contentNum, body: []byte(fmt.Sprintf("<< /Length %d >>", len(content))), stream: content})
		raw, err := zlibBytes(page.rgb)
		if err != nil {
			return nil, err
		}
		writeObj(obj{num: imageNum, body: []byte(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>",
			page.width, page.height, len(raw))), stream: raw})
	}

	out0 := bytes.Buffer{}
	out0.WriteString("%PDF-1.4\n")
	offsets := map[int]int{}
	offsets[0] = 0
	for _, o := range objects {
		offsets[o.num] = out0.Len()
		out0.WriteString(fmt.Sprintf("%d 0 obj\n", o.num))
		out0.Write(o.body)
		out0.WriteString("\n")
		if o.stream != nil {
			out0.WriteString("stream\n")
			out0.Write(o.stream)
			out0.WriteString("\nendstream\n")
		}
		out0.WriteString("endobj\n")
	}
	xrefPos := out0.Len()
	out0.WriteString(fmt.Sprintf("xref\n0 %d\n", len(objects)+1))
	out0.WriteString("0000000000 65535 f \n")
	for num := 1; num <= len(objects); num++ {
		out0.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[num]))
	}
	out0.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefPos))
	if _, err := file.Write(out0.Bytes()); err != nil {
		return nil, err
	}
	return map[string]any{"gallery_id": galleryID, "output": target, "order": order,
		"pages": len(pages), "skipped": skipped}, nil
}
