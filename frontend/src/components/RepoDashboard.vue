<script setup lang="ts">
import { t } from "../i18n";
import { ref, reactive, computed, watch, nextTick, onMounted, onUnmounted } from "vue";
import {
  FolderGit2,
  FolderOpen,
  GitBranch,
  ArrowUpCircle,
  ArrowDownCircle,
  Download,
  RefreshCw,
  Search,
  X,
  Loader2,
  AlertCircle,
  AlertTriangle,
  CheckCircle2,
  Terminal,
} from "lucide-vue-next";
import UiModal from "./ui/UiModal.vue";
import { gitService } from "../services/gitService";
import type { RepoStatus, RepoOverviewStats } from "../types";
import { useRovingList } from "../composables/useRovingList";

const props = defineProps<{
  repos: string[];
  activeRepo: string | null;
  open: boolean;
}>();

const emit = defineEmits<{
  (e: "opened", path: string): void;
  (e: "closed", path: string): void;
  (e: "fetch", path: string): void;
  (e: "fetch-all"): void;
  (e: "close-overlay"): void;
}>();

/** 并发上限：大型工作区同时打开多个仓库时，避免一次性打出过多后端命令。 */
const MAX_CONCURRENT_STATUS_LOADS = 4;
/** 打开总览时复用最近一次读取结果的时长（毫秒）。 */
const STATUS_CACHE_TTL_MS = 20000;
/** 自动刷新间隔。 */
const AUTO_REFRESH_INTERVAL_MS = 30000;

interface RepoEntry {
  status: RepoStatus | null;
  loading: boolean;
  error: string;
  loadedAt: number;
}

interface RepoMetrics {
  staged: number;
  unstaged: number;
  untracked: number;
  conflicted: number;
  changes: number;
  dirty: boolean;
}

// ---------------------------------------------------------------------------
// 状态读取（带并发限制的任务队列）
// ---------------------------------------------------------------------------
const entries = reactive<Record<string, RepoEntry>>({});
const statusQueue: string[] = [];
const queuedPaths = new Set<string>();
const loadingPaths = new Set<string>();
let activeLoads = 0;
let isDisposed = false;

const overview = ref<RepoOverviewStats | null>(null);
const isLoadingOverview = ref(false);

async function loadOverview() {
  if (!props.activeRepo) {
    overview.value = null;
    return;
  }
  isLoadingOverview.value = true;
  try {
    overview.value = await gitService.getRepoOverview(props.activeRepo);
  } catch {
    overview.value = null;
  } finally {
    isLoadingOverview.value = false;
  }
}

watch(
  () => [props.open, props.activeRepo] as const,
  ([isOpen]) => {
    if (isOpen) void loadOverview();
    else overview.value = null;
  },
  { immediate: true }
);

const searchQuery = ref("");
const autoRefresh = ref(false);
const toastText = ref("");
const searchInputRef = ref<HTMLInputElement | null>(null);

let toastTimer: ReturnType<typeof setTimeout> | null = null;
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;
// Fetch 由父组件执行；稍后延迟重读状态，让 ahead/behind 与脏数据跟上最新结果。
let fetchReloadTimers: ReturnType<typeof setTimeout>[] = [];

function ensureEntry(path: string): RepoEntry {
  if (!entries[path]) {
    entries[path] = { status: null, loading: false, error: "", loadedAt: 0 };
  }
  return entries[path]!;
}

function formatError(err: unknown): string {
  if (typeof err === "string" && err.trim()) return err.trim();
  if (err && typeof err === "object" && "message" in err) {
    const message = (err as { message?: unknown }).message;
    if (typeof message === "string" && message.trim()) return message.trim();
  }
  return t("dash.error.read.status");
}

function ensureRepoEntryLoaded(path: string, force = false) {
  if (!path) return;
  if (queuedPaths.has(path) || loadingPaths.has(path)) return;
  const entry = ensureEntry(path);
  if (!force && entry.status && Date.now() - entry.loadedAt < STATUS_CACHE_TTL_MS) return;
  queuedPaths.add(path);
  entry.loading = true;
  entry.error = "";
  statusQueue.push(path);
  pumpStatusQueue();
}

