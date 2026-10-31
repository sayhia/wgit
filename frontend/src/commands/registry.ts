// 统一命令注册表：全应用操作索引的唯一数据源。
//
// 每个条目形如 { id, group, labelKey, run }：
// - run 只复用两条既有执行通道，不发明新总线：
//   1. ctx.emitAction(id)   → CommandPalette 向 App.vue emit "action"，
//                              由 App.vue 的 handlePaletteAction 消费；
//   2. ctx.dispatchEvent()  → window.dispatchEvent(CustomEvent)，
//                              由 App.vue / 目标视图的既有监听器消费
//                              （wgit:tools-tab、wgit:settings-section 等）。
// - labelKey 存 i18n key（导航复用 nav.*，工具子页复用 tools.tab.*，
//   其余为 cmd.*），由 CommandPalette 在渲染时懒解析，语言切换即生效。
import type { Component } from "vue";
import {
  FileDiff,
  GitGraph,
  GitBranch,
  Archive,
  ClipboardCheck,
  GitPullRequest,
  Wrench,
  Settings,
  Terminal,
  RotateCcw,
  Trash2,
  Network,
  FolderGit2,
  HardDrive,
  Boxes,
  FolderTree,
  PieChart,
  Activity,
  Users,
  Gamepad2,
  GitCompare,
  Rocket,
  Keyboard,
  FileWarning,
  HeartPulse,
  FileArchive,
  Flame,
  Webhook,
  Search,
  Globe,
  FolderOpen,
  FolderPlus,
  Home,
  LayoutDashboard,
  FolderSearch,
  RotateCw,
  ArrowUpCircle,
  ArrowDownCircle,
  Plus,
  Minus,
  GitMerge,
  Filter,
  StickyNote,
  Layers,
  Lock,
  Database,
  Zap,
  ClipboardList,
  Cpu,
  Images,
  FileSearch,
  Package,
  Palette,
  User,
  ShieldCheck,
  Play,
} from "lucide-vue-next";

/** 命令分组；分组标题 key 约定为 `cmd.cat.${group}`。 */
export type CommandGroup =
  | "nav"
  | "tools"
  | "help"
  | "search"
  | "repo"
  | "git"
  | "workspace"
  | "stash"
  | "workflow"
  | "project"
  | "lfs"
  | "engine"
  | "studio"
  | "size"
  | "submodule"
  | "settings";

/** 分组的展示顺序，也是面板未搜索时的分区顺序。 */
export const COMMAND_GROUPS: readonly CommandGroup[] = [
  "nav",
  "tools",
  "help",
  "search",
  "repo",
  "git",
  "workspace",
  "stash",
  "workflow",
  "project",
  "lfs",
  "engine",
  "studio",
  "size",
  "submodule",
  "settings",
];

/** run 收到的执行上下文：只暴露两条既有通道。 */
export interface CommandRunContext {
  /** 既有通道 1：CommandPalette emit("action", id) → App.vue handlePaletteAction。 */
  emitAction: (actionId: string) => void;
  /** 既有通道 2：window CustomEvent，由 App.vue / 目标视图的既有监听器消费。 */
  dispatchWindowEvent: (eventName: string, detail?: Record<string, unknown>) => void;
}

export interface CommandEntry {
  id: string;
  group: CommandGroup;
  /** i18n key，在两个语言包中都必须存在（tests/commands-registry.test.ts 把关）。 */
  labelKey: string;
  icon: Component;
  run: (ctx: CommandRunContext) => void;
}

/** 通道 1 的便捷封装。 */
function action(actionId: string) {
  return (ctx: CommandRunContext) => ctx.emitAction(actionId);
}

/** 通道 2 的便捷封装。 */
function event(eventName: string, detail?: Record<string, unknown>) {
  return (ctx: CommandRunContext) => ctx.dispatchWindowEvent(eventName, detail);
}

/**
 * 全应用命令注册表。
 *
 * 注意：ToolsView 实际有 14 个子 Tab（TOOLS_SUB_TABS），本表逐一注册；
 * 已知无法用现有机制触发的操作不注册，避免死命令 —— 例如快捷键速查弹窗
 * 只由 App.vue 的键盘快捷键直接切换内部 showCheatSheet 状态，没有事件入口。
 */
