<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from "vue";
import { useModalA11y } from "../composables/useModalA11y";
import { t } from "../i18n";
import type { Component } from "vue";
import {
  Search,
  FileDiff,
  GitGraph,
  GitBranch,
  Archive,
  Wrench,
  RotateCw,
  ArrowUpCircle,
  ArrowDownCircle,
  Plus,
  Minus,
  Trash2,
  RotateCcw,
  Play,
  Globe,
  HardDrive,
  Settings,
  FolderOpen,
  FolderPlus,
  Home,
  GitMerge,
  Filter,
  Boxes,
  FolderTree,
  Network,
  PieChart,
  Activity,
  GitCompare,
  ClipboardCheck,
  LayoutDashboard,
  Rocket,
  GitPullRequest,
  FolderGit2,
  Users,
  Gamepad2,
  Palette,
  User,
  ShieldCheck,
  Keyboard,
  FolderSearch,
  StickyNote,
  HeartPulse,
  Flame,
  FileArchive,
  Lock,
  Database,
  Zap,
  Images,
  Cpu,
  FileSearch,
  ClipboardList,
  Gauge,
  Package,
  Clock,
  Webhook,
  Star,
  ChevronUp,
  ChevronDown,
  ChevronRight,
  FileWarning,
  Layers,
  Terminal,
} from "lucide-vue-next";

const props = defineProps<{
  isOpen: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "action", actionId: string): void;
}>();

const { containerRef: dialogRef } = useModalA11y(() => props.isOpen, () => emit("close"));

/** 命令面板条目：普通命令走 action/emit，跨视图命令直接派发 window 事件。 */
interface PaletteCommand {
  id: string;
  title: string;
  category: string;
  icon: Component;
  /** 由 App.vue / 目标视图消费的既有 action id（缺省等于 id）。 */
  action?: string;
  /** 大型项目 / 仓库级功能通过 CustomEvent 跳转，避免 App 继续膨胀。 */
  event?: { name: string; detail?: Record<string, unknown> };
}

