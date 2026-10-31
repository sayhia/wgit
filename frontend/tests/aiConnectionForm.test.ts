/**
 * AiConnectionForm — mounted behavior for the settings connection editor.
 *
 * Adapted from tests/aiQuickSetupBehavior.test.ts for the settings redesign
 * (Task 3). The real component runs against the real aiService wrapper; only
 * the Wails runtime is mocked so each RPC can be made to fail, succeed, or
 * stay pending. Covers: initial skeleton, catalog/config failure banner +
 * retry recovery, key warning banner, ARIA/label wiring, and the base
 * preset/model/key fields in both full and compact modes.
 */
import { beforeEach, describe, expect, mock, spyOn, test } from "bun:test";
import { mount, flushPromises } from "@vue/test-utils";
import "./domSetup";
import { t } from "../src/i18n";
import { resetToasts, toasts } from "../src/composables/useToast";
import { confirmState, resetConfirm, settleConfirm } from "../src/composables/useConfirm";
import type {
  AICLIOption,
  AIKeyEntry,
  AIProviderCatalog,
  AiConfigNoSecret,
} from "../src/services/aiService";

const CATALOG: AIProviderCatalog = {
  providers: [
    {
      id: "openai",
      name: "OpenAI",
      mode: "api",
      protocol: "openai",
      baseUrl: "https://api.openai.com/v1",
      models: ["gpt-4o", "gpt-4o-mini"],
      defaultModel: "gpt-4o",
      keyRequired: true,
      local: false,
    },
    {
      id: "ollama",
      name: "Ollama",
      mode: "api",
      protocol: "openai",
      baseUrl: "http://127.0.0.1:11434/v1",
      models: ["llama3.2"],
      defaultModel: "llama3.2",
      keyRequired: false,
      local: true,
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
  activeProfile: "work",
  notice: "",
};

const KEYS: AIKeyEntry[] = [
  { origin: "https://api.openai.com:443", fingerprint: "abcd1234", current: true, available: true },
];

const CLIS: AICLIOption[] = [
  {
    id: "claude",
    name: "Claude Code",
    command: "claude",
    args: ["-p"],
    input: "stdin",
    version: "1.2.3",
    available: true,
  },
];

let failCatalog = false;
let failConfig = false;
let failKeys = false;
let failCLIs = false;
let config: AiConfigNoSecret = CONFIG;
let loadGate: Promise<void> | null = null;
let releaseGate: (() => void) | null = null;

function resetState(): void {
  failCatalog = false;
  failConfig = false;
  failKeys = false;
  failCLIs = false;
  config = CONFIG;
  loadGate = null;
  releaseGate = null;
  resetToasts();
  resetConfirm();
}

/** CLI-mode config variant; `savePayload` carries args/input verbatim. */
function cliConfig(overrides: Partial<AiConfigNoSecret> = {}): AiConfigNoSecret {
  return {
    ...CONFIG,
    mode: "cli",
    baseUrl: "",
    model: "claude-sonnet-4-5",
    cliCommand: "claude",
    cliArgs: ["-p"],
    cliInput: "stdin",
    ...overrides,
  };
}

async function respond(name: string): Promise<unknown> {
  if (loadGate) await loadGate;
  switch (name) {
    case "main.AiService.GetAIProviderCatalog":
      if (failCatalog) throw new Error("catalog down");
      return CATALOG;
    case "main.AiService.GetAIConfig":
      if (failConfig) throw new Error("config down");
      return config;
    case "main.AiService.ListAIKeys":
      if (failKeys) throw new Error("keys down");
      return KEYS;
    case "main.AiService.DetectAICLIs":
      if (failCLIs) throw new Error("cli detection down");
      return CLIS;
    default:
      return null;
  }
}

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByName: (name: string) => respond(name),
    ByID: () => Promise.resolve(null),
  },
  Events: { On: () => () => {}, Emit: () => {}, Off: () => {} },
  Window: { Name: () => "test" },
  Screens: {},
}));

const aiService = await import("../src/services/aiService");
const AiConnectionForm = (await import("../src/components/settings/AiConnectionForm.vue")).default;

function holdLoad(): void {
  loadGate = new Promise<void>((resolve) => {
    releaseGate = resolve;
  });
}

beforeEach(resetState);

