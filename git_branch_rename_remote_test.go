package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameBranchSyncsConfiguredNonOriginRemote(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}

	parent := t.TempDir()
	remoteDir := filepath.Join(parent, "remote.git")
	if err := os.MkdirAll(remoteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := ExecGit(dir, args...)
		if err != nil {
			t.Fatalf("git %v in %s failed: %v (%s)", args, dir, err, out)
		}
		return out
	}
	run(remoteDir, "init", "--bare", "-q")

	repo := filepath.Join(parent, "work")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run(repo, "init", "-q", "-b", "main")
	run(repo, "config", "user.email", "tester@example.com")
	run(repo, "config", "user.name", "Tester")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "base.txt")
	run(repo, "commit", "-qm", "chore: base")
	run(repo, "remote", "add", "upstream", remoteDir)
	run(repo, "push", "-q", "-u", "upstream", "main")
	run(repo, "checkout", "-q", "-b", "topic")
	run(repo, "push", "-q", "-u", "upstream", "topic")

	if err := svc.RenameBranchWithRemote(repo, "topic", "topic-renamed", true); err != nil {
		t.Fatalf("RenameBranchWithRemote failed: %v", err)
	}
	if current := strings.TrimSpace(run(repo, "symbolic-ref", "--short", "HEAD")); current != "topic-renamed" {
		t.Fatalf("local branch = %q, want topic-renamed", current)
	}
	remoteBranches := run(remoteDir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if !strings.Contains(remoteBranches, "topic-renamed") || strings.Contains(remoteBranches, "\ntopic\n") {
		t.Fatalf("remote branches after rename = %q", remoteBranches)
	}
	if remote := strings.TrimSpace(run(repo, "config", "--get", "branch.topic-renamed.remote")); remote != "upstream" {
		t.Fatalf("tracking remote = %q, want upstream", remote)
	}
}
