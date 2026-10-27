package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupReviewExportRepo builds an isolated repository with a base commit, a
// head commit on a feature branch, and an uncommitted working-tree edit. The
// global/system git config is confined to temp files so a developer's own
// aliases or signing setup cannot leak into the exported review.
func setupReviewExportRepo(t *testing.T) (string, string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: base", time.Now(), "Alice")
	base := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	runGitTest(t, dir, nil, "checkout", "-b", "feature/review-export")
	writeAndCommit(t, dir, "src/Battle.cs", "class Battle { /* reworked */ }\n", "feat: rework", time.Now(), "Alice")

	// Uncommitted edit: the export covers the committed range only and must not
	// pick it up, nor let it mask a commit-list failure.
	if err := os.WriteFile(filepath.Join(dir, "src/Battle.cs"), []byte("class Battle { /* working copy */ }\n"), 0o644); err != nil {
		t.Fatalf("write working change: %v", err)
	}
	return dir, base
}

// installFailingReviewLogGit puts a git wrapper first on PATH that fails only
// for the commit-range read used by GetChangeReview.
func installFailingReviewLogGit(t *testing.T) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
has_log=0
has_no_merges=0
for arg in "$@"; do
  if [ "$arg" = "log" ]; then has_log=1; fi
  if [ "$arg" = "--no-merges" ]; then has_no_merges=1; fi
done
if [ "$has_log" = 1 ] && [ "$has_no_merges" = 1 ]; then
  echo "simulated review log failure: range history unavailable" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestExportChangeReviewMarkdownWarnsWhenCommitLogFails pins the export side of
// the partial-review contract: even though the report cannot list commits, it
// must carry the backend warning so a reviewer knows the range is incomplete.
func TestExportChangeReviewMarkdownWarnsWhenCommitLogFails(t *testing.T) {
	dir, base := setupReviewExportRepo(t)
	installFailingReviewLogGit(t)

	md, err := (GitService{}).ExportChangeReviewMarkdown(dir, base, "HEAD")
	if err != nil {
		t.Fatalf("failed commit read must not fail the export: %v", err)
	}
	for _, want := range []string{"# 变更评审", "## 文件结论", "src/Battle.cs", "⚠️", "提交列表读取失败"} {
		if !strings.Contains(md, want) {
			t.Fatalf("export missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "## 提交范围") {
		t.Fatalf("unreadable commit range must not render an empty commit section:\n%s", md)
	}
}

// TestExportChangeReviewMarkdownOmitsWarningOnHealthyRepo makes sure the
// warning line only appears for partial reviews.
func TestExportChangeReviewMarkdownOmitsWarningOnHealthyRepo(t *testing.T) {
	dir, base := setupReviewExportRepo(t)

	md, err := (GitService{}).ExportChangeReviewMarkdown(dir, base, "HEAD")
	if err != nil {
		t.Fatalf("ExportChangeReviewMarkdown failed: %v", err)
	}
	if strings.Contains(md, "⚠️") {
		t.Fatalf("healthy export must not contain a warning line:\n%s", md)
	}
	for _, want := range []string{"# 变更评审", "## 提交范围", "## 文件结论", "feat: rework", "src/Battle.cs"} {
		if !strings.Contains(md, want) {
			t.Fatalf("export missing %q:\n%s", want, md)
		}
	}
}
