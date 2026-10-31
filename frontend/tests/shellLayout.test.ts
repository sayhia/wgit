import { beforeEach, describe, expect, spyOn, test } from "bun:test";
import { mount } from "@vue/test-utils";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import HeaderBar from "../src/components/HeaderBar.vue";
import { gitService } from "../src/services/gitService";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import "./domSetup";

/**
 * Global shell layout rules.
 *
 * These are the regressions that only show up when the window is small (the
 * audits drive a 1500x1000 window, so they never caught any of it):
 *
 *   1. the header's command-palette label wrapped instead of truncating, which
 *      pushed the 48px bar out of shape and clipped the sync cluster;
 *   2. the sync cluster had no collapsed form, so below ~1280 the right side of
 *      the header ran past the viewport edge and was cut off by `overflow-hidden`;
 *   3. the three workspace panes had fixed widths, so on a narrow window the
 *      diff canvas was starved to a few dozen pixels and its toolbar wrapped
 *      one glyph per line;
 *   4. the rail's dirty-summary label wrapped onto three lines inside the 36px
 *      rail, and the repo-health dot sat on top of the tab's close button.
 *
 * Source-level assertions are the only way to check Tailwind breakpoints here:
 * Bun has no layout engine, so a mount test can't measure what the classes do.
 */

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (rel: string) => readFileSync(join(root, rel), "utf8");

const headerSrc = read("src/components/HeaderBar.vue");
const railSrc = read("src/components/RepoRail.vue");
const workspaceSrc = read("src/components/WorkspaceView.vue");
const settingsSrc = read("src/components/SettingsView.vue");

/** The template block of an SFC (assertions below are all about markup). */
function templateOf(source: string): string {
  const start = source.indexOf("<template>");
  expect(start).toBeGreaterThanOrEqual(0);
  return source.slice(start);
}

describe("HeaderBar: nothing may wrap or clip", () => {
  test("the header never clips its own dropdowns", () => {
    const header = templateOf(headerSrc).match(/<header[\s\S]*?>/);
    expect(header).not.toBeNull();
    expect(header![0]).toContain("flex items-center gap-2");
    // The repo / branch / context-session menus are absolutely positioned
    // children anchored below the 48px bar. `overflow-hidden` silently clipped
    // all of them (and the narrow-window sync menu), so the buttons did nothing.
    expect(header![0]).toContain("overflow-visible");
    expect(header![0]).not.toContain("overflow-hidden");
  });

  test("header dropdowns are coordinated: one open, outside click and Escape close", () => {
    expect(headerSrc).toContain("function closeAllHeaderMenus()");
    expect(headerSrc).toContain("function toggleHeaderMenu(");
    expect(headerSrc).toContain('document.addEventListener("mousedown", handleHeaderPointerDown)');
    expect(headerSrc).toContain('window.addEventListener("keydown", handleHeaderKeydown)');
    expect(headerSrc).toContain('if (event.key !== "Escape" || !hasOpenHeaderMenu()) return;');
    expect(headerSrc).toContain("lastMenuTrigger");
    expect(headerSrc).toContain("target.focus()");
    // A click on another header action (sync/palette/…) dismisses the panel
    // instead of leaving it floating over the dialog that action opens.
    expect(headerSrc).toContain('@click.capture="handleHeaderClickCapture"');
    expect(headerSrc).toContain("if (target?.closest(\"[data-header-menu], [data-header-menu-trigger]\")) return;");
    expect(headerSrc).toContain("data-header-menu-trigger");
    // Triggers must go through the coordinator, not toggle their own boolean.
    const template = templateOf(headerSrc);
    expect(template).toContain('@click="toggleHeaderMenu(\'repo\')"');
    expect(template).toContain('@click="toggleHeaderMenu(\'branch\')"');
    expect(template).toContain('@click="toggleHeaderMenu(\'context\')"');
    expect(template).toContain('@click="toggleHeaderMenu(\'action\')"');
  });

  test("the command palette trigger truncates its label instead of wrapping", () => {
    const button = templateOf(headerSrc).match(
      /<button\s+@click="emit\('toggle-command-palette'\)"[\s\S]*?<\/button>/
    );
    expect(button).not.toBeNull();
    expect(button![0]).toContain("whitespace-nowrap");
    expect(button![0]).toContain("min-w-0");
    // Label and kbd only exist once there is room for them; the icon stays.
    expect(button![0]).toContain('class="hidden lg:inline truncate');
    expect(button![0]).toContain('class="hidden lg:flex');
  });

  test("repo and branch pills shrink before the sync cluster does", () => {
    expect(headerSrc).toContain("max-w-[88px] xl:max-w-[160px] truncate");
    expect(headerSrc).toContain("max-w-[72px] xl:max-w-[130px] truncate");
  });

  test("fetch / pull / rebase collapse into one overflow menu below xl", () => {
    // Wide-only cluster…
    expect(headerSrc).toContain('class="hidden xl:flex items-center gap-1.5"');
    // …and the narrow replacement, which re-emits the same events.
    expect(headerSrc).toContain('data-testid="header-overflow-trigger"');
    expect(headerSrc).toContain('data-testid="header-overflow-menu"');
    for (const event of ["sync-fetch", "sync-pull", "open-rebase", "refresh"]) {
      expect(headerSrc).toContain(`emit('${event}')`);
    }
    // Push is the primary action: it never gets folded away.
    const pushBlock = headerSrc.slice(headerSrc.indexOf("<!-- Push Split Button -->"));
    expect(pushBlock.slice(0, 400)).not.toContain("hidden xl:");
  });
});

