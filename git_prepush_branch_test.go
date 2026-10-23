package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditPrePushChecklistExplicitBranchAuditsThatBranch(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "feat: base")

	run("checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("feature"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "wip")

	feature, err := GitService{}.AuditPrePushChecklist(repo, "feature")
	if err != nil {
		t.Fatalf("audit feature branch: %v", err)
	}
	if feature.CommitsAudited != 2 {
		t.Fatalf("expected two feature commits, got %d", feature.CommitsAudited)
	}
	foundWip := false
	for _, violation := range feature.Violations {
		if strings.Contains(violation.Description, "wip") {
			foundWip = true
		}
	}
	if !foundWip {
		t.Fatalf("feature branch wip commit should be audited: %+v", feature.Violations)
	}

	mainRes, err := GitService{}.AuditPrePushChecklist(repo, "main")
	if err != nil {
		t.Fatalf("audit main branch: %v", err)
	}
	if mainRes.CommitsAudited != 1 {
		t.Fatalf("expected only main commit, got %d", mainRes.CommitsAudited)
	}
	for _, violation := range mainRes.Violations {
		if strings.Contains(violation.Description, "wip") {
			t.Fatal("main audit must not inspect feature branch commits")
		}
	}

	if _, err := (GitService{}).AuditPrePushChecklist(repo, "missing-branch"); err == nil {
		t.Fatal("missing explicit branch must return an error")
	}
}
