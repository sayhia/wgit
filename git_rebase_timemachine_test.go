package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRebaseTimeMachineInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-rebase-tm-test-*")
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

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// Commit 1 on main
	_ = os.WriteFile(filepath.Join(tempDir, "core.go"), []byte("package main\n"), 0644)
	runGit("add", "core.go")
	runGit("commit", "-m", "initial main commit")

	// Branch feature
	runGit("checkout", "-b", "feature")
	_ = os.WriteFile(filepath.Join(tempDir, "feature_mod.go"), []byte("package main\nfunc Feature() {}\n"), 0644)
	runGit("add", "feature_mod.go")
	runGit("commit", "-m", "feat: new feature module")

	// Switch back to main and add another commit
	runGit("checkout", "main")
	_ = os.WriteFile(filepath.Join(tempDir, "other.go"), []byte("package main\nfunc Other() {}\n"), 0644)
	runGit("add", "other.go")
	runGit("commit", "-m", "feat: other main work")

	// Test SimulateRebaseTimeMachine: rebase feature onto main
	rep, err := svc.SimulateRebaseTimeMachine(tempDir, "main", "feature")
	if err != nil {
		t.Fatalf("SimulateRebaseTimeMachine failed: %v", err)
	}

	if rep.TotalCommits != 1 {
		t.Errorf("expected 1 commit to replay, got %d", rep.TotalCommits)
	}
	if rep.CleanCount != 1 {
		t.Errorf("expected 1 clean commit, got %d", rep.CleanCount)
	}
	if rep.OverallFeasibility != 100 {
		t.Errorf("expected 100%% feasibility, got %d", rep.OverallFeasibility)
	}
	if len(rep.CommitsToReplay) != 1 || rep.CommitsToReplay[0].PredictedStatus != "clean" {
		t.Errorf("unexpected commit node status: %+v", rep.CommitsToReplay)
	}
}
