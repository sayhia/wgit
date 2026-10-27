package main

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

// TestScanHistoryLargeBlobs pins the observable contract of the concurrent
// metadata lookup in ScanHistoryLargeBlobs: the result order still follows the
// size-descending blob order, every slot carries its own commit metadata (never
// a neighbour's), and the limit truncates to the largest entries.
func TestScanHistoryLargeBlobs(t *testing.T) {
	// Isolate git configuration so the fixture never inherits a user's global
	// aliases, signing or log settings.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, repo, nil, "config", "commit.gpgsign", "false")

	const fileCount = 12
	type createdBlob struct {
		path    string
		size    int64
		subject string
		date    string
	}

	created := make([]createdBlob, 0, fileCount)
	baseTime := time.Now().Add(-time.Duration(fileCount) * time.Hour).Truncate(time.Second)
	for i := 0; i < fileCount; i++ {
		size := int64(1024 * (i + 1))
		buf := make([]byte, size)
		// Deterministic pseudo-random bytes keep every file a distinct binary
		// blob, so the object enumeration sees exactly one blob per commit.
		if _, err := rand.New(rand.NewSource(int64(i) + 1)).Read(buf); err != nil {
			t.Fatalf("fill blob %d: %v", i, err)
		}
		when := baseTime.Add(time.Duration(i) * time.Hour)
		path := fmt.Sprintf("assets/blob_%02d.bin", i+1)
		subject := fmt.Sprintf("feat: add blob %02d", i+1)
		writeAndCommit(t, repo, path, string(buf), subject, when, "Alice")
		created = append(created, createdBlob{
			path:    path,
			size:    size,
			subject: subject,
			date:    when.Format("2006-01-02"),
		})
	}

	svc := GitService{}

	items, err := svc.ScanHistoryLargeBlobs(repo, 20)
	if err != nil {
		t.Fatalf("ScanHistoryLargeBlobs failed: %v", err)
	}
	if len(items) != fileCount {
		t.Fatalf("blob count = %d, want %d: %+v", len(items), fileCount, items)
	}

	expected := make(map[string]createdBlob, fileCount)
	for _, c := range created {
		expected[c.path] = c
	}

	for i, it := range items {
		if i > 0 && items[i-1].Size < it.Size {
			t.Fatalf("sizes are not non-increasing at index %d: %d then %d", i, items[i-1].Size, it.Size)
		}
		want, ok := expected[it.Path]
		if !ok {
			t.Fatalf("unexpected or duplicated path %q in results: %+v", it.Path, items)
		}
		delete(expected, it.Path)
		if it.Size != want.size {
			t.Fatalf("%s size = %d, want %d", it.Path, it.Size, want.size)
		}
		if it.SizeStr != formatByteSize(want.size) {
			t.Fatalf("%s size string = %q, want %q", it.Path, it.SizeStr, formatByteSize(want.size))
		}
		if it.OID == "" {
			t.Fatalf("%s is missing its OID", it.Path)
		}
		// Every created path is reachable from a commit, so the metadata must
		// be resolved and must belong to that path, not to another worker.
		if it.CommitHash == "" || it.CommitDate == "" || it.Author == "" || it.Subject == "" {
			t.Fatalf("%s is missing commit metadata: %+v", it.Path, it)
		}
		if it.Subject != want.subject {
			t.Fatalf("%s subject = %q, want %q", it.Path, it.Subject, want.subject)
		}
		if it.Author != "Alice" {
			t.Fatalf("%s author = %q, want Alice", it.Path, it.Author)
		}
		if it.CommitDate != want.date {
			t.Fatalf("%s commit date = %q, want %q", it.Path, it.CommitDate, want.date)
		}
	}
	if len(expected) != 0 {
		t.Fatalf("created blobs missing from results: %+v", expected)
	}

	limited, err := svc.ScanHistoryLargeBlobs(repo, 5)
	if err != nil {
		t.Fatalf("ScanHistoryLargeBlobs(limit=5) failed: %v", err)
	}
	if len(limited) != 5 {
		t.Fatalf("limited blob count = %d, want 5", len(limited))
	}
	for i, it := range limited {
		want := items[i]
		if it.Path != want.Path || it.Size != want.Size || it.OID != want.OID {
			t.Fatalf("limited item %d = %+v, want the %dth largest blob %+v", i, it, i, want)
		}
	}
}
