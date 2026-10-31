import { describe, expect, test } from "bun:test";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

/**
 * Resource lifecycle audit (dimension: defect/memory).
 *
 * Every registration of a cleanup-needing resource (setInterval / setTimeout
 * chains / addEventListener on window|document / ResizeObserver /
 * MutationObserver / EventSource / requestAnimationFrame) that lives inside a
 * Vue component must have a matching teardown in onUnmounted/onBeforeUnmount,
 * because views are mounted and unmounted on every tab switch — a missing
 * teardown is amplified into repeated callbacks and unbounded retention.
 *
 * Module-level singletons (created outside components, app-lifetime) are
 * exempt from component teardown but must be documented as intentional.
 *
 * Style: source assertions (regex on file text), matching this repo's
 * source-audit test conventions.
 */

const srcRoot = fileURLToPath(new URL("../src", import.meta.url));

function read(path: string): string {
  return readFileSync(join(srcRoot, path), "utf8");
}

/** 去空白归一化，避免格式差异影响断言。 */
const compact = (s: string) => s.replace(/\s+/g, "");

function blockOf(source: string, startMarker: string, endMarker: string): string {
  const start = source.indexOf(startMarker);
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf(endMarker, start);
  return source.slice(start, end === -1 ? undefined : end + endMarker.length);
}

function countOf(source: string, needle: string): number {
  return source.split(needle).length - 1;
}

/** Collect all .vue / .ts source files under src/ (excluding tests). */
function listSourceFiles(dir = srcRoot): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      out.push(...listSourceFiles(full));
    } else if ((name.endsWith(".vue") || name.endsWith(".ts")) && !name.endsWith(".d.ts")) {
      out.push(full);
    }
  }
  return out;
}

// ---------------------------------------------------------------------------
// 1. EventSource / WebSocket — the one confirmed leak, fixed here.
// ---------------------------------------------------------------------------

