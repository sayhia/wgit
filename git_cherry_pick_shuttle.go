package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CherryPickStepReport struct {
	CommitHash      string   `json:"commitHash"`
	ShortHash       string   `json:"shortHash"`
	Subject         string   `json:"subject"`
	Author          string   `json:"author"`
	CanApplyCleanly bool     `json:"canApplyCleanly"`
	ConflictedFiles []string `json:"conflictedFiles"`
}

type CherryPickSimulationReport struct {
	TotalCommits    int                    `json:"totalCommits"`
	CleanCommits    int                    `json:"cleanCommits"`
	ConflictCommits int                    `json:"conflictCommits"`
	ViabilityScore  int                    `json:"viabilityScore"`
	Steps           []CherryPickStepReport `json:"steps"`
	Summary         string                 `json:"summary"`
}

type CherryPickBatchResult struct {
	Success        bool     `json:"success"`
	AppliedCount   int      `json:"appliedCount"`
	AppliedCommits []string `json:"appliedCommits"`
	FailedCommit   string   `json:"failedCommit"`
	ErrorMessage   string   `json:"errorMessage"`
	Summary        string   `json:"summary"`
}

func execGitShuttle(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// SimulateCherryPick tests whether a list of commits can be applied cleanly onto HEAD.
func (GitService) SimulateCherryPick(repoPath string, targetCommits []string) (*CherryPickSimulationReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if len(targetCommits) == 0 {
		return &CherryPickSimulationReport{
			TotalCommits:    0,
			CleanCommits:    0,
			ConflictCommits: 0,
			ViabilityScore:  100,
			Steps:           []CherryPickStepReport{},
			Summary:         "No commits provided to simulate",
		}, nil
	}

	steps := make([]CherryPickStepReport, 0, len(targetCommits))
	cleanCount := 0
	conflictCount := 0

	for _, sha := range targetCommits {
		sha = strings.TrimSpace(sha)
		if sha == "" {
			continue
		}

		// Get metadata: shortHash, subject, author
		metaOut, err := execGitShuttle(repoPath, "log", "-1", "--format=%h%x00%s%x00%an", sha)
		shortHash := sha
		subject := "Commit " + sha
		author := "Unknown"
		if err == nil {
			parts := strings.Split(strings.TrimSpace(metaOut), "\x00")
			if len(parts) >= 1 && parts[0] != "" {
				shortHash = parts[0]
			}
			if len(parts) >= 2 {
				subject = parts[1]
			}
			if len(parts) >= 3 {
				author = parts[2]
			}
		}

		// Find parent sha for 3-way merge simulation: base = parent, our = HEAD, their = sha
		parentOut, pErr := execGitShuttle(repoPath, "rev-parse", sha+"^")
		parentSha := ""
		if pErr == nil {
			parentSha = strings.TrimSpace(parentOut)
		}

		canApply := false
		conflictedFiles := make([]string, 0)

		if parentSha != "" {
			// Modern merge-tree: git merge-tree base our their -> git merge-tree parent HEAD sha
			mergeOut, mErr := execGitShuttle(repoPath, "merge-tree", parentSha, "HEAD", sha)
			if mErr == nil && !strings.Contains(mergeOut, "<<<<<<<") && !strings.Contains(mergeOut, "CONFLICT") {
				canApply = true
			} else {
				// Parse conflicted file names
				lines := strings.Split(mergeOut, "\n")
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.Contains(trimmed, "conflict in ") || strings.Contains(trimmed, "Merge conflict in ") {
						parts := strings.Split(trimmed, " in ")
						if len(parts) == 2 {
							conflictedFiles = append(conflictedFiles, strings.TrimSpace(parts[1]))
						}
					}
				}
			}
		} else {
			// Root commit or fallback
			checkOut, cErr := execGitShuttle(repoPath, "merge-tree", "--write-tree", "HEAD", sha)
			if cErr == nil && !strings.Contains(checkOut, "CONFLICT") {
				canApply = true
			}
		}

		if canApply {
			cleanCount++
		} else {
			conflictCount++
		}

		steps = append(steps, CherryPickStepReport{
			CommitHash:      sha,
			ShortHash:       shortHash,
			Subject:         subject,
			Author:          author,
			CanApplyCleanly: canApply,
			ConflictedFiles: conflictedFiles,
		})
	}

	score := 100
	total := len(steps)
	if total > 0 {
		score = (cleanCount * 100) / total
	}

	summary := fmt.Sprintf("Simulated %d commits: %d clean, %d potential conflicts (viability: %d%%)", total, cleanCount, conflictCount, score)

	return &CherryPickSimulationReport{
		TotalCommits:    total,
		CleanCommits:    cleanCount,
		ConflictCommits: conflictCount,
		ViabilityScore:  score,
		Steps:           steps,
		Summary:         summary,
	}, nil
}

// ApplyCherryPickBatch applies a sequence of commits onto the current branch.
func (GitService) ApplyCherryPickBatch(repoPath string, targetCommits []string, recordOrigin bool, noCommit bool) (*CherryPickBatchResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if len(targetCommits) == 0 {
		return &CherryPickBatchResult{
			Success:        true,
			AppliedCount:   0,
			AppliedCommits: []string{},
			Summary:        "No commits to apply",
		}, nil
	}

	applied := make([]string, 0, len(targetCommits))

	for idx, sha := range targetCommits {
		sha = strings.TrimSpace(sha)
		if sha == "" {
			continue
		}

		args := []string{"cherry-pick"}
		if recordOrigin {
			args = append(args, "-x")
		}
		if noCommit {
			args = append(args, "-n")
		}
		args = append(args, sha)

		out, err := execGitShuttle(repoPath, args...)
		if err != nil {
			// Abort any paused/in-progress cherry-pick so repo remains in clean state
			_, _ = execGitShuttle(repoPath, "cherry-pick", "--abort")
			errMsg := fmt.Sprintf("Failed to cherry-pick %s: %s (%v)", sha, strings.TrimSpace(out), err)
			return &CherryPickBatchResult{
				Success:        false,
				AppliedCount:   idx,
				AppliedCommits: applied,
				FailedCommit:   sha,
				ErrorMessage:   errMsg,
				Summary:        fmt.Sprintf("Aborted at commit %d of %d", idx+1, len(targetCommits)),
			}, nil
		}
		applied = append(applied, sha)
	}

	return &CherryPickBatchResult{
		Success:        true,
		AppliedCount:   len(applied),
		AppliedCommits: applied,
		Summary:        fmt.Sprintf("Successfully cherry-picked %d commits", len(applied)),
	}, nil
}
