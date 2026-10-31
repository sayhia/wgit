/**
 * U5 follow-up: keyboard reachability of clickable "result rows".
 *
 * Three of the fixed surfaces use the established in-house roving-tabindex
 * composable (rows carry their own buttons, so no listbox role — see
 * useRovingList's doc comment), and QuickOpen adopts the CommandPalette
 * combobox pattern (focus stays in the input, aria-activedescendant moves).
 *
 * Two kinds of assertions:
 *  1. Source locks — the chosen pattern (combobox vs roving) and the
 *     focus-visible style class are pinned per site.
 *  2. Behaviour — mounted components: arrow keys move the roving target /
 *     active descendant and Enter performs the row's primary action.
 *
 * The useRovingList consumer count is locked at its post-change value so a
 * future refactor cannot silently drop a consumer (pre-change baseline: 8).
 */
import { describe, test, expect, afterEach, spyOn } from "bun:test";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { readFileSync, readdirSync } from "fs";
import { resolve } from "path";
import QuickOpen from "../src/components/QuickOpen.vue";
import StashView from "../src/components/StashView.vue";
import HeaderBar from "../src/components/HeaderBar.vue";
import RepoDashboard from "../src/components/RepoDashboard.vue";
import { gitService } from "../src/services/gitService";
import type { StashInfo, BranchInfo, RepoStatus } from "../src/types";
import "./domSetup";

const read = (rel: string) => readFileSync(resolve(__dirname, "..", rel), "utf-8");

const FOCUS_RING = "focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45";

let wrapper: VueWrapper | null = null;

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
});

// ---------------------------------------------------------------------------
// 1. QuickOpen — combobox pattern (mirrors CommandPalette)
// ---------------------------------------------------------------------------

