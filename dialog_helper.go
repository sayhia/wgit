package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ChooseFolderDialog opens the native operating system folder selection dialog.
// It prioritizes Wails v3 native dialogs, and seamlessly falls back to OS-level tools
// (AppleScript on macOS, PowerShell on Windows, Zenity/KDialog on Linux) so that
// folder selection never hangs, crashes, or fails silently under any execution mode.
func ChooseFolderDialog(title string) (string, error) {
	if title == "" {
		title = "选择目录"
	}

	// 1. Primary Layer: Wails v3 Native Application Dialog
	if app := application.Get(); app != nil && app.Dialog != nil {
		dialog := app.Dialog.OpenFile()
		dialog.CanChooseDirectories(true)
		dialog.CanChooseFiles(false)
		dialog.CanCreateDirectories(true)
		dialog.SetTitle(title)
		dialog.SetMessage(title)

		if app.Window != nil {
			if win := app.Window.Current(); win != nil {
				dialog.AttachToWindow(win)
			}
		}

		path, err := dialog.PromptForSingleSelection()
		if err == nil {
			trimmed := strings.TrimSpace(path)
			if trimmed == "" {
				return "", nil // User cancelled
			}
			return filepath.Clean(trimmed), nil
		}
		log.Printf("[ChooseFolderDialog] Wails Dialog.OpenFile error: %v, attempting OS fallback", err)
	}

	// 2. Secondary Layer: Native OS Subprocess Fallback
	return chooseFolderFallback(title)
}

func chooseFolderFallback(title string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		// AppleScript choose folder dialog with cancel handling (-128)
		script := fmt.Sprintf(`try
  POSIX path of (choose folder with prompt %q)
on error number -128
  return ""
end try`, title)
		cmd := exec.Command("osascript", "-e", script)
		out, err := cmd.Output()
		if err != nil {
			outStr := string(out) + " " + err.Error()
			if strings.Contains(outStr, "-128") || strings.Contains(outStr, "User canceled") || strings.Contains(outStr, "cancelled") {
				return "", nil
			}
			return "", fmt.Errorf("macOS 目录选择对话框启动失败: %w", err)
		}
		selected := strings.TrimSpace(string(out))
		if selected == "" {
			return "", nil
		}
		return filepath.Clean(selected), nil

	case "windows":
		// PowerShell FolderBrowserDialog
		escapedTitle := strings.ReplaceAll(title, "'", "''")
		psCmd := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; $f = New-Object System.Windows.Forms.FolderBrowserDialog; $f.Description = '%s'; $f.ShowNewFolderButton = $true; if($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK){ Write-Output $f.SelectedPath }`, escapedTitle)
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("Windows 目录选择对话框启动失败: %w", err)
		}
		selected := strings.TrimSpace(string(out))
		if selected == "" {
			return "", nil
		}
		return filepath.Clean(selected), nil

	case "linux":
		// Zenity
		if _, err := exec.LookPath("zenity"); err == nil {
			cmd := exec.Command("zenity", "--file-selection", "--directory", "--title="+title)
			out, err := cmd.Output()
			if err != nil {
				return "", nil // Zenity returns 1 on Cancel
			}
			selected := strings.TrimSpace(string(out))
			if selected == "" {
				return "", nil
			}
			return filepath.Clean(selected), nil
		}
		// KDialog (KDE)
		if _, err := exec.LookPath("kdialog"); err == nil {
			cmd := exec.Command("kdialog", "--getexistingdirectory", "--title", title)
			out, err := cmd.Output()
			if err != nil {
				return "", nil // KDialog returns 1 on Cancel
			}
			selected := strings.TrimSpace(string(out))
			if selected == "" {
				return "", nil
			}
			return filepath.Clean(selected), nil
		}
		return "", fmt.Errorf("Linux 环境未检测到图形目录选择器 (zenity 或 kdialog)")

	default:
		return "", fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}
