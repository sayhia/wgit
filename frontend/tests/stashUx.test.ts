import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const stashViewFile = fileURLToPath(
  new URL("../src/components/StashView.vue", import.meta.url)
);
const createStashModalFile = fileURLToPath(
  new URL("../src/components/branches/modals/CreateStashModal.vue", import.meta.url)
);
const stashesListFile = fileURLToPath(
  new URL("../src/components/branches/StashesList.vue", import.meta.url)
);
const branchesViewFile = fileURLToPath(
  new URL("../src/components/BranchesView.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);

describe("Stash & Snapshot Management UX (Round 7)", () => {
  describe("StashView Extended Stash & Context Menu", () => {
    const src = readFileSync(stashViewFile, "utf8");

    test("supports includeAll in save stash modal", () => {
      expect(src).toContain("saveIncludeAll");
      expect(src).toContain("stash.modal.save.all");
      expect(src).toContain("gitService.stashSaveAdvanced");
    });

    test("integrates desktop context menu with apply, pop, branch, copy, and drop", () => {
      expect(src).toContain("useContextMenu");
      expect(src).toContain("UiContextMenu");
      expect(src).toContain("stashMenuItems");
      expect(src).toContain("@contextmenu.prevent=\"openStashMenu(s, $event)\"");
      expect(src).toContain("copyStashRef");
      expect(src).toContain("stash.toast.copied");
    });

    test("provides quick hover copy ref button on stash card", () => {
      expect(src).toContain("stash.action.copy");
      expect(src).toContain("@click.stop=\"copyStashRef(s.index)\"");
    });
  });

  describe("CreateStashModal Contract", () => {
    const src = readFileSync(createStashModalFile, "utf8");

    test("declares all prop and update:all emit", () => {
      expect(src).toContain("all?: boolean");
      expect(src).toContain("(e: \"update:all\", value: boolean): void");
    });

    test("renders checkbox for branches.stash.create.all", () => {
      expect(src).toContain("v-model=\"allModel\"");
      expect(src).toContain("branches.stash.create.all");
    });
  });

  describe("StashesList Extended Actions", () => {
    const src = readFileSync(stashesListFile, "utf8");

    test("declares branch emit and context menu action", () => {
      expect(src).toContain("(e: \"branch\", stash: StashInfo): void");
      expect(src).toContain("id: \"branch\"");
      expect(src).toContain("branches.stash.menu.branch");
    });

    test("renders branch from stash button in action bar", () => {
      expect(src).toContain("@click=\"emit('branch', s)\"");
    });
  });

  describe("BranchesView Integration", () => {
    const src = readFileSync(branchesViewFile, "utf8");

    test("wires @branch handler on StashesList", () => {
      expect(src).toContain("@branch=\"handleStashBranch\"");
    });

    test("provides Stash Branch Modal with branch name input and validation", () => {
      expect(src).toContain("showStashBranchModal");
      expect(src).toContain("stashBranchName");
      expect(src).toContain("gitService.stashBranch(");
      expect(src).toContain("branches.stash.branch.title");
      expect(src).toContain("branches.stash.toast.branch.created");
    });

    test("wires CreateStashModal with stashIncludeAll", () => {
      expect(src).toContain("v-model:all=\"stashIncludeAll\"");
      expect(src).toContain("gitService.stashSaveAdvanced");
    });
  });

  describe("gitService RPC Contract", () => {
    const src = readFileSync(gitServiceFile, "utf8");

    test("exposes stashSaveAdvanced method calling backend", () => {
      expect(src).toContain("stashSaveAdvanced:");
      expect(src).toContain("StashSaveAdvanced");
    });
  });

  describe("i18n key completeness for Round 7", () => {
    const requiredKeys = [
      "stash.modal.save.all",
      "stash.action.copy",
      "stash.toast.copied",
      "branches.stash.create.all",
      "branches.stash.menu.branch",
      "branches.stash.branch.title",
      "branches.stash.branch.name",
      "branches.stash.branch.placeholder",
      "branches.stash.branch.confirm",
      "branches.stash.toast.branch.created",
      "branches.stash.toast.branch.failed",
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
