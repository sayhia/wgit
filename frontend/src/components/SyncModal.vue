<script setup lang="ts">
import { confirmAction } from "../composables/useConfirm";
import { ref, computed, watch } from "vue";
import { useModalA11y } from "../composables/useModalA11y";
import {
  X,
  ArrowDownCircle,
  ArrowUpCircle,
  RotateCw,
  GitPullRequest,
  CheckCircle2,
  AlertTriangle,
  Loader2,
  Globe,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import type {
  RepoStatus,
  RemoteDetail,
  CommitItem,
  PullAdvancedOptions,
  PushAdvancedOptions,
} from "../types";

const props = defineProps<{
  isOpen: boolean;
  initialTab?: "pull" | "push" | "fetch";
  currentRepoPath: string;
  repoStatus: RepoStatus | null;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "refresh"): void;
}>();

const { containerRef: dialogRef } = useModalA11y(() => props.isOpen, () => emit("close"));

const activeTab = ref<"pull" | "push" | "fetch">(props.initialTab || "pull");

watch(
  () => props.initialTab,
  (val) => {
    if (val) activeTab.value = val;
  }
);

// Remotes & Branches
const remotes = ref<RemoteDetail[]>([]);
const uniqueRemotes = computed(() => {
  const seen = new Set<string>();
  return remotes.value.filter((remote) => {
    if (seen.has(remote.name)) return false;
    seen.add(remote.name);
    return true;
  });
});
const selectedRemote = ref("origin");
const targetBranch = ref("");
const isLoadingRemotes = ref(false);

// Ahead / Behind Commits
const aheadCommits = ref<CommitItem[]>([]);
const behindCommits = ref<CommitItem[]>([]);
const commitsError = ref("");
const isLoadingCommits = ref(false);

// Pull Options
const pullStrategy = ref<"rebase" | "ff-only" | "merge">("merge");
const pullAutostash = ref(true);
const pullPrune = ref(true);
const pullSubmodules = ref(false);
const isPulling = ref(false);
const pullMessage = ref("");
const pullError = ref("");

// Push Options
const pushRemoteBranch = ref("");
const pushForceMode = ref<"none" | "lease" | "force">("none");
const pushSetUpstream = ref(false);
const pushTags = ref(false);
const pushFollowTags = ref(true);
const pushSkipCI = ref(false);
const isPushing = ref(false);
const pushMessage = ref("");
const pushError = ref("");

// Fetch State
const isFetchingAll = ref(false);
const fetchResult = ref("");

const currentBranch = computed(() => props.repoStatus?.currentBranch || "main");

async function loadData() {
  if (!props.currentRepoPath) return;
  isLoadingRemotes.value = true;
  pullError.value = "";
  pushError.value = "";
  try {
    const list = await gitService.getRemoteDetails(props.currentRepoPath);
    remotes.value = list || [];
    const firstRemote = remotes.value[0];
    if (firstRemote && !remotes.value.some((r) => r.name === selectedRemote.value)) {
      selectedRemote.value = firstRemote.name;
    }
    targetBranch.value = currentBranch.value;
    pushRemoteBranch.value = currentBranch.value;
  } catch (err) {
    console.debug("Failed to load remotes:", err);
  } finally {
    isLoadingRemotes.value = false;
  }

  await loadAheadBehindCommits();
}

async function loadAheadBehindCommits() {
  if (!props.currentRepoPath) return;
  isLoadingCommits.value = true;
  commitsError.value = "";
  aheadCommits.value = [];
  behindCommits.value = [];
  try {
    const [ahead, behind] = await Promise.all([
      gitService.getAheadCommits(props.currentRepoPath, selectedRemote.value, currentBranch.value),
      gitService.getBehindCommits(props.currentRepoPath, selectedRemote.value, currentBranch.value),
    ]);
    aheadCommits.value = ahead || [];
    behindCommits.value = behind || [];
  } catch (err: any) {
    commitsError.value = err?.message || String(err);
  } finally {
    isLoadingCommits.value = false;
  }
}

watch(
  () => props.isOpen,
  (open) => {
    if (open) {
      loadData();
    }
  }
);

watch(selectedRemote, () => {
  loadAheadBehindCommits();
});

