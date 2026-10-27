package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type LocalCiStepResult struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	ExitCode   int    `json:"exitCode"`
	Output     string `json:"output"`
	DurationMs int64  `json:"durationMs"`
	Success    bool   `json:"success"`
}

type LocalCiResult struct {
	Success         bool                `json:"success"`
	SandboxDir      string              `json:"sandboxDir"`
	Steps           []LocalCiStepResult `json:"steps"`
	TotalDurationMs int64               `json:"totalDurationMs"`
	Summary         string              `json:"summary"`
}

type WebhookDispatchResult struct {
	Success         bool   `json:"success"`
	StatusCode      int    `json:"statusCode"`
	Url             string `json:"url"`
	EventType       string `json:"eventType"`
	ResponseBody    string `json:"responseBody"`
	DurationMs      int64  `json:"durationMs"`
	SignatureHeader string `json:"signatureHeader"`
}

func (GitService) RunLocalCiSandbox(repoPath string, commands []string, timeoutSec int) (*LocalCiResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	if timeoutSec <= 0 {
		timeoutSec = 60
	}

	tempDir, err := os.MkdirTemp("", "wgit-ci-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	defer func() {
		_, _ = ExecGit(repoPath, "worktree", "remove", "--force", tempDir)
		_ = os.RemoveAll(tempDir)
	}()

	// Add detached worktree at HEAD so the active workspace remains undisturbed
	if _, err := ExecGit(repoPath, "worktree", "add", "--detach", tempDir, "HEAD"); err != nil {
		return nil, fmt.Errorf("failed to create isolated sandbox worktree: %w", err)
	}

	if len(commands) == 0 {
		commands = []string{"git status --short"}
	}

	overallStart := time.Now()
	var stepResults []LocalCiStepResult
	allPassed := true

	for idx, cmdStr := range commands {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}

		stepName := fmt.Sprintf("Step %d: %s", idx+1, cmdStr)
		stepStart := time.Now()

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr)
		cmd.Dir = tempDir

		var combinedBuf bytes.Buffer
		cmd.Stdout = &combinedBuf
		cmd.Stderr = &combinedBuf

		runErr := cmd.Run()
		cancel()

		stepDuration := time.Since(stepStart).Milliseconds()
		exitCode := 0
		stepSuccess := true

		if runErr != nil {
			stepSuccess = false
			allPassed = false
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		outStr := combinedBuf.String()
		if len(outStr) > 25000 {
			outStr = outStr[:25000] + "\n... (truncated)"
		}

		stepResults = append(stepResults, LocalCiStepResult{
			Name:       stepName,
			Command:    cmdStr,
			ExitCode:   exitCode,
			Output:     outStr,
			DurationMs: stepDuration,
			Success:    stepSuccess,
		})

		if !stepSuccess {
			break
		}
	}

	totalDuration := time.Since(overallStart).Milliseconds()
	summary := fmt.Sprintf("All %d steps passed in %dms", len(stepResults), totalDuration)
	if !allPassed {
		summary = fmt.Sprintf("CI failed at step %d in %dms", len(stepResults), totalDuration)
	}

	return &LocalCiResult{
		Success:         allPassed,
		SandboxDir:      tempDir,
		Steps:           stepResults,
		TotalDurationMs: totalDuration,
		Summary:         summary,
	}, nil
}

func (GitService) DispatchTeamWebhook(webhookUrl string, eventType string, payload string, secret string) (*WebhookDispatchResult, error) {
	webhookUrl = strings.TrimSpace(webhookUrl)
	if !strings.HasPrefix(webhookUrl, "http://") && !strings.HasPrefix(webhookUrl, "https://") {
		return nil, errors.New("webhookUrl must begin with http:// or https://")
	}

	if eventType == "" {
		eventType = "git.push"
	}
	if payload == "" {
		payload = fmt.Sprintf(`{"event":"%s","timestamp":%d,"source":"wgit"}`, eventType, time.Now().Unix())
	}

	start := time.Now()
	sigHeader := ""
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		sigHeader = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	req, err := http.NewRequest("POST", webhookUrl, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WGit-Webhook-Hub/1.0")
	req.Header.Set("X-WGit-Event", eventType)
	if sigHeader != "" {
		req.Header.Set("X-Hub-Signature-256", sigHeader)
		req.Header.Set("X-Signature-SHA256", sigHeader)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return &WebhookDispatchResult{
			Success:         false,
			StatusCode:      0,
			Url:             webhookUrl,
			EventType:       eventType,
			ResponseBody:    err.Error(),
			DurationMs:      duration,
			SignatureHeader: sigHeader,
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return &WebhookDispatchResult{
		Success:         success,
		StatusCode:      resp.StatusCode,
		Url:             webhookUrl,
		EventType:       eventType,
		ResponseBody:    string(bodyBytes),
		DurationMs:      duration,
		SignatureHeader: sigHeader,
	}, nil
}
