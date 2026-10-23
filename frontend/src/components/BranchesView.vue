<script setup lang="ts">
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
} from "../types";
import RemoteBranchList from "./branches/RemoteBranchList.vue";
import TagsList from "./branches/TagsList.vue";
import StashesList from "./branches/StashesList.vue";
import BranchCompare from "./branches/BranchCompare.vue";
import LocalBranchList from "./branches/LocalBranchList.vue";
import DiffViewer from "./workspace/DiffViewer.vue";
import { Loader2 } from "lucide-vue-next";
import CreateBranchModal from "./branches/modals/CreateBranchModal.vue";
import RenameBranchModal from "./branches/modals/RenameBranchModal.vue";
import MergeBranchModal from "./branches/modals/MergeBranchModal.vue";
import RebaseBranchModal from "./branches/modals/RebaseBranchModal.vue";
import CreateTagModal from "./branches/modals/CreateTagModal.vue";
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

const activeTab = ref<"local" | "remote" | "tags" | "stashes" | "compare">("local");
let repoEpoch = 0;
const branches = ref<BranchInfo[]>([]);
const tags = ref<TagInfo[]>([]);
const stashes = ref<StashInfo[]>([]);
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
  notify(`已在工作区打开对比文件: ${path}`);
}

// ---- Single-file diff preview between the compared refs ----
const showCompareFileDiff = ref(false);
const compareFileDiffPath = ref("");
const compareFileDiffText = ref("");
const compareFileDiffError = ref("");
const isLoadingCompareFileDiff = ref(false);

const { containerRef: compareFileDiffDialogRef } = useModalA11y(
  () => showCompareFileDiff.value,
  () => {
    showCompareFileDiff.value = false;
  }
);

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
    compareFileDiffError.value = `读取文件差异失败: ${err?.message || err}`;
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
    if (epoch === repoEpoch) notify(`分支比对失败: ${err?.message || err}`);
  } finally {
    if (epoch === repoEpoch) isLoadingCompare.value = false;
  }
}

async function runCompareDiff() {
  if (!props.currentRepoPath || isLoadingCompareDiff.value) return;
  if (!compareBaseBranch.value || !compareTargetBranch.value) {
    notify("请先输入基准版本与比对版本（支持分支 / 标签 / Commit Hash）");
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
        `差异读取完成：${compareDiff.value.files.length} 个文件，+${compareDiff.value.additions}/-${compareDiff.value.deletions}`
      );
    }
  } catch (err: any) {
    if (epoch === repoEpoch) notify(`读取版本差异失败: ${err?.message || err}`);
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
    notify("已在浏览器中打开对比/PR页面");
  } catch (err: any) {
    notify(`在网页端发起 PR 失败: ${err?.message || err}`);
  }
}

async function handleOpenBranchInWeb(branchName: string) {
  if (!props.currentRepoPath || !branchName) return;
  try {
    await gitService.openBranchInWeb(props.currentRepoPath, branchName);
    notify(`已在浏览器中打开分支 ${branchName}`);
  } catch (err: any) {
    notify(`在网页端打开分支失败: ${err?.message || err}`);
  }
}

// Reset progressive rendering whenever the visible dataset changes
watch([activeTab, searchQuery], () => {
  visibleCounts.value = { local: PAGE_SIZE, remote: PAGE_SIZE, tags: PAGE_SIZE, stashes: PAGE_SIZE };
});

// Toast Feedback System
const { toastText: toastMessage, showToast, notify } = useToast();
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
    notify(res.message);
  } catch (err: any) {
    notify(`标签验签失败: ${err?.message || err}`);
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
    notify(`读取标签详情失败: ${err?.message || err}`);
  } finally {
    isLoadingTagDetail.value = false;
  }
}

// Create Stash Modal
const showCreateStashModal = ref(false);
const newStashMessage = ref("");
const stashIncludeUntracked = ref(true);
const stashKeepIndex = ref(false);

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

