import { describe, expect, test } from "bun:test";
import { readFileSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const worktreesTabFile = fileURLToPath(
  new URL("../src/components/tools/WorktreesTab.vue", import.meta.url)
);
const toolsViewFile = fileURLToPath(
  new URL("../src/components/ToolsView.vue", import.meta.url)
);
const remotesTabFile = fileURLToPath(
  new URL("../src/components/tools/RemotesTab.vue", import.meta.url)
);

describe("Git Worktree Parallel Switchboard & UX Upgrade (Round 21)", () => {
  test("WorktreesTab.vue component file exists", () => {
    expect(existsSync(worktreesTabFile)).toBe(true);
  });

  const worktreesSrc = readFileSync(worktreesTabFile, "utf8");
  const toolsViewSrc = readFileSync(toolsViewFile, "utf8");
  const remotesSrc = readFileSync(remotesTabFile, "utf8");

  describe("1. Decoupled Architecture & ToolsView Integration", () => {
    test("ToolsView dynamically imports and mounts WorktreesTab for activeSubTab === 'worktrees'", () => {
      expect(toolsViewSrc).toContain('import("./tools/WorktreesTab.vue")');
      expect(toolsViewSrc).toContain("v-if=\"activeSubTab === 'worktrees'\"");
      expect(toolsViewSrc).toContain("<WorktreesTab");
      expect(toolsViewSrc).toContain(':current-repo-path="currentRepoPath"');
      expect(toolsViewSrc).toContain("@open-repo=\"emit('open-repo', $event)\"");
      expect(toolsViewSrc).toContain("@notify=");
    });

    test("RemotesTab.vue has worktree mixed logic completely removed", () => {
      expect(remotesSrc).not.toContain("mode === 'worktrees'");
      expect(remotesSrc).not.toContain("gitService.getWorktrees");
      expect(remotesSrc).not.toContain("gitService.addWorktree");
      expect(remotesSrc).not.toContain("gitService.removeWorktree");
      expect(remotesSrc).not.toContain("gitService.pruneWorktrees");
      expect(remotesSrc).not.toContain("showAddWorktreeModal");
      expect(remotesSrc).not.toContain("showLockWorktreeModal");
    });

    test("WorktreesTab has independent data-tool-section anchor and status tracking", () => {
      expect(worktreesSrc).toContain('data-tool-section="worktrees"');
      expect(worktreesSrc).toContain("toolsStatus.loading.worktrees");
      expect(worktreesSrc).toContain("toolsStatus.counts.worktrees");
    });
  });

  describe("2. Prune Confirmation Protection & Bug Fix", () => {
    test("handlePruneWorktrees exits early when confirmation dialog is cancelled", () => {
      expect(worktreesSrc).toContain("handlePruneWorktrees");
      expect(worktreesSrc).toContain("confirmAction");
      expect(worktreesSrc).toContain("worktrees.confirm.prune.stale");
      // Critical bug fix: must include { return; } or return when confirmAction is false
      expect(worktreesSrc).toMatch(
        /if\s*\(!\(await\s+confirmAction\([\s\S]*?\)\)\)\s*{\s*return;\s*}/
      );
    });

    test("pruneWorktrees executes only after confirmed", () => {
      expect(worktreesSrc).toContain("gitService.pruneWorktrees(props.currentRepoPath)");
      expect(worktreesSrc).toContain("gitService.repairWorktrees(props.currentRepoPath");
    });
  });

  describe("3. Active Worktree Safety Guard & Main Worktree Switchback", () => {
    test("defines isCurrentActive path comparison helper", () => {
      expect(worktreesSrc).toContain("function isCurrentActive");
      expect(worktreesSrc).toContain("normalizePath");
    });

    test("renders active badge and disables removal for current active worktree", () => {
      expect(worktreesSrc).toContain("isCurrentActive(w.path)");
      expect(worktreesSrc).toContain("worktrees.status.active");
      expect(worktreesSrc).toContain("worktrees.cannot.remove.active");
      expect(worktreesSrc).toMatch(/isCurrentActive\(w\.path\)[\s\S]*?disabled/);
    });

    test("handleRemoveWorktree prevents deletion of active worktree at runtime", () => {
      expect(worktreesSrc).toContain("handleRemoveWorktree");
      expect(worktreesSrc).toContain("worktrees.toast.remove.active.forbidden");
      expect(worktreesSrc).toMatch(
        /if\s*\(isCurrentActive\(path\)\)\s*{\s*notify\(t\("worktrees\.toast\.remove\.active\.forbidden"\)\);\s*return;\s*}/
      );
    });

    test("allows opening all non-active worktrees in workspace including main worktree", () => {
      // Must NOT be restricted by !w.isMain; must use !isCurrentActive(w.path)
      expect(worktreesSrc).toContain("!isCurrentActive(w.path)");
      expect(worktreesSrc).toContain("handleOpenWorktreeAsTab(w.path)");
      expect(worktreesSrc).toContain("worktrees.open.tab");
    });
  });

  describe("4. Dirty Worktree Precheck & Directory Picker", () => {
    test("handleRemoveWorktree checks dirty status before deletion", () => {
      expect(worktreesSrc).toContain("gitService.getWorktreeStatus(props.currentRepoPath, path)");
      expect(worktreesSrc).toContain("status.isDirty");
      expect(worktreesSrc).toContain("status.dirtyCount");
      expect(worktreesSrc).toContain("worktrees.confirm.remove.dirty");
    });

    test("provides folder browser dialog in Add modal", () => {
      expect(worktreesSrc).toContain("handleBrowseWorktreePath");
      expect(worktreesSrc).toContain("gitService.pickFolderDialog()");
      expect(worktreesSrc).toContain("worktrees.modal.browse");
    });

    test("supports advanced worktree creation options", () => {
      expect(worktreesSrc).toContain("handleAddWorktreeAdvanced");
      expect(worktreesSrc).toContain("gitService.addWorktreeAdvanced");
      expect(worktreesSrc).toContain("addMode");
      expect(worktreesSrc).toContain("opts.newBranch");
      expect(worktreesSrc).toContain("opts.baseRef");
      expect(worktreesSrc).toContain("opts.detach");
    });
  });

  describe("5. Stale Prune & Lock/Unlock Integrations", () => {
    test("marks prunable worktrees and provides single prune action", () => {
      expect(worktreesSrc).toContain("w.isPrunable");
      expect(worktreesSrc).toContain("worktrees.status.prunable");
      expect(worktreesSrc).toContain("handlePruneSingle");
      expect(worktreesSrc).toContain("gitService.pruneSingleWorktree");
    });

    test("provides lock and unlock actions with reason support", () => {
      expect(worktreesSrc).toContain("w.isLocked");
      expect(worktreesSrc).toContain("w.lockReason");
      expect(worktreesSrc).toContain("handleLockWorktree");
      expect(worktreesSrc).toContain("handleUnlockWorktree");
      expect(worktreesSrc).toContain("gitService.lockWorktree");
      expect(worktreesSrc).toContain("gitService.unlockWorktree");
    });
  });

  describe("6. i18n Dictionary Completeness & Symmetry", () => {
    const requiredKeys = [
      "worktrees.title",
      "worktrees.desc",
      "worktrees.prune",
      "worktrees.prune.title",
      "worktrees.repair",
      "worktrees.repair.title",
      "worktrees.add",
      "worktrees.total",
      "worktrees.count",
      "worktrees.linked",
      "worktrees.locked.state",
      "worktrees.locked.count",
      "worktrees.prunable.count",
      "worktrees.main",
      "worktrees.status.active",
      "worktrees.status.prunable",
      "worktrees.cannot.remove.active",
      "worktrees.locked.default",
      "worktrees.prunable.desc",
      "worktrees.dirty.count",
      "worktrees.copy.path",
      "worktrees.path.label",
      "worktrees.open.tab",
      "worktrees.open.tab.title",
      "worktrees.open.terminal",
      "worktrees.open.finder",
      "worktrees.lock.aria",
      "worktrees.unlock.aria",
      "worktrees.remove.aria",
      "worktrees.prune.single.title",
      "worktrees.empty",
      "worktrees.modal.add.label",
      "worktrees.modal.add.title",
      "worktrees.modal.path",
      "worktrees.modal.path.aria",
      "worktrees.modal.path.placeholder",
      "worktrees.modal.browse",
      "worktrees.modal.mode.label",
      "worktrees.modal.mode.existing",
      "worktrees.modal.mode.new",
      "worktrees.modal.mode.detach",
      "worktrees.modal.branch.select",
      "worktrees.modal.branch.select.placeholder",
      "worktrees.modal.branch.name",
      "worktrees.modal.branch.name.placeholder",
      "worktrees.modal.base.ref",
      "worktrees.modal.base.ref.placeholder",
      "worktrees.modal.open.tab",
      "worktrees.modal.creating",
      "worktrees.modal.confirm",
      "worktrees.modal.lock.label",
      "worktrees.modal.lock.title",
      "worktrees.modal.lock.target",
      "worktrees.modal.lock.reason",
      "worktrees.modal.lock.reason.aria",
      "worktrees.modal.lock.reason.placeholder",
      "worktrees.modal.locking",
      "worktrees.modal.lock.confirm",
      "worktrees.confirm.prune.stale",
      "worktrees.confirm.prune.single",
      "worktrees.confirm.remove",
      "worktrees.confirm.remove.dirty",
      "worktrees.confirm.remove.force",
      "worktrees.toast.added",
      "worktrees.toast.add.failed",
      "worktrees.toast.load.failed",
      "worktrees.toast.removed",
      "worktrees.toast.remove.failed",
      "worktrees.toast.remove.active.forbidden",
      "worktrees.toast.locked",
      "worktrees.toast.lock.failed",
      "worktrees.toast.unlocked",
      "worktrees.toast.unlock.failed",
      "worktrees.toast.prune.stale.failed",
      "worktrees.toast.prune.single.success",
      "worktrees.toast.prune.single.failed",
      "worktrees.toast.repair.failed",
      "worktrees.toast.pick.folder.failed",
      "worktrees.toast.copied",
      "worktrees.toast.terminal.failed",
      "worktrees.toast.finder.failed",
    ];

    test("zh-CN contains all required worktree keys", () => {
      const catalog = zhCN as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].trim().length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required worktree keys symmetrically", () => {
      const catalog = enUS as Record<string, string>;
      for (const key of requiredKeys) {
        expect(catalog[key]).toBeDefined();
        expect(catalog[key].trim().length).toBeGreaterThan(0);
      }
    });
  });
});
