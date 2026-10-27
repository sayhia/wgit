package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// commitEmptyOn adds an empty commit so a branch can carry a distinct ahead
// count without touching the working tree.
func commitEmptyOn(t *testing.T, dir, message string) {
	t.Helper()
	runGitTest(t, dir, nil, "commit", "--allow-empty", "-m", message)
}

// TestBranchInsightsParallelAheadBehind pins the observable contract of the
// concurrent ahead/behind collection introduced for GetBranchInsights: the
// returned item order (category rank, then branch name) and every counter must
// be exactly what the old sequential loop produced. Nine branches also exceed
// the eight-worker cap, so the queued path of the pool is exercised too.
func TestBranchInsightsParallelAheadBehind(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, dir, nil, "config", "commit.gpgsign", "false")

	writeAndCommit(t, dir, "src/Base.cs", "class Base {}\n", "feat: 基础场景", time.Now(), "Alice")

	// Snapshot main's first commit before main advances: this branch ends up
	// purely behind (and therefore merged) with no work of its own.
	runGitTest(t, dir, nil, "branch", "feature/behind", "main")
	commitEmptyOn(t, dir, "chore: main 前进 1")
	commitEmptyOn(t, dir, "chore: main 前进 2")

	// Zero-ahead branches sit on main's tip and count as merged.
	for _, name := range []string{"cleanup/alpha", "cleanup/mike", "cleanup/zeta"} {
		runGitTest(t, dir, nil, "branch", name, "main")
	}

	// Ahead-only branches: N empty commits on top of main.
	aheadPlan := []struct {
		name    string
		commits int
	}{
		{"feature/two-ahead", 2},
		{"feature/five-ahead", 5},
		{"feature/one-ahead", 1},
		{"feature/three-ahead", 3},
	}
	for _, plan := range aheadPlan {
		runGitTest(t, dir, nil, "checkout", "-b", plan.name, "main")
		for i := 1; i <= plan.commits; i++ {
			commitEmptyOn(t, dir, fmt.Sprintf("wip: %s 第 %d 次提交", plan.name, i))
		}
	}
	runGitTest(t, dir, nil, "checkout", "main")

	svc := GitService{}
	res, err := svc.GetBranchInsights(dir, "main")
	if err != nil {
		t.Fatalf("GetBranchInsights failed: %v", err)
	}
	if res.Base != "main" || res.Summary.BaseBranch != "main" {
		t.Fatalf("unexpected base: result=%q summary=%q", res.Base, res.Summary.BaseBranch)
	}

	// Category rank: current(0) < merged(4) < active(5); ties are broken by
	// branch name. This is the order the sequential implementation emitted.
	wantOrder := []string{
		"main",
		"cleanup/alpha",
		"cleanup/mike",
		"cleanup/zeta",
		"feature/behind",
		"feature/five-ahead",
		"feature/one-ahead",
		"feature/three-ahead",
		"feature/two-ahead",
	}
	gotOrder := make([]string, len(res.Items))
	byName := make(map[string]BranchInsight, len(res.Items))
	for i, it := range res.Items {
		gotOrder[i] = it.Name
		if it.StatusWarning != "" {
			t.Fatalf("branch %q unexpectedly carries a status warning: %s", it.Name, it.StatusWarning)
		}
		if _, dup := byName[it.Name]; dup {
			t.Fatalf("branch %q appears more than once: %v", it.Name, gotOrder)
		}
		byName[it.Name] = it
	}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Fatalf("branch order drifted from the sequential contract:\n got: %v\nwant: %v", gotOrder, wantOrder)
	}

	wantCounts := map[string]struct{ ahead, behind int }{
		"main":                {0, 0},
		"cleanup/alpha":       {0, 0},
		"cleanup/mike":        {0, 0},
		"cleanup/zeta":        {0, 0},
		"feature/behind":      {0, 2},
		"feature/one-ahead":   {1, 0},
		"feature/two-ahead":   {2, 0},
		"feature/three-ahead": {3, 0},
		"feature/five-ahead":  {5, 0},
	}
	for name, want := range wantCounts {
		it, ok := byName[name]
		if !ok {
			t.Fatalf("branch %q missing from insights", name)
		}
		if it.Ahead != want.ahead || it.Behind != want.behind {
			t.Fatalf("%s ahead/behind = %d/%d, want %d/%d", name, it.Ahead, it.Behind, want.ahead, want.behind)
		}
	}

	if mainItem := byName["main"]; !mainItem.IsCurrent || mainItem.Category != "current" {
		t.Fatalf("main should be the current branch: %+v", mainItem)
	}
	for _, name := range []string{"cleanup/alpha", "cleanup/mike", "cleanup/zeta", "feature/behind"} {
		if it := byName[name]; !it.Merged || it.Category != "merged" {
			t.Fatalf("%s should be classified merged: %+v", name, it)
		}
	}
	for _, plan := range aheadPlan {
		if it := byName[plan.name]; it.Merged || it.Category != "active" {
			t.Fatalf("%s should be active, not merged: %+v", plan.name, it)
		}
	}
	if twoAhead := byName["feature/two-ahead"]; twoAhead.LastSubject != "wip: feature/two-ahead 第 2 次提交" {
		t.Fatalf("metadata does not belong to feature/two-ahead: %+v", twoAhead)
	}

	sum := res.Summary
	if sum.Total != len(wantOrder) || sum.Current != "main" {
		t.Fatalf("summary totals wrong: %+v", sum)
	}
	if sum.Merged != 4 || sum.Cleanable != 4 {
		t.Fatalf("merged/cleanable counters wrong: %+v", sum)
	}
	if sum.Protected != 0 || sum.Unpushed != 0 || sum.Stale != 0 {
		t.Fatalf("unexpected category counters: %+v", sum)
	}
	if !strings.Contains(sum.Hint, "已合并") {
		t.Fatalf("unexpected hint: %s", sum.Hint)
	}
}

