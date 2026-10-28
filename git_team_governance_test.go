package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestTeamGovernanceInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-gov-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := GitService{}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
		return string(out)
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// 1. Inspect initial configs
	rep, err := svc.InspectRepoGitConfigs(tempDir)
	if err != nil {
		t.Fatalf("InspectRepoGitConfigs failed: %v", err)
	}
	if rep.ComplianceScore <= 0 {
		t.Errorf("expected positive score, got %d", rep.ComplianceScore)
	}
	if len(rep.Configs) < 5 {
		t.Errorf("expected at least 5 configs, got %d", len(rep.Configs))
	}

	// 2. Apply Preset "cross-platform"
	applyRes, err := svc.ApplyRecommendedTeamPreset(tempDir, "cross-platform")
	if err != nil {
		t.Fatalf("ApplyRecommendedTeamPreset failed: %v", err)
	}
	if !applyRes.Success || len(applyRes.AppliedSettings) == 0 {
		t.Errorf("expected applied settings, got: %+v", applyRes)
	}

	// 3. Inspect again - score should have improved or core.autocrlf should be set
	repAfter, err := svc.InspectRepoGitConfigs(tempDir)
	if err != nil {
		t.Fatalf("InspectRepoGitConfigs after failed: %v", err)
	}
	foundAutocrlf := false
	for _, c := range repAfter.Configs {
		if c.Key == "core.autocrlf" && c.Value != "" {
			foundAutocrlf = true
		}
	}
	if !foundAutocrlf {
		t.Errorf("expected core.autocrlf to be configured after applying preset")
	}
}
