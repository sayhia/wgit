<script setup lang="ts">
import { reportError } from "../composables/useToast";
import { t } from "../i18n";
import { confirmAction } from "../composables/useConfirm";
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import {
  GitBranch,
  GitFork,
  Tag,
  Archive,
  Plus,
  Search,
  RefreshCw,
  GitCompare,
  ArrowUpDown,
  X,
  ShieldCheck,
  AlertTriangle,
  Eraser,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import Toast from "./ui/Toast.vue";
import { useToast } from "../composables/useToast";
import { useModalA11y } from "../composables/useModalA11y";
import type {
  BranchInfo,
  TagInfo,
  StashInfo,
  BranchCompareResult,
  DiffComparisonResult,
  BranchInsight,
  BranchInsightsResult,
  MergePreviewResult,
  MergeOverlapResult,
  TagDetail,
  TagVerifyResult,
  GoneBranchInfo,
} from "../types";
import RemoteBranchList from "./branches/RemoteBranchList.vue";
import TagsList from "./branches/TagsList.vue";
import StashesList from "./branches/StashesList.vue";
import BranchCompare from "./branches/BranchCompare.vue";
import BranchRiverView from "./branches/BranchRiverView.vue";
import LocalBranchList from "./branches/LocalBranchList.vue";
import DiffViewer from "./workspace/DiffViewer.vue";
import { Loader2 } from "lucide-vue-next";
import CreateBranchModal from "./branches/modals/CreateBranchModal.vue";
import RenameBranchModal from "./branches/modals/RenameBranchModal.vue";
import MergeBranchModal from "./branches/modals/MergeBranchModal.vue";
import RebaseBranchModal from "./branches/modals/RebaseBranchModal.vue";
import SetUpstreamModal from "./branches/modals/SetUpstreamModal.vue";
import CreateTagModal from "./branches/modals/CreateTagModal.vue";
import { backendText } from "../i18n";
import CreateStashModal from "./branches/modals/CreateStashModal.vue";
import UiModal from "./ui/UiModal.vue";
import UiButton from "./ui/UiButton.vue";
import UiBadge from "./ui/UiBadge.vue";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "switch-branch", branchName: string): void;
  (e: "refresh"): void;
}>();

const activeTab = ref<"local" | "remote" | "tags" | "stashes" | "compare" | "river">("local");
let repoEpoch = 0;
const branches = ref<BranchInfo[]>([]);
const tags = ref<TagInfo[]>([]);
const stashes = ref<StashInfo[]>([]);
const goneBranches = ref<GoneBranchInfo[]>([]);
const isPruningGone = ref(false);
const isLoading = ref(false);
const searchQuery = ref("");

// ----- Branch Insights (merged / unpushed / stale analysis) -----
type BranchSortMode = "recent" | "name" | "ahead";
const sortMode = ref<BranchSortMode>("recent");
const insights = ref<BranchInsightsResult | null>(null);
const insightsError = ref("");
const isLoadingInsights = ref(false);
const isCleaningMerged = ref(false);
let insightRequestSeq = 0;
let insightsRepoPath = "";

// ----- Large list progressive rendering (hundreds of remote branches) -----
type ListTab = "local" | "remote" | "tags" | "stashes";
const PAGE_SIZE = 400;
const visibleCounts = ref<Record<ListTab, number>>({
  local: PAGE_SIZE,
  remote: PAGE_SIZE,
  tags: PAGE_SIZE,
  stashes: PAGE_SIZE,
});

// Branch Comparison State
const compareBaseBranch = ref("");
const compareTargetBranch = ref("");
const compareResult = ref<BranchCompareResult | null>(null);
const compareDiff = ref<DiffComparisonResult | null>(null);
const renameUpdateRemote = ref(false);
const isLoadingCompare = ref(false);
const isLoadingCompareDiff = ref(false);

/** false = two-dot A..B tree compare, true = three-dot A...B merge-base compare. */
const compareMergeBase = ref(false);

function handleOpenCompareFile(path: string) {
  showCompareFileDiff.value = false;
  window.dispatchEvent(new CustomEvent("wgit:open-file", { detail: { path } }));
  notify(t("branch.toast.diff.opened", { path }));
}

// ---- Single-file diff preview between the compared refs ----
const showCompareFileDiff = ref(false);
const compareFileDiffPath = ref("");
const compareFileDiffText = ref("");
const compareFileDiffError = ref("");
const isLoadingCompareFileDiff = ref(false);

async function handleShowCompareFileDiff(path: string) {
  if (!props.currentRepoPath) return;
  compareFileDiffPath.value = path;
  compareFileDiffText.value = "";
  compareFileDiffError.value = "";
  showCompareFileDiff.value = true;
  isLoadingCompareFileDiff.value = true;
  try {
    const text = await gitService.getFileDiffBetween(
      props.currentRepoPath,
      compareBaseBranch.value,
      compareTargetBranch.value,
      path,
      compareMergeBase.value
    );
    compareFileDiffText.value = text || "";
  } catch (err: any) {
    compareFileDiffError.value = t("branch.toast.diff.failed", { error: err?.message || err });
  } finally {
    isLoadingCompareFileDiff.value = false;
  }
}

function closeCompareFileDiff() {
  showCompareFileDiff.value = false;
}

/** Suggestions for the compare comboboxes: local/remote branches + tags. */
const compareRefOptions = computed(() => {
  const names = [...branches.value.map((b) => b.name), ...tags.value.map((t) => t.name)];
  return [...new Set(names)].sort((a, b) => a.localeCompare(b));
});

/** Command palette / cross-view deep link: jump straight to the compare panel. */
const BRANCHES_TABS = ["local", "remote", "tags", "stashes", "compare"] as const;

function handleBranchesTabEvent(event: Event) {
  const detail = (event as CustomEvent<{ tab?: string }>).detail;
  const tab = detail?.tab;
  if (!tab || !(BRANCHES_TABS as readonly string[]).includes(tab)) return;
  activeTab.value = tab as (typeof BRANCHES_TABS)[number];
  if (tab === "compare") void runBranchCompare();
}

onMounted(() => window.addEventListener("wgit:branches-tab", handleBranchesTabEvent));
onUnmounted(() => window.removeEventListener("wgit:branches-tab", handleBranchesTabEvent));

async function runBranchCompare() {
  if (!props.currentRepoPath) return;
  if (!compareBaseBranch.value) {
    compareBaseBranch.value = currentBranchName.value || "main";
  }
  if (!compareTargetBranch.value) {
    const other = branches.value.find((b) => b.name !== compareBaseBranch.value);
    compareTargetBranch.value = other ? other.name : "main";
  }
  const repoPath = props.currentRepoPath;
  const epoch = repoEpoch;
  isLoadingCompare.value = true;
  compareDiff.value = null;
  try {
    const res = await gitService.compareBranches(repoPath, compareBaseBranch.value, compareTargetBranch.value);
    if (epoch === repoEpoch) compareResult.value = res;
  } catch (err: any) {
    if (epoch === repoEpoch) notify(t("branch.toast.compare.failed", { error: err?.message || err }));
  } finally {
    if (epoch === repoEpoch) isLoadingCompare.value = false;
  }
}

