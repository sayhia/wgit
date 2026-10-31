<script setup lang="ts">
import { t, backendText } from "../i18n";
import { computed, ref, onMounted, onUnmounted, watch, nextTick } from "vue";
import {
  FolderGit2,
  FolderOpen,
  X,
  Plus,
  LayoutDashboard,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Terminal,
  Copy,
  RefreshCw,
  XCircle,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import { useContextMenu, type ContextMenuItem } from "../composables/useContextMenu";
import UiContextMenu from "./ui/UiContextMenu.vue";
import type { RepoHealthReport, RepoStatus } from "../types";

const props = defineProps<{
  repos: string[];
  activeRepo: string;
  statuses: Record<string, RepoStatus | null>;
  dashboardOpen?: boolean;
}>();

const emit = defineEmits<{
  (e: "select", path: string): void;
  (e: "close", path: string): void;
  (e: "close-others", keepPath: string): void;
  (e: "reorder", repos: string[]): void;
  (e: "refresh-repo", path: string): void;
  (e: "notify", message: string): void;
  (e: "open-dialog"): void;
  (e: "open-new"): void;
  (e: "toggle-dashboard"): void;
}>();

function repoName(path: string): string {
  const parts = path.replace(/\\/g, "/").split("/").filter(Boolean);
  return parts[parts.length - 1] || path;
}

function isDirty(path: string): boolean {
  const s = props.statuses[path];
  if (!s) return false;
  return (
    (s.stagedFiles?.length || 0) > 0 ||
    (s.unstagedFiles?.length || 0) > 0 ||
    (s.untrackedFiles?.length || 0) > 0 ||
    (s.conflictedFiles?.length || 0) > 0
  );
}

function statusFor(path: string): RepoStatus | null {
  return props.statuses[path] ?? null;
}

const dirtyCount = computed(() => props.repos.filter((p) => isDirty(p)).length);

// ----------------- Repo Health Badges (lazy, cached, silent) -----------------
//
// The backend RunRepoHealthCheck fans out six git probes per repo (config,
// fsck, ignore audit, large-file scan, signing, status), so a single check can
// cost far more than a few dozen ms. The rail therefore renders badge-less
// cards first, then probes repos one by one in a debounced background queue;
// results fill the dots in as they arrive and failures stay silent (no badge,
// no toast). Reports are cached per repoPath for the component's lifetime so
// rail refreshes never rescan an already-checked repo.

type RepoHealthLevel = "success" | "warning" | "danger";

interface RepoHealthBadge {
  level: RepoHealthLevel;
  score: number;
  summaryText: string;
  summaryKey: string;
  summaryParams: Record<string, string>;
}

const HEALTH_REFRESH_DEBOUNCE_MS = 400;

const healthBadges = ref<Record<string, RepoHealthBadge>>({});
const healthCache = new Map<string, RepoHealthBadge>();
const healthQueued = new Set<string>();
let healthQueue: string[] = [];
let healthDraining = false;
let healthDebounceTimer: ReturnType<typeof setTimeout> | null = null;
let healthDisposed = false;

function healthLevelFor(report: RepoHealthReport): RepoHealthLevel {
  if (report.failCount > 0 || report.score < 60) return "danger";
  if (report.warnCount > 0 || report.score < 90) return "warning";
  return "success";
}

function healthBadgeFor(report: RepoHealthReport): RepoHealthBadge {
  const hasIssues = report.warnCount + report.failCount > 0;
  return {
    level: healthLevelFor(report),
    score: report.score,
    summaryText: report.summary,
    summaryKey: report.summaryKey || (hasIssues ? "health.summary.mixed" : "health.summary.ok"),
    summaryParams:
      report.summaryParams ||
      (hasIssues
        ? {
            pass: String(report.passCount),
            warn: String(report.warnCount),
            fail: String(report.failCount),
            score: String(report.score),
          }
        : { score: String(report.score) }),
  };
}

function healthBadgeClass(repo: string): string {
  const badge = healthBadges.value[repo];
  if (!badge) return "";
  if (badge.level === "success") return "bg-success";
  if (badge.level === "warning") return "bg-warning-strong";
  return "bg-danger";
}

function healthBadgeTitle(repo: string): string {
  const badge = healthBadges.value[repo];
  if (!badge) return "";
  return backendText(badge.summaryKey, badge.summaryText, badge.summaryParams) || `health ${badge.score}`;
}

function scheduleHealthRefresh() {
  if (healthDebounceTimer) clearTimeout(healthDebounceTimer);
  healthDebounceTimer = setTimeout(() => {
    healthDebounceTimer = null;
    for (const repo of props.repos) {
      if (healthCache.has(repo) || healthQueued.has(repo)) continue;
      healthQueued.add(repo);
      healthQueue.push(repo);
    }
    void drainHealthQueue();
  }, HEALTH_REFRESH_DEBOUNCE_MS);
}

async function drainHealthQueue() {
  if (healthDraining) return;
  healthDraining = true;
  try {
    while (healthQueue.length > 0) {
      if (healthDisposed) return;
      const repo = healthQueue.shift();
      if (!repo) continue;
      healthQueued.delete(repo);
      // Closed while queued — skip the probe; any cached report stays cached.
      if (!props.repos.includes(repo)) continue;
      try {
        const report = await gitService.runRepoHealthCheck(repo);
        if (!report || healthDisposed) continue;
        const badge = healthBadgeFor(report);
        healthCache.set(repo, badge);
        healthBadges.value = { ...healthBadges.value, [repo]: badge };
      } catch {
        // Probe failed — leave the card badge-less, never surface a toast.
      }
    }
  } finally {
    healthDraining = false;
  }
}

function openRepoHealth(repo: string) {
  // The maintenance health panel reports on the active repo, so select first.
  emit("select", repo);
  window.dispatchEvent(new CustomEvent("wgit:tools-tab", { detail: { tab: "maintenance", section: "health" } }));
}

// ----------------- Drag & Drop Tab Reordering -----------------
const draggedIndex = ref<number | null>(null);
const dropTargetIndex = ref<number | null>(null);
const dropPosition = ref<"before" | "after" | null>(null);

function resetDragState() {
  draggedIndex.value = null;
  dropTargetIndex.value = null;
  dropPosition.value = null;
}

function reorderArray(list: string[], from: number, to: number, position: "before" | "after"): string[] {
  if (from < 0 || from >= list.length || to < 0 || to >= list.length) return list;
  if (from === to) return list;
  const item = list[from];
  if (!item) return list;
  const copy = [...list];
  copy.splice(from, 1);
  let targetIdxInCopy = to > from ? to - 1 : to;
  if (position === "after") {
    targetIdxInCopy += 1;
  }
  copy.splice(targetIdxInCopy, 0, item);
  return copy;
}

function onDragStart(idx: number, event: DragEvent) {
  draggedIndex.value = idx;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", String(idx));
  }
}

