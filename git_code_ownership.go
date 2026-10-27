package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CodeOwnershipEntry struct {
	Author         string  `json:"author"`
	Email          string  `json:"email"`
	CommitCount    int     `json:"commitCount"`
	LinesAuthored  int     `json:"linesAuthored"`
	Percentage     float64 `json:"percentage"`
	LastActiveDate string  `json:"lastActiveDate"`
	Role           string  `json:"role"` // "primary_owner", "major_contributor", "minor_contributor"
}

type CodeOwnershipReport struct {
	TargetPath      string               `json:"targetPath"`
	TotalCommits    int                  `json:"totalCommits"`
	TotalLines      int                  `json:"totalLines"`
	BusFactor       int                  `json:"busFactor"`
	BusFactorAlert  string               `json:"busFactorAlert"` // "critical", "warning", "healthy"
	Owners          []CodeOwnershipEntry `json:"owners"`
	Recommendations []string             `json:"recommendations"`
}

type FileTimelineEntry struct {
	CommitHash   string `json:"commitHash"`
	Author       string `json:"author"`
	AuthorDate   string `json:"authorDate"`
	Subject      string `json:"subject"`
	Content      string `json:"content"`
	DiffSummary  string `json:"diffSummary"`
	LinesAdded   int    `json:"linesAdded"`
	LinesDeleted int    `json:"linesDeleted"`
}

type authorAgg struct {
	name       string
	email      string
	commits    int
	lines      int
	lastActive time.Time
}

func (GitService) GetCodeOwnership(repoPath string, targetPath string) (*CodeOwnershipReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	args := []string{"log", "--format=COMMIT|%an|%ae|%aI", "--numstat", "-n", "300", "HEAD"}
	targetPath = strings.TrimSpace(targetPath)
	if targetPath != "" && targetPath != "." {
		args = append(args, "--", targetPath)
	}

	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to run git log for ownership: %w", err)
	}

	authorMap := make(map[string]*authorAgg)
	var currentAuthor *authorAgg
	totalCommits := 0
	totalLines := 0

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "COMMIT|") {
			parts := strings.Split(line, "|")
			name := "Unknown"
			email := ""
			var activeTime time.Time
			if len(parts) >= 2 {
				name = parts[1]
			}
			if len(parts) >= 3 {
				email = parts[2]
			}
			if len(parts) >= 4 {
				activeTime, _ = time.Parse(time.RFC3339, parts[3])
			}

			key := strings.ToLower(name)
			agg, exists := authorMap[key]
			if !exists {
				agg = &authorAgg{
					name:       name,
					email:      email,
					lastActive: activeTime,
				}
				authorMap[key] = agg
			}
			agg.commits++
			totalCommits++
			if activeTime.After(agg.lastActive) {
				agg.lastActive = activeTime
			}
			currentAuthor = agg
			continue
		}

		// numstat line: added \t deleted \t path
		if currentAuthor != nil {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				a, _ := strconv.Atoi(fields[0])
				d, _ := strconv.Atoi(fields[1])
				netLines := a + d
				currentAuthor.lines += netLines
				totalLines += netLines
			}
		}
	}

	var rawOwners []*authorAgg
	for _, agg := range authorMap {
		rawOwners = append(rawOwners, agg)
	}

	sort.Slice(rawOwners, func(i, j int) bool {
		if rawOwners[i].lines != rawOwners[j].lines {
			return rawOwners[i].lines > rawOwners[j].lines
		}
		return rawOwners[i].commits > rawOwners[j].commits
	})

	var owners []CodeOwnershipEntry
	for _, raw := range rawOwners {
		pct := 0.0
		if totalLines > 0 {
			pct = (float64(raw.lines) / float64(totalLines)) * 100.0
		} else if totalCommits > 0 {
			pct = (float64(raw.commits) / float64(totalCommits)) * 100.0
		}

		role := "minor_contributor"
		if pct >= 50.0 {
			role = "primary_owner"
		} else if pct >= 15.0 {
			role = "major_contributor"
		}

		lastActiveStr := ""
		if !raw.lastActive.IsZero() {
			lastActiveStr = raw.lastActive.Format(time.RFC3339)
		}

		owners = append(owners, CodeOwnershipEntry{
			Author:         raw.name,
			Email:          raw.email,
			CommitCount:    raw.commits,
			LinesAuthored:  raw.lines,
			Percentage:     pct,
			LastActiveDate: lastActiveStr,
			Role:           role,
		})
	}

	busFactor := 0
	cumPct := 0.0
	for _, o := range owners {
		busFactor++
		cumPct += o.Percentage
		if cumPct >= 80.0 {
			break
		}
	}
	if busFactor == 0 && len(owners) > 0 {
		busFactor = 1
	}

	alert := "healthy"
	if busFactor <= 1 {
		alert = "critical"
	} else if busFactor == 2 {
		alert = "warning"
	}

	var recs []string
	if alert == "critical" {
		recs = append(recs, fmt.Sprintf("总线系数偏低 (Bus Factor = %d)：核心代码由单一作者主导，建议安排代码评审移交与备份作者以防知识孤岛。", busFactor))
	} else if alert == "warning" {
		recs = append(recs, fmt.Sprintf("总线系数适中 (Bus Factor = %d)：建议吸纳更多团队成员参与此模块的核心迭代。", busFactor))
	} else {
		recs = append(recs, "总线系数健康：代码知识在多位活跃成员间均匀分布，模块协作韧性良好。")
	}

	if targetPath == "" {
		targetPath = "."
	}

	return &CodeOwnershipReport{
		TargetPath:      targetPath,
		TotalCommits:    totalCommits,
		TotalLines:      totalLines,
		BusFactor:       busFactor,
		BusFactorAlert:  alert,
		Owners:          owners,
		Recommendations: recs,
	}, nil
}

func (GitService) GetFileCommitTimeline(repoPath string, filePath string, maxCommits int) ([]FileTimelineEntry, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("filePath cannot be empty")
	}
	if maxCommits <= 0 || maxCommits > 50 {
		maxCommits = 20
	}

	args := []string{"log", "--follow", fmt.Sprintf("-n%d", maxCommits), "--format=%H|%an|%aI|%s", "--", filePath}
	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	lines := strings.Split(out, "\n")
	var entries []FileTimelineEntry
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.Split(l, "|")
		if len(parts) < 4 {
			continue
		}
		hash := parts[0]
		author := parts[1]
		date := parts[2]
		subject := parts[3]

		content, _ := ExecGit(repoPath, "show", fmt.Sprintf("%s:%s", hash, filePath))
		if len(content) > 100000 {
			content = content[:100000] + "\n... (truncated)"
		}

		numstatOut, _ := ExecGit(repoPath, "show", "--numstat", "--format=", hash, "--", filePath)
		added := 0
		deleted := 0
		for _, nl := range strings.Split(numstatOut, "\n") {
			fields := strings.Fields(nl)
			if len(fields) >= 2 {
				a, _ := strconv.Atoi(fields[0])
				d, _ := strconv.Atoi(fields[1])
				added += a
				deleted += d
			}
		}

		diffSummary := fmt.Sprintf("+%d -%d", added, deleted)

		entries = append(entries, FileTimelineEntry{
			CommitHash:   hash,
			Author:       author,
			AuthorDate:   date,
			Subject:      subject,
			Content:      content,
			DiffSummary:  diffSummary,
			LinesAdded:   added,
			LinesDeleted: deleted,
		})
	}

	// Reverse so oldest commit is first (step forward through time)
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}
