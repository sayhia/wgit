package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runGitTest(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=Alice",
		"GIT_AUTHOR_EMAIL=alice@studio.dev",
		"GIT_COMMITTER_NAME=Alice",
		"GIT_COMMITTER_EMAIL=alice@studio.dev",
	)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func writeAndCommit(t *testing.T, dir, relPath, content, message string, when time.Time, author string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, nil, "add", "-A")
	date := when.Format(time.RFC3339)
	runGitTest(t, dir, []string{
		"GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_DATE=" + date,
		"GIT_AUTHOR_NAME=" + author,
		"GIT_AUTHOR_EMAIL=" + strings.ToLower(strings.ReplaceAll(author, " ", ".")) + "@studio.dev",
		"GIT_COMMITTER_NAME=" + author,
		"GIT_COMMITTER_EMAIL=" + strings.ToLower(strings.ReplaceAll(author, " ", ".")) + "@studio.dev",
	}, "commit", "-m", message)
}

func TestTeamActivityHeatmap(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	now := time.Now()
	writeAndCommit(t, dir, "Assets/Hero.prefab", "v1\n", "art: 新增英雄预制体", now.AddDate(0, 0, -1), "Alice")
	writeAndCommit(t, dir, "Assets/Hero.prefab", "v2\nv3\n", "art: 调整英雄材质", now, "Alice")
	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: 战斗结算", now, "Bob")
	// Outside of the 30-day window.
	writeAndCommit(t, dir, "src/Old.cs", "old\n", "chore: 历史提交", now.AddDate(0, 0, -80), "Bob")

	svc := GitService{}
	res, err := svc.GetTeamActivity(dir, 30)
	if err != nil {
		t.Fatalf("GetTeamActivity failed: %v", err)
	}
	if len(res.Days) != 30 {
		t.Fatalf("expected 30 day buckets, got %d", len(res.Days))
	}
	if res.TotalCommits != 3 {
		t.Fatalf("expected 3 commits in window, got %d", res.TotalCommits)
	}
	if res.ActiveDays != 2 {
		t.Fatalf("expected 2 active days, got %d", res.ActiveDays)
	}
	if len(res.Contributors) != 2 {
		t.Fatalf("expected 2 contributors, got %d", len(res.Contributors))
	}
	if res.Contributors[0].Commits != 2 {
		t.Fatalf("expected top contributor to have 2 commits, got %d", res.Contributors[0].Commits)
	}
	if len(res.Hotspots) == 0 || res.Hotspots[0].Path != "Assets/Hero.prefab" {
		t.Fatalf("expected Hero.prefab to be the top hotspot, got %+v", res.Hotspots)
	}
	if res.Hotspots[0].Category != "游戏引擎预制体与资产" {
		t.Fatalf("unexpected hotspot category: %s", res.Hotspots[0].Category)
	}
	if res.LongestStreak != 2 {
		t.Fatalf("expected longest streak 2, got %d", res.LongestStreak)
	}
	if res.PeakHour == "" || res.PeakHour == "--" {
		t.Fatalf("expected a peak hour, got %q", res.PeakHour)
	}
	t.Logf("activity: %s | top hotspot %s risk=%d", res.Summary, res.Hotspots[0].Path, res.Hotspots[0].RiskScore)

	// Empty repo should not explode.
	empty := t.TempDir()
	runGitTest(t, empty, nil, "init", "-b", "main")
	emptyRes, err := svc.GetTeamActivity(empty, 30)
	if err != nil {
		t.Fatalf("empty repo team activity failed: %v", err)
	}
	if emptyRes.TotalCommits != 0 || len(emptyRes.Days) != 30 {
		t.Fatalf("unexpected empty repo result: %+v", emptyRes)
	}
}