const commands: PaletteCommand[] = [
  // ---- 导航 ----
  { id: "goto-workspace", title: "cmd.goto-workspace", category: "nav", icon: FileDiff },
  { id: "goto-history", title: "cmd.goto-history", category: "nav", icon: GitGraph },
  { id: "goto-branches", title: "cmd.goto-branches", category: "nav", icon: GitBranch },
  { id: "goto-stashes", title: "cmd.goto-stashes", category: "nav", icon: Archive },
  { id: "goto-review", title: "cmd.goto-review", category: "nav", icon: ClipboardCheck },
  { id: "goto-pr", title: "cmd.goto-pr", category: "nav", icon: GitPullRequest },
  { id: "goto-tools", title: "cmd.goto-tools", category: "nav", icon: Wrench },
  { id: "goto-settings", title: "cmd.goto-settings", category: "nav", icon: Settings },

  // ---- 搜索 ----
  {
    id: "quick-open-files",
    title: "cmd.quick-open-files",
    category: "search",
    icon: Search,
    event: { name: "wgit:quick-open", detail: { mode: "files" } },
  },
  {
    id: "quick-open-code",
    title: "cmd.quick-open-code",
    category: "search",
    icon: Search,
    event: { name: "wgit:quick-open", detail: { mode: "code" } },
  },

  // ---- 仓库（欢迎页 / 克隆 / 初始化） ----
  {
    id: "repo-open-dialog",
    title: "cmd.repo-open-dialog",
    category: "repo",
    icon: FolderOpen,
    action: "open-dialog",
  },
  {
    id: "repo-clone",
    title: "cmd.repo-clone",
    category: "repo",
    icon: Globe,
    event: { name: "wgit:onboarding", detail: { mode: "clone" } },
  },
  {
    id: "repo-init",
    title: "cmd.repo-init",
    category: "repo",
    icon: FolderPlus,
    event: { name: "wgit:onboarding", detail: { mode: "init" } },
  },
  {
    id: "repo-welcome",
    title: "cmd.repo-welcome",
    category: "repo",
    icon: Home,
    event: { name: "wgit:onboarding", detail: { mode: "welcome" } },
  },
  {
    id: "repo-dashboard",
    title: "cmd.repo-dashboard",
    category: "repo",
    icon: LayoutDashboard,
    action: "toggle-repo-dashboard",
  },
  {
    id: "tools-release",
    title: "cmd.tools-release",
    category: "repo",
    icon: Rocket,
    event: { name: "wgit:tools-tab", detail: { tab: "release" } },
  },
  {
    id: "repo-scan",
    title: "cmd.repo-scan",
    category: "repo",
    icon: FolderSearch,
    event: { name: "wgit:onboarding", detail: { mode: "scan" } },
  },

  // ---- Git 核心 ----
  { id: "sync-fetch", title: "cmd.sync-fetch", category: "git", icon: RotateCw },
  { id: "sync-pull", title: "cmd.sync-pull", category: "git", icon: ArrowDownCircle },
  { id: "sync-pull-options", title: "cmd.sync-pull-options", category: "git", icon: ArrowDownCircle, action: "sync-pull-options" },
  { id: "git-rebase", title: "cmd.git-rebase", category: "git", icon: GitPullRequest, action: "git-rebase" },
  { id: "sync-push", title: "cmd.sync-push", category: "git", icon: ArrowUpCircle },
  { id: "sync-push-options", title: "cmd.sync-push-options", category: "git", icon: ArrowUpCircle, action: "sync-push-options" },
  { id: "sync-fetch-options", title: "cmd.sync-fetch-options", category: "git", icon: RotateCw, action: "sync-fetch-options" },

  // ---- 工作区操作 ----
  { id: "stage-all", title: "cmd.stage-all", category: "workspace", icon: Plus },
  { id: "unstage-all", title: "cmd.unstage-all", category: "workspace", icon: Minus },
  { id: "discard-all", title: "cmd.discard-all", category: "danger", icon: Trash2 },

  // ---- 储藏操作 ----
  { id: "stash-save", title: "cmd.stash-save", category: "stash", icon: Archive },
  { id: "stash-pop", title: "cmd.stash-pop", category: "stash", icon: Play },

  // ---- 工作流 ----
  {
    id: "layout-standard",
    title: "cmd.layout-standard",
    category: "workspace",
    icon: LayoutDashboard,
    event: { name: "wgit:workspace-layout", detail: { preset: "standard" } },
  },
  {
    id: "layout-focus-source",
    title: "cmd.layout-focus-source",
    category: "workspace",
    icon: FolderTree,
    event: { name: "wgit:workspace-layout", detail: { preset: "focus-source" } },
  },
  {
    id: "layout-save-custom",
    title: "cmd.layout-save-custom",
    category: "workspace",
    icon: Plus,
    event: { name: "wgit:workspace-layout", detail: { preset: "save-custom" } },
  },
  {
    id: "layout-apply-custom",
    title: "cmd.layout-apply-custom",
    category: "workspace",
    icon: Play,
    event: { name: "wgit:workspace-layout", detail: { preset: "custom" } },
  },
  {
    id: "layout-focus-diff",
    title: "cmd.layout-focus-diff",
    category: "workspace",
    icon: GitCompare,
    event: { name: "wgit:workspace-layout", detail: { preset: "focus-diff" } },
  },
  {
    id: "history-rebase",
    title: "cmd.history-rebase",
    category: "workflow",
    icon: GitMerge,
    event: { name: "wgit:history-rebase", detail: {} },
  },
  {
    id: "history-filter",
    title: "cmd.history-filter",
    category: "workflow",
    icon: Filter,
    action: "goto-history",
  },
  {
    id: "compare-refs",
    title: "cmd.compare-refs",
    category: "workflow",
    icon: GitCompare,
    event: { name: "wgit:branches-tab", detail: { tab: "compare" } },
  },
  {
    id: "history-notes",
    title: "cmd.history-notes",
    category: "workflow",
    icon: StickyNote,
    event: { name: "wgit:history-notes", detail: {} },
  },

  // ---- 大型项目工具导航（ToolsView 对应 tab） ----
  {
    id: "tools-submodules",
    title: "cmd.tools-submodules",
    category: "project",
    icon: Boxes,
    event: { name: "wgit:tools-tab", detail: { tab: "submodules" } },
  },
  {
    id: "tools-sparse",
    title: "cmd.tools-sparse",
    category: "project",
    icon: FolderTree,
    event: { name: "wgit:tools-tab", detail: { tab: "sparse" } },
  },
  {
    id: "tools-lfs",
    title: "cmd.tools-lfs",
    category: "project",
    icon: HardDrive,
    event: { name: "wgit:tools-tab", detail: { tab: "lfs" } },
  },
  {
    id: "tools-remotes",
    title: "cmd.tools-remotes",
    category: "project",
    icon: Network,
    event: { name: "wgit:tools-tab", detail: { tab: "remotes" } },
  },
  {
    id: "tools-disk",
    title: "cmd.tools-disk",
    category: "project",
    icon: PieChart,
    event: { name: "wgit:tools-tab", detail: { tab: "disk" } },
  },
  {
    id: "tools-maintenance",
    title: "cmd.tools-maintenance",
    category: "project",
    icon: Activity,
    event: { name: "wgit:tools-tab", detail: { tab: "maintenance" } },
  },
  {
    id: "tools-health",
    title: "cmd.tools-health",
    category: "project",
    icon: HeartPulse,
    event: { name: "wgit:tools-tab", detail: { tab: "maintenance", section: "health" } },
  },
  {
    id: "tools-terminal",
    title: "cmd.tools-terminal",
    category: "project",
    icon: Terminal,
    event: { name: "wgit:tools-tab", detail: { tab: "terminal" } },
  },
  {
    id: "tools-object-store",
    title: "cmd.tools-object-store",
    category: "project",
    icon: Database,
    event: { name: "wgit:tools-tab", detail: { tab: "maintenance", section: "object-store" } },
  },
  {
    id: "tools-staged-large-lfs",
    title: "cmd.tools-staged-large-lfs",
    category: "lfs",
    icon: FileWarning,
    event: { name: "wgit:tools-tab", detail: { tab: "clean" } },
  },
  {
    id: "branches-range-diff",
    title: "cmd.branches-range-diff",
    category: "workflow",
    icon: Layers,
    event: { name: "wgit:branches-tab", detail: { tab: "compare" } },
  },
  {
    id: "tools-fsck",
    title: "cmd.tools-fsck",
    category: "project",
    icon: ShieldCheck,
    event: { name: "wgit:tools-tab", detail: { tab: "maintenance", section: "fsck" } },
  },
  {
    id: "tools-archive",
    title: "cmd.tools-archive",
    category: "project",
    icon: FileArchive,
    event: { name: "wgit:tools-tab", detail: { tab: "maintenance", section: "archive" } },
  },
  {
    id: "tools-hotfix",
    title: "cmd.tools-hotfix",
    category: "project",
    icon: Flame,
    event: { name: "wgit:tools-tab", detail: { tab: "maintenance", section: "hotfix" } },
  },
  {
    id: "tools-hooks",
    title: "cmd.tools-hooks",
    category: "project",
    icon: Webhook,
    event: { name: "wgit:tools-tab", detail: { tab: "maintenance", section: "hooks" } },
  },
  {
    id: "reflog-open",
    title: "cmd.reflog-open",
    category: "project",
    icon: RotateCcw,
    event: { name: "wgit:tools-tab", detail: { tab: "reflog" } },
  },
  {
    id: "clean-open",
    title: "cmd.clean-open",
    category: "project",
    icon: Trash2,
    event: { name: "wgit:tools-tab", detail: { tab: "clean" } },
  },
  {
    id: "tools-bisect",
    title: "cmd.tools-bisect",
    category: "project",
    icon: GitCompare,
    event: { name: "wgit:tools-tab", detail: { tab: "bisect" } },
  },
  {
    id: "tools-worktrees",
    title: "cmd.tools-worktrees",
    category: "project",
    icon: FolderGit2,
    event: { name: "wgit:tools-tab", detail: { tab: "worktrees" } },
  },
  {
    id: "tools-studio",
    title: "cmd.tools-studio",
    category: "project",
    icon: Users,
    event: { name: "wgit:tools-tab", detail: { tab: "studio" } },
  },
  {
    id: "tools-engine",
    title: "cmd.tools-engine",
    category: "project",
    icon: Gamepad2,
    event: { name: "wgit:tools-tab", detail: { tab: "engine" } },
  },

  // ---- Git LFS ----
  {
    id: "lfs-patterns",
    title: "cmd.lfs-patterns",
    category: "lfs",
    icon: HardDrive,
    event: { name: "wgit:tools-tab", detail: { tab: "lfs", section: "lfs-patterns" } },
  },
  {
    id: "lfs-locks",
    title: "cmd.lfs-locks",
    category: "lfs",
    icon: Lock,
    event: { name: "wgit:tools-tab", detail: { tab: "lfs", section: "lfs-locks" } },
  },
  {
    id: "lfs-cache",
    title: "cmd.lfs-cache",
    category: "lfs",
    icon: Database,
    event: { name: "wgit:tools-tab", detail: { tab: "lfs", section: "lfs-cache" } },
  },
  {
    id: "lfs-tuning",
    title: "cmd.lfs-tuning",
    category: "lfs",
    icon: Zap,
    event: { name: "wgit:tools-tab", detail: { tab: "lfs", section: "lfs-tuning" } },
  },
  {
    id: "lfs-revisions",
    title: "cmd.lfs-revisions",
    category: "lfs",
    icon: RotateCcw,
    event: { name: "wgit:tools-tab", detail: { tab: "lfs", section: "lfs-revisions" } },
  },
  {
    id: "lfs-lock-activity",
    title: "cmd.lfs-lock-activity",
    category: "lfs",
    icon: ClipboardList,
    event: { name: "wgit:tools-tab", detail: { tab: "lfs", section: "lfs-lock-activity" } },
  },
  {
    id: "lfs-lockable",
    title: "cmd.lfs-lockable",
    category: "lfs",
    icon: ShieldCheck,
    event: { name: "wgit:tools-tab", detail: { tab: "lfs", section: "lfs-lockable" } },
  },

  // ---- 引擎与资产 ----
  {
    id: "engine-detect",
    title: "cmd.engine-detect",
    category: "engine",
    icon: Gamepad2,
    event: { name: "wgit:tools-tab", detail: { tab: "engine", section: "engine-detect" } },
  },
  {
    id: "engine-asset",
    title: "cmd.engine-asset",
    category: "engine",
    icon: FileSearch,
    event: { name: "wgit:tools-tab", detail: { tab: "engine", section: "engine-asset" } },
  },
  {
    id: "engine-unity-meta",
    title: "cmd.engine-unity-meta",
    category: "engine",
    icon: Cpu,
    event: { name: "wgit:tools-tab", detail: { tab: "engine", section: "engine-unity-meta" } },
  },
  {
    id: "engine-unity-merge",
    title: "cmd.engine-unity-merge",
    category: "engine",
    icon: GitMerge,
    event: { name: "wgit:tools-tab", detail: { tab: "engine", section: "engine-unity-merge" } },
  },
  {
    id: "engine-texture",
    title: "cmd.engine-texture",
    category: "engine",
    icon: Images,
    event: { name: "wgit:tools-tab", detail: { tab: "engine", section: "engine-texture" } },
  },
  {
    id: "engine-runtime",
    title: "cmd.engine-runtime",
    category: "engine",
    icon: Zap,
    event: { name: "wgit:tools-tab", detail: { tab: "engine", section: "engine-runtime" } },
  },

  // ---- Studio 协作 ----
  {
    id: "studio-team",
    title: "cmd.studio-team",
    category: "studio",
    icon: Users,
    event: { name: "wgit:tools-tab", detail: { tab: "studio", section: "studio-team" } },
  },
  {
    id: "studio-audits",
    title: "cmd.studio-audits",
    category: "studio",
    icon: ShieldCheck,
    event: { name: "wgit:tools-tab", detail: { tab: "studio", section: "studio-audits" } },
  },
  {
    id: "studio-logs",
    title: "cmd.studio-logs",
    category: "studio",
    icon: ClipboardList,
    event: { name: "wgit:tools-tab", detail: { tab: "studio", section: "studio-logs" } },
  },
  {
    id: "studio-custom-actions",
    title: "cmd.studio-custom-actions",
    category: "studio",
    icon: Zap,
    event: { name: "wgit:tools-tab", detail: { tab: "studio", section: "studio-custom-actions" } },
  },
  {
    id: "studio-compliance",
    title: "cmd.studio-compliance",
    category: "studio",
    icon: Lock,
    event: { name: "wgit:tools-tab", detail: { tab: "studio", section: "studio-compliance" } },
  },

  // ---- 体积与扫描 ----
  {
    id: "disk-metrics",
    title: "cmd.disk-metrics",
    category: "size",
    icon: PieChart,
    event: { name: "wgit:tools-tab", detail: { tab: "disk", section: "disk-metrics" } },
  },
  {
    id: "disk-large-files",
    title: "cmd.disk-large-files",
    category: "size",
    icon: FileSearch,
    event: { name: "wgit:tools-tab", detail: { tab: "disk", section: "disk-large-files" } },
  },
  {
    id: "disk-history-blobs",
    title: "cmd.disk-history-blobs",
    category: "size",
    icon: Database,
    event: { name: "wgit:tools-tab", detail: { tab: "disk", section: "disk-history-blobs" } },
  },
  {
    id: "disk-package-budget",
    title: "cmd.disk-package-budget",
    category: "size",
    icon: Package,
    event: { name: "wgit:tools-tab", detail: { tab: "disk", section: "disk-package-budget" } },
  },

  // ---- 子模块与工作树 ----
  {
    id: "submodule-list",
    title: "cmd.submodule-list",
    category: "submodule",
    icon: Boxes,
    event: { name: "wgit:tools-tab", detail: { tab: "submodules", section: "submodule-list" } },
  },
  {
    id: "subtree-list",
    title: "cmd.subtree-list",
    category: "submodule",
    icon: FolderTree,
    event: { name: "wgit:tools-tab", detail: { tab: "submodules", section: "subtree-list" } },
  },
  {
    id: "remotes-panel",
    title: "cmd.remotes-panel",
    category: "submodule",
    icon: Network,
    event: { name: "wgit:tools-tab", detail: { tab: "remotes", section: "remotes" } },
  },
  {
    id: "worktrees-panel",
    title: "cmd.worktrees-panel",
    category: "submodule",
    icon: FolderGit2,
    event: { name: "wgit:tools-tab", detail: { tab: "worktrees", section: "worktrees" } },
  },
  {
    id: "sparse-folders",
    title: "cmd.sparse-folders",
    category: "submodule",
    icon: Filter,
    event: { name: "wgit:tools-tab", detail: { tab: "sparse", section: "sparse-folders" } },
  },

  // ---- 设置分区直达 ----
  {
    id: "settings-appearance",
    title: "cmd.settings-appearance",
    category: "settings",
    icon: Palette,
    event: { name: "wgit:settings-section", detail: { section: "appearance" } },
  },
  {
    id: "settings-identity",
    title: "cmd.settings-identity",
    category: "settings",
    icon: User,
    event: { name: "wgit:settings-section", detail: { section: "identity" } },
  },
  {
    id: "settings-signing",
    title: "cmd.settings-signing",
    category: "settings",
    icon: ShieldCheck,
    event: { name: "wgit:settings-section", detail: { section: "signing" } },
  },
  {
    id: "settings-proxy",
    title: "cmd.settings-proxy",
    category: "settings",
    icon: Globe,
    event: { name: "wgit:settings-section", detail: { section: "proxy" } },
  },
  {
    id: "settings-tools",
    title: "cmd.settings-tools",
    category: "settings",
    icon: GitCompare,
    event: { name: "wgit:settings-section", detail: { section: "tools" } },
  },
  {
    id: "settings-config",
    title: "cmd.settings-config",
    category: "settings",
    icon: Settings,
    event: { name: "wgit:settings-section", detail: { section: "config" } },
  },
  {
    id: "settings-shortcuts",
    title: "cmd.settings-shortcuts",
    category: "settings",
    icon: Keyboard,
    event: { name: "wgit:settings-section", detail: { section: "shortcuts" } },
  },
];

