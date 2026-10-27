import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const modalFile = fileURLToPath(
  new URL("../src/components/tools/AirGappedHubModal.vue", import.meta.url)
);
const toolsViewFile = fileURLToPath(
  new URL("../src/components/ToolsView.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Phase H4: Air-Gapped Sync & Delta Pack Hub UX", () => {
  const modalSrc = readFileSync(modalFile, "utf8");
  const toolsSrc = readFileSync(toolsViewFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  test("AirGappedHubModal renders export and import panels, checksum inspection, and apply controls", () => {
    expect(modalSrc).toContain("air-gapped-tab-export");
    expect(modalSrc).toContain("air-gapped-tab-import");
    expect(modalSrc).toContain("create-delta-bundle-btn");
    expect(modalSrc).toContain("inspect-delta-bundle-btn");
    expect(modalSrc).toContain("apply-delta-bundle-btn");
    expect(modalSrc).toContain("createDeltaBundle");
    expect(modalSrc).toContain("inspectDeltaBundle");
    expect(modalSrc).toContain("applyDeltaBundle");
    expect(modalSrc).toContain("air_gapped.title");
    expect(modalSrc).toContain("air_gapped.export.btn");
    expect(modalSrc).toContain("air_gapped.import.btn.apply");
  });

  test("ToolsView integrates AirGappedHubModal launcher", () => {
    expect(toolsSrc).toContain("open-air-gapped-hub-btn");
    expect(toolsSrc).toContain("AirGappedHubModal");
    expect(toolsSrc).toContain("showAirGappedModal");
  });

  test("gitService exposes Phase H4 RPC endpoints", () => {
    expect(gitServiceSrc).toContain("createDeltaBundle:");
    expect(gitServiceSrc).toContain("inspectDeltaBundle:");
    expect(gitServiceSrc).toContain("applyDeltaBundle:");
  });

  test("types.ts defines Phase H4 interfaces", () => {
    expect(typesSrc).toContain("export interface DeltaBundleManifest");
    expect(typesSrc).toContain("export interface DeltaBundleResult");
    expect(typesSrc).toContain("export interface DeltaApplyResult");
  });

  test("i18n parity for air-gapped hub keys", () => {
    const requiredKeys = [
      "air_gapped.title",
      "air_gapped.subtitle",
      "air_gapped.tab.export",
      "air_gapped.tab.import",
      "air_gapped.export.base_ref",
      "air_gapped.export.target_ref",
      "air_gapped.export.output_path",
      "air_gapped.export.output_placeholder",
      "air_gapped.export.btn",
      "air_gapped.export.generating",
      "air_gapped.export.success",
      "air_gapped.import.file_path",
      "air_gapped.import.file_placeholder",
      "air_gapped.import.btn.inspect",
      "air_gapped.import.inspecting",
      "air_gapped.import.target_branch",
      "air_gapped.import.target_placeholder",
      "air_gapped.import.btn.apply",
      "air_gapped.import.applying",
      "air_gapped.import.success",
      "air_gapped.import.checksum",
      "air_gapped.import.commit_count",
      "air_gapped.import.created_at",
      "air_gapped.import.invalid",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
