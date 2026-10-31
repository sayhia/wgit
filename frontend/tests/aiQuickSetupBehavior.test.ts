/**
 * Compact AiConnectionForm — mounted behavior for the P1-A resilience pass.
 *
 * The real component runs against the real aiService wrapper; only the Wails
 * runtime is mocked so each RPC can be made to fail, succeed, or stay pending.
 * Covers: initial skeleton, catalog/config failure banner + retry recovery,
 * profile/key warning banner, and the ARIA/label wiring added for a11y.
 */
import { beforeEach, describe, expect, mock, test } from "bun:test";
import { mount, flushPromises } from "@vue/test-utils";
import "./domSetup";
import { openAiSetup, closeAiSetup } from "../src/composables/useAiSetup";
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
let config = CONFIG;
let loadGate: Promise<void> | null = null;
let releaseGate: (() => void) | null = null;

function resetState(): void {
  failCatalog = false;
  failConfig = false;
  failKeys = false;
  failCLIs = false;
  config = CONFIG;
  closeAiSetup();
  loadGate = null;
  releaseGate = null;
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

const AiConnectionForm = (await import("../src/components/settings/AiConnectionForm.vue")).default;
const AiQuickSetupModal = (await import("../src/components/settings/AiQuickSetupModal.vue")).default;

function holdLoad(): void {
  loadGate = new Promise<void>((resolve) => {
    releaseGate = resolve;
  });
}

beforeEach(resetState);

describe("Compact AiConnectionForm resilience and a11y (mounted)", () => {
  test("the wizard mounts the shared editor once and dismisses from its footer", async () => {
    openAiSetup("Configure AI before generating");
    const wrapper = mount(AiQuickSetupModal);
    try {
      await flushPromises();
      expect(wrapper.findAllComponents(AiConnectionForm)).toHaveLength(1);
      expect(wrapper.findComponent(AiConnectionForm).props("compact")).toBe(true);
      expect(wrapper.find("[data-testid='ai-connection-advanced']").exists()).toBe(false);
      expect(wrapper.text()).toContain("Configure AI before generating");
      await wrapper.get("[data-testid='ai-quick-close']").trigger("click");
      expect(wrapper.find('[role="dialog"]').exists()).toBe(false);
    } finally {
      wrapper.unmount();
      closeAiSetup();
    }
  });

  test("retains the saved backend notice in the shared editor", async () => {
    config = { ...CONFIG, notice: "Using the fallback model" };
    const wrapper = mount(AiConnectionForm, { props: { compact: true } });
    try {
      await flushPromises();
      expect(wrapper.get("[data-testid='ai-connection-notice']").text()).toBe("Using the fallback model");
      expect(wrapper.get("[data-testid='ai-connection-notice']").attributes("role")).toBe("status");
    } finally {
      wrapper.unmount();
    }
  });

  test("shows a busy skeleton while the first catalog/config load is pending", async () => {
    holdLoad();
    const wrapper = mount(AiConnectionForm, { props: { compact: true } });
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
    const wrapper = mount(AiConnectionForm, { props: { compact: true } });
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
    const wrapper = mount(AiConnectionForm, { props: { compact: true } });
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
    const wrapper = mount(AiConnectionForm, { props: { compact: true } });
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
    const wrapper = mount(AiConnectionForm, { props: { compact: true } });
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
