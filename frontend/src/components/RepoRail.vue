<script setup lang="ts">
import { computed } from "vue";
import { FolderGit2, FolderOpen, X, Plus, LayoutDashboard, ChevronDown } from "lucide-vue-next";
import type { RepoStatus } from "../types";

const props = defineProps<{
  repos: string[];
  activeRepo: string;
  statuses: Record<string, RepoStatus | null>;
  dashboardOpen?: boolean;
}>();

const emit = defineEmits<{
  (e: "select", path: string): void;
  (e: "close", path: string): void;
  (e: "open-dialog"): void;
  (e: "open-new"): void;
  (e: "toggle-dashboard"): void;
}>();

function repoName(path: string): string {
  const parts = path.replace(/\\/g, "/").split("/").filter(Boolean);
  return parts[parts.length - 1] || path;
}

function isDirty(path: string): boolean {
  const s = props.statuses[path];
  if (!s) return false;
  return (
    (s.stagedFiles?.length || 0) > 0 ||
    (s.unstagedFiles?.length || 0) > 0 ||
    (s.untrackedFiles?.length || 0) > 0 ||
    (s.conflictedFiles?.length || 0) > 0
  );
}

function statusFor(path: string): RepoStatus | null {
  return props.statuses[path] ?? null;
}

const dirtyCount = computed(() => props.repos.filter((p) => isDirty(p)).length);
</script>

<template>
  <div class="h-9 border-b border-line bg-surface/40 backdrop-blur-md flex items-center gap-1 px-2 select-none shrink-0 z-20">
    <!-- Open repository tabs -->
    <div class="flex items-center gap-1 overflow-x-auto min-w-0 flex-1 scrollbar-thin">
      <button
        v-for="(repo, idx) in repos"
        :key="repo"
        @click="emit('select', repo)"
        class="group relative flex items-center gap-1.5 px-2.5 py-1 rounded-lg border text-[11px] transition cursor-pointer shrink-0 max-w-[220px]"
        :class="
          repo === activeRepo
            ? 'bg-raised border-line text-ink font-semibold shadow-xs'
            : 'bg-transparent border-transparent text-ink-secondary hover:text-ink hover:bg-raised/60'
        "
        :title="repo + (statusFor(repo)?.currentBranch ? '\n分支: ' + statusFor(repo)?.currentBranch : '')"
      >
        <FolderGit2 class="w-3 h-3 shrink-0" :class="repo === activeRepo ? 'text-accent' : 'text-ink-muted'" />
        <span class="truncate font-mono">{{ repoName(repo) }}</span>

        <!-- dirty badge -->
        <span
          v-if="isDirty(repo)"
          class="w-1.5 h-1.5 rounded-full bg-warning-strong shrink-0"
          title="有未提交变更"
        ></span>

        <!-- ahead/behind -->
        <span
          v-if="statusFor(repo) && ((statusFor(repo)!.ahead || 0) > 0 || (statusFor(repo)!.behind || 0) > 0)"
          class="text-[9px] font-mono text-ink-muted shrink-0 flex items-center gap-0.5"
        >
          <span v-if="(statusFor(repo)!.ahead || 0) > 0" class="text-cyan">↑{{ statusFor(repo)!.ahead }}</span>
          <span v-if="(statusFor(repo)!.behind || 0) > 0" class="text-warning">↓{{ statusFor(repo)!.behind }}</span>
        </span>

        <!-- close button -->
        <button
          v-if="repos.length > 1 || repo === activeRepo"
          @click.stop="emit('close', repo)"
          class="p-0.5 rounded text-ink-muted hover:text-danger hover:bg-danger/15 transition cursor-pointer shrink-0 opacity-0 group-hover:opacity-100"
          :class="{ 'opacity-100': repo === activeRepo }"
          title="关闭该仓库标签页"
        >
          <X class="w-2.5 h-2.5" />
        </button>

        <!-- shortcut hint -->
        <span
          v-if="idx < 9 && repo === activeRepo"
          class="text-[8px] font-mono text-ink-muted/70 shrink-0 hidden sm:inline"
          title="⌘/Ctrl + 数字快速切换"
        >
          ⌘{{ idx + 1 }}
        </span>
      </button>

      <div v-if="repos.length === 0" class="text-[11px] text-ink-muted px-2">尚未打开仓库</div>
    </div>

    <!-- actions -->
    <div class="flex items-center gap-1 shrink-0 pl-1 border-l border-line/60">
      <span v-if="dirtyCount > 0" class="text-[10px] text-warning font-medium mr-1">{{ dirtyCount }} 个仓库有未提交</span>
      <button
        @click="emit('open-dialog')"
        class="flex items-center gap-1 px-2 py-1 rounded-md bg-canvas hover:bg-raised border border-line/70 text-[10px] text-accent hover:text-accent-strong transition cursor-pointer"
        title="选择本地目录打开仓库 (⌘O)"
      >
        <FolderOpen class="w-3 h-3" />
        <span class="hidden md:inline">打开目录</span>
      </button>
      <button
        @click="emit('open-new')"
        class="flex items-center gap-1 px-2 py-1 rounded-md bg-canvas hover:bg-raised border border-line/70 text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer"
        title="打开欢迎页 / 克隆 / 初始化仓库"
      >
        <Plus class="w-3 h-3" />
        <span class="hidden md:inline">更多</span>
        <ChevronDown class="w-2.5 h-2.5 text-ink-muted" />
      </button>
      <button
        @click="emit('toggle-dashboard')"
        class="flex items-center gap-1 px-2 py-1 rounded-md transition cursor-pointer border text-[10px]"
        :class="
          dashboardOpen
            ? 'bg-accent/15 border-accent/40 text-accent'
            : 'bg-canvas hover:bg-raised border-line/70 text-ink-secondary hover:text-ink'
        "
        title="多仓库总览（全部分支状态 / 批量 Fetch）"
      >
        <LayoutDashboard class="w-3 h-3" />
        <span class="hidden md:inline">多仓总览</span>
      </button>
    </div>
  </div>
</template>