func TestPickaxeSearch(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	now := time.Now()
	writeAndCommit(t, dir, "src/Loot.cs", "var item = \"LegendarySword\";\n", "feat: 掉落传说武器", now.AddDate(0, 0, -2), "Alice")
	writeAndCommit(t, dir, "src/Loot.cs", "var item = \"RareShield\";\n", "refactor: 替换掉落表", now.AddDate(0, 0, -1), "Bob")

	svc := GitService{}
	res, err := svc.PickaxeSearch(dir, "LegendarySword", "content", "", "", 10, false)
	if err != nil {
		t.Fatalf("PickaxeSearch failed: %v", err)
	}
	if res.TotalHits != 2 || len(res.Hits) != 2 {
		t.Fatalf("expected 2 hits, got total=%d shown=%d", res.TotalHits, len(res.Hits))
	}
	if res.AddedLines != 1 || res.RemovedLines != 1 {
		t.Fatalf("expected 1 added and 1 removed line, got +%d -%d", res.AddedLines, res.RemovedLines)
	}
	if res.Hits[0].Change != "移除" || res.Hits[1].Change != "新增" {
		t.Fatalf("unexpected change types: %s / %s", res.Hits[0].Change, res.Hits[1].Change)
	}
	if res.FirstSeen >= res.LastSeen {
		t.Fatalf("expected firstSeen before lastSeen, got %s / %s", res.FirstSeen, res.LastSeen)
	}

	regexRes, err := svc.PickaxeSearch(dir, "Legendary(Sword|Axe)", "regex", "", "", 10, false)
	if err != nil || regexRes.TotalHits != 2 {
		t.Fatalf("regex pickaxe failed: %v hits=%d", err, regexRes.TotalHits)
	}

	if _, err := svc.PickaxeSearch(dir, "", "content", "", "", 10, false); err == nil {
		t.Fatalf("expected error for empty query")
	}
	if _, err := svc.PickaxeSearch(dir, "([", "regex", "", "", 10, false); err == nil {
		t.Fatalf("expected error for invalid regex")
	}

	// Path filter should exclude everything.
	filtered, err := svc.PickaxeSearch(dir, "LegendarySword", "content", "", "docs/", 10, false)
	if err != nil || filtered.TotalHits != 0 {
		t.Fatalf("expected path filter to exclude hits, got %d (%v)", filtered.TotalHits, err)
	}
}

func TestLintCommitMessage(t *testing.T) {
	svc := GitService{}

	good := svc.LintCommitMessage("feat(battle): 支持多人同步开战")
	if !good.Compliant || good.Score != 100 || good.Grade != "A" {
		t.Fatalf("expected clean message to score 100, got %+v", good)
	}
	if good.Type != "feat" || good.Scope != "battle" {
		t.Fatalf("unexpected parsed type/scope: %s/%s", good.Type, good.Scope)
	}

	empty := svc.LintCommitMessage("   ")
	if empty.Score != 0 || empty.Grade != "D" || len(empty.Issues) != 1 {
		t.Fatalf("unexpected empty lint result: %+v", empty)
	}

	noType := svc.LintCommitMessage("update")
	if noType.Compliant || noType.Score >= 90 || noType.Grade == "A" {
		t.Fatalf("expected vague message to be flagged, got %+v", noType)
	}

	fixable := svc.LintCommitMessage("feat:新增副本加载。")
	if !fixable.HasFix {
		t.Fatalf("expected auto-fix candidate, got %+v", fixable)
	}
	if fixable.AutoFixed != "feat: 新增副本加载" {
		t.Fatalf("unexpected auto-fixed message: %q", fixable.AutoFixed)
	}

	body := svc.LintCommitMessage("fix: 修复奖励结算\n补充了溢出保护")
	if !body.HasFix || !strings.Contains(body.AutoFixed, "\n\n补充了溢出保护") {
		t.Fatalf("expected blank line auto-fix, got %q", body.AutoFixed)
	}

	long := svc.LintCommitMessage("feat: " + strings.Repeat("很长的描述", 20))
	if long.Score >= 100 {
		t.Fatalf("expected long subject to lose score, got %d", long.Score)
	}

	wip := svc.LintCommitMessage("chore: WIP 临时提交")
	foundWip := false
	for _, issue := range wip.Issues {
		if issue.Rule == "wip" || issue.Rule == "vague" {
			foundWip = true
		}
	}
	if !foundWip {
		t.Fatalf("expected WIP warning, got %+v", wip.Issues)
	}
}
