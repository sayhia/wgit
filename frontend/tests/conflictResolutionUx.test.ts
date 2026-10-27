import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const diffViewerFile = fileURLToPath(
  new URL("../src/components/workspace/DiffViewer.vue", import.meta.url)
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

describe("Conflict Resolution & 3-Way Inline Editor (Round 6)", () => {
  describe("DiffViewer CodeLens and Conflict Block Navigation", () => {
    const src = readFileSync(diffViewerFile, "utf8");

    test("defines resolve-conflict-block emit event", () => {
      expect(src).toContain("(e: \"resolve-conflict-block\", blockIndex: number, choice: \"ours\" | \"theirs\" | \"both\"): void");
    });

    test("renders conflict navigation floating pill with previous and next buttons", () => {
      expect(src).toContain("totalConflictBlocks");
      expect(src).toContain("activeConflictIndex");
      expect(src).toContain("scrollToConflict");
      expect(src).toContain("workspace.diff.conflict.navigation");
      expect(src).toContain("workspace.diff.conflict.prev");
      expect(src).toContain("workspace.diff.conflict.next");
    });

    test("renders inline conflict block resolution buttons with anchor id", () => {
      expect(src).toContain(":id=\"'conflict-block-' + (line.conflictBlockIndex ?? 0)\"");
      expect(src).toContain("emit('resolve-conflict-block', line.conflictBlockIndex ?? 0, 'ours')");
      expect(src).toContain("emit('resolve-conflict-block', line.conflictBlockIndex ?? 0, 'theirs')");
      expect(src).toContain("emit('resolve-conflict-block', line.conflictBlockIndex ?? 0, 'both')");
    });

    test("tracks blockIndex during diff parsing for conflict markers", () => {
      expect(src).toContain("line.type === 'conflict-marker'");
      expect(src).toContain("blockIndex = blockCounter++");
    });
  });

  describe("WorkspaceView Integration", () => {
    const src = readFileSync(workspaceViewFile, "utf8");

    test("listens to @resolve-conflict-block on DiffViewer", () => {
      expect(src).toContain("@resolve-conflict-block=\"handleResolveConflictBlock\"");
    });

    test("invokes gitService.resolveConflictBlock and displays auto-stage or remaining notification", () => {
      expect(src).toContain("gitService.resolveConflictBlock(");
      expect(src).toContain("workspace.diff.conflict.auto.staged.toast");
      expect(src).toContain("workspace.diff.conflict.resolved.toast");
      expect(src).toContain("res.remainingConflicts");
    });
  });

  describe("Frontend Service and Types Contracts", () => {
    const typesSrc = readFileSync(typesFile, "utf8");
    const serviceSrc = readFileSync(gitServiceFile, "utf8");

    test("types exports ConflictBlockResolveResult", () => {
      expect(typesSrc).toContain("export interface ConflictBlockResolveResult");
      expect(typesSrc).toContain("success: boolean");
      expect(typesSrc).toContain("remainingConflicts: number");
      expect(typesSrc).toContain("fileAutoStaged: boolean");
    });

    test("gitService exposes resolveConflictBlock RPC method", () => {
      expect(serviceSrc).toContain("resolveConflictBlock:");
      expect(serviceSrc).toContain("ResolveConflictBlock");
    });

    test("types and service expose meta conflict bulk resolution and LFS lock notification", () => {
      expect(typesSrc).toContain("export interface MetaConflictResolutionResult");
      expect(typesSrc).toContain("resolvedFiles: string[]");
      expect(typesSrc).toContain("strategy: string");
      expect(typesSrc).toContain("count: number");

      expect(typesSrc).toContain("export interface LfsLockNotificationTemplate");
      expect(typesSrc).toContain("markdown: string");
      expect(typesSrc).toContain("emailSubject: string");
      expect(typesSrc).toContain("webhookJson: string");

      expect(serviceSrc).toContain("autoResolveMetaConflicts:");
      expect(serviceSrc).toContain("AutoResolveMetaConflicts");
      expect(serviceSrc).toContain("generateLfsLockNotification:");
      expect(serviceSrc).toContain("GenerateLfsLockNotification");
    });
  });

  describe("i18n completeness for Round 6", () => {
    const requiredKeys = [
      "workspace.diff.conflict.block.badge",
      "workspace.diff.conflict.block.title",
      "workspace.diff.conflict.accept.ours",
      "workspace.diff.conflict.accept.ours.hint",
      "workspace.diff.conflict.accept.theirs",
      "workspace.diff.conflict.accept.theirs.hint",
      "workspace.diff.conflict.accept.both",
      "workspace.diff.conflict.accept.both.hint",
      "workspace.diff.conflict.navigation",
      "workspace.diff.conflict.prev",
      "workspace.diff.conflict.next",
      "workspace.diff.conflict.resolved.toast",
      "workspace.diff.conflict.auto.staged.toast",
      "workspace.diff.conflict.resolve.failed",
      "ws.conflict.auto.meta",
      "ws.conflict.auto.meta.desc",
      "ws.conflict.lfs.notify",
    ];

    for (const key of requiredKeys) {
      test(`zh-CN has ${key}`, () => {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      });

      test(`en-US has ${key}`, () => {
        expect((enUS as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      });
    }
  });
});