// TestBranchInsightsWarnsWhenAheadBehindFails pins the truthfulness contract:
// when `git rev-list` cannot answer for a branch, insights must not present the
// resulting 0/0 as "in sync". Every item keeps its slot and classification but
// carries a StatusWarning explaining that the counters are unknown.
func TestBranchInsightsWarnsWhenAheadBehindFails(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, dir, nil, "config", "commit.gpgsign", "false")

	writeAndCommit(t, dir, "src/Base.cs", "class Base {}\n", "feat: 基础场景", time.Now(), "Alice")

	// feature/behind stays on main's first commit and is already merged;
	// feature/ahead carries work of its own.
	runGitTest(t, dir, nil, "branch", "feature/behind", "main")
	commitEmptyOn(t, dir, "chore: main 前进 1")
	commitEmptyOn(t, dir, "chore: main 前进 2")
	runGitTest(t, dir, nil, "checkout", "-b", "feature/ahead", "main")
	commitEmptyOn(t, dir, "wip: feature/ahead 第 1 次提交")
	commitEmptyOn(t, dir, "wip: feature/ahead 第 2 次提交")
	runGitTest(t, dir, nil, "checkout", "main")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "rev-list" ]; then
  echo "simulated rev-list failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	res, err := (GitService{}).GetBranchInsights(dir, "main")
	if err != nil {
		t.Fatalf("a rev-list failure must not fail the whole insights request: %v", err)
	}
	if len(res.Items) != 3 || res.Summary.Total != 3 {
		t.Fatalf("all branches must still be listed: items=%d summary=%+v", len(res.Items), res.Summary)
	}

	byName := make(map[string]BranchInsight, len(res.Items))
	for _, it := range res.Items {
		byName[it.Name] = it
		if it.Ahead != 0 || it.Behind != 0 {
			t.Fatalf("%s must not fabricate counts after a read failure: %+v", it.Name, it)
		}
		if it.StatusWarning == "" {
			t.Fatalf("%s is missing the status warning for unknown ahead/behind", it.Name)
		}
		if !strings.Contains(it.StatusWarning, "读取领先/落后失败") {
			t.Fatalf("%s warning must explain the failed read: %q", it.Name, it.StatusWarning)
		}
		if !strings.Contains(it.StatusWarning, "simulated rev-list failure") {
			t.Fatalf("%s warning must keep the git error: %q", it.Name, it.StatusWarning)
		}
	}

	// Classification is derived independently and must survive the read failure.
	if behind := byName["feature/behind"]; !behind.Merged || behind.Category != "merged" {
		t.Fatalf("feature/behind should stay classified merged: %+v", behind)
	}
	if ahead := byName["feature/ahead"]; ahead.Merged || ahead.Category != "active" {
		t.Fatalf("feature/ahead should stay active: %+v", ahead)
	}
	if mainItem := byName["main"]; !mainItem.IsCurrent || mainItem.Category != "current" {
		t.Fatalf("main should stay current: %+v", mainItem)
	}
}
