package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExternalDiffToolResult represents the launch outcome of an external diff or DCC tool.
type ExternalDiffToolResult struct {
	ToolID    string `json:"toolId"`
	ToolName  string `json:"toolName"`
	Launched  bool   `json:"launched"`
	Message   string `json:"message"`
	LeftPath  string `json:"leftPath"`
	RightPath string `json:"rightPath"`
}

// LaunchExternalDiffTool launches a configured external tool (Beyond Compare, P4Merge,
// Cursor, VS Code, Blender, etc.) to visually inspect changes between two revisions or assets.
func (s GitService) LaunchExternalDiffTool(repoPath string, toolID string, oldRef string, newRef string, filePath string) (*ExternalDiffToolResult, error) {
	if repoPath == "" || filePath == "" {
		return nil, errors.New("repoPath and filePath are required")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}
	filePath = cleanPath

	cfg, _ := s.GetExternalToolConfig(repoPath)
	targetToolID := strings.ToLower(strings.TrimSpace(toolID))
	if targetToolID == "" && cfg != nil && cfg.DiffTool != "" {
		targetToolID = strings.ToLower(cfg.DiffTool)
	}
	if targetToolID == "" {
		targetToolID = "beyondcompare"
	}

	candidates := getToolCandidates()
	var selectedCandidate *toolCandidate
	for i := range candidates {
		if strings.EqualFold(candidates[i].ID, targetToolID) {
			selectedCandidate = &candidates[i]
			break
		}
	}

	var exe string
	var hasExe bool
	if selectedCandidate != nil {
		exe, hasExe = resolveExecutable(selectedCandidate.Candidates)
	}

	// Fallback to any installed tool if specified one is not found
	if !hasExe {
		tools, _ := s.DetectExternalTools()
		for _, t := range tools {
			if t.IsAvailable && t.SupportsDiff {
				for i := range candidates {
					if strings.EqualFold(candidates[i].ID, t.ID) {
						selectedCandidate = &candidates[i]
						exe = t.Executable
						hasExe = true
						break
					}
				}
				if hasExe {
					break
				}
			}
		}
	}

	toolName := "外部对比工具"
	if selectedCandidate != nil {
		toolName = selectedCandidate.Name
	}

	// Prepare left (old / base) and right (new / target) files
	tmpDir := os.TempDir()
	baseName := filepath.Base(filePath)
	ext := filepath.Ext(filePath)
	plainName := strings.TrimSuffix(baseName, ext)

	safeOldRef := oldRef
	if safeOldRef == "" {
		safeOldRef = "HEAD~1"
	}
	safeOldLabel := strings.ReplaceAll(safeOldRef, "/", "_")
	leftFile := filepath.Join(tmpDir, fmt.Sprintf("wgit_%s_%s%s", safeOldLabel, plainName, ext))

	oldContent, err := ExecGitRaw(repoPath, "show", fmt.Sprintf("%s:%s", safeOldRef, filePath))
	if err != nil {
		// If revision does not exist (e.g. newly created file), write empty placeholder
		oldContent = ""
	}
	if err := os.WriteFile(leftFile, []byte(oldContent), 0o644); err != nil {
		return nil, fmt.Errorf("创建旧版本临时对比文件失败: %w", err)
	}

	var rightFile string
	if newRef == "" || strings.EqualFold(newRef, "WORKTREE") || strings.EqualFold(newRef, "HEAD") {
		rightFile = filepath.Join(repoPath, filePath)
		if _, err := os.Stat(rightFile); os.IsNotExist(err) {
			// Extract from HEAD if not on disk
			newContent, _ := ExecGitRaw(repoPath, "show", fmt.Sprintf("HEAD:%s", filePath))
			rightFile = filepath.Join(tmpDir, fmt.Sprintf("wgit_head_%s%s", plainName, ext))
			_ = os.WriteFile(rightFile, []byte(newContent), 0o644)
		}
	} else {
		safeNewLabel := strings.ReplaceAll(newRef, "/", "_")
		rightFile = filepath.Join(tmpDir, fmt.Sprintf("wgit_%s_%s%s", safeNewLabel, plainName, ext))
		newContent, err := ExecGitRaw(repoPath, "show", fmt.Sprintf("%s:%s", newRef, filePath))
		if err != nil {
			newContent = ""
		}
		if err := os.WriteFile(rightFile, []byte(newContent), 0o644); err != nil {
			return nil, fmt.Errorf("创建新版本临时对比文件失败: %w", err)
		}
	}

	if !hasExe {
		// System default opener fallback
		var openCmd *exec.Cmd
		if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
			openCmd = exec.Command("cmd", "/c", "start", "", leftFile)
		} else {
			openCmd = exec.Command("open", leftFile)
		}
		_ = startDetached(openCmd)
		return &ExternalDiffToolResult{
			ToolID:    targetToolID,
			ToolName:  "系统默认程序",
			Launched:  true,
			Message:   fmt.Sprintf("已通过系统默认关联工具打开资源快照 (%s)", baseName),
			LeftPath:  leftFile,
			RightPath: rightFile,
		}, nil
	}

	var cmd *exec.Cmd
	if selectedCandidate != nil && selectedCandidate.DiffArgs != nil {
		args := selectedCandidate.DiffArgs(exe, leftFile, rightFile)
		if len(args) > 1 {
			cmd = exec.Command(args[0], args[1:]...)
		} else {
			cmd = exec.Command(exe, leftFile, rightFile)
		}
	} else {
		cmd = exec.Command(exe, leftFile, rightFile)
	}

	if err := startDetached(cmd); err != nil {
		return nil, fmt.Errorf("启动外部对比工具 (%s) 失败: %w", toolName, err)
	}

	return &ExternalDiffToolResult{
		ToolID:    targetToolID,
		ToolName:  toolName,
		Launched:  true,
		Message:   fmt.Sprintf("已成功在 %s 中打开并对比 %s", toolName, baseName),
		LeftPath:  leftFile,
		RightPath: rightFile,
	}, nil
}
