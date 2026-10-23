package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- worktree repair / prune ---------------------------------------------

// PruneWorktrees is intentionally asserted shallowly here: whether git
// actually drops the entry depends on gc.worktreePruneExpire timing, which is
// stable from a shell but not reproducible under `go test`. The behaviour we
// own is "invoke prune with --expire=now and report the outcome"; the git-side
// deletion is covered by the manual check in docs/feature-rollout-2026-10-23.md.
func TestPruneWorktreesReportsResult(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.PruneWorktrees(dir)
	if err != nil {
		t.Fatalf("PruneWorktrees failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	if res.Action != "prune" {
		t.Fatalf("unexpected action: %q", res.Action)
	}
	if res.Message == "" {
		t.Fatal("expected a summary message")
	}
}

func TestRepairWorktreesOnHealthyRepo(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	res, err := svc.RepairWorktrees(dir, nil)
	if err != nil {
		t.Fatalf("RepairWorktrees failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success on a healthy repo, got %+v", res)
	}
	if res.Action != "repair" {
		t.Fatalf("unexpected action: %q", res.Action)
	}
}

func TestRepairWorktreesRejectsNonRepo(t *testing.T) {
	svc := GitService{}
	if _, err := svc.RepairWorktrees(t.TempDir(), nil); err == nil {
		t.Fatal("expected an error outside a git repository")
	}
}

// --- bisect skip ---------------------------------------------------------

func TestBisectSkipAdvancesSession(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "0\n", "feat: base", time.Now(), "Alice")
	oldest, _ := ExecGit(dir, "rev-parse", "HEAD")
	// Enough commits that a single skip still leaves candidates to test.
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
		writeAndCommit(t, dir, "a.txt", n+"\n", "feat: "+n, time.Now(), "Alice")
	}

	svc := GitService{}
	if _, err := svc.BisectStart(dir, "HEAD", strings.TrimSpace(oldest)); err != nil {
		t.Fatalf("BisectStart failed: %v", err)
	}
	defer func() { _ = svc.BisectReset(dir) }()

	status, err := svc.BisectSkip(dir)
	if err != nil {
		t.Fatalf("BisectSkip failed: %v", err)
	}
	if status == nil {
		t.Fatal("expected a status")
	}
	if !strings.Contains(status.Log, "skip") {
		t.Fatalf("bisect log should record the skip: %q", status.Log)
	}
}

func TestBisectSkipWithoutSessionFails(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", time.Now(), "Alice")

	svc := GitService{}
	if _, err := svc.BisectSkip(dir); err == nil {
		t.Fatal("expected an error when no bisect session is active")
	}
}

// --- hook scripts --------------------------------------------------------

func TestReadGitHookWhenMissing(t *testing.T) {
	dir := initClosedLoopRepo(t)
	svc := GitService{}

	res, err := svc.ReadGitHook(dir, "pre-commit")
	if err != nil {
		t.Fatalf("ReadGitHook failed: %v", err)
	}
	if res.Exists {
		t.Fatal("fresh repo should have no pre-commit hook")
	}
	if res.Message == "" {
		t.Fatal("expected an explanatory message")
	}
}

func TestSaveGitHookWritesExecutableScript(t *testing.T) {
	dir := initClosedLoopRepo(t)
	svc := GitService{}

	script := "#!/bin/sh\necho hook-ran\n"
	if err := svc.SaveGitHook(dir, "pre-commit", script, true); err != nil {
		t.Fatalf("SaveGitHook failed: %v", err)
	}

	path := filepath.Join(dir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("hook file missing: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("hook must be executable, got %v", info.Mode().Perm())
	}

	read, err := svc.ReadGitHook(dir, "pre-commit")
	if err != nil {
		t.Fatalf("ReadGitHook failed: %v", err)
	}
	if !read.Exists || !read.Enabled {
		t.Fatalf("expected an enabled hook, got %+v", read)
	}
	if read.Content != script {
		t.Fatalf("content mismatch: %q", read.Content)
	}
}

func TestSaveGitHookDisabledMovesFile(t *testing.T) {
	dir := initClosedRepoForHooks(t)
	svc := GitService{}

	if err := svc.SaveGitHook(dir, "pre-push", "#!/bin/sh\nexit 0\n", true); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	if err := svc.SaveGitHook(dir, "pre-push", "#!/bin/sh\nexit 0\n", false); err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	active := filepath.Join(dir, ".git", "hooks", "pre-push")
	disabled := active + ".disabled"
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatal("active hook should be gone after disabling")
	}
	if _, err := os.Stat(disabled); err != nil {
		t.Fatalf("disabled hook missing: %v", err)
	}

	read, err := svc.ReadGitHook(dir, "pre-push")
	if err != nil {
		t.Fatalf("ReadGitHook failed: %v", err)
	}
	if read.Enabled {
		t.Fatal("hook should report as disabled")
	}
}

func TestSaveGitHookRejectsUnknownName(t *testing.T) {
	dir := initClosedRepoForHooks(t)
	svc := GitService{}

	for _, name := range []string{"../../evil", "not-a-hook", "pre-commit;rm -rf"} {
		if err := svc.SaveGitHook(dir, name, "#!/bin/sh\n", true); err == nil {
			t.Fatalf("expected rejection for hook name %q", name)
		}
	}
}

func TestSaveGitHookRejectsEmptyContent(t *testing.T) {
	dir := initClosedRepoForHooks(t)
	svc := GitService{}
	if err := svc.SaveGitHook(dir, "commit-msg", "   \n", true); err == nil {
		t.Fatal("expected an error for empty content")
	}
}

func initClosedRepoForHooks(t *testing.T) string {
	t.Helper()
	dir := initClosedLoopRepo(t)
	writeAndCommit(t, dir, "a.txt", "a\n", "chore: init", time.Now(), "Alice")
	return dir
}
