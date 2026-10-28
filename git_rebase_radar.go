package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RebaseStepRisk struct {
	Hash             string   `json:"hash"`
	ShortHash        string   `json:"shortHash"`
	Subject          string   `json:"subject"`
	Status           string   `json:"status"`    // "clean", "trivial_merge", "conflict"
	RiskLevel        string   `json:"riskLevel"` // "low", "medium", "high"
	ConflictingFiles []string `json:"conflictingFiles"`
	ChangedFiles     []string `json:"changedFiles"`
}

type RebaseSimulationReport struct {
	UpstreamRef    string           `json:"upstreamRef"`
	TotalSteps     int              `json:"totalSteps"`
	CleanSteps     int              `json:"cleanSteps"`
	ConflictSteps  int              `json:"conflictSteps"`
	ViabilityScore int              `json:"viabilityScore"` // 0 - 100
	Steps          []RebaseStepRisk `json:"steps"`
	Summary        string           `json:"summary"`
}

type RebaseTodoValidationReport struct {
	Valid       bool     `json:"valid"`
	Errors      []string `json:"errors"`
	Warnings    []string `json:"warnings"`
	Suggestions []string `json:"suggestions"`
}

func execGitRadar(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// SimulateRebaseOnto simulates stepping through a sequence of commits rebased onto upstreamRef.
func (GitService) SimulateRebaseOnto(repoPath string, upstreamRef string, commitHashes []string) (*RebaseSimulationReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	upstreamRef = strings.TrimSpace(upstreamRef)
	if upstreamRef == "" {
		upstreamRef = "HEAD"
	}

	cleanSteps := 0
	conflictSteps := 0
	steps := make([]RebaseStepRisk, 0, len(commitHashes))

	// Get files modified on upstream relative to common base if possible
	upstreamFilesMap := make(map[string]bool)
	if len(commitHashes) > 0 {
		firstCommit := commitHashes[0]
		mbOut, _ := execGitRadar(repoPath, "merge-base", upstreamRef, firstCommit)
		mergeBase := strings.TrimSpace(mbOut)
		if mergeBase != "" {
			diffOut, _ := execGitRadar(repoPath, "diff", "--name-only", mergeBase, upstreamRef)
			for _, line := range strings.Split(diffOut, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					upstreamFilesMap[line] = true
				}
			}
		}
	}

	currentBase := upstreamRef

	for _, commitHash := range commitHashes {
		commitHash = strings.TrimSpace(commitHash)
		if commitHash == "" {
			continue
		}

		shortHash := commitHash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		subjOut, _ := execGitRadar(repoPath, "log", "-1", "--format=%s", commitHash)
		subject := strings.TrimSpace(subjOut)

		filesOut, _ := execGitRadar(repoPath, "show", "--name-only", "--format=", commitHash)
		var changedFiles []string
		for _, f := range strings.Split(filesOut, "\n") {
			f = strings.TrimSpace(f)
			if f != "" {
				changedFiles = append(changedFiles, f)
			}
		}

		// Check merge-tree or diff intersection
		conflictingFiles := make([]string, 0)
		status := "clean"
		riskLevel := "low"

		// Test merge-tree
		hasOverlap := false
		for _, cf := range changedFiles {
			if upstreamFilesMap[cf] {
				hasOverlap = true
				break
			}
		}

		// Try git merge-tree --write-tree currentBase commitHash
		mtOut, mtErr := execGitRadar(repoPath, "merge-tree", "--write-tree", currentBase, commitHash)
		if mtErr != nil || strings.Contains(mtOut, "CONFLICT") {
			// Try 3-way merge-tree syntax if --write-tree is not supported
			mbOut, _ := execGitRadar(repoPath, "merge-base", currentBase, commitHash)
			mb := strings.TrimSpace(mbOut)
			if mb != "" {
				mtLegacyOut, _ := execGitRadar(repoPath, "merge-tree", mb, currentBase, commitHash)
				if strings.Contains(mtLegacyOut, "<<<<<<<") || strings.Contains(mtLegacyOut, "CONFLICT") {
					status = "conflict"
					riskLevel = "high"
				}
			} else {
				status = "conflict"
				riskLevel = "high"
			}
			if status == "conflict" {
				for _, cf := range changedFiles {
					if upstreamFilesMap[cf] {
						conflictingFiles = append(conflictingFiles, cf)
					}
				}
				if len(conflictingFiles) == 0 && len(changedFiles) > 0 {
					conflictingFiles = append(conflictingFiles, changedFiles[0])
				}
			}
		} else if hasOverlap {
			status = "trivial_merge"
			riskLevel = "medium"
		}

		if status == "conflict" {
			conflictSteps++
		} else {
			cleanSteps++
			// Track that this commit's changes are now part of the upstream branch simulation
			for _, cf := range changedFiles {
				upstreamFilesMap[cf] = true
			}
			currentBase = commitHash
		}

		steps = append(steps, RebaseStepRisk{
			Hash:             commitHash,
			ShortHash:        shortHash,
			Subject:          subject,
			Status:           status,
			RiskLevel:        riskLevel,
			ConflictingFiles: conflictingFiles,
			ChangedFiles:     changedFiles,
		})
	}

	total := len(steps)
	score := 100
	if total > 0 {
		score = int(float64(cleanSteps) / float64(total) * 100)
		if conflictSteps > 0 {
			penalty := conflictSteps * 30
			score -= penalty
			if score < 0 {
				score = 0
			}
		}
	}

	summary := fmt.Sprintf("Simulated rebase of %d commits onto %s: %d clean, %d conflicts (Viability: %d%%)",
		total, upstreamRef, cleanSteps, conflictSteps, score)

	return &RebaseSimulationReport{
		UpstreamRef:    upstreamRef,
		TotalSteps:     total,
		CleanSteps:     cleanSteps,
		ConflictSteps:  conflictSteps,
		ViabilityScore: score,
		Steps:          steps,
		Summary:        summary,
	}, nil
}

