import { describe, expect, test } from "bun:test";
import { COMMAND_GROUPS, COMMAND_REGISTRY } from "../src/commands/registry";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const dict = (m: object) => m as Record<string, string>;

describe("commands registry", () => {
  test("command ids are unique", () => {
    const ids = COMMAND_REGISTRY.map((c) => c.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  test("every entry declares a known group with a cmd.cat.* key in both locales", () => {
    for (const entry of COMMAND_REGISTRY) {
      expect(COMMAND_GROUPS).toContain(entry.group);
      expect(dict(zhCN)[`cmd.cat.${entry.group}`]).toBeTruthy();
      expect(dict(enUS)[`cmd.cat.${entry.group}`]).toBeTruthy();
    }
  });

  test("every labelKey resolves to a non-empty string in both locales", () => {
    for (const entry of COMMAND_REGISTRY) {
      expect(typeof dict(zhCN)[entry.labelKey]).toBe("string");
      expect(dict(zhCN)[entry.labelKey].length).toBeGreaterThan(0);
      expect(typeof dict(enUS)[entry.labelKey]).toBe("string");
      expect(dict(enUS)[entry.labelKey].length).toBeGreaterThan(0);
    }
  });

  test("every command exposes a runnable implementation", () => {
    for (const entry of COMMAND_REGISTRY) {
      expect(typeof entry.run).toBe("function");
    }
  });
});