const searchQuery = ref("");
const searchInput = ref<HTMLInputElement | null>(null);
const listRef = ref<HTMLElement | null>(null);
const selectedIndex = ref(0);

// ---- Recently used commands (MRU, persisted) ----
/**
 * Commands store i18n keys, not translated text.
 *
 * `commands` is a module-level array, so calling `t()` while building it would
 * freeze the labels in whatever language was active at import time. Resolving
 * lazily also keeps the persisted category-collapse list stable across a
 * language switch, because the stored value is the category key, not its label.
 */
function titleOf(cmd: PaletteCommand): string {
  return t(cmd.title);
}

function categoryLabel(category: string): string {
  return t(`cmd.cat.${category}`);
}

const RECENT_KEY = "wgit.palette.recent.v1";
const MAX_RECENT = 8;

function loadRecentIds(): string[] {
  try {
    const raw = localStorage.getItem(RECENT_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed)
      ? parsed.filter((id): id is string => typeof id === "string").slice(0, MAX_RECENT)
      : [];
  } catch {
    return [];
  }
}

const recentIds = ref<string[]>(loadRecentIds());

function rememberCommand(cmd: PaletteCommand) {
  recentIds.value = [cmd.id, ...recentIds.value.filter((id) => id !== cmd.id)].slice(0, MAX_RECENT);
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(recentIds.value));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