function onDragEnd() {
  resetDragState();
}

function onDragOver(idx: number, event: DragEvent) {
  if (draggedIndex.value === null) return;
  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = "move";
  }
  const target = event.currentTarget as HTMLElement | null;
  if (!target) return;
  const rect = target.getBoundingClientRect();
  const midX = rect.left + rect.width / 2;
  dropTargetIndex.value = idx;
  dropPosition.value = event.clientX < midX ? "before" : "after";
}

function onDrop(idx: number, event: DragEvent) {
  event.preventDefault();
  const from = draggedIndex.value ?? Number(event.dataTransfer?.getData("text/plain"));
  const to = idx;
  const target = event.currentTarget as HTMLElement | null;
  const rect = target?.getBoundingClientRect();
  const pos = dropPosition.value || (rect && event.clientX < rect.left + rect.width / 2 ? "before" : "after");

  resetDragState();

  if (isNaN(from) || from < 0 || from >= props.repos.length) return;
  if (to < 0 || to >= props.repos.length) return;
  if (from === to) return;

  const next = reorderArray(props.repos, from, to, pos);
  const changed = next.some((p, i) => p !== props.repos[i]);
  if (changed) {
    emit("reorder", next);
  }
}

function isDropIndicator(idx: number, position: "before" | "after"): boolean {
  if (draggedIndex.value === null || dropTargetIndex.value !== idx || dropPosition.value !== position) {
    return false;
  }
  if (draggedIndex.value === idx) return false;
  if (position === "before" && draggedIndex.value === idx - 1) return false;
  if (position === "after" && draggedIndex.value === idx + 1) return false;
  return true;
}

