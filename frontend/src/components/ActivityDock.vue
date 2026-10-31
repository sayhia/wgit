<script lang="ts">
// 模块级导出区：与 <script setup> 并存的普通 <script> 块（SFC 规范支持具名导出）。
//
// 方案 §2.2 第 12 条 —— ActivityDock 显示后台任务（fetch/pull/push/lfs/
// maintenance/fsck/clean 等）状态；完成后用户不知道结果在哪看。这里定义：
//   1. ActivityTask        —— 后台任务的数据结构（id/类型/状态/时间/详情）；
//   2. TASK_TARGETS        —— 任务类型 → 目标视图事件 的映射表（导出常量，便于测试）；
//   3. ACTIVITY_TASK_EVENT —— 任务上报的 window CustomEvent 合同。
//
// 任务来源复用既有 window 事件总线（App.vue deep-link 同款分发模式）：
// 任意模块 `window.dispatchEvent(new CustomEvent("wgit:activity-task", { detail }))`
// 即可把任务登记进 dock，无需 import 本组件。
import { ref } from "vue";

/** 任务状态：运行中 / 已完成 / 已失败。 */
export type ActivityTaskStatus = "running" | "done" | "failed";

/** 后台任务的数据结构。 */
export interface ActivityTask {
  /** 幂等键：同一任务先报 running 再报 done/failed 时按 id 合并更新。 */
  id: string;
  /** 任务类型（kind），经 TASK_TARGETS 映射到目标视图。 */
  kind: string;
  status: ActivityTaskStatus;
  startedAt: number;
  finishedAt?: number;
  /** 结果 / 错误详情。失败任务在无对应视图时经现有 toast 展示。 */
  detail?: string;
}

/** 一个映射目标：跳转用的既有 window 事件 + 任务展示标签（既有 i18n key）。 */
export interface ActivityTaskTarget {
  /** 任务名标签的 i18n key（复用既有 key，不新增）。 */
  labelKey: string;
  /** 目标事件名（必须在 src/commands/registry.ts 中已注册，防拼写漂移）。 */
  eventName: string;
  /** 事件 detail（tab/section 取值与注册表中的 deep-link 完全一致）。 */
  detail?: Record<string, unknown>;
}

/** 任务上报事件名：detail 为 ActivityTask 的形参（id/kind/status 必填）。 */
export const ACTIVITY_TASK_EVENT = "wgit:activity-task";

/**
 * 任务类型 → 目标视图事件映射表。
 *
 * - fetch/pull/push：同步结果去分支对比页看（领先/落后与待同步提交）；
 * - lfs：LFS 子页；maintenance：健康报告区；fsck：数据完整性（fsck）区；
 * - clean：工作区清理子页。
 * - 不在此表中的类型不渲染完成态点击态（失败态走 toast 展示详情）。
 * 每条目标的事件名 + detail 都能在 src/commands/registry.ts 的既有 deep-link
 * 命令里找到同款（tests/activityDockTargets.test.ts 把关）。
 */
export const TASK_TARGETS: Record<string, ActivityTaskTarget> = {
  fetch: { labelKey: "header.sync.fetch", eventName: "wgit:branches-tab", detail: { tab: "compare" } },
  pull: { labelKey: "header.sync.pull", eventName: "wgit:branches-tab", detail: { tab: "compare" } },
  push: { labelKey: "header.sync.push", eventName: "wgit:branches-tab", detail: { tab: "compare" } },
  lfs: { labelKey: "tools.tab.lfs", eventName: "wgit:tools-tab", detail: { tab: "lfs" } },
  maintenance: { labelKey: "tools.tab.maintenance", eventName: "wgit:tools-tab", detail: { tab: "maintenance", section: "health" } },
  fsck: { labelKey: "cmd.tools-fsck", eventName: "wgit:tools-tab", detail: { tab: "maintenance", section: "fsck" } },
  clean: { labelKey: "tools.tab.clean", eventName: "wgit:tools-tab", detail: { tab: "clean" } },
};

/** dock 只保留最近几条任务，防刷屏。 */
export const MAX_TRACKED_TASKS = 4;

/** 终态任务（done/failed）在 dock 上保留的时长，之后自动移除（类似 toast）。 */
export const FINISHED_TASK_TTL_MS = 8000;

// 模块级任务仓：dock 全程只挂载一份，状态放模块作用域（同 useToast 模式）。
export const activityTasks = ref<ActivityTask[]>([]);
const taskRemovalTimers = new Map<string, ReturnType<typeof setTimeout>>();

function normalizeTaskStatus(status: unknown): ActivityTaskStatus {
  return status === "done" || status === "failed" ? status : "running";
}

