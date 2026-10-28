package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type MergeSandboxReport struct {
	TargetRef       string   `json:"targetRef"`
	CanMergeCleanly bool     `json:"canMergeCleanly"`
	IsFastForward   bool     `json:"isFastForward"`
	AheadCount      int      `json:"aheadCount"`
	BehindCount     int      `json:"behindCount"`
	ConflictedFiles []string `json:"conflictedFiles"`
	CleanFiles      []string `json:"cleanFiles"`
	Summary         string   `json:"summary"`
}

type MergeImpactReport struct {
	TargetRef         string   `json:"targetRef"`
	ImpactLevel       string   `json:"impactLevel"` // "low", "medium", "high"
	TotalFilesChanged int      `json:"totalFilesChanged"`
	CoreFilesCount    int      `json:"coreFilesCount"`
	TestFilesCount    int      `json:"testFilesCount"`
	DocFilesCount     int      `json:"docFilesCount"`
	HighRiskFiles     []string `json:"highRiskFiles"`
	Summary           string   `json:"summary"`
}

func execGitSandbox(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// DryRunPullOrMerge simulates merging or rebasing onto targetRef without touching index or working tree.
func (GitService) DryRunPullOrMerge(repoPath string, targetRef string, isRebase bool) (*MergeSandboxReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	targetRef = strings.TrimSpace(targetRef)
	if targetRef == "" {
		return nil, errors.New("targetRef cannot be empty")
	}

	headRef, _ := execGitSandbox(repoPath, "rev-parse", "HEAD")
	headRef = strings.TrimSpace(headRef)

	targetCommit, err := execGitSandbox(repoPath, "rev-parse", targetRef)
	if err != nil {
		return nil, fmt.Errorf("targetRef '%s' not found: %w", targetRef, err)
	}
	targetCommit = strings.TrimSpace(targetCommit)

	// Calculate ahead / behind count
	aheadOut, _ := execGitSandbox(repoPath, "rev-list", "--count", headRef+".."+targetCommit)
	behindOut, _ := execGitSandbox(repoPath, "rev-list", "--count", targetCommit+".."+headRef)
	ahead, _ := strconv.Atoi(strings.TrimSpace(aheadOut))
	behind, _ := strconv.Atoi(strings.TrimSpace(behindOut))

	// Fast forward check: if targetCommit is direct descendant of headRef
	mbOut, _ := execGitSandbox(repoPath, "merge-base", headRef, targetCommit)
	mergeBase := strings.TrimSpace(mbOut)
	isFastForward := (mergeBase == headRef)

	canMergeCleanly := true
	var conflictedFiles []string
	var cleanFiles []string

	if isFastForward {
		canMergeCleanly = true
		diffOut, _ := execGitSandbox(repoPath, "diff", "--name-only", headRef, targetCommit)
		for _, f := range strings.Split(diffOut, "\n") {
			f = strings.TrimSpace(f)
			if f != "" {
				cleanFiles = append(cleanFiles, f)
			}
		}
	} else {
		// Use merge-tree to test
		mtOut, mtErr := execGitSandbox(repoPath, "merge-tree", "--write-tree", headRef, targetCommit)
		if mtErr != nil || strings.Contains(mtOut, "CONFLICT") {
			// Legacy syntax check
			if mergeBase != "" {
				mtLegacy, _ := execGitSandbox(repoPath, "merge-tree", mergeBase, headRef, targetCommit)
				if strings.Contains(mtLegacy, "<<<<<<<") || strings.Contains(mtLegacy, "CONFLICT") {
					canMergeCleanly = false
				}
			} else {
				canMergeCleanly = false
			}
		}

		// Find changed files
		diffTarget, _ := execGitSandbox(repoPath, "diff", "--name-only", mergeBase, targetCommit)
		diffHead, _ := execGitSandbox(repoPath, "diff", "--name-only", mergeBase, headRef)
		headFilesMap := make(map[string]bool)
		for _, hf := range strings.Split(diffHead, "\n") {
			hf = strings.TrimSpace(hf)
			if hf != "" {
				headFilesMap[hf] = true
			}
		}

		for _, tf := range strings.Split(diffTarget, "\n") {
			tf = strings.TrimSpace(tf)
			if tf == "" {
				continue
			}
			if headFilesMap[tf] && !canMergeCleanly {
				conflictedFiles = append(conflictedFiles, tf)
			} else {
				cleanFiles = append(cleanFiles, tf)
			}
		}
		if !canMergeCleanly && len(conflictedFiles) == 0 && len(cleanFiles) > 0 {
			conflictedFiles = append(conflictedFiles, cleanFiles[0])
			cleanFiles = cleanFiles[1:]
		}
	}

	summary := ""
	if isFastForward {
		summary = fmt.Sprintf("Fast-forward merge possible with %d commits from %s.", ahead, targetRef)
	} else if canMergeCleanly {
		summary = fmt.Sprintf("Clean 3-way merge possible with %s (%d files auto-merged).", targetRef, len(cleanFiles))
	} else {
		summary = fmt.Sprintf("Merge with %s will produce %d conflicts across %d changed files.", targetRef, len(conflictedFiles), len(conflictedFiles)+len(cleanFiles))
	}

	return &MergeSandboxReport{
		TargetRef:       targetRef,
		CanMergeCleanly: canMergeCleanly,
		IsFastForward:   isFastForward,
		AheadCount:      ahead,
		BehindCount:     behind,
		ConflictedFiles: conflictedFiles,
		CleanFiles:      cleanFiles,
		Summary:         summary,
	}, nil
}

// CalculateMergeImpactMatrix analyzes which layers of the codebase are impacted by targetRef.
func (GitService) CalculateMergeImpactMatrix(repoPath string, targetRef string) (*MergeImpactReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	targetRef = strings.TrimSpace(targetRef)
	if targetRef == "" {
		return nil, errors.New("targetRef cannot be empty")
	}

	headRef, _ := execGitSandbox(repoPath, "rev-parse", "HEAD")
	headRef = strings.TrimSpace(headRef)

	targetCommit, err := execGitSandbox(repoPath, "rev-parse", targetRef)
	if err != nil {
		return nil, fmt.Errorf("targetRef '%s' not found: %w", targetRef, err)
	}
	targetCommit = strings.TrimSpace(targetCommit)

	mbOut, _ := execGitSandbox(repoPath, "merge-base", headRef, targetCommit)
	mergeBase := strings.TrimSpace(mbOut)
	if mergeBase == "" {
		mergeBase = headRef
	}

	diffOut, _ := execGitSandbox(repoPath, "diff", "--name-only", mergeBase, targetCommit)
	var files []string
	for _, l := range strings.Split(diffOut, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}

	coreCount := 0
	testCount := 0
	docCount := 0
	var highRiskFiles []string

	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		lower := strings.ToLower(f)

		if strings.Contains(lower, "test") || strings.HasSuffix(lower, "_test.go") || strings.HasSuffix(lower, ".test.ts") {
			testCount++
		} else if ext == ".md" || ext == ".txt" || strings.HasPrefix(lower, "docs/") {
			docCount++
		} else if ext == ".go" || ext == ".ts" || ext == ".vue" || ext == ".rs" || ext == ".cpp" || ext == ".c" {
			coreCount++
			if strings.Contains(lower, "config") || strings.Contains(lower, "auth") || strings.Contains(lower, "api") {
				highRiskFiles = append(highRiskFiles, f)
			}
		}
	}

	impactLevel := "low"
	if coreCount > 10 || len(highRiskFiles) > 2 {
		impactLevel = "high"
	} else if coreCount > 2 || len(highRiskFiles) > 0 {
		impactLevel = "medium"
	}

	summary := fmt.Sprintf("Impact Level: %s (%d core, %d tests, %d docs, %d total files)",
		strings.ToUpper(impactLevel), coreCount, testCount, docCount, len(files))

	return &MergeImpactReport{
		TargetRef:         targetRef,
		ImpactLevel:       impactLevel,
		TotalFilesChanged: len(files),
		CoreFilesCount:    coreCount,
		TestFilesCount:    testCount,
		DocFilesCount:     docCount,
		HighRiskFiles:     highRiskFiles,
		Summary:           summary,
	}, nil
}
