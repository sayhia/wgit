package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeFeatureFlagsConfigFile points HOME at a throwaway directory and plants
// the given config.json under ~/.config/wgit so LoadConfig/SaveConfig operate
// on it instead of the developer's real settings.
func writeFeatureFlagsConfigFile(t *testing.T, content string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "wgit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}
}

func TestLoadConfigFeatureFlags(t *testing.T) {
	tests := []struct {
		name            string
		configJSON      string
		wantFlags       FeatureFlags
		wantSchemaVer   int
		wantPreservedFn func(t *testing.T, cfg AppConfig)
	}{
		{
			name:          "legacy config migrates AI flags on",
			configJSON:    `{"recentRepos":["/tmp/repo-a"],"lastRepo":"/tmp/repo-a","theme":"light"}`,
			wantFlags:     FeatureFlags{AICommitMsg: true, AISummary: true, AIConflictBatch: true},
			wantSchemaVer: 2,
			wantPreservedFn: func(t *testing.T, cfg AppConfig) {
				if cfg.Theme != "light" {
					t.Errorf("Theme = %q, want %q (legacy fields must survive)", cfg.Theme, "light")
				}
				if len(cfg.RecentRepos) != 1 || cfg.RecentRepos[0] != "/tmp/repo-a" {
					t.Errorf("RecentRepos = %v, want [/tmp/repo-a]", cfg.RecentRepos)
				}
			},
		},
		{
			name:          "v1 flags migrate AI on but keep non-AI choices",
			configJSON:    `{"theme":"dark","flags":{"aiCommitMsg":true,"aiSummary":false,"aiConflictBatch":true,"reviewFlow":true,"assetBoard":false},"schemaVersion":1}`,
			wantFlags:     FeatureFlags{AICommitMsg: true, AISummary: true, AIConflictBatch: true, ReviewFlow: true},
			wantSchemaVer: 2,
		},
		{
			name:          "v2 flags are preserved verbatim",
			configJSON:    `{"theme":"dark","flags":{"aiCommitMsg":false,"aiSummary":true,"aiConflictBatch":false},"schemaVersion":2}`,
			wantFlags:     FeatureFlags{AISummary: true},
			wantSchemaVer: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeFeatureFlagsConfigFile(t, tt.configJSON)

			cfg := LoadConfig()

			// FeatureFlags is comparable (all bool fields).
			if cfg.Flags != tt.wantFlags {
				t.Errorf("Flags = %+v, want %+v", cfg.Flags, tt.wantFlags)
			}
			if cfg.SchemaVersion != tt.wantSchemaVer {
				t.Errorf("SchemaVersion = %d, want %d", cfg.SchemaVersion, tt.wantSchemaVer)
			}
			if tt.wantPreservedFn != nil {
				tt.wantPreservedFn(t, cfg)
			}
		})
	}
}

func TestLoadConfigMissingFileFlagsDefaultAIOn(t *testing.T) {
	// HOME points at an empty temp dir: no config file exists at all.
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := LoadConfig()

	want := FeatureFlags{AICommitMsg: true, AISummary: true, AIConflictBatch: true}
	if cfg.Flags != want {
		t.Errorf("Flags = %+v, want AI actions on and non-AI features off (%+v)", cfg.Flags, want)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", cfg.SchemaVersion, CurrentSchemaVersion)
	}
}

