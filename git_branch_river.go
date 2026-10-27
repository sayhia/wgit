package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BranchRiverNode struct {
	Name              string `json:"name"`
	Upstream          string `json:"upstream"`
	IsHead            bool   `json:"isHead"`
	AheadCount        int    `json:"aheadCount"`
	BehindCount       int    `json:"behindCount"`
	LastCommitDate    string `json:"lastCommitDate"`
	LastCommitAuthor  string `json:"lastCommitAuthor"`
	LastCommitSubject string `json:"lastCommitSubject"`
	DivergencePoint   string `json:"divergencePoint"`
	ConflictRiskScore int    `json:"conflictRiskScore"` // 0-100
	ConflictRiskLevel string `json:"conflictRiskLevel"` // "low", "medium", "high", "critical"
	IsMerged          bool   `json:"isMerged"`
	Status            string `json:"status"`            // "active", "stale", "merged", "gone"
}

type BranchRiverData struct {
	BaseBranch     string            `json:"baseBranch"`
	Nodes          []BranchRiverNode `json:"nodes"`
	TotalBranches  int               `json:"totalBranches"`
	ActiveBranches int               `json:"activeBranches"`
	StaleBranches  int               `json:"staleBranches"`
	MergedBranches int               `json:"mergedBranches"`
	ScanDurationMs int64             `json:"scanDurationMs"`
}

type StaleBranchInfo struct {
	Name              string `json:"name"`
	LastCommitDate    string `json:"lastCommitDate"`
	LastCommitSubject string `json:"lastCommitSubject"`
	Author            string `json:"author"`
	DaysInactive      int    `json:"daysInactive"`
	IsMerged          bool   `json:"isMerged"`
	RemoteGone        bool   `json:"remoteGone"`
	RecommendedAction string `json:"recommendedAction"` // "delete", "archive_bundle", "keep"
}

type StaleBranchesReport struct {
	Branches      []StaleBranchInfo `json:"branches"`
	TotalDetected int               `json:"totalDetected"`
	MergedCount   int               `json:"mergedCount"`
	DaysThreshold int               `json:"daysThreshold"`
	CanBulkClean  bool              `json:"canBulkClean"`
}

type BranchArchiveResult struct {
	Success          bool     `json:"success"`
	BundlePath       string   `json:"bundlePath"`
	ArchivedBranches []string `json:"archivedBranches"`
	DeletedBranches  []string `json:"deletedBranches"`
	FailedBranches   []string `json:"failedBranches"`
	Message          string   `json:"message"`
}

