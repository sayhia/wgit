import type {
  AIAdvancedConfig,
  AIRepoOverride,
  AISaveInput,
  AiConfigNoSecret,
} from "../services/aiService";

/**
 * Portable AI configuration bundle. Secrets never enter this shape:
 * keys stay in the OS keychain and are re-entered per origin after import.
 * Repository overrides are deliberately excluded because they contain
 * machine-specific absolute paths.
 */
export const AI_CONFIG_BUNDLE_KIND = "wgit.ai.config";
export const AI_CONFIG_BUNDLE_VERSION = 1;

export type AIConfigProtocol = "openai" | "azure" | "anthropic" | "gemini";
export type AIConfigMode = "api" | "cli";

export interface AIConfigBundleConfig {
  mode: AIConfigMode;
  baseUrl: string;
  model: string;
  timeoutSeconds: number;
  cliCommand: string;
  cliArgs: string[];
  cliInput: "stdin" | "arg";
  commitLanguage: "auto" | "zh" | "en";
  commitIncludeBody: boolean;
  commitInstructions: string;
  protocol: AIConfigProtocol;
  azureApiVersion: string;
  extraHeaders: Record<string, string>;
  cliEnvMode: "inherit" | "minimal";
}

export interface AIConfigBundleAdvanced {
  taskModels: Record<string, string>;
  inputUsdPerMTok: number;
  outputUsdPerMTok: number;
  monthlyBudgetUsd: number;
  retryCount: number;
  rateLimitPerMinute: number;
  fallbackBaseUrl: string;
  fallbackModel: string;
  fallbackProtocol: AIConfigProtocol;
  profileNames: Record<string, string>;
}

export interface AIConfigBundle {
  kind: typeof AI_CONFIG_BUNDLE_KIND;
  version: typeof AI_CONFIG_BUNDLE_VERSION;
  exportedAt: string;
  config: AIConfigBundleConfig;
  advanced: AIConfigBundleAdvanced;
}

export type AIConfigParseFailure = "json" | "shape" | "version" | "secret";
export type AIConfigParseResult =
  | { ok: true; bundle: AIConfigBundle }
  | { ok: false; reason: AIConfigParseFailure };

const PROTOCOLS: AIConfigProtocol[] = ["openai", "azure", "anthropic", "gemini"];

function text(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value.trim() : fallback;
}

function finiteNumber(value: unknown, fallback: number, min: number, max: number): number {
  const parsed = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.min(max, Math.max(min, parsed));
}

function stringMap(value: unknown, maxEntries: number): Record<string, string> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  const out: Record<string, string> = {};
  for (const [rawKey, rawValue] of Object.entries(value)) {
    if (typeof rawValue !== "string") continue;
    const key = rawKey.trim();
    const entry = rawValue.trim();
    if (!key || !entry) continue;
    out[key] = entry;
    if (Object.keys(out).length >= maxEntries) break;
  }
  return out;
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string").map((item) => item.trim()).filter(Boolean)
    : [];
}

function protocol(value: unknown): AIConfigProtocol {
  const next = text(value, "openai").toLowerCase();
  return (PROTOCOLS as string[]).includes(next) ? (next as AIConfigProtocol) : "openai";
}

function sanitizeConfig(config: AiConfigNoSecret): AIConfigBundleConfig {
  return {
    mode: config.mode === "cli" ? "cli" : "api",
    baseUrl: text(config.baseUrl),
    model: text(config.model),
    timeoutSeconds: finiteNumber(config.timeoutSeconds, 60, 1, 600),
    cliCommand: text(config.cliCommand),
    cliArgs: stringArray(config.cliArgs),
    cliInput: config.cliInput === "arg" ? "arg" : "stdin",
    commitLanguage:
      config.commitLanguage === "zh" || config.commitLanguage === "en"
        ? config.commitLanguage
        : "auto",
    commitIncludeBody: config.commitIncludeBody === true,
    commitInstructions: text(config.commitInstructions).slice(0, 2000),
    protocol: protocol(config.protocol),
    azureApiVersion: text(config.azureApiVersion) || "2024-10-21",
    extraHeaders: stringMap(config.extraHeaders, 20),
    cliEnvMode: config.cliEnvMode === "minimal" ? "minimal" : "inherit",
  };
}

function sanitizeAdvanced(advanced: AIAdvancedConfig): AIConfigBundleAdvanced {
  return {
    taskModels: stringMap(advanced.taskModels, 20),
    inputUsdPerMTok: finiteNumber(advanced.inputUsdPerMTok, 0, 0, 10000),
    outputUsdPerMTok: finiteNumber(advanced.outputUsdPerMTok, 0, 0, 10000),
    monthlyBudgetUsd: finiteNumber(advanced.monthlyBudgetUsd, 0, 0, 1_000_000),
    retryCount: Math.round(finiteNumber(advanced.retryCount, 0, 0, 3)),
    rateLimitPerMinute: Math.round(finiteNumber(advanced.rateLimitPerMinute, 0, 0, 600)),
    fallbackBaseUrl: text(advanced.fallbackBaseUrl).replace(/\/+$/, ""),
    fallbackModel: text(advanced.fallbackModel),
    fallbackProtocol: protocol(advanced.fallbackProtocol),
    profileNames: stringMap(advanced.profileNames, 20),
  };
}

