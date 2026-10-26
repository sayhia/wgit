import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const submodulesTabFile = fileURLToPath(
  new URL("../src/components/tools/SubmodulesTab.vue", import.meta.url)
);
const toolsViewFile = fileURLToPath(
  new URL("../src/components/ToolsView.vue", import.meta.url)
);
const sparseTabFile = fileURLToPath(
  new URL("../src/components/tools/SparseTab.vue", import.meta.url)
);

describe("Submodules & Monorepo Sparse Checkout UX (Round 9)", () => {
  describe("SubmodulesTab Toolbar & Card Actions", () => {
    const src = readFileSync(submodulesTabFile, "utf8");

    test("contains handleInitSubmodules and sub.init.all button", () => {
      expect(src).toContain("handleInitSubmodules");
      expect(src).toContain("sub.init.all");
      expect(src).toContain("gitService.updateSubmodules(props.currentRepoPath, true, false)");
    });

    test("provides handleOpenSubmodule emitting open-repo", () => {
      expect(src).toContain("handleOpenSubmodule");
      expect(src).toContain("emit(\"open-repo\", fullPath)");
      expect(src).toContain("sub.open.workspace");
    });
  });

  describe("ToolsView open-repo event passthrough", () => {
    const src = readFileSync(toolsViewFile, "utf8");

    test("forwards @open-repo from SubmodulesTab", () => {
      expect(src).toContain("@open-repo=\"emit('open-repo', $event)\"");
    });
  });

  describe("SparseTab Monorepo Management", () => {
    const src = readFileSync(sparseTabFile, "utf8");

    test("manages sparse checkout directories", () => {
      expect(src).toContain("loadSparseCheckout");
      expect(src).toContain("handleApplySparseCheckout");
      expect(src).toContain("handleResetSparseCheckout");
      expect(src).toContain("addSparseFolder");
      expect(src).toContain("removeSparseFolder");
    });
  });

  describe("i18n completeness for Round 9", () => {
    const requiredKeys = [
      "sub.init.all",
      "sub.init.all.title",
      "sub.toast.init.done",
      "sub.toast.init.failed",
      "sub.open.workspace",
    ];

    for (const key of requiredKeys) {
      test(`zh-CN contains ${key}`, () => {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((zhCN as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      });

      test(`en-US contains ${key}`, () => {
        expect((enUS as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]!.length).toBeGreaterThan(0);
      });
    }
  });
});