func TestSaveConfigRoundTripsFeatureFlags(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)

	cfg := LoadConfig()
	cfg.Flags = FeatureFlags{AISummary: true, AssetBoard: true}
	SaveConfig(cfg)

	// The on-disk JSON uses the lowerCamelCase keys the frontend depends on.
	raw, err := os.ReadFile(getConfigPath())
	if err != nil {
		t.Fatalf("re-read config.json: %v", err)
	}
	var onDisk struct {
		Flags         FeatureFlags `json:"flags"`
		SchemaVersion int          `json:"schemaVersion"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal saved config: %v", err)
	}
	if onDisk.Flags != cfg.Flags {
		t.Errorf("on-disk flags = %+v, want %+v", onDisk.Flags, cfg.Flags)
	}
	if onDisk.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("on-disk schemaVersion = %d, want %d", onDisk.SchemaVersion, CurrentSchemaVersion)
	}

	reloaded := LoadConfig()
	if reloaded.Flags != cfg.Flags {
		t.Errorf("reloaded flags = %+v, want %+v", reloaded.Flags, cfg.Flags)
	}
}

func TestGetAppFlagsReturnsLoadedConfigFlags(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"reviewFlow":true},"schemaVersion":1}`)

	got := (GitService{}).GetAppFlags()

	if got != (FeatureFlags{AICommitMsg: true, AISummary: true, AIConflictBatch: true, ReviewFlow: true}) {
		t.Errorf("GetAppFlags() = %+v, want AI v1 migration + ReviewFlow", got)
	}
}

