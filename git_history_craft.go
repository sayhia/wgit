package main

import (
	"errors"
	"fmt"
	"strings"
)

// SquashResult describes the outcome of a squash operation.
type SquashResult struct {
	Success bool   `json:"success"`
	NewHash string `json:"newHash"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

func toShortSha(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// SquashSelectedCommits squashes a contiguous list of commits into a single commit with custom message.
func (s GitService) SquashSelectedCommits(repoPath string, commitHashes []string, message string) (*SquashResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path cannot be empty")
	}
	if len(commitHashes) < 2 {
		return nil, errors.New("at least 2 commits are required for squash")
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return nil, errors.New("commit message cannot be empty")
	}

	// Verify working tree is clean before squashing
	status, err := s.GetRepoStatus(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check repository status: %w", err)
	}
	if !status.IsClean {
		return nil, errors.New("working tree must be clean before squashing commits; please commit or stash changes")
	}

	// Verify the commits match the top N commits from HEAD
	headSha, err := ExecGit(repoPath, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD sha: %w", err)
	}
	headSha = strings.TrimSpace(headSha)

	// Fetch log for the specified number of commits
	count := len(commitHashes)
	logLines, err := ExecGit(repoPath, "log", fmt.Sprintf("-%d", count), "--pretty=format:%H")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve commit history: %w", err)
	}
	actualHashes := strings.Split(strings.TrimSpace(logLines), "\n")

	// If hashes match the top N commits directly, soft-reset and commit
	isTopN := len(actualHashes) == count
	if isTopN {
		for i, h := range actualHashes {
			if !strings.HasPrefix(h, commitHashes[i]) && !strings.HasPrefix(commitHashes[i], h) {
				isTopN = false
				break
			}
		}
	}

	if isTopN {
		// Reset soft to the parent of the oldest commit
		_, err := ExecGit(repoPath, "reset", "--soft", fmt.Sprintf("HEAD~%d", count))
		if err != nil {
			return nil, fmt.Errorf("soft reset failed: %w", err)
		}

		out, err := ExecGit(repoPath, "commit", "-m", msg)
		if err != nil {
			return nil, fmt.Errorf("squash commit failed: %w", err)
		}

		newSha, _ := ExecGit(repoPath, "rev-parse", "HEAD")
		newSha = strings.TrimSpace(newSha)

		return &SquashResult{
			Success: true,
			NewHash: newSha,
			Message: fmt.Sprintf("成功将 %d 个提交合并为 %s: %s", count, toShortSha(newSha), firstLine(out)),
			Count:   count,
		}, nil
	}

	// Fallback to standard SquashCommits count
	out, err := s.SquashCommits(repoPath, count, msg)
	if err != nil {
		return nil, err
	}

	newSha, _ := ExecGit(repoPath, "rev-parse", "HEAD")
	newSha = strings.TrimSpace(newSha)

	return &SquashResult{
		Success: true,
		NewHash: newSha,
		Message: out,
		Count:   count,
	}, nil
}
