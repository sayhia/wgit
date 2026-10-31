import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

/**
 * Narrow-window left-pane collapse (roadmap §2.1 item 9).
 *
 * WorkspaceView already adapts below 900px of workspace width (stacked
 * changes/diff) and pins its toolbars to container widths, but between that
 * regime and the dual-pane breakpoint the source tree permanently ate 200px+
 * from the changes/diff pair. The pane can now collapse to a 36px rail while
 * the workspace is narrow, with the choice persisted across launches.
 *
 * Compatibility contract with tests/shellLayout.test.ts (the parallel
 * narrow-window layout suite):
 *   - the three pinned layout literals (the two `w-max` tab-strip rows and
 *     the `@min-[700px]` staged/unstaged counters) must survive verbatim —
 *     asserted again here as reverse protection;
 *   - the two pinned `v-show="showSourcePane"` bindings keep their exact
 *     meaning (user show/hide), so the collapse hides the pane through a
 *     wrapper element instead of editing those bindings;
 *   - below the collapse breakpoint nothing changes unless the user opts in:
 *     the collapsed rail only renders when the collapse is actually active;
 *   - the right column (composer-pin-bar included) is untouched: the whole
 *     feature lives in the left-pane region before the right `<main>`.
 *
 * Source assertions are the suite's established pattern for components that
 * need a live Wails backend; the persistence functions are additionally
 * executed against a mock storage so their fault tolerance and their
 * coexistence with the per-repo paneSizes map are actually run, not just
 * grepped.
 */

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (rel: string) => readFileSync(join(root, rel), "utf8");

const wsSrc = read("src/components/WorkspaceView.vue");

/** Only the markup region — order assertions must ignore <script> imports. */
const templateStart = wsSrc.indexOf("<template>", wsSrc.indexOf("</script>"));
const tpl = wsSrc.slice(templateStart);
/** The right column starts at <main; everything left of it is the left pane. */
const mainIdx = tpl.indexOf("<main");

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

describe("Left-pane collapse: breakpoint interval", () => {
  const belowDecl = wsSrc.match(/const LEFT_PANE_COLLAPSE_BELOW = \d+;/);

  test("the breakpoint is the 1280px dual-pane edge, measured on the workspace element", () => {
    expect(belowDecl).not.toBeNull();
    expect(belowDecl![0]).toBe("const LEFT_PANE_COLLAPSE_BELOW = 1280;");
    // Same measurement system as STACK_PANES_BELOW (workspace element, not
    // window): the dock and rail already eat part of the window width.
    expect(wsSrc).toContain("isLeftPaneCollapseWidth(workspaceWidth.value)");
  });

  /** Compile the REAL width predicate against the real constant. */
  function widthApplies(width: number): boolean {
    // The signature carries a TS return annotation; strip it before handing
    // the body to new Function (plain JS there).
    const plain = wsSrc.replace(
      /function isLeftPaneCollapseWidth\(width: number\): boolean/,
      "function isLeftPaneCollapseWidth(width)"
    );
    return new Function(
      `${belowDecl![0]}\n${extractFunction(plain, "function isLeftPaneCollapseWidth")}\nreturn isLeftPaneCollapseWidth;`
    )()(width) as boolean;
  }

  test("collapse applies in the narrow interval (0, 1280) — including the stacked regime", () => {
    // Before the ResizeObserver fires the width is unmeasured: never collapse.
    expect(widthApplies(0)).toBe(false);
    expect(widthApplies(1)).toBe(true);
    expect(widthApplies(700)).toBe(true);
    // Below 900 the changes/diff stack; collapsing the tree there is still
    // legitimate (it only frees more room) but nothing changes by default.
    expect(widthApplies(899)).toBe(true);
    expect(widthApplies(1279)).toBe(true);
  });

  test("at ≥1280px the dual-pane layout is kept and the controls disappear", () => {
    expect(widthApplies(1280)).toBe(false);
    expect(widthApplies(2000)).toBe(false);
  });

  test("a stored collapse flag survives a wide window instead of being reset", () => {
    // Same contract as fitPaneWidth: the stored preference is left untouched
    // so narrowing the window again restores the collapsed rail.
    expect(wsSrc).toContain(
      "const isLeftPaneCollapsed = computed(\n  () => leftPaneCollapsed.value && canCollapseLeftPane.value\n);"
    );
  });
});

