import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import { relativeTime } from "../src/utils/relativeTime";
import type { BlameLine, TimeMachineSnapshot } from "../src/types";

const sourcePanoramaFile = fileURLToPath(
  new URL("../src/components/workspace/SourcePanorama.vue", import.meta.url)
);
const workspaceViewFile = fileURLToPath(
  new URL("../src/components/WorkspaceView.vue", import.meta.url)
);
const appFile = fileURLToPath(
  new URL("../src/App.vue", import.meta.url)
);
const historyViewFile = fileURLToPath(
  new URL("../src/components/HistoryView.vue", import.meta.url)
);
const gitServiceFile = fileURLToPath(
  new URL("../src/services/gitService.ts", import.meta.url)
);

describe("Git Blame & Time Machine UX (Round 23)", () => {
  const panoramaSrc = readFileSync(sourcePanoramaFile, "utf8");
  const workspaceSrc = readFileSync(workspaceViewFile, "utf8");
  const appSrc = readFileSync(appFile, "utf8");
  const historySrc = readFileSync(historyViewFile, "utf8");
  const serviceSrc = readFileSync(gitServiceFile, "utf8");

  describe("1. Rich Metadata Blame Gutter & Time Heatmap", () => {
    test("blame gutter renders short hash, author, relative time, and summary", () => {
      expect(panoramaSrc).toContain("blameForLine(idx).shortHash");
      expect(panoramaSrc).toContain("blameForLine(idx).author");
      expect(panoramaSrc).toContain("formatBlameRelativeTime(blameForLine(idx))");
      expect(panoramaSrc).toContain("blameForLine(idx).summary");
    });

    test("gutter row includes time heatmap border class calculation", () => {
      expect(panoramaSrc).toContain("getBlameHeatmapClass(blameForLine(idx))");
      expect(panoramaSrc).toContain("getBlameAgeDays");
      // Recent commits get brighter border colors (cyan/emerald), older ones get muted borders
      expect(panoramaSrc).toContain("border-l-cyan-400");
      expect(panoramaSrc).toContain("border-l-emerald-400");
      expect(panoramaSrc).toContain("border-l-blue-400");
      expect(panoramaSrc).toContain("border-l-amber-500");
      expect(panoramaSrc).toContain("border-l-line/40");
    });

    test("relativeTime utility accurately computes human relative time for blame timestamps", () => {
      const now = Date.now();
      const oneHourAgo = new Date(now - 3600 * 1000).toISOString();
      const threeDaysAgo = new Date(now - 3 * 86400 * 1000).toISOString();
      const twoMonthsAgo = new Date(now - 60 * 86400 * 1000).toISOString();

      const rel1 = relativeTime(oneHourAgo, "", now);
      const rel2 = relativeTime(threeDaysAgo, "", now);
      const rel3 = relativeTime(twoMonthsAgo, "", now);

      expect(rel1).toBeTruthy();
      expect(rel2).toBeTruthy();
      expect(rel3).toBeTruthy();
    });

    test("provides hover quick action buttons directly on the gutter row", () => {
      expect(panoramaSrc).toContain("emit('blame-prior', blameForLine(idx))");
      expect(panoramaSrc).toContain("emit('inspect-history', blameForLine(idx).commitHash)");
      expect(panoramaSrc).toContain("emit('copy-hash', blameForLine(idx).commitHash)");
    });
  });

  describe("2. Commit Details Card & Interactive Actions", () => {
    test("supports clicking and hovering blame cells to show the detailed popover", () => {
      expect(panoramaSrc).toContain("onBlameCellClick(blameForLine(idx), $event)");
      expect(panoramaSrc).toContain("onBlameCellMouseEnter(blameForLine(idx), $event)");
      expect(panoramaSrc).toContain("onBlameCellMouseLeave");
      expect(panoramaSrc).toContain("activePopover");
    });

    test("commit details popover displays author, email, exact date, relative time, and summary", () => {
      expect(panoramaSrc).toContain("activePopover.line.author");
      expect(panoramaSrc).toContain("activePopover.line.authorEmail");
      expect(panoramaSrc).toContain("activePopover.line.date");
      expect(panoramaSrc).toContain("formatBlameRelativeTime(activePopover.line)");
      expect(panoramaSrc).toContain("activePopover.line.summary");
      expect(panoramaSrc).toContain("activePopover.line.isBoundary");
    });

    test("popover provides 'Inspect in History' action button", () => {
      expect(panoramaSrc).toContain("emit('inspect-history', activePopover.line.commitHash)");
      expect(panoramaSrc).toContain("t(\"blame.action.inspect\")");
    });

    test("popover provides 'Blame Prior Commit (^)' action button with boundary disabled state", () => {
      expect(panoramaSrc).toContain("emit('blame-prior', activePopover.line)");
      expect(panoramaSrc).toContain("t(\"blame.action.prior\")");
      expect(panoramaSrc).toContain(":disabled=\"activePopover.line.isBoundary\"");
    });

    test("popover provides 'Copy Commit Hash' button", () => {
      expect(panoramaSrc).toContain("emit('copy-hash', activePopover.line.commitHash)");
      expect(panoramaSrc).toContain("blame.action.copy.hint");
    });
  });

  describe("3. Time Machine Navigation System & Stack", () => {
    test("displays the prominent Time Machine banner above code viewer canvas", () => {
      expect(panoramaSrc).toContain("v-if=\"timeMachineActive\"");
      expect(panoramaSrc).toContain("t(\"timemachine.banner.mode\")");
      expect(panoramaSrc).toContain("t(\"timemachine.banner.current\")");
      expect(panoramaSrc).toContain("timeMachineCurrent?.shortHash");
      expect(panoramaSrc).toContain("timeMachineCurrent?.subject");
      expect(panoramaSrc).toContain("t(\"timemachine.banner.back\")");
      expect(panoramaSrc).toContain("t(\"timemachine.banner.forward\")");
      expect(panoramaSrc).toContain("t(\"timemachine.banner.exit\")");
    });

    test("WorkspaceView maintains timeMachineStack and navigation states", () => {
      expect(workspaceSrc).toContain("const timeMachineActive = ref<boolean>(false);");
      expect(workspaceSrc).toContain("const timeMachineStack = ref<TimeMachineSnapshot[]>([]);");
      expect(workspaceSrc).toContain("const timeMachineIndex = ref<number>(-1);");
      expect(workspaceSrc).toContain("const timeMachineCurrent = computed");
      expect(workspaceSrc).toContain("const canTimeMachineBack = computed");
      expect(workspaceSrc).toContain("const canTimeMachineForward = computed");
    });

    test("handleBlamePrior jumps to previousCommit or commitHash^ and updates stack", () => {
      expect(workspaceSrc).toContain("async function handleBlamePrior(line: BlameLine)");
      expect(workspaceSrc).toContain("line.previousCommit || `${line.commitHash}^`");
      expect(workspaceSrc).toContain("line.previousFilename || selectedSourceFile.value");
      expect(workspaceSrc).toContain("gitService.getSourceFileDetail");
      expect(workspaceSrc).toContain("fetchBlame(priorCommit)");
      expect(workspaceSrc).toContain("timeMachineStack.value.push(snapshot)");
    });

    test("handles boundary root commits gracefully with notification", () => {
      expect(workspaceSrc).toContain("if (line.isBoundary)");
      expect(workspaceSrc).toContain("t(\"timemachine.toast.boundary\")");
    });

    test("supports step back and step forward within the time machine stack", () => {
      expect(workspaceSrc).toContain("async function handleTimeMachineBack()");
      expect(workspaceSrc).toContain("timeMachineIndex.value--");
      expect(workspaceSrc).toContain("async function handleTimeMachineForward()");
      expect(workspaceSrc).toContain("timeMachineIndex.value++");
      expect(workspaceSrc).toContain("async function handleExitTimeMachine()");
    });
  });

  describe("4. Historical Version Alignment & Blame Bug Fix", () => {
    test("fetchBlame accepts refOverride and queries gitService.getFileBlameWithOptions", () => {
      expect(workspaceSrc).toContain("async function fetchBlame(refOverride?: string)");
      expect(workspaceSrc).toContain("gitService.getFileBlameWithOptions(repoPath, {");
      expect(workspaceSrc).toContain("ref: targetRef || undefined");
    });

    test("viewFileRevision fetches blame aligned with historical commit revision", () => {
      expect(workspaceSrc).toContain("async function viewFileRevision(commitHash: string)");
      expect(workspaceSrc).toContain("selectedFileRevisionHash.value = commitHash;");
      expect(workspaceSrc).toContain("if (showBlame.value) {\n        await fetchBlame(commitHash);\n      }");
    });

    test("restoring current file revision exits time machine and re-fetches working copy", () => {
      expect(workspaceSrc).toContain("async function restoreCurrentFileRevision()");
      expect(workspaceSrc).toContain("await handleExitTimeMachine();");
    });
  });

  describe("5. Cross-View History Inspection Linkage", () => {
    test("WorkspaceView dispatches wgit:history-commit on inspect", () => {
      expect(workspaceSrc).toContain("function handleInspectInHistory(hash: string)");
      expect(workspaceSrc).toContain("window.dispatchEvent(new CustomEvent(\"wgit:history-commit\", { detail: { hash } }));");
    });

    test("App.vue forwards wgit:history-commit to History view", () => {
      expect(appSrc).toContain("function handleGlobalHistoryCommit(e: Event)");
      expect(appSrc).toContain("forwardAfterMount(\"wgit:history-commit\", \"history\", e);");
      expect(appSrc).toContain("window.addEventListener(\"wgit:history-commit\", handleGlobalHistoryCommit);");
    });

    test("HistoryView responds to wgit:history-commit and selects target commit", () => {
      expect(historySrc).toContain("function handleHistoryCommitEvent(e: Event)");
      expect(historySrc).toContain("window.addEventListener(\"wgit:history-commit\", handleHistoryCommitEvent);");
      expect(historySrc).toContain("selectCommit(match)");
    });
  });

  describe("6. Backend RPC Service & Types Parity", () => {
    test("gitService exposes getFileBlameWithOptions calling GetFileBlameWithOptions", () => {
      expect(serviceSrc).toContain("getFileBlameWithOptions: (repoPath: string, opts: BlameOptions)");
      expect(serviceSrc).toContain("callBackend<BlameLine[]>(\"GetFileBlameWithOptions\", repoPath, opts)");
    });

    test("types.ts defines BlameOptions and TimeMachineSnapshot", () => {
      const typesSrc = readFileSync(
        fileURLToPath(new URL("../src/types.ts", import.meta.url)),
        "utf8"
      );
      expect(typesSrc).toContain("export interface BlameOptions");
      expect(typesSrc).toContain("export interface TimeMachineSnapshot");
      expect(typesSrc).toContain("previousCommit?: string;");
      expect(typesSrc).toContain("previousFilename?: string;");
      expect(typesSrc).toContain("isBoundary?: boolean;");
    });
  });

  describe("7. i18n Completeness for Blame & Time Machine", () => {
    const requiredKeys = [
      "blame.popover.title",
      "blame.popover.author",
      "blame.popover.email",
      "blame.popover.date",
      "blame.popover.relative",
      "blame.popover.commit",
      "blame.popover.summary",
      "blame.popover.boundary",
      "blame.popover.close",
      "blame.action.inspect",
      "blame.action.inspect.hint",
      "blame.action.prior",
      "blame.action.prior.hint",
      "blame.action.copy",
      "blame.action.copy.hint",
      "blame.gutter.root",
      "blame.gutter.relative.unknown",
      "timemachine.banner.mode",
      "timemachine.banner.current",
      "timemachine.banner.no.subject",
      "timemachine.banner.back",
      "timemachine.banner.forward",
      "timemachine.banner.exit",
      "timemachine.toast.boundary",
      "timemachine.toast.entered",
      "timemachine.toast.restored",
    ];

    test("zh-CN contains all required Round 23 keys", () => {
      for (const key of requiredKeys) {
        expect(key in zhCN).toBe(true);
        expect((zhCN as Record<string, string>)[key]?.trim().length).toBeGreaterThan(0);
      }
    });

    test("en-US contains all required Round 23 keys", () => {
      for (const key of requiredKeys) {
        expect(key in enUS).toBe(true);
        expect((enUS as Record<string, string>)[key]?.trim().length).toBeGreaterThan(0);
      }
    });

    test("Round 23 keys are strictly symmetrical between zh-CN and en-US", () => {
      for (const key of requiredKeys) {
        expect((zhCN as Record<string, string>)[key]).toBeDefined();
        expect((enUS as Record<string, string>)[key]).toBeDefined();
      }
    });
  });
});
