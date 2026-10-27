package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newSourceDetailSizeRepo builds a throwaway repository whose git configuration
// is fully isolated (no global/system config, local identity), so host-level
// user.name/user.email or signing settings cannot leak into the size-cap tests.
func newSourceDetailSizeRepo(t *testing.T) (string, GitService) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	repo := t.TempDir()
	svc := GitService{}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Size Tester", "size@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	return repo, svc
}

// writeAndCommitSourceFile writes relPath into the repo and commits it.
func writeAndCommitSourceFile(t *testing.T, repo string, svc GitService, relPath string, data []byte) {
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

// assertOversizedSourceDetail verifies the "size only, no payload" contract for
// blobs above maxSourceTextBytes.
func assertOversizedSourceDetail(t *testing.T, detail *SourceFileDetail, wantSize int64) {
	t.Helper()
	if detail == nil {
		t.Fatal("detail is nil")
	}
	if detail.Size != wantSize {
		t.Fatalf("Size = %d, want %d", detail.Size, wantSize)
	}
	if !detail.IsBinary {
		t.Fatal("oversized blob must be reported as binary")
	}
	if detail.Content != "" {
		t.Fatalf("oversized blob must not be loaded into Content, got %d bytes", len(detail.Content))
	}
}

// TestGetSourceFileDetailCapsRevisionBlobSize covers the explicit revision
// path: a 6MB text blob is larger than maxSourceTextBytes, so only its real
// size may be returned instead of the full blob.
func TestGetSourceFileDetailCapsRevisionBlobSize(t *testing.T) {
	repo, svc := newSourceDetailSizeRepo(t)
	const wantSize = 6 * 1024 * 1024
	writeAndCommitSourceFile(t, repo, svc, "big.txt", []byte(strings.Repeat("a", wantSize)))

	detail, err := svc.GetSourceFileDetail(repo, "big.txt", "HEAD")
	if err != nil {
		t.Fatalf("GetSourceFileDetail(HEAD) failed: %v", err)
	}
	assertOversizedSourceDetail(t, detail, wantSize)
}

// TestGetSourceFileDetailCapsHeadFallbackBlobSize covers the fallback used when
// the worktree copy is missing: reading HEAD:<path> must obey the same cap.
func TestGetSourceFileDetailCapsHeadFallbackBlobSize(t *testing.T) {
	repo, svc := newSourceDetailSizeRepo(t)
	const wantSize = 6 * 1024 * 1024
	writeAndCommitSourceFile(t, repo, svc, "big.txt", []byte(strings.Repeat("a", wantSize)))

	if err := os.Remove(filepath.Join(repo, "big.txt")); err != nil {
		t.Fatalf("remove worktree copy failed: %v", err)
	}

	detail, err := svc.GetSourceFileDetail(repo, "big.txt", "")
	if err != nil {
		t.Fatalf("GetSourceFileDetail fallback failed: %v", err)
	}
	assertOversizedSourceDetail(t, detail, wantSize)
}

// TestGetSourceFileDetailCapsWorkingTreeFile is the regression guard for the
// worktree path, which already respected the cap before the shared constants.
func TestGetSourceFileDetailCapsWorkingTreeFile(t *testing.T) {
	repo, svc := newSourceDetailSizeRepo(t)
	const wantSize = 6 * 1024 * 1024
	if err := os.WriteFile(filepath.Join(repo, "big.txt"), []byte(strings.Repeat("a", wantSize)), 0o644); err != nil {
		t.Fatalf("write worktree copy failed: %v", err)
	}

	detail, err := svc.GetSourceFileDetail(repo, "big.txt", "working")
	if err != nil {
		t.Fatalf("GetSourceFileDetail(working) failed: %v", err)
	}
	assertOversizedSourceDetail(t, detail, wantSize)
}

// TestGetSourceFileDetailSmallRevisionContent keeps the small-blob contract
// intact: verbatim content, line count and the non-binary flag.
func TestGetSourceFileDetailSmallRevisionContent(t *testing.T) {
	repo, svc := newSourceDetailSizeRepo(t)
	writeAndCommitSourceFile(t, repo, svc, "small.txt", []byte("line1\nline2\n"))

	detail, err := svc.GetSourceFileDetail(repo, "small.txt", "HEAD")
	if err != nil {
		t.Fatalf("GetSourceFileDetail(HEAD) failed: %v", err)
	}
	if detail.Content != "line1\nline2\n" {
		t.Fatalf("Content = %q, want %q", detail.Content, "line1\nline2\n")
	}
	if detail.LineCount != 2 {
		t.Fatalf("LineCount = %d, want 2", detail.LineCount)
	}
	if detail.IsBinary {
		t.Fatal("small text blob must not be flagged binary")
	}
	if detail.Size != int64(len("line1\nline2\n")) {
		t.Fatalf("Size = %d, want %d", detail.Size, len("line1\nline2\n"))
	}
}
