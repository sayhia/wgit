package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStudioSwitchboardReportsWorktreeListFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.name", "Tester"},
		{"config", "user.email", "tester@example.com"},
	} {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "worktree" ]; then
  echo "simulated worktree failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := (GitService{}).ManageStudioSwitchboard(repo, "LIST", "", "", ""); err == nil {
		t.Fatal("worktree list failure must return an error instead of an empty topology")
	}
}

func TestStudioSwitchboardReportsProjectStatusFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	parent := t.TempDir()
	makeRepo := func(name string) string {
		dir := filepath.Join(parent, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{
			{"init", "-q", "-b", "main"},
			{"config", "user.name", "Tester"},
			{"config", "user.email", "tester@example.com"},
		} {
			if out, err := ExecGit(dir, args...); err != nil {
				t.Fatalf("git %v failed: %v (%s)", args, err, out)
			}
		}
		if out, err := ExecGit(dir, "commit", "-q", "--allow-empty", "-m", "initial"); err != nil {
			t.Fatalf("commit failed: %v (%s)", err, out)
		}
		return dir
	}
	mainRepo := makeRepo("studio-main")
	adjacentRepo := makeRepo("studio-adjacent")
	if err := os.WriteFile(filepath.Join(adjacentRepo, ".git", "index"), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := (GitService{}).ManageStudioSwitchboard(mainRepo, "LIST", "", "", "")
	if err != nil {
		t.Fatalf("ManageStudioSwitchboard failed: %v", err)
	}
	var adjacent *StudioProjectNode
	for i := range res.Projects {
		if res.Projects[i].Name == "studio-adjacent" {
			adjacent = &res.Projects[i]
			break
		}
	}
	if adjacent == nil {
		t.Fatalf("adjacent repo missing from topology: %+v", res.Projects)
	}
	if !strings.Contains(adjacent.StatusWarning, "读取工作区状态失败") {
		t.Fatalf("status read failure must be surfaced for the adjacent repo: %+v", adjacent)
	}
}

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

	if _, err := (GitService{}).ManageStudioSwitchboard(linkRepo, "NOT_AN_ACTION", "", "", ""); err == nil {
		t.Fatal("unknown switchboard actions must fail instead of silently listing")
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
