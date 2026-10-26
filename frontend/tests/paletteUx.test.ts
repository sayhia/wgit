import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const paletteFile = fileURLToPath(
  new URL("../src/components/CommandPalette.vue", import.meta.url)
);

describe("Accessibility & Command Palette UX (Round 18)", () => {
  const src = readFileSync(paletteFile, "utf8");

  describe("W3C ARIA Combobox 1.2 Compliance", () => {
    test("searchInput specifies combobox role and aria-controls", () => {
      expect(src).toContain('role="combobox"');
      expect(src).toContain('aria-autocomplete="list"');
      expect(src).toContain(':aria-controls="\'palette-listbox\'"');
      expect(src).toContain(':aria-activedescendant="\'palette-item-\' + selectedIndex"');
    });

    test("listbox specifies role, id, and aria-label", () => {
      expect(src).toContain('id="palette-listbox"');
      expect(src).toContain('role="listbox"');
      expect(src).toContain(':aria-label="t(\'cmd.aria.list\')"');
    });

    test("command rows specify option role, aria-selected, and roving tabindex", () => {
      expect(src).toContain('role="option"');
      expect(src).toContain(':id="\'palette-item-\' + entry.index"');
      expect(src).toContain(':aria-selected="selectedIndex === entry.index"');
      expect(src).toContain(':tabindex="selectedIndex === entry.index ? 0 : -1"');
    });
  });

  describe("Extended Keyboard Navigation", () => {
    test("handles PageDown and PageUp paging navigation", () => {
      expect(src).toContain('e.key === "PageDown"');
      expect(src).toContain('moveSelection(5)');
      expect(src).toContain('e.key === "PageUp"');
      expect(src).toContain('moveSelection(-5)');
    });

    test("handles Home and End jump navigation", () => {
      expect(src).toContain('e.key === "Home"');
      expect(src).toContain('e.key === "End"');
      expect(src).toContain('selectedIndex.value = 0');
    });
  });

  describe("Newly Registered Commands & i18n", () => {
    test("registers settings-safe-directory and tools-engine-meta commands", () => {
      expect(src).toContain('id: "settings-safe-directory"');
      expect(src).toContain('title: "cmd.settings-safe-directory"');
      expect(src).toContain('id: "tools-engine-meta"');
      expect(src).toContain('title: "cmd.tools-engine-meta"');
    });

    test("both locales translate cmd.settings-safe-directory and cmd.tools-engine-meta", () => {
      expect((zhCN as Record<string, string>)["cmd.settings-safe-directory"]).toBeDefined();
      expect((enUS as Record<string, string>)["cmd.settings-safe-directory"]).toBeDefined();
      expect((zhCN as Record<string, string>)["cmd.tools-engine-meta"]).toBeDefined();
      expect((enUS as Record<string, string>)["cmd.tools-engine-meta"]).toBeDefined();
      expect((zhCN as Record<string, string>)["cmd.aria.list"]).toBeDefined();
      expect((enUS as Record<string, string>)["cmd.aria.list"]).toBeDefined();
    });
  });
});
