<script setup lang="ts">
import { confirmAction } from "../composables/useConfirm";
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import {
  GitCommit,
  GitBranch,
  ShieldCheck,
  FileText,
  Search,
  RefreshCw,
  Tag,
  Copy,
  RotateCcw,
  GitCompare,
  AlertTriangle,
  X,
  ExternalLink,
  StickyNote,
  GitPullRequest,
  GitMerge,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import Toast from "./ui/Toast.vue";
import UiModal from "./ui/UiModal.vue";
import UiButton from "./ui/UiButton.vue";
import { useToast } from "../composables/useToast";
import { useModalA11y } from "../composables/useModalA11y";
import HistoryFilterBar from "./history/HistoryFilterBar.vue";
import BatchActionToolbar from "./history/BatchActionToolbar.vue";
import CommitInspector from "./history/CommitInspector.vue";
import CommitGraphList from "./history/CommitGraphList.vue";
import type {
  CommitItem,
  CommitDetail,
  BranchInfo,
  GraphLine,
  RebaseTodoItem,
  BatchCherryPickResult,
  PickaxeResult,
  SignatureDetails,
  CommitNotesResult,
  CommitNoteItem,
  CommitTopologyStats,
} from "../types";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

/* ------------------------------------------------------------------ */
/* Layout constants — a fixed row height makes virtual scrolling exact */
/* ------------------------------------------------------------------ */
const ROW_HEIGHT = 56; // px
const OVERSCAN = 10; // extra rows rendered above / below the viewport
const PAGE_SIZE = 300; // unfiltered growing window step
const FILTER_PAGE_SIZE = 200; // server-side filtered page size
const LANE_WIDTH = 14; // px per DAG lane column
const MAX_VISIBLE_LANES = 24; // dense graph shows up to 24 lanes, then clamps + hints
const COMPACT_GUTTER_WIDTH = 20; // "简洁" mode gutter (coloured dots only)
const SHOW_GRAPH_STORAGE_KEY = "wgit.history.showGraph";
/** The Go backend emits `diffTruncated`; tolerate generated types that predate it. */
type CommitDetailWithPatch = CommitDetail & { diffTruncated?: boolean };
// Mirrors backend `LaneColors` so dots and connection strokes stay in sync.
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
/* History state                                                       */
/* ------------------------------------------------------------------ */
const commits = ref<CommitItem[]>([]);
const selectedCommit = ref<CommitItem | null>(null);
const commitDetail = ref<CommitDetailWithPatch | null>(null);
const isLoading = ref(false);
const isLoadingMore = ref(false);
const isLoadingDetail = ref(false);
const isLoadingBranches = ref(false);
const hasMore = ref(false);
const unfilteredLimit = ref(PAGE_SIZE);
const repoTotalCount = ref<number | null>(null); // from getCommitCount
const filteredTotalCount = ref<number | null>(null); // from getCommitsFiltered
const copiedHash = ref<string | null>(null);
const { toastText: actionMessage, showToast: actionToastVisible, notify: showToast } = useToast(2500);

/* Graph mode + patch preview state ---------------------------------- */
const showGraph = ref(readShowGraphPreference());

function readShowGraphPreference(): boolean {
  try {
    if (typeof localStorage === "undefined") return true;
    const raw = localStorage.getItem(SHOW_GRAPH_STORAGE_KEY);
    return raw === null ? true : raw !== "false";
  } catch {
    return true;
  }
}

function setShowGraph(enabled: boolean) {
  if (showGraph.value === enabled) return;
  showGraph.value = enabled;
  try {
    localStorage.setItem(SHOW_GRAPH_STORAGE_KEY, String(enabled));
  } catch {
    /* storage unavailable — keep the in-memory preference */
  }
}

/* Multi-select ------------------------------------------------------- */
const selectedHashes = ref<Set<string>>(new Set());
const lastSelectedIndex = ref<number | null>(null);
const selectionCount = computed(() => selectedHashes.value.size);
const selectedCommits = computed(() =>
  commits.value.filter((c) => selectedHashes.value.has(c.hash))
);
// The list is newest-first; batch cherry-pick has to apply oldest-first.
const selectedCommitsOldestFirst = computed(() => [...selectedCommits.value].reverse());

/* Branch scope ------------------------------------------------------- */
const branches = ref<BranchInfo[]>([]);
const branchScope = ref(""); // "" => 全部引用 (--all)
const localBranches = computed(() => {
  const locals = branches.value.filter((b) => !b.isRemote);
  return [...locals].sort((a, b) => {
    if (a.isCurrent !== b.isCurrent) return a.isCurrent ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
});

/* Server-side filters ------------------------------------------------- */
interface AppliedFilters {
  keyword: string;
  author: string;
  since: string;
  until: string;
}
const EMPTY_FILTERS: AppliedFilters = { keyword: "", author: "", since: "", until: "" };

const keywordFilter = ref("");
const authorFilter = ref("");
const sinceFilter = ref("");
const untilFilter = ref("");
const appliedFilters = ref<AppliedFilters>({ ...EMPTY_FILTERS });

const isFilterMode = computed(() => {
  const f = appliedFilters.value;
  return !!(f.keyword || f.author || f.since || f.until);
});
const hasActiveFilters = computed(
  () => !!(keywordFilter.value || authorFilter.value || sinceFilter.value || untilFilter.value)
);
const activeFilterSummary = computed(() => {
  const f = appliedFilters.value;
  const parts: string[] = [];
  if (f.keyword) parts.push(`关键词「${f.keyword}」`);
  if (f.author) parts.push(`作者「${f.author}」`);
  if (f.since) parts.push(`自 ${f.since}`);
  if (f.until) parts.push(`至 ${f.until}`);
  return parts.length > 0 ? `筛选：${parts.join(" · ")}` : "";
});

const graphListRef = ref<InstanceType<typeof CommitGraphList> | null>(null);
/* ------------------------------------------------------------------ */
/* Multi-select helpers (selection is keyed by hash, never by row)     */
/* ------------------------------------------------------------------ */
function isHashSelected(hash: string): boolean {
  return selectedHashes.value.has(hash);
}

function toggleSelectedHash(hash: string) {
  const next = new Set(selectedHashes.value);
  if (next.has(hash)) next.delete(hash);
  else next.add(hash);
  selectedHashes.value = next;
}

function clearSelection() {
  if (selectedHashes.value.size > 0) selectedHashes.value = new Set();
  lastSelectedIndex.value = null;
}

/** Adds every loaded commit between two list indices to the selection. */
function selectRange(fromIndex: number, toIndex: number) {
  const start = Math.min(fromIndex, toIndex);
  const end = Math.max(fromIndex, toIndex);
  const next = new Set(selectedHashes.value);
  for (let i = start; i <= end; i++) {
    const commit = commits.value[i];
    if (commit) next.add(commit.hash);
  }
  selectedHashes.value = next;
  lastSelectedIndex.value = toIndex;
}

function handleRowClick(commit: CommitItem, index: number, event: MouseEvent) {
  // Ctrl/Cmd+click toggles without moving the inspector selection.
  if (event.metaKey || event.ctrlKey) {
    event.preventDefault();
    toggleSelectedHash(commit.hash);
    lastSelectedIndex.value = index;
    return;
  }
  // Shift+click extends the selection over the displayed order.
  if (event.shiftKey) {
    event.preventDefault();
    if (lastSelectedIndex.value !== null) {
      selectRange(lastSelectedIndex.value, index);
    } else {
      toggleSelectedHash(commit.hash);
      lastSelectedIndex.value = index;
    }
    return;
  }
  // Plain click keeps the existing "inspect this commit" behaviour.
  clearSelection();
  lastSelectedIndex.value = index;
  void selectCommit(commit);
}

function handleCheckboxClick(commit: CommitItem, index: number, event: MouseEvent) {
  if (event.shiftKey && lastSelectedIndex.value !== null) {
    selectRange(lastSelectedIndex.value, index);
    return;
  }
  toggleSelectedHash(commit.hash);
  lastSelectedIndex.value = index;
}

/** Drops selected hashes that are no longer part of the loaded history. */
function pruneSelection() {
  if (selectedHashes.value.size === 0) return;
  const present = new Set(commits.value.map((c) => c.hash));
  let changed = false;
  const next = new Set<string>();
  for (const hash of selectedHashes.value) {
    if (present.has(hash)) next.add(hash);
    else changed = true;
  }
  if (changed) selectedHashes.value = next;
}




/* ------------------------------------------------------------------ */
/* Counts & labels                                                     */
/* ------------------------------------------------------------------ */
const maxLaneIndex = computed(() => {
  let max = 0;
  for (const commit of commits.value) {
    const own = Math.max(0, Math.floor(commit.lane || 0));
    if (own > max) max = own;
    for (const line of commit.lines ?? []) {
      max = Math.max(max, line.fromLane || 0, line.toLane || 0);
    }
  }
  return max;
});
const hiddenLaneCount = computed(() => Math.max(0, maxLaneIndex.value + 1 - MAX_VISIBLE_LANES));

const countLabel = computed(() => {
  const loaded = commits.value.length;
  if (isLoading.value && loaded === 0) {
    return isFilterMode.value ? "正在筛选..." : "正在加载...";
  }
  if (isFilterMode.value) {
    const total = filteredTotalCount.value;
    if (total !== null && total > loaded) return `已加载 ${loaded} / 共 ${total} 条筛选结果`;
    return hasMore.value ? `已加载 ${loaded} 条筛选结果（可继续加载）` : `共 ${loaded} 条筛选结果`;
  }
  const total = repoTotalCount.value;
  if (total !== null && total > loaded) return `已加载 ${loaded} / 共 ${total} 条提交`;
  if (total !== null) return `共 ${Math.max(total, loaded)} 条提交`;
  return hasMore.value ? `已加载 ${loaded} 条提交（可继续加载）` : `共 ${loaded} 条提交`;
});

/* ------------------------------------------------------------------ */
/* Data loading                                                        */
/* ------------------------------------------------------------------ */
let loadSeq = 0;
let totalSeq = 0;
let detailSeq = 0;
let branchSeq = 0;
let filterTimer: ReturnType<typeof setTimeout> | null = null;
let lastAppliedKey = JSON.stringify(EMPTY_FILTERS);
let suppressBranchScopeReload = false;

async function loadTotalCount() {
  const repo = props.currentRepoPath;
  if (!repo) {
    repoTotalCount.value = null;
    return;
  }
  const branch = branchScope.value;
  const seq = ++totalSeq;
  try {
    const count = await gitService.getCommitCount(repo, branch);
    if (seq !== totalSeq) return;
    repoTotalCount.value =
      typeof count === "number" && Number.isFinite(count) && count >= 0 ? count : null;
  } catch (err) {
    console.debug("GetCommitCount unavailable, falling back to window heuristics:", err);
    if (seq === totalSeq) repoTotalCount.value = null;
  }
  syncUnfilteredHasMore();
}

function syncUnfilteredHasMore() {
  if (isFilterMode.value) return;
  const total = repoTotalCount.value;
  if (total !== null) hasMore.value = commits.value.length < total;
}

async function loadBranches() {
  const repo = props.currentRepoPath;
  if (!repo) {
    branches.value = [];
    return;
  }
  const seq = ++branchSeq;
  isLoadingBranches.value = true;
  try {
    const list = await gitService.getBranches(repo);
    if (seq !== branchSeq || props.currentRepoPath !== repo) return;
    branches.value = Array.isArray(list) ? list : [];
    if (
      branchScope.value &&
      !branches.value.some((b) => !b.isRemote && b.name === branchScope.value)
    ) {
      branchScope.value = ""; // selected branch disappeared — fall back to --all
    }
  } catch (err) {
    if (seq !== branchSeq || props.currentRepoPath !== repo) return;
    console.error("Failed to load branches:", err);
    branches.value = [];
  } finally {
    if (seq === branchSeq) isLoadingBranches.value = false;
  }
}

function appendCommits(page: CommitItem[]) {
  const existing = new Set(commits.value.map((c) => c.hash));
  const fresh = page.filter((c) => !existing.has(c.hash));
  if (fresh.length > 0) commits.value = [...commits.value, ...fresh];
}

function relinkSelection() {
  const current = selectedCommit.value;
  if (!current) return;
  const match = commits.value.find((c) => c.hash === current.hash);
  if (match) selectedCommit.value = match;
}

function afterListReplaced(resetSelection: boolean) {
  if (resetSelection) clearSelection();
  else pruneSelection();
  if (!resetSelection) {
    const current = selectedCommit.value;
    if (current) {
      const match = commits.value.find((c) => c.hash === current.hash);
      if (match) {
        selectedCommit.value = match;
        return;
      }
    }
  }
  graphListRef.value?.resetScroll();
  const first = commits.value[0];
  if (first) {
    void selectCommit(first);
  } else {
    selectedCommit.value = null;
    commitDetail.value = null;
  }
}

async function loadCommits(resetSelection = true) {
  const repo = props.currentRepoPath;
  if (!repo) {
    commits.value = [];
    hasMore.value = false;
    return;
  }
  const seq = ++loadSeq;
  isLoading.value = true;
  const filtered = isFilterMode.value;
  try {
    if (filtered) {
      const res = await gitService.getCommitsFiltered(
        repo,
        branchScope.value,
        appliedFilters.value.author,
        appliedFilters.value.since,
        appliedFilters.value.until,
        appliedFilters.value.keyword,
        0,
        FILTER_PAGE_SIZE
      );
      if (seq !== loadSeq) return;
      const list = res?.commits ?? [];
      commits.value = list;
      filteredTotalCount.value =
        typeof res?.totalCount === "number" && res.totalCount >= 0 ? res.totalCount : null;
      hasMore.value = res ? !!res.hasMore : list.length >= FILTER_PAGE_SIZE;
      if (list.length === 0) hasMore.value = false;
    } else {
      unfilteredLimit.value = PAGE_SIZE;
      filteredTotalCount.value = null;
      const list = (await gitService.getCommits(repo, unfilteredLimit.value, branchScope.value)) ?? [];
      if (seq !== loadSeq) return;
      commits.value = list;
      const total = repoTotalCount.value;
      hasMore.value = total !== null ? list.length < total : list.length >= unfilteredLimit.value;
    }
    if (seq !== loadSeq) return;
    afterListReplaced(resetSelection);
    void refreshNotedHashes();
  } catch (err) {
    if (seq === loadSeq) {
      console.error("Failed to load commits:", err);
      commits.value = [];
      hasMore.value = false;
      afterListReplaced(resetSelection);
    }
  } finally {
    if (seq === loadSeq) isLoading.value = false;
  }
}

async function loadMore() {
  const repo = props.currentRepoPath;
  if (!repo || !hasMore.value || isLoading.value || isLoadingMore.value) return;
  // Snapshot the request scope so a concurrent branch/filter change cannot
  // append stale rows into the freshly loaded list.
  const seq = loadSeq;
  const filtered = isFilterMode.value;
  const branch = branchScope.value;
  const filters = { ...appliedFilters.value };
  isLoadingMore.value = true;
  try {
    if (filtered) {
      const res = await gitService.getCommitsFiltered(
        repo,
        branch,
        filters.author,
        filters.since,
        filters.until,
        filters.keyword,
        commits.value.length,
        FILTER_PAGE_SIZE
      );
      if (seq !== loadSeq) return;
      const page = res?.commits ?? [];
      appendCommits(page);
      if (typeof res?.totalCount === "number" && res.totalCount >= 0) {
        filteredTotalCount.value = res.totalCount;
      }
      hasMore.value = res ? !!res.hasMore : page.length >= FILTER_PAGE_SIZE;
      if (page.length === 0) hasMore.value = false;
    } else {
      const nextLimit = unfilteredLimit.value + PAGE_SIZE;
      const list = (await gitService.getCommits(repo, nextLimit, branch)) ?? [];
      if (seq !== loadSeq) return;
      unfilteredLimit.value = nextLimit;
      commits.value = list;
      relinkSelection();
      const total = repoTotalCount.value;
      hasMore.value = total !== null ? list.length < total : list.length >= nextLimit;
      if (list.length === 0) hasMore.value = false;
    }
    pruneSelection();
  } catch (err) {
    console.error("Failed to load more commits:", err);
  } finally {
    isLoadingMore.value = false;
  }
}

const topologyStats = ref<CommitTopologyStats | null>(null);

async function loadTopologyStats() {
  if (!props.currentRepoPath) return;
  try {
    topologyStats.value = await gitService.getCommitTopologyStats(props.currentRepoPath);
  } catch {
    topologyStats.value = null;
  }
}

async function refreshHistory(keepSelection: boolean) {
  await Promise.all([loadTotalCount(), loadCommits(!keepSelection), loadTopologyStats()]);
}

/* ------------------------------------------------------------------ */
/* Debounced server-side filtering                                     */
/* ------------------------------------------------------------------ */
function scheduleFilterReload() {
  if (filterTimer) clearTimeout(filterTimer);
  filterTimer = setTimeout(() => {
    filterTimer = null;
    const next: AppliedFilters = {
      keyword: keywordFilter.value.trim(),
      author: authorFilter.value.trim(),
      since: sinceFilter.value,
      until: untilFilter.value,
    };
    const key = JSON.stringify(next);
    if (key === lastAppliedKey) return;
    lastAppliedKey = key;
    appliedFilters.value = next;
    void loadCommits(true);
  }, 300);
}

function clearFilters() {
  if (filterTimer) {
    clearTimeout(filterTimer);
    filterTimer = null;
  }
  keywordFilter.value = "";
  authorFilter.value = "";
  sinceFilter.value = "";
  untilFilter.value = "";
  appliedFilters.value = { ...EMPTY_FILTERS };
  lastAppliedKey = JSON.stringify(EMPTY_FILTERS);
  void loadCommits(true);
}

async function handleExportReport() {
  if (!props.currentRepoPath) return;
  try {
    const markdown = await gitService.exportCommitReport(
      props.currentRepoPath,
      sinceFilter.value,
      untilFilter.value,
      "markdown"
    );
    if (!markdown) {
      showToast("没有可导出的提交记录");
      return;
    }
    try {
      await navigator.clipboard.writeText(markdown);
    } catch {
      // Fallback for WebViews that do not expose the clipboard API.
      const textarea = document.createElement("textarea");
      textarea.value = markdown;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      const ok = document.execCommand("copy");
      textarea.remove();
      if (!ok) throw new Error("无法访问剪贴板");
    }
    showToast("提交报告已复制到剪贴板");
  } catch (err: any) {
    showToast(`导出失败: ${err?.message || err}`);
  }
}

/* ------------------------------------------------------------------ */
/* Pickaxe content archaeology                                         */
/* ------------------------------------------------------------------ */
const showPickaxeModal = ref(false);
const pickaxeQuery = ref("");
const pickaxeMode = ref<"content" | "regex">("content");
const pickaxeAuthor = ref("");
const pickaxePathFilter = ref("");
const pickaxeIgnoreCase = ref(false);
const pickaxeResult = ref<PickaxeResult | null>(null);
const isPickaxeSearching = ref(false);

async function handlePickaxeSearch() {
  if (!props.currentRepoPath || !pickaxeQuery.value.trim() || isPickaxeSearching.value) return;
  isPickaxeSearching.value = true;
  try {
    pickaxeResult.value = await gitService.pickaxeSearch(
      props.currentRepoPath,
      pickaxeQuery.value.trim(),
      pickaxeMode.value,
      pickaxeAuthor.value.trim(),
      pickaxePathFilter.value.trim(),
      30,
      pickaxeIgnoreCase.value
    );
    if (pickaxeResult.value && pickaxeResult.value.totalHits === 0) {
      showToast("没有找到包含该内容变更的提交");
    }
  } catch (err: any) {
    showToast(`内容考古失败: ${err?.message || err}`);
  } finally {
    isPickaxeSearching.value = false;
  }
}

async function copyPickaxeHash(hash: string) {
  try {
    await navigator.clipboard.writeText(hash);
  } catch {
    const textarea = document.createElement("textarea");
    textarea.value = hash;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand("copy");
    textarea.remove();
  }
  showToast(`已复制提交 ${hash.substring(0, 8)}`);
}

/* ------------------------------------------------------------------ */
/* Commit signature details                                            */
/* ------------------------------------------------------------------ */
const showSignatureModal = ref(false);
const signatureDetails = ref<SignatureDetails | null>(null);
const isLoadingSignature = ref(false);

async function handleShowSignatureDetails(hash: string) {
  if (!props.currentRepoPath) return;
  showSignatureModal.value = true;
  signatureDetails.value = null;
  isLoadingSignature.value = true;
  try {
    signatureDetails.value = await gitService.getCommitSignatureDetails(props.currentRepoPath, hash);
  } catch (err: any) {
    showSignatureModal.value = false;
    showToast(`读取签名详情失败: ${err?.message || err}`);
  } finally {
    isLoadingSignature.value = false;
  }
}

function signatureStatusMeta(status: string) {
  switch (status) {
    case "G":
      return { label: "签名有效 (Good)", cls: "bg-success/15 text-success" };
    case "B":
      return { label: "签名无效 (Bad)", cls: "bg-danger/15 text-danger" };
    case "U":
      return { label: "密钥不可信 (Untrusted)", cls: "bg-warning/15 text-warning" };
    default:
      return { label: "未签名 (No Signature)", cls: "bg-surface border border-line text-ink-muted" };
  }
}

/* ------------------------------------------------------------------ */
/* Git Notes overview                                                  */
/* ------------------------------------------------------------------ */
const showNotesOverview = ref(false);
const notesOverview = ref<CommitNotesResult | null>(null);
const isLoadingNotesOverview = ref(false);

async function openNotesOverview() {
  showNotesOverview.value = true;
  if (!props.currentRepoPath) return;
  isLoadingNotesOverview.value = true;
  try {
    notesOverview.value = await gitService.getCommitNotes(props.currentRepoPath, 100);
  } catch (err: any) {
    showToast(`读取批注总览失败: ${err?.message || err}`);
  } finally {
    isLoadingNotesOverview.value = false;
  }
}

function jumpToNote(note: CommitNoteItem) {
  const match = commits.value.find((c) => c.hash === note.commitHash);
  if (match) {
    selectCommit(match);
    showNotesOverview.value = false;
  } else {
    void copyHash(note.commitHash);
    showToast("该提交不在当前列表，已复制 Hash");
  }
}

watch([keywordFilter, authorFilter, sinceFilter, untilFilter], scheduleFilterReload);

/* ------------------------------------------------------------------ */
/* Dialogs & actions (preserved from the original view)                */
/* ------------------------------------------------------------------ */
const showNewBranchDialog = ref(false);
const showNewTagDialog = ref(false);
const showResetDialog = ref(false);
const newBranchName = ref("");
const newTagName = ref("");
const resetMode = ref<"soft" | "mixed" | "hard">("mixed");

/* Batch cherry-pick dialog ------------------------------------------- */
const showBatchCherryPickDialog = ref(false);
const showRangeCherryPickDialog = ref(false);
const rangeStartRef = ref("");
const rangeEndRef = ref("");
const isRangeCherryPicking = ref(false);

async function handleRangeCherryPick() {
  if (!props.currentRepoPath || !rangeStartRef.value.trim() || !rangeEndRef.value.trim()) return;
  if (isRangeCherryPicking.value) return;
  if (
    !(await confirmAction(
      `确定拣选 ${rangeStartRef.value.trim()}..${rangeEndRef.value.trim()} 范围内的提交吗？\n\n使用 git cherry-pick A..B，起点提交本身不会被包含。`
    ))
  )
    return;
  isRangeCherryPicking.value = true;
  try {
    const out = await gitService.cherryPickRange(
      props.currentRepoPath,
      rangeStartRef.value.trim(),
      rangeEndRef.value.trim()
    );
    showToast(out ? `范围拣选完成: ${out.slice(0, 80)}` : "范围拣选完成");
    showRangeCherryPickDialog.value = false;
    rangeStartRef.value = "";
    rangeEndRef.value = "";
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(`范围拣选失败: ${err?.message || err}`);
  } finally {
    isRangeCherryPicking.value = false;
  }
}
const isBatchCherryPicking = ref(false);
const batchCherryPickResult = ref<BatchCherryPickResult | null>(null);
const batchCherryPickError = ref<string | null>(null);

/* Interactive rebase dialog ------------------------------------------ */
const showRebaseDialog = ref(false);
const rebaseBase = ref("");
const rebaseBaseOverride = ref("");
const rebasePlan = ref<RebaseTodoItem[]>([]);
const isLoadingRebasePlan = ref(false);
const isExecutingRebase = ref(false);
const rebasePlanError = ref<string | null>(null);
const rebaseOutput = ref<string | null>(null);
const rebaseError = ref<string | null>(null);
let rebasePlanSeq = 0;

const REBASE_ACTIONS = [
  { value: "pick", label: "pick（保留该提交）" },
  { value: "reword", label: "reword（改写提交说明）" },
  { value: "edit", label: "edit（提交时暂停编辑）" },
  { value: "squash", label: "squash（合并到上一条）" },
  { value: "fixup", label: "fixup（合并并丢弃说明）" },
  { value: "drop", label: "drop（丢弃该提交）" },
] as const;

interface RebaseBaseOption {
  value: string;
  label: string;
}

const currentBranchInfo = computed(
  () => branches.value.find((b) => !b.isRemote && b.isCurrent) ?? null
);
const oldestSelectedCommit = computed(() => selectedCommitsOldestFirst.value[0] ?? null);
const rebaseBaseOptions = computed<RebaseBaseOption[]>(() => {
  const options: RebaseBaseOption[] = [];
  const seen = new Set<string>();
  const add = (value: string, label: string) => {
    const trimmed = (value || "").trim();
    if (!trimmed || seen.has(trimmed)) return;
    seen.add(trimmed);
    options.push({ value: trimmed, label });
  };
  // Most precise base: parent of the oldest-selected commit.
  const oldest = oldestSelectedCommit.value;
  const oldestParent = oldest?.parents?.[0];
  if (oldestParent) add(oldestParent, `所选提交的父提交 (${oldestParent.slice(0, 7)})`);
  // Explicit base passed via the "wgit:history-rebase" window event.
  const override = rebaseBaseOverride.value.trim();
  if (override) add(override, `指定基准 (${override.slice(0, 7)})`);
  const current = currentBranchInfo.value;
  if (current?.upstream) add(current.upstream, `当前分支上游 (${current.upstream})`);
  for (const branch of localBranches.value) {
    add(branch.name, branch.isCurrent ? `${branch.name}（当前分支）` : branch.name);
  }
  return options;
});

function isDialogOpen(): boolean {
  return (
    showNewBranchDialog.value ||
    showNewTagDialog.value ||
    showResetDialog.value ||
    showBatchCherryPickDialog.value ||
    showRangeCherryPickDialog.value ||
    showRebaseDialog.value ||
    showNoteModal.value ||
    showSignatureModal.value ||
    showNotesOverview.value ||
    showPickaxeModal.value
  );
}

// Git Notes state
const notedCommitHashesSet = ref<Set<string>>(new Set());
const currentCommitNote = ref<string>("");
const showNoteModal = ref<boolean>(false);
const noteInputText = ref<string>("");
const isSavingNote = ref<boolean>(false);
const isPushingNotes = ref<boolean>(false);

const { containerRef: rangeCherryPickDialogRef } = useModalA11y(
  () => showRangeCherryPickDialog.value,
  () => { if (!isRangeCherryPicking.value) showRangeCherryPickDialog.value = false; }
);
const { containerRef: batchCherryPickDialogRef } = useModalA11y(
  () => showBatchCherryPickDialog.value,
  () => closeBatchCherryPickDialog()
);
const { containerRef: rebaseDialogRef } = useModalA11y(
  () => showRebaseDialog.value,
  () => closeRebaseDialog()
);
const { containerRef: noteDialogRef } = useModalA11y(
  () => showNoteModal.value,
  () => { showNoteModal.value = false; }
);
const { containerRef: signatureDialogRef } = useModalA11y(
  () => showSignatureModal.value,
  () => { showSignatureModal.value = false; }
);
const { containerRef: notesOverviewDialogRef } = useModalA11y(
  () => showNotesOverview.value,
  () => { showNotesOverview.value = false; }
);
const { containerRef: pickaxeDialogRef } = useModalA11y(
  () => showPickaxeModal.value,
  () => { showPickaxeModal.value = false; }
);

async function refreshNotedHashes() {
  if (!props.currentRepoPath) return;
  try {
    const hashes = await gitService.getNotedCommitHashes(props.currentRepoPath);
    notedCommitHashesSet.value = new Set(hashes || []);
  } catch {}
}

function openEditNoteModal() {
  noteInputText.value = currentCommitNote.value;
  showNoteModal.value = true;
}

async function handleSaveNote() {
  if (!props.currentRepoPath || !selectedCommit.value) return;
  isSavingNote.value = true;
  try {
    await gitService.saveCommitNote(props.currentRepoPath, selectedCommit.value.hash, noteInputText.value);
    currentCommitNote.value = noteInputText.value;
    notedCommitHashesSet.value.add(selectedCommit.value.hash);
    showNoteModal.value = false;
    showToast("已成功保存提交批注 (Git Notes)");
  } catch (err: any) {
    showToast(`保存批注失败: ${err?.message || err}`);
  } finally {
    isSavingNote.value = false;
  }
}

async function handleDeleteNote() {
  if (!props.currentRepoPath || !selectedCommit.value) return;
  if (!(await confirmAction({ message: "确定要删除该提交的 Git Notes 批注吗？", tone: "danger" }))) return;
  try {
    await gitService.deleteCommitNote(props.currentRepoPath, selectedCommit.value.hash);
    currentCommitNote.value = "";
    notedCommitHashesSet.value.delete(selectedCommit.value.hash);
    showToast("已成功删除批注");
  } catch (err: any) {
    showToast(`删除批注失败: ${err?.message || err}`);
  }
}

async function handlePushNotes() {
  if (!props.currentRepoPath) return;
  isPushingNotes.value = true;
  try {
    const msg = await gitService.pushCommitNotes(props.currentRepoPath);
    showToast(msg || "已将提交批注推送至远端");
  } catch (err: any) {
    showToast(`推送批注失败: ${err?.message || err}`);
  } finally {
    isPushingNotes.value = false;
  }
}

async function selectCommit(c: CommitItem) {
  selectedCommit.value = c;
  const repo = props.currentRepoPath;
  if (!repo) return;
  const seq = ++detailSeq;
  isLoadingDetail.value = true;
  commitDetail.value = null;
  currentCommitNote.value = "";
  // Patch preview state belongs to the previously inspected commit.
  try {
    const detail = await gitService.getCommitDetail(repo, c.hash);
    if (seq === detailSeq) commitDetail.value = detail;
  } catch (err) {
    console.error("Failed to load commit detail:", err);
    if (seq === detailSeq) commitDetail.value = null;
  } finally {
    if (seq === detailSeq) isLoadingDetail.value = false;
  }

  // Load note asynchronously
  gitService.getCommitNote(repo, c.hash).then((note) => {
    if (seq === detailSeq) currentCommitNote.value = note || "";
  }).catch(() => {});
}

function copyHash(hash: string) {
  navigator.clipboard.writeText(hash);
  copiedHash.value = hash;
  setTimeout(() => {
    copiedHash.value = null;
  }, 1500);
}

function copyText(text: string, label = "已复制到剪贴板") {
  navigator.clipboard.writeText(text);
  showToast(label);
}

function handleRebaseOntoCommit(commit?: CommitItem) {
  const target = commit || selectedCommit.value;
  if (!target) return;
  window.dispatchEvent(
    new CustomEvent("wgit:open-rebase", {
      detail: { target: target.hash },
    })
  );
}

const showContextMenu = ref(false);
const contextMenuPos = ref({ x: 0, y: 0 });
const contextMenuCommit = ref<CommitItem | null>(null);

function handleRowContextMenu(commit: CommitItem, e: MouseEvent) {
  e.preventDefault();
  contextMenuCommit.value = commit;
  void selectCommit(commit);
  const menuWidth = 240;
  const menuHeight = 360;
  const x = Math.min(e.clientX, window.innerWidth - menuWidth - 16);
  const y = Math.min(e.clientY, window.innerHeight - menuHeight - 16);
  contextMenuPos.value = { x: Math.max(8, x), y: Math.max(8, y) };
  showContextMenu.value = true;
}

function closeContextMenu() {
  showContextMenu.value = false;
}


// Cherry-pick commit
async function handleCherryPick() {
  if (!selectedCommit.value || !props.currentRepoPath) return;
  try {
    await gitService.cherryPick(props.currentRepoPath, selectedCommit.value.hash);
    showToast(`已成功 Cherry-Pick 提交 ${selectedCommit.value.shortHash}`);
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(`Cherry-pick 失败: ${err?.message || err}`);
  }
}

async function handleOpenCommitInWeb(hash: string) {
  if (!props.currentRepoPath || !hash) return;
  try {
    await gitService.openCommitInWeb(props.currentRepoPath, hash);
  } catch (err: any) {
    showToast(`在网页端打开提交失败: ${err?.message || err}`);
  }
}

// Revert commit
async function handleRevert() {
  if (!selectedCommit.value || !props.currentRepoPath) return;
  if (!(await confirmAction(`确定要回滚提交 ${selectedCommit.value.shortHash} 吗？`))) return;
  try {
    await gitService.revertCommit(props.currentRepoPath, selectedCommit.value.hash);
    showToast(`已成功回滚提交 ${selectedCommit.value.shortHash}`);
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(`回滚提交失败: ${err?.message || err}`);
  }
}

async function handleRevertNoCommit() {
  if (!selectedCommit.value || !props.currentRepoPath) return;
  if (!(await confirmAction(`确定将提交 ${selectedCommit.value.shortHash} 回滚到工作区（不自动提交）吗？`))) return;
  try {
    await gitService.revertNoCommit(props.currentRepoPath, selectedCommit.value.hash);
    showToast(`已回滚 ${selectedCommit.value.shortHash} 到工作区，未创建提交`);
    emit("refresh");
  } catch (err: any) {
    showToast(`回滚到工作区失败: ${err?.message || err}`);
  }
}

// Turn the current staged changes into a `fixup!` commit pointing at the
// selected commit. Autosquash during interactive rebase folds it back in,
// which is the cleanest way to polish history without rewriting by hand.
async function handleCreateFixup() {
  if (!selectedCommit.value || !props.currentRepoPath) return;
  const target = selectedCommit.value;
  if (
    !(await confirmAction(
      `把当前暂存区改动整理成 fixup! 提交，指向 ${target.shortHash}「${target.subject}」？\n\n` +
        `· 需要先暂存要合并进去的改动\n` +
        `· 之后在交互式变基时勾选 autosquash，会自动折叠进目标提交`
    ))
  )
    return;
  try {
    const res = await gitService.createFixupCommit(props.currentRepoPath, target.hash, "fixup");
    showToast(res.message);
    emit("refresh");
  } catch (err: any) {
    showToast(`创建 fixup 提交失败: ${err?.message || err}`);
  }
}

async function handleCheckoutDetached() {  if (!selectedCommit.value || !props.currentRepoPath) return;
  if (
    !(await confirmAction(
      `确定分离 HEAD 检出提交 ${selectedCommit.value.shortHash} 吗？当前分支将保持不变，但工作区会切换到此提交。`
    ))
  )
    return;
  try {
    await gitService.checkoutCommitDetached(props.currentRepoPath, selectedCommit.value.hash);
    showToast(`已分离检出提交 ${selectedCommit.value.shortHash}`);
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(`分离检出失败: ${err?.message || err}`);
  }
}

// Create branch from this commit
async function handleCreateBranch() {
  if (!selectedCommit.value || !props.currentRepoPath || !newBranchName.value.trim()) return;
  try {
    await gitService.createBranch(
      props.currentRepoPath,
      newBranchName.value.trim(),
      selectedCommit.value.hash
    );
    showToast(`已基于 ${selectedCommit.value.shortHash} 创建新分支 ${newBranchName.value.trim()}`);
    showNewBranchDialog.value = false;
    newBranchName.value = "";
    emit("refresh");
    void loadBranches();
  } catch (err: any) {
    showToast(`创建分支失败: ${err?.message || err}`);
  }
}

// Create tag from this commit
async function handleCreateTag() {
  if (!selectedCommit.value || !props.currentRepoPath || !newTagName.value.trim()) return;
  try {
    await gitService.createTag(
      props.currentRepoPath,
      newTagName.value.trim(),
      selectedCommit.value.hash
    );
    showToast(`已成功打上标签 ${newTagName.value.trim()}`);
    showNewTagDialog.value = false;
    newTagName.value = "";
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(`打标签失败: ${err?.message || err}`);
  }
}

// Reset branch to this commit
async function handleResetBranch() {
  if (!selectedCommit.value || !props.currentRepoPath) return;
  if (
    resetMode.value === "hard" &&
    !(await confirmAction({ message: `警告: Hard 重置将完全丢弃该提交之后的所有本地改动！确定继续吗？`, tone: "danger" }))
  ) {
    return;
  }
  try {
    await gitService.resetBranch(
      props.currentRepoPath,
      selectedCommit.value.hash,
      resetMode.value
    );
    showToast(`已执行 git reset --${resetMode.value} 到 ${selectedCommit.value.shortHash}`);
    showResetDialog.value = false;
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(`分支重置失败: ${err?.message || err}`);
  }
}

/* ------------------------------------------------------------------ */
/* Batch cherry-pick (multi-select toolbar)                            */
/* ------------------------------------------------------------------ */
function resetBatchCherryPickState() {
  batchCherryPickResult.value = null;
  batchCherryPickError.value = null;
}

async function handleCopyBatchDetails() {
  const repo = props.currentRepoPath;
  const commits = selectedCommitsOldestFirst.value;
  if (!repo || commits.length === 0) return;
  try {
    const details = await gitService.getCommitDetailsBatch(
      repo,
      commits.map((commit) => commit.hash)
    );
    const text = details
      .map(
        (commit) =>
          `${commit.hash}\n${commit.authorName} <${commit.authorEmail}> ${commit.date}\n\n${commit.subject}${commit.body ? `\n\n${commit.body.trim()}` : ""}`
      )
      .join("\n\n---\n\n");
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    }
    showToast(`已复制 ${details.length} 条提交详情`);
  } catch (err: any) {
    showToast(`复制提交详情失败: ${err?.message || err}`);
  }
}

function openBatchCherryPickDialog() {
  if (selectionCount.value === 0) return;
  resetBatchCherryPickState();
  showBatchCherryPickDialog.value = true;
}

function closeBatchCherryPickDialog() {
  if (isBatchCherryPicking.value) return;
  showBatchCherryPickDialog.value = false;
  resetBatchCherryPickState();
}

async function handleBatchCherryPick() {
  const repo = props.currentRepoPath;
  const commitsToApply = selectedCommitsOldestFirst.value;
  if (!repo || commitsToApply.length === 0 || isBatchCherryPicking.value) return;
  const hashes = commitsToApply.map((c) => c.hash);
  isBatchCherryPicking.value = true;
  batchCherryPickResult.value = null;
  batchCherryPickError.value = null;
  let attempted = false;
  try {
    const result = await gitService.cherryPickMultiple(repo, hashes);
    if (!result) {
      batchCherryPickError.value = "后端未返回批量 Cherry-Pick 结果，请检查仓库状态后重试。";
    } else {
      batchCherryPickResult.value = result;
      attempted = true;
      if (result.hasConflict || result.successCount < result.totalCount) {
        showToast(`批量 Cherry-Pick 中断：${result.successCount}/${result.totalCount} 条成功`);
      } else {
        showToast(`已成功应用 ${result.successCount}/${result.totalCount} 条提交`);
      }
    }
  } catch (err: any) {
    batchCherryPickError.value = err?.message || String(err);
  } finally {
    isBatchCherryPicking.value = false;
  }
  if (attempted) {
    emit("refresh");
    clearSelection();
    await refreshHistory(true);
    void loadBranches();
  }
}

/* ------------------------------------------------------------------ */
/* Interactive rebase planner                                          */
/* ------------------------------------------------------------------ */
const REBASE_ACTION_ALIASES: Record<string, string> = {
  p: "pick",
  r: "reword",
  e: "edit",
  s: "squash",
  f: "fixup",
  d: "drop",
};

function normalizeRebaseAction(action: string): string {
  const value = (action || "").trim().toLowerCase();
  const normalized = REBASE_ACTION_ALIASES[value] ?? value;
  return REBASE_ACTIONS.some((option) => option.value === normalized) ? normalized : "pick";
}

function openRebaseDialog(baseOverride?: string) {
  if (!props.currentRepoPath) return;
  showBatchCherryPickDialog.value = false;
  showNewBranchDialog.value = false;
  showNewTagDialog.value = false;
  showResetDialog.value = false;
  rebaseBaseOverride.value = (baseOverride || "").trim();
  showRebaseDialog.value = true;
  rebaseOutput.value = null;
  rebaseError.value = null;
  rebasePlanError.value = null;
  rebasePlan.value = [];
  const options = rebaseBaseOptions.value;
  const preferred =
    rebaseBaseOverride.value && options.some((o) => o.value === rebaseBaseOverride.value)
      ? rebaseBaseOverride.value
      : options[0]?.value ?? "";
  const previousBase = rebaseBase.value;
  rebaseBase.value = preferred;
  if (previousBase === preferred) void loadRebasePlan();
}

function closeRebaseDialog() {
  if (isExecutingRebase.value) return;
  showRebaseDialog.value = false;
  rebasePlanSeq++;
}

async function loadRebasePlan() {
  const repo = props.currentRepoPath;
  if (!repo) return;
  if (!rebaseBase.value) {
    rebasePlan.value = [];
    rebasePlanError.value = "暂无可用的变基基准，请先选择分支或上游。";
    return;
  }
  const seq = ++rebasePlanSeq;
  isLoadingRebasePlan.value = true;
  rebasePlanError.value = null;
  rebaseOutput.value = null;
  rebaseError.value = null;
  rebasePlan.value = [];
  try {
    const plan = await gitService.interactiveRebasePlan(repo, rebaseBase.value);
    if (seq !== rebasePlanSeq) return;
    rebasePlan.value = (Array.isArray(plan) ? plan : []).map((item) => ({
      action: normalizeRebaseAction(item.action),
      hash: item.hash,
      shortHash: item.shortHash || item.hash.slice(0, 7),
      subject: item.subject,
      author: item.author,
    }));
  } catch (err: any) {
    if (seq !== rebasePlanSeq) return;
    rebasePlan.value = [];
    rebasePlanError.value = err?.message || String(err);
  } finally {
    if (seq === rebasePlanSeq) isLoadingRebasePlan.value = false;
  }
}

async function executeRebase() {
  const repo = props.currentRepoPath;
  if (!repo || !rebaseBase.value || rebasePlan.value.length === 0 || isExecutingRebase.value) return;
  const confirmed = await confirmAction(
    `交互式变基将改写当前分支的历史（基准 ${rebaseBase.value}，共 ${rebasePlan.value.length} 条提交），并生成新的提交 Hash。确定继续吗？`
  );
  if (!confirmed) return;
  isExecutingRebase.value = true;
  rebaseError.value = null;
  rebaseOutput.value = null;
  let succeeded = false;
  try {
    const todos: RebaseTodoItem[] = rebasePlan.value.map((todo) => ({
      action: todo.action,
      hash: todo.hash,
      shortHash: todo.shortHash,
      subject: todo.subject,
      author: todo.author,
    }));
    const output = await gitService.interactiveRebaseExecute(repo, rebaseBase.value, todos);
    rebaseOutput.value =
      typeof output === "string" && output.trim() ? output : "交互式变基执行完成";
    console.debug("[interactive-rebase]", rebaseOutput.value);
    succeeded = true;
  } catch (err: any) {
    rebaseError.value = err?.message || String(err);
  } finally {
    isExecutingRebase.value = false;
  }
  if (succeeded) {
    const outputSummary =
      (rebaseOutput.value || "")
        .split("\n")
        .map((line) => line.trim())
        .find((line) => line.length > 0) ?? "";
    showToast(
      outputSummary ? `交互式变基完成：${outputSummary.slice(0, 80)}` : "交互式变基完成"
    );
    emit("refresh");
    showRebaseDialog.value = false;
    rebasePlanSeq++;
    clearSelection();
    await refreshHistory(true);
    void loadBranches();
  }
}

function handleHistoryRebaseEvent(event: Event) {
  if (!props.currentRepoPath) return;
  const detail = (event as CustomEvent<{ base?: string } | undefined>).detail;
  const base = detail && typeof detail.base === "string" ? detail.base.trim() : "";
  openRebaseDialog(base || undefined);
}

watch(rebaseBase, () => {
  if (showRebaseDialog.value) void loadRebasePlan();
});

watch(rebaseBaseOptions, (options) => {
  if (!showRebaseDialog.value) return;
  if (!options.some((option) => option.value === rebaseBase.value)) {
    rebaseBase.value = options[0]?.value ?? "";
  }
});

/* ------------------------------------------------------------------ */
/* Keyboard navigation (↑/↓ and j/k)                                   */
/* ------------------------------------------------------------------ */
function handleKeydown(e: KeyboardEvent) {
  // Esc has dialog priority, then clears the multi-selection.
  if (e.key === "Escape") {
    if (isDialogOpen()) {
      if (showRebaseDialog.value) closeRebaseDialog();
      else if (showBatchCherryPickDialog.value) closeBatchCherryPickDialog();
      else if (showResetDialog.value) showResetDialog.value = false;
      else if (showNewBranchDialog.value) showNewBranchDialog.value = false;
      else if (showNewTagDialog.value) showNewTagDialog.value = false;
      e.preventDefault();
      return;
    }
    if (selectionCount.value > 0) {
      clearSelection();
      e.preventDefault();
    }
    return;
  }
  if (isDialogOpen()) return;
  const target = e.target as HTMLElement | null;
  if (
    target &&
    (target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      target.tagName === "SELECT" ||
      target.isContentEditable)
  ) {
    return;
  }
  if (e.metaKey || e.ctrlKey || e.altKey) return;
  const list = commits.value;
  if (list.length === 0) return;
  const down = e.key === "ArrowDown" || e.key === "j" || e.key === "J";
  const up = e.key === "ArrowUp" || e.key === "k" || e.key === "K";
  if (!down && !up) return;
  e.preventDefault();

  const currentIndex = list.findIndex((c) => c.hash === selectedCommit.value?.hash);
  if (down) {
    if (currentIndex < 0) {
      const first = list[0];
      if (first) {
        void selectCommit(first);
        graphListRef.value?.scrollToIndex(0);
      }
      return;
    }
    if (currentIndex >= list.length - 1) {
      if (hasMore.value) void loadMore();
      return;
    }
    const next = list[currentIndex + 1];
    if (next) {
      void selectCommit(next);
      graphListRef.value?.scrollToIndex(currentIndex + 1);
    }
  } else {
    if (currentIndex <= 0) {
      if (currentIndex < 0) {
        const first = list[0];
        if (first) {
          void selectCommit(first);
          graphListRef.value?.scrollToIndex(0);
        }
      }
      return;
    }
    const prev = list[currentIndex - 1];
    if (prev) {
      void selectCommit(prev);
      graphListRef.value?.scrollToIndex(currentIndex - 1);
    }
  }
}

/* ------------------------------------------------------------------ */
/* Lifecycle & top-level watchers                                      */
/* ------------------------------------------------------------------ */
function resetRepoScopedState() {
  commits.value = [];
  selectedCommit.value = null;
  commitDetail.value = null;
  hasMore.value = false;
  repoTotalCount.value = null;
  filteredTotalCount.value = null;
  topologyStats.value = null;
  branches.value = [];
  unfilteredLimit.value = PAGE_SIZE;
  graphListRef.value?.resetScroll();
  clearSelection();
  // close dialogs that belong to the previous repository
  showRebaseDialog.value = false;
  showBatchCherryPickDialog.value = false;
  resetBatchCherryPickState();
  rebasePlan.value = [];
  rebasePlanError.value = null;
  rebaseOutput.value = null;
  rebaseError.value = null;
  rebasePlanSeq++;
  // invalidate any in-flight requests
  loadSeq++;
  totalSeq++;
  detailSeq++;
  branchSeq++;
  // reset filters without triggering a redundant debounce reload
  if (filterTimer) {
    clearTimeout(filterTimer);
    filterTimer = null;
  }
  keywordFilter.value = "";
  authorFilter.value = "";
  sinceFilter.value = "";
  untilFilter.value = "";
  appliedFilters.value = { ...EMPTY_FILTERS };
  lastAppliedKey = JSON.stringify(EMPTY_FILTERS);
  // reset branch scope (guard against the branch watcher double-loading)
  if (branchScope.value !== "") {
    suppressBranchScopeReload = true;
    branchScope.value = "";
  }
}

watch(branchScope, () => {
  if (suppressBranchScopeReload) {
    suppressBranchScopeReload = false;
    return;
  }
  if (!props.currentRepoPath) return;
  repoTotalCount.value = null;
  void loadTotalCount();
  void loadTopologyStats();
  void loadCommits(true);
});

watch(
  () => props.currentRepoPath,
  () => {
    resetRepoScopedState();
    if (!props.currentRepoPath) return;
    void loadBranches();
    void loadTotalCount();
    void loadTopologyStats();
    void loadCommits(true);
  },
  { immediate: true }
);

function handleHistoryNotesEvent() {
  void openNotesOverview();
}

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
  window.addEventListener("wgit:history-rebase", handleHistoryRebaseEvent);
  window.addEventListener("wgit:history-notes", handleHistoryNotesEvent);
  window.addEventListener("click", closeContextMenu);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleKeydown);
  window.removeEventListener("wgit:history-rebase", handleHistoryRebaseEvent);
  window.removeEventListener("wgit:history-notes", handleHistoryNotesEvent);
  window.removeEventListener("click", closeContextMenu);
  if (filterTimer) clearTimeout(filterTimer);
});
</script>

