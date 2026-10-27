import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const gitBundleModalFile = fileURLToPath(
  new URL("../src/components/tools/GitBundleModal.vue", import.meta.url)
);
const commitInspectorFile = fileURLToPath(
  new URL("../src/components/history/CommitInspector.vue", import.meta.url)
);
const reviewViewFile = fileURLToPath(
  new URL("../src/components/ReviewView.vue", import.meta.url)
);
const maintenanceTabFile = fileURLToPath(
  new URL("../src/components/tools/MaintenanceTab.vue", import.meta.url)
);

describe("Phase M2: Team Collaboration, Pipeline & Offline Review", () => {
  describe("GitBundleModal Component", () => {
    const src = readFileSync(gitBundleModalFile, "utf8");

    test("contains revision range input and HEAD / --all presets", () => {
      expect(src).toContain("revSpec");
      expect(src).toContain("HEAD");
      expect(src).toContain("--all");
      expect(src).toContain("outputPath");
    });

    test("calls createGitBundleAdvanced and verifyGitBundle", () => {
      expect(src).toContain("createGitBundleAdvanced");
      expect(src).toContain("verifyGitBundle");
      expect(src).toContain("exportResult");
    });

    test("renders unpack & clone instructions with copy buttons", () => {
      expect(src).toContain("cloneHelp");
      expect(src).toContain("verifyHelp");
      expect(src).toContain("copyText");
      expect(src).toContain("bundle.modal.instructions");
    });
  });

  describe("MaintenanceTab Bundle Integration", () => {
    const src = readFileSync(maintenanceTabFile, "utf8");

    test("imports and mounts GitBundleModal with showBundleModal toggle", () => {
      expect(src).toContain('import GitBundleModal from "./GitBundleModal.vue"');
      expect(src).toContain("showBundleModal = ref(false)");
      expect(src).toContain("<GitBundleModal");
    });
  });

  describe("Git Notes CI Pipeline Badges", () => {
    const src = readFileSync(commitInspectorFile, "utf8");

    test("queries getCommitCIStatus and tracks ciStatus", () => {
      expect(src).toContain("getCommitCIStatus");
      expect(src).toContain("ciStatus");
      expect(src).toContain("CommitCIStatus");
    });

    test("renders CI pipeline status badge with success, failure, pending styles", () => {
      expect(src).toContain("ciStatus.status === 'success'");
      expect(src).toContain("ciStatus.status === 'failure'");
      expect(src).toContain("ciStatus.status === 'pending'");
      expect(src).toContain("ciStatus.duration");
      expect(src).toContain("ciStatus.url");
    });
  });

  describe("Local Review Session Draft Persistence", () => {
    const src = readFileSync(reviewViewFile, "utf8");

    test("implements LocalStorage draft persistence helpers", () => {
      expect(src).toContain("getDraftStorageKey");
      expect(src).toContain("loadSavedDraft");
      expect(src).toContain("saveDraftToStorage");
      expect(src).toContain("clearDraftFromStorage");
      expect(src).toContain("wgit.review.draft");
    });

    test("automatically saves noteDraft on change and clears on saveNote", () => {
      expect(src).toContain("saveDraftToStorage(selectedPath.value, val)");
      expect(src).toContain("clearDraftFromStorage(file.path)");
    });

    test("supports exporting Markdown review reports", () => {
      expect(src).toContain("exportReport");
      expect(src).toContain("exportMarkdown");
      expect(src).toContain("exportOpen");
    });
  });

  describe("Phase M2 i18n Completeness", () => {
    const requiredKeys = [
      "bundle.modal.title",
      "bundle.modal.rev.label",
      "bundle.modal.rev.hint",
      "bundle.modal.output.label",
      "bundle.modal.output.placeholder",
      "bundle.modal.export",
      "bundle.modal.exporting",
      "bundle.modal.success",
      "bundle.modal.size",
      "bundle.modal.instructions",
      "bundle.modal.instructions.clone",
      "bundle.modal.instructions.verify",
      "bundle.modal.instructions.fetch",
      "bundle.modal.verify.btn",
      "bundle.modal.verifying",
      "bundle.modal.verified",
      "review.draft.badge",
      "review.draft.export",
      "review.draft.saved",
      "review.draft.clear",
      "history.inspector.ci.title",
      "history.inspector.ci.passed",
      "history.inspector.ci.failed",
      "history.inspector.ci.pending",
    ];

    test("all Phase M2 keys exist in zh-CN and en-US catalogs", () => {
      for (const key of requiredKeys) {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]).toBeDefined();
      }
    });
  });
});