describe("Left-pane collapse: persistence and fault tolerance", () => {
  const keyDecl = wsSrc.match(/const LEFT_PANE_COLLAPSED_STORAGE_KEY = "[^"]+";/);

  test("uses its own storage key, separate from every existing pane key", () => {
    expect(keyDecl).not.toBeNull();
    expect(keyDecl![0]).toBe('const LEFT_PANE_COLLAPSED_STORAGE_KEY = "wgit.workspace.leftPane.collapsed.v1";');
    for (const other of [
      "wgit.workspace.paneSizesByRepo.v1",
      "wgit.workspace.sourcePane.v1",
      "wgit.workspace.paneSizes.v1",
      "wgit.composer.collapsed.v1",
    ]) {
      expect(keyDecl![0]).not.toContain(other);
    }
  });

  /** Compile the REAL collapse persistence functions against a mock storage. */
  function compileCollapseHarness(
    map: Map<string, string>,
    opts: { failRead?: boolean; failWrite?: boolean } = {}
  ) {
    const loadFn = extractFunction(wsSrc, "function loadLeftPaneCollapsed").replace(
      /\(\): boolean/,
      "()"
    );
    // Strip the parameter annotation; new Function compiles plain JS.
    const setFn = extractFunction(
      wsSrc.replace(
        /function setLeftPaneCollapsed\(collapsed: boolean\)/,
        "function setLeftPaneCollapsed(collapsed)"
      ),
      "function setLeftPaneCollapsed"
    );
    const state = { value: false };
    const api = new Function(
      "storageShim",
      "leftPaneCollapsed",
      `
      const localStorage = storageShim;
      ${keyDecl![0]}
      ${loadFn}
      ${setFn}
      return { load: loadLeftPaneCollapsed, set: setLeftPaneCollapsed };
      `
    )(makeStorage(map, opts), state);
    return { ...api, state: () => state, map };
  }

  test("persists an explicit collapse and reloads it", () => {
    const map = new Map<string, string>();
    const harness = compileCollapseHarness(map);
    expect(harness.load()).toBe(false); // default: expanded
    harness.set(true);
    expect(harness.state().value).toBe(true);
    expect(map.get("wgit.workspace.leftPane.collapsed.v1")).toBe("true");
    expect(compileCollapseHarness(map).load()).toBe(true); // cross-launch memory
    harness.set(false);
    expect(map.get("wgit.workspace.leftPane.collapsed.v1")).toBe("false");
    expect(harness.state().value).toBe(false);
  });

  test("unreadable storage degrades to expanded instead of crashing", () => {
    const harness = compileCollapseHarness(new Map(), { failRead: true });
    expect(harness.load()).toBe(false);
  });

  test("an unwritable store keeps the in-memory collapse for this session", () => {
    const harness = compileCollapseHarness(new Map(), { failWrite: true });
    harness.set(true);
    expect(harness.state().value).toBe(true);
    expect(harness.map.size).toBe(0); // nothing written, nothing corrupted
  });

  test("collapsing never touches the pane widths (expanded restores the stored width)", () => {
    const setFn = extractFunction(wsSrc, "function setLeftPaneCollapsed");
    expect(setFn).not.toContain("leftPaneWidth");
    expect(setFn).not.toContain("changesPaneWidth");
    expect(setFn).not.toContain("persistPaneSizes");
  });
});

describe("Left-pane collapse: coexists with the per-repo paneSizes map", () => {
  /** Compile the REAL pane-size functions against a mock storage. */
  function compilePaneSizesHarness(map: Map<string, string>) {
    const paneKeyDecl = wsSrc.match(/const PANE_SIZES_STORAGE_KEY = "[^"]+";/);
    expect(paneKeyDecl).not.toBeNull();
    // The return annotation contains braces, which the brace-counting
    // extractor would mistake for the body — strip it before extracting.
    const plain = wsSrc.replace(
      /function loadPaneSizes\(repoPath: string\): \{ left: number; changes: number \}/,
      "function loadPaneSizes(repoPath)"
    );
    const loadFn = extractFunction(plain, "function loadPaneSizes");
    const persistFn = extractFunction(plain, "function persistPaneSizes");
    const identity = (v: number) => v;
    return new Function(
      "storageShim",
      "props",
      "leftPaneWidth",
      "changesPaneWidth",
      "clampLeftPane",
      "clampChangesPane",
      `
      const localStorage = storageShim;
      ${paneKeyDecl![0]}
      ${loadFn}
      ${persistFn}
      return { loadPaneSizes, persistPaneSizes };
      `
    )(
      makeStorage(map),
      { currentRepoPath: "/repo/a" },
      { value: 320 },
      { value: 320 },
      identity,
      identity
    );
  }

  test("persistPaneSizes writes widths only — the collapse flag is untouched", () => {
    const map = new Map<string, string>();
    map.set("wgit.workspace.leftPane.collapsed.v1", "true");
    compilePaneSizesHarness(map).persistPaneSizes();
    expect(map.get("wgit.workspace.paneSizesByRepo.v1")).toContain('"/repo/a"');
    expect(map.get("wgit.workspace.leftPane.collapsed.v1")).toBe("true");
  });

  test("a collapse toggle writes the boolean only — the widths map is untouched", () => {
    const map = new Map<string, string>();
    const sizes = JSON.stringify({ "/repo/a": { left: 480, changes: 240 } });
    map.set("wgit.workspace.paneSizesByRepo.v1", sizes);
    const keyDecl2 = wsSrc.match(/const LEFT_PANE_COLLAPSED_STORAGE_KEY = "[^"]+"/)![0];
    const setFn = extractFunction(
      wsSrc.replace(
        /function setLeftPaneCollapsed\(collapsed: boolean\)/,
        "function setLeftPaneCollapsed(collapsed)"
      ),
      "function setLeftPaneCollapsed"
    );
    const state = { value: false };
    new Function(
      "storageShim",
      "leftPaneCollapsed",
      `
      const localStorage = storageShim;
      ${keyDecl2};
      ${setFn}
      return setLeftPaneCollapsed;
      `
    )(makeStorage(map), state)(true);
    expect(state.value).toBe(true);
    expect(map.get("wgit.workspace.paneSizesByRepo.v1")).toBe(sizes);
    expect(map.get("wgit.workspace.leftPane.collapsed.v1")).toBe("true");
  });

  test("both loaders run cleanly side by side on the same storage", () => {
    const map = new Map<string, string>();
    const sizesApi = compilePaneSizesHarness(map);
    sizesApi.persistPaneSizes();
    const keyDecl3 = wsSrc.match(/const LEFT_PANE_COLLAPSED_STORAGE_KEY = "[^"]+";/)![0];
    const loadFn = extractFunction(wsSrc, "function loadLeftPaneCollapsed").replace(
      /\(\): boolean/,
      "()"
    );
    const loadCollapsed = new Function(
      "storageShim",
      `
      const localStorage = storageShim;
      ${keyDecl3}
      ${loadFn}
      return loadLeftPaneCollapsed;
      `
    )(makeStorage(map));
    expect(sizesApi.loadPaneSizes("/repo/a")).toEqual({ left: 320, changes: 320 });
    expect(loadCollapsed()).toBe(false);
  });
});