const CATEGORY_META: Record<string, { label: string; cls: string; desc: string }> = {
  current: { label: "当前分支", cls: "bg-accent/15 text-accent border-accent/30", desc: "当前检出的分支" },
  protected: { label: "长期分支", cls: "bg-raised text-ink-secondary border-line", desc: "release / hotfix / develop / support 等长期分支，不参与自动清理" },
  merged: { label: "已合并", cls: "bg-success/15 text-success border-success/30", desc: "已完全合并进基准分支，可安全清理" },
  unpushed: { label: "未推送", cls: "bg-warning/15 text-warning border-warning/30", desc: "存在尚未推送到远程的提交" },
  stale: { label: "停滞", cls: "bg-cyan/15 text-cyan border-cyan/30", desc: "长期没有新提交，可考虑清理" },
  active: { label: "活跃", cls: "bg-surface text-ink-muted border-line", desc: "近期有提交活动" },
};

function categoryMeta(category?: string | null) {
  if (!category) return null;
  return CATEGORY_META[category] ?? { label: category, cls: "bg-surface text-ink-muted border-line", desc: "" };
}

/** ISO/date strings -> YYYY-MM-DD for compact display */
function formatInsightDay(ins?: BranchInsight): string {
  if (!ins?.lastDate) return "";
  const day = ins.lastDate.slice(0, 10);
  return /^\d{4}-\d{2}-\d{2}$/.test(day) ? day : ins.lastDate;
}

function formatDaysAgo(days?: number | null): string {
  if (days === undefined || days === null || Number.isNaN(days)) return "";
  if (days <= 0) return "今天";
  if (days === 1) return "1 天前";
  if (days < 30) return `${days} 天前`;
  if (days < 365) return `${Math.floor(days / 30)} 个月前`;
  return `${Math.floor(days / 365)} 年前`;
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
    const [bList, tList, sList] = await Promise.all([
      gitService.getBranches(repoPath),
      gitService.getTags(repoPath).catch(() => []),
      gitService.getStashes(repoPath).catch(() => []),
    ]);
    if (epoch !== repoEpoch) return;
    if (bList) branches.value = bList;
    if (tList) tags.value = tList;
    if (sList) stashes.value = sList;
    // Insights are computed from many git calls: load them in the background so the
    // branch list renders immediately and never gets blocked by a slow analysis.
    void loadInsights();
  } catch (err) {
    console.error("Failed to load branches data:", err);
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
      insightsError.value = "当前仓库暂无分支洞察数据（可能缺少远程或基准分支）";
    }
  } catch (err: any) {
    if (requestId !== insightRequestSeq) return;
    insights.value = null;
    insightsError.value = `分支洞察加载失败: ${err?.message || err}`;
  } finally {
    if (requestId === insightRequestSeq) isLoadingInsights.value = false;
  }
}

// Checkout branch
async function handleCheckout(branchName: string) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.checkoutBranch(props.currentRepoPath, branchName);
    await loadData();
    const checkedOut = branches.value.find((b) => b.isCurrent)?.name || branchName;
    notify(`已检出到分支 ${checkedOut}`);
    emit("switch-branch", checkedOut);
    emit("refresh");
  } catch (err: any) {
    notify(`检出分支失败: ${err?.message || err}`);
  }
}

// Create branch
async function handleCreateBranch() {
  if (!newBranchName.value.trim() || !props.currentRepoPath) return;
  try {
    await gitService.createBranch(props.currentRepoPath, newBranchName.value.trim());
    notify(`分支 ${newBranchName.value.trim()} 创建成功`);
    showCreateBranchModal.value = false;
    newBranchName.value = "";
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`创建分支失败: ${err?.message || err}`);
  }
}

