<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import {
  AlertTriangle,
  ChevronDown,
  GitCommit,
  RefreshCw,
  ShieldCheck,
  StickyNote,
} from "lucide-vue-next";
import type { CommitItem, GraphLine } from "../../types";
import { t } from "../../i18n";
import { relativeTime } from "../../utils/relativeTime";
import { LruCache } from "../../utils/lruCache";
import UiSkeleton from "../ui/UiSkeleton.vue";
import UiEmpty from "../ui/UiEmpty.vue";

const props = defineProps<{
  commits: CommitItem[];
  selectedHash: string | null;
  selectedHashes: Set<string>;
  notedHashes: Set<string>;
  isLoading: boolean;
  isFilterMode: boolean;
  hasMore: boolean;
  isLoadingMore: boolean;
  showGraph: boolean;
  /** Message from the last failed load; renders an error state instead of "empty". */
  error?: string;
}>();

const emit = defineEmits<{
  (e: "select", commit: CommitItem, index: number, event: MouseEvent | KeyboardEvent): void;
  (e: "checkbox", commit: CommitItem, index: number, event: MouseEvent): void;
  (e: "context-menu", commit: CommitItem, event: MouseEvent): void;
  (e: "load-more"): void;
  (e: "clear-filters"): void;
  (e: "retry"): void;
}>();

// One-line commits use the same 40px measure for the viewport, DAG and selection.
const ROW_HEIGHT = 40; // px
const OVERSCAN = 10; // extra rows rendered above / below the viewport
const PAGE_SIZE = 300; // unfiltered growing window step
const FILTER_PAGE_SIZE = 200; // server-side filtered page size
const LANE_WIDTH = 14; // px per DAG lane column
const MAX_VISIBLE_LANES = 18; // reasonable limit preventing graph from squeezing subject text
const MAX_GRAPH_GUTTER_WIDTH = 260; // px upper bound to guard against over-compression
const COMPACT_GUTTER_WIDTH = 20; // compact mode gutter (coloured dots only)
const LANE_PALETTE = [
  "#3d63dd",
  "#12a594",
  "#6e56cf",
  "#ffc53d",
  "#d6409f",
  "#00a2c7",
  "#f76b15",
  "#8e4ec6",
];

/* ------------------------------------------------------------------ */
/* Virtual scrolling                                                   */
/* ------------------------------------------------------------------ */
const listEl = ref<HTMLDivElement | null>(null);
const scrollTop = ref(0);
const viewportHeight = ref(720);
let listResizeObserver: ResizeObserver | null = null;

const startIndex = computed(() => Math.max(0, Math.floor(scrollTop.value / ROW_HEIGHT) - OVERSCAN));
const endIndex = computed(() => {
  const total = props.commits.length;
  const from = startIndex.value;
  const perScreen = Math.ceil(viewportHeight.value / ROW_HEIGHT) + OVERSCAN * 2;
  return Math.min(total, from + Math.max(perScreen, 1));
});
const visibleRows = computed<{ commit: CommitItem; index: number }[]>(() => {
  const rows: { commit: CommitItem; index: number }[] = [];
  for (let i = startIndex.value; i < endIndex.value; i++) {
    const commit = props.commits[i];
    if (commit) rows.push({ commit, index: i });
  }
  return rows;
});
const topPad = computed(() => startIndex.value * ROW_HEIGHT);
const bottomPad = computed(() =>
  Math.max(0, (props.commits.length - endIndex.value) * ROW_HEIGHT)
);

function measureViewport() {
  const el = listEl.value;
  if (!el) return;
  if (el.clientHeight > 0) viewportHeight.value = el.clientHeight;
  scrollTop.value = el.scrollTop;
}

function handleListScroll() {
  const el = listEl.value;
  if (!el) return;
  scrollTop.value = el.scrollTop;
  if (el.clientHeight > 0) viewportHeight.value = el.clientHeight;
  if (!props.hasMore || props.isLoading || props.isLoadingMore) return;
  const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
  if (distance < ROW_HEIGHT * 3) emit('load-more');
}

