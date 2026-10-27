package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newFileContentAtCommitRepo builds a throwaway repository with fully isolated
// git configuration (no global/system config, local identity) so host-level
// user.name/user.email or signing settings cannot leak into these tests.
func newFileContentAtCommitRepo(t *testing.T) (string, GitService) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	repo := t.TempDir()
	svc := GitService{}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Content Tester", "content@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	return repo, svc
}

// commitFileContentAtCommitFile writes relPath into the repo and commits it.
func commitFileContentAtCommitFile(t *testing.T, repo string, svc GitService, relPath string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, relPath), data, 0o644); err != nil {
		t.Fatalf("write %s failed: %v", relPath, err)
	}
	if err := svc.StageFile(repo, relPath); err != nil {
		t.Fatalf("StageFile(%s) failed: %v", relPath, err)
	}
	if err := svc.Commit(repo, "add "+relPath, false); err != nil {
		t.Fatalf("Commit(%s) failed: %v", relPath, err)
	}
}

// TestGetFileContentAtCommitRejectsOversizedBlob verifies that a blob above
// maxSourceTextBytes is refused before `git show` buffers it, for both an
// explicit revision and the empty (HEAD) fallback.
func TestGetFileContentAtCommitRejectsOversizedBlob(t *testing.T) {
	repo, svc := newFileContentAtCommitRepo(t)
	const oversized = 6 * 1024 * 1024
	commitFileContentAtCommitFile(t, repo, svc, "big.txt", []byte(strings.Repeat("a", oversized)))

	for _, commitHash := range []string{"HEAD", ""} {
		content, err := svc.GetFileContentAtCommit(repo, commitHash, "big.txt")
		if err == nil {
			t.Fatalf("commitHash %q: expected size-cap error, got %d bytes", commitHash, len(content))
		}
		if !strings.Contains(err.Error(), "跳过读取") {
			t.Fatalf("commitHash %q: error = %v, want it to mention 跳过读取", commitHash, err)
		}
		if content != "" {
			t.Fatalf("commitHash %q: content = %d bytes, want empty", commitHash, len(content))
		}
	}
}

// TestGetFileContentAtCommitReturnsSmallBlob keeps the success path intact:
// small blobs still come back through `git show`. ExecGit trims the trailing
// newline, matching the pre-existing contract asserted elsewhere.
func TestGetFileContentAtCommitReturnsSmallBlob(t *testing.T) {
	repo, svc := newFileContentAtCommitRepo(t)
	commitFileContentAtCommitFile(t, repo, svc, "small.txt", []byte("line1\nline2\n"))

	content, err := svc.GetFileContentAtCommit(repo, "HEAD", "small.txt")
	if err != nil {
		t.Fatalf("GetFileContentAtCommit failed: %v", err)
	}
	if content != "line1\nline2" {
		t.Fatalf("content = %q, want %q", content, "line1\nline2")
	}
}

// TestGetFileContentAtCommitMissingPathFailsSizeQuery verifies that a path
// absent at the revision surfaces the size-query error instead of falling back
// to an unbounded read.
func TestGetFileContentAtCommitMissingPathFailsSizeQuery(t *testing.T) {
	repo, svc := newFileContentAtCommitRepo(t)
	commitFileContentAtCommitFile(t, repo, svc, "present.txt", []byte("present\n"))

	content, err := svc.GetFileContentAtCommit(repo, "HEAD", "missing.txt")
	if err == nil {
		t.Fatalf("expected size-query error for missing path, got %d bytes", len(content))
	}
	if !strings.Contains(err.Error(), "文件大小失败") {
		t.Fatalf("error = %v, want it to mention 文件大小失败", err)
	}
	if content != "" {
		t.Fatalf("content = %d bytes, want empty", len(content))
	}
}
