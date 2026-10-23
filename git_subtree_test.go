package main

import (
	"os"
	"path/filepath"
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

	write(lib, "lib.txt", "v2\n")
	run(lib, "add", "lib.txt")
	run(lib, "commit", "-qm", "lib v2")
	if pulled, err := svc.PullSubtree(main, "libs/lib", lib, "main", true); err != nil {
		t.Fatalf("PullSubtree: %v, result=%+v", err, pulled)
	}
	if data, err := os.ReadFile(filepath.Join(main, "libs", "lib", "lib.txt")); err != nil || string(data) != "v2\n" {
		t.Fatalf("PullSubtree did not update working tree: %q err=%v", data, err)
	}

	write(main, filepath.Join("libs", "lib", "lib.txt"), "from-main\n")
	run(main, "add", "libs/lib/lib.txt")
	run(main, "commit", "-qm", "main updates subtree")
	if pushed, err := svc.PushSubtree(main, "libs/lib", lib, "main"); err != nil {
		t.Fatalf("PushSubtree: %v, result=%+v", err, pushed)
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