describe("HeaderBar overflow menu: behaviour", () => {
  // The header probes recent repos on mount; stub it so the Wails runtime is
  // never touched from a Bun component test.
  beforeEach(() => {
    spyOn(gitService, "getRecentRepos").mockResolvedValue(["/test/repo"]);
  });

  test("opens on click and closes after an action fires", async () => {
    const wrapper = mount(HeaderBar, {
      props: { currentRepoPath: "/test/repo", repoStatus: null, isSyncing: false },
    });
    expect(wrapper.find('[data-testid="header-overflow-menu"]').exists()).toBe(false);

    await wrapper.find('[data-testid="header-overflow-trigger"]').trigger("click");
    const menu = wrapper.find('[data-testid="header-overflow-menu"]');
    expect(menu.exists()).toBe(true);

    // Menu entries mirror the wide cluster, one per action.
    const items = menu.findAll("button");
    expect(items.length).toBe(4);

    await items[0]!.trigger("click");
    expect(wrapper.emitted("sync-fetch")).toBeTruthy();
    expect(wrapper.find('[data-testid="header-overflow-menu"]').exists()).toBe(false);
  });

  test("the trigger reports its expanded state for assistive tech", async () => {
    const wrapper = mount(HeaderBar, {
      props: { currentRepoPath: "/test/repo", repoStatus: null, isSyncing: false },
    });
    const trigger = wrapper.find('[data-testid="header-overflow-trigger"]');
    expect(trigger.attributes("aria-expanded")).toBe("false");
    await trigger.trigger("click");
    expect(trigger.attributes("aria-expanded")).toBe("true");
    expect(trigger.attributes("aria-label")).toBe(zhCN["header.overflow.title"]);
  });

  test("overflow menu keys exist in both locales", () => {
    for (const key of ["header.overflow.title"] as const) {
      expect(zhCN[key]).toBeTruthy();
      expect(enUS[key]).toBeTruthy();
    }
  });
});

