package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type WorkContextSession struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Branch      string   `json:"branch"`
	ActiveFiles []string `json:"activeFiles"`
	CommitDraft string   `json:"commitDraft"`
	IssueOrPr   string   `json:"issueOrPr"`
	Notes       string   `json:"notes"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

func getContextsDir(repoPath string) string {
	return filepath.Join(repoPath, ".wgit", "contexts")
}

// SaveWorkContextSession saves or updates a workflow context session.
func (GitService) SaveWorkContextSession(repoPath string, session WorkContextSession) error {
	if repoPath == "" {
		return errors.New("repoPath cannot be empty")
	}
	if strings.TrimSpace(session.Name) == "" {
		return errors.New("session name cannot be empty")
	}

	dir := getContextsDir(repoPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create contexts directory: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	if session.ID == "" {
		session.ID = fmt.Sprintf("ctx-%d", time.Now().UnixMilli())
		session.CreatedAt = now
	}
	session.UpdatedAt = now

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	targetPath := filepath.Join(dir, session.ID+".json")
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// ListWorkContextSessions returns all saved workflow context sessions sorted by updated time descending.
func (GitService) ListWorkContextSessions(repoPath string) ([]WorkContextSession, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	dir := getContextsDir(repoPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []WorkContextSession{}, nil
		}
		return nil, fmt.Errorf("failed to read contexts directory: %w", err)
	}

	var sessions []WorkContextSession
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		var s WorkContextSession
		if err := json.Unmarshal(data, &s); err == nil {
			sessions = append(sessions, s)
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt > sessions[j].UpdatedAt
	})

	return sessions, nil
}

// RestoreWorkContextSession loads a workflow context session by ID.
func (GitService) RestoreWorkContextSession(repoPath string, sessionID string) (*WorkContextSession, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("sessionID cannot be empty")
	}

	targetPath := filepath.Join(getContextsDir(repoPath), sessionID+".json")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session %s: %w", sessionID, err)
	}

	var session WorkContextSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session %s: %w", sessionID, err)
	}

	return &session, nil
}

// DeleteWorkContextSession deletes a workflow context session file.
func (GitService) DeleteWorkContextSession(repoPath string, sessionID string) error {
	if repoPath == "" {
		return errors.New("repoPath cannot be empty")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("sessionID cannot be empty")
	}

	targetPath := filepath.Join(getContextsDir(repoPath), sessionID+".json")
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session %s: %w", sessionID, err)
	}
	return nil
}
