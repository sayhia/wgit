package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebaseRadarInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-radar-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := GitService{}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
		return string(out)
	}

	// Initialize git repo
	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// Commit 1 on main
	fileA := filepath.Join(tempDir, "fileA.txt")
	_ = os.WriteFile(fileA, []byte("line1\n"), 0644)
	runGit("add", "fileA.txt")
	runGit("commit", "-m", "commit 1")

	// Create feature branch
	runGit("checkout", "-b", "feature")
	fileB := filepath.Join(tempDir, "fileB.txt")
	_ = os.WriteFile(fileB, []byte("feature line\n"), 0644)
	runGit("add", "fileB.txt")
	runGit("commit", "-m", "feature commit 1")

	// Feature commit 2
	fileC := filepath.Join(tempDir, "fileC.txt")
	_ = os.WriteFile(fileC, []byte("feature line 2\n"), 0644)
	runGit("add", "fileC.txt")
	runGit("commit", "-m", "feature commit 2")

	// Get commit hashes
	logOut := runGit("log", "--format=%H", "main..feature")
	var hashes []string
	for _, l := range strings.Split(logOut, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			hashes = append(hashes, l)
		}
	}

	// Test SimulateRebaseOnto
	report, err := svc.SimulateRebaseOnto(tempDir, "main", hashes)
	if err != nil {
		t.Fatalf("SimulateRebaseOnto failed: %v", err)
	}
	if report.TotalSteps != len(hashes) {
		t.Errorf("expected %d steps, got %d", len(hashes), report.TotalSteps)
	}
	if report.ViabilityScore <= 0 {
		t.Errorf("expected positive viability score, got %d", report.ViabilityScore)
	}

	// Test ValidateRebaseTodoGraph
	// Invalid case: first commit is squash
	todoInvalid := []RebaseTodoItem{
		{Action: "squash", Hash: "abc1234", Subject: "squash commit"},
		{Action: "pick", Hash: "def5678", Subject: "pick commit"},
	}
	valReport, err := svc.ValidateRebaseTodoGraph(tempDir, todoInvalid)
	if err != nil {
		t.Fatalf("ValidateRebaseTodoGraph failed: %v", err)
	}
	if valReport.Valid {
		t.Errorf("expected todo to be invalid when first commit is squash")
	}

	// Valid case: pick followed by squash
	todoValid := []RebaseTodoItem{
		{Action: "pick", Hash: "def5678", Subject: "pick commit"},
		{Action: "squash", Hash: "abc1234", Subject: "squash commit"},
	}
	valReportValid, err := svc.ValidateRebaseTodoGraph(tempDir, todoValid)
	if err != nil {
		t.Fatalf("ValidateRebaseTodoGraph failed: %v", err)
	}
	if !valReportValid.Valid {
		t.Errorf("expected todo to be valid, got errors: %v", valReportValid.Errors)
	}
}
