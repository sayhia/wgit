package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	fakeGitToken = "ghp_ABCDEFGHIJKLMNOPQRSTUVWX1234"
	fakeGitValue = "supersecret"
)

// credentialLeaks must never survive into an error returned to the UI/RPC
// layer, where they would be rendered verbatim.
var credentialLeaks = []string{fakeGitToken, fakeGitValue, "user:"}

// fakeGitDir installs a fake git executable in a temp dir and prepends that
// dir to PATH, so execGitTimeout resolves "git" to the script.
func fakeGitDir(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	writeFakeGitScript(t, dir, script)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func writeFakeGitScript(t *testing.T, dir, script string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
}

func assertNoCredentialLeak(t *testing.T, errText string) {
	t.Helper()

	for _, leak := range credentialLeaks {
		if strings.Contains(errText, leak) {
			t.Fatalf("git error leaked %q: %q", leak, errText)
		}
	}
}

// TestExecGitErrorRedactsCredentials ensures the error returned by ExecGit does
// not echo a remote URL carrying embedded credentials or a query token.
func TestExecGitErrorRedactsCredentials(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake git uses a POSIX shell script")
	}

	fakeGitDir(t, `echo "fatal: unable to access 'https://user:`+fakeGitToken+`@example.com/o/r.git/?token=`+fakeGitValue+`'" >&2
exit 128`)

	_, err := ExecGit(t.TempDir(), "fetch")
	if err == nil {
		t.Fatal("expected ExecGit to fail")
	}

	errText := err.Error()
	assertNoCredentialLeak(t, errText)

	if !strings.Contains(errText, "fatal") && !strings.Contains(errText, "***") {
		t.Fatalf("git error lost its context: %q", errText)
	}
}

// TestExecGitTimeoutErrorRedactsCredentials covers the deadline branch, whose
// message embeds stderr directly and previously bypassed redaction.
func TestExecGitTimeoutErrorRedactsCredentials(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake git uses a POSIX shell script")
	}

	// Warm up the shell interpreter and the script file first: a cold exec can
	// take hundreds of milliseconds, which would let the 100ms deadline fire
	// before the fake git manages to write to stderr.
	dir := fakeGitDir(t, "exit 128")
	if _, err := ExecGit(t.TempDir(), "fetch"); err == nil {
		t.Fatal("expected the warm-up git invocation to fail")
	}

	writeFakeGitScript(t, dir, `echo "fatal: https://user:`+fakeGitToken+`@example.com/x?token=`+fakeGitValue+`" >&2
sleep 1`)

	start := time.Now()
	_, err := ExecGitWithTimeout(t.TempDir(), 100*time.Millisecond, "fetch")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected ExecGitWithTimeout to fail")
	}

	errText := err.Error()
	assertNoCredentialLeak(t, errText)

	if !strings.Contains(errText, "timed out") {
		t.Fatalf("timeout error lost its suffix: %q", errText)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("timeout test took %s, want under 3s", elapsed)
	}
}