// ----------------- Context Menu -----------------
const {
  isOpen: tabMenuOpen,
  position: tabMenuPos,
  items: tabMenuItems,
  open: openContextMenu,
  close: closeTabMenu,
} = useContextMenu();

const contextMenuRepo = ref<string>("");

function openTabContextMenu(repo: string, event: MouseEvent) {
  contextMenuRepo.value = repo;
  const items: ContextMenuItem[] = [
    {
      id: "reveal",
      label: t("rail.menu.reveal"),
      icon: ExternalLink,
    },
    {
      id: "terminal",
      label: t("rail.menu.terminal"),
      icon: Terminal,
    },
    {
      id: "copy-path",
      label: t("rail.menu.copy.path"),
      icon: Copy,
    },
    {
      id: "refresh",
      label: t("rail.menu.refresh"),
      icon: RefreshCw,
    },
    {
      id: "close-others",
      label: t("rail.menu.close.others"),
      icon: XCircle,
      separatorBefore: true,
      disabled: props.repos.length <= 1,
    },
    {
      id: "close",
      label: t("rail.menu.close"),
      icon: X,
      danger: true,
    },
  ];
  openContextMenu(event, items);
}

async function handleTabMenuSelect(id: string) {
  const repo = contextMenuRepo.value;
  if (!repo) return;

  switch (id) {
    case "reveal":
      try {
        await gitService.revealFileInFinder(repo, "");
      } catch (err: any) {
        emit("notify", String(err?.message || err));
      }
      break;
    case "terminal":
      try {
        await gitService.openInTerminal(repo);
      } catch (err: any) {
        emit("notify", String(err?.message || err));
      }
      break;
    case "copy-path":
      try {
        if (navigator?.clipboard?.writeText) {
          await navigator.clipboard.writeText(repo);
        }
        emit("notify", t("rail.menu.copy.done"));
      } catch {
        emit("notify", repo);
      }
      break;
    case "refresh":
      emit("refresh-repo", repo);
      break;
    case "close-others":
      emit("close-others", repo);
      break;
    case "close":
      emit("close", repo);
      break;
  }
}

// ----------------- Horizontal Scroll Controls -----------------
const tabsContainerRef = ref<HTMLDivElement | null>(null);
const canScrollLeft = ref(false);
const canScrollRight = ref(false);
let resizeObserver: ResizeObserver | null = null;

function updateScrollIndicators() {
  const el = tabsContainerRef.value;
  if (!el) {
    canScrollLeft.value = false;
    canScrollRight.value = false;
    return;
  }
  canScrollLeft.value = el.scrollLeft > 2;
  canScrollRight.value = el.scrollLeft + el.clientWidth < el.scrollWidth - 2;
}

function scrollTabs(direction: "left" | "right") {
  const el = tabsContainerRef.value;
  if (!el) return;
  const amount = 200;
  el.scrollBy({ left: direction === "left" ? -amount : amount, behavior: "smooth" });
}

function onTabsWheel(event: WheelEvent) {
  const el = tabsContainerRef.value;
  if (!el) return;
  if (Math.abs(event.deltaY) > Math.abs(event.deltaX)) {
    el.scrollLeft += event.deltaY;
    updateScrollIndicators();
  }
}

onMounted(() => {
  updateScrollIndicators();
  if (tabsContainerRef.value && typeof ResizeObserver !== "undefined") {
    resizeObserver = new ResizeObserver(() => {
      updateScrollIndicators();
    });
    resizeObserver.observe(tabsContainerRef.value);
  }
  // Lazy health badges: cards render first, probes start shortly after mount.
  scheduleHealthRefresh();
});

onUnmounted(() => {
  resizeObserver?.disconnect();
  resizeObserver = null;
  healthDisposed = true;
  if (healthDebounceTimer) {
    clearTimeout(healthDebounceTimer);
    healthDebounceTimer = null;
  }
  healthQueue = [];
});

watch(
  () => props.repos,
  () => {
    nextTick(updateScrollIndicators);
    // Newly opened repos join the (cached) background health queue.
    scheduleHealthRefresh();
  },
  { deep: true }
);

watch(
  () => props.activeRepo,
  () => {
    nextTick(() => {
      const activeEl = tabsContainerRef.value?.querySelector('[data-active="true"]') as HTMLElement | null;
      activeEl?.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "nearest" });
      updateScrollIndicators();
    });
  }
);
</script>