export function buildAIConfigBundle(
  config: AiConfigNoSecret,
  advanced: AIAdvancedConfig,
  exportedAt: Date = new Date()
): AIConfigBundle {
  return {
    kind: AI_CONFIG_BUNDLE_KIND,
    version: AI_CONFIG_BUNDLE_VERSION,
    exportedAt: exportedAt.toISOString(),
    config: sanitizeConfig(config),
    advanced: sanitizeAdvanced(advanced),
  };
}

export function serializeAIConfigBundle(bundle: AIConfigBundle): string {
  return JSON.stringify(bundle, null, 2);
}

export function parseAIConfigBundle(raw: string): AIConfigParseResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return { ok: false, reason: "json" };
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return { ok: false, reason: "shape" };
  }
  const record = parsed as Record<string, unknown>;
  if (record.kind !== AI_CONFIG_BUNDLE_KIND) return { ok: false, reason: "shape" };
  if (record.version !== AI_CONFIG_BUNDLE_VERSION) return { ok: false, reason: "version" };
  const rawConfig = record.config;
  const rawAdvanced = record.advanced;
  if (!rawConfig || typeof rawConfig !== "object" || Array.isArray(rawConfig)) {
    return { ok: false, reason: "shape" };
  }
  if (!rawAdvanced || typeof rawAdvanced !== "object" || Array.isArray(rawAdvanced)) {
    return { ok: false, reason: "shape" };
  }
  const config = rawConfig as Record<string, unknown>;
  const advanced = rawAdvanced as Record<string, unknown>;
  if (typeof config.apiKey === "string" && config.apiKey.trim() !== "") {
    return { ok: false, reason: "secret" };
  }
  const mode: AIConfigMode = config.mode === "cli" ? "cli" : "api";
  if (mode === "cli" ? !text(config.cliCommand) : !text(config.baseUrl) || !text(config.model)) {
    return { ok: false, reason: "shape" };
  }
  const normalized: AIConfigBundle = {
    kind: AI_CONFIG_BUNDLE_KIND,
    version: AI_CONFIG_BUNDLE_VERSION,
    exportedAt: text(record.exportedAt) || new Date().toISOString(),
    config: {
      mode,
      baseUrl: text(config.baseUrl),
      model: text(config.model),
      timeoutSeconds: finiteNumber(config.timeoutSeconds, 60, 1, 600),
      cliCommand: text(config.cliCommand),
      cliArgs: stringArray(config.cliArgs),
      cliInput: config.cliInput === "arg" ? "arg" : "stdin",
      commitLanguage:
        config.commitLanguage === "zh" || config.commitLanguage === "en"
          ? config.commitLanguage
          : "auto",
      commitIncludeBody: config.commitIncludeBody === true,
      commitInstructions: text(config.commitInstructions).slice(0, 2000),
      protocol: protocol(config.protocol),
      azureApiVersion: text(config.azureApiVersion) || "2024-10-21",
      extraHeaders: stringMap(config.extraHeaders, 20),
      cliEnvMode: config.cliEnvMode === "minimal" ? "minimal" : "inherit",
    },
    advanced: {
      taskModels: stringMap(advanced.taskModels, 20),
      inputUsdPerMTok: finiteNumber(advanced.inputUsdPerMTok, 0, 0, 10000),
      outputUsdPerMTok: finiteNumber(advanced.outputUsdPerMTok, 0, 0, 10000),
      monthlyBudgetUsd: finiteNumber(advanced.monthlyBudgetUsd, 0, 0, 1_000_000),
      retryCount: Math.round(finiteNumber(advanced.retryCount, 0, 0, 3)),
      rateLimitPerMinute: Math.round(finiteNumber(advanced.rateLimitPerMinute, 0, 0, 600)),
      fallbackBaseUrl: text(advanced.fallbackBaseUrl).replace(/\/+$/, ""),
      fallbackModel: text(advanced.fallbackModel),
      fallbackProtocol: protocol(advanced.fallbackProtocol),
      profileNames: stringMap(advanced.profileNames, 20),
    },
  };
  return { ok: true, bundle: normalized };
}

export function toAISaveInput(config: AIConfigBundleConfig): AISaveInput {
  return {
    mode: config.mode,
    baseUrl: config.baseUrl,
    model: config.model,
    timeoutSeconds: config.timeoutSeconds,
    cliCommand: config.cliCommand,
    cliArgs: [...config.cliArgs],
    cliInput: config.cliInput,
    commitLanguage: config.commitLanguage,
    commitIncludeBody: config.commitIncludeBody,
    commitInstructions: config.commitInstructions,
    protocol: config.protocol,
    azureApiVersion: config.azureApiVersion,
    extraHeaders: { ...config.extraHeaders },
    cliEnvMode: config.cliEnvMode,
    apiKey: "",
  };
}

export function toAdvancedSaveInput(
  advanced: AIConfigBundleAdvanced,
  currentRepoOverrides: Record<string, AIRepoOverride>
): AIAdvancedConfig {
  return {
    taskModels: { ...advanced.taskModels },
    repoOverrides: { ...currentRepoOverrides },
    profileNames: { ...advanced.profileNames },
    inputUsdPerMTok: advanced.inputUsdPerMTok,
    outputUsdPerMTok: advanced.outputUsdPerMTok,
    monthlyBudgetUsd: advanced.monthlyBudgetUsd,
    retryCount: advanced.retryCount,
    rateLimitPerMinute: advanced.rateLimitPerMinute,
    fallbackBaseUrl: advanced.fallbackBaseUrl,
    fallbackModel: advanced.fallbackModel,
    fallbackProtocol: advanced.fallbackProtocol,
  };
}