function commandById(id: string): PaletteCommand | undefined {
  return commands.find((c) => c.id === id);
}

// ---- Pinned commands (persisted) ----
const PIN_KEY = "wgit.palette.pinned.v1";
const MAX_PINNED = 6;

function loadPinnedIds(): string[] {
  try {
    const raw = localStorage.getItem(PIN_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed)
      ? parsed.filter((id): id is string => typeof id === "string").slice(0, MAX_PINNED)
      : [];
  } catch {
    return [];
  }
}

const pinnedIds = ref<string[]>(loadPinnedIds());

function persistPinned() {
  try {
    localStorage.setItem(PIN_KEY, JSON.stringify(pinnedIds.value));
  } catch {
    /* ignore */
  }
}

function isPinned(id: string): boolean {
  return pinnedIds.value.includes(id);
}

/** Move a pinned command up/down inside the 收藏 section (persisted). */
function movePinned(id: string, delta: number) {
  const index = pinnedIds.value.indexOf(id);
  const target = index + delta;
  if (index < 0 || target < 0 || target >= pinnedIds.value.length) return;
  const next = [...pinnedIds.value];
  const [item] = next.splice(index, 1);
  next.splice(target, 0, item!);
  pinnedIds.value = next;
  persistPinned();
  selectedIndex.value = target;
  void nextTick(scrollSelectedIntoView);
}