/** 终态任务到达 TTL 后自动从 dock 移除。 */
function scheduleTaskRemoval(task: ActivityTask): void {
  const prev = taskRemovalTimers.get(task.id);
  if (prev) {
    clearTimeout(prev);
    taskRemovalTimers.delete(task.id);
  }
  if (task.status === "running") return;
  taskRemovalTimers.set(
    task.id,
    setTimeout(() => {
      taskRemovalTimers.delete(task.id);
      activityTasks.value = activityTasks.value.filter((entry) => entry.id !== task.id);
    }, FINISHED_TASK_TTL_MS)
  );
}

/**
 * 登记 / 更新一条后台任务（也可由测试直接调用）。
 * 同 id 再次上报视为同一任务的进度推进（running → done/failed）。
 */
export function trackActivityTask(task: Partial<ActivityTask> & Pick<ActivityTask, "id" | "kind">): void {
  const now = Date.now();
  const normalized: ActivityTask = {
    detail: undefined,
    ...task,
    status: normalizeTaskStatus(task.status),
    startedAt: task.startedAt ?? now,
  };
  if (normalized.status === "running") {
    normalized.finishedAt = undefined;
    normalized.detail = undefined;
  } else {
    normalized.finishedAt = task.finishedAt ?? now;
  }

  const existingIndex = activityTasks.value.findIndex((entry) => entry.id === normalized.id);
  const existing = existingIndex >= 0 ? activityTasks.value[existingIndex] : undefined;
  if (existing) {
    // 保留首次上报的 startedAt，其余字段以最新上报为准。
    const merged: ActivityTask = { ...existing, ...normalized };
    merged.startedAt = existing.startedAt;
    activityTasks.value.splice(existingIndex, 1);
    activityTasks.value = [merged, ...activityTasks.value].slice(0, MAX_TRACKED_TASKS);
  } else {
    activityTasks.value = [normalized, ...activityTasks.value].slice(0, MAX_TRACKED_TASKS);
  }
  scheduleTaskRemoval(normalized);
}

/** 测试接缝：清空任务与挂起的移除定时器。 */
export function resetActivityTasks(): void {
  for (const timer of taskRemovalTimers.values()) clearTimeout(timer);
  taskRemovalTimers.clear();
  activityTasks.value = [];
}
</script>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from "vue";
import { t } from "../i18n";
import { pushToast } from "../composables/useToast";
import { Loader2, CheckCircle2, AlertTriangle, FileDiff, GitGraph, GitBranch, Archive, Wrench, Settings, ClipboardCheck } from "lucide-vue-next";

defineProps<{
  activeTab: string;
  hasDirtyFiles?: boolean;
}>();

const emit = defineEmits<{
  (e: "select-tab", tab: string): void;
}>();

const tabs = computed(() => [
  { id: "workspace", label: t("nav.workspace"), icon: FileDiff },
  { id: "history", label: t("nav.history"), icon: GitGraph },
  { id: "branches", label: t("nav.branches"), icon: GitBranch },
  { id: "stashes", label: t("nav.stashes"), icon: Archive },
  { id: "review", label: t("nav.review"), icon: ClipboardCheck },
  { id: "tools", label: t("nav.tools"), icon: Wrench },
]);

function normalizeEventTask(detail: unknown): ActivityTask | null {
  if (!detail || typeof detail !== "object") return null;
  const raw = detail as Partial<ActivityTask>;
  if (typeof raw.id !== "string" || !raw.id) return null;
  if (typeof raw.kind !== "string" || !raw.kind) return null;
  return {
    id: raw.id,
    kind: raw.kind,
    status: normalizeTaskStatus(raw.status),
    startedAt: typeof raw.startedAt === "number" ? raw.startedAt : Date.now(),
    finishedAt: typeof raw.finishedAt === "number" ? raw.finishedAt : undefined,
    detail: typeof raw.detail === "string" ? raw.detail : undefined,
  };
}

function handleActivityTaskEvent(event: Event) {
  const task = normalizeEventTask((event as CustomEvent).detail);
  if (task) trackActivityTask(task);
}

onMounted(() => {
  window.addEventListener(ACTIVITY_TASK_EVENT, handleActivityTaskEvent);
});

onUnmounted(() => {
  window.removeEventListener(ACTIVITY_TASK_EVENT, handleActivityTaskEvent);
  for (const timer of taskRemovalTimers.values()) clearTimeout(timer);
  taskRemovalTimers.clear();
});

/** 完成态：有映射才可点击；失败态：一律可点击（无映射时点击用 toast 展示详情）。 */
function isTaskClickable(task: ActivityTask): boolean {
  return task.status === "failed" || TASK_TARGETS[task.kind] !== undefined;
}

function taskStatusClass(task: ActivityTask): string {
  if (task.status === "failed") return "text-danger";
  if (task.status === "done") return "text-success";
  return "text-accent";
}

function taskIcon(task: ActivityTask) {
  if (task.status === "failed") return AlertTriangle;
  if (task.status === "done") return CheckCircle2;
  return Loader2;
}

