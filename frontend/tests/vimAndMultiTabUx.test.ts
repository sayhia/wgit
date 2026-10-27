import { describe, expect, test, beforeEach } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import {
  isVimModeEnabled,
  currentVimMode,
  setVimModeEnabled,
  toggleVimMode,
  setVimInputMode,
  handleVimKeydown,
} from "../src/utils/vimNavigation";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const appVueFile = fileURLToPath(new URL("../src/App.vue", import.meta.url));
const workspaceViewFile = fileURLToPath(new URL("../src/components/WorkspaceView.vue", import.meta.url));
const shortcutsSectionFile = fileURLToPath(new URL("../src/components/settings/ShortcutsSection.vue", import.meta.url));
const commitComposerFile = fileURLToPath(new URL("../src/components/workspace/CommitComposer.vue", import.meta.url));

function mockKeyEvent(key: string, opts: Partial<KeyboardEvent> = {}): KeyboardEvent {
  return {
    key,
    metaKey: false,
    ctrlKey: false,
    altKey: false,
    preventDefault: () => {},
    ...opts,
  } as unknown as KeyboardEvent;
}

describe("Phase E1: Geek Vim Navigation Mode & Multi-Tab Docking UX", () => {
  beforeEach(() => {
    setVimModeEnabled(true);
    setVimInputMode("NORMAL");
  });

  describe("Vim Navigation State Machine", () => {
    test("initializes enabled and toggles properly", () => {
      expect(isVimModeEnabled.value).toBe(true);
      toggleVimMode();
      expect(isVimModeEnabled.value).toBe(false);
      toggleVimMode();
      expect(isVimModeEnabled.value).toBe(true);
    });

    test("handles normal navigation keys j and k", () => {
      let nextCalled = false;
      let prevCalled = false;

      const handlers = {
        onNext: () => { nextCalled = true; },
        onPrev: () => { prevCalled = true; },
      };

      const eventJ = mockKeyEvent("j");
      const handledJ = handleVimKeydown(eventJ, handlers);
      expect(handledJ).toBe(true);
      expect(nextCalled).toBe(true);

      const eventK = mockKeyEvent("k");
      const handledK = handleVimKeydown(eventK, handlers);
      expect(handledK).toBe(true);
      expect(prevCalled).toBe(true);
    });

    test("handles stage and discard shortcuts x, space, s, d", () => {
      let toggleStageCount = 0;
      let stageCount = 0;
      let discardCount = 0;

      const handlers = {
        onToggleStage: () => { toggleStageCount++; },
        onStage: () => { stageCount++; },
        onDiscard: () => { discardCount++; },
      };

      handleVimKeydown(mockKeyEvent(" "), handlers);
      expect(toggleStageCount).toBe(1);

      handleVimKeydown(mockKeyEvent("x"), handlers);
      expect(toggleStageCount).toBe(2);

      handleVimKeydown(mockKeyEvent("s"), handlers);
      expect(stageCount).toBe(1);

      handleVimKeydown(mockKeyEvent("d"), handlers);
      expect(discardCount).toBe(1);
    });

    test("enters insert mode on c or i, exits on Escape", () => {
      let focusCommitCalled = false;
      let exitInsertCalled = false;

      const handlers = {
        onFocusCommit: () => { focusCommitCalled = true; },
        onExitInsert: () => { exitInsertCalled = true; },
      };

      handleVimKeydown(mockKeyEvent("c"), handlers);
      expect(focusCommitCalled).toBe(true);
      expect(currentVimMode.value).toBe("INSERT");

      // While in insert mode in an input, Escape exits to NORMAL
      const inputEl = { tagName: "INPUT", blur: () => {} };
      const escEvent = mockKeyEvent("Escape", { target: inputEl as any });

      const handledEsc = handleVimKeydown(escEvent, handlers);
      expect(handledEsc).toBe(true);
      expect(currentVimMode.value).toBe("NORMAL");
      expect(exitInsertCalled).toBe(true);
    });

    test("does not intercept normal typing inside input or textarea", () => {
      const inputEl = { tagName: "INPUT" };
      const charEvent = mockKeyEvent("j", { target: inputEl as any });

      const handled = handleVimKeydown(charEvent, {});
      expect(handled).toBe(false);
      expect(currentVimMode.value).toBe("INSERT");
    });

    test("handles gt and gT tab switching sequences", () => {
      let nextTab = false;
      let prevTab = false;

      const handlers = {
        onNextTab: () => { nextTab = true; },
        onPrevTab: () => { prevTab = true; },
      };

      // Press 'g'
      handleVimKeydown(mockKeyEvent("g"), handlers);
      // Press 't'
      handleVimKeydown(mockKeyEvent("t"), handlers);
      expect(nextTab).toBe(true);

      // Press 'g'
      handleVimKeydown(mockKeyEvent("g"), handlers);
      // Press 'T'
      handleVimKeydown(mockKeyEvent("T"), handlers);
      expect(prevTab).toBe(true);
    });
  });

  describe("CommitComposer Component Focus Expose", () => {
    const src = readFileSync(commitComposerFile, "utf8");

    test("exposes focus method for Vim mode jumping", () => {
      expect(src).toContain("focusSubject");
      expect(src).toContain("defineExpose({");
      expect(src).toContain("focus: focusSubject");
      expect(src).toContain('ref="subjectInputRef"');
    });
  });

  describe("WorkspaceView Integration", () => {
    const src = readFileSync(workspaceViewFile, "utf8");

    test("imports and wires vimNavigation composable", () => {
      expect(src).toContain("isVimModeEnabled");
      expect(src).toContain("currentVimMode");
      expect(src).toContain("handleVimKeydown");
      expect(src).toContain("commitComposerRef");
    });

    test("renders visual mode indicator badge (-- NORMAL -- / -- INSERT --)", () => {
      expect(src).toContain("-- {{ currentVimMode }} --");
      expect(src).toContain("isVimModeEnabled");
    });
  });

  describe("App.vue Multi-Tab Docking & Global Shortcuts", () => {
    const src = readFileSync(appVueFile, "utf8");

    test("supports ⌘T to open new tab / repo dialog", () => {
      expect(src).toContain('key.toLowerCase() === "t"');
      expect(src).toContain("handleOpenDialog()");
    });

    test("supports ⌘W to close current repo tab", () => {
      expect(src).toContain('key.toLowerCase() === "w"');
      expect(src).toContain("closeRepo(activeRepoPath.value)");
    });

    test("implements selectNextRepoTab and selectPrevRepoTab for tab cycles", () => {
      expect(src).toContain("function selectNextRepoTab()");
      expect(src).toContain("function selectPrevRepoTab()");
      expect(src).toContain('"wgit:tab-next"');
      expect(src).toContain('"wgit:tab-prev"');
    });
  });

  describe("ShortcutsSection Vim Settings Card", () => {
    const src = readFileSync(shortcutsSectionFile, "utf8");

    test("renders Vim mode configuration switch and key guide", () => {
      expect(src).toContain('t("settings.shortcuts.vim.title")');
      expect(src).toContain("isVimModeEnabled");
      expect(src).toContain("toggleVimMode");
      expect(src).toContain('t("settings.shortcuts.vim.keys.jk")');
      expect(src).toContain('t("settings.shortcuts.vim.keys.stage")');
      expect(src).toContain('t("settings.shortcuts.vim.keys.discard")');
    });
  });

  describe("i18n Parity for Vim Mode Keys", () => {
    const vimKeys = [
      "settings.shortcuts.vim.title",
      "settings.shortcuts.vim.desc",
      "settings.shortcuts.vim.enabled",
      "settings.shortcuts.vim.disabled",
      "settings.shortcuts.vim.keys.jk",
      "settings.shortcuts.vim.keys.stage",
      "settings.shortcuts.vim.keys.hunk",
      "settings.shortcuts.vim.keys.discard",
      "settings.shortcuts.vim.keys.commit",
      "settings.shortcuts.vim.keys.tab",
      "settings.shortcuts.vim.keys.esc",
    ];

    test("zh-CN and en-US contain all required Vim keys", () => {
      for (const k of vimKeys) {
        expect(zhCN[k as keyof typeof zhCN]).toBeDefined();
        expect(enUS[k as keyof typeof enUS]).toBeDefined();
      }
    });
  });
});
