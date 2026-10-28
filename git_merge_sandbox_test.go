package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMergeSandboxInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-sandbox-test-*")
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

	// Commit 1 on main
	fileA := filepath.Join(tempDir, "core_api.go")
	_ = os.WriteFile(fileA, []byte("package main\n\nfunc Run() {}\n"), 0644)
	runGit("add", "core_api.go")
	runGit("commit", "-m", "initial commit on main")

	// Branch feature-1
	runGit("checkout", "-b", "feature-1")
	fileB := filepath.Join(tempDir, "feature.go")
	_ = os.WriteFile(fileB, []byte("package main\n\nfunc Feature() {}\n"), 0644)
	runGit("add", "feature.go")
	runGit("commit", "-m", "feat: add feature")

	// Switch back to main
	runGit("checkout", "main")

	// 1. Test DryRunPullOrMerge (fast forward / clean merge)
	report, err := svc.DryRunPullOrMerge(tempDir, "feature-1", false)
	if err != nil {
		t.Fatalf("DryRunPullOrMerge failed: %v", err)
	}
	if !report.CanMergeCleanly {
		t.Errorf("expected clean merge with feature-1")
	}
	if !report.IsFastForward {
		t.Errorf("expected fast-forward merge")
	}

	// 2. Test CalculateMergeImpactMatrix
	impact, err := svc.CalculateMergeImpactMatrix(tempDir, "feature-1")
	if err != nil {
		t.Fatalf("CalculateMergeImpactMatrix failed: %v", err)
	}
	if impact.TotalFilesChanged != 1 {
		t.Errorf("expected 1 file changed, got %d", impact.TotalFilesChanged)
	}
	if impact.CoreFilesCount != 1 {
		t.Errorf("expected 1 core file, got %d", impact.CoreFilesCount)
	}
}
