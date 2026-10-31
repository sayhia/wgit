import { describe, expect, test } from "bun:test";
import {
  AI_CONFIG_BUNDLE_KIND,
  buildAIConfigBundle,
  parseAIConfigBundle,
  serializeAIConfigBundle,
  toAdvancedSaveInput,
  toAISaveInput,
} from "../src/utils/aiConfigTransfer";
import type { AIAdvancedConfig, AiConfigNoSecret } from "../src/services/aiService";

const CONFIG: AiConfigNoSecret = {
  mode: "api",
  baseUrl: "https://api.openai.com/v1",
  model: "gpt-4o",
  timeoutSeconds: 45,
  cliCommand: "",
  cliArgs: [],
  cliInput: "stdin",
  commitLanguage: "zh",
  commitIncludeBody: true,
  commitInstructions: "use team scopes",
  protocol: "openai",
  azureApiVersion: "2024-10-21",
  extraHeaders: { "X-Team": "wgit" },
  cliEnvMode: "inherit",
  hasApiKey: true,
  keyAvailable: true,
  keyFingerprint: "deadbeef",
  activeProfile: "p1",
  notice: "",
};

const ADVANCED: AIAdvancedConfig = {
  taskModels: { commit: "gpt-4o-mini" },
  repoOverrides: {
    "/Users/me/repo": { model: "repo-model" },
  },
  profileNames: { p1: "Production" },
  inputUsdPerMTok: 2.5,
  outputUsdPerMTok: 10,
  monthlyBudgetUsd: 20,
  retryCount: 1,
  rateLimitPerMinute: 30,
  fallbackBaseUrl: "https://fallback.example/v1",
  fallbackModel: "fallback-model",
  fallbackProtocol: "openai",
};

describe("AI configuration transfer", () => {
  test("exports portable config without keys, fingerprints or repo paths", () => {
    const json = serializeAIConfigBundle(buildAIConfigBundle(CONFIG, ADVANCED, new Date(0)));
    expect(json).toContain(AI_CONFIG_BUNDLE_KIND);
    expect(json).toContain("Production");
    expect(json).not.toContain("deadbeef");
    expect(json).not.toContain("keyFingerprint");
    expect(json).not.toContain("hasApiKey");
    expect(json).not.toContain("/Users/me/repo");
    expect(json).not.toContain("repoOverrides");
  });

  test("round-trips a sanitized bundle and normalizes missing values", () => {
    const parsed = parseAIConfigBundle(
      serializeAIConfigBundle(buildAIConfigBundle(CONFIG, ADVANCED, new Date(0)))
    );
    expect(parsed.ok).toBe(true);
    if (!parsed.ok) return;
    expect(parsed.bundle.config.baseUrl).toBe("https://api.openai.com/v1");
    expect(parsed.bundle.config.commitLanguage).toBe("zh");
    expect(parsed.bundle.config.extraHeaders).toEqual({ "X-Team": "wgit" });
    expect(parsed.bundle.advanced.taskModels).toEqual({ commit: "gpt-4o-mini" });
    expect(parsed.bundle.advanced.profileNames).toEqual({ p1: "Production" });
    expect(parsed.bundle.advanced.fallbackBaseUrl).toBe("https://fallback.example/v1");
    expect(parsed.bundle.exportedAt).toBe(new Date(0).toISOString());
  });

  test("rejects malformed, wrong-kind, wrong-version and secret-bearing files", () => {
    expect(parseAIConfigBundle("{")).toEqual({ ok: false, reason: "json" });
    expect(parseAIConfigBundle("[]")).toEqual({ ok: false, reason: "shape" });
    expect(parseAIConfigBundle(JSON.stringify({ kind: "other" }))).toEqual({
      ok: false,
      reason: "shape",
    });
    const bundle = buildAIConfigBundle(CONFIG, ADVANCED);
    expect(parseAIConfigBundle(JSON.stringify({ ...bundle, version: 99 }))).toEqual({
      ok: false,
      reason: "version",
    });
    const withSecret = {
      ...bundle,
      config: { ...bundle.config, apiKey: "sk-live-secret" },
    };
    expect(parseAIConfigBundle(JSON.stringify(withSecret))).toEqual({
      ok: false,
      reason: "secret",
    });
  });

  test("requires an endpoint/model for API mode and a command for CLI mode", () => {
    const bundle = buildAIConfigBundle(CONFIG, ADVANCED);
    const missingModel = { ...bundle, config: { ...bundle.config, model: " " } };
    expect(parseAIConfigBundle(JSON.stringify(missingModel))).toEqual({
      ok: false,
      reason: "shape",
    });
    const cli = {
      ...bundle,
      config: {
        ...bundle.config,
        mode: "cli",
        cliCommand: " ",
        baseUrl: "",
      },
    };
    expect(parseAIConfigBundle(JSON.stringify(cli))).toEqual({ ok: false, reason: "shape" });
  });

  test("import mapping keeps keys out and preserves local repository overrides", () => {
    const bundle = buildAIConfigBundle(CONFIG, ADVANCED);
    const save = toAISaveInput(bundle.config);
    expect(save.apiKey).toBe("");
    expect(save.mode).toBe("api");
    expect(save.extraHeaders).toEqual({ "X-Team": "wgit" });

    const advanced = toAdvancedSaveInput(bundle.advanced, {
      "/local/repo": { model: "local-model" },
    });
    expect(advanced.repoOverrides).toEqual({ "/local/repo": { model: "local-model" } });
    expect(advanced.profileNames).toEqual({ p1: "Production" });
    expect(advanced.monthlyBudgetUsd).toBe(20);
  });
});
