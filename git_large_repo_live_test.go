package main

// Live large-repository checks. These are skipped in normal `go test` runs and
// only execute when WGIT_LARGE_REPO points at a real, sizeable repository — this
// is how the large-project guarantees are verified against the user's actual repo
// (2302 commits / 276 branches at the time of writing):
//
//	WGIT_LARGE_REPO=/path/to/big/repo go test -run TestLiveLargeRepo -count=1 -v .
//
// The assertions are intentionally generous (time budgets in seconds) so the test
// is a regression guard against accidental O(files × git-process) blowups rather
// than a flaky micro-benchmark.

import (
	"os"
	"testing"
	"time"
)

func liveLargeRepo(t *testing.T) string {
	t.Helper()
	repo := os.Getenv("WGIT_LARGE_REPO")
	if repo == "" {
		t.Skip("set WGIT_LARGE_REPO to run live large-repository checks")
	}
	if !IsGitRepo(repo) {
		t.Fatalf("WGIT_LARGE_REPO=%q is not a git repository", repo)
	}
	return repo
}

func timedFetch(t *testing.T, name string, budget time.Duration, fn func() error) time.Duration {
	t.Helper()
	start := time.Now()
	if err := fn(); err != nil {
		t.Fatalf("%s failed: %v", name, err)
	}
	elapsed := time.Since(start)
	t.Logf("%-34s %8.1f ms", name, float64(elapsed.Microseconds())/1000.0)
	if elapsed > budget {
		t.Fatalf("%s exceeded budget %s (took %s)", name, budget, elapsed)
	}
	return elapsed
}

func TestLiveLargeRepoHistoryScale(t *testing.T) {
	repo := liveLargeRepo(t)
	svc := GitService{}

	var total int
	timedFetch(t, "GetCommitCount", 10*time.Second, func() error {
		n, err := svc.GetCommitCount(repo, "")
		total = n
		return err
	})
	if total < 500 {
		t.Fatalf("expected a large history (>500 commits), got %d", total)
	}

	// Growing-window history load, exactly how HistoryView pages commits. Lanes are
	// computed server-side, so each window must contain graph data.
	var window []CommitItem
	timedFetch(t, "GetCommits(limit=300)", 10*time.Second, func() error {
		list, err := svc.GetCommits(repo, 300, "")
		window = list
		return err
	})
	if len(window) != 300 {
		t.Fatalf("expected 300 commits in first window, got %d", len(window))
	}
	if len(window[0].Parents) == 0 {
		t.Fatalf("first commit unexpectedly has no parents")
	}
	if len(window[0].Lines) == 0 {
		t.Fatalf("first commit has no DAG graph lines; lane rendering would be empty")
	}

	// Server-side filters used by the new HistoryView filter bar.
	var filtered *CommitPaginationResult
	timedFetch(t, "GetCommitsFiltered(keyword)", 10*time.Second, func() error {
		res, err := svc.GetCommitsFiltered(repo, "", "", "", "", "fix", 0, 100)
		filtered = res
		return err
	})
	if filtered == nil || filtered.TotalCount == 0 || len(filtered.Commits) == 0 {
		t.Fatalf("keyword filter returned no commits: %+v", filtered)
	}
	if filtered.TotalCount < len(filtered.Commits) {
		t.Fatalf("filtered totalCount (%d) < page size (%d)", filtered.TotalCount, len(filtered.Commits))
	}
	if filtered.HasMore && filtered.TotalCount <= len(filtered.Commits) {
		t.Fatalf("hasMore=true but totalCount %d == page size %d", filtered.TotalCount, len(filtered.Commits))
	}

	var paged *CommitPaginationResult
	timedFetch(t, "GetCommitsFiltered(skip=200)", 10*time.Second, func() error {
		res, err := svc.GetCommitsFiltered(repo, "", "", "", "", "", 200, 100)
		paged = res
		return err
	})
	if paged == nil || len(paged.Commits) != 100 {
		t.Fatalf("paged history did not return 100 rows: %+v", paged)
	}
	if paged.Commits[0].Hash == window[0].Hash {
		t.Fatalf("skip pagination returned the first commit again")
	}
}

func TestLiveLargeRepoBranchScale(t *testing.T) {
	repo := liveLargeRepo(t)
	svc := GitService{}

	var branches []BranchInfo
	timedFetch(t, "GetBranches", 10*time.Second, func() error {
		list, err := svc.GetBranches(repo)
		branches = list
		return err
	})
	if len(branches) < 20 {
		t.Fatalf("expected a branch-heavy repository (>=20 branches), got %d", len(branches))
	}

	var insights *BranchInsightsResult
	timedFetch(t, "GetBranchInsights", 20*time.Second, func() error {
		res, err := svc.GetBranchInsights(repo, "")
		insights = res
		return err
	})
	if insights == nil || len(insights.Items) == 0 {
		t.Fatalf("branch insights returned no rows")
	}
	if insights.Summary.Total != len(insights.Items) {
		t.Fatalf("summary total %d does not match %d items", insights.Summary.Total, len(insights.Items))
	}
	// Categories must be one of the documented values; frontend renders badges by it.
	valid := map[string]bool{"current": true, "merged": true, "unpushed": true, "stale": true, "active": true}
	for _, it := range insights.Items {
		if !valid[it.Category] {
			t.Fatalf("unexpected branch category %q for %q", it.Category, it.Name)
		}
	}
}

func TestLiveLargeRepoFileExplorerAndSearch(t *testing.T) {
	repo := liveLargeRepo(t)
	svc := GitService{}

	var files []ProjectSourceFileItem
	timedFetch(t, "GetProjectSourceFiles", 15*time.Second, func() error {
		list, err := svc.GetProjectSourceFiles(repo)
		files = list
		return err
	})
	if len(files) < 100 {
		t.Fatalf("expected at least 100 tracked files, got %d", len(files))
	}
	// Paths must be raw UTF-8 (no git octal escaping) and sizes collected.
	sized := 0
	for _, f := range files {
		if f.Size > 0 {
			sized++
		}
	}
	if sized < len(files)/2 {
		t.Fatalf("only %d/%d files have sizes; parallel stat may have failed", sized, len(files))
	}

	var matches []ProjectFileMatch
	timedFetch(t, "SearchProjectFiles", 10*time.Second, func() error {
		list, err := svc.SearchProjectFiles(repo, "test", 50)
		matches = list
		return err
	})
	if len(matches) == 0 {
		t.Fatalf("expected file search matches for 'test'")
	}
	if len(matches) > 50 {
		t.Fatalf("SearchProjectFiles ignored the limit: %d", len(matches))
	}

	var code []GrepMatchItem
	timedFetch(t, "SearchRepoCode", 20*time.Second, func() error {
		list, err := svc.SearchRepoCode(repo, "def ", true, 100)
		code = list
		return err
	})
	if len(code) == 0 {
		t.Logf("no python 'def ' matches in this repo — acceptable")
	}
	for _, m := range code {
		if m.Path == "" || m.LineNumber <= 0 {
			t.Fatalf("malformed code search match: %+v", m)
		}
	}
}
