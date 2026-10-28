package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestGetSubmoduleMatrixInEmptyRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-submodule-matrix-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		_ = cmd.Run()
	}

	runGit("init")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	service := &GitService{}
	matrix, err := service.GetSubmoduleMatrix(tempDir)
	if err != nil {
		t.Fatalf("GetSubmoduleMatrix failed: %v", err)
	}
	if len(matrix) != 0 {
		t.Errorf("expected 0 submodules in clean repo, got %d", len(matrix))
	}

	syncRes, err := service.SyncSubmodulesRecursive(tempDir)
	if err != nil {
		t.Fatalf("SyncSubmodulesRecursive failed: %v", err)
	}
	if !syncRes.Success {
		t.Errorf("expected Success = true")
	}
}
