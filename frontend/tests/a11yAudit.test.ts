/**
 * Accessibility audit regression suite (plan §2.1 follow-up round).
 *
 * Two kinds of assertions:
 *  1. Fix-point locks — every pure-attribute a11y fix applied in this audit
 *     round is pinned so a future refactor cannot silently drop it.
 *  2. Spot checks — the modal/focus infrastructure (useModalA11y, UiModal,
 *     ariaLabelFallback) and the highest-traffic interactive surface
 *     (CommandPalette) keep their contract, and the decorative-icon
 *     aria-hidden count in the touched files never decreases.
 *
 * The audit scope was src/components/** (except the four hot files
 * WorkspaceView.vue / DiffViewer.vue / HistoryView.vue / aiService.ts, which
 * were audited read-only and reported as backlog) plus src/composables/**.
 */
import { describe, test, expect } from "bun:test";
import { readFileSync } from "fs";
import { resolve } from "path";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const read = (rel: string) => readFileSync(resolve(__dirname, "..", rel), "utf-8");

// ---------------------------------------------------------------------------
// Fix points from the audit round (pure attribute additions only)
// ---------------------------------------------------------------------------

describe("a11y audit: fix-point locks", () => {
  test("StashView clear-search (✕) button has an accessible name", () => {
    const src = read("src/components/StashView.vue");
    expect(src).toContain(':aria-label="t(\'explorer.search.clear\')"');
  });

  test("StashView comparator ref1/ref2 inputs are programmatically labelled", () => {
    const src = read("src/components/StashView.vue");
    expect(src).toContain('for="wgit-comparator-ref1"');
    expect(src).toContain('id="wgit-comparator-ref1"');
    expect(src).toContain('for="wgit-comparator-ref2"');
    expect(src).toContain('id="wgit-comparator-ref2"');
  });

  test("MergeBranchModal strategy select and message input are labelled", () => {
    const src = read("src/components/branches/modals/MergeBranchModal.vue");
    expect(src).toContain('for="wgit-merge-strategy"');
    expect(src).toContain('id="wgit-merge-strategy"');
    expect(src).toContain('for="wgit-merge-message"');
    expect(src).toContain('id="wgit-merge-message"');
  });

  test("ShortcutCheatSheetModal: named dialog, named close and clear buttons", () => {
    const src = read("src/components/ShortcutCheatSheetModal.vue");
    // dialog gets a real accessible name instead of the common.confirm fallback
    expect(src).toContain(':label="t(\'shortcuts.cheatsheet.title\')"');
    expect(src).toContain(':aria-label="t(\'common.close\')"');
    expect(src).toContain(':aria-label="t(\'explorer.search.clear\')"');
  });

  test("ShortcutCheatSheetModal search input has an explicit aria-label", () => {
    const src = read("src/components/ShortcutCheatSheetModal.vue");
    expect(src).toContain(':aria-label="t(\'shortcuts.cheatsheet.search.placeholder\')"');
  });

  test("HeaderBar save-context-session icon button has an accessible name", () => {
    const src = read("src/components/HeaderBar.vue");
    const buttonStart = src.indexOf('data-testid="save-context-session-btn"');
    expect(buttonStart).toBeGreaterThan(0);
    const tagStart = src.lastIndexOf("<button", buttonStart);
    const tagEnd = src.indexOf(">", buttonStart);
    const openTag = src.slice(tagStart, tagEnd);
    expect(openTag).toContain(":aria-label=");
  });

  test("BranchesView sort select exposes an accessible name (title alone is hover-only)", () => {
    const src = read("src/components/BranchesView.vue");
    expect(src).toContain(':aria-label="t(\'branch.sort.title\')"');
  });

  test("SettingsView search input has an explicit aria-label", () => {
    const src = read("src/components/SettingsView.vue");
    expect(src).toContain(':aria-label="t(\'settings.search.placeholder\')"');
  });

  test("every i18n key reused by the fixes exists in both locales", () => {
    const keys = [
      "common.close",
      "common.save",
      "explorer.search.clear",
      "branch.sort.title",
      "settings.search.placeholder",
      "shortcuts.cheatsheet.title",
      "shortcuts.cheatsheet.search.placeholder",
    ];
    for (const key of keys) {
      expect(zhCN[key as keyof typeof zhCN]).toBeDefined();
      expect(enUS[key as keyof typeof enUS]).toBeDefined();
    }
  });
});

// ---------------------------------------------------------------------------
// Spot checks: infrastructure + highest-traffic surfaces
// ---------------------------------------------------------------------------

