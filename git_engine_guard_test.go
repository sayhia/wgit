package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRunningGameEngines(t *testing.T) {
	tempDir := t.TempDir()
	svc := GitService{}

	// 1. Clean repo with no locks
	status, err := svc.DetectRunningGameEngines(tempDir)
	if err != nil {
		t.Fatalf("DetectRunningGameEngines clean failed: %v", err)
	}
	if status == nil {
		t.Fatalf("expected non-nil status")
	}

	// 2. Unity lock file
	unityTemp := filepath.Join(tempDir, "Temp")
	_ = os.MkdirAll(unityTemp, 0o755)
	unityLock := filepath.Join(unityTemp, "UnityLockfile")
	if err := os.WriteFile(unityLock, []byte("12345"), 0o644); err != nil {
		t.Fatalf("failed to write unity lock: %v", err)
	}

	unityStatus, err := svc.DetectRunningGameEngines(tempDir)
	if err != nil || !unityStatus.IsRunning || unityStatus.EngineName != "Unity" || unityStatus.CanSafelyCheckout {
		t.Fatalf("expected Unity lock detected, got %+v err=%v", unityStatus, err)
	}
	_ = os.Remove(unityLock)

	// 3. Unreal lock file
	unrealLock := filepath.Join(tempDir, ".editorlock")
	if err := os.WriteFile(unrealLock, []byte("unreal pid"), 0o644); err != nil {
		t.Fatalf("failed to write unreal lock: %v", err)
	}

	unrealStatus, err := svc.DetectRunningGameEngines(tempDir)
	if err != nil || !unrealStatus.IsRunning || unrealStatus.EngineName != "Unreal Engine" || unrealStatus.CanSafelyCheckout {
		t.Fatalf("expected Unreal lock detected, got %+v err=%v", unrealStatus, err)
	}
	_ = os.Remove(unrealLock)

	// 4. Godot lock file
	godotDir := filepath.Join(tempDir, ".godot")
	_ = os.MkdirAll(godotDir, 0o755)
	godotLock := filepath.Join(godotDir, "editor_lock")
	if err := os.WriteFile(godotLock, []byte("godot lock"), 0o644); err != nil {
		t.Fatalf("failed to write godot lock: %v", err)
	}

	godotStatus, err := svc.DetectRunningGameEngines(tempDir)
	if err != nil || !godotStatus.IsRunning || godotStatus.EngineName != "Godot" || godotStatus.CanSafelyCheckout {
		t.Fatalf("expected Godot lock detected, got %+v err=%v", godotStatus, err)
	}
}

func TestLaunchExternalDiffTool(t *testing.T) {
	tempDir := t.TempDir()
	svc := GitService{}

	// Validation
	if _, err := svc.LaunchExternalDiffTool("", "vscode", "HEAD~1", "HEAD", "test.txt"); err == nil {
		t.Fatalf("expected error on empty repoPath")
	}
	if _, err := svc.LaunchExternalDiffTool(tempDir, "vscode", "HEAD~1", "HEAD", ""); err == nil {
		t.Fatalf("expected error on empty filePath")
	}

	// Init test repo
	_, _ = ExecGit(tempDir, "init")
	_, _ = ExecGit(tempDir, "config", "user.name", "Test")
	_, _ = ExecGit(tempDir, "config", "user.email", "test@test.com")
	testFile := filepath.Join(tempDir, "asset.txt")
	_ = os.WriteFile(testFile, []byte("v1"), 0o644)
	_, _ = ExecGit(tempDir, "add", "asset.txt")
	_, _ = ExecGit(tempDir, "commit", "-m", "init")

	_ = os.WriteFile(testFile, []byte("v2"), 0o644)

	res, err := svc.LaunchExternalDiffTool(tempDir, "unknown_tool", "HEAD", "WORKTREE", "asset.txt")
	if err != nil {
		t.Fatalf("LaunchExternalDiffTool failed: %v", err)
	}
	if res == nil || !res.Launched {
		t.Fatalf("expected launched diff tool result, got: %+v", res)
	}
	if res.LeftPath == "" || res.RightPath == "" {
		t.Fatalf("expected valid left and right paths, got: %+v", res)
	}
}
