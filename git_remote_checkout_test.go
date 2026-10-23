package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckoutRemoteBranchFromNonOriginRemote(t *testing.T) {
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

	work := filepath.Join(parent, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	run(remoteDir, "init", "--bare", "-q")
	run(work, "init", "-q", "-b", "main")
	run(work, "config", "user.email", "tester@example.com")
	run(work, "config", "user.name", "Tester")
	if err := os.WriteFile(filepath.Join(work, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "add", "base.txt")
	run(work, "commit", "-qm", "chore: base")
	run(work, "remote", "add", "origin", remoteDir)
	run(work, "push", "-q", "origin", "main")

	run(work, "checkout", "-q", "-b", "topic")
	if err := os.WriteFile(filepath.Join(work, "topic.txt"), []byte("topic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "add", "topic.txt")
	run(work, "commit", "-qm", "feat: topic")
	run(work, "push", "-q", "origin", "topic")
	run(work, "checkout", "-q", "main")
	run(work, "remote", "rename", "origin", "upstream")
	run(work, "branch", "-D", "topic")

	if err := svc.CheckoutBranch(work, "upstream/topic", false); err != nil {
		t.Fatalf("CheckoutBranch(upstream/topic) failed: %v", err)
	}
	if current := strings.TrimSpace(run(work, "symbolic-ref", "--short", "HEAD")); current != "topic" {
		t.Fatalf("expected local branch topic, got %q", current)
	}
	if upstream := strings.TrimSpace(run(work, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")); upstream != "upstream/topic" {
		t.Fatalf("expected upstream tracking upstream/topic, got %q", upstream)
	}
	if _, err := os.Stat(filepath.Join(work, "topic.txt")); err != nil {
		t.Fatalf("topic.txt missing after checkout: %v", err)
	}

	if err := os.WriteFile(filepath.Join(work, "local.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "add", "local.txt")
	run(work, "commit", "-qm", "feat: local only")
	localHead := strings.TrimSpace(run(work, "rev-parse", "HEAD"))
	run(work, "checkout", "-q", "main")
	if err := svc.CheckoutBranch(work, "upstream/topic", false); err != nil {
		t.Fatalf("second CheckoutBranch(upstream/topic) failed: %v", err)
	}
	if got := strings.TrimSpace(run(work, "rev-parse", "HEAD")); got != localHead {
		t.Fatalf("local branch was reset: HEAD=%s want %s", got, localHead)
	}
	if current := strings.TrimSpace(run(work, "symbolic-ref", "--short", "HEAD")); current != "topic" {
		t.Fatalf("expected topic after second checkout, got %q", current)
	}
}
