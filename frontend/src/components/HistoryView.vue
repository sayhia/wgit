<script setup lang="ts">
import { confirmAction } from "../composables/useConfirm";
import { t } from "../i18n";
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
  ExternalLink,
  StickyNote,
  GitPullRequest,
  GitMerge,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import { backendText } from "../i18n";
import Toast from "./ui/Toast.vue";
import UiModal from "./ui/UiModal.vue";
import UiButton from "./ui/UiButton.vue";
import { useToast } from "../composables/useToast";
import HistoryFilterBar from "./history/HistoryFilterBar.vue";
import BatchActionToolbar from "./history/BatchActionToolbar.vue";
import CompareCommitsModal from "./history/CompareCommitsModal.vue";
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
const COMPACT_GUTTER_WIDTH = 20; // compact mode gutter (coloured dots only)
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

/* Pickaxe results ----------------------------------------------------- */
// The backend classifies a hit with a Chinese label; map the known values to
// i18n keys and pass anything else through untouched.
// These three are *backend wire values*, not UI copy: they must stay in the
// exact form the Go side emits so the mapping keeps working.
const PICKAXE_CHANGE_KEYS: Record<string, string> = {
  新增: "history.pickaxe.change.added",
  移除: "history.pickaxe.change.removed",
  修改: "history.pickaxe.change.changed",
};

function pickaxeChangeLabel(change: string): string {
  const key = PICKAXE_CHANGE_KEYS[change];
  return key ? t(key) : change;
}

function pickaxeChangeClass(change: string): string {
  if (change === "新增") return "bg-success/15 text-success";
  if (change === "移除") return "bg-danger/15 text-danger";
  return "bg-warning/15 text-warning";
}

/* Branch scope ------------------------------------------------------- */
const branches = ref<BranchInfo[]>([]);
const branchScope = ref(""); // "" => every ref (--all)
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
  if (f.keyword) parts.push(t("history.filter.keyword", { keyword: f.keyword }));
  if (f.author) parts.push(t("history.filter.author", { author: f.author }));
  if (f.since) parts.push(t("history.filter.since", { date: f.since }));
  if (f.until) parts.push(t("history.filter.until", { date: f.until }));
  return parts.length > 0 ? t("history.filter.summary", { parts: parts.join(" · ") }) : "";
});

const graphListRef = ref<InstanceType<typeof CommitGraphList> | null>(null);
/**
 * Non-empty when the last load failed. Without it a backend failure renders the
 * "no commits yet" empty state, which reads as an empty repository.
 */
const loadError = ref("");
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
    return isFilterMode.value ? t("history.loading.filter") : t("history.loading");
  }
  if (isFilterMode.value) {
    const total = filteredTotalCount.value;
    if (total !== null && total > loaded) return t("history.count.filtered.total", { loaded, total });
    return hasMore.value ? t("history.count.filtered.more", { loaded }) : t("history.count.filtered", { loaded });
  }
  const total = repoTotalCount.value;
  if (total !== null && total > loaded) return t("history.count.total", { loaded, total });
  if (total !== null) return t("history.count.plain", { total: Math.max(total, loaded) });
  return hasMore.value ? t("history.count.more", { loaded }) : t("history.count", { loaded });
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
  loadError.value = "";
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
      loadError.value = err instanceof Error ? err.message : String(err);
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
      showToast(t("history.toast.export.empty"));
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
      if (!ok) throw new Error(t("history.error.clipboard"));
    }
    showToast(t("history.toast.export.copied"));
  } catch (err: any) {
    showToast(t("history.toast.export.failed", { error: err?.message || err }));
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
      showToast(t("history.toast.pickaxe.none"));
    }
  } catch (err: any) {
    showToast(t("history.toast.pickaxe.failed", { error: err?.message || err }));
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
  showToast(t("history.toast.hash.copied", { hash: hash.substring(0, 8) }));
}

/* ------------------------------------------------------------------ */
/* Commit signature details                                            */
/* ------------------------------------------------------------------ */
const showSignatureModal = ref(false);
const signatureDetails = ref<SignatureDetails | null>(null);
const isLoadingSignature = ref(false);

async function handleShowSignatureDetails(hash?: string) {
  const commitHash = hash || selectedCommit.value?.hash;
  if (!props.currentRepoPath || !commitHash) return;
  showSignatureModal.value = true;
  signatureDetails.value = null;
  isLoadingSignature.value = true;
  try {
    signatureDetails.value = await gitService.getCommitSignatureDetails(props.currentRepoPath, commitHash);
  } catch (err: any) {
    showSignatureModal.value = false;
    showToast(t("history.toast.signature.failed", { error: err?.message || err }));
  } finally {
    isLoadingSignature.value = false;
  }
}

