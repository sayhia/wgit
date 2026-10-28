<script setup lang="ts">
import { reportError } from "./composables/useToast";
import { confirmAction } from "./composables/useConfirm";
import { useLiquidGlass } from "./composables/useLiquidGlass";
import { ref, onMounted, onUnmounted, computed, nextTick, defineAsyncComponent } from "vue";
import HeaderBar from "./components/HeaderBar.vue";
import RepoRail from "./components/RepoRail.vue";
import ActivityDock from "./components/ActivityDock.vue";
import WorkspaceView from "./components/WorkspaceView.vue";
const LazyHistory = defineAsyncComponent(() => import("./components/HistoryView.vue"));
const LazyBranches = defineAsyncComponent(() => import("./components/BranchesView.vue"));
const LazyStash = defineAsyncComponent(() => import("./components/StashView.vue"));
const LazyTools = defineAsyncComponent(() => import("./components/ToolsView.vue"));
const LazySettings = defineAsyncComponent(() => import("./components/SettingsView.vue"));
const LazyReview = defineAsyncComponent(() => import("./components/ReviewView.vue"));
const LazyPr = defineAsyncComponent(() => import("./components/pr/PrView.vue"));
const CommandPalette = defineAsyncComponent(() => import("./components/CommandPalette.vue"));
const RepoOnboarding = defineAsyncComponent(() => import("./components/RepoOnboarding.vue"));
const RepoDashboard = defineAsyncComponent(() => import("./components/RepoDashboard.vue"));
const QuickOpen = defineAsyncComponent(() => import("./components/QuickOpen.vue"));
const SyncModal = defineAsyncComponent(() => import("./components/SyncModal.vue"));
const RebaseModal = defineAsyncComponent(() => import("./components/RebaseModal.vue"));
const ShortcutCheatSheetModal = defineAsyncComponent(() => import("./components/ShortcutCheatSheetModal.vue"));
const MiniCommitHudModal = defineAsyncComponent(() => import("./components/workspace/MiniCommitHudModal.vue"));
const CrashReportModal = defineAsyncComponent(() => import("./components/CrashReportModal.vue"));
import Toast from "./components/ui/Toast.vue";
import ConfirmDialog from "./components/ui/ConfirmDialog.vue";
import { FolderGit2 } from "lucide-vue-next";
import { useToast, pushToast } from "./composables/useToast";
import { useSessionRestore } from "./composables/useSessionRestore";
import { restoreGeometry } from "./composables/useWindowChrome";
import { setupGlobalCrashReporting, hasPendingCrashes } from "./utils/crashReporter";
import { t } from "./i18n";
import { gitService } from "./services/gitService";
import { Events } from "@wailsio/runtime";
import { parseDeepLink, type DeepLinkPayload } from "./utils/deepLink";
import {
  SHORTCUT_ACTIONS,
  effectiveBinding,
  loadShortcutOverrides,
  matchesBinding,
} from "./utils/shortcuts";
import type { RepoStatus } from "./types";

/** Multi-repo workspace: every opened repository becomes a tab. */
const openRepos = ref<string[]>([]);
const activeRepoPath = ref<string>("");
const repoStatuses = ref<Record<string, RepoStatus | null>>({});
const currentRepoPath = computed(() => activeRepoPath.value);
const repoStatus = computed<RepoStatus | null>(
  () => repoStatuses.value[activeRepoPath.value] ?? null
);

const { toastText, showToast, notify } = useToast(4000);

// One delegated listener drives the specular highlight on every glass surface.
useLiquidGlass();
const { prewarmRepo, saveSessionState, loadSessionState } = useSessionRestore();
const activeTab = ref<string>("workspace");

// Remember which main view the user was on, so restarting lands back in
// context instead of always snapping to the workspace.
const ACTIVE_TAB_KEY = "wgit.active-tab";
const RESTORABLE_TABS = new Set(["workspace", "history", "branches", "stashes", "review", "pr", "tools", "settings"]);

/**
 * The review workspace has two sources: local base...head sessions and remote
 * PR/MR. They share one nav entry and one tab; this switch decides which
 * source is shown. "pr" is still accepted as a legacy tab id from older
 * versions so a saved session lands on the right source.
 */
type ReviewMode = "local" | "remote";
const REVIEW_MODE_KEY = "wgit.review-mode";
const reviewMode = ref<ReviewMode>("local");

function setReviewMode(mode: ReviewMode) {
  reviewMode.value = mode;
  try {
    localStorage.setItem(REVIEW_MODE_KEY, mode);
  } catch {
    /* storage disabled — preference just won't persist */
  }
}

function handleSelectTab(tab: string) {
  if (tab === "pr") {
    activeTab.value = "review";
    setReviewMode("remote");
    saveSessionState({ activeView: "review" });
    return;
  }
  activeTab.value = tab;
  saveSessionState({ activeView: tab });
  try {
    localStorage.setItem(ACTIVE_TAB_KEY, tab);
  } catch {
    /* storage disabled — preference just won't persist */
  }
}