describe("WorkspaceView: panes survive a narrow window", () => {
  test("pane widths are measured on the workspace element, not the window", () => {
    expect(workspaceSrc).toContain("workspaceRootRef");
    expect(workspaceSrc).toContain("new ResizeObserver(");
    expect(workspaceSrc).toContain("workspaceResizeObserver.observe(workspaceRootRef.value)");
    expect(workspaceSrc).toContain("workspaceResizeObserver?.disconnect()");
  });

  test("the source pane starts hidden and remembers an explicit toggle", () => {
    expect(workspaceSrc).toContain('SOURCE_PANE_STORAGE_KEY = "wgit.workspace.sourcePane.v1"');
    expect(workspaceSrc).toContain('localStorage.getItem(SOURCE_PANE_STORAGE_KEY) === "shown"');
    expect(workspaceSrc).toContain('data-testid="ws-source-pane-toggle"');
    expect(workspaceSrc).toContain(":aria-pressed=\"showSourcePane\"");
    // The tree keeps its state while hidden, and its divider goes with it.
    expect(workspaceSrc).toContain('v-show="showSourcePane"');
    expect((workspaceSrc.match(/v-show="showSourcePane"/g) ?? []).length).toBe(2);
  });

  test("below 900px the changes list and the diff take turns", () => {
    expect(workspaceSrc).toContain("STACK_PANES_BELOW = 900");
    expect(workspaceSrc).toContain('data-testid="ws-stacked-pane-switch"');
    // Picking a file is the cue to reveal the diff again.
    const selectFile = workspaceSrc.slice(workspaceSrc.indexOf("async function selectChangeFile"));
    expect(selectFile.slice(0, 400)).toContain('if (isStackedPanes.value) stackedPane.value = "diff"');
  });

  test("both panes stay mounted while stacked", () => {
    // Unmounting would drop the list's groups/filters and the diff's scroll position.
    expect(workspaceSrc).toContain("hidden: isStackedPanes && stackedPane === 'diff'");
    expect(workspaceSrc).toContain(":class=\"{ hidden: isStackedPanes && stackedPane === 'changes' }\"");
    // A lone pane fills the width instead of keeping its pixel width.
    expect(workspaceSrc).toContain("'flex-1 min-w-0 border-r-0': isStackedPanes");
  });

  test("a pane can never be rendered wider than the space it sits in", () => {
    expect(workspaceSrc).toContain("function fitPaneWidth(");
    expect(workspaceSrc).toContain("effectiveLeftPaneWidth");
    expect(workspaceSrc).toContain("effectiveChangesPaneWidth");
    // The stored preference is untouched so widening the window restores it.
    expect(workspaceSrc).toContain("return Math.min(preferred, ceiling);");
  });

  test("the tab strip never scrolls out of reach behind the action cluster", () => {
    // Bar clips; the action cluster owns the horizontal scroll instead.
    expect(workspaceSrc).toContain("bg-surface shrink-0 overflow-hidden");
    // The scrollable content starts at the cluster's first control. Plain
    // `justify-end` + `overflow-x-auto` overflows to the left, which painted
    // 「暂存 1」 underneath the tab strip and made it unreachable.
    expect(workspaceSrc).toContain("flex w-max min-w-full items-center justify-end gap-2");
    expect(workspaceSrc).not.toContain("flex-1 min-w-0 flex items-center justify-end gap-2");
  });

  test("the top bar adapts to the pane width, not the window width", () => {
    // The source tree can be open while the window is wide; viewport `xl:`
    // classes then showed labels the ~560px action cluster could not hold.
    expect(workspaceSrc).toContain("@container h-11");
    expect(workspaceSrc).toContain("hidden @min-[700px]:flex items-center gap-2"); // staged/unstaged counters
    expect(workspaceSrc).toContain("hidden @min-[940px]:flex items-center gap-0.5"); // layout presets
  });

  test("pane-switch labels exist in both locales", () => {
    for (const key of [
      "ws.pane.source.show",
      "ws.pane.source.hide",
      "ws.pane.switch",
      "ws.pane.changes",
      "ws.pane.diff",
    ] as const) {
      expect(zhCN[key]).toBeTruthy();
      expect(enUS[key]).toBeTruthy();
    }
  });
});

describe("RepoRail: 36px row stays one line", () => {
  test("the dirty summary is nowrap and drops out first", () => {
    expect(railSrc).toContain("hidden lg:inline text-[10px] text-warning font-medium mr-1 whitespace-nowrap");
  });

  test("action buttons never wrap their labels", () => {
    const actions = railSrc.slice(railSrc.indexOf("<!-- actions -->"));
    const buttons = actions.slice(0, actions.indexOf("<!-- Tab Context Menu -->"));
    expect((buttons.match(/whitespace-nowrap/g) ?? []).length).toBeGreaterThanOrEqual(3);
  });

  test("the health dot is docked on the tab icon, clear of the close button", () => {
    // Corner-pinned it either covered the close button or floated on the rail border.
    expect(railSrc).toContain("repo-health-badge absolute left-2 top-0.5 z-10");
  });
});

describe("SettingsView: content uses its pane", () => {
  test("the full-width column keeps its own scrolling body", () => {
    expect(settingsSrc).toContain('<div class="w-full min-w-0">');
    expect(settingsSrc).toContain("overflow-y-auto p-4 md:p-6 lg:p-8");
    // The scroll container must no longer carry the width cap itself.
    expect(settingsSrc).not.toContain('class="flex-1 overflow-y-auto p-8 max-w-3xl"');
  });
});