func TestSaveAppFlags(t *testing.T) {
	tests := []struct {
		name       string
		configJSON string
		saveFlags  FeatureFlags
		wantFlags  FeatureFlags
		// verify runs extra assertions on the config re-read from disk after
		// the save (mostly: everything except Flags survived verbatim).
		verify func(t *testing.T, cfg AppConfig)
	}{
		{
			name:       "saved flags round-trip through LoadConfig",
			configJSON: `{"theme":"dark","schemaVersion":1}`,
			saveFlags:  FeatureFlags{AICommitMsg: true, AssetBoard: true},
			wantFlags:  FeatureFlags{AICommitMsg: true, AssetBoard: true},
		},
		{
			name:       "saving over pre-existing flags replaces the whole set",
			configJSON: `{"flags":{"aiCommitMsg":true,"aiSummary":true},"schemaVersion":1}`,
			saveFlags:  FeatureFlags{AIConflictBatch: true},
			// The new set is authoritative: flags the caller left false must
			// actually turn dark on disk, not silently stay enabled.
			wantFlags: FeatureFlags{AIConflictBatch: true},
		},
		{
			name:       "saving flags leaves AI, RecentRepos, LastRepo and theme untouched",
			configJSON: `{"recentRepos":["/tmp/repo-a","/tmp/repo-b"],"lastRepo":"/tmp/repo-b","theme":"light","ai":{"baseUrl":"http://localhost:11434/v1","apiKey":"deadbeef","model":"llama3","timeoutSeconds":30},"schemaVersion":1}`,
			saveFlags:  FeatureFlags{ReviewFlow: true},
			wantFlags:  FeatureFlags{ReviewFlow: true},
			verify: func(t *testing.T, cfg AppConfig) {
				t.Helper()
				if cfg.Theme != "light" {
					t.Errorf("Theme = %q, want %q (flag save must not touch the theme)", cfg.Theme, "light")
				}
				if len(cfg.RecentRepos) != 2 || cfg.RecentRepos[0] != "/tmp/repo-a" || cfg.RecentRepos[1] != "/tmp/repo-b" {
					t.Errorf("RecentRepos = %v, want [/tmp/repo-a /tmp/repo-b]", cfg.RecentRepos)
				}
				if cfg.LastRepo != "/tmp/repo-b" {
					t.Errorf("LastRepo = %q, want %q", cfg.LastRepo, "/tmp/repo-b")
				}
				wantAI := AIConfig{BaseURL: "http://localhost:11434/v1", APIKey: "deadbeef", Model: "llama3", TimeoutSeconds: 30}
				if !reflect.DeepEqual(cfg.AI, wantAI) {
					t.Errorf("AI = %+v, want %+v (flag save must not touch AI config)", cfg.AI, wantAI)
				}
			},
		},
		{
			name:       "saving flags over a legacy config backfills schemaVersion",
			configJSON: `{"theme":"dark"}`,
			saveFlags:  FeatureFlags{AISummary: true},
			wantFlags:  FeatureFlags{AISummary: true},
			verify: func(t *testing.T, cfg AppConfig) {
				t.Helper()
				if cfg.SchemaVersion != CurrentSchemaVersion {
					t.Errorf("SchemaVersion = %d, want %d", cfg.SchemaVersion, CurrentSchemaVersion)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeFeatureFlagsConfigFile(t, tt.configJSON)

			if err := (GitService{}).SaveAppFlags(tt.saveFlags); err != nil {
				t.Fatalf("SaveAppFlags(%+v) error = %v", tt.saveFlags, err)
			}

			reloaded := LoadConfig()
			if reloaded.Flags != tt.wantFlags {
				t.Errorf("flags after save = %+v, want %+v", reloaded.Flags, tt.wantFlags)
			}
			if tt.verify != nil {
				tt.verify(t, reloaded)
			}

			// The RPC pair must agree: a GetAppFlags read after the save
			// reports exactly what was persisted.
			if got := (GitService{}).GetAppFlags(); got != tt.wantFlags {
				t.Errorf("GetAppFlags() after save = %+v, want %+v", got, tt.wantFlags)
			}
		})
	}
}

// v2FlagConfig renders a v2 config.json whose five flags each equal the given
// bool, so a table case can plant an all-dark or all-lit baseline and let only
// the environment variable under test move one flag.
func v2FlagConfig(on bool) string {
	return fmt.Sprintf(`{"flags":{"aiCommitMsg":%t,"aiSummary":%t,"aiConflictBatch":%t,"reviewFlow":%t,"assetBoard":%t},"schemaVersion":2}`, on, on, on, on, on)
}

func TestLoadConfigEnvFlagOverrides(t *testing.T) {
	allOn := FeatureFlags{AICommitMsg: true, AISummary: true, AIConflictBatch: true, ReviewFlow: true, AssetBoard: true}
	allOff := FeatureFlags{}

	tests := []struct {
		name       string
		configJSON string
		env        map[string]string
		wantFlags  FeatureFlags
	}{
		{
			name:       `"1" turns a dark v2 flag on`,
			configJSON: v2FlagConfig(false),
			env:        map[string]string{"WGIT_FLAG_ASSETBOARD": "1"},
			wantFlags:  FeatureFlags{AssetBoard: true},
		},
		{
			name:       `"TRUE" parses case-insensitively`,
			configJSON: v2FlagConfig(false),
			env:        map[string]string{"WGIT_FLAG_AISUMMARY": "TRUE"},
			wantFlags:  FeatureFlags{AISummary: true},
		},
		{
			name:       `"On" and "yEs" parse case-insensitively too`,
			configJSON: v2FlagConfig(false),
			env:        map[string]string{"WGIT_FLAG_AICONFLICTBATCH": "On", "WGIT_FLAG_REVIEWFLOW": "yEs"},
			wantFlags:  FeatureFlags{AIConflictBatch: true, ReviewFlow: true},
		},
		{
			name:       `"0" turns a lit v2 flag off`,
			configJSON: v2FlagConfig(true),
			env:        map[string]string{"WGIT_FLAG_AICOMMITMSG": "0"},
			wantFlags:  FeatureFlags{AISummary: true, AIConflictBatch: true, ReviewFlow: true, AssetBoard: true},
		},
		{
			name:       `"OFF" parses case-insensitively`,
			configJSON: v2FlagConfig(true),
			env:        map[string]string{"WGIT_FLAG_AISUMMARY": "OFF"},
			wantFlags:  FeatureFlags{AICommitMsg: true, AIConflictBatch: true, ReviewFlow: true, AssetBoard: true},
		},
		{
			name:       "empty value is ignored, not treated as off",
			configJSON: v2FlagConfig(true),
			env:        map[string]string{"WGIT_FLAG_REVIEWFLOW": ""},
			wantFlags:  allOn,
		},
		{
			name:       "invalid value is ignored without polluting other flags",
			configJSON: `{"flags":{"aiSummary":true,"reviewFlow":true},"schemaVersion":2}`,
			env:        map[string]string{"WGIT_FLAG_REVIEWFLOW": "maybe"},
			wantFlags:  FeatureFlags{AISummary: true, ReviewFlow: true},
		},
		{
			name:       "env beats the applyAICompatibility upgrade",
			configJSON: `{"theme":"dark"}`, // legacy: the migration turns all AI flags on
			env:        map[string]string{"WGIT_FLAG_AISUMMARY": "false"},
			wantFlags:  FeatureFlags{AICommitMsg: true, AIConflictBatch: true},
		},
		{
			name:       "several overrides apply in one load",
			configJSON: v2FlagConfig(false),
			env:        map[string]string{"WGIT_FLAG_AICOMMITMSG": "yes", "WGIT_FLAG_ASSETBOARD": "true"},
			wantFlags:  FeatureFlags{AICommitMsg: true, AssetBoard: true},
		},
		{
			name:       "unknown WGIT_FLAG_ variables are inert",
			configJSON: v2FlagConfig(false),
			env:        map[string]string{"WGIT_FLAG_BOGUS": "1"},
			wantFlags:  allOff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeFeatureFlagsConfigFile(t, tt.configJSON)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg := LoadConfig()

			if cfg.Flags != tt.wantFlags {
				t.Errorf("Flags = %+v, want %+v", cfg.Flags, tt.wantFlags)
			}
		})
	}
}

