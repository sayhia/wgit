<script setup lang="ts">
import { reportError } from "../composables/useToast";
import { ref, onMounted, onUnmounted, computed } from "vue";
import {
  FolderGit2,
  GitBranch,
  ArrowDownCircle,
  ArrowUpCircle,
  RotateCw,
  Search,
  FolderOpen,
  Command,
  Download,
  FolderPlus,
  ChevronDown,
  GitPullRequest,
  Bookmark,
  Plus,
  Trash2,
  EllipsisVertical,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import { useWindowDrag } from "../composables/useWindowChrome";
import { t } from "../i18n";
import type { RepoStatus, BranchInfo, WorkContextSession } from "../types";
import { useRovingList } from "../composables/useRovingList";

const props = defineProps<{
  currentRepoPath: string;
  repoStatus: RepoStatus | null;
  isSyncing: boolean;
}>();

const emit = defineEmits<{
  (e: "open-repo", path: string): void;
  (e: "open-dialog"): void;
  (e: "clone-repo"): void;
  (e: "init-repo"): void;
  (e: "refresh"): void;
  (e: "sync-fetch"): void;
  (e: "sync-pull"): void;
  (e: "sync-push"): void;
  (e: "open-sync-modal", tab?: "pull" | "push" | "fetch"): void;
  (e: "open-rebase", targetBranch?: string): void;
  (e: "open-merge", targetBranch?: string): void;
  (e: "switch-branch", branchName: string): void;
  (e: "toggle-command-palette"): void;
}>();

const headerRef = ref<HTMLElement | null>(null);
// Dragging is driven in JS on macOS (see `useWindowChrome`), so the native
// app-region is switched off there to avoid moving the window twice.
const dragStyle = {
  "-webkit-app-region": (
    (window as unknown as { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS ===
    "darwin"
      ? "no-drag"
      : "drag"
  ),
} as Record<string, string>;

useWindowDrag(headerRef);

const showRepoMenu = ref(false);
const showBranchMenu = ref(false);
// Narrow-window overflow: below `xl` the sync cluster collapses into one menu so
// the header can never clip its own buttons (see the `hidden xl:flex` guard).
const showActionMenu = ref(false);
const recentRepos = ref<string[]>([]);
const branches = ref<BranchInfo[]>([]);
const repoSearchFilter = ref("");
const branchSearchFilter = ref("");

const repoName = computed(() => {
  if (!props.currentRepoPath) return t("header.repo.none");
  const parts = props.currentRepoPath.replace(/\\/g, "/").split("/");
  return parts[parts.length - 1] || props.currentRepoPath;
});

const isDirty = computed(() => {
  if (!props.repoStatus) return false;
  return (
    (props.repoStatus.stagedFiles?.length || 0) > 0 ||
    (props.repoStatus.unstagedFiles?.length || 0) > 0 ||
    (props.repoStatus.untrackedFiles?.length || 0) > 0
  );
});

const filteredRecentRepos = computed(() => {
  if (!repoSearchFilter.value) return recentRepos.value;
  return recentRepos.value.filter((r) =>
    r.toLowerCase().includes(repoSearchFilter.value.toLowerCase())
  );
});

const filteredBranches = computed(() => {
  if (!branchSearchFilter.value) return branches.value;
  return branches.value.filter((b) =>
    b.name.toLowerCase().includes(branchSearchFilter.value.toLowerCase())
  );
});

// Keyboard parity with the branch/tag/stash lists: inside each dropdown only
// one result row is tabbable at a time and ↑/↓ moves between rows. The branch
// rows carry their own rebase/merge buttons, so no listbox role here — same
// reasoning as useRovingList's doc comment.
const {
  setItemRef: setRepoItemRef,
  tabindexFor: repoTabindexFor,
  onKeydown: onRepoRowKeydown,
  onFocus: onRepoRowFocus,
} = useRovingList(() => filteredRecentRepos.value, (repo) => repo);

const {
  setItemRef: setBranchItemRef,
  tabindexFor: branchTabindexFor,
  onKeydown: onBranchRowKeydown,
  onFocus: onBranchRowFocus,
} = useRovingList(() => filteredBranches.value, (b) => b.name);

async function loadRecents() {
  try {
    const list = await gitService.getRecentRepos();
    if (list && list.length > 0) {
      recentRepos.value = list;
    }
  } catch (err) {
    reportError("Failed to load recent repos", err);
  }
}

async function loadBranches() {
  if (!props.currentRepoPath) return;
  try {
    const list = await gitService.getBranches(props.currentRepoPath);
    if (list) branches.value = list;
  } catch (err) {
    reportError("Failed to load branches", err);
  }
}

function selectRepo(path: string) {
  showRepoMenu.value = false;
  emit("open-repo", path);
}

function selectBranch(name: string) {
  showBranchMenu.value = false;
  emit("switch-branch", name);
}

const showContextSessionMenu = ref(false);
const contextSessions = ref<WorkContextSession[]>([]);
const newSessionName = ref("");
const isSavingSession = ref(false);

async function loadContextSessions() {
  if (!props.currentRepoPath) return;
  try {
    const list = await gitService.listWorkContextSessions(props.currentRepoPath);
    contextSessions.value = list || [];
  } catch (err) {
    reportError("Failed to load context sessions", err);
  }
}

async function handleSaveSession() {
  if (!props.currentRepoPath || !newSessionName.value.trim() || isSavingSession.value) return;
  isSavingSession.value = true;
  try {
    const name = newSessionName.value.trim();
    await gitService.saveWorkContextSession(props.currentRepoPath, {
      id: "",
      name,
      branch: props.repoStatus?.currentBranch || "main",
      activeFiles: [],
      commitDraft: "",
      issueOrPr: "",
      notes: "",
      createdAt: "",
      updatedAt: "",
    });
    newSessionName.value = "";
    await loadContextSessions();
  } catch (err: any) {
    reportError("Failed to save context session", err);
  } finally {
    isSavingSession.value = false;
  }
}

async function handleDeleteSession(id: string) {
  if (!props.currentRepoPath) return;
  try {
    await gitService.deleteWorkContextSession(props.currentRepoPath, id);
    await loadContextSessions();
  } catch (err) {
    reportError("Failed to delete context session", err);
  }
}

/**
 * Header dropdown coordination.
 *
 * Only one menu may be open at a time, and a click anywhere outside the header
 * (or Escape) dismisses it. Without this, opening the repo menu and then the
 * branch pill left both absolutely-positioned panels stacked on top of each
 * other once the header stopped clipping them.
 */
function closeAllHeaderMenus() {
  showRepoMenu.value = false;
  showBranchMenu.value = false;
  showContextSessionMenu.value = false;
  showActionMenu.value = false;
}

function toggleHeaderMenu(menu: "repo" | "branch" | "context" | "action") {
  const wasOpen = {
    repo: showRepoMenu.value,
    branch: showBranchMenu.value,
    context: showContextSessionMenu.value,
    action: showActionMenu.value,
  }[menu];
  closeAllHeaderMenus();
  if (wasOpen) return;
  if (menu === "repo") {
    showRepoMenu.value = true;
    loadRecents();
  } else if (menu === "branch") {
    showBranchMenu.value = true;
    loadBranches();
  } else if (menu === "context") {
    showContextSessionMenu.value = true;
    loadContextSessions();
  } else {
    showActionMenu.value = true;
  }
}

function hasOpenHeaderMenu(): boolean {
  return showRepoMenu.value || showBranchMenu.value || showContextSessionMenu.value || showActionMenu.value;
}

function handleHeaderClickCapture(event: MouseEvent) {
  if (!hasOpenHeaderMenu()) return;
  const target = event.target as HTMLElement | null;
  if (target?.closest("[data-header-menu], [data-header-menu-trigger]")) return;
  // Header actions (sync buttons, palette trigger, …) should dismiss an open
  // panel instead of leaving it floating above the dialog they just opened.
  closeAllHeaderMenus();
}

function handleHeaderPointerDown(event: MouseEvent) {
  if (!headerRef.value || headerRef.value.contains(event.target as Node)) return;
  if (hasOpenHeaderMenu()) closeAllHeaderMenus();
}

function handleHeaderKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" && hasOpenHeaderMenu()) closeAllHeaderMenus();
}

onMounted(() => {
  loadRecents();
  document.addEventListener("mousedown", handleHeaderPointerDown);
  window.addEventListener("keydown", handleHeaderKeydown);
});

onUnmounted(() => {
  document.removeEventListener("mousedown", handleHeaderPointerDown);
  window.removeEventListener("keydown", handleHeaderKeydown);
});
</script>

<template>
  <!--
    Window drag region.

    The header wears the material again: dragging no longer depends on WebKit's
    hit-testing (`-webkit-app-region`), because `useWindowChrome` moves the
    window explicitly. `.glass-bar` is flat paint — tint plus inset edges — so
    there is no composited layer here either.
  -->
  <!--
    `overflow-visible` is load-bearing: the repo / branch / context-session
    dropdowns are absolutely positioned children anchored below the 48px bar.
    With `overflow-hidden` every one of them (including the narrow-window
    「更多同步操作」 menu) was clipped away and the buttons did nothing. Spacing
    here is gap-based and the inner groups collapse by breakpoint, so the bar
    does not need to clip its own menus.
  -->
  <header
    ref="headerRef"
    class="h-12 border-b border-line glass glass-bar flex items-center gap-2 px-3 select-none z-30 shrink-0 overflow-visible"
    :style="dragStyle"
    @click.capture="handleHeaderClickCapture"
  >
    <!-- Left: macOS traffic lights offset + Repo & Branch Switcher -->
    <div class="flex items-center gap-2 pl-18 no-drag shrink-0 min-w-0" style="-webkit-app-region: no-drag;">
      <!-- Project Switcher Pill -->
      <div class="relative">
        <button
          data-header-menu-trigger
          @click="toggleHeaderMenu('repo')"
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-raised/70 hover:bg-raised border border-line/60 hover:border-line text-xs transition duration-150 cursor-pointer text-ink font-medium shadow-sm hover:shadow"
          :title="t('header.repo.switch')"
        >
          <FolderGit2 class="w-3.5 h-3.5 text-accent" />
          <span class="max-w-[88px] xl:max-w-[160px] truncate font-semibold tracking-tight">{{ repoName }}</span>
          <span
            v-if="isDirty"
            class="w-1.5 h-1.5 rounded-full bg-warning-strong animate-pulse"
            :title="t('header.repo.dirty')"
          ></span>
        </button>

        <!-- Project Switcher Modal Dropdown -->
        <div
          v-if="showRepoMenu"
          data-header-menu
          data-glass
            class="absolute left-0 top-full mt-2 w-80 rounded-xl border glass glass-panel glass-popover p-2 z-50 flex flex-col gap-1.5"
        >
          <div class="flex items-center justify-between px-2 pt-1 pb-1.5 border-b border-line/60">
            <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">{{ t('header.repo.section') }}</span>
            <button
              @click="emit('open-dialog'); showRepoMenu = false"
              class="flex items-center gap-1 text-[11px] text-accent hover:text-accent-strong transition cursor-pointer"
            >
              <FolderOpen class="w-3 h-3" />
              <span>{{ t('header.repo.open.local') }}</span>
            </button>
          </div>

          <div class="relative px-1">
            <input
              v-model="repoSearchFilter"
              type="text"
              :placeholder="t('header.repo.search')"
              class="w-full pl-7 pr-2 py-1.5 rounded-md bg-canvas/80 border border-line/70 text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <Search class="w-3.5 h-3.5 text-ink-muted absolute left-3 top-2.5 pointer-events-none" />
          </div>

          <div class="max-h-56 overflow-y-auto space-y-0.5 px-1 py-1">
            <div
              v-for="(repo, index) in filteredRecentRepos"
              :key="repo"
              :ref="(el) => setRepoItemRef(repo, el)"
              :tabindex="repoTabindexFor(repo, index)"
              @click="selectRepo(repo)"
              @focus="onRepoRowFocus(repo)"
              @keydown="onRepoRowKeydown($event, index)"
              @keydown.enter.self="selectRepo(repo)"
              class="group flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-raised/80 cursor-pointer transition text-xs focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
              :class="{ 'bg-raised text-accent font-semibold': repo === currentRepoPath }"
            >
              <div class="flex items-center gap-2 truncate">
                <FolderGit2 class="w-3.5 h-3.5 text-ink-muted group-hover:text-accent shrink-0" />
                <span class="truncate">{{ repo.split('/').pop() || repo }}</span>
              </div>
              <span class="text-[10px] text-ink-muted truncate max-w-[100px] ml-2">{{ repo }}</span>
            </div>
            <div
              v-if="filteredRecentRepos.length === 0"
              class="py-4 text-center text-xs text-ink-muted"
            >
              {{ t('header.repo.empty') }}
            </div>
          </div>

          <!-- Large-project entry points: blobless clone / init -->
          <div class="grid grid-cols-2 gap-1.5 px-1 pt-1.5 mt-1 border-t border-line/60">
            <button
              @click="emit('clone-repo'); showRepoMenu = false"
              class="flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg bg-canvas hover:bg-raised border border-line/70 text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer"
              :title="t('header.repo.clone.title')"
            >
              <Download class="w-3 h-3 text-cyan" />
              <span>{{ t('header.repo.clone') }}</span>
            </button>
            <button
              @click="emit('init-repo'); showRepoMenu = false"
              class="flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg bg-canvas hover:bg-raised border border-line/70 text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer"
              :title="t('header.repo.init.title')"
            >
              <FolderPlus class="w-3 h-3 text-success" />
              <span>{{ t('header.repo.init') }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Current Branch Switcher Pill -->
      <div v-if="currentRepoPath" class="relative">
        <button
          data-header-menu-trigger
          @click="toggleHeaderMenu('branch')"
          class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line/60 text-xs transition duration-150 cursor-pointer text-ink font-mono"
          :title="t('header.branch.switch')"
        >
          <GitBranch class="w-3.5 h-3.5 text-cyan" />
          <span class="max-w-[72px] xl:max-w-[130px] truncate font-medium">{{ repoStatus?.currentBranch || 'main' }}</span>
        </button>

        <!-- Branch Dropdown -->
        <div
          v-if="showBranchMenu"
          data-header-menu
          data-glass
            class="absolute left-0 top-full mt-2 w-64 rounded-xl border glass glass-panel glass-popover p-2 z-50 flex flex-col gap-1.5"
        >
          <div class="flex items-center justify-between px-2 pt-1 pb-1.5 border-b border-line/60">
            <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">{{ t('header.branch.section') }}</span>
          </div>

          <div class="relative px-1">
            <input
              v-model="branchSearchFilter"
              type="text"
              :placeholder="t('header.branch.filter')"
              class="w-full pl-7 pr-2 py-1.5 rounded-md bg-canvas/80 border border-line/70 text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <Search class="w-3.5 h-3.5 text-ink-muted absolute left-3 top-2.5 pointer-events-none" />
          </div>

          <div class="max-h-56 overflow-y-auto space-y-0.5 px-1 py-1">
            <div
              v-for="(b, index) in filteredBranches"
              :key="b.name"
              :ref="(el) => setBranchItemRef(b, el)"
              :tabindex="branchTabindexFor(b, index)"
              @click="selectBranch(b.name)"
              @focus="onBranchRowFocus(b)"
              @keydown="onBranchRowKeydown($event, index)"
              @keydown.enter.self="selectBranch(b.name)"
              class="group/b flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-raised/80 cursor-pointer transition text-xs font-mono focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
              :class="{ 'bg-raised text-accent font-semibold': b.isCurrent }"
            >
              <div class="flex items-center gap-1.5 truncate">
                <GitBranch class="w-3 h-3 text-ink-muted shrink-0" />
                <span class="truncate">{{ b.name }}</span>
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <span v-if="b.isCurrent" class="text-[10px] text-accent font-sans px-1.5 py-0.5 rounded bg-accent/10">{{ t('header.branch.current') }}</span>
                <!-- Quick Rebase / Merge shortcuts for other branches (also revealed
                     by keyboard focus, not only hover) -->
                <div v-else class="hidden group-hover/b:flex group-focus-within/b:flex items-center gap-1 font-sans">
                  <button
                    type="button"
                    @click.stop="emit('open-rebase', b.name); showBranchMenu = false"
                    class="px-1.5 py-0.5 rounded bg-warning/15 hover:bg-warning/25 text-warning text-[10px] font-semibold transition"
                    :title="t('header.branch.rebase.title')"
                  >
                    {{ t('header.branch.rebase') }}
                  </button>
                  <button
                    type="button"
                    @click.stop="emit('open-merge', b.name); showBranchMenu = false"
                    class="px-1.5 py-0.5 rounded bg-accent/15 hover:bg-accent/25 text-accent text-[10px] font-semibold transition"
                    :title="t('header.branch.merge.title')"
                  >
                    {{ t('header.branch.merge') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Work Context Session Pill -->
      <div v-if="currentRepoPath" class="relative">
        <button
          data-testid="context-session-dropdown"
          data-header-menu-trigger
          @click="toggleHeaderMenu('context')"
          class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line/60 text-xs transition duration-150 cursor-pointer text-ink font-mono"
          :title="t('header.context.title')"
        >
          <Bookmark class="w-3.5 h-3.5 text-accent" />
          <span class="max-w-[110px] truncate font-medium">{{ t('header.context.label') }}</span>
          <span v-if="contextSessions.length > 0" class="px-1 rounded bg-accent/15 text-accent text-[9px] font-bold">
            {{ contextSessions.length }}
          </span>
        </button>

        <div
          v-if="showContextSessionMenu"
          data-header-menu
          data-glass
          class="absolute left-0 top-full mt-2 w-72 rounded-xl border glass glass-panel glass-popover p-2 z-50 flex flex-col gap-1.5"
        >
          <div class="flex items-center justify-between px-2 pt-1 pb-1.5 border-b border-line/60">
            <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">{{ t('header.context.section') }}</span>
          </div>

          <div class="flex items-center gap-1.5 px-1">
            <input
              v-model="newSessionName"
              type="text"
              :placeholder="t('header.context.new.placeholder')"
              @keydown.enter="handleSaveSession"
              class="flex-1 px-2 py-1 rounded-md bg-canvas/80 border border-line/70 text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <button
              type="button"
              data-testid="save-context-session-btn"
              :aria-label="t('common.save')"
              :disabled="!newSessionName.trim() || isSavingSession"
              @click="handleSaveSession"
              class="px-2 py-1 rounded bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold cursor-pointer transition disabled:opacity-50"
            >
              <Plus class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          </div>

          <div v-if="contextSessions.length === 0" class="py-4 text-center text-xs text-ink-muted">
            {{ t('header.context.empty') }}
          </div>
          <div v-else class="max-h-56 overflow-y-auto space-y-0.5 px-1 py-1">
            <div
              v-for="s in contextSessions"
              :key="s.id"
              class="flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-raised/80 cursor-pointer transition text-xs font-mono"
            >
              <div class="flex flex-col min-w-0">
                <span class="text-ink font-semibold truncate">{{ s.name }}</span>
                <span class="text-[10px] text-ink-muted truncate">{{ s.branch }} · {{ s.updatedAt.slice(0, 10) }}</span>
              </div>
              <button
                type="button"
                @click.stop="handleDeleteSession(s.id)"
                class="p-1 rounded text-ink-muted hover:text-danger transition cursor-pointer"
                :title="t('common.delete')"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Center: Global Command Palette Quick Trigger -->
    <div class="no-drag flex-1 min-w-0 flex items-center justify-center" style="-webkit-app-region: no-drag;">
      <!--
        The label must never wrap: a wrapped placeholder used to grow the button
        past the 48px header and push the whole bar out of shape at narrow widths.
      -->
      <button
        @click="emit('toggle-command-palette')"
        class="flex items-center gap-2 min-w-0 max-w-full px-2.5 py-1 rounded-lg bg-canvas/60 hover:bg-canvas border border-line/70 text-ink-secondary hover:text-ink text-xs transition duration-150 cursor-pointer shadow-inner whitespace-nowrap"
        :title="t('header.search.placeholder')"
      >
        <Search class="w-3.5 h-3.5 text-ink-muted shrink-0" />
        <span class="hidden lg:inline truncate font-normal text-ink-muted">{{ t('header.search.placeholder') }}</span>
        <kbd class="hidden lg:flex ml-2 px-1.5 py-0.5 rounded bg-surface border border-line text-[10px] text-ink-muted font-mono items-center gap-0.5 shrink-0">
          <Command class="w-2.5 h-2.5" /> K
        </kbd>
      </button>
    </div>

    <!-- Right: Sync Controls & Status -->
    <div class="flex items-center gap-1.5 no-drag shrink-0" style="-webkit-app-region: no-drag;">
      <!-- Fetch / Pull / Rebase: wide windows only, collapsed into the menu below `xl` -->
      <div class="hidden xl:flex items-center gap-1.5">
      <!-- Fetch Button -->
      <button
        @click="emit('sync-fetch')"
        :disabled="isSyncing"
        class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line/60 text-xs text-ink transition cursor-pointer disabled:opacity-50 whitespace-nowrap"
        :title="t('header.sync.fetch')"
      >
        <RotateCw class="w-3 h-3" :class="{ 'animate-spin': isSyncing }" />
        <span>Fetch</span>
      </button>

      <!-- Pull Split Button -->
      <div class="flex items-center rounded-md bg-surface border border-line/60 overflow-hidden shadow-xs">
        <button
          @click="emit('sync-pull')"
          :disabled="isSyncing"
          class="flex items-center gap-1 px-2.5 py-1 hover:bg-raised text-xs text-ink transition cursor-pointer disabled:opacity-50 border-r border-line/40 whitespace-nowrap"
          :title="t('header.sync.pull')"
        >
          <ArrowDownCircle class="w-3.5 h-3.5 text-cyan" />
          <span>Pull</span>
          <span
            v-if="(repoStatus?.behind || 0) > 0"
            class="px-1 rounded-full bg-cyan/20 text-cyan text-[10px] font-bold"
          >
            {{ repoStatus?.behind }}
          </span>
        </button>
        <button :aria-label="t('header.sync.pull.advanced')"
          @click="emit('open-sync-modal', 'pull')"
          class="px-1 py-1 hover:bg-raised text-ink-muted hover:text-cyan transition cursor-pointer"
          :title="t('header.sync.pull.advanced')"
        >
          <ChevronDown class="w-3 h-3" />
        </button>
      </div>

      <!-- Rebase Quick Action Button -->
      <button
        @click="emit('open-rebase')"
        :disabled="isSyncing"
        class="flex items-center gap-1 px-2.5 py-1 rounded-md transition cursor-pointer border text-xs font-semibold shadow-xs whitespace-nowrap"
        :class="
          repoStatus?.isRebasing
            ? 'bg-warning/20 border-warning text-warning animate-pulse'
            : 'bg-surface hover:bg-raised border-line/60 text-ink'
        "
        :title="repoStatus?.isRebasing ? t('header.sync.rebasing.title', { step: repoStatus.rebaseStep ?? 0, total: repoStatus.rebaseTotal ?? 0 }) : t('header.sync.rebase.title')"
      >
        <GitPullRequest class="w-3.5 h-3.5 text-warning" />
        <span>{{ repoStatus?.isRebasing ? t('header.sync.rebasing', { step: repoStatus.rebaseStep ?? 0, total: repoStatus.rebaseTotal ?? 0 }) : t('header.sync.rebase') }}</span>
      </button>
      </div>

      <!-- Push Split Button -->
      <div class="flex items-center rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast shadow-xs overflow-hidden">
        <button
          @click="emit('sync-push')"
          :disabled="isSyncing"
          class="flex items-center gap-1 px-2.5 py-1 font-medium text-xs transition cursor-pointer disabled:opacity-50 border-r border-accent-contrast/25"
          :title="t('header.sync.push')"
        >
          <ArrowUpCircle class="w-3.5 h-3.5" />
          <span>Push</span>
          <span
            v-if="(repoStatus?.ahead || 0) > 0"
            class="px-1 rounded-full ring-1 ring-inset ring-accent-contrast/40 text-accent-contrast text-[10px] font-bold"
          >
            {{ repoStatus?.ahead }}
          </span>
        </button>
        <button :aria-label="t('header.sync.push.advanced')"
          @click="emit('open-sync-modal', 'push')"
          class="px-1 py-1 hover:bg-accent-strong-hover text-accent-contrast/80 hover:text-accent-contrast transition cursor-pointer"
          :title="t('header.sync.push.advanced')"
        >
          <ChevronDown class="w-3 h-3" />
        </button>
      </div>

      <!-- Global Refresh -->
      <button :aria-label="t('header.sync.refresh')"
        @click="emit('refresh')"
        class="hidden xl:inline-flex p-1.5 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
        :title="t('header.sync.refresh')"
      >
        <RotateCw class="w-3.5 h-3.5" />
      </button>

      <!-- Narrow-window overflow: same actions, one entry point -->
      <div class="relative xl:hidden">
        <button
          :aria-label="t('header.overflow.title')"
          :aria-expanded="showActionMenu"
          data-testid="header-overflow-trigger"
          data-header-menu-trigger
          @click="toggleHeaderMenu('action')"
          class="p-1.5 rounded-md bg-surface hover:bg-raised border border-line/60 text-ink-muted hover:text-ink transition cursor-pointer"
          :title="t('header.overflow.title')"
        >
          <EllipsisVertical class="w-3.5 h-3.5" />
        </button>

        <div
          v-if="showActionMenu"
          data-header-menu
          data-glass
          data-testid="header-overflow-menu"
          class="absolute right-0 top-full mt-2 w-48 rounded-xl border glass glass-panel glass-popover p-1.5 z-50 flex flex-col gap-0.5"
        >
          <button
            @click="showActionMenu = false; emit('sync-fetch')"
            :disabled="isSyncing"
            class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-raised text-xs text-ink transition cursor-pointer disabled:opacity-50 text-left"
          >
            <RotateCw class="w-3 h-3 text-ink-muted" :class="{ 'animate-spin': isSyncing }" />
            <span>{{ t('header.sync.fetch') }}</span>
          </button>
          <button
            @click="showActionMenu = false; emit('sync-pull')"
            :disabled="isSyncing"
            class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-raised text-xs text-ink transition cursor-pointer disabled:opacity-50 text-left"
          >
            <ArrowDownCircle class="w-3 h-3 text-cyan" />
            <span>{{ t('header.sync.pull') }}</span>
            <span
              v-if="(repoStatus?.behind || 0) > 0"
              class="ml-auto px-1 rounded-full bg-cyan/20 text-cyan text-[10px] font-bold"
            >
              {{ repoStatus?.behind }}
            </span>
          </button>
          <button
            @click="showActionMenu = false; emit('open-rebase')"
            :disabled="isSyncing"
            class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-raised text-xs text-ink transition cursor-pointer disabled:opacity-50 text-left"
          >
            <GitPullRequest class="w-3 h-3 text-warning" />
            <span>{{ t('header.sync.rebase') }}</span>
          </button>
          <button
            @click="showActionMenu = false; emit('refresh')"
            class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-raised text-xs text-ink transition cursor-pointer text-left"
          >
            <RotateCw class="w-3 h-3 text-ink-muted" />
            <span>{{ t('header.sync.refresh') }}</span>
          </button>
        </div>
      </div>
    </div>
  </header>
</template>
