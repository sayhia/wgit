package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installFakeGit writes an executable `git` shim into dir and makes it the
// first entry on PATH. The real git path must already have been resolved by
// the caller, otherwise the shim shadows the binary used for that lookup.
func installFakeGit(t *testing.T, dir, script string) {
	t.Helper()
	path := filepath.Join(dir, "git")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake git shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestExecGitBytesRedactsCredentialsInStderr guarantees that git filter/hook
// output echoing credential-bearing URLs never reaches the UI verbatim.
func TestExecGitBytesRedactsCredentialsInStderr(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil || realGit == "" {
		t.Fatalf("real git is required to validate the shim: %v", err)
	}

	dir := t.TempDir()
	installFakeGit(t, dir, `#!/bin/sh
echo "fatal: unable to access 'https://user:ghp_ABCDEFGHIJKLMNOPQRSTUVWX1234@example.com/o/r.git/?token=supersecret'" >&2
exit 128
`)

	_, err = execGitBytes(t.TempDir(), "show", "HEAD:x")
	if err == nil {
		t.Fatalf("expected execGitBytes to fail when the shim exits 128 (real git: %s)", realGit)
	}
	msg := err.Error()
	for _, leaked := range []string{"ghp_ABCDEFGHIJKLMNOPQRSTUVWX1234", "supersecret", "user:"} {
		if strings.Contains(msg, leaked) {
			t.Fatalf("stderr leaked %q into the error: %q", leaked, msg)
		}
	}
	if !strings.Contains(msg, "***") {
		t.Fatalf("expected masked credentials in the error, got %q", msg)
	}
	if !strings.Contains(msg, "fatal") {
		t.Fatalf("expected the original diagnostic to survive redaction, got %q", msg)
	}
}

// TestExecGitBytesPreservesBinaryStdout pins the byte-for-byte contract that
// ExecGit cannot provide: NUL bytes and surrounding whitespace must not be
// trimmed or otherwise transformed.
func TestExecGitBytesPreservesBinaryStdout(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil || realGit == "" {
		t.Fatalf("real git is required to validate the shim: %v", err)
	}

	dir := t.TempDir()
	payload := []byte(" \t\n\x00\x01\xfePNG\x00blob payload \r\n")
	payloadPath := filepath.Join(dir, "stdout.bin")
	if err := os.WriteFile(payloadPath, payload, 0644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	installFakeGit(t, dir, fmt.Sprintf("#!/bin/sh\ncat %q\nexit 0\n", payloadPath))

	out, err := execGitBytes(t.TempDir(), "show", "HEAD:x")
	if err != nil {
		t.Fatalf("expected a successful read (real git: %s), got %v", realGit, err)
	}
	if !bytes.Equal(out, payload) {
		t.Fatalf("stdout was modified:\n got %q\nwant %q", out, payload)
	}
}
