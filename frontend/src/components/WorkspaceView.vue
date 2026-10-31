<script setup lang="ts">
import { confirmAction } from "../composables/useConfirm";
import { t } from "../i18n";
import {
  ref,
  computed,
  watch,
  nextTick,
  onMounted,
  onUnmounted,
} from "vue";
import {
  FileText,
  Check,
  Plus,
  Minus,
  Trash2,
  Columns,
  Columns2,
  PanelLeft,
  PanelLeftClose,
  PanelRight,
  Bookmark,
  List,
  GitCommit,
  CheckCheck,
  RefreshCw,
  Copy,
  FileSpreadsheet,
  Box,
  Image,
  Code2,
  ShieldAlert,
  ZoomIn,
  ZoomOut,
  WrapText,
  History,
  Lock,
  ExternalLink,
  Pilcrow,
  ScanEye,
  Sparkles,
  Space,
  Archive,
  AlertTriangle,
  GitMerge,
  MessageSquare,
  ChevronDown,
  ChevronUp,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import { isAIConfigured } from "../services/aiService";
import { openAiSetup } from "../composables/useAiSetup";
import AiRepoOverrideModal from "./workspace/AiRepoOverrideModal.vue";
import Toast from "./ui/Toast.vue";
import FeatureGate from "./ui/FeatureGate.vue";
import { useToast, reportError } from "../composables/useToast";
import UiModal from "./ui/UiModal.vue";
import SourceExplorer from "./workspace/SourceExplorer.vue";
import { isActionCustomized } from "../utils/shortcuts";
import SourcePanorama from "./workspace/SourcePanorama.vue";
import DiffViewer from "./workspace/DiffViewer.vue";
import AssetDiffModal from "./assets/AssetDiffModal.vue";
import SemanticConflictModal from "./workspace/SemanticConflictModal.vue";
import ThreeWayMergeModal from "./ThreeWayMergeModal.vue";
import FlightRecorderHud from "./workspace/FlightRecorderHud.vue";
import ChangesList from "./workspace/ChangesList.vue";
import ConflictBanner from "./workspace/ConflictBanner.vue";
import CommitComposer from "./workspace/CommitComposer.vue";
import AiConflictAdvisor from "./workspace/AiConflictAdvisor.vue";
import AiExplainDiffModal from "./workspace/AiExplainDiffModal.vue";
import AiConflictBatchModal, { type BatchTarget } from "./workspace/AiConflictBatchModal.vue";
import AiChatModal from "./workspace/AiChatModal.vue";
import AiReviewModal from "./workspace/AiReviewModal.vue";
import { parseConflictFileBlocks } from "../services/localAiService";
import { isVimModeEnabled, currentVimMode, handleVimKeydown } from "../utils/vimNavigation";
import type {
  RepoStatus,
  FileStatus,
  ProjectSourceFileItem,
  SourceFileDetail,
  BlameLine,
  TimeMachineSnapshot,
  FileRevisionItem,
  WorkspaceStats,
} from "../types";

const props = defineProps<{
  currentRepoPath: string;
  repoStatus: RepoStatus | null;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

// Resizable Pane State (persisted per repository so each repo reopens with its own layout)
const PANE_SIZES_STORAGE_KEY = "wgit.workspace.paneSizesByRepo.v1";
const SOURCE_PANE_STORAGE_KEY = "wgit.workspace.sourcePane.v1";
const LEGACY_PANE_SIZES_KEY = "wgit.workspace.paneSizes.v1";
const DEFAULT_LEFT_PANE_WIDTH = 320;
const DEFAULT_CHANGES_PANE_WIDTH = 320;

function clampLeftPane(value: number): number {
  return Math.max(240, Math.min(650, value));
}

// Drag snapping stops for common pane widths (within 8px).
const LEFT_PANE_SNAPS = [240, 320, 480, 650];
const CHANGES_PANE_SNAPS = [240, 320, 420, 550];

function snapPaneWidth(value: number, snaps: number[]): number {
  for (const snap of snaps) {
    if (Math.abs(value - snap) <= 8) return snap;
  }
  return value;
}

function clampChangesPane(value: number): number {
  return Math.max(240, Math.min(550, value));
}

function loadPaneSizes(repoPath: string): { left: number; changes: number } {
  try {
    const raw = localStorage.getItem(PANE_SIZES_STORAGE_KEY);
    const map = raw ? JSON.parse(raw) : {};
    const entry = repoPath ? map?.[repoPath] : undefined;
    let left = Number(entry?.left);
    let changes = Number(entry?.changes);
    if (!Number.isFinite(left) || !Number.isFinite(changes)) {
      // One-time migration from the old global pane layout.
      const legacyRaw = localStorage.getItem(LEGACY_PANE_SIZES_KEY);
      const legacy = legacyRaw ? JSON.parse(legacyRaw) : {};
      left = Number(legacy?.left);
      changes = Number(legacy?.changes);
    }
    return {
      left: Number.isFinite(left) ? clampLeftPane(left) : DEFAULT_LEFT_PANE_WIDTH,
      changes: Number.isFinite(changes) ? clampChangesPane(changes) : DEFAULT_CHANGES_PANE_WIDTH,
    };
  } catch {
    // Benign: unreadable/corrupt pane prefs just fall back to the default widths.
    return { left: DEFAULT_LEFT_PANE_WIDTH, changes: DEFAULT_CHANGES_PANE_WIDTH };
  }
}

const initialPaneSizes = loadPaneSizes(props.currentRepoPath);
const leftPaneWidth = ref(initialPaneSizes.left);
const isResizing = ref(false);

// ---- Width-driven pane layout -----------------------------------------
// Both side panes are fixed-width, so on a small window they starved the diff
// canvas down to a few pixels and its toolbar wrapped one glyph per line. The
// thresholds are measured on the workspace element (not the window) because the
// dock and rail already ate part of the width.
const workspaceRootRef = ref<HTMLElement | null>(null);
const workspaceWidth = ref(0);
const STACK_PANES_BELOW = 900;

// The source tree starts hidden so the changes/diff pair gets the width; the
// toolbar toggle remembers an explicit show/hide choice across launches.
function loadSourcePaneVisible(): boolean {
  try {
    return localStorage.getItem(SOURCE_PANE_STORAGE_KEY) === "shown";
  } catch {
    return false;
  }
}

const sourcePaneVisible = ref(loadSourcePaneVisible());
const showSourcePane = computed(() => sourcePaneVisible.value);

/** Narrower still: the changes list and the diff take turns instead of sharing. */
const isStackedPanes = computed(
  () => workspaceWidth.value > 0 && workspaceWidth.value < STACK_PANES_BELOW
);
const stackedPane = ref<"changes" | "diff">("changes");

// Never render a pane wider than the space it sits in; the stored preference is
// left untouched so widening the window restores it.
function fitPaneWidth(preferred: number, min: number, share: number): number {
  if (workspaceWidth.value <= 0) return preferred;
  const ceiling = Math.max(min, Math.round(workspaceWidth.value * share));
  return Math.min(preferred, ceiling);
}
const effectiveLeftPaneWidth = computed(() => fitPaneWidth(leftPaneWidth.value, 200, 0.34));
const effectiveChangesPaneWidth = computed(() => fitPaneWidth(changesPaneWidth.value, 240, 0.46));

function toggleSourcePane() {
  sourcePaneVisible.value = !sourcePaneVisible.value;
  try {
    localStorage.setItem(SOURCE_PANE_STORAGE_KEY, sourcePaneVisible.value ? "shown" : "hidden");
  } catch {
    // Storage may be unavailable; the in-memory choice still applies.
  }
}

// ---- Narrow-window left-pane collapse (roadmap §2.1 item 9) -------------
// Between the stacked regime and the dual-pane breakpoint the source tree can
// collapse to a 36px rail so the changes/diff pair reclaims the width. The
// breakpoint is measured on the workspace element (not the window), like
// STACK_PANES_BELOW above: the dock and rail already eat part of the window.
const LEFT_PANE_COLLAPSED_STORAGE_KEY = "wgit.workspace.leftPane.collapsed.v1";
const LEFT_PANE_COLLAPSE_BELOW = 1280;

function loadLeftPaneCollapsed(): boolean {
  try {
    return localStorage.getItem(LEFT_PANE_COLLAPSED_STORAGE_KEY) === "true";
  } catch {
    // Benign: storage unavailable/corrupt — the pane just starts expanded.
    return false;
  }
}

const leftPaneCollapsed = ref(loadLeftPaneCollapsed());

/** Collapse only applies while the workspace is narrow; ≥1280px keeps both panes. */
function isLeftPaneCollapseWidth(width: number): boolean {
  return width > 0 && width < LEFT_PANE_COLLAPSE_BELOW;
}

const canCollapseLeftPane = computed(() => isLeftPaneCollapseWidth(workspaceWidth.value));

/**
 * The stored preference stays untouched while the workspace is wide, so
 * narrowing it again restores the collapsed rail (same contract as
 * fitPaneWidth above). It is also independent of the pane *widths*:
 * loadPaneSizes / persistPaneSizes own paneSizesByRepo.v1 and this boolean
 * lives in its own key, so neither write can clobber the other.
 */
const isLeftPaneCollapsed = computed(
  () => leftPaneCollapsed.value && canCollapseLeftPane.value
);

function setLeftPaneCollapsed(collapsed: boolean) {
  leftPaneCollapsed.value = collapsed;
  try {
    localStorage.setItem(LEFT_PANE_COLLAPSED_STORAGE_KEY, collapsed ? "true" : "false");
  } catch {
    // Benign: quota/privacy-mode storage failure — the collapse still applies
    // for this session, only the cross-launch memory is lost.
  }
}

let workspaceResizeObserver: ResizeObserver | null = null;

// Floating width hint shown while dragging a pane divider.
const resizeHint = ref<{ x: number; y: number; width: number; label: string } | null>(null);

function showResizeHint(event: MouseEvent, width: number, label: string) {
  resizeHint.value = {
    x: Math.max(8, Math.min(event.clientX + 12, window.innerWidth - 130)),
    y: Math.max(8, event.clientY - 14),
    width,
    label,
  };
}

function startResizing(e: MouseEvent) {
  isResizing.value = true;
  const startX = e.clientX;
  const startWidth = leftPaneWidth.value;

  function onMouseMove(ev: MouseEvent) {
    const delta = ev.clientX - startX;
    const raw = Math.max(240, Math.min(650, startWidth + delta));
    leftPaneWidth.value = snapPaneWidth(raw, LEFT_PANE_SNAPS);
  showResizeHint(ev, leftPaneWidth.value, t("ws.resize.source"));
  }

  function onMouseUp() {
    isResizing.value = false;
    resizeHint.value = null;
    window.removeEventListener("mousemove", onMouseMove);
    window.removeEventListener("mouseup", onMouseUp);
  }

  window.addEventListener("mousemove", onMouseMove);
  window.addEventListener("mouseup", onMouseUp);
}

// Left pane: Source code file explorer
const selectedSourceFile = ref<string | null>(null);
const sourceFileDetail = ref<SourceFileDetail | null>(null);
const sourceRefreshNonce = ref(0);

const activeRightTab = ref<"diff" | "source">("diff");

// Image viewer state
const imageZoom = ref<number>(100);
const imageBgMode = ref<"checkered" | "dark" | "white">("checkered");
const imagePixelated = ref<boolean>(false);
const svgViewMode = ref<"image" | "code">("image");
const imageNaturalWidth = ref<number>(0);
const imageNaturalHeight = ref<number>(0);

const imageWidth = computed(() => {
  return sourceFileDetail.value?.imageWidth || imageNaturalWidth.value;
});

const imageHeight = computed(() => {
  return sourceFileDetail.value?.imageHeight || imageNaturalHeight.value;
});

function handleImageLoaded(size: { width: number; height: number }) {
  imageNaturalWidth.value = size.width;
  imageNaturalHeight.value = size.height;
}

function onImageLoaded(e: Event) {
  const img = e.target as HTMLImageElement;
  if (img) {
    imageNaturalWidth.value = img.naturalWidth;
    imageNaturalHeight.value = img.naturalHeight;
  }
}

function zoomIn() {
  if (imageZoom.value < 500) imageZoom.value += 25;
}

function zoomOut() {
  if (imageZoom.value > 25) imageZoom.value -= 25;
}

function resetZoom() {
  imageZoom.value = 100;
}

// Code viewer options
const isCodeWrap = ref<boolean>(false);
const showBlame = ref<boolean>(false);
const blameLines = ref<BlameLine[]>([]);

const isLoadingBlame = ref<boolean>(false);

// Time Machine Navigation System (Round 23)
const timeMachineActive = ref<boolean>(false);
const timeMachineStack = ref<TimeMachineSnapshot[]>([]);
const timeMachineIndex = ref<number>(-1);

const timeMachineCurrent = computed<TimeMachineSnapshot | null>(() => {
  if (timeMachineIndex.value >= 0 && timeMachineIndex.value < timeMachineStack.value.length) {
    return timeMachineStack.value[timeMachineIndex.value] ?? null;
  }
  return null;
});

const canTimeMachineBack = computed(() => timeMachineIndex.value > 0);
const canTimeMachineForward = computed(
  () => timeMachineIndex.value >= 0 && timeMachineIndex.value < timeMachineStack.value.length - 1
);

async function toggleBlame() {
  showBlame.value = !showBlame.value;
  if (showBlame.value && selectedSourceFile.value) {
    await fetchBlame(selectedFileRevisionHash.value || undefined);
  }
}

async function fetchBlame(refOverride?: string) {
  if (!props.currentRepoPath || !selectedSourceFile.value) return;
  const repoPath = props.currentRepoPath;
  const filePath = selectedSourceFile.value;
  const targetRef = refOverride !== undefined ? refOverride : (selectedFileRevisionHash.value || undefined);
  isLoadingBlame.value = true;
  try {
    const list = await gitService.getFileBlameWithOptions(repoPath, {
      filePath,
      ref: targetRef || undefined,
    });
    if (props.currentRepoPath !== repoPath || selectedSourceFile.value !== filePath) return;
    blameLines.value = list || [];
  } catch (err) {
    reportError("Failed to load blame:", err);
    if (props.currentRepoPath === repoPath && selectedSourceFile.value === filePath) blameLines.value = [];
  } finally {
    if (props.currentRepoPath === repoPath && selectedSourceFile.value === filePath) isLoadingBlame.value = false;
  }
}

function copyCommitHash(hash: string) {
  if (!hash) return;
  navigator.clipboard.writeText(hash);
  notify(t("ws.toast.hash.copied", { hash: hash.substring(0, 8) }));
}

// File revision history state
const showFileHistory = ref(false);
const fileHistoryList = ref<FileRevisionItem[]>([]);
const isLoadingFileHistory = ref(false);
const selectedFileRevisionHash = ref<string>("");

async function toggleFileHistory() {
  showFileHistory.value = !showFileHistory.value;
  if (showFileHistory.value && selectedSourceFile.value) {
    await fetchFileHistory();
  }
}

async function fetchFileHistory() {
  if (!props.currentRepoPath || !selectedSourceFile.value) return;
  const repoPath = props.currentRepoPath;
  const filePath = selectedSourceFile.value;
  isLoadingFileHistory.value = true;
  try {
    const timeline = await gitService.getFileRevisionTimeline(repoPath, filePath);
    if (props.currentRepoPath !== repoPath || selectedSourceFile.value !== filePath) return;
    fileHistoryList.value = timeline?.revisions || [];
  } catch (err) {
    reportError("Failed to load file history", err);
    if (props.currentRepoPath === repoPath && selectedSourceFile.value === filePath) fileHistoryList.value = [];
  } finally {
    if (props.currentRepoPath === repoPath && selectedSourceFile.value === filePath) isLoadingFileHistory.value = false;
  }
}

async function viewFileRevision(commitHash: string) {
  if (!props.currentRepoPath || !selectedSourceFile.value) return;
  const repoPath = props.currentRepoPath;
  const filePath = selectedSourceFile.value;
  selectedFileRevisionHash.value = commitHash;
  try {
    const detail = await gitService.getSourceFileDetail(repoPath, filePath, commitHash);
    if (
      props.currentRepoPath !== repoPath ||
      selectedSourceFile.value !== filePath ||
      selectedFileRevisionHash.value !== commitHash
    )
      return;
    if (detail) {
      sourceFileDetail.value = detail;
      if (showBlame.value) {
        await fetchBlame(commitHash);
      }
      const item = fileHistoryList.value.find((c) => c.commitHash === commitHash);
      const shortHash = commitHash.substring(0, 7);
      const snapshot: TimeMachineSnapshot = {
        ref: commitHash,
        shortHash,
        subject: item?.subject || shortHash,
        filePath,
      };
      timeMachineActive.value = true;
      if (timeMachineIndex.value >= 0 && timeMachineIndex.value < timeMachineStack.value.length - 1) {
        timeMachineStack.value = timeMachineStack.value.slice(0, timeMachineIndex.value + 1);
      }
      timeMachineStack.value.push(snapshot);
      timeMachineIndex.value = timeMachineStack.value.length - 1;
      notify(t("ws.toast.version.switched", { hash: commitHash.substring(0, 8) }));
    }
  } catch (err: any) {
    notify(t("ws.toast.version.failed", { error: err?.message || err }));
  }
}

async function handleBlamePrior(line: BlameLine) {
  if (!props.currentRepoPath || !selectedSourceFile.value) return;
  if (line.isBoundary) {
    notify(t("timemachine.toast.boundary"));
    return;
  }
  const priorCommit = line.previousCommit || `${line.commitHash}^`;
  const priorFile = line.previousFilename || selectedSourceFile.value;

  try {
    const detail = await gitService.getSourceFileDetail(props.currentRepoPath, priorFile, priorCommit);
    if (!detail) return;
    sourceFileDetail.value = detail;
    selectedSourceFile.value = priorFile;
    selectedFileRevisionHash.value = priorCommit;
    showBlame.value = true;
    await fetchBlame(priorCommit);

    const shortHash = priorCommit.length > 7 ? priorCommit.substring(0, 7) : priorCommit;
    const snapshot: TimeMachineSnapshot = {
      ref: priorCommit,
      shortHash,
      subject: line.summary || shortHash,
      filePath: priorFile,
    };
    timeMachineActive.value = true;
    if (timeMachineIndex.value >= 0 && timeMachineIndex.value < timeMachineStack.value.length - 1) {
      timeMachineStack.value = timeMachineStack.value.slice(0, timeMachineIndex.value + 1);
    }
    timeMachineStack.value.push(snapshot);
    timeMachineIndex.value = timeMachineStack.value.length - 1;
    notify(t("timemachine.toast.entered", { hash: shortHash }));
  } catch (err: any) {
    notify(t("ws.toast.version.failed", { error: err?.message || err }));
  }
}

async function handleTimeMachineBack() {
  if (!canTimeMachineBack.value) return;
  timeMachineIndex.value--;
  const snapshot = timeMachineStack.value[timeMachineIndex.value];
  if (snapshot) {
    await loadTimeMachineSnapshot(snapshot);
  }
}

async function handleTimeMachineForward() {
  if (!canTimeMachineForward.value) return;
  timeMachineIndex.value++;
  const snapshot = timeMachineStack.value[timeMachineIndex.value];
  if (snapshot) {
    await loadTimeMachineSnapshot(snapshot);
  }
}

async function loadTimeMachineSnapshot(snapshot: TimeMachineSnapshot) {
  if (!props.currentRepoPath) return;
  selectedSourceFile.value = snapshot.filePath;
  selectedFileRevisionHash.value = snapshot.ref;
  try {
    const detail = await gitService.getSourceFileDetail(props.currentRepoPath, snapshot.filePath, snapshot.ref);
    if (detail) {
      sourceFileDetail.value = detail;
      if (showBlame.value) {
        await fetchBlame(snapshot.ref);
      }
    }
  } catch (err: any) {
    notify(t("ws.toast.version.failed", { error: err?.message || err }));
  }
}

async function handleExitTimeMachine() {
  timeMachineActive.value = false;
  timeMachineStack.value = [];
  timeMachineIndex.value = -1;
  selectedFileRevisionHash.value = "";
  if (selectedSourceFile.value) {
    await selectSourceFile(selectedSourceFile.value);
  }
  notify(t("timemachine.toast.restored"));
}

function handleInspectInHistory(hash: string) {
  if (!hash) return;
  window.dispatchEvent(new CustomEvent("wgit:history-commit", { detail: { hash } }));
}

async function restoreCurrentFileRevision() {
  await handleExitTimeMachine();
}

async function revertSelectedFileToRevision(commitHash: string) {
  if (!props.currentRepoPath || !selectedSourceFile.value || !commitHash) return;
  const file = selectedSourceFile.value;
  if (!(await confirmAction({ message: t("ws.confirm.restore.version", { file, hash: commitHash.substring(0, 8) }), tone: "danger" }))) return;
  try {
    const res = await gitService.revertFileToRevision(props.currentRepoPath, file, commitHash);
    notify(res?.message || t("ws.toast.version.restored", { file: file.split("/").pop() ?? file, hash: commitHash.substring(0, 8) }));
    selectedFileRevisionHash.value = "";
    emit("refresh");
    sourceRefreshNonce.value++;
    await selectSourceFile(file);
    if (showFileHistory.value) await fetchFileHistory();
  } catch (err: any) {
    notify(t("ws.toast.version.restore.failed", { error: err?.message || err }));
  }
}


async function handleToggleLfsLock(file: FileStatus) {
  if (!props.currentRepoPath || !file.path) return;
  try {
    await gitService.lockFile(props.currentRepoPath, file.path);
    notify(t("ws.toast.lfs.locked", { file: file.path.split("/").pop() ?? file.path }));
    emit("refresh");
  } catch (err: any) {
    notify(t("ws.toast.lfs.lock.failed", { error: err?.message || err }));
  }
}

// Right pane: Changes & Diff
const selectedChangeFile = ref<FileStatus | null>(null);
const activeDiffMode = ref<"unified" | "split" | "word">("unified");
const ignoreWhitespace = ref(false);
const currentDiffText = ref("");
const diffScrollRef = ref<HTMLElement | null>(null);
const loadedDiffKey = ref("");

// Header summary only; the full diff parsing lives in workspace/DiffViewer.vue.
const diffTotalLineCount = computed(() =>
  currentDiffText.value ? currentDiffText.value.split("\n").length : 0
);
const diffStats = computed(() => {
  let additions = 0;
  let deletions = 0;
  for (const text of currentDiffText.value ? currentDiffText.value.split("\n") : []) {
    if (text.startsWith("+") && !text.startsWith("+++")) additions++;
    else if (text.startsWith("-") && !text.startsWith("---")) deletions++;
  }
  return { additions, deletions };
});
const isLoadingDiff = ref(false);
const isHunkActionBusy = ref(false);
const copiedCodeToast = ref(false);

// Commit Composer state
const commitComposerRef = ref<InstanceType<typeof CommitComposer> | null>(null);
const commitSubject = ref("");
const commitBody = ref("");
const workspaceStats = ref<WorkspaceStats | null>(null);
const isLoadingWorkspaceStats = ref(false);
let workspaceStatsSeq = 0;
const revisionDiff = ref("");
const revisionDiffHash = ref("");
const isLoadingRevisionDiff = ref(false);
const showRevisionDiff = ref(false);
const isAmend = ref(false);
const autoPush = ref(false);
const isNoVerify = ref(false);
const isCommitting = ref(false);

// ---- Pinned commit bar (roadmap §2.1 item 1) -----------------------------
// The composer used to be docked inside the changes pane, so stacked panes
// (narrow windows), the source tab and long lists squeezed the whole
// stage → describe → commit path out of the viewport. Its mount now lives in
// a bar pinned to the right column edge, collapsible to one summary line.
const COMPOSER_COLLAPSED_KEY = "wgit.composer.collapsed.v1";

function loadComposerCollapsed(): boolean {
  try {
    return localStorage.getItem(COMPOSER_COLLAPSED_KEY) === "true";
  } catch {
    // Benign: storage unavailable/corrupt — the bar just starts expanded.
    return false;
  }
}

const composerCollapsed = ref(loadComposerCollapsed());

function toggleComposerCollapsed() {
  composerCollapsed.value = !composerCollapsed.value;
  try {
    localStorage.setItem(COMPOSER_COLLAPSED_KEY, composerCollapsed.value ? "true" : "false");
  } catch {
    // Benign: quota/privacy-mode storage failure — collapse still applies
    // for this session, only the cross-launch memory is lost.
  }
}

/** Expand (and persist) before focusing, e.g. from the keyboard shortcuts. */
async function expandComposerForFocus() {
  if (composerCollapsed.value) {
    toggleComposerCollapsed();
    // The composer stays mounted via v-show; wait one tick so the inputs
    // become focusable before the shortcut hands over the caret.
    await nextTick();
  }
  commitComposerRef.value?.focus();
}


// Staged, Unstaged, and Conflicted file lists
const stagedFiles = computed(() => props.repoStatus?.stagedFiles || []);
const stagedNonLfsLargeFiles = computed(() => {
  return stagedFiles.value.filter((f) => {
    if (f.isLfs) return false;
    const sz = (f.size || "").trim().toUpperCase();
    if (sz.endsWith("GB")) return true;
    if (sz.endsWith("MB")) {
      const mb = parseFloat(sz);
      return !isNaN(mb) && mb >= 20;
    }
    return false;
  });
});
const isConvertingLfs = ref(false);

const unstagedFiles = computed(() => [
  ...(props.repoStatus?.unstagedFiles || []),
  ...(props.repoStatus?.untrackedFiles || []),
]);
const conflictedFiles = computed(() => props.repoStatus?.conflictedFiles || []);
const metaConflictedFiles = computed(() => {
  return conflictedFiles.value.filter((f) => {
    const ext = f.toLowerCase().slice(f.lastIndexOf("."));
    return ext === ".meta" || ext === ".json" || ext === ".yaml" || ext === ".yml";
  });
});
const hasMetaConflicts = computed(() => metaConflictedFiles.value.length > 0);
const semanticConflictedFiles = computed(() => {
  return conflictedFiles.value.filter((f) => {
    const ext = f.toLowerCase().slice(f.lastIndexOf("."));
    return [".unity", ".prefab", ".mat", ".asset", ".meta", ".t3d", ".ini"].includes(ext);
  });
});
const hasSemanticAssetConflicts = computed(() => semanticConflictedFiles.value.length > 0);
const firstSemanticConflictFile = computed(() => semanticConflictedFiles.value[0] || "");

const showSemanticConflictModal = ref(false);
const semanticConflictFilePath = ref("");

function openSemanticConflictModal(filePath?: string) {
  semanticConflictFilePath.value = filePath || firstSemanticConflictFile.value;
  showSemanticConflictModal.value = true;
}

const showThreeWayMergeModal = ref(false);
const threeWayMergeFilePath = ref("");

function openThreeWayMerge(filePath?: string) {
  threeWayMergeFilePath.value = filePath || selectedChangeFile.value?.path || conflictedFiles.value[0] || "";
  showThreeWayMergeModal.value = true;
}

function handleThreeWayResolved(filePath: string) {
  emit("refresh");
}
const isMerging = computed(() => props.repoStatus?.isMerging || false);
const isRebasing = computed(() => props.repoStatus?.isRebasing || false);
const isCherryPicking = computed(() => props.repoStatus?.isCherryPicking || false);
const hasConflict = computed(
  () => conflictedFiles.value.length > 0 || isMerging.value || isRebasing.value || isCherryPicking.value
);

const isNewRepo = computed(() => {
  const status = props.repoStatus;
  if (!status) return false;
  if (status.isInitialCommit === true) return true;
  if (typeof status.totalCommits === "number" && status.totalCommits === 0) return true;
  if (status.headCommit !== undefined) return !status.headCommit;
  return !status.currentBranch;
});

const isConflictedSelected = computed(
  () => !!selectedChangeFile.value && (selectedChangeFile.value.isConflicted || conflictedFiles.value.includes(selectedChangeFile.value.path))
);

const canUseWordDiff = computed(() => {
  const file = selectedChangeFile.value;
  return !!file && !isConflictedSelected.value && file.unstagedStatus !== "?" && file.stagedStatus !== "?";
});

const allVisibleChanges = computed<FileStatus[]>(() => [
  ...conflictedFiles.value.map((path) => ({
    path,
    stagedStatus: "U",
    unstagedStatus: "U",
    isStaged: false,
    isLfs: false,
    additions: 0,
    deletions: 0,
    size: "",
    isConflicted: true,
  })),
  ...stagedFiles.value,
  ...unstagedFiles.value,
]);

const totalChangesCount = computed(
  () => stagedFiles.value.length + unstagedFiles.value.length + conflictedFiles.value.length
);

// Resizable Changes List Pane State
const changesPaneWidth = ref(initialPaneSizes.changes);
const isResizingChanges = ref(false);
const isZenMode = ref(false);

function handleZenKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && isZenMode.value) {
    isZenMode.value = false;
  }
}

