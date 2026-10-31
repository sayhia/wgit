/**
 * The single frontend entry point for the AI subsystem (Go: AiService).
 *
 * Two transports share one configuration surface:
 *   · api — an OpenAI-compatible endpoint; the key lives in the OS secret store
 *     (macOS Keychain) and only a fingerprint reaches config.json.
 *   · cli — a local AI command (claude/codex/ollama/…) run by the Go process.
 *
 * Every generation RPC goes through the backend so prompts, keys and audit
 * entries never depend on a renderer-side configuration copy.
 */
import { Call } from "@wailsio/runtime";
import { aiCommitApi, aiSummaryApi } from "./gitService";
import type { AIStreamInput } from "./aiStreamService";

export type AIMode = "api" | "cli";

/** Sanitized AI configuration view (mirror of Go AIConfigNoSecret). */
export interface AiConfigNoSecret {
  mode: AIMode;
  baseUrl: string;
  model: string;
  timeoutSeconds: number;
  cliCommand: string;
  cliArgs: string[] | null;
  cliInput: "stdin" | "arg" | string;
  commitLanguage: "auto" | "zh" | "en" | string;
  commitIncludeBody: boolean;
  commitInstructions: string;
  protocol: "openai" | "azure" | "anthropic" | "gemini" | string;
  azureApiVersion: string;
  extraHeaders: Record<string, string> | null;
  cliEnvMode: "inherit" | "minimal" | string;
  hasApiKey: boolean;
  keyAvailable: boolean;
  keyFingerprint: string;
  activeProfile: string;
  notice: string;
}

export interface AISaveInput {
  mode: AIMode;
  baseUrl: string;
  model: string;
  timeoutSeconds: number;
  cliCommand: string;
  cliArgs: string[];
  cliInput: "stdin" | "arg";
  commitLanguage?: "auto" | "zh" | "en";
  commitIncludeBody?: boolean;
  commitInstructions?: string;
  protocol?: "openai" | "azure" | "anthropic" | "gemini";
  azureApiVersion?: string;
  extraHeaders?: Record<string, string>;
  cliEnvMode?: "inherit" | "minimal";
  /** Plaintext key for this save; empty means "keep the stored key". */
  apiKey?: string;
  clearApiKey?: boolean;
}

export interface AITestResult {
  ok: boolean;
  mode: AIMode | string;
  message: string;
  models?: string[];
  detail?: string;
}

export interface AICLIOption {
  id: string;
  name: string;
  command: string;
  args: string[];
  input: "stdin" | "arg" | string;
  version: string;
  available: boolean;
}

/** Mirrors normalizeAPIOrigin in git_ai_service.go: scheme + host + effective port. */
export function apiOrigin(raw: string): string {
  try {
    const url = new URL(raw.trim());
    if (url.protocol !== "http:" && url.protocol !== "https:") return "";
    const port = url.port || (url.protocol === "http:" ? "80" : "443");
    return `${url.protocol}//${url.hostname.toLowerCase()}:${port}`;
  } catch {
    return "";
  }
}

export function isSameApiOrigin(a: string, b: string): boolean {
  const left = apiOrigin(a);
  return left !== "" && left === apiOrigin(b);
}

/** Sanitized AI configuration; never carries the plaintext API key. */
export function getAIConfig(): Promise<AiConfigNoSecret> {
  return Call.ByName("main.AiService.GetAIConfig") as Promise<AiConfigNoSecret>;
}

function callAI<T>(method: string, ...args: unknown[]): Promise<T> {
  return Call.ByName(`main.AiService.${method}`, ...args) as Promise<T>;
}