function pumpStatusQueue() {
  if (isDisposed) return;
  while (activeLoads < MAX_CONCURRENT_STATUS_LOADS && statusQueue.length > 0) {
    const path = statusQueue.shift();
    if (!path) continue;
    queuedPaths.delete(path);
    loadingPaths.add(path);
    activeLoads += 1;
    void runRepoStatusLoad(path);
  }
}

async function runRepoStatusLoad(path: string) {
  try {
    const status = await gitService.getRepoStatus(path);
    const entry = ensureEntry(path);
    if (status) {
      entry.status = status;
      entry.loadedAt = Date.now();
      entry.error = "";
    } else {
      entry.error = t("dash.error.no.status");
    }
  } catch (err: unknown) {
    ensureEntry(path).error = formatError(err);
  } finally {
    const entry = ensureEntry(path);
    entry.loading = false;
    loadingPaths.delete(path);
    activeLoads = Math.max(0, activeLoads - 1);
    pumpStatusQueue();
  }
}

function syncRepoEntries() {
  for (const path of props.repos) ensureEntry(path);
}

function loadRepoStatuses(force = false) {
  for (const path of props.repos) ensureRepoEntryLoaded(path, force);
}

function refreshAllStatuses(showFeedback = false) {
  if (props.repos.length === 0) return;
  loadRepoStatuses(true);
  if (showFeedback) showToast(t("dash.toast.refreshing"));
}

function retryRepo(path: string) {
  ensureEntry(path).error = "";
  ensureRepoEntryLoaded(path, true);
}

function scheduleStatusReload(paths: string[], delayMs = 1600) {
  if (paths.length === 0) return;
  const timer = setTimeout(() => {
    fetchReloadTimers = fetchReloadTimers.filter((item) => item !== timer);
    if (isDisposed) return;
    for (const path of paths) ensureRepoEntryLoaded(path, true);
  }, delayMs);
  fetchReloadTimers.push(timer);
}

// ---------------------------------------------------------------------------
// Toast 反馈
// ---------------------------------------------------------------------------
function showToast(message: string) {
  toastText.value = message;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toastText.value = "";
    toastTimer = null;
  }, 2200);
}

// ---------------------------------------------------------------------------
// 自动刷新
// ---------------------------------------------------------------------------
function stopAutoRefreshTimer() {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer);
    autoRefreshTimer = null;
  }
}

function syncAutoRefreshTimer() {
  stopAutoRefreshTimer();
  if (props.open && autoRefresh.value) {
    autoRefreshTimer = setInterval(() => {
      if (document.visibilityState === "visible") loadRepoStatuses(true);
    }, AUTO_REFRESH_INTERVAL_MS);
  }
}

function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value;
  syncAutoRefreshTimer();
  showToast(autoRefresh.value ? t("dash.toast.auto.on") : t("dash.toast.auto.off"));
}

// ---------------------------------------------------------------------------
// 派生数据：过滤 / 排序 / 聚合指标
// ---------------------------------------------------------------------------
function repoName(path: string): string {
  const cleaned = (path || "").replace(/[\\/]+$/, "");
  if (!cleaned) return path || t("dash.unnamed");
  const parts = cleaned.split(/[\\/]/);
  return parts[parts.length - 1] || cleaned;
}

function metricsFor(status: RepoStatus | null | undefined): RepoMetrics {
  if (!status) {
    return { staged: 0, unstaged: 0, untracked: 0, conflicted: 0, changes: 0, dirty: false };
  }
  const staged = status.stagedFiles?.length ?? 0;
  const unstaged = status.unstagedFiles?.length ?? 0;
  const untracked = status.untrackedFiles?.length ?? 0;
  const conflicted = status.conflictedFiles?.length ?? 0;
  const changes = staged + unstaged + untracked + conflicted;
  const dirty =
    !status.isClean ||
    changes > 0 ||
    Boolean(status.isMerging) ||
    Boolean(status.isRebasing) ||
    Boolean(status.isCherryPicking);
  return { staged, unstaged, untracked, conflicted, changes, dirty };
}

