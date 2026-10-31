package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FeatureFlags gates advisor-plan §3.2/§3.3 capabilities (AI commit messages,
// AI summaries, batch conflict AI, review flow, asset board). Every flag ships
// dark (false) until its feature mounts; the lowerCamelCase JSON keys mirror
// the frontend FeatureFlags type in frontend/src/types.ts.
type FeatureFlags struct {
	AICommitMsg     bool `json:"aiCommitMsg"`
	AISummary       bool `json:"aiSummary"`
	AIConflictBatch bool `json:"aiConflictBatch"`
	ReviewFlow      bool `json:"reviewFlow"`
	AssetBoard      bool `json:"assetBoard"`
}

// CurrentSchemaVersion is the config schema generation written by SaveConfig.
// Version 2 turns the AI feature flags on and introduces the AI mode/CLI
// fields; older files carry no schemaVersion or an older one and are upgraded
// in memory by LoadConfig.
const CurrentSchemaVersion = 2

// AIConfig holds the backend AI endpoint configuration. BaseURL empty means
// "not configured". APIKey only carries the first 8 hex chars of the key's
// SHA-256 fingerprint: the plaintext lives in the OS secret store (macOS
// Keychain, 0600 file fallback) and is cached in process memory for the
// session.
type AIConfig struct {
	// Mode selects the transport: "api" (OpenAI-compatible HTTP) or "cli"
	// (a local AI command such as claude/codex/ollama). Empty means "api" so
	// configs written before modes existed keep working.
	Mode           string   `json:"mode,omitempty"`
	BaseURL        string   `json:"baseUrl"`
	APIKey         string   `json:"apiKey,omitempty"` // fingerprint only; the key lives in the OS secret store
	Model          string   `json:"model"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
	CLICommand     string   `json:"cliCommand,omitempty"`
	CLIArgs        []string `json:"cliArgs,omitempty"`
	// CLIInput is "stdin" (default) or "arg": where the prompt is delivered.
	CLIInput string `json:"cliInput,omitempty"`
	// CommitLanguage is "auto", "zh" or "en".
	CommitLanguage string `json:"commitLanguage,omitempty"`
	// CommitIncludeBody asks the generator for a subject plus a short body.
	CommitIncludeBody bool `json:"commitIncludeBody,omitempty"`
	// CommitInstructions is optional repository-level prompt guidance appended
	// to the commit-message prompt (max 2000 characters).
	CommitInstructions string `json:"commitInstructions,omitempty"`
	// Protocol selects the API wire format: openai (default), azure,
	// anthropic, or gemini.
	Protocol string `json:"protocol,omitempty"`
	// AzureAPIVersion is required by Azure OpenAI; empty uses the current default.
	AzureAPIVersion string `json:"azureApiVersion,omitempty"`
	// ExtraHeaders are non-secret custom HTTP headers. Reserved auth headers
	// are rejected by SaveAISettings so credentials stay in the key store.
	ExtraHeaders map[string]string `json:"extraHeaders,omitempty"`
	// CLIEnvMode is "inherit" (default) or "minimal"; minimal passes only
	// PATH/HOME and a small allowlist to local AI commands.
	CLIEnvMode string `json:"cliEnvMode,omitempty"`

	// TaskModels routes a generation kind (commit, summary, conflict, explain,
	// chat, review, release) to a specific model; empty inherits Model.
	TaskModels map[string]string `json:"taskModels,omitempty"`
	// RepoOverrides customises model/prompt settings for one repository. Keys
	// are cleaned absolute paths.
	RepoOverrides map[string]AIRepoOverride `json:"repoOverrides,omitempty"`
	// Pricing and budget power the local usage dashboard. Rates are USD per
	// one million tokens; zero disables the estimate for that direction.
	InputUSDPerMTok  float64 `json:"inputUsdPerMTok,omitempty"`
	OutputUSDPerMTok float64 `json:"outputUsdPerMTok,omitempty"`
	MonthlyBudgetUSD float64 `json:"monthlyBudgetUsd,omitempty"`
	// RetryCount is the number of extra attempts after a retryable API failure.
	RetryCount int `json:"retryCount,omitempty"`
	// RateLimitPerMinute paces outbound API calls for this endpoint; 0 is off.
	RateLimitPerMinute int `json:"rateLimitPerMinute,omitempty"`
	// FallbackBaseURL/FallbackModel/FallbackProtocol define an optional second
	// API provider used when the primary provider keeps failing transiently.
	FallbackBaseURL  string `json:"fallbackBaseUrl,omitempty"`
	FallbackModel    string `json:"fallbackModel,omitempty"`
	FallbackProtocol string `json:"fallbackProtocol,omitempty"`

	// Profiles are connection snapshots for one-click switching. Keys stay in
	// the per-origin secret store; a profile only carries non-secret fields and
	// the display fingerprint.
	Profiles      []AIProfile `json:"profiles,omitempty"`
	ActiveProfile string      `json:"activeProfile,omitempty"`
	// ProfileNames overrides the generated profile label. Keys are profile IDs;
	// an empty or missing entry falls back to the generated name.
	ProfileNames map[string]string `json:"profileNames,omitempty"`
}

// AIProfile is a saved AI connection preset.
type AIProfile struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Mode            string            `json:"mode"`
	BaseURL         string            `json:"baseUrl,omitempty"`
	Model           string            `json:"model,omitempty"`
	Protocol        string            `json:"protocol,omitempty"`
	AzureAPIVersion string            `json:"azureApiVersion,omitempty"`
	CLICommand      string            `json:"cliCommand,omitempty"`
	CLIArgs         []string          `json:"cliArgs,omitempty"`
	CLIInput        string            `json:"cliInput,omitempty"`
	CLIEnvMode      string            `json:"cliEnvMode,omitempty"`
	TimeoutSeconds  int               `json:"timeoutSeconds,omitempty"`
	ExtraHeaders    map[string]string `json:"extraHeaders,omitempty"`
	KeyFingerprint  string            `json:"keyFingerprint,omitempty"`
	UpdatedAt       int64             `json:"updatedAt"`
	Active          bool              `json:"active,omitempty"` // response-only
}

// AccessConfig carries the server-mode access tier from plan §3.4. Role is
// "viewer", "operator" or "admin" (lowercase in JSON); a desktop build never
// consults it because effectiveRole() short-circuits to admin there. An empty
// value loads as "admin" and an unrecognised one falls back to "admin" with a
// single [Access] warning, so a hand-edited config can never lock the
// operator out.
type AccessConfig struct {
	Role string `json:"role"`
}

type AppConfig struct {
	RecentRepos   []string     `json:"recentRepos"`
	LastRepo      string       `json:"lastRepo"`
	Theme         string       `json:"theme"` // "dark", "light", or "system"
	Flags         FeatureFlags `json:"flags"`
	AI            AIConfig     `json:"ai"`
	Access        AccessConfig `json:"access"`
	SchemaVersion int          `json:"schemaVersion"`
}

func getConfigPath() string {
	// Tests and server smoke runs can isolate the app config without changing
	// HOME, which local agent CLIs need for their own credentials.
	if dir := strings.TrimSpace(os.Getenv("WGIT_CONFIG_DIR")); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		return filepath.Join(dir, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "wgit-config.json"
	}
	dir := filepath.Join(home, ".config", "wgit")
	_ = os.MkdirAll(dir, 0755)
	target := filepath.Join(dir, "config.json")

	// Backward-compatibility: if ~/.config/wgit/config.json doesn't exist, check ~/.config/ugit/config.json
	if _, err := os.Stat(target); os.IsNotExist(err) {
		oldPath := filepath.Join(home, ".config", "ugit", "config.json")
		if oldData, err := os.ReadFile(oldPath); err == nil {
			_ = os.WriteFile(target, oldData, 0644)
		}
	}
	return target
}

func LoadConfig() AppConfig {
	cfg := AppConfig{
		RecentRepos: []string{},
		Theme:       "dark",
	}
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		// A fresh workspace gets the AI actions on: the user still has to
		// configure a backend before they can run, but the surfaces no longer
		// start dark.
		cfg.Flags = defaultAIFlags()
		cfg.SchemaVersion = CurrentSchemaVersion
		applyAccessDefaults(&cfg)
		applyEnvFlagOverrides(&cfg.Flags)
		applyEnvRoleOverride(&cfg.Access)
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Preserve the unreadable file so the next SaveConfig (e.g. from
		// AddRecentRepo) cannot silently destroy the user's recent-repo list.
		backup := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())
		_ = os.Rename(path, backup)
		fallback := AppConfig{RecentRepos: []string{}, Theme: "dark", SchemaVersion: CurrentSchemaVersion, Flags: defaultAIFlags()}
		applyAccessDefaults(&fallback)
		applyEnvFlagOverrides(&fallback.Flags)
		applyEnvRoleOverride(&fallback.Access)
		return fallback
	}
	// Backward compatibility: a pre-FeatureFlags file has no schemaVersion, and
	// an absent "flags" (or "ai") object already unmarshals to the zero value
	// (all flags dark / AI unconfigured), so only the version marker needs
	// backfilling — in memory, without rewriting the file behind the user's back.
	applyAICompatibility(&cfg)
	applyAccessDefaults(&cfg)
	// WGIT_FLAG_* / WGIT_ROLE overrides run last on every return path so an
	// explicit environment beats both the file and the compatibility upgrade —
	// and so a server deployment with no config.json yet can still light flags
	// up / assume its configured tier.
	applyEnvFlagOverrides(&cfg.Flags)
	applyEnvRoleOverride(&cfg.Access)
	return cfg
}

// applyAICompatibility upgrades pre-v2 configs in memory. The AI flags shipped
// dark while the backend channel was being built; a v1 config would otherwise
// hide every AI surface forever, even after the user configured a backend.
// The migration is one-way and persisted on the next save. A v2 config keeps
// whatever the user toggled.
func defaultAIFlags() FeatureFlags {
	return FeatureFlags{AICommitMsg: true, AISummary: true, AIConflictBatch: true}
}

func applyAICompatibility(cfg *AppConfig) {
	if cfg.SchemaVersion >= CurrentSchemaVersion {
		return
	}
	cfg.Flags.AICommitMsg = true
	cfg.Flags.AISummary = true
	cfg.Flags.AIConflictBatch = true
	cfg.SchemaVersion = CurrentSchemaVersion
}

// envFlagOverrides maps the WGIT_FLAG_* environment variables onto FeatureFlags
// fields: the server-mode escape hatch from roadmap §3.3, where there is no
// settings page to click so flags must be controllable from the environment.
// Names mirror the Go fields uppercased (WGIT_FLAG_AICOMMITMSG, ...).
var envFlagOverrides = [...]struct {
	envVar string
	apply  func(*FeatureFlags, bool)
}{
	{"WGIT_FLAG_AICOMMITMSG", func(f *FeatureFlags, on bool) { f.AICommitMsg = on }},
	{"WGIT_FLAG_AISUMMARY", func(f *FeatureFlags, on bool) { f.AISummary = on }},
	{"WGIT_FLAG_AICONFLICTBATCH", func(f *FeatureFlags, on bool) { f.AIConflictBatch = on }},
	{"WGIT_FLAG_REVIEWFLOW", func(f *FeatureFlags, on bool) { f.ReviewFlow = on }},
	{"WGIT_FLAG_ASSETBOARD", func(f *FeatureFlags, on bool) { f.AssetBoard = on }},
}

// applyEnvFlagOverrides lets the WGIT_FLAG_* environment variables override the
// feature flags LoadConfig just read (file values, defaults or the
// applyAICompatibility upgrade). Values are parsed case-insensitively:
// 1/true/on/yes enable the flag, 0/false/off/no disable it; an unset variable
// does nothing, while an empty or unrecognised value is ignored with a single
// warning line (the operator probably meant to set it). The override only
// mutates the in-memory config — like applyAICompatibility it never rewrites
// config.json behind the user's back; an explicit SaveConfig later is what
// persists the effective set. Desktop builds compile this too, but a desktop
// user who never exports these variables keeps the settings-page behaviour.
func applyEnvFlagOverrides(flags *FeatureFlags) {
	for _, o := range envFlagOverrides {
		raw, ok := os.LookupEnv(o.envVar)
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "1", "true", "on", "yes":
			o.apply(flags, true)
		case "0", "false", "off", "no":
			o.apply(flags, false)
		default:
			log.Printf("[Flags] ignoring invalid %s value %q (want 1/true/on/yes or 0/false/off/no)", o.envVar, raw)
		}
	}
}

// applyAccessDefaults normalises the access tier loaded from config.json
// (plan §3.4). An absent value means "admin" — the historical single-user
// behaviour — and an unrecognised value falls back to "admin" with a single
// [Access] warning instead of silently downgrading or locking the server out.
// Like the flag compatibility upgrade it only mutates the in-memory config;
// nothing here rewrites config.json behind the user's back.
func applyAccessDefaults(cfg *AppConfig) {
	switch role := strings.ToLower(strings.TrimSpace(cfg.Access.Role)); role {
	case "":
		cfg.Access.Role = "admin"
	case "viewer", "operator", "admin":
		cfg.Access.Role = role
	default:
		log.Printf("[Access] ignoring invalid access.role %q, falling back to \"admin\"", cfg.Access.Role)
		cfg.Access.Role = "admin"
	}
}

// applyEnvRoleOverride lets the WGIT_ROLE environment variable override the
// access tier LoadConfig just read, with the exact WGIT_FLAG_* contract: the
// value is parsed case-insensitively, an unset variable does nothing, an empty
// or unrecognised value is ignored with a single warning line, and the
// override only mutates the in-memory config — never config.json. It is what
// lets a server deployment pin or raise its tier without editing the file.
func applyEnvRoleOverride(access *AccessConfig) {
	raw, ok := os.LookupEnv("WGIT_ROLE")
	if !ok {
		return
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "viewer", "operator", "admin":
		access.Role = strings.ToLower(strings.TrimSpace(raw))
	default:
		log.Printf("[Access] ignoring invalid WGIT_ROLE value %q (want viewer/operator/admin)", raw)
	}
}

// SaveConfig persists the config atomically via writeFileAtomic (same-dir temp
// file + rename): a concurrent settings save (SaveAppFlags / SaveAIConfig /
// AddRecentRepo from another window) can then never leave a torn config.json
// behind — readers see either the complete previous file or the complete new
// one. The returned error is the only way the settings UI can tell that a
// save did not land; callers that do not care can ignore it.
func SaveConfig(cfg AppConfig) error {
	path := getConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func AddRecentRepo(repoPath string) {
	cfg := LoadConfig()
	cleaned := filepath.Clean(repoPath)

	var list []string
	list = append(list, cleaned)
	for _, p := range cfg.RecentRepos {
		if p != cleaned && IsGitRepo(p) {
			list = append(list, p)
		}
		if len(list) >= 15 {
			break
		}
	}
	cfg.RecentRepos = list
	cfg.LastRepo = cleaned
	_ = SaveConfig(cfg)
}

// GetAppFlags is a read-only, lightweight Wails RPC exposing the feature-flag
// set. It lives next to the config primitives (the GitService type itself is
// declared in git_service.go; Go permits methods in any file of the package),
// and the frontend wrapper is gitService.getFeatureFlags -> callBackend("GetAppFlags").
func (GitService) GetAppFlags() FeatureFlags {
	return LoadConfig().Flags
}

// SaveAppFlags is the write-side twin of GetAppFlags: it persists a new
// feature-flag set (the settings-page toggles) without disturbing anything
// else the user has configured. It deliberately reloads the current config
// and replaces only the Flags field before saving, so RecentRepos, LastRepo,
// Theme, AI settings and SchemaVersion ride through untouched even if a
// concurrent writer changed them since the frontend last read. The error
// return keeps the RPC signature forward-compatible; SaveConfig currently
// reports failures by leaving the file untouched rather than erroring.
// Frontend wrapper: gitService.saveAppFlags -> callBackend("SaveAppFlags").
func (GitService) SaveAppFlags(flags FeatureFlags) error {
	cfg := LoadConfig()
	old := cfg.Flags
	cfg.Flags = flags
	cfg.SchemaVersion = CurrentSchemaVersion
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	// 灰度开关的每次变更都进活动日志，含变更前后值，便于回溯何时开启了哪个功能。
	recordStateChange("app.flags", fmt.Sprintf(
		"aiCommitMsg=%v→%v aiSummary=%v→%v aiConflictBatch=%v→%v reviewFlow=%v→%v assetBoard=%v→%v",
		old.AICommitMsg, flags.AICommitMsg,
		old.AISummary, flags.AISummary,
		old.AIConflictBatch, flags.AIConflictBatch,
		old.ReviewFlow, flags.ReviewFlow,
		old.AssetBoard, flags.AssetBoard), true)
	return nil
}