describe("a11y audit: infrastructure spot checks", () => {
  test("UiModal keeps role=dialog + aria-modal and a labelled close button", () => {
    const src = read("src/components/ui/UiModal.vue");
    expect(src).toContain('role="dialog"');
    expect(src).toContain('aria-modal="true"');
    expect(src).toContain(':aria-label="label || title');
    expect(src).toContain(`:aria-label="t('common.close')"`);
  });

  test("useModalA11y keeps the focus trap (Escape close + Tab wrap) and focus restore", () => {
    const src = read("src/composables/useModalA11y.ts");
    expect(src).toContain('"Escape"');
    expect(src).toContain('"Tab"');
    expect(src).toContain("previousFocus.focus()");
    expect(src).toContain("addEventListener");
  });

  test("ariaLabelFallback still covers placeholder-only controls app-wide", () => {
    const src = read("src/composables/ariaLabelFallback.ts");
    expect(src).toContain("installAriaLabelFallback");
    expect(src).toContain('setAttribute("aria-label", placeholder)');
  });

  test("ConfirmDialog labels its type-to-confirm input", () => {
    const src = read("src/components/ui/ConfirmDialog.vue");
    expect(src).toContain('for="confirm-require-text"');
    expect(src).toContain('id="confirm-require-text"');
  });

  test("UiContextMenu keeps menu semantics (role=menu / menuitem / aria-disabled)", () => {
    const src = read("src/components/ui/UiContextMenu.vue");
    expect(src).toContain('role="menu"');
    expect(src).toContain('role="menuitem"');
    expect(src).toContain(":aria-disabled=");
  });

  test("Toast keeps a live region for async status announcements", () => {
    const src = read("src/components/ui/Toast.vue");
    // danger toasts announce assertively, everything else politely — the
    // tone-driven binding replaced the old static polite literal.
    expect(src).toContain(`:aria-live="toast.tone === 'danger' ? 'assertive' : 'polite'"`);
    expect(src).toContain(`:role="toast.tone === 'danger' ? 'alert' : 'status'"`);
  });
});

describe("a11y audit: high-traffic component spot checks", () => {
  test("CommandPalette keeps combobox/listbox wiring inside a labelled dialog", () => {
    const src = read("src/components/CommandPalette.vue");
    expect(src).toContain('role="combobox"');
    expect(src).toContain('aria-autocomplete="list"');
    expect(src).toContain(':aria-controls="\'palette-listbox\'"');
    expect(src).toContain(":aria-activedescendant=");
    expect(src).toContain('role="listbox"');
    expect(src).toContain(`:label="t('cmd.aria')"`);
    // every palette row action button keeps an explicit accessible name
    expect(src).toContain(`:aria-label="t('cmd.pin.up')"`);
    expect(src).toContain(`:aria-label="t('cmd.pin.down')"`);
    expect(src).toContain(`:aria-label="isPinned(entry.cmd.id) ? t('cmd.pin.remove') : t('cmd.pin.add')"`);
  });

  test("BranchesView refresh/drift icon buttons keep accessible names", () => {
    const src = read("src/components/BranchesView.vue");
    expect(src).toContain(`:aria-label="t('branch.refresh')"`);
    expect(src).toContain('data-testid="branch-drift-inspect-btn"');
    expect(src).toContain('role="alert"');
  });

  test("ReviewView header icon buttons keep accessible names and pressed state", () => {
    const src = read("src/components/ReviewView.vue");
    expect(src).toContain(`:aria-label="t('review.batch.approve')"`);
    expect(src).toContain(`:aria-label="t('review.refresh')"`);
    expect(src).toContain(":aria-pressed=");
  });

  test("StashView header icon buttons keep accessible names", () => {
    const src = read("src/components/StashView.vue");
    expect(src).toContain(`:aria-label="t('stash.refresh')"`);
    expect(src).toContain(`:aria-label="t('stash.snapshot.btn')"`);
    expect(src).toContain(`:aria-label="t('stash.comparator.title')"`);
    expect(src).toContain(`:aria-label="t('stash.clear.title')"`);
  });

  test("ToolsView primary tabs and more-menu trigger keep names/expanded state", () => {
    const src = read("src/components/ToolsView.vue");
    expect(src).toContain(':aria-label="t(def.labelKey)"');
    expect(src).toContain(":aria-expanded=");
  });
});

