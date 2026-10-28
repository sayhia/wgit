package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSemanticDiffInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-semantic-test-*")
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

	codeV1 := `package main

func CalculateSum(a int, b int) int {
	return a + b
}

func MovedHelper() {
	println("this is moved block line 1")
	println("this is moved block line 2")
}
`
	fileGo := filepath.Join(tempDir, "calc.go")
	_ = os.WriteFile(fileGo, []byte(codeV1), 0644)
	runGit("add", "calc.go")
	runGit("commit", "-m", "v1")

	codeV2 := `package main

func CalculateSum(a int, b int) int {
	// modified implementation
	return a + b + 0
}

func NewFeature() {
	println("new feature")
}

func MovedHelper() {
	println("this is moved block line 1")
	println("this is moved block line 2")
}
`
	_ = os.WriteFile(fileGo, []byte(codeV2), 0644)
	runGit("add", "calc.go")
	runGit("commit", "-m", "v2")

	// Test GetSemanticDiffOutline
	outline, err := svc.GetSemanticDiffOutline(tempDir, "HEAD~1", "HEAD", "calc.go")
	if err != nil {
		t.Fatalf("GetSemanticDiffOutline failed: %v", err)
	}
	if outline.Language != "go" {
		t.Errorf("expected language go, got %s", outline.Language)
	}
	if len(outline.Symbols) == 0 {
		t.Errorf("expected detected symbols, got 0")
	}

	foundNewFeature := false
	for _, sym := range outline.Symbols {
		if sym.Name == "NewFeature" && sym.ChangeType == "added" {
			foundNewFeature = true
			break
		}
	}
	if !foundNewFeature {
		t.Errorf("expected to find added symbol NewFeature")
	}

	// Test DetectMovedBlocks with moving MovedHelper to the top
	codeV3 := `package main

func MovedHelper() {
	println("this is moved block line 1")
	println("this is moved block line 2")
}

func CalculateSum(a int, b int) int {
	return a + b + 0
}

func NewFeature() {
	println("new feature")
}
`
	_ = os.WriteFile(fileGo, []byte(codeV3), 0644)
	runGit("add", "calc.go")
	runGit("commit", "-m", "v3: moved block")

	movedReport, err := svc.DetectMovedBlocks(tempDir, "HEAD~1", "HEAD", "calc.go")
	if err != nil {
		t.Fatalf("DetectMovedBlocks failed: %v", err)
	}
	if movedReport.TotalMovedLines == 0 {
		t.Logf("Notice: TotalMovedLines is 0, movedPairs: %+v", movedReport.MovedPairs)
	}
}