// Persist both widths whenever they change (debounced; scoped by repository).
let panePersistTimer: ReturnType<typeof setTimeout> | null = null;

function persistPaneSizes() {
  if (!props.currentRepoPath) return;
  try {
    const raw = localStorage.getItem(PANE_SIZES_STORAGE_KEY);
    const map = raw ? JSON.parse(raw) : {};
    map[props.currentRepoPath] = {
      left: clampLeftPane(leftPaneWidth.value),
      changes: clampChangesPane(changesPaneWidth.value),
    };
    localStorage.setItem(PANE_SIZES_STORAGE_KEY, JSON.stringify(map));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

watch([leftPaneWidth, changesPaneWidth], () => {
  if (panePersistTimer) clearTimeout(panePersistTimer);
  panePersistTimer = setTimeout(() => {
    panePersistTimer = null;
    persistPaneSizes();
  }, 150);
});

watch(
  () => props.currentRepoPath,
  (repoPath) => {
    if (panePersistTimer) {
      clearTimeout(panePersistTimer);
      panePersistTimer = null;
    }
    const sizes = loadPaneSizes(repoPath);
    leftPaneWidth.value = sizes.left;
    changesPaneWidth.value = sizes.changes;
  }
);

const WORKSPACE_LAYOUT_KEYS: Record<string, string> = {
  standard: "ws.layout.standard",
  "focus-source": "ws.layout.focus.source",
  "focus-diff": "ws.layout.focus.diff",
};

/** Resolved lazily so the label follows the active locale. */
function workspaceLayoutLabel(preset: string) {
  const key = WORKSPACE_LAYOUT_KEYS[preset];
  return key ? t(key) : preset;
}

/** Apply a layout preset that resizes the two draggable panes at once. */
function applyWorkspaceLayout(preset: string) {
  if (preset === "save-custom") {
    saveNamedLayoutPreset(t("ws.layout.mine"));
    return;
  }
  if (preset === "custom") {
    const first = layoutPresets.value[0];
    if (!first) {
      applyCustomWorkspaceLayout();
      return;
    }
    applyNamedLayoutPreset(first);
    return;
  }
  const next = WORKSPACE_LAYOUT_KEYS[preset] ? preset : "standard";
  if (next === "focus-source") {
    leftPaneWidth.value = 480;
    changesPaneWidth.value = 240;
  } else if (next === "focus-diff") {
    leftPaneWidth.value = 240;
    changesPaneWidth.value = 240;
  } else {
    leftPaneWidth.value = DEFAULT_LEFT_PANE_WIDTH;
    changesPaneWidth.value = DEFAULT_CHANGES_PANE_WIDTH;
  }
  notify(t("ws.toast.layout.switched", { layout: workspaceLayoutLabel(next) }));
}

function handleWorkspaceLayoutEvent(event: Event) {
  const detail = (event as CustomEvent<{ preset?: string }>).detail;
  applyWorkspaceLayout(detail?.preset || "standard");
}

const CUSTOM_LAYOUT_STORAGE_KEY = "wgit.workspace.customLayout.v1";

/** Current preset derived from pane widths (custom when widths don't match a preset). */
const activeLayoutPreset = computed(() => {
  if (leftPaneWidth.value === 480 && changesPaneWidth.value === 240) return "focus-source";
  if (leftPaneWidth.value === 240 && changesPaneWidth.value === 240) return "focus-diff";
  if (leftPaneWidth.value === 320 && changesPaneWidth.value === 320) return "standard";
  return "custom";
});

function saveCustomWorkspaceLayout() {
  try {
    localStorage.setItem(
      CUSTOM_LAYOUT_STORAGE_KEY,
      JSON.stringify({ left: leftPaneWidth.value, changes: changesPaneWidth.value })
    );
    notify(t("ws.toast.layout.saved"));
  } catch {
    notify(t("ws.toast.layout.save.failed"));
  }
}

function applyCustomWorkspaceLayout() {
  try {
    const raw = localStorage.getItem(CUSTOM_LAYOUT_STORAGE_KEY);
    const parsed = raw ? JSON.parse(raw) : null;
    const left = Number(parsed?.left);
    const changes = Number(parsed?.changes);
    if (!Number.isFinite(left) || !Number.isFinite(changes)) {
      notify(t("ws.toast.layout.none"));
      return;
    }
    leftPaneWidth.value = clampLeftPane(left);
    changesPaneWidth.value = clampChangesPane(changes);
    notify(t("ws.toast.layout.custom.applied"));
  } catch {
    notify(t("ws.toast.layout.read.failed"));
  }
}

// ---- Named custom layout presets (up to 5) ----
interface NamedLayoutPreset {
  id: string;
  name: string;
  left: number;
  changes: number;
}

const LAYOUT_PRESETS_KEY = "wgit.workspace.layoutPresets.v1";
const MAX_NAMED_LAYOUT_PRESETS = 5;

function loadNamedLayoutPresets(): NamedLayoutPreset[] {
  try {
    const raw = localStorage.getItem(LAYOUT_PRESETS_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    if (Array.isArray(parsed) && parsed.length > 0) {
      return parsed
        .filter(
          (item: any) =>
            item &&
            typeof item.id === "string" &&
            typeof item.name === "string" &&
            Number.isFinite(Number(item.left)) &&
            Number.isFinite(Number(item.changes))
        )
        .map((item: any) => ({
          id: item.id,
          name: item.name.slice(0, 24),
          left: clampLeftPane(Number(item.left)),
          changes: clampChangesPane(Number(item.changes)),
        }))
        .slice(0, MAX_NAMED_LAYOUT_PRESETS);
    }
    // Migrate the previous single custom layout into the named preset list.
    const legacyRaw = localStorage.getItem(CUSTOM_LAYOUT_STORAGE_KEY);
    const legacy = legacyRaw ? JSON.parse(legacyRaw) : null;
    if (legacy && Number.isFinite(Number(legacy.left)) && Number.isFinite(Number(legacy.changes))) {
      return [
        {
          id: "legacy-custom",
          name: t("ws.layout.mine"),
          left: clampLeftPane(Number(legacy.left)),
          changes: clampChangesPane(Number(legacy.changes)),
        },
      ];
    }
    return [];
  } catch {
    // Benign: unreadable layout presets just start the list empty.
    return [];
  }
}

const layoutPresets = ref<NamedLayoutPreset[]>(loadNamedLayoutPresets());
const showLayoutMenu = ref(false);
const newLayoutPresetName = ref("");
const layoutMenuPosition = ref({ x: 0, y: 0 });
const layoutMenuButtonRef = ref<HTMLElement | null>(null);

function persistNamedLayoutPresets() {
  try {
    localStorage.setItem(LAYOUT_PRESETS_KEY, JSON.stringify(layoutPresets.value));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

function toggleLayoutMenu() {
  if (showLayoutMenu.value) {
    showLayoutMenu.value = false;
    return;
  }
  const rect = layoutMenuButtonRef.value?.getBoundingClientRect();
  if (rect) {
    layoutMenuPosition.value = {
      x: Math.max(8, Math.min(rect.right - 256, window.innerWidth - 272)),
      y: Math.min(rect.bottom + 6, window.innerHeight - 260),
    };
  }
  newLayoutPresetName.value = "";
  showLayoutMenu.value = true;
}

function saveNamedLayoutPreset(nameOverride?: string) {
  const name = (nameOverride ?? newLayoutPresetName.value).trim() || t("ws.layout.default.name", { index: layoutPresets.value.length + 1 });
  const existing = layoutPresets.value.find((preset) => preset.name === name);
  if (existing) {
    existing.left = leftPaneWidth.value;
    existing.changes = changesPaneWidth.value;
    persistNamedLayoutPresets();
    notify(t("ws.toast.preset.updated", { name }));
    newLayoutPresetName.value = "";
    return;
  }
  if (layoutPresets.value.length >= MAX_NAMED_LAYOUT_PRESETS) {
    notify(t("ws.toast.preset.limit", { max: MAX_NAMED_LAYOUT_PRESETS }));
    return;
  }
  layoutPresets.value = [
    ...layoutPresets.value,
    {
      id: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`,
      name: name.slice(0, 24),
      left: leftPaneWidth.value,
      changes: changesPaneWidth.value,
    },
  ];
  persistNamedLayoutPresets();
  newLayoutPresetName.value = "";
  notify(t("ws.toast.preset.saved", { name }));
}

function applyNamedLayoutPreset(preset: NamedLayoutPreset) {
  leftPaneWidth.value = clampLeftPane(preset.left);
  changesPaneWidth.value = clampChangesPane(preset.changes);
  showLayoutMenu.value = false;
  notify(t("ws.toast.preset.applied", { name: preset.name }));
}

function deleteNamedLayoutPreset(id: string) {
  const target = layoutPresets.value.find((preset) => preset.id === id);
  layoutPresets.value = layoutPresets.value.filter((preset) => preset.id !== id);
  persistNamedLayoutPresets();
  if (target) notify(t("ws.toast.preset.deleted", { name: target.name }));
}


function resetLeftPaneWidth() {
  persistPaneSizes();
  leftPaneWidth.value = DEFAULT_LEFT_PANE_WIDTH;
}

function resetChangesPaneWidth() {
  persistPaneSizes();
  changesPaneWidth.value = DEFAULT_CHANGES_PANE_WIDTH;
}

function startResizingChanges(e: MouseEvent) {
  isResizingChanges.value = true;
  const startX = e.clientX;
  const startWidth = changesPaneWidth.value;

  function onMouseMove(ev: MouseEvent) {
    const delta = ev.clientX - startX;
    const raw = Math.max(240, Math.min(550, startWidth + delta));
    changesPaneWidth.value = snapPaneWidth(raw, CHANGES_PANE_SNAPS);
  showResizeHint(ev, changesPaneWidth.value, t("ws.resize.changes"));
  }

  function onMouseUp() {
    isResizingChanges.value = false;
    resizeHint.value = null;
    window.removeEventListener("mousemove", onMouseMove);
    window.removeEventListener("mouseup", onMouseUp);
  }

  window.addEventListener("mousemove", onMouseMove);
  window.addEventListener("mouseup", onMouseUp);
}


// Toast Feedback System
const { toastText, showToast, notify } = useToast();
// File icon by path or extension (null-safe)
function getFileIcon(pathOrExt?: string | null) {
  if (!pathOrExt) return FileText;
  const parts = pathOrExt.split(".");
  const ext = (parts.length > 1 ? parts.pop()! : pathOrExt).toLowerCase();
  if (["png", "jpg", "jpeg", "tga", "dds", "psd", "webp", "gif", "ico", "bmp", "svg", "avif", "tif", "tiff"].includes(ext)) return Image;
  if (["uasset", "umap", "prefab", "fbx", "obj", "blend", "dae", "gltf", "glb"].includes(ext)) return Box;
  if (["csv", "xlsx", "xls", "json", "tsv"].includes(ext)) return FileSpreadsheet;
  if (["go", "ts", "mts", "cts", "tsx", "js", "mjs", "cjs", "jsx", "vue", "svelte", "py", "rs", "cpp", "h", "c", "hpp", "cs", "hlsl", "shader", "usf", "ush", "glsl", "frag", "vert", "html", "css", "scss", "sass", "less", "php", "rb", "dart", "zig", "lua", "sql", "sh", "bash", "zsh", "xml", "yaml", "yml", "toml"].includes(ext)) return Code2;
  return FileText;
}


// ---- 跨视图定位: window CustomEvent "wgit:open-file" (detail: { path, line? }) ----
// 供提交历史 / 全文搜索 / 命令面板等位置“跳到源码某一行”，同一文件可重复触发滚动。
const highlightedSourceLine = ref<number | null>(null);
let sourceLineHighlightTimer: ReturnType<typeof setTimeout> | null = null;

function clearSourceLineHighlight() {
  highlightedSourceLine.value = null;
  if (sourceLineHighlightTimer) {
    clearTimeout(sourceLineHighlightTimer);
    sourceLineHighlightTimer = null;
  }
}

function highlightSourceLine(line: number) {
  clearSourceLineHighlight();
  highlightedSourceLine.value = line;
  sourceLineHighlightTimer = setTimeout(() => {
    highlightedSourceLine.value = null;
    sourceLineHighlightTimer = null;
  }, 2200);
}


async function openSourceFileAtLine(path: string, line?: number) {
  if (!path) return;
  activeRightTab.value = "source";
  // selectSourceFile 内部会再次展开父目录并加载文件内容
  await selectSourceFile(path);
  if (line && line > 0) {
    highlightSourceLine(line);
  }
}

interface WgitOpenFileDetail {
  path?: string;
  line?: number;
  view?: "source" | "diff";
}

function handleWgitOpenFile(e: Event) {
  const detail = (e as CustomEvent<WgitOpenFileDetail>).detail;
  if (!detail || typeof detail.path !== "string" || !detail.path) return;
  const rawLine = detail.line;
  const line =
    typeof rawLine === "number" && Number.isFinite(rawLine) && rawLine > 0 ? Math.floor(rawLine) : undefined;
  if (detail.view === "diff" || (!line && conflictedFiles.value.includes(detail.path))) {
    activeRightTab.value = "diff";
    const matched =
      stagedFiles.value.find((f) => f.path === detail.path) ||
      unstagedFiles.value.find((f) => f.path === detail.path);
    if (matched) {
      void selectChangeFile(matched);
    } else {
      void selectChangeFile({
        path: detail.path,
        stagedStatus: "U",
        unstagedStatus: "U",
        isStaged: false,
        isLfs: false,
        additions: 0,
        deletions: 0,
        size: "",
        isConflicted: true,
      });
    }
    return;
  }
  void openSourceFileAtLine(detail.path, line);
}

// Select a source file on the left pane
async function selectSourceFile(path: string) {
  clearSourceLineHighlight();
  if (selectedSourceFile.value !== path) {
    selectedFileRevisionHash.value = "";
    timeMachineActive.value = false;
    timeMachineStack.value = [];
    timeMachineIndex.value = -1;
  }
  selectedSourceFile.value = path;
  activeRightTab.value = "source";
  imageZoom.value = 100;
  imageNaturalWidth.value = 0;
  imageNaturalHeight.value = 0;
  svgViewMode.value = "image";
  if (!props.currentRepoPath) return;
  const repoPath = props.currentRepoPath;
  try {
    const detail = await gitService.getSourceFileDetail(repoPath, path);
    if (props.currentRepoPath !== repoPath || selectedSourceFile.value !== path) return;
    if (detail) {
      sourceFileDetail.value = detail;
      if (showBlame.value) {
        fetchBlame();
      }
      if (showFileHistory.value) {
        fetchFileHistory();
      }
    }
  } catch (err) {
    // best-effort: the inspector falls back to the list row data.
    reportError("Failed to get source file detail", err);
  }
}

// Inspect source file code in right pane
function inspectSourceCode(path: string) {
  selectSourceFile(path);
  activeRightTab.value = "source";
}

// Select a changed file on the right pane to inspect its diff
function diffKey(file: FileStatus): string {
  return `${file.path}::${file.isStaged ? "staged" : "working"}`;
}

async function selectChangeFile(file: FileStatus) {
  const key = diffKey(file);
  const sameLoadedFile = loadedDiffKey.value === key && !isLoadingDiff.value;
  // Stacked panes show one at a time: picking a file is the cue to reveal the diff.
  activeRightTab.value = "diff";
  if (isStackedPanes.value) stackedPane.value = "diff";
  // Re-clicking the file that is already on screen must not reload it: the
  // spinner used to replace the diff for a frame, which read as a flash.
  if (sameLoadedFile) return;
  const changedFile = selectedChangeFile.value?.path !== file.path;
  selectedChangeFile.value = file;
  if (activeDiffMode.value === "word" && !canUseWordDiff.value) {
    activeDiffMode.value = "unified";
  }
  if (changedFile) {
    await nextTick();
    diffScrollRef.value?.scrollTo({ top: 0 });
  }
  await loadSelectedDiff();
}

async function fetchWorkingDiff(repoPath: string, filePath: string, staged: boolean): Promise<string> {
  if (typeof gitService.getWorkingDiffAdvanced === "function") {
    try {
      return await gitService.getWorkingDiffAdvanced(repoPath, filePath, staged, ignoreWhitespace.value);
    } catch {
      // Benign: advanced-diff RPC unavailable — fall back to the plain diff;
      // the caller's outer catch still surfaces the final failure.
      return await gitService.getWorkingDiff(repoPath, filePath, staged);
    }
  }
  return await gitService.getWorkingDiff(repoPath, filePath, staged);
}

let diffRequestSeq = 0;

async function loadSelectedDiff() {
  const file = selectedChangeFile.value;
  if (!file || !props.currentRepoPath) return;
  const seq = ++diffRequestSeq;
  isLoadingDiff.value = true;
  try {
    let diff = "";
    if (file.isConflicted || conflictedFiles.value.includes(file.path)) {
      try {
        diff = await gitService.getConflictFileContent(props.currentRepoPath, file.path);
      } catch {
        // Benign: no conflict-marker content available — fall back to the working
        // diff; the outer catch of loadSelectedDiff surfaces the final failure.
        diff = await fetchWorkingDiff(props.currentRepoPath, file.path, false);
      }
    } else if (activeDiffMode.value === "word") {
      diff = await gitService.getWorkingWordDiff(props.currentRepoPath, file.path, file.isStaged);
    } else {
      diff = await fetchWorkingDiff(props.currentRepoPath, file.path, file.isStaged);
    }
    if (seq !== diffRequestSeq) return;
    currentDiffText.value = diff || t("ws.diff.empty");
    loadedDiffKey.value = diffKey(file);
  } catch (err) {
    if (seq !== diffRequestSeq) return;
    loadedDiffKey.value = "";
    currentDiffText.value = t("ws.diff.failed", { error: err instanceof Error ? err.message : String(err) });
  } finally {
    if (seq === diffRequestSeq) isLoadingDiff.value = false;
  }
}

function toggleIgnoreWhitespace() {
  ignoreWhitespace.value = !ignoreWhitespace.value;
  void loadSelectedDiff();
}

function setDiffMode(mode: "unified" | "split" | "word") {
  if (mode === "word" && !canUseWordDiff.value) return;
  if (activeDiffMode.value === mode) return;
  activeDiffMode.value = mode;
  void loadSelectedDiff();
}

// Conflict Resolution Handler
async function handleResolveConflict(
  path: string,
  resolution: "ours" | "theirs" | "backup_and_theirs" | "mark_resolved"
) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.resolveConflict(props.currentRepoPath, path, resolution);
    const label =
      resolution === "ours"
        ? t("ws.conflict.ours")
        : resolution === "theirs"
        ? t("ws.conflict.theirs")
        : resolution === "backup_and_theirs"
        ? t("ws.conflict.backup.theirs")
        : t("ws.conflict.mark.resolved");
    notify(t("ws.toast.conflict.resolved", { file: path.split("/").pop() ?? path, label }));
    emit("refresh");
    sourceRefreshNonce.value++;
    if (selectedChangeFile.value?.path === path) {
      await selectChangeFile(selectedChangeFile.value);
    }
  } catch (err: any) {
    notify(t("ws.toast.conflict.failed", { error: err?.message || err }));
  }
}

// Block-Level Conflict Resolution Handler (VS Code CodeLens style)
async function handleResolveConflictBlock(
  blockIndex: number,
  choice: "ours" | "theirs" | "both"
) {
  if (!props.currentRepoPath || !selectedChangeFile.value) return;
  const filePath = selectedChangeFile.value.path;
  try {
    const res = await gitService.resolveConflictBlock(
      props.currentRepoPath,
      filePath,
      blockIndex,
      choice
    );
    const choiceLabel =
      choice === "ours"
        ? t("ws.conflict.ours")
        : choice === "theirs"
        ? t("ws.conflict.theirs")
        : t("workspace.diff.conflict.accept.both");

    if (res.fileAutoStaged) {
      notify(
        t("workspace.diff.conflict.auto.staged.toast", {
          file: filePath.split("/").pop() ?? filePath,
        })
      );
    } else {
      notify(
        t("workspace.diff.conflict.resolved.toast", {
          block: blockIndex + 1,
          choice: choiceLabel,
          remaining: res.remainingConflicts,
        })
      );
    }
    emit("refresh");
    sourceRefreshNonce.value++;
    if (selectedChangeFile.value?.path === filePath) {
      await selectChangeFile(selectedChangeFile.value);
    }
  } catch (err: any) {
    notify(
      t("workspace.diff.conflict.resolve.failed", { error: err?.message || err })
    );
  }
}

// AI Conflict Advisor Modal State & Handlers
const showAiConflictBatch = ref(false);
const aiConflictBatchBlocks = ref<import("../services/localAiService").ParsedConflictBlock[]>([]);
const aiConflictBatchFilePath = ref("");
const aiConflictBatchTargets = ref<BatchTarget[]>([]);

async function ensureAIConfigured(): Promise<boolean> {
  if (await isAIConfigured()) return true;
  openAiSetup();
  return false;
}

async function openAiConflictBatch(filePath?: string): Promise<void> {
  const target = filePath || selectedChangeFile.value?.path || "";
  if (!props.currentRepoPath || !target) return;
  if (!(await ensureAIConfigured())) return;
  try {
    const content = await gitService.getConflictFileContent(props.currentRepoPath, target);
    const blocks = parseConflictFileBlocks(content);
    if (blocks.length === 0) {
      notify(t("workspace.conflict.ai.no.blocks"));
      return;
    }
    aiConflictBatchBlocks.value = blocks;
    aiConflictBatchFilePath.value = target;
    aiConflictBatchTargets.value = [{ filePath: target, blocks }];
    showAiConflictBatch.value = true;
  } catch (err: any) {
    notify(t("workspace.conflict.ai.load.failed", { error: err?.message || err }));
  }
}

// Whole-merge adjudication: load every conflicted file, then run the same
// sequential, block-scoped advice/apply flow across all of them.
async function openAiConflictBatchAll(): Promise<void> {
  if (!props.currentRepoPath) return;
  if (!(await ensureAIConfigured())) return;
  const paths = [...new Set(conflictedFiles.value)];
  if (paths.length === 0) {
    notify(t("workspace.conflict.ai.no.blocks"));
    return;
  }
  try {
    const targets: BatchTarget[] = [];
    for (const filePath of paths) {
      const content = await gitService.getConflictFileContent(props.currentRepoPath, filePath);
      const blocks = parseConflictFileBlocks(content);
      if (blocks.length > 0) targets.push({ filePath, blocks });
    }
    if (targets.length === 0) {
      notify(t("workspace.conflict.ai.no.blocks"));
      return;
    }
    aiConflictBatchBlocks.value = [];
    aiConflictBatchFilePath.value = "";
    aiConflictBatchTargets.value = targets;
    showAiConflictBatch.value = true;
  } catch (err: any) {
    notify(t("workspace.conflict.ai.load.failed", { error: err?.message || err }));
  }
}

function handleOpenAiBatchFromSemantic(): void {
  const target = semanticConflictFilePath.value;
  showSemanticConflictModal.value = false;
  void openAiConflictBatch(target);
}

async function handleBatchApplied(filePath: string): Promise<void> {
  emit("refresh");
  sourceRefreshNonce.value++;
  if (selectedChangeFile.value?.path === filePath) {
    await selectChangeFile(selectedChangeFile.value);
  }
}

const showAiExplainDiff = ref(false);
const aiExplainDiffText = ref("");
const aiExplainFilePath = ref("");

async function openAiExplainDiff() {
  if (!currentDiffText.value) return;
  if (!(await ensureAIConfigured())) return;
  aiExplainDiffText.value = currentDiffText.value;
  aiExplainFilePath.value = selectedChangeFile.value?.path ?? "";
  showAiExplainDiff.value = true;
}

const showAiChat = ref(false);
const aiChatScope = ref<"diff" | "repo">("diff");
const aiChatDiff = ref("");
const aiChatFilePath = ref("");

async function openAiChat() {
  if (!currentDiffText.value) return;
  if (!(await ensureAIConfigured())) return;
  aiChatScope.value = "diff";
  aiChatDiff.value = currentDiffText.value;
  aiChatFilePath.value = selectedChangeFile.value?.path ?? "";
  showAiChat.value = true;
}

async function openAiRepoChat() {
  if (!(await ensureAIConfigured())) return;
  aiChatScope.value = "repo";
  aiChatDiff.value = "";
  aiChatFilePath.value = "";
  showAiChat.value = true;
}

const showAiReview = ref(false);
const aiReviewDiff = ref("");
const aiReviewFilePath = ref("");

async function openAiReview() {
  if (!currentDiffText.value) return;
  if (!(await ensureAIConfigured())) return;
  aiReviewDiff.value = currentDiffText.value;
  aiReviewFilePath.value = selectedChangeFile.value?.path ?? "";
  showAiReview.value = true;
}

const showAiRepoOverride = ref(false);

function openAiRepoOverride(): void {
  if (!props.currentRepoPath) return;
  showAiRepoOverride.value = true;
}

const showAiConflictAdvisor = ref(false);
const aiAdvisorFilePath = ref("");
const aiAdvisorBlockIndex = ref(0);
const aiAdvisorBlockTotal = ref(0);
const aiAdvisorOurs = ref("");
const aiAdvisorTheirs = ref("");
const aiAdvisorBase = ref("");
const aiAdvisorApplying = ref(false);
const aiAdvisorReloadNonce = ref(0);

async function loadAiAdvisorBlock(filePath: string, blockIndex: number): Promise<boolean> {
  if (!props.currentRepoPath) return false;
  try {
    const content = await gitService.getConflictFileContent(props.currentRepoPath, filePath);
    const blocks = parseConflictFileBlocks(content);
    if (blocks.length === 0) {
      notify(t("workspace.conflict.ai.no.blocks"));
      return false;
    }
    const index = Math.min(Math.max(blockIndex, 0), blocks.length - 1);
    const block = blocks[index];
    if (!block) return false;
    aiAdvisorFilePath.value = filePath;
    aiAdvisorBlockIndex.value = index;
    aiAdvisorBlockTotal.value = blocks.length;
    aiAdvisorOurs.value = block.ours;
    aiAdvisorTheirs.value = block.theirs;
    aiAdvisorBase.value = block.base || "";
    return true;
  } catch (err: any) {
    notify(t("workspace.conflict.ai.load.failed", { error: err?.message || err }));
    return false;
  }
}

async function handleOpenAiConflictAdvisor(blockIndex = 0) {
  const file = selectedChangeFile.value;
  if (!file) return;
  if (!(await ensureAIConfigured())) return;
  if (await loadAiAdvisorBlock(file.path, blockIndex)) {
    showAiConflictAdvisor.value = true;
  }
}

function handleOpenAiSettings() {
  showAiConflictAdvisor.value = false;
  openAiSetup();
}

async function handleApplyAiConflictResolution(payload: {
  blockIndex: number;
  content: string;
  expectedOurs: string;
  expectedTheirs: string;
  expectedBase: string;
}) {
  const repoPath = props.currentRepoPath;
  const filePath = aiAdvisorFilePath.value;
  if (!repoPath || !filePath || aiAdvisorApplying.value) return;
  aiAdvisorApplying.value = true;
  try {
    const res = await gitService.resolveConflictBlockWithCustomContent(
      repoPath,
      filePath,
      payload.blockIndex,
      payload.content,
      payload.expectedOurs,
      payload.expectedTheirs,
      payload.expectedBase
    );
    if (!res) {
      notify(t("workspace.diff.conflict.resolve.failed", { error: "" }));
      return;
    }
    if (res.stale) {
      notify(t("workspace.conflict.ai.stale"));
      if (await loadAiAdvisorBlock(filePath, payload.blockIndex)) {
        aiAdvisorReloadNonce.value++;
      } else {
        showAiConflictAdvisor.value = false;
      }
      return;
    }
    if (!res.success) {
      notify(t("workspace.diff.conflict.resolve.failed", { error: res.message || "" }));
      return;
    }
    showAiConflictAdvisor.value = false;
    if (res.fileAutoStaged) {
      notify(
        t("workspace.diff.conflict.auto.staged.toast", {
          file: filePath.split("/").pop() ?? filePath,
        })
      );
    } else {
      notify(
        t("workspace.diff.conflict.resolved.toast", {
          block: res.blockIndex + 1,
          choice: t("workspace.conflict.ai.advisor"),
          remaining: res.remainingConflicts,
        })
      );
    }
    emit("refresh");
    sourceRefreshNonce.value++;
    if (selectedChangeFile.value?.path === filePath) {
      await selectChangeFile(selectedChangeFile.value);
    }
  } catch (err: any) {
    notify(
      t("workspace.diff.conflict.resolve.failed", { error: err?.message || err })
    );
  } finally {
    aiAdvisorApplying.value = false;
  }
}

async function handleAutoResolveMetaConflicts(strategy: "theirs" | "backup_theirs" = "theirs") {
  if (!props.currentRepoPath) return;
  try {
    const res = await gitService.autoResolveMetaConflicts(props.currentRepoPath, strategy);
    if (!res || res.count === 0) {
      notify(t("ws.toast.meta.conflict.none"));
      return;
    }
    notify(t("ws.toast.meta.conflict.resolved", { count: res.count, strategy: strategy === "theirs" ? "Theirs" : "Backup+Theirs" }));
    emit("refresh");
    sourceRefreshNonce.value++;
    if (selectedChangeFile.value && res.resolvedFiles.includes(selectedChangeFile.value.path)) {
      await selectChangeFile(selectedChangeFile.value);
    }
  } catch (err: any) {
    notify(t("ws.toast.conflict.failed", { error: err?.message || err }));
  }
}

async function handleConvertStagedNonLfs() {
  if (!props.currentRepoPath || stagedNonLfsLargeFiles.value.length === 0 || isConvertingLfs.value) return;
  const paths = stagedNonLfsLargeFiles.value.map((f) => f.path);
  isConvertingLfs.value = true;
  try {
    const res = await gitService.trackStagedLargeFilesAsLfs(props.currentRepoPath, paths, true);
    notify(res?.message || t("ws.staged.large.success"));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.conflict.failed", { error: err?.message || err }));
  } finally {
    isConvertingLfs.value = false;
  }
}

/** Localised name of the in-progress operation, used by abort/continue/skip. */
function currentOperationName() {
  if (isRebasing.value) return t("ws.op.rebase");
  if (isCherryPicking.value) return t("ws.op.cherry.pick");
  return t("ws.op.merge");
}

// Conflict Lifecycle Operations (Abort / Continue / Skip)
async function handleAbortConflict(alreadyConfirmed = false) {
  if (!props.currentRepoPath) return;
  const opName = currentOperationName();
  if (!alreadyConfirmed && !(await confirmAction({ message: t("ws.confirm.op.abort", { op: opName }), tone: "danger" }))) return;
  try {
    await gitService.abortConflictOperation(props.currentRepoPath);
    notify(t("ws.toast.op.aborted", { op: opName }));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.op.abort.failed", { error: err?.message || err }));
  }
}

async function handleContinueConflict() {
  if (!props.currentRepoPath) return;
  const opName = currentOperationName();
  try {
    const msg = await gitService.continueConflictOperation(props.currentRepoPath);
    notify(t("ws.toast.op.continued", { op: opName, message: msg || t("ws.result.success") }));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.op.continue.failed", { op: opName, error: err?.message || err }));
  }
}

async function handleSkipConflict() {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("ws.confirm.op.skip"), tone: "danger" }))) return;
  try {
    const msg = await gitService.skipConflictOperation(props.currentRepoPath);
    notify(t("ws.toast.op.skipped", { message: msg || t("ws.result.success") }));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.op.skip.failed", { error: err?.message || err }));
  }
}

// Stage single file
async function handleStageFile(path: string) {
  if (!props.currentRepoPath) return;
  if (selectedChangeFile.value && selectedChangeFile.value.path === path) {
    selectedChangeFile.value = {
      ...selectedChangeFile.value,
      isStaged: true,
      stagedStatus: selectedChangeFile.value.unstagedStatus || "M",
    };
    void loadSelectedDiff();
  }
  try {
    await gitService.stageFile(props.currentRepoPath, path);
    notify(t("ws.toast.staged", { file: path.split("/").pop() ?? path }));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.stage.failed", { error: err?.message || err }));
  }
}

// Unstage single file
async function handleUnstageFile(path: string) {
  if (!props.currentRepoPath) return;
  if (selectedChangeFile.value && selectedChangeFile.value.path === path) {
    selectedChangeFile.value = {
      ...selectedChangeFile.value,
      isStaged: false,
    };
    void loadSelectedDiff();
  }
  try {
    await gitService.unstageFile(props.currentRepoPath, path);
    notify(t("ws.toast.unstaged", { file: path.split("/").pop() ?? path }));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.unstage.failed", { error: err?.message || err }));
  }
}

async function handlePartialStashCurrentFile() {
  if (!props.currentRepoPath || !selectedChangeFile.value?.path) return;
  const path = selectedChangeFile.value.path;
  const filename = path.split("/").pop() || path;
  try {
    await gitService.createPartialStash(
      props.currentRepoPath,
      `partial: stash ${filename}`,
      [path]
    );
    notify(t("ws.partial.stash.success", { count: 1 }));
    emit("refresh");
  } catch (err: any) {
    notify(t("ws.partial.stash.failed", { error: err?.message || err }));
  }
}

// Stage all
async function handleStageAll() {
  if (!props.currentRepoPath) return;
  if (selectedChangeFile.value && !selectedChangeFile.value.isConflicted) {
    selectedChangeFile.value = {
      ...selectedChangeFile.value,
      isStaged: true,
    };
    void loadSelectedDiff();
  }
  try {
    await gitService.stageAll(props.currentRepoPath);
    notify(t("ws.toast.all.staged"));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.all.stage.failed", { error: err?.message || err }));
  }
}

/**
 * How many files "discard all" is about to touch.
 *
 * The same path can appear as staged *and* unstaged (partially staged file), so
 * the count is de-duplicated — a summary that says "7 files" when three of them
 * are the same file twice is worse than no summary.
 */
const discardAllFileCount = computed(() => {
  const status = props.repoStatus;
  if (!status) return 0;
  const paths = new Set<string>();
  for (const file of [
    ...(status.stagedFiles ?? []),
    ...(status.unstagedFiles ?? []),
    ...(status.untrackedFiles ?? []),
  ]) {
    paths.add(file.path);
  }
  return paths.size;
});

async function handleDiscardAll() {
  if (!props.currentRepoPath) return;
  if (
    !(await confirmAction({
      message: t("ws.confirm.discard.all"),
      tone: "danger",
      affectedSummary: t("ws.confirm.discard.all.scope", { count: discardAllFileCount.value }),
    }))
  )
    return;
  try {
    await gitService.discardAll(props.currentRepoPath);
    selectedChangeFile.value = null;
    notify(t("ws.toast.all.discarded"));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.all.discard.failed", { error: err?.message || err }));
  }
}

// Unstage all
async function handleUnstageAll() {
  if (!props.currentRepoPath) return;
  if (selectedChangeFile.value && !selectedChangeFile.value.isConflicted) {
    selectedChangeFile.value = {
      ...selectedChangeFile.value,
      isStaged: false,
    };
    void loadSelectedDiff();
  }
  try {
    await gitService.unstageAll(props.currentRepoPath);
    notify(t("ws.toast.all.unstaged"));
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.all.unstage.failed", { error: err?.message || err }));
  }
}

// Discard single file
async function handleDiscardFile(path: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("ws.confirm.discard.file", { path }), tone: "danger" }))) return;
  try {
    await gitService.discardFile(props.currentRepoPath, path);
    notify(t("ws.toast.file.discarded", { file: path.split("/").pop() ?? path }));
    emit("refresh");
    sourceRefreshNonce.value++;
    if (selectedChangeFile.value?.path === path) {
      selectedChangeFile.value = null;
      currentDiffText.value = "";
      autoSelectFirstChange();
    }
  } catch (err: any) {
    notify(t("ws.toast.file.discard.failed", { error: err?.message || err }));
  }
}

async function handleStageHunk(patch: string) {
  if (!props.currentRepoPath || !patch) return;
  isHunkActionBusy.value = true;
  try {
    await gitService.stagePatchHunk(props.currentRepoPath, patch, false);
    notify(t("ws.toast.hunk.staged"));
    emit("refresh");
    sourceRefreshNonce.value++;
    await loadSelectedDiff();
  } catch (err: any) {
    notify(t("ws.toast.hunk.stage.failed", { error: err?.message || err }));
  } finally {
    isHunkActionBusy.value = false;
  }
}

async function handleUnstageHunk(patch: string) {
  if (!props.currentRepoPath || !patch) return;
  isHunkActionBusy.value = true;
  try {
    await gitService.stagePatchHunk(props.currentRepoPath, patch, true);
    notify(t("ws.toast.hunk.unstaged"));
    emit("refresh");
    sourceRefreshNonce.value++;
    await loadSelectedDiff();
  } catch (err: any) {
    notify(t("ws.toast.hunk.unstage.failed", { error: err?.message || err }));
  } finally {
    isHunkActionBusy.value = false;
  }
}

async function handleDiscardHunk(patch: string) {
  if (!props.currentRepoPath || !patch) return;
  if (!(await confirmAction({ message: t("ws.confirm.discard.hunk"), tone: "danger" }))) return;
  isHunkActionBusy.value = true;
  try {
    await gitService.discardHunk(props.currentRepoPath, patch);
    notify(t("ws.toast.hunk.discarded"));
    emit("refresh");
    sourceRefreshNonce.value++;
    await loadSelectedDiff();
  } catch (err: any) {
    notify(t("ws.toast.hunk.discard.failed", { error: err?.message || err }));
  } finally {
    isHunkActionBusy.value = false;
  }
}

async function handleStageLines(patch: string) {
  if (!props.currentRepoPath || !patch) return;
  isHunkActionBusy.value = true;
  try {
    await gitService.stageLines(props.currentRepoPath, patch);
    notify(t("ws.toast.lines.staged"));
    emit("refresh");
    sourceRefreshNonce.value++;
    await loadSelectedDiff();
  } catch (err: any) {
    notify(t("ws.toast.lines.stage.failed", { error: err?.message || err }));
  } finally {
    isHunkActionBusy.value = false;
  }
}

async function handleDiscardLines(patch: string) {
  if (!props.currentRepoPath || !patch) return;
  if (!(await confirmAction({ message: t("ws.confirm.discard.lines"), tone: "danger" }))) return;
  isHunkActionBusy.value = true;
  try {
    await gitService.discardLines(props.currentRepoPath, patch);
    notify(t("ws.toast.lines.discarded"));
    emit("refresh");
    sourceRefreshNonce.value++;
    await loadSelectedDiff();
  } catch (err: any) {
    notify(t("ws.toast.lines.discard.failed", { error: err?.message || err }));
  } finally {
    isHunkActionBusy.value = false;
  }
}

// Binary asset comparison — the one thing a text diff can never show for a
// texture / model / audio change.
const ASSET_EXTS = new Set([
  ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".tga", ".psd", ".psb", ".exr", ".tif", ".tiff", ".hdr",
  ".wav", ".mp3", ".ogg", ".bank", ".fsb", ".wem", ".flac",
  ".fbx", ".obj", ".glb", ".gltf", ".ma", ".mb", ".blend", ".max", ".uasset", ".umap", ".unity", ".prefab",
]);

function isAssetPath(path: string): boolean {
  const idx = path.lastIndexOf(".");
  return idx >= 0 && ASSET_EXTS.has(path.slice(idx).toLowerCase());
}

const showAssetDiff = ref(false);
const assetDiffPath = ref("");
const assetDiffOldRef = ref("HEAD");
const assetDiffNewRef = ref("WORKTREE");

const canCompareAsset = computed(() => {
  const f = selectedChangeFile.value;
  if (!f || isConflictedSelected.value) return false;
  return isAssetPath(f.path);
});

function handleOpenAssetDiff() {
  const f = selectedChangeFile.value;
  if (!f) return;
  assetDiffPath.value = f.path;
  assetDiffOldRef.value = "HEAD";
  // A staged change lives in the index; an unstaged one only exists on disk.
  assetDiffNewRef.value = f.isStaged ? ":0" : "WORKTREE";
  showAssetDiff.value = true;
}

// External diff & merge tool helpers
async function handleOpenExternalDiff(filePath: string, isStaged: boolean) {  if (!props.currentRepoPath || !filePath) return;
  try {
    await gitService.openExternalDiff(props.currentRepoPath, filePath, isStaged);
    notify(t("ws.toast.external.diff.opened"));
  } catch (err: any) {
    notify(t("ws.toast.external.diff.failed", { error: err?.message || err }));
  }
}

async function handleOpenExternalMerge(filePath: string) {
  if (!props.currentRepoPath || !filePath) return;
  try {
    await gitService.openExternalMerge(props.currentRepoPath, filePath);
    notify(t("ws.toast.external.merge.started"));
  } catch (err: any) {
    notify(t("ws.toast.external.merge.failed", { error: err?.message || err }));
  }
}

async function handleCopyProjectFileList() {
  if (!props.currentRepoPath) return;
  try {
    const files = await gitService.getProjectAllFiles(props.currentRepoPath);
    if (!files || files.length === 0) {
      notify(t("ws.toast.no.tracked.files"));
      return;
    }
    const text = files.join("\n");
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // Benign: clipboard API rejected — best-effort execCommand fallback before
      // the outer copy-failure toast.
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    }
    notify(t("ws.toast.paths.copied", { count: files.length }));
  } catch (err: any) {
    notify(t("ws.toast.paths.copy.failed", { error: err?.message || err }));
  }
}

async function handleRevealSourceFile(filePath: string) {
  if (!props.currentRepoPath || !filePath) return;
  try {
    await gitService.revealFileInFinder(props.currentRepoPath, filePath);
  } catch (err: any) {
    notify(t("ws.toast.reveal.failed", { error: err?.message || err }));
  }
}

async function handleOpenSourceFileDefault(filePath: string) {
  if (!props.currentRepoPath || !filePath) return;
  try {
    await gitService.openFileWithDefaultApp(props.currentRepoPath, filePath);
  } catch (err: any) {
    notify(t("ws.toast.open.file.failed", { error: err?.message || err }));
  }
}

function handleOpenDiffFileInEditor() {
  if (selectedChangeFile.value?.path) {
    void handleOpenSourceFileDefault(selectedChangeFile.value.path);
  }
}

async function handleOpenFileInWeb(filePath: string) {
  if (!props.currentRepoPath || !filePath) return;
  try {
    await gitService.openFileInWeb(props.currentRepoPath, filePath);
  } catch (err: any) {
    notify(t("ws.toast.open.web.failed", { error: err?.message || err }));
  }
}

// Copy source code
function copySourceCode() {
  if (!sourceFileDetail.value?.content) return;
  navigator.clipboard.writeText(sourceFileDetail.value.content);
  copiedCodeToast.value = true;
  setTimeout(() => {
    copiedCodeToast.value = false;
  }, 1800);
}

// Generate smart commit message
function generateSmartCommitMessage() {
  if (stagedFiles.value.length === 0 && unstagedFiles.value.length === 0) {
    commitSubject.value = t("ws.commit.empty.subject");
    return;
  }

  const files = stagedFiles.value.length > 0 ? stagedFiles.value : unstagedFiles.value;
  const paths = files.map((f) => f.path);

  const hasGo = paths.some((p) => p.endsWith(".go"));
  const hasVue = paths.some((p) => p.endsWith(".vue") || p.endsWith(".ts"));
  const hasAsset = paths.some((p) =>
    p.endsWith(".uasset") || p.endsWith(".prefab") || p.endsWith(".shader") || p.endsWith(".fbx")
  );

  let prefix = "feat";
  if (paths.some((p) => p.includes("test"))) prefix = "test";
  else if (paths.some((p) => p.includes("fix") || p.includes("bug"))) prefix = "fix";
  else if (hasAsset) prefix = "asset";

  let scope = "core";
  if (hasVue) scope = "ui";
  else if (hasGo) scope = "service";
  else if (hasAsset) scope = "pipeline";

  const firstPath = paths[0]?.split("/").pop() || "workspace";
  commitSubject.value = `${prefix}(${scope}): ${t("ws.commit.summary", { path: firstPath })}${paths.length > 1 ? t("ws.commit.summary.more", { count: paths.length - 1 }) : ""}`;
  commitBody.value = `${t("ws.commit.body.title")}:\n${paths.map((p) => `- ${p}`).join("\n")}`;
}

async function handleAuditSentinel() {
  if (!props.currentRepoPath) return;
  if (stagedFiles.value.length === 0) {
    notify(t("ws.composer.audit.no.staged"));
    return;
  }
  notify(t("ws.composer.audit.scanning"));
  try {
    const findings = await gitService.scanStagedSecrets(props.currentRepoPath);
    if (!findings || findings.length === 0) {
      notify(t("ws.composer.audit.clean"));
    } else {
      notify(t("ws.composer.audit.issues", { count: findings.length }));
    }
  } catch (err: any) {
    notify(t("ws.composer.audit.failed", { error: err?.message || err }));
  }
}

// Submit commit
async function handleLoadCommitTemplate() {
  if (!props.currentRepoPath) return;
  try {
    const template = await gitService.getCommitTemplate(props.currentRepoPath);
    if (!template?.hasTemplate || !template.content.trim()) {
      notify(t("ws.toast.template.missing"));
      return;
    }
    const lines = template.content.split("\n");
    const subjectIdx = lines.findIndex((line) => line.trim() && !line.trimStart().startsWith("#"));
    if (subjectIdx < 0) {
      notify(t("ws.toast.template.comment.only"));
      return;
    }
    commitSubject.value = lines[subjectIdx]!.trim();
    commitBody.value = lines.slice(subjectIdx + 1).join("\n").trim();
    notify(t("ws.toast.template.loaded"));
  } catch (err: any) {
    notify(t("ws.toast.template.load.failed", { error: err?.message || err }));
  }
}

async function handleSaveCommitTemplate() {
  if (!props.currentRepoPath) return;
  if (!commitSubject.value.trim() && !commitBody.value.trim()) {
    notify(t("ws.toast.commit.message.required"));
    return;
  }
  if (!(await confirmAction(t("ws.confirm.template.save")))) return;
  const content = commitBody.value.trim()
    ? `${commitSubject.value.trim()}\n\n${commitBody.value.trim()}\n`
    : `${commitSubject.value.trim()}\n`;
  try {
    await gitService.saveCommitTemplate(props.currentRepoPath, content, true);
    notify(t("ws.toast.template.saved"));
  } catch (err: any) {
    notify(t("ws.toast.template.save.failed", { error: err?.message || err }));
  }
}

async function loadWorkspaceStats() {
  if (!props.currentRepoPath) return;
  const repoPath = props.currentRepoPath;
  const seq = ++workspaceStatsSeq;
  isLoadingWorkspaceStats.value = true;
  try {
    const stats = await gitService.getWorkspaceStats(repoPath);
    if (seq === workspaceStatsSeq) workspaceStats.value = stats;
  } catch (err) {
    // best-effort: the stats strip hides itself when it has no data.
    console.debug("Failed to load workspace stats:", err);
  } finally {
    if (seq === workspaceStatsSeq) isLoadingWorkspaceStats.value = false;
  }
}

async function handleLoadLastCommitMessage() {
  if (!props.currentRepoPath) return;
  if (
    (commitSubject.value.trim() || commitBody.value.trim()) &&
    !(await confirmAction({ message: t("ws.confirm.last.message.overwrite"), tone: "danger" }))
  )
    return;
  try {
    const message = (await gitService.getLastCommitMessage(props.currentRepoPath)) || "";
    const lines = message.replace(/\s+$/, "").split("\n");
    commitSubject.value = lines[0]?.trim() || "";
    commitBody.value = lines.slice(1).join("\n").trim();
    notify(t("ws.toast.last.message.loaded"));
  } catch (err: any) {
    notify(t("ws.toast.last.message.failed", { error: err?.message || err }));
  }
}

async function handleCopyImageBase64() {
  if (!props.currentRepoPath || !selectedSourceFile.value) return;
  try {
    const dataUri = await gitService.getImageBase64(
      props.currentRepoPath,
      selectedFileRevisionHash.value || "",
      selectedSourceFile.value
    );
    try {
      await navigator.clipboard.writeText(dataUri);
    } catch {
      // Benign: clipboard API rejected — best-effort execCommand fallback before
      // the outer copy-failure toast.
      const textarea = document.createElement("textarea");
      textarea.value = dataUri;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    }
    notify(t("ws.toast.image.base64.copied", { count: dataUri.length }));
  } catch (err: any) {
    notify(t("ws.toast.image.base64.failed", { error: err?.message || err }));
  }
}

async function showRevisionDiffFor(commitHash: string) {
  if (!props.currentRepoPath || !selectedSourceFile.value || isLoadingRevisionDiff.value) return;
  revisionDiffHash.value = commitHash;
  revisionDiff.value = "";
  showRevisionDiff.value = true;
  isLoadingRevisionDiff.value = true;
  try {
    revisionDiff.value =
      (await gitService.getSourceFileDiff(
        props.currentRepoPath,
        selectedSourceFile.value,
        commitHash
      )) || "";
  } catch (err: any) {
    revisionDiff.value = t("ws.diff.revision.failed", { error: err?.message || err });
  } finally {
    isLoadingRevisionDiff.value = false;
  }
}

async function handleFastCommit() {
  if (!props.currentRepoPath || !commitSubject.value.trim()) return;
  if (
    !(await confirmAction(
      t("ws.confirm.quick.commit")
    ))
  )
    return;
  isCommitting.value = true;
  try {
    const message = commitBody.value.trim()
      ? `${commitSubject.value.trim()}\n\n${commitBody.value.trim()}`
      : commitSubject.value.trim();
    const res = await gitService.fastCommit(props.currentRepoPath, message);
    notify(res?.message || t("ws.toast.quick.commit.done"));
    commitSubject.value = "";
    commitBody.value = "";
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.quick.commit.failed", { error: err?.message || err }));
  } finally {
    isCommitting.value = false;
  }
}

async function handleCopyRevisionContent() {
  if (!props.currentRepoPath || !selectedSourceFile.value || !selectedFileRevisionHash.value) return;
  try {
    const content = await gitService.getFileContentAtCommit(
      props.currentRepoPath,
      selectedFileRevisionHash.value,
      selectedSourceFile.value
    );
    try {
      await navigator.clipboard.writeText(content);
    } catch {
      // Benign: clipboard API rejected — best-effort execCommand fallback before
      // the outer copy-failure toast.
      const textarea = document.createElement("textarea");
      textarea.value = content;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    }
    notify(t("ws.toast.revision.copied", { count: content.length }));
  } catch (err: any) {
    notify(t("ws.toast.revision.copy.failed", { error: err?.message || err }));
  }
}

async function handleCommit(noVerifyParam?: boolean) {
  if (!commitSubject.value.trim() || !props.currentRepoPath) return;
  const noVerify = typeof noVerifyParam === "boolean" ? noVerifyParam : isNoVerify.value;
  isCommitting.value = true;
  try {
    // Enterprise compliance policy pre-check
    if (!noVerify) {
      const policyRes = await gitService.validateCommitPolicy(
        props.currentRepoPath,
        commitSubject.value.trim(),
        "",
        props.repoStatus?.currentBranch || ""
      );
      if (policyRes && !policyRes.valid && policyRes.violations.length > 0) {
        notify(
          t("ws.compliance.policy.blocked", {
            reason: policyRes.violations.join("; "),
          })
        );
        isCommitting.value = false;
        return;
      }
    }

    const fullMsg = commitBody.value
      ? `${commitSubject.value.trim()}\n\n${commitBody.value.trim()}`
      : commitSubject.value.trim();

    // If no files staged, stage all first
    if (stagedFiles.value.length === 0 && unstagedFiles.value.length > 0) {
      await gitService.stageAll(props.currentRepoPath);
    }

    await gitService.commitAdvanced(props.currentRepoPath, fullMsg, isAmend.value, noVerify);

    if (autoPush.value) {
      await gitService.push(props.currentRepoPath);
    }

    commitSubject.value = "";
    commitBody.value = "";
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    const rawError = err?.message || String(err);
    const isHookBlocked =
      /hook/i.test(rawError) ||
      /pre-commit/i.test(rawError) ||
      /commit-msg/i.test(rawError) ||
      /rejected by/i.test(rawError);

    if (isHookBlocked && !noVerify) {
      notify(t("ws.toast.commit.failed.hook", { error: rawError }));
    } else {
      notify(t("ws.toast.commit.failed", { error: rawError }));
    }
  } finally {
    isCommitting.value = false;
  }
}


// Source Code Viewer highlighted lines
const sourceCodeLines = computed(() => {
  if (!sourceFileDetail.value?.content) return [];
  return sourceFileDetail.value.content.split("\n");
});

/**
 * Prism plus twenty language grammars is a ~100 kB chunk, and the source viewer
 * is the only thing that needs it. Loading it statically put all of that in the
 * entry chunk, so every launch paid for a pane most sessions never open.
 *
 * The viewer falls back to unhighlighted text for the frame or two before the
 * chunk resolves — same content, no layout shift.
 */
type Highlighter = (code: string, languageOrExt?: string) => string;
let highlighter: Highlighter | null = null;

async function loadHighlighter(): Promise<Highlighter> {
  if (!highlighter) {
    const module = await import("../utils/prismLanguages");
    highlighter = module.highlightCode;
  }
  return highlighter;
}

const highlightedSourceHtml = ref("");
const highlightedSourceLines = computed<string[]>(() => {
  const html = highlightedSourceHtml.value;
  return html ? html.split("\n") : sourceCodeLines.value;
});

watch(
  [() => sourceFileDetail.value?.content, () => sourceFileDetail.value?.language],
  async ([content, language]) => {
    if (!content) {
      highlightedSourceHtml.value = "";
      return;
    }
    const highlight = await loadHighlighter();
    // A newer file may have been opened while the chunk was loading.
    if (sourceFileDetail.value?.content !== content) return;
    highlightedSourceHtml.value = highlight(content, language);
  },
  { immediate: true }
);

function autoSelectFirstChange() {
  if (conflictedFiles.value[0]) {
    selectChangeFile({
      path: conflictedFiles.value[0],
      stagedStatus: "U",
      unstagedStatus: "U",
      isStaged: false,
      isLfs: false,
      additions: 0,
      deletions: 0,
      size: "",
      isConflicted: true,
    });
  } else if (stagedFiles.value[0]) {
    selectChangeFile(stagedFiles.value[0]);
  } else if (unstagedFiles.value[0]) {
    selectChangeFile(unstagedFiles.value[0]);
  }
}

watch(
  () => props.currentRepoPath,
  () => {
    selectedChangeFile.value = null;
    selectedSourceFile.value = null;
    sourceFileDetail.value = null;
    fileHistoryList.value = [];
    showFileHistory.value = false;
    selectedFileRevisionHash.value = "";
    timeMachineActive.value = false;
    timeMachineStack.value = [];
    timeMachineIndex.value = -1;
    blameLines.value = [];
    showBlame.value = false;
    commitSubject.value = "";
    commitBody.value = "";
    sourceRefreshNonce.value++;
    autoSelectFirstChange();
    workspaceStats.value = null;
    void loadWorkspaceStats();
  },
  { immediate: true }
);

watch(
  () => props.repoStatus,
  (newStatus) => {
    if (!newStatus) return;
    if (!selectedChangeFile.value) {
      autoSelectFirstChange();
    }
    void loadWorkspaceStats();
  },
  { immediate: true, deep: true }
);

function handleWorkspaceKeydown(e: KeyboardEvent) {
  if (showRevisionDiff.value) return;
  const target = e.target as HTMLElement | null;
  const isInput =
    target &&
    (target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      target.isContentEditable);

  // Global Commit Shortcut: ⌘+Enter / Ctrl+Enter (skipped when rebound in Settings)
  if ((e.metaKey || e.ctrlKey) && e.key === "Enter" && !isActionCustomized("commit")) {
    e.preventDefault();
    void handleCommit();
    return;
  }

  // If typing in input, ignore navigation keys
  if (isInput) {
    if (e.key === "Escape") {
      target.blur();
    }
    return;
  }

  // ⌥S toggles Unified / Split diff view (only while the diff tab is active).
  if (
    e.altKey &&
    !e.metaKey &&
    !e.ctrlKey &&
    e.key.toLowerCase() === "s" &&
    !isActionCustomized("toggle-diff-split")
  ) {
    if (activeRightTab.value !== "diff") return;
    e.preventDefault();
    toggleDiffSplitWithToast();
    return;
  }

  // Vim Navigation Mode integration
  if (isVimModeEnabled.value) {
    const list = allVisibleChanges.value;
    const curIdx = selectedChangeFile.value
      ? list.findIndex((item) => item.path === selectedChangeFile.value?.path)
      : -1;

    const handled = handleVimKeydown(e, {
      onNext: () => {
        if (list.length === 0) return;
        const nextIdx = curIdx < list.length - 1 ? curIdx + 1 : 0;
        selectChangeFile(list[nextIdx]!);
      },
      onPrev: () => {
        if (list.length === 0) return;
        const prevIdx = curIdx > 0 ? curIdx - 1 : list.length - 1;
        selectChangeFile(list[prevIdx]!);
      },
      onToggleStage: () => {
        if (selectedChangeFile.value && !isConflictedSelected.value) {
          if (selectedChangeFile.value.isStaged) {
            handleUnstageFile(selectedChangeFile.value.path);
          } else {
            handleStageFile(selectedChangeFile.value.path);
          }
        }
      },
      onStage: () => {
        if (selectedChangeFile.value && !isConflictedSelected.value && !selectedChangeFile.value.isStaged) {
          handleStageFile(selectedChangeFile.value.path);
        }
      },
      onDiscard: () => {
        if (selectedChangeFile.value) {
          handleDiscardFile(selectedChangeFile.value.path);
        }
      },
      onFocusCommit: () => {
        void expandComposerForFocus();
      },
      onEnterInsert: () => {
        void expandComposerForFocus();
      },
      onNextTab: () => {
        window.dispatchEvent(new CustomEvent("wgit:tab-next"));
      },
      onPrevTab: () => {
        window.dispatchEvent(new CustomEvent("wgit:tab-prev"));
      },
    });
    if (handled) return;
  }

  // When active right tab is 'diff', allow change list navigation (fallback)
  if (activeRightTab.value === "diff" && allVisibleChanges.value.length > 0) {
    const list = allVisibleChanges.value;
    const curIdx = selectedChangeFile.value
      ? list.findIndex((item) => item.path === selectedChangeFile.value?.path)
      : -1;

    if (e.key === "ArrowDown" || e.key === "j") {
      e.preventDefault();
      const nextIdx = curIdx < list.length - 1 ? curIdx + 1 : 0;
      selectChangeFile(list[nextIdx]!);
    } else if (e.key === "ArrowUp" || e.key === "k") {
      e.preventDefault();
      const prevIdx = curIdx > 0 ? curIdx - 1 : list.length - 1;
      selectChangeFile(list[prevIdx]!);
    } else if (e.key === " " /* Space */) {
      e.preventDefault();
      if (selectedChangeFile.value && !isConflictedSelected.value) {
        if (selectedChangeFile.value.isStaged) {
          handleUnstageFile(selectedChangeFile.value.path);
        } else {
          handleStageFile(selectedChangeFile.value.path);
        }
      }
    }
  }
}

function toggleDiffSplitWithToast() {
  if (activeRightTab.value !== "diff") return;
  setDiffMode(activeDiffMode.value === "split" ? "unified" : "split");
  notify(activeDiffMode.value === "split" ? t("ws.toast.diff.split") : t("ws.toast.diff.unified"));
}

/** Custom keyboard shortcut dispatches from App.vue (Settings recorder). */
function handleShortcutToggleDiffSplit() {
  toggleDiffSplitWithToast();
}

function handleShortcutCommit() {
  void handleCommit();
}

onMounted(() => {
  window.addEventListener("wgit:workspace-layout", handleWorkspaceLayoutEvent);
  window.addEventListener("wgit:workspace-toggle-diff-split", handleShortcutToggleDiffSplit);
  window.addEventListener("wgit:workspace-commit", handleShortcutCommit);
  autoSelectFirstChange();
  void loadWorkspaceStats();
  window.addEventListener("keydown", handleWorkspaceKeydown);
  window.addEventListener("keydown", handleZenKeydown);
  window.addEventListener("wgit:open-file", handleWgitOpenFile);
  if (workspaceRootRef.value && typeof ResizeObserver !== "undefined") {
    workspaceWidth.value = workspaceRootRef.value.clientWidth;
    workspaceResizeObserver = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) workspaceWidth.value = Math.round(entry.contentRect.width);
    });
    workspaceResizeObserver.observe(workspaceRootRef.value);
  }
});

onUnmounted(() => {
  window.removeEventListener("wgit:workspace-layout", handleWorkspaceLayoutEvent);
  window.removeEventListener("wgit:workspace-toggle-diff-split", handleShortcutToggleDiffSplit);
  window.removeEventListener("wgit:workspace-commit", handleShortcutCommit);
  window.removeEventListener("keydown", handleWorkspaceKeydown);
  window.removeEventListener("keydown", handleZenKeydown);
  window.removeEventListener("wgit:open-file", handleWgitOpenFile);
  workspaceResizeObserver?.disconnect();
  workspaceResizeObserver = null;
  clearSourceLineHighlight();
});
</script>

<template>
  <div ref="workspaceRootRef" class="flex-1 flex overflow-hidden select-none">
    <!-- Pane resize width hint -->
    <div
      v-if="resizeHint"
      class="fixed z-50 pointer-events-none px-2 py-1 rounded-md bg-surface/95 border border-line shadow-lg text-[10px] font-mono text-ink whitespace-nowrap"
      :style="{ left: `${resizeHint.x}px`, top: `${resizeHint.y}px` }"
    >
      {{ resizeHint.label }} {{ resizeHint.width }}px
    </div>

    <!-- ======================================================== -->
    <!-- LEFT PANE: 源码工程浏览器 -->
    <!-- `hidden` keeps the tree's expansion state while the pane is away. -->
    <!-- The wrapper carries the narrow-window collapse (§2.1 item 9) so the
         two pinned per-pane show/hide bindings below keep meaning only the
         user's explicit choice; the collapse is a width-driven layer on top. -->
    <div v-show="showSourcePane && !isLeftPaneCollapsed" class="flex shrink-0">
      <SourceExplorer
        v-show="showSourcePane"
        :repo-path="currentRepoPath"
        :width="effectiveLeftPaneWidth"
        :resizing="isResizing"
        :selected-path="selectedSourceFile"
        :refresh-nonce="sourceRefreshNonce"
        :source-file-detail="sourceFileDetail"
        @open-file="selectSourceFile"
        @copy-file-list="handleCopyProjectFileList"
        @refresh="emit('refresh')"
        @notify="notify"
      />


      <!-- Draggable Pane Divider -->
      <div
        v-show="showSourcePane"
        @mousedown="startResizing"
        @dblclick="resetLeftPaneWidth"
        class="relative w-1 cursor-col-resize hover:bg-accent/60 active:bg-accent-strong transition-colors shrink-0 z-30"
        :class="{ 'bg-accent-strong': isResizing }"
        :title="t('ws.resize.source.title')"
      >
        <!-- Narrow workspace: collapse the tree to a 36px rail so the
             changes/diff pair reclaims the width. Never rendered at ≥1280px,
             where both panes always fit. -->
        <button
          v-if="canCollapseLeftPane"
          type="button"
          data-testid="ws-left-pane-collapse"
          @click="setLeftPaneCollapsed(true)"
          @mousedown.stop
          @dblclick.stop
          class="absolute top-2 -left-1.5 w-3.5 h-7 rounded-md border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer flex items-center justify-center shadow-xs"
          :aria-label="t('ws.pane.source.hide')"
          :title="t('ws.pane.source.hide')"
        >
          <PanelLeftClose class="w-3 h-3" />
        </button>
      </div>
    </div>

    <!-- Collapsed left pane: a 36px rail with the expand control. The tree
         itself is only display:none (v-show), so its expansion state survives
         a collapse/expand cycle; expanding restores the pane's stored width. -->
    <div
      v-if="showSourcePane && isLeftPaneCollapsed"
      data-testid="ws-left-pane-rail"
      class="w-9 shrink-0 border-r border-line bg-surface/30 backdrop-blur-sm flex flex-col items-center pt-2"
    >
      <button
        type="button"
        data-testid="ws-left-pane-expand"
        @click="setLeftPaneCollapsed(false)"
        class="p-1 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
        :aria-label="t('ws.pane.source.show')"
        :title="t('ws.pane.source.show')"
      >
        <PanelLeft class="w-4 h-4" />
      </button>
    </div>

    <!-- ======================================================== -->
    <!-- RIGHT PANE: 变更检查 & 差分工作台 (Changes & Diff - 变更放右边) -->
    <!-- ======================================================== -->
    <main class="flex-1 flex flex-col bg-canvas overflow-hidden min-w-0">
      <!-- Top Right Tab Switcher & Diff Mode Selector -->
      <div class="@container h-11 border-b border-line px-3 flex items-center justify-between gap-base bg-surface/40 backdrop-blur-sm shrink-0 overflow-hidden">
        <div class="flex items-center gap-base shrink-0">
        <!-- Dual Mode Navigation Tabs (Diff vs Source) -->
        <div class="flex items-center gap-1 bg-surface rounded-lg p-0.5 border border-line shrink-0">
          <button
            @click="activeRightTab = 'diff'"
            class="px-3 py-1 rounded-md text-xs font-medium transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeRightTab === 'diff' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <GitCommit class="w-3.5 h-3.5" />
            <span>{{ t("ws.tab.changes") }}</span>
            <span
              v-if="totalChangesCount > 0"
              class="px-1.5 py-0.2 rounded-full text-[9px] font-bold"
              :class="stagedFiles.length > 0 ? 'bg-success/20 text-success' : 'bg-warning/20 text-warning'"
            >
              {{ totalChangesCount }}
            </span>
          </button>

          <button
            @click="activeRightTab = 'source'"
            class="px-3 py-1 rounded-md text-xs font-medium transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeRightTab === 'source' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Code2 class="w-3.5 h-3.5" />
            <span>{{ t("ws.tab.source") }}</span>
          </button>
        </div>

        <!--
          Source pane toggle: the tree defaults to hidden; an explicit choice
          is persisted so the next launch reopens the same layout.
        -->
        <button
          type="button"
          data-testid="ws-source-pane-toggle"
          @click="toggleSourcePane"
          :aria-pressed="showSourcePane"
          :aria-label="showSourcePane ? t('ws.pane.source.hide') : t('ws.pane.source.show')"
          :title="showSourcePane ? t('ws.pane.source.hide') : t('ws.pane.source.show')"
          class="p-1 rounded-lg border transition cursor-pointer shrink-0"
          :class="showSourcePane
            ? 'bg-raised border-line text-accent shadow-xs'
            : 'bg-surface border-line text-ink-muted hover:text-ink hover:bg-raised'"
        >
          <PanelLeftClose v-if="showSourcePane" class="w-3.5 h-3.5" />
          <PanelLeft v-else class="w-3.5 h-3.5" />
        </button>
        </div>

        <!-- Mode-Specific Action Controls -->
        <!--
          Own scroll container: the tab strip on the left must never scroll out
          of reach behind the action cluster. The inner row is `w-max
          min-w-full`: a flex container with `justify-end` + `overflow-x-auto`
          overflows to the *left* once the content is wider than the box, so
          the first controls (「暂存 1」) were painted underneath the tab strip
          and unreachable at 1280-1440px. `w-max` makes the scrollable content
          start at the real first control.
        -->
        <div v-if="activeRightTab === 'diff'" class="flex-1 min-w-0 overflow-x-auto">
          <div class="flex w-max min-w-full items-center justify-end gap-2 whitespace-nowrap">
          <!-- Narrow windows: changes list and diff take turns instead of sharing -->
          <div
            v-if="isStackedPanes && !isZenMode"
            data-testid="ws-stacked-pane-switch"
            role="group"
            :aria-label="t('ws.pane.switch')"
            class="flex items-center gap-0.5 bg-surface rounded-lg p-0.5 border border-line shrink-0"
          >
            <button
              type="button"
              :aria-pressed="stackedPane === 'changes'"
              @click="stackedPane = 'changes'"
              class="px-2 py-1 rounded-md text-xs transition cursor-pointer whitespace-nowrap"
              :class="stackedPane === 'changes' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              {{ t("ws.pane.changes") }}
            </button>
            <button
              type="button"
              :aria-pressed="stackedPane === 'diff'"
              @click="stackedPane = 'diff'"
              class="px-2 py-1 rounded-md text-xs transition cursor-pointer whitespace-nowrap"
              :class="stackedPane === 'diff' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              {{ t("ws.pane.diff") }}
            </button>
          </div>
          <!-- Staged / Unstaged Summary Counters (hidden on a narrow pane to keep the bar readable) -->
          <div class="hidden @min-[700px]:flex items-center gap-2 text-xs">
            <span class="flex items-center gap-1 font-semibold text-success">
              <CheckCheck class="w-3.5 h-3.5" /> {{ t("ws.stat.staged", { count: stagedFiles.length }) }}
            </span>
            <span aria-hidden="true" class="w-px h-3 bg-line shrink-0"></span>
            <span class="flex items-center gap-1 font-semibold text-warning">
              <Plus class="w-3.5 h-3.5" /> {{ t("ws.stat.unstaged", { count: unstagedFiles.length }) }}
            </span>
          </div>

          <!-- Layout presets -->
          <div class="hidden @min-[940px]:flex items-center gap-0.5 bg-surface rounded-lg p-0.5 border border-line shrink-0" role="group" :aria-label="t('ws.layout.group')">
            <button :aria-label="t('ws.layout.standard.title')"
              @click="applyWorkspaceLayout('standard')"
              class="p-1 rounded transition cursor-pointer"
              :class="activeLayoutPreset === 'standard' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
              :title="t('ws.layout.standard.title')"
            >
              <Columns2 class="w-3.5 h-3.5" />
            </button>
            <button :aria-label="t('ws.layout.focus.source.title')"
              @click="applyWorkspaceLayout('focus-source')"
              class="p-1 rounded transition cursor-pointer"
              :class="activeLayoutPreset === 'focus-source' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
              :title="t('ws.layout.focus.source.title')"
            >
              <PanelLeft class="w-3.5 h-3.5" />
            </button>
            <button :aria-label="t('ws.layout.focus.diff.title')"
              @click="applyWorkspaceLayout('focus-diff')"
              class="p-1 rounded transition cursor-pointer"
              :class="activeLayoutPreset === 'focus-diff' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
              :title="t('ws.layout.focus.diff.title')"
            >
              <PanelRight class="w-3.5 h-3.5" />
            </button>
          </div>
          <button
            ref="layoutMenuButtonRef"
            @click="toggleLayoutMenu"
            class="hidden @min-[940px]:inline-flex p-1 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0 relative"
            :title="t('ws.layout.custom.title', { count: layoutPresets.length })"
          >
            <Bookmark class="w-3.5 h-3.5" />
            <span
              v-if="layoutPresets.length > 0"
              class="absolute -top-1 -right-1 min-w-[12px] h-3 px-0.5 rounded-full bg-accent-strong text-accent-contrast text-[8px] font-bold flex items-center justify-center"
            >
              {{ layoutPresets.length }}
            </span>
          </button>

          <!-- Unified vs Split Toggle -->
          <div class="flex items-center gap-0.5 bg-surface rounded-lg p-0.5 border border-line">
            <button
              @click="setDiffMode('unified')"
              class="px-2.5 py-1 rounded-md text-xs transition cursor-pointer flex items-center gap-1 shrink-0 whitespace-nowrap"
              :class="activeDiffMode === 'unified' ? 'bg-raised text-ink font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
              :title="t('ws.diff.mode.unified.title')"
            >
              <List class="w-3 h-3" /> <span class="hidden xl:inline">Unified</span>
            </button>
            <button
              @click="setDiffMode('split')"
              class="px-2.5 py-1 rounded-md text-xs transition cursor-pointer flex items-center gap-1 shrink-0 whitespace-nowrap"
              :class="activeDiffMode === 'split' ? 'bg-raised text-ink font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
              :title="t('ws.diff.mode.split.title')"
            >
              <Columns class="w-3 h-3" /> <span class="hidden xl:inline">Split</span>
            </button>
            <button
              @click="setDiffMode('word')"
              :disabled="!canUseWordDiff"
              class="px-2.5 py-1 rounded-md text-xs transition cursor-pointer flex items-center gap-1 shrink-0 whitespace-nowrap disabled:opacity-40 disabled:cursor-not-allowed"
              :class="activeDiffMode === 'word' ? 'bg-raised text-ink font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
              :title="t('ws.diff.word.level.title')"
            >
              <Pilcrow class="w-3 h-3" /> <span class="hidden xl:inline">{{ t("ws.diff.word.level") }}</span>
            </button>
          </div>

          <!-- 忽略空白差异 (-w) Toggle -->
          <button
            @click="toggleIgnoreWhitespace"
            class="px-2.5 py-1 rounded-lg border text-xs transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="ignoreWhitespace ? 'bg-accent/20 border-accent/40 text-accent font-semibold shadow-xs' : 'bg-surface border-line text-ink-muted hover:text-ink hover:bg-raised'"
            :title="t('ws.diff.ignore.whitespace.title')"
          >
            <Space class="w-3.5 h-3.5" />
            <!-- English's "Ignore Whitespace (-w)" is ~35px longer than the
                 Chinese label, so the full text appears only when the bar can
                 actually hold it; otherwise the compact "-w" (same title) is
                 used instead of scrolling the cluster off-screen. -->
            <span class="hidden @min-[1080px]:inline">{{ t("ws.diff.ignore.whitespace") }}</span>
            <span class="@min-[1080px]:hidden">-w</span>
          </button>
          </div>
        </div>

        <div v-else-if="activeRightTab === 'source' && sourceFileDetail" class="flex-1 min-w-0 overflow-x-auto">
          <div class="flex w-max min-w-full items-center justify-end gap-2 whitespace-nowrap">
          <!-- When viewing Image -->
          <template v-if="sourceFileDetail.isImage && svgViewMode === 'image'">
            <!-- Zoom controls -->
            <div class="flex items-center bg-canvas p-0.5 rounded-lg border border-line text-xs font-mono shrink-0">
              <button :aria-label="t('ws.zoom.out')"
                @click="zoomOut"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
                :title="t('ws.zoom.out')"
              >
                <ZoomOut class="w-3.5 h-3.5" />
              </button>
              <button
                @click="resetZoom"
                class="px-2 py-0.5 rounded hover:bg-raised text-ink transition cursor-pointer text-[11px]"
                :title="t('ws.zoom.reset')"
              >
                {{ imageZoom }}%
              </button>
              <button :aria-label="t('ws.zoom.in')"
                @click="zoomIn"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
                :title="t('ws.zoom.in')"
              >
                <ZoomIn class="w-3.5 h-3.5" />
              </button>
            </div>

            <!-- Background mode switcher -->
            <div class="flex items-center bg-canvas p-0.5 rounded-lg border border-line text-xs shrink-0">
              <button
                @click="imageBgMode = 'checkered'"
                class="px-2 py-0.5 rounded transition cursor-pointer text-[10px] font-medium"
                :class="imageBgMode === 'checkered' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                :title="t('ws.image.bg.checker')"
              >
                {{ t("ws.image.bg.checker") }}
              </button>
              <button
                @click="imageBgMode = 'dark'"
                class="px-2 py-0.5 rounded transition cursor-pointer text-[10px] font-medium"
                :class="imageBgMode === 'dark' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                :title="t('ws.image.bg.dark')"
              >
                {{ t("ws.image.bg.dark") }}
              </button>
              <button
                @click="imageBgMode = 'white'"
                class="px-2 py-0.5 rounded transition cursor-pointer text-[10px] font-medium"
                :class="imageBgMode === 'white' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                :title="t('ws.image.bg.light')"
              >
                {{ t("ws.image.bg.light") }}
              </button>
            </div>

            <!-- Pixelated toggle for pixel art -->
            <button
              @click="imagePixelated = !imagePixelated"
              class="px-2 py-1 rounded-md border border-line text-xs transition cursor-pointer font-medium shrink-0"
              :class="imagePixelated ? 'bg-accent/20 text-accent border-accent/40' : 'bg-surface hover:bg-raised text-ink-muted hover:text-ink'"
              :title="t('ws.image.pixel.mode.title')"
            >
              {{ t("ws.image.pixel.mode") }}
            </button>
          </template>

          <!-- SVG toggle button -->
          <div v-if="sourceFileDetail.language === 'SVG'" class="flex items-center bg-canvas p-0.5 rounded-lg border border-line text-xs shrink-0">
            <button
              @click="svgViewMode = 'image'"
              class="px-2 py-0.5 rounded transition cursor-pointer text-[10px]"
              :class="svgViewMode === 'image' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              {{ t("ws.view.image") }}
            </button>
            <button
              @click="svgViewMode = 'code'"
              class="px-2 py-0.5 rounded transition cursor-pointer text-[10px]"
              :class="svgViewMode === 'code' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
            >
              {{ t("ws.view.source") }}
            </button>
          </div>

          <!-- Blame toggle (when viewing code) -->
          <button
            v-if="!sourceFileDetail.isImage || svgViewMode === 'code'"
            @click="toggleBlame"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md border border-line text-xs font-medium transition cursor-pointer shrink-0"
            :class="showBlame ? 'bg-accent/20 text-accent border-accent/40 shadow-xs' : 'bg-surface hover:bg-raised text-ink-muted hover:text-ink'"
            :title="t('ws.blame.title')"
          >
            <GitCommit class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingBlame }" />
            <span>{{ showBlame ? t("ws.blame.hide") : t("ws.blame.show") }}</span>
          </button>

          <!-- File History toggle (when viewing file) -->
          <button
            @click="toggleFileHistory"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md border border-line text-xs font-medium transition cursor-pointer shrink-0"
            :class="showFileHistory ? 'bg-purple/20 text-purple border-purple/40 shadow-xs' : 'bg-surface hover:bg-raised text-ink-muted hover:text-ink'"
            :title="t('ws.file.history.title')"
          >
            <History class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingFileHistory }" />
            <span>{{ showFileHistory ? t("ws.file.history.hide") : t("ws.file.history.show") }}</span>
          </button>

          <!-- Wrap text toggle (when viewing code) -->
          <button
            v-if="!sourceFileDetail.isImage || svgViewMode === 'code'"
            @click="isCodeWrap = !isCodeWrap"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md border border-line text-xs font-medium transition cursor-pointer shrink-0"
            :class="isCodeWrap ? 'bg-accent/20 text-accent border-accent/40' : 'bg-surface hover:bg-raised text-ink-muted hover:text-ink'"
            :title="t('ws.wrap.title')"
          >
            <WrapText class="w-3.5 h-3.5" />
            <span>{{ t("ws.wrap") }}</span>
          </button>

          <!-- Copy Code button (when viewing code) -->
          <button
            v-if="!sourceFileDetail.isImage || svgViewMode === 'code'"
            @click="copySourceCode"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer shrink-0"
          >
            <Check v-if="copiedCodeToast" class="w-3.5 h-3.5 text-success" />
            <Copy v-else class="w-3.5 h-3.5 text-ink-muted" />
            <span>{{ copiedCodeToast ? t("ws.copy.done") : t("ws.copy") }}</span>
          </button>
          </div>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- TAB CONTENT A: 变更比对与智能提交 (Diff View) -->
      <!-- ============================================== -->
      <div v-if="activeRightTab === 'diff'" class="flex-1 flex flex-col overflow-hidden">
        <!-- Conflict & Lifecycle Emergency Operations Banner -->
        <ConflictBanner
          v-if="hasConflict"
          :is-merging="isMerging"
          :is-rebasing="isRebasing"
          :is-cherry-picking="isCherryPicking"
          :conflicted-files="conflictedFiles"
          :merge-head-branch="props.repoStatus?.mergeHeadBranch || ''"
          :rebase-head-name="props.repoStatus?.rebaseHeadName || ''"
          :rebase-step="props.repoStatus?.rebaseStep || 0"
          :rebase-total="props.repoStatus?.rebaseTotal || 0"
          @continue="handleContinueConflict"
          @skip="handleSkipConflict"
          @abort="handleAbortConflict(true)"
        />

        <!-- Main Body: Two Columns (Left: 变更文件列表 & 提交, Right: 差异比对主视口) -->
        <div class="flex-1 flex overflow-hidden">
          <!-- 1. 变更文件列表与提交面板 (Changed Files List Panel) -->
          <!-- Stacked (narrow) layout keeps both panes mounted and hides one:
               unmounting the list would drop its groups/filters, and the diff's
               scroll position. -->
          <div
            v-if="!isZenMode"
            class="border-r border-line bg-surface/30 flex flex-col overflow-hidden shrink-0 @container"
            :class="{
              'transition-[width] duration-200 ease-out': !isResizingChanges && !isStackedPanes,
              hidden: isStackedPanes && stackedPane === 'diff',
              'flex-1 min-w-0 border-r-0': isStackedPanes,
            }"
            :style="isStackedPanes ? undefined : { width: `${effectiveChangesPaneWidth}px` }"
          >
            <div
              v-if="workspaceStats"
              class="px-3 py-1.5 border-b border-line bg-surface/40 flex items-center gap-base text-[10px] font-mono shrink-0 overflow-x-auto"
              :title="workspaceStats.summary"
            >
              <span
                v-if="isVimModeEnabled"
                class="px-1.5 py-0.5 rounded text-[9px] font-mono font-bold tracking-wider shrink-0 select-none"
                :class="currentVimMode === 'NORMAL' ? 'bg-accent/20 text-accent border border-accent/30' : 'bg-warning/20 text-warning border border-warning/30'"
              >
                -- {{ currentVimMode }} --
              </span>
              <span class="text-accent shrink-0">{{ t("ws.stat.staged", { count: workspaceStats.stagedFiles }) }}</span>
              <span class="text-warning shrink-0">{{ t("ws.stat.unstaged", { count: workspaceStats.unstagedFiles }) }}</span>
              <span v-if="workspaceStats.untrackedFiles" class="text-ink-muted shrink-0">{{ t("ws.stat.untracked", { count: workspaceStats.untrackedFiles }) }}</span>
              <span v-if="workspaceStats.conflictedFiles" class="text-danger shrink-0">{{ t("ws.stat.conflicted", { count: workspaceStats.conflictedFiles }) }}</span>
              <FlightRecorderHud class="shrink-0" :repo-path="currentRepoPath" @restored="emit('refresh')" />
              <span class="ml-auto text-success shrink-0 whitespace-nowrap">+{{ workspaceStats.totalAdditions }}</span>
              <span class="text-danger shrink-0 whitespace-nowrap">-{{ workspaceStats.totalDeletions }}</span>
            </div>

            <ChangesList
              :staged-files="stagedFiles"
              :unstaged-files="unstagedFiles"
              :conflicted-files="conflictedFiles"
              :selected-path="selectedChangeFile?.path ?? ''"
              :current-repo-path="currentRepoPath"
              @select-file="selectChangeFile"
              @stage="handleStageFile"
              @unstage="handleUnstageFile"
              @discard="handleDiscardFile"
              @toggle-lfs-lock="handleToggleLfsLock"
              @stage-all="handleStageAll"
              @unstage-all="handleUnstageAll"
              @discard-all="handleDiscardAll"
              @auto-resolve-meta="handleAutoResolveMetaConflicts"
              @refresh="emit('refresh')"
              @notify="notify"
            />

            <!-- Non-LFS Staged Large Files Alert -->
            <div
              v-if="stagedNonLfsLargeFiles.length > 0"
              class="mx-2 my-1.5 p-2 rounded-xl border border-warning/40 bg-warning/10 flex items-center justify-between gap-base text-xs select-none"
            >
              <div class="flex items-center gap-1.5 min-w-0">
                <AlertTriangle class="w-3.5 h-3.5 text-warning shrink-0" />
                <span class="text-ink font-semibold truncate text-[11px]">{{
                  t("ws.staged.large.warning", { count: stagedNonLfsLargeFiles.length })
                }}</span>
              </div>
              <button
                @click="handleConvertStagedNonLfs"
                :disabled="isConvertingLfs"
                class="px-2 py-0.5 rounded-md bg-warning-strong text-white hover:bg-warning-strong-hover font-semibold text-[10px] shrink-0 cursor-pointer transition shadow-xs disabled:opacity-50"
              >
                {{ isConvertingLfs ? t("common.loading") : t("ws.staged.large.convert") }}
              </button>
            </div>

            <!-- The CommitComposer moved out of this pane into the pinned
                 commit bar at the bottom of the right column (roadmap §2.1
                 item 1): docking it here meant stacked panes (narrow
                 windows) and any long list squeezed the commit path out of
                 the viewport. -->

          </div>

          <!-- Draggable Divider between Changes List and Diff Inspector -->
          <div
            v-if="!isZenMode && !isStackedPanes"
            @mousedown="startResizingChanges"
            @dblclick="resetChangesPaneWidth"
            class="w-1 cursor-col-resize hover:bg-accent/60 active:bg-accent-strong transition-colors shrink-0 z-20"
            :class="{ 'bg-accent-strong': isResizingChanges }"
            :title="t('ws.resize.changes.title')"
          ></div>

          <!-- 2. 差异比对主区 (Diff Canvas Inspector) -->
          <div
            class="flex-1 flex flex-col overflow-hidden bg-canvas min-w-0"
            :class="{ hidden: isStackedPanes && stackedPane === 'changes' }"
          >

        <!-- Diff Canvas -->
        <div ref="diffScrollRef" class="flex-1 overflow-auto bg-canvas relative font-mono text-[12px] leading-5 select-text">
          <div v-if="isLoadingDiff && !currentDiffText" class="flex items-center justify-center h-full text-ink-muted">
            <RefreshCw class="w-5 h-5 animate-spin mr-2" /> {{ t("ws.diff.loading") }}
          </div>

          <!-- Fresh Repository with 0 commits and 0 changes -->
          <div
            v-else-if="!selectedChangeFile && totalChangesCount === 0 && isNewRepo"
            class="flex flex-col items-center justify-center h-full text-center p-8 select-none max-w-lg mx-auto"
          >
            <div class="w-14 h-14 rounded-2xl bg-accent/10 border border-accent/30 flex items-center justify-center text-accent mb-4 shadow-sm">
              <Sparkles class="w-7 h-7" />
            </div>
            <h3 class="text-base font-bold text-ink mb-1.5">{{ t("ws.empty.new.repo.title") }}</h3>
            <p class="text-xs text-ink-muted mb-6 leading-relaxed max-w-md">{{ t("ws.empty.new.repo.desc") }}</p>

            <div class="w-full bg-surface/80 border border-line rounded-xl p-4 text-left space-y-3 font-sans shadow-xs">
              <div class="flex items-start gap-loose">
                <span class="w-5 h-5 rounded-full bg-accent/15 text-accent text-xs font-bold flex items-center justify-center shrink-0 mt-0.5">1</span>
                <div class="text-xs">
                  <p class="text-ink font-medium">{{ t("ws.empty.new.repo.step1") }}</p>
                </div>
              </div>
              <div class="flex items-start gap-loose">
                <span class="w-5 h-5 rounded-full bg-accent/15 text-accent text-xs font-bold flex items-center justify-center shrink-0 mt-0.5">2</span>
                <div class="text-xs">
                  <p class="text-ink font-medium">{{ t("ws.empty.new.repo.step2") }}</p>
                </div>
              </div>
              <div class="flex items-start gap-loose">
                <span class="w-5 h-5 rounded-full bg-accent/15 text-accent text-xs font-bold flex items-center justify-center shrink-0 mt-0.5">3</span>
                <div class="text-xs">
                  <p class="text-ink font-medium">{{ t("ws.empty.new.repo.step3") }}</p>
                </div>
              </div>
            </div>
          </div>

          <div
            v-else-if="!selectedChangeFile && totalChangesCount === 0"
            class="flex flex-col items-center justify-center h-full text-center p-8 select-none"
          >
            <div class="w-12 h-12 rounded-2xl bg-surface border border-line flex items-center justify-center text-success mb-3 shadow-sm">
              <Check class="w-6 h-6" />
            </div>
            <h3 class="text-sm font-semibold text-ink mb-1">{{ t("ws.empty.clean.title") }}</h3>
            <p class="text-xs text-ink-muted max-w-sm">{{ t("ws.empty.clean.desc") }}</p>
          </div>

          <div
            v-else-if="!selectedChangeFile && totalChangesCount > 0"
            class="flex flex-col items-center justify-center h-full text-center p-8 select-none"
          >
            <div class="w-12 h-12 rounded-2xl bg-surface border border-line flex items-center justify-center text-accent mb-3 shadow-sm">
              <GitCommit class="w-6 h-6" />
            </div>
            <h3 class="text-sm font-semibold text-ink mb-1">{{ t("ws.empty.changes.title", { count: totalChangesCount }) }}</h3>
            <p class="text-xs text-ink-muted max-w-sm">{{ t("ws.empty.changes.desc") }}</p>
          </div>

          <!-- Diff Table -->
          <div v-else-if="selectedChangeFile" class="min-w-full inline-block pb-32">
            <!-- Diff Top Action Bar -->
            <div class="sticky top-0 z-10 px-4 py-2 bg-surface/90 backdrop-blur-md border-b border-line flex flex-wrap items-center justify-between gap-x-base gap-y-tight select-none">
              <div class="flex items-center gap-base min-w-0">
                <span class="font-bold text-ink truncate" :title="selectedChangeFile?.path">{{ selectedChangeFile?.path }}</span>
                <span
                  v-if="isConflictedSelected"
                  class="px-2 py-0.5 rounded bg-danger/20 text-danger text-[10px] font-bold border border-danger/30"
                >
                  {{ t("ws.group.conflicted") }}
                </span>
                <span
                  v-else-if="selectedChangeFile?.isStaged"
                  class="px-1.5 py-0.5 rounded bg-success/20 text-success text-[10px] font-semibold"
                >
                  {{ t("ws.group.staged") }}
                </span>
                <span
                  v-else
                  class="px-1.5 py-0.5 rounded bg-warning/20 text-warning text-[10px] font-semibold"
                >
                  {{ t("ws.group.unstaged") }}
                </span>

                <!-- 差异统计: 行数 / +新增 / -删除 -->
                <span
                  v-if="diffTotalLineCount > 1 && !isConflictedSelected && activeDiffMode !== 'word'"
                  class="px-1.5 py-0.5 rounded bg-surface border border-line text-[10px] font-mono font-normal text-ink-muted flex items-center gap-1"
                  :title="t('ws.diff.stats.title')"
                >
                  <span>{{ t("ws.diff.stats.lines", { count: diffTotalLineCount }) }}</span>
                  <span class="text-success font-semibold">+{{ diffStats.additions }}</span>
                  <span class="text-danger font-semibold">-{{ diffStats.deletions }}</span>
                </span>
                <RefreshCw
                  v-if="isLoadingDiff"
                  class="w-3 h-3 animate-spin text-accent shrink-0"
                  :title="t('ws.diff.loading')"
                />
              </div>

              <div class="flex flex-wrap items-center gap-base shrink-0">
                <FeatureGate flag="aiSummary">
                  <button
                    @click="openAiRepoChat"
                    class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2.5 py-1 rounded-md bg-raised hover:bg-hover text-ink text-xs font-semibold cursor-pointer transition border border-line"
                    :title="t('ai.chat.desc')"
                    data-testid="repo-chat-btn"
                  >
                    <MessageSquare class="w-3 h-3 text-accent" />
                    <span>{{ t("ai.chat.scope.repo") }}</span>
                  </button>
                </FeatureGate>
                <button
                  v-if="currentRepoPath"
                  @click="openAiRepoOverride"
                  class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2.5 py-1 rounded-md bg-raised hover:bg-hover text-ink text-xs font-semibold cursor-pointer transition border border-line"
                  :title="t('ai.repo.desc')"
                  data-testid="repo-ai-btn"
                >
                  <Sparkles class="w-3 h-3 text-accent" />
                  <span>{{ t("ai.repo.btn") }}</span>
                </button>
                <button
                  v-if="canCompareAsset"
                  @click="handleOpenAssetDiff"
                  class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2.5 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold cursor-pointer transition"
                  :title="t('ws.asset.compare.title')"
                >
                  <ScanEye class="w-3 h-3" />
                  <span>{{ t("ws.asset.compare") }}</span>
                </button>
                <button
                  v-if="selectedChangeFile && !isConflictedSelected"
                  @click="handleOpenExternalDiff(selectedChangeFile.path, selectedChangeFile.isStaged)"
                  class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2.5 py-1 rounded-md bg-raised hover:bg-hover text-ink text-xs font-semibold cursor-pointer transition border border-line"
                  :title="t('ws.external.compare.title')"
                >
                  <ExternalLink class="w-3 h-3 text-accent" />
                  <span>{{ t("ws.external.compare") }}</span>
                </button>
                <button
                  v-if="selectedChangeFile && !selectedChangeFile.isStaged && !isConflictedSelected"
                  @click="handleStageFile(selectedChangeFile.path)"
                  class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2.5 py-1 rounded-md bg-success/20 hover:bg-success/30 text-success text-xs font-semibold cursor-pointer transition"
                >
                  <Plus class="w-3 h-3" /> {{ t("ws.action.stage.file") }}
                </button>
                <button
                  v-if="selectedChangeFile && selectedChangeFile.isStaged && !isConflictedSelected"
                  @click="handleUnstageFile(selectedChangeFile.path)"
                  class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2.5 py-1 rounded-md bg-raised hover:bg-hover text-ink text-xs font-semibold cursor-pointer transition"
                >
                  <Minus class="w-3 h-3" /> {{ t("ws.action.unstage.file") }}
                </button>
                <button :aria-label="t('ws.action.discard.file')"
                  v-if="selectedChangeFile && !selectedChangeFile.isStaged && !isConflictedSelected"
                  @click="handleDiscardFile(selectedChangeFile.path)"
                  class="flex items-center gap-1 px-2 py-1 rounded-md hover:bg-danger/20 text-danger text-xs transition cursor-pointer"
                  :title="t('ws.action.discard.file')"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
                <button
                  v-if="selectedChangeFile && !isConflictedSelected"
                  @click="handlePartialStashCurrentFile"
                  class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2.5 py-1 rounded-md bg-raised hover:bg-hover text-ink text-xs font-semibold cursor-pointer transition border border-line"
                  :title="t('ws.action.partial.stash.hint')"
                >
                  <Archive class="w-3 h-3 text-accent" />
                  <span>{{ t("ws.action.partial.stash") }}</span>
                </button>
              </div>
            </div>

            <!-- Conflict Resolution Action Bar -->
            <div
              v-if="isConflictedSelected && selectedChangeFile"
              class="sticky top-10 z-10 px-4 py-2 bg-surface/95 backdrop-blur-md border-b border-line flex flex-wrap items-center justify-between gap-base select-none shadow-sm"
            >
              <div class="hidden xl:flex items-center gap-1.5 text-xs text-ink-secondary shrink-0">
                <ShieldAlert class="w-4 h-4 text-warning" />
                <span class="font-semibold text-ink">{{ t("ws.conflict.helper") }}</span>
              </div>

              <div class="flex flex-wrap items-center gap-1.5">
                <button
                  @click="handleResolveConflict(selectedChangeFile.path, 'ours')"
                  class="shrink-0 whitespace-nowrap px-2.5 py-1 rounded-lg bg-accent/20 hover:bg-accent/30 text-accent text-xs font-semibold cursor-pointer transition border border-accent/30"
                  :title="t('ws.conflict.ours.title')"
                >
                  {{ t("ws.conflict.ours.action") }}
                </button>
                <button
                  @click="handleResolveConflict(selectedChangeFile.path, 'theirs')"
                  class="shrink-0 whitespace-nowrap px-2.5 py-1 rounded-lg bg-purple/20 hover:bg-purple/30 text-purple text-xs font-semibold cursor-pointer transition border border-purple/30"
                  :title="t('ws.conflict.theirs.title')"
                >
                  {{ t("ws.conflict.theirs.action") }}
                </button>
                <button
                  @click="handleResolveConflict(selectedChangeFile.path, 'backup_and_theirs')"
                  class="shrink-0 whitespace-nowrap px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-semibold cursor-pointer transition border border-line"
                  :title="t('ws.conflict.backup.title')"
                >
                  {{ t("ws.conflict.backup.action") }}
                </button>
                <button
                  @click="handleResolveConflict(selectedChangeFile.path, 'mark_resolved')"
                  class="shrink-0 whitespace-nowrap px-2.5 py-1 rounded-lg bg-success/20 hover:bg-success/30 text-success text-xs font-semibold cursor-pointer transition border border-success/30"
                  :title="t('ws.conflict.mark.title')"
                >
                  {{ t("ws.conflict.mark.action") }}
                </button>
                <button
                  @click="handleOpenExternalMerge(selectedChangeFile.path)"
                  class="shrink-0 whitespace-nowrap px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-semibold cursor-pointer transition border border-line flex items-center gap-1"
                  :title="t('ws.conflict.external.title')"
                >
                  <ExternalLink class="w-3 h-3 text-accent" />
                  <span>{{ t("ws.conflict.external") }}</span>
                </button>
                <button
                  v-if="isConflictedSelected"
                  type="button"
                  @click="openThreeWayMerge(selectedChangeFile?.path)"
                  class="shrink-0 whitespace-nowrap px-2.5 py-1 rounded-lg bg-accent text-accent-contrast text-xs font-semibold cursor-pointer transition shadow-xs flex items-center gap-1.5"
                  :title="t('merge.3way.desc')"
                  data-testid="open-3way-merge-btn"
                >
                  <GitMerge class="w-3.5 h-3.5" />
                  <span>{{ t("merge.3way.btn") }}</span>
                </button>
                <button
                  v-if="hasMetaConflicts"
                  @click="handleAutoResolveMetaConflicts('theirs')"
                  class="shrink-0 whitespace-nowrap px-2.5 py-1 rounded-lg bg-purple/15 hover:bg-purple/25 text-purple text-xs font-semibold cursor-pointer transition border border-purple/30 flex items-center gap-1"
                  :title="t('ws.conflict.auto.meta.desc')"
                >
                  <Sparkles class="w-3 h-3 text-purple" />
                  <span>{{ t("ws.conflict.auto.meta") }} ({{ metaConflictedFiles.length }})</span>
                </button>
                <button
                  v-if="hasSemanticAssetConflicts"
                  @click="openSemanticConflictModal()"
                  class="shrink-0 whitespace-nowrap px-2.5 py-1 rounded-lg bg-accent/15 hover:bg-accent/25 text-accent text-xs font-semibold cursor-pointer transition border border-accent/30 flex items-center gap-1"
                  :title="t('conflict.semantic.desc')"
                  data-testid="open-semantic-conflict-btn"
                >
                  <Sparkles class="w-3 h-3 text-accent" />
                  <span>{{ t("conflict.semantic.inspect") }} ({{ semanticConflictedFiles.length }})</span>
                </button>
              </div>
            </div>

      <!-- File Hierarchy Breadcrumb Navigation Bar -->
      <div
        v-if="selectedChangeFile?.path"
        class="mx-2 mb-1 px-3 py-1.5 rounded-lg border border-line/60 bg-surface/40 flex items-center gap-1.5 text-xs font-mono text-ink-muted select-none overflow-x-auto shrink-0"
      >
        <FileText class="w-3.5 h-3.5 text-accent shrink-0" />
        <template v-for="(seg, sIdx) in selectedChangeFile.path.split('/')" :key="sIdx">
          <span v-if="sIdx > 0" class="text-ink-muted select-none">/</span>
          <span
            :class="sIdx === selectedChangeFile.path.split('/').length - 1 ? 'text-ink font-bold' : 'text-ink-muted hover:text-ink transition'"
          >
            {{ seg }}
          </span>
        </template>
      </div>

      <DiffViewer
        :diff-text="currentDiffText"
        :mode="activeDiffMode"
        :repo-path="currentRepoPath"
        :file-path="selectedChangeFile?.path"
        :can-hunk-action="activeDiffMode !== 'word' && !!selectedChangeFile && !isConflictedSelected"
        :can-line-action="activeDiffMode === 'unified' && !!selectedChangeFile && !selectedChangeFile.isStaged && !isConflictedSelected"
        :is-staged="selectedChangeFile?.isStaged"
        :can-compare-asset="canCompareAsset"
        :hunk-action-busy="isHunkActionBusy"
        :is-zen-mode="isZenMode"
        @stage-hunk="handleStageHunk"
        @unstage-hunk="handleUnstageHunk"
        @discard-hunk="handleDiscardHunk"
        @stage-lines="handleStageLines"
        @discard-lines="handleDiscardLines"
        @resolve-conflict-block="handleResolveConflictBlock"
        @open-ai-advisor="handleOpenAiConflictAdvisor"
        @explain-diff="openAiExplainDiff"
        @open-ai-chat="openAiChat"
        @open-ai-review="openAiReview"
        @open-ai-batch="openAiConflictBatch"
        @open-ai-batch-all="openAiConflictBatchAll"
        @open-asset-diff="handleOpenAssetDiff"
        @open-in-editor="handleOpenDiffFileInEditor"
        @toggle-zen="isZenMode = !isZenMode"
      />

          </div>
        </div>
      </div>
    </div>
    </div>

      <!-- ============================================== -->
      <!-- TAB CONTENT B: 源码全景浏览 -->
      <SourcePanorama
        v-else-if="activeRightTab === 'source'"
        :source-file-detail="sourceFileDetail"
        :highlighted-source-lines="highlightedSourceLines"
        :highlighted-source-line="highlightedSourceLine"
        :show-blame="showBlame"
        :blame-lines="blameLines"
        :show-file-history="showFileHistory"
        :file-history-list="fileHistoryList"
        :selected-file-revision-hash="selectedFileRevisionHash"
        :is-loading-file-history="isLoadingFileHistory"
        :image-zoom="imageZoom"
        :image-bg-mode="imageBgMode"
        :image-pixelated="imagePixelated"
        :svg-view-mode="svgViewMode"
        :image-width="imageWidth"
        :image-height="imageHeight"
        :is-code-wrap="isCodeWrap"
        :time-machine-active="timeMachineActive"
        :time-machine-current="timeMachineCurrent"
        :can-time-machine-back="canTimeMachineBack"
        :can-time-machine-forward="canTimeMachineForward"
        @toggle-blame="toggleBlame"
        @open-web="handleOpenFileInWeb"
        @reveal-file="handleRevealSourceFile"
        @open-default="handleOpenSourceFileDefault"
        @copy-hash="copyCommitHash"
        @update:show-file-history="showFileHistory = $event"
        @view-revision="viewFileRevision"
        @diff-revision="showRevisionDiffFor"
        @copy-image-base64="handleCopyImageBase64"
        @copy-revision-content="handleCopyRevisionContent"
        @restore-revision="restoreCurrentFileRevision"
        @revert-revision="revertSelectedFileToRevision"
        @blame-prior="handleBlamePrior"
        @inspect-history="handleInspectInHistory"
        @time-machine-back="handleTimeMachineBack"
        @time-machine-forward="handleTimeMachineForward"
        @time-machine-exit="handleExitTimeMachine"
        @update:image-bg-mode="imageBgMode = $event"
        @update:image-pixelated="imagePixelated = $event"
        @update:svg-view-mode="svgViewMode = $event"
        @zoom-in="zoomIn"
        @zoom-out="zoomOut"
        @reset-zoom="resetZoom"
        @image-loaded="handleImageLoaded"
      />

      <!-- ======================================================== -->
      <!-- Pinned Commit Bar (roadmap §2.1 item 1)                  -->
      <!-- ======================================================== -->
      <!-- Second segment of the right column: the stage → describe →
           commit path stays visible across scrolling, stacked panes and
           both right tabs (diff/source). Collapses to one summary line
           (staged count + subject preview); the choice persists in
           localStorage. The composer stays mounted via v-show so an
           in-flight AI stream survives a collapse/expand cycle. Zen mode
           keeps hiding it: it is an explicit immersive reading state. -->
      <div
        v-if="!isZenMode"
        data-testid="composer-pin-bar"
        class="sticky bottom-0 z-20 shrink-0 bg-surface"
      >
        <button
          type="button"
          data-testid="composer-pin-toggle"
          @click="toggleComposerCollapsed"
          class="w-full border-t border-line px-3 py-1.5 flex items-center gap-base text-xs select-none hover:bg-raised transition cursor-pointer text-left"
          :aria-expanded="!composerCollapsed"
          aria-controls="wgit-composer-pin-panel"
          :title="composerCollapsed ? undefined : t('workspace.diff.fold')"
        >
          <GitCommit class="w-3.5 h-3.5 text-accent shrink-0" />
          <span class="font-semibold text-ink shrink-0 whitespace-nowrap">{{ t("workspace.composer.title") }}</span>
          <span
            class="shrink-0 whitespace-nowrap font-semibold"
            :class="stagedFiles.length > 0 ? 'text-success' : 'text-ink-muted'"
          >{{ t("ws.stat.staged", { count: stagedFiles.length }) }}</span>
          <span
            v-if="composerCollapsed"
            data-testid="composer-pin-subject-preview"
            class="flex-1 min-w-0 truncate font-mono text-ink-muted"
          >{{ commitSubject.trim() || t("workspace.composer.subject.placeholder") }}</span>
          <span v-else class="flex-1" aria-hidden="true"></span>
          <ChevronUp v-if="composerCollapsed" class="w-3.5 h-3.5 shrink-0 text-ink-muted" />
          <ChevronDown v-else class="w-3.5 h-3.5 shrink-0 text-ink-muted" />
        </button>
        <div v-show="!composerCollapsed" id="wgit-composer-pin-panel">
          <CommitComposer
            ref="commitComposerRef"
            v-model:subject="commitSubject"
            v-model:body="commitBody"
            v-model:amend="isAmend"
            v-model:autoPush="autoPush"
            v-model:noVerify="isNoVerify"
            :committing="isCommitting"
            :is-new-repo="isNewRepo"
            :repo-path="currentRepoPath"
            @commit="handleCommit"
            @fast-commit="handleFastCommit"
            @generate="generateSmartCommitMessage"
            @audit-sentinel="handleAuditSentinel"
            @load-template="handleLoadCommitTemplate"
            @load-last="handleLoadLastCommitMessage"
            @save-template="handleSaveCommitTemplate"
            @notify="notify"
          />
        </div>
      </div>
    </main>

    <!-- Revision diff modal -->
    <UiModal
      :open="showRevisionDiff"
      :title="t('ws.revision.title')"
      width="max-w-5xl"
      @update:open="showRevisionDiff = $event"
    >
      <template #header>
        <div class="flex items-center gap-2 min-w-0">
          <History class="w-4 h-4 text-purple shrink-0" />
          <span class="text-sm font-bold text-ink whitespace-nowrap">{{ t("ws.revision.title") }}</span>
          <span class="text-xs text-ink-muted font-mono truncate">{{ selectedSourceFile }}</span>
          <span class="px-1.5 py-0.5 rounded bg-purple/15 text-purple text-[10px] font-mono shrink-0">
            {{ revisionDiffHash.substring(0, 8) }}
          </span>
        </div>
      </template>

      <div v-if="isLoadingRevisionDiff" class="py-10 text-center text-xs text-ink-muted">{{ t("ws.revision.loading") }}</div>
      <pre
        v-else
        class="p-3 rounded-lg bg-canvas border border-line text-[11px] leading-relaxed font-mono whitespace-pre-wrap break-all"
      >{{ revisionDiff || t("ws.revision.empty") }}</pre>
    </UiModal>

    <!-- Named layout presets popover -->
    <template v-if="showLayoutMenu">
      <div class="fixed inset-0 z-40" @click="showLayoutMenu = false"></div>
      <div
        data-glass
        class="fixed z-50 w-64 rounded-xl border glass glass-panel glass-popover p-2 space-y-1.5 select-none"
        :style="{ left: `${layoutMenuPosition.x}px`, top: `${layoutMenuPosition.y}px` }"
        @click.stop
      >
        <div class="flex items-center justify-between px-1">
          <span class="text-[10px] font-semibold text-ink-muted tracking-wider">{{ t("ws.layout.custom") }}</span>
          <span class="text-[9px] font-mono text-ink-muted">{{ layoutPresets.length }}/{{ 5 }}</span>
        </div>
        <div v-if="layoutPresets.length === 0" class="px-2 py-3 text-center text-[10px] text-ink-muted leading-relaxed">
          {{ t("ws.layout.custom.empty") }}
        </div>
        <div
          v-for="preset in layoutPresets"
          :key="preset.id"
          class="group flex items-center gap-1 px-1 py-1 rounded-lg hover:bg-raised transition"
        >
          <button
            @click="applyNamedLayoutPreset(preset)"
            class="flex-1 min-w-0 text-left cursor-pointer"
            :title="t('ws.layout.custom.apply', { name: preset.name })"
          >
            <div class="text-[11px] text-ink truncate">{{ preset.name }}</div>
            <div class="text-[9px] font-mono text-ink-muted">{{ preset.left }} / {{ preset.changes }} px</div>
          </button>
          <button :aria-label="t('ws.layout.custom.delete')"
            @click.stop="deleteNamedLayoutPreset(preset.id)"
            class="shrink-0 p-1 rounded text-ink-muted hover:text-danger hover:bg-danger/10 transition cursor-pointer opacity-0 group-hover:opacity-100"
            :title="t('ws.layout.custom.delete')"
          >
            <Trash2 class="w-3 h-3" />
          </button>
        </div>
        <div class="border-t border-line pt-2 flex items-center gap-1.5">
          <input
            v-model="newLayoutPresetName"
            type="text"
            maxlength="24"
            spellcheck="false"
            :placeholder="t('ws.layout.custom.placeholder')"
            class="flex-1 min-w-0 px-2 py-1 rounded-md bg-canvas border border-line text-[10px] text-ink focus:outline-none focus:border-accent"
            @keydown.enter.prevent="saveNamedLayoutPreset()"
          />
          <button
            @click="saveNamedLayoutPreset()"
            class="flex items-center gap-1 px-2 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-[10px] font-semibold transition cursor-pointer shrink-0"
          >
            <Plus class="w-3 h-3" /> {{ t("ws.layout.custom.save") }}
          </button>
        </div>
      </div>
    </template>

    <AssetDiffModal
      v-model:open="showAssetDiff"
      :repo-path="currentRepoPath"
      :path="assetDiffPath"
      :old-ref="assetDiffOldRef"
      :new-ref="assetDiffNewRef"
    />

    <AiExplainDiffModal
      :is-open="showAiExplainDiff"
      :repo-path="currentRepoPath"
      :file-path="aiExplainFilePath"
      :diff="aiExplainDiffText"
      @close="showAiExplainDiff = false"
    />

    <AiRepoOverrideModal
      :is-open="showAiRepoOverride"
      :repo-path="currentRepoPath"
      @close="showAiRepoOverride = false"
    />

    <AiChatModal
      :is-open="showAiChat"
      :repo-path="currentRepoPath"
      :file-path="aiChatFilePath"
      :diff="aiChatDiff"
      :scope="aiChatScope"
      @close="showAiChat = false"
    />

    <AiReviewModal
      :is-open="showAiReview"
      :repo-path="currentRepoPath"
      :file-path="aiReviewFilePath"
      :diff="aiReviewDiff"
      @close="showAiReview = false"
    />

    <AiConflictAdvisor
      :is-open="showAiConflictAdvisor"
      :file-path="aiAdvisorFilePath"
      :block-index="aiAdvisorBlockIndex"
      :block-total="aiAdvisorBlockTotal"
      :reload-nonce="aiAdvisorReloadNonce"
      :applying="aiAdvisorApplying"
      :ours="aiAdvisorOurs"
      :theirs="aiAdvisorTheirs"
      :base="aiAdvisorBase"
      @close="showAiConflictAdvisor = false"
      @apply="handleApplyAiConflictResolution"
      @open-settings="handleOpenAiSettings"
    />

    <AiConflictBatchModal
      :is-open="showAiConflictBatch"
      :repo-path="currentRepoPath"
      :file-path="aiConflictBatchFilePath"
      :blocks="aiConflictBatchBlocks"
      :targets="aiConflictBatchTargets"
      @close="showAiConflictBatch = false"
      @refresh="handleBatchApplied"
      @notify="notify"
    />

    <SemanticConflictModal
      v-model:open="showSemanticConflictModal"
      :current-repo-path="currentRepoPath"
      :file-path="semanticConflictFilePath"
      @resolved="() => { emit('refresh'); notify(t('conflict.semantic.success', { count: 1 })); }"
      @open-ai-batch="handleOpenAiBatchFromSemantic"
      @notify="notify"
    />

    <ThreeWayMergeModal
      v-model:open="showThreeWayMergeModal"
      :repo-path="currentRepoPath"
      :file-path="threeWayMergeFilePath"
      @resolved="handleThreeWayResolved"
      @notify="notify"
    />
  </div>
</template>