describe("a11y audit: decorative icon aria-hidden counts never decrease", () => {
  const floors: Record<string, number> = {
    // files edited in this round: the fix added aria-hidden markers
    "src/components/BranchesView.vue": 4,
    "src/components/branches/modals/MergeBranchModal.vue": 2,
    "src/components/ShortcutCheatSheetModal.vue": 3,
    "src/components/HeaderBar.vue": 1,
    // file already carrying the convention before this round
    "src/components/ui/Toast.vue": 1,
  };

  for (const [file, floor] of Object.entries(floors)) {
    if (floor === 0) continue;
    test(`${file} keeps >= ${floor} aria-hidden decorative markers`, () => {
      const src = read(file);
      const count = (src.match(/aria-hidden/g) ?? []).length;
      expect(count).toBeGreaterThanOrEqual(floor);
    });
  }
});

// ---------------------------------------------------------------------------
// Fix points from round 2 (U5 backlog #5): placeholder-only inputs get an
// explicit :aria-label reusing the same i18n key as their placeholder, so the
// accessible name no longer depends on the runtime ariaLabelFallback.
// ---------------------------------------------------------------------------

describe("a11y audit round 2: placeholder inputs carry explicit aria-labels", () => {
  // file -> [(v-model, i18n key shared by :placeholder and :aria-label)]
  const fixes: [string, [string, string][]][] = [
    [
      "src/components/BisectPanel.vue",
      [
        ["autoCommand", "bisect.auto.placeholder"],
        ["goodRefInput", "bisect.good.placeholder"],
      ],
    ],
    ["src/components/GitignoreEditor.vue", [["newPattern", "gitignore.placeholder"]]],
    [
      "src/components/RebaseModal.vue",
      [
        ["selectedUpstream", "rebase.base.placeholder"],
        ["rebaseOnto", "rebase.onto.placeholder"],
      ],
    ],
    [
      "src/components/ReleaseWizard.vue",
      [
        ["remoteInput", "release.remote.placeholder"],
        ["fromRefInput", "release.changelog.from.placeholder"],
        ["verifyPathInput", "release.checksum.verify.path.placeholder"],
        ["verifyHashInput", "release.checksum.verify.hash.placeholder"],
      ],
    ],
    ["src/components/RepoDashboard.vue", [["searchQuery", "dash.filter.placeholder"]]],
    ["src/components/RepoOnboarding.vue", [["scanRoot", "onboard.scan.placeholder"]]],
  ];

  for (const [file, inputs] of fixes) {
    for (const [model, key] of inputs) {
      test(`${file}: ${model} input is labelled via t('${key}')`, () => {
        const src = read(file);
        // locate the opening tag of the input bound to this v-model
        const modelIdx = src.indexOf(`v-model="${model}"`);
        expect(modelIdx).toBeGreaterThan(-1);
        const tagStart = src.lastIndexOf("<input", modelIdx);
        const tagEnd = src.indexOf(">", modelIdx);
        expect(tagStart).toBeGreaterThan(-1);
        expect(tagEnd).toBeGreaterThan(modelIdx);
        const openTag = src.slice(tagStart, tagEnd);
        // aria-label and placeholder reuse the SAME key (zero new i18n keys)
        expect(openTag).toContain(`:aria-label="t('${key}')"`);
        expect(openTag).toContain(`:placeholder="t('${key}')"`);
      });
    }
  }

  test("every round-2 aria-label key exists in both locales", () => {
    const keys = [
      "bisect.auto.placeholder",
      "bisect.good.placeholder",
      "gitignore.placeholder",
      "rebase.base.placeholder",
      "rebase.onto.placeholder",
      "release.remote.placeholder",
      "release.changelog.from.placeholder",
      "release.checksum.verify.path.placeholder",
      "release.checksum.verify.hash.placeholder",
      "dash.filter.placeholder",
      "onboard.scan.placeholder",
    ];
    for (const key of keys) {
      expect(zhCN[key as keyof typeof zhCN]).toBeDefined();
      expect(enUS[key as keyof typeof enUS]).toBeDefined();
    }
  });

  // Skipped-by-design lock: these inputs were left without an aria-label in
  // round 2 because a wrapping <label> already gives them an accessible name;
  // if that label association is ever removed the fix must be revisited.
  test("RepoOnboarding clone/init inputs keep their wrapping-label association", () => {
    const src = read("src/components/RepoOnboarding.vue");
    for (const model of ["cloneURL", "cloneTarget", "cloneBranch", "initPath"]) {
      const modelIdx = src.indexOf(`v-model="${model}"`);
      expect(modelIdx).toBeGreaterThan(-1);
      const tagStart = src.lastIndexOf("<input", modelIdx);
      const tagEnd = src.indexOf(">", modelIdx);
      const openTag = src.slice(tagStart, tagEnd);
      expect(openTag).not.toContain(":aria-label=");
      // the wrapping <label> must still open before the input and close after it
      const labelOpen = src.lastIndexOf("<label", modelIdx);
      const labelClose = src.indexOf("</label>", modelIdx);
      expect(labelOpen).toBeGreaterThan(-1);
      expect(labelClose).toBeGreaterThan(modelIdx);
      expect(labelOpen).toBeLessThan(tagStart);
    }
  });
});

