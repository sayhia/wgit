<script setup lang="ts">
import { t } from "../i18n";
import { confirmAction } from "../composables/useConfirm";
import { ref, computed, watch } from "vue";
import UiModal from "./ui/UiModal.vue";
import { relativeTime } from "../utils/relativeTime";
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
  ShieldCheck,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import type {
  RepoStatus,
  RemoteDetail,
  CommitItem,
  PullAdvancedOptions,
  PushAdvancedOptions,
  MergeSandboxReport,
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
const isDryRunning = ref(false);
const sandboxReport = ref<MergeSandboxReport | null>(null);

async function runMergeSandbox() {
  if (!props.currentRepoPath || isDryRunning.value) return;
  isDryRunning.value = true;
  sandboxReport.value = null;
  try {
    const target = targetBranch.value ? `${selectedRemote.value}/${targetBranch.value}` : `${selectedRemote.value}/${currentBranch.value}`;
    const rep = await gitService.dryRunPullOrMerge(props.currentRepoPath, target, false);
    sandboxReport.value = rep;
  } catch (err: any) {
    pullError.value = String(err?.message || err);
  } finally {
    isDryRunning.value = false;
  }
}

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
    // best-effort: the remote picker stays empty; sync buttons still work.
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
      pullError.value = t("sync.error.conflict", { list: (res.conflicts || []).join(", ") || res.message });
    } else {
      pullMessage.value = res.alreadyUp
        ? t("sync.pull.up.to.date")
        : res.fastForward
        ? t("sync.pull.fast.forward")
        : t("sync.pull.merged");
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
    const confirmed = await confirmAction({
      message: t("sync.confirm.force.push"),
      tone: "danger",
      // Diverged histories lose the remote-only commits; say how many.
      affectedSummary:
        behindCommits.value.length > 0
          ? t("sync.confirm.force.push.scope", { count: behindCommits.value.length })
          : undefined,
    });
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
    pushMessage.value = res.message || t("sync.push.done");
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
    fetchResult.value = t("sync.fetch.done", { remote: selectedRemote.value });
    emit("refresh");
    await loadAheadBehindCommits();
  } catch (err: any) {
    fetchResult.value = t("sync.fetch.failed", { error: err?.message || err });
  } finally {
    isFetchingAll.value = false;
  }
}
</script>