function metricsOfRepo(path: string): RepoMetrics {
  return metricsFor(entries[path]?.status);
}

const normalizedQuery = computed(() => searchQuery.value.trim().toLowerCase());

const filteredRepoPaths = computed(() => {
  const query = normalizedQuery.value;
  if (!query) return props.repos;
  return props.repos.filter(
    (path) => repoName(path).toLowerCase().includes(query) || path.toLowerCase().includes(query)
  );
});

const sortedRepoPaths = computed(() => {
  const activePath = props.activeRepo;
  return filteredRepoPaths.value.slice().sort((a, b) => {
    // 1. 当前仓库置顶
    if (a === activePath && b !== activePath) return -1;
    if (b === activePath && a !== activePath) return 1;
    // 2. 有未提交改动的仓库靠前
    const aDirty = metricsOfRepo(a).dirty;
    const bDirty = metricsOfRepo(b).dirty;
    if (aDirty !== bDirty) return aDirty ? -1 : 1;
    // 3. 名称排序
    return repoName(a).localeCompare(repoName(b), "zh-CN");
  });
});

const visibleRepoCount = computed(() => sortedRepoPaths.value.length);

const repoRows = computed(() =>
  sortedRepoPaths.value.map((path) => ({
    path,
    name: repoName(path),
    entry: entries[path],
    metrics: metricsOfRepo(path),
    isActive: path === props.activeRepo,
  }))
);

// Keyboard parity with the branch/stash lists: each card carries its own
// action bar (fetch / terminal / close), so one card is tabbable at a time,
// ↑/↓ moves between cards and Enter performs the card's primary action
// (open the repo). Same reasoning as useRovingList's doc comment.
const {
  setItemRef: setCardRef,
  tabindexFor: cardTabindexFor,
  onKeydown: onCardKeydown,
  onFocus: onCardFocus,
} = useRovingList(() => repoRows.value, (row) => row.path);

const aggregate = computed(() => {
  let dirtyRepos = 0;
  let totalChanges = 0;
  let totalAhead = 0;
  let totalBehind = 0;
  let loadedCount = 0;
  let loadingCount = 0;
  let errorCount = 0;

  for (const path of props.repos) {
    const entry = entries[path];
    if (!entry) continue;
    if (entry.loading) loadingCount += 1;
    if (entry.error) errorCount += 1;
    if (!entry.status) continue;
    loadedCount += 1;
    const metrics = metricsFor(entry.status);
    if (metrics.dirty) dirtyRepos += 1;
    totalChanges += metrics.changes;
    totalAhead += Math.max(0, entry.status.ahead || 0);
    totalBehind += Math.max(0, entry.status.behind || 0);
  }

  return {
    repoCount: props.repos.length,
    dirtyRepos,
    totalChanges,
    totalAhead,
    totalBehind,
    loadedCount,
    loadingCount,
    errorCount,
  };
});

