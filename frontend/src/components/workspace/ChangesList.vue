<script setup lang="ts">
import { t } from "../../i18n";
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import { GitCommit, Search, RefreshCw, Plus, Minus, Lock, ChevronDown, Trash2, FileText, Image, Box, FileSpreadsheet, Code2, AlertTriangle, Check, Archive, Link, Sparkles, ShieldCheck } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { generateDeepLink } from "../../utils/deepLink";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import type { ToastOptions } from "../../composables/useToast";
import type { FileStatus, SafetyNetSnapshot } from "../../types";

const props = defineProps<{
  stagedFiles: FileStatus[];
  unstagedFiles: FileStatus[];
  conflictedFiles: string[];
  selectedPath: string;
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "select-file", file: FileStatus): void;
  (e: "stage", path: string): void;
  (e: "unstage", path: string): void;
  (e: "discard", path: string): void;
  (e: "toggle-lfs-lock", file: FileStatus): void;
  (e: "stage-all"): void;
  (e: "unstage-all"): void;
  (e: "discard-all"): void;
  (e: "auto-resolve-meta", strategy: "theirs" | "backup_theirs"): void;
  (e: "refresh"): void;
  (e: "notify", msg: string, options?: ToastOptions): void;
}>();

const totalChangesCount = computed(
  () => props.stagedFiles.length + props.unstagedFiles.length + props.conflictedFiles.length
);

// Batch Selection for Changes
const selectedUnstagedPaths = ref<Set<string>>(new Set());
const selectedStagedPaths = ref<Set<string>>(new Set());

function toggleUnstagedSelect(path: string) {
  const next = new Set(selectedUnstagedPaths.value);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  selectedUnstagedPaths.value = next;
}

function toggleAllUnstaged() {
  if (selectedUnstagedPaths.value.size === props.unstagedFiles.length) {
    selectedUnstagedPaths.value = new Set();
  } else {
    selectedUnstagedPaths.value = new Set(props.unstagedFiles.map((f) => f.path));
  }
}

function toggleStagedSelect(path: string) {
  const next = new Set(selectedStagedPaths.value);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  selectedStagedPaths.value = next;
}

function toggleAllStaged() {
  if (selectedStagedPaths.value.size === props.stagedFiles.length) {
    selectedStagedPaths.value = new Set();
  } else {
    selectedStagedPaths.value = new Set(props.stagedFiles.map((f) => f.path));
  }
}

function clearBatchSelection() {
  selectedUnstagedPaths.value = new Set();
  selectedStagedPaths.value = new Set();
}

async function handleBatchStage() {
  if (!props.currentRepoPath || selectedUnstagedPaths.value.size === 0) return;
  const paths = Array.from(selectedUnstagedPaths.value);
  try {
    await gitService.batchStageFiles(props.currentRepoPath, paths);
    emit('notify', t('changes.toast.batch.staged', { count: paths.length }));
    selectedUnstagedPaths.value = new Set();
    emit("refresh");
  } catch (err: any) {
    emit('notify', t('changes.toast.batch.stage.failed', { error: err?.message || err }));
  }
}

async function handleBatchUnstage() {
  if (!props.currentRepoPath || selectedStagedPaths.value.size === 0) return;
  const paths = Array.from(selectedStagedPaths.value);
  try {
    await gitService.batchUnstageFiles(props.currentRepoPath, paths);
    emit('notify', t('changes.toast.batch.unstaged', { count: paths.length }));
    selectedStagedPaths.value = new Set();
    emit("refresh");
  } catch (err: any) {
    emit('notify', t('changes.toast.batch.unstage.failed', { error: err?.message || err }));
  }
}

// Batch stash: stash only the checked unstaged files.
const showStashSelectionModal = ref(false);
const stashSelectionMessage = ref("");
const isStashingSelection = ref(false);

function onStashModalToggle(open: boolean) {
  if (!open && isStashingSelection.value) return;
  showStashSelectionModal.value = open;
}

function openStashSelectionModal() {
  if (selectedUnstagedPaths.value.size === 0) return;
  const selected = props.unstagedFiles.filter((f) => selectedUnstagedPaths.value.has(f.path));
  const untracked = selected.filter((f) => f.unstagedStatus === "?");
  if (untracked.length > 0) {
    emit(
      "notify",
      t("changes.toast.stash.untracked", { count: untracked.length })
    );
    return;
  }
  stashSelectionMessage.value = "";
  showStashSelectionModal.value = true;
}

async function handleStashSelection() {
  if (!props.currentRepoPath || selectedUnstagedPaths.value.size === 0) return;
  const paths = Array.from(selectedUnstagedPaths.value);
  isStashingSelection.value = true;
  try {
    await gitService.saveStashWithFiles(
      props.currentRepoPath,
      stashSelectionMessage.value.trim(),
      paths
    );
    emit("notify", t("changes.toast.stash.selected", { count: paths.length }));
    selectedUnstagedPaths.value = new Set();
    showStashSelectionModal.value = false;
    emit("refresh");
  } catch (err: any) {
    emit("notify", t("changes.toast.stash.selected.failed", { error: err?.message || err }));
  } finally {
    isStashingSelection.value = false;
  }
}

// Discard Safety Net & Instant Undo
const lastSafetyNetSnapshot = ref<SafetyNetSnapshot | null>(null);
const isRestoringSafetyNet = ref(false);
let safetyNetTimer: ReturnType<typeof setTimeout> | null = null;

async function handleBatchDiscard() {
  if (!props.currentRepoPath || selectedUnstagedPaths.value.size === 0) return;
  const paths = Array.from(selectedUnstagedPaths.value);
  if (!(await confirmAction({ message: t("changes.confirm.discard.selected", { count: paths.length }), tone: "danger" }))) return;
  try {
    const snap = await gitService.discardWithSafetyNet(props.currentRepoPath, paths);
    lastSafetyNetSnapshot.value = snap;
    if (safetyNetTimer) clearTimeout(safetyNetTimer);
    safetyNetTimer = setTimeout(() => {
      lastSafetyNetSnapshot.value = null;
    }, 10000);
    // Offer the undo on the toast itself. The snapshot only lives for ten
    // seconds, so the window to act is short — making the user hunt for the
    // banner in the list instead of tapping the message they are already
    // looking at wastes most of it.
    emit('notify', t('changes.toast.discard.safetynet', { count: paths.length }), {
      tone: 'warning',
      duration: 10000,
      action: {
        label: t('changes.toast.discard.undo'),
        run: () => {
          void handleUndoDiscard();
        },
      },
    });
    selectedUnstagedPaths.value = new Set();
    emit("refresh");
  } catch (err: any) {
    emit('notify', t('changes.toast.discard.selected.failed', { error: err?.message || err }));
  }
}

