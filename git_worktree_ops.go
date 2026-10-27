package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WorktreeDiskUsage represents the disk footprint of a linked worktree.
type WorktreeDiskUsage struct {
	SizeBytes     int64  `json:"sizeBytes"`
	FormattedSize string `json:"formattedSize"`
	FileCount     int    `json:"fileCount"`
}

// GetWorktreeDiskUsage calculates the real disk footprint of a target worktree.
func (GitService) GetWorktreeDiskUsage(repoPath string, worktreePath string) (*WorktreeDiskUsage, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path cannot be empty")
	}
	target := strings.TrimSpace(worktreePath)
	if target == "" {
		target = repoPath
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(repoPath, target)
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("工作树路径不可访问: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("工作树路径不是有效目录")
	}

	var totalBytes int64
	var fileCount int

	err = filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable files gracefully
		}
		if d.IsDir() {
			name := d.Name()
			// Don't recurse into .git inside worktree (it's either a gitfile or linked dir)
			if name == ".git" && path != target {
				return filepath.SkipDir
			}
			return nil
		}
		fileInfo, infoErr := d.Info()
		if infoErr == nil {
			totalBytes += fileInfo.Size()
			fileCount++
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("统计工作树体积出错: %w", err)
	}

	return &WorktreeDiskUsage{
		SizeBytes:     totalBytes,
		FormattedSize: formatBytes(totalBytes),
		FileCount:     fileCount,
	}, nil
}

// WorktreeBranchValidation represents result of verifying whether a branch is already checked out.
type WorktreeBranchValidation struct {
	Available      bool   `json:"available"`
	Message        string `json:"message"`
	WorktreePath   string `json:"worktreePath,omitempty"`
}

// ValidateWorktreeBranch verifies if a branch is already checked out by any worktree in the repository.
func (s GitService) ValidateWorktreeBranch(repoPath string, branchName string) (*WorktreeBranchValidation, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path cannot be empty")
	}
	branch := strings.TrimSpace(branchName)
	if branch == "" {
		return nil, errors.New("branch name cannot be empty")
	}

	worktrees, err := s.GetWorktrees(repoPath)
	if err != nil {
		return nil, fmt.Errorf("读取工作树列表失败: %w", err)
	}

	normalizedTarget := strings.TrimPrefix(branch, "refs/heads/")

	for _, wt := range worktrees {
		wtBranch := strings.TrimPrefix(wt.Branch, "refs/heads/")
		if wtBranch == normalizedTarget {
			return &WorktreeBranchValidation{
				Available:    false,
				Message:      fmt.Sprintf("分支 %s 已在工作树 %s 中检出", normalizedTarget, wt.Path),
				WorktreePath: wt.Path,
			}, nil
		}
	}

	return &WorktreeBranchValidation{
		Available: true,
		Message:   "分支可用，未被其他工作树检出",
	}, nil
}
