package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetProjectsDetailsReportsStatusFailure(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	for _, args := range [][]string{
		{"config", "user.name", "Tester"},
		{"config", "user.email", "tester@example.com"},
	} {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "a.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: init", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	details, err := svc.GetProjectsDetails([]string{repo}, repo)
	if err != nil || len(details) != 1 {
		t.Fatalf("GetProjectsDetails failed: %v, details=%+v", err, details)
	}
	if details[0].StatusWarning != "" {
		t.Fatalf("clean repo must not carry a status warning: %+v", details[0])
	}

	if err := os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	details, err = svc.GetProjectsDetails([]string{repo}, repo)
	if err != nil || len(details) != 1 {
		t.Fatalf("GetProjectsDetails after corruption failed: %v, details=%+v", err, details)
	}
	if !strings.Contains(details[0].StatusWarning, "读取工作区状态失败") {
		t.Fatalf("status read failure must be surfaced instead of a clean repo: %+v", details[0])
	}
}
