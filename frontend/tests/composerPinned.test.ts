import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import "./domSetup";

/**
 * Pinned commit bar (roadmap §2.1 item 1: 提交区常驻).
 *
 * The CommitComposer used to be docked inside the changes pane, so stacked
 * panes (narrow windows), the source tab and long lists squeezed the whole
 * stage → describe → commit path out of the viewport. Its mount now lives in
 * a dedicated bar pinned to the right column edge (second segment of the
 * two-segment right column), collapsible to one summary line with the choice
 * persisted in localStorage.
 *
 * Placement note: the bar sits at the tail of the right column rather than
 * its top because (a) the ConflictBanner must stay the first element in
 * conflict state without moving it, and (b) the composer's three menus open
 * upward (`bottom-full`) and would be clipped under the app header if the
 * composer sat at the top. Tail placement keeps every existing overlay
 * geometry valid while still being visible in every scroll/tab state.
 *
 * Source assertions are the suite's established pattern for components that
 * need a live Wails backend; the persistence functions are additionally
 * executed against a mock storage so their fault tolerance is actually run,
 * not just grepped.
 */

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (rel: string) => readFileSync(join(root, rel), "utf8");

const wsSrc = read("src/components/WorkspaceView.vue");
const composerSrc = read("src/components/workspace/CommitComposer.vue");

/** Only the markup region — order assertions must ignore <script> imports. */
const templateStart = wsSrc.indexOf("<template>", wsSrc.indexOf("</script>"));
const tpl = wsSrc.slice(templateStart);

function extractFunction(src: string, signature: string): string {
  const start = src.indexOf(signature);
  if (start < 0) throw new Error(`function not found: ${signature}`);
  const bodyStart = src.indexOf("{", start);
  let depth = 0;
  for (let i = bodyStart; i < src.length; i++) {
    if (src[i] === "{") depth++;
    else if (src[i] === "}") {
      depth--;
      if (depth === 0) return src.slice(start, i + 1);
    }
  }
  throw new Error(`unbalanced function body: ${signature}`);
}

function makeStorage(
  map: Map<string, string>,
  opts: { failRead?: boolean; failWrite?: boolean } = {}
) {
  return {
    getItem(k: string): string | null {
      if (opts.failRead) throw new Error("storage unavailable");
      return map.has(k) ? (map.get(k) as string) : null;
    },
    setItem(k: string, v: string) {
      if (opts.failWrite) throw new Error("quota exceeded");
      map.set(k, v);
    },
  };
}

/** Compile the REAL persistence functions against a mock storage. */
function compilePersistenceHarness(
  map: Map<string, string>,
  opts: { failRead?: boolean; failWrite?: boolean } = {}
) {
  const keyDecl = wsSrc.match(/const COMPOSER_COLLAPSED_KEY = "[^"]+";/);
  if (!keyDecl) throw new Error("COMPOSER_COLLAPSED_KEY declaration missing");
  const storage = makeStorage(map, opts);
  const state = { value: false };
  // The source is TypeScript; the harness only needs the annotation stripped.
  const loadFn = extractFunction(wsSrc, "function loadComposerCollapsed").replace(
    /\(\): boolean/,
    "()"
  );
  const toggleFn = extractFunction(wsSrc, "function toggleComposerCollapsed");
  const api = new Function(
    "storageShim",
    "composerCollapsed",
    `
    const localStorage = storageShim;
    ${keyDecl[0]}
    ${loadFn}
    ${toggleFn}
    return { load: loadComposerCollapsed, toggle: toggleComposerCollapsed };
    `
  )(storage, state);
  return { ...api, state: () => state, storage };
}

