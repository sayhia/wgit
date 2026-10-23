package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyPrePushLfsDetectsMissingObjectInRootCommit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "tester@example.com")
	run("config", "user.name", "Tester")

	sha := strings.Repeat("a", 64)
	if err := os.MkdirAll(filepath.Join(repo, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:" + sha + "\nsize 5\n"
	if err := os.WriteFile(filepath.Join(repo, "assets", "root.bin"), []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.bin filter=lfs diff=lfs merge=lfs -text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".gitattributes", "assets/root.bin")
	run("commit", "-qm", "feat: root lfs asset")

	res, err := (GitService{}).VerifyPrePushLfs(repo, "", "")
	if err != nil {
		t.Fatalf("VerifyPrePushLfs failed: %v", err)
	}
	if len(res.UnpushedCommits) != 1 || res.TotalLfsPointers != 1 || res.MissingLfsCount != 1 || res.IsSafe {
		t.Fatalf("root-commit LFS object should be missing and unsafe: %+v", res)
	}
	if len(res.MissingLfsFiles) != 1 || res.MissingLfsFiles[0] != "assets/root.bin" {
		t.Fatalf("missing files = %+v", res.MissingLfsFiles)
	}
}
