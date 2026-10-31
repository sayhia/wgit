package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// =========================================================================
// AI control plane: durable call history, usage/cost, per-task routing,
// repository overrides and the multi-origin key inventory.
//
// Everything here stores metadata only — never prompts, responses or keys.
// =========================================================================

const (
	aiCallLogsFileName = "ai-call-logs.jsonl"
	maxAICallLogs      = 5000
	aiCallLogBytesCap  = 3 << 20
	aiDefaultSummary   = 30
	aiMaxSummaryDays   = 365
)

var knownAITaskModels = []string{"commit", "summary", "conflict", "explain", "chat", "review", "release"}

func isKnownAITask(kind string) bool {
	for _, candidate := range knownAITaskModels {
		if candidate == kind {
			return true
		}
	}
	return false
}

// AIRepoOverride is the per-repository slice of AIConfig. A nil
// CommitIncludeBody means "inherit the global setting".
type AIRepoOverride struct {
	Model              string            `json:"model,omitempty"`
	CommitLanguage     string            `json:"commitLanguage,omitempty"`
	CommitIncludeBody  *bool             `json:"commitIncludeBody,omitempty"`
	CommitInstructions string            `json:"commitInstructions,omitempty"`
	TaskModels         map[string]string `json:"taskModels,omitempty"`
}

// AIAdvancedConfig is the sanitized advanced settings view. It deliberately
// excludes the API key: keys are managed by the dedicated origin RPCs.
type AIAdvancedConfig struct {
	TaskModels         map[string]string         `json:"taskModels"`
	RepoOverrides      map[string]AIRepoOverride `json:"repoOverrides"`
	ProfileNames       map[string]string         `json:"profileNames"`
	InputUSDPerMTok    float64                   `json:"inputUsdPerMTok"`
	OutputUSDPerMTok   float64                   `json:"outputUsdPerMTok"`
	MonthlyBudgetUSD   float64                   `json:"monthlyBudgetUsd"`
	RetryCount         int                       `json:"retryCount"`
	RateLimitPerMinute int                       `json:"rateLimitPerMinute"`
	FallbackBaseURL    string                    `json:"fallbackBaseUrl"`
	FallbackModel      string                    `json:"fallbackModel"`
	FallbackProtocol   string                    `json:"fallbackProtocol"`
}

// AIKeyEntry is one stored origin key. Fingerprint is the same SHA-256 prefix
// that config.json carries; the plaintext never crosses the process boundary.
type AIKeyEntry struct {
	Origin      string `json:"origin"`
	Fingerprint string `json:"fingerprint"`
	Current     bool   `json:"current"`
	Available   bool   `json:"available"`
}

// AICallLog is one durable generation record.
type AICallLog struct {
	ID            int64   `json:"id"`
	At            int64   `json:"at"`
	Time          string  `json:"time"`
	Kind          string  `json:"kind"`
	Operation     string  `json:"operation"`
	Mode          string  `json:"mode"`
	Model         string  `json:"model"`
	RepoPath      string  `json:"repoPath,omitempty"`
	Status        string  `json:"status"`
	Error         string  `json:"error,omitempty"`
	DurationMs    int64   `json:"durationMs"`
	TokensIn      int     `json:"tokensIn"`
	TokensOut     int     `json:"tokensOut"`
	UsageKnown    bool    `json:"usageKnown"`
	Success       bool    `json:"success"`
	Cancelled     bool    `json:"cancelled"`
	EstimatedCost float64 `json:"estimatedCostUsd"`
	CostKnown     bool    `json:"costKnown"`
}

type AIUsageBucket struct {
	Key             string  `json:"key"`
	Calls           int     `json:"calls"`
	Success         int     `json:"success"`
	Failed          int     `json:"failed"`
	Cancelled       int     `json:"cancelled"`
	TokensIn        int     `json:"tokensIn"`
	TokensOut       int     `json:"tokensOut"`
	UsageKnownCalls int     `json:"usageKnownCalls"`
	CostKnownCalls  int     `json:"costKnownCalls"`
	EstimatedCost   float64 `json:"estimatedCostUsd"`
}

type AIUsageDay struct {
	Date          string  `json:"date"`
	Calls         int     `json:"calls"`
	TokensIn      int     `json:"tokensIn"`
	TokensOut     int     `json:"tokensOut"`
	EstimatedCost float64 `json:"estimatedCostUsd"`
}