function handleViewportResize() {
  measureViewport();
}

function scrollToIndex(index: number) {
  const el = listEl.value;
  if (!el || index < 0) return;
  const rowTop = index * ROW_HEIGHT;
  const rowBottom = rowTop + ROW_HEIGHT;
  if (rowTop < el.scrollTop) {
    el.scrollTop = rowTop;
  } else if (rowBottom > el.scrollTop + el.clientHeight) {
    el.scrollTop = rowBottom - el.clientHeight;
  }
  scrollTop.value = el.scrollTop;
}

function resetScroll() {
  scrollTop.value = 0;
  if (listEl.value) listEl.value.scrollTop = 0;
}

function isHashSelected(hash: string): boolean {
  return props.selectedHashes.has(hash);
}

/** Row background: inspector selection wins, then multi-selection tint. */
function rowClasses(commit: CommitItem): string {
  if (props.selectedHash === commit.hash) return "bg-raised";
  if (props.selectedHashes.has(commit.hash)) return "bg-accent-bg";
  return "hover:bg-raised";
}

/* ------------------------------------------------------------------ */
/* DAG graph rendering helpers (dense, up to MAX_VISIBLE_LANES lanes)  */
/* ------------------------------------------------------------------ */
function rawLane(lane: number): number {
  return Number.isFinite(lane) ? Math.max(0, Math.floor(lane)) : 0;
}

/**
 * Highest lane index seen in the loaded window (commit dot + line endpoints).
 * The windowed rows render from this layout; it recomputes only when the
 * loaded commit list changes, never per scroll frame.
 */
const maxLaneIndex = computed(() => {
  let max = 0;
  for (const commit of props.commits) {
    const own = rawLane(commit.lane);
    if (own > max) max = own;
    for (const line of commit.lines ?? []) {
      const from = rawLane(line.fromLane);
      const to = rawLane(line.toLane);
      if (from > max) max = from;
      if (to > max) max = to;
    }
  }
  return max;
});

const totalLaneCount = computed(() =>
  Math.max(1, Math.min(maxLaneIndex.value + 1, MAX_VISIBLE_LANES))
);
const hiddenLaneCount = computed(() =>
  Math.max(0, maxLaneIndex.value + 1 - MAX_VISIBLE_LANES)
);
const graphGutterWidth = computed(() => {
  if (!props.showGraph) return COMPACT_GUTTER_WIDTH;
  const calculated = totalLaneCount.value * LANE_WIDTH;
  return Math.min(calculated, MAX_GRAPH_GUTTER_WIDTH);
});

function clampLane(lane: number): number {
  return Math.min(rawLane(lane), MAX_VISIBLE_LANES - 1);
}

function laneX(lane: number): number {
  return clampLane(lane) * LANE_WIDTH + LANE_WIDTH / 2;
}

/** X coordinate of a commit dot — lanes collapse into one column in compact mode. */
function graphDotX(lane: number): number {
  return props.showGraph ? laneX(lane) : COMPACT_GUTTER_WIDTH / 2;
}

function isMergeCommit(commit: CommitItem): boolean {
  return (commit.parents?.length ?? 0) > 1;
}

function laneColor(lane: number): string {
  return LANE_PALETTE[clampLane(lane) % LANE_PALETTE.length] ?? LANE_PALETTE[0]!;
}

interface GraphPath {
  d: string;
  color: string;
}

const graphPathsCache = new LruCache<string, GraphPath[]>(600);
watch(
  () => [props.commits, props.showGraph],
  () => {
    graphPathsCache.clear();
  }
);

/**
 * Builds the SVG segments for one row.
 * - Pass-through lanes (vertical at a lane that is not this commit's lane) span
 *   the full row so active branch lanes stay continuous.
 * - The commit's own lane starts at the node and continues to the bottom edge.
 * - Merge edges curve from the commit node (row centre) down to the parent lane.
 * - A short top→node stub is added when the previous row was already expecting
 *   this lane (i.e. the commit has an incoming edge), so lanes never "float".
 */