describe("Pinned commit bar: persistent container", () => {
  const barIdx = tpl.indexOf('data-testid="composer-pin-bar"');

  test("the bar exists as its own segment and is pinned to the column edge", () => {
    expect(barIdx).toBeGreaterThan(-1);
    const classMatch = tpl.slice(barIdx, barIdx + 400).match(/class="([^"]*)"/);
    // sticky structure: pinned segment that never scrolls with the content
    expect(classMatch?.[1]).toContain("sticky bottom-0");
    expect(classMatch?.[1]).toContain("z-20");
    expect(classMatch?.[1]).toContain("shrink-0");
  });

  test("the bar is the last segment of the right column, after both tab contents", () => {
    const panoramaIdx = tpl.indexOf("<SourcePanorama");
    const mainEnd = tpl.indexOf("</main>");
    expect(panoramaIdx).toBeGreaterThan(-1);
    expect(barIdx).toBeGreaterThan(panoramaIdx);
    expect(barIdx).toBeLessThan(mainEnd);
  });

  test("the composer mount was extracted from the changes pane scroll flow", () => {
    // The old docked spot is gone; the composer now mounts inside the bar.
    expect(tpl).not.toContain("Docked Commit Composer at bottom of Changes Pane");
    const composerIdx = tpl.indexOf("<CommitComposer");
    const changesListIdx = tpl.indexOf("<ChangesList");
    const panoramaIdx = tpl.indexOf("<SourcePanorama");
    expect(changesListIdx).toBeGreaterThan(-1);
    expect(composerIdx).toBeGreaterThan(panoramaIdx);
    expect(composerIdx).toBeGreaterThan(changesListIdx);
    const panelIdx = tpl.indexOf('id="wgit-composer-pin-panel"');
    expect(panelIdx).toBeGreaterThan(barIdx);
    expect(composerIdx).toBeGreaterThan(panelIdx);
  });

  test("the composer stays mounted via v-show so collapse keeps its state", () => {
    expect(tpl).toContain('<div v-show="!composerCollapsed" id="wgit-composer-pin-panel">');
    expect(tpl).not.toContain('v-if="!composerCollapsed" id="wgit-composer-pin-panel"');
  });

  test("zen mode keeps hiding the bar (explicit immersive reading state)", () => {
    expect(tpl.slice(Math.max(0, barIdx - 200), barIdx)).toContain('v-if="!isZenMode"');
  });

  test("keyboard paths to the composer expand the bar before focusing", () => {
    expect(wsSrc).toContain("async function expandComposerForFocus()");
    expect(wsSrc).toContain("void expandComposerForFocus();");
    expect(wsSrc).toContain("await nextTick();");
    expect(wsSrc).toContain("commitComposerRef.value?.focus();");
  });

  test("diff/source content scroll behavior is untouched", () => {
    expect(wsSrc).toContain(
      'ref="diffScrollRef" class="flex-1 overflow-auto bg-canvas relative font-mono text-[12px] leading-5 select-text"'
    );
    expect(tpl).toContain('v-else-if="activeRightTab === \'source\'"');
  });
});

describe("Pinned commit bar: collapsed one-line summary", () => {
  const barIdx = tpl.indexOf('data-testid="composer-pin-bar"');
  const barEnd = tpl.indexOf("</main>");
  const barBlock = tpl.slice(barIdx, barEnd);

  test("the toggle row drives collapse/expand with correct aria state", () => {
    expect(barBlock).toContain('data-testid="composer-pin-toggle"');
    expect(barBlock).toContain('@click="toggleComposerCollapsed"');
    expect(barBlock).toContain(':aria-expanded="!composerCollapsed"');
    expect(barBlock).toContain('aria-controls="wgit-composer-pin-panel"');
  });

  test("collapsed summary shows the staged count", () => {
    expect(barBlock).toContain('t("ws.stat.staged", { count: stagedFiles.length })');
    // The count chip never wraps — the row stays one line.
    expect(barBlock).toContain("whitespace-nowrap");
  });

  test("collapsed summary shows the subject preview with a placeholder fallback", () => {
    const previewIdx = barBlock.indexOf('data-testid="composer-pin-subject-preview"');
    expect(previewIdx).toBeGreaterThan(-1);
    // Rendered only in the collapsed state, truncated to a single line.
    expect(barBlock.slice(Math.max(0, previewIdx - 120), previewIdx)).toContain(
      'v-if="composerCollapsed"'
    );
    expect(barBlock).toContain("truncate");
    expect(barBlock).toContain(
      'commitSubject.trim() || t("workspace.composer.subject.placeholder")'
    );
  });

  test("expand affordance: chevron flips with the state", () => {
    expect(barBlock).toContain('<ChevronUp v-if="composerCollapsed"');
    expect(barBlock).toContain("<ChevronDown v-else");
  });

  test("no new i18n keys were introduced — every reused key exists in both locales", () => {
    const usedKeys = [
      "workspace.composer.title",
      "ws.stat.staged",
      "workspace.composer.subject.placeholder",
      "workspace.diff.fold",
    ];
    for (const key of usedKeys) {
      expect((zhCN as Record<string, string>)[key]).toBeDefined();
      expect((enUS as Record<string, string>)[key]).toBeDefined();
    }
  });
});

