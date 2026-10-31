// git_conflict_resolution.go — 自 git_service.go 抽离（conflict 族），
// 见 docs/big-files-assessment.md。函数级搬移，零逻辑改动。

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ResolveConflict chooses ours, theirs, backup_and_theirs, mergetool, or marks conflict resolved
func (GitService) ResolveConflict(repoPath string, filePath string, resolution string) error {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return err
	}
	filePath = cleanPath

	switch resolution {
	case "ours":
		if _, err := ExecGit(repoPath, "checkout", "--ours", "--", filePath); err != nil {
			return err
		}
		_, err := ExecGit(repoPath, "add", "--", filePath)
		return err
	case "theirs":
		if _, err := ExecGit(repoPath, "checkout", "--theirs", "--", filePath); err != nil {
			return err
		}
		_, err := ExecGit(repoPath, "add", "--", filePath)
		return err
	case "backup_and_theirs", "backup_theirs":
		// Tencent UGit safe resolution: Backup local file then checkout theirs
		src := filepath.Join(repoPath, filePath)
		ext := filepath.Ext(filePath)
		base := strings.TrimSuffix(filePath, ext)
		ts := time.Now().Format("20060102_150405")
		backupPath := filepath.Join(repoPath, fmt.Sprintf("%s.backup_%s%s", base, ts, ext))
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("读取本地冲突文件失败，未覆盖: %w", err)
		}
		if err := os.WriteFile(backupPath, data, 0o644); err != nil {
			return fmt.Errorf("创建本地备份失败，未覆盖: %w", err)
		}
		if _, err := ExecGit(repoPath, "checkout", "--theirs", "--", filePath); err != nil {
			return err
		}
		_, err = ExecGit(repoPath, "add", "--", filePath)
		return err
	case "mergetool":
		_, err := ExecGit(repoPath, "mergetool", "-y", "--", filePath)
		return err
	case "mark", "mark_resolved":
		_, err := ExecGit(repoPath, "add", "--", filePath)
		return err
	default:
		return fmt.Errorf("unknown resolution: %s", resolution)
	}
}

// MetaConflictResolutionResult represents the outcome of bulk resolving meta/config conflicts.
type MetaConflictResolutionResult struct {
	ResolvedFiles []string `json:"resolvedFiles"`
	Strategy      string   `json:"strategy"`
	Count         int      `json:"count"`
}

// AutoResolveMetaConflicts bulk-resolves Unity .meta files and configuration files (json, yaml, yml)
// using a specified strategy ("theirs", "ours", or "backup_theirs").
func (s GitService) AutoResolveMetaConflicts(repoPath string, strategy string) (*MetaConflictResolutionResult, error) {
	if strategy == "" {
		strategy = "theirs"
	}
	switch strategy {
	case "theirs", "ours", "backup_theirs", "backup_and_theirs":
	default:
		return nil, fmt.Errorf("unsupported meta conflict resolution strategy: %s (must be 'theirs', 'ours', or 'backup_theirs')", strategy)
	}

	st, err := s.GetRepoStatus(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository status: %w", err)
	}

	result := &MetaConflictResolutionResult{
		ResolvedFiles: make([]string, 0),
		Strategy:      strategy,
		Count:         0,
	}

	for _, f := range st.ConflictedFiles {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == ".meta" || ext == ".json" || ext == ".yaml" || ext == ".yml" {
			if err := s.ResolveConflict(repoPath, f, strategy); err != nil {
				return nil, fmt.Errorf("failed to auto-resolve conflict for %s: %w", f, err)
			}
			result.ResolvedFiles = append(result.ResolvedFiles, f)
		}
	}
	result.Count = len(result.ResolvedFiles)
	return result, nil
}

