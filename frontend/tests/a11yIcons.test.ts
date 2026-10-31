/**
 * a11y icon guard — U5 incremental slice 1.
 *
 * Scope: self-closing lucide icons in src/components/ui/*.vue and
 * src/components/branches/*.vue (modals excluded). Every icon sitting inside
 * an interactive element (button/a/label) that ALREADY has an accessible name
 * (aria-label/title attribute, or a non-icon text sibling) got
 * aria-hidden="true" (48 additions across 9 files). Icons that are the sole
 * content of an UNNAMED interactive element must NEVER get aria-hidden (that
 * would make the control nameless) — this slice found none of those in scope;
 * the reverse-protection tests below pin the invariant at three sampled sites
 * so a future refactor that strips a button's naming source cannot silently
 * leave a hidden icon behind.
 *
 * U5 slice 2 (same rules): src/components/tools/, src/components/review/,
 * src/components/settings/, src/components/lfs/. 246 additions across 43
 * files (see SLICE2_ARIA_HIDDEN floors; the Studio integration removal later
 * dropped the floor to 243). The three icon-only copy buttons in
 * tools/GitBundleModal.vue (Check/Copy pairs at the path/clone/verify rows)
 * are now named via aria-label + title with bundle.modal.copy.* keys, and
 * their decorative icons are aria-hidden. This promoted the former RECORD
 * guard to a FIX assertion below.
 *
 * U5 slice 3 (same rules): src/components/workspace/ (AI modals owned by a
 * parallel session excluded), src/components/pr/, src/components/history/.
 * 153 additions across 21 files (see SLICE3_ARIA_HIDDEN floors; per-file
 * floors are the actual post-slice counts — CommitGraphList's 4 includes two
 * pre-existing decorative spacer divs). The three signature-badge icons in
 * history/CommitInspector.vue (ShieldCheck/ShieldAlert/ShieldX) are now
 * aria-hidden; the badge button keeps its accessible name via :title + text.
 * The 4 icon-only unnamed buttons in history/RebaseKanbanModal.vue (MoveUp /
 * MoveDown reorder, Trash2 partition remove) and history/CommitSplitterModal.vue
 * (Trash2 bucket remove) started as RECORD — while unnamed, their icons must
 * never carry aria-hidden. The all-slices grand-total floor was raised
 * accordingly (291 -> 447).
 *
 * U5 slice 4 (RECORD -> FIX promotion): all four buttons above are now named
 * via :aria-label="t(...)" — two reuse the existing history.rebase.move.up /
 * history.rebase.move.down keys, two got the new keys
 * history.rebase.plan.remove and history.splitter.plan.remove (added to
 * zh-CN/en-US) — and their icons are now safely aria-hidden="true". The
 * slice-3 RECORD guard was rewritten as a FIX floor pinning each aria-label,
 * its zh/en i18n key and the all-lowercase key literals. Floors raised:
 * CommitSplitterModal 1 -> 2, RebaseKanbanModal 1 -> 4; slice-3 total
 * 155 -> 162 and grand total 447 -> 460 as merged with this round's other
 * promotions (GitBundleModal copy buttons, CommitInspector signature badges).
 */
import { describe, test, expect } from "bun:test";
import { readFileSync, readdirSync } from "fs";
import { join, resolve } from "path";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const read = (rel: string) => readFileSync(resolve(__dirname, "..", rel), "utf-8");

/** aria-hidden="true" additions per file in this slice (floor guards). */
const ADDED_ARIA_HIDDEN: Record<string, number> = {
  "src/components/ui/UiModal.vue": 1,
  "src/components/ui/AiReasoningPanel.vue": 1,
  "src/components/ui/Toast.vue": 1,
  "src/components/branches/RemoteBranchList.vue": 6,
  "src/components/branches/StashesList.vue": 7,
  "src/components/branches/BranchRiverView.vue": 5,
  "src/components/branches/TagsList.vue": 5,
  "src/components/branches/BranchCompare.vue": 11,
  "src/components/branches/LocalBranchList.vue": 11,
};

