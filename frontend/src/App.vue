<script setup lang="ts">
import { confirmAction } from "./composables/useConfirm";
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
import CommandPalette from "./components/CommandPalette.vue";
import RepoOnboarding from "./components/RepoOnboarding.vue";
import RepoDashboard from "./components/RepoDashboard.vue";
import QuickOpen from "./components/QuickOpen.vue";
import SyncModal from "./components/SyncModal.vue";
import RebaseModal from "./components/RebaseModal.vue";
import Toast from "./components/ui/Toast.vue";
import ConfirmDialog from "./components/ui/ConfirmDialog.vue";
import { useToast } from "./composables/useToast";
import { gitService } from "./services/gitService";
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
const activeTab = ref<string>("workspace");

// Remember which main view the user was on, so restarting lands back in
// context instead of always snapping to the workspace.
const ACTIVE_TAB_KEY = "wgit.active-tab";
const RESTORABLE_TABS = new Set(["workspace", "history", "branches", "stashes", "review", "pr", "tools", "settings"]);

function handleSelectTab(tab: string) {
  activeTab.value = tab;
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
    if (saved && RESTORABLE_TABS.has(saved)) activeTab.value = saved;
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
    console.debug("Failed to persist open repos:", err);
  }
}

async function refreshStatus() {
  const path = activeRepoPath.value;
  if (!path) return;
  try {
    const status = await gitService.getRepoStatus(path);
    if (status) {
      repoStatuses.value = { ...repoStatuses.value, [path]: status };
    }
  } catch (err: any) {
    console.debug("Failed to refresh status:", err);
    notify(`刷新状态失败: ${err?.message || err}`);
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

async function openRepo(path: string) {
  try {
    const info = await gitService.openRepo(path);
    if (info?.path) {
      handleRepoOpened(info.path);
    }
  } catch (err) {
    console.error("Failed to open repo:", err);
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
  if (openRepos.value.length > 1) refreshAllStatuses();
}

function selectRepo(path: string) {
  if (!path) return;
  activeRepoPath.value = path;
  showRepoDashboard.value = false;
  persistTabs();
  refreshStatus();
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

async function fetchSpecificRepo(path: string) {
  try {
    await gitService.fetch(path);
  } catch (err: any) {
    console.debug(`Fetch failed for ${path}:`, err);
    notify(`Fetch 失败 (${path.split("/").pop()}): ${err?.message || err}`);
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
      notify(`Fetch 失败 ${failures.length}/${paths.length}: ${failures[0]}`);
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

async function handleOpenDialog() {
  try {
    const info = await gitService.openRepoDialog();
    if (info?.path) {
      handleRepoOpened(info.path);
    }
  } catch (err: any) {
    console.error("Failed to open repo dialog:", err);
    const msg = err?.message || String(err);
    if (msg.includes("not a valid git repository") || msg.includes("不是一个有效的 Git 仓库") || msg.includes("不是有效的 Git 仓库")) {
      const wantInit = await confirmAction(`${msg}\n\n是否要在该目录初始化并打开 Git 仓库？`);
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
            notify(`初始化仓库失败: ${initErr?.message || initErr}`);
            return;
          }
        }
        openOnboarding("init");
      }
    } else {
      notify(`打开仓库失败: ${msg}`);
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
    notify(`Fetch 失败: ${err?.message || err}`);
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
    notify(`Pull 失败: ${err?.message || err}`);
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
    notify(`Push 失败: ${err?.message || err}`);
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
    notify(`检出分支失败: ${err?.message || err}`);
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
      break;
    case "goto-pr":
      activeTab.value = "pr";
      break;
    case "goto-settings":
      activeTab.value = "settings";
      break;
    case "toggle-repo-dashboard":
      showRepoDashboard.value = !showRepoDashboard.value;
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
    console.debug("Failed to load recent repos:", err);
  }
  showOnboarding.value = true;
}

onMounted(async () => {
  window.addEventListener("keydown", handleGlobalKeydown);
  window.addEventListener("wgit:open-file", handleGlobalOpenFile);
  window.addEventListener("wgit:onboarding", handleGlobalOnboarding);
  window.addEventListener("wgit:tools-tab", handleGlobalToolsTab);
  window.addEventListener("wgit:history-rebase", handleGlobalHistoryRebase);
  window.addEventListener("wgit:history-notes", handleGlobalHistoryNotes);
  window.addEventListener("wgit:branches-tab", handleGlobalBranchesTab);
  window.addEventListener("wgit:workspace-layout", handleGlobalWorkspaceLayout);
  window.addEventListener("wgit:settings-section", handleGlobalSettingsSection);
  window.addEventListener("wgit:open-rebase", handleGlobalOpenRebase);
  await restoreTabs();
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleGlobalKeydown);
  window.removeEventListener("wgit:open-file", handleGlobalOpenFile);
  window.removeEventListener("wgit:onboarding", handleGlobalOnboarding);
  window.removeEventListener("wgit:tools-tab", handleGlobalToolsTab);
  window.removeEventListener("wgit:history-rebase", handleGlobalHistoryRebase);
  window.removeEventListener("wgit:history-notes", handleGlobalHistoryNotes);
  window.removeEventListener("wgit:branches-tab", handleGlobalBranchesTab);
  window.removeEventListener("wgit:workspace-layout", handleGlobalWorkspaceLayout);
  window.removeEventListener("wgit:settings-section", handleGlobalSettingsSection);
  window.removeEventListener("wgit:open-rebase", handleGlobalOpenRebase);
});
</script>

<template>
  <div class="h-screen w-screen flex flex-col bg-canvas text-ink overflow-hidden select-none font-sans">
    <!-- Top Header Navigation & Controls (Mac drag region, repo switcher, branch pill, sync buttons) -->
    <a
      href="#wgit-main"
      class="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[100] focus:px-3 focus:py-1.5 focus:rounded-lg focus:bg-accent-strong focus:text-white focus:text-xs focus:font-semibold focus:shadow-lg"
    >
      跳到主内容
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
        <!-- 1. Workspace: 源码放左面 变更放右边 -->
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

        <!-- 5. Change review workspace (base...head review sessions) -->
        <LazyReview
          v-else-if="activeTab === 'review'"
          :current-repo-path="currentRepoPath"
          @refresh="refreshStatus"
        />

        <!-- 5b. Remote PR / MR review (GitHub / GitLab) -->
        <LazyPr
          v-else-if="activeTab === 'pr'"
          :current-repo-path="currentRepoPath"
        />

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

    <Toast :text="toastText" :visible="showToast" />
  </div>
</template>
