package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// buildDiffComparison runs numstat + patch for the given git diff range and
// assembles the shared comparison payload.
func buildDiffComparison(repoPath string, rangeArg string, refA string, refB string) (*DiffComparisonResult, error) {
	rawStat, err := ExecGit(repoPath, "diff", "--numstat", rangeArg)
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
			filePath := pts[2]
			totalAdd += add
			totalDel += del
			files = append(files, FileStatus{
				Path:      filePath,
				Additions: add,
				Deletions: del,
				IsLFS:     isPathLFS(filePath, lfsExtensions),
			})
		}
	}

	rawDiff, err := ExecGit(repoPath, "diff", "--patch", rangeArg)
	if err != nil {
		return nil, fmt.Errorf("读取比对差异失败: %w", err)
	}
	diffText, _ := truncateDiffPayload(rawDiff)

	return &DiffComparisonResult{
		RefA:      refA,
		RefB:      refB,
		Files:     files,
		Additions: totalAdd,
		Deletions: totalDel,
		Diff:      diffText,
	}, nil
}

// GetMergeBaseDiff compares refA against the merge-base of refA and refB
// (three-dot `refA...refB`), i.e. only the changes introduced on refB's side
// since the branches diverged. This is the common "compare branch to main"
// semantics used by hosting providers.
func (GitService) GetMergeBaseDiff(repoPath string, refA string, refB string) (*DiffComparisonResult, error) {
	if refA == "" || refB == "" {
		return nil, errors.New("both references are required for comparison")
	}
	rangeArg := fmt.Sprintf("%s...%s", refA, refB)
	return buildDiffComparison(repoPath, rangeArg, refA, refB)
}

// GetFileDiffBetween returns the unified patch for a single file between two
// refs. mergeBase selects three-dot (A...B) semantics.
func (GitService) GetFileDiffBetween(repoPath string, refA string, refB string, filePath string, mergeBase bool) (string, error) {
	if refA == "" || refB == "" {
		return "", errors.New("both references are required for comparison")
	}
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return "", err
	}
	separator := ".."
	if mergeBase {
		separator = "..."
	}
	rangeArg := fmt.Sprintf("%s%s%s", refA, separator, refB)
	raw, err := ExecGit(repoPath, "diff", "--patch", rangeArg, "--", cleanPath)
	if err != nil {
		return "", err
	}
	diffText, _ := truncateDiffPayload(raw)
	return diffText, nil
}

// ExportPatchRange exports commits in startHash..endHash as a unified format-patch file to outputPath.
func (GitService) ExportPatchRange(repoPath string, startHash string, endHash string, outputPath string) error {
	repoPath = strings.TrimSpace(repoPath)
	startHash = strings.TrimSpace(startHash)
	endHash = strings.TrimSpace(endHash)
	outputPath = strings.TrimSpace(outputPath)

	if repoPath == "" {
		return errors.New("repoPath cannot be empty")
	}
	if startHash == "" || endHash == "" {
		return errors.New("startHash and endHash are required")
	}
	if strings.HasPrefix(startHash, "-") || strings.HasPrefix(endHash, "-") {
		return errors.New("invalid commit reference")
	}
	if outputPath == "" {
		return errors.New("outputPath cannot be empty")
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}
	if fi, err := os.Stat(absPath); err == nil && fi.IsDir() {
		return errors.New("outputPath must be a file, not a directory")
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	rangeArg := fmt.Sprintf("%s..%s", startHash, endHash)
	patchContent, err := ExecGit(repoPath, "format-patch", rangeArg, "--stdout")
	if err != nil {
		return err
	}
	return os.WriteFile(absPath, []byte(patchContent), 0o644)
}

// GeneratePatchRange returns the unified diff between startHash and endHash.
func (GitService) GeneratePatchRange(repoPath string, startHash string, endHash string) (string, error) {
	repoPath = strings.TrimSpace(repoPath)
	startHash = strings.TrimSpace(startHash)
	endHash = strings.TrimSpace(endHash)

	if repoPath == "" {
		return "", errors.New("repoPath cannot be empty")
	}
	if startHash == "" || endHash == "" {
		return "", errors.New("startHash and endHash are required")
	}
	if strings.HasPrefix(startHash, "-") || strings.HasPrefix(endHash, "-") {
		return "", errors.New("invalid commit reference")
	}

	rangeArg := fmt.Sprintf("%s..%s", startHash, endHash)
	raw, err := ExecGit(repoPath, "diff", rangeArg)
	if err != nil {
		return "", err
	}
	diffText, _ := truncateDiffPayload(raw)
	return diffText, nil
}
