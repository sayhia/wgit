package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StashFileApplyResult records the outcome of retrieving a single file from a stash snapshot into the worktree.
type StashFileApplyResult struct {
	StashIndex int    `json:"stashIndex"`
	FilePath   string `json:"filePath"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
}

// ApplyStashFile extracts a specific file from stash@{stashIndex} into the working tree without popping the whole stash.
func (a *GitService) ApplyStashFile(repoPath string, stashIndex int, filePath string) (*StashFileApplyResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stashRef := fmt.Sprintf("stash@{%d}", stashIndex)
	cmd := exec.CommandContext(ctx, "git", "checkout", stashRef, "--", filePath)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to extract '%s' from %s: %s", filePath, stashRef, strings.TrimSpace(string(out)))
	}

	return &StashFileApplyResult{
		StashIndex: stashIndex,
		FilePath:   filePath,
		Success:    true,
		Message:    fmt.Sprintf("已成功从 %s 取回文件 '%s'", stashRef, filePath),
	}, nil
}
