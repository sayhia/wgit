package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddWorktreeAdvanced creates a new linked working tree with advanced options.
func (GitService) AddWorktreeAdvanced(repoPath string, opts AddWorktreeAdvancedOptions) error {
	if strings.TrimSpace(repoPath) == "" {
		return errors.New("repository path cannot be empty")
	}
	targetPath := strings.TrimSpace(opts.Path)
	if targetPath == "" {
		return errors.New("worktree path cannot be empty")
	}

	branch := strings.TrimSpace(opts.Branch)
	baseRef := strings.TrimSpace(opts.BaseRef)

	if opts.NewBranch && branch == "" {
		return errors.New("branch name cannot be empty when creating a new branch")
	}

	args := []string{"worktree", "add"}

	if opts.Detach {
		args = append(args, "--detach", targetPath)
		if baseRef != "" {
			args = append(args, baseRef)
		} else if branch != "" {
			args = append(args, branch)
		}
	} else if opts.NewBranch && branch != "" {
		args = append(args, "-b", branch, targetPath)
		if baseRef != "" {
			args = append(args, baseRef)
		}
	} else if branch != "" {
		args = append(args, targetPath, branch)
	} else if baseRef != "" {
		args = append(args, targetPath, baseRef)
	} else {
		args = append(args, targetPath)
	}

	_, err := ExecGit(repoPath, args...)
	return err
}

// MoveWorktree moves a linked working tree to a new location.
func (GitService) MoveWorktree(repoPath string, oldPath string, newPath string) error {
	if strings.TrimSpace(repoPath) == "" {
		return errors.New("repository path cannot be empty")
	}
	src := strings.TrimSpace(oldPath)
	if src == "" {
		return errors.New("source worktree path cannot be empty")
	}
	dst := strings.TrimSpace(newPath)
	if dst == "" {
		return errors.New("target worktree path cannot be empty")
	}

	_, err := ExecGit(repoPath, "worktree", "move", src, dst)
	return err
}

// GetWorktreeStatus checks if a target worktree has uncommitted changes.
func (GitService) GetWorktreeStatus(repoPath string, worktreePath string) (*WorktreeStatus, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path cannot be empty")
	}
	targetPath := strings.TrimSpace(worktreePath)
	if targetPath == "" {
		targetPath = repoPath
	}
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(repoPath, targetPath)
	}

	raw, err := ExecGitRaw(targetPath, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("读取工作树状态失败: %w", err)
	}

	dirtyCount := 0
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) != "" {
			dirtyCount++
		}
	}

	isDirty := dirtyCount > 0
	var summary string
	if isDirty {
		summary = fmt.Sprintf("存在 %d 处未提交的变更", dirtyCount)
	} else {
		summary = "工作区整洁，无未提交的变更"
	}

	return &WorktreeStatus{
		Path:       targetPath,
		IsDirty:    isDirty,
		DirtyCount: dirtyCount,
		Summary:    summary,
	}, nil
}

// PruneSingleWorktree safely prunes or cleans an invalid or unneeded worktree.
func (s GitService) PruneSingleWorktree(repoPath string, worktreePath string) error {
	if strings.TrimSpace(repoPath) == "" {
		return errors.New("repository path cannot be empty")
	}
	target := strings.TrimSpace(worktreePath)
	if target == "" {
		return errors.New("worktree path cannot be empty")
	}

	// Check if this is the main worktree
	repoAbs, errRepo := filepath.EvalSymlinks(repoPath)
	targetAbs, errTarget := filepath.EvalSymlinks(target)
	if errRepo == nil && errTarget == nil && repoAbs == targetAbs {
		return errors.New("cannot prune the main worktree")
	}

	// 1. If directory doesn't exist on disk, unlock it first (if locked) then prune
	if _, err := os.Stat(target); os.IsNotExist(err) {
		_ = s.UnlockWorktree(repoPath, target)
		_, pruneErr := ExecGit(repoPath, "worktree", "prune", "--expire=now")
		return pruneErr
	}

	// 2. If directory exists, attempt unlock and safe remove first
	_ = s.UnlockWorktree(repoPath, target)
	if _, err := ExecGit(repoPath, "worktree", "remove", "--force", target); err == nil {
		return nil
	}

	// 3. If remove failed (e.g. broken git metadata link), prune and remove directory
	_, pruneErr := ExecGit(repoPath, "worktree", "prune", "--expire=now")
	removeErr := os.RemoveAll(target)
	switch {
	case pruneErr != nil && removeErr != nil:
		return fmt.Errorf("工作树元数据与目录均清理失败: prune: %v; 目录: %w", pruneErr, removeErr)
	case pruneErr != nil:
		return pruneErr
	case removeErr != nil:
		return fmt.Errorf("工作树元数据已清理，但删除目录失败: %w", removeErr)
	default:
		return nil
	}
}