/** Applied only after a repo is actually open: tools/settings need one. */
function restoreActiveTab() {
  if (!activeRepoPath.value) return;
  try {
    const saved = localStorage.getItem(ACTIVE_TAB_KEY);
    if (saved === "pr") {
      activeTab.value = "review";
      setReviewMode("remote");
      return;
    }
    if (saved && RESTORABLE_TABS.has(saved)) activeTab.value = saved;
    if (activeTab.value === "review") {
      const savedMode = localStorage.getItem(REVIEW_MODE_KEY);
      if (savedMode === "local" || savedMode === "remote") reviewMode.value = savedMode;
    }
  } catch {
    /* ignore */
  }
}
const isSyncing = ref<boolean>(false);
const showCommandPalette = ref<boolean>(false);
const showOnboarding = ref<boolean>(false);
const onboardingMode = ref<"welcome" | "clone" | "init" | "scan">("welcome");
const showRepoDashboard = ref<boolean>(false);
const showSyncModal = ref<boolean>(false);
const syncModalTab = ref<"pull" | "push" | "fetch">("pull");
const showRebaseModal = ref<boolean>(false);
const rebaseModalTarget = ref<string | undefined>(undefined);
const showCheatSheet = ref<boolean>(false);
const showMiniCommitHud = ref<boolean>(false);
const showCrashReportModal = ref<boolean>(false);

function openSyncModal(tab: "pull" | "push" | "fetch" = "pull") {
  syncModalTab.value = tab;
  showSyncModal.value = true;
}

function openRebaseModal(targetBranch?: string) {
  rebaseModalTarget.value = targetBranch;
  showRebaseModal.value = true;
}

function handleLaunchInteractiveRebase(base: string) {
  showRebaseModal.value = false;
  activeTab.value = "history";
  nextTick(() => {
    window.dispatchEvent(new CustomEvent("wgit:history-rebase", { detail: { base } }));
  });
}

const OPEN_REPOS_KEY = "wgit.openRepos.v1";

const hasDirtyFiles = computed(() => {
  if (!repoStatus.value) return false;
  return (
    (repoStatus.value.stagedFiles?.length || 0) > 0 ||
    (repoStatus.value.unstagedFiles?.length || 0) > 0 ||
    (repoStatus.value.untrackedFiles?.length || 0) > 0
  );
});

function persistTabs() {
  try {
    localStorage.setItem(
      OPEN_REPOS_KEY,
      JSON.stringify({ repos: openRepos.value.slice(0, 12), active: activeRepoPath.value })
    );
  } catch (err) {
    // best-effort: localStorage may be disabled; the tab list is a convenience.
    console.debug("Failed to persist open repos:", err);
  }
}

const isGlobalRefreshing = ref(false);
const isDraggingOver = ref(false);

function handleDragOver(e: DragEvent) {
  e.preventDefault();
  if (e.dataTransfer?.types.includes("Files")) {
    isDraggingOver.value = true;
  }
}

function handleDragLeave(e: DragEvent) {
  if (e.relatedTarget === null || !(e.currentTarget as HTMLElement)?.contains(e.relatedTarget as Node)) {
    isDraggingOver.value = false;
  }
}

function handleDrop(e: DragEvent) {
  e.preventDefault();
  isDraggingOver.value = false;
}

async function refreshStatus() {
  const path = activeRepoPath.value;
  if (!path) return;
  isGlobalRefreshing.value = true;
  try {
    const status = await gitService.getRepoStatus(path);
    if (status) {
      repoStatuses.value = { ...repoStatuses.value, [path]: status };
    }
  } catch (err: any) {
    console.debug("Failed to refresh status:", err);
    notify(t("app.toast.refresh.failed", { error: err?.message || err }));
  } finally {
    isGlobalRefreshing.value = false;
  }
}

/** Refresh branch/dirty status for every open repository (tab badges + dashboard). */
async function refreshAllStatuses() {
  const paths = openRepos.value.slice();
  const entries = await Promise.all(
    paths.map(async (p) => {
      try {
        return [p, await gitService.getRepoStatus(p)] as const;
      } catch {
        return [p, null] as const;
      }
    })
  );
  const next: Record<string, RepoStatus | null> = { ...repoStatuses.value };
  for (const [p, st] of entries) next[p] = st;
  repoStatuses.value = next;
}

/**
 * A folder dropped on the window.
 *
 * The path comes from Go (`WindowFilesDropped`), not from `DataTransfer`:
 * WKWebView never fills in `File.path`, so the HTML5 route silently did
 * nothing on macOS. Go forwards the absolute path and the usual validation in
 * `openRepo` reports a non-repository drop instead of failing quietly.
 */
async function handleGlobalRepoPath(event: Event) {
  const detail = (event as CustomEvent<string>).detail;
  const path = typeof detail === "string" ? detail : (event as unknown as { data?: string }).data;
  if (typeof path === "string" && path) {
    await openRepo(path);
  }
}

async function openRepo(path: string) {
  try {
    const info = await gitService.openRepo(path);
    if (info?.path) {
      handleRepoOpened(info.path);
    }
  } catch (err) {
    reportError("Failed to open repo", err);
  }
}