async function handleUndoDiscard() {
  if (!props.currentRepoPath || !lastSafetyNetSnapshot.value || isRestoringSafetyNet.value) return;
  isRestoringSafetyNet.value = true;
  try {
    await gitService.restoreFromSafetyNet(props.currentRepoPath, lastSafetyNetSnapshot.value.id);
    emit('notify', t('changes.toast.discard.restored'));
    lastSafetyNetSnapshot.value = null;
    emit("refresh");
  } catch (err: any) {
    emit('notify', t('changes.toast.discard.restore.failed', { error: err?.message || err }));
  } finally {
    isRestoringSafetyNet.value = false;
  }
}

// Changes filter & group collapse states
const changesSearch = ref("");
// 变更列表搜索同样做输入防抖（输入框即时回显，过滤使用防抖后的 changesQuery）
const changesQuery = ref("");
const CHANGES_SEARCH_DEBOUNCE_MS = 150;
const isChangesSearchDebouncing = ref(false);
let changesSearchTimer: ReturnType<typeof setTimeout> | null = null;

watch(changesSearch, (value) => {
  if (changesSearchTimer) {
    clearTimeout(changesSearchTimer);
    changesSearchTimer = null;
  }
  if (!value) {
    changesQuery.value = "";
    isChangesSearchDebouncing.value = false;
    return;
  }
  isChangesSearchDebouncing.value = true;
  changesSearchTimer = setTimeout(() => {
    changesQuery.value = changesSearch.value;
    isChangesSearchDebouncing.value = false;
    changesSearchTimer = null;
  }, CHANGES_SEARCH_DEBOUNCE_MS);
});

// Multi-dimensional status & extension filters
const statusFilter = ref<"all" | "staged" | "unstaged" | "conflicted">("all");
const extensionFilter = ref<string>("");
const extensionMenuOpen = ref(false);

function pickExtension(ext: string) {
  extensionFilter.value = extensionFilter.value === ext ? "" : ext;
  extensionMenuOpen.value = false;
}

