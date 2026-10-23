package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInteractiveRebaseExecuteInTempRepo(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	commitFile := func(name, message string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(message+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := svc.StageFile(repo, name); err != nil {
			t.Fatalf("StageFile(%s) failed: %v", name, err)
		}
		if err := svc.Commit(repo, message, false); err != nil {
			t.Fatalf("Commit(%s) failed: %v", message, err)
		}
	}
	commitFile("base.txt", "chore: base")
	baseHash, _ := ExecGit(repo, "rev-parse", "HEAD")
	baseHash = strings.TrimSpace(baseHash)
	commitFile("a.txt", "feat: a")
	commitFile("b.txt", "feat: b")
	commitFile("c.txt", "feat: c")

	plan, err := svc.InteractiveRebasePlan(repo, baseHash)
	if err != nil || len(plan) != 3 {
		t.Fatalf("InteractiveRebasePlan failed: %+v err=%v", plan, err)
	}
	if plan[0].Subject != "feat: a" || plan[1].Subject != "feat: b" || plan[2].Subject != "feat: c" {
		t.Fatalf("unexpected plan order: %+v", plan)
	}
	if _, err := svc.InteractiveRebasePlan(repo, "definitely-not-a-ref"); err == nil {
		t.Fatal("invalid upstream must return an error instead of an empty plan")
	}
	if out, err := svc.InteractiveRebaseExecute(repo, baseHash, nil); err != nil || !strings.Contains(out, "没有可变基的提交") {
		t.Fatalf("empty rebase plan should be a no-op: %q err=%v", out, err)
	}

	todos := []RebaseTodoItem{
		{Action: "pick", ShortHash: plan[0].ShortHash, Subject: plan[0].Subject},
		{Action: "drop", ShortHash: plan[1].ShortHash, Subject: plan[1].Subject},
		{Action: "pick", ShortHash: plan[2].ShortHash, Subject: plan[2].Subject},
	}
	if out, err := svc.InteractiveRebaseExecute(repo, baseHash, todos); err != nil {
		t.Fatalf("InteractiveRebaseExecute failed: %v (%s)", err, out)
	}
	subjects, err := ExecGit(repo, "log", "--format=%s")
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(subjects), "\n")
	if len(lines) != 3 || lines[0] != "feat: c" || lines[1] != "feat: a" || lines[2] != "chore: base" {
		t.Fatalf("unexpected history after dropping feat: b: %q", lines)
	}
	if _, err := os.Stat(filepath.Join(repo, "b.txt")); !os.IsNotExist(err) {
		t.Fatalf("dropped commit file should not exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "c.txt")); err != nil {
		t.Fatalf("kept commit file should exist: %v", err)
	}
	if status, _ := svc.GetRepoStatus(repo); !status.IsClean {
		t.Fatalf("workspace should be clean after rebase, got %+v", status)
	}
}