/** Shared post-open flow for onboarding / clone / init / repo switcher / dashboard. */
function handleRepoOpened(path: string) {
  if (!path) return;
  if (!openRepos.value.includes(path)) {
    openRepos.value = [...openRepos.value, path];
  }
  activeRepoPath.value = path;
  showOnboarding.value = false;
  persistTabs();
  refreshStatus();
  void prewarmRepo(path);
  saveSessionState({ lastRepoPath: path, activeView: activeTab.value });
  if (openRepos.value.length > 1) refreshAllStatuses();
}

function selectRepo(path: string) {
  if (!path) return;
  activeRepoPath.value = path;
  showRepoDashboard.value = false;
  persistTabs();
  refreshStatus();
  void prewarmRepo(path);
  saveSessionState({ lastRepoPath: path, activeView: activeTab.value });
}

function handleDashboardOpened(path: string) {
  showRepoDashboard.value = false;
  selectRepo(path);
}

function closeRepo(path: string) {
  const idx = openRepos.value.indexOf(path);
  if (idx === -1) return;
  const remaining = openRepos.value.filter((p) => p !== path);
  openRepos.value = remaining;
  const nextStatuses = { ...repoStatuses.value };
  delete nextStatuses[path];
  repoStatuses.value = nextStatuses;

  if (activeRepoPath.value === path) {
    const fallback = remaining[Math.max(0, idx - 1)] ?? remaining[0] ?? "";
    activeRepoPath.value = fallback;
    if (!fallback) {
      showOnboarding.value = true;
      onboardingMode.value = "welcome";
    } else {
      refreshStatus();
    }
  }
  persistTabs();
}

function handleReorderRepos(repos: string[]) {
  openRepos.value = repos;
  persistTabs();
}

function handleCloseOtherRepos(keepPath: string) {
  if (!openRepos.value.includes(keepPath)) return;
  openRepos.value = [keepPath];
  activeRepoPath.value = keepPath;
  const keepStatus = repoStatuses.value[keepPath] ?? null;
  repoStatuses.value = { [keepPath]: keepStatus };
  showOnboarding.value = false;
  persistTabs();
  refreshStatus();
}

async function fetchSpecificRepo(path: string) {
  try {
    await gitService.fetch(path);
  } catch (err: any) {
    console.debug(`Fetch failed for ${path}:`, err);
    notify(
      t("app.toast.fetch.failed.repo", {
        repo: path.split("/").pop() ?? path,
        error: err?.message || err,
      })
    );
  }
  refreshAllStatuses();
}

async function fetchAllRepos() {
  isSyncing.value = true;
  try {
    const paths = openRepos.value.slice();
    const failures: string[] = [];
    await Promise.all(
      paths.map(async (p) => {
        try {
          await gitService.fetch(p);
        } catch (err: any) {
          failures.push(`${p.split("/").pop()}: ${err?.message || err}`);
        }
      })
    );
    if (failures.length > 0) {
      notify(
        t("app.toast.fetch.failed.batch", {
          failed: failures.length,
          total: paths.length,
          first: failures[0] ?? "",
        })
      );
    }
    await refreshAllStatuses();
  } finally {
    isSyncing.value = false;
  }
}

function openOnboarding(mode: "welcome" | "clone" | "init" | "scan") {
  onboardingMode.value = mode;
  showOnboarding.value = true;
}

/** CommandPalette can jump straight to clone/init/welcome onboarding. */
function handleGlobalOnboarding(e: Event) {
  const detail = (e as CustomEvent).detail as { mode?: string } | undefined;
  const mode: "welcome" | "clone" | "init" | "scan" =
    detail?.mode === "clone"
      ? "clone"
      : detail?.mode === "init"
        ? "init"
        : detail?.mode === "scan"
          ? "scan"
          : "welcome";
  openOnboarding(mode);
}

/** Re-emit cross-view navigation events after the target view has mounted. */
function forwardAfterMount(eventName: string, targetTab: string, e: Event) {
  const detail = (e as CustomEvent).detail;
  if (activeTab.value === targetTab) return; // target view is already mounted and listening
  activeTab.value = targetTab;

  const dispatch = () => window.dispatchEvent(new CustomEvent(eventName, { detail }));
  // The target view is mounted lazily by `v-if` and only attaches its window
  // listener in `onMounted`, which can land after the nextTick microtask. Emit
  // once on the microtask and once after paint so the first cross-view jump is
  // never swallowed (duplicate delivery is idempotent for these targets).
  nextTick(() => {
    dispatch();
    setTimeout(dispatch, 120);
  });
}

function handleGlobalToolsTab(e: Event) {
  forwardAfterMount("wgit:tools-tab", "tools", e);
}

function handleGlobalHistoryRebase(e: Event) {
  forwardAfterMount("wgit:history-rebase", "history", e);
}

function handleGlobalHistoryCommit(e: Event) {
  forwardAfterMount("wgit:history-commit", "history", e);
}

function handleGlobalSettingsSection(e: Event) {
  forwardAfterMount("wgit:settings-section", "settings", e);
}

function handleGlobalHistoryNotes(e: Event) {
  forwardAfterMount("wgit:history-notes", "history", e);
}

function handleGlobalBranchesTab(e: Event) {
  forwardAfterMount("wgit:branches-tab", "branches", e);
}

