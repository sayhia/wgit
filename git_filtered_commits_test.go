package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCommitsFilteredRejectsInvalidBranch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.name", "Tester"},
		{"config", "user.email", "tester@example.com"},
	} {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := ExecGit(repo, "add", "-A"); err != nil {
		t.Fatalf("add: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "commit", "-qm", "fix: base"); err != nil {
		t.Fatalf("commit: %v (%s)", err, out)
	}

	if _, err := (GitService{}).GetCommitsFiltered(repo, "missing-branch", "", "", "", "", 0, 10); err == nil {
		t.Fatal("invalid branch must return an error")
	}
	res, err := (GitService{}).GetCommitsFiltered(repo, "main", "", "", "", "fix", 0, 10)
	if err != nil {
		t.Fatalf("filter commits: %v", err)
	}
	if len(res.Commits) != 1 || res.TotalCount != 1 {
		t.Fatalf("unexpected filtered result: %+v", res)
	}
}
