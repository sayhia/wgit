package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// newBatchErrorHashes returns the commit hashes of a fixture repository in
// oldest-first (input) order, matching the convention of
// newCommitDetailsRepo's callers.
func newBatchErrorHashes(t *testing.T, dir string) []string {
	t.Helper()
	raw, err := ExecGit(dir, "log", "--format=%H")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	newestFirst := strings.Fields(raw)
	oldestFirst := make([]string, len(newestFirst))
	for i, hash := range newestFirst {
		oldestFirst[len(newestFirst)-1-i] = hash
	}
	return oldestFirst
}

// TestGetCommitDetailsBatchWithStatusReportsFailures covers the status-aware
// bulk lookup: valid hashes resolve in input order while broken ones come back
// in Failed together with a reason, instead of disappearing silently.
func TestGetCommitDetailsBatchWithStatusReportsFailures(t *testing.T) {
	svc := GitService{}
	dir, _ := newCommitDetailsRepo(t, svc, 3)
	hashes := newBatchErrorHashes(t, dir)
	bogus := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	tests := []struct {
		name       string
		input      []string
		wantItems  []string
		wantFailed []string
	}{
		{
			name:       "all valid hashes resolve with empty Failed",
			input:      append([]string(nil), hashes...),
			wantItems:  hashes,
			wantFailed: nil,
		},
		{
			name:       "unknown hash is reported alongside resolved items",
			input:      []string{hashes[0], bogus, hashes[2]},
			wantItems:  []string{hashes[0], hashes[2]},
			wantFailed: []string{bogus},
		},
		{
			name:       "unknown short name is reported with a reason",
			input:      []string{"nosuchref", hashes[1]},
			wantItems:  []string{hashes[1]},
			wantFailed: []string{"nosuchref"},
		},
		{
			name:       "blank entries are skipped without being reported",
			input:      []string{hashes[0], "   ", hashes[2]},
			wantItems:  []string{hashes[0], hashes[2]},
			wantFailed: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := svc.GetCommitDetailsBatchWithStatus(dir, tt.input)
			if err != nil {
				t.Fatalf("GetCommitDetailsBatchWithStatus returned error: %v", err)
			}
			if res == nil {
				t.Fatal("GetCommitDetailsBatchWithStatus returned nil result")
			}
			gotItems := make([]string, 0, len(res.Items))
			for _, item := range res.Items {
				gotItems = append(gotItems, item.Hash)
			}
			if !reflect.DeepEqual(gotItems, tt.wantItems) {
				t.Fatalf("Items hashes = %v, want %v", gotItems, tt.wantItems)
			}
			if tt.wantFailed == nil {
				if len(res.Failed) != 0 {
					t.Fatalf("Failed = %+v, want empty", res.Failed)
				}
				return
			}
			gotFailed := make([]string, 0, len(res.Failed))
			for _, f := range res.Failed {
				gotFailed = append(gotFailed, f.Hash)
				if strings.TrimSpace(f.Reason) == "" {
					t.Fatalf("Failed entry for hash %s has empty Reason: %+v", f.Hash, f)
				}
			}
			if !reflect.DeepEqual(gotFailed, tt.wantFailed) {
				t.Fatalf("Failed hashes = %v, want %v (full: %+v)", gotFailed, tt.wantFailed, res.Failed)
			}
		})
	}
}

// TestGetCommitDetailsBatchDropsFailedHashes pins the legacy wrapper contract:
// callers that only want items keep receiving exactly the resolved ones with a
// nil error, even when other hashes in the batch fail.
func TestGetCommitDetailsBatchDropsFailedHashes(t *testing.T) {
	svc := GitService{}
	dir, _ := newCommitDetailsRepo(t, svc, 2)
	hashes := newBatchErrorHashes(t, dir)

	items, err := svc.GetCommitDetailsBatch(dir, []string{hashes[0], "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "", hashes[1]})
	if err != nil {
		t.Fatalf("GetCommitDetailsBatch returned error: %v", err)
	}
	if len(items) != 2 || items[0].Hash != hashes[0] || items[1].Hash != hashes[1] {
		t.Fatalf("legacy batch must return only resolved items in order, got %+v (err=%v)", items, err)
	}
}

// TestGetWorkingDiffAdvancedUntrackedNoIndexFailure covers the untracked-file
// fallback of GetWorkingDiffAdvanced. `git diff --no-index` exits 1 whenever a
// difference exists, so a readable untracked file must keep returning its real
// patch even though ExecGit reports an error, while a no-index run that fails
// outright (unreadable path) must surface a placeholder line instead of a
// silently empty diff.
func TestGetWorkingDiffAdvancedUntrackedNoIndexFailure(t *testing.T) {
	svc := GitService{}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	writeAndCommit(t, repo, "tracked.txt", "tracked\n", "chore: base", time.Now(), "Tester")

	fresh := filepath.Join(repo, "fresh.txt")
	if err := os.WriteFile(fresh, []byte("new content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	locked := filepath.Join(repo, "locked.txt")
	if err := os.WriteFile(locked, []byte("hidden\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })

	tests := []struct {
		name           string
		file           string
		skipIfReadable bool   // platforms able to read 0o000 files cannot exercise the failure path
		wantPrefix     string // expected prefix of the returned diff text
	}{
		{
			name:       "readable untracked file keeps its patch",
			file:       "fresh.txt",
			wantPrefix: "diff --git a/fresh.txt",
		},
		{
			name:           "unreadable untracked file yields placeholder instead of empty diff",
			file:           "locked.txt",
			skipIfReadable: true,
			wantPrefix:     "(unable to generate diff for untracked file:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipIfReadable {
				// Guard: on such platforms no-index succeeds and the failure
				// scenario cannot be constructed at all.
				if out, niErr := ExecGit(repo, "diff", "--no-index", "/dev/null", tt.file); niErr == nil || strings.TrimSpace(out) != "" {
					t.Skipf("unreadable-file precondition not met on this platform (out=%q err=%v)", out, niErr)
				}
			}
			got, err := svc.GetWorkingDiffAdvanced(repo, tt.file, false, false)
			if err != nil {
				t.Fatalf("GetWorkingDiffAdvanced(%s) returned error: %v", tt.file, err)
			}
			if strings.TrimSpace(got) == "" {
				t.Fatalf("GetWorkingDiffAdvanced(%s) returned an empty diff, want prefix %q", tt.file, tt.wantPrefix)
			}
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Fatalf("GetWorkingDiffAdvanced(%s) = %q, want prefix %q", tt.file, got, tt.wantPrefix)
			}
		})
	}
}
