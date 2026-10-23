package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrePushChecklistUsesNonOriginRemoteRange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}

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
	if err := os.WriteFile(filepath.Join(repo, "next.txt"), []byte("next\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "next.txt")
	run(repo, "commit", "-qm", "feat: next")
	// Simulate a branch without configured upstream; only the remote-tracking ref remains.
	run(repo, "config", "--unset", "branch.main.remote")
	run(repo, "config", "--unset", "branch.main.merge")

	res, err := svc.AuditPrePushChecklist(repo, "main")
	if err != nil {
		t.Fatalf("AuditPrePushChecklist failed: %v", err)
	}
	if res.CommitsAudited != 1 {
		t.Fatalf("expected only the unpushed commit to be audited, got %d (summary=%s)", res.CommitsAudited, res.Summary)
	}
}
