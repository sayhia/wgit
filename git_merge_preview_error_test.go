package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewMergeRejectsUnrelatedHistories(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "base.txt"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatal(err)
	}
	baseBranch, err := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "checkout", "--orphan", "other"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "base.txt"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Commit(repo, "feat: unrelated root", false); err != nil {
		t.Fatalf("orphan commit failed: %v", err)
	}
	if _, err := ExecGit(repo, "checkout", strings.TrimSpace(baseBranch)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PreviewMerge(repo, "other"); err == nil || !strings.Contains(err.Error(), "无法计算共同祖先") {
		t.Fatalf("unrelated histories must not be reported as a clean/conflicting merge: %v", err)
	}
}
