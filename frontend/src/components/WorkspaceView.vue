<script setup lang="ts">
import { confirmAction } from "../composables/useConfirm";
import { t } from "../i18n";
import {
  ref,
  computed,
  watch,
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
} from "lucide-vue-next";
import { highlightCode } from "../utils/prismLanguages";
import { gitService } from "../services/gitService";
import Toast from "./ui/Toast.vue";
import { useToast } from "../composables/useToast";
import UiModal from "./ui/UiModal.vue";
import SourceExplorer from "./workspace/SourceExplorer.vue";
import { isActionCustomized } from "../utils/shortcuts";
import SourcePanorama from "./workspace/SourcePanorama.vue";
import DiffViewer from "./workspace/DiffViewer.vue";
import AssetDiffModal from "./assets/AssetDiffModal.vue";
import ChangesList from "./workspace/ChangesList.vue";
import ConflictBanner from "./workspace/ConflictBanner.vue";
import CommitComposer from "./workspace/CommitComposer.vue";
import type {
  RepoStatus,
  FileStatus,
  ProjectSourceFileItem,
  SourceFileDetail,
  BlameLine,
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
    return { left: DEFAULT_LEFT_PANE_WIDTH, changes: DEFAULT_CHANGES_PANE_WIDTH };
  }
}

const initialPaneSizes = loadPaneSizes(props.currentRepoPath);
const leftPaneWidth = ref(initialPaneSizes.left);
const isResizing = ref(false);

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

async function toggleBlame() {
  showBlame.value = !showBlame.value;
  if (showBlame.value && selectedSourceFile.value) {
    await fetchBlame();
  }
}

async function fetchBlame() {
  if (!props.currentRepoPath || !selectedSourceFile.value) return;
  const repoPath = props.currentRepoPath;
  const filePath = selectedSourceFile.value;
  isLoadingBlame.value = true;
  try {
    const list = await gitService.getFileBlame(repoPath, filePath);
    if (props.currentRepoPath !== repoPath || selectedSourceFile.value !== filePath) return;
    blameLines.value = list || [];
  } catch (err) {
    console.error("Failed to load blame:", err);
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
    console.error("Failed to load file history:", err);
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
      notify(t("ws.toast.version.switched", { hash: commitHash.substring(0, 8) }));
    }
  } catch (err: any) {
    notify(t("ws.toast.version.failed", { error: err?.message || err }));
  }
}

async function restoreCurrentFileRevision() {
  if (!selectedSourceFile.value) return;
  selectedFileRevisionHash.value = "";
  await selectSourceFile(selectedSourceFile.value);
  notify(t("ws.toast.version.current"));
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
const isCommitting = ref(false);


// Staged, Unstaged, and Conflicted file lists
const stagedFiles = computed(() => props.repoStatus?.stagedFiles || []);
const unstagedFiles = computed(() => [
  ...(props.repoStatus?.unstagedFiles || []),
  ...(props.repoStatus?.untrackedFiles || []),
]);
const conflictedFiles = computed(() => props.repoStatus?.conflictedFiles || []);
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
    console.error("Failed to get source file detail:", err);
  }
}

// Inspect source file code in right pane
function inspectSourceCode(path: string) {
  selectSourceFile(path);
  activeRightTab.value = "source";
}

// Select a changed file on the right pane to inspect its diff
async function selectChangeFile(file: FileStatus) {
  selectedChangeFile.value = file;
  activeRightTab.value = "diff";
  if (activeDiffMode.value === "word" && !canUseWordDiff.value) {
    activeDiffMode.value = "unified";
  }
  await loadSelectedDiff();
}

async function fetchWorkingDiff(repoPath: string, filePath: string, staged: boolean): Promise<string> {
  if (typeof gitService.getWorkingDiffAdvanced === "function") {
    try {
      return await gitService.getWorkingDiffAdvanced(repoPath, filePath, staged, ignoreWhitespace.value);
    } catch {
      return await gitService.getWorkingDiff(repoPath, filePath, staged);
    }
  }
  return await gitService.getWorkingDiff(repoPath, filePath, staged);
}