// Delete branch
async function handleDeleteBranch(branchName: string, force = false) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: `确定要${force ? "强制" : ""}删除分支 "${branchName}" 吗？此操作无法撤销。`, tone: "danger" }))) return;
  try {
    await gitService.deleteBranch(props.currentRepoPath, branchName, force);
    notify(`分支 ${branchName} 已删除`);
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`删除分支失败: ${err?.message || err}`);
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
      notify(`没有可清理的已合并分支（基准: ${base}）`);
      return;
    }
    const preview = candidates.slice(0, 15).join("\n");
    const overflow = candidates.length > 15 ? `\n…以及另外 ${candidates.length - 15} 个分支` : "";
    const ok = await confirmAction(
      { message: `将安全删除 ${candidates.length} 个已完全合并到 "${base}" 的本地分支：\n\n${preview}${overflow}\n\n此操作不可撤销，是否继续？`, tone: "danger" }
    );
    if (!ok) return;
    const deleted = await gitService.deleteMergedBranches(repoPath, candidates);
    notify(`已清理 ${deleted?.length ?? candidates.length} 个已合并分支`);
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`清理已合并分支失败: ${err?.message || err}`);
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
  const reason = meta ? `${meta.desc}。` : "";
  const suggestion = insight?.suggestion ? `\n\n建议：${insight.suggestion}` : "";
  const ok = await confirmAction(
    { message: `确定要清理分支 "${branch.name}"${meta ? `（${meta.label}）` : ""} 吗？\n\n${reason}此操作无法撤销。${suggestion}`, tone: "danger" }
  );
  if (!ok) return;
  try {
    await gitService.deleteBranch(repoPath, branch.name, false);
    notify(`分支 ${branch.name} 已清理`);
    await loadData();
    emit("refresh");
  } catch (err: any) {
    const message = err?.message || String(err);
    if (insight?.category === "stale") {
      if (!(await confirmAction({ message: `分支 "${branch.name}" 未被完全合并，Git 已拒绝安全删除。\n\n是否强制删除？未合并的提交将永久丢失。`, tone: "danger", requireText: branch.name, requireTextHint: `输入分支名「${branch.name}」以确认强制删除` }))) return;
      try {
        await gitService.deleteBranch(repoPath, branch.name, true);
        notify(`分支 ${branch.name} 已强制删除`);
        await loadData();
        emit("refresh");
      } catch (forceErr: any) {
        notify(`删除分支失败: ${forceErr?.message || forceErr}`);
      }
      return;
    }
    notify(`删除分支失败: ${message}`);
  }
}

// Pair a local branch with a remote tracking branch (only for branches without upstream)
async function handleSetUpstream(branch: BranchInfo) {
  const repoPath = props.currentRepoPath;
  if (!repoPath) return;
  const suggested = defaultUpstreamFor(branch.name);
  const input = prompt(`为分支 "${branch.name}" 设置上游跟踪分支（例如 ${suggested}）：`, suggested);
  const upstream = input?.trim();
  if (!upstream) return;
  try {
    await gitService.setUpstream(repoPath, branch.name, upstream);
    notify(`已为 ${branch.name} 设置上游 ${upstream}`);
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`设置上游失败: ${err?.message || err}`);
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
          { message: `确定同时重命名远端分支吗？将推送 ${renameNewName.value.trim()} 到跟踪远程并删除远端 ${renameTargetBranch.value}。`, tone: "danger" }
        ))
      )
        return;
      await gitService.renameBranchWithRemote(
        props.currentRepoPath,
        renameTargetBranch.value,
        renameNewName.value.trim(),
        true
      );
    } else {
      await gitService.renameBranch(
        props.currentRepoPath,
        renameTargetBranch.value,
        renameNewName.value.trim()
      );
    }
    notify(`分支已成功重命名为 ${renameNewName.value.trim()}`);
    showRenameBranchModal.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`重命名分支失败: ${err?.message || err}`);
  }
}

// Merge dialog
async function openMergeModal(sourceBranch: string) {
  mergeSourceBranch.value = sourceBranch;
  mergeCommitMessage.value = `Merge branch '${sourceBranch}' into ${currentBranchName.value}`;
  mergePreview.value = null;
  mergeOverlap.value = null;
  showMergeModal.value = true;
  isLoadingMergePreview.value = true;
  isLoadingMergeOverlap.value = true;
  const epoch = repoEpoch;
  try {
    // Overlap analysis answers a question merge-tree cannot: WHO else is
    // editing the same files, which is the real predictor of merge pain.
    const [preview, overlap] = await Promise.all([
      gitService.previewMerge(props.currentRepoPath, sourceBranch).catch(() => null),
      gitService.getMergeOverlapWarning(props.currentRepoPath, sourceBranch, 7).catch(() => null),
    ]);
    if (epoch === repoEpoch) {
      if (preview) mergePreview.value = preview;
      mergeOverlap.value = overlap;
    }
  } catch (err) {
    console.debug("Merge preview error:", err);
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
      notify(`合并发生代码冲突，请切换至工作区解决冲突。`);
    } else {
      notify(`分支 ${mergeSourceBranch.value} 合并成功!`);
    }
    showMergeModal.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`合并失败: ${err?.message || err}`);
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
      notify(`变基在重放提交时发生冲突，请至工作区解决冲突后继续。`);
    } else {
      notify(`已成功变基到 ${rebaseUpstreamBranch.value}!`);
    }
    showRebaseModal.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`变基失败: ${err?.message || err}`);
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
    notify(`标签 ${name} 创建成功${newTagSign.value ? "（已签名）" : ""}`);
    showCreateTagModal.value = false;
    newTagName.value = "";
    newTagMessage.value = "";
    newTagSign.value = false;
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`创建标签失败: ${err?.message || err}`);
  }
}

