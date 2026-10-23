package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateGitkeepsRejectsEscapeAndReportsStageFailure(t *testing.T) {
	svc := GitService{}
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "init"); err != nil {
		t.Fatal(err)
	}
	emptyDir := filepath.Join(repo, "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.GenerateGitkeeps(repo, []string{"../outside"}); err == nil {
		t.Fatal("path escaping the repository must be rejected")
	}
	if _, err := os.Stat(filepath.Join(outside, ".gitkeep")); !os.IsNotExist(err) {
		t.Fatalf("escaping path must not create files outside the repo: %v", err)
	}

	link := filepath.Join(repo, "linked-outside")
	if err := os.Symlink(outside, link); err == nil {
		if _, err := svc.GenerateGitkeeps(repo, []string{"linked-outside"}); err == nil {
			t.Fatal("symlinked directory escaping the repository must be rejected")
		}
		if _, err := os.Stat(filepath.Join(outside, ".gitkeep")); !os.IsNotExist(err) {
			t.Fatalf("symlink escape must not create files outside the repo: %v", err)
		}
	}

	count, err := svc.GenerateGitkeeps(repo, []string{"empty"})
	if err != nil || count != 1 {
		t.Fatalf("GenerateGitkeeps failed: count=%d err=%v", count, err)
	}
	staged, _ := ExecGit(repo, "diff", "--cached", "--name-only")
	if !strings.Contains(staged, "empty/.gitkeep") {
		t.Fatalf("generated .gitkeep was not staged:\n%s", staged)
	}

	failRepo := t.TempDir()
	if _, err := ExecGit(failRepo, "init"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(failRepo, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(failRepo, ".git")
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, 0o755) })
	count, err = svc.GenerateGitkeeps(failRepo, []string{"empty"})
	if err == nil || count != 0 {
		t.Fatalf("stage failure must be surfaced: count=%d err=%v", count, err)
	}
}

func TestGetSourceFileDiffRejectsUnknownRevision(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	writeAndCommit(t, repo, "a.txt", "old\n", "chore: base", time.Now(), "Tester")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := svc.GetSourceFileDiff(repo, "a.txt", "working")
	if err != nil || !strings.Contains(diff, "-old") || !strings.Contains(diff, "+new") {
		t.Fatalf("working diff failed: %q err=%v", diff, err)
	}
	if _, err := svc.GetSourceFileDiff(repo, "a.txt", "definitely-not-a-revision"); err == nil {
		t.Fatal("unknown revision must return an error instead of an empty diff")
	}
}

func TestScanEmptyDirectoriesPropagatesReadFailure(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	locked := filepath.Join(repo, "locked")
	if err := os.MkdirAll(filepath.Join(locked, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, err := svc.ScanEmptyDirectories(repo); err == nil {
		t.Fatal("unreadable directories must not be silently skipped")
	}
}
