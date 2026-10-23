package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckStagedNonLfsLargeFilesUsesIndexSize(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "base.txt")
	run("commit", "-qm", "base")

	const size = 2 * 1024 * 1024
	bigPath := filepath.Join(repo, "big.bin")
	f, err := os.Create(bigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	run("add", "big.bin")

	// Shrinking the working copy must not hide the staged 2 MB blob.
	if err := os.WriteFile(bigPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := (GitService{}).CheckStagedNonLfsLargeFiles(repo, 1)
	if err != nil {
		t.Fatalf("CheckStagedNonLfsLargeFiles failed: %v", err)
	}
	if len(files) != 1 || files[0].Path != "big.bin" || files[0].SizeBytes != size {
		t.Fatalf("staged blob size should be used: %+v", files)
	}
}
