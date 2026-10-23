package main

import (
	"strings"
	"testing"
	"time"
)

func TestBranchInsights(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: 战斗结算", time.Now(), "Alice")
	writeAndCommit(t, dir, "src/Hud.cs", "class Hud {}\n", "feat: HUD", time.Now(), "Alice")

	svc := GitService{}

	// a branch that will be merged into main
	runGitTest(t, dir, nil, "checkout", "-b", "feature/merged-one")
	writeAndCommit(t, dir, "src/Loot.cs", "class Loot {}\n", "feat: 掉落", time.Now(), "Bob")
	runGitTest(t, dir, nil, "checkout", "main")

	// a branch that keeps unmerged work
	runGitTest(t, dir, nil, "checkout", "-b", "feature/wip-one")
	writeAndCommit(t, dir, "src/Ai.cs", "class Ai {}\n", "wip: 行为树", time.Now(), "Bob")
	runGitTest(t, dir, nil, "checkout", "main")

	// before the merge nothing is merged yet
	pre, err := svc.GetBranchInsights(dir, "main")
	if err != nil {
		t.Fatalf("GetBranchInsights failed: %v", err)
	}
	if pre.Base != "main" || len(pre.Items) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(pre.Items))
	}
	if pre.Summary.Merged != 0 {
		t.Fatalf("nothing should be merged yet: %+v", pre.Summary)
	}
	for _, it := range pre.Items {
		if it.Name == "main" && !it.IsCurrent {
			t.Fatalf("main should be current: %+v", it)
		}
		if it.Name == "feature/wip-one" && it.Behind != 0 {
			t.Fatalf("wip branch should not be behind: %+v", it)
		}
	}

	runGitTest(t, dir, nil, "merge", "--no-ff", "-m", "merge: 掉落系统", "feature/merged-one")
	runGitTest(t, dir, nil, "branch", "release/1.0", "main")

	res, err := svc.GetBranchInsights(dir, "main")
	if err != nil {
		t.Fatalf("GetBranchInsights after merge failed: %v", err)
	}
	byName := map[string]BranchInsight{}
	for _, it := range res.Items {
		byName[it.Name] = it
	}
	mergedOne := byName["feature/merged-one"]
	if !mergedOne.Merged || mergedOne.Category != "merged" {
		t.Fatalf("merged branch not detected: %+v", mergedOne)
	}
	if mergedOne.Ahead != 0 {
		t.Fatalf("a merged branch must not be ahead: %+v", mergedOne)
	}
	wip := byName["feature/wip-one"]
	if wip.Merged || wip.Category == "merged" {
		t.Fatalf("wip branch must not be reported merged: %+v", wip)
	}
	if wip.Ahead != 1 {
		t.Fatalf("wip branch should be 1 ahead: %+v", wip)
	}
	if wip.LastSubject != "wip: 行为树" || wip.LastAuthor == "" {
		t.Fatalf("last commit metadata missing: %+v", wip)
	}
	if res.Summary.Merged != 1 || res.Summary.Cleanable != 1 {
		t.Fatalf("summary counters wrong: %+v", res.Summary)
	}
	release := byName["release/1.0"]
	if !release.Protected || release.Category != "protected" || !release.Merged {
		t.Fatalf("long-lived release branch must be protected even when merged: %+v", release)
	}
	if !strings.Contains(release.Suggestion, "保留") {
		t.Fatalf("protected branch suggestion should say to keep it: %q", release.Suggestion)
	}
	if res.Summary.Protected != 1 {
		t.Fatalf("protected branch should be counted separately: %+v", res.Summary)
	}
	mergedForCleanup, err := svc.GetMergedBranches(dir, "main")
	if err != nil {
		t.Fatalf("GetMergedBranches failed: %v", err)
	}
	for _, name := range mergedForCleanup {
		if name == "release/1.0" {
			t.Fatalf("protected release branch must not be listed for cleanup: %+v", mergedForCleanup)
		}
	}
	if !strings.Contains(res.Summary.Hint, "已合并") {
		t.Fatalf("unexpected hint: %s", res.Summary.Hint)
	}
	if res.Summary.Current != "main" {
		t.Fatalf("unexpected current branch: %q", res.Summary.Current)
	}

	// an unknown base must fail loudly instead of silently reporting zero
	if _, err := svc.GetBranchInsights(dir, "definitely-not-a-branch"); err == nil {
		t.Fatalf("expected error for an unknown base branch")
	}
}