function graphPaths(index: number): GraphPath[] {
  // Compact mode and filtered mode render coloured dots only (no lane lines).
  if (!props.showGraph || props.isFilterMode) return [];
  const row = props.commits[index];
  if (!row) return [];

  const cacheKey = `${row.hash}:${totalLaneCount.value}`;
  const cached = graphPathsCache.get(cacheKey);
  if (cached) return cached;

  const paths: GraphPath[] = [];
  const seen = new Set<string>();
  const push = (d: string, color: string) => {
    const key = `${color}|${d}`;
    if (seen.has(key)) return;
    seen.add(key);
    paths.push({ d, color });
  };
  const lines: GraphLine[] = row.lines ?? [];
  const ownLane = clampLane(row.lane ?? 0);
  const midY = ROW_HEIGHT / 2;
  const prev = index > 0 ? props.commits[index - 1] : undefined;
  const prevTargets = new Set((prev?.lines ?? []).map((line) => clampLane(line.toLane)));
  // Lanes that are created by a merge edge in this very row must not draw a
  // phantom pass-through above the merge — the curve itself is the start.
  const mergeTargets = new Set(
    lines
      .filter((line) => clampLane(line.fromLane) !== clampLane(line.toLane))
      .map((line) => clampLane(line.toLane))
  );

  for (const line of lines) {
    const fromLane = clampLane(line.fromLane);
    const toLane = clampLane(line.toLane);
    const x1 = laneX(line.fromLane);
    const x2 = laneX(line.toLane);
    const color = line.color || laneColor(line.toLane);
    if (fromLane === toLane) {
      if (fromLane === ownLane) {
        push(`M ${x1} ${midY} L ${x1} ${ROW_HEIGHT}`, color);
      } else if (!mergeTargets.has(fromLane)) {
        push(`M ${x1} 0 L ${x1} ${ROW_HEIGHT}`, color);
      }
    } else {
      const laneDiff = Math.abs(fromLane - toLane);
      // Adaptive vertical shoulder: wider lane jumps drop deeper before sweeping horizontally
      // so the curve clears intervening commit nodes around midY instead of slicing through them.
      const shoulder = Math.min(ROW_HEIGHT - 6, midY + 8 + Math.min(laneDiff * 3, 16));
      push(`M ${x1} ${midY} C ${x1} ${shoulder}, ${x2} ${shoulder}, ${x2} ${ROW_HEIGHT}`, color);
    }
  }

  if (index > 0 && prevTargets.has(ownLane)) {
    push(`M ${laneX(ownLane)} 0 L ${laneX(ownLane)} ${midY}`, laneColor(ownLane));
  }

  graphPathsCache.set(cacheKey, paths);
  return paths;
}

/* Ref badges ---------------------------------------------------------- */
function visibleRefs(c: CommitItem): string[] {
  return (c.refs || []).slice(0, 3);
}
function extraRefCount(c: CommitItem): number {
  return Math.max(0, (c.refs || []).length - 3);
}
function isTagRef(ref: string): boolean {
  return ref.includes("tag:");
}
function refLabel(ref: string): string {
  return ref.replace("HEAD -> ", "").replace("tag: ", "");
}

const showMinimap = computed(() => props.commits.length >= 30);
const minimapEl = ref<HTMLDivElement | null>(null);
const isDraggingMinimap = ref(false);

const minimapSlider = computed(() => {
  const total = props.commits.length;
  if (total === 0) return { top: 0, height: 100 };
  const totalHeight = total * ROW_HEIGHT;
  const ratio = Math.min(1, viewportHeight.value / totalHeight);
  const sliderHeight = Math.max(24, ratio * viewportHeight.value);
  const scrollRatio = scrollTop.value / Math.max(1, totalHeight - viewportHeight.value);
  const maxTop = Math.max(0, viewportHeight.value - sliderHeight);
  const sliderTop = Math.min(maxTop, Math.max(0, scrollRatio * maxTop));
  return { top: sliderTop, height: sliderHeight };
});

