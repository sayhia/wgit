import { describe, expect, test } from "bun:test";
import { gitService } from "../src/services/gitService";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import { readFileSync, existsSync } from "fs";
import { resolve } from "path";

describe("Phase F1: Material & Texture Visual Diff UX", () => {
  test("gitService exposes compareMaterialProperties and inspectTextureMetadata", () => {
    expect(typeof gitService.compareMaterialProperties).toBe("function");
    expect(typeof gitService.inspectTextureMetadata).toBe("function");
  });

  test("MaterialDiffViewer component exists, has clean template and zero hardcoded Chinese", () => {
    const viewerPath = resolve(__dirname, "../src/components/assets/MaterialDiffViewer.vue");
    expect(existsSync(viewerPath)).toBe(true);

    const src = readFileSync(viewerPath, "utf-8");
    expect(src).toContain("assets.material.title");
    expect(src).toContain("assets.material.changes.count");
    expect(src).toContain("assets.material.filter.changed");
    expect(src).toContain("assets.material.shader");
    expect(src).toContain("gitService.compareMaterialProperties");

    const templateMatch = src.match(/<template>([\s\S]*?)<\/template>/);
    const template = templateMatch ? templateMatch[1] : "";
    const stripped = template
      .replace(/t\(['"][^'"]+['"]\)/g, "")
      .replace(/<!--[\s\S]*?-->/g, "");

    const chineseRegex = /[\u4e00-\u9fa5]/;
    expect(chineseRegex.test(stripped)).toBe(false);
  });

  test("AssetDiffModal integrates MaterialDiffViewer and image channel controls", () => {
    const modalPath = resolve(__dirname, "../src/components/assets/AssetDiffModal.vue");
    const src = readFileSync(modalPath, "utf-8");

    expect(src).toContain("MaterialDiffViewer");
    expect(src).toContain("isMaterialFile");
    expect(src).toContain("activeChannel");
    expect(src).toContain("exposureEv");
    expect(src).toContain("imageFilterStyle");
  });

  test("i18n catalogs contain complete material diff keys in both languages", () => {
    const requiredKeys = [
      "assets.material.title",
      "assets.material.changes.count",
      "assets.material.search",
      "assets.material.filter.changed",
      "assets.material.filter.all",
      "assets.material.shader",
      "assets.material.empty",
      "assets.material.badge.modified",
      "assets.material.badge.unchanged",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]).toBeDefined();
    }
  });
});