type AIUsageSummary struct {
	Days             int             `json:"days"`
	Since            string          `json:"since"`
	Calls            int             `json:"calls"`
	Success          int             `json:"success"`
	Failed           int             `json:"failed"`
	Cancelled        int             `json:"cancelled"`
	TokensIn         int             `json:"tokensIn"`
	TokensOut        int             `json:"tokensOut"`
	UsageKnownCalls  int             `json:"usageKnownCalls"`
	CostKnownCalls   int             `json:"costKnownCalls"`
	CostUnknownCalls int             `json:"costUnknownCalls"`
	EstimatedCost    float64         `json:"estimatedCostUsd"`
	MonthCost        float64         `json:"monthCostUsd"`
	MonthlyBudget    float64         `json:"monthlyBudgetUsd"`
	BudgetPercent    float64         `json:"budgetPercent"`
	OverBudget       bool            `json:"overBudget"`
	InputUSDPerMTok  float64         `json:"inputUsdPerMTok"`
	OutputUSDPerMTok float64         `json:"outputUsdPerMTok"`
	ByKind           []AIUsageBucket `json:"byKind"`
	ByModel          []AIUsageBucket `json:"byModel"`
	ByRepo           []AIUsageBucket `json:"byRepo"`
	ByDay            []AIUsageDay    `json:"byDay"`
}

func aiCallLogsPath() string {
	return filepath.Join(filepath.Dir(getConfigPath()), aiCallLogsFileName)
}

// aiLogCost prefers the cost captured when the call happened, then recomputes
// legacy rows through the model catalog.
func aiLogCost(cfg AIConfig, entry AICallLog) (float64, bool) {
	if entry.CostKnown || entry.EstimatedCost > 0 {
		return entry.EstimatedCost, entry.CostKnown || entry.EstimatedCost > 0
	}
	if entry.TokensIn+entry.TokensOut <= 0 && !entry.UsageKnown {
		// No usage was reported, so the cost is unknown even when the endpoint
		// happens to be local.
		return 0, false
	}
	return aiCostForModel(cfg, entry.Model, entry.TokensIn, entry.TokensOut)
}

var aiCallLogsMu sync.Mutex

