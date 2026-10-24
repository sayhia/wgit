package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetSourceFileDetailReportsReadFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	if _, err := (GitService{}).InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	path := filepath.Join(repo, "unreadable.txt")
	if err := os.WriteFile(path, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := (GitService{}).GetSourceFileDetail(repo, "unreadable.txt", "working"); err == nil || !strings.Contains(err.Error(), "读取文件内容失败") {
		t.Fatalf("unreadable file must surface a read error, got %v", err)
	}

	// Absolute paths inside the repository remain supported after the
	// path-normalization change.
	absolute := filepath.Join(repo, "absolute.txt")
	if err := os.WriteFile(absolute, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	detail, err := (GitService{}).GetSourceFileDetail(repo, absolute, "working")
	if err != nil || !strings.Contains(detail.Content, "hello") {
		t.Fatalf("absolute path inside repo should be readable: %+v err=%v", detail, err)
	}
	png := filepath.Join(repo, "pic.png")
	if err := os.WriteFile(png, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}
	dataURI, err := (GitService{}).GetImageBase64(repo, "WORKING", png)
	if err != nil || !strings.HasPrefix(dataURI, "data:image/png;base64,") {
		t.Fatalf("absolute image path inside repo should be readable: %q err=%v", dataURI, err)
	}

	if _, err := (GitService{}).GetSourceFileDetail(repo, "missing.txt", "working"); err == nil {
		t.Fatal("missing file with no committed blob must return an error, not empty content")
	}
	if _, err := (GitService{}).GetSourceFileDetail(repo, "absolute.txt", "no-such-revision"); err == nil {
		t.Fatal("an unknown revision must return an error, not empty content")
	}
}

func TestGetSourceFileDetailRevisions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	svc := GitService{}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	path := filepath.Join(repo, "a.txt")
	if err := os.WriteFile(path, []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "a.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("working\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	working, err := svc.GetSourceFileDetail(repo, "a.txt", "working")
	if err != nil || !strings.Contains(working.Content, "working") {
		t.Fatalf("working revision must read the worktree: %+v err=%v", working, err)
	}
	head, err := svc.GetSourceFileDetail(repo, "a.txt", "HEAD")
	if err != nil || !strings.Contains(head.Content, "committed") || strings.Contains(head.Content, "working") {
		t.Fatalf("HEAD revision must read the committed blob: %+v err=%v", head, err)
	}
}
