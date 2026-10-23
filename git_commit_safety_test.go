package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCommitContentSafetyUsesRealCountAndBody(t *testing.T) {
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
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "feat: safe subject", "-m", "body contains secret=abc123")
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "fix: clean subject")

	res, err := GitService{}.AuditCommitContentSafety(repo, 10)
	if err != nil {
		t.Fatalf("audit commit safety: %v", err)
	}
	if !res.AnalysisKnown || res.CommitsAudited != 2 || res.Passed || res.ViolationsCount != 1 {
		t.Fatalf("body secret should be detected with real commit count: %+v", res)
	}
	if !strings.Contains(res.Violations[0].OffendingText, "secret=") {
		t.Fatalf("unexpected violation: %+v", res.Violations[0])
	}
	if !strings.Contains(res.Summary, "实际审计 2") {
		t.Fatalf("summary should report actual count: %q", res.Summary)
	}

	emptyRepo := t.TempDir()
	runEmpty := func(args ...string) {
		t.Helper()
		if out, err := ExecGit(emptyRepo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	runEmpty("init", "-q", "-b", "main")
	empty, err := GitService{}.AuditCommitContentSafety(emptyRepo, 10)
	if err != nil {
		t.Fatalf("audit empty commit safety: %v", err)
	}
	if empty.AnalysisKnown || empty.Passed || empty.CommitsAudited != 0 || !strings.Contains(empty.Summary, "未作") {
		t.Fatalf("empty repo must not pass by default: %+v", empty)
	}
}
