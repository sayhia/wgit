package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findStateChange returns the newest "app"-source activity entry for op.
func findStateChange(t *testing.T, op string) *GitCommandLog {
	t.Helper()
	logs := GetRecentActivityLogs()
	for i := len(logs) - 1; i >= 0; i-- {
		if logs[i].Source == "app" && logs[i].Command == op {
			return &logs[i]
		}
	}
	return nil
}

// stateChangeTestRepo creates a real repo with one committed image asset so
// SetAssetState accepts the path.
func stateChangeTestRepo(t *testing.T) string {
	t.Helper()
	repo := tempGitRepoForAI(t)
	if _, err := ExecGit(repo, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("config user.email: %v", err)
	}
	if _, err := ExecGit(repo, "config", "user.name", "Test"); err != nil {
		t.Fatalf("config user.name: %v", err)
	}
	asset := filepath.Join(repo, "assets")
	if err := os.MkdirAll(asset, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(asset, "tex.png"), []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	if _, err := ExecGit(repo, "add", "assets/tex.png"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := ExecGit(repo, "commit", "-m", "add asset"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	return repo
}

// TestStateChangeSetAssetStateLogsTransition pins the audit trail of asset
// state transitions: each accepted change lands in the activity feed with the
// path and the new state.
func TestStateChangeSetAssetStateLogsTransition(t *testing.T) {
	ClearRecentActivityLogs()
	repo := stateChangeTestRepo(t)
	svc := AssetService{}

	if err := svc.SetAssetState(repo, "assets/tex.png", "review"); err != nil {
		t.Fatalf("SetAssetState review: %v", err)
	}
	entry := findStateChange(t, "app.asset.state")
	if entry == nil {
		t.Fatalf("SetAssetState produced no app.asset.state activity entry")
	}
	if !strings.Contains(entry.Output, "assets/tex.png → review") {
		t.Errorf("entry.Output = %q, want the path and new state", entry.Output)
	}
	if !entry.Success {
		t.Errorf("accepted transition logged with Success=false")
	}

	if err := svc.SetAssetState(repo, "assets/tex.png", "approved"); err != nil {
		t.Fatalf("SetAssetState approved: %v", err)
	}
	entry = findStateChange(t, "app.asset.state")
	if entry == nil || !strings.Contains(entry.Output, "assets/tex.png → approved") {
		t.Errorf("second transition missing or wrong: %+v", entry)
	}
}

// TestStateChangeSaveAppFlagsLogsDiff pins that flag saves record the
// before→after value of every flag, so rollouts can be traced in the feed.
func TestStateChangeSaveAppFlagsLogsDiff(t *testing.T) {
	ClearRecentActivityLogs()
	// v2 显式 flags 作为受控基线；v1 文件会被 applyAICompatibility 在内存中
	// 默认点亮 AI 开关（有意设计），diff 的"变更前"就不可控了。
	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2,"theme":"dark","flags":{"assetBoard":true}}`)

	svc := GitService{}
	if err := svc.SaveAppFlags(FeatureFlags{AICommitMsg: true, AISummary: true}); err != nil {
		t.Fatalf("SaveAppFlags: %v", err)
	}
	entry := findStateChange(t, "app.flags")
	if entry == nil {
		t.Fatalf("SaveAppFlags produced no app.flags activity entry")
	}
	for _, want := range []string{"aiCommitMsg=false→true", "aiSummary=false→true", "assetBoard=true→false"} {
		if !strings.Contains(entry.Output, want) {
			t.Errorf("entry.Output = %q, missing %q", entry.Output, want)
		}
	}
}

// TestStateChangeSaveAISettingsLogsNoKeyMaterial pins that the AI settings
// audit entry describes the change without ever carrying the plaintext key.
func TestStateChangeSaveAISettingsLogsNoKeyMaterial(t *testing.T) {
	ClearRecentActivityLogs()
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)

	svc := AiService{}
	if err := svc.SaveAIConfig("https://api.example.com", "test-model", 30, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	entry := findStateChange(t, "app.ai.config")
	if entry == nil {
		t.Fatalf("SaveAIConfig produced no app.ai.config activity entry")
	}
	if !strings.Contains(entry.Output, "mode=api") || !strings.Contains(entry.Output, "test-model") {
		t.Errorf("entry.Output = %q, want mode and model context", entry.Output)
	}
	for _, log := range GetRecentActivityLogs() {
		combined := log.Command + log.Output + log.Error + log.Dir
		if strings.Contains(combined, aiTestKey) {
			t.Fatalf("plaintext key leaked into activity log entry %+v", log)
		}
	}
}