const extensionStats = computed(() => {
  const counts = new Map<string, number>();
  const addPath = (p: string) => {
    const idx = p.lastIndexOf(".");
    if (idx !== -1 && idx < p.length - 1) {
      const ext = p.slice(idx).toLowerCase();
      if (ext.length <= 10) {
        counts.set(ext, (counts.get(ext) || 0) + 1);
      }
    }
  };
  for (const f of props.stagedFiles) addPath(f.path);
  for (const f of props.unstagedFiles) addPath(f.path);
  for (const p of props.conflictedFiles) addPath(p);
  return Array.from(counts.entries())
    .map(([ext, count]) => ({ ext, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 8);
});

function matchesExtension(path: string): boolean {
  if (!extensionFilter.value) return true;
  return path.toLowerCase().endsWith(extensionFilter.value.toLowerCase());
}

// 组级折叠状态持久化：localStorage 中以 { staged, unstaged, conflicted } 布尔对象存储
// （untracked 文件在 WorkspaceView 中已并入 unstaged 组，实际只有这三个分组）。
// 读写均 try/catch 吞错：隐私模式、存储被禁用或配额已满时 localStorage 会抛异常，
// 折叠偏好属于可丢失的 UI 状态，失败时静默回退为全展开，不应影响变更列表本身。
const CHANGES_GROUPS_COLLAPSED_KEY = "wgit.changes.groups.collapsed.v1";

type ChangesGroupsCollapsed = { staged: boolean; unstaged: boolean; conflicted: boolean };

function loadChangesGroupsCollapsed(): ChangesGroupsCollapsed {
  try {
    const raw = localStorage.getItem(CHANGES_GROUPS_COLLAPSED_KEY);
    const parsed = raw ? (JSON.parse(raw) as Partial<ChangesGroupsCollapsed>) : null;
    return {
      staged: parsed?.staged === true,
      unstaged: parsed?.unstaged === true,
      conflicted: parsed?.conflicted === true,
    };
  } catch {
    // 读失败（JSON 损坏 / 存储不可用）：按默认值（全展开）处理。
    return { staged: false, unstaged: false, conflicted: false };
  }
}

const initialCollapsedGroups = loadChangesGroupsCollapsed();
const isConflictedCollapsed = ref(initialCollapsedGroups.conflicted);
const isStagedCollapsed = ref(initialCollapsedGroups.staged);
const isUnstagedCollapsed = ref(initialCollapsedGroups.unstaged);

function persistChangesGroupsCollapsed() {
  const state: ChangesGroupsCollapsed = {
    staged: isStagedCollapsed.value,
    unstaged: isUnstagedCollapsed.value,
    conflicted: isConflictedCollapsed.value,
  };
  try {
    localStorage.setItem(CHANGES_GROUPS_COLLAPSED_KEY, JSON.stringify(state));
  } catch {
    // 写失败（配额已满 / 存储被禁用）：仅放弃持久化本次折叠偏好，不影响交互。
  }
}

watch([isStagedCollapsed, isUnstagedCollapsed, isConflictedCollapsed], persistChangesGroupsCollapsed);

const filteredConflictedFiles = computed(() => {
  if (statusFilter.value !== "all" && statusFilter.value !== "conflicted") return [];
  const q = changesQuery.value.toLowerCase();
  return props.conflictedFiles.filter((p) => {
    const matchesQuery = !q || p.toLowerCase().includes(q);
    return matchesQuery && matchesExtension(p);
  });
});

const metaConflictedFiles = computed(() => {
  return filteredConflictedFiles.value.filter((f) => {
    const ext = f.toLowerCase().slice(f.lastIndexOf("."));
    return ext === ".meta" || ext === ".json" || ext === ".yaml" || ext === ".yml";
  });
});
const hasMetaConflicts = computed(() => metaConflictedFiles.value.length > 0);

const filteredStagedFiles = computed(() => {
  if (statusFilter.value !== "all" && statusFilter.value !== "staged") return [];
  const q = changesQuery.value.toLowerCase();
  return props.stagedFiles.filter((f) => {
    const matchesQuery = !q || f.path.toLowerCase().includes(q);
    return matchesQuery && matchesExtension(f.path);
  });
});

const filteredUnstagedFiles = computed(() => {
  if (statusFilter.value !== "all" && statusFilter.value !== "unstaged") return [];
  const q = changesQuery.value.toLowerCase();
  return props.unstagedFiles.filter((f) => {
    const matchesQuery = !q || f.path.toLowerCase().includes(q);
    return matchesQuery && matchesExtension(f.path);
  });
});

const allVisibleFiles = computed<FileStatus[]>(() => {
  const result: FileStatus[] = [];
  for (const cPath of filteredConflictedFiles.value) {
    result.push({ path: cPath, stagedStatus: "U", unstagedStatus: "U", isStaged: false, isLfs: false, additions: 0, deletions: 0, size: "", isConflicted: true });
  }
  for (const f of filteredStagedFiles.value) {
    result.push(f);
  }
  for (const f of filteredUnstagedFiles.value) {
    result.push(f);
  }
  return result;
});

function handleChangesKeydown(e: KeyboardEvent) {
  const activeEl = document.activeElement;
  const isInput = activeEl instanceof HTMLInputElement || activeEl instanceof HTMLTextAreaElement || (activeEl as HTMLElement)?.isContentEditable;
  if (isInput) return;

  if (e.code === "Space") {
    if (!props.selectedPath) return;
    e.preventDefault();
    const stagedMatch = props.stagedFiles.find((f) => f.path === props.selectedPath);
    if (stagedMatch) {
      emit("unstage", props.selectedPath);
      return;
    }
    const unstagedMatch = props.unstagedFiles.find((f) => f.path === props.selectedPath);
    if (unstagedMatch) {
      emit("stage", props.selectedPath);
      return;
    }
  } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
    const list = allVisibleFiles.value;
    if (list.length === 0) return;
    e.preventDefault();
    const currentIndex = list.findIndex((f) => f.path === props.selectedPath);
    if (currentIndex === -1) {
      const first = list[0];
      if (first) emit("select-file", first);
    } else if (e.key === "ArrowDown") {
      const nextIndex = Math.min(list.length - 1, currentIndex + 1);
      const next = list[nextIndex];
      if (next) emit("select-file", next);
    } else if (e.key === "ArrowUp") {
      const prevIndex = Math.max(0, currentIndex - 1);
      const prev = list[prevIndex];
      if (prev) emit("select-file", prev);
    }
  }
}

onMounted(() => {
  window.addEventListener("keydown", handleChangesKeydown);
});

// 变更列表分组渲染上限：每组最多渲染 300 行，仅影响 DOM，过滤/全选/键盘导航仍基于完整列表。
const CHANGES_GROUP_RENDER_LIMIT = 300;
const conflictedRenderLimit = ref(CHANGES_GROUP_RENDER_LIMIT);
const stagedRenderLimit = ref(CHANGES_GROUP_RENDER_LIMIT);
const unstagedRenderLimit = ref(CHANGES_GROUP_RENDER_LIMIT);

const renderedConflictedFiles = computed(() =>
  filteredConflictedFiles.value.slice(0, conflictedRenderLimit.value)
);
const renderedStagedFiles = computed(() => filteredStagedFiles.value.slice(0, stagedRenderLimit.value));
const renderedUnstagedFiles = computed(() => filteredUnstagedFiles.value.slice(0, unstagedRenderLimit.value));

const hiddenConflictedCount = computed(() =>
  Math.max(0, filteredConflictedFiles.value.length - renderedConflictedFiles.value.length)
);
const hiddenStagedCount = computed(() =>
  Math.max(0, filteredStagedFiles.value.length - renderedStagedFiles.value.length)
);
const hiddenUnstagedCount = computed(() =>
  Math.max(0, filteredUnstagedFiles.value.length - renderedUnstagedFiles.value.length)
);

function showMoreChanges(group: "conflicted" | "staged" | "unstaged") {
  if (group === "conflicted") conflictedRenderLimit.value += CHANGES_GROUP_RENDER_LIMIT;
  else if (group === "staged") stagedRenderLimit.value += CHANGES_GROUP_RENDER_LIMIT;
  else unstagedRenderLimit.value += CHANGES_GROUP_RENDER_LIMIT;
}

watch([() => props.currentRepoPath, () => changesQuery.value], () => {
  conflictedRenderLimit.value = CHANGES_GROUP_RENDER_LIMIT;
  stagedRenderLimit.value = CHANGES_GROUP_RENDER_LIMIT;
  unstagedRenderLimit.value = CHANGES_GROUP_RENDER_LIMIT;
});


function splitFilePath(fullPath: string): { name: string; dir: string } {
  const lastSlash = fullPath.lastIndexOf("/");
  if (lastSlash === -1) {
    return { name: fullPath, dir: "" };
  }
  return {
    name: fullPath.slice(lastSlash + 1),
    dir: fullPath.slice(0, lastSlash + 1),
  };
}

function getStatusBadge(status?: string) {
  switch (status) {
    case "M":
      return { label: "M", title: t("changes.status.modified"), bg: "bg-warning/20 text-warning" };
    case "A":
      return { label: "A", title: t("changes.status.added"), bg: "bg-success/20 text-success" };
    case "D":
      return { label: "D", title: t("changes.status.deleted"), bg: "bg-danger/20 text-danger" };
    case "R":
      return { label: "R", title: t("changes.status.renamed"), bg: "bg-purple/20 text-purple" };
    case "?":
      return { label: "?", title: t("changes.status.untracked"), bg: "bg-cyan/20 text-cyan" };
    case "U":
      return { label: "U", title: t("changes.status.conflicted"), bg: "bg-danger-strong text-white" };
    default:
      return { label: "M", title: t("changes.status.modified.short"), bg: "bg-warning/20 text-warning" };
  }
}

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


async function copyFileDeepLink(filePath: string) {
  const link = generateDeepLink({
    action: "open",
    repo: props.currentRepoPath || undefined,
    tab: "workspace",
    file: filePath,
  });
  try {
    if (typeof navigator !== "undefined" && navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(link);
    }
    emit("notify", t("changes.toast.deeplink.copied"));
  } catch {
    emit("notify", t("common.copied"));
  }
}

async function handleApplyTemplate(templateType: string) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.generateGitignoreTemplate(props.currentRepoPath, templateType);
    emit("notify", t("changes.toast.template.applied", { type: templateType.toUpperCase() }));
    emit("refresh");
  } catch (err: any) {
    emit("notify", String(err?.message || err));
  }
}

