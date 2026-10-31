import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { ACTIVITY_TASK_EVENT, TASK_TARGETS } from "../src/components/ActivityDock.vue";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";

const dockFile = fileURLToPath(new URL("../src/components/ActivityDock.vue", import.meta.url));
const src = readFileSync(dockFile, "utf8");
const registryFile = fileURLToPath(new URL("../src/commands/registry.ts", import.meta.url));
const registrySrc = readFileSync(registryFile, "utf8");
const appFile = fileURLToPath(new URL("../src/App.vue", import.meta.url));
const appSrc = readFileSync(appFile, "utf8");

function scriptOf(source: string): string {
  // 模块级 <script> 块在前、<script setup> 在后（注释里也可能出现该字样），
  // 取最后一个 script 标签才是 setup 块。
  const start = source.lastIndexOf("<script");
  const end = source.indexOf("</script>", start);
  return source.slice(start, end);
}

function blockOf(source: string, startMarker: string, endMarker: string): string {
  const start = source.indexOf(startMarker);
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf(endMarker, start);
  return source.slice(start, end === -1 ? undefined : end);
}

/** 去空白归一化，避免格式差异影响源码断言。 */
const compact = (s: string) => s.replace(/\s+/g, "");

/** 把 detail 对象还原成注册表里的字面量形式（键不带引号）。 */
function compactDetail(detail?: Record<string, unknown>): string {
  if (!detail) return "";
  const body = Object.entries(detail)
    .map(([key, value]) => `${key}:${JSON.stringify(value)}`)
    .join(",");
  return `{${body}}`;
}

const dict = (m: object) => m as Record<string, string>;

describe("ActivityDock task targets", () => {
  const setup = scriptOf(src);

  test("TASK_TARGETS covers the background task kinds from the plan", () => {
    for (const kind of ["fetch", "pull", "push", "lfs", "maintenance", "fsck", "clean"]) {
      const target = TASK_TARGETS[kind];
      expect(target).toBeDefined();
      expect(typeof target.labelKey).toBe("string");
      expect(typeof target.eventName).toBe("string");
    }
  });

  test("every target event name + detail is registered in src/commands/registry.ts (no spelling drift)", () => {
    const registryCompact = compact(registrySrc);
    for (const target of Object.values(TASK_TARGETS)) {
      const literal = `event("${target.eventName}"${target.detail ? `, ${compactDetail(target.detail)}` : ""})`;
      expect(registryCompact).toContain(compact(literal));
    }
  });

  test("target events are consumed by App.vue deep-link listeners", () => {
    const eventNames = new Set(Object.values(TASK_TARGETS).map((target) => target.eventName));
    expect(eventNames).toEqual(new Set(["wgit:branches-tab", "wgit:tools-tab"]));
    for (const eventName of eventNames) {
      expect(appSrc).toContain(`addEventListener("${eventName}"`);
    }
  });

  test("tools-tab targets only use tab/section values ToolsView actually handles", () => {
    // ToolsView.handleToolsTabEvent 的 tab 取值必须命中 TOOLS_SUB_TABS。
    const toolsViewFile = fileURLToPath(new URL("../src/components/ToolsView.vue", import.meta.url));
    const toolsViewSrc = readFileSync(toolsViewFile, "utf8");
    for (const target of Object.values(TASK_TARGETS)) {
      if (target.eventName !== "wgit:tools-tab" || !target.detail) continue;
      const tab = target.detail.tab;
      expect(typeof tab).toBe("string");
      expect(toolsViewSrc).toContain(`"${tab}"`);
    }
  });

  test("aria-label only reuses existing i18n keys (both locales)", () => {
    const statusKeys = ["ops.queue.done", "ops.queue.failed", "common.loading", "common.error"];
    for (const key of [...Object.values(TASK_TARGETS).map((target) => target.labelKey), ...statusKeys]) {
      expect(typeof dict(zhCN)[key]).toBe("string");
      expect(dict(zhCN)[key].length).toBeGreaterThan(0);
      expect(typeof dict(enUS)[key]).toBe("string");
      expect(dict(enUS)[key].length).toBeGreaterThan(0);
    }
  });

  test("clickable completed tasks dispatch the mapped deep-link event", () => {
    const clickBlock = blockOf(setup, "function handleTaskClick", "</script>");
    expect(clickBlock).toContain("const target = TASK_TARGETS[task.kind]");
    expect(clickBlock).toContain('window.dispatchEvent(new CustomEvent(target.eventName, { detail: target.detail }))');
  });

  test("failed tasks without a mapped view fall back to the existing toast", () => {
    const clickBlock = blockOf(setup, "function handleTaskClick", "</script>");
    expect(clickBlock).toContain('task.status === "failed"');
    expect(clickBlock).toContain("pushToast(");
    expect(clickBlock).toContain("task.detail");
  });

  test("only mapped completed tasks and failed tasks render a click state", () => {
    expect(setup).toContain('function isTaskClickable(task: ActivityTask): boolean {');
    expect(setup).toContain('return task.status === "failed" || TASK_TARGETS[task.kind] !== undefined');
    // 可点击分支：button + cursor-pointer + hover 高亮 + aria-label
    const clickableBlock = blockOf(src, 'v-if="isTaskClickable(task)"', "</template>");
    expect(clickableBlock).toContain("cursor-pointer");
    expect(clickableBlock).toContain("hover:bg-raised");
    expect(clickableBlock).toContain('@click="handleTaskClick(task)"');
    expect(clickableBlock).toContain(':aria-label="taskAriaLabel(task)"');
    // 无映射完成态 / 运行中：仅状态展示，不渲染点击态
    const plainBlock = blockOf(src, 'v-else', "</template>");
    expect(plainBlock).toContain('role="status"');
    expect(plainBlock).not.toContain("@click");
  });

  test("task status styling maps to success / danger / accent", () => {
    expect(setup).toContain('if (task.status === "failed") return "text-danger"');
    expect(setup).toContain('if (task.status === "done") return "text-success"');
    expect(setup).toContain('return "text-accent"');
  });

  test("task feed listens on the window event bus and tears down with the component", () => {
    expect(src).toContain('export const ACTIVITY_TASK_EVENT = "wgit:activity-task"');
    const mountedBlock = blockOf(setup, "onMounted(", "</script>");
    expect(mountedBlock).toContain("window.addEventListener(ACTIVITY_TASK_EVENT, handleActivityTaskEvent)");
    const unmountedBlock = blockOf(setup, "onUnmounted(", "</script>");
    expect(unmountedBlock).toContain("window.removeEventListener(ACTIVITY_TASK_EVENT, handleActivityTaskEvent)");
    expect(unmountedBlock).toContain("clearTimeout(");
  });

  test("finished tasks are capped and auto-expire like toasts", () => {
    expect(src).toContain("export const MAX_TRACKED_TASKS = 4");
    expect(src).toContain("export const FINISHED_TASK_TTL_MS = 8000");
    const trackBlock = blockOf(src, "export function trackActivityTask", "</script>");
    expect(trackBlock).toContain(".slice(0, MAX_TRACKED_TASKS)");
    expect(trackBlock).toContain("scheduleTaskRemoval(normalized)");
  });
});
