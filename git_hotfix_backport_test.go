package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraceHotfixBackportStatusUsesRealCommits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(repo, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	write("base.txt", "base\n")
	run("add", "base.txt")
	run("commit", "-qm", "base")

	run("checkout", "-q", "-b", "hotfix/v1.0.1")
	write("fix-a.txt", "fix a\n")
	run("add", "fix-a.txt")
	run("commit", "-qm", "fix: hotfix A")
	fixA := strings.TrimSpace(run("rev-parse", "HEAD"))
	write("fix-b.txt", "fix b\n")
	run("add", "fix-b.txt")
	run("commit", "-qm", "fix: hotfix B")
	fixB := strings.TrimSpace(run("rev-parse", "HEAD"))

	run("checkout", "-q", "main")
	write("main.txt", "main\n")
	run("add", "main.txt")
	run("commit", "-qm", "feat: main work")
	run("cherry-pick", "-x", fixA)

	svc := GitService{}
	res, err := svc.TraceHotfixBackportStatus(repo, "hotfix/v1.0.1", "main")
	if err != nil {
		t.Fatalf("TraceHotfixBackportStatus failed: %v", err)
	}
	if res.TotalHotfixCommits != 2 || res.BackportedCommits != 1 || res.IsFullyBackported {
		t.Fatalf("expected one missing hotfix commit: %+v", res)
	}
	if len(res.MissingBackports) != 1 || res.MissingBackports[0].Hash != fixB {
		t.Fatalf("unexpected missing commits: %+v", res.MissingBackports)
	}
	if !strings.Contains(res.MissingBackports[0].CherryPickCmd, "git cherry-pick -x "+fixB) {
		t.Fatalf("missing cherry-pick command: %+v", res.MissingBackports[0])
	}

	run("cherry-pick", "-x", fixB)
	res, err = svc.TraceHotfixBackportStatus(repo, "hotfix/v1.0.1", "main")
	if err != nil {
		t.Fatalf("TraceHotfixBackportStatus after full backport failed: %v", err)
	}
	if res.BackportedCommits != 2 || !res.IsFullyBackported || len(res.MissingBackports) != 0 {
		t.Fatalf("expected fully backported hotfix: %+v", res)
	}

	if _, err := svc.TraceHotfixBackportStatus(repo, "missing-hotfix", "main"); err == nil {
		t.Fatal("missing hotfix branch must fail")
	}
}