onUnmounted(() => {
  window.removeEventListener("keydown", handleChangesKeydown);
  if (changesSearchTimer) {
    clearTimeout(changesSearchTimer);
    changesSearchTimer = null;
  }
});
</script>

<template>
            <!-- Changes List Header & Search Filter -->
            <div class="px-2.5 py-2 border-b border-line flex flex-col gap-2 shrink-0 bg-surface/50">
              <div class="flex flex-wrap items-center justify-between gap-x-2 gap-y-1 min-w-0">
                <div class="flex items-center gap-1.5 font-sans shrink-0 whitespace-nowrap">
                  <GitCommit class="w-4 h-4 text-accent shrink-0" />
                  <span class="text-xs font-bold text-ink whitespace-nowrap">{{ t("changes.title") }}</span>
                  <span
                    v-if="totalChangesCount > 0"
                    class="px-1.5 py-0.2 rounded-full text-[10px] font-bold whitespace-nowrap shrink-0"
                    :class="stagedFiles.length > 0 ? 'bg-success/20 text-success' : 'bg-warning/20 text-warning'"
                  >
                    {{ totalChangesCount }}
                  </span>
                </div>

                <!-- Batch Action Buttons: Stage All / Unstage All -->
                <div class="flex flex-wrap items-center gap-1 text-[11px] shrink-0 whitespace-nowrap">
                  <button
                    v-if="unstagedFiles.length > 0"
                    @click="emit('stage-all')"
                    class="px-1.5 py-0.5 rounded hover:bg-success/20 text-success text-[10px] font-semibold transition cursor-pointer flex items-center gap-0.5 whitespace-nowrap shrink-0"
                    :title="t('changes.stage.all.title')"
                  >
                    <Plus class="w-3 h-3 shrink-0" aria-hidden="true" /> {{ t("changes.stage.all") }}
                  </button>
                  <button
                    v-if="stagedFiles.length > 0"
                    @click="emit('unstage-all')"
                    class="px-1.5 py-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink text-[10px] font-semibold transition cursor-pointer flex items-center gap-0.5 whitespace-nowrap shrink-0"
                    :title="t('changes.unstage.all.title')"
                  >
                    <Minus class="w-3 h-3 shrink-0" aria-hidden="true" /> {{ t("changes.unstage.all") }}
                  </button>
                  <button
                    v-if="totalChangesCount > 0"
                    @click="emit('discard-all')"
                    class="px-1.5 py-0.5 rounded hover:bg-danger/20 text-ink-muted hover:text-danger text-[10px] font-semibold transition cursor-pointer flex items-center gap-0.5 whitespace-nowrap shrink-0"
                    :title="t('changes.discard.all.title')"
                  >
                    <Trash2 class="w-3 h-3 shrink-0" aria-hidden="true" /> {{ t("changes.discard.all") }}
                  </button>
                </div>
              </div>

              <!-- Filter Changed Files Input -->
              <div class="relative">
                <input
                  v-model="changesSearch"
                  type="text"
                  :aria-label="t('changes.filter.aria')"
                  :placeholder="t('changes.filter.placeholder')"
                  class="w-full pl-6 py-1 rounded-md bg-canvas/80 border border-line text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono transition"
                  :class="isChangesSearchDebouncing ? 'pr-12' : 'pr-5'"
                />
                <Search v-if="!isChangesSearchDebouncing" class="w-3 h-3 text-ink-muted absolute left-2 top-2 pointer-events-none" />
                <RefreshCw v-else class="w-3 h-3 text-accent absolute left-2 top-2 pointer-events-none animate-spin" />
                <span
                  v-if="isChangesSearchDebouncing"
                  class="absolute right-5 top-1.5 text-[9px] leading-none text-accent font-sans animate-pulse pointer-events-none select-none whitespace-nowrap"
                >
                  {{ t("changes.searching") }}
                </span>
                <button
                  v-if="changesSearch"
                  @click="changesSearch = ''"
                  class="absolute right-1.5 top-1.5 text-[10px] text-ink-muted hover:text-ink cursor-pointer font-bold"
                >
                  ✕
                </button>
              </div>

              <!--
                Status filter pills. Wrapped rather than hidden-scrollable: the
                changes column is ~300px wide, so a `no-scrollbar` row silently
                hid filters off the right edge. Wrapping keeps every filter
                visible and clickable.
              -->
              <div class="flex flex-wrap items-center gap-x-1 gap-y-1 text-[10px] py-0.5" data-testid="changes-status-filters">
                <button
                  type="button"
                  data-testid="status-filter-all"
                  @click="statusFilter = 'all'"
                  class="px-2 py-0.5 rounded-full font-semibold transition cursor-pointer flex items-center gap-1 shrink-0"
                  :class="statusFilter === 'all' ? 'bg-accent-strong text-accent-contrast' : 'bg-surface/80 border border-line text-ink-muted hover:text-ink'"
                >
                  <span>{{ t("changes.filter.status.all") }}</span>
                  <span class="text-[9px] opacity-80">({{ totalChangesCount }})</span>
                </button>
                <button
                  type="button"
                  data-testid="status-filter-staged"
                  @click="statusFilter = 'staged'"
                  class="px-2 py-0.5 rounded-full font-semibold transition cursor-pointer flex items-center gap-1 shrink-0"
                  :class="statusFilter === 'staged' ? 'bg-accent-strong text-accent-contrast' : 'bg-surface/80 border border-line text-ink-muted hover:text-ink'"
                >
                  <span>{{ t("changes.filter.status.staged") }}</span>
                  <span class="text-[9px] opacity-80">({{ stagedFiles.length }})</span>
                </button>
                <button
                  type="button"
                  data-testid="status-filter-unstaged"
                  @click="statusFilter = 'unstaged'"
                  class="px-2 py-0.5 rounded-full font-semibold transition cursor-pointer flex items-center gap-1 shrink-0"
                  :class="statusFilter === 'unstaged' ? 'bg-accent-strong text-accent-contrast' : 'bg-surface/80 border border-line text-ink-muted hover:text-ink'"
                >
                  <span>{{ t("changes.filter.status.unstaged") }}</span>
                  <span class="text-[9px] opacity-80">({{ unstagedFiles.length }})</span>
                </button>
                <button
                  type="button"
                  v-if="conflictedFiles.length > 0"
                  data-testid="status-filter-conflicted"
                  @click="statusFilter = 'conflicted'"
                  class="px-2 py-0.5 rounded-full font-semibold transition cursor-pointer flex items-center gap-1 shrink-0"
                  :class="statusFilter === 'conflicted' ? 'bg-danger text-white' : 'bg-danger/10 border border-danger/30 text-danger hover:bg-danger/20'"
                >
                  <span>{{ t("changes.filter.status.conflicted") }}</span>
                  <span class="text-[9px] opacity-80">({{ conflictedFiles.length }})</span>
                </button>
              <!-- Extension filter: one trigger instead of a chip wall; the menu keeps every extension reachable. -->
              <div v-if="extensionStats.length > 0" data-testid="changes-ext-filters" class="relative ml-auto flex flex-wrap items-center gap-1 py-0.5 text-[10px] font-mono" @mouseleave="extensionMenuOpen = false">
                <button
                  type="button"
                  data-testid="changes-ext-trigger"
                  @click="extensionMenuOpen = !extensionMenuOpen"
                  class="px-2 py-0.5 rounded-full flex items-center gap-1 shrink-0 border transition cursor-pointer"
                  :class="extensionFilter ? 'bg-accent-strong text-accent-contrast border-accent-strong' : 'bg-surface/80 border-line text-ink-muted hover:text-ink'"
                >
                  <span class="opacity-70">{{ t("changes.filter.ext.label") }}:</span>
                  <span>{{ extensionFilter || t("changes.filter.ext.any") }}</span>
                  <ChevronDown class="w-3 h-3" aria-hidden="true" />
                </button>
                <button
                  v-if="extensionFilter"
                  type="button"
                  :title="t('changes.filter.clear')"
                  :aria-label="t('changes.filter.clear')"
                  class="px-1 py-0.5 rounded text-ink-muted hover:text-ink cursor-pointer"
                  @click="extensionFilter = ''"
                >
                  ✕
                </button>
                <div
                  v-if="extensionMenuOpen"
                  class="absolute right-0 top-full mt-1 z-50 w-56 p-2 rounded-lg bg-canvas border border-line shadow-xl"
                >
                  <div class="grid grid-cols-3 gap-1">
                    <button
                      type="button"
                      data-testid="ext-chip-all"
                      @click="pickExtension('')"
                      class="px-1.5 py-1 rounded transition cursor-pointer text-center"
                      :class="!extensionFilter ? 'bg-accent-strong text-accent-contrast font-bold' : 'bg-surface border border-line text-ink-muted hover:text-ink'"
                    >
                      {{ t("changes.filter.ext.any") }}
                    </button>
                    <button
                      type="button"
                      v-for="stat in extensionStats"
                      :key="stat.ext"
                      :data-testid="`ext-chip-${stat.ext.replace('.', '')}`"
                      @click="pickExtension(stat.ext)"
                      class="px-1.5 py-1 rounded transition cursor-pointer text-center"
                      :class="extensionFilter === stat.ext ? 'bg-accent-strong text-accent-contrast font-bold' : 'bg-surface border border-line text-ink-muted hover:text-ink'"
                    >
                      {{ stat.ext }} <span class="opacity-70">({{ stat.count }})</span>
                    </button>
                  </div>
                </div>
              </div>

              </div>

              <!-- Keyboard shortcuts hint (the pane is too narrow for it on most layouts) -->
              <div class="@max-[560px]:hidden flex items-center justify-between text-[9px] text-ink-muted px-0.5 pt-0.5 select-none font-sans">
                <span><kbd class="px-1 py-0.2 rounded bg-surface border border-line font-mono text-[8px]">↑</kbd> <kbd class="px-1 py-0.2 rounded bg-surface border border-line font-mono text-[8px]">↓</kbd> {{ t("changes.hint.browse") }}</span>
                <span><kbd class="px-1 py-0.2 rounded bg-surface border border-line font-mono text-[8px]">Space</kbd> {{ t("changes.hint.toggle") }}</span>
                <span><kbd class="px-1 py-0.2 rounded bg-surface border border-line font-mono text-[8px]">⌘↵</kbd> {{ t("changes.hint.commit") }}</span>
              </div>

              <!-- Safety Net Instant Undo Banner -->
              <div
                v-if="lastSafetyNetSnapshot"
                class="mx-0.5 my-0.5 px-2 py-1.5 rounded-lg bg-warning/15 border border-warning/30 flex items-center justify-between text-[11px] text-warning select-none"
                data-testid="safetynet-undo-bar"
              >
                <div class="flex items-center gap-1.5 truncate">
                  <ShieldCheck class="w-3.5 h-3.5 text-warning shrink-0" />
                  <span class="font-medium truncate">{{ t("changes.toast.discard.safetynet", { count: lastSafetyNetSnapshot.count }) }}</span>
                </div>
                <button
                  type="button"
                  data-testid="undo-discard-btn"
                  @click="handleUndoDiscard"
                  :disabled="isRestoringSafetyNet"
                  class="px-2 py-0.5 rounded bg-warning text-black font-bold hover:opacity-90 transition cursor-pointer text-[10px] shrink-0"
                >
                  {{ isRestoringSafetyNet ? '...' : t("changes.toast.discard.undo") }}
                </button>
              </div>
            </div>

            <!-- Scrollable Vertical Changes List -->
            <div class="flex-1 overflow-y-auto p-1.5 space-y-2 select-none text-[11px] font-mono">
              <!-- 1. 冲突文件列表 (Conflicted Files Group) -->
              <div v-if="filteredConflictedFiles.length > 0" class="flex flex-col">
                <div
                  data-testid="changes-group-header-conflicted"
                  @click="isConflictedCollapsed = !isConflictedCollapsed"
                  class="flex items-center justify-between px-2 py-1 rounded cursor-pointer hover:bg-danger/10 text-danger text-[11px] font-bold select-none"
                >
                  <div class="flex items-center gap-1">
                    <ChevronDown class="w-3 h-3 shrink-0 transition-transform" :class="isConflictedCollapsed ? '-rotate-90' : ''" />
                    <span>{{ t("changes.group.conflicted") }}</span>
                    <span class="px-1.5 py-0.2 rounded-full bg-danger-strong text-white text-[9px]">{{ filteredConflictedFiles.length }}</span>
                  </div>
                </div>

                <div v-if="!isConflictedCollapsed" class="mt-0.5 space-y-0.5">
                  <div
                    v-if="hasMetaConflicts"
                    class="mx-1 my-1 px-2 py-1 rounded-md bg-purple/10 border border-purple/20 flex items-center justify-between gap-1 text-[10px]"
                  >
                    <span class="text-purple font-semibold truncate">{{ t("ws.conflict.auto.meta.desc") }}</span>
                    <button
                      @click.stop="emit('auto-resolve-meta', 'theirs')"
                      class="px-1.5 py-0.5 rounded bg-purple text-white hover:bg-purple-strong font-bold shrink-0 cursor-pointer text-[9px] transition"
                      :title="t('ws.conflict.theirs.title')"
                    >
                      {{ t("ws.conflict.theirs.action") }} ({{ metaConflictedFiles.length }})
                    </button>
                  </div>
                  <div
                    v-for="cPath in renderedConflictedFiles"
                    :key="cPath"
                    @click="emit('select-file', { path: cPath, stagedStatus: 'U', unstagedStatus: 'U', isStaged: false, isLfs: false, additions: 0, deletions: 0, size: '', isConflicted: true })"
                    class="group flex items-center justify-between px-2 py-1.5 rounded-lg cursor-pointer transition"
                    :class="selectedPath === cPath ? 'bg-danger/20 border-l-2 border-danger text-danger font-bold' : 'hover:bg-danger/10 text-danger/80'"
                  >
                    <div class="flex items-center gap-2 truncate min-w-0">
                      <AlertTriangle class="w-3.5 h-3.5 shrink-0 text-danger" />
                      <div class="flex flex-col truncate">
                        <span class="truncate font-semibold text-danger">{{ splitFilePath(cPath).name }}</span>
                        <span v-if="splitFilePath(cPath).dir" class="text-[9px] text-danger/60 truncate font-mono">{{ splitFilePath(cPath).dir }}</span>
                      </div>
                    </div>
                    <span class="shrink-0 px-1 py-0.2 rounded bg-danger-strong text-white text-[9px] font-bold">CONFLICT</span>
                  </div>

                  <!-- 冲突分组渲染上限 -->
                  <button
                    v-if="hiddenConflictedCount > 0"
                    @click="showMoreChanges('conflicted')"
                    class="mt-0.5 w-full px-2 py-1 rounded-lg border border-dashed border-danger/30 text-danger/80 hover:text-danger hover:bg-danger/10 text-[10px] font-semibold transition cursor-pointer flex items-center justify-center gap-1"
                  >
                    <ChevronDown class="w-3 h-3" aria-hidden="true" /> {{ t("changes.show.more", { count: hiddenConflictedCount }) }}
                  </button>
                </div>
              </div>

              <!-- 2. 已暂存列表 (Staged Changes Group) -->
              <div v-if="filteredStagedFiles.length > 0" class="flex flex-col">
                <div
                  data-testid="changes-group-header-staged"
                  @click="isStagedCollapsed = !isStagedCollapsed"
                  class="flex items-center justify-between px-2 py-1 rounded cursor-pointer hover:bg-surface text-ink-muted hover:text-ink text-[11px] font-bold select-none"
                >
                  <div class="flex items-center gap-1 text-success">
                    <ChevronDown class="w-3 h-3 shrink-0 transition-transform" :class="isStagedCollapsed ? '-rotate-90' : ''" />
                    <span>{{ t("changes.group.staged") }}</span>
                    <span class="px-1.5 py-0.2 rounded-full bg-success/20 text-success text-[9px]">{{ filteredStagedFiles.length }}</span>
                  </div>
                  <div class="flex items-center gap-1">
                    <button
                      @click.stop="toggleAllStaged"
                      class="text-[10px] text-ink-muted hover:text-ink hover:underline p-0.5 rounded font-normal"
                      :title="t('workspace.changes.toggle.all')"
                    >
                      {{ selectedStagedPaths.size === filteredStagedFiles.length ? t("changes.unselect") : t("changes.select.all") }}
                    </button>
                    <button
                      @click.stop="emit('unstage-all')"
                      class="hover:text-warning p-0.5 rounded text-[10px] font-normal"
                      :title="t('workspace.changes.unstage.all')"
                    >
                      {{ t("changes.unstage.all.short") }}
                    </button>
                  </div>
                </div>

                <div v-if="!isStagedCollapsed" class="mt-0.5 space-y-0.5">
                  <div
                    v-for="file in renderedStagedFiles"
                    :key="file.path"
                    @click="emit('select-file', file)"
                    class="group flex items-center justify-between px-2 py-1.5 rounded-lg cursor-pointer transition"
                    :class="selectedPath === file.path ? 'bg-raised border-l-2 border-accent text-accent font-medium shadow-xs' : 'hover:bg-raised/50 text-ink-secondary hover:text-ink'"
                  >
                    <div class="flex items-center gap-2 truncate min-w-0">
                      <input
                        type="checkbox"
                        :checked="selectedStagedPaths.has(file.path)"
                        @click.stop="toggleStagedSelect(file.path)"
                        :aria-label="t('workspace.changes.select.file', { name: file.path })"
                        class="w-3.5 h-3.5 rounded border-line accent-accent cursor-pointer shrink-0"
                      />
                      <component
                        :is="getFileIcon(file.path)"
                        class="w-3.5 h-3.5 shrink-0"
                        :class="selectedPath === file.path ? 'text-accent' : 'text-success'"
                      />
                      <div class="flex flex-col truncate">
                        <span class="truncate font-semibold" :class="selectedPath === file.path ? 'text-ink' : 'text-ink/90'">{{ splitFilePath(file.path).name }}</span>
                        <span v-if="splitFilePath(file.path).dir" class="text-[9px] text-ink-muted truncate font-mono">{{ splitFilePath(file.path).dir }}</span>
                      </div>
                    </div>

                    <div class="flex items-center gap-1.5 shrink-0 pl-1">
                      <button :aria-label="t('changes.lock.lfs.aria')"
                        v-if="file.isLfs"
                        @click.stop="emit('toggle-lfs-lock', file)"
                        class="p-1 rounded hover:bg-canvas text-warning transition cursor-pointer"
                        :title="t('changes.lock.lfs.aria')"
                      >
                        <Lock class="w-3 h-3" aria-hidden="true" />
                      </button>
                      <span
                        class="px-1.5 py-0.2 rounded text-[9px] font-bold"
                        :class="getStatusBadge(file.stagedStatus).bg"
                        :title="getStatusBadge(file.stagedStatus).title"
                      >
                        {{ getStatusBadge(file.stagedStatus).label }}
                      </span>
                      <button :aria-label="t('changes.copy.deeplink')"
                        @click.stop="copyFileDeepLink(file.path)"
                        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-canvas hover:text-accent text-ink-muted transition cursor-pointer"
                        :title="t('changes.copy.deeplink')"
                      >
                        <Link class="w-3 h-3" aria-hidden="true" />
                      </button>
                      <button :aria-label="t('changes.unstage.one')"
                        @click.stop="emit('unstage', file.path)"
                        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-canvas hover:text-warning text-ink-muted transition cursor-pointer"
                        :title="t('changes.unstage.one')"
                      >
                        <Minus class="w-3 h-3" aria-hidden="true" />
                      </button>
                    </div>
                  </div>

                  <!-- 已暂存分组渲染上限 -->
                  <button
                    v-if="hiddenStagedCount > 0"
                    @click="showMoreChanges('staged')"
                    class="mt-0.5 w-full px-2 py-1 rounded-lg border border-dashed border-line text-ink-muted hover:text-ink hover:bg-raised/50 text-[10px] font-semibold transition cursor-pointer flex items-center justify-center gap-1"
                  >
                    <ChevronDown class="w-3 h-3" aria-hidden="true" /> {{ t("changes.show.more", { count: hiddenStagedCount }) }}
                  </button>
                </div>
              </div>

              <!-- 3. 工作区未暂存列表 (Unstaged Changes Group) -->
              <div v-if="filteredUnstagedFiles.length > 0" class="flex flex-col">
                <div
                  data-testid="changes-group-header-unstaged"
                  @click="isUnstagedCollapsed = !isUnstagedCollapsed"
                  class="flex items-center justify-between px-2 py-1 rounded cursor-pointer hover:bg-surface text-ink-muted hover:text-ink text-[11px] font-bold select-none"
                >
                  <div class="flex items-center gap-1 text-warning">
                    <ChevronDown class="w-3 h-3 shrink-0 transition-transform" :class="isUnstagedCollapsed ? '-rotate-90' : ''" />
                    <span>{{ t("changes.group.unstaged") }}</span>
                    <span class="px-1.5 py-0.2 rounded-full bg-warning/20 text-warning text-[9px]">{{ filteredUnstagedFiles.length }}</span>
                  </div>
                  <div class="flex items-center gap-1">
                    <button
                      @click.stop="toggleAllUnstaged"
                      class="text-[10px] text-ink-muted hover:text-ink hover:underline p-0.5 rounded font-normal"
                      :title="t('workspace.changes.toggle.all')"
                    >
                      {{ selectedUnstagedPaths.size === filteredUnstagedFiles.length ? t("changes.unselect") : t("changes.select.all") }}
                    </button>
                    <button
                      @click.stop="emit('stage-all')"
                      class="hover:text-success p-0.5 rounded text-[10px] font-normal"
                      :title="t('workspace.changes.stage.all')"
                    >
                      {{ t("changes.stage.all.short") }}
                    </button>
                  </div>
                </div>

                <div v-if="!isUnstagedCollapsed" class="mt-0.5 space-y-0.5">
                  <div
                    v-for="file in renderedUnstagedFiles"
                    :key="file.path"
                    @click="emit('select-file', file)"
                    class="group flex items-center justify-between px-2 py-1.5 rounded-lg cursor-pointer transition"
                    :class="selectedPath === file.path ? 'bg-raised border-l-2 border-accent text-accent font-medium shadow-xs' : 'hover:bg-raised/50 text-ink-secondary hover:text-ink'"
                  >
                    <div class="flex items-center gap-2 truncate min-w-0">
                      <input
                        type="checkbox"
                        :checked="selectedUnstagedPaths.has(file.path)"
                        @click.stop="toggleUnstagedSelect(file.path)"
                        :aria-label="t('workspace.changes.select.file', { name: file.path })"
                        class="w-3.5 h-3.5 rounded border-line accent-accent cursor-pointer shrink-0"
                      />
                      <component
                        :is="getFileIcon(file.path)"
                        class="w-3.5 h-3.5 shrink-0"
                        :class="selectedPath === file.path ? 'text-accent' : 'text-warning'"
                      />
                      <div class="flex flex-col truncate">
                        <span class="truncate font-semibold" :class="selectedPath === file.path ? 'text-ink' : 'text-ink/90'">{{ splitFilePath(file.path).name }}</span>
                        <span v-if="splitFilePath(file.path).dir" class="text-[9px] text-ink-muted truncate font-mono">{{ splitFilePath(file.path).dir }}</span>
                      </div>
                    </div>

                    <div class="flex items-center gap-1 shrink-0 pl-1">
                      <button :aria-label="t('changes.lock.lfs.aria')"
                        v-if="file.isLfs"
                        @click.stop="emit('toggle-lfs-lock', file)"
                        class="p-1 rounded hover:bg-canvas text-warning transition cursor-pointer"
                        :title="t('changes.lock.lfs.aria')"
                      >
                        <Lock class="w-3 h-3" aria-hidden="true" />
                      </button>
                      <span
                        class="px-1.5 py-0.2 rounded text-[9px] font-bold"
                        :class="getStatusBadge(file.unstagedStatus).bg"
                        :title="getStatusBadge(file.unstagedStatus).title"
                      >
                        {{ getStatusBadge(file.unstagedStatus).label }}
                      </span>
                      <button :aria-label="t('changes.copy.deeplink')"
                        @click.stop="copyFileDeepLink(file.path)"
                        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-canvas hover:text-accent text-ink-muted transition cursor-pointer"
                        :title="t('changes.copy.deeplink')"
                      >
                        <Link class="w-3 h-3" aria-hidden="true" />
                      </button>
                      <button :aria-label="t('changes.stage.one')"
                        @click.stop="emit('stage', file.path)"
                        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-canvas hover:text-success text-ink-muted transition cursor-pointer"
                        :title="t('changes.stage.one')"
                      >
                        <Plus class="w-3 h-3" aria-hidden="true" />
                      </button>
                      <button :aria-label="t('changes.discard.one')"
                        @click.stop="emit('discard', file.path)"
                        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-danger/20 hover:text-danger text-ink-muted transition cursor-pointer"
                        :title="t('changes.discard.one')"
                      >
                        <Trash2 class="w-3 h-3" aria-hidden="true" />
                      </button>
                    </div>
                  </div>

                  <!-- 未暂存分组渲染上限 -->
                  <button
                    v-if="hiddenUnstagedCount > 0"
                    @click="showMoreChanges('unstaged')"
                    class="mt-0.5 w-full px-2 py-1 rounded-lg border border-dashed border-line text-ink-muted hover:text-ink hover:bg-raised/50 text-[10px] font-semibold transition cursor-pointer flex items-center justify-center gap-1"
                  >
                    <ChevronDown class="w-3 h-3" aria-hidden="true" /> {{ t("changes.show.more", { count: hiddenUnstagedCount }) }}
                  </button>
                </div>
              </div>

              <!-- Empty state when no changes match -->
              <div
                v-if="totalChangesCount > 0 && filteredConflictedFiles.length === 0 && filteredStagedFiles.length === 0 && filteredUnstagedFiles.length === 0"
                class="py-8 text-center text-xs text-ink-muted font-sans"
              >
                {{ t("changes.empty.filtered") }}
              </div>
              <div
                v-else-if="totalChangesCount === 0"
                class="py-8 px-3 text-center text-xs text-ink-muted font-sans flex flex-col items-center gap-3"
              >
                <div class="flex flex-col items-center gap-1.5">
                  <Check class="w-6 h-6 text-success/60" />
                  <span class="font-medium text-ink">{{ t("changes.empty.clean") }}</span>
                </div>

                <!-- Starter Template Card for new/empty workspace -->
                <div class="w-full p-3 rounded-xl border border-line bg-surface/40 flex flex-col gap-2 text-left">
                  <div class="flex items-center gap-1.5 text-xs font-bold text-ink">
                    <Sparkles class="w-3.5 h-3.5 text-accent" />
                    <span>{{ t("changes.starter.templates.title") }}</span>
                  </div>
                  <p class="text-[10px] text-ink-muted leading-relaxed">
                    {{ t("changes.starter.templates.desc") }}
                  </p>
                  <div class="flex flex-wrap gap-1.5 pt-1">
                    <button
                      v-for="tpl in ['unity', 'unreal', 'node', 'go']"
                      :key="tpl"
                      @click="handleApplyTemplate(tpl)"
                      class="px-2 py-1 rounded bg-raised hover:bg-hover text-ink text-[10px] font-semibold transition cursor-pointer border border-line uppercase"
                    >
                      + {{ tpl }}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Batch Action Toolbar (appears when any files are checked) -->
            <div
              v-if="selectedUnstagedPaths.size > 0 || selectedStagedPaths.size > 0"
              class="p-2.5 bg-raised/95 border-t border-line flex flex-wrap items-center justify-between gap-x-2 gap-y-1.5 text-xs shrink-0 backdrop-blur-sm shadow-md"
            >
              <div class="flex items-center gap-2 shrink-0">
                <span class="font-bold text-accent text-[11px]">
                  {{ t("changes.selected.count", { count: selectedUnstagedPaths.size + selectedStagedPaths.size }) }}
                </span>
                <button
                  @click="clearBatchSelection"
                  class="text-[10px] text-ink-muted hover:text-ink hover:underline cursor-pointer"
                >
                  {{ t("changes.clear") }}
                </button>
              </div>

              <div class="flex flex-wrap items-center gap-1.5">
                <button
                  v-if="selectedUnstagedPaths.size > 0"
                  @click="handleBatchStage"
                  class="px-2.5 py-1 rounded bg-success-strong hover:bg-success-dark text-white font-semibold text-[10px] transition cursor-pointer shadow-xs"
                >
                  {{ t("changes.action.stage", { count: selectedUnstagedPaths.size }) }}
                </button>
                <button
                  v-if="selectedStagedPaths.size > 0"
                  @click="handleBatchUnstage"
                  class="px-2.5 py-1 rounded bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink text-[10px] transition cursor-pointer"
                >
                  {{ t("changes.action.unstage", { count: selectedStagedPaths.size }) }}
                </button>
                <button
                  v-if="selectedUnstagedPaths.size > 0"
                  @click="openStashSelectionModal"
                  class="px-2.5 py-1 rounded bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink text-[10px] transition cursor-pointer"
                  :title="t('changes.action.stash.title')"
                >
                  {{ t("changes.action.stash", { count: selectedUnstagedPaths.size }) }}
                </button>
                <button
                  v-if="selectedUnstagedPaths.size > 0"
                  @click="handleBatchDiscard"
                  class="px-2 py-1 rounded hover:bg-danger/20 text-danger text-[10px] transition cursor-pointer"
                >
                  {{ t("changes.action.discard") }}
                </button>
              </div>
            </div>

            <!-- Batch stash modal: stash only the checked files -->
            <UiModal
              :open="showStashSelectionModal"
              :title="t('changes.modal.stash.title')"
              width="max-w-md"
              @update:open="onStashModalToggle"
            >
              <template #header>
                <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                  <Archive class="w-4 h-4 text-warning" />
                  {{ t("changes.modal.stash.heading") }}
                </h3>
              </template>

              <div class="space-y-3">
                <p class="text-xs text-ink-muted leading-relaxed">
                  {{ t("changes.modal.stash.desc", { count: selectedUnstagedPaths.size }) }}
                </p>

                <div class="flex flex-col gap-1.5">
                  <label class="text-xs text-ink-secondary font-medium">{{ t("changes.modal.stash.message") }}</label>
                  <input
                    v-model="stashSelectionMessage"
                    type="text"
                    :placeholder="t('changes.modal.stash.placeholder')"
                    @keydown.enter="handleStashSelection"
                    class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                  />
                </div>
              </div>

              <template #footer>
                <UiButton variant="secondary" @click="showStashSelectionModal = false">{{ t("common.cancel") }}</UiButton>
                <UiButton variant="warning" :disabled="isStashingSelection" @click="handleStashSelection">
                  {{ isStashingSelection ? t("changes.modal.stash.running") : t("changes.modal.stash.confirm") }}
                </UiButton>
              </template>
            </UiModal>
</template>
