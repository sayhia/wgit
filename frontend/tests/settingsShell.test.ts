/**
 * Settings shell integration (Task 9).
 *
 * The shell switch replaces the flat 11-category sidebar with four groups and
 * thirteen leaves, moves search out of `SettingsView`, widens the content
 * column and deletes the legacy AI panels. These tests pin the navigation
 * contract, the deep-link normalization, the i18n/command surfaces and the
 * removal of the old components.
 */
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { mount } from "@vue/test-utils";
import { nextTick, type Component } from "vue";
import SettingsNav from "../src/components/settings/SettingsNav.vue";
import SettingsView from "../src/components/SettingsView.vue";
import { COMMAND_REGISTRY } from "../src/commands/registry";
import { SETTINGS_GROUPS, SETTINGS_LEAVES } from "../src/utils/settingsStructure";
import { SETTINGS_ITEM_INDEX } from "../src/utils/settingsIndex";
import { locale, setLocale, t } from "../src/i18n";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import "./domSetup";

const read = (path: string): string =>
  readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");

const dict = (m: object) => m as Record<string, string>;

const NAV_KEY = "wgit.settings.active.v1";
const HISTORY_KEY = "wgit.settings.searchHistory.v1";
const originalLocale = locale.value;

const SECTION_NAMES = [
  "AppearanceSection",
  "IdentitySection",
  "SigningSection",
  "ProxySection",
  "ToolsSection",
  "AliasesSection",
  "GitHooksSection",
  "AdvancedConfigSection",
  "ShortcutsSection",
  "AiConnectionSection",
  "AiBehaviorSection",
  "AiInsightsSection",
  "AboutSection",
] as const;

const sectionStubs: Record<string, Component> = Object.fromEntries(
  SECTION_NAMES.map((name) => [name, { template: `<div data-page="${name}"></div>` }])
);

let wrapper: ReturnType<typeof mount> | undefined;

function openSettings(): ReturnType<typeof mount> {
  wrapper = mount(SettingsView, {
    props: { currentRepoPath: "/test/repo" },
    global: { stubs: sectionStubs },
  });
  return wrapper;
}

beforeEach(() => {
  localStorage.removeItem(NAV_KEY);
  localStorage.removeItem(HISTORY_KEY);
  setLocale("zh-CN");
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  localStorage.removeItem(NAV_KEY);
  localStorage.removeItem(HISTORY_KEY);
  setLocale(originalLocale);
});

describe("settings shell: grouped navigation", () => {
  test("renders four groups and thirteen leaves with one current page", () => {
    const nav = mount(SettingsNav, { props: { activeLeaf: "ai-connection" } });

    expect(nav.findAll("[data-settings-leaf]")).toHaveLength(13);
    expect(nav.findAll("[data-settings-nav] > section")).toHaveLength(4);
    expect(nav.findAll("[data-settings-nav] > section").map((group) => group.attributes("role"))).toEqual([
      "group",
      "group",
      "group",
      "group",
    ]);
    expect(nav.text()).toContain(t("settings.group.general"));
    expect(nav.text()).toContain(t("settings.group.git"));
    expect(nav.text()).toContain(t("settings.group.ai"));
    expect(nav.text()).toContain(t("settings.group.system"));
    expect(nav.find("[data-settings-leaf='ai-connection']").attributes("aria-current")).toBe("page");
    expect(nav.findAll('[data-settings-leaf][aria-current="page"]')).toHaveLength(1);
    expect(new Set(nav.findAll("[data-settings-leaf]").map((leaf) => leaf.attributes("data-settings-leaf"))).size).toBe(13);
    nav.unmount();
  });

  test("clicking a leaf emits select-leaf and keeps it the only current page in SettingsView", async () => {
    const nav = mount(SettingsNav, { props: { activeLeaf: "appearance" } });
    await nav.find("[data-settings-leaf='hooks']").trigger("click");
    expect(nav.emitted("select-leaf")?.[0]).toEqual(["hooks"]);
    nav.unmount();

    const view = openSettings();
    await view.find("[data-settings-leaf='ai-insights']").trigger("click");
    expect(view.find('[data-page="AiInsightsSection"]').exists()).toBe(true);
    expect(view.findAll('[data-settings-leaf][aria-current="page"]')).toHaveLength(1);
    expect(view.find("[data-settings-leaf='ai-insights']").attributes("aria-current")).toBe("page");
  });
});

describe("settings shell: search lives in SettingsNav", () => {
  test("filters leaves and highlights matches without leaving the nav", async () => {
    const nav = mount(SettingsNav, { props: { activeLeaf: "appearance" } });
    const input = nav.find('input[type="text"]');
    expect(input.attributes("aria-label")).toBe(t("settings.search.placeholder"));

    await input.setValue("身份");
    await nextTick();

    const leaves = nav.findAll("[data-settings-leaf]");
    expect(leaves).toHaveLength(1);
    expect(leaves[0]!.attributes("data-settings-leaf")).toBe("identity");
    expect(nav.find("[data-settings-leaf='identity'] mark").exists()).toBe(true);
    nav.unmount();
  });

  test("Enter opens the first matching leaf and records the query", async () => {
    const nav = mount(SettingsNav, { props: { activeLeaf: "appearance" } });
    const input = nav.find('input[type="text"]');
    await input.setValue("预算");
    await input.trigger("keydown", { key: "Enter" });
    await nextTick();

    expect(nav.emitted("select-leaf")?.at(-1)).toEqual(["ai-insights"]);
    expect(JSON.parse(localStorage.getItem(HISTORY_KEY) ?? "[]")).toContain("预算");
    nav.unmount();
  });

  test("empty results show the existing empty-state copy", async () => {
    const nav = mount(SettingsNav, { props: { activeLeaf: "appearance" } });
    await nav.find('input[type="text"]').setValue("zzzz-no-match");
    await nextTick();

    expect(nav.text()).toContain(t("settings.search.empty"));
    expect(nav.text()).toContain(t("settings.search.empty.hint"));
    nav.unmount();
  });
});

