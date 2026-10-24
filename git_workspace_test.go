package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceStats(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Core.cs", "line1\nline2\nline3\n", "feat: 核心逻辑", time.Now(), "Alice")

	// Modify one tracked file and add one untracked asset.
	if err := os.WriteFile(filepath.Join(dir, "src/Core.cs"), []byte("line1\nlineX\nline3\nline4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	assets := filepath.Join(dir, "Assets")
	if err := os.MkdirAll(assets, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "Hero.png"), []byte("png"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	stats, err := svc.GetWorkspaceStats(dir)
	if err != nil {
		t.Fatalf("GetWorkspaceStats failed: %v", err)
	}
	if stats.UnstagedFiles != 1 || stats.UntrackedFiles != 1 || stats.StagedFiles != 0 {
		t.Fatalf("unexpected file counts: %+v", stats)
	}
	if stats.UnstagedAdditions != 2 || stats.UnstagedDeletions != 1 {
		t.Fatalf("unexpected churn: +%d -%d", stats.UnstagedAdditions, stats.UnstagedDeletions)
	}
	if stats.TotalAdditions != 2 || stats.TotalDeletions != 1 {
		t.Fatalf("unexpected total churn: +%d -%d", stats.TotalAdditions, stats.TotalDeletions)
	}

	cats := map[string]WorkspaceCategoryStat{}
	for _, cat := range stats.Categories {
		cats[cat.Category] = cat
	}
	if cats["代码"].Files != 1 || cats["代码"].Additions != 2 {
		t.Fatalf("expected 代码 category with churn, got %+v", cats["代码"])
	}
	if cats["美术"].Files != 1 {
		t.Fatalf("expected 美术 category for the untracked png, got %+v", cats["美术"])
	}
	if !strings.Contains(stats.Summary, "变更文件") {
		t.Fatalf("unexpected summary: %s", stats.Summary)
	}

	// Stage the tracked file: churn moves to the staged bucket.
	runGitTest(t, dir, nil, "add", "src/Core.cs")
	staged, err := svc.GetWorkspaceStats(dir)
	if err != nil {
		t.Fatalf("second GetWorkspaceStats failed: %v", err)
	}
	if staged.StagedFiles != 1 || staged.StagedAdditions != 2 || staged.StagedDeletions != 1 {
		t.Fatalf("unexpected staged stats: %+v", staged)
	}
	if staged.UnstagedAdditions != 0 {
		t.Fatalf("expected no unstaged churn after staging, got %d", staged.UnstagedAdditions)
	}
	for _, file := range staged.Files {
		if file.Path == "src/Core.cs" && !file.Staged {
			t.Fatalf("Core.cs should be reported as staged: %+v", file)
		}
	}

	// Clean tree.
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "feat: 资产与逻辑更新")
	clean, err := svc.GetWorkspaceStats(dir)
	if err != nil {
		t.Fatalf("clean stats failed: %v", err)
	}
	if clean.StagedFiles != 0 || clean.UnstagedFiles != 0 || clean.UntrackedFiles != 0 {
		t.Fatalf("expected clean workspace, got %+v", clean)
	}
	if len(clean.Categories) != 0 || clean.TotalAdditions != 0 {
		t.Fatalf("expected empty breakdown, got %+v", clean)
	}
}

func TestWorkspaceStatsReportsReadFailures(t *testing.T) {
	if _, err := (GitService{}).GetWorkspaceStats(t.TempDir()); err == nil {
		t.Fatal("non-git directory must return an error instead of an empty workspace")
	}

	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "a.txt", "a\n", "chore: init", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "--cached" ]; then
    echo "simulated cached diff failure" >&2
    exit 128
  fi
done
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := (GitService{}).GetWorkspaceStats(repo); err == nil {
		t.Fatal("staged diff read failure must return an error instead of partial stats")
	}
}
