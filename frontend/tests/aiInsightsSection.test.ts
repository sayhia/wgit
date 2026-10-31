/**
 * AiInsightsSection — settings redesign Task 8.
 *
 * The insights leaf composes three read-only surfaces: usage (existing
 * `AiUsageCard`), local-only diagnostics and config export/import. Source
 * assertions pin the extraction contract (order, SettingsCard, legacy ids,
 * no network send, no hardcoded copy); mounted cases drive the real components
 * with only the Wails runtime mocked (house pattern: aiBehaviorSection.test.ts).
 */
import { beforeEach, describe, expect, mock, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flushPromises, mount } from "@vue/test-utils";
import "./domSetup";
import { resetToasts, toasts } from "../src/composables/useToast";
import { confirmState, resetConfirm, settleConfirm } from "../src/composables/useConfirm";
import type {
  AIAdvancedConfig,
  AICLIOption,
  AIProviderCatalog,
  AIUsageSummary,
  AISaveInput,
  AiConfigNoSecret,
} from "../src/services/aiService";

const read = (path: string): string =>
  readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

const SECTIONS = [
  "../src/components/settings/AiInsightsSection.vue",
  "../src/components/settings/AiDiagnosticsCard.vue",
  "../src/components/settings/AiTransferActions.vue",
] as const;

function baseConfig(): AiConfigNoSecret {
  return {
    mode: "api",
    baseUrl: "https://api.openai.com/v1",
    model: "gpt-4o",
    timeoutSeconds: 60,
    cliCommand: "",
    cliArgs: [],
    cliInput: "stdin",
    commitLanguage: "zh",
    commitIncludeBody: true,
    commitInstructions: "Keep it short",
    protocol: "openai",
    azureApiVersion: "2024-10-21",
    extraHeaders: { "X-Title": "wgit" },
    cliEnvMode: "inherit",
    hasApiKey: true,
    keyAvailable: true,
    keyFingerprint: "abcd1234",
    activeProfile: "p1",
    notice: "",
  };
}

function baseAdvanced(): AIAdvancedConfig {
  return {
    taskModels: {},
    repoOverrides: { "/local/repo": { model: "local-model" } },
    profileNames: {},
    inputUsdPerMTok: 2.5,
    outputUsdPerMTok: 10,
    monthlyBudgetUsd: 20,
    retryCount: 0,
    rateLimitPerMinute: 0,
    fallbackBaseUrl: "",
    fallbackModel: "",
    fallbackProtocol: "openai",
  };
}

function baseCatalog(): AIProviderCatalog {
  return {
    providers: [
      {
        id: "openai",
        name: "OpenAI",
        mode: "api",
        protocol: "openai",
        baseUrl: "https://api.openai.com/v1",
        models: ["gpt-4o"],
        defaultModel: "gpt-4o",
        keyRequired: true,
        local: false,
      },
    ],
    models: [
      {
        model: "gpt-4o",
        provider: "openai",
        inputUsdPerMTok: 2.5,
        outputUsdPerMTok: 10,
        contextWindow: 128000,
      },
    ],
  };
}

function baseUsage(): AIUsageSummary {
  return {
    days: 30,
    since: "2026-10-01T00:00:00Z",
    calls: 4,
    success: 4,
    failed: 0,
    cancelled: 0,
    tokensIn: 100,
    tokensOut: 200,
    usageKnownCalls: 4,
    costKnownCalls: 4,
    costUnknownCalls: 0,
    estimatedCostUsd: 0.01,
    monthCostUsd: 0.02,
    monthlyBudgetUsd: 20,
    budgetPercent: 0.1,
    overBudget: false,
    inputUsdPerMTok: 2.5,
    outputUsdPerMTok: 10,
    byKind: [],
    byModel: [],
    byRepo: [],
    byDay: [],
  };
}

const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T;

let config: AiConfigNoSecret = baseConfig();
let advanced: AIAdvancedConfig = baseAdvanced();
let usage: AIUsageSummary = baseUsage();
let savedSettings: AISaveInput | null = null;
let savedAdvanced: AIAdvancedConfig | null = null;
let calledRPCs: string[] = [];

function resetState(): void {
  config = baseConfig();
  advanced = baseAdvanced();
  usage = baseUsage();
  savedSettings = null;
  savedAdvanced = null;
  calledRPCs = [];
  resetToasts();
  resetConfirm();
}

