package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateHotfixManifestValidatesRefsAndSupportsWorktree(t *testing.T) {
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
	run("commit", "-qm", "base")

	if _, err := (GitService{}).GenerateHotfixManifest(repo, "missing-base", "HEAD", "v1"); err == nil {
		t.Fatal("invalid base ref must return an error")
	}
	if _, err := (GitService{}).GenerateHotfixManifest(repo, "HEAD", "missing-target", "v1"); err == nil {
		t.Fatal("invalid target ref must return an error")
	}
	if _, err := (GitService{}).GenerateHotfixManifest(repo, "WORKTREE", "HEAD", "v1"); err == nil {
		t.Fatal("WORKTREE base must be rejected")
	}

	if err := os.WriteFile(filepath.Join(repo, "hot.txt"), []byte("hotfix"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "add hot")
	baseHashOut, err := ExecGit(repo, "rev-parse", "HEAD~1")
	if err != nil {
		t.Fatalf("rev-parse base: %v", err)
	}
	baseHash := baseHashOut
	if err := os.WriteFile(filepath.Join(repo, "hot.txt"), []byte("hotfix v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := (GitService{}).GenerateHotfixManifest(repo, baseHash, "WORKTREE", "v1")
	if err != nil {
		t.Fatalf("generate worktree hotfix manifest: %v", err)
	}
	if !res.Success || res.TotalFilesCount == 0 {
		t.Fatalf("expected changes from WORKTREE: %+v", res)
	}
	found := false
	for _, file := range res.PatchFiles {
		if file.Path == "hot.txt" {
			found = true
			if file.Action != "ADDED" && file.Action != "MODIFIED" {
				t.Fatalf("unexpected hot.txt action: %+v", file)
			}
		}
	}
	if !found {
		t.Fatalf("hot.txt missing from manifest: %+v", res.PatchFiles)
	}
}
