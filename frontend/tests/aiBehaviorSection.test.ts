/**
 * AiBehaviorSection — settings redesign Task 7.
 *
 * The behavior leaf owns commit generation preferences (same SaveAISettings
 * surface as the connection editor), task routing and reliability (advanced
 * config) plus the five immediate-save feature flags. Source assertions pin
 * the extraction contract; mounted cases drive the real components with only
 * the Wails runtime mocked (house pattern: aiProfilesKeys.test.ts).
 */
import { beforeEach, describe, expect, mock, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flushPromises, mount } from "@vue/test-utils";
import "./domSetup";
import { resetToasts, toasts } from "../src/composables/useToast";
import type {
  AIAdvancedConfig,
  AiConfigNoSecret,
  AISaveInput,
} from "../src/services/aiService";
import type { FeatureFlags } from "../src/types";

const read = (path: string): string =>
  readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

const TASK_KINDS = [
  "commit",
  "summary",
  "conflict",
  "explain",
  "chat",
  "review",
  "release",
] as const;

function baseAdvanced(): AIAdvancedConfig {
  return {
    taskModels: {
      commit: "gpt-4o-mini",
      summary: "gpt-4o-mini",
      conflict: "gpt-4o",
      explain: "gpt-4o",
      chat: "gpt-4o",
      review: "claude-sonnet-4-5",
      release: "gpt-4o",
    },
    repoOverrides: { "/repo/a": { model: "repo-model" } },
    profileNames: { p1: "Production" },
    inputUsdPerMTok: 2.5,
    outputUsdPerMTok: 10,
    monthlyBudgetUsd: 20,
    retryCount: 2,
    rateLimitPerMinute: 30,
    fallbackBaseUrl: "https://fallback.example/v1",
    fallbackModel: "fallback-model",
    fallbackProtocol: "anthropic",
  };
}

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

function baseFlags(): FeatureFlags {
  return {
    aiCommitMsg: true,
    aiSummary: true,
    aiConflictBatch: true,
    reviewFlow: true,
    assetBoard: false,
  };
}

const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T;

let advanced: AIAdvancedConfig = baseAdvanced();
let config: AiConfigNoSecret = baseConfig();
let flags: FeatureFlags = baseFlags();
let savedAdvanced: AIAdvancedConfig | null = null;
let savedSettings: AISaveInput | null = null;
let savedFlags: FeatureFlags | null = null;
let advancedLoads = 0;
let advancedSaves = 0;
let settingsWrites = 0;
let flagLoads = 0;
let failAdvanced = false;
let failAdvancedSave = false;
let failConfig = false;
let failSettings = false;
let failFlags = false;
let failFlagSave = false;

function resetState(): void {
  advanced = baseAdvanced();
  config = baseConfig();
  flags = baseFlags();
  savedAdvanced = null;
  savedSettings = null;
  savedFlags = null;
  advancedLoads = 0;
  advancedSaves = 0;
  settingsWrites = 0;
  flagLoads = 0;
  failAdvanced = false;
  failAdvancedSave = false;
  failConfig = false;
  failSettings = false;
  failFlags = false;
  failFlagSave = false;
  resetToasts();
}

