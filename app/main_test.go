package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRootUsesCwdWithoutGitParent(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cwd := filepath.Join(root, "copied-project")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	got, err := filepath.Abs(resolveRoot(""))
	if err != nil {
		t.Fatal(err)
	}
	if got != cwd {
		t.Errorf("resolveRoot: got %q want %q", got, cwd)
	}

	explicit := filepath.Join(root, "scratch/live")
	got, err = filepath.Abs(resolveRoot(explicit))
	if err != nil {
		t.Fatal(err)
	}
	if got != explicit {
		t.Errorf("explicit resolveRoot: got %q want %q", got, explicit)
	}
}

func TestFindCommandKeepsFlagValues(t *testing.T) {
	cmd, rest := findCommand([]string{"--root", "C:\\data", "--no-vt"})
	if cmd != "scan" || len(rest) != 3 {
		t.Errorf("flags only: cmd=%q rest=%v", cmd, rest)
	}
	cmd, rest = findCommand([]string{"search", "--term", "female:yuri", "--page", "2"})
	if cmd != "search" || len(rest) != 4 {
		t.Errorf("subcommand first: cmd=%q rest=%v", cmd, rest)
	}
	cmd, rest = findCommand([]string{"--root", "C:\\data", "list", "--list", "unread"})
	if cmd != "list" || len(rest) != 4 {
		t.Errorf("subcommand after flags: cmd=%q rest=%v", cmd, rest)
	}
	cmd, rest = findCommand([]string{"--root=C:\\data", "fav", "--id", "125"})
	if cmd != "fav" || len(rest) != 3 {
		t.Errorf("inline value: cmd=%q rest=%v", cmd, rest)
	}
}