async function respond(name: string, args: unknown[]): Promise<unknown> {
  calledRPCs.push(name);
  switch (name) {
    case "main.AiService.GetAIConfig":
      return clone(config);
    case "main.AiService.GetAIAdvancedConfig":
      return clone(advanced);
    case "main.AiService.GetAIProviderCatalog":
      return baseCatalog();
    case "main.AiService.DetectAICLIs":
      return [] as AICLIOption[];
    case "main.AiService.GetAIUsageSummary":
      return clone(usage);
    case "main.AiService.SaveAISettings": {
      const payload = clone(args[0] as AISaveInput);
      savedSettings = payload;
      config = {
        ...config,
        mode: payload.mode,
        baseUrl: payload.baseUrl,
        model: payload.model,
        commitLanguage: payload.commitLanguage ?? "auto",
        commitIncludeBody: payload.commitIncludeBody ?? false,
        commitInstructions: payload.commitInstructions ?? "",
      };
      return null;
    }
    case "main.AiService.SaveAIAdvancedConfig": {
      const payload = clone(args[0] as AIAdvancedConfig);
      savedAdvanced = payload;
      advanced = clone(payload);
      return null;
    }
    default:
      return null;
  }
}

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByName: (name: string, ...args: unknown[]) => respond(name, args),
    ByID: () => Promise.resolve(null),
  },
  Events: { On: () => () => {}, Emit: () => {}, Off: () => {} },
  Window: { Name: () => "test" },
  Screens: {},
}));

const AiInsightsSection = (
  await import("../src/components/settings/AiInsightsSection.vue")
).default;
const { buildAIConfigBundle, serializeAIConfigBundle } = await import(
  "../src/utils/aiConfigTransfer"
);

beforeEach(resetState);

function setImportFile(wrapper: ReturnType<typeof mount>, raw: string): void {
  const input = wrapper.find("[data-testid='ai-config-import-file']");
  expect(input.exists()).toBe(true);
  Object.defineProperty(input.element, "files", {
    value: [{ size: raw.length, text: async () => raw }],
    configurable: true,
  });
}

describe("AiInsightsSection (source contract)", () => {
  test("composes usage, diagnostics and transfer in that order", () => {
    const src = read(SECTIONS[0]);
    const usageAt = src.indexOf("<AiUsageCard");
    const diagnosticsAt = src.indexOf("<AiDiagnosticsCard");
    const transferAt = src.indexOf("<AiTransferActions");
    expect(usageAt).toBeGreaterThan(-1);
    expect(diagnosticsAt).toBeGreaterThan(usageAt);
    expect(transferAt).toBeGreaterThan(diagnosticsAt);
    for (const text of ["AiUsageCard", "AiDiagnosticsCard", "AiTransferActions"]) {
      expect(src).toContain(text);
    }
  });

  test("forwards refreshKey to every child and emits changed from transfer only", () => {
    const src = read(SECTIONS[0]);
    expect(src).toContain(':refresh-key="refreshKey"');
    expect(src).toContain("emit('changed')");
    expect(src).toContain('@changed="emit(\'changed\')"');
    expect(src).toContain("refreshKey?: number");
    // No profiles/keys/routing/reliability surfaces in this leaf.
    for (const absent of [
      "AiProfilesCard",
      "AiKeysCard",
      "AiTaskRoutingCard",
      "AiReliabilityCard",
      "AiConnectionForm",
    ]) {
      expect(src).not.toContain(absent);
    }
  });

  test("diagnostics card is read-only, uses SettingsCard and keeps the legacy ids", () => {
    const src = read(SECTIONS[1]);
    expect(src).toContain("SettingsCard");
    expect(src).toContain("ai-diagnostics-card");
    expect(src).toContain("ai-section-diagnostics");
    expect(src).toContain("ai-diagnostic-");
    for (const level of ['"ok"', '"warn"', '"fail"']) {
      expect(src).toContain(level);
    }
    for (const forbidden of [
      "StartAIStream",
      "testAIConnection",
      "TestAI",
      "PreviewAIRequest",
      "startAIStream",
    ]) {
      expect(src).not.toContain(forbidden);
    }
  });

  test("transfer keeps export/import ids, the hidden file input and secret-safe helpers", () => {
    const src = read(SECTIONS[2]);
    expect(src).toContain("SettingsCard");
    for (const text of [
      'data-testid="ai-config-export"',
      'data-testid="ai-config-import"',
      'data-testid="ai-config-import-file"',
      'type="file"',
      "buildAIConfigBundle",
      "parseAIConfigBundle",
      "serializeAIConfigBundle",
      "toAISaveInput",
      "toAdvancedSaveInput",
      "confirmAction",
      "pushToast",
      "1_000_000",
      "repoOverrides",
    ]) {
      expect(src).toContain(text);
    }
    // Key material never enters the bundle or the surface.
    for (const secret of ["keyFingerprint", "keyAvailable", "apiKey:"]) {
      expect(src).not.toContain(secret);
    }
  });

  test("no hardcoded UI copy in the three new components", () => {
    for (const path of SECTIONS) {
      const src = read(path);
      expect(src).toContain("t(");
      expect(src).not.toMatch(/[\u4e00-\u9fff]/);
    }
  });
});

