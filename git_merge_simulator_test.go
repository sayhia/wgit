package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSimulateMergeInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-merge-sim-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
	}

	runCmd("init", "-b", "main")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")

	// 1. Initial commit
	sharedFile := filepath.Join(tempDir, "shared.txt")
	if err := os.WriteFile(sharedFile, []byte("initial shared content\n"), 0644); err != nil {
		t.Fatalf("failed to write shared file: %v", err)
	}
	runCmd("add", ".")
	runCmd("commit", "-m", "initial commit")

	// 2. Create feature branch with clean changes
	runCmd("checkout", "-b", "feature-clean")
	cleanFile := filepath.Join(tempDir, "clean.txt")
	if err := os.WriteFile(cleanFile, []byte("clean file content\n"), 0644); err != nil {
		t.Fatalf("failed to write clean file: %v", err)
	}
	runCmd("add", ".")
	runCmd("commit", "-m", "feature clean commit")

	// 3. Switch back to main and test Fast-Forward simulation
	runCmd("checkout", "main")

	svc := &GitService{}
	simFF, err := svc.SimulateMerge(tempDir, "main", "feature-clean")
	if err != nil {
		t.Fatalf("SimulateMerge FF failed: %v", err)
	}
	if !simFF.CanMergeCleanly || !simFF.IsFastForward || simFF.HasConflicts {
		t.Fatalf("expected fast-forward clean merge, got: %+v", simFF)
	}
	if len(simFF.ChangedFiles) == 0 || simFF.ChangedFiles[0] != "clean.txt" {
		t.Fatalf("expected changedFiles to contain clean.txt, got: %v", simFF.ChangedFiles)
	}

	// 4. Commit on main to force a 3-way merge
	mainOnlyFile := filepath.Join(tempDir, "main_only.txt")
	if err := os.WriteFile(mainOnlyFile, []byte("main only\n"), 0644); err != nil {
		t.Fatalf("failed to write main only file: %v", err)
	}
	runCmd("add", ".")
	runCmd("commit", "-m", "main branch commit")

	// 5. Test 3-way clean merge simulation
	sim3Way, err := svc.SimulateMerge(tempDir, "main", "feature-clean")
	if err != nil {
		t.Fatalf("SimulateMerge 3-way failed: %v", err)
	}
	if !sim3Way.CanMergeCleanly || sim3Way.HasConflicts || sim3Way.TreeHash == "" {
		t.Fatalf("expected clean 3-way merge with treeHash, got: %+v", sim3Way)
	}

	// 6. Create conflicting branch
	runCmd("checkout", "-b", "feature-conflict")
	if err := os.WriteFile(sharedFile, []byte("conflict from feature\n"), 0644); err != nil {
		t.Fatalf("failed to write shared file on feature: %v", err)
	}
	runCmd("commit", "-am", "feature conflict commit")

	runCmd("checkout", "main")
	if err := os.WriteFile(sharedFile, []byte("conflict from main\n"), 0644); err != nil {
		t.Fatalf("failed to write shared file on main: %v", err)
	}
	runCmd("commit", "-am", "main conflict commit")

	// 7. Test Conflicted simulation
	simConflict, err := svc.SimulateMerge(tempDir, "main", "feature-conflict")
	if err != nil {
		t.Fatalf("SimulateMerge conflict failed: %v", err)
	}
	if simConflict.CanMergeCleanly || !simConflict.HasConflicts {
		t.Fatalf("expected conflicted merge, got: %+v", simConflict)
	}
	if len(simConflict.ConflictedFiles) == 0 {
		t.Fatalf("expected at least 1 conflicted file, got none")
	}
	t.Logf("Conflicted files detected: %v", simConflict.ConflictedFiles)
}