async function handleExecutePull() {
  if (!props.currentRepoPath || isPulling.value) return;
  isPulling.value = true;
  pullError.value = "";
  pullMessage.value = "";
  try {
    const opts: PullAdvancedOptions = {
      remote: selectedRemote.value,
      branch: targetBranch.value.trim(),
      strategy: pullStrategy.value,
      autostash: pullAutostash.value,
      prune: pullPrune.value,
      submodules: pullSubmodules.value,
    };
    const res = await gitService.pullAdvanced(props.currentRepoPath, opts);
    if (res.hasConflicts) {
      pullError.value = `拉取产生冲突！${(res.conflicts || []).join(", ") || res.message}`;
    } else {
      pullMessage.value = res.alreadyUp
        ? "本地已经是最新版本 (Already up to date)"
        : res.fastForward
        ? "成功执行快进拉取 (Fast-forward)"
        : "拉取并合并成功完成！";
    }
    emit("refresh");
    await loadAheadBehindCommits();
  } catch (err: any) {
    pullError.value = err?.message || String(err);
  } finally {
    isPulling.value = false;
  }
}

async function handleExecutePush() {
  if (!props.currentRepoPath || isPushing.value) return;
  isPushing.value = true;
  pushError.value = "";
  pushMessage.value = "";

  if (pushForceMode.value === "force") {
    const confirmed = await confirmAction(
      { message: `【警告】您选择了完全强制推送 (--force)！这可能会永久覆盖远程仓库的历史与团队成员的提交。确定继续吗？`, tone: "danger" }
    );
    if (!confirmed) {
      isPushing.value = false;
      return;
    }
  }

  try {
    const opts: PushAdvancedOptions = {
      remote: selectedRemote.value,
      branch: currentBranch.value,
      remoteBranch: pushRemoteBranch.value.trim() || undefined,
      forceMode: pushForceMode.value,
      setUpstream: pushSetUpstream.value,
      pushTags: pushTags.value,
      followTags: pushFollowTags.value,
      skipCI: pushSkipCI.value,
    };
    const res = await gitService.pushAdvanced(props.currentRepoPath, opts);
    pushMessage.value = res.message || "推送成功完成！";
    emit("refresh");
    await loadAheadBehindCommits();
  } catch (err: any) {
    pushError.value = err?.message || String(err);
  } finally {
    isPushing.value = false;
  }
}

async function handleFetchAll() {
  if (!props.currentRepoPath || isFetchingAll.value) return;
  isFetchingAll.value = true;
  fetchResult.value = "";
  try {
    await gitService.fetch(props.currentRepoPath, selectedRemote.value);
    fetchResult.value = `已从 [${selectedRemote.value}] 获取全部最新引用与标签`;
    emit("refresh");
    await loadAheadBehindCommits();
  } catch (err: any) {
    fetchResult.value = `获取失败: ${err?.message || err}`;
  } finally {
    isFetchingAll.value = false;
  }
}
</script>