async function handlePushTag(tagName: string) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.pushTag(props.currentRepoPath, "", tagName);
    notify(`标签 ${tagName} 已推送到远端`);
  } catch (err: any) {
    notify(`推送标签失败: ${err?.message || err}`);
  }
}

async function handleDeleteTagsBatch(tagNames: string[]) {
  if (!props.currentRepoPath || tagNames.length === 0) return;
  if (!(await confirmAction({ message: `确定删除选中的 ${tagNames.length} 个本地标签吗？\n${tagNames.join(", ")}`, tone: "danger" }))) return;
  try {
    const deleted = (await gitService.deleteBatchTags(props.currentRepoPath, tagNames, false, "")) || [];
    const removed = new Set(deleted);
    tags.value = tags.value.filter((tag) => !removed.has(tag.name));
    if (deleted.length === tagNames.length) {
      notify(`已删除 ${deleted.length} 个本地标签`);
    } else {
      notify(`已删除 ${deleted.length}/${tagNames.length} 个本地标签，其余保留`);
    }
  } catch (err: any) {
    notify(`批量删除标签失败: ${err?.message || err}`);
  } finally {
    // Failures may still have deleted some tags; always reload the real state.
    await loadData();
    emit("refresh");
  }
}

async function handleDeleteRemoteTag(tagName: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: `确定从远端删除标签 "${tagName}" 吗？本地标签会保留。`, tone: "danger" }))) return;
  try {
    await gitService.deleteRemoteTag(props.currentRepoPath, "", tagName);
    notify(`远程标签 ${tagName} 已删除`);
  } catch (err: any) {
    notify(`删除远程标签失败: ${err?.message || err}`);
  }
}

async function handleDeleteTag(tagName: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: `确定要删除标签 "${tagName}" 吗？`, tone: "danger" }))) return;
  try {
    await gitService.deleteTag(props.currentRepoPath, tagName);
    tags.value = tags.value.filter((tag) => tag.name !== tagName);
    notify(`标签 ${tagName} 已删除`);
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`删除标签失败: ${err?.message || err}`);
  }
}

// Stashes
async function handleCreateStash() {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashSave(
      props.currentRepoPath,
      newStashMessage.value.trim() || `WIP on ${currentBranchName.value}`,
      stashKeepIndex.value,
      stashIncludeUntracked.value
    );
    notify("当前工作区修改已存入储藏 Shelf");
    showCreateStashModal.value = false;
    newStashMessage.value = "";
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`保存储藏失败: ${err?.message || err}`);
  }
}

async function handleStashApply(index: number) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashApply(props.currentRepoPath, index);
    notify(`已应用储藏 stash@{${index}}`);
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`应用储藏失败: ${err?.message || err}`);
  }
}

async function handleStashPop(index: number) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.stashPop(props.currentRepoPath, index);
    notify(`已弹出储藏 stash@{${index}} 并移出列表`);
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`弹出储藏失败: ${err?.message || err}`);
  }
}

async function handleStashDrop(index: number) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: `确定要丢弃储藏 stash@{${index}} 吗？`, tone: "danger" }))) return;
  try {
    await gitService.stashDrop(props.currentRepoPath, index);
    notify(`已丢弃储藏 stash@{${index}}`);
    await loadData();
    emit("refresh");
  } catch (err: any) {
    notify(`丢弃储藏失败: ${err?.message || err}`);
  }
}

