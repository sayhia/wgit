package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validateRepoRelativePath rejects empty/absolute paths and any path that would
// escape the repository root (e.g. "../../etc/passwd").
func validateRepoRelativePath(repoPath string, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", errors.New("file path cannot be empty")
	}
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) {
		return "", errors.New("file path must be relative to the repository")
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", errors.New("file path escapes the repository")
	}
	return filepath.Join(repoPath, clean), nil
}

// repoRelativePathFromInput normalizes a user-supplied path that may be
// repo-relative or absolute-inside-the-repo, and rejects anything that escapes
// the repository via ".." or an absolute path elsewhere.
func repoRelativePathFromInput(repoPath string, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("file path cannot be empty")
	}
	clean := filepath.Clean(input)
	if filepath.IsAbs(clean) {
		rel, err := filepath.Rel(filepath.Clean(repoPath), clean)
		if err != nil {
			return "", errors.New("file path must stay inside the repository")
		}
		clean = rel
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", errors.New("file path must stay inside the repository")
	}
	return clean, nil
}

func isTrackedPath(repoPath string, rel string) bool {
	_, err := ExecGit(repoPath, "ls-files", "--error-unmatch", "--", rel)
	return err == nil
}

// DeleteRepoFile removes a tracked file through `git rm -f` (staging the
// deletion) or deletes an untracked file/directory from disk.
func (GitService) DeleteRepoFile(repoPath string, filePath string) error {
	target, err := validateRepoRelativePath(repoPath, filePath)
	if err != nil {
		return err
	}
	if _, statErr := os.Lstat(target); statErr != nil {
		return fmt.Errorf("file not found: %s", filePath)
	}
	if isTrackedPath(repoPath, filePath) {
		_, err := ExecGit(repoPath, "rm", "-f", "--", filePath)
		return err
	}
	return os.RemoveAll(target)
}

// RenameRepoFile renames/moves a file. Tracked files use `git mv` so the rename
// is staged; untracked files are moved with a plain filesystem rename.
func (GitService) RenameRepoFile(repoPath string, oldPath string, newPath string) error {
	if _, err := validateRepoRelativePath(repoPath, oldPath); err != nil {
		return err
	}
	target, err := validateRepoRelativePath(repoPath, newPath)
	if err != nil {
		return err
	}
	if _, statErr := os.Lstat(target); statErr == nil {
		return fmt.Errorf("target path already exists: %s", newPath)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if isTrackedPath(repoPath, oldPath) {
		_, err := ExecGit(repoPath, "mv", "--", oldPath, newPath)
		return err
	}
	source, err := validateRepoRelativePath(repoPath, oldPath)
	if err != nil {
		return err
	}
	return os.Rename(source, target)
}

// UntrackRepoFile stops tracking a file while keeping it on disk
// (`git rm --cached`), useful for large assets that should leave the index.
func (GitService) UntrackRepoFile(repoPath string, filePath string) error {
	if _, err := validateRepoRelativePath(repoPath, filePath); err != nil {
		return err
	}
	_, err := ExecGit(repoPath, "rm", "--cached", "-f", "--", filePath)
	return err
}

// AddGitignorePattern appends a pattern to .gitignore (idempotent).
func (GitService) AddGitignorePattern(repoPath string, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return errors.New("gitignore pattern cannot be empty")
	}
	ignorePath := filepath.Join(repoPath, ".gitignore")
	existing, readErr := os.ReadFile(ignorePath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if len(existing) > 0 {
		for _, line := range strings.Split(string(existing), "\n") {
			if strings.TrimSpace(line) == pattern {
				return nil
			}
		}
	}
	prefix := ""
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		prefix = "\n"
	}
	f, err := os.OpenFile(ignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(prefix + pattern + "\n")
	return err
}

// GetGitignorePatterns returns the non-empty pattern lines from .gitignore.
func (GitService) GetGitignorePatterns(repoPath string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, ".gitignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	patterns := make([]string, 0, 16)
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			patterns = append(patterns, trimmed)
		}
	}
	return patterns, nil
}

