package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupBlameTestRepo(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	run("config", "user.name", "Alice Author")
	run("config", "user.email", "alice@example.com")
	run("checkout", "-q", "-b", "main")

	// Commit 1: Initial creation of test.txt with 7 lines
	c1Content := "line 1: alice initial\n" +
		"line 2: alice initial\n" +
		"line 3: alice initial\n" +
		"line 4: alice initial\n" +
		"line 5: alice initial\n" +
		"line 6: alice initial\n" +
		"line 7: alice initial\n"
	write("test.txt", c1Content)
	run("add", "test.txt")
	run("commit", "-qm", "feat: initial commit by alice")
	c1Hash := run("rev-parse", "HEAD")

	// Commit 2: Bob modifies lines 2 and 3
	run("config", "user.name", "Bob Committer")
	run("config", "user.email", "bob@example.com")
	c2Content := "line 1: alice initial\n" +
		"line 2: bob modified\n" +
		"line 3: bob modified\n" +
		"line 4: alice initial\n" +
		"line 5: alice initial\n" +
		"line 6: alice initial\n" +
		"line 7: alice initial\n"
	write("test.txt", c2Content)
	run("add", "test.txt")
	run("commit", "-qm", "fix: bob updates lines 2 and 3")
	c2Hash := run("rev-parse", "HEAD")

	return dir, c1Hash, c2Hash
}

