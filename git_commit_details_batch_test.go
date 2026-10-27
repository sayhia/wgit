package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newCommitDetailsRepo builds a throwaway repository with n commits whose
// messages and author dates are all distinct, so a batch that cross-wires
// results between hashes cannot pass unnoticed.
func newCommitDetailsRepo(t *testing.T, svc GitService, n int) (string, []string) {
	t.Helper()

	// Keep the fixture isolated from whatever git config the developer has.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	dir := t.TempDir()
	if _, err := svc.InitRepo(dir); err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	for _, args := range [][]string{
		{"config", "user.name", "Batch Tester"},
		{"config", "user.email", "batch-tester@example.com"},
	} {
		if out, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	subjects := make([]string, 0, n)
	base := time.Date(2024, time.March, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		stamp := base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339)
		t.Setenv("GIT_AUTHOR_DATE", stamp)
		t.Setenv("GIT_COMMITTER_DATE", stamp)

		subject := fmt.Sprintf("batch commit %02d", i)
		subjects = append(subjects, subject)

		file := filepath.Join(dir, "log.txt")
		if err := os.WriteFile(file, []byte(subject+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
		if out, err := ExecGit(dir, "add", "-A"); err != nil {
			t.Fatalf("git add (%s): %v", out, err)
		}
		if out, err := ExecGit(dir, "commit", "-m", subject); err != nil {
			t.Fatalf("git commit %q: %v (%s)", subject, err, out)
		}
	}

	raw, err := ExecGit(dir, "log", "--format=%H")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	hashes := strings.Fields(raw)
	if len(hashes) != n {
		t.Fatalf("expected %d hashes, got %d", n, len(hashes))
	}

	// git log is newest-first; reverse so input and assertion order agree.
	for i, j := 0, len(hashes)-1; i < j; i, j = i+1, j-1 {
		hashes[i], hashes[j] = hashes[j], hashes[i]
	}
	return dir, subjects
}

// TestGetCommitDetailsBatchPreservesOrderAndSkipsInvalid covers the parallel
// rewrite of GetCommitDetailsBatch: results must keep the caller's order even
// though the git calls run concurrently, and empty/bogus hashes must still be
// silently skipped (the function reports no error for them).
func TestGetCommitDetailsBatchPreservesOrderAndSkipsInvalid(t *testing.T) {
	svc := GitService{}
	const commitCount = 12
	dir, subjects := newCommitDetailsRepo(t, svc, commitCount)

	raw, err := ExecGit(dir, "log", "--format=%H")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	newestFirst := strings.Fields(raw)
	valid := make([]string, len(newestFirst))
	for i, hash := range newestFirst {
		valid[len(newestFirst)-1-i] = hash
	}

	// Interleave a blank and an unknown hash so skipping happens mid-batch.
	bogus := "0123456789abcdef0123456789abcdef01234567"
	input := make([]string, 0, len(valid)+2)
	input = append(input, valid[:6]...)
	input = append(input, "", "   ", bogus)
	input = append(input, valid[6:]...)

	items, err := svc.GetCommitDetailsBatch(dir, input)
	if err != nil {
		t.Fatalf("GetCommitDetailsBatch returned error: %v", err)
	}
	if len(items) != commitCount {
		t.Fatalf("expected %d items, got %d: %+v", commitCount, len(items), items)
	}

	subjectByHash := make(map[string]string, commitCount)
	for i, hash := range valid {
		subjectByHash[hash] = subjects[i]
	}

	for i, item := range items {
		if item.Hash != valid[i] {
			t.Fatalf("items[%d].Hash = %q, want %q (input order must be preserved)", i, item.Hash, valid[i])
		}
		if item.Subject != subjectByHash[valid[i]] {
			t.Fatalf("items[%d].Subject = %q, want %q for hash %s (results cross-wired)", i, item.Subject, subjectByHash[valid[i]], valid[i])
		}
		for field, value := range map[string]string{
			"Hash":        item.Hash,
			"ShortHash":   item.ShortHash,
			"AuthorName":  item.AuthorName,
			"AuthorEmail": item.AuthorEmail,
			"Date":        item.Date,
			"Subject":     item.Subject,
		} {
			if strings.TrimSpace(value) == "" {
				t.Fatalf("items[%d].%s is empty: %+v", i, field, item)
			}
		}
		if !strings.HasPrefix(item.Hash, item.ShortHash) {
			t.Fatalf("items[%d].ShortHash = %q is not a prefix of %q", i, item.ShortHash, item.Hash)
		}
		if item.AuthorName != "Batch Tester" || item.AuthorEmail != "batch-tester@example.com" {
			t.Fatalf("items[%d] author = %q <%s>, want Batch Tester <batch-tester@example.com>", i, item.AuthorName, item.AuthorEmail)
		}
	}
}

// TestGetCommitDetailsBatchEmptyInput pins the zero-input behaviour: no items
// and no error.
func TestGetCommitDetailsBatchEmptyInput(t *testing.T) {
	svc := GitService{}
	dir, _ := newCommitDetailsRepo(t, svc, 4)

	items, err := svc.GetCommitDetailsBatch(dir, nil)
	if err != nil {
		t.Fatalf("GetCommitDetailsBatch(nil) returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected zero items for empty input, got %d: %+v", len(items), items)
	}

	items, err = svc.GetCommitDetailsBatch(dir, []string{})
	if err != nil {
		t.Fatalf("GetCommitDetailsBatch(empty) returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected zero items for empty input, got %d: %+v", len(items), items)
	}
}
