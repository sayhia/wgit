package main

import (
	"errors"
	"fmt"
	"strings"
)

// MergeConflictPrediction holds the conflict forecast and overlapping file list.
type MergeConflictPrediction struct {
	TargetBranch     string   `json:"targetBranch"`
	HasConflicts     bool     `json:"hasConflicts"`
	ConflictRisk     string   `json:"conflictRisk"` // "none", "low", "medium", "high", "conflict"
	RiskScore        int      `json:"riskScore"`    // 0 to 100
	ConflictingFiles []string `json:"conflictingFiles"`
	TotalModified    int      `json:"totalModified"`
	Summary          string   `json:"summary"`
}

// PredictMergeConflicts evaluates merge conflict risk against targetBranch before actual merge or rebase.
func (GitService) PredictMergeConflicts(repoPath string, targetBranch string) (*MergeConflictPrediction, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		return nil, errors.New("target branch is required")
	}

	res := &MergeConflictPrediction{
		TargetBranch:     targetBranch,
		HasConflicts:     false,
		ConflictRisk:     "none",
		RiskScore:        0,
		ConflictingFiles: []string{},
		TotalModified:    0,
		Summary:          fmt.Sprintf("与分支 %s 无检测到合并冲突", targetBranch),
	}

	// 1. Find merge-base
	base, err := ExecGit(repoPath, "merge-base", "HEAD", targetBranch)
	if err != nil || strings.TrimSpace(base) == "" {
		res.ConflictRisk = "high"
		res.RiskScore = 80
		res.Summary = fmt.Sprintf("无法计算与 %s 的共同祖先提交，可能为孤立分支", targetBranch)
		return res, nil
	}
	baseHash := strings.TrimSpace(base)

	// 2. Try git merge-tree (modern 3-way merge in memory without modifying worktree)
	out, err := ExecGit(repoPath, "merge-tree", baseHash, "HEAD", targetBranch)
	if err == nil {
		lines := strings.Split(out, "\n")
		conflicts := []string{}
		inConflict := false
		for _, line := range lines {
			if strings.Contains(line, "CONFLICT") || strings.Contains(line, "conflict") {
				fields := strings.Fields(line)
				for _, f := range fields {
					if strings.Contains(f, "/") || strings.Contains(f, ".") {
						conflicts = append(conflicts, strings.Trim(f, "\":'"))
					}
				}
				inConflict = true
			}
		}
		if inConflict && len(conflicts) > 0 {
			res.HasConflicts = true
			res.ConflictRisk = "conflict"
			res.RiskScore = 95
			res.ConflictingFiles = uniqueStrings(conflicts)
			res.Summary = fmt.Sprintf("预测与 %s 存在 %d 个冲突文件", targetBranch, len(res.ConflictingFiles))
			return res, nil
		}
	}

	// 3. Fallback / Intersection check: files changed in HEAD vs base AND changed in target vs base
	headFilesRaw, _ := ExecGit(repoPath, "diff", "--name-only", baseHash, "HEAD")
	targetFilesRaw, _ := ExecGit(repoPath, "diff", "--name-only", baseHash, targetBranch)

	headMap := make(map[string]bool)
	for _, f := range strings.Split(headFilesRaw, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			headMap[f] = true
		}
	}

	overlapping := []string{}
	for _, f := range strings.Split(targetFilesRaw, "\n") {
		f = strings.TrimSpace(f)
		if f != "" && headMap[f] {
			overlapping = append(overlapping, f)
		}
	}

	res.TotalModified = len(headMap)
	if len(overlapping) > 0 {
		res.ConflictingFiles = overlapping
		res.RiskScore = minInt(100, len(overlapping)*25)
		if res.RiskScore >= 75 {
			res.ConflictRisk = "high"
		} else if res.RiskScore >= 40 {
			res.ConflictRisk = "medium"
		} else {
			res.ConflictRisk = "low"
		}
		res.Summary = fmt.Sprintf("发现 %d 个并发修改文件，合并时需重点关注", len(overlapping))
	} else {
		res.ConflictRisk = "none"
		res.RiskScore = 0
		res.Summary = fmt.Sprintf("未发现与 %s 的重叠修改文件，可安全合并", targetBranch)
	}

	return res, nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	out := []string{}
	for _, s := range in {
		if !seen[s] && s != "" {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
