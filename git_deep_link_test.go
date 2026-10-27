package main

import (
	"testing"
)

func TestInitialDeepLink(t *testing.T) {
	svc := GitService{}

	// Initial should be empty or whatever was set
	SetInitialDeepLink("")
	link, err := svc.GetInitialDeepLink()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link != "" {
		t.Fatalf("expected empty link, got: %s", link)
	}

	// Set and retrieve
	testURL := "wgit://open?repo=%2Ftmp%2Ftest-repo&tab=history&ref=main"
	SetInitialDeepLink(testURL)

	link, err = svc.GetInitialDeepLink()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link != testURL {
		t.Fatalf("expected %s, got %s", testURL, link)
	}

	// Should be consumed (one-time read)
	link, err = svc.GetInitialDeepLink()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link != "" {
		t.Fatalf("expected consumed link to be empty, got: %s", link)
	}
}
