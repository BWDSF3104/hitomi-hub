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

// jpegFrameSize は DCTDecode embed 用に、decode せず最初の SOF marker から width/height を
// 読む。magic 不正・SOF 不在（SOS 前に frame なし）は false（undecodable skip へ）。
func jpegFrameSize(data []byte) (int, int, bool) {
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, 0, false
	}
	i := 2
	for i+1 < len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		m := data[i+1]
		if m == 0x00 || m == 0xFF || (m >= 0xD0 && m <= 0xD7) {
			i += 2
			continue
		}
		if m == 0xD9 || m == 0xDA {
			break
		}
		if m >= 0xC0 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC {
			if i+9 > len(data) {
				break
			}
			height := int(data[i+5])<<8 | int(data[i+6])
			width := int(data[i+7])<<8 | int(data[i+8])
			if height > 0 && width > 0 {
				return width, height, true
			}
			break
		}
		if i+3 >= len(data) {
			break
		}
		segLen := int(data[i+2])<<8 | int(data[i+3])
		i += 2 + segLen
	}
	return 0, 0, false
}

// ycbcrToRGB は image/color の ycbcrToRGB と同一算式（byte 一致）で Y'CbCr を RGB に換算する。
func ycbcrToRGB(y, cb, cr uint8) (uint8, uint8, uint8) {
	yy1 := int32(y) * 0x10101
	cb1 := int32(cb) - 128
	cr1 := int32(cr) - 128
	r := yy1 + 91881*cr1
	if uint32(r)&0xff000000 == 0 {
		r >>= 16
	} else {
		r = ^(r >> 31)
	}
	g := yy1 - 22554*cb1 - 46802*cr1
	if uint32(g)&0xff000000 == 0 {
		g >>= 16
	} else {
		g = ^(g >> 31)
	}
	b := yy1 + 116130*cb1
	if uint32(b)&0xff000000 == 0 {
		b >>= 16
	} else {
		b = ^(b >> 31)
	}
	return uint8(r), uint8(g), uint8(b)
}

// validYCbCrRatio は image パッケージの既知 subsample ratio（COffset の switch が扱う値）。
// これ以外は添字保証がないため generic path へ。
func validYCbCrRatio(r image.YCbCrSubsampleRatio) bool {
	switch r {
	case image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422,
		image.YCbCrSubsampleRatio420, image.YCbCrSubsampleRatio440,
		image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410:
		return true
	}
	return false
}

// rawRGB は img の raw RGB を buf（page 間で再利用）へ書き出す。A-11: page ごとの
// working set として 1 枚分の容量しか保持しない（append 再割当て・page ごとの copy を避ける）。
// 型 switch（R-14）: decoder が返す concrete 型は pixel ごとに interface 呼び出し（At）ではなく
// buffer へ直接書く。alpha 型の stored alpha を別 buffer（abuf）へ書き、page 内に opaque で
// 無い alpha が 1 つでもあれば true を返す（SMask object の判定）。opaque のみの page は
// abuf を確保せず、byte は従来（At 経由）と一致する。alpha 換算は stored 値を用いる
// （At().RGBA() の premultiply は alpha=0 で RGB を 0 に寄せるため透過が黒くなる）。
// bounds.Min が原点でない decoder 画像は非現実的なため generic path（At 換算・alpha なし）へ。
func rawRGB(img image.Image, buf, abuf []byte) ([]byte, []byte, bool) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	n := w * h * 3
	if cap(buf) < n {
		buf = make([]byte, n)
	}
	buf = buf[:n]

	if bounds.Min != (image.Point{}) {
		return genericRGB(img, bounds, buf), nil, false
	}

	switch v := img.(type) {
	case *image.NRGBA:
		i := 0
		nonOpaque := false
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				p := v.Pix[v.PixOffset(x, y):]
				buf[i] = p[0]
				buf[i+1] = p[1]
				buf[i+2] = p[2]
				if p[3] != 255 {
					nonOpaque = true
				}
				i += 3
			}
		}
		if nonOpaque {
			abuf = fillAlpha(w*h, abuf, func(j int) uint8 {
				return v.Pix[v.PixOffset(j%w, j/w)+3]
			})
		}
		return buf, abuf, nonOpaque
	case *image.RGBA:
		i := 0
		nonOpaque := false
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				p := v.Pix[v.PixOffset(x, y):]
				buf[i] = p[0]
				buf[i+1] = p[1]
				buf[i+2] = p[2]
				if p[3] != 255 {
					nonOpaque = true
				}
				i += 3
			}
		}
		if nonOpaque {
			abuf = fillAlpha(w*h, abuf, func(j int) uint8 {
				return v.Pix[v.PixOffset(j%w, j/w)+3]
			})
		}
		return buf, abuf, nonOpaque
	case *image.YCbCr:
		if !validYCbCrRatio(v.SubsampleRatio) {
			return genericRGB(img, bounds, buf), nil, false
		}
		i := 0
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				yi := v.YOffset(x, y)
				ci := v.COffset(x, y)
				r, g, b := ycbcrToRGB(v.Y[yi], v.Cb[ci], v.Cr[ci])
				buf[i] = r
				buf[i+1] = g
				buf[i+2] = b
				i += 3
			}
		}
		return buf, nil, false
	case *image.NYCbCrA:
		if !validYCbCrRatio(v.SubsampleRatio) {
			return genericRGB(img, bounds, buf), nil, false
		}
		i := 0
		nonOpaque := false
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				yi := v.YOffset(x, y)
				ci := v.COffset(x, y)
				r, g, b := ycbcrToRGB(v.Y[yi], v.Cb[ci], v.Cr[ci])
				buf[i] = r
				buf[i+1] = g
				buf[i+2] = b
				if v.A[yi] != 255 {
					nonOpaque = true
				}
				i += 3
			}
		}
		if nonOpaque {
			abuf = fillAlpha(w*h, abuf, func(j int) uint8 {
				return v.A[v.YOffset(j%w, j/w)]
			})
		}
		return buf, abuf, nonOpaque
	default:
		return genericRGB(img, bounds, buf), nil, false
	}
}