describe("Left-pane collapse: template placement and shellLayout contract", () => {
  test("the wrapper hides pane + divider on collapse without editing the pinned bindings", () => {
    expect(tpl).toContain('<div v-show="showSourcePane && !isLeftPaneCollapsed"');
    // Reverse protection for shellLayout.test.ts: the two explicit show/hide
    // bindings must stay verbatim and stay count-exact.
    expect(wsSrc).toContain('v-show="showSourcePane"');
    expect((wsSrc.match(/v-show="showSourcePane"/g) ?? []).length).toBe(2);
  });

  test("both the collapse chevron and the 36px rail live in the left-pane region", () => {
    const collapseIdx = tpl.indexOf('data-testid="ws-left-pane-collapse"');
    const railIdx = tpl.indexOf('data-testid="ws-left-pane-rail"');
    expect(collapseIdx).toBeGreaterThan(-1);
    expect(railIdx).toBeGreaterThan(-1);
    expect(collapseIdx).toBeLessThan(mainIdx);
    expect(railIdx).toBeGreaterThan(collapseIdx);
    expect(railIdx).toBeLessThan(mainIdx);
  });

  test("the collapse control only exists while the workspace is narrow", () => {
    const start = tpl.indexOf('data-testid="ws-left-pane-collapse"');
    const button = tpl.slice(start - 200, start + 700);
    expect(button).toContain('v-if="canCollapseLeftPane"');
    expect(button).toContain("@click=\"setLeftPaneCollapsed(true)\"");
    // Icon + aria-label reuse the existing pane keys — no new i18n.
    expect(button).toContain(":aria-label=\"t('ws.pane.source.hide')\"");
  });

  test("the collapsed rail is a 36px strip with an expand control", () => {
    const start = tpl.indexOf('data-testid="ws-left-pane-rail"');
    const rail = tpl.slice(start, start + 400);
    // w-9 = 36px, inside the 32–40px band.
    expect(rail).toContain("w-9 shrink-0");
    const expand = tpl.slice(tpl.indexOf('data-testid="ws-left-pane-expand"'), tpl.indexOf('data-testid="ws-left-pane-expand"') + 500);
    expect(expand).toContain("@click=\"setLeftPaneCollapsed(false)\"");
    expect(expand).toContain(":aria-label=\"t('ws.pane.source.show')\"");
  });

  test("the right column (composer-pin-bar included) carries none of the feature", () => {
    const rightColumn = tpl.slice(mainIdx);
    expect(rightColumn).toContain('data-testid="composer-pin-bar"');
    expect(rightColumn).not.toContain("ws-left-pane");
    expect(rightColumn).not.toContain("isLeftPaneCollapsed");
  });

  test("the three shellLayout-pinned literals survive verbatim", () => {
    // WorkspaceView top-bar rows (L2651/L2780) and the @min-[700px] counters
    // (L2680) are pinned by tests/shellLayout.test.ts — guarded again here so
    // a later collapse refactor cannot silently break that contract.
    expect(
      (wsSrc.match(/flex w-max min-w-full items-center justify-end gap-2 whitespace-nowrap/g) ?? []).length
    ).toBe(2);
    expect(wsSrc).toContain("hidden @min-[700px]:flex items-center gap-2 text-xs");
  });

  test("the reused labels exist in both locales (no new i18n keys)", () => {
    for (const key of ["ws.pane.source.show", "ws.pane.source.hide"] as const) {
      expect(zhCN[key]).toBeTruthy();
      expect(enUS[key]).toBeTruthy();
    }
  });
});