describe("LanReviewModal EventSource teardown", () => {
  const file = "components/review/LanReviewModal.vue";
  const script = blockOf(read(file), "<script", "</script>");

  test("opens exactly one EventSource and closes it in disconnectEventSource", () => {
    expect(compact(script)).toContain("newEventSource(");
    expect(compact(script)).toContain("eventSource.close()");
  });

  test("unmount closes the stream (tab switch while open must not leak the connection)", () => {
    // ReviewView mounts LanReviewModal unconditionally and is itself torn down
    // by v-else-if on every App tab switch, so watch()-based close alone is
    // not enough — the watch dies with the component.
    expect(compact(script)).toContain("onUnmounted(disconnectEventSource)");
    expect(compact(script)).toContain("onUnmounted");
  });

  test("whole src has no other WebSocket / EventSource / IntersectionObserver", () => {
    const all = listSourceFiles().map((f) => readFileSync(f, "utf8")).join("\n");
    expect(countOf(all, "new EventSource")).toBe(1);
    expect(countOf(all, "new WebSocket")).toBe(0);
    expect(countOf(all, "IntersectionObserver")).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// 2. Window/document listeners — registration and removal must be paired.
// ---------------------------------------------------------------------------

describe("paired window/document listeners (add count == remove count + unmount hook)", () => {
  const pairedFiles = [
    "App.vue",
    "components/CommandPalette.vue",
    "components/RepoDashboard.vue",
    "components/WorkspaceView.vue",
    "components/HistoryView.vue",
    "components/ActivityDock.vue",
    "components/ThreeWayMergeModal.vue",
    "components/QuickOpen.vue",
    "components/ToolsView.vue",
    "components/ReviewView.vue",
    "components/HeaderBar.vue",
    "components/BranchesView.vue",
    "components/SettingsView.vue",
    "components/settings/ShortcutsSection.vue",
    "components/workspace/SourceExplorer.vue",
    "components/workspace/ChangesList.vue",
    "components/assets/Model3dDiffViewer.vue",
    "components/workspace/DiffViewer.vue",
    "components/workspace/CommitComposer.vue",
  ];

  for (const file of pairedFiles) {
    test(`${file}: every addEventListener has a removal and an unmount hook`, () => {
      const src = read(file);
      expect(countOf(src, "addEventListener(")).toBe(countOf(src, "removeEventListener("));
      expect(compact(src)).toMatch(/onUnmounted|onBeforeUnmount/);
    });
  }

  test("drag-style files may remove more than they add (defensive double removal)", () => {
    // CommitGraphList removes the minimap drag listeners both in the drag-stop
    // handler AND in onUnmounted (covers unmount-mid-drag); UiContextMenu
    // removes in both watch(false) and onUnmounted.
    for (const file of ["components/history/CommitGraphList.vue", "components/ui/UiContextMenu.vue"]) {
      const src = read(file);
      expect(countOf(src, "removeEventListener(")).toBeGreaterThanOrEqual(countOf(src, "addEventListener("));
      expect(compact(src)).toMatch(/onUnmounted|onBeforeUnmount/);
    }
  });

  test("CommitGraphList unmount disconnects its ResizeObserver and drag listeners", () => {
    const unmount = blockOf(read("components/history/CommitGraphList.vue"), "onUnmounted(", "});");
    const c = compact(unmount);
    expect(c).toContain("listResizeObserver.disconnect()");
    expect(c).toContain('window.removeEventListener("resize",handleViewportResize)');
    expect(c).toContain("handleMinimapMouseMove");
    expect(c).toContain("handleMinimapMouseUp");
  });

  test("DiffViewer split-drag listeners are removed by stopSplitDrag (self-healing) and scroll listener on unmount", () => {
    const src = read("components/workspace/DiffViewer.vue");
    const stop = blockOf(src, "function stopSplitDrag()", "}");
    const c = compact(stop);
    expect(c).toContain('window.removeEventListener("mousemove",onSplitDrag)');
    expect(c).toContain('window.removeEventListener("mouseup",stopSplitDrag)');
    const unmount = blockOf(src, "onUnmounted(", "});");
    const u = compact(unmount);
    expect(u).toContain("scrollTarget.removeEventListener");
    expect(u).toContain('window.removeEventListener("keydown",handleDiffKeydown)');
  });
});

// ---------------------------------------------------------------------------
// 3. Composables — lifecycle-bound teardown.
// ---------------------------------------------------------------------------

describe("composable teardowns", () => {
  test("useModalA11y: capture-phase keydown is removed in stop() and stop runs onBeforeUnmount", () => {
    const src = read("composables/useModalA11y.ts");
    expect(compact(src)).toContain('document.addEventListener("keydown",onKeydown,true)');
    expect(compact(src)).toContain('document.removeEventListener("keydown",onKeydown,true)');
    expect(compact(src)).toContain("onBeforeUnmount(stop)");
  });

  test("useLiquidGlass: pointermove + rAF flushed and torn down on unmount", () => {
    const src = read("composables/useLiquidGlass.ts");
    const unmount = blockOf(src, "onUnmounted(", "});");
    const c = compact(unmount);
    expect(c).toContain('window.removeEventListener("pointermove",onPointerMove)');
    expect(c).toContain("cancelAnimationFrame(frame)");
  });

  test("useWindowChrome: drag/resize listeners, geometry debounce and rAF all released onBeforeUnmount", () => {
    const src = read("composables/useWindowChrome.ts");
    const unmount = blockOf(src, "onBeforeUnmount(", "});");
    const c = compact(unmount);
    expect(c).toContain('host.removeEventListener("pointerdown",onPointerDown)');
    expect(c).toContain('host.removeEventListener("dblclick",onDoubleClick)');
    expect(c).toContain('window.removeEventListener("pointermove",onPointerMove)');
    expect(c).toContain('window.removeEventListener("pointerup",finish)');
    expect(c).toContain('window.removeEventListener("pointercancel",finish)');
    expect(c).toContain('window.removeEventListener("resize",onResize)');
    expect(c).toContain("window.clearTimeout(resizeTimer)");
    expect(c).toContain("cancelAnimationFrame(frame)");
  });

  test("useToast: module-level timer map is cleared per toast and reset via resetToasts", () => {
    const src = read("composables/useToast.ts");
    const c = compact(src);
    // App-lifetime singleton store: each setTimeout lands in `timers` and is
    // cleared on dismiss; resetToasts() is the global fallback.
    expect(c).toContain("consttimers=newMap<");
    expect(countOf(c, "timers.set(")).toBe(2);
    expect(c).toContain("exportfunctionresetToasts()");
    expect(compact(blockOf(src, "export function resetToasts()", "}"))).toContain("clearTimeout(timer)");
  });
});

// ---------------------------------------------------------------------------
// 4. Observers / rAF / intervals in components.
// ---------------------------------------------------------------------------

describe("observer / rAF / interval teardowns", () => {
  test("TerminalTab: Wails subscription off, ResizeObserver disconnect, xterm dispose", () => {
    const unmount = blockOf(read("components/tools/TerminalTab.vue"), "onUnmounted(", "});");
    const c = compact(unmount);
    expect(c).toContain("offOutput?.()");
    expect(c).toContain("observer?.disconnect()");
    expect(c).toContain("term?.dispose()");
  });

  test("WorkspaceView: ResizeObserver disconnected, highlight timer cleared", () => {
    const unmount = blockOf(read("components/WorkspaceView.vue"), "onUnmounted(", "});");
    const c = compact(unmount);
    expect(c).toContain("workspaceResizeObserver?.disconnect()");
    expect(c).toContain("workspaceResizeObserver=null");
    expect(c).toContain("clearSourceLineHighlight()");
  });

  test("RepoRail: ResizeObserver disconnected and health debounce timer cleared", () => {
    const unmount = blockOf(read("components/RepoRail.vue"), "onUnmounted(", "});");
    const c = compact(unmount);
    expect(c).toContain("resizeObserver?.disconnect()");
    expect(c).toContain("clearTimeout(healthDebounceTimer)");
  });

  test("CommitComposer: capture pointerdown removed, ResizeObserver disconnected, debounce timers cleared", () => {
    const unmount = blockOf(read("components/workspace/CommitComposer.vue"), "onUnmounted(", "});");
    const c = compact(unmount);
    expect(c).toContain('document.removeEventListener("pointerdown",handleComposerPointerDown,true)');
    expect(c).toContain("resizeObserver?.disconnect()");
    expect(c).toContain("clearTimeout(lintTimer)");
    expect(c).toContain("clearTimeout(shakeTimer)");
    expect(c).toContain("clearTimeout(warningTimer)");
  });

  test("AudioDiffViewer: rAF progress loop cancelled on unmount", () => {
    const unmount = blockOf(read("components/assets/AudioDiffViewer.vue"), "onUnmounted(", "});");
    expect(compact(unmount)).toContain("cancelAnimationFrame(animFrameId)");
  });

  test("CommitTimeLapseModal: playback interval stopped via stopPlayback on unmount", () => {
    const src = read("components/history/CommitTimeLapseModal.vue");
    expect(compact(blockOf(src, "function stopPlayback()", "}"))).toContain("clearInterval(playTimer)");
    expect(compact(blockOf(src, "onUnmounted(", "});"))).toContain("stopPlayback()");
  });

  test("RepoDashboard: auto-refresh interval stopped and one-shot timers flushed on unmount", () => {
    const src = read("components/RepoDashboard.vue");
    expect(compact(blockOf(src, "function stopAutoRefreshTimer()", "}"))).toContain(
      "clearInterval(autoRefreshTimer)"
    );
    const unmount = blockOf(src, "onUnmounted(", "});");
    const c = compact(unmount);
    expect(c).toContain("stopAutoRefreshTimer()");
    expect(c).toContain("clearTimeout(toastTimer)");
    expect(c).toContain("for(consttimeroffetchReloadTimers)clearTimeout(timer)");
  });

  test("HistoryView: context-menu + keydown listeners removed and filter debounce cleared on unmount", () => {
    const unmount = blockOf(read("components/HistoryView.vue"), "onUnmounted(", "});");
    const c = compact(unmount);
    expect(c).toContain('window.removeEventListener("keydown",handleKeydown)');
    expect(c).toContain('window.removeEventListener("click",closeContextMenu)');
    expect(c).toContain("clearTimeout(filterTimer)");
  });

  test("HistoryView rebase-dialog poll interval is self-terminating (2s backstop)", () => {
    const src = read("components/HistoryView.vue");
    expect(compact(src)).toContain("setTimeout(()=>clearInterval(timer),2000)");
  });

  test("ToolsView section scroll retry loop is bounded (15 attempts max)", () => {
    const src = read("components/ToolsView.vue");
    expect(compact(src)).toContain("if(++attempts<15)setTimeout(tryScroll,120)");
  });
});

// ---------------------------------------------------------------------------
// 5. Services and global fallbacks.
// ---------------------------------------------------------------------------

describe("service / global teardowns", () => {
  test("aiStreamService: every Events.On lands in disposers and cleanup() pops them", () => {
    const src = read("services/aiStreamService.ts");
    expect(countOf(src, "disposers.push(")).toBe(3);
    const cleanup = blockOf(src, "const cleanup =", "};");
    const c = compact(cleanup);
    expect(c).toContain("disposers.pop()");
    expect(c).toContain("off?.()");
    expect(compact(src)).toContain("dispose:cleanup");
  });

  test("adaptivePolling: stop() clears the timer chain and removes document/window listeners", () => {
    const src = read("utils/adaptivePolling.ts");
    const stop = blockOf(src, "stop() {", "},");
    const c = compact(stop);
    expect(c).toContain("clearTimeout(timer)");
    expect(c).toContain('document.removeEventListener("visibilitychange",handleVisibilityChange)');
    expect(c).toContain('window.removeEventListener("online",handleOnline)');
  });

  test("crashReporter: setup returns a teardown and App.vue pairs it in onUnmounted", () => {
    const src = read("utils/crashReporter.ts");
    // Anchor after the registration calls: the one-line `return () => {}`
    // no-window early-exit must not match.
    const afterRegister = src.slice(src.indexOf('window.addEventListener("error", errorHandler)'));
    const teardown = compact(blockOf(afterRegister, "return () => {", "};"));
    expect(teardown).toContain('window.removeEventListener("error",errorHandler)');
    expect(teardown).toContain('window.removeEventListener("unhandledrejection",rejectionHandler)');
    const appUnmount = blockOf(read("App.vue"), "onUnmounted(", "});");
    expect(compact(appUnmount)).toContain("teardownCrashReporter()");
  });
});

// ---------------------------------------------------------------------------
// 6. Intentional app-lifetime singletons — exempt from component teardown,
//    but must carry an explanatory annotation.
// ---------------------------------------------------------------------------

describe("intentional app-lifetime singletons are annotated", () => {
  test("main.ts window error handlers are marked as app-lifetime singletons", () => {
    const src = read("main.ts");
    expect(compact(src)).toContain("app-lifetimesingleton");
    expect(countOf(src, "window.addEventListener(")).toBe(2);
    // main.ts runs once per process and never unmounts: no removal expected.
    expect(countOf(src, "removeEventListener(")).toBe(0);
  });

  test("useTheme matchMedia listener is marked as a guarded app-lifetime singleton", () => {
    const src = read("composables/useTheme.ts");
    expect(compact(src)).toContain("app-lifetimesingleton");
    expect(compact(src)).toContain("initialized");
  });

  test("ariaLabelFallback MutationObserver is marked as an app-lifetime singleton", () => {
    const src = read("composables/ariaLabelFallback.ts");
    expect(compact(src)).toContain("app-lifetimesingleton");
    expect(compact(src)).toContain("newMutationObserver(");
  });
});