describe("AiInsightsSection (mounted)", () => {
  test("renders usage, local diagnostics and transfer against one backend state", async () => {
    const wrapper = mount(AiInsightsSection);
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-usage-card']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-diagnostics-card']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-config-export']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-config-import']").exists()).toBe(true);

    expect(wrapper.find("[data-testid='ai-diagnostic-endpoint']").text()).toContain(
      "https://api.openai.com/v1"
    );
    expect(wrapper.find("[data-testid='ai-diagnostic-key']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-diagnostic-model']").text()).toContain("gpt-4o");
    expect(wrapper.find("[data-testid='ai-diagnostic-fallback']").exists()).toBe(true);

    // ok semantics on a complete configuration.
    for (const id of ["endpoint", "key", "model", "fallback"]) {
      expect(wrapper.find(`[data-testid='ai-diagnostic-${id}'] svg`).classes()).toContain(
        "text-success"
      );
    }

    // Read-only: no connection test, stream or prompt preview is ever sent.
    expect(calledRPCs.some((name) => /TestAI|StartAIStream|PreviewAIRequest/.test(name))).toBe(
      false
    );

    wrapper.unmount();
  });

  test("keeps warn and fail semantics for a missing key and a half-configured fallback", async () => {
    config = { ...config, keyAvailable: false };
    advanced = { ...advanced, fallbackBaseUrl: "https://fallback.example/v1" };
    const wrapper = mount(AiInsightsSection);
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-diagnostic-key'] svg").classes()).toContain(
      "text-warning"
    );
    expect(wrapper.find("[data-testid='ai-diagnostic-fallback'] svg").classes()).toContain(
      "text-danger"
    );

    wrapper.unmount();
  });

  test("exports a portable bundle without key material", async () => {
    let copied = "";
    Object.defineProperty(navigator, "clipboard", {
      value: {
        writeText: async (text: string) => {
          copied = text;
        },
      },
      configurable: true,
    });
    const wrapper = mount(AiInsightsSection);
    await flushPromises();

    await wrapper.find("[data-testid='ai-config-export']").trigger("click");
    await flushPromises();

    expect(copied).toContain("wgit.ai.config");
    expect(copied).toContain("https://api.openai.com/v1");
    expect(copied).not.toContain("abcd1234");
    expect(copied).not.toContain("keyFingerprint");
    expect(copied).not.toContain("/local/repo");
    expect(toasts.value.some((toast) => toast.tone === "success")).toBe(true);

    wrapper.unmount();
  });

  test("imports a bundle, keeps local repository overrides and emits changed", async () => {
    const wrapper = mount(AiInsightsSection);
    await flushPromises();

    const bundle = buildAIConfigBundle(
      { ...baseConfig(), baseUrl: "https://imported.example/v1", model: "imported-model" },
      { ...baseAdvanced(), monthlyBudgetUsd: 42 }
    );
    setImportFile(wrapper, serializeAIConfigBundle(bundle));
    await wrapper.find("[data-testid='ai-config-import-file']").trigger("change");
    await flushPromises();

    // The existing confirm gate is preserved before anything is written.
    expect(confirmState.open).toBe(true);
    expect(savedSettings).toBeNull();
    settleConfirm(true);
    await flushPromises();

    expect(savedSettings).not.toBeNull();
    expect(savedSettings!.baseUrl).toBe("https://imported.example/v1");
    expect(savedSettings!.apiKey).toBe("");
    expect(savedAdvanced).not.toBeNull();
    expect(savedAdvanced!.monthlyBudgetUsd).toBe(42);
    // Machine-local repository overrides are preserved, never imported.
    expect(savedAdvanced!.repoOverrides).toEqual({ "/local/repo": { model: "local-model" } });
    expect(wrapper.emitted("changed")).toHaveLength(1);
    expect(toasts.value.some((toast) => toast.tone === "success")).toBe(true);

    wrapper.unmount();
  });

  test("rejects a secret-bearing file before asking for confirmation", async () => {
    const wrapper = mount(AiInsightsSection);
    await flushPromises();

    const bundle = buildAIConfigBundle(baseConfig(), baseAdvanced());
    const raw = JSON.stringify({
      ...bundle,
      config: { ...bundle.config, apiKey: "sk-live-secret" },
    });
    setImportFile(wrapper, raw);
    await wrapper.find("[data-testid='ai-config-import-file']").trigger("change");
    await flushPromises();

    expect(confirmState.open).toBe(false);
    expect(savedSettings).toBeNull();
    expect(savedAdvanced).toBeNull();
    expect(wrapper.emitted("changed")).toBeUndefined();
    expect(toasts.value.some((toast) => toast.tone === "warning")).toBe(true);

    wrapper.unmount();
  });
});
