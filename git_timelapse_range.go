package main

import (
	"fmt"
	"strconv"
	"strings"
)

// GetFileCommitRangeDiff compares a specific file between two historical commit hashes.
func (a *GitService) GetFileCommitRangeDiff(repoPath string, filePath string, fromHash string, toHash string) (*DiffComparisonResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}
	fromHash = strings.TrimSpace(fromHash)
	toHash = strings.TrimSpace(toHash)
	if fromHash == "" || toHash == "" {
		return nil, fmt.Errorf("both fromHash and toHash are required")
	}

	rangeArg := fmt.Sprintf("%s..%s", fromHash, toHash)
	rawStat, err := ExecGit(repoPath, "diff", "--numstat", rangeArg, "--", filePath)
	if err != nil {
		return nil, err
	}

	lfsExtensions := getLFSExtensions(repoPath)
	var files []FileStatus
	totalAdd := 0
	totalDel := 0

	for _, line := range strings.Split(rawStat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pts := strings.Split(line, "\t")
		if len(pts) >= 3 {
			add, _ := strconv.Atoi(pts[0])
			del, _ := strconv.Atoi(pts[1])
			fPath := pts[2]
			totalAdd += add
			totalDel += del
			files = append(files, FileStatus{
				Path:      fPath,
				Additions: add,
				Deletions: del,
				IsLFS:     isPathLFS(fPath, lfsExtensions),
			})
		}
	}

	rawDiff, err := ExecGit(repoPath, "diff", "--patch", rangeArg, "--", filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read range diff: %w", err)
	}
	diffText, _ := truncateDiffPayload(rawDiff)

	return &DiffComparisonResult{
		RefA:      fromHash,
		RefB:      toHash,
		Files:     files,
		Diff:      diffText,
		Additions: totalAdd,
		Deletions: totalDel,
	}, nil
}
