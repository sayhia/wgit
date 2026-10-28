package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// LineStageResult reports the status of staging or unstaging selected diff lines.
type LineStageResult struct {
	FilePath   string `json:"filePath"`
	LinesCount int    `json:"linesCount"`
	IsUnstage  bool   `json:"isUnstage"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
}

// StageSelectedPatch applies a targeted unified patch fragment representing selected lines to the git index.
func (a *GitService) StageSelectedPatch(repoPath, filePath, patchContent string, isUnstage bool) (*LineStageResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	patchContent = strings.TrimSpace(patchContent)
	if patchContent == "" {
		return nil, fmt.Errorf("patch content cannot be empty")
	}
	if !strings.HasSuffix(patchContent, "\n") {
		patchContent += "\n"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var args []string
	if isUnstage {
		args = []string{"apply", "--cached", "--reverse", "--unidiff-zero", "-"}
	} else {
		args = []string{"apply", "--cached", "--unidiff-zero", "-"}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader(patchContent)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderrBuf.String())
		if errStr != "" {
			return nil, fmt.Errorf("git apply failed: %s", errStr)
		}
		return nil, fmt.Errorf("git apply failed: %w", err)
	}

	lineCount := strings.Count(patchContent, "\n+") + strings.Count(patchContent, "\n-")
	if lineCount == 0 {
		lineCount = 1
	}

	msg := "成功暂存所选代码行"
	if isUnstage {
		msg = "成功从暂存区撤出所选代码行"
	}

	return &LineStageResult{
		FilePath:   filePath,
		LinesCount: lineCount,
		IsUnstage:  isUnstage,
		Success:    true,
		Message:    msg,
	}, nil
}