function handleMinimapClick(e: MouseEvent) {
  if (!minimapEl.value || !listEl.value) return;
  const rect = minimapEl.value.getBoundingClientRect();
  const clickY = e.clientY - rect.top;
  const ratio = Math.max(0, Math.min(1, clickY / rect.height));
  const totalHeight = props.commits.length * ROW_HEIGHT;
  listEl.value.scrollTop = ratio * Math.max(0, totalHeight - viewportHeight.value);
}

function handleMinimapMouseDown(e: MouseEvent) {
  isDraggingMinimap.value = true;
  handleMinimapClick(e);
  window.addEventListener("mousemove", handleMinimapMouseMove);
  window.addEventListener("mouseup", handleMinimapMouseUp);
}

function handleMinimapMouseMove(e: MouseEvent) {
  if (!isDraggingMinimap.value) return;
  handleMinimapClick(e);
}

function handleMinimapMouseUp() {
  isDraggingMinimap.value = false;
  window.removeEventListener("mousemove", handleMinimapMouseMove);
  window.removeEventListener("mouseup", handleMinimapMouseUp);
}

onMounted(() => {
  window.addEventListener("resize", handleViewportResize);
  if (typeof ResizeObserver !== "undefined" && listEl.value) {
    listResizeObserver = new ResizeObserver(() => measureViewport());
    listResizeObserver.observe(listEl.value);
  }
  measureViewport();
});

onUnmounted(() => {
  window.removeEventListener("resize", handleViewportResize);
  if (isDraggingMinimap.value) {
    window.removeEventListener("mousemove", handleMinimapMouseMove);
    window.removeEventListener("mouseup", handleMinimapMouseUp);
  }
  if (listResizeObserver) {
    listResizeObserver.disconnect();
    listResizeObserver = null;
  }
});

defineExpose({ scrollToIndex, resetScroll });
</script>

