package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeDirectoryLifecycle(t *testing.T) {
	tempGlobal := filepath.Join(t.TempDir(), "mock_gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", tempGlobal)
	defer os.Remove(tempGlobal)

	svc := GitService{}
	repoPath := "/Users/test/workspace/myrepo"

	// 1. Initial check: should not be safe
	isSafe, err := svc.CheckSafeDirectory(repoPath)
	if err != nil {
		t.Fatalf("CheckSafeDirectory failed: %v", err)
	}
	if isSafe {
		t.Fatalf("expected isSafe to be false initially")
	}

	// 2. Add safe directory
	if err := svc.AddSafeDirectory(repoPath); err != nil {
		t.Fatalf("AddSafeDirectory failed: %v", err)
	}

	// 3. Check again: should be safe now
	isSafe, err = svc.CheckSafeDirectory(repoPath)
	if err != nil {
		t.Fatalf("CheckSafeDirectory after add failed: %v", err)
	}
	if !isSafe {
		t.Fatalf("expected isSafe to be true after AddSafeDirectory")
	}

	// 4. Listing safe directories
	dirs, err := svc.GetSafeDirectories()
	if err != nil {
		t.Fatalf("GetSafeDirectories failed: %v", err)
	}
	found := false
	for _, d := range dirs {
		if d == repoPath {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s in dirs, got %v", repoPath, dirs)
	}

	// 5. Remove safe directory
	if err := svc.RemoveSafeDirectory(repoPath); err != nil {
		t.Fatalf("RemoveSafeDirectory failed: %v", err)
	}

	// 6. Check after remove
	isSafe, err = svc.CheckSafeDirectory(repoPath)
	if err != nil {
		t.Fatalf("CheckSafeDirectory after remove failed: %v", err)
	}
	if isSafe {
		t.Fatalf("expected isSafe to be false after RemoveSafeDirectory")
	}
}
