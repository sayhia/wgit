package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetBranchesExcludesDetachedHeadPseudoBranch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(repo, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "tester@example.com")
	run("config", "user.name", "Tester")
	if err := os.WriteFile(filepath.Join(repo, "app.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "app.txt")
	run("commit", "-qm", "feat: v1")
	run("tag", "-a", "v1.0.0", "-m", "release v1")
	if err := os.WriteFile(filepath.Join(repo, "app.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("commit", "-qam", "feat: v2")
	if _, err := ExecGit(repo, "checkout", "-q", "v1.0.0"); err != nil {
		t.Fatalf("checkout tag failed: %v", err)
	}

	branches, err := (GitService{}).GetBranches(repo)
	if err != nil {
		t.Fatalf("GetBranches failed: %v", err)
	}
	for _, b := range branches {
		if strings.HasPrefix(b.Name, "(") {
			t.Fatalf("detached pseudo branch leaked into branch list: %+v", b)
		}
		if b.IsCurrent {
			t.Fatalf("no real branch is current in detached HEAD: %+v", b)
		}
	}
	if len(branches) != 1 || branches[0].Name != "main" {
		t.Fatalf("expected only main, got %+v", branches)
	}
}