async function runCompareDiff() {
  if (!props.currentRepoPath || isLoadingCompareDiff.value) return;
  if (!compareBaseBranch.value || !compareTargetBranch.value) {
    notify(t("branch.toast.compare.refs.required"));
    return;
  }
  const epoch = repoEpoch;
  isLoadingCompareDiff.value = true;
  try {
    const compareFn = compareMergeBase.value
      ? gitService.getMergeBaseDiff
      : gitService.getDiffBetween;
    const diff = await compareFn(
      props.currentRepoPath,
      compareBaseBranch.value,
      compareTargetBranch.value
    );
    if (epoch !== repoEpoch) return;
    compareDiff.value = diff;
    if (compareDiff.value) {
      notify(
        t("branch.toast.diff.loaded", { files: compareDiff.value.files.length, additions: compareDiff.value.additions, deletions: compareDiff.value.deletions })
      );
    }
  } catch (err: any) {
    if (epoch === repoEpoch) notify(t("branch.toast.diff.read.failed", { error: err?.message || err }));
  } finally {
    if (epoch === repoEpoch) isLoadingCompareDiff.value = false;
  }
}

watch(activeTab, (tab) => {
  if (tab === "compare") {
    runBranchCompare();
  }
});

watch(compareMergeBase, () => {
  if (compareDiff.value) void runCompareDiff();
});

async function handleOpenCompareInWeb() {
  if (!props.currentRepoPath) return;
  try {
    await gitService.openCompareInWeb(
      props.currentRepoPath,
      compareBaseBranch.value,
      compareTargetBranch.value
    );
    notify(t("branch.toast.web.opened"));
  } catch (err: any) {
    notify(t("branch.toast.pr.failed", { error: err?.message || err }));
  }
}

async function handleOpenBranchInWeb(branchName: string) {
  if (!props.currentRepoPath || !branchName) return;
  try {
    await gitService.openBranchInWeb(props.currentRepoPath, branchName);
    notify(t("branch.toast.branch.web", { branch: branchName }));
  } catch (err: any) {
    notify(t("branch.toast.branch.web.failed", { error: err?.message || err }));
  }
}

// Reset progressive rendering whenever the visible dataset changes
watch([activeTab, searchQuery], () => {
  visibleCounts.value = { local: PAGE_SIZE, remote: PAGE_SIZE, tags: PAGE_SIZE, stashes: PAGE_SIZE };
});

// Toast Feedback System
const { toastText: toastMessage, showToast, notify } = useToast();
// Set Upstream Modal
const showSetUpstreamModal = ref(false);
const upstreamTargetBranch = ref<BranchInfo | null>(null);
const availableRemoteBranches = computed(() =>
  remoteBranches.value.map((b) => b.name)
);

// New Branch Modal
const showCreateBranchModal = ref(false);
const newBranchName = ref("");

// Rename Branch Modal
const showRenameBranchModal = ref(false);
const renameTargetBranch = ref("");
const renameNewName = ref("");

// Merge Modal
const showMergeModal = ref(false);
const mergeSourceBranch = ref("");
const mergeMode = ref<"default" | "no-ff" | "squash" | "ff-only">("no-ff");
const mergeCommitMessage = ref("");
const mergeStrategyOption = ref<"" | "ours" | "theirs">("");
const mergeAutostash = ref(true);
const mergePreview = ref<MergePreviewResult | null>(null);
const isLoadingMergePreview = ref(false);
const mergeOverlap = ref<MergeOverlapResult | null>(null);
const isLoadingMergeOverlap = ref(false);
/** Non-empty when one/both pre-merge checks failed for the current modal. */
const mergePreviewError = ref("");
const isMerging = ref(false);

// Rebase Modal
const showRebaseModal = ref(false);
const rebaseUpstreamBranch = ref("");
const rebaseOnto = ref("");
const rebaseAutostash = ref(true);
const rebaseAutosquash = ref(true);
const isRebasing = ref(false);

// Create Tag Modal
const showCreateTagModal = ref(false);
const newTagName = ref("");
const newTagMessage = ref("");
const newTagSign = ref(false);

const showTagDetailModal = ref(false);
const tagDetail = ref<TagDetail | null>(null);
const isLoadingTagDetail = ref(false);
const tagVerifyResult = ref<TagVerifyResult | null>(null);
const isVerifyingTag = ref(false);

async function handleVerifyTag() {
  if (!props.currentRepoPath || !tagDetail.value || isVerifyingTag.value) return;
  isVerifyingTag.value = true;
  tagVerifyResult.value = null;
  try {
    const res = await gitService.verifyTagSignature(props.currentRepoPath, tagDetail.value.name);
    tagVerifyResult.value = res;
    notify(backendText(res.messageKey, res.message, res.params));
  } catch (err: any) {
    notify(t("branch.toast.tag.verify.failed", { error: err?.message || err }));
  } finally {
    isVerifyingTag.value = false;
  }
}

async function handleShowTagDetail(tagName: string) {
  if (!props.currentRepoPath) return;
  showTagDetailModal.value = true;
  tagDetail.value = null;
  tagVerifyResult.value = null;
  isLoadingTagDetail.value = true;
  try {
    tagDetail.value = await gitService.getTagDetail(props.currentRepoPath, tagName);
  } catch (err: any) {
    showTagDetailModal.value = false;
    notify(t("branch.toast.tag.detail.failed", { error: err?.message || err }));
  } finally {
    isLoadingTagDetail.value = false;
  }
}

// Create Stash Modal
const showCreateStashModal = ref(false);
const newStashMessage = ref("");
const stashIncludeUntracked = ref(true);
const stashIncludeAll = ref(false);
const stashKeepIndex = ref(false);

// Stash Branch Modal
const showStashBranchModal = ref(false);
const stashBranchTarget = ref<StashInfo | null>(null);
const stashBranchName = ref("");
const isStashBranching = ref(false);

function handleStashBranch(stash: StashInfo) {
  stashBranchTarget.value = stash;
  stashBranchName.value = `stash-${stash.index}-recovery`;
  showStashBranchModal.value = true;
}

async function handleConfirmStashBranch() {
  if (!props.currentRepoPath || !stashBranchTarget.value || !stashBranchName.value.trim()) return;
  isStashBranching.value = true;
  try {
    await gitService.stashBranch(
      props.currentRepoPath,
      stashBranchTarget.value.index,
      stashBranchName.value.trim()
    );
    notify(
      t("branches.stash.toast.branch.created", {
        index: stashBranchTarget.value.index,
        branch: stashBranchName.value.trim(),
      })
    );
    showStashBranchModal.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branches.stash.toast.branch.failed", { error: err?.message || err }));
  } finally {
    isStashBranching.value = false;
  }
}

const localBranches = computed(() =>
  branches.value.filter((b) => !b.isRemote)
);

const remoteBranches = computed(() =>
  branches.value.filter((b) => b.isRemote)
);

const currentBranchName = computed(() => {
  const cur = branches.value.find((b) => b.isCurrent);
  return cur ? cur.name : "main";
});

// ----- Insight lookup / sorting / filtering -----
const insightMap = computed<Map<string, BranchInsight>>(() => {
  const map = new Map<string, BranchInsight>();
  for (const item of insights.value?.items ?? []) {
    if (!item.isRemote) map.set(item.name, item);
  }
  return map;
});

