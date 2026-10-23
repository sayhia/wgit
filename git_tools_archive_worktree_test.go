package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveAndPatchWorkflowsInTempRepo(t *testing.T) {
	svc := GitService{}
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(name, message string) string {
		t.Helper()
		if err := svc.StageFile(repo, name); err != nil {
			t.Fatalf("StageFile(%s) failed: %v", name, err)
		}
		if err := svc.Commit(repo, message, false); err != nil {
			t.Fatalf("Commit(%s) failed: %v", message, err)
		}
		hash, err := ExecGit(repo, "rev-parse", "HEAD")
		if err != nil {
			t.Fatalf("rev-parse HEAD failed: %v", err)
		}
		return strings.TrimSpace(hash)
	}

	write("first.txt", "first\n")
	firstHash := commit("first.txt", "feat: first")
	write("second.txt", "second\n")
	secondHash := commit("second.txt", "feat: second")

	archivePath := filepath.Join(parent, "repo.zip")
	if err := svc.ExportArchive(repo, "HEAD", archivePath); err != nil {
		t.Fatalf("ExportArchive failed: %v", err)
	}
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open exported archive failed: %v", err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	_ = zr.Close()
	if !names["first.txt"] || !names["second.txt"] {
		t.Fatalf("archive should contain both files, got %+v", names)
	}
	if err := svc.ExportArchive(repo, "does-not-exist", filepath.Join(parent, "bad.zip")); err == nil {
		t.Fatal("expected error exporting an unknown ref")
	}
	if err := svc.ExportArchive(repo, "HEAD", ""); err == nil {
		t.Fatal("expected error for empty archive output path")
	}

	patchPath := filepath.Join(parent, "second.patch")
	if err := svc.ExportPatch(repo, secondHash, patchPath); err != nil {
		t.Fatalf("ExportPatch failed: %v", err)
	}
	patchData, err := os.ReadFile(patchPath)
	if err != nil || !strings.Contains(string(patchData), "feat: second") || !strings.Contains(string(patchData), "second.txt") {
		t.Fatalf("unexpected patch content: %q err=%v", patchData, err)
	}
	if err := svc.ExportPatch(repo, "", patchPath); err == nil {
		t.Fatal("expected error for empty commit hash")
	}

	clonePath := filepath.Join(parent, "clone")
	if _, err := ExecGit(parent, "clone", repo, clonePath); err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	if _, err := ExecGit(clonePath, "reset", "--hard", firstHash); err != nil {
		t.Fatalf("reset clone failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(clonePath, "second.txt")); !os.IsNotExist(err) {
		t.Fatalf("clone should be at first commit before applying patch: %v", err)
	}
	if err := svc.ApplyPatch(clonePath, patchPath); err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(clonePath, "second.txt")); err != nil || string(data) != "second\n" {
		t.Fatalf("ApplyPatch did not create second.txt: %q err=%v", data, err)
	}
	if status, _ := svc.GetRepoStatus(clonePath); status.IsClean || len(status.UntrackedFiles) == 0 {
		t.Fatalf("expected applied patch to leave a new untracked file, got %+v", status)
	}
	if err := svc.ApplyPatch(clonePath, ""); err == nil {
		t.Fatal("expected error for empty patch path")
	}
}

func TestWorktreeLifecycleAndMaintenanceInTempRepo(t *testing.T) {
	svc := GitService{}
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "base.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := svc.CreateBranch(repo, "wt-branch", ""); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	wtPath := filepath.Join(parent, "wt")
	if err := svc.AddWorktree(repo, wtPath, "wt-branch", false); err != nil {
		t.Fatalf("AddWorktree failed: %v", err)
	}
	worktrees, err := svc.GetWorktrees(repo)
	if err != nil || len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %+v err=%v", worktrees, err)
	}
	if !worktrees[0].IsMain || worktrees[1].IsMain {
		t.Fatalf("expected only the first worktree to be main: %+v", worktrees)
	}
	var found bool
	for _, wt := range worktrees {
		got, err := filepath.EvalSymlinks(wt.Path)
		if err != nil {
			continue
		}
		want, _ := filepath.EvalSymlinks(wtPath)
		if got == want && wt.Branch == "wt-branch" && !wt.IsBare && !wt.IsMain {
			found = true
		}
	}
	if !found {
		t.Fatalf("linked worktree not listed correctly: %+v", worktrees)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "wt.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(wtPath, "add", "wt.txt"); err != nil {
		t.Fatalf("stage in worktree failed: %v", err)
	}
	if _, err := ExecGit(wtPath, "commit", "-m", "feat: worktree change"); err != nil {
		t.Fatalf("commit in worktree failed: %v", err)
	}
	if err := svc.RemoveWorktree(repo, wtPath, false); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree directory should be removed: %v", err)
	}
	if worktrees, _ := svc.GetWorktrees(repo); len(worktrees) != 1 {
		t.Fatalf("expected only the main worktree, got %+v", worktrees)
	}

	wtNew := filepath.Join(parent, "wt-new")
	if err := svc.AddWorktree(repo, wtNew, "wt-new", true); err != nil {
		t.Fatalf("AddWorktree new branch failed: %v", err)
	}
	branches, _ := svc.GetBranches(repo)
	var newBranch bool
	for _, b := range branches {
		if b.Name == "wt-new" {
			newBranch = true
		}
	}
	if !newBranch {
		t.Fatalf("expected wt-new branch, got %+v", branches)
	}
	if err := svc.RemoveWorktree(repo, wtNew, true); err != nil {
		t.Fatalf("RemoveWorktree(force) failed: %v", err)
	}
	if err := svc.AddWorktree(repo, "", "wt-branch", false); err == nil {
		t.Fatal("expected error for empty worktree path")
	}
	if err := svc.RemoveWorktree(repo, "", false); err == nil {
		t.Fatal("expected error removing empty worktree path")
	}

	stats, err := svc.RunMaintenance(repo)
	if err != nil || stats == nil || stats.GitDirSize == "" || stats.PackFilesCount < 0 || stats.LooseObjects < 0 {
		t.Fatalf("RunMaintenance returned invalid stats: %+v err=%v", stats, err)
	}
	if !stats.Success || stats.RemotePruneStatus != "SKIPPED" || stats.Warnings == nil || stats.Summary == "" {
		t.Fatalf("maintenance without a remote should succeed and report the skip: %+v", stats)
	}
}

