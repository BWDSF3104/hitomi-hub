package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var scanTargets = map[string]bool{"pdf": true, "cbz": true, "zip": true}

var execExt = map[string]bool{
	"exe": true, "dll": true, "scr": true, "jar": true, "js": true, "vbs": true,
	"ps1": true, "bat": true, "cmd": true, "com": true, "msi": true, "lnk": true,
}

var expectedMagic = map[string]map[string]bool{
	"webp": {"webp": true},
	"avif": {"avif": true},
	"png":  {"png": true},
	"jpg":  {"jpeg": true},
	"jpeg": {"jpeg": true},
	"gif":  {"gif": true},
	"bmp":  {"bmp": true},
	"cbz":  {"zip": true},
	"zip":  {"zip": true},
	"pdf":  {"pdf": true},
}

var pdfKeys = [][]byte{
	[]byte("/Launch"), []byte("/OpenAction"), []byte("/AA"), []byte("/JS"),
	[]byte("/JavaScript"), []byte("/EmbeddedFile"), []byte("/XFA"),
}

var drivePrefix = regexp.MustCompile(`^[A-Za-z]:`)

// containsDotDotSegment は path separator（/ \）を跨いだ `..` segment のみを hit する。
// 部分一致の `..`（例 `Vol...01.webp`）は hit しない。Python 参照実装・YARA rule と同一基準。
func containsDotDotSegment(name string) bool {
	for _, seg := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return true
		}
	}
	return false
}

// YaraGate は Python 参照実装の yara_gate と同じ。scan target の全ファイルで YARA を走らせる
// （構造検査が既に hit を捕えたファイルでも YARA 固有規則が hit できる）
func YaraGate(ext string, cfg Config) bool {
	return cfg.Yr != "" && ext != "" && scanTargets[ext]
}

func DetectMagic(head []byte) string {
	switch {
	case len(head) >= 4 && (string(head[:4]) == "PK\x03\x04" || string(head[:4]) == "PK\x05\x06" || string(head[:4]) == "PK\x07\x08"):
		return "zip"
	case len(head) >= 4 && string(head[:4]) == "%PDF":
		return "pdf"
	case len(head) >= 2 && string(head[:2]) == "MZ":
		return "pe"
	case len(head) >= 4 && string(head[:4]) == "\x7fELF":
		return "elf"
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "webp"
	case len(head) >= 8 && hex.EncodeToString(head[:8]) == "89504e470d0a1a0a":
		return "png"
	case len(head) >= 6 && (string(head[:6]) == "GIF87a" || string(head[:6]) == "GIF89a"):
		return "gif"
	case len(head) >= 2 && string(head[:2]) == "BM":
		return "bmp"
	case len(head) >= 3 && head[0] == 0xff && head[1] == 0xd8 && head[2] == 0xff:
		return "jpeg"
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		brand := string(head[8:12])
		if brand == "avif" || brand == "avis" {
			return "avif"
		}
		if brand == "heic" || brand == "heix" {
			return "heif"
		}
		return "iso-bmff"
	case len(head) >= 6 && hex.EncodeToString(head[:6]) == "377abcaf271c":
		return "7z"
	}
	return "unknown"
}

func IsPE(head []byte) bool {
	if len(head) < 2 || string(head[:2]) != "MZ" {
		return false
	}
	if len(head) < 64 {
		return false
	}
	e_lfanew := int(head[0x3C]) | int(head[0x3D])<<8 | int(head[0x3E])<<16 | int(head[0x3F])<<24
	if e_lfanew <= 0 || e_lfanew+4 > len(head) {
		return false
	}
	return string(head[e_lfanew:e_lfanew+4]) == "PE\x00\x00"
}

func ExtOf(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

func Sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ScanZipEntries(path string, l Limits) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := zip.NewReader(f, fileSize(path))
	if err != nil {
		return nil, errBadZip
	}
	findings := []string{}
	if len(zr.File) > l.MaxEntries {
		findings = append(findings, fmt.Sprintf("entry_count_exceeded:%d", len(zr.File)))
	}
	for i, entry := range zr.File {
		if i >= l.MaxEntries {
			break
		}
		name := entry.Name
		if containsDotDotSegment(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") || drivePrefix.MatchString(name) {
			findings = append(findings, "path_traversal:"+name)
		}
		entryExt := ""
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			entryExt = strings.ToLower(name[idx+1:])
		}
		if execExt[entryExt] {
			findings = append(findings, "executable_entry:"+name)
		}
		if entry.UncompressedSize64 > uint64(l.MaxUncompressed) {
			findings = append(findings, "entry_size_exceeded:"+name)
		}
		if entry.CompressedSize64 > 0 && entry.UncompressedSize64 > 0 && entry.UncompressedSize64 > uint64(l.MaxRatio)*entry.CompressedSize64 {
			findings = append(findings, "ratio_exceeded:"+name)
		}
	}
	return findings, nil
}

func ScanPDFKeys(path string, l Limits) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, l.MaxScanBytes))
	if err != nil {
		return nil, err
	}
	return pdfKeyFindings(data), nil
}

// pdfKeyFindings は key を []byte で保持し、data を key ごとにコピーしない（A-13）。
func pdfKeyFindings(data []byte) []string {
	found := []string{}
	for _, key := range pdfKeys {
		if bytes.Contains(data, key) {
			found = append(found, "pdf_object:"+string(key))
		}
	}
	return found
}

// ScanFile mirrors the Python reference implementation's status semantics.
func ScanFile(path string, cfg Config) (string, string, []string, string) {
	l := cfg.Limits
	ext := ExtOf(path)
	head := readHead(path, 64)
	magic := DetectMagic(head)
	findings := []string{}
	expected := expectedMagic[ext]
	if expected != nil && !expected[magic] {
		findings = append(findings, "magic_mismatch:"+ext+"_vs_"+magic)
	}
	if magic == "pe" && !IsPE(head) {
		findings = append(findings, "mz_without_pe_header")
	}
	if magic == "zip" && scanTargets[ext] {
		entryFindings, err := ScanZipEntries(path, l)
		if err != nil {
			if err == errBadZip {
				return ext, magic, []string{"bad_zip"}, "scan_error"
			}
			return ext, magic, []string{"read_error:" + err.Error()}, "scan_error"
		}
		findings = append(findings, entryFindings...)
	}
	if magic == "pdf" {
		pdfFindings, err := ScanPDFKeys(path, l)
		if err != nil {
			return ext, magic, []string{"read_error:" + err.Error()}, "scan_error"
		}
		findings = append(findings, pdfFindings...)
	}
	if len(findings) > 0 {
		return ext, magic, findings, "hit"
	}
	if scanTargets[ext] {
		return ext, magic, findings, "pass"
	}
	return ext, magic, findings, "not_evaluated"
}