// ---------------------------------------------------------------------------
// Round 3 (U5 explicit-label progressive slice 2): per-input audit of the
// remaining settings / tools / modal surfaces. Same contract as round 2: an
// input whose :placeholder is a plain t(key) and that has no other accessible
// name (aria-label, for/id label, wrapping label) gets :aria-label="t(same
// key)". Every skip decision is locked together with the reason it was made.
// ---------------------------------------------------------------------------

describe("a11y audit round 3: placeholder inputs carry explicit aria-labels", () => {
  // [file, control tag, [(v-model, i18n key shared by :placeholder and :aria-label)]]
  const fixes: [string, string, [string, string][]][] = [
    [
      "src/components/settings/GitHooksSection.vue",
      "input",
      [["searchQuery", "settings.hooks.search.placeholder"]],
    ],
    [
      "src/components/ThreeWayMergeModal.vue",
      "textarea",
      [["currentResult", "merge.3way.editor.placeholder"]],
    ],
    [
      "src/components/settings/AliasesSection.vue",
      "input",
      [["filter", "settings.aliases.search.placeholder"]],
    ],
    [
      "src/components/settings/ToolsSection.vue",
      "input",
      [["unityMergeCustomPath", "settings.tools.unity.yaml.placeholder"]],
    ],
    [
      "src/components/SyncModal.vue",
      "input",
      [["pushRemoteBranch", "sync.push.remote.placeholder"]],
    ],
    [
      "src/components/HistoryView.vue",
      "textarea",
      [["noteInputText", "history.note.placeholder"]],
    ],
  ];

  for (const [file, tag, inputs] of fixes) {
    for (const [model, key] of inputs) {
      test(`${file}: ${model} ${tag} is labelled via t('${key}')`, () => {
        const src = read(file);
        // locate the opening tag of the control bound to this v-model
        const modelIdx = src.indexOf(`v-model="${model}"`);
        expect(modelIdx).toBeGreaterThan(-1);
        const tagStart = src.lastIndexOf(`<${tag}`, modelIdx);
        const tagEnd = src.indexOf(">", modelIdx);
        expect(tagStart).toBeGreaterThan(-1);
        expect(tagEnd).toBeGreaterThan(modelIdx);
        const openTag = src.slice(tagStart, tagEnd);
        // aria-label and placeholder reuse the SAME key (zero new i18n keys)
        expect(openTag).toContain(`:aria-label="t('${key}')"`);
        expect(openTag).toContain(`:placeholder="t('${key}')"`);
      });
    }
  }

  test("every round-3 aria-label key exists in both locales", () => {
    const keys = [
      "settings.hooks.search.placeholder",
      "merge.3way.editor.placeholder",
      "settings.aliases.search.placeholder",
      "settings.tools.unity.yaml.placeholder",
      "sync.push.remote.placeholder",
      "history.note.placeholder",
    ];
    for (const key of keys) {
      expect(zhCN[key as keyof typeof zhCN]).toBeDefined();
      expect(enUS[key as keyof typeof enUS]).toBeDefined();
    }
  });

  // --- Skip-decision locks: recorded so the reason survives refactors ------

  test("HistoryView range cherry-pick inputs keep their wrapping labels (skip: implicit label name)", () => {
    const src = read("src/components/HistoryView.vue");
    for (const model of ["rangeStartRef", "rangeEndRef"]) {
      const modelIdx = src.indexOf(`v-model="${model}"`);
      expect(modelIdx).toBeGreaterThan(-1);
      const tagStart = src.lastIndexOf("<input", modelIdx);
      const tagEnd = src.indexOf(">", modelIdx);
      const openTag = src.slice(tagStart, tagEnd);
      expect(openTag).not.toContain(":aria-label=");
      expect(openTag).toContain(`:placeholder="t('history.range.${model === "rangeStartRef" ? "start" : "end"}.placeholder')"`);
      // the wrapping <label> must still open before the input and close after it
      const labelOpen = src.lastIndexOf("<label", modelIdx);
      const labelClose = src.indexOf("</label>", modelIdx);
      expect(labelOpen).toBeGreaterThan(-1);
      expect(labelClose).toBeGreaterThan(modelIdx);
      expect(labelOpen).toBeLessThan(tagStart);
    }
  });

  test("ThreeWayMergeModal auto-stage checkbox keeps its wrapping label (skip)", () => {
    const src = read("src/components/ThreeWayMergeModal.vue");
    const modelIdx = src.indexOf(`v-model="autoStage"`);
    expect(modelIdx).toBeGreaterThan(-1);
    const tagStart = src.lastIndexOf("<input", modelIdx);
    const labelOpen = src.lastIndexOf("<label", modelIdx);
    const labelClose = src.indexOf("</label>", modelIdx);
    expect(labelOpen).toBeGreaterThan(-1);
    expect(labelClose).toBeGreaterThan(modelIdx);
    expect(labelOpen).toBeLessThan(tagStart);
  });

  test("AliasesSection scope radios keep their wrapping labels (skip)", () => {
    const src = read("src/components/settings/AliasesSection.vue");
    for (const value of ["global", "local"]) {
      const modelIdx = src.indexOf(`value="${value}"`);
      expect(modelIdx).toBeGreaterThan(-1);
      const tagStart = src.lastIndexOf("<input", modelIdx);
      const labelOpen = src.lastIndexOf("<label", modelIdx);
      const labelClose = src.indexOf("</label>", modelIdx);
      expect(labelOpen).toBeGreaterThan(-1);
      expect(labelClose).toBeGreaterThan(modelIdx);
      expect(labelOpen).toBeLessThan(tagStart);
    }
  });

  test("StashView save-modal checkboxes keep their wrapping labels (skip)", () => {
    const src = read("src/components/StashView.vue");
    for (const model of ["saveIncludeUntracked", "saveIncludeAll", "saveKeepIndex", "saveStagedOnly"]) {
      const modelIdx = src.indexOf(`v-model="${model}"`);
      expect(modelIdx).toBeGreaterThan(-1);
      const tagStart = src.lastIndexOf("<input", modelIdx);
      const labelOpen = src.lastIndexOf("<label", modelIdx);
      const labelClose = src.indexOf("</label>", modelIdx);
      expect(labelOpen).toBeGreaterThan(-1);
      expect(labelClose).toBeGreaterThan(modelIdx);
      expect(labelOpen).toBeLessThan(tagStart);
    }
  });

  test("LocalAiSection api-key input keeps its mixed ternary placeholder unlabelled (skip: not a plain t(key) placeholder)", () => {
    const src = read("src/components/settings/LocalAiSection.vue");
    const modelIdx = src.indexOf(`v-model="form.apiKey"`);
    expect(modelIdx).toBeGreaterThan(-1);
    const tagStart = src.lastIndexOf("<input", modelIdx);
    const tagEnd = src.indexOf(">", modelIdx);
    const openTag = src.slice(tagStart, tagEnd);
    // the placeholder alternates between an i18n branch and a static example,
    // so naming it with a single t() key would be wrong in the other state.
    expect(openTag).toContain(`:placeholder="backend?.keyAvailable ? t('settings.localai.key.kept') : 'sk-...'"`);
    expect(openTag).not.toContain(":aria-label=");
  });

  test("LocalAiSection commit-instructions textarea keeps its for/id label association (skip)", () => {
    const src = read("src/components/settings/LocalAiSection.vue");
    expect(src).toContain('for="localai-commit-instructions"');
    expect(src).toContain('id="localai-commit-instructions"');
  });

  test("ToolsSection custom diff/merge inputs keep static example placeholders with explicit labels (recorded, no key reuse)", () => {
    const src = read("src/components/settings/ToolsSection.vue");
    // static placeholders stay static (no fake i18n key invented)…
    expect(src).toContain('placeholder="bcomp {left} {right}"');
    expect(src).toContain('placeholder="code --wait --merge {theirs} {ours} {base} {result}"');
    // …and each input keeps an explicit aria-label from its section heading
    expect(src).toContain(`:aria-label="t('settings.tools.custom.diff')"`);
    expect(src).toContain(`:aria-label="t('settings.tools.custom.merge')"`);
  });
});
