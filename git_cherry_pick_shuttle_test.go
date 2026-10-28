package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCherryPickShuttleInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-cherrypick-test-*")
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
	_ = os.WriteFile(filepath.Join(tempDir, "base.txt"), []byte("base line 1\n"), 0644)
	runGit("add", "base.txt")
	runGit("commit", "-m", "initial base commit")

	// Create feature branch
	runGit("checkout", "-b", "feature")
	_ = os.WriteFile(filepath.Join(tempDir, "feature_clean.txt"), []byte("clean feature code\n"), 0644)
	runGit("add", "feature_clean.txt")
	runGit("commit", "-m", "feat: clean cherry-pick target")
	featureSha := strings.TrimSpace(runGit("rev-parse", "HEAD"))

	// Switch back to main
	runGit("checkout", "main")

	// 1. Test SimulateCherryPick
	sim, err := svc.SimulateCherryPick(tempDir, []string{featureSha})
	if err != nil {
		t.Fatalf("SimulateCherryPick failed: %v", err)
	}
	if sim.TotalCommits != 1 {
		t.Errorf("expected 1 total commit, got %d", sim.TotalCommits)
	}
	if sim.ViabilityScore < 50 {
		t.Errorf("expected high viability score, got %d", sim.ViabilityScore)
	}
	if len(sim.Steps) != 1 || !sim.Steps[0].CanApplyCleanly {
		t.Errorf("expected step to apply cleanly, got: %+v", sim.Steps)
	}

	// 2. Test ApplyCherryPickBatch
	res, err := svc.ApplyCherryPickBatch(tempDir, []string{featureSha}, true, false)
	if err != nil {
		t.Fatalf("ApplyCherryPickBatch failed: %v", err)
	}
	if !res.Success || res.AppliedCount != 1 {
		t.Errorf("expected success with 1 applied commit, got: %+v", res)
	}

	// Verify file now exists on main
	if _, err := os.Stat(filepath.Join(tempDir, "feature_clean.txt")); err != nil {
		t.Errorf("expected feature_clean.txt to exist after cherry-pick: %v", err)
	}
}
