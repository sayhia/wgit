package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ThreeWayMergeBlock struct {
	Index       int      `json:"index"`
	StartLine   int      `json:"startLine"`
	EndLine     int      `json:"endLine"`
	OursLines   []string `json:"oursLines"`
	BaseLines   []string `json:"baseLines"`
	TheirsLines []string `json:"theirsLines"`
}

type ThreeWayMergeData struct {
	FilePath       string               `json:"filePath"`
	BaseContent    string               `json:"baseContent"`
	OursContent    string               `json:"oursContent"`
	TheirsContent  string               `json:"theirsContent"`
	CurrentContent string               `json:"currentContent"`
	Blocks         []ThreeWayMergeBlock `json:"blocks"`
	TotalConflicts int                  `json:"totalConflicts"`
}

// parseThreeWayBlocks parses conflict blocks from raw lines, extracting ours, base (if diff3), and theirs lines.
func parseThreeWayBlocks(lines []string) []ThreeWayMergeBlock {
	var blocks []ThreeWayMergeBlock
	const (
		stateOutside = iota
		stateOurs
		stateBase
		stateTheirs
	)

	state := stateOutside
	currStart := -1
	var currOurs []string
	var currBase []string
	var currTheirs []string

	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(trimmed, "<<<<<<<"):
			state = stateOurs
			currStart = i
			currOurs = nil
			currBase = nil
			currTheirs = nil
		case strings.HasPrefix(trimmed, "|||||||") && state == stateOurs:
			state = stateBase
		case (trimmed == "=======" || strings.HasPrefix(trimmed, "=======")) && (state == stateOurs || state == stateBase):
			state = stateTheirs
		case strings.HasPrefix(trimmed, ">>>>>>>") && state == stateTheirs:
			blocks = append(blocks, ThreeWayMergeBlock{
				Index:       len(blocks),
				StartLine:   currStart,
				EndLine:     i,
				OursLines:   currOurs,
				BaseLines:   currBase,
				TheirsLines: currTheirs,
			})
			state = stateOutside
			currStart = -1
			currOurs = nil
			currBase = nil
			currTheirs = nil
		default:
			switch state {
			case stateOurs:
				currOurs = append(currOurs, line)
			case stateBase:
				currBase = append(currBase, line)
			case stateTheirs:
				currTheirs = append(currTheirs, line)
			}
		}
	}

	return blocks
}

// GetThreeWayMergeData reads the conflicted file and extracts Base, Ours, and Theirs versions.
func (GitService) GetThreeWayMergeData(repoPath string, filePath string) (*ThreeWayMergeData, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(repoPath, cleanPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", cleanPath, err)
	}

	currentContent := string(data)
	normalized := strings.ReplaceAll(currentContent, "\r\n", "\n")
	rawLines := strings.Split(normalized, "\n")
	blocks := parseThreeWayBlocks(rawLines)

	// Attempt to query git stage 1 (Base), stage 2 (Ours), and stage 3
	// (Theirs). Exit 128 "path ... does not exist (in stage N)" is an ordinary
	// state for a marker file: added-in-both has no stage 1, and a conflict
	// reopened after `git add` has no stages at all. A genuine read failure
	// lands in the same place by design: the block synthesis below rebuilds
	// the streams from the conflict markers in the working file, while
	// CurrentContent read from disk above stays the ground truth shown next to
	// them. Keep these reads best-effort so the merge editor opens instead of
	// failing wholesale on a degraded stage lookup.
	baseContent, _ := ExecGit(repoPath, "show", fmt.Sprintf(":1:%s", cleanPath))
	oursContent, _ := ExecGit(repoPath, "show", fmt.Sprintf(":2:%s", cleanPath))
	theirsContent, _ := ExecGit(repoPath, "show", fmt.Sprintf(":3:%s", cleanPath))

	// If git stage content is not present, synthesize clean streams from blocks
	if strings.TrimSpace(oursContent) == "" && len(blocks) > 0 {
		var oursLines []string
		lastLine := 0
		for _, b := range blocks {
			if b.StartLine > lastLine {
				oursLines = append(oursLines, rawLines[lastLine:b.StartLine]...)
			}
			oursLines = append(oursLines, b.OursLines...)
			lastLine = b.EndLine + 1
		}
		if lastLine < len(rawLines) {
			oursLines = append(oursLines, rawLines[lastLine:]...)
		}
		oursContent = strings.Join(oursLines, "\n")
	}

	if strings.TrimSpace(theirsContent) == "" && len(blocks) > 0 {
		var theirsLines []string
		lastLine := 0
		for _, b := range blocks {
			if b.StartLine > lastLine {
				theirsLines = append(theirsLines, rawLines[lastLine:b.StartLine]...)
			}
			theirsLines = append(theirsLines, b.TheirsLines...)
			lastLine = b.EndLine + 1
		}
		if lastLine < len(rawLines) {
			theirsLines = append(theirsLines, rawLines[lastLine:]...)
		}
		theirsContent = strings.Join(theirsLines, "\n")
	}

	if strings.TrimSpace(baseContent) == "" && len(blocks) > 0 {
		var baseLines []string
		lastLine := 0
		for _, b := range blocks {
			if b.StartLine > lastLine {
				baseLines = append(baseLines, rawLines[lastLine:b.StartLine]...)
			}
			baseLines = append(baseLines, b.BaseLines...)
			lastLine = b.EndLine + 1
		}
		if lastLine < len(rawLines) {
			baseLines = append(baseLines, rawLines[lastLine:]...)
		}
		baseContent = strings.Join(baseLines, "\n")
	}

	return &ThreeWayMergeData{
		FilePath:       cleanPath,
		BaseContent:    baseContent,
		OursContent:    oursContent,
		TheirsContent:  theirsContent,
		CurrentContent: currentContent,
		Blocks:         blocks,
		TotalConflicts: len(blocks),
	}, nil
}

// SaveThreeWayMergeResult writes the resolved content back to disk, optionally auto-staging the file.
func (GitService) SaveThreeWayMergeResult(repoPath string, filePath string, resolvedContent string, autoStage bool) (bool, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return false, err
	}

	fullPath := filepath.Join(repoPath, cleanPath)
	info, err := os.Stat(fullPath)
	perm := os.FileMode(0o644)
	if err == nil {
		perm = info.Mode().Perm()
	}

	if err := os.WriteFile(fullPath, []byte(resolvedContent), perm); err != nil {
		return false, fmt.Errorf("failed to write resolved content to %s: %w", cleanPath, err)
	}

	if autoStage {
		if _, err := ExecGit(repoPath, "add", "--", cleanPath); err != nil {
			return false, fmt.Errorf("failed to auto-stage file %s: %w", cleanPath, err)
		}
	}

	return true, nil
}
