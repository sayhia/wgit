package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConflictResolutionInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, "conflict.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) {
		t.Helper()
		if err := svc.StageFile(repo, "conflict.txt"); err != nil {
			t.Fatalf("StageFile failed: %v", err)
		}
		if err := svc.Commit(repo, message, false); err != nil {
			t.Fatalf("Commit(%s) failed: %v", message, err)
		}
	}
	startMerge := func() {
		t.Helper()
		_, _ = ExecGit(repo, "merge", "--abort")
		if out, err := ExecGit(repo, "merge", "side"); err == nil {
			log, _ := ExecGit(repo, "log", "--oneline", "--all", "--graph", "--decorate")
			content, _ := os.ReadFile(filepath.Join(repo, "conflict.txt"))
			t.Fatalf("expected merge conflict, merge output=%q file=%q log=%s", out, content, log)
		}
		status, err := svc.GetRepoStatus(repo)
		if err != nil || len(status.ConflictedFiles) != 1 || status.ConflictedFiles[0] != "conflict.txt" {
			t.Fatalf("expected conflict.txt in conflicted files, got %+v (err=%v)", status, err)
		}
	}

	write("base\n")
	commit("chore: base")
	baseBranch, _ := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	baseBranch = strings.TrimSpace(baseBranch)
	if err := svc.CreateBranch(repo, "side", ""); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := svc.CheckoutBranch(repo, "side", false); err != nil {
		t.Fatalf("checkout side failed: %v", err)
	}
	write("side\n")
	commit("feat: side")
	if err := svc.CheckoutBranch(repo, baseBranch, false); err != nil {
		t.Fatalf("checkout base failed: %v", err)
	}
	write("main\n")
	commit("feat: main")

	startMerge()
	content, err := svc.GetConflictFileContent(repo, "conflict.txt")
	if err != nil || !strings.Contains(content, "<<<<<<<") || !strings.Contains(content, "side") {
		t.Fatalf("expected conflict markers in file content, got %q err=%v", content, err)
	}
	if _, err := svc.GetConflictFileContent(repo, "missing.txt"); err == nil {
		t.Fatal("expected error for missing conflict file")
	}
	if err := svc.ResolveConflict(repo, "conflict.txt", "bogus"); err == nil {
		t.Fatal("expected error for unknown resolution")
	}
	if err := svc.ResolveConflict(repo, "conflict.txt", "ours"); err != nil {
		t.Fatalf("ResolveConflict(ours) failed: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "conflict.txt")); strings.TrimSpace(string(data)) != "main" {
		t.Fatalf("ours resolution should keep main content, got %q", data)
	}
	if status, _ := svc.GetRepoStatus(repo); len(status.ConflictedFiles) != 0 {
		t.Fatalf("ours resolution left conflicts: %+v", status.ConflictedFiles)
	}

	startMerge()
	if err := svc.ResolveConflict(repo, "conflict.txt", "theirs"); err != nil {
		t.Fatalf("ResolveConflict(theirs) failed: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "conflict.txt")); strings.TrimSpace(string(data)) != "side" {
		t.Fatalf("theirs resolution should keep side content, got %q", data)
	}

	startMerge()
	if err := os.Chmod(repo, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo, 0o755) })
	if err := svc.ResolveConflict(repo, "conflict.txt", "backup_and_theirs"); err == nil {
		t.Fatal("failed backup must not be followed by overwriting the local file")
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "conflict.txt")); !strings.Contains(string(data), "<<<<<<<") {
		t.Fatalf("failed backup must leave the conflicted file untouched, got %q", data)
	}
	if err := os.Chmod(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResolveConflict(repo, "conflict.txt", "backup_and_theirs"); err != nil {
		t.Fatalf("ResolveConflict(backup_and_theirs) failed: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "conflict.txt")); strings.TrimSpace(string(data)) != "side" {
		t.Fatalf("backup resolution should keep theirs content, got %q", data)
	}
	backups, _ := filepath.Glob(filepath.Join(repo, "conflict.backup_*.txt"))
	if len(backups) != 1 {
		t.Fatalf("expected one conflict backup, got %+v", backups)
	}
	if data, _ := os.ReadFile(backups[0]); !strings.Contains(string(data), "<<<<<<<") {
		t.Fatalf("backup should preserve conflict markers, got %q", data)
	}
	_ = os.Remove(backups[0])

	startMerge()
	if err := svc.ResolveConflict(repo, "conflict.txt", "mark"); err != nil {
		t.Fatalf("ResolveConflict(mark) failed: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "conflict.txt")); !strings.Contains(string(data), "<<<<<<<") {
		t.Fatalf("mark resolution should keep markers for manual edit, got %q", data)
	}
	if status, _ := svc.GetRepoStatus(repo); len(status.ConflictedFiles) != 0 {
		t.Fatalf("mark resolution left conflicts: %+v", status.ConflictedFiles)
	}
}

func TestHunkStagingAndWorkspaceCleanupInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	cfg, err := svc.GetGitConfig(repo)
	if err != nil || cfg.UserName != "Tester" || cfg.UserEmail != "tester@example.com" {
		t.Fatalf("GetGitConfig failed: %+v err=%v", cfg, err)
	}

	hunkPath := filepath.Join(repo, "hunk.txt")
	if err := os.WriteFile(hunkPath, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "hunk.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: hunk base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := os.WriteFile(hunkPath, []byte("a\nB\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "diff --git a/hunk.txt b/hunk.txt\n--- a/hunk.txt\n+++ b/hunk.txt\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"

	if err := svc.StageHunk(repo, patch); err != nil {
		t.Fatalf("StageHunk failed: %v", err)
	}
	status, _ := svc.GetRepoStatus(repo)
	if len(status.StagedFiles) != 1 || status.StagedFiles[0].Path != "hunk.txt" {
		t.Fatalf("expected hunk.txt staged, got %+v", status.StagedFiles)
	}
	if len(status.UnstagedFiles) != 0 {
		t.Fatalf("expected no unstaged files after staging hunk, got %+v", status.UnstagedFiles)
	}
	if err := svc.UnstageAll(repo); err != nil {
		t.Fatalf("UnstageAll failed: %v", err)
	}
	if status, _ := svc.GetRepoStatus(repo); len(status.StagedFiles) != 0 {
		t.Fatalf("UnstageAll left staged files: %+v", status.StagedFiles)
	}
	if err := svc.DiscardHunk(repo, patch); err != nil {
		t.Fatalf("DiscardHunk failed: %v", err)
	}
	if data, _ := os.ReadFile(hunkPath); string(data) != "a\nb\nc\n" {
		t.Fatalf("DiscardHunk did not restore base content: %q", data)
	}

	if err := os.WriteFile(hunkPath, []byte("a\nB\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageLines(repo, patch); err != nil {
		t.Fatalf("StageLines failed: %v", err)
	}
	if status, _ := svc.GetRepoStatus(repo); len(status.StagedFiles) != 1 {
		t.Fatalf("StageLines did not stage the line change: %+v", status.StagedFiles)
	}
	if err := svc.UnstageAll(repo); err != nil {
		t.Fatalf("UnstageAll after StageLines failed: %v", err)
	}
	if err := svc.DiscardLines(repo, patch); err != nil {
		t.Fatalf("DiscardLines failed: %v", err)
	}
	if data, _ := os.ReadFile(hunkPath); string(data) != "a\nb\nc\n" {
		t.Fatalf("DiscardLines did not restore base content: %q", data)
	}

	if err := os.WriteFile(filepath.Join(repo, "junk.txt"), []byte("junk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := svc.CleanWorkingTree(repo, true)
	if err != nil || len(files) != 1 || files[0] != "junk.txt" {
		t.Fatalf("CleanWorkingTree dry run should list junk.txt, got %+v err=%v", files, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "junk.txt")); err != nil {
		t.Fatalf("dry run must not delete files: %v", err)
	}
	files, err = svc.CleanWorkingTree(repo, false)
	if err != nil || len(files) != 1 || files[0] != "junk.txt" {
		t.Fatalf("CleanWorkingTree should remove junk.txt, got %+v err=%v", files, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "junk.txt")); !os.IsNotExist(err) {
		t.Fatalf("CleanWorkingTree did not delete junk.txt: %v", err)
	}

	if err := svc.AddToGitignore(repo, "*.log"); err != nil {
		t.Fatalf("AddToGitignore failed: %v", err)
	}
	if err := svc.AddToGitignore(repo, "build/"); err != nil {
		t.Fatalf("AddToGitignore second pattern failed: %v", err)
	}
	ignore, _ := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if string(ignore) != "*.log\nbuild/\n" {
		t.Fatalf("unexpected .gitignore content: %q", ignore)
	}

	if err := os.WriteFile(hunkPath, []byte("a\nchanged\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.DiscardFile(repo, "hunk.txt"); err != nil {
		t.Fatalf("DiscardFile tracked failed: %v", err)
	}
	if data, _ := os.ReadFile(hunkPath); string(data) != "a\nb\nc\n" {
		t.Fatalf("DiscardFile did not restore tracked file: %q", data)
	}
	untrackedPath := filepath.Join(repo, "untracked.txt")
	if err := os.WriteFile(untrackedPath, []byte("tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.DiscardFile(repo, "untracked.txt"); err != nil {
		t.Fatalf("DiscardFile untracked failed: %v", err)
	}
	if _, err := os.Stat(untrackedPath); !os.IsNotExist(err) {
		t.Fatalf("DiscardFile did not delete untracked file: %v", err)
	}
}

func TestResolveConflictBlockInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	filePath := filepath.Join(repo, "multi.txt")
	conflictContent := `head_clean
<<<<<<< HEAD
line_ours_1
=======
line_theirs_1
>>>>>>> feature
middle_clean
<<<<<<< HEAD
line_ours_2
=======
line_theirs_2
>>>>>>> feature
tail_clean
`
	if err := os.WriteFile(filePath, []byte(conflictContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Resolve block 0 using "ours"
	res1, err := svc.ResolveConflictBlock(repo, "multi.txt", 0, "ours")
	if err != nil {
		t.Fatalf("ResolveConflictBlock(0, ours) failed: %v", err)
	}
	if !res1.Success || res1.RemainingConflicts != 1 || res1.FileAutoStaged {
		t.Fatalf("unexpected res1: %+v", res1)
	}

	data1, _ := os.ReadFile(filePath)
	str1 := string(data1)
	if !strings.Contains(str1, "line_ours_1") || strings.Contains(str1, "line_theirs_1") {
		t.Fatalf("block 0 was not resolved with ours: %s", str1)
	}
	if !strings.Contains(str1, "<<<<<<<") {
		t.Fatalf("block 1 should still have conflict markers: %s", str1)
	}

	// Resolve remaining block (now at index 0) using "both"
	res2, err := svc.ResolveConflictBlock(repo, "multi.txt", 0, "both")
	if err != nil {
		t.Fatalf("ResolveConflictBlock(0, both) failed: %v", err)
	}
	if !res2.Success || res2.RemainingConflicts != 0 || !res2.FileAutoStaged {
		t.Fatalf("unexpected res2: %+v", res2)
	}

	data2, _ := os.ReadFile(filePath)
	str2 := string(data2)
	if strings.Contains(str2, "<<<<<<<") || strings.Contains(str2, "=======") || strings.Contains(str2, ">>>>>>>") {
		t.Fatalf("all conflict markers should be gone: %s", str2)
	}
	if !strings.Contains(str2, "line_ours_2") || !strings.Contains(str2, "line_theirs_2") {
		t.Fatalf("both lines should be preserved: %s", str2)
	}
}

func TestAutoResolveMetaConflicts(t *testing.T) {
	repo := initClosedLoopRepo(t)
	svc := GitService{}

	// 1. Invalid strategy
	if _, err := svc.AutoResolveMetaConflicts(repo, "invalid_strategy"); err == nil {
		t.Fatal("expected error for unsupported strategy")
	}

	metaFile := "Assets/Hero.prefab.meta"
	jsonFile := "config.json"
	fullMeta := filepath.Join(repo, metaFile)
	fullJSON := filepath.Join(repo, jsonFile)
	_ = os.MkdirAll(filepath.Dir(fullMeta), 0755)

	_ = os.WriteFile(fullMeta, []byte("guid: 11111111\n"), 0644)
	_ = os.WriteFile(fullJSON, []byte("{\"version\": 1}\n"), 0644)
	_, _ = ExecGit(repo, "add", "-A")
	_, _ = ExecGit(repo, "commit", "-m", "init")

	baseBranch, _ := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	baseBranch = strings.TrimSpace(baseBranch)
	_ = svc.CreateBranch(repo, "feature", "")
	_ = svc.CheckoutBranch(repo, "feature", false)

	_ = os.WriteFile(fullMeta, []byte("guid: 22222222\n"), 0644)
	_ = os.WriteFile(fullJSON, []byte("{\"version\": 2}\n"), 0644)
	_, _ = ExecGit(repo, "add", "-A")
	_, _ = ExecGit(repo, "commit", "-m", "feature update")

	_ = svc.CheckoutBranch(repo, baseBranch, false)
	_ = os.WriteFile(fullMeta, []byte("guid: 33333333\n"), 0644)
	_ = os.WriteFile(fullJSON, []byte("{\"version\": 3}\n"), 0644)
	_, _ = ExecGit(repo, "add", "-A")
	_, _ = ExecGit(repo, "commit", "-m", "main update")

	// Trigger merge conflict
	_, _ = ExecGit(repo, "merge", "feature")

	status, err := svc.GetRepoStatus(repo)
	if err != nil || len(status.ConflictedFiles) != 2 {
		t.Fatalf("expected 2 conflicted files, got %+v (err=%v)", status, err)
	}

	// Resolve all meta/config conflicts with "theirs"
	res, err := svc.AutoResolveMetaConflicts(repo, "theirs")
	if err != nil {
		t.Fatalf("AutoResolveMetaConflicts failed: %v", err)
	}
	if res.Count != 2 || len(res.ResolvedFiles) != 2 {
		t.Fatalf("expected 2 resolved files, got %+v", res)
	}

	statusAfter, _ := svc.GetRepoStatus(repo)
	if len(statusAfter.ConflictedFiles) != 0 {
		t.Fatalf("all conflicts should be resolved, remaining: %+v", statusAfter.ConflictedFiles)
	}

	// Verify content is "theirs"
	metaData, _ := os.ReadFile(fullMeta)
	if !strings.Contains(string(metaData), "22222222") {
		t.Fatalf("expected theirs version 22222222, got %s", metaData)
	}
}

func TestResolveConflictWithCustomContent(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	target := filepath.Join(repo, "conflict.txt")
	conflictBody := "<<<<<<< HEAD\nline from ours\n=======\nline from theirs\n>>>>>>> feat\n"
	if err := os.WriteFile(target, []byte(conflictBody), 0644); err != nil {
		t.Fatal(err)
	}

	// Resolve with custom merged content without markers -> should auto-stage
	resolvedCustom := "line from ours\nline from theirs\n"
	res, err := svc.ResolveConflictWithCustomContent(repo, "conflict.txt", resolvedCustom)
	if err != nil {
		t.Fatalf("ResolveConflictWithCustomContent failed: %v", err)
	}
	if !res.Success || !res.FileAutoStaged || res.RemainingConflicts != 0 {
		t.Fatalf("unexpected resolve result: %+v", res)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != resolvedCustom {
		t.Fatalf("expected file content %q, got %q", resolvedCustom, string(data))
	}
}
