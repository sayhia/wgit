package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAheadBehindCommitQueriesRequireRefs(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	writeAndCommit(t, repo, "a.txt", "a\n", "chore: base", time.Now(), "Tester")
	branchOut, err := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	branch := strings.TrimSpace(branchOut)

	if _, err := svc.GetAheadCommits(repo, "origin", branch); err == nil || !strings.Contains(err.Error(), "远端分支") {
		t.Fatalf("missing tracking ref must not fabricate ahead commits: %v", err)
	}
	if _, err := svc.GetAheadCommits(repo, "origin", "missing-branch"); err == nil {
		t.Fatal("missing local branch must return an error")
	}
	behind, err := svc.GetBehindCommits(repo, "origin", branch)
	if err != nil {
		t.Fatalf("missing remote branch should mean nothing to pull: %v", err)
	}
	if len(behind) != 0 {
		t.Fatalf("missing remote branch should produce no behind commits: %+v", behind)
	}
}

func TestAheadBehindCommitsDefaultResolveAndTracking(t *testing.T) {
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	svc := GitService{}
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatal(err)
	}
	writeAndCommit(t, repo, "a.txt", "a\n", "chore: base", time.Now(), "Tester")
	bare := filepath.Join(parent, "remote.git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(bare, "init", "--bare"); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "remote", "add", "origin", bare); err != nil {
		t.Fatal(err)
	}
	branchOut, err := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	branch := strings.TrimSpace(branchOut)
	if _, err := ExecGit(repo, "push", "-u", "origin", branch); err != nil {
		t.Fatalf("initial push failed: %v", err)
	}

	ahead, err := svc.GetAheadCommits(repo, "", "")
	if err != nil || len(ahead) != 0 {
		t.Fatalf("default ahead query failed: %+v err=%v", ahead, err)
	}
	behind, err := svc.GetBehindCommits(repo, "", "")
	if err != nil || len(behind) != 0 {
		t.Fatalf("default behind query failed: %+v err=%v", behind, err)
	}

	writeAndCommit(t, repo, "b.txt", "b\n", "feat: local", time.Now(), "Tester")
	ahead, err = svc.GetAheadCommits(repo, "", "")
	if err != nil || len(ahead) != 1 || ahead[0].Subject != "feat: local" {
		t.Fatalf("expected one ahead commit, got %+v err=%v", ahead, err)
	}
}