function handleGlobalWorkspaceLayout(e: Event) {
  forwardAfterMount("wgit:workspace-layout", "workspace", e);
}

function handleGlobalOpenRebase(e: Event) {
  const detail = (e as CustomEvent).detail as { target?: string } | undefined;
  openRebaseModal(detail?.target);
}

/** QuickOpen / WorkspaceView open-file event: switch to the workspace tab. */
function handleGlobalOpenFile(e: Event) {
  const detail = (e as CustomEvent).detail as { path?: string; line?: number } | undefined;
  if (!detail || !detail.path || !currentRepoPath.value) return;
  if (activeTab.value === "workspace") return; // WorkspaceView already mounted and handling it.
  activeTab.value = "workspace";
  // WorkspaceView only exists while the workspace tab is active; re-emit once it
  // has mounted so the file/line selection is not lost during the tab switch.
  nextTick(() => {
    window.dispatchEvent(new CustomEvent("wgit:open-file", { detail }));
  });
}

/**
 * Dispatches deep link actions (wgit://open, wgit://commit, wgit://diff, wgit://clone).
 */
async function handleDeepLink(raw: string | DeepLinkPayload) {
  const payload = typeof raw === "string" ? parseDeepLink(raw) : raw;
  if (!payload) return;

  // 1. Clone onboarding dialog
  if (payload.action === "clone") {
    openOnboarding("clone");
    if (payload.url) {
      nextTick(() => {
        window.dispatchEvent(new CustomEvent("wgit:clone-url", { detail: { url: payload.url } }));
      });
    }
    return;
  }

  // 2. Switch or open target repository
  if (payload.repo) {
    const targetRepo = payload.repo;
    if (!openRepos.value.includes(targetRepo)) {
      await openRepo(targetRepo);
    } else if (activeRepoPath.value !== targetRepo) {
      selectRepo(targetRepo);
    }
  }

  // 3. Switch view tab
  const targetTab = payload.tab || (payload.action === "commit" ? "history" : payload.file || payload.action === "diff" ? "workspace" : undefined);
  if (targetTab) {
    handleSelectTab(targetTab);
  }

  // 4. Target commit hash in history view
  const targetCommit = payload.ref || payload.hash;
  if (targetCommit) {
    nextTick(() => {
      window.dispatchEvent(new CustomEvent("wgit:history-commit", { detail: { hash: targetCommit } }));
    });
  }

  // 5. Target file & line in workspace view
  if (payload.file) {
    nextTick(() => {
      window.dispatchEvent(new CustomEvent("wgit:open-file", {
        detail: { path: payload.file, line: payload.line }
      }));
    });
  }
}

function handleGlobalDeepLinkEvent(e: Event) {
  const detail = (e as CustomEvent).detail;
  const link = typeof detail === "string" ? detail : (detail as { url?: string })?.url;
  if (link) {
    void handleDeepLink(link);
  }
}

let offWailsOpenUrl: (() => void) | null = null;
let offRepoStatusChanged: (() => void) | null = null;
let teardownCrashReporter: (() => void) | null = null;

async function handleOpenDialog() {
  try {
    const info = await gitService.openRepoDialog();
    if (info?.path) {
      handleRepoOpened(info.path);
    }
  } catch (err: any) {
    console.error("Failed to open repo dialog:", err);
    const msg = err?.message || String(err);
    // Backend wire values (English + both historical Chinese forms) — matching
    // literals, never displayed.
    if (msg.includes("not a valid git repository") || msg.includes("不是一个有效的 Git 仓库") || msg.includes("不是有效的 Git 仓库")) {
      const wantInit = await confirmAction(t("app.confirm.init.repo", { message: msg }));
      if (wantInit) {
        const match = msg.match(/['"「]([^'"」]+)['"」]/);
        const folder = match ? match[1] : "";
        if (folder) {
          try {
            const inited = await gitService.initRepo(folder);
            if (inited?.path) {
              handleRepoOpened(inited.path);
              return;
            }
          } catch (initErr: any) {
            notify(t("app.toast.init.failed", { error: initErr?.message || initErr }));
            return;
          }
        }
        openOnboarding("init");
      }
    } else {
      notify(t("app.toast.open.failed", { error: msg }));
    }
  }
}

async function handleSyncFetch() {
  if (!currentRepoPath.value) return;
  isSyncing.value = true;
  try {
    await gitService.fetch(currentRepoPath.value);
    await refreshStatus();
    await refreshAllStatuses();
  } catch (err: any) {
    notify(t("app.toast.fetch.failed", { error: err?.message || err }));
  } finally {
    isSyncing.value = false;
  }
}

async function handleSyncPull() {
  if (!currentRepoPath.value) return;
  isSyncing.value = true;
  try {
    await gitService.pull(currentRepoPath.value);
    await refreshStatus();
    await refreshAllStatuses();
  } catch (err: any) {
    notify(t("app.toast.pull.failed", { error: err?.message || err }));
  } finally {
    isSyncing.value = false;
  }
}

async function handleSyncPush() {
  if (!currentRepoPath.value) return;
  isSyncing.value = true;
  try {
    await gitService.push(currentRepoPath.value);
    await refreshStatus();
    await refreshAllStatuses();
  } catch (err: any) {
    notify(t("app.toast.push.failed", { error: err?.message || err }));
  } finally {
    isSyncing.value = false;
  }
}