describe("a11y keyboard: QuickOpen combobox pattern", () => {
  test("source: input is a combobox wired to the listbox, rows are options", () => {
    const src = read("src/components/QuickOpen.vue");
    expect(src).toContain('role="combobox"');
    expect(src).toContain('aria-autocomplete="list"');
    expect(src).toContain('aria-expanded="true"');
    expect(src).toContain(`:aria-controls="'quickopen-listbox'"`);
    expect(src).toContain(`:aria-activedescendant="'quickopen-item-' + selectedIndex"`);
    expect(src).toContain('id="quickopen-listbox"');
    expect(src).toContain('role="listbox"');
    // both result branches (files + code) render options
    expect((src.match(/role="option"/g) ?? []).length).toBe(2);
    expect(src).toContain(`:id="'quickopen-item-' + idx"`);
    expect(src).toContain(':aria-selected="selectedIndex === idx"');
  });

  test("source: existing global ↑/↓/Enter selection is kept", () => {
    const src = read("src/components/QuickOpen.vue");
    expect(src).toContain('e.key === "ArrowDown"');
    expect(src).toContain('e.key === "ArrowUp"');
    expect(src).toContain('e.key === "Enter"');
    expect(src).toContain("selectCurrent()");
  });

  test("behaviour: activedescendant follows ArrowDown from the input", async () => {
    const spy = spyOn(gitService, "searchProjectFiles").mockResolvedValue([
      { path: "src/a.ts", name: "a.ts", dir: "src" },
      { path: "src/b.ts", name: "b.ts", dir: "src" },
      { path: "src/c.ts", name: "c.ts", dir: "src" },
    ]);

    wrapper = mount(QuickOpen, { props: { currentRepoPath: "/repo" } });
    window.dispatchEvent(new CustomEvent("wgit:quick-open", { detail: { mode: "files" } }));
    await flushPromises();

    const input = wrapper.find('input[role="combobox"]');
    expect(input.exists()).toBe(true);
    expect(input.attributes("aria-expanded")).toBe("true");
    expect(input.attributes("aria-controls")).toBe("quickopen-listbox");
    expect(input.attributes("aria-activedescendant")).toBe("quickopen-item-0");

    const options = wrapper.findAll('[role="option"]');
    expect(options.length).toBe(3);
    expect(options[0].attributes("id")).toBe("quickopen-item-0");
    expect(options[0].attributes("aria-selected")).toBe("true");
    expect(options[1].attributes("aria-selected")).toBe("false");

    window.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown" }));
    await flushPromises();

    expect(input.attributes("aria-activedescendant")).toBe("quickopen-item-1");
    expect(wrapper.findAll('[role="option"]')[1].attributes("aria-selected")).toBe("true");

    spy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// 2. StashView — roving tabindex on the stash cards
// ---------------------------------------------------------------------------

const mockStashes: StashInfo[] = [
  {
    index: 0,
    name: "stash@{0}",
    branch: "main",
    message: "WIP on main: keyboard stash",
    date: "2026-10-30 07:00:00",
    commitHash: "12345678",
  },
  {
    index: 1,
    name: "stash@{1}",
    branch: "feature",
    message: "WIP on feature: keyboard stash",
    date: "2026-10-29 07:00:00",
    commitHash: "87654321",
  },
];

describe("a11y keyboard: StashView roving stash cards", () => {
  test("source: cards use useRovingList with Enter-to-select and a focus ring", () => {
    const src = read("src/components/StashView.vue");
    expect(src).toContain('from "../composables/useRovingList"');
    expect(src).toContain("setItemRef: setStashItemRef");
    expect(src).toContain(`:ref="(el) => setStashItemRef(s, el)"`);
    expect(src).toContain(`:tabindex="stashTabindexFor(s, index)"`);
    expect(src).toContain(`@keydown="onStashRowKeydown($event, index)"`);
    expect(src).toContain(`@keydown.enter.self="selectStash(s)"`);
    expect(src).toContain(FOCUS_RING);
  });

  test("behaviour: ArrowDown moves the single tab stop, Enter selects the card", async () => {
    const spy = spyOn(gitService, "getStashes").mockResolvedValue(mockStashes);

    wrapper = mount(StashView, {
      props: { currentRepoPath: "/path/to/test-repo" },
      attachTo: document.body,
    });
    await new Promise((r) => setTimeout(r, 30));
    await flushPromises();

    let cards = wrapper.findAll("[tabindex]");
    expect(cards.length).toBe(2);
    expect(cards[0].attributes("tabindex")).toBe("0");
    expect(cards[1].attributes("tabindex")).toBe("-1");

    await cards[0].trigger("keydown", { key: "ArrowDown" });
    await flushPromises();

    cards = wrapper.findAll("[tabindex]");
    expect(cards[0].attributes("tabindex")).toBe("-1");
    expect(cards[1].attributes("tabindex")).toBe("0");

    await cards[1].trigger("keydown", { key: "Enter" });
    await flushPromises();

    cards = wrapper.findAll("[tabindex]");
    expect(cards[1].classes()).toContain("border-accent/50");
    expect(cards[0].classes()).not.toContain("border-accent/50");

    spy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// 3. HeaderBar — roving tabindex in both dropdown menus
// ---------------------------------------------------------------------------

const mockBranches: BranchInfo[] = [
  { name: "main", isCurrent: true, isRemote: false, commitHash: "a1", upstream: "", ahead: 0, behind: 0 },
  { name: "feature", isCurrent: false, isRemote: false, commitHash: "b2", upstream: "", ahead: 0, behind: 0 },
];

describe("a11y keyboard: HeaderBar roving menu rows", () => {
  test("source: both menus use useRovingList, branch quick actions gain focus-within", () => {
    const src = read("src/components/HeaderBar.vue");
    expect(src).toContain('from "../composables/useRovingList"');
    // repo menu rows
    expect(src).toContain(`:ref="(el) => setRepoItemRef(repo, el)"`);
    expect(src).toContain(`:tabindex="repoTabindexFor(repo, index)"`);
    expect(src).toContain(`@keydown.enter.self="selectRepo(repo)"`);
    // branch menu rows
    expect(src).toContain(`:ref="(el) => setBranchItemRef(b, el)"`);
    expect(src).toContain(`:tabindex="branchTabindexFor(b, index)"`);
    expect(src).toContain(`@keydown.enter.self="selectBranch(b.name)"`);
    // hover-only rebase/merge shortcuts are also revealed by keyboard focus
    expect(src).toContain("group-hover/b:flex group-focus-within/b:flex");
    // focus ring on both row sets
    expect((src.match(new RegExp(FOCUS_RING.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "g")) ?? []).length).toBe(2);
  });

  test("behaviour: repo rows rove and Enter emits open-repo", async () => {
    const recentsSpy = spyOn(gitService, "getRecentRepos").mockResolvedValue([
      "/wk/alpha",
      "/wk/beta",
    ]);

    wrapper = mount(HeaderBar, {
      props: { currentRepoPath: "/wk/alpha", repoStatus: null, isSyncing: false },
      attachTo: document.body,
    });

    const triggers = wrapper.findAll("[data-header-menu-trigger]");
    await triggers[0].trigger("click");
    await flushPromises();

    let rows = wrapper.findAll("[tabindex]");
    expect(rows.length).toBe(2);
    expect(rows[0].attributes("tabindex")).toBe("0");
    expect(rows[1].attributes("tabindex")).toBe("-1");

    await rows[1].trigger("keydown", { key: "ArrowUp" });
    await flushPromises();

    rows = wrapper.findAll("[tabindex]");
    expect(rows[0].attributes("tabindex")).toBe("0");
    expect(rows[1].attributes("tabindex")).toBe("-1");

    await rows[1].trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(wrapper.emitted("open-repo")).toEqual([["/wk/beta"]]);

    recentsSpy.mockRestore();
  });

  test("behaviour: branch rows rove and Enter emits switch-branch", async () => {
    const recentsSpy = spyOn(gitService, "getRecentRepos").mockResolvedValue([]);
    const branchesSpy = spyOn(gitService, "getBranches").mockResolvedValue(mockBranches);

    wrapper = mount(HeaderBar, {
      props: { currentRepoPath: "/wk/alpha", repoStatus: null, isSyncing: false },
      attachTo: document.body,
    });

    const triggers = wrapper.findAll("[data-header-menu-trigger]");
    await triggers[1].trigger("click"); // branch pill
    await flushPromises();

    let rows = wrapper.findAll("[tabindex]");
    expect(rows.length).toBe(2);
    expect(rows[0].attributes("tabindex")).toBe("0");

    await rows[0].trigger("keydown", { key: "ArrowDown" });
    await flushPromises();

    rows = wrapper.findAll("[tabindex]");
    expect(rows[0].attributes("tabindex")).toBe("-1");
    expect(rows[1].attributes("tabindex")).toBe("0");

    await rows[1].trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(wrapper.emitted("switch-branch")).toEqual([["feature"]]);

    recentsSpy.mockRestore();
    branchesSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// 4. RepoDashboard — roving tabindex on the repo cards
// ---------------------------------------------------------------------------

const mockStatus: RepoStatus = {
  currentBranch: "main",
  trackingBranch: "origin/main",
  ahead: 0,
  behind: 0,
  isClean: true,
  stagedFiles: [],
  unstagedFiles: [],
  untrackedFiles: [],
  conflictedFiles: [],
  isMerging: false,
  isRebasing: false,
  isCherryPicking: false,
};

describe("a11y keyboard: RepoDashboard roving repo cards", () => {
  test("source: cards use useRovingList with Enter-to-open and a focus ring", () => {
    const src = read("src/components/RepoDashboard.vue");
    expect(src).toContain('from "../composables/useRovingList"');
    expect(src).toContain(`:ref="(el) => setCardRef(row, el)"`);
    expect(src).toContain(`:tabindex="cardTabindexFor(row, index)"`);
    expect(src).toContain(`@keydown="onCardKeydown($event, index)"`);
    expect(src).toContain(`@keydown.enter.self="handleOpenRepo(row.path)"`);
    expect(src).toContain(FOCUS_RING);
  });

  test("behaviour: cards rove and Enter emits opened", async () => {
    const statusSpy = spyOn(gitService, "getRepoStatus").mockResolvedValue(mockStatus);
    const overviewSpy = spyOn(gitService, "getRepoOverview").mockResolvedValue({
      totalCommits: 1,
      totalBranches: 2,
      totalTags: 0,
      contributors: [],
    });

    wrapper = mount(RepoDashboard, {
      props: { repos: ["/r/alpha", "/r/beta"], activeRepo: "/r/alpha", open: true },
      attachTo: document.body,
    });
    await flushPromises();
    await new Promise((r) => setTimeout(r, 30));
    await flushPromises();

    let cards = wrapper.findAll("[tabindex]");
    expect(cards.length).toBe(2);
    expect(cards[0].attributes("tabindex")).toBe("0");
    expect(cards[1].attributes("tabindex")).toBe("-1");

    await cards[0].trigger("keydown", { key: "ArrowDown" });
    await flushPromises();

    cards = wrapper.findAll("[tabindex]");
    expect(cards[0].attributes("tabindex")).toBe("-1");
    expect(cards[1].attributes("tabindex")).toBe("0");

    await cards[1].trigger("keydown", { key: "Enter" });
    await flushPromises();

    expect(wrapper.emitted("opened")).toEqual([["/r/beta"]]);

    statusSpy.mockRestore();
    overviewSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// 5. useRovingList consumer count never decreases
// ---------------------------------------------------------------------------

describe("a11y keyboard: useRovingList consumer count", () => {
  function collectVueFiles(dir: string): string[] {
    const out: string[] = [];
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = resolve(dir, entry.name);
      if (entry.isDirectory()) out.push(...collectVueFiles(full));
      else if (entry.name.endsWith(".vue")) out.push(full);
    }
    return out;
  }

  test("consumer count grew from the pre-change baseline of 8 to 11", () => {
    const consumers = collectVueFiles(resolve(__dirname, "../src/components")).filter((file) =>
      readFileSync(file, "utf-8").includes("composables/useRovingList")
    );
    const names = consumers.map((file) => file.split("/").pop());
    // pre-change baseline (branches/tools/pr surfaces): 8 components
    expect(consumers.length).toBeGreaterThan(8);
    // post-change: + StashView, HeaderBar, RepoDashboard
    expect(consumers.length).toBe(11);
    for (const expected of ["StashView.vue", "HeaderBar.vue", "RepoDashboard.vue"]) {
      expect(names).toContain(expected);
    }
    // the pre-existing consumers are still there
    for (const expected of ["StashesList.vue", "LocalBranchList.vue", "SubmodulesTab.vue"]) {
      expect(names).toContain(expected);
    }
  });
});
