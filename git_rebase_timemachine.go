package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ReplayCommitNode struct {
	CommitHash       string   `json:"commitHash"`
	ShortHash        string   `json:"shortHash"`
	Subject          string   `json:"subject"`
	Author           string   `json:"author"`
	Date             string   `json:"date"`
	PredictedStatus  string   `json:"predictedStatus"` // "clean", "conflict"
	ConflictingFiles []string `json:"conflictingFiles"`
	ChangedFiles     []string `json:"changedFiles"`
}

type RebaseTimeMachineReport struct {
	BaseBranch         string             `json:"baseBranch"`
	TargetBranch       string             `json:"targetBranch"`
	CommonBaseSha      string             `json:"commonBaseSha"`
	CommitsToReplay    []ReplayCommitNode `json:"commitsToReplay"`
	TotalCommits       int                `json:"totalCommits"`
	CleanCount         int                `json:"cleanCount"`
	ConflictCount      int                `json:"conflictCount"`
	OverallFeasibility int                `json:"overallFeasibility"`
	Summary            string             `json:"summary"`
}

func execGitTimeMachine(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// SimulateRebaseTimeMachine simulates replaying ontoBranch commits on top of upstreamBranch.
func (GitService) SimulateRebaseTimeMachine(repoPath string, upstreamBranch string, ontoBranch string) (*RebaseTimeMachineReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	upstreamBranch = strings.TrimSpace(upstreamBranch)
	ontoBranch = strings.TrimSpace(ontoBranch)
	if upstreamBranch == "" || ontoBranch == "" {
		return nil, errors.New("both upstreamBranch and ontoBranch must be specified")
	}

	// 1. Get common merge base
	baseOut, err := execGitTimeMachine(repoPath, "merge-base", upstreamBranch, ontoBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to find merge-base between %s and %s: %v", upstreamBranch, ontoBranch, err)
	}
	commonBaseSha := strings.TrimSpace(baseOut)

	// 2. Commits on ontoBranch not on upstreamBranch
	logOut, err := execGitTimeMachine(repoPath, "log", "--reverse", "--format=%H%x00%h%x00%s%x00%an%x00%ad", "--date=short", fmt.Sprintf("%s..%s", upstreamBranch, ontoBranch))
	if err != nil {
		return nil, fmt.Errorf("failed to list commits to rebase: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(logOut), "\n")
	nodes := make([]ReplayCommitNode, 0)
	cleanCount := 0
	conflictCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts := strings.Split(trimmed, "\x00")
		if len(parts) < 3 {
			continue
		}
		fullSha := parts[0]
		shortSha := parts[1]
		subject := parts[2]
		author := ""
		date := ""
		if len(parts) >= 4 {
			author = parts[3]
		}
		if len(parts) >= 5 {
			date = parts[4]
		}

		// Changed files for this commit
		diffFilesOut, _ := execGitTimeMachine(repoPath, "diff-tree", "--no-commit-id", "--name-only", "-r", fullSha)
		changedFiles := make([]string, 0)
		for _, f := range strings.Split(strings.TrimSpace(diffFilesOut), "\n") {
			if strings.TrimSpace(f) != "" {
				changedFiles = append(changedFiles, strings.TrimSpace(f))
			}
		}

		// 3-way merge check: parent of fullSha, upstreamBranch, fullSha
		parentOut, pErr := execGitTimeMachine(repoPath, "rev-parse", fullSha+"^")
		parentSha := ""
		if pErr == nil {
			parentSha = strings.TrimSpace(parentOut)
		}

		isClean := true
		conflictedFiles := make([]string, 0)

		if parentSha != "" {
			mergeOut, mErr := execGitTimeMachine(repoPath, "merge-tree", parentSha, upstreamBranch, fullSha)
			if mErr != nil || strings.Contains(mergeOut, "<<<<<<<") || strings.Contains(mergeOut, "CONFLICT") {
				isClean = false
				linesM := strings.Split(mergeOut, "\n")
				for _, l := range linesM {
					if strings.Contains(l, "Merge conflict in ") || strings.Contains(l, "conflict in ") {
						parts := strings.Split(l, " in ")
						if len(parts) == 2 {
							conflictedFiles = append(conflictedFiles, strings.TrimSpace(parts[1]))
						}
					}
				}
			}
		}

		status := "clean"
		if isClean {
			cleanCount++
		} else {
			conflictCount++
			status = "conflict"
		}

		nodes = append(nodes, ReplayCommitNode{
			CommitHash:       fullSha,
			ShortHash:        shortSha,
			Subject:          subject,
			Author:           author,
			Date:             date,
			PredictedStatus:  status,
			ConflictingFiles: conflictedFiles,
			ChangedFiles:     changedFiles,
		})
	}

	feasibility := 100
	total := len(nodes)
	if total > 0 {
		feasibility = (cleanCount * 100) / total
	}

	summary := fmt.Sprintf("Rebase simulation of %s onto %s: %d commits (%d clean, %d conflicts, %d%% feasibility)", ontoBranch, upstreamBranch, total, cleanCount, conflictCount, feasibility)

	return &RebaseTimeMachineReport{
		BaseBranch:         upstreamBranch,
		TargetBranch:       ontoBranch,
		CommonBaseSha:      commonBaseSha,
		CommitsToReplay:    nodes,
		TotalCommits:       total,
		CleanCount:         cleanCount,
		ConflictCount:      conflictCount,
		OverallFeasibility: feasibility,
		Summary:            summary,
	}, nil
}