<template>
  <div data-glass class="h-9 border-b border-line glass glass-bar flex items-center gap-1 px-2 select-none shrink-0 z-20">
    <!-- Open repository tabs container with scroll controls -->
    <div class="relative flex items-center min-w-0 flex-1 h-full">
      <!-- Left scroll indicator & button -->
      <button
        v-show="canScrollLeft"
        type="button"
        @click="scrollTabs('left')"
        class="absolute left-0 top-1 bottom-1 z-30 w-5 flex items-center justify-center rounded-l bg-surface hover:bg-raised border-r border-line text-ink-muted hover:text-ink transition cursor-pointer"
        title="Scroll left"
      >
        <ChevronLeft class="w-3 h-3" />
      </button>

      <div
        ref="tabsContainerRef"
        @scroll.passive="updateScrollIndicators"
        @wheel.passive="onTabsWheel"
        class="flex items-center gap-1 overflow-x-auto min-w-0 flex-1 scrollbar-none h-full py-0.5"
        :class="{ 'pl-5': canScrollLeft, 'pr-5': canScrollRight }"
      >
        <div
          v-for="(repo, idx) in repos"
          :key="repo"
          class="relative flex items-center shrink-0"
          @dragover.prevent="onDragOver(idx, $event)"
          @drop.prevent="onDrop(idx, $event)"
        >
          <!-- Drop insertion indicator line (before) -->
          <div
            v-if="isDropIndicator(idx, 'before')"
            class="absolute -left-1 top-1 bottom-1 w-0.5 bg-accent z-30 rounded-full pointer-events-none"
          />

          <button
            :data-active="repo === activeRepo"
            draggable="true"
            @dragstart="onDragStart(idx, $event)"
            @dragend="onDragEnd"
            @click="emit('select', repo)"
            @contextmenu.prevent="openTabContextMenu(repo, $event)"
            class="group relative flex items-center gap-1.5 px-2.5 py-1 rounded-md border text-[11px] transition-all cursor-pointer select-none max-w-[220px]"
            :class="[
              repo === activeRepo
                ? 'bg-surface border-line text-ink font-semibold'
                : 'bg-transparent border-transparent text-ink-secondary hover:text-ink hover:bg-raised',
              draggedIndex === idx ? 'opacity-40 scale-[0.98]' : '',
              dropTargetIndex === idx && draggedIndex !== null && draggedIndex !== idx
                ? 'ring-1 ring-accent/60'
                : '',
            ]"
            :title="repo + (statusFor(repo)?.currentBranch ? t('rail.branch.suffix', { branch: statusFor(repo)?.currentBranch ?? '' }) : '')"
          >
            <FolderGit2 class="w-3 h-3 shrink-0" :class="repo === activeRepo ? 'text-accent' : 'text-ink-muted'" />
            <span class="truncate font-mono">{{ repoName(repo) }}</span>

            <!-- dirty badge -->
            <span
              v-if="isDirty(repo)"
              class="w-1.5 h-1.5 rounded-full bg-warning-strong shrink-0"
              :title="t('rail.dirty')"
            ></span>

            <!-- ahead/behind -->
            <span
              v-if="statusFor(repo) && ((statusFor(repo)!.ahead || 0) > 0 || (statusFor(repo)!.behind || 0) > 0)"
              class="text-[9px] font-mono text-ink-muted shrink-0 flex items-center gap-0.5"
            >
              <span v-if="(statusFor(repo)!.ahead || 0) > 0" class="text-cyan">↑{{ statusFor(repo)!.ahead }}</span>
              <span v-if="(statusFor(repo)!.behind || 0) > 0" class="text-warning">↓{{ statusFor(repo)!.behind }}</span>
            </span>

            <!-- close button -->
            <button
              v-if="repos.length > 1 || repo === activeRepo"
              @click.stop="emit('close', repo)"
              @dragstart.stop.prevent
              class="p-0.5 rounded text-ink-muted hover:text-danger hover:bg-danger-bg transition cursor-pointer shrink-0 opacity-0 group-hover:opacity-100"
              :class="{ 'opacity-100': repo === activeRepo }"
              :title="t('rail.close')"
            >
              <X class="w-2.5 h-2.5" />
            </button>

            <!-- shortcut hint -->
            <span
              v-if="idx < 9 && repo === activeRepo"
              class="text-[8px] font-mono text-ink-muted shrink-0 hidden sm:inline"
              :title="t('rail.switch.hint')"
            >
              ⌘{{ idx + 1 }}
            </span>
          </button>

          <!-- repo health badge — lazy: the dot appears only once the background probe lands -->
          <!--
            Rendered inline rather than absolutely positioned: pinned to a corner
            it either sat on the close button (top-right) or floated outside the
            tab on the rail border (top-left). Inline it groups with the dirty /
            ahead-behind chips and can never cover another control.
          -->
          <button
            v-if="healthBadges[repo]"
            type="button"
            data-testid="repo-health-badge"
            class="repo-health-badge absolute left-2 top-0.5 z-10 w-2 h-2 rounded-full cursor-pointer ring-1 ring-surface transition before:absolute before:-inset-1.5 before:content-[''] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/70"
            :class="healthBadgeClass(repo)"
            :title="healthBadgeTitle(repo)"
            :aria-label="healthBadgeTitle(repo)"
            @click.stop="openRepoHealth(repo)"
          ></button>

          <!-- Drop insertion indicator line (after) -->
          <div
            v-if="isDropIndicator(idx, 'after')"
            class="absolute -right-1 top-1 bottom-1 w-0.5 bg-accent z-30 rounded-full pointer-events-none"
          />
        </div>

        <div v-if="repos.length === 0" class="text-[11px] text-ink-muted px-2">{{ t('rail.empty') }}</div>
      </div>

      <!-- Right scroll indicator & button -->
      <button
        v-show="canScrollRight"
        type="button"
        @click="scrollTabs('right')"
        class="absolute right-0 top-1 bottom-1 z-30 w-5 flex items-center justify-center rounded-r bg-surface hover:bg-raised border-l border-line text-ink-muted hover:text-ink transition cursor-pointer"
        title="Scroll right"
      >
        <ChevronRight class="w-3 h-3" />
      </button>
    </div>

    <!-- actions -->
    <div class="flex items-center gap-1 shrink-0 pl-1 border-l border-line">
      <!-- Dirty summary is the first thing to go: it is informational only, and
           wrapping it used to stretch the 36px rail into three lines. -->
      <span
        v-if="dirtyCount > 0"
        class="hidden lg:inline text-[10px] text-warning font-medium mr-1 whitespace-nowrap"
      >{{ t('rail.dirty.count', { count: dirtyCount }) }}</span>
      <button
        @click="emit('open-dialog')"
        class="flex items-center gap-1 px-2 py-1 rounded-md bg-transparent hover:bg-raised border border-transparent text-[10px] text-accent hover:text-accent-strong transition cursor-pointer whitespace-nowrap"
        :title="t('rail.open.dir.title')"
      >
        <FolderOpen class="w-3 h-3" />
        <span class="hidden md:inline">{{ t('rail.open.dir') }}</span>
      </button>
      <button
        @click="emit('open-new')"
        class="flex items-center gap-1 px-2 py-1 rounded-md bg-transparent hover:bg-raised border border-transparent text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer whitespace-nowrap"
        :title="t('rail.more.title')"
      >
        <Plus class="w-3 h-3" />
        <span class="hidden md:inline">{{ t('rail.more') }}</span>
        <ChevronDown class="w-2.5 h-2.5 text-ink-muted" />
      </button>
      <button
        @click="emit('toggle-dashboard')"
        class="flex items-center gap-1 px-2 py-1 rounded-md transition cursor-pointer border text-[10px] whitespace-nowrap"
        :class="
          dashboardOpen
            ? 'bg-accent-bg border-accent-border text-accent'
            : 'bg-transparent hover:bg-raised border-transparent text-ink-secondary hover:text-ink'
        "
        :title="t('rail.dashboard.title')"
      >
        <LayoutDashboard class="w-3 h-3" />
        <span class="hidden md:inline">{{ t('rail.dashboard') }}</span>
      </button>
    </div>

    <!-- Tab Context Menu -->
    <UiContextMenu
      :open="tabMenuOpen"
      :x="tabMenuPos.x"
      :y="tabMenuPos.y"
      :items="tabMenuItems"
      @select="handleTabMenuSelect"
      @close="closeTabMenu"
    />
  </div>
</template>
