package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Branch insights answer the question a team actually asks before a
// milestone: which of these dozens of branches are finished, which carry
// unmerged work, and which can be deleted safely. Everything is derived from
// git at read time — nothing is cached in the repository.

const branchStaleDays = 30

type BranchInsight struct {
	Name            string `json:"name"`
	IsCurrent       bool   `json:"isCurrent"`
	IsRemote        bool   `json:"isRemote"`
	ShortHash       string `json:"shortHash"`
	LastAuthor      string `json:"lastAuthor"`
	LastDate        string `json:"lastDate"`
	LastSubject     string `json:"lastSubject"`
	Ahead           int    `json:"ahead"`
	Behind          int    `json:"behind"`
	HasUpstream     bool   `json:"hasUpstream"`
	Upstream        string `json:"upstream"`
	Merged          bool   `json:"merged"`
	Protected       bool   `json:"protected"`
	DaysSinceCommit int    `json:"daysSinceCommit"`
	Category        string `json:"category"` // current | protected | merged | unpushed | stale | active
	Suggestion      string `json:"suggestion"`
}

type BranchInsightSummary struct {
	Total      int    `json:"total"`
	Current    string `json:"current"`
	Merged     int    `json:"merged"`
	Protected  int    `json:"protected"`
	Unpushed   int    `json:"unpushed"`
	Stale      int    `json:"stale"`
	Cleanable  int    `json:"cleanable"`
	StaleDays  int    `json:"staleDays"`
	BaseBranch string `json:"baseBranch"`
	Hint       string `json:"hint"`
}

type BranchInsightsResult struct {
	Base    string               `json:"base"`
	Items   []BranchInsight      `json:"items"`
	Summary BranchInsightSummary `json:"summary"`
}

// resolveInsightBase picks a sensible base branch to compare against.
func resolveInsightBase(repoPath string, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base != "" {
		if out, err := ExecGit(repoPath, "rev-parse", "--verify", "--quiet", base); err == nil && out != "" {
			return base, nil
		}
		return "", errors.New("基准分支不存在：" + base)
	}
	for _, cand := range []string{"main", "master", "develop", "trunk"} {
		if out, err := ExecGit(repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+cand); err == nil && out != "" {
			return cand, nil
		}
	}
	if out, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out), nil
	}
	return "", errors.New("仓库没有任何分支")
}

// mergedBranchSet returns every branch already reachable from base.
func mergedBranchSet(repoPath string, base string) map[string]bool {
	out, err := ExecGit(repoPath, "branch", "--merged", base, "--format=%(refname:short)")
	set := map[string]bool{}
	if err != nil {
		return set
	}
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			set[name] = true
		}
	}
	return set
}

// aheadBehind counts commits on either side of base...branch.
func aheadBehind(repoPath string, base string, branch string) (int, int) {
	out, err := ExecGit(repoPath, "rev-list", "--left-right", "--count", base+"..."+branch)
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return 0, 0
	}
	behind, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
	ahead, _ := strconv.Atoi(strings.TrimSpace(fields[1]))
	return ahead, behind
}

// GetBranchInsights reports the state of every local branch against base.
func (GitService) GetBranchInsights(repoPath string, base string) (*BranchInsightsResult, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	base, err := resolveInsightBase(repoPath, base)
	if err != nil {
		return nil, err
	}
	merged := mergedBranchSet(repoPath, base)
	remotesOut, _ := ExecGit(repoPath, "remote")
	hasRemote := strings.TrimSpace(remotesOut) != ""

	// ExecGitRaw matters here: `for-each-ref` ends every line with a tab when
	// the branch has no upstream, and trimming the whole stdout would drop the
	// final separator — the last branch would then fail the field count check
	// and silently disappear from the list.
	raw, err := ExecGitRaw(repoPath, "for-each-ref",
		"--format=%(refname:short)%09%(objectname:short)%09%(authorname)%09%(authordate:short)%09%(subject)%09%(upstream:short)",
		"refs/heads")
	if err != nil {
		return nil, err
	}

	current := ""
	if out, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		current = strings.TrimSpace(out)
	}

	items := []BranchInsight{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\t")
		name := refField(f, 0)
		if name == "" {
			continue
		}
		ahead, behind := aheadBehind(repoPath, base, name)
		item := BranchInsight{
			Name:            name,
			IsCurrent:       name == current,
			ShortHash:       refField(f, 1),
			LastAuthor:      refField(f, 2),
			LastDate:        refField(f, 3),
			LastSubject:     refField(f, 4),
			Upstream:        refField(f, 5),
			HasUpstream:     refField(f, 5) != "",
			Merged:          merged[name],
			Protected:       protectedBranchName(name),
			Ahead:           ahead,
			Behind:          behind,
			DaysSinceCommit: daysSince(refField(f, 3)),
		}
		item.Category, item.Suggestion = classifyBranchInsight(item, hasRemote)
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		pi, pj := insightRank(items[i].Category), insightRank(items[j].Category)
		if pi != pj {
			return pi < pj
		}
		return items[i].Name < items[j].Name
	})

	sum := BranchInsightSummary{
		Total:      len(items),
		Current:    current,
		StaleDays:  branchStaleDays,
		BaseBranch: base,
	}
	for _, it := range items {
		switch it.Category {
		case "current":
			// tracked separately
		case "protected":
			sum.Protected++
		case "merged":
			sum.Merged++
			sum.Cleanable++
		case "unpushed":
			sum.Unpushed++
		case "stale":
			sum.Stale++
		}
	}
	switch {
	case sum.Merged > 0:
		sum.Hint = fmt.Sprintf("有 %d 个分支已合并进 %s，可以安全清理", sum.Merged, base)
	case sum.Unpushed > 0:
		sum.Hint = fmt.Sprintf("有 %d 个分支尚未推送到远端", sum.Unpushed)
	case sum.Stale > 0:
		sum.Hint = fmt.Sprintf("有 %d 个分支超过 %d 天没有新提交", sum.Stale, branchStaleDays)
	default:
		sum.Hint = "分支状态健康"
	}

	return &BranchInsightsResult{Base: base, Items: items, Summary: sum}, nil
}

// classifyBranchInsight turns raw counters into one actionable verdict.
func classifyBranchInsight(item BranchInsight, hasRemote bool) (string, string) {
	if item.IsCurrent {
		return "current", "当前工作分支"
	}
	if item.Protected {
		return "protected", "长期分支，保留且不参与自动清理"
	}
	if item.Merged {
		return "merged", "已合并，可删除"
	}
	// Only flag a missing upstream when the repo actually has a remote: a
	// purely local repository never tracks anything, and reporting every
	// branch as "unpushed" there would be noise.
	if hasRemote && !item.HasUpstream {
		return "unpushed", "没有上游分支，推送后才不会丢失工作"
	}
	if item.DaysSinceCommit >= branchStaleDays {
		return "stale", "距离上次提交已 " + strconv.Itoa(item.DaysSinceCommit) + " 天，确认是否还在推进"
	}
	return "active", "活跃分支"
}

func insightRank(category string) int {
	switch category {
	case "current":
		return 0
	case "protected":
		return 1
	case "stale":
		return 2
	case "unpushed":
		return 3
	case "merged":
		return 4
	default:
		return 5
	}
}

// refField reads a tab separated for-each-ref field without panicking when a
// trailing field is missing (no upstream configured).
func refField(f []string, i int) string {
	if i < len(f) {
		return strings.TrimSpace(f[i])
	}
	return ""
}

func daysSince(date string) int {
	date = strings.TrimSpace(date)
	if date == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0
	}
	days := int(time.Since(t).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