/** aria-hidden="true" additions per file in U5 slice 2 (floor guards). */
const SLICE2_ARIA_HIDDEN: Record<string, number> = {
  // tools/
  "src/components/tools/AirGappedHubModal.vue": 5,
  "src/components/tools/CleanTab.vue": 3,
  "src/components/tools/DiskTab.vue": 4,
  "src/components/tools/EngineBootstrapSection.vue": 1,
  "src/components/tools/EngineDemoSection.vue": 1,
  "src/components/tools/EngineQualitySection.vue": 3,
  "src/components/tools/EngineTab.vue": 5,
  "src/components/tools/FixCenterSection.vue": 1,
  "src/components/tools/GitBundleModal.vue": 10,
  "src/components/tools/LfsTab.vue": 16,
  "src/components/tools/LocalCiModal.vue": 2,
  "src/components/tools/MaintenanceTab.vue": 17,
  "src/components/tools/ReflogTab.vue": 3,
  "src/components/tools/ReleaseChangelogModal.vue": 5,
  "src/components/tools/RemotesTab.vue": 13,
  "src/components/tools/SparseTab.vue": 7,
  "src/components/tools/StudioIntegrationSection.vue": 2,
  "src/components/tools/StudioOpsSection.vue": 9,
  "src/components/tools/StudioTab.vue": 7,
  "src/components/tools/SubmodulesTab.vue": 16,
  "src/components/tools/TerminalTab.vue": 7,
  "src/components/tools/WorktreesTab.vue": 15,
  // review/
  "src/components/review/ChangeList.vue": 2,
  "src/components/review/ExportModal.vue": 5,
  "src/components/review/FileDetailPanel.vue": 7,
  "src/components/review/LanReviewModal.vue": 6,
  "src/components/review/SessionList.vue": 3,
  // settings/
  "src/components/settings/AdvancedConfigSection.vue": 5,
  "src/components/settings/AiControlPlaneSection.vue": 9,
  "src/components/settings/AiQuickSetup.vue": 4,
  "src/components/settings/AiUsageCard.vue": 2,
  "src/components/settings/AliasesSection.vue": 6,
  "src/components/settings/AppearanceSection.vue": 3,
  "src/components/settings/GitHooksSection.vue": 3,
  "src/components/settings/IdentitySection.vue": 1,
  "src/components/settings/LocalAiSection.vue": 16,
  "src/components/settings/ProxySection.vue": 1,
  "src/components/settings/ShortcutsSection.vue": 9,
  "src/components/settings/SigningSection.vue": 3,
  "src/components/settings/TeamWebhookSection.vue": 4,
  "src/components/settings/ToolsSection.vue": 4,
  // lfs/
  "src/components/lfs/LockNotificationModal.vue": 4,
};

/** aria-hidden="true" totals per file in U5 slice 3 (floor guards). */
const SLICE3_ARIA_HIDDEN: Record<string, number> = {
  // workspace/ (parallel-session-owned AI modals excluded)
  "src/components/workspace/AiConflictAdvisor.vue": 3,
  "src/components/workspace/ChangesList.vue": 14,
  "src/components/workspace/CommitComposer.vue": 14,
  "src/components/workspace/ConflictBanner.vue": 2,
  "src/components/workspace/FlightRecorderHud.vue": 1,
  "src/components/workspace/MiniCommitHudModal.vue": 1,
  "src/components/workspace/SemanticConflictModal.vue": 2,
  "src/components/workspace/SourceExplorer.vue": 29,
  "src/components/workspace/SourcePanorama.vue": 16,
  // pr/
  "src/components/pr/PrDetailPanel.vue": 2,
  "src/components/pr/PrView.vue": 1,
  // history/
  "src/components/history/BatchActionToolbar.vue": 7,
  "src/components/history/BatchCherryPickModal.vue": 4,
  "src/components/history/CommitGraphList.vue": 4, // 2 icons + 2 pre-existing spacer divs
  "src/components/history/CommitInspector.vue": 32, // 3 signature-badge Shield icons excluded (contract test pins their exact source)
  "src/components/history/CommitSplitterModal.vue": 2, // +1: bucket-remove Trash2 hidden after aria-label (slice 4)
  "src/components/history/CommitTimeLapseModal.vue": 5,
  "src/components/history/CompareCommitsModal.vue": 4,
  "src/components/history/HistoryFilterBar.vue": 4,
  "src/components/history/InteractiveRebaseModal.vue": 11,
  "src/components/history/RebaseKanbanModal.vue": 4, // +3: MoveUp/MoveDown/Trash2 hidden after aria-labels (slice 4)
};

