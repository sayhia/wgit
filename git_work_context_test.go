package main

import (
	"os"
	"testing"
)

func TestWorkContextSessionsInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-context-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := GitService{}

	// 1. Initial list should be empty
	initialList, err := svc.ListWorkContextSessions(tempDir)
	if err != nil {
		t.Fatalf("ListWorkContextSessions failed: %v", err)
	}
	if len(initialList) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(initialList))
	}

	// 2. Save a new session
	sess := WorkContextSession{
		ID:          "ctx-feat-1",
		Name:        "Feature Checkout",
		Branch:      "feature/checkout",
		ActiveFiles: []string{"calc.go", "main.go"},
		CommitDraft: "wip: add checkout modal",
		IssueOrPr:   "#1024",
		Notes:       "need to check discount logic",
	}

	err = svc.SaveWorkContextSession(tempDir, sess)
	if err != nil {
		t.Fatalf("SaveWorkContextSession failed: %v", err)
	}

	// 3. List should have 1 item
	listAfterSave, err := svc.ListWorkContextSessions(tempDir)
	if err != nil {
		t.Fatalf("ListWorkContextSessions failed: %v", err)
	}
	if len(listAfterSave) != 1 {
		t.Fatalf("expected 1 session, got %d", len(listAfterSave))
	}
	if listAfterSave[0].Name != "Feature Checkout" {
		t.Errorf("expected name 'Feature Checkout', got %s", listAfterSave[0].Name)
	}

	// 4. Restore session
	restored, err := svc.RestoreWorkContextSession(tempDir, "ctx-feat-1")
	if err != nil {
		t.Fatalf("RestoreWorkContextSession failed: %v", err)
	}
	if restored.Branch != "feature/checkout" {
		t.Errorf("expected branch feature/checkout, got %s", restored.Branch)
	}
	if len(restored.ActiveFiles) != 2 {
		t.Errorf("expected 2 active files, got %d", len(restored.ActiveFiles))
	}

	// 5. Delete session
	err = svc.DeleteWorkContextSession(tempDir, "ctx-feat-1")
	if err != nil {
		t.Fatalf("DeleteWorkContextSession failed: %v", err)
	}

	listAfterDelete, err := svc.ListWorkContextSessions(tempDir)
	if err != nil {
		t.Fatalf("ListWorkContextSessions after delete failed: %v", err)
	}
	if len(listAfterDelete) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(listAfterDelete))
	}
}