// ---------------------------------------------------------------------------
// 时间格式化
// ---------------------------------------------------------------------------
function formatLoadedTime(timestamp: number): string {
  if (!timestamp) return "";
  const date = new Date(timestamp);
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function formatLoadedTitle(timestamp: number): string {
  if (!timestamp) return "";
  return new Date(timestamp).toLocaleString("zh-CN");
}

// ---------------------------------------------------------------------------
// 交互
// ---------------------------------------------------------------------------
function focusSearchInput() {
  void nextTick(() => searchInputRef.value?.focus());
}

function handleOpenRepo(path: string) {
  emit("opened", path);
}

function handleFetchRepo(path: string) {
  emit("fetch", path);
  showToast(t("dash.toast.fetching", { name: repoName(path) }));
  scheduleStatusReload([path]);
}

function handleFetchAll() {
  if (props.repos.length === 0) return;
  emit("fetch-all");
  showToast(t("dash.toast.fetch.all"));
  scheduleStatusReload(props.repos.slice(), 1800);
}

function handleCloseRepo(path: string) {
  emit("closed", path);
  showToast(t("dash.toast.closed", { name: repoName(path) }));
}

async function handleOpenTerminal(path: string) {
  if (!path) return;
  try {
    await gitService.openInTerminal(path);
    showToast(t("dash.toast.terminal.opened", { name: repoName(path) }));
  } catch (err: unknown) {
    showToast(t("dash.toast.terminal.failed", { error: formatError(err) }));
  }
}

async function handleOpenLocalRepo() {
  try {
    const info = await gitService.openRepoDialog();
    if (info?.path) {
      emit("opened", info.path);
    }
  } catch (err: unknown) {
    showToast(t("dash.toast.open.failed", { error: formatError(err) }));
  }
}

function handleGlobalKeydown(event: KeyboardEvent) {
  if (!props.open) return;
  if (event.key === "Escape") {
    event.preventDefault();
    emit("close-overlay");
  }
}

// ---------------------------------------------------------------------------
// 生命周期
// ---------------------------------------------------------------------------
watch(
  [() => props.open, () => props.repos.join("\u0000")],
  ([open]) => {
    syncRepoEntries();
    if (open) {
      loadRepoStatuses(false);
      focusSearchInput();
      syncAutoRefreshTimer();
    } else {
      stopAutoRefreshTimer();
    }
  },
  { immediate: true }
);

watch(autoRefresh, () => {
  syncAutoRefreshTimer();
});

onMounted(() => {
  window.addEventListener("keydown", handleGlobalKeydown);
});

onUnmounted(() => {
  isDisposed = true;
  window.removeEventListener("keydown", handleGlobalKeydown);
  stopAutoRefreshTimer();
  if (toastTimer) {
    clearTimeout(toastTimer);
    toastTimer = null;
  }
  for (const timer of fetchReloadTimers) clearTimeout(timer);
  fetchReloadTimers = [];
  statusQueue.length = 0;
  queuedPaths.clear();
  loadingPaths.clear();
});
</script>

<template>
  <UiModal
    :open="open"
    variant="bare"
    width="max-w-5xl"
    overlay="canvas"
    :label="t('dash.title')"
    panel-class="relative max-h-[calc(100vh-2.5rem)] select-none"
    close-on-backdrop
    @update:open="(open) => { if (!open) emit('close-overlay'); }"
  >
      <!-- ============================================== -->
      <!-- Header: 标题 / 聚合指标 / 全局操作 / 搜索过滤     -->
      <!-- ============================================== -->
      <div class="px-5 pt-4 pb-3 border-b border-line flex flex-col gap-3 shrink-0">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <FolderGit2 class="w-5 h-5 text-accent shrink-0" />
              <h2 class="text-sm font-bold text-ink whitespace-nowrap">{{ t("dash.title") }}</h2>
              <span
                v-if="aggregate.loadingCount > 0"
                class="flex items-center gap-1 text-[10px] text-ink-muted font-mono whitespace-nowrap"
              >
                <Loader2 class="w-3 h-3 animate-spin" />
                {{ t("dash.loading.count", { count: aggregate.loadingCount }) }}
              </span>
            </div>

            <!-- 副标题：说明总览用途 -->
            <p class="text-[10px] text-ink-muted mt-1 truncate">
              {{ t("dash.subtitle") }}
            </p>

            <!-- 聚合指标 -->
            <p class="text-[11px] text-ink-muted mt-1.5 flex flex-wrap items-center gap-x-2.5 gap-y-1 font-mono">
              <span class="text-ink-secondary">{{ t("dash.repo.count", { count: aggregate.repoCount }) }}</span>
              <span aria-hidden="true" class="w-px h-2.5 bg-line shrink-0"></span>
              <span :class="aggregate.dirtyRepos > 0 ? 'text-warning font-semibold' : 'text-ink-muted'">
                {{ t("dash.dirty.count", { count: aggregate.dirtyRepos }) }}
              </span>
              <span aria-hidden="true" class="w-px h-2.5 bg-line shrink-0"></span>
              <span>{{ t("dash.changes.count", { count: aggregate.totalChanges }) }}</span>
              <span aria-hidden="true" class="w-px h-2.5 bg-line shrink-0"></span>
              <span class="text-success" :title="t('dash.ahead.title')">↑{{ aggregate.totalAhead }}</span>
              <span class="text-purple" :title="t('dash.behind.title')">↓{{ aggregate.totalBehind }}</span>
              <span v-if="aggregate.errorCount > 0" class="text-danger">· {{ t("dash.errors.count", { count: aggregate.errorCount }) }}</span>
            </p>

            <p v-if="overview" class="text-[10px] text-ink-muted mt-1 font-mono truncate">
              {{ t("dash.active.summary", { name: activeRepo?.split("/").pop() ?? "", commits: overview.totalCommits, branches: overview.totalBranches }) }}
            </p>
            <p v-else-if="isLoadingOverview" class="text-[10px] text-ink-muted mt-1">{{ t("dash.overview.loading") }}</p>
          </div>

          <!-- 全局操作 -->
          <div class="flex items-center gap-2 shrink-0">
            <button
              type="button"
              @click="handleFetchAll"
              :disabled="aggregate.repoCount === 0"
              class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-accent-bg hover:bg-accent-bg-strong text-accent text-[11px] font-semibold transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed whitespace-nowrap"
              :title="t('dash.fetch.all.title')"
            >
              <Download class="w-3.5 h-3.5 shrink-0" />
              {{ t("dash.fetch.all") }}
            </button>

            <button
              type="button"
              @click="toggleAutoRefresh"
              class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border text-[11px] font-medium transition cursor-pointer whitespace-nowrap"
              :class="
                autoRefresh
                  ? 'bg-accent-bg border-accent/40 text-accent shadow-xs'
                  : 'bg-surface border-line text-ink-muted hover:text-ink hover:bg-raised'
              "
              :title="autoRefresh ? t('dash.auto.on.title') : t('dash.auto.off.title')"
            >
              <RefreshCw class="w-3.5 h-3.5 shrink-0" :class="{ 'animate-spin': autoRefresh }" />
              {{ t("dash.auto") }}
            </button>

            <button
              type="button"
              @click="refreshAllStatuses(true)"
              class="p-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer"
              :title="t('dash.refresh.title')"
            >
              <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': aggregate.loadingCount > 0 }" />
            </button>

            <button :aria-label="t('dash.close.title')"
              type="button"
              @click="emit('close-overlay')"
              class="p-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer"
              :title="t('dash.close.title')"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- 搜索过滤 -->
        <div class="flex items-center gap-3">
          <div class="relative flex-1 min-w-0">
            <input
              ref="searchInputRef"
              v-model="searchQuery"
              type="text"
              spellcheck="false"
              autocomplete="off"
              :aria-label="t('dash.filter.placeholder')"
              :placeholder="t('dash.filter.placeholder')"
              class="w-full pl-8 pr-8 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono transition"
            />
            <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2.5 top-2.5 pointer-events-none" />
            <button
              v-if="searchQuery"
              type="button"
              @click="searchQuery = ''"
              class="absolute right-2 top-2 text-[10px] text-ink-muted hover:text-ink font-bold cursor-pointer"
              :title="t('dash.filter.clear')"
            >
              ✕
            </button>
          </div>
          <span v-if="normalizedQuery" class="text-[10px] text-ink-muted font-mono whitespace-nowrap shrink-0">
            {{ t("dash.filter.count", { shown: visibleRepoCount, total: aggregate.repoCount }) }}
          </span>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- Body: 仓库卡片网格 / 空状态                     -->
      <!-- ============================================== -->
      <div class="flex-1 overflow-y-auto p-4">
        <!-- 空状态：没有任何已打开仓库 -->
        <div v-if="aggregate.repoCount === 0" class="py-16 flex flex-col items-center gap-3 text-center">
          <div class="w-12 h-12 rounded-2xl bg-raised border border-line flex items-center justify-center text-ink-muted shadow-sm">
            <FolderGit2 class="w-6 h-6" />
          </div>
          <div class="text-sm font-semibold text-ink">{{ t("dash.empty.title") }}</div>
          <p class="text-xs text-ink-muted max-w-sm leading-relaxed">
            {{ t("dash.empty.desc") }}
          </p>
          <div class="flex items-center gap-2 mt-2">
            <button
              @click="handleOpenLocalRepo"
              class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-accent-strong text-accent-contrast text-xs font-medium hover:bg-accent-strong-hover transition shadow-sm cursor-pointer"
            >
              <FolderOpen class="w-3.5 h-3.5" />
              <span>{{ t("dash.empty.open") }}</span>
            </button>
          </div>
        </div>

        <!-- 空状态：过滤无匹配 -->
        <div v-else-if="visibleRepoCount === 0" class="py-14 flex flex-col items-center gap-2 text-center">
          <Search class="w-6 h-6 text-ink-muted" />
          <div class="text-xs text-ink-muted">{{ t("dash.no.match") }}</div>
          <button
            type="button"
            @click="searchQuery = ''"
            class="text-[11px] text-accent hover:underline font-semibold cursor-pointer"
          >
            {{ t("dash.filter.clear.action") }}
          </button>
        </div>

        <!-- 仓库卡片网格 -->
        <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
          <div
            v-for="(row, index) in repoRows"
            :key="row.path"
            :ref="(el) => setCardRef(row, el)"
            :tabindex="cardTabindexFor(row, index)"
            @click="handleOpenRepo(row.path)"
            @focus="onCardFocus(row)"
            @keydown="onCardKeydown($event, index)"
            @keydown.enter.self="handleOpenRepo(row.path)"
            class="group relative flex flex-col gap-2.5 p-3.5 rounded-xl border transition-colors cursor-pointer min-w-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
            :class="
              row.isActive
                ? 'border-accent/60 bg-accent-bg shadow-xs'
                : 'border-line bg-canvas hover:bg-raised'
            "
            :title="t('dash.switch.title', { path: row.path })"
          >
            <!-- 标题行：名称 / 当前标记 / 更新时间 -->
            <div class="flex items-start justify-between gap-2 min-w-0">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5 min-w-0">
                  <FolderGit2 class="w-4 h-4 shrink-0" :class="row.isActive ? 'text-accent' : 'text-ink-muted'" />
                  <span class="text-sm font-semibold text-ink truncate">{{ row.name }}</span>
                  <span
                    v-if="row.isActive"
                    class="px-1.5 py-0.5 rounded-full bg-accent-bg text-accent text-[9px] font-bold shrink-0"
                  >
                    {{ t("dash.current") }}
                  </span>
                  <Loader2
                    v-if="row.entry?.loading"
                    class="w-3 h-3 text-ink-muted animate-spin shrink-0"
                    :title="t('dash.reading')"
                  />
                </div>
                <div class="text-[10px] text-ink-muted font-mono truncate mt-1" :title="row.path">
                  {{ row.path }}
                </div>
              </div>

              <span
                v-if="row.entry?.loadedAt"
                class="text-[9px] text-ink-muted font-mono shrink-0 whitespace-nowrap"
                :title="t('dash.updated.title', { time: formatLoadedTitle(row.entry.loadedAt) })"
              >
                {{ t("dash.updated", { time: formatLoadedTime(row.entry.loadedAt) }) }}
              </span>
            </div>

            <!-- 读取失败 -->
            <div
              v-if="row.entry?.error"
              class="flex items-center gap-2 px-2 py-1.5 rounded-lg bg-danger-bg border border-danger/25 text-[11px] text-danger/90"
            >
              <AlertCircle class="w-3.5 h-3.5 shrink-0" />
              <span class="truncate flex-1 min-w-0" :title="row.entry.error">{{ row.entry.error }}</span>
              <button
                type="button"
                @click.stop="retryRepo(row.path)"
                class="shrink-0 px-1.5 py-0.5 rounded bg-danger-bg hover:bg-danger-bg-strong text-danger font-semibold transition cursor-pointer"
              >
                {{ t("dash.retry") }}
              </button>
            </div>

            <!-- 首次读取中的占位 -->
            <div
              v-if="row.entry?.loading && !row.entry.status"
              class="flex items-center gap-2 py-1 text-[11px] text-ink-muted"
            >
              <Loader2 class="w-3.5 h-3.5 animate-spin shrink-0" />
              {{ t("dash.reading.status") }}
            </div>

            <!-- 状态主体 -->
            <template v-if="row.entry?.status">
              <!-- 分支 / 上下游 -->
              <div class="flex items-center gap-1.5 flex-wrap min-w-0">
                <span
                  class="flex items-center gap-1 px-1.5 py-0.5 rounded bg-raised border border-line text-[10px] font-mono text-ink-secondary max-w-[65%]"
                  :title="t('dash.branch.title', { branch: row.entry.status.currentBranch || t('dash.unknown') })"
                >
                  <GitBranch class="w-3 h-3 shrink-0 text-ink-muted" />
                  <span class="truncate">{{ row.entry.status.currentBranch || t("dash.unknown.branch") }}</span>
                </span>

                <span
                  v-if="(row.entry.status.ahead || 0) > 0"
                  class="flex items-center gap-0.5 px-1 py-0.5 rounded bg-success-bg text-success text-[10px] font-mono font-semibold"
                  :title="t('dash.ahead.commits', { count: row.entry.status.ahead })"
                >
                  <ArrowUpCircle class="w-2.5 h-2.5 shrink-0" />{{ row.entry.status.ahead }}
                </span>
                <span
                  v-if="(row.entry.status.behind || 0) > 0"
                  class="flex items-center gap-0.5 px-1 py-0.5 rounded bg-purple-bg text-purple text-[10px] font-mono font-semibold"
                  :title="t('dash.behind.commits', { count: row.entry.status.behind })"
                >
                  <ArrowDownCircle class="w-2.5 h-2.5 shrink-0" />{{ row.entry.status.behind }}
                </span>
                <span
                  v-if="
                    !(row.entry.status.ahead || 0) &&
                    !(row.entry.status.behind || 0) &&
                    row.entry.status.trackingBranch
                  "
                  class="flex items-center gap-1 text-[10px] text-ink-muted font-mono"
                  :title="t('dash.in.sync.title')"
                >
                  <CheckCircle2 class="w-2.5 h-2.5 text-success/70 shrink-0" />
                  {{ t("dash.in.sync") }}
                </span>
                <span
                  v-else-if="!row.entry.status.trackingBranch"
                  class="text-[9px] text-ink-muted font-mono"
                  :title="t('dash.no.upstream.title')"
                >
                  {{ t("dash.no.upstream") }}
                </span>
              </div>

              <!-- 进行中的合并/变基操作 -->
              <div
                v-if="
                  row.entry.status.isMerging ||
                  row.entry.status.isRebasing ||
                  row.entry.status.isCherryPicking
                "
                class="flex items-center gap-1 px-2 py-1 rounded-md bg-warning-bg border border-warning/25 text-[10px] text-warning font-semibold"
              >
                <AlertTriangle class="w-3 h-3 shrink-0" />
                {{
                  row.entry.status.isMerging
                    ? t("dash.op.merge")
                    : row.entry.status.isRebasing
                    ? t("dash.op.rebase")
                    : t("dash.op.cherry.pick")
                }}
              </div>

              <!-- 改动摘要（彩色圆点计数） -->
              <div class="flex items-center gap-3 flex-wrap text-[10px] font-mono min-h-[16px]">
                <template v-if="row.metrics.changes > 0">
                  <span v-if="row.metrics.staged > 0" class="flex items-center gap-1 text-success">
                    <span class="w-1.5 h-1.5 rounded-full bg-success-strong shrink-0"></span>
                    {{ t("dash.stat.staged", { count: row.metrics.staged }) }}
                  </span>
                  <span v-if="row.metrics.unstaged > 0" class="flex items-center gap-1 text-warning">
                    <span class="w-1.5 h-1.5 rounded-full bg-warning-strong shrink-0"></span>
                    {{ t("dash.stat.unstaged", { count: row.metrics.unstaged }) }}
                  </span>
                  <span v-if="row.metrics.untracked > 0" class="flex items-center gap-1 text-cyan">
                    <span class="w-1.5 h-1.5 rounded-full bg-cyan-strong shrink-0"></span>
                    {{ t("dash.stat.untracked", { count: row.metrics.untracked }) }}
                  </span>
                  <span v-if="row.metrics.conflicted > 0" class="flex items-center gap-1 text-danger">
                    <span class="w-1.5 h-1.5 rounded-full bg-danger-strong shrink-0"></span>
                    {{ t("dash.stat.conflicted", { count: row.metrics.conflicted }) }}
                  </span>
                </template>
                <span v-else class="flex items-center gap-1 text-ink-muted">
                  <CheckCircle2 class="w-3 h-3 text-success/60 shrink-0" />
                  {{ t("dash.clean") }}
                </span>
              </div>
            </template>

            <!-- 兜底：等待读取 -->
            <div
              v-else-if="!row.entry?.loading && !row.entry?.error"
              class="py-1 text-[11px] text-ink-muted"
            >
              {{ t("dash.waiting") }}
            </div>

            <!-- 卡片操作栏 -->
            <div
              class="mt-auto flex items-center justify-between gap-1.5 pt-2 border-t border-line/50"
              @click.stop
            >
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  @click="handleFetchRepo(row.path)"
                  class="px-2 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[10px] text-ink-secondary hover:text-ink font-semibold transition cursor-pointer flex items-center gap-1"
                  :title="t('dash.fetch.one.title', { name: row.name })"
                >
                  <Download class="w-3 h-3 shrink-0" />
                  Fetch
                </button>
                <button :aria-label="t('dash.terminal.title', { path: row.path })"
                  type="button"
                  @click="handleOpenTerminal(row.path)"
                  class="p-1 rounded-md bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer"
                  :title="t('dash.terminal.title', { path: row.path })"
                >
                  <Terminal class="w-3 h-3" />
                </button>
              </div>

              <button
                type="button"
                @click="handleCloseRepo(row.path)"
                class="px-2 py-1 rounded-md border border-transparent hover:border-danger/25 hover:bg-danger-bg text-[10px] text-ink-muted hover:text-danger font-semibold transition cursor-pointer flex items-center gap-1"
                :title="t('dash.close.tab.title', { name: row.name })"
              >
                <X class="w-3 h-3 shrink-0" />
                {{ t("dash.close.tab") }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- Footer 提示                                      -->
      <!-- ============================================== -->
      <div
        class="px-4 py-2 border-t border-line bg-canvas text-[10px] text-ink-muted flex items-center justify-between font-mono shrink-0"
      >
        <span class="truncate">{{ t("dash.footer.hint") }}</span>
        <span class="shrink-0 pl-3">{{ t("dash.footer.loaded", { loaded: aggregate.loadedCount, total: aggregate.repoCount }) }}</span>
      </div>

      <!-- 轻量操作反馈 Toast -->
      <transition
        enter-active-class="transition duration-150 ease-out"
        enter-from-class="transform translate-y-2 opacity-0"
        enter-to-class="transform translate-y-0 opacity-100"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="transform translate-y-0 opacity-100"
        leave-to-class="transform translate-y-2 opacity-0"
      >
        <div
          v-if="toastText"
          class="absolute bottom-11 left-1/2 -translate-x-1/2 px-3 py-1.5 rounded-full bg-ink text-canvas text-[11px] shadow-2xl flex items-center gap-1.5 pointer-events-none z-10 font-medium whitespace-nowrap"
        >
          <CheckCircle2 class="w-3.5 h-3.5 text-accent shrink-0" />
          <span>{{ toastText }}</span>
        </div>
      </transition>
  </UiModal>
</template>
