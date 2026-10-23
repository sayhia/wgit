package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBatchAndStashRecoveryWorkflowsInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	files := map[string]string{"a.txt": "a\n", "b.txt": "b\n"}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.StageAll(repo); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	baseHash, _ := ExecGit(repo, "rev-parse", "HEAD")
	baseHash = strings.TrimSpace(baseHash)
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil {
			t.Fatalf("read %s failed: %v", name, err)
		}
		return string(data)
	}
	resetClean := func() {
		t.Helper()
		if _, err := ExecGit(repo, "reset", "--hard", "HEAD"); err != nil {
			t.Fatalf("reset --hard failed: %v", err)
		}
		if _, err := svc.CleanWorkingTree(repo, false); err != nil {
			t.Fatalf("clean failed: %v", err)
		}
	}

	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StashSave(repo, "multi", false, false); err != nil {
		t.Fatalf("StashSave failed: %v", err)
	}
	if stashes, _ := svc.GetStashes(repo); len(stashes) != 1 {
		t.Fatalf("expected one stash, got %+v", stashes)
	}
	if err := svc.ApplyFileFromStash(repo, "stash@{0}", "b.txt"); err != nil {
		t.Fatalf("ApplyFileFromStash failed: %v", err)
	}
	if read("b.txt") != "b2\n" {
		t.Fatalf("expected b.txt restored from stash, got %q", read("b.txt"))
	}
	if read("a.txt") != "a\n" {
		t.Fatalf("ApplyFileFromStash should not restore a.txt, got %q", read("a.txt"))
	}
	if stashes, _ := svc.GetStashes(repo); len(stashes) != 1 {
		t.Fatalf("ApplyFileFromStash should keep the stash, got %+v", stashes)
	}
	if err := svc.StashPop(repo, 0); err != nil {
		t.Fatalf("StashPop after file apply failed: %v", err)
	}
	resetClean()

	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a-staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "a.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b-unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StashStagedOnly(repo, "staged only"); err != nil {
		t.Fatalf("StashStagedOnly failed: %v", err)
	}
	if stashes, _ := svc.GetStashes(repo); len(stashes) != 1 || !strings.Contains(stashes[0].Message, "staged only") {
		t.Fatalf("expected staged-only stash, got %+v", stashes)
	}
	if read("a.txt") != "a\n" {
		t.Fatalf("staged change should be stashed, got %q", read("a.txt"))
	}
	if read("b.txt") != "b-unstaged\n" {
		t.Fatalf("unstaged change should remain, got %q", read("b.txt"))
	}
	if err := svc.StashPop(repo, 0); err != nil {
		t.Fatalf("StashPop after staged-only failed: %v", err)
	}
	resetClean()

	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a-modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b-modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.BatchDiscardFiles(repo, []string{"a.txt", "b.txt", "untracked.txt"}); err != nil {
		t.Fatalf("BatchDiscardFiles failed: %v", err)
	}
	if read("a.txt") != "a\n" || read("b.txt") != "b\n" {
		t.Fatalf("batch discard did not restore tracked files: a=%q b=%q", read("a.txt"), read("b.txt"))
	}
	if _, err := os.Stat(filepath.Join(repo, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatalf("batch discard did not remove untracked file: %v", err)
	}
	if err := svc.BatchDiscardFiles(repo, nil); err != nil {
		t.Fatalf("BatchDiscardFiles(nil) failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a-revert\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "a.txt"); err != nil {
		t.Fatalf("StageFile before revert failed: %v", err)
	}
	if err := svc.Commit(repo, "feat: revert me", false); err != nil {
		t.Fatalf("Commit before revert failed: %v", err)
	}
	revertHash, _ := ExecGit(repo, "rev-parse", "HEAD")
	revertHash = strings.TrimSpace(revertHash)
	if _, err := svc.RevertNoCommit(repo, revertHash); err != nil {
		t.Fatalf("RevertNoCommit failed: %v", err)
	}
	if status, _ := svc.GetRepoStatus(repo); len(status.StagedFiles) == 0 {
		t.Fatalf("RevertNoCommit should leave staged changes, got %+v", status)
	}
	if read("a.txt") != "a\n" {
		t.Fatalf("RevertNoCommit should restore previous content, got %q", read("a.txt"))
	}
	if _, err := svc.RevertNoCommit(repo, ""); err == nil {
		t.Fatal("expected error for empty revert hash")
	}
	if _, err := ExecGit(repo, "reset", "--hard", baseHash); err != nil {
		t.Fatalf("reset after revert failed: %v", err)
	}

	for _, tag := range []string{"t1", "t2"} {
		if err := svc.CreateTag(repo, tag, "", ""); err != nil {
			t.Fatalf("CreateTag(%s) failed: %v", tag, err)
		}
	}
	deleted, err := svc.DeleteBatchTags(repo, []string{"t1", "t2"}, false, "")
	if err != nil || len(deleted) != 2 {
		t.Fatalf("DeleteBatchTags failed: %+v err=%v", deleted, err)
	}
	if tags, _ := svc.GetTags(repo); len(tags) != 0 {
		t.Fatalf("expected no tags after batch delete, got %+v", tags)
	}
	if deleted, err := svc.DeleteBatchTags(repo, nil, false, ""); err != nil || len(deleted) != 0 {
		t.Fatalf("empty DeleteBatchTags should succeed, got %+v err=%v", deleted, err)
	}

	if err := svc.CreateTag(repo, "keep-me", "", ""); err != nil {
		t.Fatalf("CreateTag(keep-me) failed: %v", err)
	}
	deleted, err = svc.DeleteBatchTags(repo, []string{"keep-me", "missing-tag", "bad..tag"}, false, "")
	if len(deleted) != 1 || deleted[0] != "keep-me" {
		t.Fatalf("partial batch delete should report only removed tags: %+v", deleted)
	}
	if err == nil || !strings.Contains(err.Error(), "missing-tag") || !strings.Contains(err.Error(), "bad..tag") {
		t.Fatalf("partial batch delete must surface failures: %v", err)
	}
	if tags, _ := svc.GetTags(repo); len(tags) != 0 {
		t.Fatalf("successfully deleted tag should be gone, got %+v", tags)
	}

	if err := svc.CreateTag(repo, "keep-safe", "", ""); err != nil {
		t.Fatalf("CreateTag(keep-safe) failed: %v", err)
	}
	if err := svc.DeleteTag(repo, "--all"); err == nil {
		t.Fatal("option-like tag name must be treated as a literal tag, not a flag")
	}
	if deleted, err := svc.DeleteBatchTags(repo, []string{"--all"}, false, ""); err == nil || len(deleted) != 0 {
		t.Fatalf("batch delete must not accept option-like tag names: %+v err=%v", deleted, err)
	}
	if tags, _ := svc.GetTags(repo); len(tags) != 1 || tags[0].Name != "keep-safe" {
		t.Fatalf("option-like deletes must not remove real tags: %+v", tags)
	}
}