async function respond(name: string, args: unknown[]): Promise<unknown> {
  switch (name) {
    case "main.AiService.GetAIAdvancedConfig":
      advancedLoads += 1;
      if (failAdvanced) throw new Error("advanced down");
      return clone(advanced);
    case "main.AiService.GetAIConfig":
      if (failConfig) throw new Error("config down");
      return clone(config);
    case "main.AiService.SaveAIAdvancedConfig": {
      advancedSaves += 1;
      if (failAdvancedSave) throw new Error("advanced save down");
      const payload = args[0] as AIAdvancedConfig;
      savedAdvanced = clone(payload);
      advanced = clone(payload);
      return null;
    }
    case "main.AiService.SaveAISettings": {
      settingsWrites += 1;
      if (failSettings) throw new Error("settings save down");
      const payload = args[0] as AISaveInput;
      savedSettings = clone(payload);
      config = {
        ...config,
        commitLanguage: payload.commitLanguage ?? "auto",
        commitIncludeBody: payload.commitIncludeBody ?? false,
        commitInstructions: payload.commitInstructions ?? "",
      };
      return null;
    }
    case "main.GitService.GetAppFlags":
      flagLoads += 1;
      if (failFlags) throw new Error("flags down");
      return clone(flags);
    case "main.GitService.SaveAppFlags": {
      if (failFlagSave) throw new Error("flags save down");
      savedFlags = clone(args[0] as FeatureFlags);
      flags = clone(savedFlags);
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

const AiBehaviorSection = (
  await import("../src/components/settings/AiBehaviorSection.vue")
).default;
const AiTaskRoutingCard = (
  await import("../src/components/settings/AiTaskRoutingCard.vue")
).default;
const AiReliabilityCard = (
  await import("../src/components/settings/AiReliabilityCard.vue")
).default;
const { featureFlags, resetFeatureFlagsForTest } = await import(
  "../src/composables/useFeatureFlags"
);

beforeEach(() => {
  resetState();
  resetFeatureFlagsForTest();
});

describe("AiBehaviorSection (source contract)", () => {
  test("keeps commit preferences, routing, reliability and five flags", () => {
    const src = read("../src/components/settings/AiBehaviorSection.vue");
    for (const text of [
      "getAIAdvancedConfig",
      "saveAIAdvancedConfig",
      "aiCommitMsg",
      "aiSummary",
      "aiConflictBatch",
      "reviewFlow",
      "assetBoard",
      "AiTaskRoutingCard",
      "AiReliabilityCard",
    ]) {
      expect(src).toContain(text);
    }
  });

  test("uses SettingsCard for every surface and owns the single save path", () => {
    for (const path of [
      "../src/components/settings/AiBehaviorSection.vue",
      "../src/components/settings/AiTaskRoutingCard.vue",
      "../src/components/settings/AiReliabilityCard.vue",
    ]) {
      expect(read(path)).toContain("SettingsCard");
    }
    const src = read("../src/components/settings/AiBehaviorSection.vue");
    expect(src).toContain('data-testid="ai-behavior-save"');
    expect(src).toContain('role="alert"');
    expect(src).toContain("saveAISettings");
    expect(src).toContain("getAIConfig");
    expect(src).toContain("featureFlagsApi.getFeatureFlags()");
    expect(src).toContain("featureFlagsApi.saveAppFlags({ ...flags })");
    expect(src).toContain("reviewFlow");
  });

  test("keeps the legacy test ids for commit fields and the flag rows", () => {
    const src = read("../src/components/settings/AiBehaviorSection.vue");
    for (const id of [
      "commit-prefs-card",
      "localai-commit-language",
      "localai-commit-body",
      "localai-commit-instructions",
      "feature-flags-card",
      "flags-row-",
      "flags-toggle-",
    ]) {
      expect(src).toContain(id);
    }
  });
});

describe("routing and reliability cards (source contract)", () => {
  test("routing card exposes every task kind with the legacy card id", () => {
    const src = read("../src/components/settings/AiTaskRoutingCard.vue");
    for (const kind of TASK_KINDS) {
      expect(src).toContain(kind);
    }
    expect(src).toContain("ai-task-models-card");
    expect(src).toContain("ai-section-routing");
  });

  test("reliability card keeps retry, rate limit and the fallback fields", () => {
    const src = read("../src/components/settings/AiReliabilityCard.vue");
    for (const field of [
      "retryCount",
      "rateLimitPerMinute",
      "fallbackBaseUrl",
      "fallbackModel",
      "fallbackProtocol",
      "ai-reliability-card",
      "ai-section-reliability",
    ]) {
      expect(src).toContain(field);
    }
  });

  test("cards stay presentational so the section owns loading and saving", () => {
    for (const path of [
      "../src/components/settings/AiTaskRoutingCard.vue",
      "../src/components/settings/AiReliabilityCard.vue",
    ]) {
      const src = read(path);
      expect(src).not.toContain("saveAIAdvancedConfig");
      expect(src).not.toContain("getAIAdvancedConfig");
    }
  });
});

describe("AiBehaviorSection (mounted)", () => {
  test("loads commit preferences, routing, reliability and all five flags", async () => {
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    expect(wrapper.find("[data-testid='commit-prefs-card']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-task-models-card']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-task-models-card']").attributes("id")).toBe(
      "ai-section-routing"
    );
    expect(wrapper.find("[data-testid='ai-reliability-card']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-reliability-card']").attributes("id")).toBe(
      "ai-section-reliability"
    );
    expect(wrapper.find("[data-testid='feature-flags-card']").exists()).toBe(true);

    for (const kind of TASK_KINDS) {
      const input = wrapper.find(`[data-testid='ai-task-model-${kind}']`);
      expect(input.exists()).toBe(true);
      expect((input.element as HTMLInputElement).value).toBe(advanced.taskModels[kind]!);
    }
    expect(
      (
        wrapper.find("[data-testid='ai-reliability-card'] select").element as HTMLSelectElement
      ).value
    ).toBe("2");

    expect(
      (wrapper.find("[data-testid='localai-commit-language']").element as HTMLSelectElement).value
    ).toBe("zh");
    expect(
      (wrapper.find("[data-testid='localai-commit-body']").element as HTMLInputElement).checked
    ).toBe(true);
    expect(
      (wrapper.find("[data-testid='localai-commit-instructions']").element as HTMLTextAreaElement)
        .value
    ).toBe("Keep it short");

    for (const key of Object.keys(flags) as Array<keyof FeatureFlags>) {
      const toggle = wrapper.find(`[data-testid='flags-toggle-${key}']`);
      expect(toggle.exists()).toBe(true);
      expect(toggle.attributes("aria-checked")).toBe(String(flags[key]));
    }
    expect(wrapper.find("[data-testid='ai-behavior-save']").exists()).toBe(true);
    expect(advancedLoads).toBe(1);
    expect(flagLoads).toBe(1);
  });

  test("saves commit preferences and advanced config in one action, preserving other owners", async () => {
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    // Another leaf (usage / profiles / connection) changes the fields this
    // section does not own after the initial load; saving must not roll them
    // back. The section re-reads both records right before writing.
    advanced.monthlyBudgetUsd = 99;
    advanced.repoOverrides = { "/repo/changed": { model: "new-model" } };
    advanced.profileNames = { p1: "Renamed" };
    config.baseUrl = "https://proxy.example/v1";

    await wrapper.find("[data-testid='ai-task-model-commit']").setValue("gpt-4.1-mini");
    await wrapper.find("[data-testid='localai-commit-language']").setValue("en");
    await wrapper.find("[data-testid='localai-commit-body']").setValue(false);
    await wrapper
      .find("[data-testid='localai-commit-instructions']")
      .setValue("  Use conventional scopes  ");
    await wrapper.find("[data-testid='localai-commit-instructions']").trigger("change");
    await wrapper.find("[data-testid='ai-behavior-save']").trigger("click");
    await flushPromises();

    expect(savedAdvanced).not.toBeNull();
    expect(savedAdvanced!.taskModels).toEqual({
      ...baseAdvanced().taskModels,
      commit: "gpt-4.1-mini",
    });
    expect(savedAdvanced!.retryCount).toBe(2);
    expect(savedAdvanced!.rateLimitPerMinute).toBe(30);
    expect(savedAdvanced!.fallbackBaseUrl).toBe("https://fallback.example/v1");
    expect(savedAdvanced!.fallbackModel).toBe("fallback-model");
    expect(savedAdvanced!.fallbackProtocol).toBe("anthropic");
    expect(savedAdvanced!.monthlyBudgetUsd).toBe(99);
    expect(savedAdvanced!.inputUsdPerMTok).toBe(2.5);
    expect(savedAdvanced!.outputUsdPerMTok).toBe(10);
    expect(savedAdvanced!.repoOverrides).toEqual({ "/repo/changed": { model: "new-model" } });
    expect(savedAdvanced!.profileNames).toEqual({ p1: "Renamed" });

    expect(savedSettings).not.toBeNull();
    expect(savedSettings!.commitLanguage).toBe("en");
    expect(savedSettings!.commitIncludeBody).toBe(false);
    expect(savedSettings!.commitInstructions).toBe("Use conventional scopes");
    expect(savedSettings!.baseUrl).toBe("https://proxy.example/v1");
    expect(savedSettings!.mode).toBe("api");
    expect(savedSettings!.model).toBe("gpt-4o");
    expect(savedSettings!.extraHeaders).toEqual({ "X-Title": "wgit" });
    // Empty key is the keep-key contract; this surface never touches the secret.
    expect(savedSettings!.apiKey).toBe("");

    expect(wrapper.emitted("changed")).toHaveLength(1);
    expect(toasts.value.some((toast) => toast.tone === "success")).toBe(true);
  });

  test("routing/reliability saves do not touch the AI settings write", async () => {
    // A fresh install has no usable connection yet; SaveAISettings would reject
    // an empty endpoint. A routing-only save must still go through.
    config = { ...config, baseUrl: "", model: "" };
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    const reliability = wrapper.find("[data-testid='ai-reliability-card']");
    await reliability.findAll("select")[0]!.setValue("0");
    await reliability.find("input[type='number']").setValue("45");
    await reliability.find("input[type='url']").setValue("https://backup.example/v1");
    await reliability.find("input[placeholder='fallback-model']").setValue("backup-model");
    await reliability.findAll("select")[1]!.setValue("gemini");

    await wrapper.find("[data-testid='ai-behavior-save']").trigger("click");
    await flushPromises();

    expect(savedAdvanced!.retryCount).toBe(0);
    expect(savedAdvanced!.rateLimitPerMinute).toBe(45);
    expect(savedAdvanced!.fallbackBaseUrl).toBe("https://backup.example/v1");
    expect(savedAdvanced!.fallbackModel).toBe("backup-model");
    expect(savedAdvanced!.fallbackProtocol).toBe("gemini");
    expect(savedSettings).toBeNull();
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  test("a flag toggle persists all five flags, refreshes the singleton and emits changed", async () => {
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    await wrapper.find("[data-testid='flags-toggle-assetBoard']").trigger("click");
    await flushPromises();

    expect(savedFlags).toEqual({ ...baseFlags(), assetBoard: true });
    expect(featureFlags.assetBoard).toBe(true);
    expect(featureFlags.reviewFlow).toBe(true);
    expect(wrapper.find("[data-testid='flags-toggle-assetBoard']").attributes("aria-checked")).toBe(
      "true"
    );
    expect(wrapper.emitted("changed")).toHaveLength(1);
  });

  test("a failed flag save rolls the toggle back, toasts danger and does not emit changed", async () => {
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    failFlagSave = true;
    await wrapper.find("[data-testid='flags-toggle-assetBoard']").trigger("click");
    await flushPromises();

    expect(wrapper.find("[data-testid='flags-toggle-assetBoard']").attributes("aria-checked")).toBe(
      "false"
    );
    expect(featureFlags.assetBoard).toBe(false);
    expect(wrapper.emitted("changed")).toBeUndefined();
    expect(toasts.value.some((toast) => toast.tone === "danger")).toBe(true);
  });

  test("a failed advanced save keeps the draft and reports failure without emitting", async () => {
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    await wrapper.find("[data-testid='ai-task-model-commit']").setValue("gpt-4.1-mini");
    // Break the RPC after mount by deleting through an invalid fallback
    // protocol would require a live backend; instead fail the re-read.
    failAdvanced = true;
    await wrapper.find("[data-testid='ai-behavior-save']").trigger("click");
    await flushPromises();

    expect(savedAdvanced).toBeNull();
    expect(savedSettings).toBeNull();
    expect(wrapper.emitted("changed")).toBeUndefined();
    expect((wrapper.find("[data-testid='ai-task-model-commit']").element as HTMLInputElement).value)
      .toBe("gpt-4.1-mini");
    expect(toasts.value.some((toast) => toast.tone === "danger")).toBe(true);
  });

  test("shows a role=alert load failure and retries", async () => {
    failAdvanced = true;
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    const banner = wrapper.find("[data-testid='ai-behavior-load-error']");
    expect(banner.exists()).toBe(true);
    expect(banner.attributes("role")).toBe("alert");
    expect(banner.text()).toContain("advanced down");
    expect(
      wrapper.find("[data-testid='ai-behavior-save']").attributes("disabled")
    ).toBeDefined();

    failAdvanced = false;
    await wrapper.find("[data-testid='ai-behavior-retry']").trigger("click");
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-behavior-load-error']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='ai-task-model-commit']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-behavior-save']").attributes("disabled")).toBeUndefined();
    expect(advancedLoads).toBe(2);
  });

  test("a failed flags load shows an alert, hides the switches and recovers on retry", async () => {
    failFlags = true;
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    const alert = wrapper.find("[data-testid='ai-behavior-flags-error']");
    expect(alert.exists()).toBe(true);
    expect(alert.attributes("role")).toBe("alert");
    expect(alert.text()).toContain("flags down");

    // No fabricated default is presented as the authoritative switch state:
    // the rows only appear once real flags arrive.
    expect(wrapper.find("[data-testid='feature-flags-card']").findAll("[role='switch']")).toHaveLength(0);
    expect(wrapper.find("[data-testid='flags-toggle-aiCommitMsg']").exists()).toBe(false);

    // The rest of the leaf still works: routing loaded and save stays enabled.
    expect(
      (wrapper.find("[data-testid='ai-task-model-commit']").element as HTMLInputElement).value
    ).toBe("gpt-4o-mini");
    expect(wrapper.find("[data-testid='ai-behavior-save']").attributes("disabled")).toBeUndefined();

    failFlags = false;
    await wrapper.find("[data-testid='ai-behavior-flags-retry']").trigger("click");
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-behavior-flags-error']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='flags-toggle-aiCommitMsg']").attributes("aria-checked")).toBe(
      "true"
    );
    expect(wrapper.find("[data-testid='flags-toggle-assetBoard']").attributes("aria-checked")).toBe(
      "false"
    );
    expect(flagLoads).toBe(2);
  });

  test("a config-only failure keeps routing, reliability and flags usable", async () => {
    failConfig = true;
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    const inline = wrapper.find("[data-testid='ai-behavior-config-error']");
    expect(inline.exists()).toBe(true);
    expect(inline.attributes("role")).toBe("alert");
    expect(inline.text()).toContain("config down");
    // A config-only failure must not be reported as an advanced-settings failure.
    expect(wrapper.find("[data-testid='ai-behavior-load-error']").exists()).toBe(false);

    // The successful advanced load still renders routing/reliability.
    expect(
      (wrapper.find("[data-testid='ai-task-model-commit']").element as HTMLInputElement).value
    ).toBe("gpt-4o-mini");
    expect(
      (
        wrapper.find("[data-testid='ai-reliability-card'] select").element as HTMLSelectElement
      ).value
    ).toBe("2");

    // Commit preferences are unknown, so their inputs are disabled rather than
    // presenting fabricated defaults, and the settings write is skipped.
    expect(wrapper.find("[data-testid='localai-commit-language']").attributes("disabled")).toBeDefined();
    expect(wrapper.find("[data-testid='localai-commit-body']").attributes("disabled")).toBeDefined();
    expect(wrapper.find("[data-testid='localai-commit-instructions']").attributes("disabled")).toBeDefined();

    // Flags are independent and remain usable.
    expect(wrapper.find("[data-testid='flags-toggle-reviewFlow']").attributes("aria-checked")).toBe(
      "true"
    );
    expect(wrapper.find("[data-testid='flags-toggle-reviewFlow']").attributes("disabled")).toBeUndefined();

    // The save action stays available for routing/reliability.
    expect(wrapper.find("[data-testid='ai-behavior-save']").attributes("disabled")).toBeUndefined();

    await wrapper.find("[data-testid='ai-task-model-commit']").setValue("gpt-4.1-mini");
    await wrapper.find("[data-testid='ai-behavior-save']").trigger("click");
    await flushPromises();

    expect(savedAdvanced!.taskModels.commit).toBe("gpt-4.1-mini");
    expect(savedSettings).toBeNull();
    expect(settingsWrites).toBe(0);
    expect(wrapper.emitted("changed")).toHaveLength(1);

    // Recovering the config load re-enables the commit fields with real values.
    failConfig = false;
    await wrapper.find("[data-testid='ai-behavior-config-retry']").trigger("click");
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-behavior-config-error']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='localai-commit-language']").attributes("disabled")).toBeUndefined();
    expect(
      (wrapper.find("[data-testid='localai-commit-language']").element as HTMLSelectElement).value
    ).toBe("zh");
    expect(
      (wrapper.find("[data-testid='localai-commit-instructions']").element as HTMLTextAreaElement)
        .value
    ).toBe("Keep it short");
  });

  test("a failed settings write never reaches the advanced config write", async () => {
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    await wrapper.find("[data-testid='localai-commit-language']").setValue("en");
    failSettings = true;
    await wrapper.find("[data-testid='ai-behavior-save']").trigger("click");
    await flushPromises();

    expect(settingsWrites).toBe(1);
    expect(savedSettings).toBeNull();
    // Commit preferences are written first; when that fails nothing else is
    // persisted, so the UI never reports total failure over a partial write.
    expect(advancedSaves).toBe(0);
    expect(savedAdvanced).toBeNull();
    expect(wrapper.emitted("changed")).toBeUndefined();
    expect(toasts.value.some((toast) => toast.tone === "danger")).toBe(true);
    expect(
      (wrapper.find("[data-testid='localai-commit-language']").element as HTMLSelectElement).value
    ).toBe("en");
  });

  test("an advanced failure after a settings success emits changed and reports the error", async () => {
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();

    await wrapper.find("[data-testid='localai-commit-language']").setValue("en");
    failAdvancedSave = true;
    await wrapper.find("[data-testid='ai-behavior-save']").trigger("click");
    await flushPromises();

    // Commit preferences did persist, so the shell must still refresh.
    expect(savedSettings).not.toBeNull();
    expect(savedSettings!.commitLanguage).toBe("en");
    expect(advancedSaves).toBe(1);
    expect(savedAdvanced).toBeNull();
    expect(wrapper.emitted("changed")).toHaveLength(1);
    expect(toasts.value.some((toast) => toast.tone === "danger")).toBe(true);
  });

  test("refreshKey reloads advanced config and flags", async () => {
    const wrapper = mount(AiBehaviorSection);
    await flushPromises();
    expect(
      (wrapper.find("[data-testid='ai-task-model-commit']").element as HTMLInputElement).value
    ).toBe("gpt-4o-mini");

    advanced = { ...advanced, taskModels: { ...advanced.taskModels, commit: "reloaded-model" } };
    flags = { ...flags, assetBoard: true };
    await wrapper.setProps({ refreshKey: 1 });
    await flushPromises();

    expect(
      (wrapper.find("[data-testid='ai-task-model-commit']").element as HTMLInputElement).value
    ).toBe("reloaded-model");
    expect(wrapper.find("[data-testid='flags-toggle-assetBoard']").attributes("aria-checked")).toBe(
      "true"
    );
    expect(advancedLoads).toBe(2);
    expect(flagLoads).toBe(2);
  });
});
