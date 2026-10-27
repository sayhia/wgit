package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// newNotesRepo isolates the test from the developer's git configuration and
// prepares a repository with a local identity. Tests in this package run
// sequentially (other tests mutate PATH), so t.Setenv is safe here.
func newNotesRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, dir, nil, "config", "commit.gpgsign", "false")
	return dir
}

// TestGetCommitNotesParallelInspection pins the observable contract of the
// concurrent note inspection: every note ref is inspected exactly once, each
// item keeps the metadata and content of its own commit, and the limit still
// truncates the rendered list while Total reports every note in the repo.
// Twelve notes also exceed the eight-worker cap, exercising the queued path.
func TestGetCommitNotesParallelInspection(t *testing.T) {
	dir := newNotesRepo(t)

	const noteCount = 12
	hashes := make([]string, noteCount+1)
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	for n := 1; n <= noteCount; n++ {
		writeAndCommit(t, dir, fmt.Sprintf("src/feature_%02d.txt", n), fmt.Sprintf("v%d\n", n),
			fmt.Sprintf("feat: 第 %d 次提交", n), base.AddDate(0, 0, n), "Alice")
		hashes[n] = strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))
		runGitTest(t, dir, nil, "notes", "add", "-m", fmt.Sprintf("note %d", n), hashes[n])
	}

	svc := GitService{}
	all, err := svc.GetCommitNotes(dir, 0)
	if err != nil {
		t.Fatalf("GetCommitNotes failed: %v", err)
	}
	if all.Total != noteCount {
		t.Fatalf("Total = %d, want %d", all.Total, noteCount)
	}
	if len(all.Notes) != noteCount {
		t.Fatalf("len(Notes) = %d, want %d", len(all.Notes), noteCount)
	}

	byContent := make(map[string]CommitNoteItem, len(all.Notes))
	for _, item := range all.Notes {
		if item.CommitHash == "" || item.ShortHash == "" || item.Author == "" ||
			item.Date == "" || item.Subject == "" || item.Content == "" || item.Preview == "" {
			t.Fatalf("note item is missing fields: %+v", item)
		}
		if !strings.HasPrefix(item.CommitHash, item.ShortHash) {
			t.Fatalf("short hash %q does not belong to %q", item.ShortHash, item.CommitHash)
		}
		if _, dup := byContent[item.Content]; dup {
			t.Fatalf("note content %q appears more than once", item.Content)
		}
		byContent[item.Content] = item
	}
	if len(byContent) != noteCount {
		t.Fatalf("got %d distinct contents, want %d", len(byContent), noteCount)
	}

	for n := 1; n <= noteCount; n++ {
		content := fmt.Sprintf("note %d", n)
		item, ok := byContent[content]
		if !ok {
			t.Fatalf("note %q is missing; contents: %v", content, noteContents(all.Notes))
		}
		if item.CommitHash != hashes[n] {
			t.Fatalf("%q belongs to commit %s, want %s", content, item.CommitHash, hashes[n])
		}
		if want := fmt.Sprintf("feat: 第 %d 次提交", n); item.Subject != want {
			t.Fatalf("%q paired with subject %q, want %q", content, item.Subject, want)
		}
		if item.Author != "Alice" {
			t.Fatalf("%q paired with author %q, want Alice", content, item.Author)
		}
		if item.Preview != content {
			t.Fatalf("%q paired with preview %q", content, item.Preview)
		}
	}

	// Distinct commit dates make the stable sort fully deterministic: newest
	// commit first. This mirrors the order the sequential implementation built
	// before sorting.
	for i := 1; i < len(all.Notes); i++ {
		if all.Notes[i-1].Date < all.Notes[i].Date {
			t.Fatalf("notes are not sorted newest first: %q before %q",
				all.Notes[i-1].Date, all.Notes[i].Date)
		}
	}

	limited, err := svc.GetCommitNotes(dir, 5)
	if err != nil {
		t.Fatalf("GetCommitNotes(limit=5) failed: %v", err)
	}
	if limited.Total != noteCount {
		t.Fatalf("limited Total = %d, want %d", limited.Total, noteCount)
	}
	if len(limited.Notes) != 5 {
		t.Fatalf("limited len(Notes) = %d, want 5", len(limited.Notes))
	}
}

func TestGetCommitNotesNoNotesRepo(t *testing.T) {
	dir := newNotesRepo(t)
	writeAndCommit(t, dir, "src/Base.cs", "class Base {}\n", "feat: 基础场景", time.Now(), "Alice")

	res, err := (GitService{}).GetCommitNotes(dir, 0)
	if err != nil {
		t.Fatalf("GetCommitNotes on a repo without notes failed: %v", err)
	}
	if res.Total != 0 || len(res.Notes) != 0 {
		t.Fatalf("expected no notes, got total=%d notes=%d", res.Total, len(res.Notes))
	}
}

func noteContents(items []CommitNoteItem) []string {
	contents := make([]string, len(items))
	for i, item := range items {
		contents[i] = item.Content
	}
	return contents
}