describe("Pinned commit bar: collapse persistence", () => {
  test("persists under the agreed key wgit.composer.collapsed.v1", () => {
    expect(wsSrc).toContain('const COMPOSER_COLLAPSED_KEY = "wgit.composer.collapsed.v1";');
  });

  test("both the read and the write are guarded by try/catch with rationale", () => {
    expect(wsSrc).toContain("// Benign: storage unavailable/corrupt — the bar just starts expanded.");
    expect(wsSrc).toContain(
      "// Benign: quota/privacy-mode storage failure — collapse still applies"
    );
  });

  test("load: missing, corrupt or unreadable storage all start expanded", () => {
    expect(compilePersistenceHarness(new Map()).load()).toBe(false);
    expect(
      compilePersistenceHarness(new Map([["wgit.composer.collapsed.v1", "true"]])).load()
    ).toBe(true);
    expect(
      compilePersistenceHarness(new Map([["wgit.composer.collapsed.v1", "false"]])).load()
    ).toBe(false);
    expect(
      compilePersistenceHarness(new Map([["wgit.composer.collapsed.v1", "1"]])).load()
    ).toBe(false);
    expect(compilePersistenceHarness(new Map(), { failRead: true }).load()).toBe(false);
  });

  test("toggle: flips the state and writes it back for the next launch", () => {
    const h = compilePersistenceHarness(new Map());
    h.toggle();
    expect(h.state().value).toBe(true);
    expect(h.storage.getItem("wgit.composer.collapsed.v1")).toBe("true");
    h.toggle();
    expect(h.state().value).toBe(false);
    expect(h.storage.getItem("wgit.composer.collapsed.v1")).toBe("false");
  });

  test("toggle: a failing write still applies the in-memory choice", () => {
    const h = compilePersistenceHarness(new Map(), { failWrite: true });
    expect(() => h.toggle()).not.toThrow();
    expect(h.state().value).toBe(true);
  });

  test("round trip: what toggle writes, load reads back", () => {
    const map = new Map<string, string>();
    const writer = compilePersistenceHarness(map);
    writer.toggle();
    expect(compilePersistenceHarness(map).load()).toBe(true);
    writer.toggle();
    expect(compilePersistenceHarness(map).load()).toBe(false);
  });
});

describe("Conflict state: banner stays above the commit area", () => {
  test("ConflictBanner renders before the CommitComposer mount", () => {
    const bannerIdx = tpl.indexOf("<ConflictBanner");
    const composerIdx = tpl.indexOf("<CommitComposer");
    expect(bannerIdx).toBeGreaterThan(-1);
    expect(composerIdx).toBeGreaterThan(bannerIdx);
  });

  test("ConflictBanner contract is untouched", () => {
    expect(tpl).toContain('v-if="hasConflict"');
    expect(tpl).toContain(':conflicted-files="conflictedFiles"');
    expect(tpl).toContain('@continue="handleContinueConflict"');
    expect(tpl).toContain('@abort="handleAbortConflict(true)"');
  });
});

describe("Existing composer contract unchanged", () => {
  test("all pre-existing data-testids in CommitComposer are intact", () => {
    const testids = [
      "smart-generate-btn",
      "commit-draft-history-btn",
      "commit-draft-menu",
      "commit-tools-btn",
      "commit-tools-menu",
      "commit-type-trigger",
      "commit-type-menu",
      "commit-auto-format-btn",
      "conventional-pills-bar",
      "no-verify-checkbox",
      "ai-stream-preview",
      "ai-stream-cancel",
      "ai-preview-payload-btn",
      "ai-generate-commit-msg-btn",
      "commit-lint-badge",
      "gpg-health-chip",
    ];
    for (const id of testids) {
      expect(composerSrc).toContain(`data-testid="${id}"`);
    }
  });

  test("the composer root element keeps its contract (a11y attrs allowed)", () => {
    // 根元素契约：ref 与全部原始 class 逐 token 保留；后续 a11y 增强
    // （role="region" / :aria-label）允许插入，格式变化不破坏契约。
    const rootMatch = composerSrc.match(/<div\s+ref="rootRef"([\s\S]*?)>/);
    expect(rootMatch).toBeTruthy();
    const attrs = rootMatch![1]!.replace(/\s+/g, " ");
    for (const cls of [
      "relative",
      "p-2.5",
      "border-t",
      "border-line",
      "bg-surface/70",
      "shrink-0",
      "flex",
      "flex-col",
      "gap-2",
      "shadow-lg",
    ]) {
      expect(attrs).toContain(cls);
    }
    expect(attrs).toContain('role="region"');
    expect(attrs).toContain(`:aria-label="t('workspace.composer.title')"`);
  });

  test("every composer binding and event moved with the mount verbatim", () => {
    const bindings = [
      'ref="commitComposerRef"',
      'v-model:subject="commitSubject"',
      'v-model:body="commitBody"',
      'v-model:amend="isAmend"',
      'v-model:autoPush="autoPush"',
      'v-model:noVerify="isNoVerify"',
      ':committing="isCommitting"',
      ':is-new-repo="isNewRepo"',
      ':repo-path="currentRepoPath"',
      '@commit="handleCommit"',
      '@fast-commit="handleFastCommit"',
      '@generate="generateSmartCommitMessage"',
      '@audit-sentinel="handleAuditSentinel"',
      '@load-template="handleLoadCommitTemplate"',
      '@load-last="handleLoadLastCommitMessage"',
      '@save-template="handleSaveCommitTemplate"',
      '@notify="notify"',
    ];
    for (const binding of bindings) {
      expect(tpl).toContain(binding);
    }
  });

  test("pre-existing workspace testids are intact", () => {
    expect(wsSrc).toContain('data-testid="ws-source-pane-toggle"');
    expect(wsSrc).toContain('data-testid="ws-stacked-pane-switch"');
  });
});
