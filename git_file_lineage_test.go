package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFileLineageInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-lineage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := GitService{}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
		return string(out)
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// Commit 1: create old_utils.go
	oldFile := filepath.Join(tempDir, "old_utils.go")
	_ = os.WriteFile(oldFile, []byte("package utils\n\nfunc CommonHelper() int {\n\treturn 42\n}\n"), 0644)
	runGit("add", "old_utils.go")
	runGit("commit", "-m", "add old_utils.go")

	// Commit 2: rename old_utils.go to new_utils.go
	runGit("mv", "old_utils.go", "new_utils.go")
	runGit("commit", "-m", "rename old_utils.go to new_utils.go")

	// 1. Test TraceFileRenamesAcrossHistory
	report, err := svc.TraceFileRenamesAcrossHistory(tempDir, "new_utils.go")
	if err != nil {
		t.Fatalf("TraceFileRenamesAcrossHistory failed: %v", err)
	}
	if report.TotalRenames != 1 {
		t.Errorf("expected 1 rename, got %d", report.TotalRenames)
	}
	if report.OriginPath != "old_utils.go" {
		t.Errorf("expected originPath 'old_utils.go', got '%s'", report.OriginPath)
	}

	// 2. Test GetCrossFileBlame
	blame, err := svc.GetCrossFileBlame(tempDir, "new_utils.go", 3)
	if err != nil {
		t.Fatalf("GetCrossFileBlame failed: %v", err)
	}
	if blame.Author == "" {
		t.Errorf("expected non-empty author")
	}
}