<template>
  <div class="relative flex-1 flex min-h-0 w-full overflow-hidden">
    <!-- Commit Graph Rows (virtualized) -->
    <div
      ref="listEl"
      class="flex-1 overflow-y-auto relative"
      @scroll.passive="handleListScroll"
    >
        <!-- Loading state: rows, not a spinner — the list keeps its height -->
        <div v-if="isLoading && commits.length === 0" class="py-1">
          <UiSkeleton variant="rows" :rows="9" :row-height="ROW_HEIGHT" :gutter="graphGutterWidth" />
          <p class="sr-only">{{ isFilterMode ? t("history.graph.loading.filter") : t("history.graph.loading") }}</p>
        </div>

        <!-- Error state: a failed load must not read as "no commits" -->
        <div
          v-else-if="error"
          class="flex flex-col items-center justify-center gap-2 py-16 px-6 text-center"
        >
          <AlertTriangle class="w-6 h-6 text-danger" />
          <span class="text-xs font-semibold text-ink">{{ t("history.graph.error.title") }}</span>
          <span class="text-[10px] text-ink-muted max-w-sm leading-relaxed break-words">{{ error }}</span>
          <button
            @click="emit('retry')"
            class="mt-1 px-2.5 py-1 rounded-md bg-raised border border-line text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer"
          >
            {{ t("history.graph.error.retry") }}
          </button>
        </div>

        <!-- Empty state: icon + one sentence + one action (see DESIGN_SYSTEM 8.5) -->
        <UiEmpty
          v-else-if="commits.length === 0"
          :icon="GitCommit"
          :title="isFilterMode ? t('history.graph.empty.filter') : t('history.graph.empty')"
          :hint="isFilterMode ? '' : t('history.graph.empty.hint')"
          class="py-16"
        >
          <button
            v-if="isFilterMode"
            @click="emit('clear-filters')"
            class="mt-1 px-2.5 py-1 rounded-md bg-raised border border-line text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer"
          >
            {{ t("history.graph.clear.filter") }}
          </button>
        </UiEmpty>

        <!-- Virtualized rows -->
        <template v-else>
        <div role="listbox" :aria-label="t('history.graph.list')" class="contents">
          <div :style="{ height: topPad + 'px' }" aria-hidden="true"></div>

          <div
            v-for="row in visibleRows"
            :key="row.commit.hash"
            role="option"
            tabindex="0"
            :aria-selected="selectedHash === row.commit.hash"
            @click="emit('select', row.commit, row.index, $event)"
            @keydown.enter.prevent="emit('select', row.commit, row.index, $event)"
            @keydown.space.prevent="emit('select', row.commit, row.index, $event)"
            @contextmenu.prevent="emit('context-menu', row.commit, $event)"
            class="group relative flex items-center gap-2 pr-3 border-b border-line/30 transition-colors duration-100 cursor-pointer overflow-hidden focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:ring-inset"
            :class="rowClasses(row.commit)"
            :style="{ height: ROW_HEIGHT + 'px' }"
          >
            <span
              v-if="selectedHash === row.commit.hash"
              class="absolute left-0 top-1 bottom-1 w-0.5 rounded-full bg-accent-strong"
            ></span>

            <!-- Multi-select checkbox (Ctrl/⌘+click or Shift+click on the row also works) -->
            <div
              class="shrink-0 flex items-center justify-center w-7 self-stretch"
              :class="isHashSelected(row.commit.hash) ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'"
            >
              <input
                type="checkbox"
                :checked="isHashSelected(row.commit.hash)"
                @click.stop.prevent="emit('checkbox', row.commit, row.index, $event)"
                class="w-3.5 h-3.5 rounded border border-line bg-canvas accent-accent cursor-pointer"
                :title="t('history.graph.select.hint')"
              />
            </div>

            <!-- Dense DAG graph gutter (dynamic width, explicit pixel coordinates) -->
            <div class="relative shrink-0 self-stretch" :style="{ width: graphGutterWidth + 'px' }">
              <svg
                class="absolute inset-0 pointer-events-none overflow-hidden"
                :width="graphGutterWidth"
                :height="ROW_HEIGHT"
              >
                <path
                  v-for="(path, pathIndex) in graphPaths(row.index)"
                  :key="pathIndex"
                  :d="path.d"
                  :stroke="path.color"
                  stroke-width="1.5"
                  fill="none"
                  stroke-linecap="round"
                  opacity="0.85"
                />
                <!-- Merge commit: extra ring around the node -->
                <circle
                  v-if="isMergeCommit(row.commit)"
                  :cx="graphDotX(row.commit.lane)"
                  :cy="ROW_HEIGHT / 2"
                  r="6.5"
                  fill="none"
                  :stroke="laneColor(row.commit.lane)"
                  stroke-width="1"
                  opacity="0.75"
                />
                <!-- HEAD: hollow accent ring -->
                <circle
                  v-if="row.commit.isHead"
                  :cx="graphDotX(row.commit.lane)"
                  :cy="ROW_HEIGHT / 2"
                  r="8.5"
                  fill="none"
                  stroke="var(--color-accent)"
                  stroke-width="1.25"
                  opacity="0.9"
                />
                <circle
                  :cx="graphDotX(row.commit.lane)"
                  :cy="ROW_HEIGHT / 2"
                  :r="selectedHash === row.commit.hash ? 5 : 4"
                  :fill="laneColor(row.commit.lane)"
                  stroke="var(--color-canvas)"
                  stroke-width="1.5"
                />
              </svg>
              <!-- Faded right edge when lanes beyond MAX_VISIBLE_LANES are folded -->
              <div
                v-if="showGraph && hiddenLaneCount > 0"
                class="absolute right-0 top-0 bottom-0 w-2 pointer-events-none"
                style="background: linear-gradient(to left, var(--color-canvas), transparent)"
              ></div>
            </div>

            <!-- Subject & Branch Tags -->
            <div class="flex items-center gap-2 min-w-0 flex-1">
              <div
                v-if="visibleRefs(row.commit).length > 0"
                class="hidden sm:flex items-center gap-1 shrink-0 max-w-[45%] overflow-hidden"
              >
                <span
                  v-for="r in visibleRefs(row.commit)"
                  :key="r"
                  class="px-1.5 py-0.5 rounded-md text-[10px] font-mono font-semibold whitespace-nowrap"
                  :class="isTagRef(r) ? 'bg-purple-bg text-purple border border-purple/30' : 'bg-cyan-bg text-cyan border border-cyan/30'"
                >
                  {{ refLabel(r) }}
                </span>
                <span
                  v-if="extraRefCount(row.commit) > 0"
                  class="px-1.5 py-0.5 rounded-md bg-surface border border-line text-[10px] font-mono text-ink-muted whitespace-nowrap"
                >
                  +{{ extraRefCount(row.commit) }}
                </span>
              </div>

              <!-- Subject -->
              <span
                class="text-xs truncate font-medium"
                :title="row.commit.subject"
                :class="selectedHash === row.commit.hash ? 'text-ink font-semibold' : 'text-ink-secondary group-hover:text-ink'"
              >
                {{ row.commit.subject }}
              </span>

              <!-- Git Notes Badge -->
              <span
                v-if="notedHashes.has(row.commit.hash)"
                class="inline-flex items-center gap-1 px-1.5 py-0.2 rounded text-[10px] font-mono bg-warning-bg text-warning border border-warning/30 shrink-0 select-none"
                :title="t('history.graph.notes.hint')"
              >
                <StickyNote class="w-2.5 h-2.5" /> Note
              </span>
            </div>

            <!-- Right: Author & Date & Hash -->
            <div class="flex items-center gap-3 shrink-0 text-xs font-mono text-ink-muted">
              <!-- Signature status -->
              <span
                v-if="row.commit.signatureStatus === 'G'"
                class="flex items-center text-success"
                :title="t('history.graph.signed.hint')"
              >
                <ShieldCheck class="w-3.5 h-3.5" />
              </span>

              <span class="text-ink-secondary text-[11px] font-sans truncate max-w-[100px]" :title="row.commit.authorName">{{ row.commit.authorName }}</span>
              <span class="text-[11px] font-sans text-ink-muted whitespace-nowrap">{{ relativeTime(row.commit.date, row.commit.relativeDate) }}</span>
              <span class="px-1.5 py-0.5 rounded bg-surface border border-line text-[10px] text-ink-secondary">
                {{ row.commit.shortHash }}
              </span>
            </div>
          </div>

          <div :style="{ height: bottomPad + 'px' }" aria-hidden="true"></div>
        </div>

          <!-- Pagination footer -->
          <div class="flex items-center justify-center gap-3 px-4 py-3 border-t border-line/30 bg-surface">
            <template v-if="hasMore">
              <button
                @click="emit('load-more')"
                :disabled="isLoadingMore"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-raised hover:bg-hover-strong border border-line text-xs font-medium text-ink transition cursor-pointer disabled:opacity-60 disabled:cursor-wait"
              >
                <RefreshCw v-if="isLoadingMore" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
                <ChevronDown v-else class="w-3.5 h-3.5" aria-hidden="true" />
                <span>{{ isLoadingMore ? t("history.graph.loading.more") : t("history.graph.load.more") }}</span>
              </button>
              <span class="text-[10px] text-ink-muted font-mono">
                {{ t("history.graph.loaded", { count: commits.length }) }}
              </span>
            </template>
            <span v-else class="text-[10px] text-ink-muted font-mono">
              {{ t("history.graph.loaded.all", { count: commits.length }) }}
            </span>
          </div>
        </template>
      </div>

    <!-- Minimap Navigator Strip -->
    <div
      v-if="showMinimap"
      ref="minimapEl"
      @mousedown="handleMinimapMouseDown"
      class="w-3.5 shrink-0 z-20 bg-canvas hover:bg-canvas border-l border-line/40 select-none cursor-pointer group transition-colors relative"
      :title="t('history.graph.minimap.hint')"
    >
      <div
        class="absolute left-0.5 right-0.5 rounded-full bg-accent/40 group-hover:bg-accent/70 transition-all shadow-xs"
        :style="{
          top: minimapSlider.top + 'px',
          height: minimapSlider.height + 'px'
        }"
      ></div>
    </div>
  </div>
</template>
