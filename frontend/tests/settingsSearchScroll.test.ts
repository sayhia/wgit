import { afterEach, beforeEach, expect, test } from "bun:test";
import { mount } from "@vue/test-utils";
import SettingsNav from "../src/components/settings/SettingsNav.vue";
import { locale, setLocale } from "../src/i18n";
import { SETTINGS_LEAVES } from "../src/utils/settingsStructure";
import "./domSetup";

const originalLocale = locale.value;
let wrapper: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  setLocale("en-US");
  localStorage.removeItem("wgit.settings.searchHistory.v1");
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  localStorage.removeItem("wgit.settings.searchHistory.v1");
  setLocale(originalLocale);
});

test("keyboard search reveals the selected result and Enter still opens that destination", async () => {
  wrapper = mount(SettingsNav, { props: { activeLeaf: "appearance" } });
  const input = wrapper.find('input[type="text"]');
  await input.setValue("i");
  const results = wrapper.findAll('[data-testid="settings-search-results"] [data-search-selected]');
  expect(results.length).toBeGreaterThan(5);
  const scrolled: number[] = [];
  results.forEach((result, index) => {
    (result.element as HTMLElement).scrollIntoView = () => { scrolled.push(index); };
  });

  for (let index = 1; index <= 5; index++) {
    await input.trigger("keydown", { key: "ArrowDown" });
    expect(results[index]!.attributes("data-search-selected")).toBe("true");
    expect(scrolled.at(-1)).toBe(index);
  }

  await input.trigger("keydown", { key: "Enter" });
  const destination = wrapper.emitted("select-leaf")?.at(-1)?.[0];
  expect(SETTINGS_LEAVES).toContain(destination);
  expect((input.element as HTMLInputElement).value).toBe("");
});
