package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStudioSwitchboardDeduplicatesSymlinkedRepos(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	root := t.TempDir()
	realRepo := filepath.Join(root, "real")
	if err := os.MkdirAll(realRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		if out, err := ExecGit(realRepo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	run("commit", "-q", "--allow-empty", "-m", "initial")

	linkRepo := filepath.Join(root, "link")
	if err := os.Symlink(realRepo, linkRepo); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	res, err := (GitService{}).ManageStudioSwitchboard(linkRepo, "LIST", "", "", "")
	if err != nil {
		t.Fatalf("ManageStudioSwitchboard failed: %v", err)
	}
	if res.TotalProjects != 1 || res.AdjacentRepos != 0 || res.LinkedWorktrees != 0 {
		t.Fatalf("symlinked repo must not be counted twice: %+v", res)
	}
	if len(res.Projects) != 1 || res.Projects[0].Category == "AdjacentRepo" {
		t.Fatalf("unexpected project list: %+v", res.Projects)
	}
	if res.ActiveProjectPath != filepath.Clean(linkRepo) {
		t.Fatalf("active path should preserve the user-supplied path: %q", res.ActiveProjectPath)
	}
}