const insightSummary = computed(() => insights.value?.summary ?? null);
const cleanableCount = computed(() => insightSummary.value?.cleanable ?? 0);
// Insights unavailable => keep cleanup enabled: getMergedBranches can still answer
const cleanupDisabled = computed(
  () => isCleaningMerged.value || (!!insightSummary.value && cleanableCount.value === 0)
);

/**
 * Only colours live in a module-level map. Labels and descriptions are resolved
 * per call, otherwise they would freeze in whichever language was active when
 * this module was imported.
 */
const CATEGORY_STYLES: Record<string, string> = {
  current: "bg-accent/15 text-accent border-accent/30",
  protected: "bg-raised text-ink-secondary border-line",
  merged: "bg-success/15 text-success border-success/30",
  unpushed: "bg-warning/15 text-warning border-warning/30",
  stale: "bg-cyan/15 text-cyan border-cyan/30",
  active: "bg-surface text-ink-muted border-line",
};

function categoryMeta(category?: string | null) {
  if (!category) return null;
  const cls = CATEGORY_STYLES[category];
  if (!cls) return { label: category, cls: "bg-surface text-ink-muted border-line", desc: "" };
  return { label: t(`branch.cat.${category}`), cls, desc: t(`branch.cat.${category}.desc`) };
}

/** Parse insight lastDate (git authordate:short, e.g. 2026-01-02) defensively. */
function parseInsightDate(raw: string): number {
  const direct = Date.parse(raw);
  if (!Number.isNaN(direct)) return direct;
  const isoish = raw.trim().replace(/\s+/, "T").replace(/([+-]\d{2})(\d{2})$/, "$1:$2");
  const parsed = Date.parse(isoish);
  if (!Number.isNaN(parsed)) return parsed;
  const dayOnly = Date.parse(raw.slice(0, 10));
  return Number.isNaN(dayOnly) ? 0 : dayOnly;
}

function insightTimestamp(item: BranchInsight | undefined): number {
  if (!item?.lastDate) return 0;
  return parseInsightDate(item.lastDate);
}

function isCleanupCandidate(name: string): boolean {
  const category = insightMap.value.get(name)?.category;
  return category === "merged" || category === "stale";
}

function defaultUpstreamFor(branchName: string): string {
  const tracked = branches.value.map((b) => b.upstream).find((u) => !!u && u.includes("/"));
  const prefix = tracked ? tracked.split("/")[0] : remoteBranches.value[0]?.name.split("/")[0];
  return `${prefix || "origin"}/${branchName}`;
}

function matchesSearch(name: string): boolean {
  const q = searchQuery.value.trim().toLowerCase();
  return !q || name.toLowerCase().includes(q);
}

const sortedLocalBranches = computed<BranchInfo[]>(() => {
  const list = [...localBranches.value];
  list.sort((a, b) => {
    // Current branch is always pinned first
    if (a.isCurrent !== b.isCurrent) return a.isCurrent ? -1 : 1;
    if (sortMode.value === "name") return a.name.localeCompare(b.name);
    if (sortMode.value === "ahead") {
      return (b.ahead || 0) - (a.ahead || 0) || a.name.localeCompare(b.name);
    }
    // 最近活动：优先使用洞察数据中的最后提交时间，缺失时回退到名称顺序
    const ta = insightTimestamp(insightMap.value.get(a.name));
    const tb = insightTimestamp(insightMap.value.get(b.name));
    if (ta !== tb) return tb - ta;
    // 日期不可解析时用 daysSinceCommit 兜底（越小越新）
    const ia = insightMap.value.get(a.name);
    const ib = insightMap.value.get(b.name);
    const da = ia?.daysSinceCommit;
    const db = ib?.daysSinceCommit;
    if (typeof da === "number" && typeof db === "number" && da !== db) return da - db;
    return a.name.localeCompare(b.name);
  });
  return list;
});

const filteredLocalBranches = computed(() => sortedLocalBranches.value.filter((b) => matchesSearch(b.name)));
const filteredRemoteBranches = computed(() => remoteBranches.value.filter((b) => matchesSearch(b.name)));

const filteredTags = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  return tags.value.filter((t) => t.name.toLowerCase().includes(q));
});

const filteredStashes = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  return stashes.value.filter((s) => s.message.toLowerCase().includes(q));
});

// Progressive rendering: only render the first PAGE_SIZE rows per tab
const visibleLocalBranches = computed(() => filteredLocalBranches.value.slice(0, visibleCounts.value.local));
const visibleRemoteBranches = computed(() => filteredRemoteBranches.value.slice(0, visibleCounts.value.remote));
const visibleTags = computed(() => filteredTags.value.slice(0, visibleCounts.value.tags));
const visibleStashes = computed(() => filteredStashes.value.slice(0, visibleCounts.value.stashes));

interface LocalBranchRow {
  branch: BranchInfo;
  insight: BranchInsight | null;
  meta: { label: string; cls: string; desc: string } | null;
  cleanupCandidate: boolean;
}

const visibleLocalRows = computed<LocalBranchRow[]>(() =>
  visibleLocalBranches.value.map((branch) => {
    const insight = insightMap.value.get(branch.name) ?? null;
    return {
      branch,
      insight,
      meta: insight ? categoryMeta(insight.category) : null,
      cleanupCandidate: isCleanupCandidate(branch.name),
    };
  })
);

const localMoreCount = computed(() => Math.max(0, filteredLocalBranches.value.length - visibleLocalBranches.value.length));
const remoteMoreCount = computed(() => Math.max(0, filteredRemoteBranches.value.length - visibleRemoteBranches.value.length));
const tagsMoreCount = computed(() => Math.max(0, filteredTags.value.length - visibleTags.value.length));
const stashesMoreCount = computed(() => Math.max(0, filteredStashes.value.length - visibleStashes.value.length));

function showMore(tab: ListTab) {
  visibleCounts.value[tab] += PAGE_SIZE;
}

async function loadData() {
  if (!props.currentRepoPath) return;
  const repoPath = props.currentRepoPath;
  const epoch = repoEpoch;
  isLoading.value = true;
  try {
    const [bRes, tRes, sRes, gRes] = await Promise.allSettled([
      gitService.getBranches(repoPath),
      gitService.getTags(repoPath),
      gitService.getStashes(repoPath),
      gitService.getGoneLocalBranches(repoPath),
    ]);
    if (epoch !== repoEpoch) return;
    const errors: string[] = [];
    if (bRes.status === "fulfilled") {
      if (bRes.value) branches.value = bRes.value;
    } else {
      errors.push(String((bRes.reason as any)?.message || bRes.reason));
    }
    if (tRes.status === "fulfilled") {
      tags.value = tRes.value || [];
    } else {
      errors.push(String((tRes.reason as any)?.message || tRes.reason));
    }
    if (sRes.status === "fulfilled") {
      stashes.value = sRes.value || [];
    } else {
      errors.push(String((sRes.reason as any)?.message || sRes.reason));
    }
    if (gRes.status === "fulfilled") {
      goneBranches.value = gRes.value || [];
    }
    if (errors.length > 0) {
      notify(t("branch.toast.load.failed", { error: errors.join("; ") }));
    }
    // Insights are computed from many git calls: load them in the background so the
    // branch list renders immediately and never gets blocked by a slow analysis.
    void loadInsights();
  } finally {
    if (epoch === repoEpoch) isLoading.value = false;
  }
}