// GetConflictFileContent reads content of conflicted file from disk
func (GitService) GetConflictFileContent(repoPath string, filePath string) (string, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(filepath.Join(repoPath, cleanPath))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

type ConflictBlockResolveResult struct {
	Success            bool   `json:"success"`
	Stale              bool   `json:"stale"`
	BlockIndex         int    `json:"blockIndex"`
	RemainingConflicts int    `json:"remainingConflicts"`
	FileAutoStaged     bool   `json:"fileAutoStaged"`
	Message            string `json:"message"`
}

type conflictBlock struct {
	startLine   int
	endLine     int
	oursLines   []string
	theirsLines []string
	baseLines   []string
}

func parseConflictBlocks(lines []string) []conflictBlock {
	var blocks []conflictBlock
	const (
		stateOutside = iota
		stateOurs
		stateBase
		stateTheirs
	)

	state := stateOutside
	currStart := -1
	var currOurs []string
	var currTheirs []string
	var currBase []string

	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(trimmed, "<<<<<<<"):
			state = stateOurs
			currStart = i
			currOurs = nil
			currTheirs = nil
			currBase = nil
		case strings.HasPrefix(trimmed, "|||||||") && state == stateOurs:
			state = stateBase
		case (trimmed == "=======" || strings.HasPrefix(trimmed, "=======")) && (state == stateOurs || state == stateBase):
			state = stateTheirs
			currTheirs = nil
		case strings.HasPrefix(trimmed, ">>>>>>>") && state == stateTheirs:
			blocks = append(blocks, conflictBlock{
				startLine:   currStart,
				endLine:     i,
				oursLines:   currOurs,
				theirsLines: currTheirs,
				baseLines:   currBase,
			})
			state = stateOutside
			currStart = -1
			currOurs = nil
			currTheirs = nil
			currBase = nil
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

func normalizeConflictNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// splitConflictFile returns normalized lines plus whether the original file
// used CRLF, so callers can restore the same line endings after editing.
func splitConflictFile(content string) (lines []string, isCRLF bool) {
	isCRLF = strings.Contains(content, "\r\n")
	return strings.Split(normalizeConflictNewlines(content), "\n"), isCRLF
}

// conflictReplacementLines prepares model output for insertion between two
// markers. A single trailing newline is dropped so the replacement does not
// introduce a blank line before the following conflict-marker boundary.
func conflictReplacementLines(content string) []string {
	content = strings.TrimSuffix(normalizeConflictNewlines(content), "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func replaceConflictBlockLines(lines []string, target conflictBlock, replacement []string) []string {
	newLines := make([]string, 0, len(lines)-(target.endLine-target.startLine+1)+len(replacement))
	newLines = append(newLines, lines[:target.startLine]...)
	newLines = append(newLines, replacement...)
	newLines = append(newLines, lines[target.endLine+1:]...)
	return newLines
}

func writeConflictFilePreservingMode(fullPath, content string) error {
	info, err := os.Stat(fullPath)
	perm := os.FileMode(0o644)
	if err == nil {
		perm = info.Mode().Perm()
	}
	return os.WriteFile(fullPath, []byte(content), perm)
}

func conflictBlockMatches(target conflictBlock, ours, theirs, base string) bool {
	normalize := func(s string) string {
		return strings.TrimSuffix(normalizeConflictNewlines(s), "\n")
	}
	return normalize(strings.Join(target.oursLines, "\n")) == normalize(ours) &&
		normalize(strings.Join(target.theirsLines, "\n")) == normalize(theirs) &&
		normalize(strings.Join(target.baseLines, "\n")) == normalize(base)
}

// ResolveConflictBlock resolves a specific conflict block in a file by blockIndex
// with choice ("ours", "theirs", "both"), updates the file, and auto-stages if no conflicts remain.
func (GitService) ResolveConflictBlock(repoPath string, filePath string, blockIndex int, choice string) (*ConflictBlockResolveResult, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(repoPath, cleanPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", cleanPath, err)
	}

	choiceNorm := strings.ToLower(strings.TrimSpace(choice))
	if choiceNorm != "ours" && choiceNorm != "theirs" && choiceNorm != "both" {
		return nil, fmt.Errorf("invalid choice %q: must be 'ours', 'theirs', or 'both'", choice)
	}

	rawLines, isCRLF := splitConflictFile(string(data))

	blocks := parseConflictBlocks(rawLines)
	if blockIndex < 0 || blockIndex >= len(blocks) {
		return nil, fmt.Errorf("conflict block index %d out of range (found %d blocks)", blockIndex, len(blocks))
	}

	target := blocks[blockIndex]
	var replacementLines []string
	switch choiceNorm {
	case "ours":
		replacementLines = target.oursLines
	case "theirs":
		replacementLines = target.theirsLines
	case "both":
		replacementLines = append(append([]string{}, target.oursLines...), target.theirsLines...)
	}

	newContent := strings.Join(replaceConflictBlockLines(rawLines, target, replacementLines), "\n")
	if isCRLF {
		newContent = strings.ReplaceAll(newContent, "\n", "\r\n")
	}

	if err := writeConflictFilePreservingMode(fullPath, newContent); err != nil {
		return nil, fmt.Errorf("failed to write updated file %s: %w", cleanPath, err)
	}

	remaining := strings.Count(newContent, "<<<<<<<")
	fileAutoStaged := false
	if remaining == 0 {
		if _, err := ExecGit(repoPath, "add", "--", cleanPath); err != nil {
			return nil, fmt.Errorf("failed to auto-stage file %s: %w", cleanPath, err)
		}
		fileAutoStaged = true
	}

	var msg string
	if fileAutoStaged {
		msg = fmt.Sprintf("Conflict block %d resolved with %s; all conflicts resolved and %s auto-staged", blockIndex, choiceNorm, cleanPath)
	} else {
		msg = fmt.Sprintf("Conflict block %d resolved with %s; %d conflict marker(s) remaining", blockIndex, choiceNorm, remaining)
	}

	return &ConflictBlockResolveResult{
		Success:            true,
		BlockIndex:         blockIndex,
		RemainingConflicts: remaining,
		FileAutoStaged:     fileAutoStaged,
		Message:            msg,
	}, nil
}

// ResolveConflictBlockWithCustomContent replaces one conflict block with
// AI-edited content while leaving every other line of the file untouched. The
// expected ours/theirs/base text is verified against the current file before
// writing; a mismatch returns Stale=true and performs no write, so a stale
// suggestion can never clobber an externally edited or shifted block.
func (GitService) ResolveConflictBlockWithCustomContent(repoPath string, filePath string, blockIndex int, content, expectedOurs, expectedTheirs, expectedBase string) (*ConflictBlockResolveResult, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(repoPath, cleanPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", cleanPath, err)
	}

	rawLines, isCRLF := splitConflictFile(string(data))
	blocks := parseConflictBlocks(rawLines)
	stale := func(message string) (*ConflictBlockResolveResult, error) {
		return &ConflictBlockResolveResult{
			Success:            false,
			Stale:              true,
			BlockIndex:         blockIndex,
			RemainingConflicts: len(blocks),
			Message:            message,
		}, nil
	}
	if blockIndex < 0 || blockIndex >= len(blocks) {
		return stale(fmt.Sprintf("conflict block %d no longer exists (found %d blocks)", blockIndex, len(blocks)))
	}

	target := blocks[blockIndex]
	if !conflictBlockMatches(target, expectedOurs, expectedTheirs, expectedBase) {
		return stale("conflict block changed since the suggestion was generated")
	}

	newContent := strings.Join(replaceConflictBlockLines(rawLines, target, conflictReplacementLines(content)), "\n")
	if isCRLF {
		newContent = strings.ReplaceAll(newContent, "\n", "\r\n")
	}
	if err := writeConflictFilePreservingMode(fullPath, newContent); err != nil {
		return nil, fmt.Errorf("failed to write updated file %s: %w", cleanPath, err)
	}

	remaining := strings.Count(newContent, "<<<<<<<")
	fileAutoStaged := false
	if remaining == 0 {
		if _, err := ExecGit(repoPath, "add", "--", cleanPath); err != nil {
			return nil, fmt.Errorf("failed to auto-stage file %s: %w", cleanPath, err)
		}
		fileAutoStaged = true
	}

	var msg string
	if fileAutoStaged {
		msg = fmt.Sprintf("Conflict block %d resolved with custom content; all conflicts resolved and %s auto-staged", blockIndex, cleanPath)
	} else {
		msg = fmt.Sprintf("Conflict block %d resolved with custom content; %d conflict marker(s) remaining", blockIndex, remaining)
	}

	return &ConflictBlockResolveResult{
		Success:            true,
		BlockIndex:         blockIndex,
		RemainingConflicts: remaining,
		FileAutoStaged:     fileAutoStaged,
		Message:            msg,
	}, nil
}

// ResolveConflictWithCustomContent writes custom resolved text into a conflicted file,
// replacing its content, and auto-stages the file if all conflict markers are gone.
// Deprecated for advisor use: prefer ResolveConflictBlockWithCustomContent, which
// replaces only one block. Kept for existing whole-file callers.
func (GitService) ResolveConflictWithCustomContent(repoPath string, filePath string, content string) (*ConflictBlockResolveResult, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(repoPath, cleanPath)
	info, err := os.Stat(fullPath)
	perm := os.FileMode(0o644)
	if err == nil {
		perm = info.Mode().Perm()
	}

	if err := os.WriteFile(fullPath, []byte(content), perm); err != nil {
		return nil, fmt.Errorf("failed to write updated file %s: %w", cleanPath, err)
	}

	remaining := strings.Count(content, "<<<<<<<")
	fileAutoStaged := false
	if remaining == 0 {
		if _, err := ExecGit(repoPath, "add", "--", cleanPath); err != nil {
			return nil, fmt.Errorf("failed to auto-stage file %s: %w", cleanPath, err)
		}
		fileAutoStaged = true
	}

	var msg string
	if fileAutoStaged {
		msg = fmt.Sprintf("Conflict in %s resolved with custom content and auto-staged", cleanPath)
	} else {
		msg = fmt.Sprintf("Conflict in %s updated with custom content; %d conflict marker(s) remaining", cleanPath, remaining)
	}

	return &ConflictBlockResolveResult{
		Success:            true,
		BlockIndex:         -1,
		RemainingConflicts: remaining,
		FileAutoStaged:     fileAutoStaged,
		Message:            msg,
	}, nil
}
