package main

// Stress checks for very large flat monorepos (>=100k files). Skipped by default;
// enable by pointing WGIT_STRESS_REPO at a generated fixture, e.g.:
//
//	WGIT_STRESS_REPO=/tmp/wgit-stress-repo go test -run TestStressLargeMonorepo -count=1 -v .
//
// The fixture can be generated with the script documented in the README section
// "测试与验证". These checks are regression guards against accidental
// O(files × git-process) patterns, not micro-benchmarks.

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func stressRepo(t *testing.T) string {
	t.Helper()
	repo := os.Getenv("WGIT_STRESS_REPO")
	if repo == "" {
		t.Skip("set WGIT_STRESS_REPO to run the 100k-file monorepo stress checks")
	}
	if !IsGitRepo(repo) {
		t.Fatalf("WGIT_STRESS_REPO=%q is not a git repository", repo)
	}
	return repo
}

func TestStressLargeMonorepo(t *testing.T) {
	repo := stressRepo(t)
	svc := GitService{}

	// 1. Full source listing: git ls-files -z + parallel stat.
	var files []ProjectSourceFileItem
	start := time.Now()
	files, err := svc.GetProjectSourceFiles(repo)
	if err != nil {
		t.Fatalf("GetProjectSourceFiles failed: %v", err)
	}
	listDuration := time.Since(start)
	if len(files) < 100_000 {
		t.Fatalf("expected >=100k files, got %d", len(files))
	}
	t.Logf("GetProjectSourceFiles: %d files in %s (%.1f µs/file)",
		len(files), listDuration, float64(listDuration.Microseconds())/float64(len(files)))
	if listDuration > 15*time.Second {
		t.Fatalf("source listing too slow for 100k files: %s", listDuration)
	}

	// 2. RPC payload cost: the webview receives this JSON. Keep an eye on it.
	start = time.Now()
	payload, err := json.Marshal(files)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	marshalDuration := time.Since(start)
	t.Logf("source list JSON payload: %.1f MB, marshal %s", float64(len(payload))/(1024*1024), marshalDuration)
	if len(payload) > 32*1024*1024 {
		t.Fatalf("source list payload unexpectedly large: %.1f MB", float64(len(payload))/(1024*1024))
	}

	// 3. Server-side fuzzy file search (⌘P) must never ship the full list.
	start = time.Now()
	matches, err := svc.SearchProjectFiles(repo, "file_0012", 80)
	if err != nil {
		t.Fatalf("SearchProjectFiles failed: %v", err)
	}
	searchDuration := time.Since(start)
	if len(matches) == 0 || len(matches) > 80 {
		t.Fatalf("unexpected match count %d", len(matches))
	}
	t.Logf("SearchProjectFiles(file_0012): %d matches in %s", len(matches), searchDuration)
	if searchDuration > 5*time.Second {
		t.Fatalf("server-side file search too slow: %s", searchDuration)
	}

	// 4. Multi-token query over 100k paths.
	start = time.Now()
	multi, err := svc.SearchProjectFiles(repo, "pkg07 module_005", 20)
	if err != nil {
		t.Fatalf("multi-token search failed: %v", err)
	}
	t.Logf("SearchProjectFiles(pkg07 module_005): %d matches in %s", len(multi), time.Since(start))

	// 5. Working tree status: clean 100k-file tree should be fast (no per-file diff).
	start = time.Now()
	status, err := svc.GetRepoStatus(repo)
	if err != nil {
		t.Fatalf("GetRepoStatus failed: %v", err)
	}
	statusDuration := time.Since(start)
	t.Logf("GetRepoStatus: clean=%v in %s", status.IsClean, statusDuration)
	if statusDuration > 10*time.Second {
		t.Fatalf("status too slow on large clean tree: %s", statusDuration)
	}
}