async function loadInsights() {
  const repoPath = props.currentRepoPath;
  if (!repoPath) {
    insights.value = null;
    insightsError.value = "";
    insightsRepoPath = "";
    return;
  }
  // Drop stale data immediately when the repository changes
  if (insightsRepoPath !== repoPath) {
    insights.value = null;
    insightsError.value = "";
    insightsRepoPath = repoPath;
  }
  const requestId = ++insightRequestSeq;
  isLoadingInsights.value = true;
  insightsError.value = "";
  try {
    const result = await gitService.getBranchInsights(repoPath, "");
    if (requestId !== insightRequestSeq) return;
    insights.value = result ?? null;
    if (!result) {
      insightsError.value = t("branch.error.insights.none");
    }
  } catch (err: any) {
    if (requestId !== insightRequestSeq) return;
    insights.value = null;
    insightsError.value = t("branch.error.insights.failed", { error: err?.message || err });
  } finally {
    if (requestId === insightRequestSeq) isLoadingInsights.value = false;
  }
}

// Checkout branch
async function handleCheckout(branchName: string) {
  if (!props.currentRepoPath) return;

  try {
    const engineStatus = await gitService.detectRunningGameEngines(props.currentRepoPath);
    if (engineStatus?.isRunning && !engineStatus.canSafelyCheckout) {
      const proceed = await confirmAction({
        message: `${engineStatus.warning}\n${engineStatus.recommendation}`,
        tone: "warning",
      });
      if (!proceed) return;
    }
  } catch (err) {
    // best-effort: the guard is advisory; the action proceeds and reports itself.
    console.debug("Engine guard pre-check failed:", err);
  }

  try {
    await gitService.checkoutBranch(props.currentRepoPath, branchName);
    await loadData();
    const checkedOut = branches.value.find((b) => b.isCurrent)?.name || branchName;
    notify(t("branch.toast.checked.out", { branch: checkedOut }));
    emit("switch-branch", checkedOut);
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.checkout.failed", { error: err?.message || err }));
  }
}

// Create branch
async function handleCreateBranch() {
  if (!newBranchName.value.trim() || !props.currentRepoPath) return;
  try {
    await gitService.createBranch(props.currentRepoPath, newBranchName.value.trim());
    notify(t("branch.toast.created", { name: newBranchName.value.trim() }));
    showCreateBranchModal.value = false;
    newBranchName.value = "";
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.create.failed", { error: err?.message || err }));
  }
}

