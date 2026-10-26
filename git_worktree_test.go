package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreePorcelainParser(t *testing.T) {
	raw := `worktree /Users/tester/project
HEAD a1b2c3d4e5f60718293a4b5c6d7e8f9012345678
branch refs/heads/main

worktree /Users/tester/project-locked-no-reason
HEAD b2c3d4e5f60718293a4b5c6d7e8f9012345678a1
branch refs/heads/feat/no-reason
locked

worktree /Users/tester/project-locked-reason
HEAD c3d4e5f60718293a4b5c6d7e8f9012345678a1b2
branch refs/heads/feat/with-reason
locked maintenance in progress

worktree /Users/tester/project-prunable-no-reason
HEAD d4e5f60718293a4b5c6d7e8f9012345678a1b2c3
branch refs/heads/dead/no-reason
prunable

worktree /Users/tester/project-prunable-reason
HEAD e5f60718293a4b5c6d7e8f9012345678a1b2c3d4
branch refs/heads/dead/with-reason
prunable gitdir file points to non-existent location

worktree /Users/tester/project-detached
HEAD f60718293a4b5c6d7e8f9012345678a1b2c3d4e5
detached

worktree /Users/tester/project-bare
bare
`

	worktrees := parseWorktreeListPorcelain(raw)
	if len(worktrees) != 7 {
		t.Fatalf("expected 7 worktrees, got %d", len(worktrees))
	}

	// 1. Main worktree
	wt0 := worktrees[0]
	if !wt0.IsMain {
		t.Errorf("wt0 should be main")
	}
	if wt0.Path != "/Users/tester/project" || wt0.Branch != "main" {
		t.Errorf("wt0 parsed incorrectly: %+v", wt0)
	}
	if wt0.IsLocked || wt0.IsPrunable || wt0.IsDetached {
		t.Errorf("wt0 unexpected flags: %+v", wt0)
	}

	// 2. Locked without reason
	wt1 := worktrees[1]
	if wt1.IsMain {
		t.Errorf("wt1 should not be main")
	}
	if !wt1.IsLocked {
		t.Errorf("wt1 should be locked")
	}
	if wt1.LockReason != "" {
		t.Errorf("wt1 lockReason should be empty, got: %q", wt1.LockReason)
	}
	if wt1.Branch != "feat/no-reason" {
		t.Errorf("wt1 branch mismatch: %s", wt1.Branch)
	}

	// 3. Locked with reason
	wt2 := worktrees[2]
	if !wt2.IsLocked {
		t.Errorf("wt2 should be locked")
	}
	if wt2.LockReason != "maintenance in progress" {
		t.Errorf("wt2 lockReason expected 'maintenance in progress', got: %q", wt2.LockReason)
	}

	// 4. Prunable without reason
	wt3 := worktrees[3]
	if !wt3.IsPrunable {
		t.Errorf("wt3 should be prunable")
	}
	if wt3.PruneReason != "" {
		t.Errorf("wt3 pruneReason should be empty, got: %q", wt3.PruneReason)
	}

	// 5. Prunable with reason
	wt4 := worktrees[4]
	if !wt4.IsPrunable {
		t.Errorf("wt4 should be prunable")
	}
	if wt4.PruneReason != "gitdir file points to non-existent location" {
		t.Errorf("wt4 pruneReason expected 'gitdir file points to non-existent location', got: %q", wt4.PruneReason)
	}

	// 6. Detached HEAD
	wt5 := worktrees[5]
	if !wt5.IsDetached {
		t.Errorf("wt5 should be detached")
	}
	if wt5.Branch != "" {
		t.Errorf("wt5 detached branch should be empty, got: %q", wt5.Branch)
	}

	// 7. Bare worktree
	wt6 := worktrees[6]
	if !wt6.IsBare {
		t.Errorf("wt6 should be bare")
	}
}