func readAICallLogs() ([]AICallLog, error) {
	file, err := os.Open(aiCallLogsPath())
	if errors.Is(err, os.ErrNotExist) {
		return []AICallLog{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	logs := make([]AICallLog, 0, 256)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry AICallLog
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		logs = append(logs, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(logs) > maxAICallLogs {
		logs = logs[len(logs)-maxAICallLogs:]
	}
	return logs, nil
}

func appendAICallLog(entry AICallLog) error {
	aiCallLogsMu.Lock()
	defer aiCallLogsMu.Unlock()

	path := aiCallLogsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil && info.Size() > aiCallLogBytesCap {
		logs, readErr := readAICallLogs()
		if readErr != nil {
			return readErr
		}
		if len(logs) > maxAICallLogs {
			logs = logs[len(logs)-maxAICallLogs:]
		}
		lines := make([]string, 0, len(logs))
		for _, item := range logs {
			data, err := json.Marshal(item)
			if err == nil {
				lines = append(lines, string(data))
			}
		}
		if err := writeFileAtomic(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			return err
		}
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// recordAICallLog is the non-fatal hook used by the generation funnel: an
// audit write failure must never fail the user's AI call.
func recordAICallLog(entry AICallLog) {
	_ = appendAICallLog(entry)
}

// GetAICallLogs returns durable call records newest first.
func (AiService) GetAICallLogs(limit int) ([]AICallLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	logs, err := readAICallLogs()
	if err != nil {
		return nil, err
	}
	cfg := LoadConfig().AI
	out := make([]AICallLog, 0, minInt(limit, len(logs)))
	for i := len(logs) - 1; i >= 0 && len(out) < limit; i-- {
		entry := logs[i]
		entry.EstimatedCost, entry.CostKnown = aiLogCost(cfg, entry)
		out = append(out, entry)
	}
	return out, nil
}

// ClearAICallLogs removes the durable history. The in-memory activity log is
// cleared separately by ClearActivityLogs.
func (AiService) ClearAICallLogs() error {
	aiCallLogsMu.Lock()
	defer aiCallLogsMu.Unlock()
	err := os.Remove(aiCallLogsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ExportAICallLogs writes JSON or CSV into the config directory (0600) and
// returns the absolute path. It never includes prompts or credentials.
func (AiService) ExportAICallLogs(format string) (string, error) {
	logs, err := readAICallLogs()
	if err != nil {
		return "", err
	}
	cfg := LoadConfig().AI
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "csv" {
		format = "json"
	}
	path := filepath.Join(filepath.Dir(getConfigPath()),
		fmt.Sprintf("ai-usage-export-%d.%s", time.Now().UnixNano(), format))
	if format == "json" {
		for i := range logs {
			logs[i].EstimatedCost, logs[i].CostKnown = aiLogCost(cfg, logs[i])
		}
		data, err := json.MarshalIndent(logs, "", "  ")
		if err != nil {
			return "", err
		}
		if err := writeFileAtomic(path, data, 0o600); err != nil {
			return "", err
		}
		return path, nil
	}

	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	_ = writer.Write([]string{
		"time", "kind", "operation", "mode", "model", "repo", "status", "success",
		"cancelled", "duration_ms", "tokens_in", "tokens_out", "usage_known", "estimated_cost_usd", "cost_known",
	})
	for _, entry := range logs {
		cost, costKnown := aiLogCost(cfg, entry)
		_ = writer.Write([]string{
			entry.Time, entry.Kind, entry.Operation, entry.Mode, entry.Model, entry.RepoPath,
			entry.Status, strconv.FormatBool(entry.Success), strconv.FormatBool(entry.Cancelled),
			strconv.FormatInt(entry.DurationMs, 10), strconv.Itoa(entry.TokensIn),
			strconv.Itoa(entry.TokensOut), strconv.FormatBool(entry.UsageKnown),
			strconv.FormatFloat(cost, 'f', 6, 64), strconv.FormatBool(costKnown),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	if err := writeFileAtomic(path, []byte(builder.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// GetAIUsageSummary aggregates the durable log for the dashboard and budget
// warning. BudgetPercent is based on the current calendar month.
func (AiService) GetAIUsageSummary(days int) (AIUsageSummary, error) {
	if days <= 0 {
		days = aiDefaultSummary
	}
	if days > aiMaxSummaryDays {
		days = aiMaxSummaryDays
	}
	logs, err := readAICallLogs()
	if err != nil {
		return AIUsageSummary{}, err
	}
	cfg := LoadConfig().AI
	now := time.Now()
	since := now.AddDate(0, 0, -days)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	summary := AIUsageSummary{
		Days:             days,
		Since:            since.Format(time.RFC3339),
		MonthlyBudget:    cfg.MonthlyBudgetUSD,
		InputUSDPerMTok:  cfg.InputUSDPerMTok,
		OutputUSDPerMTok: cfg.OutputUSDPerMTok,
		ByKind:           []AIUsageBucket{},
		ByModel:          []AIUsageBucket{},
		ByRepo:           []AIUsageBucket{},
		ByDay:            []AIUsageDay{},
	}
	kindBuckets := map[string]*AIUsageBucket{}
	modelBuckets := map[string]*AIUsageBucket{}
	repoBuckets := map[string]*AIUsageBucket{}
	dayBuckets := map[string]*AIUsageDay{}

	for _, entry := range logs {
		at := time.Unix(entry.At, 0)
		if entry.At <= 0 {
			if parsed, parseErr := time.Parse(time.RFC3339, entry.Time); parseErr == nil {
				at = parsed
			}
		}
		cost, costKnown := aiLogCost(cfg, entry)
		if !at.Before(monthStart) {
			summary.MonthCost += cost
		}
		if at.Before(since) {
			continue
		}
		summary.Calls++
		if entry.Success {
			summary.Success++
		} else if entry.Cancelled {
			summary.Cancelled++
		} else {
			summary.Failed++
		}
		summary.TokensIn += entry.TokensIn
		summary.TokensOut += entry.TokensOut
		if entry.UsageKnown || entry.TokensIn+entry.TokensOut > 0 {
			summary.UsageKnownCalls++
		}
		if costKnown {
			summary.CostKnownCalls++
		} else {
			summary.CostUnknownCalls++
		}
		summary.EstimatedCost += cost

		kind := entry.Kind
		if kind == "" {
			kind = "complete"
		}
		model := entry.Model
		if model == "" {
			model = "(default)"
		}
		accumulateUsageBucket(kindBuckets, kind, entry, cost, costKnown)
		accumulateUsageBucket(modelBuckets, model, entry, cost, costKnown)
		accumulateUsageBucket(repoBuckets, entry.RepoPath, entry, cost, costKnown)
		dayKey := at.Format("2006-01-02")
		day := dayBuckets[dayKey]
		if day == nil {
			day = &AIUsageDay{Date: dayKey}
			dayBuckets[dayKey] = day
		}
		day.Calls++
		day.TokensIn += entry.TokensIn
		day.TokensOut += entry.TokensOut
		day.EstimatedCost += cost
	}

	summary.ByKind = sortedUsageBuckets(kindBuckets)
	summary.ByModel = sortedUsageBuckets(modelBuckets)
	summary.ByRepo = sortedUsageBuckets(repoBuckets)
	for _, day := range dayBuckets {
		summary.ByDay = append(summary.ByDay, *day)
	}
	sort.Slice(summary.ByDay, func(i, j int) bool { return summary.ByDay[i].Date < summary.ByDay[j].Date })
	if summary.MonthlyBudget > 0 {
		summary.BudgetPercent = summary.MonthCost / summary.MonthlyBudget * 100
		summary.OverBudget = summary.MonthCost > summary.MonthlyBudget
	}
	return summary, nil
}

func accumulateUsageBucket(buckets map[string]*AIUsageBucket, key string, entry AICallLog, cost float64, costKnown bool) {
	bucket := buckets[key]
	if bucket == nil {
		bucket = &AIUsageBucket{Key: key}
		buckets[key] = bucket
	}
	bucket.Calls++
	if entry.Success {
		bucket.Success++
	} else if entry.Cancelled {
		bucket.Cancelled++
	} else {
		bucket.Failed++
	}
	bucket.TokensIn += entry.TokensIn
	bucket.TokensOut += entry.TokensOut
	if entry.UsageKnown || entry.TokensIn+entry.TokensOut > 0 {
		bucket.UsageKnownCalls++
	}
	if costKnown {
		bucket.CostKnownCalls++
	}
	bucket.EstimatedCost += cost
}

func sortedUsageBuckets(buckets map[string]*AIUsageBucket) []AIUsageBucket {
	out := make([]AIUsageBucket, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, *bucket)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// -------------------------------------------------------------------------
// Multi-origin key inventory
// -------------------------------------------------------------------------

// ListAIKeys reports stored key origins and fingerprints. It migrates a legacy
// raw secret into the configured origin on first read, mirroring the settings
// page behaviour.
func (AiService) ListAIKeys() ([]AIKeyEntry, error) {
	cfg := LoadConfig()
	current := normalizeAPIOrigin(cfg.AI.BaseURL)
	if current != "" {
		if _, err := getAIStoredKeyForOrigin(current, true); err != nil {
			return nil, err
		}
	}
	keys, _, err := readAIKeyring()
	if err != nil {
		return nil, err
	}
	out := make([]AIKeyEntry, 0, len(keys)+1)
	seenCurrent := false
	for origin, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		isCurrent := origin == current
		seenCurrent = seenCurrent || isCurrent
		out = append(out, AIKeyEntry{
			Origin:      origin,
			Fingerprint: aiKeyFingerprint(key),
			Current:     isCurrent,
			Available:   true,
		})
	}
	if current != "" && !seenCurrent && strings.TrimSpace(cfg.AI.APIKey) != "" {
		out = append(out, AIKeyEntry{
			Origin:      current,
			Fingerprint: cfg.AI.APIKey,
			Current:     true,
			Available:   false,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Current != out[j].Current {
			return out[i].Current
		}
		return out[i].Origin < out[j].Origin
	})
	return out, nil
}

// SaveAIKeyForOrigin stores a key for any API origin without switching the
// active endpoint. When it matches the configured origin, the config
// fingerprint is updated too.
func (AiService) SaveAIKeyForOrigin(origin, key string) error {
	normalized := normalizeAPIOrigin(origin)
	if normalized == "" {
		return errors.New("请填写有效的 http(s) API 端点")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("API Key 不能为空")
	}
	if strings.ContainsAny(key, "\r\n") {
		return errors.New("API Key 不能包含换行")
	}
	if len(key) > 8192 {
		return errors.New("API Key 过长")
	}
	if err := setAIStoredKeyForOrigin(normalized, key); err != nil {
		return fmt.Errorf("保存 API Key 失败: %w", err)
	}
	setAISessionKeyForOrigin(normalized, key)
	cfg := LoadConfig()
	if normalizeAPIOrigin(cfg.AI.BaseURL) == normalized {
		cfg.AI.APIKey = aiKeyFingerprint(key)
		clearActiveProfileFingerprint(&cfg.AI)
		cfg.SchemaVersion = CurrentSchemaVersion
		if err := SaveConfig(cfg); err != nil {
			return err
		}
	}
	return nil
}

// DeleteAIKeyForOrigin removes one stored key and clears the fingerprint when
// it belongs to the active endpoint.
func (AiService) DeleteAIKeyForOrigin(origin string) error {
	normalized := normalizeAPIOrigin(origin)
	if normalized == "" {
		return errors.New("无效的 API 端点")
	}
	if err := deleteAIStoredKeyForOrigin(normalized, false); err != nil {
		return err
	}
	clearAISessionKeyForOrigin(normalized)
	cfg := LoadConfig()
	if normalizeAPIOrigin(cfg.AI.BaseURL) == normalized {
		cfg.AI.APIKey = ""
		clearActiveProfileFingerprint(&cfg.AI)
		cfg.SchemaVersion = CurrentSchemaVersion
		return SaveConfig(cfg)
	}
	return nil
}

// -------------------------------------------------------------------------
// Advanced settings + repository overrides
// -------------------------------------------------------------------------

func (AiService) GetAIAdvancedConfig() AIAdvancedConfig {
	cfg := LoadConfig().AI
	out := AIAdvancedConfig{
		TaskModels:         safeTaskModels(cfg.TaskModels),
		RepoOverrides:      map[string]AIRepoOverride{},
		ProfileNames:       safeProfileNames(cfg.ProfileNames, cfg.Profiles),
		InputUSDPerMTok:    cfg.InputUSDPerMTok,
		OutputUSDPerMTok:   cfg.OutputUSDPerMTok,
		MonthlyBudgetUSD:   cfg.MonthlyBudgetUSD,
		RetryCount:         cfg.RetryCount,
		RateLimitPerMinute: cfg.RateLimitPerMinute,
		FallbackBaseURL:    cfg.FallbackBaseURL,
		FallbackModel:      cfg.FallbackModel,
		FallbackProtocol:   cfg.FallbackProtocol,
	}
	for path, override := range cfg.RepoOverrides {
		override.TaskModels = safeTaskModels(override.TaskModels)
		out.RepoOverrides[path] = override
	}
	return out
}

// safeProfileNames keeps only labels that still point at a live profile. An
// empty label is removed so the generated name can take over again.
func safeProfileNames(in map[string]string, profiles []AIProfile) map[string]string {
	out := map[string]string{}
	valid := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		valid[profile.ID] = struct{}{}
	}
	for id, name := range in {
		id = strings.TrimSpace(id)
		name = strings.TrimSpace(name)
		if id == "" || name == "" {
			continue
		}
		if _, ok := valid[id]; !ok {
			continue
		}
		out[id] = truncateRunes(name, 80)
	}
	return out
}

func safeTaskModels(in map[string]string) map[string]string {
	out := map[string]string{}
	for kind, model := range in {
		kind = strings.ToLower(strings.TrimSpace(kind))
		model = strings.TrimSpace(model)
		if isKnownAITask(kind) && model != "" {
			out[kind] = model
		}
	}
	return out
}

func (AiService) SaveAIAdvancedConfig(in AIAdvancedConfig) error {
	taskModels, err := cleanAITaskModels(in.TaskModels)
	if err != nil {
		return err
	}
	if in.InputUSDPerMTok < 0 || in.OutputUSDPerMTok < 0 || in.InputUSDPerMTok > 10000 || in.OutputUSDPerMTok > 10000 {
		return errors.New("token 单价超出有效范围")
	}
	if in.MonthlyBudgetUSD < 0 || in.MonthlyBudgetUSD > 1_000_000 {
		return errors.New("月度预算超出有效范围")
	}
	if in.RetryCount < 0 || in.RetryCount > 3 {
		return errors.New("重试次数需在 0-3 之间")
	}
	if in.RateLimitPerMinute < 0 || in.RateLimitPerMinute > 600 {
		return errors.New("每分钟请求上限需在 0-600 之间")
	}
	fallbackBase := strings.TrimRight(strings.TrimSpace(in.FallbackBaseURL), "/")
	fallbackModel := strings.TrimSpace(in.FallbackModel)
	fallbackProtocol := strings.ToLower(strings.TrimSpace(in.FallbackProtocol))
	if fallbackProtocol == "" {
		fallbackProtocol = "openai"
	}
	switch fallbackProtocol {
	case "openai", "azure", "anthropic", "gemini":
	default:
		return errors.New("未知的备用 API 协议")
	}
	if fallbackBase != "" || fallbackModel != "" {
		if fallbackBase == "" || fallbackModel == "" {
			return errors.New("备用提供商需要同时填写端点和模型")
		}
		parsed := normalizeAPIOrigin(fallbackBase)
		if parsed == "" {
			return errors.New("备用端点需要以 http:// 或 https:// 开头")
		}
	}
	cfg := LoadConfig()
	profileNames := safeProfileNames(in.ProfileNames, cfg.AI.Profiles)
	overrides := map[string]AIRepoOverride{}
	for path, override := range in.RepoOverrides {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		// Keep stale overrides removable even if the repository is temporarily
		// unavailable; the actual generation paths still validate IsGitRepo.
		if !filepath.IsAbs(path) {
			return fmt.Errorf("仓库路径必须是绝对路径: %s", path)
		}
		path = filepath.Clean(path)
		override, err = normalizeAIRepoOverride(override)
		if err != nil {
			return err
		}
		if aiRepoOverrideEmpty(override) {
			continue
		}
		overrides[path] = override
		if len(overrides) > 50 {
			return errors.New("仓库覆盖配置不能超过 50 个")
		}
	}
	cfg.AI.TaskModels = taskModels
	// A partial caller that omits the map must not wipe renamed profiles.
	if in.ProfileNames != nil {
		cfg.AI.ProfileNames = profileNames
	}
	// Repository overrides have their own RPC now. A partial caller that omits
	// the map must not wipe entries edited from the repository menu.
	if in.RepoOverrides != nil {
		cfg.AI.RepoOverrides = overrides
	}
	cfg.AI.InputUSDPerMTok = in.InputUSDPerMTok
	cfg.AI.OutputUSDPerMTok = in.OutputUSDPerMTok
	cfg.AI.MonthlyBudgetUSD = in.MonthlyBudgetUSD
	cfg.AI.RetryCount = in.RetryCount
	cfg.AI.RateLimitPerMinute = in.RateLimitPerMinute
	cfg.AI.FallbackBaseURL = fallbackBase
	cfg.AI.FallbackModel = fallbackModel
	cfg.AI.FallbackProtocol = fallbackProtocol
	cfg.SchemaVersion = CurrentSchemaVersion
	return SaveConfig(cfg)
}

// normalizeAIRepoOverride keeps repository overrides sparse: empty fields mean
// "inherit the global value" rather than resetting it.
func normalizeAIRepoOverride(override AIRepoOverride) (AIRepoOverride, error) {
	language := strings.ToLower(strings.TrimSpace(override.CommitLanguage))
	if language != "zh" && language != "en" && language != "auto" {
		language = ""
	}
	override.Model = strings.TrimSpace(truncateRunes(override.Model, 200))
	override.CommitLanguage = language
	override.CommitInstructions = truncateRunes(strings.TrimSpace(override.CommitInstructions), 2000)
	taskModels, err := cleanAITaskModels(override.TaskModels)
	if err != nil {
		return AIRepoOverride{}, err
	}
	override.TaskModels = taskModels
	return override, nil
}

func aiRepoOverrideEmpty(override AIRepoOverride) bool {
	return override.Model == "" &&
		override.CommitLanguage == "" &&
		override.CommitIncludeBody == nil &&
		override.CommitInstructions == "" &&
		len(override.TaskModels) == 0
}

// GetAIRepoOverride returns the override for one repository (zero when none).
func (AiService) GetAIRepoOverride(repoPath string) AIRepoOverride {
	path := strings.TrimSpace(repoPath)
	if path == "" {
		return AIRepoOverride{}
	}
	override := LoadConfig().AI.RepoOverrides[filepath.Clean(path)]
	if override.TaskModels == nil {
		override.TaskModels = map[string]string{}
	}
	return override
}

// SaveAIRepoOverride writes only one repository entry. An empty override
// removes the entry.
func (AiService) SaveAIRepoOverride(repoPath string, override AIRepoOverride) error {
	path := strings.TrimSpace(repoPath)
	if !filepath.IsAbs(path) {
		return fmt.Errorf("仓库路径必须是绝对路径: %s", path)
	}
	path = filepath.Clean(path)
	normalized, err := normalizeAIRepoOverride(override)
	if err != nil {
		return err
	}
	cfg := LoadConfig()
	if cfg.AI.RepoOverrides == nil {
		cfg.AI.RepoOverrides = map[string]AIRepoOverride{}
	}
	if aiRepoOverrideEmpty(normalized) {
		delete(cfg.AI.RepoOverrides, path)
	} else {
		cfg.AI.RepoOverrides[path] = normalized
	}
	cfg.SchemaVersion = CurrentSchemaVersion
	return SaveConfig(cfg)
}

func (AiService) DeleteAIRepoOverride(repoPath string) error {
	path := strings.TrimSpace(repoPath)
	if path == "" {
		return nil
	}
	cfg := LoadConfig()
	delete(cfg.AI.RepoOverrides, filepath.Clean(path))
	cfg.SchemaVersion = CurrentSchemaVersion
	return SaveConfig(cfg)
}

func cleanAITaskModels(in map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for rawKind, rawModel := range in {
		kind := strings.ToLower(strings.TrimSpace(rawKind))
		model := strings.TrimSpace(rawModel)
		if model == "" {
			continue
		}
		if !isKnownAITask(kind) {
			return nil, fmt.Errorf("未知的 AI 任务类型: %s", rawKind)
		}
		if len(model) > 200 || strings.ContainsAny(model, "\r\n") {
			return nil, fmt.Errorf("任务 %s 的模型名无效", kind)
		}
		out[kind] = model
	}
	return out, nil
}

// aiRepoOverride returns the configured override for repoPath, preferring an
// exact cleaned match.
func aiRepoOverride(cfg AIConfig, repoPath string) (AIRepoOverride, bool) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" || len(cfg.RepoOverrides) == 0 {
		return AIRepoOverride{}, false
	}
	cleaned := filepath.Clean(repoPath)
	override, ok := cfg.RepoOverrides[cleaned]
	return override, ok
}

// aiResolvedAIConfig applies task routing and repository overrides. Precedence
// (lowest to highest): base model, global task model, repo model, repo task
// model. Scalar prompt fields are overridden only when set.
func aiResolvedAIConfig(cfg AIConfig, repoPath, kind string) AIConfig {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if model := strings.TrimSpace(cfg.TaskModels[kind]); model != "" {
		cfg.Model = model
	}
	if override, ok := aiRepoOverride(cfg, repoPath); ok {
		if model := strings.TrimSpace(override.Model); model != "" {
			cfg.Model = model
		}
		if model := strings.TrimSpace(override.TaskModels[kind]); model != "" {
			cfg.Model = model
		}
		switch strings.ToLower(strings.TrimSpace(override.CommitLanguage)) {
		case "auto", "zh", "en":
			cfg.CommitLanguage = strings.ToLower(strings.TrimSpace(override.CommitLanguage))
		}
		if override.CommitIncludeBody != nil {
			cfg.CommitIncludeBody = *override.CommitIncludeBody
		}
		if strings.TrimSpace(override.CommitInstructions) != "" {
			cfg.CommitInstructions = override.CommitInstructions
		}
	}
	return cfg
}

func aiConfigForTask(cfg AIConfig, repoPath, kind string) AIConfig {
	return aiResolvedAIConfig(cfg, repoPath, kind)
}
