package main

import (
	"strings"
	"testing"
	"time"
)

// TestActivityLogRedactsGitCommandCredentials ensures the in-memory activity
// log never stores credentials taken from command arguments or from
// stdout/stderr text that echoes URLs, while keeping the entry readable.
func TestActivityLogRedactsGitCommandCredentials(t *testing.T) {
	ClearRecentActivityLogs()

	const (
		dir          = "/tmp/redaction-repo"
		secret       = "supersecret"
		querySecret  = "abc123"
		credential   = "https://user:" + secret + "@example.com/repo.git?access_token=" + querySecret
		benignStatus = "git status --porcelain"
	)

	recordGitCommand(
		dir,
		[]string{"remote", "add", "origin", credential},
		"cloned https://user:"+secret+"@example.com/x",
		"fatal: unable to access 'https://user:"+secret+"@example.com/x'",
		5*time.Millisecond,
		false,
	)
	recordGitCommand(dir, []string{"status", "--porcelain"}, " M file.go\n", "", time.Millisecond, true)

	logs := GetRecentActivityLogs()
	if len(logs) != 2 {
		t.Fatalf("expected 2 activity log entries, got %d: %+v", len(logs), logs)
	}

	entry := logs[0]
	joined := entry.Command + "\n" + entry.Output + "\n" + entry.Error

	for _, leaked := range []string{secret, querySecret} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("activity log leaked %q: %+v", leaked, entry)
		}
	}

	if !strings.Contains(entry.Command, "remote add origin") {
		t.Fatalf("command lost its arguments: %q", entry.Command)
	}
	if !strings.Contains(entry.Command, "***") {
		t.Fatalf("command was not masked: %q", entry.Command)
	}
	if !strings.Contains(entry.Command, "example.com/repo.git") {
		t.Fatalf("command lost the remote host/path: %q", entry.Command)
	}
	if !strings.Contains(entry.Command, "access_token=***") {
		t.Fatalf("command query secret was not masked: %q", entry.Command)
	}

	if !strings.Contains(entry.Output, "cloned https://***@example.com/x") {
		t.Fatalf("output was not masked: %q", entry.Output)
	}
	if !strings.Contains(entry.Error, "fatal: unable to access 'https://***@example.com/x'") {
		t.Fatalf("error was not masked: %q", entry.Error)
	}

	if entry.Dir != dir || entry.DurationMs != 5 || entry.Success {
		t.Fatalf("entry metadata changed: %+v", entry)
	}

	benign := logs[1]
	if benign.Command != benignStatus {
		t.Fatalf("benign command was altered: %q", benign.Command)
	}
	if benign.Output != " M file.go\n" {
		t.Fatalf("benign output was altered: %q", benign.Output)
	}
	if benign.Error != "" || !benign.Success {
		t.Fatalf("benign entry metadata was altered: %+v", benign)
	}

	ClearRecentActivityLogs()
}

// TestActivityLogRedactsBareTokens covers secrets echoed without a URL wrapper:
// provider-prefixed access tokens, AWS access key IDs, and key=value secrets.
func TestActivityLogRedactsBareTokens(t *testing.T) {
	ClearRecentActivityLogs()
	defer ClearRecentActivityLogs()

	const (
		dir = "/tmp/bare-token-repo"
		// 20+ characters after each provider prefix.
		gitHubSuffix = "a1B2c3D4e5a1B2c3D4e5a1B2c3D4e5" // 30 alphanumerics
		gitLabSuffix = "Ab_1-Ab_1-Ab_1-Ab_1-Ab_1-"      // 25 [A-Za-z0-9_-]
		awsSuffix    = "ABCDEFGHIJKLMNOP"               // 16 uppercase alphanumerics
		queryValue   = "supersecret123"
		benign       = "fix token handling"
	)

	gitHubToken := "ghp_" + gitHubSuffix
	gitLabToken := "glpat-" + gitLabSuffix
	awsKey := "AKIA" + awsSuffix

	recordGitCommand(
		dir,
		[]string{"push", "origin", gitHubToken},
		"fetched with "+gitLabToken+" and "+awsKey+"\n"+benign,
		"fatal: authentication failed token="+queryValue+"&ok=1",
		time.Millisecond,
		false,
	)

	logs := GetRecentActivityLogs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 activity log entry, got %d: %+v", len(logs), logs)
	}

	entry := logs[0]
	joined := entry.Command + "\n" + entry.Output + "\n" + entry.Error

	for _, leaked := range []string{
		gitHubToken, gitLabToken, awsKey,
		gitHubSuffix, gitLabSuffix, awsSuffix,
		queryValue,
	} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("activity log leaked %q: command=%q output=%q error=%q", leaked, entry.Command, entry.Output, entry.Error)
		}
	}

	for _, marker := range []string{"ghp_***", "glpat-***", "AKIA***", "token=***"} {
		if !strings.Contains(joined, marker) {
			t.Fatalf("activity log is missing redaction marker %q: command=%q output=%q error=%q", marker, entry.Command, entry.Output, entry.Error)
		}
	}

	if !strings.Contains(entry.Command, "push origin") {
		t.Fatalf("command lost its readable arguments: %q", entry.Command)
	}
	if !strings.Contains(entry.Output, benign) {
		t.Fatalf("plain text %q was altered: %q", benign, entry.Output)
	}
	if !strings.Contains(entry.Error, "token=***&ok=1") {
		t.Fatalf("key=value secret was not masked without eating the query tail: %q", entry.Error)
	}
}