/** Persist the full AI configuration (mode, API or CLI fields, optional key). */
export function saveAISettings(input: AISaveInput): Promise<void> {
  return callAI<void>("SaveAISettings", {
    mode: input.mode,
    baseUrl: input.baseUrl,
    model: input.model,
    timeoutSeconds: input.timeoutSeconds,
    cliCommand: input.cliCommand,
    cliArgs: input.cliArgs,
    cliInput: input.cliInput,
    commitLanguage: input.commitLanguage ?? "auto",
    commitIncludeBody: input.commitIncludeBody ?? false,
    commitInstructions: input.commitInstructions ?? "",
    protocol: input.protocol ?? "openai",
    azureApiVersion: input.azureApiVersion ?? "",
    extraHeaders: input.extraHeaders ?? {},
    cliEnvMode: input.cliEnvMode ?? "inherit",
    apiKey: input.apiKey ?? "",
    clearApiKey: input.clearApiKey ?? false,
  });
}

/**
 * Compatibility wrapper for the old API-only save form. New callers use
 * saveAISettings; empty apiKey keeps the stored one.
 */
export function saveAIConfig(config: {
  baseUrl: string;
  model: string;
  timeoutSeconds: number;
  apiKey?: string;
}): Promise<void> {
  return saveAISettings({
    mode: "api",
    baseUrl: config.baseUrl,
    model: config.model,
    timeoutSeconds: config.timeoutSeconds,
    cliCommand: "",
    cliArgs: [],
    cliInput: "stdin",
    apiKey: config.apiKey ?? "",
  });
}

export function clearAIKey(): Promise<void> {
  return callAI<void>("ClearAIKey");
}

export function detectAICLIs(): Promise<AICLIOption[]> {
  return callAI<AICLIOption[]>("DetectAICLIs");
}

export function testAIConnectionResult(): Promise<AITestResult> {
  return callAI<AITestResult>("TestAIConnectionResult");
}

/** Builds the redacted outbound prompt without sending any request. */
export function previewAIRequest(
  input: Omit<AIStreamInput, "requestId">
): Promise<string> {
  return callAI<string>("PreviewAIRequest", {
    requestId: "",
    kind: input.kind,
    repoPath: input.repoPath ?? "",
    filePath: input.filePath ?? "",
    diff: input.diff ?? "",
    commitHash: input.commitHash ?? "",
    ours: input.ours ?? "",
    theirs: input.theirs ?? "",
    base: input.base ?? "",
    question: input.question ?? "",
    scope: input.scope ?? "",
    history: input.history ?? [],
    from: input.from ?? "",
    to: input.to ?? "",
    maxCommits: input.maxCommits ?? 0,
  });
}

/** Sends one tiny real generation to prove the channel actually works. */
export function testAIDeepResult(): Promise<AITestResult> {
  return callAI<AITestResult>("TestAIDeepResult");
}

/**
 * Fetch the model list for the (possibly unsaved) API form. When apiKey is
 * blank the backend only reuses the stored key if the endpoint origin matches
 * the saved config.
 */
export function listAIModels(
  baseUrl: string,
  apiKey = "",
  protocol: "openai" | "azure" | "anthropic" | "gemini" = "openai",
  azureApiVersion = ""
): Promise<AITestResult> {
  return callAI<AITestResult>("ListAIModels", baseUrl, apiKey, protocol, azureApiVersion);
}

/** True when the selected mode has enough configuration to run. */
export async function isAIConfigured(): Promise<boolean> {
  try {
    const cfg = await getAIConfig();
    if (!cfg) return false;
    if (cfg.mode === "cli") return Boolean(cfg.cliCommand && cfg.cliCommand.trim() !== "");
    // Keyless local endpoints (Ollama, LM Studio) are valid: the endpoint is
    // the requirement, not a key.
    return Boolean(cfg.baseUrl && cfg.baseUrl.trim() !== "");
  } catch {
    return false;
  }
}

/**
 * Generate a single-line conventional commit subject from a staged diff.
 * Backend flag aiCommitMsg must be enabled or the RPC rejects with
 * "feature disabled"; oversized diffs are truncated server-side.
 */
