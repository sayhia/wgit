package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// EngineProcessStatus captures live game engine editor process and lock status.
type EngineProcessStatus struct {
	IsRunning         bool   `json:"isRunning"`
	EngineName        string `json:"engineName"`
	ProcessPID        int    `json:"processPid"`
	LockFilePath      string `json:"lockFilePath,omitempty"`
	Warning           string `json:"warning"`
	CanSafelyCheckout bool   `json:"canSafelyCheckout"`
	Recommendation    string `json:"recommendation"`
}

// DetectRunningGameEngines checks for active game engines (Unity, Unreal Engine, Godot)
// locking repository files or holding dirty in-memory scene buffers.
func (GitService) DetectRunningGameEngines(repoPath string) (*EngineProcessStatus, error) {
	if repoPath == "" {
		repoPath = "."
	}

	status := &EngineProcessStatus{
		IsRunning:         false,
		EngineName:        "None",
		ProcessPID:        0,
		CanSafelyCheckout: true,
		Warning:           "",
		Recommendation:    "当前无引擎进程锁定工程，可安全切换分支或执行 Git 操作。",
	}

	// 1. Check file-based editor lock markers
	unityLock := filepath.Join(repoPath, "Temp", "UnityLockfile")
	if fi, err := os.Stat(unityLock); err == nil && !fi.IsDir() {
		status.IsRunning = true
		status.EngineName = "Unity"
		status.LockFilePath = unityLock
		status.CanSafelyCheckout = false
		status.Warning = "Unity 编辑器正在运行并独占锁定当前工程 (Temp/UnityLockfile)"
		status.Recommendation = "建议先在 Unity 中保存场景 (Ctrl+S / ⌘S)，并在安全切分支后切回 Unity 触发资源重载。"
		return status, nil
	}

	unrealLock := filepath.Join(repoPath, ".editorlock")
	if fi, err := os.Stat(unrealLock); err == nil && !fi.IsDir() {
		status.IsRunning = true
		status.EngineName = "Unreal Engine"
		status.LockFilePath = unrealLock
		status.CanSafelyCheckout = false
		status.Warning = "Unreal Engine 编辑器处于活动运行状态 (.editorlock)"
		status.Recommendation = "建议先在 Unreal Editor 中保存所有脏关卡/蓝图资产，避免外部覆盖引发断言崩溃。"
		return status, nil
	}

	godotLock := filepath.Join(repoPath, ".godot", "editor_lock")
	if fi, err := os.Stat(godotLock); err == nil && !fi.IsDir() {
		status.IsRunning = true
		status.EngineName = "Godot"
		status.LockFilePath = godotLock
		status.CanSafelyCheckout = false
		status.Warning = "Godot 编辑器正在运行 (.godot/editor_lock)"
		status.Recommendation = "建议先在 Godot 中保存当前场景，切分支后点击编辑器窗口重新加载。"
		return status, nil
	}

	// 2. Cross-platform process scan for known editor binaries
	if runtime.GOOS != "windows" {
		// macOS / Linux: scan with pgrep
		cmd := exec.Command("pgrep", "-l", "-i", "Unity|UnrealEditor|Godot")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					pid, _ := strconv.Atoi(parts[0])
					name := parts[1]
					var engine string
					if strings.Contains(strings.ToLower(name), "unity") {
						engine = "Unity"
					} else if strings.Contains(strings.ToLower(name), "unreal") {
						engine = "Unreal Engine"
					} else if strings.Contains(strings.ToLower(name), "godot") {
						engine = "Godot"
					}
					if engine != "" {
						status.IsRunning = true
						status.EngineName = engine
						status.ProcessPID = pid
						status.CanSafelyCheckout = true // Warning advisory
						status.Warning = fmt.Sprintf("检测到正在运行的 %s 进程 (PID: %d)", engine, pid)
						status.Recommendation = "切换包含大幅度资源变更的分支前，建议确保编辑器中无未保存场景。"
						return status, nil
					}
				}
			}
		}
	}

	return status, nil
}
