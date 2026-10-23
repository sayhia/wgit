package main

import (
	"testing"
	"time"
)

func TestGetWorktreesRejectsInvalidRepo(t *testing.T) {
	svc := GitService{}
	if _, err := svc.GetWorktrees(t.TempDir()); err == nil {
		t.Fatal("expected error when listing worktrees outside a Git repository")
	}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	worktrees, err := svc.GetWorktrees(repo)
	if err != nil {
		t.Fatalf("GetWorktrees failed: %v", err)
	}
	if worktrees == nil || len(worktrees) == 0 {
		t.Fatalf("valid repository should list its main worktree: %+v", worktrees)
	}
}

func TestGetRebaseCommitsRejectsInvalidUpstream(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	writeAndCommit(t, repo, "a.txt", "a\n", "chore: base", time.Now(), "Tester")
	if _, err := svc.GetRebaseCommits(repo, "definitely-not-a-ref"); err == nil {
		t.Fatal("invalid upstream must return an error instead of an empty commit list")
	}
	commits, err := svc.GetRebaseCommits(repo, "HEAD")
	if err != nil {
		t.Fatalf("same-ref rebase list failed: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("HEAD..HEAD should be empty, got %+v", commits)
	}
}
