package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
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

func TestGenerateHotfixManifestHandlesSpacedAndRenamedPaths(t *testing.T) {
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
	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	write("base.txt", "base")
	write("old.txt", "rename me")
	write("art/my file.txt", "v1")
	run("add", "-A")
	run("commit", "-qm", "base")

	write("art/my file.txt", "v2 with spaces")
	run("add", "-A")
	run("commit", "-qm", "update spaced file")

	run("mv", "old.txt", "new name.txt")
	run("commit", "-qm", "rename file")

	baseHashOut, err := ExecGit(repo, "rev-parse", "HEAD~2")
	if err != nil {
		t.Fatalf("rev-parse base: %v", err)
	}

	res, err := (GitService{}).GenerateHotfixManifest(repo, strings.TrimSpace(baseHashOut), "HEAD", "v1")
	if err != nil {
		t.Fatalf("GenerateHotfixManifest: %v", err)
	}
	spacedSHA := sha256.Sum256([]byte("v2 with spaces"))
	wantSpacedSHA := hex.EncodeToString(spacedSHA[:])
	foundSpaced, foundRenamed := false, false
	for _, file := range res.PatchFiles {
		switch file.Path {
		case "art/my file.txt":
			foundSpaced = true
			if file.Action != "MODIFIED" || file.Size != int64(len("v2 with spaces")) || file.SHA256 != wantSpacedSHA {
				t.Fatalf("spaced path entry is wrong: %+v", file)
			}
		case "new name.txt":
			foundRenamed = true
			if file.Action != "MODIFIED" || file.Size != int64(len("rename me")) {
				t.Fatalf("renamed path entry is wrong: %+v", file)
			}
		case "old.txt":
			t.Fatalf("rename must be reported by its new path, got %+v", file)
		}
	}
	if !foundSpaced || !foundRenamed {
		t.Fatalf("manifest is missing spaced/renamed entries: %+v", res.PatchFiles)
	}
	delta := strings.Join(res.DeltaPackageList, "\n")
	if !strings.Contains(delta, "art/my file.txt") || !strings.Contains(delta, "new name.txt") {
		t.Fatalf("delta package list is missing paths: %+v", res.DeltaPackageList)
	}

	// A WORKTREE entry whose content cannot be read must abort the manifest
	// instead of silently hashing empty bytes.
	if err := os.Remove(filepath.Join(repo, "art", "my file.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, "dir-target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "dir-target"), filepath.Join(repo, "art", "my file.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := (GitService{}).GenerateHotfixManifest(repo, "HEAD", "WORKTREE", "v1"); err == nil || !strings.Contains(err.Error(), "读取热更文件") {
		t.Fatalf("unreadable WORKTREE file must abort the manifest, got %v", err)
	}
}
