package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// tools/yr.exe を実際に呼ぶ integration test（未導入環境では skip）
func TestRunYaraExecutionPath(t *testing.T) {
	yr := filepath.Join(dataRoot(), "tools", "yr.exe")
	if _, err := os.Stat(yr); err != nil {
		t.Skip("tools/yr.exe not present")
	}
	rulesFile := filepath.Join(dataRoot(), "rules", "structural.yar")
	dir := t.TempDir()

	cbz := makeZip(t, dir, map[string]string{"../payload.exe": "MZ\x90\x00", "bomb.webp": strings.Repeat("\x00", 2000000)})
	got := RunYara(cbz, rulesFile, yr, cfg().Limits)
	for _, want := range []string{"Zip_Path_Traversal", "Zip_Executable_Entry", "Zip_Decompression_Ratio"} {
		if !contains(got, want) {
			t.Errorf("cbz rules: got %v, want %s", got, want)
		}
	}

	pdf := filepath.Join(dir, "a.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4\n1 0 obj\n<< /OpenAction << /S /JavaScript /JS (x) >> >>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := RunYara(pdf, rulesFile, yr, cfg().Limits); len(got) != 1 || got[0] != "PDF_Structural_Risk" {
		t.Errorf("pdf rules: got %v", got)
	}

	spoof := filepath.Join(dir, "image.png")
	if err := os.WriteFile(spoof, []byte("MZ\x90\x00"+strings.Repeat("\x00", 60)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := RunYara(spoof, rulesFile, yr, cfg().Limits); len(got) != 1 || got[0] != "MZ_Header_At_Offset0" {
		t.Errorf("spoof rules: got %v", got)
	}
}
