import { describe, expect, test } from "bun:test";
import {
  SETTINGS_CATEGORY_LABEL_KEYS,
  SETTINGS_ITEM_INDEX,
} from "../src/utils/settingsIndex";
import { setWarnMissing, t } from "../src/i18n";

setWarnMissing(false);

describe("settings index integrity", () => {
  test("every indexed item points at a known category", () => {
    const known = new Set(Object.keys(SETTINGS_CATEGORY_LABEL_KEYS));
    for (const item of SETTINGS_ITEM_INDEX) {
      expect(known.has(item.category)).toBe(true);
    }
  });

  test("every category has at least one discoverable item", () => {
    for (const category of Object.keys(SETTINGS_CATEGORY_LABEL_KEYS)) {
      const items = SETTINGS_ITEM_INDEX.filter((i) => i.category === category);
      expect(items.length).toBeGreaterThan(0);
    }
  });

  test("every category and item label resolves to real copy", () => {
    for (const key of Object.values(SETTINGS_CATEGORY_LABEL_KEYS)) {
      const label = t(key);
      expect(label).not.toBe(key);
      expect(label.trim().length).toBeGreaterThan(0);
    }
    for (const item of SETTINGS_ITEM_INDEX) {
      const label = t(item.labelKey);
      expect(label).not.toBe(item.labelKey);
      expect(label.trim().length).toBeGreaterThan(0);
    }
  });

  test("labels are unique", () => {
    const labels = SETTINGS_ITEM_INDEX.map((i) => t(i.labelKey));
    expect(new Set(labels).size).toBe(labels.length);
  });

  test("every item has at least one keyword", () => {
    for (const item of SETTINGS_ITEM_INDEX) {
      expect(item.keywords.length).toBeGreaterThan(0);
      for (const keyword of item.keywords) {
        expect(keyword.trim().length).toBeGreaterThan(0);
      }
    }
  });

  test("keywords are lower-case so search does not depend on casing", () => {
    for (const item of SETTINGS_ITEM_INDEX) {
      for (const keyword of item.keywords) {
        expect(keyword).toBe(keyword.toLowerCase());
      }
    }
  });

  test("no item promises a feature that was removed from the index", () => {
    // These were the phantom entries that made the search box lie: they were
    // indexed but had no UI behind them. Guard against reintroduction.
    const forbidden = ["界面密度与字号", "动效与过渡效果", "无效 / 冲突配置清理", "代理环境变量与加速"];
    for (const label of forbidden) {
      expect(SETTINGS_ITEM_INDEX.some((i) => t(i.labelKey) === label)).toBe(false);
    }
  });
});
