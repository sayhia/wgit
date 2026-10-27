import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const modalFile = fileURLToPath(
  new URL("../src/components/workspace/SemanticConflictModal.vue", import.meta.url)
);
const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);
const typesFile = fileURLToPath(
  new URL("../src/types.ts", import.meta.url)
);

describe("Phase H1: Semantic 3-Way Asset Conflict Resolver UX", () => {
  const modalSrc = readFileSync(modalFile, "utf8");
  const wsSrc = readFileSync(workspaceViewFile, "utf8");
  const gitServiceSrc = readFileSync(gitServiceFile, "utf8");
  const typesSrc = readFileSync(typesFile, "utf8");

  test("SemanticConflictModal renders conflict blocks and auto-resolution strategies", () => {
    expect(modalSrc).toContain("loadAnalysis");
    expect(modalSrc).toContain("handleResolve");
    expect(modalSrc).toContain("smart-resolve-btn");
    expect(modalSrc).toContain("merge_both");
    expect(modalSrc).toContain("prefer_ours");
    expect(modalSrc).toContain("prefer_theirs");
    expect(modalSrc).toContain("conflict.semantic.title");
    expect(modalSrc).toContain("conflict.semantic.btn.smart");
  });

  test("WorkspaceView integrates semantic conflict detection and modal trigger", () => {
    expect(wsSrc).toContain("open-semantic-conflict-btn");
    expect(wsSrc).toContain("hasSemanticAssetConflicts");
    expect(wsSrc).toContain("SemanticConflictModal");
    expect(wsSrc).toContain("conflict.semantic.inspect");
  });

  test("gitService exposes Phase H1 RPC endpoints", () => {
    expect(gitServiceSrc).toContain("analyzeYaml3WayConflict:");
    expect(gitServiceSrc).toContain("autoResolveSemanticYamlConflict:");
  });

  test("types.ts defines Phase H1 interfaces", () => {
    expect(typesSrc).toContain("export interface YamlConflictBlock");
    expect(typesSrc).toContain("export interface YamlConflictAnalysisResult");
    expect(typesSrc).toContain("export interface YamlResolveResult");
  });

  test("i18n parity for semantic conflict resolver keys", () => {
    const requiredKeys = [
      "conflict.semantic.title",
      "conflict.semantic.desc",
      "conflict.semantic.blocks",
      "conflict.semantic.autocount",
      "conflict.semantic.btn.smart",
      "conflict.semantic.btn.both",
      "conflict.semantic.btn.ours",
      "conflict.semantic.btn.theirs",
      "conflict.semantic.type.component",
      "conflict.semantic.type.transform",
      "conflict.semantic.type.property",
      "conflict.semantic.empty",
      "conflict.semantic.resolving",
      "conflict.semantic.success",
      "conflict.semantic.inspect",
    ];

    for (const key of requiredKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      expect((enUS as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
    }
  });
});
