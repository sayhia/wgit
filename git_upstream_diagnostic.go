package main

import (
	"errors"
	"fmt"
	"strings"
)

// GoneBranchInfo represents a local branch whose upstream tracking branch has been deleted on remote.
type GoneBranchInfo struct {
	Name              string `json:"name"`
	Upstream          string `json:"upstream"`
	LastCommitHash    string `json:"lastCommitHash"`
	LastCommitSubject string `json:"lastCommitSubject"`
}

// GetGoneLocalBranches identifies local branches whose remote tracking upstream has been deleted (status contains "[gone]").
func (GitService) GetGoneLocalBranches(repoPath string) ([]GoneBranchInfo, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path cannot be empty")
	}

	format := "%(refname:short)|%(upstream:short)|%(upstream:track)|%(objectname:short)|%(contents:subject)"
	out, err := ExecGit(repoPath, "for-each-ref", "--format="+format, "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var results []GoneBranchInfo
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		branchName := strings.TrimSpace(parts[0])
		upstream := strings.TrimSpace(parts[1])
		track := strings.TrimSpace(parts[2])
		commitHash := strings.TrimSpace(parts[3])
		subject := strings.TrimSpace(parts[4])

		if strings.Contains(track, "[gone]") {
			results = append(results, GoneBranchInfo{
				Name:              branchName,
				Upstream:          upstream,
				LastCommitHash:    commitHash,
				LastCommitSubject: subject,
			})
		}
	}

	return results, nil
}