export function generateCommitMessage(repoPath: string, diff: string): Promise<string> {
  return aiCommitApi.generateCommitMessage(repoPath, diff);
}

/**
 * Generate a ≤3-line summary of one commit. Backend flag aiSummary must be
 * enabled or the RPC rejects with "feature disabled"; oversized commit
 * contexts are truncated server-side.
 */
export function summarizeCommit(repoPath: string, commitHash: string): Promise<string> {
  return aiSummaryApi.summarizeCommit(repoPath, commitHash);
}

/** Raw merge suggestion: one fenced block + ≤5 risk lines. */
export function generateConflictAdvice(ours: string, theirs: string, base = ""): Promise<string> {
  return callAI<string>("GenerateConflictAdvice", ours, theirs, base);
}

// ---------------------------------------------------------------------------
// Control plane: advanced settings, origin keys, usage/cost and durable logs
// ---------------------------------------------------------------------------

export type AITaskKind = "commit" | "summary" | "conflict" | "explain" | "chat" | "review" | "release";

export interface AIRepoOverride {
  model?: string;
  commitLanguage?: "auto" | "zh" | "en" | string;
  commitIncludeBody?: boolean | null;
  commitInstructions?: string;
  taskModels?: Record<string, string>;
}

export interface AIAdvancedConfig {
  taskModels: Record<string, string>;
  repoOverrides: Record<string, AIRepoOverride>;
  /** User labels keyed by generated profile ID; empty falls back to the auto name. */
  profileNames: Record<string, string>;
  inputUsdPerMTok: number;
  outputUsdPerMTok: number;
  monthlyBudgetUsd: number;
  retryCount: number;
  rateLimitPerMinute: number;
  fallbackBaseUrl: string;
  fallbackModel: string;
  fallbackProtocol: "openai" | "azure" | "anthropic" | "gemini" | string;
}

export interface AIKeyEntry {
  origin: string;
  fingerprint: string;
  current: boolean;
  available: boolean;
}

export interface AICallLog {
  id: number;
  at: number;
  time: string;
  kind: string;
  operation: string;
  mode: string;
  model: string;
  repoPath?: string;
  status: string;
  error?: string;
  durationMs: number;
  tokensIn: number;
  tokensOut: number;
  usageKnown: boolean;
  success: boolean;
  cancelled: boolean;
  estimatedCostUsd: number;
  costKnown: boolean;
}

export interface AIUsageBucket {
  key: string;
  calls: number;
  success: number;
  failed: number;
  cancelled: number;
  tokensIn: number;
  tokensOut: number;
  usageKnownCalls: number;
  costKnownCalls: number;
  estimatedCostUsd: number;
}

export interface AIUsageDay {
  date: string;
  calls: number;
  tokensIn: number;
  tokensOut: number;
  estimatedCostUsd: number;
}

export interface AIUsageSummary {
  days: number;
  since: string;
  calls: number;
  success: number;
  failed: number;
  cancelled: number;
  tokensIn: number;
  tokensOut: number;
  usageKnownCalls: number;
  costKnownCalls: number;
  costUnknownCalls: number;
  estimatedCostUsd: number;
  monthCostUsd: number;
  monthlyBudgetUsd: number;
  budgetPercent: number;
  overBudget: boolean;
  inputUsdPerMTok: number;
  outputUsdPerMTok: number;
  byKind: AIUsageBucket[];
  byModel: AIUsageBucket[];
  /** Key is an absolute repo path; an empty key means the call had no repo. */
  byRepo: AIUsageBucket[];
  byDay: AIUsageDay[];
}

export function getAIAdvancedConfig(): Promise<AIAdvancedConfig> {
  return callAI<AIAdvancedConfig>("GetAIAdvancedConfig");
}

export function saveAIAdvancedConfig(input: AIAdvancedConfig): Promise<void> {
  return callAI<void>("SaveAIAdvancedConfig", input);
}

