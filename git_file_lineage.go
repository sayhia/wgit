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

type FileRenameEvent struct {
	OldPath    string `json:"oldPath"`
	NewPath    string `json:"newPath"`
	CommitHash string `json:"commitHash"`
	CommitDate string `json:"commitDate"`
	Author     string `json:"author"`
	Subject    string `json:"subject"`
	Similarity int    `json:"similarity"`
}

type FileLineageReport struct {
	CurrentPath     string            `json:"currentPath"`
	HistoricalPaths []string          `json:"historicalPaths"`
	Renames         []FileRenameEvent `json:"renames"`
	TotalRenames    int               `json:"totalRenames"`
	OriginPath      string            `json:"originPath"`
	Summary         string            `json:"summary"`
}

type CrossFileBlameResult struct {
	OriginalPath string `json:"originalPath"`
	OriginalLine int    `json:"originalLine"`
	CommitHash   string `json:"commitHash"`
	Author       string `json:"author"`
	Date         string `json:"date"`
	Summary      string `json:"summary"`
}

func execGitLineage(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TraceFileRenamesAcrossHistory tracks the rename lineage of a file across git history.
func (GitService) TraceFileRenamesAcrossHistory(repoPath string, filePath string) (*FileLineageReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("filePath cannot be empty")
	}

	out, err := execGitLineage(repoPath, "log", "--follow", "--name-status", "-M", "--format=COMMIT:%H|%ad|%an|%s", "--date=short", "--", filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to trace file renames: %w", err)
	}

	var renames []FileRenameEvent
	pathSet := make(map[string]bool)
	pathSet[filePath] = true
	var currentCommitInfo string

	lines := strings.Split(out, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "COMMIT:") {
			currentCommitInfo = strings.TrimPrefix(l, "COMMIT:")
			continue
		}

		// Check for rename status R100 oldPath newPath
		if strings.HasPrefix(l, "R") {
			parts := strings.Fields(l)
			if len(parts) >= 3 {
				status := parts[0]
				oldPath := parts[1]
				newPath := parts[2]
				sim := 100
				if len(status) > 1 {
					if s, err := strconv.Atoi(status[1:]); err == nil {
						sim = s
					}
				}

				commitHash, commitDate, author, subject := "", "", "", ""
				infoParts := strings.Split(currentCommitInfo, "|")
				if len(infoParts) >= 4 {
					commitHash = infoParts[0]
					commitDate = infoParts[1]
					author = infoParts[2]
					subject = infoParts[3]
				}

				renames = append(renames, FileRenameEvent{
					OldPath:    oldPath,
					NewPath:    newPath,
					CommitHash: commitHash,
					CommitDate: commitDate,
					Author:     author,
					Subject:    subject,
					Similarity: sim,
				})

				pathSet[oldPath] = true
				pathSet[newPath] = true
			}
		}
	}

	var historicalPaths []string
	for p := range pathSet {
		historicalPaths = append(historicalPaths, p)
	}

	originPath := filePath
	if len(renames) > 0 {
		originPath = renames[len(renames)-1].OldPath
	}

	summary := fmt.Sprintf("File '%s' has %d recorded renames/moves across history.", filePath, len(renames))

	return &FileLineageReport{
		CurrentPath:     filePath,
		HistoricalPaths: historicalPaths,
		Renames:         renames,
		TotalRenames:    len(renames),
		OriginPath:      originPath,
		Summary:         summary,
	}, nil
}

// GetCrossFileBlame traces the origin commit and author across renames and copies.
func (GitService) GetCrossFileBlame(repoPath string, filePath string, lineNum int) (*CrossFileBlameResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("filePath cannot be empty")
	}
	if lineNum <= 0 {
		lineNum = 1
	}

	loc := fmt.Sprintf("%d,%d", lineNum, lineNum)
	out, err := execGitLineage(repoPath, "blame", "-C", "-C", "-C", "-L", loc, "--porcelain", "--", filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get cross file blame: %w", err)
	}

	var commitHash, author, date, origPath string
	origLine := lineNum

	lines := strings.Split(out, "\n")
	if len(lines) > 0 {
		fields := strings.Fields(lines[0])
		if len(fields) >= 1 {
			commitHash = fields[0]
		}
	}

	for _, l := range lines {
		if strings.HasPrefix(l, "author ") {
			author = strings.TrimPrefix(l, "author ")
		} else if strings.HasPrefix(l, "author-time ") {
			tsStr := strings.TrimPrefix(l, "author-time ")
			if sec, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
				date = time.Unix(sec, 0).Format("2006-01-02")
			}
		} else if strings.HasPrefix(l, "filename ") {
			origPath = strings.TrimPrefix(l, "filename ")
		}
	}

	if origPath == "" {
		origPath = filePath
	}

	summary := fmt.Sprintf("Line %d originates from %s at %s (%s)", lineNum, origPath, commitHash, author)

	return &CrossFileBlameResult{
		OriginalPath: origPath,
		OriginalLine: origLine,
		CommitHash:   commitHash,
		Author:       author,
		Date:         date,
		Summary:      summary,
	}, nil
}