<template>
  <div class="flex-1 flex overflow-hidden relative select-none">
    <!-- Center: Commit DAG Timeline -->
    <div class="flex-1 flex flex-col bg-canvas border-r border-line overflow-hidden min-w-0">
      <!-- Timeline Header -->
      <div class="h-11 border-b border-line px-4 flex items-center justify-between gap-3 bg-surface/30 backdrop-blur-sm shrink-0">
        <div class="flex items-center gap-2 min-w-0 overflow-hidden">
          <GitCommit class="w-4 h-4 text-accent shrink-0" />
          <span class="text-xs font-semibold text-ink whitespace-nowrap">提交历史与图谱</span>
          <span class="text-[10px] text-ink-muted font-mono font-normal truncate">{{ countLabel }}</span>
          <span
            v-if="topologyStats && topologyStats.totalCommits > 0"
            class="px-1.5 py-0.5 rounded bg-surface border border-line text-[9px] text-ink-muted font-mono font-normal shrink-0"
            :title="`直达提交 ${topologyStats.directCommits}，合并提交 ${topologyStats.mergeCommits}`"
          >
            合并 {{ topologyStats.mergeCommits }} · {{ (topologyStats.mergeRatio * 100).toFixed(0) }}%
          </span>
          <span
            v-if="isFilterMode"
            class="px-1.5 py-0.5 rounded-md bg-accent/15 border border-accent/30 text-accent text-[10px] font-medium whitespace-nowrap shrink-0"
          >
            服务端筛选中
          </span>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <!-- Dense graph / compact dots toggle -->
          <div
            class="flex items-center rounded-md border border-line bg-canvas overflow-hidden shrink-0"
            title="切换 DAG 图谱显示（偏好会记住）"
          >
            <button
              @click="setShowGraph(true)"
              class="px-2 py-1 text-[10px] font-medium cursor-pointer transition whitespace-nowrap shrink-0"
              :class="showGraph ? 'bg-raised text-ink' : 'text-ink-muted hover:text-ink'"
              title="显示完整的车道连接图谱"
            >
              图谱
            </button>
            <button
              @click="setShowGraph(false)"
              class="px-2 py-1 text-[10px] font-medium cursor-pointer transition whitespace-nowrap shrink-0"
              :class="!showGraph ? 'bg-raised text-ink' : 'text-ink-muted hover:text-ink'"
              title="仅显示彩色提交圆点，适合超宽历史"
            >
              简洁
            </button>
          </div>
          <span
            v-if="showGraph && hiddenLaneCount > 0"
            class="text-[10px] text-warning font-mono whitespace-nowrap shrink-0"
            :title="`图谱最多显示 ${MAX_VISIBLE_LANES} 条车道，超出的 ${hiddenLaneCount} 条折叠到最右列（渐隐提示）`"
          >
            …+{{ hiddenLaneCount }} 车道
          </span>

          <!-- Branch scope selector -->
          <div class="relative">
            <GitBranch class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-1.5 pointer-events-none z-10" />
            <select
              v-model="branchScope"
              :disabled="isLoadingBranches"
              class="pl-7 pr-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink font-mono max-w-[140px] xl:max-w-[210px] focus:outline-none focus:border-accent cursor-pointer transition disabled:opacity-50"
              title="选择提交历史的引用范围（默认 --all 全部引用）"
            >
              <option value="">全部引用 (--all)</option>
              <option v-for="b in localBranches" :key="b.name" :value="b.name">
                {{ b.name }}{{ b.isCurrent ? " (当前)" : "" }}
              </option>
            </select>
          </div>

          <button
            @click="showRangeCherryPickDialog = true"
            class="flex items-center gap-1 px-2 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-ink transition cursor-pointer"
            title="按 start..end 范围批量拣选提交 (git cherry-pick A..B)"
          >
            <GitCompare class="w-3 h-3 text-accent" />
            <span class="hidden xl:inline">范围拣选</span>
          </button>
          <button aria-label="刷新提交记录"
            @click="refreshHistory(true)"
            :disabled="isLoading"
            class="p-1 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
            title="刷新提交记录"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
          </button>
        </div>
      </div>

      <HistoryFilterBar
        v-model:keyword="keywordFilter"
        v-model:author="authorFilter"
        v-model:since="sinceFilter"
        v-model:until="untilFilter"
        :has-active-filters="hasActiveFilters"
        :is-filter-mode="isFilterMode"
        :active-filter-summary="activeFilterSummary"
        @clear="clearFilters"
        @export-report="handleExportReport"
        @open-pickaxe="showPickaxeModal = true"
        @open-notes="openNotesOverview"
      />
      <BatchActionToolbar
        v-if="selectionCount > 0"
        :selection-count="selectionCount"
        @batch-cherry-pick="openBatchCherryPickDialog"
        @copy-details="handleCopyBatchDetails"
        @rebase="openRebaseDialog()"
        @clear="clearSelection"
      />
      <!-- Commit Graph Rows (virtualized) -->
      <CommitGraphList
        ref="graphListRef"
        :commits="commits"
        :selected-hash="selectedCommit?.hash ?? null"
        :selected-hashes="selectedHashes"
        :noted-hashes="notedCommitHashesSet"
        :is-loading="isLoading"
        :is-filter-mode="isFilterMode"
        :has-more="hasMore"
        :is-loading-more="isLoadingMore"
        :show-graph="showGraph"
        @select="handleRowClick"
        @checkbox="handleCheckboxClick"
        @context-menu="handleRowContextMenu"
        @load-more="loadMore"
        @clear-filters="clearFilters"
      />
    </div>

    <CommitInspector
      :selected-commit="selectedCommit"
      :copied-hash="copiedHash"
      :commit-detail="commitDetail"
      :is-loading-detail="isLoadingDetail"
      :current-commit-note="currentCommitNote"
      :is-pushing-notes="isPushingNotes"
      @copy-hash="copyHash"
      @rebase-onto="handleRebaseOntoCommit()"
      @rebase-dialog="openRebaseDialog"
      @cherry-pick="handleCherryPick"
      @create-fixup="handleCreateFixup"
      @revert="handleRevert"
      @revert-no-commit="handleRevertNoCommit"
      @checkout-detached="handleCheckoutDetached"
      @new-branch="showNewBranchDialog = true"
      @new-tag="showNewTagDialog = true"
      @reset="showResetDialog = true"
      @open-web="handleOpenCommitInWeb"
      @signature-details="handleShowSignatureDetails"
      @edit-note="openEditNoteModal"
      @delete-note="handleDeleteNote"
      @push-notes="handlePushNotes"
    />

    <Toast :text="actionMessage" :visible="actionToastVisible" />

    <!-- Modal: Create Branch from Commit -->
    <UiModal
      :open="showNewBranchDialog"
      label="基于提交创建新分支"
      width="max-w-sm"
      @update:open="showNewBranchDialog = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-accent" />
          <span>基于提交创建新分支</span>
        </h3>
      </template>

      <div class="space-y-3">
        <p class="text-xs text-ink-muted font-mono">
          起点提交: {{ selectedCommit?.shortHash }} ({{ selectedCommit?.subject }})
        </p>
        <input
          v-model="newBranchName"
          type="text"
          aria-label="新分支名称"
          placeholder="例如: feature/hotfix-issue..."
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
          @keydown.enter="handleCreateBranch"
        />
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showNewBranchDialog = false">取消</UiButton>
        <UiButton variant="primary" :disabled="!newBranchName.trim()" @click="handleCreateBranch">
          创建分支
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Create Tag from Commit -->
    <UiModal
      :open="showNewTagDialog"
      label="基于提交创建标签"
      width="max-w-sm"
      @update:open="showNewTagDialog = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Tag class="w-4 h-4 text-purple" />
          <span>基于提交创建标签</span>
        </h3>
      </template>

      <div class="space-y-3">
        <p class="text-xs text-ink-muted font-mono">
          目标提交: {{ selectedCommit?.shortHash }}
        </p>
        <input
          v-model="newTagName"
          type="text"
          aria-label="标签名"
          placeholder="例如: v2.0.1 或 release-candidate..."
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
          @keydown.enter="handleCreateTag"
        />
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showNewTagDialog = false">取消</UiButton>
        <UiButton variant="purple" :disabled="!newTagName.trim()" @click="handleCreateTag">
          创建标签
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Reset Branch -->
    <UiModal
      :open="showResetDialog"
      label="重置分支"
      width="max-w-md"
      @update:open="showResetDialog = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-danger flex items-center gap-2">
          <AlertTriangle class="w-4 h-4" />
          <span>重置当前分支到此提交</span>
        </h3>
      </template>

      <div class="space-y-3">
        <p class="text-xs text-ink-muted font-mono">
          目标提交: {{ selectedCommit?.shortHash }}
        </p>

        <!-- Reset Mode Selector -->
        <div class="flex flex-col gap-2">
          <label class="flex items-start gap-2 p-2.5 rounded-lg bg-canvas border border-line cursor-pointer">
            <input type="radio" value="soft" v-model="resetMode" class="mt-0.5 accent-accent" />
            <div class="flex flex-col text-xs">
              <span class="font-bold text-ink">--soft (保留所有改动在暂存区)</span>
              <span class="text-ink-muted text-[11px]">仅将 HEAD 回退，所有差异保留在 Staged 暂存区。</span>
            </div>
          </label>

          <label class="flex items-start gap-2 p-2.5 rounded-lg bg-canvas border border-line cursor-pointer">
            <input type="radio" value="mixed" v-model="resetMode" class="mt-0.5 accent-accent" />
            <div class="flex flex-col text-xs">
              <span class="font-bold text-ink">--mixed (保留改动在工作区，默认)</span>
              <span class="text-ink-muted text-[11px]">将 HEAD 回退并取消暂存，差异保留在未暂存区。</span>
            </div>
          </label>

          <label class="flex items-start gap-2 p-2.5 rounded-lg bg-canvas border border-danger/40 cursor-pointer">
            <input type="radio" value="hard" v-model="resetMode" class="mt-0.5 accent-danger" />
            <div class="flex flex-col text-xs">
              <span class="font-bold text-danger">--hard (强制抛弃此后所有改动 - 危险)</span>
              <span class="text-ink-muted text-[11px]">彻底丢弃目标提交之后的所有未提交改动与新增文件。</span>
            </div>
          </label>
        </div>

      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showResetDialog = false">取消</UiButton>
        <UiButton variant="danger" @click="handleResetBranch">确认重置分支</UiButton>
      </template>
    </UiModal>

    <!-- Modal: Range Cherry-Pick -->
    <div
      v-if="showRangeCherryPickDialog"
      ref="rangeCherryPickDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="范围拣选提交"
      @click.self="!isRangeCherryPicking && (showRangeCherryPickDialog = false)"
      class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="w-full max-w-md rounded-2xl bg-surface border border-line shadow-2xl flex flex-col gap-3 p-5">
        <div class="flex items-center gap-2 pb-2 border-b border-line">
          <GitCompare class="w-4 h-4 text-accent" />
          <h3 class="text-sm font-bold text-ink">范围 Cherry-Pick</h3>
        </div>
        <p class="text-[11px] text-ink-muted leading-relaxed">
          使用 <span class="font-mono text-ink">git cherry-pick A..B</span> 按时间顺序拣选提交；
          <span class="text-warning">起点 A 本身不会被包含</span>。
        </p>
        <label class="flex flex-col gap-1">
          <span class="text-[10px] text-ink-muted">起点引用 A（不含）</span>
          <input
            v-model="rangeStartRef"
            type="text"
            placeholder="例如 v1.0.0 或 HEAD~5"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-[10px] text-ink-muted">终点引用 B（包含）</span>
          <input
            v-model="rangeEndRef"
            type="text"
            placeholder="例如 feature/backport 或 HEAD"
            @keydown.enter="handleRangeCherryPick"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </label>
        <div class="flex items-center justify-end gap-2 pt-2 border-t border-line">
          <button
            @click="showRangeCherryPickDialog = false"
            :disabled="isRangeCherryPicking"
            class="px-3 py-1.5 rounded-lg hover:bg-raised text-xs text-ink-muted transition cursor-pointer disabled:opacity-50"
          >
            取消
          </button>
          <button
            @click="handleRangeCherryPick"
            :disabled="isRangeCherryPicking || !rangeStartRef.trim() || !rangeEndRef.trim()"
            class="px-4 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
          >
            {{ isRangeCherryPicking ? "拣选中..." : "开始拣选" }}
          </button>
        </div>
      </div>
    </div>

    <!-- Modal: Batch Cherry-Pick (multi-select) -->
    <div
      v-if="showBatchCherryPickDialog"
      ref="batchCherryPickDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="批量拣选提交"
      @click.self="closeBatchCherryPickDialog"
      class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="w-full max-w-lg rounded-2xl bg-surface border border-line shadow-2xl flex flex-col overflow-hidden">
        <div class="px-5 py-3 border-b border-line flex items-center justify-between shrink-0">
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <GitCompare class="w-4 h-4 text-accent" />
            批量 Cherry-Pick
          </h3>
          <button aria-label="关闭 (Esc)"
            @click="closeBatchCherryPickDialog"
            :disabled="isBatchCherryPicking"
            class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
            title="关闭 (Esc)"
          >
            <X class="w-3.5 h-3.5" />
          </button>
        </div>

        <div class="px-5 py-3 flex flex-col gap-3 overflow-y-auto">
          <p v-if="!batchCherryPickResult" class="text-[11px] text-ink-muted leading-relaxed">
            将按 <span class="text-accent font-semibold">从旧到新</span> 的顺序，把下列
            <span class="text-ink font-mono">{{ selectedCommitsOldestFirst.length }}</span>
            条提交应用到当前检出分支（即列表中自下而上的顺序）：
          </p>

          <div v-if="!batchCherryPickResult" class="max-h-52 overflow-y-auto space-y-1 pr-1">
            <div
              v-for="(c, index) in selectedCommitsOldestFirst"
              :key="c.hash"
              class="flex items-center gap-2 px-2 py-1.5 rounded-lg bg-canvas border border-line/60 text-xs"
            >
              <span class="w-5 text-[10px] text-ink-muted font-mono text-right shrink-0">{{ index + 1 }}</span>
              <span class="font-mono text-[10px] text-accent shrink-0">{{ c.shortHash }}</span>
              <span class="text-ink-secondary truncate flex-1">{{ c.subject }}</span>
            </div>
          </div>

          <!-- Operation result -->
          <div
            v-if="batchCherryPickResult"
            class="rounded-lg border p-2.5 text-[11px] flex flex-col gap-1"
            :class="
              batchCherryPickResult.hasConflict || batchCherryPickResult.successCount < batchCherryPickResult.totalCount
                ? 'bg-warning/10 border-warning/30'
                : 'bg-success/10 border-success/30'
            "
          >
            <span
              class="font-semibold"
              :class="
                batchCherryPickResult.hasConflict || batchCherryPickResult.successCount < batchCherryPickResult.totalCount
                  ? 'text-warning'
                  : 'text-success'
              "
            >
              {{
                batchCherryPickResult.hasConflict || batchCherryPickResult.successCount < batchCherryPickResult.totalCount
                  ? "批量 Cherry-Pick 中断"
                  : "批量 Cherry-Pick 完成"
              }}
            </span>
            <span class="text-ink-secondary font-mono">
              成功 {{ batchCherryPickResult.successCount }} / 共 {{ batchCherryPickResult.totalCount }} 条
            </span>
            <template v-if="batchCherryPickResult.hasConflict || batchCherryPickResult.failedCommitHash">
              <span class="text-danger font-mono break-all">
                失败提交: {{ batchCherryPickResult.failedCommitHash || "未知" }}
              </span>
              <span v-if="batchCherryPickResult.errorMessage" class="text-ink-secondary whitespace-pre-wrap">
                {{ batchCherryPickResult.errorMessage }}
              </span>
              <span class="text-warning">
                请在工作区解决冲突后继续本次 Cherry-Pick，或中止操作后再重试。
              </span>
            </template>
          </div>

          <div
            v-if="batchCherryPickError"
            class="rounded-lg border border-danger/30 bg-danger/10 p-2.5 text-[11px] text-danger whitespace-pre-wrap"
          >
            {{ batchCherryPickError }}
          </div>
        </div>

        <div class="px-5 py-3 border-t border-line flex items-center justify-between gap-2 shrink-0">
          <span class="text-[10px] text-ink-muted font-mono">
            {{ batchCherryPickResult || batchCherryPickError ? "操作结果" : "应用到当前检出分支" }}
          </span>
          <div class="flex items-center gap-2">
            <button
              @click="closeBatchCherryPickDialog"
              :disabled="isBatchCherryPicking"
              class="px-3 py-1.5 rounded-lg hover:bg-raised text-xs text-ink-muted cursor-pointer disabled:opacity-50"
            >
              {{ batchCherryPickResult || batchCherryPickError ? "关闭" : "取消" }}
            </button>
            <button
              v-if="!batchCherryPickResult && !batchCherryPickError"
              @click="handleBatchCherryPick"
              :disabled="isBatchCherryPicking || selectedCommitsOldestFirst.length === 0"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-white text-xs font-semibold cursor-pointer disabled:opacity-50"
            >
              <RefreshCw v-if="isBatchCherryPicking" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ isBatchCherryPicking ? "执行中..." : "确认批量 Cherry-Pick" }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Interactive Rebase Planner -->
    <div
      v-if="showRebaseDialog"
      ref="rebaseDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="交互式变基"
      @click.self="closeRebaseDialog"
      class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4"
    >
      <div class="w-full max-w-2xl max-h-[86vh] rounded-2xl bg-surface border border-line shadow-2xl flex flex-col overflow-hidden">
        <div class="px-5 py-3 border-b border-line flex items-center justify-between shrink-0">
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <GitBranch class="w-4 h-4 text-cyan" />
            交互式变基（Interactive Rebase）
          </h3>
          <button aria-label="关闭 (Esc)"
            @click="closeRebaseDialog"
            :disabled="isExecutingRebase"
            class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
            title="关闭 (Esc)"
          >
            <X class="w-3.5 h-3.5" />
          </button>
        </div>

        <div class="px-5 py-3 flex flex-col gap-3 overflow-y-auto">
          <div class="flex items-start gap-2 p-2.5 rounded-lg bg-warning/10 border border-warning/30 text-[11px] text-ink-secondary leading-relaxed">
            <AlertTriangle class="w-3.5 h-3.5 text-warning shrink-0 mt-0.5" />
            <span>
              交互式变基会改写当前检出分支的提交历史并生成新的提交 Hash。若因冲突中断，请在工作区解决冲突后继续，或使用“中止变基”恢复原状。
            </span>
          </div>

          <div class="flex items-center gap-2 flex-wrap">
            <label class="text-[11px] text-ink-muted shrink-0">变基基准 (upstream)</label>
            <select
              v-model="rebaseBase"
              :disabled="isLoadingRebasePlan || isExecutingRebase"
              class="flex-1 min-w-[200px] px-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent cursor-pointer transition disabled:opacity-60"
            >
              <option v-if="rebaseBaseOptions.length === 0" value="">暂无可选基准</option>
              <option v-for="option in rebaseBaseOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <button aria-label="重新加载变基计划"
              @click="loadRebasePlan"
              :disabled="isLoadingRebasePlan || isExecutingRebase"
              class="p-1 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
              title="重新加载变基计划"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingRebasePlan }" />
            </button>
          </div>

          <div class="flex items-center justify-between gap-2 text-[10px] text-ink-muted flex-wrap">
            <span>
              计划提交数：<span class="text-ink font-mono">{{ rebasePlan.length }}</span> · 顺序由 git 固定，用每行左侧操作下拉调整动作（pick/reword/edit/squash/fixup/drop）。
            </span>
            <span v-if="selectionCount > 0">工具栏已选 {{ selectionCount }} 条，在计划中高亮</span>
          </div>

          <div
            v-if="isLoadingRebasePlan"
            class="py-10 text-center text-xs text-ink-muted flex flex-col items-center gap-2"
          >
            <RefreshCw class="w-4 h-4 animate-spin text-accent" />
            <span>正在生成变基计划...</span>
          </div>
          <div
            v-else-if="rebasePlanError"
            class="rounded-lg border border-danger/30 bg-danger/10 p-2.5 text-[11px] text-danger whitespace-pre-wrap"
          >
            {{ rebasePlanError }}
          </div>
          <div v-else-if="rebasePlan.length === 0" class="py-10 text-center text-xs text-ink-muted">
            该基准与当前分支之间没有可编辑的提交
          </div>
          <div v-else class="flex flex-col gap-1 max-h-[42vh] overflow-y-auto pr-1">
            <div
              v-for="(todo, index) in rebasePlan"
              :key="todo.hash + ':' + index"
              class="flex items-center gap-2 px-2 py-1.5 rounded-lg border text-xs"
              :class="isHashSelected(todo.hash) ? 'bg-accent/10 border-accent/30' : 'bg-canvas border-line/60'"
            >
              <span class="w-5 text-[10px] text-ink-muted font-mono text-right shrink-0">{{ index + 1 }}</span>
              <select
                v-model="todo.action"
                :disabled="isExecutingRebase"
                class="px-1.5 py-0.5 rounded-md bg-surface border border-line text-[10px] text-ink font-mono focus:outline-none focus:border-accent cursor-pointer disabled:opacity-60 shrink-0"
              >
                <option v-for="action in REBASE_ACTIONS" :key="action.value" :value="action.value">
                  {{ action.label }}
                </option>
              </select>
              <span class="font-mono text-[10px] text-accent shrink-0">{{ todo.shortHash }}</span>
              <span class="text-ink-secondary truncate flex-1">{{ todo.subject }}</span>
              <span class="text-[10px] text-ink-muted truncate max-w-[110px] shrink-0">{{ todo.author }}</span>
              <span
                v-if="isHashSelected(todo.hash)"
                class="px-1 py-0.5 rounded bg-accent/15 border border-accent/30 text-accent text-[9px] shrink-0"
              >
                已选
              </span>
            </div>
          </div>

          <div
            v-if="rebaseError"
            class="rounded-lg border border-danger/30 bg-danger/10 p-2.5 text-[11px] text-danger whitespace-pre-wrap max-h-32 overflow-y-auto"
          >
            {{ rebaseError }}
            <p class="mt-1 text-ink-muted">
              提示：若变基因冲突中断，请在工作区解决冲突后继续，或执行“中止变基”恢复原状。
            </p>
          </div>
        </div>

        <div class="px-5 py-3 border-t border-line flex items-center justify-between gap-2 shrink-0">
          <span class="text-[10px] text-ink-muted font-mono truncate max-w-[55%]" :title="rebaseBase">
            基准: {{ rebaseBase || "未选择" }}
          </span>
          <div class="flex items-center gap-2">
            <button
              @click="closeRebaseDialog"
              :disabled="isExecutingRebase"
              class="px-3 py-1.5 rounded-lg hover:bg-raised text-xs text-ink-muted cursor-pointer disabled:opacity-50"
            >
              取消 (Esc)
            </button>
            <button
              @click="executeRebase"
              :disabled="isExecutingRebase || isLoadingRebasePlan || rebasePlan.length === 0 || !rebaseBase"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-white text-xs font-semibold cursor-pointer disabled:opacity-50"
            >
              <RefreshCw v-if="isExecutingRebase" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ isExecutingRebase ? "变基执行中..." : "执行交互式变基" }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Git Notes Editor -->
    <div
      v-if="showNoteModal"
      ref="noteDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="编辑提交备注"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4"
    >
      <div class="w-full max-w-lg rounded-2xl bg-surface border border-line p-5 shadow-2xl flex flex-col gap-4 animate-in fade-in zoom-in-95 duration-150">
        <div class="flex items-center gap-2 pb-2 border-b border-line">
          <StickyNote class="w-4 h-4 text-warning" />
          <h3 class="text-sm font-bold text-ink">
            {{ currentCommitNote ? "编辑提交批注 (Git Notes)" : "新增提交批注 (Git Notes)" }}
          </h3>
        </div>

        <div class="flex items-center gap-2 text-xs font-mono text-ink-muted">
          <span>提交目标:</span>
          <span class="px-2 py-0.5 rounded bg-canvas border border-line text-accent font-bold">
            {{ selectedCommit?.shortHash }}
          </span>
          <span class="truncate">{{ selectedCommit?.subject }}</span>
        </div>

        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">批注正文内容 (支持多行，如 QA 验收结论 / 代码评审日志):</label>
          <textarea
            v-model="noteInputText"
            rows="5"
            placeholder="例如: [QA-VERIFIED] 已经在自动化压测流水线通过，无性能回退问题..."
            class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-sans leading-relaxed"
          ></textarea>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-line">
          <button
            @click="showNoteModal = false"
            class="px-3 py-1.5 rounded-lg hover:bg-raised text-xs text-ink-muted hover:text-ink transition cursor-pointer"
          >
            取消
          </button>
          <button
            @click="handleSaveNote"
            :disabled="isSavingNote || !noteInputText.trim()"
            class="px-4 py-1.5 rounded-lg bg-warning-strong hover:bg-warning-strong-hover text-warning-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-50"
          >
            {{ isSavingNote ? "保存中..." : "保存批注" }}
          </button>
        </div>
      </div>
    </div>

    <!-- Commit Row Right-Click Context Menu -->
    <div
      v-if="showContextMenu && contextMenuCommit"
      class="fixed z-50 w-60 rounded-xl bg-surface/95 backdrop-blur-xl border border-line shadow-2xl py-1 text-xs select-none animate-in fade-in zoom-in-95 duration-100"
      :style="{ left: contextMenuPos.x + 'px', top: contextMenuPos.y + 'px' }"
      @click.stop
    >
      <div class="px-3 py-1 border-b border-line/60 flex items-center justify-between text-[10px] text-ink-muted">
        <span class="font-mono font-semibold text-accent">{{ contextMenuCommit.shortHash }}</span>
        <span>快捷操作</span>
      </div>

      <div class="py-0.5">
        <button
          type="button"
          @click="handleRebaseOntoCommit(contextMenuCommit); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-warning/15 hover:text-warning text-ink text-left transition cursor-pointer font-medium"
        >
          <GitPullRequest class="w-3.5 h-3.5 text-warning shrink-0" />
          <span>变基当前分支到此提交</span>
        </button>

        <button
          type="button"
          @click="openRebaseDialog(contextMenuCommit.hash); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-cyan/15 hover:text-cyan text-ink text-left transition cursor-pointer font-medium"
        >
          <GitMerge class="w-3.5 h-3.5 text-cyan shrink-0" />
          <span>以此提交为起点交互式变基</span>
        </button>
      </div>

      <div class="h-px bg-line/60 my-0.5"></div>

      <div class="py-0.5">
        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; handleCherryPick(); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <GitCompare class="w-3.5 h-3.5 text-accent shrink-0" />
          <span>Cherry-Pick 应用此提交</span>
        </button>

        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; handleRevert(); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <RotateCcw class="w-3.5 h-3.5 text-warning shrink-0" />
          <span>Revert 回滚此更改</span>
        </button>

        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; showNewBranchDialog = true; closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <GitBranch class="w-3.5 h-3.5 text-cyan shrink-0" />
          <span>从此提交创建分支...</span>
        </button>

        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; showNewTagDialog = true; closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <Tag class="w-3.5 h-3.5 text-purple shrink-0" />
          <span>为此提交创建标签...</span>
        </button>

        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; showResetDialog = true; closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-danger/15 text-danger text-left transition cursor-pointer"
        >
          <AlertTriangle class="w-3.5 h-3.5 shrink-0" />
          <span>重置当前分支到此提交...</span>
        </button>
      </div>

      <div class="h-px bg-line/60 my-0.5"></div>

      <div class="py-0.5">
        <button
          type="button"
          @click="copyHash(contextMenuCommit.hash); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <Copy class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span>复制 Commit Hash</span>
        </button>

        <button
          type="button"
          @click="copyText(contextMenuCommit.subject, '已复制提交说明'); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <FileText class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span>复制提交说明</span>
        </button>

        <button
          type="button"
          @click="handleOpenCommitInWeb(contextMenuCommit.hash); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <ExternalLink class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span>在网页端查看</span>
        </button>
      </div>
    </div>

    <!-- Modal: Commit signature details -->
    <div
      v-if="showSignatureModal"
      ref="signatureDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="提交签名详情"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4"
    >
      <div class="w-full max-w-lg rounded-2xl bg-surface border border-line shadow-2xl flex flex-col overflow-hidden">
        <div class="flex items-center justify-between px-5 py-3 border-b border-line">
          <div class="flex items-center gap-2">
            <ShieldCheck class="w-4 h-4 text-success" />
            <h3 class="text-sm font-bold text-ink">提交签名校验详情</h3>
          </div>
          <button aria-label="关闭"
            @click="showSignatureModal = false"
            class="p-1 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-5 space-y-3 text-xs">
          <div v-if="isLoadingSignature" class="py-8 text-center text-ink-muted">正在校验签名...</div>
          <template v-else-if="signatureDetails">
            <div class="flex items-center gap-2">
              <span class="px-2 py-0.5 rounded text-[11px] font-bold" :class="signatureStatusMeta(signatureDetails.status).cls">
                {{ signatureStatusMeta(signatureDetails.status).label }}
              </span>
              <span class="text-[10px] text-ink-muted font-mono">%G? = {{ signatureDetails.status }}</span>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div class="p-2.5 rounded-lg bg-canvas/40 border border-line">
                <div class="text-[10px] text-ink-muted">签名者</div>
                <div class="text-ink font-medium truncate">{{ signatureDetails.signer || "—" }}</div>
              </div>
              <div class="p-2.5 rounded-lg bg-canvas/40 border border-line">
                <div class="text-[10px] text-ink-muted">密钥 ID</div>
                <div class="font-mono text-ink truncate">{{ signatureDetails.keyId || "—" }}</div>
              </div>
              <div class="p-2.5 rounded-lg bg-canvas/40 border border-line col-span-2">
                <div class="text-[10px] text-ink-muted">指纹</div>
                <div class="font-mono text-[10px] text-ink-secondary break-all">{{ signatureDetails.fingerprint || "—" }}</div>
              </div>
              <div class="p-2.5 rounded-lg bg-canvas/40 border border-line col-span-2">
                <div class="text-[10px] text-ink-muted">签名时间</div>
                <div class="text-ink-secondary">{{ signatureDetails.date || "—" }}</div>
              </div>
            </div>
            <div v-if="signatureDetails.raw" class="p-2.5 rounded-lg bg-canvas/60 border border-line font-mono text-[10px] text-ink-muted whitespace-pre-wrap max-h-40 overflow-y-auto">
              {{ signatureDetails.raw }}
            </div>
            <p v-if="signatureDetails.status === 'N'" class="text-[11px] text-ink-muted">
              该提交没有 GPG / SSH 签名。可在「设置 → GPG / SSH 防伪签名」中配置签名密钥，后续提交将自动签名。
            </p>
          </template>
        </div>
      </div>
    </div>

    <!-- Modal: Git Notes overview -->
    <div
      v-if="showNotesOverview"
      ref="notesOverviewDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="Git Notes 批注总览"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4"
    >
      <div class="w-full max-w-2xl max-h-[85vh] rounded-2xl bg-surface border border-line shadow-2xl flex flex-col overflow-hidden">
        <div class="flex items-center justify-between px-5 py-3 border-b border-line shrink-0">
          <div class="flex items-center gap-2">
            <StickyNote class="w-4 h-4 text-warning" />
            <h3 class="text-sm font-bold text-ink">Git Notes 批注总览</h3>
            <span v-if="notesOverview" class="text-[10px] text-ink-muted font-mono">
              {{ notesOverview.total }} 条 · {{ notesOverview.ref }}
            </span>
          </div>
          <button aria-label="关闭"
            @click="showNotesOverview = false"
            class="p-1 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-4 overflow-y-auto space-y-2">
          <div v-if="isLoadingNotesOverview" class="py-10 text-center text-xs text-ink-muted">正在读取 Notes...</div>
          <div v-else-if="notesOverview && notesOverview.notes.length === 0" class="py-10 text-center text-xs text-ink-muted">
            当前仓库还没有任何 Git Notes 批注。
          </div>
          <div
            v-for="note in notesOverview?.notes || []"
            :key="`${note.commitHash}:${note.date}`"
            class="p-3 rounded-lg border border-line bg-surface/50 hover:bg-raised/40 transition cursor-pointer space-y-1.5"
            @click="jumpToNote(note)"
          >
            <div class="flex items-center gap-2 min-w-0">
              <span class="font-mono text-accent text-[11px] font-bold shrink-0">{{ note.shortHash }}</span>
              <span class="text-xs text-ink font-medium truncate">{{ note.subject }}</span>
              <span class="ml-auto text-[10px] text-ink-muted shrink-0">{{ note.date }}</span>
            </div>
            <div class="text-[10px] text-ink-muted">{{ note.authorName }}</div>
            <div class="px-2 py-1 rounded bg-canvas/70 border border-line/60 text-[11px] text-ink-secondary line-clamp-2">
              {{ note.preview || note.content }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Pickaxe content archaeology -->
    <div
      v-if="showPickaxeModal"
      ref="pickaxeDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="内容考古搜索"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4"
    >
      <div class="w-full max-w-3xl max-h-[85vh] rounded-2xl bg-surface border border-line shadow-2xl flex flex-col overflow-hidden">
        <div class="flex items-center justify-between px-5 py-3 border-b border-line shrink-0">
          <div class="flex items-center gap-2">
            <Search class="w-4 h-4 text-purple" />
            <h3 class="text-sm font-bold text-ink">内容考古 (git log -S / -G)</h3>
          </div>
          <button aria-label="关闭"
            @click="showPickaxeModal = false"
            class="p-1 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-4 space-y-3 overflow-y-auto">
          <div class="flex flex-wrap items-center gap-2">
            <input
              v-model="pickaxeQuery"
              type="text"
              placeholder="要追踪的字符串或正则，例如 LegendarySword"
              @keydown.enter="handlePickaxeSearch"
              class="flex-1 min-w-[220px] px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted font-mono focus:outline-none focus:border-accent"
            />
            <select
              v-model="pickaxeMode"
              class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
              title="content = git log -S；regex = git log -G"
            >
              <option value="content">内容字面量</option>
              <option value="regex">正则表达式</option>
            </select>
            <input
              v-model="pickaxeAuthor"
              type="text"
              placeholder="作者（可选）"
              class="w-32 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <input
              v-model="pickaxePathFilter"
              type="text"
              placeholder="路径过滤（可选）"
              class="w-40 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted font-mono focus:outline-none focus:border-accent"
            />
            <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
              <input v-model="pickaxeIgnoreCase" type="checkbox" class="rounded accent-purple" />
              <span>忽略大小写</span>
            </label>
            <button
              @click="handlePickaxeSearch"
              :disabled="!pickaxeQuery.trim() || isPickaxeSearching"
              class="px-3 py-1.5 rounded-lg bg-purple-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isPickaxeSearching ? "检索中..." : "开始考古" }}
            </button>
          </div>

          <div v-if="pickaxeResult" class="flex flex-wrap items-center gap-3 px-3 py-2 rounded-lg bg-canvas/60 border border-line text-[11px] text-ink-muted font-mono">
            <span>命中 {{ pickaxeResult.totalHits }} 次 / 分析 {{ pickaxeResult.analyzedHits }} 次</span>
            <span class="text-success">+{{ pickaxeResult.addedLines }}</span>
            <span class="text-danger">-{{ pickaxeResult.removedLines }}</span>
            <span>{{ pickaxeResult.fileCount }} 个文件</span>
            <span v-if="pickaxeResult.firstSeen">首次 {{ pickaxeResult.firstSeen }}</span>
            <span v-if="pickaxeResult.lastSeen">最近 {{ pickaxeResult.lastSeen }}</span>
            <span v-if="pickaxeResult.truncated" class="text-warning">结果已截断</span>
          </div>

          <div v-if="pickaxeResult && pickaxeResult.hits.length > 0" class="space-y-2">
            <div
              v-for="hit in pickaxeResult.hits"
              :key="`${hit.hash}:${hit.change}`"
              class="p-2.5 rounded-lg border border-line bg-surface/50 space-y-1.5"
            >
              <div class="flex items-center gap-2 min-w-0">
                <button
                  @click="copyPickaxeHash(hit.hash)"
                  class="font-mono text-accent hover:underline text-[11px] font-bold shrink-0"
                  title="复制完整 Commit Hash"
                >
                  {{ hit.shortHash }}
                </button>
                <span
                  class="px-1.5 py-0.2 rounded text-[9px] font-bold shrink-0"
                  :class="hit.change === '新增' ? 'bg-success/15 text-success' : hit.change === '移除' ? 'bg-danger/15 text-danger' : 'bg-warning/15 text-warning'"
                >
                  {{ hit.change }}
                </span>
                <span class="text-xs text-ink font-medium truncate">{{ hit.subject }}</span>
                <span class="ml-auto text-[10px] text-ink-muted font-mono shrink-0">{{ hit.date }}</span>
              </div>
              <div class="flex items-center gap-2 text-[10px] text-ink-muted">
                <span>{{ hit.author }}</span>
                <span class="font-mono text-success">+{{ hit.added }}</span>
                <span class="font-mono text-danger">-{{ hit.removed }}</span>
              </div>
              <div v-if="hit.snippet" class="px-2 py-1 rounded bg-canvas/70 border border-line/60 font-mono text-[10px] text-ink-secondary break-all">
                {{ hit.snippet }}
              </div>
              <div v-if="hit.files.length > 0" class="text-[10px] text-ink-muted font-mono truncate" :title="hit.files.join(', ')">
                {{ hit.files.join(" · ") }}
              </div>
            </div>
          </div>
          <div v-else-if="pickaxeResult && !isPickaxeSearching" class="py-10 text-center text-xs text-ink-muted">
            没有找到包含该内容变更的提交。
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
