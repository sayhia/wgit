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

// TestChangeReviewWarnsWhenCommitLogFails pins the partial-review contract:
// `git log` failing must not fail the whole session (files and diffs still
// load), and the payload must say the commit list is unavailable instead of
// letting the UI render an empty range as "no commits".
func TestChangeReviewWarnsWhenCommitLogFails(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: base", time.Now(), "Alice")
	base := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	runGitTest(t, dir, nil, "checkout", "-b", "feature/review-log-failure")
	writeAndCommit(t, dir, "src/Battle.cs", "class Battle { /* reworked */ }\n", "feat: rework", time.Now(), "Alice")

	// Leave the working tree dirty: the review range is committed-only, and an
	// uncommitted edit must neither leak into the review nor mask the warning.
	if err := os.WriteFile(filepath.Join(dir, "src/Battle.cs"), []byte("class Battle { /* working copy */ }\n"), 0o644); err != nil {
		t.Fatalf("write working change: %v", err)
	}

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
  echo "simulated log failure: range history unavailable" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	review, err := (GitService{}).GetChangeReview(dir, base, "feature/review-log-failure")
	if err != nil {
		t.Fatalf("log failure must not fail the review: %v", err)
	}
	if review == nil {
		t.Fatal("review is nil")
	}
	if len(review.Files) == 0 {
		t.Fatal("files must still load when git log fails")
	}
	if len(review.Commits) != 0 {
		t.Fatalf("expected no commits when git log fails, got %d", len(review.Commits))
	}

	if review.WarningKey != "review.warning.commits.unavailable" {
		t.Fatalf("WarningKey = %q, want %q", review.WarningKey, "review.warning.commits.unavailable")
	}
	if !strings.Contains(review.Warning, "提交列表读取失败") {
		t.Fatalf("Warning = %q, want it to contain the failure summary", review.Warning)
	}
	if !strings.Contains(review.Warning, "simulated log failure") {
		t.Fatalf("Warning = %q, want it to include the git error", review.Warning)
	}
	paramErr := review.WarningParams["error"]
	if paramErr == "" {
		t.Fatalf("WarningParams[error] is empty: %+v", review.WarningParams)
	}
	if !strings.Contains(paramErr, "simulated log failure") || !strings.Contains(paramErr, "exit status 128") {
		t.Fatalf("WarningParams[error] = %q, want the raw git failure", paramErr)
	}
	if !strings.Contains(review.Warning, paramErr) {
		t.Fatalf("Warning %q must embed WarningParams[error] %q", review.Warning, paramErr)
	}

	// The per-file diff path stays usable even though the commit list failed.
	diff, err := (GitService{}).GetReviewFileDiff(dir, base, "feature/review-log-failure", "src/Battle.cs")
	if err != nil {
		t.Fatalf("GetReviewFileDiff failed: %v", err)
	}
	if !strings.Contains(diff, "reworked") {
		t.Fatalf("unexpected file diff: %q", diff)
	}
}