// Delete branch
async function handleDeleteBranch(branchName: string, force = false) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t(force ? "branch.confirm.delete.force" : "branch.confirm.delete", { name: branchName }), tone: "danger" }))) return;
  try {
    await gitService.deleteBranch(props.currentRepoPath, branchName, force);
    notify(t("branch.toast.deleted", { name: branchName }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.delete.failed", { error: err?.message || err }));
  }
}

// Bulk prune branches whose upstream is [gone]
async function handlePruneGoneBranches() {
  if (!props.currentRepoPath || goneBranches.value.length === 0 || isPruningGone.value) return;
  const names = goneBranches.value.map((b) => b.name);
  if (
    !(await confirmAction({
      message: t("branch.confirm.prune.gone", { count: names.length, list: names.join(", ") }),
      tone: "danger",
    }))
  )
    return;

  isPruningGone.value = true;
  try {
    let deletedCount = 0;
    for (const name of names) {
      try {
        await gitService.deleteBranch(props.currentRepoPath, name, true);
        deletedCount++;
      } catch {
        // best-effort
      }
    }
    notify(t("branch.toast.pruned.gone", { count: deletedCount }));
    goneBranches.value = [];
    await loadData();
    emit("refresh");
  } finally {
    isPruningGone.value = false;
  }
}

// Bulk cleanup: delete all local branches fully merged into the base branch
async function handleCleanupMerged() {
  const repoPath = props.currentRepoPath;
  if (!repoPath || isCleaningMerged.value) return;
  const base = insightSummary.value?.baseBranch || currentBranchName.value || "main";
  isCleaningMerged.value = true;
  try {
    const merged = await gitService.getMergedBranches(repoPath, base);
    const candidates = (merged || []).filter((name) => !!name && name !== currentBranchName.value);
    if (candidates.length === 0) {
      notify(t("branch.toast.cleanup.none", { base }));
      return;
    }
    const preview = candidates.slice(0, 15).join("\n");
    const overflow = candidates.length > 15 ? t("branch.cleanup.overflow", { count: candidates.length - 15 }) : "";
    const ok = await confirmAction(
      { message: t("branch.confirm.cleanup", { count: candidates.length, base, preview, overflow }), tone: "danger" }
    );
    if (!ok) return;
    const deleted = await gitService.deleteMergedBranches(repoPath, candidates);
    notify(t("branch.toast.cleanup.done", { count: deleted?.length ?? candidates.length }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.cleanup.failed", { error: err?.message || err }));
  } finally {
    isCleaningMerged.value = false;
  }
}

// Per-branch cleanup for stale / already-merged branches
async function handleCleanupBranch(branch: BranchInfo) {
  const repoPath = props.currentRepoPath;
  if (!repoPath) return;
  const insight = insightMap.value.get(branch.name);
  const meta = categoryMeta(insight?.category);
  // The category description already *is* the reason/suggestion; the backend's
  // per-branch `suggestion` is Chinese-only, so it is no longer rendered.
  const reason = meta ? t("branch.cleanup.reason", { text: meta.desc }) : "";
  const ok = await confirmAction(
    { message: t("branch.confirm.cleanup.one", { name: branch.name, meta: meta ? t("branch.cleanup.meta", { label: meta.label }) : "", reason }), tone: "danger" }
  );
  if (!ok) return;
  try {
    await gitService.deleteBranch(repoPath, branch.name, false);
    notify(t("branch.toast.cleaned", { name: branch.name }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    const message = err?.message || String(err);
    if (insight?.category === "stale") {
      if (!(await confirmAction({ message: t("branch.confirm.force.delete", { name: branch.name }), tone: "danger", requireText: branch.name, requireTextHint: t("branch.confirm.force.delete.hint", { name: branch.name }) }))) return;
      try {
        await gitService.deleteBranch(repoPath, branch.name, true);
        notify(t("branch.toast.force.deleted", { name: branch.name }));
        await loadData();
        emit("refresh");
      } catch (forceErr: any) {
        notify(t("branch.toast.delete.failed", { error: forceErr?.message || forceErr }));
      }
      return;
    }
    notify(t("branch.toast.delete.failed", { error: message }));
  }
}

// Upstream tracking configuration
function handleSetUpstream(branch: BranchInfo) {
  upstreamTargetBranch.value = branch;
  showSetUpstreamModal.value = true;
}

function handleCloseSetUpstreamModal() {
  showSetUpstreamModal.value = false;
  upstreamTargetBranch.value = null;
}

async function handleSubmitSetUpstream(upstream: string) {
  const branch = upstreamTargetBranch.value;
  const repoPath = props.currentRepoPath;
  if (!branch || !repoPath) return;
  showSetUpstreamModal.value = false;
  try {
    await gitService.setUpstream(repoPath, branch.name, upstream);
    notify(t("branch.toast.upstream.set", { name: branch.name, upstream }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.upstream.failed", { error: err?.message || err }));
  }
}

async function handleUnsetUpstream(branch: BranchInfo) {
  const repoPath = props.currentRepoPath;
  if (!repoPath || !branch.name) return;
  try {
    await gitService.unsetUpstream(repoPath, branch.name);
    notify(t("branch.toast.upstream.unset", { name: branch.name }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.upstream.unset.failed", { error: err?.message || err }));
  }
}

async function handleDeleteRemoteBranch(branchName: string) {
  const repoPath = props.currentRepoPath;
  if (!repoPath || !branchName) return;
  if (
    !(await confirmAction({
      message: t("branch.confirm.delete.remote", { name: branchName }),
      tone: "danger",
    }))
  ) {
    return;
  }
  try {
    await gitService.deleteRemoteBranch(repoPath, branchName);
    notify(t("branch.toast.delete.remote.success", { name: branchName }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.delete.remote.failed", { error: err?.message || err }));
  }
}

// Rename branch
function openRenameBranchModal(branchName: string) {
  renameTargetBranch.value = branchName;
  renameNewName.value = branchName;
  renameUpdateRemote.value = false;
  showRenameBranchModal.value = true;
}

async function handleRenameBranch() {
  if (!renameNewName.value.trim() || !renameTargetBranch.value || !props.currentRepoPath) return;
  try {
    if (renameUpdateRemote.value) {
      if (
        !(await confirmAction(
          { message: t("branch.confirm.rename.remote", { name: renameNewName.value.trim(), old: renameTargetBranch.value }), tone: "danger" }
        ))
      )
        return;
    }
    const res = await gitService.renameBranchWithUpstream(
      props.currentRepoPath,
      renameTargetBranch.value,
      renameNewName.value.trim(),
      renameUpdateRemote.value
    );
    notify(res.message || t("branch.toast.renamed", { name: renameNewName.value.trim() }));
    showRenameBranchModal.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.rename.failed", { error: err?.message || err }));
  }
}

// Merge dialog
function describePreviewError(reason: unknown): string {
  const err = reason as any;
  return err?.message || String(reason);
}

async function openMergeModal(sourceBranch: string) {
  mergeSourceBranch.value = sourceBranch;
  mergeCommitMessage.value = `Merge branch '${sourceBranch}' into ${currentBranchName.value}`;
  mergePreview.value = null;
  mergeOverlap.value = null;
  mergePreviewError.value = "";
  showMergeModal.value = true;
  isLoadingMergePreview.value = true;
  isLoadingMergeOverlap.value = true;
  const epoch = repoEpoch;
  try {
    // Overlap analysis answers a question merge-tree cannot: WHO else is
    // editing the same files, which is the real predictor of merge pain.
    // allSettled keeps whichever half succeeded while still surfacing the
    // failure, so the modal never silently looks "clean" because a check died.
    const [previewResult, overlapResult] = await Promise.allSettled([
      gitService.previewMerge(props.currentRepoPath, sourceBranch),
      gitService.getMergeOverlapWarning(props.currentRepoPath, sourceBranch, 7),
    ]);
    const errors: string[] = [];
    const preview =
      previewResult.status === "fulfilled" ? previewResult.value : null;
    if (previewResult.status === "rejected") {
      errors.push(describePreviewError(previewResult.reason));
    }
    const overlap =
      overlapResult.status === "fulfilled" ? overlapResult.value : null;
    if (overlapResult.status === "rejected") {
      errors.push(describePreviewError(overlapResult.reason));
    }
    if (epoch !== repoEpoch) return;
    if (preview) mergePreview.value = preview;
    mergeOverlap.value = overlap;
    if (errors.length > 0) {
      mergePreviewError.value = errors.join("; ");
    }
  } catch (err) {
    reportError("Merge preview error:", err);
  } finally {
    if (epoch === repoEpoch) {
      isLoadingMergePreview.value = false;
      isLoadingMergeOverlap.value = false;
    }
  }
}

async function executeMerge() {
  if (!mergeSourceBranch.value || !props.currentRepoPath) return;
  isMerging.value = true;
  try {
    const res = await gitService.mergeAdvanced(props.currentRepoPath, {
      targetBranch: mergeSourceBranch.value,
      strategy: mergeMode.value,
      strategyOption: mergeStrategyOption.value,
      commitMessage: mergeCommitMessage.value.trim(),
      autostash: mergeAutostash.value,
    });
    if (res.hasConflicts) {
      notify(t("branch.toast.merge.conflict"));
    } else {
      notify(t("branch.toast.merge.done", { name: mergeSourceBranch.value }));
    }
    showMergeModal.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.merge.failed", { error: err?.message || err }));
  } finally {
    isMerging.value = false;
  }
}

// Rebase dialog
function openRebaseModal(upstreamBranch: string) {
  rebaseUpstreamBranch.value = upstreamBranch;
  rebaseOnto.value = "";
  showRebaseModal.value = true;
}

async function executeRebase() {
  if (!rebaseUpstreamBranch.value || !props.currentRepoPath) return;
  isRebasing.value = true;
  try {
    const res = await gitService.rebaseAdvanced(props.currentRepoPath, {
      upstream: rebaseUpstreamBranch.value,
      onto: rebaseOnto.value.trim() || undefined,
      autostash: rebaseAutostash.value,
      autosquash: rebaseAutosquash.value,
    });
    if (res.hasConflicts) {
      notify(t("branch.toast.rebase.conflict"));
    } else {
      notify(t("branch.toast.rebase.done", { upstream: rebaseUpstreamBranch.value }));
    }
    showRebaseModal.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.rebase.failed", { error: err?.message || err }));
  } finally {
    isRebasing.value = false;
  }
}

// Tags
async function handleCreateTag() {
  if (!newTagName.value.trim() || !props.currentRepoPath) return;
  const name = newTagName.value.trim();
  const message = newTagMessage.value.trim();
  try {
    if (message || newTagSign.value) {
      await gitService.createAnnotatedTag(props.currentRepoPath, name, message, newTagSign.value);
    } else {
      await gitService.createTag(props.currentRepoPath, name, "", "");
    }
    notify(t("branch.toast.tag.created", { name, signed: newTagSign.value ? t("branch.tag.signed.suffix") : "" }));
    showCreateTagModal.value = false;
    newTagName.value = "";
    newTagMessage.value = "";
    newTagSign.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.tag.create.failed", { error: err?.message || err }));
  }
}

async function handlePushTag(tagName: string) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.pushTag(props.currentRepoPath, "", tagName);
    notify(t("branch.toast.tag.pushed", { name: tagName }));
  } catch (err: any) {
    notify(t("branch.toast.tag.push.failed", { error: err?.message || err }));
  }
}

async function handleDeleteTagsBatch(tagNames: string[]) {
  if (!props.currentRepoPath || tagNames.length === 0) return;
  if (!(await confirmAction({ message: t("branch.confirm.tags.delete", { count: tagNames.length, list: tagNames.join(", ") }), tone: "danger" }))) return;
  try {
    const deleted = (await gitService.deleteBatchTags(props.currentRepoPath, tagNames, false, "")) || [];
    const removed = new Set(deleted);
    tags.value = tags.value.filter((tag) => !removed.has(tag.name));
    if (deleted.length === tagNames.length) {
      notify(t("branch.toast.tags.deleted", { count: deleted.length }));
    } else {
      notify(t("branch.toast.tags.deleted.partial", { done: deleted.length, total: tagNames.length }));
    }
  } catch (err: any) {
    notify(t("branch.toast.tags.delete.failed", { error: err?.message || err }));
  } finally {
    // Failures may still have deleted some tags; always reload the real state.
    await loadData();
    emit("refresh");
  }
}

async function handleDeleteRemoteTag(tagName: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("branch.confirm.tag.delete.remote", { name: tagName }), tone: "danger" }))) return;
  try {
    await gitService.deleteRemoteTag(props.currentRepoPath, "", tagName);
    notify(t("branch.toast.tag.remote.deleted", { name: tagName }));
  } catch (err: any) {
    notify(t("branch.toast.tag.remote.delete.failed", { error: err?.message || err }));
  }
}

async function handleDeleteTag(tagName: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("branch.confirm.tag.delete", { name: tagName }), tone: "danger" }))) return;
  try {
    await gitService.deleteTag(props.currentRepoPath, tagName);
    tags.value = tags.value.filter((tag) => tag.name !== tagName);
    notify(t("branch.toast.tag.deleted", { name: tagName }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.tag.delete.failed", { error: err?.message || err }));
  }
}

// Stashes
async function handleCreateStash() {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashSaveAdvanced(
      props.currentRepoPath,
      newStashMessage.value.trim() || `WIP on ${currentBranchName.value}`,
      stashKeepIndex.value,
      stashIncludeUntracked.value,
      stashIncludeAll.value
    );
    notify(t("branch.toast.stash.saved"));
    showCreateStashModal.value = false;
    newStashMessage.value = "";
    stashIncludeAll.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.stash.save.failed", { error: err?.message || err }));
  }
}

async function handleStashApply(index: number) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashApply(props.currentRepoPath, index);
    notify(t("branch.toast.stash.applied", { index }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.stash.apply.failed", { error: err?.message || err }));
  }
}

async function handleStashPop(index: number) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashPop(props.currentRepoPath, index);
    notify(t("branch.toast.stash.popped", { index }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.stash.pop.failed", { error: err?.message || err }));
  }
}

async function handleStashDrop(index: number) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("branch.confirm.stash.drop", { index }), tone: "danger" }))) return;
  try {
    await gitService.stashDrop(props.currentRepoPath, index);
    notify(t("branch.toast.stash.dropped", { index }));
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(t("branch.toast.stash.drop.failed", { error: err?.message || err }));
  }
}