export function listAIKeys(): Promise<AIKeyEntry[]> {
  return callAI<AIKeyEntry[]>("ListAIKeys");
}

/** Stores a key for an origin without switching the active endpoint. */
export function saveAIKeyForOrigin(origin: string, apiKey: string): Promise<void> {
  return callAI<void>("SaveAIKeyForOrigin", origin, apiKey);
}

export function deleteAIKeyForOrigin(origin: string): Promise<void> {
  return callAI<void>("DeleteAIKeyForOrigin", origin);
}

export function getAICallLogs(limit = 100): Promise<AICallLog[]> {
  return callAI<AICallLog[]>("GetAICallLogs", limit);
}

export function clearAICallLogs(): Promise<void> {
  return callAI<void>("ClearAICallLogs");
}

/** Writes a JSON/CSV export into the config directory and returns its path. */
export function exportAICallLogs(format: "json" | "csv" = "csv"): Promise<string> {
  return callAI<string>("ExportAICallLogs", format);
}

export function getAIUsageSummary(days = 30): Promise<AIUsageSummary> {
  return callAI<AIUsageSummary>("GetAIUsageSummary", days);
}

// ---------------------------------------------------------------------------
// Provider catalog, connection profiles and repository-scoped overrides
// ---------------------------------------------------------------------------

export interface AIProviderPreset {
  id: string;
  name: string;
  mode: "api" | string;
  protocol: "openai" | "azure" | "anthropic" | "gemini" | string;
  baseUrl: string;
  models: string[] | null;
  defaultModel: string;
  keyRequired: boolean;
  local?: boolean;
}

export interface AIModelPrice {
  model: string;
  provider: string;
  inputUsdPerMTok: number;
  outputUsdPerMTok: number;
  contextWindow: number;
}

export interface AIProviderCatalog {
  providers: AIProviderPreset[] | null;
  models: AIModelPrice[] | null;
}

export interface AIProfile {
  id: string;
  name: string;
  mode: "api" | "cli" | string;
  baseUrl?: string;
  model?: string;
  protocol?: string;
  azureApiVersion?: string;
  cliCommand?: string;
  cliArgs?: string[] | null;
  cliInput?: string;
  cliEnvMode?: string;
  timeoutSeconds?: number;
  extraHeaders?: Record<string, string> | null;
  keyFingerprint?: string;
  updatedAt?: number;
  active?: boolean;
}

export function getAIProviderCatalog(): Promise<AIProviderCatalog> {
  return callAI<AIProviderCatalog>("GetAIProviderCatalog");
}

export function listAIProfiles(): Promise<AIProfile[]> {
  return callAI<AIProfile[]>("ListAIProfiles");
}

export function activateAIProfile(id: string): Promise<void> {
  return callAI<void>("ActivateAIProfile", id);
}

export function deleteAIProfile(id: string): Promise<void> {
  return callAI<void>("DeleteAIProfile", id);
}

export function getAIRepoOverride(repoPath: string): Promise<AIRepoOverride> {
  return callAI<AIRepoOverride>("GetAIRepoOverride", repoPath);
}

export function saveAIRepoOverride(repoPath: string, override: AIRepoOverride): Promise<void> {
  return callAI<void>("SaveAIRepoOverride", repoPath, override);
}

export function deleteAIRepoOverride(repoPath: string): Promise<void> {
  return callAI<void>("DeleteAIRepoOverride", repoPath);
}

/** Longest-prefix model price lookup mirroring the Go catalog. */
export function findModelPrice(catalog: AIProviderCatalog | null, model: string): AIModelPrice | null {
  const key = model.trim().toLowerCase().split("/").pop() || "";
  if (!key || !catalog?.models) return null;
  let best: AIModelPrice | null = null;
  for (const price of catalog.models) {
    const candidate = price.model.toLowerCase();
    if (key === candidate || key.startsWith(candidate)) {
      if (!best || candidate.length > best.model.length) best = price;
    }
  }
  return best;
}
