package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// MergeSimulationResult represents the dry-run outcome of merging targetRef into baseRef.
type MergeSimulationResult struct {
	BaseRef         string   `json:"baseRef"`
	TargetRef       string   `json:"targetRef"`
	CanMergeCleanly bool     `json:"canMergeCleanly"`
	HasConflicts    bool     `json:"hasConflicts"`
	ConflictedFiles []string `json:"conflictedFiles"`
	ChangedFiles    []string `json:"changedFiles"`
	MergeBase       string   `json:"mergeBase"`
	IsFastForward   bool     `json:"isFastForward"`
	TreeHash        string   `json:"treeHash,omitempty"`
	Message         string   `json:"message"`
}

// SimulateMerge runs a dry-run merge simulation between baseRef and targetRef without touching the worktree.
func (a *GitService) SimulateMerge(repoPath string, baseRef string, targetRef string) (*MergeSimulationResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	baseRef = strings.TrimSpace(baseRef)
	targetRef = strings.TrimSpace(targetRef)
	if baseRef == "" || targetRef == "" {
		return nil, fmt.Errorf("both baseRef and targetRef are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Resolve commit hashes
	cmdBaseHash := exec.CommandContext(ctx, "git", "rev-parse", "--verify", baseRef)
	cmdBaseHash.Dir = repoPath
	baseHashBytes, err := cmdBaseHash.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve baseRef %q: %w", baseRef, err)
	}
	baseHash := strings.TrimSpace(string(baseHashBytes))

	cmdTargetHash := exec.CommandContext(ctx, "git", "rev-parse", "--verify", targetRef)
	cmdTargetHash.Dir = repoPath
	targetHashBytes, err := cmdTargetHash.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve targetRef %q: %w", targetRef, err)
	}
	targetHash := strings.TrimSpace(string(targetHashBytes))

	// 2. Find common ancestor (merge-base)
	cmdMB := exec.CommandContext(ctx, "git", "merge-base", baseRef, targetRef)
	cmdMB.Dir = repoPath
	mbBytes, err := cmdMB.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find merge-base between %q and %q: %w", baseRef, targetRef, err)
	}
	mergeBase := strings.TrimSpace(string(mbBytes))

	result := &MergeSimulationResult{
		BaseRef:         baseRef,
		TargetRef:       targetRef,
		MergeBase:       mergeBase,
		ConflictedFiles: []string{},
		ChangedFiles:    []string{},
	}

	// Case A: target is already merged into base (already up-to-date)
	if mergeBase == targetHash {
		result.CanMergeCleanly = true
		result.HasConflicts = false
		result.IsFastForward = false
		result.Message = "Already up to date"
		return result, nil
	}

	// Case B: base is an ancestor of target (Fast-Forward merge)
	if mergeBase == baseHash {
		result.CanMergeCleanly = true
		result.HasConflicts = false
		result.IsFastForward = true
		result.Message = "Fast-forward merge possible"

		// Get changed files
		cmdDiff := exec.CommandContext(ctx, "git", "diff", "--name-only", baseRef, targetRef)
		cmdDiff.Dir = repoPath
		if diffOut, err := cmdDiff.Output(); err == nil {
			for _, line := range strings.Split(string(diffOut), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					result.ChangedFiles = append(result.ChangedFiles, line)
				}
			}
		}
		return result, nil
	}

	// Case C: 3-way merge using git merge-tree --write-tree
	cmdMT := exec.CommandContext(ctx, "git", "merge-tree", "--write-tree", baseRef, targetRef)
	cmdMT.Dir = repoPath
	var stdoutBuf, stderrBuf bytes.Buffer
	cmdMT.Stdout = &stdoutBuf
	cmdMT.Stderr = &stderrBuf

	mtErr := cmdMT.Run()
	stdoutStr := stdoutBuf.String()

	if mtErr == nil {
		// Clean merge
		lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
		if len(lines) > 0 {
			result.TreeHash = strings.TrimSpace(lines[0])
		}
		result.CanMergeCleanly = true
		result.HasConflicts = false
		result.Message = "Clean 3-way merge"

		// Query changed files between base and the merged tree
		if result.TreeHash != "" {
			cmdTreeDiff := exec.CommandContext(ctx, "git", "diff-tree", "-r", "--name-only", baseRef, result.TreeHash)
			cmdTreeDiff.Dir = repoPath
			if tdOut, err := cmdTreeDiff.Output(); err == nil {
				for _, line := range strings.Split(string(tdOut), "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						result.ChangedFiles = append(result.ChangedFiles, line)
					}
				}
			}
		}
		return result, nil
	}

	// Conflicted merge
	result.CanMergeCleanly = false
	result.HasConflicts = true

	conflictMap := make(map[string]bool)
	reConflict := regexp.MustCompile(`(?i)CONFLICT\s*\([^)]*\):\s*(?:Merge conflict in\s+)?(.+)`)
	reStageEntry := regexp.MustCompile(`^\d+\s+[a-f0-9]+\s+[123]\t(.+)$`)

	for _, line := range strings.Split(stdoutStr, "\n") {
		line = strings.TrimSpace(line)
		if m := reConflict.FindStringSubmatch(line); len(m) > 1 {
			filePath := strings.TrimSpace(m[1])
			if filePath != "" {
				conflictMap[filePath] = true
			}
		} else if m := reStageEntry.FindStringSubmatch(line); len(m) > 1 {
			filePath := strings.TrimSpace(m[1])
			if filePath != "" {
				conflictMap[filePath] = true
			}
		}
	}

	for f := range conflictMap {
		result.ConflictedFiles = append(result.ConflictedFiles, f)
	}
	result.Message = fmt.Sprintf("%d conflicted file(s) detected", len(result.ConflictedFiles))
	return result, nil
}
