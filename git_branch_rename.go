package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BranchRenameResult records the outcome of renaming a local branch and adjusting upstream tracking.
type BranchRenameResult struct {
	OldName        string `json:"oldName"`
	NewName        string `json:"newName"`
	HasUpstream    bool   `json:"hasUpstream"`
	UpstreamBranch string `json:"upstreamBranch,omitempty"`
	RemoteRenamed  bool   `json:"remoteRenamed"`
	Message        string `json:"message"`
	Success        bool   `json:"success"`
}

// RenameBranchWithUpstream renames a local branch and optionally re-points or migrates remote upstream tracking.
func (a *GitService) RenameBranchWithUpstream(repoPath, oldName, newName string, syncUpstream bool) (*BranchRenameResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return nil, fmt.Errorf("old and new branch names are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	result := &BranchRenameResult{
		OldName: oldName,
		NewName: newName,
	}

	// 1. Detect existing upstream
	cmdUp := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", fmt.Sprintf("%s@{upstream}", oldName))
	cmdUp.Dir = repoPath
	if upOut, err := cmdUp.Output(); err == nil {
		upstream := strings.TrimSpace(string(upOut))
		if upstream != "" {
			result.HasUpstream = true
			result.UpstreamBranch = upstream
		}
	}

	// 2. Rename local branch (git branch -m oldName newName)
	cmdMv := exec.CommandContext(ctx, "git", "branch", "-m", oldName, newName)
	cmdMv.Dir = repoPath
	if out, err := cmdMv.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to rename branch '%s' to '%s': %s", oldName, newName, strings.TrimSpace(string(out)))
	}

	// 3. Re-align upstream if present
	if result.HasUpstream {
		parts := strings.SplitN(result.UpstreamBranch, "/", 2)
		remote := "origin"
		if len(parts) == 2 {
			remote = parts[0]
		}

		if syncUpstream {
			// Push new branch to remote and track it
			cmdPush := exec.CommandContext(ctx, "git", "push", "-u", remote, newName)
			cmdPush.Dir = repoPath
			if pOut, err := cmdPush.CombinedOutput(); err == nil {
				// Delete old branch on remote
				cmdDel := exec.CommandContext(ctx, "git", "push", remote, "--delete", oldName)
				cmdDel.Dir = repoPath
				_ = cmdDel.Run()
				result.RemoteRenamed = true
				result.UpstreamBranch = fmt.Sprintf("%s/%s", remote, newName)
			} else {
				// Fallback to tracking existing upstream
				cmdFallback := exec.CommandContext(ctx, "git", "branch", fmt.Sprintf("--set-upstream-to=%s", result.UpstreamBranch), newName)
				cmdFallback.Dir = repoPath
				_ = cmdFallback.Run()
				result.Message = fmt.Sprintf("已重命名本地分支，远端推送失败: %s", strings.TrimSpace(string(pOut)))
			}
		} else {
			// Keep pointing to old remote branch
			cmdLink := exec.CommandContext(ctx, "git", "branch", fmt.Sprintf("--set-upstream-to=%s", result.UpstreamBranch), newName)
			cmdLink.Dir = repoPath
			_ = cmdLink.Run()
		}
	}

	result.Success = true
	if result.Message == "" {
		if result.RemoteRenamed {
			result.Message = fmt.Sprintf("成功重命名分支为 '%s'，并同步迁移远端上游", newName)
		} else {
			result.Message = fmt.Sprintf("成功重命名本地分支为 '%s'", newName)
		}
	}

	return result, nil
}
