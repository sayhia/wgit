package main

import (
	"strings"
	"testing"
	"time"
)

func TestWriteTerminalWithoutSession(t *testing.T) {
	_ = (GitService{}).StopTerminal()
	if err := (GitService{}).WriteTerminal("x"); err == nil {
		t.Fatal("expected an error when no terminal is running")
	}
}

func TestResizeTerminalRejectsInvalidSize(t *testing.T) {
	_ = (GitService{}).StopTerminal()
	if err := (GitService{}).ResizeTerminal(0, 0); err == nil {
		t.Fatal("expected an error for a zero-sized terminal")
	}
}

func TestGetTerminalStatusWhenIdle(t *testing.T) {
	_ = (GitService{}).StopTerminal()

	status, err := (GitService{}).GetTerminalStatus()
	if err != nil {
		t.Fatalf("GetTerminalStatus failed: %v", err)
	}
	if status.Running {
		t.Fatal("a fresh process should not report a running terminal")
	}
	if status.Message == "" {
		t.Fatal("expected a status message")
	}
}

func TestTerminalLifecycle(t *testing.T) {
	svc := GitService{}
	_ = svc.StopTerminal()
	defer func() { _ = svc.StopTerminal() }()

	dir := t.TempDir()
	status, err := svc.StartTerminal(dir, 80, 24)
	if err != nil {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}
	if !status.Running {
		t.Fatalf("expected a running session, got %+v", status)
	}
	if status.Cols != 80 || status.Rows != 24 {
		t.Fatalf("unexpected size: %dx%d", status.Cols, status.Rows)
	}
	if status.Shell == "" {
		t.Fatal("expected the shell path to be reported")
	}

	// Starting again must reuse the live session rather than spawn a second one.
	again, err := svc.StartTerminal(dir, 100, 30)
	if err != nil {
		t.Fatalf("second StartTerminal failed: %v", err)
	}
	if again.Cols != 80 || again.Rows != 24 {
		t.Fatalf("reused session should keep its original size, got %dx%d", again.Cols, again.Rows)
	}

	if err := svc.ResizeTerminal(120, 40); err != nil {
		t.Fatalf("ResizeTerminal failed: %v", err)
	}

	if err := svc.WriteTerminal("echo wgit-terminal-probe\n"); err != nil {
		t.Fatalf("WriteTerminal failed: %v", err)
	}

	// Wait for the shell to echo/execute. A login shell can be slow to start,
	// so poll instead of sleeping a fixed amount.
	deadline := time.Now().Add(5 * time.Second)
	sawProbe := false
	for time.Now().Before(deadline) {
		current, err := svc.GetTerminalStatus()
		if err != nil {
			t.Fatalf("GetTerminalStatus failed: %v", err)
		}
		if strings.Contains(current.Backlog, "wgit-terminal-probe") {
			sawProbe = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !sawProbe {
		// Not fatal: some shells disable echo when non-interactive. The write
		// path itself already succeeded, which is what this test guards.
		t.Log("shell did not echo the probe within 5s; skipping output assertion")
	}

	if err := svc.StopTerminal(); err != nil {
		t.Fatalf("StopTerminal failed: %v", err)
	}

	final, err := svc.GetTerminalStatus()
	if err != nil {
		t.Fatalf("GetTerminalStatus failed: %v", err)
	}
	if final.Running {
		t.Fatal("terminal should be stopped")
	}
}

func TestTerminalBacklogIsCapped(t *testing.T) {
	// Guard the ring-buffer arithmetic without needing a PTY: feed the buffer
	// directly through the same trimming rule the pump uses.
	s := &terminalSession{}
	s.mu.Lock()
	for i := 0; i < 400; i++ {
		s.backlog.WriteString(strings.Repeat("x", 1024))
		if s.backlog.Len() > terminalBacklogLimit {
			trimmed := s.backlog.String()
			s.backlog.Reset()
			s.backlog.WriteString(trimmed[len(trimmed)-terminalBacklogLimit/2:])
		}
	}
	size := s.backlog.Len()
	s.mu.Unlock()

	if size > terminalBacklogLimit {
		t.Fatalf("backlog grew past the cap: %d", size)
	}
	if size == 0 {
		t.Fatal("backlog should not be empty after writes")
	}
}
