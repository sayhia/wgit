package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BlameHeatmapLine represents a single line with age and author data for visual heatmaps.
type BlameHeatmapLine struct {
	LineNumber int     `json:"lineNumber"`
	CommitHash string  `json:"commitHash"`
	Author     string  `json:"author"`
	Timestamp  int64   `json:"timestamp"`
	AgeDays    int     `json:"ageDays"`
	HeatRatio  float64 `json:"heatRatio"` // 1.0 = newest, 0.0 = oldest
	AgeTier    string  `json:"ageTier"`   // "recent" | "month" | "quarter" | "year" | "ancient"
	Content    string  `json:"content"`
}

// BlameHeatmapResult holds the aggregated blame time-gradient metrics for a file.
type BlameHeatmapResult struct {
	FilePath   string             `json:"filePath"`
	TotalLines int                `json:"totalLines"`
	NewestTime int64              `json:"newestTime"`
	OldestTime int64              `json:"oldestTime"`
	Lines      []BlameHeatmapLine `json:"lines"`
	ChurnScore int                `json:"churnScore"`
	TopCommit  string             `json:"topCommit"`
}

// GetFileBlameHeatmap calculates age gradients and churn distribution for all lines of a file.
func (a *GitService) GetFileBlameHeatmap(repoPath, filePath string) (*BlameHeatmapResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "blame", "--line-porcelain", "--", filePath)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame failed: %w", err)
	}

	result := &BlameHeatmapResult{
		FilePath: filePath,
		Lines:    []BlameHeatmapLine{},
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	commitCounts := make(map[string]int)

	var currentHash string
	var currentAuthor string
	var currentAuthorTime int64
	currentLineNumber := 1

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "\t") {
			// Tab prefix denotes the actual source line content
			content := line[1:]
			ageDays := int(math.Floor(time.Since(time.Unix(currentAuthorTime, 0)).Hours() / 24))
			if ageDays < 0 {
				ageDays = 0
			}

			var ageTier string
			switch {
			case ageDays <= 7:
				ageTier = "recent"
			case ageDays <= 30:
				ageTier = "month"
			case ageDays <= 90:
				ageTier = "quarter"
			case ageDays <= 365:
				ageTier = "year"
			default:
				ageTier = "ancient"
			}

			if result.NewestTime == 0 || currentAuthorTime > result.NewestTime {
				result.NewestTime = currentAuthorTime
			}
			if result.OldestTime == 0 || currentAuthorTime < result.OldestTime {
				result.OldestTime = currentAuthorTime
			}

			result.Lines = append(result.Lines, BlameHeatmapLine{
				LineNumber: currentLineNumber,
				CommitHash: currentHash,
				Author:     currentAuthor,
				Timestamp:  currentAuthorTime,
				AgeDays:    ageDays,
				AgeTier:    ageTier,
				Content:    content,
			})

			commitCounts[currentHash]++
			currentLineNumber++
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) >= 3 && len(parts[0]) == 40 {
			currentHash = parts[0]
			continue
		}

		if strings.HasPrefix(line, "author ") {
			currentAuthor = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "author-time ") {
			sec, _ := strconv.ParseInt(strings.TrimPrefix(line, "author-time "), 10, 64)
			currentAuthorTime = sec
		}
	}

	result.TotalLines = len(result.Lines)

	// Calculate normalized heat ratio (1.0 = newest, 0.0 = oldest)
	timeSpan := float64(result.NewestTime - result.OldestTime)
	for i := range result.Lines {
		if timeSpan > 0 {
			ratio := float64(result.Lines[i].Timestamp-result.OldestTime) / timeSpan
			if ratio < 0.0 {
				ratio = 0.0
			} else if ratio > 1.0 {
				ratio = 1.0
			}
			result.Lines[i].HeatRatio = math.Round(ratio*100) / 100
		} else {
			result.Lines[i].HeatRatio = 1.0
		}
	}

	// Find top commit
	topCount := 0
	for h, c := range commitCounts {
		if c > topCount {
			topCount = c
			result.TopCommit = h
		}
	}

	if result.TotalLines > 0 {
		result.ChurnScore = int(float64(len(commitCounts)) / float64(result.TotalLines) * 100)
		if result.ChurnScore > 100 {
			result.ChurnScore = 100
		}
	}

	return result, nil
}
