package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEvaluateBranchHealthInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-branch-health-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runGit := func(args ...string) {
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
	}

	runGit("init")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// Base commit on main
	f1 := filepath.Join(tempDir, "base.txt")
	_ = os.WriteFile(f1, []byte("line1\nline2\n"), 0644)
	runGit("add", "base.txt")
	runGit("commit", "-m", "initial commit on main")
	runGit("branch", "-M", "main")

	// Branch feature
	runGit("checkout", "-b", "feature/awesome")
	f2 := filepath.Join(tempDir, "feat.txt")
	_ = os.WriteFile(f2, []byte("feature code\n"), 0644)
	runGit("add", "feat.txt")
	runGit("commit", "-m", "feat: add feature code")

	service := &GitService{}
	report, err := service.EvaluateBranchHealth(tempDir, "feature/awesome", "main")
	if err != nil {
		t.Fatalf("EvaluateBranchHealth failed: %v", err)
	}

	if report.AheadCount != 1 {
		t.Errorf("expected AheadCount = 1, got %d", report.AheadCount)
	}
	if report.BehindCount != 0 {
		t.Errorf("expected BehindCount = 0, got %d", report.BehindCount)
	}
	if !report.CanFastForward {
		t.Errorf("expected CanFastForward = true")
	}
	if report.ReadinessScore < 80 {
		t.Errorf("expected ReadinessScore >= 80, got %d", report.ReadinessScore)
	}
	if report.ReadinessLevel != "ready" {
		t.Errorf("expected ReadinessLevel = 'ready', got %s", report.ReadinessLevel)
	}
}
