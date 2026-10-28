package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRepoBloatInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-bloat-test-*")
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

	// Commit 1: add a large dummy binary file (e.g. 50KB)
	largeData := make([]byte, 50*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	largeFile := filepath.Join(tempDir, "large_ghost.bin")
	_ = os.WriteFile(largeFile, largeData, 0644)
	runGit("add", "large_ghost.bin")
	runGit("commit", "-m", "add large ghost binary")

	// Commit 2: delete the large file so it becomes a ghost blob!
	runGit("rm", "large_ghost.bin")
	runGit("commit", "-m", "remove large binary from HEAD")

	// Commit 3: add small file
	smallFile := filepath.Join(tempDir, "small.txt")
	_ = os.WriteFile(smallFile, []byte("hello world\n"), 0644)
	runGit("add", "small.txt")
	runGit("commit", "-m", "add small file")

	// Test InspectRepoBloat
	report, err := svc.InspectRepoBloat(tempDir, 10)
	if err != nil {
		t.Fatalf("InspectRepoBloat failed: %v", err)
	}
	if len(report.TopBloatObjects) == 0 {
		t.Errorf("expected top bloat objects, got 0")
	}

	// Verify the ghost blob was identified
	foundGhost := false
	for _, obj := range report.TopBloatObjects {
		if obj.Path == "large_ghost.bin" && obj.IsGhostBlob {
			foundGhost = true
			break
		}
	}
	if !foundGhost {
		t.Errorf("expected large_ghost.bin to be identified as ghost blob")
	}

	// Test GenerateSlimmingPlan
	plan, err := svc.GenerateSlimmingPlan(tempDir, []string{"large_ghost.bin"})
	if err != nil {
		t.Fatalf("GenerateSlimmingPlan failed: %v", err)
	}
	if len(plan.FilterRepoCommand) == 0 {
		t.Errorf("expected non-empty FilterRepoCommand")
	}
	if len(plan.SafetyNotice) == 0 {
		t.Errorf("expected non-empty SafetyNotice")
	}
}
