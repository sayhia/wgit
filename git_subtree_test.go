package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddSubtreePersistsMetadata(t *testing.T) {
	lib := t.TempDir()
	main := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v in %s failed: %v", args, dir, err)
		}
	}
	write := func(dir, name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, dir := range []string{lib, main} {
		run(dir, "init", "-q", "-b", "main")
		run(dir, "config", "user.email", "t@example.com")
		run(dir, "config", "user.name", "tester")
	}
	run(lib, "config", "receive.denyCurrentBranch", "ignore")
	write(lib, "lib.txt", "v1\n")
	run(lib, "add", "lib.txt")
	run(lib, "commit", "-qm", "lib base")
	write(main, "README.md", "main\n")
	run(main, "add", "README.md")
	run(main, "commit", "-qm", "main base")

	svc := GitService{}
	if _, err := svc.AddSubtree(main, "libs/lib", lib, "main", true); err != nil {
		t.Fatalf("AddSubtree: %v", err)
	}

	res, err := svc.ListSubtrees(main)
	if err != nil {
		t.Fatalf("ListSubtrees: %v", err)
	}
	if len(res.Subtrees) != 1 {
		t.Fatalf("got %d subtrees, want 1: %+v", len(res.Subtrees), res.Subtrees)
	}
	got := res.Subtrees[0]
	if got.Prefix != "libs/lib" || got.RemoteOrURL != lib || got.Branch != "main" {
		t.Fatalf("subtree metadata = %+v", got)
	}

	// Make the metadata path unwritable (a directory) while leaving git itself
	// healthy: the subtree operation must still succeed and report the warning.
	metaPath := filepath.Join(gitCommonDir(main), "wgit-subtrees.json")
	if err := os.Remove(metaPath); err != nil {
		t.Fatalf("remove subtree metadata: %v", err)
	}
	if err := os.Mkdir(metaPath, 0o755); err != nil {
		t.Fatalf("replace subtree metadata with directory: %v", err)
	}

	write(lib, "lib.txt", "v2\n")
	run(lib, "add", "lib.txt")
	run(lib, "commit", "-qm", "lib v2")
	pulled, err := svc.PullSubtree(main, "libs/lib", lib, "main", true)
	if err != nil {
		t.Fatalf("PullSubtree: %v, result=%+v", err, pulled)
	}
	if !pulled.Success || !strings.Contains(pulled.Message, "本地子树信息保存失败") {
		t.Fatalf("PullSubtree must report metadata persistence failure: %+v", pulled)
	}
	if data, err := os.ReadFile(filepath.Join(main, "libs", "lib", "lib.txt")); err != nil || string(data) != "v2\n" {
		t.Fatalf("PullSubtree did not update working tree: %q err=%v", data, err)
	}

	write(main, filepath.Join("libs", "lib", "lib.txt"), "from-main\n")
	run(main, "add", "libs/lib/lib.txt")
	run(main, "commit", "-qm", "main updates subtree")
	pushed, err := svc.PushSubtree(main, "libs/lib", lib, "main")
	if err != nil {
		t.Fatalf("PushSubtree: %v, result=%+v", err, pushed)
	}
	if !pushed.Success || !strings.Contains(pushed.Message, "本地子树信息保存失败") {
		t.Fatalf("PushSubtree must report metadata persistence failure: %+v", pushed)
	}
	if err := os.Remove(metaPath); err != nil {
		t.Fatalf("remove metadata directory: %v", err)
	}
	pushedContent, err := ExecGit(lib, "show", "main:lib.txt")
	if err != nil || pushedContent != "from-main" {
		t.Fatalf("PushSubtree did not update upstream: %q err=%v", pushedContent, err)
	}
	res, _ = svc.ListSubtrees(main)
	if len(res.Subtrees) != 1 || res.Subtrees[0].Prefix != "libs/lib" {
		t.Fatalf("subtree metadata lost after pull/push: %+v", res.Subtrees)
	}
	if _, err := svc.PullSubtree(main, "", lib, "main", true); err == nil {
		t.Fatal("expected error for empty subtree prefix")
	}
	if _, err := svc.PushSubtree(main, "libs/lib", "", "main"); err == nil {
		t.Fatal("expected error for empty subtree URL")
	}
}

func TestSubtreeMetadataCorruptionIsNotOverwritten(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "config", "user.name", "Tester"); err != nil {
		t.Fatalf("git config: %v (%s)", err, out)
	}
	if out, err := ExecGit(repo, "config", "user.email", "tester@example.com"); err != nil {
		t.Fatalf("git config: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "base"}} {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	metaPath := subtreeMetadataPath(repo)
	corrupt := []byte(`{"libs/lib":`)
	if err := os.WriteFile(metaPath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := (GitService{}).ListSubtrees(repo); err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("ListSubtrees must report corrupt metadata, got %v", err)
	}
	if err := saveSubtreeMetadata(repo, "libs/lib", subtreeMetadata{RemoteOrURL: "x", Branch: "main"}); err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("saveSubtreeMetadata must refuse to overwrite corrupt metadata, got %v", err)
	}
	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read corrupt metadata: %v", err)
	}
	if !bytes.Equal(after, corrupt) {
		t.Fatalf("corrupt metadata must stay untouched, got %q", after)
	}
}
