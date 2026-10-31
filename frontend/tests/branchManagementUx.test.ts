import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const modalFile = fileURLToPath(
  new URL("../src/components/branches/modals/SetUpstreamModal.vue", import.meta.url)
);
const localListFile = fileURLToPath(
  new URL("../src/components/branches/LocalBranchList.vue", import.meta.url)
);
const remoteListFile = fileURLToPath(
  new URL("../src/components/branches/RemoteBranchList.vue", import.meta.url)
);
const branchesViewFile = fileURLToPath(
  new URL("../src/components/BranchesView.vue", import.meta.url)
);

describe("Branch Management & Remote Tracking UX", () => {
  describe("SetUpstreamModal contract", () => {
    const src = readFileSync(modalFile, "utf8");

    test("declares expected props", () => {
      expect(src).toContain("isOpen: boolean");
      expect(src).toContain("branch: BranchInfo | null");
      expect(src).toContain("availableRemoteBranches: string[]");
    });

    test("declares expected emits", () => {
      expect(src).toContain('(e: "close"): void');
      expect(src).toContain('(e: "submit", upstream: string): void');
    });

    test("uses UiModal and searchable input", () => {
      expect(src).toContain("<UiModal");
      expect(src).toContain("v-model=\"searchQuery\"");
      expect(src).toContain("v-model=\"upstreamInput\"");
      expect(src).toContain("filteredBranches");
    });
  });

  describe("LocalBranchList contract", () => {
    const src = readFileSync(localListFile, "utf8");

    test("declares unset-upstream emit", () => {
      expect(src).toContain('(e: "unset-upstream", branch: BranchInfo): void');
    });

    test("shows edit and unset upstream when branch has upstream", () => {
      expect(src).toContain('id: "set-upstream"');
      expect(src).toContain('id: "unset-upstream"');
      expect(src).toContain('emit("unset-upstream", branch)');
    });
  });

  describe("RemoteBranchList contract", () => {
    const src = readFileSync(remoteListFile, "utf8");

    test("declares all required emits", () => {
      expect(src).toContain('(e: "checkout", name: string): void');
      expect(src).toContain('(e: "merge", name: string): void');
      expect(src).toContain('(e: "rebase", name: string): void');
      expect(src).toContain('(e: "open-web", name: string): void');
      expect(src).toContain('(e: "delete-remote", name: string): void');
    });

    test("uses useContextMenu with full desktop action items", () => {
      expect(src).toContain("useContextMenu");
      expect(src).toContain("<UiContextMenu");
      expect(src).toContain('id: "checkout"');
      expect(src).toContain('id: "merge"');
      expect(src).toContain('id: "rebase"');
      expect(src).toContain('id: "copy"');
      expect(src).toContain('id: "open-web"');
      expect(src).toContain('id: "delete-remote"');
    });

    test("includes hover action buttons", () => {
      expect(src).toContain("group-hover:opacity-100");
      expect(src).toContain("emit('merge'");
      expect(src).toContain("emit('rebase'");
      expect(src).toContain("emit('open-web'");
      expect(src).toContain("emit('delete-remote'");
    });
  });

  describe("BranchesView integration", () => {
    const src = readFileSync(branchesViewFile, "utf8");

    test("integrates SetUpstreamModal without prompt()", () => {
      expect(src).toContain("SetUpstreamModal");
      expect(src).not.toContain("prompt(");
      expect(src).toContain("handleSetUpstream");
      expect(src).toContain("handleSubmitSetUpstream");
    });

    test("wires unsetUpstream handler with gitService and toast", () => {
      expect(src).toContain("handleUnsetUpstream");
      expect(src).toContain("gitService.unsetUpstream(repoPath, branch.name)");
      expect(src).toContain('notify(t("branch.toast.upstream.unset"');
    });

    test("wires deleteRemoteBranch with confirmation and gitService", () => {
      expect(src).toContain("handleDeleteRemoteBranch");
      expect(src).toContain("confirmAction");
      expect(src).toContain("gitService.deleteRemoteBranch");
      expect(src).toContain('notify(t("branch.toast.delete.remote.success"');
    });
  });

  describe("Branch drift analysis error surface", () => {
    const src = readFileSync(branchesViewFile, "utf8");

    test("captures drift/overlap failures instead of swallowing them", () => {
      expect(src).toContain("const driftError = ref");
      expect(src).not.toContain(".catch(() => null)");
      // allSettled style, mirroring the mergePreviewError preflight pattern.
      const driftBlock = src.slice(src.indexOf("async function runDriftAnalysis"), src.indexOf("function openDriftRadar"));
      expect(driftBlock).toContain("Promise.allSettled");
      expect(driftBlock).toContain("describePreviewError");
      expect(driftBlock).toContain('driftError.value = "";');
    });

    test("clears driftError on re-analysis and repo switch", () => {
      const resetStart = src.indexOf("function resetRepoScopedState");
      const resetBlock = src.slice(resetStart, src.indexOf("watch(", resetStart));
      expect(resetBlock).toContain('driftError.value = "";');
      expect(resetBlock).toContain("driftReport.value = null;");
      expect(resetBlock).toContain("driftMatrixReport.value = null;");
    });

    test("renders a warning alert with the i18n key and a retry action", () => {
      expect(src).toContain('role="alert"');
      expect(src).toContain('data-testid="branch-drift-error-banner"');
      expect(src).toContain('data-testid="branch-drift-retry-btn"');
      expect(src).toContain('t("branch.drift.analysis.failed", { message: driftError })');
      expect(src).toContain('@click="runDriftAnalysis"');
    });

    test("i18n key branch.drift.analysis.failed exists in zh-CN and en-US", () => {
      const key = "branch.drift.analysis.failed";
      expect(key in zhCN).toBe(true);
      expect(key in enUS).toBe(true);
      expect((zhCN as any)[key]).toContain("{message}");
      expect((enUS as any)[key]).toContain("{message}");
      expect((zhCN as any)[key].trim().length).toBeGreaterThan(0);
      expect((enUS as any)[key].trim().length).toBeGreaterThan(0);
    });
  });

  describe("Branch drift result panel", () => {
    const src = readFileSync(branchesViewFile, "utf8");

    test("renders a result panel bound to driftReport / driftMatrixReport", () => {
      // v-if precedes data-testid on the panel div, so assert it on the full source.
      expect(src).toContain('v-if="driftReport || driftMatrixReport || isAnalyzingDrift"');
      const panelStart = src.indexOf('data-testid="branch-drift-panel"');
      expect(panelStart).toBeGreaterThan(-1);
      const panelBlock = src.slice(panelStart);
      expect(panelBlock).toContain('data-testid="branch-drift-panel-summary"');
      expect(panelBlock).toContain('data-testid="branch-drift-panel-overlap"');
      expect(panelBlock).toContain("driftSuggestions");
      expect(panelBlock).toContain("topOverlapPairs");
      expect(panelBlock).toContain("overlapPairCount");
      expect(panelBlock).toContain("driftRiskLabel");
    });

    test("keeps the radar entry wired to the analysis and drops dead modal state", () => {
      expect(src).toContain('@click="openDriftRadar()"');
      expect(src).toContain("void runDriftAnalysis();");
      // The modal flag was written but never read by any template or component.
      expect(src).not.toContain("showDriftModal");
    });

    test("branch.drift.panel i18n keys exist in zh-CN and en-US", () => {
      const keys = [
        "branch.drift.panel.title",
        "branch.drift.panel.loading",
        "branch.drift.panel.scope",
        "branch.drift.panel.aheadbehind",
        "branch.drift.panel.health",
        "branch.drift.panel.level.low",
        "branch.drift.panel.level.medium",
        "branch.drift.panel.level.high",
        "branch.drift.panel.suggestions",
        "branch.drift.panel.suggest.conflicts",
        "branch.drift.panel.suggest.overlap",
        "branch.drift.panel.suggest.behind",
        "branch.drift.panel.suggest.clean",
        "branch.drift.panel.overlap.title",
        "branch.drift.panel.overlap.pairs",
        "branch.drift.panel.overlap.none",
        "branch.drift.panel.overlap.shared",
        "branch.drift.panel.risk.low",
        "branch.drift.panel.risk.medium",
        "branch.drift.panel.risk.high",
      ];
      for (const key of keys) {
        expect(key in zhCN).toBe(true);
        expect(key in enUS).toBe(true);
        expect((zhCN as any)[key].trim().length).toBeGreaterThan(0);
        expect((enUS as any)[key].trim().length).toBeGreaterThan(0);
      }
    });
  });

  describe("i18n key parity for new features", () => {
    const requiredKeys = [
      "branches.upstream.modal.title",
      "branches.upstream.modal.intro",
      "branches.upstream.modal.current",
      "branches.upstream.modal.target",
      "branches.upstream.modal.target.placeholder",
      "branches.upstream.modal.invalid",
      "branches.upstream.modal.select.label",
      "branches.upstream.modal.search.placeholder",
      "branches.upstream.modal.empty",
      "branches.upstream.modal.cancel",
      "branches.upstream.modal.confirm",
      "branches.local.menu.upstream.edit",
      "branches.local.menu.upstream.unset",
      "branches.remote.menu.checkout",
      "branches.remote.menu.merge",
      "branches.remote.menu.rebase",
      "branches.remote.menu.copy",
      "branches.remote.menu.open.web",
      "branches.remote.menu.delete",
      "branches.remote.merge",
      "branches.remote.merge.hint",
      "branches.remote.rebase",
      "branches.remote.rebase.hint",
      "branches.remote.open.web.hint",
      "branches.remote.delete.hint",
      "branch.confirm.delete.remote",
      "branch.toast.delete.remote.success",
      "branch.toast.delete.remote.failed",
      "branch.toast.upstream.unset",
      "branch.toast.upstream.unset.failed",
    ];

    for (const key of requiredKeys) {
      test(`key "${key}" exists in zh-CN and en-US with non-empty translation`, () => {
        expect(key in zhCN).toBe(true);
        expect(key in enUS).toBe(true);
        expect((zhCN as any)[key].trim().length).toBeGreaterThan(0);
        expect((enUS as any)[key].trim().length).toBeGreaterThan(0);
      });
    }
  });
});