function resetRepoScopedState() {
  activeTab.value = "local";
  isLoadingCompare.value = false;
  isLoadingCompareDiff.value = false;
  isLoadingMergePreview.value = false;
  isLoadingMergeOverlap.value = false;
  mergeOverlap.value = null;
  mergePreviewError.value = "";
  isMerging.value = false;
  isRebasing.value = false;
  showSetUpstreamModal.value = false;
  upstreamTargetBranch.value = null;
  showCreateBranchModal.value = false;
  newBranchName.value = "";
  showRenameBranchModal.value = false;
  renameTargetBranch.value = "";
  renameNewName.value = "";
  showMergeModal.value = false;
  mergeSourceBranch.value = "";
  mergePreview.value = null;
  mergeCommitMessage.value = "";
  showRebaseModal.value = false;
  rebaseUpstreamBranch.value = "";
  rebaseOnto.value = "";
  showCreateTagModal.value = false;
  newTagName.value = "";
  newTagMessage.value = "";
  newTagSign.value = false;
  showTagDetailModal.value = false;
  tagDetail.value = null;
  showCreateStashModal.value = false;
  newStashMessage.value = "";
  compareBaseBranch.value = "";
  compareTargetBranch.value = "";
  compareResult.value = null;
  compareDiff.value = null;
}

watch(
  () => props.currentRepoPath,
  () => {
    repoEpoch++;
    resetRepoScopedState();
    loadData();
  },
  { immediate: true }
);
</script>