func TestWorktreeAdvancedLifecycleAndStatus(t *testing.T) {
	svc := GitService{}
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "main-repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(repoPath); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repoPath, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}

	// Create initial commit
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# Main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repoPath, "README.md"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Commit(repoPath, "chore: initial commit", false); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repoPath, "tag", "v1.0.0"); err != nil {
		t.Fatal(err)
	}

	// 1. Test parameter validation
	if err := svc.AddWorktreeAdvanced("", AddWorktreeAdvancedOptions{Path: "/tmp/wt"}); err == nil {
		t.Error("expected error for empty repoPath")
	}
	if err := svc.AddWorktreeAdvanced(repoPath, AddWorktreeAdvancedOptions{Path: ""}); err == nil {
		t.Error("expected error for empty worktree path")
	}
	if err := svc.AddWorktreeAdvanced(repoPath, AddWorktreeAdvancedOptions{Path: "/tmp/wt", NewBranch: true, Branch: ""}); err == nil {
		t.Error("expected error for empty branch name with NewBranch=true")
	}

	// 2. AddWorktreeAdvanced with new branch derived from BaseRef
	wt1Path := filepath.Join(tmpDir, "wt-feature")
	opts1 := AddWorktreeAdvancedOptions{
		Path:      wt1Path,
		Branch:    "feature/wt-adv",
		NewBranch: true,
		BaseRef:   "v1.0.0",
	}
	if err := svc.AddWorktreeAdvanced(repoPath, opts1); err != nil {
		t.Fatalf("AddWorktreeAdvanced failed: %v", err)
	}

	// Verify wt1 in GetWorktrees
	list, err := svc.GetWorktrees(repoPath)
	if err != nil {
		t.Fatalf("GetWorktrees failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(list))
	}
	if !list[0].IsMain {
		t.Errorf("first worktree must be main")
	}

	var foundWt1 bool
	for _, wt := range list {
		wtEval, _ := filepath.EvalSymlinks(wt.Path)
		wt1Eval, _ := filepath.EvalSymlinks(wt1Path)
		if wtEval == wt1Eval {
			foundWt1 = true
			if wt.Branch != "feature/wt-adv" {
				t.Errorf("expected branch feature/wt-adv, got %s", wt.Branch)
			}
			if wt.IsMain {
				t.Errorf("linked worktree should not be main")
			}
		}
	}
	if !foundWt1 {
		t.Fatalf("wt1 not found in worktree list")
	}

	// 3. GetWorktreeStatus initially clean
	st1, err := svc.GetWorktreeStatus(repoPath, wt1Path)
	if err != nil {
		t.Fatalf("GetWorktreeStatus failed: %v", err)
	}
	if st1.IsDirty || st1.DirtyCount != 0 {
		t.Errorf("expected clean status, got dirty=%v count=%d", st1.IsDirty, st1.DirtyCount)
	}
	if !strings.Contains(st1.Summary, "工作区整洁") {
		t.Errorf("expected clean summary, got %q", st1.Summary)
	}

	// Add dirty changes in wt1
	if err := os.WriteFile(filepath.Join(wt1Path, "dirty.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st1Dirty, err := svc.GetWorktreeStatus(repoPath, wt1Path)
	if err != nil {
		t.Fatalf("GetWorktreeStatus failed after dirty change: %v", err)
	}
	if !st1Dirty.IsDirty || st1Dirty.DirtyCount != 1 {
		t.Errorf("expected dirty status, got isDirty=%v count=%d", st1Dirty.IsDirty, st1Dirty.DirtyCount)
	}
	if !strings.Contains(st1Dirty.Summary, "1 处未提交") {
		t.Errorf("expected dirty summary to mention 1 change, got %q", st1Dirty.Summary)
	}

	// Clean up dirty change for move test
	_ = os.Remove(filepath.Join(wt1Path, "dirty.txt"))

	// 4. MoveWorktree
	wt1MovedPath := filepath.Join(tmpDir, "wt-feature-moved")
	if err := svc.MoveWorktree(repoPath, wt1Path, wt1MovedPath); err != nil {
		t.Fatalf("MoveWorktree failed: %v", err)
	}
	if _, err := os.Stat(wt1Path); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist after move: %s", wt1Path)
	}
	if _, err := os.Stat(wt1MovedPath); err != nil {
		t.Errorf("new worktree path should exist after move: %s (%v)", wt1MovedPath, err)
	}

	// Verify moved path appears in GetWorktrees
	listMoved, err := svc.GetWorktrees(repoPath)
	if err != nil {
		t.Fatalf("GetWorktrees after move failed: %v", err)
	}
	var foundMoved bool
	for _, wt := range listMoved {
		wtEval, _ := filepath.EvalSymlinks(wt.Path)
		targetEval, _ := filepath.EvalSymlinks(wt1MovedPath)
		if wtEval == targetEval {
			foundMoved = true
			if wt.Branch != "feature/wt-adv" {
				t.Errorf("expected branch feature/wt-adv after move, got %s", wt.Branch)
			}
		}
	}
	if !foundMoved {
		t.Fatalf("moved worktree not found in GetWorktrees")
	}

	// 5. AddWorktreeAdvanced with Detach
	wtDetachPath := filepath.Join(tmpDir, "wt-detached")
	optsDetach := AddWorktreeAdvancedOptions{
		Path:    wtDetachPath,
		Detach:  true,
		BaseRef: "v1.0.0",
	}
	if err := svc.AddWorktreeAdvanced(repoPath, optsDetach); err != nil {
		t.Fatalf("AddWorktreeAdvanced(detach) failed: %v", err)
	}

	listWithDetach, err := svc.GetWorktrees(repoPath)
	if err != nil {
		t.Fatalf("GetWorktrees with detach failed: %v", err)
	}
	var foundDetach bool
	for _, wt := range listWithDetach {
		wtEval, _ := filepath.EvalSymlinks(wt.Path)
		detachEval, _ := filepath.EvalSymlinks(wtDetachPath)
		if wtEval == detachEval {
			foundDetach = true
			if !wt.IsDetached {
				t.Errorf("expected wt to be detached: %+v", wt)
			}
		}
	}
	if !foundDetach {
		t.Fatalf("detached worktree not found in list")
	}

	// 6. Test LockWorktree and porcelain parsing with real git
	if err := svc.LockWorktree(repoPath, wtDetachPath, "testing lock"); err != nil {
		t.Fatalf("LockWorktree failed: %v", err)
	}
	listLocked, _ := svc.GetWorktrees(repoPath)
	for _, wt := range listLocked {
		wtEval, _ := filepath.EvalSymlinks(wt.Path)
		detachEval, _ := filepath.EvalSymlinks(wtDetachPath)
		if wtEval == detachEval {
			if !wt.IsLocked || wt.LockReason != "testing lock" {
				t.Errorf("expected locked with reason, got isLocked=%v reason=%q", wt.IsLocked, wt.LockReason)
			}
		}
	}

	// 7. PruneSingleWorktree on locked existing worktree
	if err := svc.PruneSingleWorktree(repoPath, wtDetachPath); err != nil {
		t.Fatalf("PruneSingleWorktree failed on locked worktree: %v", err)
	}
	if _, err := os.Stat(wtDetachPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir should have been removed: %s", wtDetachPath)
	}

	// 8. PruneSingleWorktree on missing/orphaned worktree
	// Manually delete moved worktree directory to simulate orphaned prunable worktree
	if err := os.RemoveAll(wt1MovedPath); err != nil {
		t.Fatal(err)
	}
	// Verify git reports it as prunable
	listPrunable, err := svc.GetWorktrees(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range listPrunable {
		wtEval, _ := filepath.EvalSymlinks(wt.Path)
		movedEval, _ := filepath.EvalSymlinks(wt1MovedPath)
		if wtEval == movedEval {
			if !wt.IsPrunable {
				t.Errorf("deleted worktree should be recognized as isPrunable: %+v", wt)
			}
		}
	}

	// Now prune it
	if err := svc.PruneSingleWorktree(repoPath, wt1MovedPath); err != nil {
		t.Fatalf("PruneSingleWorktree on missing worktree failed: %v", err)
	}

	// Verify only main worktree remains
	finalList, err := svc.GetWorktrees(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(finalList) != 1 {
		t.Fatalf("expected 1 worktree remaining, got %d: %+v", len(finalList), finalList)
	}

	// 9. PruneSingleWorktree cannot prune main worktree
	if err := svc.PruneSingleWorktree(repoPath, repoPath); err == nil {
		t.Error("expected error when trying to prune main worktree")
	}
}
