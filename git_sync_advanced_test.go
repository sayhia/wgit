package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdvancedSyncOpsInTempRepo(t *testing.T) {
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
	write := func(dir, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commitFile := func(dir, name, content, message string) {
		t.Helper()
		write(dir, name, content)
		if _, err := ExecGit(dir, "add", "--", name); err != nil {
			t.Fatalf("git add %s failed: %v", name, err)
		}
		if _, err := ExecGit(dir, "commit", "-m", message); err != nil {
			t.Fatalf("git commit %s failed: %v", message, err)
		}
	}

	write(repo, "base.txt", "base\n")
	if _, err := ExecGit(repo, "add", "base.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "commit", "-m", "chore: base"); err != nil {
		t.Fatal(err)
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
	if err := svc.SetUpstream(repo, baseBranch, "origin/"+baseBranch); err != nil {
		t.Fatalf("SetUpstream failed: %v", err)
	}

	commitFile(repo, "local.txt", "local\n", "feat: local")
	ahead, err := svc.GetAheadCommits(repo, "origin", baseBranch)
	if err != nil || len(ahead) != 1 || ahead[0].Subject != "feat: local" {
		t.Fatalf("GetAheadCommits failed: %+v err=%v", ahead, err)
	}
	if behind, err := svc.GetBehindCommits(repo, "origin", baseBranch); err != nil || len(behind) != 0 {
		t.Fatalf("expected no behind commits, got %+v err=%v", behind, err)
	}
	pushRes, err := svc.PushAdvanced(repo, PushAdvancedOptions{
		Remote: "origin", Branch: baseBranch, RemoteBranch: baseBranch, SetUpstream: true,
	})
	if err != nil || pushRes == nil || !pushRes.Success {
		t.Fatalf("PushAdvanced failed: %+v err=%v", pushRes, err)
	}
	commitFile(repo, "local2.txt", "local2\n", "feat: local2")

	second := filepath.Join(parent, "second")
	if _, err := ExecGit(parent, "clone", remote, second); err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	if _, err := ExecGit(second, "config", "user.name", "Tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(second, "config", "user.email", "tester@example.com"); err != nil {
		t.Fatal(err)
	}
	commitFile(second, "remote.txt", "remote\n", "feat: remote")
	if _, err := ExecGit(second, "push", "origin", baseBranch); err != nil {
		t.Fatalf("second push failed: %v", err)
	}
	if _, err := svc.Fetch(repo, "origin"); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	behind, err := svc.GetBehindCommits(repo, "origin", baseBranch)
	if err != nil || len(behind) != 1 || behind[0].Subject != "feat: remote" {
		t.Fatalf("GetBehindCommits failed: %+v err=%v", behind, err)
	}

	preview, err := svc.PreviewMerge(repo, "origin/"+baseBranch)
	if err != nil || preview == nil || !preview.CanMergeCleanly || preview.HasConflicts || preview.AheadCount != 1 || preview.BehindCount != 1 {
		t.Fatalf("PreviewMerge clean failed: %+v err=%v", preview, err)
	}
	pullRes, err := svc.PullAdvanced(repo, PullAdvancedOptions{Remote: "origin", Branch: baseBranch, Strategy: "rebase"})
	if err != nil || pullRes == nil || !pullRes.Success || pullRes.HasConflicts || !pullRes.Updated {
		t.Fatalf("PullAdvanced rebase failed: %+v err=%v", pullRes, err)
	}
	for _, file := range []string{"local.txt", "local2.txt", "remote.txt"} {
		if _, err := os.Stat(filepath.Join(repo, file)); err != nil {
			t.Fatalf("rebase pull lost %s: %v", file, err)
		}
	}

	commitFile(repo, "base.txt", "local-line\n", "feat: local conflict")
	commitFile(second, "base.txt", "remote-line\n", "feat: remote conflict")
	if _, err := ExecGit(second, "push", "origin", baseBranch); err != nil {
		t.Fatalf("second conflict push failed: %v", err)
	}
	if _, err := svc.Fetch(repo, "origin"); err != nil {
		t.Fatalf("Fetch before conflict failed: %v", err)
	}
	preview, err = svc.PreviewMerge(repo, "origin/"+baseBranch)
	if err != nil || preview == nil || !preview.HasConflicts || preview.CanMergeCleanly {
		t.Fatalf("PreviewMerge should detect conflicts: %+v err=%v", preview, err)
	}
	var sawBase bool
	for _, file := range preview.Conflicts {
		if strings.Contains(file, "base.txt") {
			sawBase = true
		}
	}
	if !sawBase {
		t.Fatalf("expected base.txt conflict, got %+v", preview.Conflicts)
	}
	conflictPull, err := svc.PullAdvanced(repo, PullAdvancedOptions{Remote: "origin", Branch: baseBranch, Strategy: "merge"})
	if err != nil {
		t.Fatalf("conflict pull should return a structured result, got err=%v result=%+v", err, conflictPull)
	}
	if conflictPull == nil || !conflictPull.HasConflicts || len(conflictPull.Conflicts) == 0 {
		t.Fatalf("expected structured pull conflicts, got %+v", conflictPull)
	}
	if _, err := ExecGit(repo, "merge", "--abort"); err != nil {
		t.Fatalf("merge abort failed: %v", err)
	}
	if status, _ := svc.GetRepoStatus(repo); status.IsMerging {
		t.Fatalf("merge state should be aborted: %+v", status)
	}

	progress, err := svc.GetRebaseProgress(repo)
	if err != nil || progress == nil || progress.IsRebasing || progress.ConflictedFiles == nil {
		t.Fatalf("GetRebaseProgress idle failed: %+v err=%v", progress, err)
	}
}
