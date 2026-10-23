package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSquashMergeCreatesSingleCommitWithMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}

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
	run("config", "user.email", "tester@example.com")
	run("config", "user.name", "Tester")
	write("base.txt", "base\n")
	run("add", "base.txt")
	run("commit", "-qm", "chore: base")

	run("checkout", "-q", "-b", "feature")
	write("sq1.txt", "squash 1\n")
	run("add", "sq1.txt")
	run("commit", "-qm", "feat: one")
	write("sq2.txt", "squash 2\n")
	run("add", "sq2.txt")
	run("commit", "-qm", "feat: two")

	run("checkout", "-q", "main")
	res, err := svc.MergeAdvanced(repo, MergeAdvancedOptions{
		TargetBranch:  "feature",
		Strategy:      "squash",
		CommitMessage: "feat: squashed feature",
	})
	if err != nil || res == nil || !res.Success {
		t.Fatalf("squash merge failed: %v res=%+v", err, res)
	}
	last, err := svc.GetLastCommitMessage(repo)
	if err != nil || !strings.Contains(last, "feat: squashed feature") {
		t.Fatalf("squash commit message = %q err=%v", last, err)
	}
	if count := strings.TrimSpace(run("rev-list", "--count", "HEAD")); count != "2" {
		t.Fatalf("expected base + one squash commit, got %s commits", count)
	}
	status, err := svc.GetRepoStatus(repo)
	if err != nil || len(status.StagedFiles) != 0 || len(status.UnstagedFiles) != 0 {
		t.Fatalf("expected clean workspace after squash commit, got %+v err=%v", status, err)
	}
	for _, name := range []string{"sq1.txt", "sq2.txt"} {
		if _, err := os.Stat(filepath.Join(repo, name)); err != nil {
			t.Fatalf("%s missing after squash merge: %v", name, err)
		}
	}
}