describe("HistoryView: the commit column keeps its title", () => {
  const historySrc = read("src/components/HistoryView.vue");

  test("secondary traversal tools fold into a menu on a narrow commit column", () => {
    // The column is ~375px narrower than the window once the inspector is
    // open, so this must key off the column width (`@container`) rather than
    // the viewport: at a 1440px window `xl:` used to show every label and
    // squeeze the title to one glyph.
    expect(historySrc).toContain("@container relative z-20 h-11");
    expect(historySrc).toContain('class="hidden @min-[900px]:flex items-center gap-2"');
    // … narrow: one trigger, same four actions.
    expect(historySrc).toContain('data-testid="history-overflow-trigger"');
    expect(historySrc).toContain('data-testid="history-overflow-menu"');
    for (const action of [
      "showRangeCherryPickDialog = true",
      "openBisectConsole()",
      "showRebaseKanbanModal = true",
      "showBatchCherryPickDialog = true",
    ]) {
      // Once inline, once in the menu.
      expect((historySrc.match(new RegExp(action.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "g")) ?? []).length)
        .toBeGreaterThanOrEqual(2);
    }
  });

  test("the title truncates instead of being sliced mid-glyph", () => {
    expect(historySrc).toContain('<span class="text-xs font-semibold text-ink truncate">{{ t("history.title") }}</span>');
    // Counts and topology stats are secondary: they leave the bar first.
    expect(historySrc).toContain('class="hidden @min-[760px]:inline text-[10px] text-ink-muted font-mono font-normal truncate"');
    expect(historySrc).toContain('class="hidden @min-[900px]:inline px-1.5 py-0.5 rounded bg-surface border border-line');
  });

  test("the overflow label exists in both locales", () => {
    expect(zhCN["history.overflow.title" as keyof typeof zhCN]).toBeTruthy();
    expect(enUS["history.overflow.title" as keyof typeof enUS]).toBeTruthy();
  });
});

/**
 * Toolbars that live inside a resizable pane (or a fixed-width sidebar) must
 * respond to their own width, not the window's. Two real regressions motivated
 * these: with the source tree open at a 1280px window the diff toolbar wrapped
 * its labels mid-word (「补丁试验 / 场」), and the 320px stash sidebar wrapped
 * its title across two lines while pushing the refresh icon into the divider.
 */