async function handleSwitchBranch(branchName: string) {
  if (!currentRepoPath.value) return;
  try {
    await gitService.checkoutBranch(currentRepoPath.value, branchName);
    await refreshStatus();
    await refreshAllStatuses();
  } catch (err: any) {
    notify(t("app.toast.checkout.failed", { error: err?.message || err }));
  }
}

function handlePaletteAction(actionId: string) {
  switch (actionId) {
    case "goto-workspace":
      activeTab.value = "workspace";
      break;
    case "goto-history":
      activeTab.value = "history";
      break;
    case "goto-branches":
      activeTab.value = "branches";
      break;
    case "goto-stashes":
      activeTab.value = "stashes";
      break;
    case "goto-tools":
      activeTab.value = "tools";
      break;
    case "goto-review":
      activeTab.value = "review";
      setReviewMode("local");
      break;
    case "goto-pr":
      activeTab.value = "review";
      setReviewMode("remote");
      break;
    case "goto-settings":
      activeTab.value = "settings";
      break;
    case "toggle-repo-dashboard":
      showRepoDashboard.value = !showRepoDashboard.value;
      break;
    case "mini-commit-hud":
      showMiniCommitHud.value = true;
      break;
    case "open-dialog":
    case "repo-open-dialog":
      handleOpenDialog();
      break;
    case "sync-fetch":
      handleSyncFetch();
      break;
    case "sync-pull":
      handleSyncPull();
      break;
    case "sync-push":
      handleSyncPush();
      break;
    case "sync-pull-options":
      openSyncModal("pull");
      break;
    case "sync-push-options":
      openSyncModal("push");
      break;
    case "sync-fetch-options":
      openSyncModal("fetch");
      break;
    case "git-rebase":
    case "rebase-open":
      openRebaseModal();
      break;
    case "stage-all":
      gitService.stageAll(currentRepoPath.value).then(refreshStatus);
      break;
    case "unstage-all":
      gitService.unstageAll(currentRepoPath.value).then(refreshStatus);
      break;
    case "stash-save":
      activeTab.value = "stashes";
      break;
    case "stash-pop":
      gitService.stashPop(currentRepoPath.value, 0).then(() => {
        refreshStatus();
      });
      break;
    case "clean-open":
      activeTab.value = "tools";
      break;
  }
}

function handleGlobalKeydown(e: KeyboardEvent) {
  const mod = e.metaKey || e.ctrlKey;

  // Help Cheat Sheet: ⌘/Ctrl + / or ? (when not typing in an input)
  if (mod && e.key === "/") {
    e.preventDefault();
    showCheatSheet.value = !showCheatSheet.value;
    return;
  }
  if (!mod && !e.altKey && e.key === "?") {
    const tag = (document.activeElement as HTMLElement)?.tagName?.toLowerCase();
    if (tag !== "input" && tag !== "textarea") {
      e.preventDefault();
      showCheatSheet.value = !showCheatSheet.value;
      return;
    }
  }

  if (!mod && !e.altKey) return;

  // Fixed binding: ⌘/Ctrl + 1..9 switch between open repository tabs.
  if (mod && !e.shiftKey && !e.altKey && /^[1-9]$/.test(e.key)) {
    const idx = Number(e.key) - 1;
    const target = openRepos.value[idx];
    if (target) {
      e.preventDefault();
      selectRepo(target);
    }
    return;
  }

  // Fixed binding: ⌘/Ctrl + T opens repo dialog / new tab
  if (mod && !e.shiftKey && !e.altKey && e.key.toLowerCase() === "t") {
    e.preventDefault();
    handleOpenDialog();
    return;
  }

  // Fixed binding: ⌘/Ctrl + W closes current repo tab
  if (mod && !e.shiftKey && !e.altKey && e.key.toLowerCase() === "w") {
    e.preventDefault();
    if (activeRepoPath.value) {
      closeRepo(activeRepoPath.value);
    }
    return;
  }

  // Customizable global actions live in utils/shortcuts.ts; the Settings page
  // records overrides and persists them to localStorage.
  const overrides = loadShortcutOverrides();
  for (const action of SHORTCUT_ACTIONS) {
    if (!action.customizable) continue;
    const customized = !!overrides[action.id];
    // Component-owned defaults are still handled inside their own views; the
    // scheduler only takes over once a custom binding has been recorded.
    if (action.owner === "component" && !customized) continue;
    if (!matchesBinding(e, effectiveBinding(action, overrides))) continue;
    e.preventDefault();
    if (action.owner === "app") {
      switch (action.id) {
        case "command-palette":
          showCommandPalette.value = !showCommandPalette.value;
          break;
        case "open-repo":
          handleOpenDialog();
          break;
        case "repo-dashboard":
          showRepoDashboard.value = !showRepoDashboard.value;
          break;
        case "interactive-rebase":
          openRebaseModal();
          break;
        case "mini-commit-hud":
          showMiniCommitHud.value = !showMiniCommitHud.value;
          break;
      }
    } else if (action.eventName) {
      window.dispatchEvent(new CustomEvent(action.eventName, { detail: action.eventDetail }));
    }
    return;
  }
}