describe("settings shell: SettingsView wiring", () => {
  test("uses the structure helpers, a full-width column and no inline search", () => {
    const src = read("../src/components/SettingsView.vue");
    expect(src).not.toContain("max-w-3xl");
    expect(src).not.toContain("max-w-3xl mx-auto");
    expect(src).toContain("normalizeSettingsLeaf");
    expect(src).toContain("SETTINGS_LEAVES");
    expect(src).toContain("SettingsNav");
    expect(src).toContain("w-full min-w-0");
    expect(src).not.toContain("settingsSearch");
    expect(src).not.toContain('type="search"');
    expect(src).not.toContain("LocalAiSection");
    expect(src).not.toContain("AiControlPlaneSection");
    expect(src).not.toContain("AiQuickSetup");
  });

  test("deep links through localai open the AI connection leaf", async () => {
    const view = openSettings();
    window.dispatchEvent(new CustomEvent("wgit:settings-section", { detail: { section: "localai" } }));
    await nextTick();

    expect(view.find('[data-page="AiConnectionSection"]').exists()).toBe(true);
    expect(view.find("[data-settings-leaf='ai-connection']").attributes("aria-current")).toBe("page");
    expect(localStorage.getItem(NAV_KEY)).toBe("ai-connection");
  });

  test("AI leaves own their page header; the shell never adds a second one", () => {
    const shellSrc = read("../src/components/SettingsView.vue");
    expect(shellSrc).not.toContain("<SettingsPageHeader");
    for (const section of [
      "../src/components/settings/AiConnectionSection.vue",
      "../src/components/settings/AiBehaviorSection.vue",
      "../src/components/settings/AiInsightsSection.vue",
    ]) {
      const src = read(section);
      expect(src.match(/<SettingsPageHeader/g) ?? []).toHaveLength(1);
    }
  });
});

describe("settings shell: index, copy and commands", () => {
  test("every indexed item points at a real leaf and the AI entries are split", () => {
    for (const item of SETTINGS_ITEM_INDEX) {
      expect(SETTINGS_LEAVES).toContain(item.category);
    }
    const categories = new Set(SETTINGS_ITEM_INDEX.map((item) => item.category));
    expect(categories.has("ai-connection")).toBe(true);
    expect(categories.has("ai-behavior")).toBe(true);
    expect(categories.has("ai-insights")).toBe(true);
    expect(Array.from(categories)).not.toContain("localai");
  });

  test("group and AI leaf labels resolve in both catalogs", () => {
    const keys = [
      "settings.group.general",
      "settings.group.git",
      "settings.group.ai",
      "settings.group.system",
      "settings.category.ai-connection",
      "settings.category.ai-behavior",
      "settings.category.ai-insights",
      "settings.nav.label",
    ];
    for (const key of keys) {
      expect(dict(zhCN)[key]?.trim().length).toBeGreaterThan(0);
      expect(dict(enUS)[key]?.trim().length).toBeGreaterThan(0);
    }
  });

  test("the settings group exposes commands for the three AI leaves", () => {
    const ids = COMMAND_REGISTRY.map((command) => command.id);
    for (const id of ["settings-ai-connection", "settings-ai-behavior", "settings-ai-insights"]) {
      expect(ids).toContain(id);
    }
    for (const id of ["settings-appearance", "settings-identity", "settings-config"]) {
      expect(ids).toContain(id);
    }
  });

  test("the runtime quick-setup modal reuses the compact connection form", () => {
    const src = read("../src/components/settings/AiQuickSetupModal.vue");
    expect(src).toContain("AiConnectionForm");
    expect(src).toContain("compact");
    expect(src).not.toContain("AiQuickSetup.vue");
  });
});

describe("settings shell: legacy AI panels are gone", () => {
  test("the three legacy components no longer exist", () => {
    for (const file of [
      "../src/components/settings/AiControlPlaneSection.vue",
      "../src/components/settings/AiQuickSetup.vue",
      "../src/components/settings/LocalAiSection.vue",
    ]) {
      expect(existsSync(fileURLToPath(new URL(file, import.meta.url)))).toBe(false);
    }
  });

  test("no source file still imports a deleted component", () => {
    const src = read("../src/components/SettingsView.vue");
    for (const legacy of ["AiControlPlaneSection", "AiQuickSetup", "LocalAiSection"]) {
      expect(src).not.toContain(legacy);
    }
    expect(SETTINGS_GROUPS.length).toBe(4);
  });
});