<template>
  <UiModal
    :open="isOpen"
    variant="bare"
    width="max-w-2xl"
    :label="t('sync.title')"
    panel-class="max-h-[90vh] select-none animate-in fade-in duration-150"
    close-on-backdrop
    @update:open="(open) => { if (!open) emit('close'); }"
  >
      <!-- Header -->
      <div data-testid="sync-modal-header" class="px-5 py-3.5 border-b border-line flex items-start justify-between gap-3 bg-raised shrink-0">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <div class="w-8 h-8 rounded-xl bg-accent-bg text-accent flex items-center justify-center font-bold shrink-0">
            <GitPullRequest class="w-4 h-4" />
          </div>
          <div class="min-w-0 flex-1">
            <h2 class="text-sm font-bold text-ink truncate" :title="t('sync.title')">{{ t("sync.title") }}</h2>
            <div class="mt-0.5 min-w-0 text-[11px] text-ink-muted">
              <div class="flex min-w-0 items-center gap-1">
                <span class="shrink-0">{{ t("sync.current.branch") }}</span>
                <strong class="min-w-0 truncate text-ink font-mono" :title="currentBranch">{{ currentBranch }}</strong>
              </div>
              <div v-if="repoStatus?.trackingBranch" class="truncate text-accent font-mono" :title="repoStatus.trackingBranch">
                → {{ repoStatus.trackingBranch }}
              </div>
            </div>
          </div>
        </div>

        <button :aria-label="t('common.close')"
          @click="emit('close')"
          class="p-1 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Nav Tabs -->
      <div data-testid="sync-modal-tabs" class="flex items-center gap-1 overflow-x-auto px-5 pt-3 border-b border-line/60 bg-surface shrink-0">
        <button
          @click="activeTab = 'pull'"
          class="flex shrink-0 items-center gap-1.5 whitespace-nowrap px-4 py-2 border-b-2 text-xs font-semibold transition cursor-pointer"
          :class="
            activeTab === 'pull'
              ? 'border-cyan text-cyan'
              : 'border-transparent text-ink-secondary hover:text-ink'
          "
        >
          <ArrowDownCircle class="w-3.5 h-3.5" />
          <span>{{ t("sync.tab.pull") }}</span>
          <span
            v-if="behindCommits.length > 0"
            class="px-1.5 py-0.2 rounded-full bg-cyan-bg text-cyan text-[10px] font-bold"
          >
            {{ behindCommits.length }}
          </span>
        </button>

        <button
          @click="activeTab = 'push'"
          class="flex shrink-0 items-center gap-1.5 whitespace-nowrap px-4 py-2 border-b-2 text-xs font-semibold transition cursor-pointer"
          :class="
            activeTab === 'push'
              ? 'border-accent text-accent font-bold'
              : 'border-transparent text-ink-secondary hover:text-ink'
          "
        >
          <ArrowUpCircle class="w-3.5 h-3.5" />
          <span>{{ t("sync.tab.push") }}</span>
          <span
            v-if="aheadCommits.length > 0"
            class="px-1.5 py-0.2 rounded-full bg-accent-bg text-accent text-[10px] font-bold"
          >
            {{ aheadCommits.length }}
          </span>
        </button>

        <button
          @click="activeTab = 'fetch'"
          class="flex shrink-0 items-center gap-1.5 whitespace-nowrap px-4 py-2 border-b-2 text-xs font-semibold transition cursor-pointer"
          :class="
            activeTab === 'fetch'
              ? 'border-ink text-ink font-bold'
              : 'border-transparent text-ink-secondary hover:text-ink'
          "
        >
          <RotateCw class="w-3.5 h-3.5" />
          <span>{{ t("sync.tab.fetch") }}</span>
        </button>
      </div>

      <!-- Tab Content Area -->
      <div data-testid="sync-modal-body" class="flex-1 min-h-0 overflow-y-auto p-5 space-y-4">
        <!-- Common Remote Picker -->
        <div class="flex items-center gap-3 p-3 rounded-xl bg-canvas border border-line/60">
          <Globe class="w-4 h-4 text-ink-muted shrink-0" />
          <div class="flex-1 flex items-center gap-2">
            <span class="text-xs text-ink-secondary shrink-0">{{ t("sync.remote") }}</span>
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
          class="flex items-start gap-2 p-3 rounded-xl border border-warning/30 bg-warning-bg text-warning text-xs"
        >
          <AlertTriangle class="w-4 h-4 shrink-0" />
          <span>{{ t("sync.commits.error", { error: commitsError }) }}</span>
        </div>

        <!-- ============================================================== -->
        <!-- 1. PULL TAB                                                    -->
        <!-- ============================================================== -->
        <div v-if="activeTab === 'pull'" class="space-y-4">
          <!-- Strategy Cards -->
          <div class="space-y-2">
            <label class="text-xs font-bold text-ink">{{ t("sync.pull.strategy") }}</label>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <button
                type="button"
                @click="pullStrategy = 'merge'"
                class="p-3 rounded-xl border text-left transition cursor-pointer flex flex-col justify-between"
                :class="
                  pullStrategy === 'merge'
                    ? 'border-cyan bg-cyan-bg text-cyan shadow-sm'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div>
                  <div class="font-bold text-xs flex items-center gap-1">
                    <span>{{ t("sync.pull.merge") }}</span>
                    <span class="text-[9px] px-1 rounded bg-cyan-bg font-mono">--no-rebase</span>
                  </div>
                  <div class="text-[10px] text-ink-muted mt-1 leading-snug">
                    {{ t("sync.pull.merge.desc") }}
                  </div>
                </div>
              </button>

              <button
                type="button"
                @click="pullStrategy = 'rebase'"
                class="p-3 rounded-xl border text-left transition cursor-pointer flex flex-col justify-between"
                :class="
                  pullStrategy === 'rebase'
                    ? 'border-cyan bg-cyan-bg text-cyan shadow-sm'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div>
                  <div class="font-bold text-xs flex items-center gap-1">
                    <span>{{ t("sync.pull.rebase") }}</span>
                    <span class="text-[9px] px-1 rounded bg-cyan-bg font-mono">--rebase</span>
                  </div>
                  <div class="text-[10px] text-ink-muted mt-1 leading-snug">
                    {{ t("sync.pull.rebase.desc") }}
                  </div>
                </div>
              </button>

              <button
                type="button"
                @click="pullStrategy = 'ff-only'"
                class="p-3 rounded-xl border text-left transition cursor-pointer flex flex-col justify-between"
                :class="
                  pullStrategy === 'ff-only'
                    ? 'border-cyan bg-cyan-bg text-cyan shadow-sm'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div>
                  <div class="font-bold text-xs flex items-center gap-1">
                    <span>{{ t("sync.pull.ff.only") }}</span>
                    <span class="text-[9px] px-1 rounded bg-cyan-bg font-mono">--ff-only</span>
                  </div>
                  <div class="text-[10px] text-ink-muted mt-1 leading-snug">
                    {{ t("sync.pull.ff.only.desc") }}
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
              <span>{{ t("sync.pull.autostash") }}</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pullPrune"
                type="checkbox"
                class="rounded border-line bg-surface text-cyan focus:ring-0"
              />
              <span>{{ t("sync.pull.prune") }}</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pullSubmodules"
                type="checkbox"
                class="rounded border-line bg-surface text-cyan focus:ring-0"
              />
              <span>{{ t("sync.pull.recurse") }}</span>
            </label>
          </div>

          <!-- Behind Commits Preview -->
          <div v-if="behindCommits.length > 0" class="space-y-1.5">
            <div class="text-[11px] font-bold text-cyan flex items-center gap-1.5">
              <ArrowDownCircle class="w-3.5 h-3.5" />
              <span>{{ t("sync.pull.behind", { count: behindCommits.length }) }}</span>
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
                <span class="text-[10px] text-ink-muted whitespace-nowrap font-sans">{{ relativeTime(c.date, c.relativeDate) }}</span>
              </div>
            </div>
          </div>

          <!-- Dry Run Merge Sandbox Report Panel -->
          <div
            v-if="sandboxReport"
            data-testid="merge-sandbox-report-panel"
            class="p-3.5 rounded-xl border text-xs space-y-2"
            :class="sandboxReport.canMergeCleanly ? 'bg-success-bg border-success/30 text-success' : 'bg-warning-bg border-warning/30 text-warning'"
          >
            <div class="flex items-center justify-between font-bold">
              <span class="flex items-center gap-1.5">
                <CheckCircle2 v-if="sandboxReport.canMergeCleanly" class="w-4 h-4 text-success" />
                <AlertTriangle v-else class="w-4 h-4 text-warning" />
                <span>{{ sandboxReport.canMergeCleanly ? t("sync.sandbox.clean") : t("sync.sandbox.conflict") }}</span>
              </span>
              <span class="text-[11px] font-mono text-ink-muted">
                {{ sandboxReport.targetRef }} (ahead {{ sandboxReport.aheadCount }} / behind {{ sandboxReport.behindCount }})
              </span>
            </div>
            <div v-if="sandboxReport.conflictedFiles && sandboxReport.conflictedFiles.length > 0" class="mt-2 text-[11px] text-danger space-y-1">
              <div class="font-semibold">{{ t("sync.sandbox.conflicts.title") }}:</div>
              <ul class="list-disc list-inside font-mono text-[10px]">
                <li v-for="cf in sandboxReport.conflictedFiles" :key="cf">{{ cf }}</li>
              </ul>
            </div>
            <div v-else-if="sandboxReport.isFastForward" class="text-[11px] text-ink-muted font-mono">
              Strategy: fast-forward
            </div>
          </div>

          <!-- Messages -->
          <div
            v-if="pullMessage"
            class="p-3 rounded-xl bg-success-bg border border-success/30 text-success text-xs flex items-center gap-2"
          >
            <CheckCircle2 class="w-4 h-4 shrink-0" />
            <span>{{ pullMessage }}</span>
          </div>

          <div
            v-if="pullError"
            class="p-3 rounded-xl bg-danger-bg border border-danger/30 text-danger text-xs flex items-start gap-2"
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
            <label class="text-xs text-ink-secondary shrink-0">{{ t("sync.push.remote.branch") }}</label>
            <input
              v-model="pushRemoteBranch"
              type="text"
              :aria-label="t('sync.push.remote.placeholder')"
              :placeholder="t('sync.push.remote.placeholder')"
              class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
          </div>

          <!-- Ahead Commits Preview -->
          <div class="space-y-1.5">
            <div class="text-[11px] font-bold text-ink flex items-center justify-between">
              <span class="flex items-center gap-1.5">
                <ArrowUpCircle class="w-3.5 h-3.5 text-accent" />
                <span>{{ t("sync.push.ahead", { count: aheadCommits.length }) }}</span>
              </span>
              <span v-if="aheadCommits.length === 0" class="text-ink-muted font-normal text-[10px]">
                {{ t("sync.push.up.to.date") }}
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
                  <span>{{ relativeTime(c.date, c.relativeDate) }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Force Push Mode -->
          <div class="space-y-2">
            <label class="text-xs font-bold text-ink">{{ t("sync.push.mode") }}</label>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <button
                type="button"
                @click="pushForceMode = 'none'"
                class="p-2.5 rounded-xl border text-left transition cursor-pointer"
                :class="
                  pushForceMode === 'none'
                    ? 'border-accent bg-accent-bg text-accent font-semibold'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div class="text-xs">{{ t("sync.push.mode.safe") }}</div>
                <div class="text-[10px] text-ink-muted font-normal mt-0.5">{{ t("sync.push.mode.safe.desc") }}</div>
              </button>

              <button
                type="button"
                @click="pushForceMode = 'lease'"
                class="p-2.5 rounded-xl border text-left transition cursor-pointer"
                :class="
                  pushForceMode === 'lease'
                    ? 'border-warning bg-warning-bg text-warning font-semibold'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div class="text-xs flex flex-wrap items-center gap-1">
                  <span>{{ t("sync.push.mode.lease") }}</span>
                  <span class="text-[9px] px-1 rounded bg-warning-bg font-mono whitespace-nowrap">--force-with-lease</span>
                </div>
                <div class="text-[10px] text-ink-muted font-normal mt-0.5">{{ t("sync.push.mode.lease.desc") }}</div>
              </button>

              <button
                type="button"
                @click="pushForceMode = 'force'"
                class="p-2.5 rounded-xl border text-left transition cursor-pointer"
                :class="
                  pushForceMode === 'force'
                    ? 'border-danger bg-danger-bg text-danger font-semibold'
                    : 'border-line bg-canvas text-ink-secondary hover:bg-raised'
                "
              >
                <div class="text-xs flex flex-wrap items-center gap-1">
                  <span>{{ t("sync.push.mode.force") }}</span>
                  <span class="text-[9px] px-1 rounded bg-danger-bg font-mono whitespace-nowrap">--force</span>
                </div>
                <div class="text-[10px] text-ink-muted font-normal mt-0.5">{{ t("sync.push.mode.force.desc") }}</div>
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
              <span>{{ t("sync.push.set.upstream") }}</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pushFollowTags"
                type="checkbox"
                class="rounded border-line bg-surface text-accent focus:ring-0"
              />
              <span>{{ t("sync.push.follow.tags") }}</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pushTags"
                type="checkbox"
                class="rounded border-line bg-surface text-accent focus:ring-0"
              />
              <span>{{ t("sync.push.all.tags") }}</span>
            </label>

            <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
              <input
                v-model="pushSkipCI"
                type="checkbox"
                class="rounded border-line bg-surface text-accent focus:ring-0"
              />
              <span>{{ t("sync.push.skip.ci") }}</span>
            </label>
          </div>

          <!-- Messages -->
          <div
            v-if="pushMessage"
            class="p-3 rounded-xl bg-success-bg border border-success/30 text-success text-xs flex items-center gap-2"
          >
            <CheckCircle2 class="w-4 h-4 shrink-0" />
            <span class="whitespace-pre-line">{{ pushMessage }}</span>
          </div>

          <div
            v-if="pushError"
            class="p-3 rounded-xl bg-danger-bg border border-danger/30 text-danger text-xs flex items-start gap-2"
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
              <div class="text-xs font-bold text-ink">{{ t("sync.fetch.all.title") }}</div>
              <div class="text-[11px] text-ink-muted mt-0.5">{{ t("sync.fetch.all.desc") }}</div>
            </div>
            <button
              @click="handleFetchAll"
              :disabled="isFetchingAll"
              class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-raised hover:bg-raised text-xs font-semibold text-ink transition cursor-pointer border border-line"
            >
              <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isFetchingAll }" />
              <span>{{ isFetchingAll ? t("sync.fetching") : t("sync.fetch.run") }}</span>
            </button>
          </div>

          <!-- Remotes Table -->
          <div class="space-y-2">
            <span class="text-xs font-bold text-ink">{{ t("sync.remotes.title") }}</span>
            <div class="rounded-xl border border-line bg-canvas overflow-hidden">
              <table class="w-full text-left text-xs font-mono">
                <caption class="sr-only">{{ t("sync.remotes.title") }}</caption>
                <thead class="bg-surface border-b border-line text-[10px] text-ink-muted">
                  <tr>
                    <th scope="col" class="p-2.5 font-semibold">{{ t("sync.remotes.name") }}</th>
                    <th scope="col" class="p-2.5 font-semibold">{{ t("sync.remotes.url") }}</th>
                    <th scope="col" class="p-2.5 font-semibold">{{ t("sync.remotes.type") }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-line/60">
                  <tr v-for="r in remotes" :key="`${r.name}-${r.type}`" class="hover:bg-raised">
                    <td class="p-2.5 font-bold text-accent">{{ r.name }}</td>
                    <td class="p-2.5 truncate max-w-xs text-ink-secondary">{{ r.url }}</td>
                    <td class="p-2.5 text-[10px] text-ink-muted uppercase">{{ r.type }}</td>
                  </tr>
                  <tr v-if="remotes.length === 0">
                    <td colspan="3" class="p-4 text-center text-ink-muted font-sans text-xs">
                      {{ t("sync.remotes.empty") }}
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
      <div data-testid="sync-modal-footer" class="px-5 py-3.5 border-t border-line flex flex-wrap items-center justify-between gap-2 bg-raised shrink-0">
        <button
          @click="emit('close')"
          class="px-3.5 py-1.5 rounded-lg text-xs text-ink-secondary hover:text-ink cursor-pointer transition whitespace-nowrap shrink-0"
        >
          {{ t("sync.close") }}
        </button>

        <div class="ml-auto flex max-w-full flex-wrap items-center justify-end gap-2 [&>button]:whitespace-nowrap [&>button]:shrink-0">
          <button
            v-if="activeTab === 'pull'"
            type="button"
            data-testid="merge-sandbox-dryrun-btn"
            @click="runMergeSandbox"
            :disabled="isDryRunning || isPulling"
            class="flex items-center gap-1.5 px-3.5 py-2 rounded-xl bg-surface border border-line text-xs font-semibold text-ink hover:bg-raised transition cursor-pointer disabled:opacity-50"
          >
            <Loader2 v-if="isDryRunning" class="w-3.5 h-3.5 animate-spin" />
            <ShieldCheck v-else class="w-3.5 h-3.5 text-accent" />
            <span>{{ isDryRunning ? t("sync.sandbox.dryrun.simulating") : t("sync.sandbox.dryrun.btn") }}</span>
          </button>

          <button
            v-if="activeTab === 'pull'"
            @click="handleExecutePull"
            :disabled="isPulling"
            class="flex items-center gap-1.5 px-5 py-2 rounded-xl bg-cyan-strong text-white text-xs font-bold transition shadow hover:bg-cyan-strong-hover cursor-pointer disabled:opacity-50"
          >
            <Loader2 v-if="isPulling" class="w-3.5 h-3.5 animate-spin" />
            <ArrowDownCircle v-else class="w-3.5 h-3.5" />
            <span>{{ isPulling ? t("sync.pulling") : t("sync.pull.run") }}</span>
          </button>

          <button
            v-else-if="activeTab === 'push'"
            @click="handleExecutePush"
            :disabled="isPushing"
            class="flex items-center gap-1.5 px-5 py-2 rounded-xl text-xs font-bold transition shadow cursor-pointer disabled:opacity-50"
            :class="
              pushForceMode === 'force'
                ? 'bg-danger-strong hover:bg-danger-strong-hover text-accent-contrast'
                : pushForceMode === 'lease'
                ? 'bg-warning-strong hover:bg-warning-strong-hover text-warning-contrast'
                : 'bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast'
            "
          >
            <Loader2 v-if="isPushing" class="w-3.5 h-3.5 animate-spin" />
            <ArrowUpCircle v-else class="w-3.5 h-3.5" />
            <span>{{ isPushing ? t("sync.pushing") : pushForceMode !== 'none' ? t("sync.push.run.force") : t("sync.push.run") }}</span>
          </button>
        </div>
      </div>
  </UiModal>
</template>
