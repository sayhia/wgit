package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPushRemoteResolution(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	parent := t.TempDir()
	remoteDir := filepath.Join(parent, "remote.git")
	if err := os.MkdirAll(remoteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v in %s failed: %v", args, dir, err)
		}
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

	if got := defaultPushRemote(repo); got != "upstream" {
		t.Fatalf("tracking remote = %q, want upstream", got)
	}
	if got := defaultUpstreamRef(repo); got != "upstream/main" {
		t.Fatalf("default upstream = %q, want upstream/main", got)
	}
	if _, err := (GitService{}).Fetch(repo, ""); err != nil {
		t.Fatalf("Fetch with default remote failed for non-origin repo: %v", err)
	}
	run(repo, "tag", "v1.0.0")
	if _, err := (GitService{}).PushTag(repo, "", "v1.0.0"); err != nil {
		t.Fatalf("PushTag with default remote failed: %v", err)
	}
	tags, err := ExecGit(remoteDir, "for-each-ref", "--format=%(refname:short)", "refs/tags")
	if err != nil || !strings.Contains(tags, "v1.0.0") {
		t.Fatalf("tag not pushed to upstream: %q err=%v", tags, err)
	}
	if _, err := (GitService{}).DeleteRemoteTag(repo, "", "v1.0.0"); err != nil {
		t.Fatalf("DeleteRemoteTag with default remote failed: %v", err)
	}
	tags, err = ExecGit(remoteDir, "for-each-ref", "--format=%(refname:short)", "refs/tags")
	if err != nil || strings.Contains(tags, "v1.0.0") {
		t.Fatalf("tag still on upstream after delete: %q err=%v", tags, err)
	}

	if err := os.RemoveAll(filepath.Join(repo, ".git", "refs", "remotes", "upstream")); err != nil {
		t.Fatal(err)
	}
	run(repo, "config", "--unset", "branch.main.remote")
	run(repo, "config", "--unset", "branch.main.merge")
	if got := defaultPushRemote(repo); got != "upstream" {
		t.Fatalf("fallback remote = %q, want upstream", got)
	}
}