func TestBlameWithOptions(t *testing.T) {
	repoDir, c1Hash, c2Hash := setupBlameTestRepo(t)
	svc := GitService{}

	t.Run("BlameCommitRef", func(t *testing.T) {
		// Blame historical commit C1
		lines, err := svc.GetFileBlameWithOptions(repoDir, BlameOptions{
			FilePath: "test.txt",
			Ref:      c1Hash,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lines) != 7 {
			t.Fatalf("expected 7 lines, got %d", len(lines))
		}

		for idx, line := range lines {
			if line.LineNumber != idx+1 {
				t.Errorf("line %d: expected lineNumber %d, got %d", idx, idx+1, line.LineNumber)
			}
			if line.CommitHash != c1Hash {
				t.Errorf("line %d: expected hash %s, got %s", idx, c1Hash, line.CommitHash)
			}
			if line.Author != "Alice Author" {
				t.Errorf("line %d: expected author 'Alice Author', got '%s'", idx, line.Author)
			}
			if line.AuthorEmail != "alice@example.com" {
				t.Errorf("line %d: expected authorEmail 'alice@example.com', got '%s'", idx, line.AuthorEmail)
			}
			if line.AuthorTimestamp <= 0 {
				t.Errorf("line %d: expected positive authorTimestamp, got %d", idx, line.AuthorTimestamp)
			}
			if line.AuthorTz == "" {
				t.Errorf("line %d: expected non-empty authorTz", idx)
			}
			if !line.IsBoundary {
				t.Errorf("line %d: expected isBoundary to be true for root commit", idx)
			}
			if line.PreviousCommit != "" {
				t.Errorf("line %d: expected empty PreviousCommit on initial commit, got '%s'", idx, line.PreviousCommit)
			}
		}
	})

	t.Run("BlameWorkspaceAndPreviousCommit", func(t *testing.T) {
		// Modify line 5 in workspace without committing
		dirtyContent := "line 1: alice initial\n" +
			"line 2: bob modified\n" +
			"line 3: bob modified\n" +
			"line 4: alice initial\n" +
			"line 5: dirty workspace edit\n" +
			"line 6: alice initial\n" +
			"line 7: alice initial\n"
		if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte(dirtyContent), 0o644); err != nil {
			t.Fatal(err)
		}

		lines, err := svc.GetFileBlameWithOptions(repoDir, BlameOptions{
			FilePath: "test.txt",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lines) != 7 {
			t.Fatalf("expected 7 lines, got %d", len(lines))
		}

		// Line 1: from C1 (Alice, boundary)
		if lines[0].Author != "Alice Author" || lines[0].AuthorEmail != "alice@example.com" {
			t.Errorf("line 1 author mismatch: %s <%s>", lines[0].Author, lines[0].AuthorEmail)
		}
		if !lines[0].IsBoundary {
			t.Errorf("line 1: expected isBoundary=true")
		}

		// Line 2 & 3: from C2 (Bob, has PreviousCommit=c1Hash, PreviousFilename=test.txt, IsBoundary=false)
		for _, idx := range []int{1, 2} {
			line := lines[idx]
			if line.CommitHash != c2Hash {
				t.Errorf("line %d: expected commitHash %s, got %s", idx+1, c2Hash, line.CommitHash)
			}
			if line.Author != "Bob Committer" {
				t.Errorf("line %d: expected author 'Bob Committer', got '%s'", idx+1, line.Author)
			}
			if line.AuthorEmail != "bob@example.com" {
				t.Errorf("line %d: expected authorEmail 'bob@example.com', got '%s'", idx+1, line.AuthorEmail)
			}
			if line.AuthorTimestamp <= 0 {
				t.Errorf("line %d: expected valid authorTimestamp, got %d", idx+1, line.AuthorTimestamp)
			}
			if line.PreviousCommit != c1Hash {
				t.Errorf("line %d: expected PreviousCommit %s, got %s", idx+1, c1Hash, line.PreviousCommit)
			}
			if line.PreviousFilename != "test.txt" {
				t.Errorf("line %d: expected PreviousFilename 'test.txt', got '%s'", idx+1, line.PreviousFilename)
			}
			if line.IsBoundary {
				t.Errorf("line %d: expected isBoundary=false", idx+1)
			}
		}

		// Line 5: dirty workspace line (not committed yet)
		line5 := lines[4]
		if line5.Content != "line 5: dirty workspace edit" {
			t.Errorf("line 5 content mismatch: %s", line5.Content)
		}
		if strings.Trim(line5.CommitHash, "0") != "" && line5.CommitHash != "0000000000000000000000000000000000000000" {
			t.Errorf("line 5: expected uncommitted hash (all 0s), got %s", line5.CommitHash)
		}
		if line5.PreviousCommit != c2Hash {
			t.Errorf("line 5: expected previous commit %s, got %s", c2Hash, line5.PreviousCommit)
		}
	})

	t.Run("BlameLineRange", func(t *testing.T) {
		// Test -L 1,5
		lines, err := svc.GetFileBlameWithOptions(repoDir, BlameOptions{
			FilePath:  "test.txt",
			StartLine: 1,
			EndLine:   5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lines) != 5 {
			t.Fatalf("expected 5 lines for -L 1,5, got %d", len(lines))
		}
		for i := 0; i < 5; i++ {
			if lines[i].LineNumber != i+1 {
				t.Errorf("index %d: expected lineNumber %d, got %d", i, i+1, lines[i].LineNumber)
			}
		}

		// Test -L 2,4
		subLines, err := svc.GetFileBlameWithOptions(repoDir, BlameOptions{
			FilePath:  "test.txt",
			StartLine: 2,
			EndLine:   4,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(subLines) != 3 {
			t.Fatalf("expected 3 lines for -L 2,4, got %d", len(subLines))
		}
		if subLines[0].LineNumber != 2 || subLines[1].LineNumber != 3 || subLines[2].LineNumber != 4 {
			t.Errorf("unexpected lineNumbers for -L 2,4: %d, %d, %d", subLines[0].LineNumber, subLines[1].LineNumber, subLines[2].LineNumber)
		}
	})

	t.Run("CompatibilityGetFileBlame", func(t *testing.T) {
		lines, err := svc.GetFileBlame(repoDir, "test.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lines) != 7 {
			t.Fatalf("expected 7 lines, got %d", len(lines))
		}
		// Confirm all fields are properly populated
		if lines[0].Author != "Alice Author" || lines[0].AuthorEmail != "alice@example.com" {
			t.Errorf("compatibility check author mismatch: %s <%s>", lines[0].Author, lines[0].AuthorEmail)
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		_, err := svc.GetFileBlameWithOptions(repoDir, BlameOptions{FilePath: ""})
		if err == nil {
			t.Error("expected error for empty filePath, got nil")
		}

		_, err = svc.GetFileBlameWithOptions(repoDir, BlameOptions{FilePath: "non_existent.txt"})
		if err == nil {
			t.Error("expected error for non-existent file, got nil")
		}
	})
}
