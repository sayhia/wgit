package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetFileBlameHeatmapInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-blame-heatmap-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Alice",
			"GIT_AUTHOR_EMAIL=alice@example.com",
			"GIT_COMMITTER_NAME=Alice",
			"GIT_COMMITTER_EMAIL=alice@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.name", "Alice")
	runGit("config", "user.email", "alice@example.com")

	f := filepath.Join(tempDir, "script.py")
	_ = os.WriteFile(f, []byte("def hello():\n    print('hi')\n"), 0644)
	runGit("add", "script.py")
	runGit("commit", "-m", "init script")

	service := &GitService{}
	res, err := service.GetFileBlameHeatmap(tempDir, "script.py")
	if err != nil {
		t.Fatalf("GetFileBlameHeatmap failed: %v", err)
	}

	if res.TotalLines != 2 {
		t.Errorf("expected 2 lines, got %d", res.TotalLines)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("expected 2 parsed lines")
	}
	if res.Lines[0].Author != "Alice" {
		t.Errorf("expected author Alice, got %s", res.Lines[0].Author)
	}
	if res.Lines[0].HeatRatio < 0.0 || res.Lines[0].HeatRatio > 1.0 {
		t.Errorf("invalid heat ratio: %f", res.Lines[0].HeatRatio)
	}
}