export const COMMAND_REGISTRY: readonly CommandEntry[] = [
  // ---- navigation：主 Tab（App.vue handlePaletteAction 的 goto-* 通道） ----
  { id: "goto-workspace", group: "nav", labelKey: "cmd.goto-workspace", icon: FileDiff, run: action("goto-workspace") },
  { id: "goto-history", group: "nav", labelKey: "cmd.goto-history", icon: GitGraph, run: action("goto-history") },
  { id: "goto-branches", group: "nav", labelKey: "cmd.goto-branches", icon: GitBranch, run: action("goto-branches") },
  { id: "goto-stashes", group: "nav", labelKey: "cmd.goto-stashes", icon: Archive, run: action("goto-stashes") },
  { id: "goto-review", group: "nav", labelKey: "cmd.goto-review", icon: ClipboardCheck, run: action("goto-review") },
  { id: "goto-pr", group: "nav", labelKey: "cmd.goto-pr", icon: GitPullRequest, run: action("goto-pr") },
  { id: "goto-tools", group: "nav", labelKey: "cmd.goto-tools", icon: Wrench, run: action("goto-tools") },
  { id: "goto-settings", group: "nav", labelKey: "cmd.goto-settings", icon: Settings, run: action("goto-settings") },

  // ---- tools：ToolsView 全部子 Tab（既有 wgit:tools-tab 事件） ----
  { id: "tools-terminal", group: "tools", labelKey: "tools.tab.terminal", icon: Terminal, run: event("wgit:tools-tab", { tab: "terminal" }) },
  { id: "tools-reflog", group: "tools", labelKey: "tools.tab.reflog", icon: RotateCcw, run: event("wgit:tools-tab", { tab: "reflog" }) },
  { id: "tools-clean", group: "tools", labelKey: "tools.tab.clean", icon: Trash2, run: event("wgit:tools-tab", { tab: "clean" }) },
  { id: "tools-remotes", group: "tools", labelKey: "tools.tab.remotes", icon: Network, run: event("wgit:tools-tab", { tab: "remotes" }) },
  { id: "tools-worktrees", group: "tools", labelKey: "tools.tab.worktrees", icon: FolderGit2, run: event("wgit:tools-tab", { tab: "worktrees" }) },
  { id: "tools-lfs", group: "tools", labelKey: "tools.tab.lfs", icon: HardDrive, run: event("wgit:tools-tab", { tab: "lfs" }) },
  { id: "tools-submodules", group: "tools", labelKey: "tools.tab.submodules", icon: Boxes, run: event("wgit:tools-tab", { tab: "submodules" }) },
  { id: "tools-sparse", group: "tools", labelKey: "tools.tab.sparse", icon: FolderTree, run: event("wgit:tools-tab", { tab: "sparse" }) },
  { id: "tools-disk", group: "tools", labelKey: "tools.tab.disk", icon: PieChart, run: event("wgit:tools-tab", { tab: "disk" }) },
  { id: "tools-maintenance", group: "tools", labelKey: "tools.tab.maintenance", icon: Activity, run: event("wgit:tools-tab", { tab: "maintenance" }) },
  { id: "tools-studio", group: "tools", labelKey: "tools.tab.studio", icon: Users, run: event("wgit:tools-tab", { tab: "studio" }) },
  { id: "tools-engine", group: "tools", labelKey: "tools.tab.engine", icon: Gamepad2, run: event("wgit:tools-tab", { tab: "engine" }) },
  { id: "tools-bisect", group: "tools", labelKey: "tools.tab.bisect", icon: GitCompare, run: event("wgit:tools-tab", { tab: "bisect" }) },
  { id: "tools-release", group: "tools", labelKey: "tools.tab.release", icon: Rocket, run: event("wgit:tools-tab", { tab: "release" }) },

  // ---- help：帮助与诊断（全部走既有事件入口） ----
  { id: "settings-shortcuts", group: "help", labelKey: "cmd.settings-shortcuts", icon: Keyboard, run: event("wgit:settings-section", { section: "shortcuts" }) },
  { id: "open-crash-report", group: "help", labelKey: "crash.modal.title", icon: FileWarning, run: event("wgit:open-crash-report") },

  // ---- search：快速打开（QuickOpen 的既有事件） ----
  { id: "quick-open-files", group: "search", labelKey: "cmd.quick-open-files", icon: Search, run: event("wgit:quick-open", { mode: "files" }) },
  { id: "quick-open-code", group: "search", labelKey: "cmd.quick-open-code", icon: Search, run: event("wgit:quick-open", { mode: "code" }) },

  // ---- repo：仓库（欢迎页 / 克隆 / 初始化 / 扫描） ----
  { id: "repo-open-dialog", group: "repo", labelKey: "cmd.repo-open-dialog", icon: FolderOpen, run: action("open-dialog") },
  { id: "repo-clone", group: "repo", labelKey: "cmd.repo-clone", icon: Globe, run: event("wgit:onboarding", { mode: "clone" }) },
  { id: "repo-init", group: "repo", labelKey: "cmd.repo-init", icon: FolderPlus, run: event("wgit:onboarding", { mode: "init" }) },
  { id: "repo-welcome", group: "repo", labelKey: "cmd.repo-welcome", icon: Home, run: event("wgit:onboarding", { mode: "welcome" }) },
  { id: "repo-dashboard", group: "repo", labelKey: "cmd.repo-dashboard", icon: LayoutDashboard, run: action("toggle-repo-dashboard") },
  { id: "repo-scan", group: "repo", labelKey: "cmd.repo-scan", icon: FolderSearch, run: event("wgit:onboarding", { mode: "scan" }) },

  // ---- git：同步与变基（App.vue 既有 action） ----
  { id: "sync-fetch", group: "git", labelKey: "cmd.sync-fetch", icon: RotateCw, run: action("sync-fetch") },
  { id: "sync-pull", group: "git", labelKey: "cmd.sync-pull", icon: ArrowDownCircle, run: action("sync-pull") },
  { id: "sync-pull-options", group: "git", labelKey: "cmd.sync-pull-options", icon: ArrowDownCircle, run: action("sync-pull-options") },
  { id: "git-rebase", group: "git", labelKey: "cmd.git-rebase", icon: GitPullRequest, run: action("git-rebase") },
  { id: "sync-push", group: "git", labelKey: "cmd.sync-push", icon: ArrowUpCircle, run: action("sync-push") },
  { id: "sync-push-options", group: "git", labelKey: "cmd.sync-push-options", icon: ArrowUpCircle, run: action("sync-push-options") },
  { id: "sync-fetch-options", group: "git", labelKey: "cmd.sync-fetch-options", icon: RotateCw, run: action("sync-fetch-options") },

  // ---- workspace：工作区操作与布局 ----
  { id: "stage-all", group: "workspace", labelKey: "cmd.stage-all", icon: Plus, run: action("stage-all") },
  { id: "unstage-all", group: "workspace", labelKey: "cmd.unstage-all", icon: Minus, run: action("unstage-all") },
  { id: "layout-standard", group: "workspace", labelKey: "cmd.layout-standard", icon: LayoutDashboard, run: event("wgit:workspace-layout", { preset: "standard" }) },
  { id: "layout-focus-source", group: "workspace", labelKey: "cmd.layout-focus-source", icon: FolderTree, run: event("wgit:workspace-layout", { preset: "focus-source" }) },
  { id: "layout-save-custom", group: "workspace", labelKey: "cmd.layout-save-custom", icon: Plus, run: event("wgit:workspace-layout", { preset: "save-custom" }) },
  { id: "layout-apply-custom", group: "workspace", labelKey: "cmd.layout-apply-custom", icon: Play, run: event("wgit:workspace-layout", { preset: "custom" }) },
  { id: "layout-focus-diff", group: "workspace", labelKey: "cmd.layout-focus-diff", icon: GitCompare, run: event("wgit:workspace-layout", { preset: "focus-diff" }) },

  // ---- stash：储藏 ----
  { id: "stash-save", group: "stash", labelKey: "cmd.stash-save", icon: Archive, run: action("stash-save") },
  { id: "stash-pop", group: "stash", labelKey: "cmd.stash-pop", icon: Play, run: action("stash-pop") },

  // ---- workflow：跨视图工作流 ----
  { id: "history-rebase", group: "workflow", labelKey: "cmd.history-rebase", icon: GitMerge, run: event("wgit:history-rebase") },
  { id: "history-filter", group: "workflow", labelKey: "cmd.history-filter", icon: Filter, run: action("goto-history") },
  { id: "compare-refs", group: "workflow", labelKey: "cmd.compare-refs", icon: GitCompare, run: event("wgit:branches-tab", { tab: "compare" }) },
  { id: "history-notes", group: "workflow", labelKey: "cmd.history-notes", icon: StickyNote, run: event("wgit:history-notes") },
  { id: "branches-range-diff", group: "workflow", labelKey: "cmd.branches-range-diff", icon: Layers, run: event("wgit:branches-tab", { tab: "compare" }) },

  // ---- project：大型项目工具分区直达（ToolsView 子页内 section） ----
  { id: "tools-health", group: "project", labelKey: "cmd.tools-health", icon: HeartPulse, run: event("wgit:tools-tab", { tab: "maintenance", section: "health" }) },
  { id: "tools-object-store", group: "project", labelKey: "cmd.tools-object-store", icon: Database, run: event("wgit:tools-tab", { tab: "maintenance", section: "object-store" }) },
  { id: "tools-fsck", group: "project", labelKey: "cmd.tools-fsck", icon: ShieldCheck, run: event("wgit:tools-tab", { tab: "maintenance", section: "fsck" }) },
  { id: "tools-archive", group: "project", labelKey: "cmd.tools-archive", icon: FileArchive, run: event("wgit:tools-tab", { tab: "maintenance", section: "archive" }) },
  { id: "tools-hotfix", group: "project", labelKey: "cmd.tools-hotfix", icon: Flame, run: event("wgit:tools-tab", { tab: "maintenance", section: "hotfix" }) },
  { id: "tools-hooks", group: "project", labelKey: "cmd.tools-hooks", icon: Webhook, run: event("wgit:tools-tab", { tab: "maintenance", section: "hooks" }) },
  { id: "tools-engine-meta", group: "project", labelKey: "cmd.tools-engine-meta", icon: Gamepad2, run: event("wgit:tools-tab", { tab: "engine", section: "engine-unity-meta" }) },

  // ---- lfs：Git LFS 分区直达 ----
  { id: "tools-staged-large-lfs", group: "lfs", labelKey: "cmd.tools-staged-large-lfs", icon: FileWarning, run: event("wgit:tools-tab", { tab: "clean" }) },
  { id: "lfs-patterns", group: "lfs", labelKey: "cmd.lfs-patterns", icon: HardDrive, run: event("wgit:tools-tab", { tab: "lfs", section: "lfs-patterns" }) },
  { id: "lfs-locks", group: "lfs", labelKey: "cmd.lfs-locks", icon: Lock, run: event("wgit:tools-tab", { tab: "lfs", section: "lfs-locks" }) },
  { id: "lfs-cache", group: "lfs", labelKey: "cmd.lfs-cache", icon: Database, run: event("wgit:tools-tab", { tab: "lfs", section: "lfs-cache" }) },
  { id: "lfs-tuning", group: "lfs", labelKey: "cmd.lfs-tuning", icon: Zap, run: event("wgit:tools-tab", { tab: "lfs", section: "lfs-tuning" }) },
  { id: "lfs-revisions", group: "lfs", labelKey: "cmd.lfs-revisions", icon: RotateCcw, run: event("wgit:tools-tab", { tab: "lfs", section: "lfs-revisions" }) },
  { id: "lfs-lock-activity", group: "lfs", labelKey: "cmd.lfs-lock-activity", icon: ClipboardList, run: event("wgit:tools-tab", { tab: "lfs", section: "lfs-lock-activity" }) },
  { id: "lfs-lockable", group: "lfs", labelKey: "cmd.lfs-lockable", icon: ShieldCheck, run: event("wgit:tools-tab", { tab: "lfs", section: "lfs-lockable" }) },

  // ---- engine：引擎与资产分区直达 ----
  { id: "engine-detect", group: "engine", labelKey: "cmd.engine-detect", icon: Gamepad2, run: event("wgit:tools-tab", { tab: "engine", section: "engine-detect" }) },
  { id: "engine-asset", group: "engine", labelKey: "cmd.engine-asset", icon: FileSearch, run: event("wgit:tools-tab", { tab: "engine", section: "engine-asset" }) },
  { id: "engine-unity-meta", group: "engine", labelKey: "cmd.engine-unity-meta", icon: Cpu, run: event("wgit:tools-tab", { tab: "engine", section: "engine-unity-meta" }) },
  { id: "engine-unity-merge", group: "engine", labelKey: "cmd.engine-unity-merge", icon: GitMerge, run: event("wgit:tools-tab", { tab: "engine", section: "engine-unity-merge" }) },
  { id: "engine-texture", group: "engine", labelKey: "cmd.engine-texture", icon: Images, run: event("wgit:tools-tab", { tab: "engine", section: "engine-texture" }) },
  { id: "engine-runtime", group: "engine", labelKey: "cmd.engine-runtime", icon: Zap, run: event("wgit:tools-tab", { tab: "engine", section: "engine-runtime" }) },

  // ---- studio：Studio 协作分区直达 ----
  { id: "studio-team", group: "studio", labelKey: "cmd.studio-team", icon: Users, run: event("wgit:tools-tab", { tab: "studio", section: "studio-team" }) },
  { id: "studio-audits", group: "studio", labelKey: "cmd.studio-audits", icon: ShieldCheck, run: event("wgit:tools-tab", { tab: "studio", section: "studio-audits" }) },
  { id: "studio-logs", group: "studio", labelKey: "cmd.studio-logs", icon: ClipboardList, run: event("wgit:tools-tab", { tab: "studio", section: "studio-logs" }) },
  { id: "studio-custom-actions", group: "studio", labelKey: "cmd.studio-custom-actions", icon: Zap, run: event("wgit:tools-tab", { tab: "studio", section: "studio-custom-actions" }) },
  { id: "studio-compliance", group: "studio", labelKey: "cmd.studio-compliance", icon: Lock, run: event("wgit:tools-tab", { tab: "studio", section: "studio-compliance" }) },

  // ---- size：体积与扫描分区直达 ----
  { id: "disk-metrics", group: "size", labelKey: "cmd.disk-metrics", icon: PieChart, run: event("wgit:tools-tab", { tab: "disk", section: "disk-metrics" }) },
  { id: "disk-large-files", group: "size", labelKey: "cmd.disk-large-files", icon: FileSearch, run: event("wgit:tools-tab", { tab: "disk", section: "disk-large-files" }) },
  { id: "disk-history-blobs", group: "size", labelKey: "cmd.disk-history-blobs", icon: Database, run: event("wgit:tools-tab", { tab: "disk", section: "disk-history-blobs" }) },
  { id: "disk-package-budget", group: "size", labelKey: "cmd.disk-package-budget", icon: Package, run: event("wgit:tools-tab", { tab: "disk", section: "disk-package-budget" }) },

  // ---- submodule：子模块与工作树分区直达 ----
  { id: "submodule-list", group: "submodule", labelKey: "cmd.submodule-list", icon: Boxes, run: event("wgit:tools-tab", { tab: "submodules", section: "submodule-list" }) },
  { id: "subtree-list", group: "submodule", labelKey: "cmd.subtree-list", icon: FolderTree, run: event("wgit:tools-tab", { tab: "submodules", section: "subtree-list" }) },
  { id: "remotes-panel", group: "submodule", labelKey: "cmd.remotes-panel", icon: Network, run: event("wgit:tools-tab", { tab: "remotes", section: "remotes" }) },
  { id: "worktrees-panel", group: "submodule", labelKey: "cmd.worktrees-panel", icon: FolderGit2, run: event("wgit:tools-tab", { tab: "worktrees", section: "worktrees" }) },
  { id: "sparse-folders", group: "submodule", labelKey: "cmd.sparse-folders", icon: Filter, run: event("wgit:tools-tab", { tab: "sparse", section: "sparse-folders" }) },

  // ---- settings：设置分区直达 ----
  { id: "settings-appearance", group: "settings", labelKey: "cmd.settings-appearance", icon: Palette, run: event("wgit:settings-section", { section: "appearance" }) },
  { id: "settings-identity", group: "settings", labelKey: "cmd.settings-identity", icon: User, run: event("wgit:settings-section", { section: "identity" }) },
  { id: "settings-signing", group: "settings", labelKey: "cmd.settings-signing", icon: ShieldCheck, run: event("wgit:settings-section", { section: "signing" }) },
  { id: "settings-proxy", group: "settings", labelKey: "cmd.settings-proxy", icon: Globe, run: event("wgit:settings-section", { section: "proxy" }) },
  { id: "settings-tools", group: "settings", labelKey: "cmd.settings-tools", icon: GitCompare, run: event("wgit:settings-section", { section: "tools" }) },
  { id: "settings-config", group: "settings", labelKey: "cmd.settings-config", icon: Settings, run: event("wgit:settings-section", { section: "config" }) },
  { id: "settings-safe-directory", group: "settings", labelKey: "cmd.settings-safe-directory", icon: ShieldCheck, run: event("wgit:settings-section", { section: "config" }) },
];