const draggingPinId = ref<string | null>(null);
const dragOverPinId = ref<string | null>(null);

function startPinDrag(id: string, event: DragEvent) {
  draggingPinId.value = id;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", id);
  }
}

function dragOverPin(id: string) {
  if (draggingPinId.value && draggingPinId.value !== id) dragOverPinId.value = id;
}

function dropPin(targetId: string, event: DragEvent) {
  event.preventDefault();
  const sourceId = draggingPinId.value || event.dataTransfer?.getData("text/plain") || "";
  draggingPinId.value = null;
  dragOverPinId.value = null;
  if (!sourceId || sourceId === targetId) return;
  const from = pinnedIds.value.indexOf(sourceId);
  const to = pinnedIds.value.indexOf(targetId);
  if (from < 0 || to < 0) return;
  const next = [...pinnedIds.value];
  next.splice(from, 1);
  next.splice(to, 0, sourceId);
  pinnedIds.value = next;
  persistPinned();
  selectedIndex.value = to;
  void nextTick(scrollSelectedIntoView);
}

function endPinDrag() {
  draggingPinId.value = null;
  dragOverPinId.value = null;
}

const allPaletteCategories = computed(() => [...new Set(commands.map((c) => c.category))]);

const allCategoriesCollapsed = computed(
  () =>
    allPaletteCategories.value.length > 0 &&
    allPaletteCategories.value.every((category) => isCategoryCollapsed(category))
);

function toggleAllCategories() {
  collapsedCategories.value = allCategoriesCollapsed.value
    ? []
    : [...allPaletteCategories.value];
  persistCollapsed();
  selectedIndex.value = 0;
  void nextTick(scrollSelectedIntoView);
}

function togglePin(cmd: PaletteCommand) {
  if (isPinned(cmd.id)) {
    pinnedIds.value = pinnedIds.value.filter((id) => id !== cmd.id);
  } else {
    pinnedIds.value = [cmd.id, ...pinnedIds.value].slice(0, MAX_PINNED);
  }
  persistPinned();
}

// ---- Collapsible categories (persisted) ----
const COLLAPSE_KEY = "wgit.palette.collapsed.v1";

function loadCollapsedCategories(): string[] {
  try {
    const raw = localStorage.getItem(COLLAPSE_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed)
      ? parsed.filter((c): c is string => typeof c === "string")
      : [];
  } catch {
    return [];
  }
}

const collapsedCategories = ref<string[]>(loadCollapsedCategories());

function persistCollapsed() {
  try {
    localStorage.setItem(COLLAPSE_KEY, JSON.stringify(collapsedCategories.value));
  } catch {
    /* ignore */
  }
}

function isCategoryCollapsed(category: string): boolean {
  return collapsedCategories.value.includes(category);
}

function toggleCategory(category: string) {
  if (isCategoryCollapsed(category)) {
    collapsedCategories.value = collapsedCategories.value.filter((c) => c !== category);
  } else {
    collapsedCategories.value = [...collapsedCategories.value, category];
  }
  persistCollapsed();
  selectedIndex.value = 0;
  void nextTick(scrollSelectedIntoView);
}

function clearRecents() {
  recentIds.value = [];
  try {
    localStorage.removeItem(RECENT_KEY);
  } catch {
    /* ignore */
  }
  selectedIndex.value = 0;
}

const isSearching = computed(() => !!searchQuery.value.trim());

/** Search results grouped by category with ranking: pinned → recent → prefix match →
 *  title match → category-only match. Categories are ordered by their best hit. */
