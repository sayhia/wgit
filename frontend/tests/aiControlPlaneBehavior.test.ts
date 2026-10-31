import { beforeEach, describe, expect, mock, test } from "bun:test";
import { flushPromises, mount } from "@vue/test-utils";
import "./domSetup";
import type {
  AIAdvancedConfig,
  AIKeyEntry,
  AIProfile,
  AIProviderCatalog,
  AiConfigNoSecret,
} from "../src/services/aiService";

const CONFIG: AiConfigNoSecret = {
  mode: "api",
  baseUrl: "https://api.openai.com/v1",
  model: "gpt-4o",
  timeoutSeconds: 60,
  cliCommand: "",
  cliArgs: [],
  cliInput: "stdin",
  commitLanguage: "auto",
  commitIncludeBody: false,
  commitInstructions: "",
  protocol: "openai",
  azureApiVersion: "2024-10-21",
  extraHeaders: {},
  cliEnvMode: "inherit",
  hasApiKey: true,
  keyAvailable: true,
  keyFingerprint: "abcd1234",
  activeProfile: "p1",
  notice: "",
};

const CATALOG: AIProviderCatalog = {
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

const PROFILES: AIProfile[] = [
  {
    id: "p1",
    name: "Auto A",
    mode: "api",
    baseUrl: "https://api.openai.com/v1",
    model: "gpt-4o",
    protocol: "openai",
    active: true,
  },
];

const KEYS: AIKeyEntry[] = [
  { origin: "https://api.openai.com:443", fingerprint: "abcd1234", current: true, available: true },
];

function baseAdvanced(): AIAdvancedConfig {
  return {
    taskModels: {},
    repoOverrides: {},
    profileNames: {},
    inputUsdPerMTok: 2.5,
    outputUsdPerMTok: 10,
    monthlyBudgetUsd: 20,
    retryCount: 1,
    rateLimitPerMinute: 0,
    fallbackBaseUrl: "",
    fallbackModel: "",
    fallbackProtocol: "openai",
  };
}

let advanced: AIAdvancedConfig = baseAdvanced();
let profiles: AIProfile[] = [...PROFILES];
let savedAdvanced: AIAdvancedConfig | null = null;
let calledRPCs: string[] = [];
let flags = { aiCommitMsg: true, aiSummary: true, aiConflictBatch: true, reviewFlow: false, assetBoard: false };
let savedFlags: typeof flags | null = null;

async function respond(name: string, args: unknown[]): Promise<unknown> {
  calledRPCs.push(name);
  switch (name) {
    case "main.AiService.GetAIAdvancedConfig":
      return advanced;
    case "main.AiService.ListAIKeys":
      return KEYS;
    case "main.AiService.ListAIProfiles":
      return profiles;
    case "main.AiService.GetAIConfig":
      return CONFIG;
    case "main.AiService.GetAIProviderCatalog":
      return CATALOG;
    case "main.AiService.DetectAICLIs":
      return [];
    case "main.GitService.GetAppFlags":
      return { ...flags };
    case "main.GitService.SaveAppFlags":
      savedFlags = { ...(args[0] as typeof flags) };
      flags = { ...savedFlags };
      return null;
    case "main.AiService.SaveAIAdvancedConfig": {
      const payload = args[0] as AIAdvancedConfig;
      savedAdvanced = payload;
      advanced = { ...payload, profileNames: { ...(payload.profileNames || {}) } };
      profiles = profiles.map((profile) => ({
        ...profile,
        name: payload.profileNames?.[profile.id] || "Auto A",
      }));
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

const AiDiagnosticsCard = (await import("../src/components/settings/AiDiagnosticsCard.vue")).default;
const AiProfilesCard = (await import("../src/components/settings/AiProfilesCard.vue")).default;
const AiBehaviorSection = (await import("../src/components/settings/AiBehaviorSection.vue")).default;
const AiInsightsSection = (await import("../src/components/settings/AiInsightsSection.vue")).default;

beforeEach(() => {
  advanced = baseAdvanced();
  profiles = [{ ...PROFILES[0]! }];
  savedAdvanced = null;
  calledRPCs = [];
  flags = { aiCommitMsg: true, aiSummary: true, aiConflictBatch: true, reviewFlow: false, assetBoard: false };
  savedFlags = null;
});

describe("extracted AI settings diagnostics, behavior and profile rename", () => {
  test("renders local-only diagnostics without sending a connection test", async () => {
    const wrapper = mount(AiDiagnosticsCard);
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-diagnostics-card']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-diagnostic-endpoint']").text()).toContain(
      "https://api.openai.com/v1"
    );
    expect(wrapper.find("[data-testid='ai-diagnostic-key']").text()).toContain("系统钥匙串");
    expect(wrapper.find("[data-testid='ai-diagnostic-model']").text()).toContain("gpt-4o");
    expect(wrapper.find("[data-testid='ai-diagnostic-fallback']").text()).toContain("未启用");
    expect(calledRPCs.some((name) => name.includes("TestAI"))).toBe(false);
  });

  test("flags a half-configured fallback endpoint", async () => {
    advanced = { ...baseAdvanced(), fallbackBaseUrl: "https://fallback.example/v1" };
    const wrapper = mount(AiDiagnosticsCard);
    await flushPromises();
    expect(wrapper.find("[data-testid='ai-diagnostic-fallback']").text()).toContain("只填了一半");
  });

  test("behavior controls update task routing and flags through their real settings owner", async () => {
    const wrapper = mount(AiBehaviorSection, { attachTo: document.body });
    await flushPromises();
    try {
      expect(wrapper.find("[data-testid='ai-behavior-section']").exists()).toBe(true);
      for (const kind of ["commit", "summary", "conflict", "explain", "chat", "review", "release"]) {
        expect(wrapper.find(`[data-testid='ai-task-model-${kind}']`).exists()).toBe(true);
      }
      expect(wrapper.find("[data-testid='ai-reliability-card']").exists()).toBe(true);
      expect(wrapper.find("[data-testid='ai-task-models-card']").attributes("data-settings-card")).toBeDefined();
      const input = wrapper.find("[data-testid='ai-task-model-commit']");
      expect(input.classes()).toContain("ai-input");
      expect(wrapper.find(".ai-field-label").exists()).toBe(true);
      await input.setValue("commit-model");
      await wrapper.find("[data-testid='ai-behavior-save']").trigger("click");
      await flushPromises();
      expect(savedAdvanced?.taskModels.commit).toBe("commit-model");
      expect(savedAdvanced?.monthlyBudgetUsd).toBe(20);

      await wrapper.find("[data-testid='flags-toggle-aiSummary']").trigger("click");
      await flushPromises();
      expect(savedFlags?.aiSummary).toBe(false);
      expect(savedFlags?.aiCommitMsg).toBe(true);
      expect(savedFlags?.aiConflictBatch).toBe(true);
    } finally {
      wrapper.unmount();
    }
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
    expect(wrapper.find("[data-testid='ai-config-import']").exists()).toBe(true);
  });

  test("renames a profile through the advanced config overlay and keeps the label", async () => {
    const wrapper = mount(AiProfilesCard);
    await flushPromises();
    expect(wrapper.text()).toContain("Auto A");

    await wrapper.find("[data-testid='ai-profile-rename']").trigger("click");
    const input = wrapper.find("[data-testid='ai-profile-rename-input']");
    expect(input.exists()).toBe(true);
    await input.setValue("Production");
    await input.trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(savedAdvanced?.profileNames).toEqual({ p1: "Production" });
    expect(wrapper.text()).toContain("Production");
    expect(wrapper.find("[data-testid='ai-profile-rename-input']").exists()).toBe(false);
  });

  test("an empty name removes the override and restores the automatic label", async () => {
    advanced = { ...baseAdvanced(), profileNames: { p1: "Temporary" } };
    profiles = [{ ...PROFILES[0]!, name: "Temporary" }];
    const wrapper = mount(AiProfilesCard);
    await flushPromises();
    expect(wrapper.text()).toContain("Temporary");

    await wrapper.find("[data-testid='ai-profile-rename']").trigger("click");
    const input = wrapper.find("[data-testid='ai-profile-rename-input']");
    await input.setValue("");
    await input.trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(savedAdvanced?.profileNames).toEqual({});
    expect(wrapper.text()).toContain("Auto A");
  });
});
