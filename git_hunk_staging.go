package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StagePatchHunk applies a single diff patch (or hunk) to the git index, or unstages it if unstage is true.
func (a *GitService) StagePatchHunk(repoPath string, patchText string, unstage bool) error {
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}
	patchText = strings.TrimSpace(patchText)
	if patchText == "" {
		return fmt.Errorf("patch text cannot be empty")
	}
	if !strings.HasSuffix(patchText, "\n") {
		patchText += "\n"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var args []string
	if unstage {
		args = []string{"apply", "--cached", "--reverse", "-"}
	} else {
		args = []string{"apply", "--cached", "-"}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader(patchText)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderrBuf.String())
		if errStr != "" {
			return fmt.Errorf("git apply failed: %s", errStr)
		}
		return fmt.Errorf("git apply failed: %w", err)
	}
	return nil
}

// DiscardPatchHunk reverses a single diff hunk from the working tree.
func (a *GitService) DiscardPatchHunk(repoPath string, patchText string) error {
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}
	patchText = strings.TrimSpace(patchText)
	if patchText == "" {
		return fmt.Errorf("patch text cannot be empty")
	}
	if !strings.HasSuffix(patchText, "\n") {
		patchText += "\n"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "apply", "--reverse", "-")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader(patchText)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderrBuf.String())
		if errStr != "" {
			return fmt.Errorf("git apply --reverse failed: %s", errStr)
		}
		return fmt.Errorf("git apply --reverse failed: %w", err)
	}
	return nil
}