async function loadSelectedDiff() {
  const file = selectedChangeFile.value;
  if (!file || !props.currentRepoPath) return;
  isLoadingDiff.value = true;
  try {
    let diff = "";
    if (file.isConflicted || conflictedFiles.value.includes(file.path)) {
      try {
        diff = await gitService.getConflictFileContent(props.currentRepoPath, file.path);
      } catch {
        diff = await fetchWorkingDiff(props.currentRepoPath, file.path, false);
      }
    } else if (activeDiffMode.value === "word") {
      diff = await gitService.getWorkingWordDiff(props.currentRepoPath, file.path, file.isStaged);
    } else {
      diff = await fetchWorkingDiff(props.currentRepoPath, file.path, file.isStaged);
    }
    currentDiffText.value = diff || t("ws.diff.empty");
  } catch (err) {
    currentDiffText.value = t("ws.diff.failed", { error: err instanceof Error ? err.message : String(err) });
  } finally {
    isLoadingDiff.value = false;
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

async function handleDiscardAll() {
  if (!props.currentRepoPath) return;
  if (
    !(await confirmAction(
      { message: t("ws.confirm.discard.all"), tone: "danger" }
    ))
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
    await gitService.stageHunk(props.currentRepoPath, patch);
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

async function handleCommit() {
  if (!commitSubject.value.trim() || !props.currentRepoPath) return;
  isCommitting.value = true;
  try {
    const fullMsg = commitBody.value
      ? `${commitSubject.value.trim()}\n\n${commitBody.value.trim()}`
      : commitSubject.value.trim();

    // If no files staged, stage all first
    if (stagedFiles.value.length === 0 && unstagedFiles.value.length > 0) {
      await gitService.stageAll(props.currentRepoPath);
    }

    await gitService.commit(props.currentRepoPath, fullMsg, isAmend.value);

    if (autoPush.value) {
      await gitService.push(props.currentRepoPath);
    }

    commitSubject.value = "";
    commitBody.value = "";
    emit("refresh");
    sourceRefreshNonce.value++;
  } catch (err: any) {
    notify(t("ws.toast.commit.failed", { error: err?.message || err }));
  } finally {
    isCommitting.value = false;
  }
}


// Source Code Viewer highlighted lines
const highlightedSourceLines = computed<string[]>(() => {
  if (!sourceFileDetail.value?.content) return [];
  const code = sourceFileDetail.value.content;
  const lang = sourceFileDetail.value.language;
  const html = highlightCode(code, lang);
  return html.split("\n");
});

const sourceCodeLines = computed(() => {
  if (!sourceFileDetail.value?.content) return [];
  return sourceFileDetail.value.content.split("\n");
});

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

  // When active right tab is 'diff', allow change list navigation
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
  window.addEventListener("wgit:open-file", handleWgitOpenFile);
});

onUnmounted(() => {
  window.removeEventListener("wgit:workspace-layout", handleWorkspaceLayoutEvent);
  window.removeEventListener("wgit:workspace-toggle-diff-split", handleShortcutToggleDiffSplit);
  window.removeEventListener("wgit:workspace-commit", handleShortcutCommit);
  window.removeEventListener("keydown", handleWorkspaceKeydown);
  window.removeEventListener("wgit:open-file", handleWgitOpenFile);
  clearSourceLineHighlight();
});
</script>

<template>
  <div class="flex-1 flex overflow-hidden select-none">
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
    <SourceExplorer
      :repo-path="currentRepoPath"
      :width="leftPaneWidth"
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
      @mousedown="startResizing"
      @dblclick="resetLeftPaneWidth"
      class="w-1 cursor-col-resize hover:bg-accent/60 active:bg-accent-strong transition-colors shrink-0 z-30"
      :class="{ 'bg-accent-strong': isResizing }"
      :title="t('ws.resize.source.title')"
    ></div>

    <!-- ======================================================== -->
    <!-- RIGHT PANE: 变更检查 & 差分工作台 (Changes & Diff - 变更放右边) -->
    <!-- ======================================================== -->
    <main class="flex-1 flex flex-col bg-canvas overflow-hidden min-w-0">
      <!-- Top Right Tab Switcher & Diff Mode Selector -->
      <div class="h-11 border-b border-line px-3 flex items-center justify-between gap-2 bg-surface/40 backdrop-blur-sm shrink-0 overflow-x-auto">
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

        <!-- Mode-Specific Action Controls -->
        <div v-if="activeRightTab === 'diff'" class="flex items-center gap-2 shrink-0 whitespace-nowrap">
          <!-- Staged / Unstaged Summary Counters (hidden on narrow viewports to keep the bar readable) -->
          <div class="hidden xl:flex items-center gap-2 text-xs">
            <span class="flex items-center gap-1 font-semibold text-success">
              <CheckCheck class="w-3.5 h-3.5" /> {{ t("ws.stat.staged", { count: stagedFiles.length }) }}
            </span>
            <span aria-hidden="true" class="w-px h-3 bg-line shrink-0"></span>
            <span class="flex items-center gap-1 font-semibold text-warning">
              <Plus class="w-3.5 h-3.5" /> {{ t("ws.stat.unstaged", { count: unstagedFiles.length }) }}
            </span>
          </div>

          <!-- Layout presets -->
          <div class="hidden lg:flex items-center gap-0.5 bg-surface rounded-lg p-0.5 border border-line shrink-0" role="group" :aria-label="t('ws.layout.group')">
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
            class="p-1 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0 relative"
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
            <span class="hidden xl:inline">{{ t("ws.diff.ignore.whitespace") }}</span>
            <span class="xl:hidden">-w</span>
          </button>
        </div>

        <div v-else-if="activeRightTab === 'source' && sourceFileDetail" class="flex items-center gap-2 shrink-0 whitespace-nowrap">
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
          <div
            class="border-r border-line bg-surface/30 flex flex-col overflow-hidden shrink-0"
            :class="{ 'transition-[width] duration-200 ease-out': !isResizingChanges }"
            :style="{ width: `${changesPaneWidth}px` }"
          >
            <div
              v-if="workspaceStats"
              class="px-3 py-1.5 border-b border-line bg-surface/40 flex items-center gap-2 text-[10px] font-mono shrink-0 overflow-x-auto"
              :title="workspaceStats.summary"
            >
              <span class="text-accent shrink-0">{{ t("ws.stat.staged", { count: workspaceStats.stagedFiles }) }}</span>
              <span class="text-warning shrink-0">{{ t("ws.stat.unstaged", { count: workspaceStats.unstagedFiles }) }}</span>
              <span v-if="workspaceStats.untrackedFiles" class="text-ink-muted shrink-0">{{ t("ws.stat.untracked", { count: workspaceStats.untrackedFiles }) }}</span>
              <span v-if="workspaceStats.conflictedFiles" class="text-danger shrink-0">{{ t("ws.stat.conflicted", { count: workspaceStats.conflictedFiles }) }}</span>
              <span class="ml-auto text-success shrink-0">+{{ workspaceStats.totalAdditions }}</span>
              <span class="text-danger shrink-0">-{{ workspaceStats.totalDeletions }}</span>
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
              @refresh="emit('refresh')"
              @notify="notify"
            />

            <!-- Docked Commit Composer at bottom of Changes Pane -->
            <CommitComposer
              v-model:subject="commitSubject"
              v-model:body="commitBody"
              v-model:amend="isAmend"
              v-model:autoPush="autoPush"
              :committing="isCommitting"
              :is-new-repo="isNewRepo"
              @commit="handleCommit"
              @fast-commit="handleFastCommit"
              @generate="generateSmartCommitMessage"
              @load-template="handleLoadCommitTemplate"
              @load-last="handleLoadLastCommitMessage"
              @save-template="handleSaveCommitTemplate"
              @notify="notify"
            />

          </div>

          <!-- Draggable Divider between Changes List and Diff Inspector -->
          <div
            @mousedown="startResizingChanges"
            @dblclick="resetChangesPaneWidth"
            class="w-1 cursor-col-resize hover:bg-accent/60 active:bg-accent-strong transition-colors shrink-0 z-20"
            :class="{ 'bg-accent-strong': isResizingChanges }"
            :title="t('ws.resize.changes.title')"
          ></div>

          <!-- 2. 差异比对主区 (Diff Canvas Inspector) -->
          <div class="flex-1 flex flex-col overflow-hidden bg-canvas min-w-0">

        <!-- Diff Canvas -->
        <div class="flex-1 overflow-auto bg-canvas relative font-mono text-[12px] leading-5 select-text">
          <div v-if="isLoadingDiff" class="flex items-center justify-center h-full text-ink-muted">
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
              <div class="flex items-start gap-3">
                <span class="w-5 h-5 rounded-full bg-accent/15 text-accent text-xs font-bold flex items-center justify-center shrink-0 mt-0.5">1</span>
                <div class="text-xs">
                  <p class="text-ink font-medium">{{ t("ws.empty.new.repo.step1") }}</p>
                </div>
              </div>
              <div class="flex items-start gap-3">
                <span class="w-5 h-5 rounded-full bg-accent/15 text-accent text-xs font-bold flex items-center justify-center shrink-0 mt-0.5">2</span>
                <div class="text-xs">
                  <p class="text-ink font-medium">{{ t("ws.empty.new.repo.step2") }}</p>
                </div>
              </div>
              <div class="flex items-start gap-3">
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
            <div class="sticky top-0 z-10 px-4 py-2 bg-surface/90 backdrop-blur-md border-b border-line flex flex-wrap items-center justify-between gap-x-2 gap-y-1 select-none">
              <div class="flex items-center gap-2 min-w-0">
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
              </div>

              <div class="flex flex-wrap items-center gap-2 shrink-0">
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
              </div>
            </div>

            <!-- Conflict Resolution Action Bar -->
            <div
              v-if="isConflictedSelected && selectedChangeFile"
              class="sticky top-10 z-10 px-4 py-2 bg-surface/95 backdrop-blur-md border-b border-line flex flex-wrap items-center justify-between gap-2 select-none shadow-sm"
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
              </div>
            </div>

      <DiffViewer
        :diff-text="currentDiffText"
        :mode="activeDiffMode"
        :can-hunk-action="activeDiffMode !== 'word' && !!selectedChangeFile && !selectedChangeFile.isStaged && !isConflictedSelected"
        :can-line-action="activeDiffMode === 'unified' && !!selectedChangeFile && !selectedChangeFile.isStaged && !isConflictedSelected"
        :hunk-action-busy="isHunkActionBusy"
        @stage-hunk="handleStageHunk"
        @discard-hunk="handleDiscardHunk"
        @stage-lines="handleStageLines"
        @discard-lines="handleDiscardLines"
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
        @update:image-bg-mode="imageBgMode = $event"
        @update:image-pixelated="imagePixelated = $event"
        @update:svg-view-mode="svgViewMode = $event"
        @zoom-in="zoomIn"
        @zoom-out="zoomOut"
        @reset-zoom="resetZoom"
        @image-loaded="handleImageLoaded"
      />
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

    <Toast :text="toastText" :visible="showToast" />

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
  </div>
</template>
