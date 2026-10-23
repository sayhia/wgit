package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareBranchesRejectsInvalidRefs(t *testing.T) {
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
	if out, err := ExecGit(repo, "commit", "-qm", "base"); err != nil {
		t.Fatalf("commit: %v (%s)", err, out)
	}

	if _, err := (GitService{}).CompareBranches(repo, "missing-base", "main"); err == nil {
		t.Fatal("invalid base branch must return an error")
	}
	if _, err := (GitService{}).CompareBranches(repo, "main", "missing-target"); err == nil {
		t.Fatal("invalid target branch must return an error")
	}
	res, err := (GitService{}).CompareBranches(repo, "main", "main")
	if err != nil {
		t.Fatalf("compare same branch: %v", err)
	}
	if res.AheadCount != 0 || res.BehindCount != 0 {
		t.Fatalf("same branch should have zero divergence: %+v", res)
	}
}
