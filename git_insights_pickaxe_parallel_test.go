package main

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPickaxeSearchParallelPreservesOrder drives PickaxeSearch with more hits
// than the bounded worker pool (pickaxeShowWorkers = 8), so the pool must be
// reused across waves. It pins the exact hit order, per-hit expansion and the
// aggregate counters to the sequential contract.
func TestPickaxeSearchParallelPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	const totalCommits = 12 // > pickaxeShowWorkers, forces pool reuse
	// Commits are dated in creation order (HEAD ends up newest), matching how
	// git log walks the parent chain.
	base := time.Now().AddDate(0, 0, -totalCommits)
	content := ""
	for k := 0; k < totalCommits; k++ {
		content += "var v" + string(rune('A'+k)) + " = \"PickaxeNeedle\";\n"
		writeAndCommit(t, dir, "src/Needle.cs", content,
			"feat: needle commit "+strconv.Itoa(k), base.Add(time.Duration(k)*time.Hour), "Alice")
	}

	out := runGitTest(t, dir, nil, "rev-list", "HEAD")
	expected := strings.Fields(out) // newest first, matching git log order
	if len(expected) != totalCommits {
		t.Fatalf("setup error: expected %d commits, got %d", totalCommits, len(expected))
	}

	svc := GitService{}
	res, err := svc.PickaxeSearch(dir, "PickaxeNeedle", "content", "", "", 20, false)
	if err != nil {
		t.Fatalf("PickaxeSearch failed: %v", err)
	}
	if res.TotalHits != totalCommits || res.AnalyzedHits != totalCommits || len(res.Hits) != totalCommits {
		t.Fatalf("expected %d hits (total/analyzed/shown), got %d/%d/%d",
			totalCommits, res.TotalHits, res.AnalyzedHits, len(res.Hits))
	}
	for i, hit := range res.Hits {
		if hit.Hash != expected[i] {
			t.Fatalf("hit %d out of order: got %s, want %s", i, hit.Hash, expected[i])
		}
		if hit.Change != "新增" {
			t.Fatalf("hit %d (%s): expected 新增, got %s", i, hit.Hash, hit.Change)
		}
		if hit.Added != 1 || hit.Removed != 0 {
			t.Fatalf("hit %d (%s): expected +1/-0, got +%d/-%d", i, hit.Hash, hit.Added, hit.Removed)
		}
		if len(hit.Files) != 1 || hit.Files[0] != "src/Needle.cs" {
			t.Fatalf("hit %d (%s): expected files [src/Needle.cs], got %v", i, hit.Hash, hit.Files)
		}
	}
	if res.AddedLines != totalCommits || res.RemovedLines != 0 {
		t.Fatalf("expected totals +%d/-0, got +%d/-%d", totalCommits, res.AddedLines, res.RemovedLines)
	}
	if res.FileCount != 1 {
		t.Fatalf("expected 1 touched file, got %d", res.FileCount)
	}
	if res.LastSeen == "" || res.FirstSeen == "" {
		t.Fatalf("expected a non-empty seen range, got %s .. %s", res.FirstSeen, res.LastSeen)
	}
}

// TestPickaxeSearchLimitKeepsSeenRange pins the subtle truncation contract:
// AnalyzedHits and Hits stop at the limit, but FirstSeen/LastSeen are still
// derived from the full hit list, not the truncated one.
func TestPickaxeSearchLimitKeepsSeenRange(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	content := ""
	for k := 0; k < 5; k++ {
		content += "var v" + string(rune('A'+k)) + " = \"PickaxeNeedle\";\n"
		writeAndCommit(t, dir, "src/Needle.cs", content,
			"feat: needle commit "+strconv.Itoa(k), time.Now().AddDate(0, 0, -(4-k)), "Alice")
	}

	svc := GitService{}
	res, err := svc.PickaxeSearch(dir, "PickaxeNeedle", "content", "", "", 3, false)
	if err != nil {
		t.Fatalf("PickaxeSearch failed: %v", err)
	}
	if res.TotalHits != 5 {
		t.Fatalf("expected 5 total hits, got %d", res.TotalHits)
	}
	if res.AnalyzedHits != 3 || len(res.Hits) != 3 {
		t.Fatalf("expected limit to cap analyzed/shown at 3, got %d/%d", res.AnalyzedHits, len(res.Hits))
	}
	wantOldest := time.Now().AddDate(0, 0, -4).Format("2006-01-02")
	wantNewest := time.Now().Format("2006-01-02")
	if res.FirstSeen != wantOldest || res.LastSeen != wantNewest {
		t.Fatalf("seen range must come from the full list: got %s .. %s, want %s .. %s",
			res.FirstSeen, res.LastSeen, wantOldest, wantNewest)
	}
}
