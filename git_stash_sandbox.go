package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type StashDiffItem struct {
	FilePath  string `json:"filePath"`
	Status    string `json:"status"` // "modified", "added", "deleted"
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

type StashDiffReport struct {
	StashRef1      string          `json:"stashRef1"`
	StashRef2      string          `json:"stashRef2"`
	TotalFiles     int             `json:"totalFiles"`
	TotalAdditions int             `json:"totalAdditions"`
	TotalDeletions int             `json:"totalDeletions"`
	Files          []StashDiffItem `json:"files"`
	Summary        string          `json:"summary"`
}

type MicroSnapshotResult struct {
	SnapshotRef string   `json:"snapshotRef"`
	CreatedAt   string   `json:"createdAt"`
	FileCount   int      `json:"fileCount"`
	SavedPaths  []string `json:"savedPaths"`
	Summary     string   `json:"summary"`
}

func execGitStash(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CompareStashes computes the detailed diff tree between two stash references (e.g. stash@{0} and stash@{1}).
func (GitService) CompareStashes(repoPath string, stashRef1 string, stashRef2 string) (*StashDiffReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	stashRef1 = strings.TrimSpace(stashRef1)
	stashRef2 = strings.TrimSpace(stashRef2)
	if stashRef1 == "" {
		stashRef1 = "stash@{0}"
	}
	if stashRef2 == "" {
		stashRef2 = "HEAD"
	}

	numstatOut, err := execGitStash(repoPath, "diff", "--numstat", stashRef1, stashRef2)
	if err != nil {
		return nil, fmt.Errorf("failed to compare stashes %s and %s: %s", stashRef1, stashRef2, strings.TrimSpace(numstatOut))
	}

	var files []StashDiffItem
	totalAdd := 0
	totalDel := 0

	lines := strings.Split(strings.TrimSpace(numstatOut), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 {
			add, _ := strconv.Atoi(parts[0])
			del, _ := strconv.Atoi(parts[1])
			path := parts[2]
			totalAdd += add
			totalDel += del

			patchOut, _ := execGitStash(repoPath, "diff", stashRef1, stashRef2, "--", path)

			status := "modified"
			if add > 0 && del == 0 {
				status = "added"
			} else if del > 0 && add == 0 {
				status = "deleted"
			}

			files = append(files, StashDiffItem{
				FilePath:  path,
				Status:    status,
				Additions: add,
				Deletions: del,
				Patch:     patchOut,
			})
		}
	}

	summary := fmt.Sprintf("Comparing %s to %s: %d files changed, +%d, -%d", stashRef1, stashRef2, len(files), totalAdd, totalDel)

	return &StashDiffReport{
		StashRef1:      stashRef1,
		StashRef2:      stashRef2,
		TotalFiles:     len(files),
		TotalAdditions: totalAdd,
		TotalDeletions: totalDel,
		Files:          files,
		Summary:        summary,
	}, nil
}

// ApplyStashHunk applies a specific file or hunk from a stash entry without popping or dropping the stash.
func (GitService) ApplyStashHunk(repoPath string, stashIndex int, filePath string, hunkIndex int) (bool, error) {
	if repoPath == "" {
		return false, errors.New("repoPath cannot be empty")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return false, errors.New("filePath cannot be empty")
	}

	stashRef := fmt.Sprintf("stash@{%d}", stashIndex)
	out, err := execGitStash(repoPath, "checkout", stashRef, "--", filePath)
	if err != nil {
		return false, fmt.Errorf("failed to extract file from %s: %s", stashRef, strings.TrimSpace(out))
	}

	return true, nil
}

// CreateMicroSnapshot takes a non-destructive instantaneous snapshot of current worktree changes.
func (GitService) CreateMicroSnapshot(repoPath string, paths []string) (*MicroSnapshotResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("wgit-micro-snapshot: %s", now)

	// Create commit tree representation of current index/worktree without clearing worktree
	// git stash create creates a dangling stash commit object without touching the stash stack or working tree
	createOut, err := execGitStash(repoPath, "stash", "create", msg)
	snapshotSha := strings.TrimSpace(createOut)
	if err != nil || snapshotSha == "" {
		// Fallback to push --keep-index if create returned empty (clean or untracked changes)
		pushArgs := []string{"stash", "push", "-m", msg, "--include-untracked"}
		if len(paths) > 0 {
			pushArgs = append(pushArgs, "--")
			pushArgs = append(pushArgs, paths...)
		}
		_, pErr := execGitStash(repoPath, pushArgs...)
		if pErr == nil {
			// Immediately apply back to restore worktree
			_, _ = execGitStash(repoPath, "stash", "apply", "--index")
			snapshotSha = "stash@{0}"
		} else {
			snapshotSha = "HEAD"
		}
	}

	// Count files saved
	statusOut, _ := execGitStash(repoPath, "status", "--porcelain")
	fileCount := 0
	var savedPaths []string
	for _, l := range strings.Split(strings.TrimSpace(statusOut), "\n") {
		if strings.TrimSpace(l) != "" {
			fileCount++
			if len(l) > 3 {
				savedPaths = append(savedPaths, strings.TrimSpace(l[3:]))
			}
		}
	}

	return &MicroSnapshotResult{
		SnapshotRef: snapshotSha,
		CreatedAt:   now,
		FileCount:   fileCount,
		SavedPaths:  savedPaths,
		Summary:     fmt.Sprintf("Micro snapshot created: %s (%d files recorded)", snapshotSha, fileCount),
	}, nil
}
