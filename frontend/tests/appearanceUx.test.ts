import { describe, expect, test, beforeEach } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { useTheme } from "../src/composables/useTheme";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

function setupMockEnv() {
  const store = new Map<string, string>();
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => void store.set(key, value),
      removeItem: (key: string) => void store.delete(key),
      clear: () => store.clear(),
    },
  });

  const classListSet = new Set<string>();
  const classList = {
    add: (c: string) => classListSet.add(c),
    remove: (c: string) => classListSet.delete(c),
    toggle: (c: string, force?: boolean) => {
      const shouldAdd = force !== undefined ? force : !classListSet.has(c);
      if (shouldAdd) classListSet.add(c);
      else classListSet.delete(c);
      return shouldAdd;
    },
    contains: (c: string) => classListSet.has(c),
  };

  const docEl = {
    classList,
    dataset: {} as Record<string, string>,
  };

  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: {
      documentElement: docEl,
    },
  });

  return { store, classListSet };
}

describe("Theme, Liquid Glass & Visual Accessibility UX (Round 19)", () => {
  beforeEach(() => {
    setupMockEnv();
  });

  test("useTheme exposes highContrast and setHighContrast", () => {
    const { highContrast, setHighContrast } = useTheme();
    expect(highContrast.value).toBe(false);

    setHighContrast(true);
    expect(highContrast.value).toBe(true);
    expect(localStorage.getItem("wgit-high-contrast")).toBe("true");
    expect(document.documentElement.classList.contains("high-contrast")).toBe(true);

    setHighContrast(false);
    expect(highContrast.value).toBe(false);
    expect(localStorage.getItem("wgit-high-contrast")).toBe("false");
    expect(document.documentElement.classList.contains("high-contrast")).toBe(false);
  });

  test("useTheme exposes reduceTransparency and setReduceTransparency", () => {
    const { reduceTransparency, setReduceTransparency } = useTheme();
    expect(reduceTransparency.value).toBe(false);

    setReduceTransparency(true);
    expect(reduceTransparency.value).toBe(true);
    expect(localStorage.getItem("wgit-reduce-transparency")).toBe("true");
    expect(document.documentElement.classList.contains("reduce-transparency")).toBe(true);

    setReduceTransparency(false);
    expect(reduceTransparency.value).toBe(false);
    expect(localStorage.getItem("wgit-reduce-transparency")).toBe("false");
    expect(document.documentElement.classList.contains("reduce-transparency")).toBe(false);
  });

  test("AppearanceSection.vue contains accessibility controls and event toggles", () => {
    const appearanceCode = readFileSync(
      join(__dirname, "../src/components/settings/AppearanceSection.vue"),
      "utf8"
    );
    expect(appearanceCode).toContain("toggleHighContrast");
    expect(appearanceCode).toContain("toggleReduceTransparency");
    expect(appearanceCode).toContain("settings.appearance.accessibility.title");
    expect(appearanceCode).toContain("settings.appearance.highcontrast.title");
    expect(appearanceCode).toContain("settings.appearance.transparency.title");
  });

  test("glass.css contains classes for high contrast and reduced transparency", () => {
    const glassCss = readFileSync(join(__dirname, "../src/styles/glass.css"), "utf8");
    expect(glassCss).toContain(":root.reduce-transparency .glass");
    expect(glassCss).toContain(":root.high-contrast");
    expect(glassCss).toContain(".high-contrast .glass");
  });

  test("i18n completeness for Round 19 appearance accessibility keys", () => {
    const requiredKeys = [
      "settings.appearance.accessibility.title",
      "settings.appearance.accessibility.desc",
      "settings.appearance.highcontrast.title",
      "settings.appearance.highcontrast.desc",
      "settings.appearance.highcontrast.toast.enabled",
      "settings.appearance.highcontrast.toast.disabled",
      "settings.appearance.transparency.title",
      "settings.appearance.transparency.desc",
      "settings.appearance.transparency.toast.enabled",
      "settings.appearance.transparency.toast.disabled",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key].length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key].length).toBeGreaterThan(0);
    }
  });
});