func (GitService) GetBranchRiverData(repoPath string, baseBranch string) (*BranchRiverData, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	start := time.Now()
	resolvedBase, err := resolveInsightBase(repoPath, baseBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base branch: %w", err)
	}

	mergedSet := mergedBranchSet(repoPath, resolvedBase)

	format := "%(refname:short)|%(upstream:short)|%(HEAD)|%(authordate:iso-strict)|%(authorname)|%(subject)|%(objectname:short)"
	rawBranches, err := ExecGit(repoPath, "branch", "--format="+format)
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	lines := strings.Split(rawBranches, "\n")
	var nodes []BranchRiverNode
	activeCount := 0
	staleCount := 0
	mergedCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		for len(parts) < 7 {
			parts = append(parts, "")
		}

		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}

		upstream := strings.TrimSpace(parts[1])
		isHead := strings.TrimSpace(parts[2]) == "*"
		lastDate := strings.TrimSpace(parts[3])
		lastAuthor := strings.TrimSpace(parts[4])
		lastSubject := strings.TrimSpace(parts[5])

		isMerged := mergedSet[name] || name == resolvedBase

		ahead := 0
		behind := 0
		if name != resolvedBase {
			if a, b, err := aheadBehind(repoPath, resolvedBase, name); err == nil {
				ahead = a
				behind = b
			}
		}

		divPoint := ""
		if name != resolvedBase {
			if dp, err := ExecGit(repoPath, "merge-base", resolvedBase, name); err == nil {
				divPoint = strings.TrimSpace(dp)
				if len(divPoint) > 8 {
					divPoint = divPoint[:8]
				}
			}
		}

		days := 0
		if lastDate != "" {
			if pt, err := time.Parse(time.RFC3339, lastDate); err == nil {
				days = int(time.Since(pt).Hours() / 24)
				if days < 0 {
					days = 0
				}
			}
		}

		conflictScore := 0
		conflictLevel := "low"
		if !isMerged && name != resolvedBase {
			if behind > 0 {
				conflictScore = behind*5 + days*2
				if conflictScore > 100 {
					conflictScore = 100
				}
				if conflictScore >= 80 {
					conflictLevel = "critical"
				} else if conflictScore >= 50 {
					conflictLevel = "high"
				} else if conflictScore >= 25 {
					conflictLevel = "medium"
				}
			}
		}

		status := "active"
		if name == resolvedBase {
			status = "active"
			activeCount++
		} else if isMerged {
			status = "merged"
			mergedCount++
		} else if days >= 30 {
			status = "stale"
			staleCount++
		} else {
			status = "active"
			activeCount++
		}

		nodes = append(nodes, BranchRiverNode{
			Name:              name,
			Upstream:          upstream,
			IsHead:            isHead,
			AheadCount:        ahead,
			BehindCount:       behind,
			LastCommitDate:    lastDate,
			LastCommitAuthor:  lastAuthor,
			LastCommitSubject: lastSubject,
			DivergencePoint:   divPoint,
			ConflictRiskScore: conflictScore,
			ConflictRiskLevel: conflictLevel,
			IsMerged:          isMerged,
			Status:            status,
		})
	}

	return &BranchRiverData{
		BaseBranch:     resolvedBase,
		Nodes:          nodes,
		TotalBranches:  len(nodes),
		ActiveBranches: activeCount,
		StaleBranches:  staleCount,
		MergedBranches: mergedCount,
		ScanDurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func (GitService) DetectStaleBranches(repoPath string, daysThreshold int) (*StaleBranchesReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if daysThreshold <= 0 {
		daysThreshold = 30
	}

	resolvedBase, err := resolveInsightBase(repoPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base branch: %w", err)
	}

	mergedSet := mergedBranchSet(repoPath, resolvedBase)
	headBranch, _ := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	headBranch = strings.TrimSpace(headBranch)

	format := "%(refname:short)|%(upstream:short)|%(authordate:iso-strict)|%(authorname)|%(subject)"
	rawBranches, err := ExecGit(repoPath, "branch", "--format="+format)
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	lines := strings.Split(rawBranches, "\n")
	var staleList []StaleBranchInfo
	mergedCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		for len(parts) < 5 {
			parts = append(parts, "")
		}

		name := strings.TrimSpace(parts[0])
		if name == "" || name == resolvedBase || name == headBranch {
			continue
		}

		upstream := strings.TrimSpace(parts[1])
		lastDate := strings.TrimSpace(parts[2])
		author := strings.TrimSpace(parts[3])
		subject := strings.TrimSpace(parts[4])

		isMerged := mergedSet[name]
		remoteGone := false
		if upstream != "" {
			if _, err := ExecGit(repoPath, "rev-parse", "--verify", "--quiet", upstream); err != nil {
				remoteGone = true
			}
		}

		days := 0
		if lastDate != "" {
			if pt, err := time.Parse(time.RFC3339, lastDate); err == nil {
				days = int(time.Since(pt).Hours() / 24)
				if days < 0 {
					days = 0
				}
			}
		}

		if isMerged || remoteGone || days >= daysThreshold {
			if isMerged {
				mergedCount++
			}
			recAction := "archive_bundle"
			if isMerged || remoteGone {
				recAction = "delete"
			}

			staleList = append(staleList, StaleBranchInfo{
				Name:              name,
				LastCommitDate:    lastDate,
				LastCommitSubject: subject,
				Author:            author,
				DaysInactive:      days,
				IsMerged:          isMerged,
				RemoteGone:        remoteGone,
				RecommendedAction: recAction,
			})
		}
	}

	return &StaleBranchesReport{
		Branches:      staleList,
		TotalDetected: len(staleList),
		MergedCount:   mergedCount,
		DaysThreshold: daysThreshold,
		CanBulkClean:  mergedCount > 0,
	}, nil
}

func (GitService) ArchiveAndDeleteBranches(repoPath string, branches []string, bundlePath string) (*BranchArchiveResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if len(branches) == 0 {
		return nil, errors.New("branches list cannot be empty")
	}

	headBranch, _ := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	headBranch = strings.TrimSpace(headBranch)

	var validBranches []string
	var failedBranches []string

	for _, b := range branches {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		if b == headBranch {
			failedBranches = append(failedBranches, b)
			continue
		}
		validBranches = append(validBranches, b)
	}

	if len(validBranches) == 0 {
		return &BranchArchiveResult{
			Success:        false,
			FailedBranches: failedBranches,
			Message:        "Cannot archive or delete currently checked-out HEAD branch",
		}, nil
	}

	if bundlePath == "" {
		archiveDir := filepath.Join(repoPath, ".git", "branch_archives")
		_ = os.MkdirAll(archiveDir, 0755)
		bundlePath = filepath.Join(archiveDir, fmt.Sprintf("archive-%d.bundle", time.Now().Unix()))
	} else {
		_ = os.MkdirAll(filepath.Dir(bundlePath), 0755)
	}

	// Create git bundle
	bundleArgs := append([]string{"bundle", "create", bundlePath}, validBranches...)
	out, err := ExecGit(repoPath, bundleArgs...)
	if err != nil {
		return &BranchArchiveResult{
			Success:        false,
			BundlePath:     bundlePath,
			FailedBranches: validBranches,
			Message:        fmt.Sprintf("Failed to create git bundle: %v (output: %s)", err, out),
		}, nil
	}

	var deletedBranches []string
	for _, b := range validBranches {
		if _, err := ExecGit(repoPath, "branch", "-D", b); err == nil {
			deletedBranches = append(deletedBranches, b)
		} else {
			failedBranches = append(failedBranches, b)
		}
	}

	return &BranchArchiveResult{
		Success:          len(deletedBranches) > 0,
		BundlePath:       bundlePath,
		ArchivedBranches: validBranches,
		DeletedBranches:  deletedBranches,
		FailedBranches:   failedBranches,
		Message:          fmt.Sprintf("Archived %d branches to %s and deleted %d branches", len(validBranches), filepath.Base(bundlePath), len(deletedBranches)),
	}, nil
}
