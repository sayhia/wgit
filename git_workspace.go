package main

import (
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// WorkspaceStats summarizes the working tree so the UI can show churn at a glance.

type FileChurnStat struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	IsBinary  bool   `json:"isBinary"`
	Staged    bool   `json:"staged"`
}

type WorkspaceCategoryStat struct {
	Category  string `json:"category"`
	Files     int    `json:"files"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type WorkspaceStats struct {
	StagedFiles       int                     `json:"stagedFiles"`
	StagedAdditions   int                     `json:"stagedAdditions"`
	StagedDeletions   int                     `json:"stagedDeletions"`
	UnstagedFiles     int                     `json:"unstagedFiles"`
	UnstagedAdditions int                     `json:"unstagedAdditions"`
	UnstagedDeletions int                     `json:"unstagedDeletions"`
	UntrackedFiles    int                     `json:"untrackedFiles"`
	ConflictedFiles   int                     `json:"conflictedFiles"`
	TotalAdditions    int                     `json:"totalAdditions"`
	TotalDeletions    int                     `json:"totalDeletions"`
	Categories        []WorkspaceCategoryStat `json:"categories"`
	Files             []FileChurnStat         `json:"files"`
	Summary           string                  `json:"summary"`
}

// GetWorkspaceStats aggregates staged/unstaged churn plus a per-category breakdown.
func (GitService) GetWorkspaceStats(repoPath string) (*WorkspaceStats, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}

	res := &WorkspaceStats{
		Categories: []WorkspaceCategoryStat{},
		Files:      []FileChurnStat{},
	}

	statusOut, _ := ExecGitRaw(repoPath, "status", "--porcelain=v1", "-uall")
	var untrackedPaths []string
	for _, line := range strings.Split(statusOut, "\n") {
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		path := strings.TrimSpace(line[3:])
		path = strings.Trim(path, "\"")
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			path = parts[len(parts)-1]
		}

		if x == '?' && y == '?' {
			res.UntrackedFiles++
			untrackedPaths = append(untrackedPaths, path)
			continue
		}
		if (x == 'U' || y == 'U') || (x == 'A' && y == 'A') || (x == 'D' && y == 'D') {
			res.ConflictedFiles++
		}
		if x != ' ' && x != '?' {
			res.StagedFiles++
		}
		if y != ' ' && y != '?' {
			res.UnstagedFiles++
		}
	}

	if raw, err := ExecGit(repoPath, "diff", "--numstat", "--no-renames"); err == nil {
		for _, stat := range parseNumstat(raw, false) {
			res.Files = append(res.Files, stat)
			res.UnstagedAdditions += stat.Additions
			res.UnstagedDeletions += stat.Deletions
		}
	}
	if raw, err := ExecGit(repoPath, "diff", "--cached", "--numstat", "--no-renames"); err == nil {
		for _, stat := range parseNumstat(raw, true) {
			res.Files = append(res.Files, stat)
			res.StagedAdditions += stat.Additions
			res.StagedDeletions += stat.Deletions
		}
	}

	res.TotalAdditions = res.StagedAdditions + res.UnstagedAdditions
	res.TotalDeletions = res.StagedDeletions + res.UnstagedDeletions

	// Category breakdown: tracked churn plus untracked file counts.
	type bucket struct {
		files, additions, deletions int
	}
	buckets := map[string]*bucket{}
	addToBucket := func(path string, additions, deletions int) {
		cat := shortAssetCategory(path)
		b := buckets[cat]
		if b == nil {
			b = &bucket{}
			buckets[cat] = b
		}
		b.files++
		b.additions += additions
		b.deletions += deletions
	}
	for _, stat := range res.Files {
		addToBucket(stat.Path, stat.Additions, stat.Deletions)
	}
	for _, path := range untrackedPaths {
		addToBucket(path, 0, 0)
	}

	for cat, b := range buckets {
		res.Categories = append(res.Categories, WorkspaceCategoryStat{
			Category:  cat,
			Files:     b.files,
			Additions: b.additions,
			Deletions: b.deletions,
		})
	}
	sort.Slice(res.Categories, func(i, j int) bool {
		if res.Categories[i].Files == res.Categories[j].Files {
			return res.Categories[i].Category < res.Categories[j].Category
		}
		return res.Categories[i].Files > res.Categories[j].Files
	})

	total := res.StagedFiles + res.UnstagedFiles + res.UntrackedFiles + res.ConflictedFiles
	switch {
	case total == 0:
		res.Summary = "工作区干净，没有任何待提交的改动"
	case res.ConflictedFiles > 0:
		res.Summary = "存在未解决冲突，请先处理后再提交"
	default:
		res.Summary = "共 " + strconv.Itoa(total) + " 个变更文件"
	}

	return res, nil
}

func parseNumstat(raw string, staged bool) []FileChurnStat {
	stats := []FileChurnStat{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		path := strings.TrimSpace(parts[2])
		if path == "" {
			continue
		}
		stat := FileChurnStat{Path: path, Staged: staged}
		if parts[0] == "-" || parts[1] == "-" {
			stat.IsBinary = true
		} else {
			stat.Additions, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
			stat.Deletions, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}
		stats = append(stats, stat)
	}
	return stats
}

// shortAssetCategory collapses the verbose asset classifier into legend-sized buckets.
func shortAssetCategory(path string) string {
	cat := classifyAssetCategory(strings.ToLower(filepath.Ext(path)))
	switch {
	case strings.Contains(cat, "源代码"):
		return "代码"
	case strings.Contains(cat, "纹理") || strings.Contains(cat, "3D"):
		return "美术"
	case strings.Contains(cat, "音频"):
		return "音频"
	case strings.Contains(cat, "配置") || strings.Contains(cat, "数据"):
		return "配置"
	case strings.Contains(cat, "引擎"):
		return "引擎资产"
	case strings.Contains(cat, "视频"):
		return "视频"
	default:
		return "其他"
	}
}