/** Restore previously opened repository tabs, validating each path still exists. */
async function restoreTabs() {
  let saved: { repos?: string[]; active?: string } = {};
  try {
    const raw = localStorage.getItem(OPEN_REPOS_KEY);
    if (raw) saved = JSON.parse(raw) || {};
  } catch (err) {
    // best-effort: a corrupt tab list should not block startup.
    console.debug("Failed to read persisted tabs:", err);
  }

  const candidates = Array.isArray(saved.repos) ? saved.repos.filter((p) => typeof p === "string" && p) : [];
  if (candidates.length > 0) {
    const checks = await Promise.all(
      candidates.map(async (p) => {
        try {
          const st = await gitService.getRepoStatus(p);
          return { p, ok: !!st, st };
        } catch {
          return { p, ok: false, st: null };
        }
      })
    );
    const valid = checks.filter((c) => c.ok);
    if (valid.length > 0) {
      openRepos.value = valid.map((c) => c.p);
      const next: Record<string, RepoStatus | null> = {};
      for (const c of valid) next[c.p] = c.st;
      repoStatuses.value = next;
      activeRepoPath.value = valid.some((c) => c.p === saved.active) ? (saved.active as string) : valid[0]!.p;
      persistTabs();
      restoreActiveTab();
      await refreshAllStatuses();
      return;
    }
  }

  // Fallback: open the most recent repository from the backend config.
  try {
    const recents = await gitService.getRecentRepos();
    if (recents && recents.length > 0 && recents[0]) {
      handleRepoOpened(recents[0]);
      return;
    }
  } catch (err) {
    // best-effort: the picker simply shows no history.
    console.debug("Failed to load recent repos:", err);
  }
  showOnboarding.value = true;
}

function selectNextRepoTab() {
  if (openRepos.value.length <= 1) return;
  const curIdx = openRepos.value.indexOf(activeRepoPath.value);
  const nextIdx = (curIdx + 1) % openRepos.value.length;
  selectRepo(openRepos.value[nextIdx]!);
}

function selectPrevRepoTab() {
  if (openRepos.value.length <= 1) return;
  const curIdx = openRepos.value.indexOf(activeRepoPath.value);
  const prevIdx = (curIdx - 1 + openRepos.value.length) % openRepos.value.length;
  selectRepo(openRepos.value[prevIdx]!);
}

function handleGlobalOpenCrashReport() {
  showCrashReportModal.value = true;
}

onMounted(async () => {
  teardownCrashReporter = setupGlobalCrashReporting();
  // Reopen where the window was last closed, if that spot still exists.
  void restoreGeometry();
  window.addEventListener("keydown", handleGlobalKeydown);
  window.addEventListener("wgit:open-file", handleGlobalOpenFile);
  window.addEventListener("wgit:tab-next", selectNextRepoTab);
  window.addEventListener("wgit:tab-prev", selectPrevRepoTab);
  window.addEventListener("wgit:onboarding", handleGlobalOnboarding);
  window.addEventListener("wgit:tools-tab", handleGlobalToolsTab);
  window.addEventListener("wgit:history-rebase", handleGlobalHistoryRebase);
  window.addEventListener("wgit:history-commit", handleGlobalHistoryCommit);
  window.addEventListener("wgit:history-notes", handleGlobalHistoryNotes);
  window.addEventListener("wgit:branches-tab", handleGlobalBranchesTab);
  window.addEventListener("wgit:workspace-layout", handleGlobalWorkspaceLayout);
  window.addEventListener("wgit:settings-section", handleGlobalSettingsSection);
  window.addEventListener("wgit:open-rebase", handleGlobalOpenRebase);
  window.addEventListener("wgit:open-crash-report", handleGlobalOpenCrashReport);
  window.addEventListener("wgit:open-url", handleGlobalDeepLinkEvent);
  window.addEventListener("wgit:deep-link", handleGlobalDeepLinkEvent);
  window.addEventListener("wgit:open-repo-path", handleGlobalRepoPath);

  if (hasPendingCrashes()) {
    pushToast(t("crash.detected.banner"), {
      tone: "warning",
      duration: 8000,
      action: {
        label: t("crash.view.report"),
        run: () => {
          showCrashReportModal.value = true;
        },
      },
    });
  }

  try {
    offWailsOpenUrl = Events.On("wgit:open-url", (event: any) => {
      const url = typeof event?.data === "string" ? event.data : typeof event === "string" ? event : "";
      if (url) {
        void handleDeepLink(url);
      }
    });
    offRepoStatusChanged = Events.On("repo:status-changed", (event: any) => {
      const changedPath = typeof event?.data === "string" ? event.data : typeof event === "string" ? event : "";
      if (!changedPath || changedPath === currentRepoPath.value) {
        void refreshStatus();
      }
    });
  } catch {
    /* running outside Wails runtime */
  }

  await restoreTabs();

  // Process initial deep link from CLI args / OS launch or browser search
  try {
    const initLink = await gitService.getInitialDeepLink();
    if (initLink) {
      await handleDeepLink(initLink);
    }
  } catch {
    /* ignore */
  }

  if (typeof window !== "undefined" && window.location?.search) {
    const searchPayload = parseDeepLink(window.location.href);
    if (searchPayload && (searchPayload.repo || searchPayload.tab || searchPayload.file || searchPayload.hash)) {
      void handleDeepLink(searchPayload);
    }
  }
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleGlobalKeydown);
  window.removeEventListener("wgit:open-file", handleGlobalOpenFile);
  window.removeEventListener("wgit:tab-next", selectNextRepoTab);
  window.removeEventListener("wgit:tab-prev", selectPrevRepoTab);
  window.removeEventListener("wgit:onboarding", handleGlobalOnboarding);
  window.removeEventListener("wgit:tools-tab", handleGlobalToolsTab);
  window.removeEventListener("wgit:history-rebase", handleGlobalHistoryRebase);
  window.removeEventListener("wgit:history-commit", handleGlobalHistoryCommit);
  window.removeEventListener("wgit:history-notes", handleGlobalHistoryNotes);
  window.removeEventListener("wgit:branches-tab", handleGlobalBranchesTab);
  window.removeEventListener("wgit:workspace-layout", handleGlobalWorkspaceLayout);
  window.removeEventListener("wgit:settings-section", handleGlobalSettingsSection);
  window.removeEventListener("wgit:open-rebase", handleGlobalOpenRebase);
  window.removeEventListener("wgit:open-crash-report", handleGlobalOpenCrashReport);
  window.removeEventListener("wgit:open-url", handleGlobalDeepLinkEvent);
  window.removeEventListener("wgit:deep-link", handleGlobalDeepLinkEvent);
  window.removeEventListener("wgit:open-repo-path", handleGlobalRepoPath);
  if (offWailsOpenUrl) {
    offWailsOpenUrl();
    offWailsOpenUrl = null;
  }
  if (offRepoStatusChanged) {
    offRepoStatusChanged();
    offRepoStatusChanged = null;
  }
  if (teardownCrashReporter) {
    teardownCrashReporter();
    teardownCrashReporter = null;
  }
});
</script>