// fillAlpha は alpha stream（P byte）を buf（page 間で再利用）へ書き出す。opaque のみの
// page では呼ばれない（A-11 gate の項目構成を変えない）。
func fillAlpha(n int, buf []byte, at func(int) uint8) []byte {
	if cap(buf) < n {
		buf = make([]byte, n)
	}
	buf = buf[:n]
	for j := 0; j < n; j++ {
		buf[j] = at(j)
	}
	return buf
}

// genericRGB は interface 経由の At 換算（premultiply）。concrete 型でない画像の fallback。
func genericRGB(img image.Image, bounds image.Rectangle, buf []byte) []byte {
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
// byte 形式（`N 0 obj`・`stream`/`endstream`・xref 10桁 offset 行・trailer）は現行と同一。
// decodable page が 0 件の場合、file を作成せずに error を返す既存動作を lazy create で維持する。
// R-14: `.jpg` entry は decode せず保存済みバイトを DCTDecode stream へ embed（Flate raw RGB
// の 20.6x 縮小）。magic と最初の SOF で size を取り、.invalid は undecodable skip 維持。
// alpha を持つ page（stored alpha に非 opaque が 1 つでもあれば）は `/SMask`（DeviceGray・
// Flate）を追加 object として書く。opaque のみの page は byte 形式が従来と同一。object
// 番号は page の base（3+i*3）で連続、SMask を書く page のみ +1 ずれ、xref は object 番号順。
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
	alphaBuf := []byte{}
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	nextObj := 3
	for _, entry := range entries {
		if pageCount >= cfg.Limits.MaxEntries {
			skipped["entry_count_exceeded"]++
			continue
		}
		if entry.status == "hit" || entry.status == "scan_error" {
			skipped[entry.status]++
			continue
		}
		path := filepath.Join(root, entry.path)
		pageNum, contentNum, imageNum := nextObj, nextObj+1, nextObj+2
		smaskNum := nextObj + 3
		dct := extOf(path) == "jpg" || extOf(path) == "jpeg"

		var width, height int
		var raw []byte
		var alpha []byte
		hasAlpha := false
		if dct {
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					skipped["missing"]++
				} else {
					skipped["undecodable"]++
				}
				continue
			}
			w, h, ok := jpegFrameSize(data)
			if !ok {
				skipped["undecodable"]++
				continue
			}
			width, height = w, h
			raw = data
		} else {
			img, err := decodeStored(path)
			if err != nil {
				if os.IsNotExist(err) {
					skipped["missing"]++
				} else {
					skipped["undecodable"]++
				}
				continue
			}
			bounds := img.Bounds()
			width, height = bounds.Dx(), bounds.Dy()
			raw, alpha, hasAlpha = rawRGB(img, rawBuf, alphaBuf)
			img = nil
			raw, err = zlibBytes(raw, &zbuf, zw)
			if err != nil {
				return nil, err
			}
		}
		if err := open(); err != nil {
			return nil, err
		}
		kids = append(kids, fmt.Sprintf("%d 0 R", pageNum))
		if err := writeObj(pageNum, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>",
			width, height, imageNum, contentNum), nil); err != nil {
			return nil, err
		}
		content := fmt.Sprintf("q %d 0 0 %d 0 0 cm /Im0 Do Q", width, height)
		if err := writeObj(contentNum, fmt.Sprintf("<< /Length %d >>", len(content)), []byte(content)); err != nil {
			return nil, err
		}
		smaskRef := ""
		if hasAlpha {
			smaskRef = fmt.Sprintf(" /SMask %d 0 R", smaskNum)
		}
		filter := "/FlateDecode"
		if dct {
			filter = "/DCTDecode"
		}
		if err := writeObj(imageNum, fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter %s%s /Length %d >>",
			width, height, filter, smaskRef, len(raw)), raw); err != nil {
			return nil, err
		}
		if hasAlpha {
			alphaRaw, err := zlibBytes(alpha, &zbuf, zw)
			if err != nil {
				return nil, err
			}
			if err := writeObj(smaskNum, fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>",
				width, height, len(alphaRaw)), alphaRaw); err != nil {
				return nil, err
			}
			nextObj += 4
		} else {
			nextObj += 3
		}
		pageCount++
		raw = nil
		alpha = nil
	}
	if pageCount == 0 {
		return nil, errors.New("no decodable images for pdf export")
	}

	if err := writeObj(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), pageCount), nil); err != nil {
		return nil, err
	}
	maxNum := nextObj - 1
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
