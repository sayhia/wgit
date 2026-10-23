package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBindCommitToTAPDRejectsUnknownCommit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	run("commit", "-q", "--allow-empty", "-m", "initial")

	if _, err := (GitService{}).BindCommitToTAPD(repo, "deadbeef", "1002341", "resolved"); err == nil {
		t.Fatal("binding an unknown commit must fail instead of reporting success")
	}
}
