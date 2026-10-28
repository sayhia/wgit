package main

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BranchHealthReport summarizes the divergence, commit equivalence, and merge readiness of a branch.
type BranchHealthReport struct {
	Branch            string   `json:"branch"`
	BaseBranch        string   `json:"baseBranch"`
	AheadCount        int      `json:"aheadCount"`
	BehindCount       int      `json:"behindCount"`
	DivergenceDays    int      `json:"divergenceDays"`
	EquivalentCommits int      `json:"equivalentCommits"`
	ReadinessScore    int      `json:"readinessScore"` // 0 - 100
	ReadinessLevel    string   `json:"readinessLevel"` // "ready" | "needs_sync" | "diverged"
	RiskFactors       []string `json:"riskFactors"`
	Recommendations   []string `json:"recommendations"`
	CanFastForward    bool     `json:"canFastForward"`
}

// EvaluateBranchHealth analyzes how cleanly a branch can integrate into baseBranch.
func (a *GitService) EvaluateBranchHealth(repoPath, branch, baseBranch string) (*BranchHealthReport, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Resolve current branch if empty
	if branch == "" {
		cmdCur := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
		cmdCur.Dir = repoPath
		out, err := cmdCur.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to detect current branch: %w", err)
		}
		branch = strings.TrimSpace(string(out))
	}

	// Resolve base branch if empty
	if baseBranch == "" {
		for _, candidate := range []string{"main", "master", "develop", "origin/main", "origin/master"} {
			cmdCheck := exec.CommandContext(ctx, "git", "rev-parse", "--verify", candidate)
			cmdCheck.Dir = repoPath
			if err := cmdCheck.Run(); err == nil {
				baseBranch = candidate
				break
			}
		}
		if baseBranch == "" {
			baseBranch = "main"
		}
	}

	report := &BranchHealthReport{
		Branch:          branch,
		BaseBranch:      baseBranch,
		RiskFactors:     []string{},
		Recommendations: []string{},
	}

	// 1. Ahead count (baseBranch..branch)
	cmdAhead := exec.CommandContext(ctx, "git", "rev-list", "--count", fmt.Sprintf("%s..%s", baseBranch, branch))
	cmdAhead.Dir = repoPath
	if out, err := cmdAhead.Output(); err == nil {
		report.AheadCount, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	// 2. Behind count (branch..baseBranch)
	cmdBehind := exec.CommandContext(ctx, "git", "rev-list", "--count", fmt.Sprintf("%s..%s", branch, baseBranch))
	cmdBehind.Dir = repoPath
	if out, err := cmdBehind.Output(); err == nil {
		report.BehindCount, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	report.CanFastForward = (report.BehindCount == 0)

	// 3. Find merge base and calculate divergence age
	cmdMb := exec.CommandContext(ctx, "git", "merge-base", baseBranch, branch)
	cmdMb.Dir = repoPath
	if mbOut, err := cmdMb.Output(); err == nil {
		mbHash := strings.TrimSpace(string(mbOut))
		if mbHash != "" {
			cmdTime := exec.CommandContext(ctx, "git", "log", "-1", "--format=%ct", mbHash)
			cmdTime.Dir = repoPath
			if tOut, err := cmdTime.Output(); err == nil {
				if sec, err := strconv.ParseInt(strings.TrimSpace(string(tOut)), 10, 64); err == nil && sec > 0 {
					mbTime := time.Unix(sec, 0)
					days := int(math.Floor(time.Since(mbTime).Hours() / 24))
					if days < 0 {
						days = 0
					}
					report.DivergenceDays = days
				}
			}
		}
	}

	// 4. Cherry equivalent commit detection (git cherry base branch)
	cmdCherry := exec.CommandContext(ctx, "git", "cherry", baseBranch, branch)
	cmdCherry.Dir = repoPath
	if chOut, err := cmdCherry.Output(); err == nil {
		lines := strings.Split(string(chOut), "\n")
		for _, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "-") {
				report.EquivalentCommits++
			}
		}
	}

	// 5. Calculate readiness score (0-100)
	score := 100
	if report.BehindCount > 0 {
		penalty := report.BehindCount * 5
		if penalty > 45 {
			penalty = 45
		}
		score -= penalty
		report.RiskFactors = append(report.RiskFactors, fmt.Sprintf("落后基准分支 %d 个提交", report.BehindCount))
		report.Recommendations = append(report.Recommendations, "建议在合并前先变基 (Rebase) 或同步合并上游最新改动")
	}

	if report.DivergenceDays > 14 {
		penalty := (report.DivergenceDays - 14) * 2
		if penalty > 35 {
			penalty = 35
		}
		score -= penalty
		report.RiskFactors = append(report.RiskFactors, fmt.Sprintf("分支偏离基准已达 %d 天", report.DivergenceDays))
		report.Recommendations = append(report.Recommendations, "长期未同步可能累积深层逻辑冲突，建议尽早完成阶段性合并")
	}

	if report.EquivalentCommits > 0 {
		score += 5
		report.Recommendations = append(report.Recommendations, fmt.Sprintf("检测到 %d 个上游等价提交，变基时将自动跳过冗余修改", report.EquivalentCommits))
	}

	if score > 100 {
		score = 100
	}
	if score < 10 {
		score = 10
	}
	report.ReadinessScore = score

	if report.ReadinessScore >= 80 {
		report.ReadinessLevel = "ready"
	} else if report.ReadinessScore >= 50 {
		report.ReadinessLevel = "needs_sync"
	} else {
		report.ReadinessLevel = "diverged"
	}

	if len(report.Recommendations) == 0 {
		report.Recommendations = append(report.Recommendations, "分支状态健康，随时可安全合并入主基准")
	}

	return report, nil
}