<template>
  <div
    data-file-drop-target
    :data-drop-hint="t('app.drop.repo.title')"
    class="h-screen w-screen flex flex-col bg-canvas text-ink overflow-hidden select-none font-sans"
    @dragover="handleDragOver"
    @dragleave="handleDragLeave"
    @drop="handleDrop"
  >
    <!-- Non-blanking global refresh progress bar -->
    <div
      v-if="isGlobalRefreshing"
      data-testid="global-refresh-bar"
      class="fixed top-0 left-0 right-0 h-0.5 bg-accent z-[100] pointer-events-none shadow-xs animate-pulse"
    />

    <!-- Drag and drop repository overlay -->
    <div
      v-if="isDraggingOver"
      data-testid="repo-drop-overlay"
      class="fixed inset-0 z-[120] bg-canvas/80 backdrop-blur-sm border-2 border-dashed border-accent flex flex-col items-center justify-center gap-3 pointer-events-none"
    >
      <FolderGit2 class="w-12 h-12 text-accent animate-bounce" />
      <span class="text-sm font-semibold text-ink">{{ t("app.drop.repo.title") }}</span>
    </div>

    <!-- Top Header Navigation & Controls (Mac drag region, repo switcher, branch pill, sync buttons) -->
    <a
      href="#wgit-main"
      class="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[100] focus:px-3 focus:py-1.5 focus:rounded-lg focus:bg-accent-strong focus:text-accent-contrast focus:text-xs focus:font-semibold focus:shadow-lg"
    >
      {{ t("app.skip.to.content") }}
    </a>

    <HeaderBar
      :current-repo-path="currentRepoPath"
      :repo-status="repoStatus"
      :is-syncing="isSyncing"
      @open-repo="openRepo"
      @open-dialog="handleOpenDialog"
      @clone-repo="openOnboarding('clone')"
      @init-repo="openOnboarding('init')"
      @refresh="refreshStatus"
      @sync-fetch="handleSyncFetch"
      @sync-pull="handleSyncPull"
      @sync-push="handleSyncPush"
      @open-sync-modal="openSyncModal"
      @open-rebase="openRebaseModal"
      @switch-branch="handleSwitchBranch"
      @toggle-command-palette="showCommandPalette = true"
    />

    <!-- Multi-repo tab rail: one tab per opened repository + dashboard entry -->
    <RepoRail
      :repos="openRepos"
      :active-repo="activeRepoPath"
      :statuses="repoStatuses"
      :dashboard-open="showRepoDashboard"
      @select="selectRepo"
      @close="closeRepo"
      @close-others="handleCloseOtherRepos"
      @reorder="handleReorderRepos"
      @refresh-repo="fetchSpecificRepo"
      @notify="notify"
      @open-dialog="handleOpenDialog"
      @open-new="openOnboarding('welcome')"
      @toggle-dashboard="showRepoDashboard = !showRepoDashboard"
    />

    <!-- Main Workspace Body: Activity Dock + Main View -->
    <div class="flex-1 flex overflow-hidden">
      <!-- Activity Dock (Left slim 48px) -->
      <ActivityDock
        :active-tab="activeTab"
        :has-dirty-files="hasDirtyFiles"
        @select-tab="handleSelectTab"
      />

      <!-- Content Views -->
      <main id="wgit-main" tabindex="-1" class="flex-1 flex overflow-hidden relative focus:outline-none">
        <!-- 1. Workspace: source tree on the left, changes on the right -->
        <WorkspaceView
          v-if="activeTab === 'workspace'"
          :current-repo-path="currentRepoPath"
          :repo-status="repoStatus"
          @refresh="refreshStatus"
        />

        <!-- 2. History & Commit DAG -->
        <LazyHistory
          v-else-if="activeTab === 'history'"
          :current-repo-path="currentRepoPath"
        />

        <!-- 3. Branches & Tags -->
        <LazyBranches
          v-else-if="activeTab === 'branches'"
          :current-repo-path="currentRepoPath"
          @switch-branch="handleSwitchBranch"
          @refresh="refreshStatus"
        />

        <!-- 4. Stashes & Snapshots -->
        <LazyStash
          v-else-if="activeTab === 'stashes'"
          :current-repo-path="currentRepoPath"
          @refresh="refreshStatus"
        />

        <!-- 5. Review workspace: local base...head sessions or remote PR/MR -->
        <template v-else-if="activeTab === 'review'">
          <LazyReview
            v-if="reviewMode === 'local'"
            :current-repo-path="currentRepoPath"
            @refresh="refreshStatus"
            @switch-mode="setReviewMode"
          />
          <LazyPr
            v-else
            :current-repo-path="currentRepoPath"
            @switch-mode="setReviewMode"
          />
        </template>

        <!-- 6. Universal Tools & Diagnostics -->
        <LazyTools
          v-else-if="activeTab === 'tools'"
          :current-repo-path="currentRepoPath"
          @refresh="refreshStatus"
          @open-repo="handleRepoOpened"
        />

        <!-- 7. Settings -->
        <LazySettings
          v-else-if="activeTab === 'settings'"
          :current-repo-path="currentRepoPath"
          @refresh="refreshStatus"
          @open-crash-report="showCrashReportModal = true"
        />
      </main>
    </div>

    <!-- Floating Raycast-Style Command Palette (⌘K) -->
    <CommandPalette
      :is-open="showCommandPalette"
      @close="showCommandPalette = false"
      @action="handlePaletteAction"
    />

    <!-- Multi-repo dashboard overlay (⌘⇧O): status of every opened repository -->
    <RepoDashboard
      v-if="showRepoDashboard"
      :repos="openRepos"
      :active-repo="activeRepoPath"
      :open="showRepoDashboard"
      @opened="handleDashboardOpened"
      @closed="closeRepo"
      @fetch="fetchSpecificRepo"
      @fetch-all="fetchAllRepos"
      @close-overlay="showRepoDashboard = false"
    />

    <!-- Large-project onboarding: welcome / clone (blobless, shallow) / init -->
    <RepoOnboarding
      v-if="showOnboarding"
      :mode="onboardingMode"
      :can-close="!!currentRepoPath"
      @opened="handleRepoOpened"
      @close="showOnboarding = false"
    />

    <!-- Advanced Git Sync (Pull / Push / Fetch Options & Lifecycle) -->
    <SyncModal
      :is-open="showSyncModal"
      :initial-tab="syncModalTab"
      :current-repo-path="currentRepoPath"
      :repo-status="repoStatus"
      @close="showSyncModal = false"
      @refresh="refreshStatus"
    />

    <!-- Advanced Git Rebase (⌘R / HeaderBar / History) -->
    <RebaseModal
      :is-open="showRebaseModal"
      :current-repo-path="currentRepoPath"
      :repo-status="repoStatus"
      :target-branch="rebaseModalTarget"
      @close="showRebaseModal = false"
      @refresh="refreshStatus"
      @launch-interactive="handleLaunchInteractiveRebase"
    />

    <!-- Large-project navigation: ⌘P quick open files / ⌘⇧F repo code search -->
    <QuickOpen :current-repo-path="currentRepoPath" />

    <!-- Shared confirm dialog host: every view calls confirmAction() -->
    <ConfirmDialog />

    <!-- Shortcuts Cheat Sheet Modal (⌘/ or ?) -->
    <ShortcutCheatSheetModal
      :open="showCheatSheet"
      @close="showCheatSheet = false"
    />

    <!-- Mini Fast Commit HUD (⌘⌥C) -->
    <MiniCommitHudModal
      v-model:open="showMiniCommitHud"
      :repo-path="currentRepoPath"
      @committed="refreshStatus"
      @notify="notify"
    />

    <!--
      Folder-drop affordance is painted by CSS on `.file-drop-target-active`,
      the class the Wails runtime toggles while a native drag hovers the drop
      target. No JS state: the drag never reaches the page as a DOM event on
      macOS, so a `v-if` overlay would simply never appear.
    -->

    <!-- Crash & Diagnostics Report Modal -->
    <CrashReportModal
      :is-open="showCrashReportModal"
      @close="showCrashReportModal = false"
      @notify="notify"
      @safe-reset="restoreGeometry"
    />

    <Toast :text="toastText" :visible="showToast" />
  </div>
</template>
