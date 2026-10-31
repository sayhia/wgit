import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const here = dirname(fileURLToPath(import.meta.url));
const src = readFileSync(join(here, "../src/components/ToolsView.vue"), "utf8");
const dict = (m: object) => m as Record<string, string>;

const ALL_TAB_IDS = [
  "terminal",
  "reflog",
  "clean",
  "remotes",
  "worktrees",
  "lfs",
  "submodules",
  "sparse",
  "disk",
  "maintenance",
  "studio",
  "engine",
  "bisect",
  "release",
];

describe("ToolsView two-tier tabs", () => {
  test("PRIMARY_TABS keeps 5 high-frequency entries, MORE_TABS holds the other 9", () => {
    const primary = src.match(/const PRIMARY_TABS[^=]*=\s*\[([\s\S]*?)\]/)?.[1] ?? "";
    const more = src.match(/const MORE_TABS[^=]*=\s*\[([\s\S]*?)\]/)?.[1] ?? "";
    const primaryIds = [...primary.matchAll(/id: "([a-z]+)"/g)].map((m) => m[1]);
    const moreIds = [...more.matchAll(/id: "([a-z]+)"/g)].map((m) => m[1]);
    expect(primaryIds).toHaveLength(5);
    expect(moreIds).toHaveLength(9);
    expect([...primaryIds, ...moreIds].sort()).toEqual([...ALL_TAB_IDS].sort());
    expect(new Set([...primaryIds, ...moreIds]).size).toBe(ALL_TAB_IDS.length);
  });

  test("every tab labelKey and tools.more exist in both locales", () => {
    const labelKeys = [...src.matchAll(/labelKey: "([a-z.]+)"/g)].map((m) => m[1]);
    expect(labelKeys.length).toBe(14);
    for (const key of labelKeys) {
      expect(dict(zhCN)[key]).toBeTruthy();
      expect(dict(enUS)[key]).toBeTruthy();
    }
    expect(dict(zhCN)["tools.more"]).toBeTruthy();
    expect(dict(enUS)["tools.more"]).toBeTruthy();
  });

  test("dropdown wiring: more button, overlay close, escape-free selectTab close", () => {
    expect(src).toContain('data-testid="tools-more-btn"');
    expect(src).toContain('data-testid="tools-more-menu"');
    expect(src).toContain('showMoreMenu = false');
    expect(src).toContain("function selectTab(");
    // 14 个手写按钮已被数据驱动替换：模板中不再有 `activeSubTab = 'xxx'` 直赋按钮
    const directButtonTabs = [...src.matchAll(/@click="activeSubTab = '([a-z]+)'"/g)].map((m) => m[1]);
    expect(directButtonTabs).toEqual([]);
  });

  test("badge definitions preserved for clean/worktrees/submodules/sparse", () => {
    for (const id of ["clean", "worktrees", "submodules", "sparse"]) {
      expect(src).toMatch(new RegExp(`id: "${id}"[^}]*badgeKey: "${id}"`));
    }
  });
});