<template>
  <div class="flex-1 flex flex-col bg-canvas overflow-hidden relative">
    <!-- Header with Tabs and Global Actions -->
    <div class="min-h-12 border-b border-line px-4 py-1.5 flex flex-wrap items-center justify-between gap-2 bg-surface/40 backdrop-blur-sm shrink-0">
      <div class="flex items-center gap-1 bg-surface p-1 rounded-xl border border-line min-w-0 overflow-x-auto">
        <button
          @click="activeTab = 'local'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'local' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <GitBranch class="w-3.5 h-3.5" />
          <span>{{ t("branch.tab.local", { count: localBranches.length }) }}</span>
        </button>
        <button
          @click="activeTab = 'remote'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'remote' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <GitFork class="w-3.5 h-3.5" />
          <span>{{ t("branch.tab.remote", { count: remoteBranches.length }) }}</span>
        </button>
        <button
          @click="activeTab = 'tags'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'tags' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <Tag class="w-3.5 h-3.5" />
          <span>{{ t("branch.tab.tags", { count: tags.length }) }}</span>
        </button>
        <button
          @click="activeTab = 'stashes'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'stashes' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <Archive class="w-3.5 h-3.5" />
          <span>{{ t("branch.tab.stashes", { count: stashes.length }) }}</span>
        </button>
        <button
          @click="activeTab = 'compare'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'compare' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <GitCompare class="w-3.5 h-3.5" />
          <span>{{ t("branch.tab.compare") }}</span>
        </button>
        <button
          @click="activeTab = 'river'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'river' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <GitFork class="w-3.5 h-3.5" />
          <span>{{ t("branch.tab.river") }}</span>
        </button>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <div v-if="activeTab === 'local'" class="relative flex items-center shrink-0">
          <ArrowUpDown class="w-3 h-3 text-ink-muted absolute left-2 pointer-events-none" />
          <select
            v-model="sortMode"
            class="pl-6 pr-4 py-1 rounded-md bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
            :title="t('branch.sort.title')"
          >
            <option value="recent">{{ t("branch.sort.recent") }}</option>
            <option value="name">{{ t("branch.sort.name") }}</option>
            <option value="ahead">{{ t("branch.sort.ahead") }}</option>
          </select>
        </div>

        <div class="relative">
          <input
            v-model="searchQuery"
            type="text"
            :aria-label="t('branch.filter.aria')"
            :placeholder="t('branch.filter.placeholder')"
            class="w-32 xl:w-44 pl-7 pr-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans"
          />
          <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-2 pointer-events-none" />
        </div>

        <button :aria-label="t('branch.refresh')"
          @click="loadData"
          class="p-1.5 rounded-md hover:bg-hover text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
          :title="t('branch.refresh')"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
        </button>

        <button
          v-if="activeTab === 'local' || activeTab === 'remote'"
          @click="showCreateBranchModal = true"
          class="flex items-center gap-1 px-3 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast font-medium text-xs transition cursor-pointer shadow-sm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>{{ t("branch.new") }}</span>
        </button>

        <button
          v-else-if="activeTab === 'tags'"
          @click="showCreateTagModal = true"
          class="flex items-center gap-1 px-3 py-1 rounded-md bg-purple-strong hover:opacity-90 text-white font-medium text-xs transition cursor-pointer shadow-sm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>{{ t("branch.new.tag") }}</span>
        </button>

        <button
          v-else-if="activeTab === 'stashes'"
          @click="showCreateStashModal = true"
          class="flex items-center gap-1 px-3 py-1 rounded-md bg-warning-strong hover:opacity-90 text-warning-contrast font-medium text-xs transition cursor-pointer shadow-sm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>{{ t("branch.stash") }}</span>
        </button>
      </div>
    </div>

    <!-- Branch / Tag / Stash Content Area -->
    <div class="flex-1 overflow-y-auto p-4 max-w-5xl mx-auto w-full space-y-2">
      <!-- Gone Upstream Branches Alert Banner -->
      <div
        v-if="activeTab === 'local' && goneBranches.length > 0"
        class="mb-3 px-3.5 py-2.5 rounded-xl border border-warning/40 bg-warning/10 flex items-center justify-between gap-3 text-xs select-none shadow-xs"
        data-testid="gone-branches-banner"
      >
        <div class="flex items-center gap-2 min-w-0">
          <AlertTriangle class="w-4 h-4 text-warning shrink-0" />
          <div class="min-w-0">
            <span class="font-semibold text-warning">
              {{ t("branch.gone.warning.title", { count: goneBranches.length }) }}
            </span>
            <span class="text-ink-secondary ml-1 hidden sm:inline">
              {{ t("branch.gone.warning.desc") }}
            </span>
          </div>
        </div>
        <button
          @click="handlePruneGoneBranches"
          :disabled="isPruningGone"
          class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-warning-strong hover:opacity-90 text-warning-contrast text-xs font-semibold transition cursor-pointer shrink-0 disabled:opacity-50"
          data-testid="prune-gone-branches-btn"
        >
          <Eraser class="w-3.5 h-3.5" />
          <span>{{ isPruningGone ? t("branch.pruning.gone") : t("branch.prune.gone.btn") }}</span>
        </button>
      </div>

      <LocalBranchList
        v-if="activeTab === 'local'"
        :summary="insightSummary"
        :loading="isLoading"
        :insights-loading="isLoadingInsights"
        :error="insightsError"
        :cleanup-disabled="cleanupDisabled"
        :cleaning-merged="isCleaningMerged"
        :cleanable-count="cleanableCount"
        :filtered-count="filteredLocalBranches.length"
        :rows="visibleLocalRows"
        :more-count="localMoreCount"
        @retry-insights="loadInsights"
        @cleanup-merged="handleCleanupMerged"
        @set-upstream="handleSetUpstream"
        @unset-upstream="handleUnsetUpstream"
        @checkout="handleCheckout"
        @merge="openMergeModal"
        @rebase="openRebaseModal"
        @rename="openRenameBranchModal"
        @open-web="handleOpenBranchInWeb"
        @cleanup="handleCleanupBranch"
        @delete="handleDeleteBranch"
        @show-more="showMore('local')"
      />

      <RemoteBranchList
        v-else-if="activeTab === 'remote'"
        :total="filteredRemoteBranches.length"
        :visible="visibleRemoteBranches"
        :more-count="remoteMoreCount"
        @checkout="handleCheckout"
        @merge="openMergeModal"
        @rebase="openRebaseModal"
        @open-web="handleOpenBranchInWeb"
        @delete-remote="handleDeleteRemoteBranch"
        @show-more="showMore('remote')"
      />
      <TagsList
        v-else-if="activeTab === 'tags'"
        :total="tags.length"
        :filtered-count="filteredTags.length"
        :visible="visibleTags"
        :more-count="tagsMoreCount"
        @checkout="handleCheckout"
        @push="handlePushTag"
        @detail="handleShowTagDetail"
        @delete="handleDeleteTag"
        @delete-remote="handleDeleteRemoteTag"
        @delete-selected="handleDeleteTagsBatch"
        @show-more="showMore('tags')"
      />
      <StashesList
        v-else-if="activeTab === 'stashes'"
        :repo-path="currentRepoPath"
        :total="stashes.length"
        :filtered-count="filteredStashes.length"
        :visible="visibleStashes"
        :more-count="stashesMoreCount"
        @apply="handleStashApply"
        @pop="handleStashPop"
        @drop="handleStashDrop"
        @branch="handleStashBranch"
        @show-more="showMore('stashes')"
        @notify="notify"
        @refresh="emit('refresh')"
      />
      <BranchCompare
        v-else-if="activeTab === 'compare'"
        :branches="branches"
        :ref-options="compareRefOptions"
        :merge-base-mode="compareMergeBase"
        :base="compareBaseBranch"
        :target="compareTargetBranch"
        :result="compareResult"
        :loading="isLoadingCompare"
        :diff="compareDiff"
        :diff-loading="isLoadingCompareDiff"
        :current-repo-path="currentRepoPath"
        @update:base="compareBaseBranch = $event"
        @update:target="compareTargetBranch = $event"
        @update:merge-base-mode="compareMergeBase = $event"
        @open-file="handleOpenCompareFile"
        @show-file-diff="handleShowCompareFileDiff"
        @run="runBranchCompare"
        @show-diff="runCompareDiff"
        @open-web="handleOpenCompareInWeb"
        @rebase="openRebaseModal"
        @merge="openMergeModal"
      />
      <BranchRiverView
        v-if="activeTab === 'river'"
        :current-repo-path="currentRepoPath"
        @switch-branch="handleCheckout"
        @compare-branch="(b) => { compareTargetBranch = b; activeTab = 'compare'; }"
        @notify="notify"
      />
    </div>

    <CreateBranchModal
      v-model:open="showCreateBranchModal"
      v-model:name="newBranchName"
      :current-branch="currentBranchName"
      @confirm="handleCreateBranch"
    />
    <RenameBranchModal
      v-model:open="showRenameBranchModal"
      :target="renameTargetBranch"
      v-model:name="renameNewName"
      v-model:update-remote="renameUpdateRemote"
      @confirm="handleRenameBranch"
    />
    <MergeBranchModal
      v-model:open="showMergeModal"
      :source="mergeSourceBranch"
      :current-branch="currentBranchName"
      :preview="mergePreview"
      :loading-preview="isLoadingMergePreview"
      :overlap="mergeOverlap"
      :loading-overlap="isLoadingMergeOverlap"
      :preview-error="mergePreviewError"
      v-model:mode="mergeMode"
      v-model:strategy="mergeStrategyOption"
      v-model:autostash="mergeAutostash"
      v-model:message="mergeCommitMessage"
      :merging="isMerging"
      @confirm="executeMerge"
    />
    <RebaseBranchModal
      v-model:open="showRebaseModal"
      :current-branch="currentBranchName"
      :upstream="rebaseUpstreamBranch"
      v-model:onto="rebaseOnto"
      v-model:autostash="rebaseAutostash"
      v-model:autosquash="rebaseAutosquash"
      :rebasing="isRebasing"
      @confirm="executeRebase"
    />
    <CreateTagModal
      v-model:open="showCreateTagModal"
      v-model:name="newTagName"
      v-model:message="newTagMessage"
      v-model:sign="newTagSign"
      :last-tag="tags[0]?.name"
      @confirm="handleCreateTag"
    />
    <SetUpstreamModal
      :is-open="showSetUpstreamModal"
      :branch="upstreamTargetBranch"
      :available-remote-branches="availableRemoteBranches"
      @close="handleCloseSetUpstreamModal"
      @submit="handleSubmitSetUpstream"
    />
    <!-- Modal: Tag detail -->
    <UiModal
      :open="showTagDetailModal"
      :label="t('branch.tag.detail.label')"
      width="max-w-md"
      @update:open="showTagDetailModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Tag class="w-4 h-4 text-purple" />
          <span>{{ t("branch.tag.detail.label") }}</span>
        </h3>
      </template>

      <div v-if="isLoadingTagDetail" class="py-6 text-center text-xs text-ink-muted">{{ t("branch.tag.detail.loading") }}</div>
      <div v-else-if="tagDetail" class="space-y-3">
        <div class="flex items-center gap-2 flex-wrap">
          <span class="font-mono text-sm font-bold text-purple">{{ tagDetail.name }}</span>
          <UiBadge :tone="tagDetail.isAnnotated ? 'purple' : 'muted'" size="sm">
            {{ tagDetail.isAnnotated ? t("branch.tag.annotated") : t("branch.tag.lightweight") }}
          </UiBadge>
        </div>

        <div class="grid grid-cols-2 gap-2 text-xs">
          <div class="flex flex-col gap-0.5 p-2 rounded-lg bg-canvas/40 border border-line">
            <span class="text-[10px] text-ink-muted">{{ t("branch.tag.target") }}</span>
            <span class="font-mono text-ink">{{ tagDetail.commitHash?.substring(0, 12) }}</span>
          </div>
          <div class="flex flex-col gap-0.5 p-2 rounded-lg bg-canvas/40 border border-line">
            <span class="text-[10px] text-ink-muted">{{ t("branch.tag.date") }}</span>
            <span class="text-ink-secondary">{{ tagDetail.date || "—" }}</span>
          </div>
          <div class="flex flex-col gap-0.5 p-2 rounded-lg bg-canvas/40 border border-line">
            <span class="text-[10px] text-ink-muted">{{ t("branch.tag.tagger") }}</span>
            <span class="text-ink-secondary truncate">{{ tagDetail.taggerName || "—" }}</span>
          </div>
          <div class="flex flex-col gap-0.5 p-2 rounded-lg bg-canvas/40 border border-line">
            <span class="text-[10px] text-ink-muted">{{ t("branch.tag.email") }}</span>
            <span class="text-ink-secondary font-mono text-[10px] truncate">{{ tagDetail.taggerEmail || "—" }}</span>
          </div>
        </div>

        <div class="flex flex-col gap-1">
          <span class="text-[10px] text-ink-muted">{{ t("branch.tag.message") }}</span>
          <div class="px-2.5 py-2 rounded-lg bg-canvas/60 border border-line text-xs text-ink-secondary whitespace-pre-wrap max-h-40 overflow-y-auto">
            {{ tagDetail.message || t("branch.tag.no.message") }}
          </div>
        </div>

        <div class="flex items-center gap-2 flex-wrap">
          <UiButton variant="secondary" :disabled="isVerifyingTag" @click="handleVerifyTag">
            <ShieldCheck class="w-3.5 h-3.5 text-success" />
            <span>{{ isVerifyingTag ? t("branch.tag.verifying") : t("branch.tag.verify") }}</span>
          </UiButton>
          <span class="text-[10px] text-ink-muted">{{ t("branch.tag.verify.hint") }}</span>
        </div>

        <div
          v-if="tagVerifyResult"
          class="rounded-lg border p-2.5 text-[11px] leading-relaxed"
          :class="tagVerifyResult.verified ? 'border-success/30 bg-success/10 text-success' : 'border-warning/30 bg-warning/10 text-warning'"
        >
          <div class="font-semibold">{{ tagVerifyResult.message }}</div>
          <pre v-if="tagVerifyResult.detail" class="mt-1 max-h-28 overflow-y-auto font-mono text-[10px] text-ink-muted whitespace-pre-wrap break-all">{{ tagVerifyResult.detail }}</pre>
        </div>
      </div>
    </UiModal>


    <!-- Compare single-file diff modal -->
    <UiModal
      :open="showCompareFileDiff"
      :title="t('branch.diff.title')"
      width="max-w-4xl"
      @update:open="!$event && closeCompareFileDiff()"
    >
      <template #header>
        <div class="flex items-center gap-3 min-w-0">
          <GitCompare class="w-4 h-4 text-accent shrink-0" />
          <div class="min-w-0">
            <div class="text-sm font-bold text-ink flex items-center gap-2">
              {{ t("branch.diff.title") }}
              <span class="px-1.5 py-0.5 rounded bg-canvas border border-line text-[10px] font-mono font-normal text-ink-muted">
                {{ compareMergeBase ? t("branch.diff.three.dot") : t("branch.diff.two.dot") }}
              </span>
            </div>
            <div class="text-[10px] font-mono text-ink-muted truncate" :title="compareFileDiffPath">
              {{ compareBaseBranch }}{{ compareMergeBase ? "..." : ".." }}{{ compareTargetBranch }} · {{ compareFileDiffPath }}
            </div>
          </div>
        </div>
      </template>

      <div v-if="isLoadingCompareFileDiff" class="flex items-center justify-center gap-2 py-12 text-xs text-ink-muted">
        <Loader2 class="w-4 h-4 animate-spin text-accent" /> {{ t("branch.diff.loading") }}
      </div>
      <div v-else-if="compareFileDiffError" class="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-[11px] text-danger">
        {{ compareFileDiffError }}
      </div>
      <DiffViewer v-else-if="compareFileDiffText" :diff-text="compareFileDiffText" mode="unified" />
      <div v-else class="py-12 text-center text-xs text-ink-muted">{{ t("branch.diff.empty") }}</div>

      <template #footer>
        <UiButton variant="secondary" @click="handleOpenCompareFile(compareFileDiffPath)">
          {{ t("branch.diff.open.workspace") }}
        </UiButton>
        <UiButton variant="primary" @click="closeCompareFileDiff">{{ t("common.close") }}</UiButton>
      </template>
    </UiModal>

    <CreateStashModal
      v-model:open="showCreateStashModal"
      v-model:message="newStashMessage"
      v-model:untracked="stashIncludeUntracked"
      v-model:all="stashIncludeAll"
      v-model:keep="stashKeepIndex"
      @confirm="handleCreateStash"
    />

    <!-- Stash Branch Modal -->
    <UiModal
      :open="showStashBranchModal"
      :label="t('branches.stash.branch.title')"
      width="max-w-md"
      @update:open="showStashBranchModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <GitBranch class="w-4 h-4 text-purple" />
          <span>{{ t("branches.stash.branch.title") }}</span>
        </h3>
      </template>

      <div class="space-y-3">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">{{ t("branches.stash.branch.name") }}</label>
          <input
            v-model="stashBranchName"
            type="text"
            :aria-label="t('branches.stash.branch.name')"
            :placeholder="t('branches.stash.branch.placeholder')"
            @keydown.enter="handleConfirmStashBranch"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-purple font-mono"
          />
        </div>
        <p class="text-[11px] text-ink-muted leading-relaxed">
          {{ t("stash.modal.branch.hint", { index: stashBranchTarget?.index ?? 0 }) }}
        </p>
      </div>

      <template #footer>
        <UiButton variant="ghost" @click="showStashBranchModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton
          variant="purple"
          :disabled="isStashBranching || !stashBranchName.trim()"
          @click="handleConfirmStashBranch"
        >
          {{ isStashBranching ? t("stash.modal.branch.creating") : t("branches.stash.branch.confirm") }}
        </UiButton>
      </template>
    </UiModal>
  </div>
</template>

