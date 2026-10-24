<script setup lang="ts">
import { t } from "../../i18n";
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onUnmounted } from "vue";
import { GitCommit, Search, RefreshCw, Plus, Minus, Lock, ChevronDown, ChevronRight, Trash2, FileText, Image, Box, FileSpreadsheet, Code2, AlertTriangle, Check, Archive } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import type { FileStatus } from "../../types";

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
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
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

async function handleBatchDiscard() {
  if (!props.currentRepoPath || selectedUnstagedPaths.value.size === 0) return;
  const paths = Array.from(selectedUnstagedPaths.value);
  if (!(await confirmAction({ message: t("changes.confirm.discard.selected", { count: paths.length }), tone: "danger" }))) return;
  try {
    await gitService.batchDiscardFiles(props.currentRepoPath, paths);
    emit('notify', t('changes.toast.discarded.selected', { count: paths.length }));
    selectedUnstagedPaths.value = new Set();
    emit("refresh");
  } catch (err: any) {
    emit('notify', t('changes.toast.discard.selected.failed', { error: err?.message || err }));
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

const isConflictedCollapsed = ref(false);
const isStagedCollapsed = ref(false);
const isUnstagedCollapsed = ref(false);

const filteredConflictedFiles = computed(() => {
  if (!changesQuery.value) return props.conflictedFiles;
  const q = changesQuery.value.toLowerCase();
  return props.conflictedFiles.filter((p) => p.toLowerCase().includes(q));
});

const filteredStagedFiles = computed(() => {
  if (!changesQuery.value) return props.stagedFiles;
  const q = changesQuery.value.toLowerCase();
  return props.stagedFiles.filter((f) => f.path.toLowerCase().includes(q));
});

const filteredUnstagedFiles = computed(() => {
  if (!changesQuery.value) return props.unstagedFiles;
  const q = changesQuery.value.toLowerCase();
  return props.unstagedFiles.filter((f) => f.path.toLowerCase().includes(q));
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


onUnmounted(() => {
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
                    <Plus class="w-3 h-3 shrink-0" /> {{ t("changes.stage.all") }}
                  </button>
                  <button
                    v-if="stagedFiles.length > 0"
                    @click="emit('unstage-all')"
                    class="px-1.5 py-0.5 rounded hover:bg-raised text-ink-muted hover:text-ink text-[10px] font-semibold transition cursor-pointer flex items-center gap-0.5 whitespace-nowrap shrink-0"
                    :title="t('changes.unstage.all.title')"
                  >
                    <Minus class="w-3 h-3 shrink-0" /> {{ t("changes.unstage.all") }}
                  </button>
                  <button
                    v-if="totalChangesCount > 0"
                    @click="emit('discard-all')"
                    class="px-1.5 py-0.5 rounded hover:bg-danger/20 text-ink-muted hover:text-danger text-[10px] font-semibold transition cursor-pointer flex items-center gap-0.5 whitespace-nowrap shrink-0"
                    :title="t('changes.discard.all.title')"
                  >
                    <Trash2 class="w-3 h-3 shrink-0" /> {{ t("changes.discard.all") }}
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

              <!-- Keyboard shortcuts hint -->
              <div class="flex items-center justify-between text-[9px] text-ink-muted px-0.5 pt-0.5 select-none font-sans">
                <span><kbd class="px-1 py-0.2 rounded bg-surface border border-line font-mono text-[8px]">↑</kbd> <kbd class="px-1 py-0.2 rounded bg-surface border border-line font-mono text-[8px]">↓</kbd> {{ t("changes.hint.browse") }}</span>
                <span><kbd class="px-1 py-0.2 rounded bg-surface border border-line font-mono text-[8px]">Space</kbd> {{ t("changes.hint.toggle") }}</span>
                <span><kbd class="px-1 py-0.2 rounded bg-surface border border-line font-mono text-[8px]">⌘↵</kbd> {{ t("changes.hint.commit") }}</span>
              </div>
            </div>

            <!-- Scrollable Vertical Changes List -->
            <div class="flex-1 overflow-y-auto p-1.5 space-y-2 select-none text-[11px] font-mono">
              <!-- 1. 冲突文件列表 (Conflicted Files Group) -->
              <div v-if="filteredConflictedFiles.length > 0" class="flex flex-col">
                <div
                  @click="isConflictedCollapsed = !isConflictedCollapsed"
                  class="flex items-center justify-between px-2 py-1 rounded cursor-pointer hover:bg-danger/10 text-danger text-[11px] font-bold select-none"
                >
                  <div class="flex items-center gap-1">
                    <component :is="isConflictedCollapsed ? ChevronRight : ChevronDown" class="w-3 h-3" />
                    <span>{{ t("changes.group.conflicted") }}</span>
                    <span class="px-1.5 py-0.2 rounded-full bg-danger-strong text-white text-[9px]">{{ filteredConflictedFiles.length }}</span>
                  </div>
                </div>

                <div v-show="!isConflictedCollapsed" class="mt-0.5 space-y-0.5">
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
                    <ChevronDown class="w-3 h-3" /> {{ t("changes.show.more", { count: hiddenConflictedCount }) }}
                  </button>
                </div>
              </div>

              <!-- 2. 已暂存列表 (Staged Changes Group) -->
              <div v-if="filteredStagedFiles.length > 0" class="flex flex-col">
                <div
                  @click="isStagedCollapsed = !isStagedCollapsed"
                  class="flex items-center justify-between px-2 py-1 rounded cursor-pointer hover:bg-surface text-ink-muted hover:text-ink text-[11px] font-bold select-none"
                >
                  <div class="flex items-center gap-1 text-success">
                    <component :is="isStagedCollapsed ? ChevronRight : ChevronDown" class="w-3 h-3" />
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

                <div v-show="!isStagedCollapsed" class="mt-0.5 space-y-0.5">
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
                        <Lock class="w-3 h-3" />
                      </button>
                      <span
                        class="px-1.5 py-0.2 rounded text-[9px] font-bold"
                        :class="getStatusBadge(file.stagedStatus).bg"
                        :title="getStatusBadge(file.stagedStatus).title"
                      >
                        {{ getStatusBadge(file.stagedStatus).label }}
                      </span>
                      <button :aria-label="t('changes.unstage.one')"
                        @click.stop="emit('unstage', file.path)"
                        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-canvas hover:text-warning text-ink-muted transition cursor-pointer"
                        :title="t('changes.unstage.one')"
                      >
                        <Minus class="w-3 h-3" />
                      </button>
                    </div>
                  </div>

                  <!-- 已暂存分组渲染上限 -->
                  <button
                    v-if="hiddenStagedCount > 0"
                    @click="showMoreChanges('staged')"
                    class="mt-0.5 w-full px-2 py-1 rounded-lg border border-dashed border-line text-ink-muted hover:text-ink hover:bg-raised/50 text-[10px] font-semibold transition cursor-pointer flex items-center justify-center gap-1"
                  >
                    <ChevronDown class="w-3 h-3" /> {{ t("changes.show.more", { count: hiddenStagedCount }) }}
                  </button>
                </div>
              </div>

              <!-- 3. 工作区未暂存列表 (Unstaged Changes Group) -->
              <div v-if="filteredUnstagedFiles.length > 0" class="flex flex-col">
                <div
                  @click="isUnstagedCollapsed = !isUnstagedCollapsed"
                  class="flex items-center justify-between px-2 py-1 rounded cursor-pointer hover:bg-surface text-ink-muted hover:text-ink text-[11px] font-bold select-none"
                >
                  <div class="flex items-center gap-1 text-warning">
                    <component :is="isUnstagedCollapsed ? ChevronRight : ChevronDown" class="w-3 h-3" />
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

                <div v-show="!isUnstagedCollapsed" class="mt-0.5 space-y-0.5">
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
                        <Lock class="w-3 h-3" />
                      </button>
                      <span
                        class="px-1.5 py-0.2 rounded text-[9px] font-bold"
                        :class="getStatusBadge(file.unstagedStatus).bg"
                        :title="getStatusBadge(file.unstagedStatus).title"
                      >
                        {{ getStatusBadge(file.unstagedStatus).label }}
                      </span>
                      <button :aria-label="t('changes.stage.one')"
                        @click.stop="emit('stage', file.path)"
                        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-canvas hover:text-success text-ink-muted transition cursor-pointer"
                        :title="t('changes.stage.one')"
                      >
                        <Plus class="w-3 h-3" />
                      </button>
                      <button :aria-label="t('changes.discard.one')"
                        @click.stop="emit('discard', file.path)"
                        class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-danger/20 hover:text-danger text-ink-muted transition cursor-pointer"
                        :title="t('changes.discard.one')"
                      >
                        <Trash2 class="w-3 h-3" />
                      </button>
                    </div>
                  </div>

                  <!-- 未暂存分组渲染上限 -->
                  <button
                    v-if="hiddenUnstagedCount > 0"
                    @click="showMoreChanges('unstaged')"
                    class="mt-0.5 w-full px-2 py-1 rounded-lg border border-dashed border-line text-ink-muted hover:text-ink hover:bg-raised/50 text-[10px] font-semibold transition cursor-pointer flex items-center justify-center gap-1"
                  >
                    <ChevronDown class="w-3 h-3" /> {{ t("changes.show.more", { count: hiddenUnstagedCount }) }}
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
                class="py-12 text-center text-xs text-ink-muted font-sans flex flex-col items-center gap-2"
              >
                <Check class="w-6 h-6 text-success/60" />
                <span>{{ t("changes.empty.clean") }}</span>
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