func TestEnvFlagOverrideWarnsOnInvalidValue(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":2}`)
	t.Setenv("WGIT_FLAG_AICOMMITMSG", "sometimes")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	cfg := LoadConfig()

	// The rejected value must not pollute the flags: the file value survives.
	if !cfg.Flags.AICommitMsg {
		t.Errorf("Flags.AICommitMsg = false, want the file value true to survive an invalid override")
	}
	// Exactly one warning line, naming the correctly spelled variable and the
	// rejected value.
	out := buf.String()
	if n := strings.Count(out, "\n"); n != 1 {
		t.Fatalf("LoadConfig logged %d lines, want exactly one warning line: %q", n, out)
	}
	if !strings.Contains(out, "WGIT_FLAG_AICOMMITMSG") {
		t.Errorf("warning %q does not name WGIT_FLAG_AICOMMITMSG", out)
	}
	if !strings.Contains(out, `"sometimes"`) {
		t.Errorf("warning %q does not quote the rejected value", out)
	}
}

func TestEnvFlagOverridesDoNotRewriteConfigFile(t *testing.T) {
	original := `{"theme":"dark","flags":{"aiCommitMsg":false,"reviewFlow":false},"schemaVersion":2}`
	writeFeatureFlagsConfigFile(t, original)

	t.Setenv("WGIT_FLAG_REVIEWFLOW", "true")
	cfg := LoadConfig()
	if !cfg.Flags.ReviewFlow {
		t.Fatalf("Flags.ReviewFlow = false, want the env override to win in memory")
	}

	// The override is memory-only: LoadConfig under WGIT_FLAG_* must leave
	// config.json byte-for-byte untouched.
	raw, err := os.ReadFile(getConfigPath())
	if err != nil {
		t.Fatalf("re-read config.json: %v", err)
	}
	if string(raw) != original {
		t.Errorf("LoadConfig under WGIT_FLAG_* rewrote config.json:\n got %q\nwant %q", string(raw), original)
	}

	// And no residue either: clear the env and the file value comes back.
	os.Unsetenv("WGIT_FLAG_REVIEWFLOW")
	if LoadConfig().Flags.ReviewFlow {
		t.Errorf("ReviewFlow on after clearing the env: the override leaked to disk")
	}

	// Only an explicit save — as any later AddRecentRepo flow would perform —
	// persists the effective in-memory set, the same one-way contract as
	// applyAICompatibility. The reload above proves the override itself never
	// writes; this proves SaveConfig is what does.
	SaveConfig(cfg)
	if got := LoadConfig(); !got.Flags.ReviewFlow || got.Flags.AICommitMsg {
		t.Errorf("flags after explicit save = %+v, want only ReviewFlow persisted by SaveConfig", got.Flags)
	}
}
