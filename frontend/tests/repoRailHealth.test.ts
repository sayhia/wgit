import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const railFile = fileURLToPath(new URL("../src/components/RepoRail.vue", import.meta.url));
const src = readFileSync(railFile, "utf8");

function scriptOf(source: string): string {
  const start = source.indexOf("<script setup");
  const end = source.indexOf("</script>");
  return source.slice(start, end);
}

function blockOf(source: string, startMarker: string, endMarker: string): string {
  const start = source.indexOf(startMarker);
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf(endMarker, start);
  return source.slice(start, end === -1 ? undefined : end);
}

describe("RepoRail repo health badges", () => {
  const script = scriptOf(src);

  test("renders an 8px round health dot on each repo card", () => {
    expect(src).toContain('data-testid="repo-health-badge"');
    // Docked onto the tab's own icon corner: a corner-pinned dot used to sit on
    // the close button (top-right) or float outside the tab on the rail border.
    expect(src).toContain("repo-health-badge absolute left-2 top-0.5 z-10 w-2 h-2 rounded-full");
  });

  test("dot color maps to health level (green / yellow / red)", () => {
    expect(src).toContain('"bg-success"');
    expect(src).toContain('"bg-warning-strong"');
    expect(src).toContain('"bg-danger"');
    expect(script).toContain('report.failCount > 0 || report.score < 60) return "danger"');
    expect(script).toContain('report.warnCount > 0 || report.score < 90) return "warning"');
    expect(script).toContain('return "success"');
  });

  test("badges are lazy: cards render first, probes run in a background queue", () => {
    // The dot only appears once a report lands in the reactive map.
    expect(src).toContain('v-if="healthBadges[repo]"');
    // The health RPC is only ever called from the queue drain, never on mount.
    const mountedBlock = blockOf(script, "onMounted(", "onUnmounted(");
    expect(mountedBlock).toContain("scheduleHealthRefresh()");
    expect(mountedBlock).not.toContain("runRepoHealthCheck");
    const drainBlock = blockOf(script, "async function drainHealthQueue", "function openRepoHealth");
    expect(drainBlock).toContain("await gitService.runRepoHealthCheck(repo)");
    // One repo at a time: the drain guards against concurrent runs.
    expect(drainBlock).toContain("if (healthDraining) return");
  });

  test("health refresh is debounced, not fired on every rail render", () => {
    expect(script).toContain("HEALTH_REFRESH_DEBOUNCE_MS");
    const scheduleBlock = blockOf(script, "function scheduleHealthRefresh", "async function drainHealthQueue");
    expect(scheduleBlock).toContain("setTimeout(");
    expect(scheduleBlock).toContain("clearTimeout(");
  });

  test("health reports are cached per repoPath in a component Map", () => {
    expect(script).toContain("new Map<string, RepoHealthBadge>()");
    const scheduleBlock = blockOf(script, "function scheduleHealthRefresh", "async function drainHealthQueue");
    // Cached repos never re-enqueue on rail refresh.
    expect(scheduleBlock).toContain("healthCache.has(repo) || healthQueued.has(repo)");
    const drainBlock = blockOf(script, "async function drainHealthQueue", "function openRepoHealth");
    expect(drainBlock).toContain("healthCache.set(repo, badge)");
  });

  test("probe failures stay silent: no badge and no toast", () => {
    const drainBlock = blockOf(script, "async function drainHealthQueue", "function openRepoHealth");
    expect(drainBlock).toContain("} catch {");
    expect(drainBlock).not.toContain('emit("notify"');
  });

  test("clicking the badge selects the repo and opens maintenance health section", () => {
    expect(src).toContain('@click.stop="openRepoHealth(repo)"');
    expect(script).toContain(
      'window.dispatchEvent(new CustomEvent("wgit:tools-tab", { detail: { tab: "maintenance", section: "health" } }));'
    );
    expect(script).toContain('emit("select", repo)');
  });

  test("tooltip reuses existing health.summary i18n keys, no new catalog entries", () => {
    expect(script).toContain("backendText(");
    expect(script).toContain('"health.summary.mixed"');
    expect(script).toContain('"health.summary.ok"');
    // The reused keys must already exist in both catalogs.
    expect("health.summary.ok" in zhCN).toBe(true);
    expect("health.summary.mixed" in zhCN).toBe(true);
    expect("health.summary.ok" in enUS).toBe(true);
    expect("health.summary.mixed" in enUS).toBe(true);
  });

  test("background probing is torn down with the component", () => {
    const unmountBlock = blockOf(script, "onUnmounted(", "</script>");
    expect(unmountBlock).toContain("healthDisposed = true");
    expect(unmountBlock).toContain("clearTimeout(healthDebounceTimer)");
    expect(unmountBlock).toContain("healthQueue = []");
  });
});
