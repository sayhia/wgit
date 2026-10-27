package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type ChangelogEntry struct {
	Hash       string `json:"hash"`
	ShortHash  string `json:"shortHash"`
	Subject    string `json:"subject"`
	Scope      string `json:"scope,omitempty"`
	Type       string `json:"type"`
	Author     string `json:"author"`
	Date       string `json:"date"`
	IsBreaking bool   `json:"isBreaking"`
}

type ReleaseChangelogResult struct {
	FromRef         string                     `json:"fromRef"`
	ToRef           string                     `json:"toRef"`
	TotalCommits    int                        `json:"totalCommits"`
	Categories      map[string][]ChangelogEntry `json:"categories"`
	BreakingChanges []ChangelogEntry           `json:"breakingChanges"`
	Contributors    []string                   `json:"contributors"`
	Markdown        string                     `json:"markdown"`
}

var conventionalCommitRe = regexp.MustCompile(`^([a-zA-Z]+)(\(([^)]+)\))?(!)?:\s*(.+)$`)

func (s GitService) GenerateReleaseChangelog(repoPath string, fromRef string, toRef string) (*ReleaseChangelogResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	fromRef = strings.TrimSpace(fromRef)
	toRef = strings.TrimSpace(toRef)
	if toRef == "" {
		toRef = "HEAD"
	}

	if fromRef == "" {
		// Auto-detect last tag reachable from toRef
		lastTag, err := ExecGit(repoPath, "describe", "--tags", "--abbrev=0", toRef)
		if err == nil && strings.TrimSpace(lastTag) != "" {
			fromRef = strings.TrimSpace(lastTag)
		}
	}

	var rangeSpec string
	if fromRef != "" {
		rangeSpec = fmt.Sprintf("%s..%s", fromRef, toRef)
	} else {
		rangeSpec = toRef
	}

	// Delimit records by \x1e and fields by \x1f
	logOut, err := ExecGit(repoPath, "log", "--max-count=300", "--date=short", "--format=%H%x1f%s%x1f%an%x1f%ad%x1f%b%x1e", rangeSpec)
	if err != nil {
		if fromRef != "" {
			logOut, err = ExecGit(repoPath, "log", "--max-count=100", "--date=short", "--format=%H%x1f%s%x1f%an%x1f%ad%x1f%b%x1e", toRef)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to query git log: %w", err)
		}
	}

	records := strings.Split(logOut, "\x1e")
	entries := make([]ChangelogEntry, 0, len(records))
	contribMap := make(map[string]struct{})
	categories := map[string][]ChangelogEntry{
		"feat":     {},
		"fix":      {},
		"perf":     {},
		"refactor": {},
		"docs":     {},
		"chore":    {},
		"other":    {},
	}
	var breaking []ChangelogEntry

	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		parts := strings.Split(rec, "\x1f")
		if len(parts) < 4 {
			continue
		}
		hash := parts[0]
		subject := parts[1]
		author := parts[2]
		date := parts[3]
		body := ""
		if len(parts) > 4 {
			body = parts[4]
		}

		shortHash := hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		if author != "" {
			contribMap[author] = struct{}{}
		}

		isBreaking := strings.Contains(body, "BREAKING CHANGE:") || strings.Contains(body, "BREAKING-CHANGE:")

		cType := "other"
		scope := ""
		cleanSubject := subject

		if m := conventionalCommitRe.FindStringSubmatch(subject); len(m) >= 6 {
			parsedType := strings.ToLower(m[1])
			scope = m[3]
			if m[4] == "!" {
				isBreaking = true
			}
			cleanSubject = strings.TrimSpace(m[5])
			switch parsedType {
			case "feat", "feature":
				cType = "feat"
			case "fix", "bugfix":
				cType = "fix"
			case "perf":
				cType = "perf"
			case "refactor":
				cType = "refactor"
			case "docs":
				cType = "docs"
			case "chore", "build", "ci", "test", "style":
				cType = "chore"
			default:
				cType = "other"
			}
		}

		entry := ChangelogEntry{
			Hash:       hash,
			ShortHash:  shortHash,
			Subject:    cleanSubject,
			Scope:      scope,
			Type:       cType,
			Author:     author,
			Date:       date,
			IsBreaking: isBreaking,
		}

		entries = append(entries, entry)
		categories[cType] = append(categories[cType], entry)
		if isBreaking {
			breaking = append(breaking, entry)
		}
	}

	contributors := make([]string, 0, len(contribMap))
	for author := range contribMap {
		contributors = append(contributors, author)
	}
	sort.Strings(contributors)

	// Build formatted Markdown
	var sb strings.Builder
	titleRange := toRef
	if fromRef != "" {
		titleRange = fmt.Sprintf("%s...%s", fromRef, toRef)
	}
	sb.WriteString(fmt.Sprintf("# Release Notes (%s)\n\n", titleRange))

	if len(breaking) > 0 {
		sb.WriteString("### ⚠️ Breaking Changes\n\n")
		for _, e := range breaking {
			scopeStr := ""
			if e.Scope != "" {
				scopeStr = fmt.Sprintf("**%s**: ", e.Scope)
			}
			sb.WriteString(fmt.Sprintf("- %s%s (`%s`) by %s\n", scopeStr, e.Subject, e.ShortHash, e.Author))
		}
		sb.WriteString("\n")
	}

	sectionNames := []struct {
		key   string
		title string
		icon  string
	}{
		{"feat", "Features", "✨"},
		{"fix", "Bug Fixes", "🐛"},
		{"perf", "Performance", "⚡"},
		{"refactor", "Code Refactoring", "♻️"},
		{"docs", "Documentation", "📝"},
		{"chore", "Maintenance & Chores", "🔧"},
		{"other", "Other Changes", "📦"},
	}

	for _, sec := range sectionNames {
		items := categories[sec.key]
		if len(items) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s %s\n\n", sec.icon, sec.title))
		for _, e := range items {
			scopeStr := ""
			if e.Scope != "" {
				scopeStr = fmt.Sprintf("**%s**: ", e.Scope)
			}
			sb.WriteString(fmt.Sprintf("- %s%s (`%s`) by %s\n", scopeStr, e.Subject, e.ShortHash, e.Author))
		}
		sb.WriteString("\n")
	}

	if len(contributors) > 0 {
		sb.WriteString("### 👥 Contributors\n\n")
		for _, author := range contributors {
			sb.WriteString(fmt.Sprintf("- %s\n", author))
		}
		sb.WriteString("\n")
	}

	return &ReleaseChangelogResult{
		FromRef:         fromRef,
		ToRef:           toRef,
		TotalCommits:    len(entries),
		Categories:      categories,
		BreakingChanges: breaking,
		Contributors:    contributors,
		Markdown:        sb.String(),
	}, nil
}
