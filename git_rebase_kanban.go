package main

import (
	"errors"
	"fmt"
	"strings"
)

type RebaseKanbanItem struct {
	Hash             string   `json:"hash"`
	ShortHash        string   `json:"shortHash"`
	Author           string   `json:"author"`
	AuthorDate       string   `json:"authorDate"`
	Subject          string   `json:"subject"`
	Action           string   `json:"action"` // "pick", "squash", "fixup", "reword", "drop", "split"
	OriginalOrder    int      `json:"originalOrder"`
	CurrentOrder     int      `json:"currentOrder"`
	ConflictRisk     string   `json:"conflictRisk"` // "low", "medium", "high"
	OverlappingFiles []string `json:"overlappingFiles"`
}

type RebasePlanPreview struct {
	BaseRef                string             `json:"baseRef"`
	HeadRef                string             `json:"headRef"`
	TotalCommits           int                `json:"totalCommits"`
	Items                  []RebaseKanbanItem `json:"items"`
	EstimatedConflictCount int                `json:"estimatedConflictCount"`
	CanAutoLinearize       bool               `json:"canAutoLinearize"`
	Summary                string             `json:"summary"`
}

type CommitPartition struct {
	Subject string   `json:"subject"`
	Files   []string `json:"files"`
}

type CommitSplitResult struct {
	Success          bool     `json:"success"`
	OriginalCommit   string   `json:"originalCommit"`
	GeneratedCommits []string `json:"generatedCommits"`
	Message          string   `json:"message"`
}

func (GitService) PreviewRebaseTopology(repoPath string, baseRef string, headRef string) (*RebasePlanPreview, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		resolved, err := resolveInsightBase(repoPath, "")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve base ref: %w", err)
		}
		baseRef = resolved
	}

	headRef = strings.TrimSpace(headRef)
	if headRef == "" {
		headRef = "HEAD"
	}

	rangeArg := fmt.Sprintf("%s..%s", baseRef, headRef)
	out, err := ExecGit(repoPath, "log", "--reverse", "--format=%H|%h|%an|%aI|%s", rangeArg)
	if err != nil {
		return nil, fmt.Errorf("failed to read commits between %s and %s: %w", baseRef, headRef, err)
	}

	lines := strings.Split(out, "\n")
	var items []RebaseKanbanItem
	seenFiles := make(map[string]int)
	order := 1
	estimatedConflicts := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		hash := parts[0]
		shortHash := parts[1]
		author := parts[2]
		authorDate := parts[3]
		subject := parts[4]

		filesOut, _ := ExecGit(repoPath, "show", "--name-only", "--format=", hash)
		var files []string
		var overlapping []string
		for _, f := range strings.Split(filesOut, "\n") {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			files = append(files, f)
			if seenFiles[f] > 0 {
				overlapping = append(overlapping, f)
			}
			seenFiles[f]++
		}

		risk := "low"
		if len(overlapping) > 3 {
			risk = "high"
			estimatedConflicts++
		} else if len(overlapping) > 0 {
			risk = "medium"
		}

		items = append(items, RebaseKanbanItem{
			Hash:             hash,
			ShortHash:        shortHash,
			Author:           author,
			AuthorDate:       authorDate,
			Subject:          subject,
			Action:           "pick",
			OriginalOrder:    order,
			CurrentOrder:     order,
			ConflictRisk:     risk,
			OverlappingFiles: overlapping,
		})
		order++
	}

	summary := fmt.Sprintf("%d commits found between %s and %s", len(items), baseRef, headRef)
	if estimatedConflicts > 0 {
		summary = fmt.Sprintf("%d commits with %d potential file overlap conflict risks", len(items), estimatedConflicts)
	}

	return &RebasePlanPreview{
		BaseRef:                baseRef,
		HeadRef:                headRef,
		TotalCommits:           len(items),
		Items:                  items,
		EstimatedConflictCount: estimatedConflicts,
		CanAutoLinearize:       len(items) > 0,
		Summary:                summary,
	}, nil
}

func (GitService) SplitCommitSurgically(repoPath string, commitHash string, partitions []CommitPartition) (*CommitSplitResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	commitHash = strings.TrimSpace(commitHash)
	if commitHash == "" {
		return nil, errors.New("commitHash cannot be empty")
	}
	if len(partitions) < 2 {
		return nil, errors.New("at least 2 partitions are required to split a commit")
	}

	// 1. Verify target commit exists
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", commitHash); err != nil {
		return nil, fmt.Errorf("invalid commit hash: %s", commitHash)
	}

	// 2. Perform soft reset to unstage the target commit
	if _, err := ExecGit(repoPath, "reset", "--soft", commitHash+"~1"); err != nil {
		return nil, fmt.Errorf("failed to soft reset commit %s: %w", commitHash, err)
	}

	// 3. Unstage everything to start surgical partitioning
	_, _ = ExecGit(repoPath, "reset", "HEAD", ".")

	var generatedCommits []string

	for idx, p := range partitions {
		subj := strings.TrimSpace(p.Subject)
		if subj == "" {
			subj = fmt.Sprintf("chore: split commit partition %d", idx+1)
		}

		if len(p.Files) == 0 {
			continue
		}

		// Stage partition files
		for _, f := range p.Files {
			f = strings.TrimSpace(f)
			if f != "" {
				_, _ = ExecGit(repoPath, "add", f)
			}
		}

		// Commit partition
		if _, err := ExecGit(repoPath, "commit", "-m", subj); err != nil {
			return &CommitSplitResult{
				Success:          false,
				OriginalCommit:   commitHash,
				GeneratedCommits: generatedCommits,
				Message:          fmt.Sprintf("Failed to commit partition %d: %v", idx+1, err),
			}, nil
		}

		// Read new commit hash
		newHash, _ := ExecGit(repoPath, "rev-parse", "HEAD")
		generatedCommits = append(generatedCommits, strings.TrimSpace(newHash))
	}

	// Any remaining unstaged files are committed as final partition if not empty
	statusOut, _ := ExecGit(repoPath, "status", "--porcelain")
	if strings.TrimSpace(statusOut) != "" {
		_, _ = ExecGit(repoPath, "add", ".")
		if _, err := ExecGit(repoPath, "commit", "-m", "chore: remaining changes from split"); err == nil {
			remHash, _ := ExecGit(repoPath, "rev-parse", "HEAD")
			generatedCommits = append(generatedCommits, strings.TrimSpace(remHash))
		}
	}

	return &CommitSplitResult{
		Success:          len(generatedCommits) >= 2,
		OriginalCommit:   commitHash,
		GeneratedCommits: generatedCommits,
		Message:          fmt.Sprintf("Successfully split commit into %d atomic commits", len(generatedCommits)),
	}, nil
}
