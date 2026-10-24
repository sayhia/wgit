package main

import (
	"strings"
	"testing"
	"time"
)

func TestCommitNotes(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: 战斗结算", time.Now(), "Alice")
	hash := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	svc := GitService{}

	empty, err := svc.GetCommitNotes(dir, 10)
	if err != nil {
		t.Fatalf("GetCommitNotes on empty notes failed: %v", err)
	}
	if empty.Total != 0 || len(empty.Notes) != 0 {
		t.Fatalf("expected no notes, got %+v", empty)
	}

	if note, err := svc.GetCommitNote(dir, hash); err != nil || note != "" {
		t.Fatalf("expected empty note, got %q err=%v", note, err)
	}

	msg, err := svc.SaveCommitNote(dir, hash, "QA: BOSS 二阶段偶发卡顿\n复现步骤见 TAPD-1024")
	if err != nil {
		t.Fatalf("SaveCommitNote failed: %v", err)
	}
	t.Logf("saved: %s", msg)

	list, err := svc.GetCommitNotes(dir, 10)
	if err != nil {
		t.Fatalf("GetCommitNotes failed: %v", err)
	}
	if list.Total != 1 || len(list.Notes) != 1 {
		t.Fatalf("expected 1 note, got %+v", list)
	}
	item := list.Notes[0]
	if item.CommitHash != hash || !strings.Contains(item.Content, "TAPD-1024") {
		t.Fatalf("unexpected note item: %+v", item)
	}
	if item.Subject != "feat: 战斗结算" || item.ShortHash == "" || item.Preview == "" {
		t.Fatalf("note metadata missing: %+v", item)
	}

	hashes, err := svc.GetNotedCommitHashes(dir)
	if err != nil || len(hashes) != 1 || hashes[0] != hash {
		t.Fatalf("unexpected noted hashes: %v err=%v", hashes, err)
	}

	// Replace the note.
	if _, err := svc.SaveCommitNote(dir, hash, "策划: 数值已确认"); err != nil {
		t.Fatalf("replace note failed: %v", err)
	}
	updated, _ := svc.GetCommitNote(dir, hash)
	if updated != "策划: 数值已确认" {
		t.Fatalf("expected replaced note, got %q", updated)
	}

	if _, err := svc.SaveCommitNote(dir, hash, "   "); err == nil {
		t.Fatalf("expected empty note content to be rejected")
	}

	if _, err := svc.DeleteCommitNote(dir, hash); err != nil {
		t.Fatalf("DeleteCommitNote failed: %v", err)
	}
	final, _ := svc.GetCommitNotes(dir, 10)
	if final.Total != 0 {
		t.Fatalf("expected notes to be deleted, got %+v", final)
	}
	if _, err := svc.DeleteCommitNote(dir, hash); err == nil {
		t.Fatalf("expected deleting a missing note to fail")
	}
}

func TestReleasePlanAndCreate(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Core.cs", "core\n", "feat: 初始化战斗框架", time.Now().Add(-2*time.Hour), "Alice")
	runGitTest(t, dir, nil, "tag", "v1.0.0")
	writeAndCommit(t, dir, "src/Bag.cs", "bag\n", "feat(ui): 新增背包拖拽排序", time.Now(), "Alice")

	svc := GitService{}
	plan, err := svc.GetReleasePlan(dir)
	if err != nil {
		t.Fatalf("GetReleasePlan failed: %v", err)
	}
	if plan.LastTag != "v1.0.0" || plan.CommitCount != 1 {
		t.Fatalf("unexpected plan range: %+v", plan)
	}
	if plan.SuggestedVersion != "v1.1.0" || !plan.NextMinor {
		t.Fatalf("expected minor bump to v1.1.0, got %s (minor=%v)", plan.SuggestedVersion, plan.NextMinor)
	}
	if len(plan.Checks) != 5 {
		t.Fatalf("expected 5 checks, got %d", len(plan.Checks))
	}

	// Each check carries catalog keys so the desktop UI can render the same
	// sentence in the active language, plus the params that sentence needs.
	requiredParams := map[string][]string{
		"release.check.commits.detail.ok":      {"base", "count"},
		"release.check.workspace.detail.dirty": {"count"},
		"release.check.branch.detail.ok":       {"branch"},
		"release.check.branch.detail.other":    {"branch"},
		"release.check.remote.detail.behind":   {"upstream", "count"},
		"release.check.remote.detail.ahead":    {"upstream", "count"},
		"release.check.remote.detail.synced":   {"upstream"},
	}
	for _, check := range plan.Checks {
		if check.TitleKey == "" || check.DetailKey == "" {
			t.Fatalf("check %q is missing i18n keys: %+v", check.Key, check)
		}
		for _, name := range requiredParams[check.DetailKey] {
			if check.Params[name] == "" {
				t.Fatalf("check %q (%s) is missing param %q: %+v", check.Key, check.DetailKey, name, check.Params)
			}
		}
	}
	if plan.SummaryKey != "release.summary.ready" || plan.SummaryParams["version"] != "v1.1.0" {
		t.Fatalf("unexpected summary keys: %q %+v", plan.SummaryKey, plan.SummaryParams)
	}
	if !plan.Ready {
		t.Fatalf("expected plan to be ready: %+v", plan.Checks)
	}
	if !strings.Contains(plan.Changelog, "背包") {
		t.Fatalf("changelog should mention the new commit, got %q", plan.Changelog)
	}
	t.Logf("plan: %s", plan.Summary)

	res, err := svc.CreateRelease(dir, "v1.1.0", "首个可玩版本", false, "", false)
	if err != nil {
		t.Fatalf("CreateRelease failed: %v", err)
	}
	if !res.Success || res.Pushed {
		t.Fatalf("unexpected release result: %+v", res)
	}
	if tags := runGitTest(t, dir, nil, "tag", "-l", "v1.1.0"); strings.TrimSpace(tags) != "v1.1.0" {
		t.Fatalf("tag v1.1.0 was not created")
	}

	if _, err := svc.CreateRelease(dir, "v1.1.0", "", false, "", false); err == nil {
		t.Fatalf("expected duplicate tag creation to fail")
	}
	if _, err := svc.CreateRelease(dir, "bad version!", "", false, "", false); err == nil {
		t.Fatalf("expected invalid version to fail")
	}

	// A fix-only commit should suggest a patch bump.
	writeAndCommit(t, dir, "src/Bag.cs", "bag2\n", "fix: 修复背包拖拽丢失", time.Now(), "Alice")
	plan2, err := svc.GetReleasePlan(dir)
	if err != nil {
		t.Fatalf("second GetReleasePlan failed: %v", err)
	}
	if plan2.SuggestedVersion != "v1.1.1" || plan2.NextMinor {
		t.Fatalf("expected patch bump to v1.1.1, got %s", plan2.SuggestedVersion)
	}
}