/** aria-label / title 只复用既有 i18n key：任务名标签 + 既有状态词。 */
function taskAriaLabel(task: ActivityTask): string {
  const target = TASK_TARGETS[task.kind];
  const kindLabel = target ? t(target.labelKey) : task.kind;
  const statusKey = task.status === "failed" ? "ops.queue.failed" : task.status === "running" ? "common.loading" : "ops.queue.done";
  return `${kindLabel} · ${t(statusKey)}`;
}

/**
 * 点击任务：有映射 → 派发既有 deep-link 事件（App.vue forwardAfterMount /
 * ToolsView.handleToolsTabEvent / BranchesView 消费），跳到对应视图并高亮结果；
 * 失败且无映射 → 用现有 toast 展示 detail（本组件无错误展开区，复用全局 toast）。
 */
function handleTaskClick(task: ActivityTask): void {
  const target = TASK_TARGETS[task.kind];
  if (target) {
    window.dispatchEvent(new CustomEvent(target.eventName, { detail: target.detail }));
    return;
  }
  if (task.status === "failed") {
    pushToast(task.detail ? `${t("common.error")}: ${task.detail}` : t("common.error"), {
      tone: "danger",
      duration: 6000,
    });
  }
}
</script>

<template>
  <aside data-glass class="w-13 border-r border-line glass glass-bar flex flex-col items-center justify-between py-3 shrink-0 z-20 select-none">
    <!-- Top Main Navigation -->
    <div class="flex flex-col items-center gap-2">
      <!-- App Brand Mark -->
      <img
        src="/brand/wgit-icon.svg"
        alt="wgit"
        width="32"
        height="32"
        :draggable="false"
        class="w-8 h-8 mb-2 shrink-0 select-none"
      />

      <!-- Navigation Icons -->
      <button
        v-for="tab in tabs"
        :key="tab.id"
        @click="emit('select-tab', tab.id)"
        class="relative w-9 h-9 rounded-xl flex items-center justify-center transition duration-150 group cursor-pointer"
        :class="
          activeTab === tab.id
            ? 'bg-[var(--glass-raise-strong)] text-accent shadow-sm'
            : 'text-ink-muted hover:text-ink hover:bg-[var(--glass-raise)]'
        "
        :title="tab.label"
        :aria-label="tab.label"
        :aria-current="activeTab === tab.id ? 'page' : undefined"
      >
        <component :is="tab.icon" class="w-4.5 h-4.5 transition-transform duration-150 group-hover:scale-105" />
        
        <!-- Active indicator pill on left -->
        <span
          v-if="activeTab === tab.id"
          class="absolute -left-1.5 w-1 h-4.5 rounded-r bg-accent-strong"
        ></span>

        <!-- Dirty files badge indicator -->
        <span
          v-if="tab.id === 'workspace' && hasDirtyFiles"
          class="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-warning-strong border-2 border-surface"
        ></span>
      </button>
    </div>

    <!-- Bottom Settings & Utilities -->
    <div class="flex flex-col items-center gap-1.5">
      <!-- Background tasks: running = spinner, done/failed = clickable result -->
      <template v-if="activityTasks.length > 0">
        <div data-testid="activity-dock-tasks" class="flex flex-col items-center gap-1.5">
          <template v-for="task in activityTasks" :key="task.id">
            <button
              v-if="isTaskClickable(task)"
              type="button"
              class="w-9 h-9 rounded-xl flex items-center justify-center transition duration-150 cursor-pointer hover:bg-[var(--glass-raise)]"
              :class="taskStatusClass(task)"
              :title="taskAriaLabel(task)"
              :aria-label="taskAriaLabel(task)"
              @click="handleTaskClick(task)"
            >
              <component
                :is="taskIcon(task)"
                class="w-4 h-4"
                :class="{ 'animate-spin': task.status === 'running' }"
              />
            </button>
            <!-- 完成态无映射（或运行中）：仅状态展示，不渲染点击态 -->
            <div
              v-else
              role="status"
              class="w-9 h-9 rounded-xl flex items-center justify-center"
              :class="taskStatusClass(task)"
              :title="taskAriaLabel(task)"
              :aria-label="taskAriaLabel(task)"
            >
              <component
                :is="taskIcon(task)"
                class="w-4 h-4"
                :class="{ 'animate-spin': task.status === 'running' }"
              />
            </div>
          </template>
        </div>
        <div class="w-6 h-px bg-line" aria-hidden="true"></div>
      </template>

      <button
        @click="emit('select-tab', 'settings')"
        class="w-9 h-9 rounded-xl flex items-center justify-center text-ink-muted hover:text-ink hover:bg-raised/50 transition cursor-pointer"
        :class="{ 'bg-raised text-accent': activeTab === 'settings' }"
        :title="t('nav.settings')"
        :aria-label="t('nav.settings')"
        :aria-current="activeTab === 'settings' ? 'page' : undefined"
      >
        <Settings class="w-4 h-4" />
      </button>
    </div>
  </aside>
</template>
