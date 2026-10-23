package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreRepoWorkflowInTempRepo(t *testing.T) {
	svc := GitService{}
	parent := t.TempDir()
	repo := filepath.Join(parent, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	info, err := svc.InitRepo(repo)
	if err != nil || info == nil || info.Name != "demo" || info.CurrentBranch == "" {
		t.Fatalf("InitRepo failed: %v, info=%+v", err, info)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	opened, err := svc.OpenRepo(repo)
	if err != nil || opened == nil || opened.Name != "demo" {
		t.Fatalf("OpenRepo failed: %v, info=%+v", err, opened)
	}

	filePath := filepath.Join(repo, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := svc.GetRepoStatus(repo)
	if err != nil || status.IsClean {
		t.Fatalf("GetRepoStatus should report a dirty workspace: %v, status=%+v", err, status)
	}

	diff, err := svc.GetWorkingDiff(repo, "hello.txt", false)
	if err != nil || !strings.Contains(diff, "+hello") {
		t.Fatalf("GetWorkingDiff failed: %v, diff=%q", err, diff)
	}

	if err := svc.StageFile(repo, "hello.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	status, _ = svc.GetRepoStatus(repo)
	if len(status.StagedFiles) != 1 {
		t.Fatalf("expected 1 staged file, got %+v", status.StagedFiles)
	}
	stagedDiff, err := svc.GetWorkingDiff(repo, "hello.txt", true)
	if err != nil || !strings.Contains(stagedDiff, "+hello") {
		t.Fatalf("staged GetWorkingDiff failed: %v, diff=%q", err, stagedDiff)
	}

	if err := svc.UnstageFile(repo, "hello.txt"); err != nil {
		t.Fatalf("UnstageFile failed: %v", err)
	}
	status, _ = svc.GetRepoStatus(repo)
	if len(status.StagedFiles) != 0 {
		t.Fatalf("expected no staged files after unstage, got %+v", status.StagedFiles)
	}

	if err := svc.StageFile(repo, "hello.txt"); err != nil {
		t.Fatalf("StageFile before commit failed: %v", err)
	}
	if err := svc.Commit(repo, "feat: initial hello", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	status, _ = svc.GetRepoStatus(repo)
	if !status.IsClean {
		t.Fatalf("expected a clean workspace after commit: %+v", status)
	}

	files, err := svc.GetProjectAllFiles(repo)
	if err != nil || len(files) != 1 || files[0] != "hello.txt" {
		t.Fatalf("GetProjectAllFiles failed: %v, files=%+v", err, files)
	}

	repos, err := svc.ScanLocalProjects(parent)
	if err != nil || len(repos) != 1 || filepath.Clean(repos[0]) != filepath.Clean(repo) {
		t.Fatalf("ScanLocalProjects failed: %v, repos=%+v", err, repos)
	}

	details, err := svc.GetProjectsDetails([]string{repo}, repo)
	if err != nil || len(details) != 1 || details[0].Path != repo {
		t.Fatalf("GetProjectsDetails failed: %v, details=%+v", err, details)
	}

	oldLogs := GetRecentActivityLogs()
	if len(oldLogs) == 0 {
		t.Fatalf("expected ExecGit calls to be recorded in the activity log")
	}
	ClearRecentActivityLogs()
	if logs := GetRecentActivityLogs(); len(logs) != 0 {
		t.Fatalf("ClearRecentActivityLogs left %d entries", len(logs))
	}
}