describe("a11y icons: aria-hidden floors (never decrease)", () => {
  for (const [mapName, map] of [
    ["slice 1", ADDED_ARIA_HIDDEN],
    ["slice 2", SLICE2_ARIA_HIDDEN],
    ["slice 3", SLICE3_ARIA_HIDDEN],
  ] as const) {
    for (const [file, floor] of Object.entries(map)) {
      test(`${file} keeps >= ${floor} aria-hidden decorative markers (${mapName})`, () => {
        const count = (read(file).match(/aria-hidden/g) ?? []).length;
        expect(count).toBeGreaterThanOrEqual(floor);
      });
    }
  }

  test("the slice-1 touched files keep >= 48 aria-hidden decorative markers in total", () => {
    const total = Object.keys(ADDED_ARIA_HIDDEN).reduce(
      (sum, f) => sum + (read(f).match(/aria-hidden/g) ?? []).length,
      0,
    );
    expect(total).toBeGreaterThanOrEqual(48);
  });

  test("the slice-2 touched files keep >= 249 aria-hidden decorative markers in total", () => {
    const total = Object.keys(SLICE2_ARIA_HIDDEN).reduce(
      (sum, f) => sum + (read(f).match(/aria-hidden/g) ?? []).length,
      0,
    );
    expect(total).toBeGreaterThanOrEqual(249);
  });

  test("the slice-3 touched files keep >= 162 aria-hidden decorative markers in total", () => {
    const total = Object.keys(SLICE3_ARIA_HIDDEN).reduce(
      (sum, f) => sum + (read(f).match(/aria-hidden/g) ?? []).length,
      0,
    );
    expect(total).toBeGreaterThanOrEqual(162);
  });

  test("all slices keep >= 460 aria-hidden decorative markers in total", () => {
    const total = [
      ...Object.keys(ADDED_ARIA_HIDDEN),
      ...Object.keys(SLICE2_ARIA_HIDDEN),
      ...Object.keys(SLICE3_ARIA_HIDDEN),
    ].reduce((sum, f) => sum + (read(f).match(/aria-hidden/g) ?? []).length, 0);
    expect(total).toBeGreaterThanOrEqual(460);
  });

  test("BranchCompare file rows expose the primary action as a real button", () => {
    const source = read("src/components/branches/BranchCompare.vue");
    expect(source).toContain('type="button"');
    expect(source).toContain(`@click="emit('show-file-diff', file.path)"`);
    expect(source).toContain(`:title="t('branches.compare.diff.file.hint')"`);
    expect(source).toContain(`@click.stop="emit('open-file', file.path)"`);
  });

  test("SourceExplorer tree rows are keyboard operable treeitems", () => {
    const source = read("src/components/workspace/SourceExplorer.vue");
    expect(source).toContain('role="tree"');
    expect(source).toContain('role="treeitem"');
    expect(source).toContain('@keydown.enter.prevent="toggleFolder(node.path)"');
    expect(source).toContain('@keydown.space.prevent="emit(\'open-file\', node.path)"');
    expect((zhCN as Record<string, string>)["explorer.tree.label"]).toBeTruthy();
    expect((enUS as Record<string, string>)["explorer.tree.label"]).toBeTruthy();
  });

  test("audio playback and diff-close icon-only controls are named", () => {
    const audio = read("src/components/assets/AudioDiffViewer.vue");
    for (const key of ["assets.audio.play", "assets.audio.pause", "assets.audio.mute", "assets.audio.unmute"]) {
      expect(audio).toContain(`t('${key}')`);
      expect((zhCN as Record<string, string>)[key]).toBeTruthy();
      expect((enUS as Record<string, string>)[key]).toBeTruthy();
    }
    expect(audio).toContain('@click="togglePlay"');
    expect(audio).toContain('@click="toggleMute"');
    expect((audio.match(/aria-hidden="true"/g) ?? []).length).toBeGreaterThanOrEqual(4);

    const diff = read("src/components/workspace/DiffViewer.vue");
    expect(diff).toContain(':aria-label="t(\'common.close\')"');
    expect(diff).toContain(':aria-label="t(\'blame.popover.close\')"');
  });

  test("no icon-only button anywhere is left without an accessible name", () => {
    const srcRoot = resolve(__dirname, "../src");
    const files: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = join(dir, entry.name);
        if (entry.isDirectory()) walk(full);
        else if (entry.name.endsWith(".vue")) files.push(full);
      }
    };
    walk(srcRoot);
    const offenders: string[] = [];
    const button = /<button\b((?:[^>"']|"[^"]*"|'[^']*')*)>(.*?)<\/button>/gs;
    const icon = /<[A-Z][A-Za-z0-9]*\b[^>]*?\/>/;
    for (const file of files) {
      const source = readFileSync(file, "utf8").replace(/<!--[\s\S]*?-->/g, "");
      for (const match of source.matchAll(button)) {
        const attrs = match[1] ?? "";
        const inner = match[2] ?? "";
        if (/(?:^|\s)(?::|v-bind:)?(?:aria-label|aria-labelledby|title)\s*=/.test(attrs)) continue;
        if (/\{\{/.test(inner)) continue;
        const text = inner.replace(/<[^>]+>/g, "").trim();
        if (text) continue;
        if (!icon.test(inner)) continue;
        offenders.push(file.slice(srcRoot.length + 1));
      }
    }
    expect(offenders).toEqual([]);
  });

  test("GitBundleModal copy buttons are named and their decorative icons are hidden", () => {
    const src = read("src/components/tools/GitBundleModal.vue");
    const keys = [
      ["path", "bundle.modal.copy.path"],
      ["clone", "bundle.modal.copy.clone"],
      ["verify", "bundle.modal.copy.verify"],
    ] as const;
    for (const [field, key] of keys) {
      expect(src).toContain(`:aria-label="t('${key}')"`);
      expect(src).toContain(`:title="t('${key}')"`);
      expect(src).toContain(`'${field}'`);
      expect((zhCN as Record<string, string>)[key]).toBeTruthy();
      expect((enUS as Record<string, string>)[key]).toBeTruthy();
    }
  });
});

/** Extract the full `<button …>…</button>` region containing position `at`. */
function buttonRegionAround(src: string, at: number): string {
  const open = src.lastIndexOf("<button", at);
  expect(open).toBeGreaterThan(-1);
  let depth = 0;
  let i = open;
  while (i < src.length) {
    const nextOpen = src.indexOf("<button", i);
    const nextClose = src.indexOf("</button>", i);
    if (nextClose < 0) throw new Error("unbalanced <button> region");
    if (nextOpen >= 0 && nextOpen < nextClose) {
      depth++;
      i = nextOpen + 7;
    } else {
      depth--;
      i = nextClose + 9;
      if (depth === 0) return src.slice(open, i);
    }
  }
  throw new Error("unbalanced <button> region");
}

describe("a11y icons: reverse protection (sampled sites)", () => {
  // Each sampled aria-hidden icon must live in a button whose accessible name
  // comes from something other than the icon itself (text sibling, or an
  // aria-label/title attribute for icon-only buttons). If that naming source
  // disappears later, these tests fail and the aria-hidden must be reviewed —
  // an aria-hidden icon as an unnamed button's only content = nameless button.

  test("RemoteBranchList merge button: title attr + text sibling keep the name source", () => {
    const src = read("src/components/branches/RemoteBranchList.vue");
    const button = buttonRegionAround(src, src.indexOf("<GitMerge"));
    expect(button).toContain('aria-hidden="true"');
    expect(button).toContain(`:title="t('branches.remote.merge.hint')"`);
    // non-icon content (the text sibling) must remain in the button
    expect(button.replace(/<GitMerge[^>]*\/>/, "")).toMatch(/\S/);
    expect(button).toContain('t("branches.remote.merge")');
  });

  test("AiReasoningPanel toggle: text sibling keeps the name source", () => {
    const src = read("src/components/ui/AiReasoningPanel.vue");
    const button = buttonRegionAround(src, src.indexOf("<ChevronDown"));
    expect(button).toContain('aria-hidden="true"');
    expect(button).toContain('{{ t("ai.chat.reasoning") }}');
  });

  test("LocalBranchList delete button: icon-only button is named via aria-label", () => {
    // The row delete button's only DOM content is the Trash2 icon; it is safe
    // only because the button itself carries an explicit aria-label.
    const src = read("src/components/branches/LocalBranchList.vue");
    const labelAt = src.indexOf(`:aria-label="t('branches.local.delete.hint')"`);
    expect(labelAt).toBeGreaterThan(-1);
    const button = buttonRegionAround(src, labelAt);
    expect(button).toContain("<Trash2");
    expect(button).toContain('aria-hidden="true"');
    expect(button.replace(/<Trash2[^>]*\/>/, "").replace(/<button[^>]*>/, "").replace("</button>", "").trim())
      .toBe(""); // documents it IS an icon-only button — guarded by the aria-label above
  });
});

describe("a11y icons slice 2: reverse protection (sampled sites)", () => {
  // Same invariant as slice 1, sampled in the slice-2 directories: each of
  // these buttons is icon-only, so its aria-hidden icon is only correct while
  // the button keeps its explicit aria-label. If the aria-label disappears,
  // these tests fail and the aria-hidden must be reviewed.

  test("StudioTab refresh-logs button: icon-only, named via aria-label", () => {
    const src = read("src/components/tools/StudioTab.vue");
    const labelAt = src.indexOf(`:aria-label="t('studio.logs.refresh')"`);
    expect(labelAt).toBeGreaterThan(-1);
    const button = buttonRegionAround(src, labelAt);
    expect(button).toContain("<RefreshCw");
    expect(button).toContain('aria-hidden="true"');
    expect(stripButtonToIcon(button, "RefreshCw")).toBe(""); // icon-only, kept safe by the aria-label above
  });

  test("StudioTab clear-logs button: icon-only, named via aria-label", () => {
    const src = read("src/components/tools/StudioTab.vue");
    const labelAt = src.indexOf(`:aria-label="t('studio.logs.clear')"`);
    expect(labelAt).toBeGreaterThan(-1);
    const button = buttonRegionAround(src, labelAt);
    expect(button).toContain("<Trash2");
    expect(button).toContain('aria-hidden="true"');
    expect(stripButtonToIcon(button, "Trash2")).toBe("");
  });

  test("StudioOpsSection remove-webhook button: icon-only, named via aria-label", () => {
    const src = read("src/components/tools/StudioOpsSection.vue");
    const labelAt = src.indexOf(`:aria-label="t('ops.webhook.remove')"`);
    expect(labelAt).toBeGreaterThan(-1);
    const button = buttonRegionAround(src, labelAt);
    expect(button).toContain("<Trash2");
    expect(button).toContain('aria-hidden="true"');
    expect(stripButtonToIcon(button, "Trash2")).toBe("");
  });
});

/** Button inner content with the open tag, the named icon tag and the close tag removed. */
function stripButtonToIcon(button: string, icon: string): string {
  return button
    .replace(new RegExp(`<${icon}[\\s\\S]*?/>`), "")
    // Attribute values may contain ">" (e.g. v-if="plans.length > 1"), so
    // strip the opening tag with quote-aware matching rather than the first ">".
    .replace(/<button\b(?:[^>"']|"[^"]*"|'[^']*')*>/, "")
    .replace("</button>", "")
    .trim();
}

describe("a11y icons slice 2: RECORD guard (unnamed icon-only controls)", () => {
  // GitBundleModal's three copy buttons (path/clone/verify rows) are icon-only
  // and currently have NO accessible name (RECORD backlog: they need new i18n
  // aria-label keys, a later slice). Until they are named, their Check/Copy
  // icons must NOT carry aria-hidden — that would leave nameless buttons.

  test("GitBundleModal copy buttons keep icons un-hidden while the buttons are unnamed", () => {
    const src = read("src/components/tools/GitBundleModal.vue");
    const needles: Array<[string, string]> = [
      ["path", `copyText(exportResult.outputPath, 'path')`],
      ["clone", `copyText(exportResult.cloneHelp, 'clone')`],
      ["verify", `copyText(exportResult.verifyHelp, 'verify')`],
    ];
    for (const [field, needle] of needles) {
      const at = src.indexOf(needle);
      expect(at).toBeGreaterThan(-1);
      const button = buttonRegionAround(src, at);
      const openTag = button.slice(0, button.indexOf(">"));
      const named = /(:|v-bind:)?(aria-label|aria-labelledby|title)\s*=/.test(openTag);
      const hiddenIcons = (button.match(/aria-hidden="true"/g) ?? []).length;
      if (!named) {
        expect(hiddenIcons, `${field} copy button is unnamed but carries a hidden icon`).toBe(0);
        // and document the RECORD state: icon-only (no text content beyond the icons)
        expect(stripButtonToIcon(button.replace(/<(Check|Copy)[\s\S]*?\/>/g, ""), "Check")).toBe("");
      }
    }
  });
});

/** End index (within `region`) of the open tag starting at region start; quote-aware. */
function openTagEnd(region: string, tag: string): number {
  const m = region.match(new RegExp(`^<${tag}(?:[^>"']|"[^"]*"|'[^']*')*>`));
  if (!m) throw new Error(`no open <${tag}> at region start`);
  return m[0].length;
}

/** Full `<tag …>…</tag>` region containing position `at` (like buttonRegionAround, any tag). */
function elementRegionAround(src: string, at: number, tag: string): string {
  const needle = `<${tag}`;
  let open = -1;
  let from = at;
  while (from >= 0) {
    const i = src.lastIndexOf(needle, from);
    if (i < 0) break;
    const next = src[i + needle.length];
    if (next === undefined || /\s|>/.test(next)) {
      open = i;
      break;
    }
    from = i - 1;
  }
  if (open < 0) throw new Error(`no <${tag}> region found`);
  let depth = 0;
  let i = open;
  const closeNeedle = `</${tag}>`;
  while (i < src.length) {
    const nextOpen = src.indexOf(needle, i);
    const nextClose = src.indexOf(closeNeedle, i);
    if (nextClose < 0) throw new Error(`unbalanced <${tag}> region`);
    if (nextOpen >= 0 && nextOpen < nextClose && /\s|>/.test(src[nextOpen + needle.length] ?? ">")) {
      depth++;
      i = nextOpen + needle.length;
    } else {
      depth--;
      i = nextClose + closeNeedle.length;
      if (depth === 0) return src.slice(open, i);
    }
  }
  throw new Error(`unbalanced <${tag}> region`);
}

/** Inner content of a region between its open tag and its close tag. */
function innerContent(region: string, tag: string): string {
  return region.slice(openTagEnd(region, tag), region.length - `</${tag}>`.length);
}

describe("a11y icons slice 3: reverse protection (sampled sites)", () => {
  // Same invariant as slices 1-2, sampled in the slice-3 directories. Each
  // sampled interactive element keeps an accessible name from something other
  // than the icon itself; if that naming source disappears later, these tests
  // fail and the aria-hidden must be reviewed.

  test("SourceExplorer dir menu button: icon-only, named via :aria-label + :title", () => {
    const src = read("src/components/workspace/SourceExplorer.vue");
    const labelAt = src.indexOf(`:aria-label="t('explorer.dir.actions')"`);
    expect(labelAt).toBeGreaterThan(-1);
    const button = buttonRegionAround(src, labelAt);
    expect(button).toContain("<MoreHorizontal");
    expect(button).toContain('aria-hidden="true"');
    expect(stripButtonToIcon(button, "MoreHorizontal")).toBe(""); // icon-only, kept safe by the aria-label above
  });

  test("ChangesList LFS lock button: icon-only, named via :aria-label + :title", () => {
    const src = read("src/components/workspace/ChangesList.vue");
    const labelAt = src.indexOf(`:aria-label="t('changes.lock.lfs.aria')"`);
    expect(labelAt).toBeGreaterThan(-1);
    const button = buttonRegionAround(src, labelAt);
    expect(button).toContain("<Lock");
    expect(button).toContain('aria-hidden="true"');
    expect(stripButtonToIcon(button, "Lock")).toBe("");
  });

  test("CommitInspector CI link: <a> named by its text sibling keeps the name source", () => {
    const src = read("src/components/history/CommitInspector.vue");
    const hrefAt = src.indexOf(`:href="ciStatus.url"`);
    expect(hrefAt).toBeGreaterThan(-1);
    const anchor = elementRegionAround(src, hrefAt, "a");
    expect(anchor).toContain("<ExternalLink");
    expect(anchor).toContain('aria-hidden="true"');
    // the text sibling must survive: stripping the icon leaves the "CI" label
    const inner = innerContent(anchor, "a").replace(/<ExternalLink[\s\S]*?\/>/, "");
    expect(inner).toContain("CI");
  });
});

describe("a11y icons slice 3: promoted FIX floor (named icon-only controls)", () => {
  // U5 slice 4 promotion: the four icon-only buttons that slice 3 left as
  // RECORD (unnamed) now carry explicit :aria-label names, so their icons are
  // safely aria-hidden="true". These FIX-floor assertions pin the naming per
  // button: if an aria-label, its i18n key or the hidden icon disappears, a
  // hidden icon on an unnamed icon-only button would leave a nameless control
  // again.

  const PROMOTED_CONTROLS: Array<{
    file: string;
    click: string;
    key: string;
    icon: string;
    isNewKey: boolean;
  }> = [
    { file: "src/components/history/RebaseKanbanModal.vue", click: `@click="moveItem(idx, -1)"`, key: "history.rebase.move.up", icon: "MoveUp", isNewKey: false },
    { file: "src/components/history/RebaseKanbanModal.vue", click: `@click="moveItem(idx, 1)"`, key: "history.rebase.move.down", icon: "MoveDown", isNewKey: false },
    { file: "src/components/history/RebaseKanbanModal.vue", click: `@click="removePartition(pIdx)"`, key: "history.rebase.plan.remove", icon: "Trash2", isNewKey: true },
    { file: "src/components/history/CommitSplitterModal.vue", click: `@click="removePlan(idx)"`, key: "history.splitter.plan.remove", icon: "Trash2", isNewKey: true },
  ];

  for (const { file, click, key, icon } of PROMOTED_CONTROLS) {
    test(`${file}: ${icon} control is named via :aria-label and its icon is hidden`, () => {
      const src = read(file);
      const at = src.indexOf(click);
      expect(at).toBeGreaterThan(-1);
      const button = buttonRegionAround(src, at);
      expect(button).toContain(`:aria-label="t('${key}')"`);
      expect(button).toContain(`<${icon}`);
      expect(button).toContain('aria-hidden="true"');
      // still icon-only — the aria-label above is its only accessible name.
      // innerContent/openTagEnd are quote-aware: the CommitSplitterModal
      // button's v-if="plans.length > 1" holds a ">" that would defeat the
      // naive stripButtonToIcon open-tag regex.
      const inner = innerContent(button, "button").replace(new RegExp(`<${icon}[\\s\\S]*?/>`), "");
      expect(inner.trim()).toBe("");
    });

    test(`i18n key "${key}" exists in zh-CN and en-US (non-empty)`, () => {
      const zh = (zhCN as Record<string, string>)[key];
      const en = (enUS as Record<string, string>)[key];
      expect(zh).toBeDefined();
      expect(zh?.trim().length).toBeGreaterThan(0);
      expect(en).toBeDefined();
      expect(en?.trim().length).toBeGreaterThan(0);
    });
  }

  test("the two newly introduced keys are all-lowercase dotted literals in zh-CN/en-US", () => {
    const newKeys = PROMOTED_CONTROLS.filter((c) => c.isNewKey).map((c) => c.key);
    expect(newKeys).toEqual(["history.rebase.plan.remove", "history.splitter.plan.remove"]);
    for (const file of ["src/i18n/messages/zh-CN.ts", "src/i18n/messages/en-US.ts"]) {
      const src = read(file);
      for (const key of newKeys) {
        // the written literal must be exactly the lowercase key (no case drift)
        const line = src.split("\n").find((l) => l.includes(`"${key}"`));
        expect(line, `${file} is missing "${key}"`).toBeDefined();
        const literal = (line ?? "").split(":")[0]?.trim().replace(/^["']|["']$/g, "") ?? "";
        expect(literal).toBe(key);
        expect(key).toMatch(/^[a-z0-9]+(\.[a-z0-9]+)*$/);
      }
    }
  });
});