// ValidateRebaseTodoGraph validates custom rebase todo items for syntax, sequence, and dependency issues.
func (GitService) ValidateRebaseTodoGraph(repoPath string, todoItems []RebaseTodoItem) (*RebaseTodoValidationReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	report := &RebaseTodoValidationReport{
		Valid:       true,
		Errors:      make([]string, 0),
		Warnings:    make([]string, 0),
		Suggestions: make([]string, 0),
	}

	if len(todoItems) == 0 {
		report.Warnings = append(report.Warnings, "Rebase todo list is empty; rebasing an empty list results in a no-op.")
		return report, nil
	}

	// 1. Check if first commit is squash or fixup
	firstAction := strings.ToLower(strings.TrimSpace(todoItems[0].Action))
	if firstAction == "squash" || firstAction == "fixup" {
		report.Valid = false
		report.Errors = append(report.Errors, fmt.Sprintf("First commit (%s) cannot be '%s' without a preceding base commit", todoItems[0].Hash, firstAction))
		report.Suggestions = append(report.Suggestions, "Change first action to 'pick' or reorder commits.")
	}

	// 2. Check if all items are dropped
	allDropped := true
	for _, it := range todoItems {
		act := strings.ToLower(strings.TrimSpace(it.Action))
		if act != "drop" {
			allDropped = false
			break
		}
	}
	if allDropped {
		report.Warnings = append(report.Warnings, "All commits are marked 'drop'. This will remove all commits in this range.")
	}

	// 3. Track dropped files and check if subsequent commits depend on them
	droppedFileMap := make(map[string]string) // file -> droppedCommitHash
	for idx, it := range todoItems {
		act := strings.ToLower(strings.TrimSpace(it.Action))
		commitHash := strings.TrimSpace(it.Hash)
		if commitHash == "" {
			continue
		}

		filesOut, _ := execGitRadar(repoPath, "show", "--name-only", "--format=", commitHash)
		lines := strings.Split(filesOut, "\n")
		var currentFiles []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				currentFiles = append(currentFiles, l)
			}
		}

		if act == "drop" {
			for _, f := range currentFiles {
				droppedFileMap[f] = commitHash
			}
		} else {
			// Check if this commit touches any file previously dropped
			for _, f := range currentFiles {
				if droppedBy, exists := droppedFileMap[f]; exists {
					report.Warnings = append(report.Warnings,
						fmt.Sprintf("Step %d (%s) modifies '%s' which was dropped in earlier commit %s. This may cause merge conflicts.",
							idx+1, commitHash, filepath.Base(f), droppedBy))
					report.Suggestions = append(report.Suggestions, fmt.Sprintf("Review modifications to '%s' or keep commit %s.", filepath.Base(f), droppedBy))
					delete(droppedFileMap, f) // warn once per file
				}
			}
		}
	}

	if len(report.Errors) > 0 {
		report.Valid = false
	}

	return report, nil
}
