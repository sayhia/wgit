package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetOperationLogPropagatesReflogFailure(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not found: %v", err)
	}
	binDir := t.TempDir()
	wrapper := "#!/bin/sh\n" +
		"if [ \"$1\" = \"reflog\" ]; then\n" +
		"  echo 'fatal: simulated reflog failure' >&2\n" +
		"  exit 128\n" +
		"fi\n" +
		"exec \"$WGIT_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WGIT_REAL_GIT", gitPath)
	t.Setenv("PATH", binDir)

	if _, err := (GitService{}).GetOperationLog(dir, 10); err == nil {
		t.Fatal("a real reflog failure must not be reported as an empty history")
	}
}

func TestGetOperationLogKeepsTimeSeparateFromSubject(t *testing.T) {
	dir := initClosedLoopRepo(t)
	// Git normalizes tabs in commit subjects to spaces in the reflog; the
	// parser must still keep the subject and timestamp in their own fields.
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: with\ttab", time.Now(), "Alice")

	res, err := (GitService{}).GetOperationLog(dir, 5)
	if err != nil {
		t.Fatalf("GetOperationLog failed: %v", err)
	}
	if len(res.Entries) == 0 {
		t.Fatal("expected the commit entry")
	}
	if !strings.Contains(res.Entries[0].Subject, "with tab") {
		t.Fatalf("unexpected subject: %q", res.Entries[0].Subject)
	}
	if strings.Contains(res.Entries[0].Time, "with") || strings.Contains(res.Entries[0].Time, "\t") {
		t.Fatalf("time was polluted by the subject: %q", res.Entries[0].Time)
	}
}

func TestGetOperationLogReturnsEmptyForRepoWithoutCommits(t *testing.T) {
	dir := initClosedLoopRepo(t)
	res, err := (GitService{}).GetOperationLog(dir, 10)
	if err != nil {
		t.Fatalf("an empty repository should not be an error: %v", err)
	}
	if res.Total != 0 || len(res.Entries) != 0 {
		t.Fatalf("expected an empty log, got %+v", res)
	}
}

func TestClassifyReflogActions(t *testing.T) {
	cases := []struct {
		subject  string
		wantKey  string
		undoable bool
	}{
		{"commit: feat: a", "commit", true},
		{"commit (amend): feat: a", "amend", true},
		{"commit (initial): init", "commit-initial", true},
		{"merge feature: Merge made by ort", "merge", true},
		{"cherry-pick: fix: b", "cherry-pick", true},
		{"reset: moving to HEAD~1", "reset", true},
		{"rebase (finish): returning to refs/heads/main", "rebase", false},
		{"checkout: moving from main to dev", "checkout", false},
		{"pull: Fast-forward", "pull", false},
		{"随手一写", "other", false},
	}
	if _, _, _, hint := classifyReflog("reset: moving to HEAD~1"); !strings.Contains(hint, "hard") {
		t.Fatalf("reset undo hint must warn about hard-reset data loss: %q", hint)
	}
	for _, c := range cases {
		key, _, undoable, _ := classifyReflog(c.subject)
		if key != c.wantKey {
			t.Fatalf("classifyReflog(%q) key = %q, want %q", c.subject, key, c.wantKey)
		}
		if undoable != c.undoable {
			t.Fatalf("classifyReflog(%q) undoable = %v, want %v", c.subject, undoable, c.undoable)
		}
	}
}

func TestGetOperationLogReadsReflog(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	writeAndCommit(t, dir, "b.txt", "b\n", "feat: b", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.GetOperationLog(dir, 10)
	if err != nil {
		t.Fatalf("GetOperationLog failed: %v", err)
	}
	if len(res.Entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(res.Entries))
	}
	if res.Entries[0].Action != "commit" {
		t.Fatalf("newest action should be commit, got %q (%q)", res.Entries[0].Action, res.Entries[0].Subject)
	}
	if !res.Entries[0].Undoable {
		t.Fatal("a plain commit must be undoable")
	}
	if !res.CanUndo {
		t.Fatalf("expected CanUndo, message=%q", res.Message)
	}
	if len(res.Entries[0].ShortHash) != 7 {
		t.Fatalf("expected a 7-char short hash, got %q", res.Entries[0].ShortHash)
	}
}

func TestUndoLastOperationKeepsChangesStaged(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	first, _ := ExecGit(dir, "rev-parse", "HEAD")
	first = strings.TrimSpace(first)

	writeAndCommit(t, dir, "b.txt", "b\n", "feat: b", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.UndoLastOperation(dir)
	if err != nil {
		t.Fatalf("UndoLastOperation failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}

	head, _ := ExecGit(dir, "rev-parse", "HEAD")
	if strings.TrimSpace(head) != first {
		t.Fatalf("HEAD did not move back: got %s want %s", strings.TrimSpace(head), first)
	}

	// The undone commit's file must survive as a staged addition.
	staged, _ := ExecGit(dir, "diff", "--cached", "--name-only")
	if !strings.Contains(staged, "b.txt") {
		t.Fatalf("b.txt should be staged after undo, staged=%q", staged)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("b.txt disappeared from the working tree: %v", err)
	}
}

func TestUndoLastOperationRejectsSingleCommit(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	if _, err := svc.UndoLastOperation(dir); err == nil {
		t.Fatal("expected an error when there is nothing to go back to")
	}
}

func TestUndoLastOperationRejectsNonUndoable(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	writeAndCommit(t, dir, "b.txt", "b\n", "feat: b", time.Now(), "Alice")
	runGitTest(t, dir, nil, "checkout", "-b", "feature")

	svc := GitService{}
	if _, err := svc.UndoLastOperation(dir); err == nil {
		t.Fatal("a branch checkout must not be one-click undoable")
	}
}

func TestUndoLastOperationRejectsConflictCheckFailure(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")
	writeAndCommit(t, dir, "b.txt", "b\n", "feat: b", time.Now(), "Alice")

	indexPath := filepath.Join(dir, ".git", "index")
	if err := os.Chmod(indexPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(indexPath, 0o644) })

	svc := GitService{}
	if _, err := svc.UndoLastOperation(dir); err == nil || !strings.Contains(err.Error(), "检查工作区冲突失败") {
		t.Fatalf("failed conflict detection must abort undo: %v", err)
	}
}
