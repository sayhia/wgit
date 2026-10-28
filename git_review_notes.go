package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ReviewNoteExportItem struct {
	CommitHash string `json:"commitHash"`
	Content    string `json:"content"`
}

type ReviewNotesBundle struct {
	Version    int                    `json:"version"`
	ExportedAt string                 `json:"exportedAt"`
	RepoName   string                 `json:"repoName"`
	Notes      []ReviewNoteExportItem `json:"notes"`
}

type ReviewNotesImportResult struct {
	TotalImported int    `json:"totalImported"`
	Success       bool   `json:"success"`
	Message       string `json:"message"`
}

// ExportReviewNotes exports all git commit notes into a JSON bundle.
func (a *GitService) ExportReviewNotes(repoPath string, outputPath string) error {
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return fmt.Errorf("output path is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmdList := exec.CommandContext(ctx, "git", "notes", "list")
	cmdList.Dir = repoPath
	out, err := cmdList.Output()
	if err != nil {
		// No notes exist yet
		bundle := ReviewNotesBundle{
			Version:    1,
			ExportedAt: time.Now().Format(time.RFC3339),
			Notes:      []ReviewNoteExportItem{},
		}
		data, _ := json.MarshalIndent(bundle, "", "  ")
		return os.WriteFile(outputPath, data, 0644)
	}

	var items []ReviewNoteExportItem
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "<noteBlobHash> <commitHash>"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			commitHash := parts[1]
			cmdShow := exec.CommandContext(ctx, "git", "notes", "show", commitHash)
			cmdShow.Dir = repoPath
			if noteContent, err := cmdShow.Output(); err == nil {
				items = append(items, ReviewNoteExportItem{
					CommitHash: commitHash,
					Content:    string(noteContent),
				})
			}
		}
	}

	bundle := ReviewNotesBundle{
		Version:    1,
		ExportedAt: time.Now().Format(time.RFC3339),
		Notes:      items,
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode review notes: %w", err)
	}
	return os.WriteFile(outputPath, data, 0644)
}

// ImportReviewNotes reads a JSON bundle and applies commit notes to the local repository.
func (a *GitService) ImportReviewNotes(repoPath string, inputPath string) (*ReviewNotesImportResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	inputPath = strings.TrimSpace(inputPath)
	if inputPath == "" {
		return nil, fmt.Errorf("input path is required")
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read input bundle: %w", err)
	}

	var bundle ReviewNotesBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("invalid review notes bundle format: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	importedCount := 0
	for _, item := range bundle.Notes {
		if item.CommitHash == "" {
			continue
		}
		// Check if commit exists locally
		checkCmd := exec.CommandContext(ctx, "git", "cat-file", "-e", item.CommitHash)
		checkCmd.Dir = repoPath
		if err := checkCmd.Run(); err != nil {
			continue
		}

		addCmd := exec.CommandContext(ctx, "git", "notes", "add", "-f", "-m", item.Content, item.CommitHash)
		addCmd.Dir = repoPath
		if err := addCmd.Run(); err == nil {
			importedCount++
		}
	}

	return &ReviewNotesImportResult{
		TotalImported: importedCount,
		Success:       true,
		Message:       fmt.Sprintf("Successfully imported %d review note(s)", importedCount),
	}, nil
}
