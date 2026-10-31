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
 * dropped the floor to 243). 6 icon occurrences remain
 * RECORD —
 * the three icon-only unnamed copy buttons in tools/GitBundleModal.vue
 * (Check/Copy pairs at the path/clone/verify rows) need aria-labels from new
 * i18n keys and belong to a later slice; the RECORD guard below pins that
 * they must never get aria-hidden while still unnamed.
 */
import { describe, test, expect } from "bun:test";
import { readFileSync } from "fs";
import { resolve } from "path";

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
  "src/components/tools/GitBundleModal.vue": 4,
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

describe("a11y icons: aria-hidden floors (never decrease)", () => {
  for (const [mapName, map] of [
    ["slice 1", ADDED_ARIA_HIDDEN],
    ["slice 2", SLICE2_ARIA_HIDDEN],
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

  test("the slice-2 touched files keep >= 243 aria-hidden decorative markers in total", () => {
    const total = Object.keys(SLICE2_ARIA_HIDDEN).reduce(
      (sum, f) => sum + (read(f).match(/aria-hidden/g) ?? []).length,
      0,
    );
    expect(total).toBeGreaterThanOrEqual(243);
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
    .replace(/<button[\s\S]*?>/, "")
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