const searchGroups = computed<{ category: string; cmds: PaletteCommand[] }[]>(() => {
  if (!isSearching.value) return [];
  const q = searchQuery.value.trim().toLowerCase();
  const scores = new Map<string, number>();
  for (const cmd of commands) {
    const title = titleOf(cmd).toLowerCase();
    const category = categoryLabel(cmd.category).toLowerCase();
    const titleMatch = title.includes(q);
    const categoryMatch = category.includes(q);
    if (!titleMatch && !categoryMatch) continue;
    let score = 0;
    if (isPinned(cmd.id)) score -= 100;
    else if (recentIds.value.includes(cmd.id)) score -= 50;
    if (title.startsWith(q)) score -= 20;
    else if (titleMatch) score -= 10;
    scores.set(cmd.id, score);
  }
  const categoryOrder = [...new Set(commands.map((c) => c.category))];
  const groups = categoryOrder
    .map((category) => {
      const cmds = commands
        .filter((c) => c.category === category && scores.has(c.id))
        .sort(
          (a, b) =>
            scores.get(a.id)! - scores.get(b.id)! ||
            commands.indexOf(a) - commands.indexOf(b)
        );
      const best = cmds.length ? Math.min(...cmds.map((c) => scores.get(c.id)!)) : 0;
      return { category, cmds, best };
    })
    .filter((g) => g.cmds.length > 0);
  groups.sort(
    (a, b) => a.best - b.best || categoryOrder.indexOf(a.category) - categoryOrder.indexOf(b.category)
  );
  return groups.map((g) => ({ category: g.category, cmds: g.cmds }));
});

const searchFlat = computed(() => searchGroups.value.flatMap((group) => group.cmds));

/** Flat order used by keyboard navigation (pinned → recent → remaining categories). */
const filteredCommands = computed<PaletteCommand[]>(() => {
  if (isSearching.value) return searchFlat.value;
  const pinned = pinnedIds.value
    .map((id) => commandById(id))
    .filter((c): c is PaletteCommand => !!c);
  const pinnedSet = new Set(pinned.map((c) => c.id));
  const recents = recentIds.value
    .map((id) => commandById(id))
    .filter((c): c is PaletteCommand => !!c && !pinnedSet.has(c.id));
  const recentSet = new Set(recents.map((c) => c.id));
  const rest = commands.filter(
    (c) => !pinnedSet.has(c.id) && !recentSet.has(c.id) && !isCategoryCollapsed(c.category)
  );
  return [...pinned, ...recents, ...rest];
});

type PaletteSection = {
  category: string;
  items: { cmd: PaletteCommand; index: number }[];
  recent?: boolean;
  pinned?: boolean;
  collapsed?: boolean;
  fullCount?: number;
};

/** 按分类分组成带标题的 section；index 仍是扁平序号，保证键盘导航不断。 */
const commandSections = computed<PaletteSection[]>(() => {
  const sections: PaletteSection[] = [];
  if (isSearching.value) {
    // Search results: ranked category groups, no pinned/recent/collapse sections.
    let searchIndex = 0;
    for (const group of searchGroups.value) {
      sections.push({
        category: group.category,
        items: group.cmds.map((cmd) => ({ cmd, index: searchIndex++ })),
      });
    }
    return sections;
  }

  const pinned = pinnedIds.value
    .map((id) => commandById(id))
    .filter((c): c is PaletteCommand => !!c);
  const pinnedSet = new Set(pinned.map((c) => c.id));
  const recents = recentIds.value
    .map((id) => commandById(id))
    .filter((c): c is PaletteCommand => !!c && !pinnedSet.has(c.id));
  const recentSet = new Set(recents.map((c) => c.id));

  let index = 0;
  if (pinned.length > 0) {
    sections.push({
      category: "pinned",
      pinned: true,
      items: pinned.map((cmd) => ({ cmd, index: index++ })),
    });
  }
  if (recents.length > 0) {
    sections.push({
      category: "recent",
      recent: true,
      items: recents.map((cmd) => ({ cmd, index: index++ })),
    });
  }

  const rest = commands.filter((c) => !pinnedSet.has(c.id) && !recentSet.has(c.id));
  const categoryOrder: string[] = [];
  const grouped = new Map<string, PaletteCommand[]>();
  for (const cmd of rest) {
    if (!grouped.has(cmd.category)) {
      grouped.set(cmd.category, []);
      categoryOrder.push(cmd.category);
    }
    grouped.get(cmd.category)!.push(cmd);
  }
  for (const category of categoryOrder) {
    const cmds = grouped.get(category)!;
    if (isCategoryCollapsed(category)) {
      sections.push({ category, items: [], collapsed: true, fullCount: cmds.length });
    } else {
      sections.push({
        category,
        items: cmds.map((cmd) => ({ cmd, index: index++ })),
      });
    }
  }
  return sections;
});

/** Split a label into plain/highlighted segments for the active search query. */
function titleParts(title: string): Array<{ text: string; hit: boolean }> {
  const query = searchQuery.value.trim();
  if (!query) return [{ text: title, hit: false }];
  const haystack = title.toLowerCase();
  const needle = query.toLowerCase();
  const parts: Array<{ text: string; hit: boolean }> = [];
  let cursor = 0;
  while (cursor < title.length) {
    const match = haystack.indexOf(needle, cursor);
    if (match < 0) {
      parts.push({ text: title.slice(cursor), hit: false });
      break;
    }
    if (match > cursor) parts.push({ text: title.slice(cursor, match), hit: false });
    parts.push({ text: title.slice(match, match + needle.length), hit: true });
    cursor = match + needle.length;
  }
  return parts.length > 0 ? parts : [{ text: title, hit: false }];
}

