package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyPrePushLfsUsesCommittedBlobSize(t *testing.T) {
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
	run("config", "user.email", "tester@example.com")
	run("config", "user.name", "Tester")

	const largeSize = 50 * 1024 * 1024
	largePath := filepath.Join(repo, "big.bin")
	f, err := os.Create(largePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(largeSize); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	run("add", "big.bin")
	run("commit", "-qm", "add large binary")

	// Shrinking the working copy must not hide the committed 50 MB blob.
	if err := os.WriteFile(largePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := (GitService{}).VerifyPrePushLfs(repo, "", "")
	if err != nil {
		t.Fatalf("VerifyPrePushLfs failed: %v", err)
	}
	if len(res.OversizedBlobs) != 1 {
		t.Fatalf("expected the committed large blob to be reported: %+v", res)
	}
	if blob := res.OversizedBlobs[0]; blob.Path != "big.bin" || blob.SizeBytes != largeSize || blob.IsLFS {
		t.Fatalf("unexpected oversized blob: %+v", blob)
	}
	if res.IsSafe {
		t.Fatalf("push with a committed 50 MB non-LFS blob should be unsafe: %+v", res)
	}
}