describe("Global layout: toolbars respond to their own pane", () => {
  const diffSrc = read("src/components/workspace/DiffViewer.vue");
  const composerSrc = read("src/components/workspace/CommitComposer.vue");
  const changesSrc = read("src/components/workspace/ChangesList.vue");
  const stashSrc = read("src/components/StashView.vue");
  const reviewSrc = read("src/components/ReviewView.vue");
  const toolsSrc = read("src/components/ToolsView.vue");
  const branchesSrc = read("src/components/BranchesView.vue");
  const appearanceSrc = read("src/components/settings/AppearanceSection.vue");

  test("diff toolbar labels collapse by pane width and never break mid-word", () => {
    expect(diffSrc).toContain("@container relative z-20 mx-2 mt-2 flex flex-wrap");
    expect(diffSrc).toContain("@max-[640px]:hidden");
    expect(diffSrc).toContain("flex shrink-0 items-center gap-1.5 whitespace-nowrap");
    // The icon-only state must stay announced.
    expect(diffSrc).toContain(':aria-label="t(\'workspace.diff.playground.btn\')"');
  });

  test("conventional types live in a compact trigger plus menu", () => {
    expect(composerSrc).toContain('data-testid="commit-type-trigger"');
    expect(composerSrc).toContain('data-testid="commit-type-menu"');
    expect(composerSrc).toContain('data-testid="commit-auto-format-btn"');
    // The nine colored tone chips are gone; selection is the only accent.
    expect(composerSrc).not.toContain("item.tone +");
  });

  test("secondary composer actions live behind one tools menu", () => {
    expect(composerSrc).toContain('data-testid="commit-tools-btn"');
    expect(composerSrc).toContain('data-testid="commit-tools-menu"');
    expect(composerSrc).toContain("emit('fast-commit')");
    expect(composerSrc).toContain("emit('audit-sentinel')");
  });

  test("changes filters stay reachable instead of being stranded off-screen", () => {
    expect(changesSrc).not.toContain('overflow-x-auto no-scrollbar');
    const status = changesSrc.slice(changesSrc.indexOf("changes-status-filters") - 400, changesSrc.indexOf("changes-status-filters"));
    expect(status).toContain("flex flex-wrap");
    // Every extension is one click away in the menu instead of a chip wall.
    const extStart = changesSrc.indexOf("changes-ext-filters");
    expect(changesSrc.slice(extStart, extStart + 320)).toContain("flex flex-wrap");
    expect(changesSrc).toContain('data-testid="changes-ext-trigger"');
    expect(changesSrc).toContain('data-testid="ext-chip-all"');
  });

  test("stash sidebar header is two rows with icon actions, not a squeezed single row", () => {
    expect(stashSrc).toContain('data-testid="create-micro-snapshot-btn"');
    // Secondary actions lost their viewport-based labels; icons keep title+aria.
    expect(stashSrc).not.toContain('<span class="hidden sm:inline">{{ t("stash.snapshot.btn") }}</span>');
    expect(stashSrc).toContain(':aria-label="t(\'stash.snapshot.btn\')"');
    expect(stashSrc).toContain(':aria-label="t(\'stash.comparator.title\')"');
    expect(stashSrc).toContain('<div class="flex items-center gap-1.5">\n          <div class="relative flex-1 min-w-0">');
  });

  test("review header wraps and collapses labels instead of clipping the last action", () => {
    expect(reviewSrc).toContain("@container min-h-12 shrink-0 border-b");
    expect(reviewSrc).toContain("flex flex-wrap items-center gap-x-3 gap-y-1.5");
    expect(reviewSrc).toContain('<span class="hidden @min-[1100px]:inline">{{ t("review.export") }}</span>');
  });

  test("tools tab labels collapse before the strip can clip a tab", () => {
    expect(toolsSrc).toContain('<span class="hidden xl:inline">{{ t(def.labelKey) }}</span>');
    expect(toolsSrc).toContain(':aria-label="t(def.labelKey)"');
  });

  test("capped views keep the scrollbar on the pane edge", () => {
    // The width cap belongs to the inner column; the scroll container stays
    // full-width so the scrollbar is not stranded mid-screen at 1920px.
    expect(branchesSrc).toContain('<div class="flex-1 overflow-y-auto">');
    expect(branchesSrc).toContain('<div class="p-4 max-w-5xl mx-auto w-full space-y-2">');
    expect(branchesSrc).not.toContain('overflow-y-auto p-4 max-w-5xl mx-auto');
    expect(toolsSrc).toContain('<div :key="`${currentRepoPath}:${activeSubTab}`" class="flex-1 min-w-0 min-h-0 overflow-y-auto">');
    expect(toolsSrc).toContain('<div class="p-4 lg:p-6 max-w-6xl mx-auto w-full min-w-0">');
    expect(toolsSrc).not.toContain('overflow-y-auto p-6 max-w-6xl mx-auto');
  });

  test("review session cards keep base/head on their own row", () => {
    const sessionSrc = read("src/components/review/SessionList.vue");
    // The verdict chip moved to the metadata row; the range must not share its
    // line, otherwise the labels collapse to 「orig… → H…」 in a 190px card.
    expect(sessionSrc).toContain('<div class="flex items-center gap-1 min-w-0">');
    expect(sessionSrc).toContain("flex flex-wrap items-center gap-x-1.5 gap-y-1 text-[9px] font-mono text-ink-muted");
  });

  test("review file filters wrap as whole chips instead of squeezing", () => {
    const changeListSrc = read("src/components/review/ChangeList.vue");
    expect(changeListSrc).toContain("flex flex-wrap items-center gap-1 text-[10px]");
    expect(changeListSrc).toContain("font-medium whitespace-nowrap shrink-0");
  });

  test("the whitespace toggle keeps its compact form until the bar can hold it", () => {
    // English's label is ~35px longer than Chinese; at 1280/1440 the full text
    // used to be clipped by the bar's overflow-hidden.
    expect(workspaceSrc).toContain('<span class="hidden @min-[1080px]:inline">{{ t("ws.diff.ignore.whitespace") }}</span>');
    expect(workspaceSrc).toContain('<span class="@min-[1080px]:hidden">-w</span>');
  });

  test("branch toolbar shares the capped column with the list", () => {
    expect(branchesSrc).toContain('<div class="max-w-5xl mx-auto w-full px-4 py-1.5 flex flex-wrap items-center justify-between gap-2">');
  });

  test("accessibility hints are allowed two lines instead of one-clause truncation", () => {
    expect(appearanceSrc).toContain("line-clamp-2");
    expect(appearanceSrc).not.toContain('mt-0.5 truncate">{{ t("settings.appearance.highcontrast.desc")');
  });
});