// RemoveGitignorePattern deletes every exact occurrence of pattern from
// .gitignore while preserving comments, blank lines and other rules.
func (GitService) RemoveGitignorePattern(repoPath string, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return errors.New("gitignore pattern cannot be empty")
	}
	ignorePath := filepath.Join(repoPath, ".gitignore")
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := strings.Split(string(data), "\n")
	kept := make([]string, 0, len(lines))
	removed := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == pattern {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return nil
	}
	// Preserve the original trailing-newline shape when possible.
	out := strings.Join(kept, "\n")
	if strings.HasSuffix(string(data), "\n") && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if out == "" || out == "\n" {
		// Replacing with an empty file keeps re-enabling the rule later smooth.
		out = ""
	}
	return os.WriteFile(ignorePath, []byte(out), 0o644)
}

// StageRepoPath stages all changes under a repository-relative path
// (stage a whole directory from the source explorer shortcut).
func (GitService) StageRepoPath(repoPath string, path string) error {
	if _, err := validateRepoRelativePath(repoPath, path); err != nil {
		return err
	}
	_, err := ExecGit(repoPath, "add", "-A", "--", path)
	return err
}

// UnstageRepoPath unstages changes under a repository-relative path.
func (GitService) UnstageRepoPath(repoPath string, path string) error {
	if _, err := validateRepoRelativePath(repoPath, path); err != nil {
		return err
	}
	if _, err := ExecGit(repoPath, "restore", "--staged", "--", path); err != nil {
		// Older git versions (or unborn HEAD) fall back to reset.
		_, err = ExecGit(repoPath, "reset", "-q", "HEAD", "--", path)
		return err
	}
	return nil
}

// UpdateGitignorePattern renames an existing ignore rule in place. The new
// pattern must be single-line and must not already exist as another rule.
func (GitService) UpdateGitignorePattern(repoPath string, oldPattern string, newPattern string) error {
	oldPattern = strings.TrimSpace(oldPattern)
	newPattern = strings.TrimSpace(newPattern)
	if oldPattern == "" || newPattern == "" {
		return errors.New("gitignore pattern cannot be empty")
	}
	if strings.ContainsAny(newPattern, "\r\n") || strings.ContainsAny(oldPattern, "\r\n") {
		return errors.New("gitignore pattern must be a single line")
	}
	ignorePath := filepath.Join(repoPath, ".gitignore")
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("gitignore file does not exist")
		}
		return err
	}
	lines := strings.Split(string(data), "\n")
	replaced := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == newPattern && newPattern != oldPattern {
			return fmt.Errorf("gitignore rule already exists: %s", newPattern)
		}
		if trimmed == oldPattern {
			lines[i] = newPattern
			replaced++
		}
	}
	if replaced == 0 {
		return fmt.Errorf("gitignore rule not found: %s", oldPattern)
	}
	return os.WriteFile(ignorePath, []byte(strings.Join(lines, "\n")), 0o644)
}

// SwapGitignorePatterns swaps the physical lines of two existing patterns,
// letting the UI reorder rules (order matters for negation rules such as !foo).
func (GitService) SwapGitignorePatterns(repoPath string, firstPattern string, secondPattern string) error {
	firstPattern = strings.TrimSpace(firstPattern)
	secondPattern = strings.TrimSpace(secondPattern)
	if firstPattern == "" || secondPattern == "" {
		return errors.New("gitignore pattern cannot be empty")
	}
	if firstPattern == secondPattern {
		return nil
	}
	ignorePath := filepath.Join(repoPath, ".gitignore")
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	firstIdx := -1
	secondIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if firstIdx < 0 && trimmed == firstPattern {
			firstIdx = i
			continue
		}
		if secondIdx < 0 && trimmed == secondPattern {
			secondIdx = i
		}
	}
	if firstIdx < 0 {
		return fmt.Errorf("gitignore rule not found: %s", firstPattern)
	}
	if secondIdx < 0 {
		return fmt.Errorf("gitignore rule not found: %s", secondPattern)
	}
	lines[firstIdx], lines[secondIdx] = lines[secondIdx], lines[firstIdx]
	return os.WriteFile(ignorePath, []byte(strings.Join(lines, "\n")), 0o644)
}
