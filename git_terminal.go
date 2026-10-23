package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// =========================================================================
// 内置终端
//
// 用一个真实的 PTY 跑用户的登录 shell，输出通过 Wails 事件推给前端 xterm。
// 这是「工作台」和「查看器」的分界：不必再为了敲一条命令切到 Terminal.app、
// 复制路径、再切回来。
//
// 状态是进程级的单例（一个窗口一个终端），因此放在包级变量里而不是
// GitService 的字段上——GitService 刻意保持无状态。
// =========================================================================

const terminalBacklogLimit = 128 * 1024

// mainWindowRef is assigned once in main.go; the terminal needs it to push
// output, and GitService cannot hold it (value receiver, stateless by design).
var mainWindowRef *application.WebviewWindow

type TerminalStatus struct {
	Running bool   `json:"running"`
	Shell   string `json:"shell"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	Dir     string `json:"dir"`
	Backlog string `json:"backlog"`
	Message string `json:"message"`
}

type terminalSession struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	ptmx    *os.File
	backlog bytes.Buffer
	shell   string
	dir     string
	cols    int
	rows    int
	running bool
}

var (
	terminalMu  sync.Mutex
	terminalSes *terminalSession
)

func defaultShell() (string, []string) {
	if runtime.GOOS == "windows" {
		if os.Getenv("COMSPEC") != "" {
			return "powershell.exe", []string{"-NoLogo"}
		}
		return "cmd.exe", nil
	}
	if sh := strings.TrimSpace(os.Getenv("SHELL")); sh != "" {
		return sh, []string{"-l"}
	}
	return "/bin/sh", []string{"-l"}
}

func (s *terminalSession) status() *TerminalStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := "终端运行中"
	if !s.running {
		msg = "会话已结束，可重新启动"
	}
	return &TerminalStatus{
		Running: s.running,
		Shell:   s.shell,
		Cols:    s.cols,
		Rows:    s.rows,
		Dir:     s.dir,
		Backlog: s.backlog.String(),
		Message: msg,
	}
}

func (s *terminalSession) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// pump streams PTY output to the frontend and keeps a bounded backlog so a
// re-mounted terminal can restore context.
func (s *terminalSession) pump() {
	buf := make([]byte, 8192)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])

			s.mu.Lock()
			s.backlog.WriteString(chunk)
			if s.backlog.Len() > terminalBacklogLimit {
				trimmed := s.backlog.String()
				s.backlog.Reset()
				// Keep the newest half; a partial UTF-8 rune at the cut is
				// harmless once JSON-encoded.
				s.backlog.WriteString(trimmed[len(trimmed)-terminalBacklogLimit/2:])
			}
			s.mu.Unlock()

			emitTerminalOutput(chunk)
		}
		if err != nil {
			break
		}
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	emitTerminalOutput("\r\n\x1b[2m[会话已结束]\x1b[0m\r\n")
}

func emitTerminalOutput(chunk string) {
	window := mainWindowRef
	if window == nil {
		return
	}
	// A closed window must not take the whole app down.
	defer func() { _ = recover() }()
	window.EmitEvent("terminal:output", chunk)
}

// StartTerminal launches the user's login shell in repoPath (or $HOME).
// Calling it while a session is alive returns the existing status instead of
// spawning a second shell.
func (GitService) StartTerminal(repoPath string, cols int, rows int) (*TerminalStatus, error) {
	terminalMu.Lock()
	defer terminalMu.Unlock()

	if terminalSes != nil && terminalSes.isRunning() {
		return terminalSes.status(), nil
	}
	if cols <= 0 {
		cols = 100
	}
	if rows <= 0 {
		rows = 30
	}

	dir := strings.TrimSpace(repoPath)
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir, _ = os.UserHomeDir()
	}

	shell, args := defaultShell()
	cmd := exec.Command(shell, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, fmt.Errorf("无法启动终端（%s）: %w", shell, err)
	}

	s := &terminalSession{
		cmd:     cmd,
		ptmx:    ptmx,
		shell:   shell,
		dir:     dir,
		cols:    cols,
		rows:    rows,
		running: true,
	}
	s.backlog.WriteString(fmt.Sprintf("\x1b[2m[wgit] %s · %s · %dx%d\x1b[0m\r\n", shell, dir, cols, rows))
	terminalSes = s
	go s.pump()
	return s.status(), nil
}

// WriteTerminal forwards keystrokes (including control sequences) to the PTY.
func (GitService) WriteTerminal(data string) error {
	if data == "" {
		return nil
	}
	terminalMu.Lock()
	s := terminalSes
	terminalMu.Unlock()

	if s == nil || !s.isRunning() {
		return errors.New("终端未启动")
	}
	_, err := s.ptmx.Write([]byte(data))
	return err
}

// ResizeTerminal keeps the PTY window size in sync with the rendered terminal,
// so curses apps (vim / htop / git log pager) lay out correctly.
func (GitService) ResizeTerminal(cols int, rows int) error {
	if cols <= 0 || rows <= 0 {
		return errors.New("终端尺寸无效")
	}
	terminalMu.Lock()
	s := terminalSes
	terminalMu.Unlock()

	if s == nil || !s.isRunning() {
		return errors.New("终端未启动")
	}
	s.mu.Lock()
	s.cols = cols
	s.rows = rows
	s.mu.Unlock()

	return pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (GitService) StopTerminal() error {
	terminalMu.Lock()
	s := terminalSes
	terminalSes = nil
	terminalMu.Unlock()

	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	_ = s.ptmx.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return nil
}

func (GitService) GetTerminalStatus() (*TerminalStatus, error) {
	terminalMu.Lock()
	s := terminalSes
	terminalMu.Unlock()

	if s == nil {
		return &TerminalStatus{
			Running: false,
			Message: "终端未启动",
		}, nil
	}
	return s.status(), nil
}
