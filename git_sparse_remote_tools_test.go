package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSparseCheckoutAndWorktreeLockingInTempRepo(t *testing.T) {
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
	for _, file := range []string{"src/main.go", "docs/readme.md"} {
		full := filepath.Join(repo, file)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "root.txt"), []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageAll(repo); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if folders, err := svc.GetSparseCheckoutFolders(repo); err != nil || len(folders) != 0 {
		t.Fatalf("expected no sparse folders initially, got %+v err=%v", folders, err)
	}
	if err := svc.SetSparseCheckoutFolders(repo, []string{"src"}); err != nil {
		t.Fatalf("SetSparseCheckoutFolders failed: %v", err)
	}
	folders, err := svc.GetSparseCheckoutFolders(repo)
	if err != nil || len(folders) != 1 || folders[0] != "src" {
		t.Fatalf("expected sparse folder src, got %+v err=%v", folders, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "src", "main.go")); err != nil {
		t.Fatalf("selected src folder should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "docs", "readme.md")); !os.IsNotExist(err) {
		t.Fatalf("unselected docs folder should be excluded: %v", err)
	}
	if err := svc.SetSparseCheckoutFolders(repo, []string{"src", "docs"}); err != nil {
		t.Fatalf("expanding sparse folders failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "docs", "readme.md")); err != nil {
		t.Fatalf("expanded docs folder should be restored: %v", err)
	}
	if err := svc.SetSparseCheckoutFolders(repo, nil); err != nil {
		t.Fatalf("disabling sparse checkout failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "docs", "readme.md")); err != nil {
		t.Fatalf("disabling sparse checkout should restore all files: %v", err)
	}
	if folders, _ := svc.GetSparseCheckoutFolders(repo); len(folders) != 0 {
		t.Fatalf("expected sparse checkout to be disabled, got %+v", folders)
	}

	if err := svc.CreateBranch(repo, "wt-branch", ""); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	wtPath := filepath.Join(parent, "wt")
	if err := svc.AddWorktree(repo, wtPath, "wt-branch", false); err != nil {
		t.Fatalf("AddWorktree failed: %v", err)
	}
	if err := svc.LockWorktree(repo, wtPath, "hold for release"); err != nil {
		t.Fatalf("LockWorktree failed: %v", err)
	}
	worktrees, _ := svc.GetWorktrees(repo)
	var locked bool
	for _, wt := range worktrees {
		if filepath.Base(wt.Path) == "wt" {
			locked = wt.IsLocked && strings.Contains(wt.LockReason, "hold for release")
		}
	}
	if !locked {
		t.Fatalf("expected locked worktree with reason, got %+v", worktrees)
	}
	if err := svc.UnlockWorktree(repo, wtPath); err != nil {
		t.Fatalf("UnlockWorktree failed: %v", err)
	}
	worktrees, _ = svc.GetWorktrees(repo)
	for _, wt := range worktrees {
		if filepath.Base(wt.Path) == "wt" && wt.IsLocked {
			t.Fatalf("worktree should be unlocked: %+v", wt)
		}
	}
	if err := svc.RemoveWorktree(repo, wtPath, true); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}
}

func TestRemoteRenamePruneBundleReportSearchInTempRepo(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "needle.go"), []byte("package demo\n// NEEDLE symbol\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "root.txt"), []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageAll(repo); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}
	if err := svc.Commit(repo, "feat: searchable base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	baseBranch, _ := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	baseBranch = strings.TrimSpace(baseBranch)

	remote := filepath.Join(parent, "remote.git")
	if _, err := ExecGit(parent, "init", "--bare", remote); err != nil {
		t.Fatalf("init bare remote failed: %v", err)
	}
	if err := svc.AddRemote(repo, "origin", remote); err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}
	if _, err := svc.Push(repo, "origin", baseBranch, false); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if err := svc.CreateBranch(repo, "feature-old", ""); err != nil {
		t.Fatalf("CreateBranch(feature-old) failed: %v", err)
	}
	if err := svc.CheckoutBranch(repo, "feature-old", false); err != nil {
		t.Fatalf("checkout feature-old failed: %v", err)
	}
	if _, err := svc.Push(repo, "origin", "feature-old", false); err != nil {
		t.Fatalf("Push(feature-old) failed: %v", err)
	}
	if err := svc.RenameBranchWithRemote(repo, "feature-old", "feature-renamed", true); err != nil {
		t.Fatalf("RenameBranchWithRemote failed: %v", err)
	}
	if current, _ := ExecGit(repo, "symbolic-ref", "--short", "HEAD"); strings.TrimSpace(current) != "feature-renamed" {
		t.Fatalf("expected current branch feature-renamed, got %q", current)
	}
	remoteHeads, err := ExecGit(repo, "ls-remote", "--heads", "origin")
	if err != nil || !strings.Contains(remoteHeads, "refs/heads/feature-renamed") || strings.Contains(remoteHeads, "refs/heads/feature-old") {
		t.Fatalf("remote branch rename not synced: %q err=%v", remoteHeads, err)
	}
	if err := svc.RenameBranchWithRemote(repo, "", "x", false); err == nil {
		t.Fatal("expected error renaming empty branch")
	}

	if _, err := ExecGit(repo, "push", "origin", "HEAD:stale"); err != nil {
		t.Fatalf("push stale branch failed: %v", err)
	}
	if _, err := svc.Fetch(repo, "origin"); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if _, err := ExecGit(remote, "update-ref", "-d", "refs/heads/stale"); err != nil {
		t.Fatalf("delete stale remote ref failed: %v", err)
	}
	branches, _ := svc.GetBranches(repo)
	var staleTracked bool
	for _, branch := range branches {
		if branch.IsRemote && strings.HasSuffix(branch.Name, "/stale") {
			staleTracked = true
		}
	}
	if !staleTracked {
		t.Fatalf("expected stale remote-tracking branch before prune, got %+v", branches)
	}
	if _, err := svc.PruneRemote(repo, "origin"); err != nil {
		t.Fatalf("PruneRemote failed: %v", err)
	}
	branches, _ = svc.GetBranches(repo)
	for _, branch := range branches {
		if branch.IsRemote && strings.HasSuffix(branch.Name, "/stale") {
			t.Fatalf("stale remote-tracking branch should be pruned: %+v", branches)
		}
	}

	bundlePath := filepath.Join(parent, "repo.bundle")
	if _, err := svc.CreateGitBundle(repo, bundlePath, "--all"); err != nil {
		t.Fatalf("CreateGitBundle failed: %v", err)
	}
	if info, err := os.Stat(bundlePath); err != nil || info.Size() == 0 {
		t.Fatalf("bundle file should exist and be non-empty: %v", err)
	}
	if _, err := svc.VerifyGitBundle(repo, bundlePath); err != nil {
		t.Fatalf("VerifyGitBundle failed: %v", err)
	}
	if _, err := svc.VerifyGitBundle(repo, ""); err == nil {
		t.Fatal("expected error for empty bundle path")
	}
	if _, err := svc.CreateGitBundle(repo, "", "--all"); err == nil {
		t.Fatal("expected error for empty bundle output path")
	}

	report, err := svc.ExportCommitReport(repo, "", "", "markdown")
	if err != nil || !strings.Contains(report, "# wgit 提交发布报告") || !strings.Contains(report, "feat: searchable base") {
		t.Fatalf("markdown report failed: %q err=%v", report, err)
	}
	report, err = svc.ExportCommitReport(repo, "", "", "html")
	if err != nil || !strings.Contains(report, "<table") || !strings.Contains(report, "feat: searchable base") {
		t.Fatalf("html report failed: %q err=%v", report, err)
	}

	matches, err := svc.SearchRepoCode(repo, "needle", true, 10)
	if err != nil || len(matches) != 1 {
		t.Fatalf("SearchRepoCode failed: %+v err=%v", matches, err)
	}
	if matches[0].Path != "src/needle.go" || matches[0].LineNumber != 2 || !strings.Contains(matches[0].LineContent, "NEEDLE") {
		t.Fatalf("unexpected search match: %+v", matches[0])
	}
	if matches, _ := svc.SearchRepoCode(repo, "needle", false, 10); len(matches) != 0 {
		t.Fatalf("case-sensitive search should not match NEEDLE, got %+v", matches)
	}
	if matches, err := svc.SearchRepoCode(repo, "   ", true, 10); err != nil || len(matches) != 0 {
		t.Fatalf("empty query should return no matches: %+v err=%v", matches, err)
	}
}

func TestExportCommitReportEscapesMarkup(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "report.txt"), []byte("report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "report.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	subject := `feat: <script>alert("x")</script> | pipe`
	if err := svc.Commit(repo, subject, false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	markdown, err := svc.ExportCommitReport(repo, "", "", "markdown")
	if err != nil {
		t.Fatalf("markdown report failed: %v", err)
	}
	if !strings.Contains(markdown, `\| pipe`) {
		t.Fatalf("markdown report should escape table pipes: %q", markdown)
	}
	if !strings.Contains(markdown, `feat: <script>alert("x")</script> \| pipe`) {
		t.Fatalf("markdown report should preserve the full subject with escaped pipe: %q", markdown)
	}

	htmlReport, err := svc.ExportCommitReport(repo, "", "", "html")
	if err != nil {
		t.Fatalf("html report failed: %v", err)
	}
	if strings.Contains(htmlReport, "<script>alert") {
		t.Fatalf("html report must not inject commit markup: %q", htmlReport)
	}
	if !strings.Contains(htmlReport, "&lt;script&gt;alert") {
		t.Fatalf("html report should escape commit markup: %q", htmlReport)
	}
	if !strings.Contains(htmlReport, "| pipe") {
		t.Fatalf("html report should preserve pipes in the subject: %q", htmlReport)
	}
}
