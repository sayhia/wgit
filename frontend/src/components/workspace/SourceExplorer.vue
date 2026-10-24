<script setup lang="ts">
import { t } from "../../i18n";
import { ref, computed, shallowRef, watch, onMounted, onUnmounted } from "vue";
import {
  FileCode,
  FileText,
  Image,
  Box,
  FileSpreadsheet,
  Code2,
  FolderTree,
  ListFilter,
  FolderOpen,
  Folder,
  RefreshCw,
  Search,
  ChevronDown,
  ChevronRight,
  Eye,
  ClipboardList,
  MoreHorizontal,
  EyeOff,
  Pencil,
  Trash2,
  FileX,
  ExternalLink,
  Check,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import GitignoreEditor from "../GitignoreEditor.vue";
import { isActionCustomized } from "../../utils/shortcuts";
import type { ProjectSourceFileItem, SourceFileDetail } from "../../types";

const props = defineProps<{
  repoPath: string;
  width: number;
  selectedPath: string | null;
  refreshNonce: number;
  sourceFileDetail: SourceFileDetail | null;
  resizing?: boolean;
}>();

const emit = defineEmits<{
  (e: "open-file", path: string): void;
  (e: "copy-file-list"): void;
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
}>();

// sourceFiles 只做整体替换（刷新时赋新数组），逐项深层响应式对 10 万文件是纯开销，因此使用 shallowRef。
const sourceFiles = shallowRef<ProjectSourceFileItem[]>([]);
const sourceSearch = ref("");
const isLoadingSource = ref(false);
const sourceDisplayMode = ref<"tree" | "flat">("tree");
// Tree expansion persists per repository so reopening a big project keeps context.
const EXPANDED_STORAGE_KEY = "wgit.sourceExpanded.v1";
const MAX_PERSISTED_EXPANDED = 2000;

function loadExpandedFolders(repoPath: string): Set<string> {
  if (!repoPath) return new Set();
  try {
    const raw = localStorage.getItem(EXPANDED_STORAGE_KEY);
    const map = raw ? JSON.parse(raw) : {};
    const list = Array.isArray(map?.[repoPath]) ? map[repoPath] : [];
    return new Set(
      list
        .filter((path: unknown): path is string => typeof path === "string")
        .slice(0, MAX_PERSISTED_EXPANDED)
    );
  } catch {
    return new Set();
  }
}

function persistExpandedFolders(repoPath: string, folders: Set<string>) {
  if (!repoPath) return;
  try {
    const raw = localStorage.getItem(EXPANDED_STORAGE_KEY);
    const map = raw ? JSON.parse(raw) : {};
    map[repoPath] = [...folders].slice(0, MAX_PERSISTED_EXPANDED);
    localStorage.setItem(EXPANDED_STORAGE_KEY, JSON.stringify(map));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

const expandedFolders = ref<Set<string>>(loadExpandedFolders(props.repoPath));

// Flat-mode directory group collapse also persists per repository.
const COLLAPSED_FLAT_KEY = "wgit.flatGroupsCollapsed.v1";

function loadCollapsedFlatGroups(repoPath: string): Set<string> {
  if (!repoPath) return new Set();
  try {
    const raw = localStorage.getItem(COLLAPSED_FLAT_KEY);
    const map = raw ? JSON.parse(raw) : {};
    const list = Array.isArray(map?.[repoPath]) ? map[repoPath] : [];
    return new Set(list.filter((name: unknown): name is string => typeof name === "string"));
  } catch {
    return new Set();
  }
}

function persistCollapsedFlatGroups(repoPath: string, groups: Set<string>) {
  if (!repoPath) return;
  try {
    const raw = localStorage.getItem(COLLAPSED_FLAT_KEY);
    const map = raw ? JSON.parse(raw) : {};
    map[repoPath] = [...groups];
    localStorage.setItem(COLLAPSED_FLAT_KEY, JSON.stringify(map));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

const collapsedFlatGroups = ref<Set<string>>(loadCollapsedFlatGroups(props.repoPath));

watch(collapsedFlatGroups, (groups) => persistCollapsedFlatGroups(props.repoPath, groups));
watch(
  () => props.repoPath,
  (repoPath) => {
    collapsedFlatGroups.value = loadCollapsedFlatGroups(repoPath);
  }
);

function toggleFlatGroup(title: string) {
  const next = new Set(collapsedFlatGroups.value);
  if (next.has(title)) next.delete(title);
  else next.add(title);
  collapsedFlatGroups.value = next;
}

function collapseAllFlatGroups() {
  const titles = flatDisplayGroups.value
    .map((group) => group.title)
    .filter((title): title is string => !!title);
  collapsedFlatGroups.value = new Set(titles);
}

function expandAllFlatGroups() {
  collapsedFlatGroups.value = new Set();
}

watch(expandedFolders, (folders) => persistExpandedFolders(props.repoPath, folders));
watch(
  () => props.repoPath,
  (repoPath) => {
    expandedFolders.value = loadExpandedFolders(repoPath);
  }
);

/** ⌘⇧E toggles expand-all / collapse-all while the workspace is visible. */
function toggleExpandAllTrees() {
  const willCollapse = isAnyFolderExpanded.value;
  if (willCollapse) collapseAllFolders();
  else expandAllFolders();
  emit("notify", willCollapse ? t("explorer.collapsed.all") : t("explorer.expanded.all"));
}

function handleShortcutExpandAllTrees() {
  toggleExpandAllTrees();
}

function handleExplorerKeydown(event: KeyboardEvent) {
  if (!event.metaKey || !event.shiftKey || event.key.toLowerCase() !== "e") return;
  if (isActionCustomized("expand-all-tree")) return;
  const target = event.target as HTMLElement | null;
  if (target && (["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName) || target.isContentEditable)) {
    return;
  }
  event.preventDefault();
  toggleExpandAllTrees();
}

onMounted(() => {
  window.addEventListener("keydown", handleExplorerKeydown);
  window.addEventListener("wgit:source-toggle-expand-all", handleShortcutExpandAllTrees);
});
onUnmounted(() => {
  window.removeEventListener("keydown", handleExplorerKeydown);
  window.removeEventListener("wgit:source-toggle-expand-all", handleShortcutExpandAllTrees);
});

// 源码搜索：输入框绑定 sourceSearch（即时回显），真正驱动过滤/建树的是防抖后的 sourceQuery。
// 10 万文件仓库下，逐字符同步过滤 + 重建全量目录树会阻塞主线程，因此这里统一做输入防抖。
const sourceQuery = ref("");
const SOURCE_SEARCH_DEBOUNCE_MS = 200;
const isSourceSearchDebouncing = ref(false);
let sourceSearchTimer: ReturnType<typeof setTimeout> | null = null;

// 过滤结果只依赖「防抖后的搜索词 + 文件列表」，树形 / 平铺 / 计数 / 空状态共用同一份结果
const filteredSourceFiles = computed(() => {
  if (!sourceQuery.value) return sourceFiles.value;
  const q = sourceQuery.value.toLowerCase();
  return sourceFiles.value.filter(
    (f) => f.path.toLowerCase().includes(q) || f.name.toLowerCase().includes(q)
  );
});

const isSourceSearchActive = computed(() => !!sourceQuery.value.trim());
// 搜索命中量超过该阈值时给出“仅渲染前 N 项”的显式提示
const SOURCE_MATCH_HINT_THRESHOLD = 5000;

watch(sourceSearch, (value) => {
  if (sourceSearchTimer) {
    clearTimeout(sourceSearchTimer);
    sourceSearchTimer = null;
  }
  // 清空搜索立即生效，避免残留上一次的过滤结果
  if (!value) {
    sourceQuery.value = "";
    isSourceSearchDebouncing.value = false;
    return;
  }
  isSourceSearchDebouncing.value = true;
  sourceSearchTimer = setTimeout(() => {
    sourceQuery.value = sourceSearch.value;
    isSourceSearchDebouncing.value = false;
    sourceSearchTimer = null;
  }, SOURCE_SEARCH_DEBOUNCE_MS);
});

export interface FileTreeNode {
  id: string;
  name: string;
  path: string;
  isDir: boolean;
  level: number;
  status: string; // "clean" | "modified" | "staged" | "untracked"
  fileCount: number;
  dirtyCount: number;
  children: FileTreeNode[];
}

// Hierarchical directory tree builder.
// 提取为函数以便复用：全量树（无搜索）与搜索结果树（仅命中文件）分开构建并各自缓存。
function buildSourceTree(files: ProjectSourceFileItem[]): FileTreeNode[] {
  const rootNodes: FileTreeNode[] = [];
  const dirMap = new Map<string, FileTreeNode>();

  function getOrCreateDir(dirPath: string): FileTreeNode {
    if (dirMap.has(dirPath)) {
      return dirMap.get(dirPath)!;
    }
    const parts = dirPath.split("/").filter(Boolean);
    const name = parts[parts.length - 1] ?? dirPath;
    const level = parts.length - 1;

    const dirNode: FileTreeNode = {
      id: dirPath,
      name,
      path: dirPath,
      isDir: true,
      level,
      status: "clean",
      fileCount: 0,
      dirtyCount: 0,
      children: [],
    };
    dirMap.set(dirPath, dirNode);

    if (parts.length === 1) {
      rootNodes.push(dirNode);
    } else {
      const parentPath = parts.slice(0, -1).join("/");
      const parentNode = getOrCreateDir(parentPath);
      parentNode.children.push(dirNode);
    }
    return dirNode;
  }

  for (const file of files) {
    const parts = file.path.split("/").filter(Boolean);
    const fileName = parts[parts.length - 1] ?? file.path;

    if (parts.length === 1) {
      rootNodes.push({
        id: file.path,
        name: fileName,
        path: file.path,
        isDir: false,
        level: 0,
        status: file.status || "clean",
        fileCount: 1,
        dirtyCount: file.status && file.status !== "clean" ? 1 : 0,
        children: [],
      });
    } else {
      const parentPath = parts.slice(0, -1).join("/");
      const parentNode = getOrCreateDir(parentPath);
      parentNode.children.push({
        id: file.path,
        name: fileName,
        path: file.path,
        isDir: false,
        level: parts.length - 1,
        status: file.status || "clean",
        fileCount: 1,
        dirtyCount: file.status && file.status !== "clean" ? 1 : 0,
        children: [],
      });
    }
  }

  function sortAndAggregate(nodes: FileTreeNode[]) {
    nodes.sort((a, b) => {
      if (a.isDir && !b.isDir) return -1;
      if (!a.isDir && b.isDir) return 1;
      return a.name.localeCompare(b.name);
    });

    for (const node of nodes) {
      if (node.isDir) {
        sortAndAggregate(node.children);
        node.fileCount = node.children.reduce((acc, c) => acc + c.fileCount, 0);
        node.dirtyCount = node.children.reduce((acc, c) => acc + c.dirtyCount, 0);
        if (node.dirtyCount > 0) {
          node.status = "modified";
        }
      }
    }
  }

  sortAndAggregate(rootNodes);
  return rootNodes;
}

// 全量基础树只依赖 sourceFiles：仓库加载/刷新后构建一次，搜索输入与展开状态都不会再触发 10 万级重建。
const baseSourceTree = computed<FileTreeNode[]>(() => buildSourceTree(sourceFiles.value));

// 搜索结果树仅用命中文件构建（命中集通常远小于全量），清空搜索时直接复用已缓存的基础树。
const filteredSourceTree = computed<FileTreeNode[]>(() => buildSourceTree(filteredSourceFiles.value));

const sourceTree = computed<FileTreeNode[]>(() =>
  isSourceSearchActive.value ? filteredSourceTree.value : baseSourceTree.value
);

// Flattened visible tree list (respecting expandedFolders: default collapsed)
const visibleTreeNodes = computed<FileTreeNode[]>(() => {
  const result: FileTreeNode[] = [];
  const isSearching = isSourceSearchActive.value;

  function traverse(nodes: FileTreeNode[]) {
    for (const node of nodes) {
      result.push(node);
      if (node.isDir) {
        const isExpanded = isSearching || expandedFolders.value.has(node.path);
        if (isExpanded) {
          traverse(node.children);
        }
      }
    }
  }

  traverse(sourceTree.value);
  return result;
});

const isAnyFolderExpanded = computed(() => expandedFolders.value.size > 0);

function isFolderExpanded(folderPath: string): boolean {
  return expandedFolders.value.has(folderPath);
}

// Toggle individual folder collapse/expand
function toggleFolder(folderPath: string) {
  const next = new Set(expandedFolders.value);
  if (next.has(folderPath)) {
    next.delete(folderPath);
  } else {
    next.add(folderPath);
  }
  expandedFolders.value = next;
}

// Expand all folders
function expandAllFolders() {
  const next = new Set<string>();
  function collect(nodes: FileTreeNode[]) {
    for (const n of nodes) {
      if (n.isDir) {
        next.add(n.path);
        collect(n.children);
      }
    }
  }
  collect(sourceTree.value);
  expandedFolders.value = next;
}

// Collapse all folders (default collapsed state)
function collapseAllFolders() {
  expandedFolders.value = new Set();
}

// Expand parent folders of a file so it stays visible in the tree
function expandParentsOf(filePath: string) {
  const parts = filePath.split("/").filter(Boolean);
  if (parts.length <= 1) return;
  const next = new Set(expandedFolders.value);
  let cur = "";
  for (let i = 0; i < parts.length - 1; i++) {
    cur = cur ? `${cur}/${parts[i]}` : parts[i]!;
    next.add(cur);
  }
  expandedFolders.value = next;
}

// Right pane tabs: 'diff' | 'source'

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

// ---- Large-repo display guards: cap rendered source-explorer rows (display only) ----
// 搜索/选中/展开等逻辑始终基于完整数据，此处仅限制单次渲染的 DOM 节点数量。
const SOURCE_RENDER_LIMIT = 1500;
const sourceRenderLimit = ref(SOURCE_RENDER_LIMIT);

const renderedTreeNodes = computed(() => visibleTreeNodes.value.slice(0, sourceRenderLimit.value));
const renderedFlatSourceFiles = computed(() => filteredSourceFiles.value.slice(0, sourceRenderLimit.value));

/** Flat mode groups files by their top-level directory; search results stay flat. */
const flatDisplayGroups = computed<
  { title: string | null; files: ProjectSourceFileItem[]; collapsed: boolean }[]
>(() => {
  const files = renderedFlatSourceFiles.value;
  if (!files.length) return [];
  if (isSourceSearchActive.value) return [{ title: null, files, collapsed: false }];
  const groups: { title: string | null; files: ProjectSourceFileItem[]; collapsed: boolean }[] = [];
  for (const file of files) {
    const parts = file.path.split("/").filter(Boolean);
    const title = parts.length > 1 ? `${parts[0]}/` : t("explorer.root");
    const last = groups[groups.length - 1];
    if (last && last.title === title) last.files.push(file);
    else groups.push({ title, files: [file], collapsed: collapsedFlatGroups.value.has(title) });
  }
  return groups;
});
const hiddenTreeCount = computed(() =>
  Math.max(0, visibleTreeNodes.value.length - renderedTreeNodes.value.length)
);
const hiddenFlatCount = computed(() =>
  Math.max(0, filteredSourceFiles.value.length - renderedFlatSourceFiles.value.length)
);

function showMoreSourceNodes() {
  sourceRenderLimit.value += SOURCE_RENDER_LIMIT;
}

// Load source files on mount or repo change
async function loadSourceFiles() {
  if (!props.repoPath) return;
  isLoadingSource.value = true;
  try {
    const list = await gitService.getProjectSourceFiles(props.repoPath);
    if (list) {
      sourceFiles.value = list;
    }
  } catch (err) {
    console.error("Failed to load source files:", err);
  } finally {
    isLoadingSource.value = false;
  }
}

// ---------- Working tree file/dir actions (right-click-ish menu) ----------
type SourceFileMenuState = {
  kind: "file" | "dir";
  path: string;
  status: string;
  x: number;
  y: number;
};

const fileMenu = ref<SourceFileMenuState | null>(null);
const fileActionBusy = ref("");
const renamePath = ref<string | null>(null);
const renameDraft = ref("");
const deleteConfirmPath = ref<string | null>(null);
const gitignorePatterns = ref<string[]>([]);
const showGitignoreEditor = ref(false);

/** .gitignore pattern that would cover the current menu target. */
const menuIgnorePattern = computed(() => {
  const menu = fileMenu.value;
  if (!menu) return "";
  return menu.kind === "dir" ? `/${menu.path}/` : `/${menu.path}`;
});

const menuIsIgnored = computed(
  () => !!fileMenu.value && gitignorePatterns.value.includes(menuIgnorePattern.value)
);

async function loadGitignorePatterns() {
  try {
    gitignorePatterns.value = (await gitService.getGitignorePatterns(props.repoPath)) || [];
  } catch {
    gitignorePatterns.value = [];
  }
}

function placeMenu(event: MouseEvent) {
  return {
    x: Math.max(8, Math.min(event.clientX, window.innerWidth - 232)),
    y: Math.max(8, Math.min(event.clientY, window.innerHeight - 300)),
  };
}

function openFileMenu(file: { path: string; status: string }, event: MouseEvent) {
  event.preventDefault();
  event.stopPropagation();
  fileMenu.value = { kind: "file", ...file, ...placeMenu(event) };
  renamePath.value = null;
  deleteConfirmPath.value = null;
  void loadGitignorePatterns();
}

function openDirMenu(node: { path: string }, event: MouseEvent) {
  event.preventDefault();
  event.stopPropagation();
  fileMenu.value = { kind: "dir", path: node.path, status: "", ...placeMenu(event) };
  renamePath.value = null;
  deleteConfirmPath.value = null;
  void loadGitignorePatterns();
}

async function revealMenuPath(path: string) {
  try {
    await gitService.revealFileInFinder(props.repoPath, path);
    emit("notify", t("explorer.toast.revealed", { path }));
  } catch (err: any) {
    emit("notify", t("explorer.toast.reveal.failed", { error: err?.message || err }));
  }
  closeFileMenu();
}

function closeFileMenu() {
  fileMenu.value = null;
  renamePath.value = null;
  deleteConfirmPath.value = null;
}

async function copySourcePath(path: string) {
  try {
    await navigator.clipboard.writeText(path);
    emit("notify", t("explorer.toast.path.copied", { path }));
  } catch {
    emit("notify", t("explorer.toast.copy.failed"));
  }
  closeFileMenu();
}

async function afterFileMutation(message: string) {
  emit("notify", message);
  await loadSourceFiles();
  emit("refresh");
}

async function addIgnorePattern(pattern: string) {
  if (!pattern || fileActionBusy.value) return;
  fileActionBusy.value = pattern;
  try {
    await gitService.addGitignorePattern(props.repoPath, pattern);
    await afterFileMutation(t("explorer.toast.ignored", { pattern }));
  } catch (err: any) {
    emit("notify", t("explorer.toast.ignore.failed", { error: err?.message || err }));
  } finally {
    fileActionBusy.value = "";
    closeFileMenu();
  }
}

async function removeIgnorePattern(pattern: string) {
  if (!pattern || fileActionBusy.value) return;
  fileActionBusy.value = pattern;
  try {
    await gitService.removeGitignorePattern(props.repoPath, pattern);
    await afterFileMutation(t("explorer.toast.ignore.removed", { pattern }));
  } catch (err: any) {
    emit("notify", t("explorer.toast.ignore.remove.failed", { error: err?.message || err }));
  } finally {
    fileActionBusy.value = "";
    closeFileMenu();
  }
}

async function untrackSourceFile(path: string) {
  if (fileActionBusy.value) return;
  fileActionBusy.value = path;
  try {
    await gitService.untrackRepoFile(props.repoPath, path);
    await afterFileMutation(t("explorer.toast.untracked", { path }));
  } catch (err: any) {
    emit("notify", t("explorer.toast.untrack.failed", { error: err?.message || err }));
  } finally {
    fileActionBusy.value = "";
    closeFileMenu();
  }
}

function startRenameSourceFile(path: string) {
  renamePath.value = path;
  renameDraft.value = path;
}

async function submitRenameSourceFile() {
  const oldPath = renamePath.value;
  const newPath = renameDraft.value.trim();
  if (!oldPath || !newPath || newPath === oldPath) {
    renamePath.value = null;
    return;
  }
  if (fileActionBusy.value) return;
  fileActionBusy.value = oldPath;
  try {
    await gitService.renameRepoFile(props.repoPath, oldPath, newPath);
    await afterFileMutation(t("explorer.toast.renamed", { from: oldPath, to: newPath }));
    closeFileMenu();
  } catch (err: any) {
    emit("notify", t("explorer.toast.rename.failed", { error: err?.message || err }));
  } finally {
    fileActionBusy.value = "";
  }
}

async function deleteSourceFile(path: string) {
  if (fileActionBusy.value) return;
  fileActionBusy.value = path;
  try {
    await gitService.deleteRepoFile(props.repoPath, path);
    await afterFileMutation(t("explorer.toast.deleted", { path }));
    closeFileMenu();
  } catch (err: any) {
    emit("notify", t("explorer.toast.delete.failed", { error: err?.message || err }));
  } finally {
    fileActionBusy.value = "";
  }
}

async function stageMenuDirectory(path: string) {
  if (fileActionBusy.value) return;
  fileActionBusy.value = path;
  try {
    await gitService.stageRepoPath(props.repoPath, path);
    await afterFileMutation(t("explorer.toast.dir.staged", { path }));
  } catch (err: any) {
    emit("notify", t("explorer.toast.dir.stage.failed", { error: err?.message || err }));
  } finally {
    fileActionBusy.value = "";
    closeFileMenu();
  }
}

async function unstageMenuDirectory(path: string) {
  if (fileActionBusy.value) return;
  fileActionBusy.value = path;
  try {
    await gitService.unstageRepoPath(props.repoPath, path);
    await afterFileMutation(t("explorer.toast.dir.unstaged", { path }));
  } catch (err: any) {
    emit("notify", t("explorer.toast.dir.unstage.failed", { error: err?.message || err }));
  } finally {
    fileActionBusy.value = "";
    closeFileMenu();
  }
}

// 深链定位时确保目标文件已包含在当前渲染上限内（否则左侧列表看不到选中高亮）
function ensureSourceFileRenderedInExplorer(path: string) {
  const idx =
    sourceDisplayMode.value === "tree"
      ? visibleTreeNodes.value.findIndex((node) => !node.isDir && node.path === path)
      : filteredSourceFiles.value.findIndex((file) => file.path === path);
  if (idx >= sourceRenderLimit.value) {
    sourceRenderLimit.value = Math.ceil((idx + 1) / SOURCE_RENDER_LIMIT) * SOURCE_RENDER_LIMIT;
  }
}

watch(() => props.repoPath, loadSourceFiles, { immediate: true });
watch(() => props.refreshNonce, loadSourceFiles);
watch(
  () => props.selectedPath,
  (path) => {
    if (!path) return;
    expandParentsOf(path);
    ensureSourceFileRenderedInExplorer(path);
  }
);
</script>

<template>
    <!-- LEFT PANE: 源码工程浏览器 (Source Explorer - 源码放左面) -->
    <!-- ======================================================== -->
    <section
      :style="{ width: `${width}px` }"
      class="border-r border-line bg-surface/30 backdrop-blur-sm flex flex-col shrink-0 overflow-hidden relative"
      :class="{ 'transition-[width] duration-200 ease-out': !resizing }"
    >
      <!-- Left Header: Title, Counts & Switcher -->
      <div class="px-2.5 py-2 border-b border-line flex flex-col gap-2 shrink-0 bg-surface/50">
        <div class="flex items-center justify-between gap-1 min-w-0">
          <div class="flex items-center gap-1.5 font-semibold text-xs tracking-wide text-ink shrink-0 whitespace-nowrap">
            <FileCode class="w-4 h-4 text-accent shrink-0" />
            <span class="whitespace-nowrap">{{ t("explorer.title") }}</span>
            <span class="text-[10px] text-ink-muted font-mono font-normal whitespace-nowrap">({{ filteredSourceFiles.length }})</span>
          </div>

          <div class="flex items-center gap-1 shrink-0">
            <!-- Mode Switcher: Tree vs Flat -->
            <div class="flex items-center bg-canvas p-0.5 rounded-lg border border-line shrink-0">
              <button
                @click="sourceDisplayMode = 'tree'"
                class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] transition cursor-pointer font-medium whitespace-nowrap shrink-0"
                :class="sourceDisplayMode === 'tree' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                :title="t('explorer.mode.tree.title')"
              >
                <FolderTree class="w-3 h-3 shrink-0" />
                <span class="whitespace-nowrap" :class="width < 290 ? 'hidden' : 'inline'">{{ t("explorer.mode.tree") }}</span>
              </button>
              <button
                @click="sourceDisplayMode = 'flat'"
                class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] transition cursor-pointer font-medium whitespace-nowrap shrink-0"
                :class="sourceDisplayMode === 'flat' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
                :title="t('explorer.mode.flat.title')"
              >
                <ListFilter class="w-3 h-3 shrink-0" />
                <span class="whitespace-nowrap" :class="width < 290 ? 'hidden' : 'inline'">{{ t("explorer.mode.flat") }}</span>
              </button>
            </div>

            <!-- Expand / Collapse All (in Tree mode) -->
            <template v-if="sourceDisplayMode === 'tree'">
              <button :aria-label="t('explorer.expand.all.aria')"
                v-if="!isAnyFolderExpanded"
                @click="expandAllFolders"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
                :title="t('explorer.expand.all.aria')"
              >
                <FolderOpen class="w-3.5 h-3.5 shrink-0" />
              </button>
              <button :aria-label="t('explorer.collapse.all.aria')"
                v-else
                @click="collapseAllFolders"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
                :title="t('explorer.collapse.all.aria')"
              >
                <Folder class="w-3.5 h-3.5 shrink-0" />
              </button>
            </template>

            <!-- Flat mode: collapse / expand all directory groups -->
            <template v-else>
              <button :aria-label="t('explorer.collapse.groups')"
                @click="collapseAllFlatGroups"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
                :title="t('explorer.collapse.groups')"
              >
                <ChevronRight class="w-3.5 h-3.5 shrink-0" />
              </button>
              <button :aria-label="t('explorer.expand.groups')"
                @click="expandAllFlatGroups"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
                :title="t('explorer.expand.groups')"
              >
                <ChevronDown class="w-3.5 h-3.5 shrink-0" />
              </button>
            </template>

            <!-- Copy tracked file list -->
            <button :aria-label="t('explorer.copy.paths.aria')"
              @click="emit('copy-file-list')"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
              :title="t('explorer.copy.paths.aria')"
            >
              <ClipboardList class="w-3.5 h-3.5 shrink-0" />
            </button>
            <!-- Refresh button -->
            <button :aria-label="t('explorer.rescan.aria')"
              @click="loadSourceFiles"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
              :title="t('explorer.rescan.aria')"
            >
              <RefreshCw class="w-3.5 h-3.5 shrink-0" :class="{ 'animate-spin': isLoadingSource }" />
            </button>
          </div>
        </div>

        <!-- Search Bar -->
        <div class="relative">
          <input
            v-model="sourceSearch"
            type="text"
            :aria-label="t('explorer.search.aria')"
            :placeholder="t('explorer.search.placeholder')"
            class="w-full pl-7 py-1.5 rounded-lg bg-canvas/70 border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono transition"
            :class="isSourceSearchDebouncing ? 'pr-20' : 'pr-6'"
          />
          <Search v-if="!isSourceSearchDebouncing" class="w-3.5 h-3.5 text-ink-muted absolute left-2.5 top-2.5 pointer-events-none" />
          <RefreshCw v-else class="w-3.5 h-3.5 text-accent absolute left-2.5 top-2.5 pointer-events-none animate-spin" />
          <span
            v-if="isSourceSearchDebouncing"
            class="absolute right-7 top-2 text-[9px] leading-none text-accent font-sans animate-pulse pointer-events-none select-none whitespace-nowrap"
          >
            {{ t("explorer.searching") }}
          </span>
          <button
            v-if="sourceSearch"
            @click="sourceSearch = ''"
            class="absolute right-2 top-2 text-[10px] text-ink-muted hover:text-ink font-bold cursor-pointer"
            :title="t('explorer.search.clear')"
          >
            ✕
          </button>
        </div>

        <!-- 10 万级仓库搜索命中量提示：只渲染前 N 项，避免用户误以为结果只有这么多 -->
        <div
          v-if="isSourceSearchActive && filteredSourceFiles.length > SOURCE_MATCH_HINT_THRESHOLD"
          class="flex items-center gap-1 px-0.5 text-[9px] leading-tight text-accent/90 font-sans select-none"
        >
          <Search class="w-2.5 h-2.5 shrink-0" />
          <span>{{ t("explorer.match.limit", { count: filteredSourceFiles.length, limit: SOURCE_RENDER_LIMIT }) }}</span>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- SOURCE EXPLORER: 1. 目录树模式 (TREE MODE)    -->
      <!-- ============================================== -->
      <div
        v-if="sourceDisplayMode === 'tree'"
        class="flex-1 overflow-y-auto p-1.5 space-y-0.5 font-mono text-[11px]"
      >
        <template v-for="node in renderedTreeNodes" :key="node.id">
          <!-- Folder Node -->
          <div
            v-if="node.isDir"
            @click="toggleFolder(node.path)"
            class="group flex items-center justify-between py-1 px-1.5 rounded-lg cursor-pointer hover:bg-raised/50 text-ink select-none transition"
            :style="{ paddingLeft: `${node.level * 14 + 6}px` }"
          >
            <div class="flex items-center gap-1.5 truncate">
              <component
                :is="isFolderExpanded(node.path) ? ChevronDown : ChevronRight"
                class="w-3 h-3 text-ink-muted shrink-0 transition-transform"
              />
              <component
                :is="isFolderExpanded(node.path) ? FolderOpen : Folder"
                class="w-3.5 h-3.5 shrink-0"
                :class="isFolderExpanded(node.path) ? 'text-accent' : 'text-ink-muted'"
              />
              <span class="truncate font-semibold text-ink">{{ node.name }}</span>
            </div>

            <!-- Folder Status Badges -->
            <div class="flex items-center gap-1.5 shrink-0 pr-1">
              <span
                v-if="node.dirtyCount > 0"
                class="px-1.5 py-0.2 rounded-full bg-warning/20 text-warning text-[9px] font-bold"
                :title="t('explorer.dir.dirty', { count: node.dirtyCount })"
              >
                {{ node.dirtyCount }}
              </span>
              <span class="text-[9px] text-ink-muted font-sans">
                {{ node.fileCount }}
              </span>
              <button :aria-label="t('explorer.dir.actions')"
                @click.stop="openDirMenu({ path: node.path }, $event)"
                class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-canvas text-ink-muted hover:text-ink transition"
                :title="t('explorer.dir.actions')"
              >
                <MoreHorizontal class="w-3 h-3" />
              </button>
            </div>

          </div>

          <!-- File Node -->
          <div
            v-else
            @click="emit('open-file', node.path)"
            @dblclick="emit('open-file', node.path)"
            class="group flex items-center justify-between py-1 px-1.5 rounded-lg cursor-pointer transition select-none"
            :class="selectedPath === node.path ? 'bg-raised text-accent font-medium shadow-xs' : 'hover:bg-raised/50 text-ink-secondary hover:text-ink'"
            :style="{ paddingLeft: `${node.level * 14 + 20}px` }"
          >
            <div class="flex items-center gap-1.5 truncate">
              <component
                :is="getFileIcon(node.path)"
                class="w-3.5 h-3.5 shrink-0"
                :class="node.status !== 'clean' ? 'text-accent' : 'text-ink-muted group-hover:text-ink'"
              />
              <span class="truncate">{{ node.name }}</span>
            </div>

            <!-- File Status & Actions -->
            <div class="flex items-center gap-1 shrink-0 pr-1">
              <button :aria-label="t('explorer.preview.source')"
                @click.stop="emit('open-file', node.path)"
                class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-canvas text-ink-muted hover:text-accent transition"
                :title="t('explorer.preview.source')"
              >
                <Eye class="w-3 h-3" />
              </button>
              <button :aria-label="t('explorer.file.actions')"
                @click.stop="openFileMenu({ path: node.path, status: node.status }, $event)"
                class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-canvas text-ink-muted hover:text-ink transition"
                :title="t('explorer.file.actions')"
              >
                <MoreHorizontal class="w-3 h-3" />
              </button>

              <span
                v-if="node.status === 'modified'"
                class="px-1.5 py-0.2 rounded bg-warning/20 text-warning text-[9px] font-bold"
                :title="t('explorer.status.modified')"
              >
                M
              </span>
              <span
                v-else-if="node.status === 'staged'"
                class="px-1.5 py-0.2 rounded bg-success/20 text-success text-[9px] font-bold"
                :title="t('explorer.status.staged')"
              >
                +
              </span>
              <span
                v-else-if="node.status === 'untracked'"
                class="px-1.5 py-0.2 rounded bg-cyan/20 text-cyan text-[9px] font-bold"
                :title="t('explorer.status.untracked')"
              >
                ?
              </span>
            </div>

          </div>
        </template>

        <div v-if="visibleTreeNodes.length === 0" class="py-12 text-center text-xs text-ink-muted font-sans flex flex-col items-center gap-2">
          <RefreshCw v-if="isLoadingSource && sourceFiles.length === 0" class="w-4 h-4 animate-spin text-accent" />
          <span>{{ isLoadingSource && sourceFiles.length === 0 ? t("explorer.scanning") : t("explorer.empty") }}</span>
        </div>

        <!-- 大仓渲染上限提示（树形模式） -->
        <div v-if="hiddenTreeCount > 0" class="mt-2 px-2 pb-3 text-center font-sans">
          <p class="text-[10px] text-ink-muted leading-relaxed mb-1.5">
            {{ t("explorer.tree.limit", { shown: renderedTreeNodes.length, total: visibleTreeNodes.length }) }}
          </p>
          <button
            @click="showMoreSourceNodes"
            class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[10px] text-accent font-semibold transition cursor-pointer inline-flex items-center gap-1"
          >
            <ChevronDown class="w-3 h-3" /> {{ t("explorer.show.more", { count: hiddenTreeCount }) }}
          </button>
        </div>
      </div>

      <!-- ============================================== -->
      <!-- SOURCE EXPLORER: 2. 平铺列表模式 (FLAT MODE)   -->
      <!-- ============================================== -->
      <div
        v-else
        class="flex-1 overflow-y-auto p-1.5 space-y-0.5 font-mono text-[11px]"
      >
        <template v-for="group in flatDisplayGroups" :key="group.title ?? '__search__'">
          <button
            v-if="group.title"
            @click="toggleFlatGroup(group.title)"
            class="sticky top-0 z-[5] -mx-1.5 w-[calc(100%+0.75rem)] px-3 py-1 mt-1 bg-surface/95 backdrop-blur-sm border-y border-line/50 flex items-center gap-1.5 text-[10px] font-sans font-semibold text-ink-muted select-none cursor-pointer hover:text-ink transition text-left"
            :title="group.collapsed ? t('explorer.group.expand') : t('explorer.group.collapse')"
          >
            <component :is="group.collapsed ? ChevronRight : ChevronDown" class="w-3 h-3 shrink-0" />
            <Folder class="w-3 h-3 shrink-0" />
            <span class="truncate">{{ group.title }}</span>
            <span class="ml-auto shrink-0 font-mono text-[9px] text-ink-muted">{{ group.files.length }}</span>
          </button>
          <template v-if="!group.collapsed">
          <div
            v-for="file in group.files"
            :key="file.path"
            @click="emit('open-file', file.path)"
          @dblclick="emit('open-file', file.path)"
          class="group flex items-center justify-between px-2.5 py-1.5 rounded-lg cursor-pointer transition select-none"
          :class="selectedPath === file.path ? 'bg-raised text-accent font-medium shadow-sm' : 'hover:bg-raised/50 text-ink-secondary hover:text-ink'"
        >
          <div class="flex items-center gap-2 truncate">
            <component
              :is="getFileIcon(file.path)"
              class="w-3.5 h-3.5 shrink-0"
              :class="file.status !== 'clean' ? 'text-accent' : 'text-ink-muted group-hover:text-ink'"
            />
            <span class="truncate">{{ file.path }}</span>
          </div>

          <!-- Status & Actions -->
          <div class="flex items-center gap-1.5 shrink-0">
            <button :aria-label="t('explorer.preview.source')"
              @click.stop="emit('open-file', file.path)"
              class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-canvas text-ink-muted hover:text-accent transition"
              :title="t('explorer.preview.source')"
            >
              <Eye class="w-3 h-3" />
            </button>
            <button :aria-label="t('explorer.file.actions')"
              @click.stop="openFileMenu({ path: file.path, status: file.status }, $event)"
              class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-canvas text-ink-muted hover:text-ink transition"
              :title="t('explorer.file.actions')"
            >
              <MoreHorizontal class="w-3 h-3" />
            </button>

            <span
              v-if="file.status === 'modified'"
              class="px-1.5 py-0.2 rounded bg-warning/20 text-warning text-[9px] font-bold"
              :title="t('explorer.status.modified')"
            >
              M
            </span>
            <span
              v-else-if="file.status === 'staged'"
              class="px-1.5 py-0.2 rounded bg-success/20 text-success text-[9px] font-bold"
              :title="t('explorer.status.staged')"
            >
              +
            </span>
            <span
              v-else-if="file.status === 'untracked'"
              class="px-1.5 py-0.2 rounded bg-cyan/20 text-cyan text-[9px] font-bold"
              :title="t('explorer.status.untracked')"
            >
              ?
            </span>
          </div>
          </div>
          </template>
        </template>

        <div v-if="filteredSourceFiles.length === 0" class="py-12 text-center text-xs text-ink-muted font-sans flex flex-col items-center gap-2">
          <RefreshCw v-if="isLoadingSource && sourceFiles.length === 0" class="w-4 h-4 animate-spin text-accent" />
          <span>{{ isLoadingSource && sourceFiles.length === 0 ? t("explorer.scanning") : t("explorer.empty") }}</span>
        </div>

        <!-- 大仓渲染上限提示（平铺模式） -->
        <div v-if="hiddenFlatCount > 0" class="mt-2 px-2 pb-3 text-center font-sans">
          <p class="text-[10px] text-ink-muted leading-relaxed mb-1.5">
            {{ t("explorer.flat.limit", { shown: renderedFlatSourceFiles.length, total: filteredSourceFiles.length }) }}
          </p>
          <button
            @click="showMoreSourceNodes"
            class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[10px] text-accent font-semibold transition cursor-pointer inline-flex items-center gap-1"
          >
            <ChevronDown class="w-3 h-3" /> {{ t("explorer.show.more", { count: hiddenFlatCount }) }}
          </button>
        </div>
      </div>

      <!-- Bottom Source File Metadata Mini-Panel -->
      <div v-if="sourceFileDetail" class="p-3 border-t border-line bg-canvas/60 text-[11px] font-sans flex flex-col gap-1.5 shrink-0">
        <div class="flex items-center justify-between">
          <span class="font-semibold text-ink truncate max-w-[180px]">{{ sourceFileDetail.name }}</span>
          <span class="text-ink-muted font-mono text-[10px]">{{ sourceFileDetail.sizeFormatted }}</span>
        </div>
        <div class="flex items-center justify-between text-ink-muted text-[10px]">
          <span>{{ t("explorer.detail.line", { count: sourceFileDetail.lineCount }) }} · {{ sourceFileDetail.language }}</span>
          <button
            @click="emit('open-file', sourceFileDetail.path)"
            class="text-accent hover:underline flex items-center gap-0.5 font-medium cursor-pointer"
          >
            <Eye class="w-3 h-3" /> {{ t("explorer.browse") }}
          </button>
        </div>
      </div>
    <!-- File / Directory Actions Menu (fixed-position popover) -->
    <template v-if="fileMenu">
      <div class="fixed inset-0 z-40" @click="closeFileMenu"></div>
      <div
        data-glass
        class="fixed z-50 w-56 rounded-xl border glass glass-panel glass-popover overflow-hidden text-[11px] select-none"
        :style="{ left: `${fileMenu.x}px`, top: `${fileMenu.y}px` }"
        @click.stop
      >
        <div class="px-2.5 py-1.5 border-b border-line/60 font-mono text-[9px] text-ink-muted truncate" :title="fileMenu.path">
          {{ fileMenu.path }}{{ fileMenu.kind === "dir" ? "/" : "" }}
        </div>

        <!-- Inline rename form (files only) -->
        <div v-if="fileMenu.kind === 'file' && renamePath === fileMenu.path" class="p-2 space-y-1.5">
          <input
            v-model="renameDraft"
            type="text"
            spellcheck="false"
            class="w-full px-2 py-1 rounded-md bg-canvas border border-line text-[11px] font-mono text-ink focus:outline-none focus:border-accent"
            :placeholder="t('explorer.rename.placeholder')"
            @keydown.enter.prevent="submitRenameSourceFile"
            @keydown.esc.prevent="renamePath = null"
          />
          <div class="flex items-center gap-1.5">
            <button
              @click.stop="submitRenameSourceFile"
              :disabled="!!fileActionBusy || !renameDraft.trim()"
              class="flex-1 px-2 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast font-semibold transition cursor-pointer disabled:opacity-40"
            >
              {{ fileActionBusy ? t("explorer.processing") : t("explorer.rename.confirm") }}
            </button>
            <button
              @click.stop="renamePath = null"
              class="px-2 py-1 rounded-md border border-line bg-surface hover:bg-raised text-ink-muted transition cursor-pointer"
            >
              {{ t("common.cancel") }}
            </button>
          </div>
        </div>

        <template v-else>
          <button
            v-if="fileMenu.kind === 'file'"
            @click.stop="emit('open-file', fileMenu.path); closeFileMenu()"
            class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-ink-secondary hover:bg-raised hover:text-ink transition cursor-pointer"
          >
            <Eye class="w-3 h-3" /> {{ t("explorer.menu.open") }}
          </button>
          <button
            @click.stop="copySourcePath(fileMenu.path)"
            class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-ink-secondary hover:bg-raised hover:text-ink transition cursor-pointer"
          >
            <ClipboardList class="w-3 h-3" /> {{ t("explorer.menu.copy.path") }}
          </button>
          <button
            @click.stop="revealMenuPath(fileMenu.path)"
            class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-ink-secondary hover:bg-raised hover:text-ink transition cursor-pointer"
          >
            <ExternalLink class="w-3 h-3" /> {{ t("explorer.menu.reveal") }}
          </button>

          <template v-if="fileMenu.kind === 'dir'">
            <div class="h-px bg-line/60 mx-2"></div>
            <button
              @click.stop="stageMenuDirectory(fileMenu.path)"
              :disabled="!!fileActionBusy"
              class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-success hover:bg-success/10 transition cursor-pointer disabled:opacity-40"
            >
              <FolderOpen class="w-3 h-3" /> {{ t("explorer.menu.stage.dir") }}
            </button>
            <button
              @click.stop="unstageMenuDirectory(fileMenu.path)"
              :disabled="!!fileActionBusy"
              class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-ink-secondary hover:bg-raised hover:text-ink transition cursor-pointer disabled:opacity-40"
            >
              <RefreshCw class="w-3 h-3" /> {{ t("explorer.menu.unstage.dir") }}
            </button>
          </template>

          <div class="h-px bg-line/60 mx-2"></div>

          <button
            v-if="!menuIsIgnored"
            @click.stop="addIgnorePattern(menuIgnorePattern)"
            :disabled="!!fileActionBusy"
            class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-ink-secondary hover:bg-raised hover:text-ink transition cursor-pointer disabled:opacity-40"
          >
            <FileX class="w-3 h-3" /> {{ t("explorer.menu.ignore") }}
          </button>
          <button
            v-else
            @click.stop="removeIgnorePattern(menuIgnorePattern)"
            :disabled="!!fileActionBusy"
            class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-warning hover:bg-warning/10 transition cursor-pointer disabled:opacity-40"
            :title="t('explorer.menu.ignore.remove.title', { pattern: menuIgnorePattern })"
          >
            <Check class="w-3 h-3" /> {{ t("explorer.menu.ignore.remove") }}
          </button>

          <button
            @click.stop="closeFileMenu(); showGitignoreEditor = true"
            class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-ink-secondary hover:bg-raised hover:text-ink transition cursor-pointer"
          >
            <FileX class="w-3 h-3" /> {{ t("explorer.menu.ignore.manage") }}
          </button>

          <template v-if="fileMenu.kind === 'file'">
            <div class="h-px bg-line/60 mx-2"></div>
            <button
              v-if="fileMenu.status !== 'untracked'"
              @click.stop="untrackSourceFile(fileMenu.path)"
              class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-ink-secondary hover:bg-raised hover:text-ink transition cursor-pointer"
              :title="t('explorer.menu.untrack.title')"
            >
              <EyeOff class="w-3 h-3" /> {{ t("explorer.menu.untrack") }}
            </button>
            <button
              @click.stop="startRenameSourceFile(fileMenu.path)"
              class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-ink-secondary hover:bg-raised hover:text-ink transition cursor-pointer"
            >
              <Pencil class="w-3 h-3" /> {{ t("explorer.menu.rename") }}
            </button>
            <button
              @click.stop="deleteConfirmPath = fileMenu.path"
              class="w-full flex items-center gap-2 px-2.5 py-1.5 text-left text-danger hover:bg-danger/10 transition cursor-pointer"
              :title="t('explorer.menu.delete.title')"
            >
              <Trash2 class="w-3 h-3" /> {{ t("explorer.menu.delete") }}
            </button>
            <div v-if="deleteConfirmPath === fileMenu.path" class="p-2 border-t border-line/60 space-y-1.5">
              <p class="text-[9px] text-danger leading-relaxed">{{ t("explorer.menu.delete.confirm") }}</p>
              <div class="flex items-center gap-1.5">
                <button
                  @click.stop="deleteSourceFile(fileMenu.path)"
                  :disabled="!!fileActionBusy"
                  class="flex-1 px-2 py-1 rounded-md bg-danger-strong hover:bg-danger-strong-hover text-white font-semibold transition cursor-pointer disabled:opacity-40"
                >
                  {{ fileActionBusy ? t("explorer.processing") : t("explorer.delete.confirm") }}
                </button>
                <button
                  @click.stop="deleteConfirmPath = null"
                  class="px-2 py-1 rounded-md border border-line bg-surface hover:bg-raised text-ink-muted transition cursor-pointer"
                >
                  {{ t("common.cancel") }}
                </button>
              </div>
            </div>
          </template>
        </template>
      </div>
    </template>


    <GitignoreEditor
      :is-open="showGitignoreEditor"
      :repo-path="repoPath"
      @close="showGitignoreEditor = false"
      @notify="emit('notify', $event)"
      @refresh="emit('refresh')"
    />

    </section>
</template>