<template>
  <div
    v-if="isOpen"
    ref="dialogRef"
    role="dialog"
    aria-modal="true"
    aria-label="同步操作"
    class="fixed inset-0 z-50 bg-black/60 backdrop-blur-md flex items-center justify-center p-4 animate-in fade-in duration-150 select-none"
    @click.self="emit('close')"
  >
    <div
      class="w-full max-w-2xl bg-surface/95 border border-line rounded-2xl shadow-2xl flex flex-col overflow-hidden max-h-[90vh]"
    >
      <!-- Header -->
      <div class="px-5 py-3.5 border-b border-line flex items-center justify-between bg-raised/40 shrink-0">
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-xl bg-accent/15 text-accent flex items-center justify-center font-bold">
            <GitPullRequest class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-sm font-bold text-ink">远程同步中心 (Git Sync)</h2>
            <div class="flex items-center gap-2 text-[11px] text-ink-muted">
              <span>当前分支: <strong class="text-ink font-mono">{{ currentBranch }}</strong></span>
              <span v-if="repoStatus?.trackingBranch" class="text-accent font-mono">
                → {{ repoStatus.trackingBranch }}
              </span>
            </div>
          </div>
        </div>

        <button aria-label="关闭"
          @click="emit('close')"
          class="p-1 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Nav Tabs -->
      <div class="flex items-center gap-1 px-5 pt-3 border-b border-line/60 bg-surface shrink-0">
        <button
          @click="activeTab = 'pull'"
          class="flex items-center gap-1.5 px-4 py-2 border-b-2 text-xs font-semibold transition cursor-pointer"
          :class="
            activeTab === 'pull'
              ? 'border-cyan text-cyan'
              : 'border-transparent text-ink-secondary hover:text-ink'
          "
        >
          <ArrowDownCircle class="w-3.5 h-3.5" />
          <span>拉取 (Pull)</span>
          <span
            v-if="behindCommits.length > 0"
            class="px-1.5 py-0.2 rounded-full bg-cyan/20 text-cyan text-[10px] font-bold"
          >
            {{ behindCommits.length }}
          </span>
        </button>

        <button
          @click="activeTab = 'push'"
          class="flex items-center gap-1.5 px-4 py-2 border-b-2 text-xs font-semibold transition cursor-pointer"
          :class="
            activeTab === 'push'
              ? 'border-accent text-accent font-bold'
              : 'border-transparent text-ink-secondary hover:text-ink'
          "
        >
          <ArrowUpCircle class="w-3.5 h-3.5" />
          <span>推送 (Push)</span>
          <span
            v-if="aheadCommits.length > 0"
            class="px-1.5 py-0.2 rounded-full bg-accent/20 text-accent text-[10px] font-bold"
          >
            {{ aheadCommits.length }}
          </span>
        </button>

        <button
          @click="activeTab = 'fetch'"
          class="flex items-center gap-1.5 px-4 py-2 border-b-2 text-xs font-semibold transition cursor-pointer"
          :class="
            activeTab === 'fetch'
              ? 'border-ink text-ink font-bold'
              : 'border-transparent text-ink-secondary hover:text-ink'
          "
        >
          <RotateCw class="w-3.5 h-3.5" />
          <span>获取与远端 (Fetch & Remotes)</span>
        </button>
      </div>

      <!-- Tab Content Area -->
      <div class="flex-1 overflow-y-auto p-5 space-y-4">
        <!-- Common Remote Picker -->
        <div class="flex items-center gap-3 p-3 rounded-xl bg-canvas border border-line/60">
          <Globe class="w-4 h-4 text-ink-muted shrink-0" />
          <div class="flex-1 flex items-center gap-2">
            <span class="text-xs text-ink-secondary shrink-0">目标远程仓库:</span>
            <select
              v-model="selectedRemote"
              class="px-2.5 py-1 rounded-lg bg-surface border border-line text-xs text-ink font-semibold focus:outline-none focus:border-accent"
            >
              <option v-for="r in uniqueRemotes" :key="r.name" :value="r.name">
                {{ r.name }} ({{ r.url }})
              </option>
              <option v-if="uniqueRemotes.length === 0" value="origin">origin</option>
            </select>
          </div>
        </div>

        <div
          v-if="commitsError"
          class="flex items-start gap-2 p-3 rounded-xl border border-warning/30 bg-warning/10 text-warning text-xs"
        >
          <AlertTriangle class="w-4 h-4 shrink-0" />
          <span>无法计算领先/落后提交：{{ commitsError }}</span>
        </div>

        <!-- ============================================================== -->
        <!-- 1. PULL TAB                                                    -->
        <!-- ============================================================== -->
        <div v-if="activeTab === 'pull'" class="space-y-4">
          <!-- Strategy Cards -->
          <div class="space-y-2">
            <label class="text-xs font-bold text-ink">拉取策略 (Pull Strategy)</label>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <button
                type="button"
                @click="pullStrategy = 'merge'"
                class="p-3 rounded-xl border text-left transition cursor-pointer flex flex-col justify-between"
                :class="
                  pullStrategy === 'merge'
                    ? 'border-cyan bg-cyan/10 text-cyan shadow-sm'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div>
                  <div class="font-bold text-xs flex items-center gap-1">
                    <span>合并拉取</span>
                    <span class="text-[9px] px-1 rounded bg-cyan/20 font-mono">--no-rebase</span>
                  </div>
                  <div class="text-[10px] text-ink-muted mt-1 leading-snug">
                    分叉时显式创建合并提交，不依赖 Git 全局 pull 策略。
                  </div>
                </div>
              </button>

              <button
                type="button"
                @click="pullStrategy = 'rebase'"
                class="p-3 rounded-xl border text-left transition cursor-pointer flex flex-col justify-between"
                :class="
                  pullStrategy === 'rebase'
                    ? 'border-cyan bg-cyan/10 text-cyan shadow-sm'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div>
                  <div class="font-bold text-xs flex items-center gap-1">
                    <span>变基拉取</span>
                    <span class="text-[9px] px-1 rounded bg-cyan/20 font-mono">--rebase</span>
                  </div>
                  <div class="text-[10px] text-ink-muted mt-1 leading-snug">
                    将本地提交在线性重放至远端最新，避免多余分支合并节点。
                  </div>
                </div>
              </button>

              <button
                type="button"
                @click="pullStrategy = 'ff-only'"
                class="p-3 rounded-xl border text-left transition cursor-pointer flex flex-col justify-between"
                :class="
                  pullStrategy === 'ff-only'
                    ? 'border-cyan bg-cyan/10 text-cyan shadow-sm'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div>
                  <div class="font-bold text-xs flex items-center gap-1">
                    <span>仅快进</span>
                    <span class="text-[9px] px-1 rounded bg-cyan/20 font-mono">--ff-only</span>
                  </div>
                  <div class="text-[10px] text-ink-muted mt-1 leading-snug">
                    若发生分叉则立即拒绝合并，严格保证提交线完全向前。
                  </div>
                </div>
              </button>
            </div>
          </div>

          <!-- Options -->
          <div class="p-3.5 rounded-xl bg-canvas border border-line/60 space-y-2.5">
            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pullAutostash"
                type="checkbox"
                class="rounded border-line bg-surface text-cyan focus:ring-0"
              />
              <span>自动暂存未提交修改 (--autostash)，并在拉取完成后自动弹出还原</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pullPrune"
                type="checkbox"
                class="rounded border-line bg-surface text-cyan focus:ring-0"
              />
              <span>清理远程已删除分支引用 (--prune)</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pullSubmodules"
                type="checkbox"
                class="rounded border-line bg-surface text-cyan focus:ring-0"
              />
              <span>递归拉取子模块改动 (--recurse-submodules)</span>
            </label>
          </div>

          <!-- Behind Commits Preview -->
          <div v-if="behindCommits.length > 0" class="space-y-1.5">
            <div class="text-[11px] font-bold text-cyan flex items-center gap-1.5">
              <ArrowDownCircle class="w-3.5 h-3.5" />
              <span>远端有 {{ behindCommits.length }} 条待拉取的新提交:</span>
            </div>
            <div class="max-h-36 overflow-y-auto rounded-xl bg-canvas border border-line/60 divide-y divide-line/40 font-mono text-[11px]">
              <div
                v-for="c in behindCommits"
                :key="c.hash"
                class="p-2 flex items-center justify-between gap-2"
              >
                <div class="flex items-center gap-2 truncate">
                  <span class="text-accent font-semibold">{{ c.shortHash }}</span>
                  <span class="truncate text-ink font-sans">{{ c.subject }}</span>
                </div>
                <span class="text-[10px] text-ink-muted whitespace-nowrap font-sans">{{ c.relativeDate }}</span>
              </div>
            </div>
          </div>

          <!-- Messages -->
          <div
            v-if="pullMessage"
            class="p-3 rounded-xl bg-success/10 border border-success/30 text-success text-xs flex items-center gap-2"
          >
            <CheckCircle2 class="w-4 h-4 shrink-0" />
            <span>{{ pullMessage }}</span>
          </div>

          <div
            v-if="pullError"
            class="p-3 rounded-xl bg-danger/10 border border-danger/30 text-danger text-xs flex items-start gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
            <span class="break-all">{{ pullError }}</span>
          </div>
        </div>

        <!-- ============================================================== -->
        <!-- 2. PUSH TAB                                                    -->
        <!-- ============================================================== -->
        <div v-if="activeTab === 'push'" class="space-y-4">
          <!-- Target Remote Branch -->
          <div class="flex items-center gap-2">
            <label class="text-xs text-ink-secondary shrink-0">推送到远程分支:</label>
            <input
              v-model="pushRemoteBranch"
              type="text"
              placeholder="默认与本地分支同名"
              class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
          </div>

          <!-- Ahead Commits Preview -->
          <div class="space-y-1.5">
            <div class="text-[11px] font-bold text-ink flex items-center justify-between">
              <span class="flex items-center gap-1.5">
                <ArrowUpCircle class="w-3.5 h-3.5 text-accent" />
                <span>即将推送到远端的本地提交 (Ahead: {{ aheadCommits.length }}):</span>
              </span>
              <span v-if="aheadCommits.length === 0" class="text-ink-muted font-normal text-[10px]">
                本地没有领先提交，已与远端完全同步
              </span>
            </div>

            <div
              v-if="aheadCommits.length > 0"
              class="max-h-40 overflow-y-auto rounded-xl bg-canvas border border-line/60 divide-y divide-line/40 font-mono text-[11px]"
            >
              <div
                v-for="c in aheadCommits"
                :key="c.hash"
                class="p-2 flex items-center justify-between gap-2"
              >
                <div class="flex items-center gap-2 truncate">
                  <span class="text-accent font-semibold">{{ c.shortHash }}</span>
                  <span class="truncate text-ink font-sans">{{ c.subject }}</span>
                </div>
                <div class="flex items-center gap-2 shrink-0 font-sans text-[10px] text-ink-muted">
                  <span>{{ c.authorName }}</span>
                  <span>{{ c.relativeDate }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Force Push Mode -->
          <div class="space-y-2">
            <label class="text-xs font-bold text-ink">推送模式 (Force Strategy)</label>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <button
                type="button"
                @click="pushForceMode = 'none'"
                class="p-2.5 rounded-xl border text-left transition cursor-pointer"
                :class="
                  pushForceMode === 'none'
                    ? 'border-accent bg-accent/10 text-accent font-semibold'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div class="text-xs">普通安全推送</div>
                <div class="text-[10px] text-ink-muted font-normal mt-0.5">遇分叉自动拒绝，最安全</div>
              </button>

              <button
                type="button"
                @click="pushForceMode = 'lease'"
                class="p-2.5 rounded-xl border text-left transition cursor-pointer"
                :class="
                  pushForceMode === 'lease'
                    ? 'border-warning bg-warning/10 text-warning font-semibold'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div class="text-xs flex items-center gap-1">
                  <span>租约保护强推</span>
                  <span class="text-[9px] px-1 rounded bg-warning/20 font-mono">--force-with-lease</span>
                </div>
                <div class="text-[10px] text-ink-muted font-normal mt-0.5">无他人新改动时允许安全重写</div>
              </button>

              <button
                type="button"
                @click="pushForceMode = 'force'"
                class="p-2.5 rounded-xl border text-left transition cursor-pointer"
                :class="
                  pushForceMode === 'force'
                    ? 'border-danger bg-danger/10 text-danger font-semibold'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div class="text-xs flex items-center gap-1">
                  <span>完全强推</span>
                  <span class="text-[9px] px-1 rounded bg-danger/20 font-mono">--force</span>
                </div>
                <div class="text-[10px] text-ink-muted font-normal mt-0.5">无视冲突直接覆盖 (谨慎)</div>
              </button>
            </div>
          </div>

          <!-- Options -->
          <div class="p-3.5 rounded-xl bg-canvas border border-line/60 space-y-2.5">
            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pushSetUpstream"
                type="checkbox"
                class="rounded border-line bg-surface text-accent focus:ring-0"
              />
              <span>设置此远程分支为默认上游跟踪分支 (-u / --set-upstream)</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pushFollowTags"
                type="checkbox"
                class="rounded border-line bg-surface text-accent focus:ring-0"
              />
              <span>推送可达的附注标签 (--follow-tags)</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pushTags"
                type="checkbox"
                class="rounded border-line bg-surface text-accent focus:ring-0"
              />
              <span>推送本地所有标签到远端 (--tags)</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pushSkipCI"
                type="checkbox"
                class="rounded border-line bg-surface text-accent focus:ring-0"
              />
              <span>跳过远程自动化流水线构建 (-o ci.skip)</span>
            </label>
          </div>

          <!-- Messages -->
          <div
            v-if="pushMessage"
            class="p-3 rounded-xl bg-success/10 border border-success/30 text-success text-xs flex items-center gap-2"
          >
            <CheckCircle2 class="w-4 h-4 shrink-0" />
            <span class="whitespace-pre-line">{{ pushMessage }}</span>
          </div>

          <div
            v-if="pushError"
            class="p-3 rounded-xl bg-danger/10 border border-danger/30 text-danger text-xs flex items-start gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
            <span class="break-all">{{ pushError }}</span>
          </div>
        </div>

        <!-- ============================================================== -->
        <!-- 3. FETCH TAB                                                   -->
        <!-- ============================================================== -->
        <div v-if="activeTab === 'fetch'" class="space-y-4">
          <div class="flex items-center justify-between p-4 rounded-xl bg-canvas border border-line">
            <div>
              <div class="text-xs font-bold text-ink">获取全部远程引用 (Fetch All)</div>
              <div class="text-[11px] text-ink-muted mt-0.5">从所有远程仓库拉取分支、标签与对象元数据，不修改工作区文件。</div>
            </div>
            <button
              @click="handleFetchAll"
              :disabled="isFetchingAll"
              class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-raised hover:bg-raised/80 text-xs font-semibold text-ink transition cursor-pointer border border-line"
            >
              <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isFetchingAll }" />
              <span>{{ isFetchingAll ? "正在获取..." : "立即 Fetch" }}</span>
            </button>
          </div>

          <!-- Remotes Table -->
          <div class="space-y-2">
            <span class="text-xs font-bold text-ink">已配置的远程镜像与源</span>
            <div class="rounded-xl border border-line bg-canvas overflow-hidden">
              <table class="w-full text-left text-xs font-mono">
                <thead class="bg-surface/80 border-b border-line text-[10px] text-ink-muted">
                  <tr>
                    <th class="p-2.5 font-semibold">名称</th>
                    <th class="p-2.5 font-semibold">远程地址</th>
                    <th class="p-2.5 font-semibold">类型</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-line/60">
                  <tr v-for="r in remotes" :key="`${r.name}-${r.type}`" class="hover:bg-raised/40">
                    <td class="p-2.5 font-bold text-accent">{{ r.name }}</td>
                    <td class="p-2.5 truncate max-w-xs text-ink-secondary">{{ r.url }}</td>
                    <td class="p-2.5 text-[10px] text-ink-muted uppercase">{{ r.type }}</td>
                  </tr>
                  <tr v-if="remotes.length === 0">
                    <td colspan="3" class="p-4 text-center text-ink-muted font-sans text-xs">
                      尚未配置任何远程仓库，请在设置中添加远端。
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div
            v-if="fetchResult"
            class="p-3 rounded-xl bg-surface border border-line text-xs font-mono whitespace-pre-wrap text-ink-secondary"
          >
            {{ fetchResult }}
          </div>
        </div>
      </div>

      <!-- Footer Buttons -->
      <div class="px-5 py-3.5 border-t border-line flex items-center justify-between bg-raised/30 shrink-0">
        <button
          @click="emit('close')"
          class="px-3.5 py-1.5 rounded-lg text-xs text-ink-secondary hover:text-ink cursor-pointer transition"
        >
          关闭
        </button>

        <div class="flex items-center gap-2">
          <button
            v-if="activeTab === 'pull'"
            @click="handleExecutePull"
            :disabled="isPulling"
            class="flex items-center gap-1.5 px-5 py-2 rounded-xl bg-cyan-strong text-white text-xs font-bold transition shadow hover:bg-cyan-strong-hover cursor-pointer disabled:opacity-50"
          >
            <Loader2 v-if="isPulling" class="w-3.5 h-3.5 animate-spin" />
            <ArrowDownCircle v-else class="w-3.5 h-3.5" />
            <span>{{ isPulling ? "正在拉取..." : "执行拉取 (Pull)" }}</span>
          </button>

          <button
            v-else-if="activeTab === 'push'"
            @click="handleExecutePush"
            :disabled="isPushing"
            class="flex items-center gap-1.5 px-5 py-2 rounded-xl text-xs font-bold transition shadow cursor-pointer disabled:opacity-50"
            :class="
              pushForceMode === 'force'
                ? 'bg-danger-strong hover:bg-danger-strong-hover text-white'
                : pushForceMode === 'lease'
                ? 'bg-warning-strong hover:bg-warning-strong-hover text-warning-contrast'
                : 'bg-accent-strong hover:bg-accent-strong-hover text-white'
            "
          >
            <Loader2 v-if="isPushing" class="w-3.5 h-3.5 animate-spin" />
            <ArrowUpCircle v-else class="w-3.5 h-3.5" />
            <span>{{ isPushing ? "正在推送..." : pushForceMode !== 'none' ? "强制推送 (Push)" : "确认推送 (Push)" }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
