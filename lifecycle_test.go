package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// These tests pin the background-task lifecycle contract exercised by
// shutdownBackgroundTasks (wired to Wails' Options.OnShutdown in main.go):
// every long-lived task — LAN review server, repo watcher, terminal PTY —
// must have a working start→stop path with observable resource release.

func freeTCPPort(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 5; attempt++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			continue
		}
		port := l.Addr().(*net.TCPAddr).Port
		_ = l.Close()
		return port
	}
	t.Fatal("could not find a free TCP port")
	return 0
}

func tcpConnectable(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func TestLifecycleLanServerShutdownReleasesPort(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	var info *LanServerInfo
	var err error
	for attempt := 0; attempt < 3 && info == nil; attempt++ {
		port := freeTCPPort(t)
		info, err = svc.StartLanReviewServer(repo, port)
		if err != nil {
			info = nil // port lost to a race; try another
		}
	}
	if info == nil {
		t.Fatalf("StartLanReviewServer never bound a port: %v", err)
	}

	lanMu.Lock()
	server := lanHttpServer
	lanMu.Unlock()
	if server == nil {
		t.Fatal("expected lanHttpServer to be registered after start")
	}

	deadline := time.Now().Add(3 * time.Second)
	for !tcpConnectable(info.Port) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !tcpConnectable(info.Port) {
		t.Fatalf("LAN server not accepting connections on port %d after start", info.Port)
	}

	shutdownBackgroundTasks()

	lanMu.Lock()
	running := lanInfo.Running
	gone := lanHttpServer == nil
	lanMu.Unlock()
	if running || !gone {
		t.Fatalf("LAN server state not cleared after shutdown: running=%v server=%v", running, gone)
	}

	// Port must be released: connections get refused once the listener closed.
	released := true
	for i := 0; i < 6; i++ {
		if tcpConnectable(info.Port) {
			released = false
			time.Sleep(100 * time.Millisecond)
		} else {
			released = true
			break
		}
	}
	if !released {
		t.Fatalf("port %d still accepting connections after shutdown", info.Port)
	}
}

func TestLifecycleRepoWatcherShutdownClosesStopChannel(t *testing.T) {
	repoA := t.TempDir()
	repoB := t.TempDir()
	for _, dir := range []string{repoA, repoB} {
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatalf("seed .git failed: %v", err)
		}
	}

	svc := GitService{}
	if _, err := svc.StartRepoWatcher(repoA); err != nil {
		t.Fatalf("StartRepoWatcher(A) failed: %v", err)
	}

	repoWatcherMu.Lock()
	chA := activeWatcherStop
	watchingA := activeWatchPath
	repoWatcherMu.Unlock()
	if chA == nil || watchingA != filepath.Clean(repoA) {
		t.Fatalf("watcher A not registered: path=%q stop=%v", watchingA, chA != nil)
	}

	// Switching repositories must stop the previous watcher, not stack a
	// second one (fsnotify handles and the event goroutine are released).
	if _, err := svc.StartRepoWatcher(repoB); err != nil {
		t.Fatalf("StartRepoWatcher(B) failed: %v", err)
	}
	select {
	case <-chA:
	default:
		t.Fatal("previous watcher stop channel still open after repo switch")
	}

	repoWatcherMu.Lock()
	chB := activeWatcherStop
	repoWatcherMu.Unlock()
	if chB == nil {
		t.Fatal("watcher B stop channel missing after switch")
	}

	shutdownBackgroundTasks()

	repoWatcherMu.Lock()
	stillActive := activeRepoWatcher != nil
	stillStop := activeWatcherStop != nil
	repoWatcherMu.Unlock()
	if stillActive || stillStop {
		t.Fatalf("watcher not released by shutdown: active=%v stop=%v", stillActive, stillStop)
	}
	select {
	case <-chB:
	default:
		t.Fatal("watcher stop channel not closed by shutdown")
	}

	// StopRepoWatcher on a clean state reports nothing to stop (idempotent).
	if svc.StopRepoWatcher() {
		t.Fatal("StopRepoWatcher should report false after shutdown cleared the watcher")
	}
}

func TestLifecycleTerminalShutdownReapsShell(t *testing.T) {
	svc := GitService{}
	_ = svc.StopTerminal()

	dir := t.TempDir()
	if _, err := svc.StartTerminal(dir, 80, 24); err != nil {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}

	terminalMu.Lock()
	ses := terminalSes
	terminalMu.Unlock()
	if ses == nil {
		t.Fatal("terminal session not registered after start")
	}
	ses.mu.Lock()
	pid := ses.cmd.Process.Pid
	ses.mu.Unlock()

	shutdownBackgroundTasks()

	terminalMu.Lock()
	cleared := terminalSes == nil
	terminalMu.Unlock()
	if !cleared {
		t.Fatal("terminal session registry not cleared by shutdown")
	}
	status, err := svc.GetTerminalStatus()
	if err != nil {
		t.Fatalf("GetTerminalStatus failed: %v", err)
	}
	if status.Running {
		t.Fatal("terminal still reports running after shutdown")
	}

	// The killed shell must be reaped: a zombie still answers signal 0.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if processGone(pid) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("terminal shell pid %d still in the process table after shutdown", pid)
}

func TestLifecycleStartDetachedReapsChild(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on PATH; reaping test needs a short-lived child")
	}
	cmd := exec.Command(shell, "-c", "exit 0")
	if err := startDetached(cmd); err != nil {
		t.Fatalf("startDetached failed: %v", err)
	}
	pid := cmd.Process.Pid

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if processGone(pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("detached child pid %d not reaped; it lingers as a zombie", pid)
}

func TestLifecycleStartDetachedReportsLaunchFailure(t *testing.T) {
	if err := startDetached(exec.Command("/definitely/not/a/wgit/binary")); err == nil {
		t.Fatal("expected an error when the executable does not exist")
	}
}

func TestLifecycleShutdownBackgroundTasksIdempotent(t *testing.T) {
	// Nothing is running; double shutdown must be a no-op, not a double-close
	// of the watcher stop channel or a nil-pointer walk.
	shutdownBackgroundTasks()
	shutdownBackgroundTasks()

	lanMu.Lock()
	running := lanInfo.Running
	lanMu.Unlock()
	if running {
		t.Fatal("LAN server reports running after idempotent shutdown")
	}
	repoWatcherMu.Lock()
	active := activeRepoWatcher != nil
	repoWatcherMu.Unlock()
	if active {
		t.Fatal("repo watcher still active after idempotent shutdown")
	}
}