function signatureStatusMeta(status: string) {
  switch (status) {
    case "G":
      return { label: t("history.signature.good"), cls: "bg-success/15 text-success" };
    case "B":
      return { label: t("history.signature.bad"), cls: "bg-danger/15 text-danger" };
    case "U":
      return { label: t("history.signature.untrusted"), cls: "bg-warning/15 text-warning" };
    default:
      return { label: t("history.signature.none"), cls: "bg-surface border border-line text-ink-muted" };
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
    showToast(t("history.toast.notes.failed", { error: err?.message || err }));
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
    showToast(t("history.toast.hash.copied.unlisted"));
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

/* Compare two commits modal ------------------------------------------ */
const showCompareModal = ref(false);
const compareBaseCommit = ref<CommitItem | null>(null);
const compareTargetCommit = ref<CommitItem | null>(null);

async function handleRangeCherryPick() {
  if (!props.currentRepoPath || !rangeStartRef.value.trim() || !rangeEndRef.value.trim()) return;
  if (isRangeCherryPicking.value) return;
  if (
    !(await confirmAction(
      t("history.confirm.range.pick", { start: rangeStartRef.value.trim(), end: rangeEndRef.value.trim() })
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
    showToast(out ? t("history.toast.range.done.detail", { detail: out.slice(0, 80) }) : t("history.toast.range.done"));
    showRangeCherryPickDialog.value = false;
    rangeStartRef.value = "";
    rangeEndRef.value = "";
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(t("history.toast.range.failed", { error: err?.message || err }));
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
  { value: "pick", label: t("history.rebase.action.pick") },
  { value: "reword", label: t("history.rebase.action.reword") },
  { value: "edit", label: t("history.rebase.action.edit") },
  { value: "squash", label: t("history.rebase.action.squash") },
  { value: "fixup", label: t("history.rebase.action.fixup") },
  { value: "drop", label: t("history.rebase.action.drop") },
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
  if (oldestParent) add(oldestParent, t("history.rebase.base.parent", { hash: oldestParent.slice(0, 7) }));
  // Explicit base passed via the "wgit:history-rebase" window event.
  const override = rebaseBaseOverride.value.trim();
  if (override) add(override, t("history.rebase.base.override", { hash: override.slice(0, 7) }));
  const current = currentBranchInfo.value;
  if (current?.upstream) add(current.upstream, t("history.rebase.base.upstream", { name: current.upstream }));
  for (const branch of localBranches.value) {
    add(branch.name, branch.isCurrent ? t("history.rebase.base.current.branch", { name: branch.name }) : branch.name);
  }
  return options;
});

function isDialogOpen(): boolean {
  return (
    showCompareModal.value ||
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


async function refreshNotedHashes() {
  if (!props.currentRepoPath) return;
  try {
    const hashes = await gitService.getNotedCommitHashes(props.currentRepoPath);
    notedCommitHashesSet.value = new Set(hashes || []);
  } catch (err: any) {
    showToast(t("history.toast.note.read.failed", { error: err?.message || err }));
  }
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
    showToast(t("history.toast.note.saved"));
  } catch (err: any) {
    showToast(t("history.toast.note.save.failed", { error: err?.message || err }));
  } finally {
    isSavingNote.value = false;
  }
}

async function handleDeleteNote() {
  if (!props.currentRepoPath || !selectedCommit.value) return;
  if (!(await confirmAction({ message: t("history.confirm.note.delete"), tone: "danger" }))) return;
  try {
    await gitService.deleteCommitNote(props.currentRepoPath, selectedCommit.value.hash);
    currentCommitNote.value = "";
    notedCommitHashesSet.value.delete(selectedCommit.value.hash);
    showToast(t("history.toast.note.deleted"));
  } catch (err: any) {
    showToast(t("history.toast.note.delete.failed", { error: err?.message || err }));
  }
}

async function handlePushNotes() {
  if (!props.currentRepoPath) return;
  isPushingNotes.value = true;
  try {
    const msg = await gitService.pushCommitNotes(props.currentRepoPath);
    showToast(msg || t("history.toast.note.pushed"));
  } catch (err: any) {
    showToast(t("history.toast.note.push.failed", { error: err?.message || err }));
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
  }).catch((err: any) => {
    if (seq === detailSeq) {
      showToast(t("history.toast.note.read.failed", { error: err?.message || err }));
    }
  });
}

function copyHash(hash: string) {
  navigator.clipboard.writeText(hash);
  copiedHash.value = hash;
  setTimeout(() => {
    copiedHash.value = null;
  }, 1500);
}

function copyText(text: string, label = t("history.toast.copied")) {
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
    showToast(t("history.toast.cherry.picked", { hash: selectedCommit.value.shortHash }));
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(t("history.toast.cherry.failed", { error: err?.message || err }));
  }
}

async function handleOpenCommitInWeb(hash: string) {
  if (!props.currentRepoPath || !hash) return;
  try {
    await gitService.openCommitInWeb(props.currentRepoPath, hash);
  } catch (err: any) {
    showToast(t("history.toast.open.web.failed", { error: err?.message || err }));
  }
}

// Revert commit
async function handleRevert() {
  if (!selectedCommit.value || !props.currentRepoPath) return;
  if (!(await confirmAction(t("history.confirm.revert", { hash: selectedCommit.value.shortHash })))) return;
  try {
    await gitService.revertCommit(props.currentRepoPath, selectedCommit.value.hash);
    showToast(t("history.toast.reverted", { hash: selectedCommit.value.shortHash }));
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(t("history.toast.revert.failed", { error: err?.message || err }));
  }
}

async function handleRevertNoCommit() {
  if (!selectedCommit.value || !props.currentRepoPath) return;
  if (!(await confirmAction(t("history.confirm.revert.worktree", { hash: selectedCommit.value.shortHash })))) return;
  try {
    await gitService.revertNoCommit(props.currentRepoPath, selectedCommit.value.hash);
    showToast(t("history.toast.reverted.worktree", { hash: selectedCommit.value.shortHash }));
    emit("refresh");
  } catch (err: any) {
    showToast(t("history.toast.revert.worktree.failed", { error: err?.message || err }));
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
      t("history.confirm.fixup", { hash: target.shortHash, subject: target.subject })
    ))
  )
    return;
  try {
    const res = await gitService.createFixupCommit(props.currentRepoPath, target.hash, "fixup");
    showToast(backendText(res.messageKey, res.message, res.params));
    emit("refresh");
  } catch (err: any) {
    showToast(t("history.toast.fixup.failed", { error: err?.message || err }));
  }
}

async function handleCheckoutDetached() {  if (!selectedCommit.value || !props.currentRepoPath) return;
  if (
    !(await confirmAction(
      t("history.confirm.detach", { hash: selectedCommit.value.shortHash })
    ))
  )
    return;
  try {
    await gitService.checkoutCommitDetached(props.currentRepoPath, selectedCommit.value.hash);
    showToast(t("history.toast.detached", { hash: selectedCommit.value.shortHash }));
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(t("history.toast.detach.failed", { error: err?.message || err }));
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
    showToast(t("history.toast.branch.created", { hash: selectedCommit.value.shortHash, branch: newBranchName.value.trim() }));
    showNewBranchDialog.value = false;
    newBranchName.value = "";
    emit("refresh");
    void loadBranches();
  } catch (err: any) {
    showToast(t("history.toast.branch.failed", { error: err?.message || err }));
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
    showToast(t("history.toast.tag.created", { tag: newTagName.value.trim() }));
    showNewTagDialog.value = false;
    newTagName.value = "";
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(t("history.toast.tag.failed", { error: err?.message || err }));
  }
}

// Reset branch to this commit
async function handleResetBranch() {
  if (!selectedCommit.value || !props.currentRepoPath) return;
  if (
    resetMode.value === "hard" &&
    !(await confirmAction({ message: t("history.confirm.reset.hard"), tone: "danger" }))
  ) {
    return;
  }
  try {
    await gitService.resetBranch(
      props.currentRepoPath,
      selectedCommit.value.hash,
      resetMode.value
    );
    showToast(t("history.toast.reset.done", { mode: resetMode.value, hash: selectedCommit.value.shortHash }));
    showResetDialog.value = false;
    emit("refresh");
    await refreshHistory(true);
  } catch (err: any) {
    showToast(t("history.toast.reset.failed", { error: err?.message || err }));
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
    showToast(t("history.toast.details.copied", { count: details.length }));
  } catch (err: any) {
    showToast(t("history.toast.details.failed", { error: err?.message || err }));
  }
}

function handleCompareTwoCommits() {
  if (selectedHashes.value.size !== 2) return;
  const hashArr = Array.from(selectedHashes.value);
  const c1 = commits.value.find((c) => c.hash === hashArr[0]);
  const c2 = commits.value.find((c) => c.hash === hashArr[1]);
  if (!c1 || !c2) return;

  const time1 = new Date(c1.date).getTime();
  const time2 = new Date(c2.date).getTime();
  if (!isNaN(time1) && !isNaN(time2)) {
    if (time1 <= time2) {
      compareBaseCommit.value = c1;
      compareTargetCommit.value = c2;
    } else {
      compareBaseCommit.value = c2;
      compareTargetCommit.value = c1;
    }
  } else {
    const idx1 = commits.value.indexOf(c1);
    const idx2 = commits.value.indexOf(c2);
    if (idx1 >= idx2) {
      compareBaseCommit.value = c1;
      compareTargetCommit.value = c2;
    } else {
      compareBaseCommit.value = c2;
      compareTargetCommit.value = c1;
    }
  }

  showCompareModal.value = true;
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
      batchCherryPickError.value = t("history.error.batch.no.result");
    } else {
      batchCherryPickResult.value = result;
      attempted = true;
      if (result.hasConflict || result.successCount < result.totalCount) {
        showToast(t("history.toast.batch.interrupted", { ok: result.successCount, total: result.totalCount }));
      } else {
        showToast(t("history.toast.batch.done", { ok: result.successCount, total: result.totalCount }));
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
    rebasePlanError.value = t("history.error.rebase.no.base");
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
    t("history.confirm.rebase", { base: rebaseBase.value, count: rebasePlan.value.length })
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
      typeof output === "string" && output.trim() ? output : t("history.toast.rebase.done");
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
      outputSummary ? t("history.toast.rebase.done.detail", { detail: outputSummary.slice(0, 80) }) : t("history.toast.rebase.done")
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
      if (showCompareModal.value) showCompareModal.value = false;
      else if (showRebaseDialog.value) closeRebaseDialog();
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
  showCompareModal.value = false;
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
          <span class="text-xs font-semibold text-ink whitespace-nowrap">{{ t("history.title") }}</span>
          <span class="text-[10px] text-ink-muted font-mono font-normal truncate">{{ countLabel }}</span>
          <span
            v-if="topologyStats && topologyStats.totalCommits > 0"
            class="px-1.5 py-0.5 rounded bg-surface border border-line text-[9px] text-ink-muted font-mono font-normal shrink-0"
            :title="t('history.topology.summary', { direct: topologyStats.directCommits, merge: topologyStats.mergeCommits })"
          >
            {{ t("history.topology.merge", { count: topologyStats.mergeCommits, ratio: (topologyStats.mergeRatio * 100).toFixed(0) }) }}
          </span>
          <span
            v-if="isFilterMode"
            class="px-1.5 py-0.5 rounded-md bg-accent/15 border border-accent/30 text-accent text-[10px] font-medium whitespace-nowrap shrink-0"
          >
            {{ t("history.filter.server") }}
          </span>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <!-- Dense graph / compact dots toggle -->
          <div
            class="flex items-center rounded-md border border-line bg-canvas overflow-hidden shrink-0"
            :title="t('history.graph.toggle.title')"
          >
            <button
              @click="setShowGraph(true)"
              class="px-2 py-1 text-[10px] font-medium cursor-pointer transition whitespace-nowrap shrink-0"
              :class="showGraph ? 'bg-raised text-ink' : 'text-ink-muted hover:text-ink'"
              :title="t('history.graph.full.title')"
            >
              {{ t("history.graph.full") }}
            </button>
            <button
              @click="setShowGraph(false)"
              class="px-2 py-1 text-[10px] font-medium cursor-pointer transition whitespace-nowrap shrink-0"
              :class="!showGraph ? 'bg-raised text-ink' : 'text-ink-muted hover:text-ink'"
              :title="t('history.graph.compact.title')"
            >
              {{ t("history.graph.compact") }}
            </button>
          </div>
          <span
            v-if="showGraph && hiddenLaneCount > 0"
            class="text-[10px] text-warning font-mono whitespace-nowrap shrink-0"
            :title="t('history.lanes.overflow', { max: MAX_VISIBLE_LANES, hidden: hiddenLaneCount })"
          >
            {{ t("history.lanes.more", { count: hiddenLaneCount }) }}
          </span>

          <!-- Branch scope selector -->
          <div class="relative">
            <GitBranch class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-1.5 pointer-events-none z-10" />
            <select
              v-model="branchScope"
              :disabled="isLoadingBranches"
              class="pl-7 pr-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink font-mono max-w-[140px] xl:max-w-[210px] focus:outline-none focus:border-accent cursor-pointer transition disabled:opacity-50"
              :title="t('history.scope.title')"
            >
              <option value="">{{ t("history.scope.all") }}</option>
              <option v-for="b in localBranches" :key="b.name" :value="b.name">
                {{ b.name }}{{ b.isCurrent ? " " + t("history.scope.current") : "" }}
              </option>
            </select>
          </div>

          <button
            @click="showRangeCherryPickDialog = true"
            class="flex items-center gap-1 px-2 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-ink transition cursor-pointer"
            :title="t('history.range.toolbar.title')"
          >
            <GitCompare class="w-3 h-3 text-accent" />
            <span class="hidden xl:inline">{{ t("history.range") }}</span>
          </button>
          <button :aria-label="t('history.refresh.title')"
            @click="refreshHistory(true)"
            :disabled="isLoading"
            class="p-1 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
            :title="t('history.refresh.title')"
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
        @compare-two="handleCompareTwoCommits"
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
        :error="loadError"
        @select="handleRowClick"
        @checkbox="handleCheckboxClick"
        @context-menu="handleRowContextMenu"
        @load-more="loadMore"
        @clear-filters="clearFilters"
        @retry="loadCommits(true)"
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
      :label="t('history.dialog.branch.label')"
      width="max-w-sm"
      @update:open="showNewBranchDialog = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-accent" />
          <span>{{ t("history.dialog.branch.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <p class="text-xs text-ink-muted font-mono">
          {{ t("history.dialog.base", { hash: selectedCommit?.shortHash ?? "", subject: selectedCommit?.subject ?? "" }) }}
        </p>
        <input
          v-model="newBranchName"
          type="text"
          :aria-label="t('history.dialog.branch.name')"
          :placeholder="t('history.dialog.branch.placeholder')"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
          @keydown.enter="handleCreateBranch"
        />
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showNewBranchDialog = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="primary" :disabled="!newBranchName.trim()" @click="handleCreateBranch">
          {{ t("history.dialog.branch.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Create Tag from Commit -->
    <UiModal
      :open="showNewTagDialog"
      :label="t('history.dialog.tag.label')"
      width="max-w-sm"
      @update:open="showNewTagDialog = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Tag class="w-4 h-4 text-purple" />
          <span>{{ t("history.dialog.tag.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <p class="text-xs text-ink-muted font-mono">
          {{ t("history.dialog.target", { hash: selectedCommit?.shortHash ?? "" }) }}
        </p>
        <input
          v-model="newTagName"
          type="text"
          :aria-label="t('history.dialog.tag.name')"
          :placeholder="t('history.dialog.tag.placeholder')"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
          @keydown.enter="handleCreateTag"
        />
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showNewTagDialog = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="purple" :disabled="!newTagName.trim()" @click="handleCreateTag">
          {{ t("history.dialog.tag.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Reset Branch -->
    <UiModal
      :open="showResetDialog"
      :label="t('history.dialog.reset.label')"
      width="max-w-md"
      @update:open="showResetDialog = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-danger flex items-center gap-2">
          <AlertTriangle class="w-4 h-4" />
          <span>{{ t("history.dialog.reset.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <p class="text-xs text-ink-muted font-mono">
          {{ t("history.dialog.target", { hash: selectedCommit?.shortHash ?? "" }) }}
        </p>

        <!-- Reset Mode Selector -->
        <div class="flex flex-col gap-2">
          <label class="flex items-start gap-2 p-2.5 rounded-lg bg-canvas border border-line cursor-pointer">
            <input type="radio" value="soft" v-model="resetMode" class="mt-0.5 accent-accent" />
            <div class="flex flex-col text-xs">
              <span class="font-bold text-ink">{{ t("history.reset.soft.label") }}</span>
              <span class="text-ink-muted text-[11px]">{{ t("history.reset.soft.desc") }}</span>
            </div>
          </label>

          <label class="flex items-start gap-2 p-2.5 rounded-lg bg-canvas border border-line cursor-pointer">
            <input type="radio" value="mixed" v-model="resetMode" class="mt-0.5 accent-accent" />
            <div class="flex flex-col text-xs">
              <span class="font-bold text-ink">{{ t("history.reset.mixed.label") }}</span>
              <span class="text-ink-muted text-[11px]">{{ t("history.reset.mixed.desc") }}</span>
            </div>
          </label>

          <label class="flex items-start gap-2 p-2.5 rounded-lg bg-canvas border border-danger/40 cursor-pointer">
            <input type="radio" value="hard" v-model="resetMode" class="mt-0.5 accent-danger" />
            <div class="flex flex-col text-xs">
              <span class="font-bold text-danger">{{ t("history.reset.hard.label") }}</span>
              <span class="text-ink-muted text-[11px]">{{ t("history.reset.hard.desc") }}</span>
            </div>
          </label>
        </div>

      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showResetDialog = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="danger" @click="handleResetBranch">{{ t("history.dialog.reset.confirm") }}</UiButton>
      </template>
    </UiModal>

    <!-- Modal: Range Cherry-Pick -->
    <UiModal
      :open="showRangeCherryPickDialog"
      :title="t('history.range.title')"
      width="max-w-md"
      @update:open="!isRangeCherryPicking && (showRangeCherryPickDialog = $event)"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitCompare class="w-4 h-4 text-accent" />
          {{ t("history.range.title") }}
        </h3>
      </template>

      <div class="space-y-3"><p class="text-[11px] text-ink-muted leading-relaxed">
          {{ t("history.range.desc.before") }} <span class="font-mono text-ink">git cherry-pick A..B</span> {{ t("history.range.desc.after") }}
          <span class="text-warning">{{ t("history.range.warn") }}</span>
        </p>
        <label class="flex flex-col gap-1">
          <span class="text-[10px] text-ink-muted">{{ t("history.range.start") }}</span>
          <input
            v-model="rangeStartRef"
            type="text"
            :placeholder="t('history.range.start.placeholder')"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-[10px] text-ink-muted">{{ t("history.range.end") }}</span>
          <input
            v-model="rangeEndRef"
            type="text"
            :placeholder="t('history.range.end.placeholder')"
            @keydown.enter="handleRangeCherryPick"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </label>
</div>

      <template #footer>
        <UiButton variant="secondary" :disabled="isRangeCherryPicking" @click="showRangeCherryPickDialog = false">
          {{ t("common.cancel") }}
        </UiButton>
        <UiButton
          variant="primary"
          :disabled="isRangeCherryPicking || !rangeStartRef.trim() || !rangeEndRef.trim()"
          @click="handleRangeCherryPick"
        >
          {{ isRangeCherryPicking ? t("history.range.picking") : t("history.range.pick") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Compare 2 Selected Commits -->
    <CompareCommitsModal
      :is-open="showCompareModal"
      :repo-path="currentRepoPath"
      :base-commit="compareBaseCommit"
      :target-commit="compareTargetCommit"
      @close="showCompareModal = false"
    />

    <!-- Modal: Batch Cherry-Pick (multi-select) -->
    <UiModal
      :open="showBatchCherryPickDialog"
      :title="t('history.batch.title')"
      width="max-w-lg"
      @update:open="!isBatchCherryPicking && closeBatchCherryPickDialog()"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitCompare class="w-4 h-4 text-accent" />
          {{ t("history.batch.title") }}
        </h3>
      </template>

      <div class="space-y-3"><p v-if="!batchCherryPickResult" class="text-[11px] text-ink-muted leading-relaxed">
            {{ t("history.batch.desc.before") }} <span class="text-accent font-semibold">{{ t("history.batch.desc.order") }}</span> {{ t("history.batch.desc.after") }}
            <span class="text-ink font-mono">{{ selectedCommitsOldestFirst.length }}</span>
            {{ t("history.batch.desc.tail") }}
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
                  ? t("history.batch.interrupted")
                  : t("history.batch.done")
              }}
            </span>
            <span class="text-ink-secondary font-mono">
              {{ t("history.batch.summary", { ok: batchCherryPickResult.successCount, total: batchCherryPickResult.totalCount }) }}
            </span>
            <template v-if="batchCherryPickResult.hasConflict || batchCherryPickResult.failedCommitHash">
              <span class="text-danger font-mono break-all">
                {{ t("history.batch.failed", { hash: batchCherryPickResult.failedCommitHash || t("history.unknown") }) }}
              </span>
              <span v-if="batchCherryPickResult.errorMessage" class="text-ink-secondary whitespace-pre-wrap">
                {{ batchCherryPickResult.errorMessage }}
              </span>
              <span class="text-warning">
                {{ t("history.batch.hint") }}
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

      <template #footer>
        <span class="mr-auto text-[10px] text-ink-muted font-mono">
          {{ batchCherryPickResult || batchCherryPickError ? t("history.batch.result") : t("history.batch.apply") }}
        </span>
        <UiButton variant="secondary" :disabled="isBatchCherryPicking" @click="closeBatchCherryPickDialog">
          {{ batchCherryPickResult || batchCherryPickError ? t("common.close") : t("common.cancel") }}
        </UiButton>
        <UiButton
          v-if="!batchCherryPickResult && !batchCherryPickError"
          variant="primary"
          :disabled="isBatchCherryPicking || selectedCommitsOldestFirst.length === 0"
          @click="handleBatchCherryPick"
        >
          <RefreshCw v-if="isBatchCherryPicking" class="w-3.5 h-3.5 animate-spin" />
          <span>{{ isBatchCherryPicking ? t("history.batch.running") : t("history.batch.confirm") }}</span>
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Interactive Rebase Planner -->
    <UiModal
      :open="showRebaseDialog"
      :title="t('history.rebase.title')"
      width="max-w-2xl"
      @update:open="!isExecutingRebase && closeRebaseDialog()"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-cyan" />
          {{ t("history.rebase.title") }}
        </h3>
      </template>

      <div class="space-y-3"><div class="flex items-start gap-2 p-2.5 rounded-lg bg-warning/10 border border-warning/30 text-[11px] text-ink-secondary leading-relaxed">
            <AlertTriangle class="w-3.5 h-3.5 text-warning shrink-0 mt-0.5" />
            <span>
              {{ t("history.rebase.desc") }}
            </span>
          </div>

          <div class="flex items-center gap-2 flex-wrap">
            <label class="text-[11px] text-ink-muted shrink-0">{{ t("history.rebase.base.label") }}</label>
            <select
              v-model="rebaseBase"
              :disabled="isLoadingRebasePlan || isExecutingRebase"
              class="flex-1 min-w-[200px] px-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent cursor-pointer transition disabled:opacity-60"
            >
              <option v-if="rebaseBaseOptions.length === 0" value="">{{ t("history.rebase.base.none") }}</option>
              <option v-for="option in rebaseBaseOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <button :aria-label="t('history.rebase.reload.title')"
              @click="loadRebasePlan"
              :disabled="isLoadingRebasePlan || isExecutingRebase"
              class="p-1 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
              :title="t('history.rebase.reload.title')"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingRebasePlan }" />
            </button>
          </div>

          <div class="flex items-center justify-between gap-2 text-[10px] text-ink-muted flex-wrap">
            <span>
              {{ t("history.rebase.plan.count") }}<span class="text-ink font-mono">{{ rebasePlan.length }}</span>{{ t("history.rebase.plan.hint") }}
            </span>
            <span v-if="selectionCount > 0">{{ t("history.rebase.plan.selected", { count: selectionCount }) }}</span>
          </div>

          <div
            v-if="isLoadingRebasePlan"
            class="py-10 text-center text-xs text-ink-muted flex flex-col items-center gap-2"
          >
            <RefreshCw class="w-4 h-4 animate-spin text-accent" />
            <span>{{ t("history.rebase.plan.loading") }}</span>
          </div>
          <div
            v-else-if="rebasePlanError"
            class="rounded-lg border border-danger/30 bg-danger/10 p-2.5 text-[11px] text-danger whitespace-pre-wrap"
          >
            {{ rebasePlanError }}
          </div>
          <div v-else-if="rebasePlan.length === 0" class="py-10 text-center text-xs text-ink-muted">
            {{ t("history.rebase.plan.empty") }}
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
                {{ t("history.rebase.plan.badge") }}
              </span>
            </div>
          </div>

          <div
            v-if="rebaseError"
            class="rounded-lg border border-danger/30 bg-danger/10 p-2.5 text-[11px] text-danger whitespace-pre-wrap max-h-32 overflow-y-auto"
          >
            {{ rebaseError }}
            <p class="mt-1 text-ink-muted">
              {{ t("history.rebase.hint") }}
            </p>
          </div>
        </div>

      <template #footer>
        <span class="mr-auto text-[10px] text-ink-muted font-mono truncate max-w-[55%]" :title="rebaseBase">
          {{ t("history.rebase.base.current", { base: rebaseBase || t("history.rebase.base.unset") }) }}
        </span>
        <UiButton variant="secondary" :disabled="isExecutingRebase" @click="closeRebaseDialog">
          {{ t("history.rebase.cancel") }}
        </UiButton>
        <UiButton
          variant="primary"
          :disabled="isExecutingRebase || isLoadingRebasePlan || rebasePlan.length === 0 || !rebaseBase"
          @click="executeRebase"
        >
          <RefreshCw v-if="isExecutingRebase" class="w-3.5 h-3.5 animate-spin" />
          <span>{{ isExecutingRebase ? t("history.rebase.running") : t("history.rebase.execute") }}</span>
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: Git Notes Editor -->
    <UiModal
      :open="showNoteModal"
      :title="currentCommitNote ? t('history.note.edit') : t('history.note.new')"
      width="max-w-lg"
      @update:open="showNoteModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <StickyNote class="w-4 h-4 text-warning" />
          {{ currentCommitNote ? t("history.note.edit") : t("history.note.new") }}
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex items-center gap-2 text-xs font-mono text-ink-muted">
          <span>{{ t("history.note.target") }}</span>
          <span class="px-2 py-0.5 rounded bg-canvas border border-line text-accent font-bold">
            {{ selectedCommit?.shortHash }}
          </span>
          <span class="truncate">{{ selectedCommit?.subject }}</span>
        </div>

        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("history.note.body") }}</label>
          <textarea
            v-model="noteInputText"
            rows="5"
            :placeholder="t('history.note.placeholder')"
            class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-sans leading-relaxed"
          ></textarea>
        </div>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="showNoteModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="warning"
          :disabled="isSavingNote || !noteInputText.trim()"
          @click="handleSaveNote"
        >
          {{ isSavingNote ? t("history.note.saving") : t("history.note.save") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Commit Row Right-Click Context Menu -->
    <div
      v-if="showContextMenu && contextMenuCommit"
      class="fixed z-50 w-60 rounded-xl bg-surface/95 backdrop-blur-xl border border-line shadow-2xl py-1 text-xs select-none animate-in fade-in zoom-in-95 duration-100"
      :style="{ left: contextMenuPos.x + 'px', top: contextMenuPos.y + 'px' }"
      @click.stop
    >
      <div class="px-3 py-1 border-b border-line/60 flex items-center justify-between text-[10px] text-ink-muted">
        <span class="font-mono font-semibold text-accent">{{ contextMenuCommit.shortHash }}</span>
        <span>{{ t("history.menu.quick") }}</span>
      </div>

      <div class="py-0.5">
        <button
          type="button"
          @click="handleRebaseOntoCommit(contextMenuCommit); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-warning/15 hover:text-warning text-ink text-left transition cursor-pointer font-medium"
        >
          <GitPullRequest class="w-3.5 h-3.5 text-warning shrink-0" />
          <span>{{ t("history.menu.rebase.onto") }}</span>
        </button>

        <button
          type="button"
          @click="openRebaseDialog(contextMenuCommit.hash); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-cyan/15 hover:text-cyan text-ink text-left transition cursor-pointer font-medium"
        >
          <GitMerge class="w-3.5 h-3.5 text-cyan shrink-0" />
          <span>{{ t("history.menu.rebase.interactive") }}</span>
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
          <span>{{ t("history.menu.cherry.pick") }}</span>
        </button>

        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; handleRevert(); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <RotateCcw class="w-3.5 h-3.5 text-warning shrink-0" />
          <span>{{ t("history.menu.revert") }}</span>
        </button>

        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; showNewBranchDialog = true; closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <GitBranch class="w-3.5 h-3.5 text-cyan shrink-0" />
          <span>{{ t("history.menu.branch") }}</span>
        </button>

        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; showNewTagDialog = true; closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <Tag class="w-3.5 h-3.5 text-purple shrink-0" />
          <span>{{ t("history.menu.tag") }}</span>
        </button>

        <button
          type="button"
          @click="selectedCommit = contextMenuCommit; showResetDialog = true; closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-danger/15 text-danger text-left transition cursor-pointer"
        >
          <AlertTriangle class="w-3.5 h-3.5 shrink-0" />
          <span>{{ t("history.menu.reset") }}</span>
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
          <span>{{ t("history.menu.copy.hash") }}</span>
        </button>

        <button
          type="button"
          @click="copyText(contextMenuCommit.subject, t('history.toast.subject.copied')); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <FileText class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span>{{ t("history.menu.copy.subject") }}</span>
        </button>

        <button
          type="button"
          @click="handleOpenCommitInWeb(contextMenuCommit.hash); closeContextMenu()"
          class="w-full px-3 py-1.5 flex items-center gap-2 hover:bg-raised text-ink text-left transition cursor-pointer"
        >
          <ExternalLink class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span>{{ t("history.menu.open.web") }}</span>
        </button>
      </div>
    </div>

    <!-- Modal: Commit signature details -->
    <UiModal
      :open="showSignatureModal"
      :title="t('history.signature.title')"
      width="max-w-lg"
      @update:open="showSignatureModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <ShieldCheck class="w-4 h-4 text-success" />
          {{ t("history.signature.title") }}
        </h3>
      </template>

      <div class="space-y-3 text-xs">
          <div v-if="isLoadingSignature" class="py-8 text-center text-ink-muted">{{ t("history.signature.loading") }}</div>
          <template v-else-if="signatureDetails">
            <div class="flex items-center gap-2">
              <span class="px-2 py-0.5 rounded text-[11px] font-bold" :class="signatureStatusMeta(signatureDetails.status).cls">
                {{ signatureStatusMeta(signatureDetails.status).label }}
              </span>
              <span class="text-[10px] text-ink-muted font-mono">%G? = {{ signatureDetails.status }}</span>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div class="p-2.5 rounded-lg bg-canvas/40 border border-line">
                <div class="text-[10px] text-ink-muted">{{ t("history.signature.signer") }}</div>
                <div class="text-ink font-medium truncate">{{ signatureDetails.signer || "—" }}</div>
              </div>
              <div class="p-2.5 rounded-lg bg-canvas/40 border border-line">
                <div class="text-[10px] text-ink-muted">{{ t("history.signature.key") }}</div>
                <div class="font-mono text-ink truncate">{{ signatureDetails.keyId || "—" }}</div>
              </div>
              <div class="p-2.5 rounded-lg bg-canvas/40 border border-line col-span-2">
                <div class="text-[10px] text-ink-muted">{{ t("history.signature.fingerprint") }}</div>
                <div class="font-mono text-[10px] text-ink-secondary break-all">{{ signatureDetails.fingerprint || "—" }}</div>
              </div>
              <div class="p-2.5 rounded-lg bg-canvas/40 border border-line col-span-2">
                <div class="text-[10px] text-ink-muted">{{ t("history.signature.signed.at") }}</div>
                <div class="text-ink-secondary">{{ signatureDetails.date || "—" }}</div>
              </div>
            </div>
            <div v-if="signatureDetails.raw" class="p-2.5 rounded-lg bg-canvas/60 border border-line font-mono text-[10px] text-ink-muted whitespace-pre-wrap max-h-40 overflow-y-auto">
              {{ signatureDetails.raw }}
            </div>
            <p v-if="signatureDetails.status === 'N'" class="text-[11px] text-ink-muted">
              {{ t("history.signature.none.hint") }}
            </p>
          </template></div>
    </UiModal>

    <!-- Modal: Git Notes overview -->
    <UiModal
      :open="showNotesOverview"
      :title="t('history.notes.title')"
      width="max-w-2xl"
      @update:open="showNotesOverview = $event"
    >
      <template #header>
        <div class="flex items-center gap-2 min-w-0">
          <StickyNote class="w-4 h-4 text-warning shrink-0" />
          <h3 class="text-sm font-bold text-ink whitespace-nowrap">{{ t("history.notes.title") }}</h3>
          <span v-if="notesOverview" class="text-[10px] text-ink-muted font-mono truncate">
            {{ t("history.notes.summary", { count: notesOverview.total, ref: notesOverview.ref }) }}
          </span>
        </div>
      </template>

      <div class="space-y-2">
          <div v-if="isLoadingNotesOverview" class="py-10 text-center text-xs text-ink-muted">{{ t("history.notes.loading") }}</div>
          <div v-else-if="notesOverview && notesOverview.notes.length === 0" class="py-10 text-center text-xs text-ink-muted">
            {{ t("history.notes.empty") }}
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
    </UiModal>

    <!-- Modal: Pickaxe content archaeology -->
    <UiModal
      :open="showPickaxeModal"
      :title="t('history.pickaxe.title')"
      width="max-w-3xl"
      @update:open="showPickaxeModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Search class="w-4 h-4 text-purple" />
          {{ t("history.pickaxe.title") }}
        </h3>
      </template>

      <div class="space-y-3">
          <div class="flex flex-wrap items-center gap-2">
            <input
              v-model="pickaxeQuery"
              type="text"
              :placeholder="t('history.pickaxe.query.placeholder')"
              @keydown.enter="handlePickaxeSearch"
              class="flex-1 min-w-[220px] px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted font-mono focus:outline-none focus:border-accent"
            />
            <select
              v-model="pickaxeMode"
              class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
              title="content = git log -S; regex = git log -G"
            >
              <option value="content">{{ t("history.pickaxe.mode.content") }}</option>
              <option value="regex">{{ t("history.pickaxe.mode.regex") }}</option>
            </select>
            <input
              v-model="pickaxeAuthor"
              type="text"
              :placeholder="t('history.pickaxe.author.placeholder')"
              class="w-32 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <input
              v-model="pickaxePathFilter"
              type="text"
              :placeholder="t('history.pickaxe.path.placeholder')"
              class="w-40 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted font-mono focus:outline-none focus:border-accent"
            />
            <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
              <input v-model="pickaxeIgnoreCase" type="checkbox" class="rounded accent-purple" />
              <span>{{ t("history.pickaxe.ignore.case") }}</span>
            </label>
            <button
              @click="handlePickaxeSearch"
              :disabled="!pickaxeQuery.trim() || isPickaxeSearching"
              class="px-3 py-1.5 rounded-lg bg-purple-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isPickaxeSearching ? t("history.pickaxe.searching") : t("history.pickaxe.search") }}
            </button>
          </div>

          <div v-if="pickaxeResult" class="flex flex-wrap items-center gap-3 px-3 py-2 rounded-lg bg-canvas/60 border border-line text-[11px] text-ink-muted font-mono">
            <span>{{ t("history.pickaxe.hits", { hits: pickaxeResult.totalHits, analyzed: pickaxeResult.analyzedHits }) }}</span>
            <span class="text-success">+{{ pickaxeResult.addedLines }}</span>
            <span class="text-danger">-{{ pickaxeResult.removedLines }}</span>
            <span>{{ t("history.pickaxe.files", { count: pickaxeResult.fileCount }) }}</span>
            <span v-if="pickaxeResult.firstSeen">{{ t("history.pickaxe.first", { date: pickaxeResult.firstSeen }) }}</span>
            <span v-if="pickaxeResult.lastSeen">{{ t("history.pickaxe.last", { date: pickaxeResult.lastSeen }) }}</span>
            <span v-if="pickaxeResult.truncated" class="text-warning">{{ t("history.pickaxe.truncated") }}</span>
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
                  :title="t('history.pickaxe.copy.hash')"
                >
                  {{ hit.shortHash }}
                </button>
                <span
                  class="px-1.5 py-0.2 rounded text-[9px] font-bold shrink-0"
                  :class="pickaxeChangeClass(hit.change)"
                >
                  {{ pickaxeChangeLabel(hit.change) }}
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
            {{ t("history.pickaxe.none") }}
          </div>
</div>
    </UiModal>
  </div>
</template>
