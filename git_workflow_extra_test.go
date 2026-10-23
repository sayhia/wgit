package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func initWorkflowTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")
	return dir
}

// TestBisectLifecycle drives a complete git bisect session through the service API:
// start (bad HEAD / good first commit), a sequence of good/bad judgments, finding
// the first bad commit, and resetting back to the original branch.
func TestBisectLifecycle(t *testing.T) {
	dir := initWorkflowTestRepo(t)
	now := time.Now()

	// 8 linear commits; the regression appears in commit #5 (src/step.txt content).
	var bugHash string
	for i := 1; i <= 8; i++ {
		content := "ok step " + string(rune('0'+i)) + "\n"
		msg := "feat: step " + string(rune('0'+i))
		if i >= 5 {
			content = "broken step " + string(rune('0'+i)) + "\n"
		}
		writeAndCommit(t, dir, "src/step.txt", content, msg, now.Add(time.Duration(i)*time.Minute), "Alice")
		if i == 5 {
			bugHash = strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))
		}
	}
	goodHash := strings.TrimSpace(runGitTest(t, dir, nil, "rev-list", "--max-parents=0", "HEAD"))

	svc := GitService{}

	status, err := svc.BisectStart(dir, "HEAD", goodHash)
	if err != nil {
		t.Fatalf("BisectStart failed: %v", err)
	}
	if status == nil || !status.IsActive {
		t.Fatalf("bisect should be active after start: %+v", status)
	}
	if status.BadRef == "" || len(status.GoodRefs) == 0 {
		t.Fatalf("bad/good refs not reported: %+v", status)
	}

	found := ""
	for i := 0; i < 10; i++ {
		behaviorBroken, _ := os.ReadFile(filepath.Join(dir, "src/step.txt"))
		var next *BisectStatus
		if strings.Contains(string(behaviorBroken), "broken") {
			next, err = svc.BisectBad(dir)
		} else {
			next, err = svc.BisectGood(dir)
		}
		if err != nil {
			t.Fatalf("bisect judgment %d failed: %v", i, err)
		}
		status = next
		headNext := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))
		if headNext == bugHash {
			found = headNext
			break
		}
		if status == nil || !status.IsActive {
			found = headNext
			break
		}
	}
	if found != bugHash {
		t.Fatalf("bisect did not converge on the first bad commit: want %s got %s (status=%+v)", bugHash[:8], found, status)
	}

	if err := svc.BisectReset(dir); err != nil {
		t.Fatalf("BisectReset failed: %v", err)
	}
	after, err := svc.BisectStatus(dir)
	if err != nil {
		t.Fatalf("BisectStatus after reset failed: %v", err)
	}
	if after.IsActive {
		t.Fatalf("bisect should be inactive after reset: %+v", after)
	}
	branch := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "--abbrev-ref", "HEAD"))
	if branch != "main" {
		t.Fatalf("reset should return to main, got %q", branch)
	}
}

// TestCherryPickMultipleBackport covers the batch backport workflow used to port a
// run of fixes onto a release branch.
func TestCherryPickMultipleBackport(t *testing.T) {
	dir := initWorkflowTestRepo(t)
	now := time.Now()
	writeAndCommit(t, dir, "src/base.txt", "base\n", "feat: base", now, "Alice")

	runGitTest(t, dir, nil, "checkout", "-b", "feature/fixes")
	writeAndCommit(t, dir, "src/fix1.txt", "fix1\n", "fix: one", now, "Alice")
	h1 := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))
	writeAndCommit(t, dir, "src/fix2.txt", "fix2\n", "fix: two", now, "Alice")
	h2 := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))
	runGitTest(t, dir, nil, "checkout", "main")

	svc := GitService{}
	res, err := svc.CherryPickMultiple(dir, []string{h1, h2})
	if err != nil {
		t.Fatalf("CherryPickMultiple failed: %v", err)
	}
	if res == nil || res.SuccessCount != 2 || res.TotalCount != 2 || res.HasConflict {
		t.Fatalf("expected 2 successful cherry-picks, got %+v", res)
	}
	if len(res.AppliedHashes) != 2 {
		t.Fatalf("applied hashes not reported: %+v", res)
	}
	for _, p := range []string{"src/fix1.txt", "src/fix2.txt"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Fatalf("expected %s after cherry-pick: %v", p, err)
		}
	}
}

// TestCherryPickMultipleConflict reports the failing commit and conflict state
// instead of leaving the caller blind.
func TestCherryPickMultipleConflict(t *testing.T) {
	dir := initWorkflowTestRepo(t)
	now := time.Now()
	writeAndCommit(t, dir, "src/shared.txt", "base\n", "feat: base", now, "Alice")

	runGitTest(t, dir, nil, "checkout", "-b", "feature/conflict")
	writeAndCommit(t, dir, "src/shared.txt", "feature change\n", "feat: feature", now, "Alice")
	conflictHash := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))
	runGitTest(t, dir, nil, "checkout", "main")
	writeAndCommit(t, dir, "src/shared.txt", "main change\n", "feat: main", now.Add(time.Minute), "Alice")

	svc := GitService{}
	res, err := svc.CherryPickMultiple(dir, []string{conflictHash})
	if err != nil {
		t.Fatalf("CherryPickMultiple should report conflicts in the result, got error: %v", err)
	}
	if res == nil || !res.HasConflict {
		t.Fatalf("expected HasConflict=true, got %+v", res)
	}
	if res.FailedCommitHash != conflictHash {
		t.Fatalf("expected failing hash %s, got %q", conflictHash[:8], res.FailedCommitHash)
	}
	if res.SuccessCount != 0 {
		t.Fatalf("no commit should be applied on conflict, got %d", res.SuccessCount)
	}
	status, _ := svc.GetRepoStatus(dir)
	if status == nil || len(status.ConflictedFiles) == 0 {
		t.Fatalf("repository should be in conflict state: %+v", status)
	}
}
