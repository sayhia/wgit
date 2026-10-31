import { afterEach, describe, expect, spyOn, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import AiConnectionForm from "../src/components/settings/AiConnectionForm.vue";
import * as aiService from "../src/services/aiService";
import { resetToasts } from "../src/composables/useToast";
import { t } from "../src/i18n";

const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
const restores: Array<() => void> = [];
let wrapper: VueWrapper | null = null;

function stubConnection() {
  const config = {
    mode: "api", baseUrl: "https://api.example.com/v1", model: "a11y-test",
    timeoutSeconds: 60, protocol: "openai", cliCommand: "", cliArgs: [],
    cliInput: "stdin", extraHeaders: {}, hasApiKey: false, keyAvailable: false,
    commitLanguage: "auto", commitIncludeBody: false, commitInstructions: "",
  };
  const mocks = [
    spyOn(aiService, "getAIConfig").mockResolvedValue(config as aiService.AiConfigNoSecret),
    spyOn(aiService, "getAIProviderCatalog").mockResolvedValue({ providers: [], models: [] }),
    spyOn(aiService, "listAIKeys").mockResolvedValue([]),
    spyOn(aiService, "detectAICLIs").mockResolvedValue([]),
    spyOn(aiService, "saveAISettings").mockResolvedValue(undefined as never),
  ];
  restores.push(...mocks.map((mock) => () => mock.mockRestore()));
  const result = spyOn(aiService, "testAIConnectionResult");
  restores.push(() => result.mockRestore());
  return result;
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  restores.splice(0).forEach((restore) => restore());
  resetToasts();
});

describe("settings inline results announce to assistive tech", () => {
  test("active AI connection results switch between polite status and assertive alert", async () => {
    const result = stubConnection();
    result.mockResolvedValue({ ok: true, mode: "api", message: "Connected" });
    wrapper = mount(AiConnectionForm);
    await flushPromises();
    await wrapper.find('[data-testid="ai-quick-save"]').trigger("click");
    await flushPromises();
    let banner = wrapper.find('[data-testid="ai-quick-test-result"]');
    expect(banner.attributes("role")).toBe("status");
    expect(banner.attributes("aria-live")).toBe("polite");
    expect(banner.text()).toContain("Connected");

    result.mockResolvedValue({ ok: false, mode: "api", message: "Connection refused" });
    await wrapper.find('[data-testid="ai-quick-save"]').trigger("click");
    await flushPromises();
    banner = wrapper.find('[data-testid="ai-quick-test-result"]');
    expect(banner.attributes("role")).toBe("alert");
    expect(banner.attributes("aria-live")).toBe("assertive");
    expect(banner.text()).toContain("Connection refused");
  });

  test("connection transport errors are announced and fields keep visible accessible names", async () => {
    const result = stubConnection();
    result.mockRejectedValue(new Error("Transport unavailable"));
    wrapper = mount(AiConnectionForm);
    await flushPromises();
    expect(wrapper.find('label[for="ai-quick-key-input"]').text()).toBe(t("ai.quick.key"));
    expect(wrapper.find("#ai-quick-key-input").attributes("type")).toBe("password");
    expect(wrapper.find('label[for="ai-quick-model-input"]').text()).toBe(t("ai.quick.model"));
    expect(wrapper.find('[data-testid="ai-connection-timeout"]').element.closest("label")?.textContent).toContain(t("settings.localai.timeout.label"));
    // Mode controls retain readable names while their icons stay decorative.
    for (const mode of ["api", "cli"]) {
      const button = wrapper.find(`[data-testid="ai-quick-mode-${mode}"]`);
      expect(button.text()).toBeTruthy();
      expect(button.find("svg").attributes("aria-hidden")).toBe("true");
    }
    await wrapper.find('[data-testid="ai-quick-save"]').trigger("click");
    await flushPromises();
    const banner = wrapper.find('[data-testid="ai-quick-test-result"]');
    expect(banner.attributes("role")).toBe("alert");
    expect(banner.attributes("aria-live")).toBe("assertive");
    expect(banner.text()).toContain("Transport unavailable");
  });

  test("hook dry-run failures are announced as alerts", () => {
    const src = read("../src/components/settings/GitHooksSection.vue");
    expect(src).toContain(':role="runResult.success ? \'status\' : \'alert\'"');
  });

  test("shortcut import errors are alerts", () => {
    const src = read("../src/components/settings/ShortcutsSection.vue");
    expect(src).toMatch(/<p v-if="transferError"\s+role="alert"/);
  });
});