describe("AiConnectionForm resilience and a11y (mounted)", () => {
  test("renders preset, model and key fields", async () => {
    const wrapper = mount(AiConnectionForm);
    await flushPromises();
    expect(wrapper.find("[data-testid='ai-quick-preset-openai']").exists()).toBe(true);
    expect(wrapper.find("#ai-quick-model-input").exists()).toBe(true);
    expect(wrapper.find("#ai-quick-key-input").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-quick-save']").exists()).toBe(true);
  });

  test("compact mode keeps the preset, model and key fields", async () => {
    const wrapper = mount(AiConnectionForm, { props: { compact: true } });
    await flushPromises();
    expect(wrapper.find("#ai-quick-model-input").exists()).toBe(true);
    expect(wrapper.find("#ai-quick-key-input").exists()).toBe(true);
  });

  test("shows a busy skeleton while the first catalog/config load is pending", async () => {
    holdLoad();
    const wrapper = mount(AiConnectionForm);
    await wrapper.vm.$nextTick();

    expect(wrapper.attributes("aria-busy")).toBe("true");
    expect(wrapper.find("[data-testid='ai-quick-loading']").exists()).toBe(true);

    releaseGate?.();
    loadGate = null;
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-quick-loading']").exists()).toBe(false);
    expect(wrapper.attributes("aria-busy")).toBe("false");
  });

  test("renders an alert when catalog and config fail, then recovers via retry", async () => {
    failCatalog = true;
    failConfig = true;
    const wrapper = mount(AiConnectionForm);
    await flushPromises();

    const banner = wrapper.find("[data-testid='ai-quick-load-error']");
    expect(banner.exists()).toBe(true);
    expect(banner.attributes("role")).toBe("alert");
    expect(banner.text()).toContain("catalog down");
    expect(banner.text()).toContain("config down");
    expect(wrapper.find("[data-testid='ai-quick-load-warning']").exists()).toBe(false);

    failCatalog = false;
    failConfig = false;
    await wrapper.find("[data-testid='ai-quick-retry']").trigger("click");
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-quick-load-error']").exists()).toBe(false);
    expect(wrapper.find("[data-testid='ai-quick-preset-openai']").exists()).toBe(true);
    expect(wrapper.find("[data-testid='ai-quick-preset-ollama']").exists()).toBe(true);
  });

  test("degrades to a status warning when keys fail", async () => {
    failKeys = true;
    const wrapper = mount(AiConnectionForm);
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-quick-load-error']").exists()).toBe(false);
    const warning = wrapper.find("[data-testid='ai-quick-load-warning']");
    expect(warning.exists()).toBe(true);
    expect(warning.attributes("role")).toBe("status");
    expect(warning.text()).toContain("keys down");
    // Catalog and config survived, so the provider picker is usable.
    expect(wrapper.find("[data-testid='ai-quick-preset-openai']").exists()).toBe(true);
  });

  test("exposes pressed state, group labels and label/input associations", async () => {
    const wrapper = mount(AiConnectionForm);
    await flushPromises();

    expect(wrapper.find("[data-testid='ai-quick-mode-switch']").attributes("role")).toBe("group");
    expect(wrapper.find("[data-testid='ai-quick-mode-api']").attributes("aria-pressed")).toBe("true");
    expect(wrapper.find("[data-testid='ai-quick-mode-cli']").attributes("aria-pressed")).toBe("false");
    expect(wrapper.find("[data-testid='ai-quick-preset-openai']").attributes("aria-pressed")).toBe("true");
    expect(wrapper.find("[data-testid='ai-quick-preset-ollama']").attributes("aria-pressed")).toBe("false");

    const modelInput = wrapper.find("#ai-quick-model-input");
    expect(modelInput.exists()).toBe(true);
    expect(wrapper.find("label[for='ai-quick-model-input']").exists()).toBe(true);
    expect(wrapper.find("label[for='ai-quick-key-input']").exists()).toBe(true);

    await wrapper.find("[data-testid='ai-quick-preset-ollama']").trigger("click");
    expect(wrapper.find("[data-testid='ai-quick-preset-ollama']").attributes("aria-pressed")).toBe("true");
    expect(wrapper.find("[data-testid='ai-quick-preset-openai']").attributes("aria-pressed")).toBe("false");
    expect(wrapper.find("#ai-quick-endpoint-input").exists()).toBe(false);
  });

  test("keeps endpoint and command inputs labelled in custom API/CLI modes", async () => {
    const wrapper = mount(AiConnectionForm);
    await flushPromises();

    await wrapper.find("[data-testid='ai-quick-preset-custom']").trigger("click");
    await wrapper.vm.$nextTick();
    expect(wrapper.find("#ai-quick-endpoint-input").exists()).toBe(true);
    expect(wrapper.find("label[for='ai-quick-endpoint-input']").exists()).toBe(true);
    expect(wrapper.find("label[for='ai-quick-model-input']").exists()).toBe(true);

    await wrapper.find("[data-testid='ai-quick-mode-cli']").trigger("click");
    await wrapper.vm.$nextTick();
    expect(wrapper.find("[data-testid='ai-quick-mode-cli']").attributes("aria-pressed")).toBe("true");
    expect(wrapper.find("[data-testid='ai-quick-preset-cli-claude']").exists()).toBe(true);

    await wrapper.find("[data-testid='ai-quick-preset-customcli']").trigger("click");
    await wrapper.vm.$nextTick();
    expect(wrapper.find("#ai-quick-command-input").exists()).toBe(true);
    expect(wrapper.find("label[for='ai-quick-command-input']").exists()).toBe(true);
  });
});

describe("AiConnectionForm advanced connection (mounted)", () => {
  test("advanced connection fields are editable and saved", async () => {
    const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue(undefined);
    try {
      const wrapper = mount(AiConnectionForm);
      await flushPromises();
      await wrapper.find("[data-testid='ai-connection-advanced'] summary").trigger("click");
      await wrapper.find("[data-testid='ai-connection-protocol']").setValue("anthropic");
      await wrapper.find("[data-testid='ai-connection-timeout']").setValue("90");
      await wrapper.find("[data-testid='ai-connection-headers']").setValue("X-Test: 1");
      await wrapper.find("[data-testid='ai-quick-save-only']").trigger("click");
      await flushPromises();
      expect(saveSpy).toHaveBeenCalled();
      const payload = saveSpy.mock.calls[0]![0] as unknown as Record<string, unknown>;
      expect(payload.protocol).toBe("anthropic");
      expect(payload.timeoutSeconds).toBe(90);
      expect(payload.extraHeaders).toEqual({ "X-Test": "1" });
      expect(payload.cliArgs).toEqual([]);
      expect(payload.cliInput).toBe("stdin");
    } finally {
      saveSpy.mockRestore();
    }
  });

  test("compact mode hides the advanced connection disclosure", async () => {
    const wrapper = mount(AiConnectionForm, { props: { compact: true } });
    await flushPromises();
    expect(wrapper.find("[data-testid='ai-connection-advanced']").exists()).toBe(false);
  });

  test("keeps saved CLI args verbatim when the text field is untouched", async () => {
    config = cliConfig({ cliArgs: ["--flag", "a b", "--mode=fast"], cliInput: "arg" });
    const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue(undefined);
    try {
      const wrapper = mount(AiConnectionForm);
      await flushPromises();
      await wrapper.find("[data-testid='ai-connection-advanced'] summary").trigger("click");
      const argsInput = wrapper.find("[data-testid='ai-connection-cli-args']");
      expect((argsInput.element as HTMLInputElement).value).toBe("--flag a b --mode=fast");

      await wrapper.find("[data-testid='ai-quick-save-only']").trigger("click");
      await flushPromises();

      expect(saveSpy).toHaveBeenCalledTimes(1);
      const payload = saveSpy.mock.calls[0]![0] as unknown as Record<string, unknown>;
      expect(payload.cliArgs).toEqual(["--flag", "a b", "--mode=fast"]);
      expect(payload.cliInput).toBe("arg");
    } finally {
      saveSpy.mockRestore();
    }
  });

  test("tokenizes edited CLI args with quotes instead of splitting on whitespace", async () => {
    config = cliConfig();
    const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue(undefined);
    try {
      const wrapper = mount(AiConnectionForm);
      await flushPromises();
      await wrapper.find("[data-testid='ai-connection-advanced'] summary").trigger("click");
      await wrapper.find("[data-testid='ai-connection-cli-args']").setValue('--flag "a b"');
      await wrapper.find("[data-testid='ai-quick-save-only']").trigger("click");
      await flushPromises();

      expect(saveSpy).toHaveBeenCalledTimes(1);
      const payload = saveSpy.mock.calls[0]![0] as unknown as Record<string, unknown>;
      expect(payload.cliArgs).toEqual(["--flag", "a b"]);
    } finally {
      saveSpy.mockRestore();
    }
  });

  test("warns and blocks an API save with malformed header lines", async () => {
    const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue(undefined);
    try {
      const wrapper = mount(AiConnectionForm);
      await flushPromises();
      await wrapper.find("[data-testid='ai-connection-advanced'] summary").trigger("click");
      await wrapper.find("[data-testid='ai-connection-headers']").setValue("not-a-header");
      await wrapper.find("[data-testid='ai-quick-save-only']").trigger("click");
      await flushPromises();

      expect(saveSpy).not.toHaveBeenCalled();
      expect(
        toasts.value.some(
          (toast) =>
            toast.tone === "warning" &&
            toast.text === t("settings.localai.headers.lines.invalid")
        )
      ).toBe(true);
    } finally {
      saveSpy.mockRestore();
    }
  });

  test("CLI options omit API fields and persist the selected environment", async () => {
    config = cliConfig();
    const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue(undefined);
    try {
      const wrapper = mount(AiConnectionForm);
      await flushPromises();
      await wrapper.find("[data-testid='ai-connection-advanced'] summary").trigger("click");
      expect(wrapper.find("[data-testid='ai-connection-headers']").exists()).toBe(false);
      expect(wrapper.find("[data-testid='ai-connection-protocol']").exists()).toBe(false);
      expect(wrapper.find("[data-testid='localai-azure-version']").exists()).toBe(false);
      await wrapper.find("[data-testid='localai-cli-env']").setValue("minimal");
      await wrapper.find("[data-testid='ai-quick-save-only']").trigger("click");
      await flushPromises();

      expect(saveSpy).toHaveBeenCalledTimes(1);
      const payload = saveSpy.mock.calls[0]![0] as unknown as Record<string, unknown>;
      expect(payload.extraHeaders).toEqual({});
      expect(payload.cliArgs).toEqual(["-p"]);
      expect(payload.cliEnvMode).toBe("minimal");
    } finally {
      saveSpy.mockRestore();
    }
  });

  test("compact save preserves stored headers verbatim", async () => {
    config = { ...CONFIG, extraHeaders: { "X-Padded": " padded " } };
    const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue(undefined);
    try {
      const wrapper = mount(AiConnectionForm, { props: { compact: true } });
      await flushPromises();
      await wrapper.find("[data-testid='ai-quick-save-only']").trigger("click");
      await flushPromises();

      expect(saveSpy).toHaveBeenCalledTimes(1);
      const payload = saveSpy.mock.calls[0]![0] as unknown as Record<string, unknown>;
      expect(payload.extraHeaders).toEqual({ "X-Padded": " padded " });
    } finally {
      saveSpy.mockRestore();
    }
  });
});

describe("AiConnectionForm complete connection actions", () => {
  test("Azure version is editable and saving preserves current generation preferences", async () => {
    const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue(undefined);
    const wrapper = mount(AiConnectionForm);
    try {
      await flushPromises();
      expect(wrapper.find("[data-testid='localai-cli-env']").exists()).toBe(false);
      await wrapper.get("[data-testid='ai-connection-protocol']").setValue("azure");
      await wrapper.get("[data-testid='localai-azure-version']").setValue("2025-04-01-preview");
      config = { ...CONFIG, commitLanguage: "en", commitIncludeBody: true, commitInstructions: "Use the issue number", cliEnvMode: "minimal" };
      await wrapper.get("[data-testid='ai-quick-save-only']").trigger("click");
      await flushPromises();
      expect(saveSpy).toHaveBeenCalledWith(expect.objectContaining({
        protocol: "azure", azureApiVersion: "2025-04-01-preview",
        commitLanguage: "en", commitIncludeBody: true, commitInstructions: "Use the issue number", cliEnvMode: "minimal",
      }));
    } finally { wrapper.unmount(); saveSpy.mockRestore(); }
  });

  test("deep test saves the edited connection before sending a generation probe", async () => {
    const operations: string[] = [];
    const saveSpy = spyOn(aiService, "saveAISettings").mockImplementation(async () => { operations.push("save"); });
    const deepSpy = spyOn(aiService, "testAIDeepResult").mockImplementation(async () => {
      operations.push("deep");
      return { ok: true, mode: "api", message: "Generation probe succeeded" };
    });
    const shallowSpy = spyOn(aiService, "testAIConnectionResult").mockResolvedValue({ ok: true, mode: "api", message: "connected" });
    const wrapper = mount(AiConnectionForm);
    try {
      await flushPromises();
      await wrapper.get("#ai-quick-model-input").setValue("gpt-4o-mini");
      await wrapper.get("[data-testid='localai-deep-test-btn']").trigger("click");
      await flushPromises();
      expect(operations).toEqual(["save", "deep"]);
      expect(saveSpy).toHaveBeenCalledWith(expect.objectContaining({ model: "gpt-4o-mini" }));
      expect(shallowSpy).not.toHaveBeenCalled();
      expect(wrapper.get("[data-testid='ai-quick-test-result']").text()).toContain("Generation probe succeeded");
      expect(wrapper.get("[data-testid='ai-quick-test-result']").attributes("role")).toBe("status");
      expect(wrapper.emitted("saved")).toHaveLength(1);
    } finally { wrapper.unmount(); saveSpy.mockRestore(); deepSpy.mockRestore(); shallowSpy.mockRestore(); }
  });

  test("deep test does not send a request when saving fails", async () => {
    const saveSpy = spyOn(aiService, "saveAISettings").mockRejectedValue(new Error("write failed"));
    const deepSpy = spyOn(aiService, "testAIDeepResult").mockResolvedValue({ ok: true, mode: "api", message: "unused" });
    const wrapper = mount(AiConnectionForm);
    try {
      await flushPromises();
      await wrapper.get("[data-testid='localai-deep-test-btn']").trigger("click");
      await flushPromises();
      expect(deepSpy).not.toHaveBeenCalled();
      expect(wrapper.emitted("saved")).toBeUndefined();
      expect(toasts.value.some((toast) => toast.tone === "danger" && toast.text.includes("write failed"))).toBe(true);
    } finally { wrapper.unmount(); saveSpy.mockRestore(); deepSpy.mockRestore(); }
  });

  test("deep test exposes provider failure details and keeps them after a parent refresh", async () => {
    const saveSpy = spyOn(aiService, "saveAISettings").mockResolvedValue(undefined);
    const deepSpy = spyOn(aiService, "testAIDeepResult").mockResolvedValue({
      ok: false,
      mode: "api",
      message: "AI generation test failed",
      detail: 'AI endpoint returned HTTP 500: {"error":"provider failure"}',
    });
    const wrapper = mount(AiConnectionForm, { props: { refreshKey: 0 } });
    try {
      await flushPromises();
      await wrapper.get("[data-testid='localai-deep-test-btn']").trigger("click");
      await flushPromises();
      const result = wrapper.get("[data-testid='ai-quick-test-result']");
      expect(result.attributes("role")).toBe("alert");
      expect(result.text()).toContain("AI generation test failed");
      expect(result.text()).toContain("HTTP 500");
      expect(wrapper.emitted("saved")).toHaveLength(1);
      // SettingsView refreshes sibling profile/key cards after `saved`.
      await wrapper.setProps({ refreshKey: 1 });
      await flushPromises();
      expect(wrapper.get("[data-testid='ai-quick-test-result']").text()).toContain("HTTP 500");
      expect(deepSpy).toHaveBeenCalledTimes(1);
    } finally { wrapper.unmount(); saveSpy.mockRestore(); deepSpy.mockRestore(); }
  });

  test("saved key removal requires confirmation and refreshes the configured state", async () => {
    const clearSpy = spyOn(aiService, "clearAIKey").mockImplementation(async () => {
      config = { ...CONFIG, hasApiKey: false, keyAvailable: false, keyFingerprint: "" };
    });
    const wrapper = mount(AiConnectionForm);
    try {
      await flushPromises();
      await wrapper.get("[data-testid='localai-key-clear']").trigger("click");
      expect(confirmState.open).toBe(true);
      expect(clearSpy).not.toHaveBeenCalled();
      settleConfirm(false);
      await flushPromises();
      expect(clearSpy).not.toHaveBeenCalled();
      await wrapper.get("[data-testid='localai-key-clear']").trigger("click");
      settleConfirm(true);
      await flushPromises();
      expect(clearSpy).toHaveBeenCalledTimes(1);
      expect(wrapper.find("[data-testid='localai-key-clear']").exists()).toBe(false);
      expect(wrapper.emitted("saved")).toHaveLength(1);
    } finally { wrapper.unmount(); clearSpy.mockRestore(); }
  });

  test("key removal cannot clear a different saved endpoint after confirmation", async () => {
    const clearSpy = spyOn(aiService, "clearAIKey").mockResolvedValue(undefined);
    const wrapper = mount(AiConnectionForm);
    try {
      await flushPromises();
      await wrapper.get("[data-testid='localai-key-clear']").trigger("click");
      config = { ...CONFIG, baseUrl: "https://other.example/v1" };
      settleConfirm(true);
      await flushPromises();
      expect(clearSpy).not.toHaveBeenCalled();
      expect(toasts.value.some((toast) => toast.tone === "warning")).toBe(true);
      await wrapper.get("[data-testid='ai-quick-endpoint']").setValue("https://draft.example/v1");
      expect(wrapper.find("[data-testid='localai-key-clear']").exists()).toBe(false);
    } finally { wrapper.unmount(); clearSpy.mockRestore(); }
  });
});