function scrollSelectedIntoView() {
  const el = listRef.value?.querySelector<HTMLElement>(`[data-idx="${selectedIndex.value}"]`);
  el?.scrollIntoView({ block: "nearest" });
}

function executeCommand(cmd: PaletteCommand) {
  rememberCommand(cmd);
  if (cmd.event) {
    // 跨视图跳转（QuickOpen / 引导页 / 工具箱 tab / 交互式变基）。
    window.dispatchEvent(new CustomEvent(cmd.event.name, { detail: cmd.event.detail }));
    emit("close");
    return;
  }

  emit("action", cmd.action ?? cmd.id);
  emit("close");
}

function moveSelection(delta: number) {
  const len = filteredCommands.value.length;
  if (len === 0) return;
  selectedIndex.value = (selectedIndex.value + delta + len) % len;
  void nextTick(scrollSelectedIntoView);
}

function handleKeydown(e: KeyboardEvent) {
  if (!props.isOpen) return;
  if (e.isComposing) return;

  if (e.key === "ArrowDown") {
    e.preventDefault();
    moveSelection(1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    moveSelection(-1);
  } else if (e.key === "Enter") {
    e.preventDefault();
    const cmd = filteredCommands.value[selectedIndex.value];
    if (cmd) executeCommand(cmd);
  } else if (e.key === "Escape") {
    emit("close");
  }
}

// 输入即过滤：选中项回到首条并滚回顶部。
watch(searchQuery, () => {
  selectedIndex.value = 0;
  void nextTick(scrollSelectedIntoView);
});

// 每次打开都重置搜索与选中，并聚焦输入框。
watch(
  () => props.isOpen,
  (open) => {
    if (!open) return;
    searchQuery.value = "";
    selectedIndex.value = 0;
    void nextTick(() => {
      searchInput.value?.focus();
      scrollSelectedIntoView();
    });
  }
);

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleKeydown);
});
</script>

