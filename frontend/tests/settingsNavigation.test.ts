import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import SettingsView from "../src/components/SettingsView.vue";
import { locale, setLocale } from "../src/i18n";
import "./domSetup";

const NAV_KEY = "wgit.settings.active.v1";
const originalLocale = locale.value;
const pageNames = [
  "AppearanceSection", "IdentitySection", "SigningSection", "ProxySection",
  "ToolsSection", "AliasesSection", "GitHooksSection", "AdvancedConfigSection",
  "ShortcutsSection", "AiConnectionSection", "AiBehaviorSection", "AiInsightsSection", "AboutSection",
];
const stubs = Object.fromEntries(pageNames.map(name => [name, {
  template: `<div data-page="${name}"></div>`,
}]));
let wrapper: ReturnType<typeof mount> | undefined;

function openSettings() {
  wrapper = mount(SettingsView, { props: { currentRepoPath: "/test/repo" }, global: { stubs } });
  return wrapper;
}

beforeEach(() => {
  localStorage.removeItem(NAV_KEY);
  localStorage.removeItem("wgit.settings.searchHistory.v1");
  setLocale("zh-CN");
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  localStorage.removeItem(NAV_KEY);
  localStorage.removeItem("wgit.settings.searchHistory.v1");
  setLocale(originalLocale);
});

describe("settings navigation", () => {
  test("groups thirteen concise destinations and exposes one current page", () => {
    const view = openSettings();
    expect(view.findAll("[data-settings-nav] > section")).toHaveLength(4);
    const links = view.findAll("[data-settings-leaf]");
    expect(links).toHaveLength(13);
    expect(new Set(links.map(link => link.attributes("data-settings-leaf"))).size).toBe(13);
    expect(links.every(link => link.text().length <= 8)).toBe(true);
    expect(view.findAll('[data-settings-leaf][aria-current="page"]')).toHaveLength(1);
    expect(view.find('[data-page="AppearanceSection"]').exists()).toBe(true);
  });

  test("switches AI pages independently and remembers the chosen destination", async () => {
    const view = openSettings();
    await view.find('[data-settings-leaf="ai-behavior"]').trigger("click");
    expect(view.find('[data-page="AiBehaviorSection"]').exists()).toBe(true);
    expect(view.find('[data-page="AiConnectionSection"]').exists()).toBe(false);
    expect(localStorage.getItem(NAV_KEY)).toBe("ai-behavior");
    await view.find('[data-settings-leaf="ai-insights"]').trigger("click");
    expect(view.find('[data-page="AiInsightsSection"]').exists()).toBe(true);
    expect(view.findAll('[data-settings-leaf][aria-current="page"]')).toHaveLength(1);
  });

  test("restores a saved page and migrates the legacy AI destination", () => {
    localStorage.setItem(NAV_KEY, "localai");
    const view = openSettings();
    expect(view.find('[data-page="AiConnectionSection"]').exists()).toBe(true);
  });

  test("searching for budget and pressing Enter opens usage instead of a monolithic AI page", async () => {
    const view = openSettings();
    const search = view.find('input[type="text"]');
    await search.setValue("预算");
    await search.trigger("keydown", { key: "Enter" });
    expect(view.find('[data-page="AiInsightsSection"]').exists()).toBe(true);
    expect((search.element as HTMLInputElement).value).toBe("");
    expect(JSON.parse(localStorage.getItem("wgit.settings.searchHistory.v1")!)).toContain("预算");
  });

  test("keeps external settings jumps compatible and ignores invalid destinations", async () => {
    const view = openSettings();
    window.dispatchEvent(new CustomEvent("wgit:settings-section", { detail: { section: "localai" } }));
    await nextTick();
    expect(view.find('[data-page="AiConnectionSection"]').exists()).toBe(true);
    window.dispatchEvent(new CustomEvent("wgit:settings-section", { detail: { section: "missing" } }));
    await nextTick();
    expect(view.find('[data-page="AiConnectionSection"]').exists()).toBe(true);
  });
});