func TestSubmoduleLifecycleInTempRepo(t *testing.T) {
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	svc := GitService{}
	parent := t.TempDir()

	sub := filepath.Join(parent, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(sub); err != nil {
		t.Fatalf("InitRepo(sub) failed: %v", err)
	}
	subBranch, _ := ExecGit(sub, "symbolic-ref", "--short", "HEAD")
	subBranch = strings.TrimSpace(subBranch)
	if err := svc.SetGitConfig(sub, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig(sub) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "sub.txt"), []byte("sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(sub, "sub.txt"); err != nil {
		t.Fatalf("StageFile(sub) failed: %v", err)
	}
	if err := svc.Commit(sub, "feat: sub", false); err != nil {
		t.Fatalf("Commit(sub) failed: %v", err)
	}

	super := filepath.Join(parent, "super")
	if err := os.MkdirAll(super, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(super); err != nil {
		t.Fatalf("InitRepo(super) failed: %v", err)
	}
	if err := svc.SetGitConfig(super, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig(super) failed: %v", err)
	}
	if _, err := ExecGit(super, "config", "protocol.file.allow", "always"); err != nil {
		t.Fatalf("allow file protocol failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(super, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(super, "base.txt"); err != nil {
		t.Fatalf("StageFile(super) failed: %v", err)
	}
	if err := svc.Commit(super, "chore: super", false); err != nil {
		t.Fatalf("Commit(super) failed: %v", err)
	}

	if err := svc.AddSubmodule(super, sub, "vendor/sub"); err != nil {
		t.Fatalf("AddSubmodule failed: %v", err)
	}
	submodules, err := svc.GetSubmodules(super)
	if err != nil || len(submodules) != 1 {
		t.Fatalf("expected one submodule, got %+v err=%v", submodules, err)
	}
	sm := submodules[0]
	if sm.Path != "vendor/sub" || sm.Commit == "" || sm.Status != "synced" || !strings.Contains(sm.URL, "sub") {
		t.Fatalf("unexpected submodule info: %+v", sm)
	}
	if _, err := svc.UpdateSubmodules(super, true, true); err != nil {
		t.Fatalf("UpdateSubmodules failed: %v", err)
	}
	if err := svc.SetSubmoduleBranch(super, "vendor/sub", subBranch); err != nil {
		t.Fatalf("SetSubmoduleBranch failed: %v", err)
	}
	gitmodules, err := os.ReadFile(filepath.Join(super, ".gitmodules"))
	if err != nil || !strings.Contains(string(gitmodules), "branch = "+subBranch) {
		t.Fatalf("expected .gitmodules branch entry, got %q err=%v", gitmodules, err)
	}
	if err := svc.SetSubmoduleBranch(super, "", "main"); err == nil {
		t.Fatal("expected error for empty submodule path")
	}
	if out, err := svc.SyncSubmodule(super, "vendor/sub"); err != nil || strings.TrimSpace(out) == "" {
		t.Fatalf("SyncSubmodule failed: %q err=%v", out, err)
	}
	if _, err := svc.UpdateSubmodulesRecursive(super); err != nil {
		t.Fatalf("UpdateSubmodulesRecursive failed: %v", err)
	}
	if _, err := svc.UpdateSubmoduleRemote(super, "vendor/sub"); err != nil {
		t.Fatalf("UpdateSubmoduleRemote failed: %v", err)
	}
	if err := svc.RemoveSubmodule(super, "vendor/sub"); err != nil {
		t.Fatalf("RemoveSubmodule failed: %v", err)
	}
	if submodules, _ := svc.GetSubmodules(super); len(submodules) != 0 {
		t.Fatalf("expected no submodules after removal, got %+v", submodules)
	}
	if _, err := os.Stat(filepath.Join(super, ".gitmodules")); !os.IsNotExist(err) {
		t.Fatalf("empty .gitmodules should be removed: %v", err)
	}
	if status, _ := svc.GetRepoStatus(super); len(status.StagedFiles) != 0 {
		t.Fatalf("removing the only submodule should leave no staged .gitmodules, got %+v", status.StagedFiles)
	}
	if _, err := os.Stat(filepath.Join(super, "vendor", "sub")); !os.IsNotExist(err) {
		t.Fatalf("submodule working directory should be removed: %v", err)
	}
	if err := svc.AddSubmodule(super, "", "vendor/bad"); err == nil {
		t.Fatal("expected error for empty submodule URL")
	}
	if err := svc.RemoveSubmodule(super, ""); err == nil {
		t.Fatal("expected error for empty submodule path")
	}
}
