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

type BranchDriftReport struct {
	TargetBranch       string   `json:"targetBranch"`
	BaseBranch         string   `json:"baseBranch"`
	AheadCount         int      `json:"aheadCount"`
	BehindCount        int      `json:"behindCount"`
	CommonBaseSha      string   `json:"commonBaseSha"`
	OverlappingFiles   []string `json:"overlappingFiles"`
	PotentialConflicts []string `json:"potentialConflicts"`
	HealthScore        int      `json:"healthScore"` // 0 - 100
	Summary            string   `json:"summary"`
}

type BranchPairOverlap struct {
	BranchA          string   `json:"branchA"`
	BranchB          string   `json:"branchB"`
	SharedFilesCount int      `json:"sharedFilesCount"`
	SharedFiles      []string `json:"sharedFiles"`
	RiskLevel        string   `json:"riskLevel"` // "low", "medium", "high"
}

type BranchOverlapMatrixReport struct {
	TotalBranches int                 `json:"totalBranches"`
	PairOverlaps  []BranchPairOverlap `json:"pairOverlaps"`
	Summary       string              `json:"summary"`
}

func execGitDrift(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// AnalyzeBranchDrift computes branch divergence, behind/ahead metrics, and overlapping files.
func (GitService) AnalyzeBranchDrift(repoPath string, targetBranch string, baseBranch string) (*BranchDriftReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	targetBranch = strings.TrimSpace(targetBranch)
	baseBranch = strings.TrimSpace(baseBranch)
	if targetBranch == "" {
		return nil, errors.New("targetBranch cannot be empty")
	}
	if baseBranch == "" {
		baseBranch = "main"
	}

	baseShaOut, err := execGitDrift(repoPath, "merge-base", baseBranch, targetBranch)
	commonBase := strings.TrimSpace(baseShaOut)
	if err != nil || commonBase == "" {
		// Fallback to HEAD
		commonBase = "HEAD"
	}

	aheadOut, _ := execGitDrift(repoPath, "rev-list", "--count", commonBase+".."+targetBranch)
	ahead, _ := strconv.Atoi(strings.TrimSpace(aheadOut))

	behindOut, _ := execGitDrift(repoPath, "rev-list", "--count", commonBase+".."+baseBranch)
	behind, _ := strconv.Atoi(strings.TrimSpace(behindOut))

	targetDiffOut, _ := execGitDrift(repoPath, "diff", "--name-only", commonBase+".."+targetBranch)
	targetFiles := make(map[string]bool)
	for _, f := range strings.Split(strings.TrimSpace(targetDiffOut), "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			targetFiles[f] = true
		}
	}

	baseDiffOut, _ := execGitDrift(repoPath, "diff", "--name-only", commonBase+".."+baseBranch)
	var overlapping []string
	for _, f := range strings.Split(strings.TrimSpace(baseDiffOut), "\n") {
		f = strings.TrimSpace(f)
		if f != "" && targetFiles[f] {
			overlapping = append(overlapping, f)
		}
	}

	// Conflict test using merge-tree
	var conflicts []string
	mergeTreeOut, _ := execGitDrift(repoPath, "merge-tree", baseBranch, targetBranch)
	for _, l := range strings.Split(mergeTreeOut, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "CONFLICT") {
			conflicts = append(conflicts, l)
		}
	}

	health := 100 - (behind * 2) - (len(overlapping) * 8) - (len(conflicts) * 20)
	if health < 0 {
		health = 0
	} else if health > 100 {
		health = 100
	}

	summary := fmt.Sprintf("%s vs %s: +%d ahead, -%d behind, %d overlapping files, %d potential conflicts (Health: %d/100)",
		targetBranch, baseBranch, ahead, behind, len(overlapping), len(conflicts), health)

	return &BranchDriftReport{
		TargetBranch:       targetBranch,
		BaseBranch:         baseBranch,
		AheadCount:         ahead,
		BehindCount:        behind,
		CommonBaseSha:      commonBase,
		OverlappingFiles:   overlapping,
		PotentialConflicts: conflicts,
		HealthScore:        health,
		Summary:            summary,
	}, nil
}

// CalculateBranchOverlapMatrix inspects shared file modifications across multiple active branches.
func (s GitService) CalculateBranchOverlapMatrix(repoPath string, branches []string) (*BranchOverlapMatrixReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	if len(branches) == 0 {
		// Discover active local branches
		out, err := execGitDrift(repoPath, "branch", "--format=%(refname:short)")
		if err == nil {
			for _, b := range strings.Split(strings.TrimSpace(out), "\n") {
				b = strings.TrimSpace(b)
				if b != "" {
					branches = append(branches, b)
				}
			}
		}
	}

	branchFiles := make(map[string]map[string]bool)
	for _, b := range branches {
		filesMap := make(map[string]bool)
		diffOut, _ := execGitDrift(repoPath, "diff", "--name-only", "HEAD.."+b)
		if diffOut == "" {
			diffOut, _ = execGitDrift(repoPath, "diff", "--name-only", b+"~1.."+b)
		}
		for _, f := range strings.Split(strings.TrimSpace(diffOut), "\n") {
			f = strings.TrimSpace(f)
			if f != "" {
				filesMap[f] = true
			}
		}
		branchFiles[b] = filesMap
	}

	var pairOverlaps []BranchPairOverlap
	for i := 0; i < len(branches); i++ {
		for j := i + 1; j < len(branches); j++ {
			bA := branches[i]
			bB := branches[j]
			var shared []string
			for f := range branchFiles[bA] {
				if branchFiles[bB][f] {
					shared = append(shared, f)
				}
			}
			risk := "low"
			if len(shared) >= 3 {
				risk = "high"
			} else if len(shared) > 0 {
				risk = "medium"
			}

			if len(shared) > 0 || len(branches) <= 4 {
				pairOverlaps = append(pairOverlaps, BranchPairOverlap{
					BranchA:          bA,
					BranchB:          bB,
					SharedFilesCount: len(shared),
					SharedFiles:      shared,
					RiskLevel:        risk,
				})
			}
		}
	}

	summary := fmt.Sprintf("Calculated overlap matrix across %d branches: %d branch pairs analyzed", len(branches), len(pairOverlaps))

	return &BranchOverlapMatrixReport{
		TotalBranches: len(branches),
		PairOverlaps:  pairOverlaps,
		Summary:       summary,
	}, nil
}
