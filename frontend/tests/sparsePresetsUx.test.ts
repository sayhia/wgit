import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const sparseTabFile = fileURLToPath(
  new URL("../src/components/tools/SparseTab.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Phase G1: Role-Based Sparse & Scene LFS Hydration UX", () => {
  const sparseSrc = readFileSync(sparseTabFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  test("SparseTab includes role preset selection and application", () => {
    expect(sparseSrc).toContain("loadSparsePresets");
    expect(sparseSrc).toContain("handleApplyPreset");
    expect(sparseSrc).toContain("sparsePresets");
    expect(sparseSrc).toContain("activePresetId");
    expect(sparseSrc).toContain("sparse-role-presets");
    expect(sparseSrc).toContain("t(\"sparse.role.title\")");
  });

  test("SparseTab includes targeted Scene LFS Asset hydration card", () => {
    expect(sparseSrc).toContain("scene-lfs-hydration");
    expect(sparseSrc).toContain("handleHydrateScene");
    expect(sparseSrc).toContain("scenePath");
    expect(sparseSrc).toContain("sceneResult");
    expect(sparseSrc).toContain("t(\"sparse.scene.title\")");
    expect(sparseSrc).toContain("t(\"sparse.scene.btn\")");
  });

  test("gitService exposes Phase G1 RPC endpoints", () => {
    expect(gitServiceSrc).toContain("getSparsePresets:");
    expect(gitServiceSrc).toContain("applyRoleSparsePreset:");
    expect(gitServiceSrc).toContain("hydrateSceneLfsAssets:");
  });

  test("types.ts defines SparsePreset, SparseApplyResult, SceneHydrateResult", () => {
    expect(typesSrc).toContain("export interface SparsePreset");
    expect(typesSrc).toContain("export interface SparseApplyResult");
    expect(typesSrc).toContain("export interface SceneHydrateResult");
  });

  test("i18n parity for sparse role and scene hydration keys", () => {
    const requiredKeys = [
      "sparse.role.title",
      "sparse.role.desc",
      "sparse.role.apply",
      "sparse.role.applying",
      "sparse.role.applied",
      "sparse.role.savings",
      "sparse.role.custom",
      "sparse.scene.title",
      "sparse.scene.desc",
      "sparse.scene.placeholder",
      "sparse.scene.btn",
      "sparse.scene.hydrating",
      "sparse.scene.success",
      "sparse.scene.empty",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
