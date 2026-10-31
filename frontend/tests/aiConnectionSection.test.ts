/**
 * AiConnectionSection — settings redesign Task 6.
 *
 * The source assertions pin the leaf composition contract: form + saved
 * profiles + stored keys + the legacy localStorage migration banner, with no
 * duplicated status card, mode switch or manual connection form (those live in
 * `AiConnectionForm`). The mounted cases drive the real children against the
 * real aiService wrappers with only the Wails runtime mocked, covering the
 * legacy import/discard flows that moved out of `LocalAiSection`.
 */
import { beforeEach, describe, expect, mock, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flushPromises, mount } from "@vue/test-utils";
import "./domSetup";
import { t } from "../src/i18n";
import { confirmState, resetConfirm, settleConfirm } from "../src/composables/useConfirm";
import { resetToasts, toasts } from "../src/composables/useToast";
import { readLegacyLocalAiConfig } from "../src/services/localAiService";
import type { AIAdvancedConfig, AiConfigNoSecret } from "../src/services/aiService";

const LEGACY_KEY = "wgit.localai.config.v1";

const read = (path: string): string =>
  readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

const BASE_CONFIG: AiConfigNoSecret = {
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
  activeProfile: "",
  notice: "",
};

const BASE_ADVANCED: AIAdvancedConfig = {
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

let config: AiConfigNoSecret = { ...BASE_CONFIG };
let saveCalls: Array<Record<string, unknown>> = [];
let configLoads = 0;
let profileLoads = 0;
let keyLoads = 0;
let failSave = false;
let configGate: Promise<void> | null = null;
let releaseConfig: (() => void) | null = null;

function resetState(): void {
  config = { ...BASE_CONFIG };
  saveCalls = [];
  configLoads = 0;
  profileLoads = 0;
  keyLoads = 0;
  failSave = false;
  configGate = null;
  releaseConfig = null;
  localStorage.clear();
  resetConfirm();
  resetToasts();
}

async function respond(name: string, args: unknown[]): Promise<unknown> {
  switch (name) {
    case "main.AiService.GetAIConfig":
      configLoads += 1;
      if (configGate) await configGate;
      return { ...config };
    case "main.AiService.GetAIProviderCatalog":
      return { providers: [], models: [] };
    case "main.AiService.DetectAICLIs":
      return [];
    case "main.AiService.ListAIProfiles":
      profileLoads += 1;
      return [];
    case "main.AiService.ListAIKeys":
      keyLoads += 1;
      return [];
    case "main.AiService.GetAIAdvancedConfig":
      return { ...BASE_ADVANCED, profileNames: {} };
    case "main.AiService.SaveAISettings": {
      if (failSave) throw new Error("keychain denied");
      const payload = args[0] as Record<string, unknown>;
      saveCalls.push(payload);
      config = {
        ...config,
        mode: payload.mode as AiConfigNoSecret["mode"],
        baseUrl: String(payload.baseUrl ?? ""),
        model: String(payload.model ?? ""),
      };
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

const AiConnectionSection = (await import("../src/components/settings/AiConnectionSection.vue"))
  .default;
const AiConnectionForm = (await import("../src/components/settings/AiConnectionForm.vue")).default;
const AiProfilesCard = (await import("../src/components/settings/AiProfilesCard.vue")).default;
const AiKeysCard = (await import("../src/components/settings/AiKeysCard.vue")).default;

function seedLegacy(config: { endpoint?: string; model?: string; apiKey?: string }): void {
  localStorage.setItem(LEGACY_KEY, JSON.stringify(config));
}

beforeEach(resetState);

describe("AiConnectionSection (source)", () => {
  test("composes form, profiles, keys and legacy migration", () => {
    const src = read("../src/components/settings/AiConnectionSection.vue");
    for (const text of [
      "AiConnectionForm",
      "AiProfilesCard",
      "AiKeysCard",
      "localai-legacy-banner",
      "readLegacyLocalAiConfig",
      "clearLegacyLocalAiKey",
      "refreshKey",
    ]) {
      expect(src).toContain(text);
    }
    for (const testId of ["localai-legacy-import", "localai-legacy-discard"]) {
      expect(src).toContain(testId);
    }
  });

  test("does not duplicate the status card, mode switch or manual connection form", () => {
    const src = read("../src/components/settings/AiConnectionSection.vue");
    for (const text of [
      "localai-backend-status",
      "ai-mode-switch",
      "localai-save-btn",
      "localai-test-btn",
      "localai-endpoint-input",
      "localai-apikey-input",
      "LocalAiSection",
    ]) {
      expect(src).not.toContain(text);
    }
  });
});

describe("AiConnectionSection (mounted)", () => {
  test("passes refreshKey to all three children and reloads them when it changes", async () => {
    const wrapper = mount(AiConnectionSection, { props: { refreshKey: 0 } });
    await flushPromises();

    expect(wrapper.findComponent(AiConnectionForm).props("refreshKey")).toBe(0);
    expect(wrapper.findComponent(AiProfilesCard).props("refreshKey")).toBe(0);
    expect(wrapper.findComponent(AiKeysCard).props("refreshKey")).toBe(0);
    const keysBefore = keyLoads;
    const profilesBefore = profileLoads;

    await wrapper.setProps({ refreshKey: 1 });
    await flushPromises();

    expect(wrapper.findComponent(AiConnectionForm).props("refreshKey")).toBe(1);
    expect(wrapper.findComponent(AiProfilesCard).props("refreshKey")).toBe(1);
    expect(wrapper.findComponent(AiKeysCard).props("refreshKey")).toBe(1);
    expect(keyLoads).toBeGreaterThan(keysBefore);
    expect(profileLoads).toBeGreaterThan(profilesBefore);
  });

  test("re-emits changed from the form save and either card change", async () => {
    const wrapper = mount(AiConnectionSection);
    await flushPromises();

    wrapper.findComponent(AiConnectionForm).vm.$emit("saved");
    wrapper.findComponent(AiProfilesCard).vm.$emit("changed");
    wrapper.findComponent(AiKeysCard).vm.$emit("changed");
    await flushPromises();

    expect(wrapper.emitted("changed")).toHaveLength(3);
  });

  test("hides the legacy banner when localStorage has no legacy key", async () => {
    const wrapper = mount(AiConnectionSection);
    await flushPromises();
    expect(wrapper.find("[data-testid='localai-legacy-banner']").exists()).toBe(false);
  });

  test("imports the legacy key into the saved endpoint, clears the banner and emits changed", async () => {
    seedLegacy({
      endpoint: "https://api.openai.com/v1",
      model: "gpt-4o",
      apiKey: "sk-legacy",
    });
    const wrapper = mount(AiConnectionSection);
    await flushPromises();

    const banner = wrapper.find("[data-testid='localai-legacy-banner']");
    expect(banner.exists()).toBe(true);
    expect(banner.text()).toContain(t("settings.localai.legacy.title"));
    expect(banner.text()).toContain(t("settings.localai.legacy.target.note"));

    await wrapper.find("[data-testid='localai-legacy-import']").trigger("click");
    await flushPromises();

    expect(saveCalls).toHaveLength(1);
    expect(saveCalls[0]?.apiKey).toBe("sk-legacy");
    expect(saveCalls[0]?.mode).toBe("api");
    expect(saveCalls[0]?.baseUrl).toBe("https://api.openai.com/v1");
    expect(saveCalls[0]?.model).toBe("gpt-4o");
    // The plaintext key is gone; readLegacy returns null and the banner closes.
    expect(readLegacyLocalAiConfig()).toBeNull();
    expect(wrapper.find("[data-testid='localai-legacy-banner']").exists()).toBe(false);
    expect(wrapper.emitted("changed")).toHaveLength(1);
    expect(toasts.value.some((toast) => toast.text === t("settings.localai.legacy.imported"))).toBe(
      true
    );
  });

  test("falls back to the legacy endpoint/model when the saved config is empty", async () => {
    config = { ...BASE_CONFIG, baseUrl: "", model: "", hasApiKey: false, keyAvailable: false };
    seedLegacy({
      endpoint: "http://localhost:11434/v1",
      model: "llama3",
      apiKey: "sk-legacy",
    });
    const wrapper = mount(AiConnectionSection);
    await flushPromises();

    await wrapper.find("[data-testid='localai-legacy-import']").trigger("click");
    await flushPromises();

    expect(saveCalls).toHaveLength(1);
    expect(saveCalls[0]?.baseUrl).toBe("http://localhost:11434/v1");
    expect(saveCalls[0]?.model).toBe("llama3");
    expect(saveCalls[0]?.apiKey).toBe("sk-legacy");
  });

  test("discards after confirmation without forcing a connection refresh", async () => {
    seedLegacy({ endpoint: "https://api.openai.com/v1", model: "gpt-4o", apiKey: "sk-legacy" });
    const wrapper = mount(AiConnectionSection);
    await flushPromises();

    await wrapper.find("[data-testid='localai-legacy-discard']").trigger("click");
    expect(confirmState.open).toBe(true);
    settleConfirm(true);
    await flushPromises();

    expect(saveCalls).toHaveLength(0);
    expect(readLegacyLocalAiConfig()).toBeNull();
    expect(wrapper.find("[data-testid='localai-legacy-banner']").exists()).toBe(false);
    expect(wrapper.emitted("changed")).toBeUndefined();
    expect(toasts.value.some((toast) => toast.text === t("settings.localai.legacy.discarded"))).toBe(
      true
    );
  });

  test("keeps import disabled until the saved connection finishes loading", async () => {
    seedLegacy({ endpoint: "https://api.openai.com/v1", model: "gpt-4o", apiKey: "sk-legacy" });
    configGate = new Promise<void>((resolve) => {
      releaseConfig = resolve;
    });
    const wrapper = mount(AiConnectionSection);
    await flushPromises();

    const importButton = wrapper.find("[data-testid='localai-legacy-import']");
    expect(importButton.attributes("disabled")).toBeDefined();

    releaseConfig?.();
    configGate = null;
    await flushPromises();
    expect(importButton.attributes("disabled")).toBeUndefined();
  });

  test("a cancelled discard keeps the banner", async () => {
    seedLegacy({ endpoint: "https://api.openai.com/v1", model: "gpt-4o", apiKey: "sk-legacy" });
    const wrapper = mount(AiConnectionSection);
    await flushPromises();

    await wrapper.find("[data-testid='localai-legacy-discard']").trigger("click");
    settleConfirm(false);
    await flushPromises();

    expect(readLegacyLocalAiConfig()).not.toBeNull();
    expect(wrapper.find("[data-testid='localai-legacy-banner']").exists()).toBe(true);
    expect(wrapper.emitted("changed")).toBeUndefined();
  });

  test("disables import in CLI mode and refuses a forced click", async () => {
    config = { ...BASE_CONFIG, mode: "cli", baseUrl: "", model: "", cliCommand: "claude" };
    seedLegacy({ endpoint: "https://api.openai.com/v1", model: "gpt-4o", apiKey: "sk-legacy" });
    const wrapper = mount(AiConnectionSection);
    await flushPromises();

    const banner = wrapper.find("[data-testid='localai-legacy-banner']");
    const importButton = banner.find("[data-testid='localai-legacy-import']");
    expect(importButton.attributes("disabled")).toBeDefined();
    expect(banner.text()).toContain(t("settings.localai.legacy.climode"));

    // The guard also runs in the handler, not just via the disabled attribute.
    (importButton.element as HTMLButtonElement).disabled = false;
    await importButton.trigger("click");
    await flushPromises();

    expect(saveCalls).toHaveLength(0);
    expect(readLegacyLocalAiConfig()).not.toBeNull();
    expect(
      toasts.value.some((toast) => toast.text === t("settings.localai.legacy.climode"))
    ).toBe(true);
  });

  test("a failed import keeps the banner and toasts danger", async () => {
    failSave = true;
    seedLegacy({ endpoint: "https://api.openai.com/v1", model: "gpt-4o", apiKey: "sk-legacy" });
    const wrapper = mount(AiConnectionSection);
    await flushPromises();

    await wrapper.find("[data-testid='localai-legacy-import']").trigger("click");
    await flushPromises();

    expect(readLegacyLocalAiConfig()).not.toBeNull();
    expect(wrapper.find("[data-testid='localai-legacy-banner']").exists()).toBe(true);
    expect(wrapper.emitted("changed")).toBeUndefined();
    expect(toasts.value.find((toast) => toast.tone === "danger")?.text).toContain(
      "keychain denied"
    );
  });
});