<template>
  <div
    v-if="isOpen"
    ref="dialogRef"
    role="dialog"
    aria-modal="true"
    :aria-label="t('cmd.aria')"
    @click.self="emit('close')"
    class="fixed inset-0 bg-black/60 backdrop-blur-md flex items-start justify-center pt-24 z-50 select-none animate-in fade-in duration-150"
  >
    <div data-glass class="w-[560px] rounded-2xl glass glass-panel border flex flex-col overflow-hidden">
      <!-- Search Input -->
      <div class="p-3 border-b border-line flex items-center gap-2.5">
        <Search class="w-4 h-4 text-ink-muted ml-1 shrink-0" />
        <input
          ref="searchInput"
          v-model="searchQuery"
          type="text"
          :placeholder="t('cmd.placeholder')"
          autofocus
          spellcheck="false"
          autocomplete="off"
          class="flex-1 min-w-0 bg-transparent border-none text-sm text-ink placeholder-ink-muted focus:outline-none font-sans"
        />
        <span v-if="searchQuery.trim()" class="text-[10px] text-ink-muted font-mono shrink-0">
          {{ t("cmd.count", { count: filteredCommands.length }) }}
        </span>
        <kbd class="px-1.5 py-0.5 rounded bg-raised border border-line text-[10px] text-ink-muted font-mono shrink-0">
          ESC
        </kbd>
      </div>

      <!-- Grouped Command List -->
      <div ref="listRef" class="max-h-[420px] overflow-y-auto p-2">
        <div v-for="section in commandSections" :key="section.category" class="mb-0.5 last:mb-0">
          <!-- Category Header (粘性吸顶；普通分类可点击折叠) -->
          <div
            class="flex items-center gap-2 px-3 pt-1.5 pb-1 select-none sticky top-0 z-10 bg-surface/95 backdrop-blur-sm"
            :class="!section.pinned && !section.recent ? 'cursor-pointer group/header' : ''"
            :title="
              !section.pinned && !section.recent
                ? t(section.collapsed ? 'cmd.category.expand' : 'cmd.category.collapse', {
                    category: categoryLabel(section.category),
                  })
                : ''
            "
            @click="!section.pinned && !section.recent ? toggleCategory(section.category) : undefined"
          >
            <Star v-if="section.pinned" class="w-3 h-3 text-warning shrink-0" fill="currentColor" />
            <Clock v-else-if="section.recent" class="w-3 h-3 text-accent shrink-0" />
            <ChevronRight
              v-else-if="section.collapsed"
              class="w-3 h-3 text-ink-muted shrink-0 group-hover/header:text-ink transition"
            />
            <ChevronDown
              v-else
              class="w-3 h-3 text-ink-muted shrink-0 group-hover/header:text-ink transition"
            />
            <span
              class="text-[10px] font-semibold tracking-wider"
              :class="section.pinned ? 'text-warning' : section.recent ? 'text-accent' : 'text-ink-muted group-hover/header:text-ink'"
            >
              <template v-for="(part, index) in titleParts(categoryLabel(section.category))" :key="`cat-${index}`">
                <mark v-if="part.hit" class="bg-accent/25 text-accent rounded-[3px] px-0.5">{{ part.text }}</mark>
                <template v-else>{{ part.text }}</template>
              </template>
            </span>
            <span class="h-px flex-1 bg-line/60"></span>
            <button
              v-if="section.recent"
              @click.stop="clearRecents"
              class="pointer-events-auto shrink-0 px-1.5 py-0.5 rounded border border-line bg-surface text-[9px] text-ink-muted hover:text-danger hover:border-danger-border transition cursor-pointer"
              :title="t('cmd.recent.clear.title')"
            >
              {{ t("cmd.recent.clear") }}
            </button>
            <span v-if="section.recent" class="text-[9px] text-ink-muted">{{ t("cmd.recent.hint") }}</span>
            <span class="text-[9px] font-mono text-ink-muted">
              {{ section.collapsed ? section.fullCount : section.items.length }}
            </span>
          </div>

          <div class="space-y-px">
            <div
              v-for="entry in section.items"
              :key="entry.cmd.id"
              :data-idx="entry.index"
              @click="executeCommand(entry.cmd)"
              @mouseenter="selectedIndex = entry.index"
              class="group/row flex items-center justify-between gap-2 px-3 py-1.5 rounded-lg border-l-2 transition-all duration-100 cursor-pointer"
              :class="[
                selectedIndex === entry.index
                  ? 'bg-raised text-accent font-medium shadow-xs border-accent'
                  : 'text-ink-secondary hover:text-ink border-transparent',
                draggingPinId === entry.cmd.id ? 'opacity-40' : '',
                dragOverPinId === entry.cmd.id && draggingPinId !== entry.cmd.id ? 'ring-1 ring-accent/60' : '',
              ]"
              :draggable="section.pinned ? 'true' : undefined"
              @dragstart="section.pinned ? startPinDrag(entry.cmd.id, $event) : undefined"
              @dragover.prevent="section.pinned ? dragOverPin(entry.cmd.id) : undefined"
              @dragleave="section.pinned && dragOverPinId === entry.cmd.id ? (dragOverPinId = null) : undefined"
              @drop="section.pinned ? dropPin(entry.cmd.id, $event) : undefined"
              @dragend="endPinDrag"
            >
              <div class="flex items-center gap-2.5 min-w-0">
                <component :is="entry.cmd.icon" class="w-3.5 h-3.5 shrink-0" />
                <span class="text-xs truncate">
                  <template v-for="(part, index) in titleParts(titleOf(entry.cmd))" :key="`hit-${index}`">
                    <mark v-if="part.hit" class="bg-accent/25 text-accent rounded-[3px] px-0.5">{{ part.text }}</mark>
                    <template v-else>{{ part.text }}</template>
                  </template>
                </span>
              </div>
              <div class="flex items-center gap-0.5 shrink-0">
                <template v-if="section.pinned">
                  <button :aria-label="t('cmd.pin.up')"
                    @click.stop="movePinned(entry.cmd.id, -1)"
                    :disabled="entry.index === 0"
                    class="p-0.5 rounded text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-25 disabled:cursor-not-allowed"
                    :title="t('cmd.pin.up')"
                  >
                    <ChevronUp class="w-3 h-3" />
                  </button>
                  <button :aria-label="t('cmd.pin.down')"
                    @click.stop="movePinned(entry.cmd.id, 1)"
                    :disabled="entry.index === pinnedIds.length - 1"
                    class="p-0.5 rounded text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-25 disabled:cursor-not-allowed"
                    :title="t('cmd.pin.down')"
                  >
                    <ChevronDown class="w-3 h-3" />
                  </button>
                </template>
                <button
                  :aria-label="isPinned(entry.cmd.id) ? t('cmd.pin.remove') : t('cmd.pin.add')"
                  @click.stop="togglePin(entry.cmd)"
                  class="p-0.5 rounded transition cursor-pointer"
                  :class="
                    isPinned(entry.cmd.id)
                      ? 'text-warning'
                      : 'text-ink-muted/0 group-hover/row:text-ink-muted hover:text-warning'
                  "
                  :title="isPinned(entry.cmd.id) ? t('cmd.pin.remove') : t('cmd.pin.add')"
                >
                  <Star class="w-3 h-3" :fill="isPinned(entry.cmd.id) ? 'currentColor' : 'none'" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="filteredCommands.length === 0" class="py-8 text-center text-xs text-ink-muted">
          {{ t("cmd.empty") }}
        </div>
      </div>

      <!-- Bottom Hint Bar -->
      <div
        class="p-2 border-t border-line bg-canvas/30 text-[10px] text-ink-muted flex items-center justify-between gap-2 px-3 font-mono"
      >
        <span class="truncate">{{ t("cmd.footer.hint") }}</span>
        <span class="flex items-center gap-2 shrink-0">
          <button
            @click.stop="toggleAllCategories"
            class="px-1.5 py-0.5 rounded border border-line bg-surface hover:bg-raised text-[9px] text-ink-muted hover:text-ink transition cursor-pointer"
            :title="allCategoriesCollapsed ? t('cmd.collapse.expand.all') : t('cmd.collapse.collapse.all')"
          >
            {{ allCategoriesCollapsed ? t("cmd.expand.all") : t("cmd.collapse.all") }}
          </button>
          <span>{{ t("cmd.footer.shortcuts") }}</span>
        </span>
      </div>
    </div>
  </div>
</template>
