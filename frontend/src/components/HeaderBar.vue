<script setup lang="ts">
import { reportError } from "../composables/useToast";
import { ref, onMounted, computed } from "vue";
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
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import { useWindowDrag } from "../composables/useWindowChrome";
import { t } from "../i18n";
import type { RepoStatus, BranchInfo } from "../types";

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

onMounted(() => {
  loadRecents();
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
  <header
    ref="headerRef"
    class="h-12 border-b border-line glass glass-bar flex items-center justify-between px-3 select-none z-30 shrink-0"
    :style="dragStyle"
  >
    <!-- Left: macOS traffic lights offset + Repo & Branch Switcher -->
    <div class="flex items-center gap-2 pl-18 no-drag" style="-webkit-app-region: no-drag;">
      <!-- Project Switcher Pill -->
      <div class="relative">
        <button
          @click="showRepoMenu = !showRepoMenu; if (showRepoMenu) loadRecents()"
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-raised/70 hover:bg-raised border border-line/60 hover:border-line text-xs transition duration-150 cursor-pointer text-ink font-medium shadow-sm hover:shadow"
          :title="t('header.repo.switch')"
        >
          <FolderGit2 class="w-3.5 h-3.5 text-accent" />
          <span class="max-w-[160px] truncate font-semibold tracking-tight">{{ repoName }}</span>
          <span
            v-if="isDirty"
            class="w-1.5 h-1.5 rounded-full bg-warning-strong animate-pulse"
            :title="t('header.repo.dirty')"
          ></span>
        </button>

        <!-- Project Switcher Modal Dropdown -->
        <div
          v-if="showRepoMenu"
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
              v-for="repo in filteredRecentRepos"
              :key="repo"
              @click="selectRepo(repo)"
              class="group flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-raised/80 cursor-pointer transition text-xs"
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
          @click="showBranchMenu = !showBranchMenu; if (showBranchMenu) loadBranches()"
          class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line/60 text-xs transition duration-150 cursor-pointer text-ink font-mono"
          :title="t('header.branch.switch')"
        >
          <GitBranch class="w-3.5 h-3.5 text-cyan" />
          <span class="max-w-[130px] truncate font-medium">{{ repoStatus?.currentBranch || 'main' }}</span>
        </button>

        <!-- Branch Dropdown -->
        <div
          v-if="showBranchMenu"
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
              v-for="b in filteredBranches"
              :key="b.name"
              @click="selectBranch(b.name)"
              class="group/b flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-raised/80 cursor-pointer transition text-xs font-mono"
              :class="{ 'bg-raised text-accent font-semibold': b.isCurrent }"
            >
              <div class="flex items-center gap-1.5 truncate">
                <GitBranch class="w-3 h-3 text-ink-muted shrink-0" />
                <span class="truncate">{{ b.name }}</span>
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <span v-if="b.isCurrent" class="text-[10px] text-accent font-sans px-1.5 py-0.5 rounded bg-accent/10">{{ t('header.branch.current') }}</span>
                <!-- Quick Rebase / Merge shortcuts for other branches -->
                <div v-else class="hidden group-hover/b:flex items-center gap-1 font-sans">
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
    </div>

    <!-- Center: Global Command Palette Quick Trigger -->
    <div class="no-drag flex items-center" style="-webkit-app-region: no-drag;">
      <button
        @click="emit('toggle-command-palette')"
        class="flex items-center gap-2 px-3 py-1 rounded-lg bg-canvas/60 hover:bg-canvas border border-line/70 text-ink-secondary hover:text-ink text-xs transition duration-150 cursor-pointer shadow-inner"
      >
        <Search class="w-3.5 h-3.5 text-ink-muted" />
        <span class="font-normal text-ink-muted">{{ t('header.search.placeholder') }}</span>
        <kbd class="ml-4 px-1.5 py-0.5 rounded bg-surface border border-line text-[10px] text-ink-muted font-mono flex items-center gap-0.5">
          <Command class="w-2.5 h-2.5" /> K
        </kbd>
      </button>
    </div>

    <!-- Right: Sync Controls & Status -->
    <div class="flex items-center gap-1.5 no-drag" style="-webkit-app-region: no-drag;">
      <!-- Fetch Button -->
      <button
        @click="emit('sync-fetch')"
        :disabled="isSyncing"
        class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line/60 text-xs text-ink transition cursor-pointer disabled:opacity-50"
        :title="t('header.sync.fetch')"
      >
        <RotateCw class="w-3 h-3" :class="{ 'animate-spin': isSyncing }" />
        <span class="hidden sm:inline">Fetch</span>
      </button>

      <!-- Pull Split Button -->
      <div class="flex items-center rounded-md bg-surface border border-line/60 overflow-hidden shadow-xs">
        <button
          @click="emit('sync-pull')"
          :disabled="isSyncing"
          class="flex items-center gap-1 px-2.5 py-1 hover:bg-raised text-xs text-ink transition cursor-pointer disabled:opacity-50 border-r border-line/40"
          :title="t('header.sync.pull')"
        >
          <ArrowDownCircle class="w-3.5 h-3.5 text-cyan" />
          <span class="hidden sm:inline">Pull</span>
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
        class="flex items-center gap-1 px-2.5 py-1 rounded-md transition cursor-pointer border text-xs font-semibold shadow-xs"
        :class="
          repoStatus?.isRebasing
            ? 'bg-warning/20 border-warning text-warning animate-pulse'
            : 'bg-surface hover:bg-raised border-line/60 text-ink'
        "
        :title="repoStatus?.isRebasing ? t('header.sync.rebasing.title', { step: repoStatus.rebaseStep ?? 0, total: repoStatus.rebaseTotal ?? 0 }) : t('header.sync.rebase.title')"
      >
        <GitPullRequest class="w-3.5 h-3.5 text-warning" />
        <span class="hidden sm:inline">{{ repoStatus?.isRebasing ? t('header.sync.rebasing', { step: repoStatus.rebaseStep ?? 0, total: repoStatus.rebaseTotal ?? 0 }) : t('header.sync.rebase') }}</span>
      </button>

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
        class="p-1.5 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
        :title="t('header.sync.refresh')"
      >
        <RotateCw class="w-3.5 h-3.5" />
      </button>
    </div>
  </header>
</template>
