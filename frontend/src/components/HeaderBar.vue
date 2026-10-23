<script setup lang="ts">
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

const showRepoMenu = ref(false);
const showBranchMenu = ref(false);
const recentRepos = ref<string[]>([]);
const branches = ref<BranchInfo[]>([]);
const repoSearchFilter = ref("");
const branchSearchFilter = ref("");

const repoName = computed(() => {
  if (!props.currentRepoPath) return "未打开项目";
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
    console.error("Failed to load recent repos:", err);
  }
}

async function loadBranches() {
  if (!props.currentRepoPath) return;
  try {
    const list = await gitService.getBranches(props.currentRepoPath);
    if (list) branches.value = list;
  } catch (err) {
    console.error("Failed to load branches:", err);
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
  <header
    class="h-12 border-b border-line bg-surface/70 backdrop-blur-xl flex items-center justify-between px-3 select-none z-30 shrink-0"
    style="-webkit-app-region: drag;"
  >
    <!-- Left: macOS traffic lights offset + Repo & Branch Switcher -->
    <div class="flex items-center gap-2 pl-18 no-drag" style="-webkit-app-region: no-drag;">
      <!-- Project Switcher Pill -->
      <div class="relative">
        <button
          @click="showRepoMenu = !showRepoMenu; if (showRepoMenu) loadRecents()"
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-raised/70 hover:bg-raised border border-line/60 hover:border-line text-xs transition duration-150 cursor-pointer text-ink font-medium shadow-sm hover:shadow"
          title="切换项目 / 打开已有仓库"
        >
          <FolderGit2 class="w-3.5 h-3.5 text-accent" />
          <span class="max-w-[160px] truncate font-semibold tracking-tight">{{ repoName }}</span>
          <span
            v-if="isDirty"
            class="w-1.5 h-1.5 rounded-full bg-warning-strong animate-pulse"
            title="工作区有未提交变更"
          ></span>
        </button>

        <!-- Project Switcher Modal Dropdown -->
        <div
          v-if="showRepoMenu"
          class="absolute left-0 top-full mt-2 w-80 rounded-xl border border-line bg-surface/95 backdrop-blur-2xl shadow-2xl p-2 z-50 flex flex-col gap-1.5"
        >
          <div class="flex items-center justify-between px-2 pt-1 pb-1.5 border-b border-line/60">
            <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">项目切换</span>
            <button
              @click="emit('open-dialog'); showRepoMenu = false"
              class="flex items-center gap-1 text-[11px] text-accent hover:text-accent-strong transition cursor-pointer"
            >
              <FolderOpen class="w-3 h-3" />
              <span>打开本地目录</span>
            </button>
          </div>

          <div class="relative px-1">
            <input
              v-model="repoSearchFilter"
              type="text"
              placeholder="搜索项目路径..."
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
              暂无最近项目，请克隆或初始化一个仓库
            </div>
          </div>

          <!-- Large-project entry points: blobless clone / init -->
          <div class="grid grid-cols-2 gap-1.5 px-1 pt-1.5 mt-1 border-t border-line/60">
            <button
              @click="emit('clone-repo'); showRepoMenu = false"
              class="flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg bg-canvas hover:bg-raised border border-line/70 text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer"
              title="克隆远程仓库（支持 Blobless / 浅克隆）"
            >
              <Download class="w-3 h-3 text-cyan" />
              <span>克隆仓库</span>
            </button>
            <button
              @click="emit('init-repo'); showRepoMenu = false"
              class="flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg bg-canvas hover:bg-raised border border-line/70 text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer"
              title="在本地目录初始化仓库"
            >
              <FolderPlus class="w-3 h-3 text-success" />
              <span>初始化</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Current Branch Switcher Pill -->
      <div v-if="currentRepoPath" class="relative">
        <button
          @click="showBranchMenu = !showBranchMenu; if (showBranchMenu) loadBranches()"
          class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line/60 text-xs transition duration-150 cursor-pointer text-ink font-mono"
          title="切换分支"
        >
          <GitBranch class="w-3.5 h-3.5 text-cyan" />
          <span class="max-w-[130px] truncate font-medium">{{ repoStatus?.currentBranch || 'main' }}</span>
        </button>

        <!-- Branch Dropdown -->
        <div
          v-if="showBranchMenu"
          class="absolute left-0 top-full mt-2 w-64 rounded-xl border border-line bg-surface/95 backdrop-blur-2xl shadow-2xl p-2 z-50 flex flex-col gap-1.5"
        >
          <div class="flex items-center justify-between px-2 pt-1 pb-1.5 border-b border-line/60">
            <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">切换分支</span>
          </div>

          <div class="relative px-1">
            <input
              v-model="branchSearchFilter"
              type="text"
              placeholder="过滤分支名称..."
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
                <span v-if="b.isCurrent" class="text-[10px] text-accent font-sans px-1.5 py-0.5 rounded bg-accent/10">当前</span>
                <!-- Quick Rebase / Merge shortcuts for other branches -->
                <div v-else class="hidden group-hover/b:flex items-center gap-1 font-sans">
                  <button
                    type="button"
                    @click.stop="emit('open-rebase', b.name); showBranchMenu = false"
                    class="px-1.5 py-0.5 rounded bg-warning/15 hover:bg-warning/25 text-warning text-[10px] font-semibold transition"
                    title="变基当前分支到此分支 (Rebase)"
                  >
                    变基
                  </button>
                  <button
                    type="button"
                    @click.stop="emit('open-merge', b.name); showBranchMenu = false"
                    class="px-1.5 py-0.5 rounded bg-accent/15 hover:bg-accent/25 text-accent text-[10px] font-semibold transition"
                    title="合并该分支到当前分支 (Merge)"
                  >
                    合并
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
        <span class="font-normal text-ink-muted">搜索文件、分支与工作室工具...</span>
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
        title="获取远端更新 (Fetch)"
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
          title="快速拉取 (Pull)"
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
        <button aria-label="高级拉取设置与变基策略..."
          @click="emit('open-sync-modal', 'pull')"
          class="px-1 py-1 hover:bg-raised text-ink-muted hover:text-cyan transition cursor-pointer"
          title="高级拉取设置与变基策略..."
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
        :title="repoStatus?.isRebasing ? `正在变基：步骤 ${repoStatus.rebaseStep}/${repoStatus.rebaseTotal}` : '变基当前分支到上游 (Rebase ⌘R)'"
      >
        <GitPullRequest class="w-3.5 h-3.5 text-warning" />
        <span class="hidden sm:inline">{{ repoStatus?.isRebasing ? `变基中 (${repoStatus.rebaseStep}/${repoStatus.rebaseTotal})` : '变基' }}</span>
      </button>

      <!-- Push Split Button -->
      <div class="flex items-center rounded-md bg-accent-strong hover:bg-accent-strong-hover text-white shadow-xs overflow-hidden">
        <button
          @click="emit('sync-push')"
          :disabled="isSyncing"
          class="flex items-center gap-1 px-2.5 py-1 font-medium text-xs transition cursor-pointer disabled:opacity-50 border-r border-white/20"
          title="快速推送 (Push)"
        >
          <ArrowUpCircle class="w-3.5 h-3.5" />
          <span>Push</span>
          <span
            v-if="(repoStatus?.ahead || 0) > 0"
            class="px-1 rounded-full bg-white/20 text-white text-[10px] font-bold"
          >
            {{ repoStatus?.ahead }}
          </span>
        </button>
        <button aria-label="高级推送设置 (查看待推提交/租约强推/推送标签)..."
          @click="emit('open-sync-modal', 'push')"
          class="px-1 py-1 hover:bg-accent-strong-hover text-white/80 hover:text-white transition cursor-pointer"
          title="高级推送设置 (查看待推提交/租约强推/推送标签)..."
        >
          <ChevronDown class="w-3 h-3" />
        </button>
      </div>

      <!-- Global Refresh -->
      <button aria-label="刷新状态"
        @click="emit('refresh')"
        class="p-1.5 rounded-md hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
        title="刷新状态"
      >
        <RotateCw class="w-3.5 h-3.5" />
      </button>
    </div>
  </header>
</template>