function resetRepoScopedState() {
  activeTab.value = "local";
  isLoadingCompare.value = false;
  isLoadingCompareDiff.value = false;
  isLoadingMergePreview.value = false;
  isLoadingMergeOverlap.value = false;
  mergeOverlap.value = null;
  isMerging.value = false;
  isRebasing.value = false;
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
          <span>本地分支 ({{ localBranches.length }})</span>
        </button>
        <button
          @click="activeTab = 'remote'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'remote' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <GitFork class="w-3.5 h-3.5" />
          <span>远程跟踪 ({{ remoteBranches.length }})</span>
        </button>
        <button
          @click="activeTab = 'tags'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'tags' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <Tag class="w-3.5 h-3.5" />
          <span>标签 ({{ tags.length }})</span>
        </button>
        <button
          @click="activeTab = 'stashes'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'stashes' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <Archive class="w-3.5 h-3.5" />
          <span>储藏 Shelf ({{ stashes.length }})</span>
        </button>
        <button
          @click="activeTab = 'compare'"
          class="flex items-center gap-1.5 px-2.5 xl:px-3 py-1 rounded-lg text-xs transition cursor-pointer font-medium shrink-0 whitespace-nowrap"
          :class="activeTab === 'compare' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
        >
          <GitCompare class="w-3.5 h-3.5" />
          <span>版本对比 (Refs)</span>
        </button>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <div v-if="activeTab === 'local'" class="relative flex items-center shrink-0">
          <ArrowUpDown class="w-3 h-3 text-ink-muted absolute left-2 pointer-events-none" />
          <select
            v-model="sortMode"
            class="pl-6 pr-4 py-1 rounded-md bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
            title="本地分支排序方式"
          >
            <option value="recent">最近活动</option>
            <option value="name">名称</option>
            <option value="ahead">Ahead 数</option>
          </select>
        </div>

        <div class="relative">
          <input
            v-model="searchQuery"
            type="text"
            aria-label="过滤分支或标签"
            placeholder="过滤分支或标签..."
            class="w-32 xl:w-44 pl-7 pr-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans"
          />
          <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2 top-2 pointer-events-none" />
        </div>

        <button aria-label="刷新分支"
          @click="loadData"
          class="p-1.5 rounded-md hover:bg-hover text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
          title="刷新分支"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
        </button>

        <button
          v-if="activeTab === 'local' || activeTab === 'remote'"
          @click="showCreateBranchModal = true"
          class="flex items-center gap-1 px-3 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-white font-medium text-xs transition cursor-pointer shadow-sm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>新建分支</span>
        </button>

        <button
          v-else-if="activeTab === 'tags'"
          @click="showCreateTagModal = true"
          class="flex items-center gap-1 px-3 py-1 rounded-md bg-purple-strong hover:opacity-90 text-white font-medium text-xs transition cursor-pointer shadow-sm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>新建标签</span>
        </button>

        <button
          v-else-if="activeTab === 'stashes'"
          @click="showCreateStashModal = true"
          class="flex items-center gap-1 px-3 py-1 rounded-md bg-warning-strong hover:opacity-90 text-warning-contrast font-medium text-xs transition cursor-pointer shadow-sm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>储藏修改</span>
        </button>
      </div>
    </div>

    <!-- Branch / Tag / Stash Content Area -->
    <div class="flex-1 overflow-y-auto p-4 max-w-5xl mx-auto w-full space-y-2">
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
    </div>

    <Toast :text="toastMessage" :visible="showToast" />

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
      @confirm="handleCreateTag"
    />
    <!-- Modal: Tag detail -->
    <UiModal
      :open="showTagDetailModal"
      label="标签详情"
      width="max-w-md"
      @update:open="showTagDetailModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Tag class="w-4 h-4 text-purple" />
          <span>标签详情</span>
        </h3>
      </template>

      <div v-if="isLoadingTagDetail" class="py-6 text-center text-xs text-ink-muted">正在读取标签信息...</div>
      <div v-else-if="tagDetail" class="space-y-3">
        <div class="flex items-center gap-2 flex-wrap">
          <span class="font-mono text-sm font-bold text-purple">{{ tagDetail.name }}</span>
          <UiBadge :tone="tagDetail.isAnnotated ? 'purple' : 'muted'" size="sm">
            {{ tagDetail.isAnnotated ? "附注标签 (Annotated)" : "轻量标签 (Lightweight)" }}
          </UiBadge>
        </div>

        <div class="grid grid-cols-2 gap-2 text-xs">
          <div class="flex flex-col gap-0.5 p-2 rounded-lg bg-canvas/40 border border-line">
            <span class="text-[10px] text-ink-muted">目标提交</span>
            <span class="font-mono text-ink">{{ tagDetail.commitHash?.substring(0, 12) }}</span>
          </div>
          <div class="flex flex-col gap-0.5 p-2 rounded-lg bg-canvas/40 border border-line">
            <span class="text-[10px] text-ink-muted">打标时间</span>
            <span class="text-ink-secondary">{{ tagDetail.date || "—" }}</span>
          </div>
          <div class="flex flex-col gap-0.5 p-2 rounded-lg bg-canvas/40 border border-line">
            <span class="text-[10px] text-ink-muted">打标者</span>
            <span class="text-ink-secondary truncate">{{ tagDetail.taggerName || "—" }}</span>
          </div>
          <div class="flex flex-col gap-0.5 p-2 rounded-lg bg-canvas/40 border border-line">
            <span class="text-[10px] text-ink-muted">邮箱</span>
            <span class="text-ink-secondary font-mono text-[10px] truncate">{{ tagDetail.taggerEmail || "—" }}</span>
          </div>
        </div>

        <div class="flex flex-col gap-1">
          <span class="text-[10px] text-ink-muted">附注说明</span>
          <div class="px-2.5 py-2 rounded-lg bg-canvas/60 border border-line text-xs text-ink-secondary whitespace-pre-wrap max-h-40 overflow-y-auto">
            {{ tagDetail.message || "（无附注说明）" }}
          </div>
        </div>

        <div class="flex items-center gap-2 flex-wrap">
          <UiButton variant="secondary" :disabled="isVerifyingTag" @click="handleVerifyTag">
            <ShieldCheck class="w-3.5 h-3.5 text-success" />
            <span>{{ isVerifyingTag ? "校验中..." : "校验签名" }}</span>
          </UiButton>
          <span class="text-[10px] text-ink-muted">发布标签是否被篡改，靠这一步确认</span>
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
    <div
      v-if="showCompareFileDiff"
      ref="compareFileDiffDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="版本对比文件差异"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4"
      @click.self="closeCompareFileDiff"
    >
      <div class="w-full max-w-4xl max-h-[85vh] rounded-2xl bg-surface border border-line shadow-2xl flex flex-col overflow-hidden">
        <div class="flex items-center gap-3 px-4 py-3 border-b border-line shrink-0 bg-raised/30">
          <GitCompare class="w-4 h-4 text-accent shrink-0" />
          <div class="min-w-0">
            <div class="text-sm font-bold text-ink flex items-center gap-2">
              文件差异
              <span class="px-1.5 py-0.5 rounded bg-canvas border border-line text-[10px] font-mono font-normal text-ink-muted">
                {{ compareMergeBase ? "三点 A...B" : "两点 A..B" }}
              </span>
            </div>
            <div class="text-[10px] font-mono text-ink-muted truncate" :title="compareFileDiffPath">
              {{ compareBaseBranch }}{{ compareMergeBase ? "..." : ".." }}{{ compareTargetBranch }} · {{ compareFileDiffPath }}
            </div>
          </div>
          <button aria-label="关闭 (Esc)"
            @click="closeCompareFileDiff"
            class="ml-auto shrink-0 p-1.5 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
            title="关闭 (Esc)"
          >
            <X class="w-4 h-4" />
          </button>
        </div>
        <div class="flex-1 min-h-0 overflow-y-auto p-3">
          <div v-if="isLoadingCompareFileDiff" class="flex items-center justify-center gap-2 py-12 text-xs text-ink-muted">
            <Loader2 class="w-4 h-4 animate-spin text-accent" /> 正在读取文件差异…
          </div>
          <div v-else-if="compareFileDiffError" class="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-[11px] text-danger">
            {{ compareFileDiffError }}
          </div>
          <DiffViewer v-else-if="compareFileDiffText" :diff-text="compareFileDiffText" mode="unified" />
          <div v-else class="py-12 text-center text-xs text-ink-muted">两个版本之间没有该文件的差异</div>
        </div>
        <div class="shrink-0 border-t border-line px-4 py-2.5 flex items-center justify-end gap-2">
          <button
            @click="handleOpenCompareFile(compareFileDiffPath)"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
          >
            在工作区打开
          </button>
          <button
            @click="closeCompareFileDiff"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer"
          >
            关闭
          </button>
        </div>
      </div>
    </div>

    <CreateStashModal
      v-model:open="showCreateStashModal"
      v-model:message="newStashMessage"
      v-model:untracked="stashIncludeUntracked"
      v-model:keep="stashKeepIndex"
      @confirm="handleCreateStash"
    />
  </div>
</template>