/**
 * Second pass: popovers that were clipped by a scroll/overflow ancestor and
 * section headers whose action labels were squeezed by long descriptions.
 */
describe("Global layout: popovers escape their clipping ancestors", () => {
  const headerSrc = read("src/components/HeaderBar.vue");
  const toolsSrc = read("src/components/ToolsView.vue");

  test("header dropdowns are never clipped by the 48px bar", () => {
    const header = templateOf(headerSrc).match(/<header[\s\S]*?>/);
    expect(header![0]).toContain("overflow-visible");
    expect(header![0]).not.toContain("overflow-hidden");
  });

  test("the tools ▾ menu is teleported out of the scrollable tab strip", () => {
    // It used to be an absolute child of `overflow-x-auto`, so the nine
    // secondary tools were unreachable by mouse.
    expect(toolsSrc).toContain('<Teleport to="body">');
    expect(toolsSrc).toContain('class="fixed z-50 w-48');
    expect(toolsSrc).toContain(':style="{ left: `${moreMenuPosition.left}px`, top: `${moreMenuPosition.top}px` }"');
    expect(toolsSrc).not.toContain('class="absolute right-0 top-full mt-1 z-40 w-48');
    expect(toolsSrc).toContain("function toggleMoreMenu()");
  });

  test("the tools menu closes on Escape and on the backdrop", () => {
    expect(toolsSrc).toContain('if (event.key === "Escape" && showMoreMenu.value) showMoreMenu.value = false');
    expect(toolsSrc).toContain('<div v-if="showMoreMenu" class="fixed inset-0 z-40" @click="showMoreMenu = false"></div>');
  });
});

describe("Global layout: section headers never squeeze their actions", () => {
  const worktreesSrc = read("src/components/tools/WorktreesTab.vue");
  const submodulesSrc = read("src/components/tools/SubmodulesTab.vue");
  const lfsSrc = read("src/components/tools/LfsTab.vue");
  const diskSrc = read("src/components/tools/DiskTab.vue");
  const engineSrc = read("src/components/tools/EngineTab.vue");

  test("title + actions wrap as a row instead of shrinking labels", () => {
    for (const src of [worktreesSrc, submodulesSrc, lfsSrc, diskSrc, engineSrc]) {
      expect(src).toContain("flex flex-wrap items-center justify-between gap-x-3 gap-y-2");
    }
  });

  test("the worktrees header keeps its action labels on one line", () => {
    expect(worktreesSrc).toContain('class="ml-auto flex flex-wrap items-center gap-2 shrink-0"');
    expect(worktreesSrc).toContain("flex items-center gap-1.5 whitespace-nowrap px-3 py-1.5 rounded-lg bg-surface");
  });

  test("the worktree path row gives the path room and wraps metadata", () => {
    expect(worktreesSrc).toContain('truncate flex-1 min-w-[140px]');
    expect(worktreesSrc).toContain("flex flex-wrap items-center gap-x-2 gap-y-1 mt-1 text-xs font-mono");
  });

  test("disk threshold buttons and scan labels never break mid-label", () => {
    expect(diskSrc).toContain("flex-wrap items-center justify-between gap-x-3 gap-y-2 pb-2");
    expect(diskSrc).toContain("whitespace-nowrap px-3 py-1.5 rounded-lg bg-accent-strong");
  });

  test("submodule status badges stay on one line", () => {
    expect(submodulesSrc).toContain("uppercase tracking-wider font-mono whitespace-nowrap shrink-0");
  });

  test("the history header lifts above the list and its menu fits its labels", () => {
    const historySrc = read("src/components/HistoryView.vue");
    // `@container` creates a stacking context; without its own z-index the
    // commit list painted over the traversal menu. w-64 holds the longest
    // Chinese label (w-52 clipped 「智能挑选穿梭机 (Cherry-pick Shuttle)」).
    expect(historySrc).toContain("@container relative z-20 h-11");
    expect(historySrc).toContain('class="absolute right-0 top-full mt-2 w-64');
  });

  test("the maintenance cache card renders a number, not the size() source", () => {
    const maintenanceSrc = read("src/components/tools/MaintenanceTab.vue");
    expect(maintenanceSrc).toContain("detailCount: commitDetailCache.size(),");
    expect(maintenanceSrc).not.toContain("detailCount: commitDetailCache.size,");
  });

  test("force-push code chips never break mid-token", () => {
    const syncSrc = read("src/components/SyncModal.vue");
    expect(syncSrc).toContain("font-mono whitespace-nowrap\">--force-with-lease</span>");
    expect(syncSrc).toContain('class="text-xs flex flex-wrap items-center gap-1"');
  });

  test("the settings nav keeps its desktop width and folds into a phone pill strip", () => {
    const settingsSrc = read("src/components/SettingsView.vue");
    // Short labels fit the narrower rail while its own scrolling body keeps
    // every category accessible in short windows.
    const rail = settingsSrc.match(/<aside[^>]*class="[^"]*md:w-52[^"]*"/);
    expect(rail).not.toBeNull();
    expect(rail![0]).toContain("w-full md:w-52");
    expect(rail![0]).toContain("border-b md:border-b-0 md:border-r");
    // Phone: categories become a horizontally swipeable row above the content
    // instead of squeezing the content column to a single character per line.
    const nav = read("src/components/settings/SettingsNav.vue");
    expect(nav).toContain("flex-row md:flex-col");
    expect(nav).toContain("overflow-x-auto md:overflow-visible");
  });
});

