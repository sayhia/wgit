package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type BlastRadiusReport struct {
	ActionType            string   `json:"actionType"`
	TargetRef             string   `json:"targetRef"`
	RiskLevel             string   `json:"riskLevel"` // "low", "medium", "high", "critical"
	UncommittedFilesCount int      `json:"uncommittedFilesCount"`
	DanglingCommitsCount  int      `json:"danglingCommitsCount"`
	OverwrittenCommits    []string `json:"overwrittenCommits"`
	WarningMessage        string   `json:"warningMessage"`
	SafeToProceed         bool     `json:"safeToProceed"`
	SuggestedAlternative  string   `json:"suggestedAlternative"`
	Summary               string   `json:"summary"`
}

func execGitRadius(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CalculateBlastRadius assesses the destructive blast radius before risky Git operations.
func (GitService) CalculateBlastRadius(repoPath string, actionType string, targetRef string) (*BlastRadiusReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	actionType = strings.TrimSpace(strings.ToLower(actionType))
	targetRef = strings.TrimSpace(targetRef)

	// 1. Check uncommitted changes in current worktree
	statusOut, _ := execGitRadius(repoPath, "status", "--porcelain")
	uncommittedLines := strings.Split(strings.TrimSpace(statusOut), "\n")
	uncommittedCount := 0
	for _, l := range uncommittedLines {
		if strings.TrimSpace(l) != "" {
			uncommittedCount++
		}
	}

	riskLevel := "low"
	danglingCount := 0
	overwritten := make([]string, 0)
	warningMsg := ""
	altMsg := ""
	safe := true

	switch actionType {
	case "reset-hard":
		// Target ref e.g. "HEAD~1" or commit sha
		if targetRef == "" {
			targetRef = "HEAD"
		}
		// Count commits that will be detached/lost
		revOut, err := execGitRadius(repoPath, "rev-list", fmt.Sprintf("%s..HEAD", targetRef))
		if err == nil && strings.TrimSpace(revOut) != "" {
			lines := strings.Split(strings.TrimSpace(revOut), "\n")
			danglingCount = len(lines)
			for _, h := range lines {
				if len(h) >= 7 {
					overwritten = append(overwritten, h[:7])
				}
			}
		}

		if uncommittedCount > 0 && danglingCount > 0 {
			riskLevel = "critical"
			warningMsg = fmt.Sprintf("强制重置将永久销毁 %d 个未提交修改，并丢弃 %d 个未推送提交！", uncommittedCount, danglingCount)
			altMsg = "建议先执行 Stash 暂存并创建备份分支：git branch backup-head"
			safe = false
		} else if uncommittedCount > 0 {
			riskLevel = "high"
			warningMsg = fmt.Sprintf("强制重置将丢失当前工作区与暂存区的 %d 个未提交变更", uncommittedCount)
			altMsg = "建议优先使用 git stash 保留当前状态"
			safe = false
		} else if danglingCount > 0 {
			riskLevel = "medium"
			warningMsg = fmt.Sprintf("重置后将有 %d 个提交从当前分支脱落", danglingCount)
			altMsg = "如需保留变更历史，建议使用 Soft Reset (git reset --soft)"
		} else {
			riskLevel = "low"
			warningMsg = "当前工作区干净，无未提交修改将被覆盖"
			altMsg = "可安全执行"
		}

	case "branch-delete":
		if targetRef == "" {
			return nil, errors.New("targetRef branch must be specified for branch-delete")
		}
		// Check if merged into HEAD
		mergedOut, err := execGitRadius(repoPath, "branch", "--merged", "HEAD")
		isMerged := false
		if err == nil {
			for _, b := range strings.Split(mergedOut, "\n") {
				cleanB := strings.TrimSpace(strings.TrimPrefix(b, "*"))
				if cleanB == targetRef {
					isMerged = true
					break
				}
			}
		}

		if !isMerged {
			// Find unmerged commits
			unmergedOut, _ := execGitRadius(repoPath, "rev-list", fmt.Sprintf("HEAD..%s", targetRef))
			lines := strings.Split(strings.TrimSpace(unmergedOut), "\n")
			if strings.TrimSpace(unmergedOut) != "" {
				danglingCount = len(lines)
				for _, h := range lines {
					if len(h) >= 7 {
						overwritten = append(overwritten, h[:7])
					}
				}
			}
			riskLevel = "high"
			warningMsg = fmt.Sprintf("分支 '%s' 尚未合入当前分支，删除将导致 %d 个提交变为孤儿提交！", targetRef, danglingCount)
			altMsg = "建议先确认合并或创建标签锚定归档：git tag archive/" + targetRef
			safe = false
		} else {
			riskLevel = "low"
			warningMsg = fmt.Sprintf("分支 '%s' 的提交已全部合入主线，删除安全无损", targetRef)
			altMsg = "可安全删除已合入分支"
		}

	case "push-force":
		if targetRef == "" {
			targetRef = "origin"
		}
		// Check behind count on remote
		behindOut, err := execGitRadius(repoPath, "rev-list", "--count", "HEAD..@{u}")
		behindCount := 0
		if err == nil {
			behindCount, _ = strconv.Atoi(strings.TrimSpace(behindOut))
		}
		if behindCount > 0 {
			riskLevel = "critical"
			warningMsg = fmt.Sprintf("强推将暴力覆盖远端 %d 个领先提交，可能破坏团队协作者代码！", behindCount)
			altMsg = "强烈建议使用带租约安全强推 (--force-with-lease) 或先 pull --rebase"
			safe = false
			danglingCount = behindCount
		} else {
			riskLevel = "medium"
			warningMsg = "当前本地领先于上游，强推风险可控但仍需谨慎"
			altMsg = "推荐优先使用普通 push 或 force-with-lease"
		}

	default:
		riskLevel = "low"
		warningMsg = "评估操作风险较小"
		altMsg = "正常执行"
	}

	summary := fmt.Sprintf("[%s] Blast Radius: %s risk, %d uncommitted, %d dangling commits", strings.ToUpper(actionType), riskLevel, uncommittedCount, danglingCount)

	return &BlastRadiusReport{
		ActionType:            actionType,
		TargetRef:             targetRef,
		RiskLevel:             riskLevel,
		UncommittedFilesCount: uncommittedCount,
		DanglingCommitsCount:  danglingCount,
		OverwrittenCommits:    overwritten,
		WarningMessage:        warningMsg,
		SafeToProceed:         safe,
		SuggestedAlternative:  altMsg,
		Summary:               summary,
	}, nil
}
