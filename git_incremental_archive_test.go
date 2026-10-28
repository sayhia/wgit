package main

import (
	"archive/zip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExportChangedFilesArchiveInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-archive-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(dir string, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %s %v failed: %v, out: %s", name, args, err, string(out))
		}
	}

	runCmd(tempDir, "git", "init")
	runCmd(tempDir, "git", "config", "user.name", "TestUser")
	runCmd(tempDir, "git", "config", "user.email", "test@example.com")

	// Commit 1
	fileA := filepath.Join(tempDir, "fileA.txt")
	fileB := filepath.Join(tempDir, "fileB.txt")
	_ = os.WriteFile(fileA, []byte("version 1 of A\n"), 0644)
	_ = os.WriteFile(fileB, []byte("version 1 of B\n"), 0644)
	runCmd(tempDir, "git", "add", ".")
	runCmd(tempDir, "git", "commit", "-m", "first commit")

	// Commit 2: modify A, add C
	fileC := filepath.Join(tempDir, "fileC.txt")
	_ = os.WriteFile(fileA, []byte("version 2 of A - modified\n"), 0644)
	_ = os.WriteFile(fileC, []byte("new file C\n"), 0644)
	runCmd(tempDir, "git", "add", ".")
	runCmd(tempDir, "git", "commit", "-m", "second commit with changes")

	svc := GitService{}

	// Test ExportChangedFilesArchive
	res, err := svc.ExportChangedFilesArchive(tempDir, "HEAD~1", "HEAD", "zip", "")
	if err != nil {
		t.Fatalf("ExportChangedFilesArchive failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success to be true")
	}
	if res.FileCount != 2 {
		t.Fatalf("expected 2 changed files, got %d", res.FileCount)
	}
	defer os.Remove(res.OutputPath)

	// Verify the zip file exists and contains fileA.txt and fileC.txt
	r, err := zip.OpenReader(res.OutputPath)
	if err != nil {
		t.Fatalf("failed to open generated zip file: %v", err)
	}
	defer r.Close()

	found := make(map[string]string)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to read zip entry %s: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		found[f.Name] = string(data)
	}

	if found["fileA.txt"] != "version 2 of A - modified\n" {
		t.Fatalf("unexpected content for fileA.txt in zip: %q", found["fileA.txt"])
	}
	if found["fileC.txt"] != "new file C\n" {
		t.Fatalf("unexpected content for fileC.txt in zip: %q", found["fileC.txt"])
	}
	if _, hasB := found["fileB.txt"]; hasB {
		t.Fatalf("fileB.txt was not changed but found in archive")
	}
}