/**
 * Third pass: the workspace moved the composer into a full-width pinned bar.
 * Menus that were sized for the old 320px column suddenly spanned the window,
 * two headings repeated the same label, and a modal body's negative margins
 * pushed its scroll box past the panel.
 */
describe("Global layout: the pinned composer stays a compact bottom bar", () => {
  const composerSrc = read("src/components/workspace/CommitComposer.vue");
  const diffSrc = read("src/components/workspace/DiffViewer.vue");
  const usageSrc = read("src/components/settings/AiUsageCard.vue");
  const exportSrc = read("src/components/review/ExportModal.vue");
  const uiModalSrc = read("src/components/ui/UiModal.vue");
  const quickSrc = read("src/components/settings/AiConnectionForm.vue");

  test("the composer does not repeat the pinned bar's heading", () => {
    // The pinned bar already renders 「提交改动 / 暂存 N」 with the collapse
    // control; the composer keeps an accessible name but no second heading.
    expect(composerSrc).toContain('role="region"');
    expect(composerSrc).toContain(':aria-label="t(\'workspace.composer.title\')"');
    expect(composerSrc).not.toContain('<span>{{ t("workspace.composer.title") }}</span>');
  });

  test("composer menus are anchored to their trigger and stay compact", () => {
    expect(composerSrc).toContain('class="relative" data-composer-menu-root');
    expect(composerSrc).toContain("w-[340px]");
    expect(composerSrc).toContain('class="absolute right-0 bottom-full mb-1 w-80');
    expect(composerSrc).toContain('class="absolute right-0 bottom-full mb-1 w-60');
    // Full-bleed `left-2 right-2` panels spanned the whole window at 1920.
    expect(composerSrc).not.toContain("left-2 right-2 bottom-full");
  });

  test("diff toolbar menus open inside the pane instead of crossing the divider", () => {
    // Both menus are anchored to the left of their trigger so they stay within
    // the diff pane; `right-0` pushed them over the changes pane where the
    // divider painted on top.
    const leftAnchored = (diffSrc.match(/class="absolute left-0 top-full mt-1 w-56/g) ?? []).length;
    expect(leftAnchored).toBeGreaterThanOrEqual(2);
    expect(diffSrc).not.toContain('class="absolute right-0 top-full mt-1 w-56');
  });

  test("AI usage renders its empty state once", () => {
    expect(usageSrc).toContain('<p v-else-if="usage" class="text-xs text-ink-muted">{{ t("ai.usage.empty") }}</p>');
    expect(usageSrc).not.toContain('<p v-else class="text-xs text-ink-muted">{{ t("ai.usage.empty") }}</p>');
  });

  test("a flush modal body removes padding instead of fighting it with negative margins", () => {
    expect(uiModalSrc).toContain("flushBody?: boolean");
    expect(uiModalSrc).toContain("flushBody ? 'p-0' : 'px-5 py-4'");
    expect(exportSrc).toContain("flush-body");
    expect(exportSrc).not.toContain("-mx-5 -my-4");
  });

  test("blank backend error messages fall back to a real sentence", () => {
    expect(quickSrc).toContain('if (!text || text === "[object Object]") return t("common.error");');
  });
});
