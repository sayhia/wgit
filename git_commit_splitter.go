package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type SplitCommitPlan struct {
	Message string   `json:"message"`
	Files   []string `json:"files"`
}

type SplitCommitResult struct {
	OriginalCommitSha string   `json:"originalCommitSha"`
	NewCommitShas     []string `json:"newCommitShas"`
	Success           bool     `json:"success"`
	CreatedCount      int      `json:"createdCount"`
	Summary           string   `json:"summary"`
}

func execGitSplit(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// DeconstructCommit takes a commit (HEAD or specified commit) and splits it into multiple atomic commits.
func (GitService) DeconstructCommit(repoPath string, commitSha string, splitPlans []SplitCommitPlan) (*SplitCommitResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	commitSha = strings.TrimSpace(commitSha)
	if commitSha == "" {
		commitSha = "HEAD"
	}
	if len(splitPlans) == 0 {
		return nil, errors.New("splitPlans must contain at least one plan")
	}

	fullShaOut, err := execGitSplit(repoPath, "rev-parse", "--verify", commitSha)
	if err != nil {
		return nil, fmt.Errorf("invalid commit sha %s: %s", commitSha, fullShaOut)
	}
	targetSha := strings.TrimSpace(fullShaOut)

	headShaOut, _ := execGitSplit(repoPath, "rev-parse", "HEAD")
	headSha := strings.TrimSpace(headShaOut)

	if targetSha != headSha {
		// Non-HEAD commit splitting can be executed when clean
		return &SplitCommitResult{
			OriginalCommitSha: targetSha,
			NewCommitShas:     []string{},
			Success:           false,
			CreatedCount:      0,
			Summary:           fmt.Sprintf("Commit %s is not HEAD. Please checkout or rebase to this commit first before splitting.", targetSha[:7]),
		}, nil
	}

	// 1. Soft reset to HEAD~1 to keep all changes in index
	_, err = execGitSplit(repoPath, "reset", "--soft", "HEAD~1")
	if err != nil {
		return nil, fmt.Errorf("failed to soft reset: %v", err)
	}

	// 2. Unstage all changes so files can be added atomically
	_, _ = execGitSplit(repoPath, "reset", "HEAD")

	var newShas []string

	// 3. Incrementally add files for each split plan
	for _, plan := range splitPlans {
		if len(plan.Files) == 0 {
			continue
		}
		addArgs := append([]string{"add", "--"}, plan.Files...)
		_, err := execGitSplit(repoPath, addArgs...)
		if err != nil {
			continue
		}

		msg := strings.TrimSpace(plan.Message)
		if msg == "" {
			msg = "split: atomic changes"
		}

		commitOut, err := execGitSplit(repoPath, "commit", "-m", msg)
		if err == nil {
			newHead, _ := execGitSplit(repoPath, "rev-parse", "HEAD")
			newShas = append(newShas, strings.TrimSpace(newHead))
		} else {
			_ = commitOut
		}
	}

	// 4. If any remaining files left uncommitted, commit them as remaining
	statusOut, _ := execGitSplit(repoPath, "status", "--porcelain")
	if strings.TrimSpace(statusOut) != "" {
		_, _ = execGitSplit(repoPath, "add", "-A")
		_, _ = execGitSplit(repoPath, "commit", "-m", "split: remaining changes")
		remainingHead, _ := execGitSplit(repoPath, "rev-parse", "HEAD")
		newShas = append(newShas, strings.TrimSpace(remainingHead))
	}

	summary := fmt.Sprintf("Successfully deconstructed commit %s into %d atomic commits", targetSha[:7], len(newShas))

	return &SplitCommitResult{
		OriginalCommitSha: targetSha,
		NewCommitShas:     newShas,
		Success:           len(newShas) > 0,
		CreatedCount:      len(newShas),
		Summary:           summary,
	}, nil
}
