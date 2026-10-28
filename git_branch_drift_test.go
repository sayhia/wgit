package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBranchDriftInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-drift-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v, out: %s", args, err, out)
		}
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	f1 := filepath.Join(tempDir, "main.go")
	os.WriteFile(f1, []byte("package main\n\nfunc main() {}\n"), 0644)
	runGit("add", "main.go")
	runGit("commit", "-m", "initial main commit")

	// Branch feature-a
	runGit("checkout", "-b", "feature-a")
	os.WriteFile(f1, []byte("package main\n\n// feature A\nfunc main() {}\n"), 0644)
	runGit("commit", "-am", "commit feature A")

	// Branch feature-b from main
	runGit("checkout", "main")
	runGit("checkout", "-b", "feature-b")
	os.WriteFile(f1, []byte("package main\n\n// feature B\nfunc main() {}\n"), 0644)
	runGit("commit", "-am", "commit feature B")

	svc := GitService{}

	// Test AnalyzeBranchDrift
	report, err := svc.AnalyzeBranchDrift(tempDir, "feature-a", "main")
	if err != nil {
		t.Fatalf("AnalyzeBranchDrift failed: %v", err)
	}
	if report == nil || report.TargetBranch != "feature-a" {
		t.Fatalf("Unexpected report: %+v", report)
	}

	// Test CalculateBranchOverlapMatrix
	matrix, err := svc.CalculateBranchOverlapMatrix(tempDir, []string{"main", "feature-a", "feature-b"})
	if err != nil {
		t.Fatalf("CalculateBranchOverlapMatrix failed: %v", err)
	}
	if matrix == nil || matrix.TotalBranches != 3 {
		t.Fatalf("Unexpected matrix: %+v", matrix)
	}
}
