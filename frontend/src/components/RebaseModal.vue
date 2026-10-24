<script setup lang="ts">
import { t } from "../i18n";
import { confirmAction } from "../composables/useConfirm";
import { ref, computed, watch } from "vue";
import { useModalA11y } from "../composables/useModalA11y";
import { relativeTime } from "../utils/relativeTime";
import {
  X,
  GitPullRequest,
  GitBranch,
  ArrowRight,
  ShieldAlert,
  Search,
  CheckCircle2,
  AlertTriangle,
  Loader2,
  Layers,
  Sparkles,
  Play,
  SkipForward,
  AlertOctagon,
  FileCode,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import type {
  RepoStatus,
  BranchInfo,
  CommitItem,
  RebaseAdvancedOptions,
  RebaseProgressStatus,
} from "../types";

const props = defineProps<{
  isOpen: boolean;
  currentRepoPath: string;
  repoStatus: RepoStatus | null;
  targetBranch?: string;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "refresh"): void;
  (e: "launch-interactive", base: string): void;
}>();

const { containerRef: dialogRef } = useModalA11y(() => props.isOpen, () => emit("close"));

const branches = ref<BranchInfo[]>([]);
const selectedUpstream = ref("");
const searchFilter = ref("");
const rebaseOnto = ref("");
const autostash = ref(true);
const autosquash = ref(true);
const isRebasing = ref(false);
const isLoadingData = ref(false);
const rebaseError = ref("");
const rebaseSuccess = ref("");

// Ongoing Rebase Progress
const rebaseProgress = ref<RebaseProgressStatus | null>(null);
const isOperating = ref(false);

// Rebase Commits Preview
const rebaseCommits = ref<CommitItem[]>([]);
const isLoadingCommits = ref(false);

const isRebasingActive = computed(() => {
  return !!props.repoStatus?.isRebasing;
});

const currentBranchName = computed(
  () => props.repoStatus?.currentBranch || "main"
);

// Intelligent default recommendations (tracking branch > origin/main > main > first remote)
const recommendedBranches = computed(() => {
  const result: { branch: string; reason: string }[] = [];
  const tracking = props.repoStatus?.trackingBranch;
  if (tracking && tracking !== currentBranchName.value) {
    result.push({ branch: tracking, reason: t("rebase.reason.tracking") });
  }

  const mainCandidates = ["origin/main", "main", "origin/master", "master", "origin/dev", "dev"];
  for (const cand of mainCandidates) {
    if (cand !== currentBranchName.value && cand !== tracking) {
      const exists = branches.value.some((b) => b.name === cand);
      if (exists && !result.some((r) => r.branch === cand)) {
        result.push({ branch: cand, reason: cand.includes("origin/") ? t("rebase.reason.remote.main") : t("rebase.reason.local.main") });
      }
    }
  }
  return result;
});

const filteredBranches = computed(() => {
  const q = searchFilter.value.trim().toLowerCase();
  return branches.value.filter((b) => {
    if (b.name === currentBranchName.value) return false;
    if (!q) return true;
    return b.name.toLowerCase().includes(q);
  });
});

async function loadData() {
  if (!props.currentRepoPath) return;
  isLoadingData.value = true;
  rebaseError.value = "";
  rebaseSuccess.value = "";

  if (isRebasingActive.value) {
    await loadProgress();
    isLoadingData.value = false;
    return;
  }

  try {
    const list = await gitService.getBranches(props.currentRepoPath);
    branches.value = list || [];

    // Pre-select target
    if (props.targetBranch) {
      selectedUpstream.value = props.targetBranch;
    } else if (recommendedBranches.value.length > 0) {
      selectedUpstream.value = recommendedBranches.value[0]?.branch ?? "";
    } else {
      const firstOther = branches.value.find((b) => b.name !== currentBranchName.value);
      selectedUpstream.value = firstOther ? firstOther.name : "";
    }
  } catch (err) {
    console.debug("Failed to load branches:", err);
  } finally {
    isLoadingData.value = false;
  }

  await loadRebaseCommits();
}

async function loadProgress() {
  if (!props.currentRepoPath) return;
  try {
    const prog = await gitService.getRebaseProgress(props.currentRepoPath);
    rebaseProgress.value = prog || null;
  } catch (err) {
    console.debug("Failed to load rebase progress:", err);
  }
}

async function loadRebaseCommits() {
  if (!props.currentRepoPath || !selectedUpstream.value || isRebasingActive.value) {
    rebaseCommits.value = [];
    return;
  }
  isLoadingCommits.value = true;
  try {
    const list = await gitService.getRebaseCommits(
      props.currentRepoPath,
      selectedUpstream.value
    );
    rebaseCommits.value = list || [];
  } catch (err) {
    console.debug("Failed to load rebase commits:", err);
    rebaseCommits.value = [];
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

watch(
  () => props.targetBranch,
  (tgt) => {
    if (tgt) {
      selectedUpstream.value = tgt;
    }
  }
);

watch(selectedUpstream, () => {
  if (!isRebasingActive.value) {
    loadRebaseCommits();
  }
});

async function handleExecuteRebase() {
  if (!props.currentRepoPath || !selectedUpstream.value || isRebasing.value) return;
  isRebasing.value = true;
  rebaseError.value = "";
  rebaseSuccess.value = "";
  try {
    const opts: RebaseAdvancedOptions = {
      upstream: selectedUpstream.value,
      onto: rebaseOnto.value.trim() || undefined,
      autostash: autostash.value,
      autosquash: autosquash.value,
    };
    const res = await gitService.rebaseAdvanced(props.currentRepoPath, opts);
    if (res.hasConflicts) {
      rebaseError.value = t("rebase.error.conflict", { list: (res.conflicts || []).join("\n") || res.message });
      emit("refresh");
      await loadProgress();
    } else {
      rebaseSuccess.value = t("rebase.success.done", { upstream: selectedUpstream.value });
      emit("refresh");
      setTimeout(() => {
        emit("close");
      }, 1200);
    }
  } catch (err: any) {
    rebaseError.value = err?.message || String(err);
  } finally {
    isRebasing.value = false;
  }
}

async function handleContinue() {
  if (!props.currentRepoPath || isOperating.value) return;
  isOperating.value = true;
  rebaseError.value = "";
  try {
    await gitService.continueConflictOperation(props.currentRepoPath);
    rebaseSuccess.value = t("rebase.success.continued");
    emit("refresh");
    await loadProgress();
    if (!props.repoStatus?.isRebasing) {
      setTimeout(() => emit("close"), 1200);
    }
  } catch (err: any) {
    rebaseError.value = t("rebase.error.continue.failed", { error: err?.message || err });
  } finally {
    isOperating.value = false;
  }
}

async function handleSkip() {
  if (!props.currentRepoPath || isOperating.value) return;
  if (!(await confirmAction(t("rebase.confirm.skip")))) return;
  isOperating.value = true;
  rebaseError.value = "";
  try {
    await gitService.skipConflictOperation(props.currentRepoPath);
    rebaseSuccess.value = t("rebase.success.skipped");
    emit("refresh");
    await loadProgress();
    if (!props.repoStatus?.isRebasing) {
      setTimeout(() => emit("close"), 1200);
    }
  } catch (err: any) {
    rebaseError.value = t("rebase.error.skip.failed", { error: err?.message || err });
  } finally {
    isOperating.value = false;
  }
}

async function handleAbort() {
  if (!props.currentRepoPath || isOperating.value) return;
  if (!(await confirmAction({ message: t("rebase.confirm.abort"), tone: "danger" }))) return;
  isOperating.value = true;
  rebaseError.value = "";
  try {
    await gitService.abortConflictOperation(props.currentRepoPath);
    rebaseSuccess.value = t("rebase.success.aborted");
    emit("refresh");
    setTimeout(() => emit("close"), 1000);
  } catch (err: any) {
    rebaseError.value = t("rebase.error.abort.failed", { error: err?.message || err });
  } finally {
    isOperating.value = false;
  }
}

function handleStartInteractive() {
  emit("launch-interactive", selectedUpstream.value);
  emit("close");
}
</script>

<template>
  <div
    v-if="isOpen"
    ref="dialogRef"
    role="dialog"
    aria-modal="true"
    :aria-label="t('rebase.title')"
    class="fixed inset-0 z-50 bg-black/60 backdrop-blur-md flex items-center justify-center p-4 select-none animate-in fade-in duration-150"
    @click.self="emit('close')"
  >
    <div
      data-glass
      class="w-full max-w-xl glass glass-panel border rounded-2xl flex flex-col overflow-hidden max-h-[90vh]"
    >
      <!-- Header -->
      <div class="px-5 py-3.5 border-b border-line flex items-center justify-between bg-raised/40 shrink-0">
        <div class="flex items-center gap-3">
          <div
            class="w-8 h-8 rounded-xl flex items-center justify-center font-bold"
            :class="isRebasingActive ? 'bg-warning/20 text-warning animate-pulse' : 'bg-warning/15 text-warning'"
          >
            <GitPullRequest class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-sm font-bold text-ink flex items-center gap-2">
              <span>{{ t("rebase.title") }}</span>
              <span
                class="text-[10px] px-1.5 py-0.2 rounded-full font-mono font-semibold"
                :class="isRebasingActive ? 'bg-warning-strong text-warning-contrast' : 'bg-warning/20 text-warning'"
              >
                {{ isRebasingActive ? t("rebase.in.progress") : t("rebase.shortcut") }}
              </span>
            </h2>
            <div class="text-[11px] text-ink-muted">
              {{
                isRebasingActive
                  ? t('rebase.desc.active')
                  : t('rebase.desc.idle')
              }}
            </div>
          </div>
        </div>

        <button :aria-label="t('common.close')"
          @click="emit('close')"
          class="p-1 rounded-lg hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Body: Rebasing Active State -->
      <div v-if="isRebasingActive" class="flex-1 overflow-y-auto p-5 space-y-4">
        <!-- Progress Ribbon -->
        <div class="p-4 rounded-xl bg-warning/10 border border-warning/30 flex flex-col gap-2.5">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span class="w-2.5 h-2.5 rounded-full bg-warning-strong animate-ping"></span>
              <span class="text-xs font-bold text-warning">{{ t("rebase.paused.title") }}</span>
            </div>
            <span class="text-xs font-mono font-bold text-warning">
              {{ t("rebase.step", { step: repoStatus?.rebaseStep || rebaseProgress?.currentStep || 1, total: repoStatus?.rebaseTotal || rebaseProgress?.totalSteps || "?" }) }}
            </span>
          </div>

          <div class="text-xs text-ink leading-relaxed">
            {{ t("rebase.onto") }} <code class="px-1 py-0.5 rounded bg-raised text-warning font-mono">{{ rebaseProgress?.onto || repoStatus?.rebaseHeadName || t("rebase.upstream.fallback") }}</code>
          </div>

          <div v-if="rebaseProgress?.currentCommit" class="text-[11px] text-ink-secondary">
            {{ t("rebase.paused.commit") }} <code class="font-mono text-accent">{{ rebaseProgress.currentCommit }}</code>
          </div>
        </div>

        <!-- Conflicted files notice if any -->
        <div
          v-if="(repoStatus?.conflictedFiles?.length || 0) > 0 || (rebaseProgress?.conflictedFiles?.length || 0) > 0"
          class="p-3.5 rounded-xl bg-danger/10 border border-danger/30 space-y-2"
        >
          <div class="text-xs font-bold text-danger flex items-center gap-1.5">
            <AlertOctagon class="w-4 h-4" />
            <span>{{ t("rebase.conflicts", { count: (repoStatus?.conflictedFiles || rebaseProgress?.conflictedFiles || []).length }) }}</span>
          </div>

          <div class="max-h-36 overflow-y-auto rounded-lg bg-canvas border border-danger/20 divide-y divide-danger/10 text-xs font-mono">
            <div
              v-for="f in (repoStatus?.conflictedFiles || rebaseProgress?.conflictedFiles || [])"
              :key="f"
              class="px-2.5 py-1.5 text-danger flex items-center gap-1.5 truncate"
            >
              <FileCode class="w-3.5 h-3.5 shrink-0 text-danger/70" />
              <span class="truncate">{{ f }}</span>
            </div>
          </div>
        </div>

        <!-- Feedback Messages -->
        <div
          v-if="rebaseSuccess"
          class="p-3 rounded-xl bg-success/10 border border-success/30 text-success text-xs flex items-center gap-2"
        >
          <CheckCircle2 class="w-4 h-4 shrink-0" />
          <span>{{ rebaseSuccess }}</span>
        </div>

        <div
          v-if="rebaseError"
          class="p-3 rounded-xl bg-danger/10 border border-danger/30 text-danger text-xs flex items-start gap-2"
        >
          <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
          <span class="break-all whitespace-pre-wrap">{{ rebaseError }}</span>
        </div>

        <!-- Help description -->
        <div class="p-3 rounded-xl bg-canvas border border-line text-xs text-ink-muted leading-relaxed">
          💡 <strong>{{ t("rebase.guide.title") }}</strong>{{ t("rebase.guide.body") }}
        </div>
      </div>

      <!-- Body: Normal Launch Rebase State -->
      <div v-else class="flex-1 overflow-y-auto p-5 space-y-4">
        <!-- Target Overview Card -->
        <div class="p-3.5 rounded-xl bg-canvas border border-line flex items-center justify-between">
          <div class="flex flex-col">
            <span class="text-[10px] text-ink-muted uppercase tracking-wider font-semibold">{{ t("rebase.current.branch") }}</span>
            <span class="font-mono text-xs font-bold text-ink mt-0.5">{{ currentBranchName }}</span>
          </div>

          <ArrowRight class="w-4 h-4 text-ink-muted shrink-0 mx-2" />

          <div class="flex flex-col items-end">
            <span class="text-[10px] text-ink-muted uppercase tracking-wider font-semibold">{{ t("rebase.target.upstream") }}</span>
            <span class="font-mono text-xs font-bold text-warning mt-0.5 truncate max-w-[200px]">
              {{ selectedUpstream || t('rebase.select.upstream') }}
            </span>
          </div>
        </div>

        <!-- Smart Recommendations -->
        <div v-if="recommendedBranches.length > 0" class="space-y-1.5">
          <div class="flex items-center gap-1.5 text-[11px] font-bold text-ink">
            <Sparkles class="w-3.5 h-3.5 text-warning" />
            <span>{{ t("rebase.suggest.title") }}</span>
          </div>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="rec in recommendedBranches"
              :key="rec.branch"
              type="button"
              @click="selectedUpstream = rec.branch"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg border text-xs font-mono transition cursor-pointer"
              :class="
                selectedUpstream === rec.branch
                  ? 'bg-warning/15 border-warning/50 text-warning font-bold'
                  : 'bg-canvas border-line text-ink-secondary hover:bg-raised'
              "
            >
              <span>{{ rec.branch }}</span>
              <span class="text-[9px] px-1 rounded bg-raised text-ink-muted font-sans font-normal">
                {{ rec.reason }}
              </span>
            </button>
          </div>
        </div>

        <!-- Target Branch Picker / Direct input -->
        <div class="space-y-1.5">
          <label class="text-xs font-bold text-ink flex items-center justify-between">
            <span>{{ t("rebase.base.label") }}</span>
            <span class="text-[10px] text-ink-muted font-normal">{{ t("rebase.base.hint") }}</span>
          </label>
          <div class="relative">
            <Search class="w-3.5 h-3.5 text-ink-muted absolute left-3 top-2.5 pointer-events-none" />
            <input
              v-model="selectedUpstream"
              type="text"
              :placeholder="t('rebase.base.placeholder')"
              class="w-full pl-8 pr-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
            />
          </div>

          <div class="max-h-36 overflow-y-auto rounded-xl bg-canvas border border-line/60 divide-y divide-line/40 text-xs font-mono">
            <div
              v-for="b in filteredBranches"
              :key="b.name"
              @click="selectedUpstream = b.name"
              class="px-3 py-2 flex items-center justify-between hover:bg-raised/80 cursor-pointer transition"
              :class="{ 'bg-warning/10 text-warning font-bold': selectedUpstream === b.name }"
            >
              <div class="flex items-center gap-2 truncate">
                <GitBranch class="w-3 h-3 text-ink-muted shrink-0" />
                <span class="truncate">{{ b.name }}</span>
              </div>
              <span v-if="b.isRemote" class="text-[10px] text-ink-muted uppercase">remote</span>
            </div>
            <div v-if="filteredBranches.length === 0" class="p-3 text-center text-ink-muted text-xs font-sans">
              {{ t("rebase.base.no.match") }}
            </div>
          </div>
        </div>

        <!-- Commits to be replayed preview -->
        <div class="space-y-1.5">
          <div class="flex items-center justify-between text-[11px] font-bold text-ink">
            <span class="flex items-center gap-1.5">
              <Layers class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("rebase.commits", { count: rebaseCommits.length }) }}</span>
            </span>
            <span v-if="isLoadingCommits" class="text-[10px] text-ink-muted flex items-center gap-1">
              <Loader2 class="w-3 h-3 animate-spin" /> {{ t("rebase.commits.loading") }}
            </span>
          </div>

          <div
            v-if="rebaseCommits.length > 0"
            class="max-h-28 overflow-y-auto rounded-xl bg-canvas border border-line/60 divide-y divide-line/40 text-[11px] font-mono"
          >
            <div
              v-for="c in rebaseCommits"
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
          <div
            v-else-if="!isLoadingCommits"
            class="p-2.5 rounded-xl bg-canvas border border-line/60 text-center text-xs text-ink-muted"
          >
            {{ t("rebase.fast.forward") }}
          </div>
        </div>

        <!-- Optional --onto -->
        <div class="flex flex-col gap-1">
          <label class="text-[11px] text-ink-secondary font-medium">{{ t("rebase.onto.label") }}</label>
          <input
            v-model="rebaseOnto"
            type="text"
            :placeholder="t('rebase.onto.placeholder')"
            class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <!-- Checkbox Options -->
        <div class="space-y-2 p-3.5 rounded-xl bg-canvas border border-line/60">
          <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
            <input
              v-model="autostash"
              type="checkbox"
              class="rounded border-line bg-surface text-warning focus:ring-0"
            />
            <span>{{ t("rebase.autostash") }}</span>
          </label>

          <label class="flex items-center gap-2.5 text-xs text-ink cursor-pointer select-none">
            <input
              v-model="autosquash"
              type="checkbox"
              class="rounded border-line bg-surface text-warning focus:ring-0"
            />
            <span>{{ t("rebase.autosquash") }}</span>
          </label>
        </div>

        <!-- Feedback Messages -->
        <div
          v-if="rebaseSuccess"
          class="p-3 rounded-xl bg-success/10 border border-success/30 text-success text-xs flex items-center gap-2"
        >
          <CheckCircle2 class="w-4 h-4 shrink-0" />
          <span>{{ rebaseSuccess }}</span>
        </div>

        <div
          v-if="rebaseError"
          class="p-3 rounded-xl bg-danger/10 border border-danger/30 text-danger text-xs flex items-start gap-2"
        >
          <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
          <span class="break-all whitespace-pre-wrap">{{ rebaseError }}</span>
        </div>

        <!-- Warning Notice -->
        <div class="p-3 rounded-xl bg-warning/10 border border-warning/20 text-xs text-warning flex items-start gap-2">
          <ShieldAlert class="w-4 h-4 shrink-0 mt-0.5" />
          <div class="leading-relaxed">
            {{ t("rebase.hint") }}
          </div>
        </div>
      </div>

      <!-- Footer: Rebasing Active State Actions -->
      <div
        v-if="isRebasingActive"
        class="px-5 py-3.5 border-t border-line flex items-center justify-between bg-raised/30 shrink-0"
      >
        <button
          type="button"
          @click="handleAbort"
          :disabled="isOperating"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-danger/15 hover:bg-danger/25 text-danger border border-danger/30 text-xs font-semibold cursor-pointer transition disabled:opacity-50"
        >
          <AlertOctagon class="w-3.5 h-3.5" />
          <span>{{ t("rebase.abort") }}</span>
        </button>

        <div class="flex items-center gap-2">
          <button
            type="button"
            @click="handleSkip"
            :disabled="isOperating"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised text-ink border border-line text-xs font-semibold cursor-pointer transition disabled:opacity-50"
            :title="t('rebase.skip.title')"
          >
            <SkipForward class="w-3.5 h-3.5 text-ink-muted" />
            <span>{{ t("rebase.skip") }}</span>
          </button>

          <button
            type="button"
            @click="handleContinue"
            :disabled="isOperating"
            class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-warning-strong hover:bg-warning-strong-hover text-warning-contrast text-xs font-bold cursor-pointer transition shadow disabled:opacity-50"
            :title="t('rebase.continue.title')"
          >
            <Loader2 v-if="isOperating" class="w-3.5 h-3.5 animate-spin" />
            <Play v-else class="w-3.5 h-3.5 fill-current" />
            <span>{{ t("rebase.continue") }}</span>
          </button>
        </div>
      </div>

      <!-- Footer: Normal Launch State Actions -->
      <div
        v-else
        class="px-5 py-3.5 border-t border-line flex items-center justify-between bg-raised/30 shrink-0"
      >
        <button
          @click="emit('close')"
          class="px-3.5 py-1.5 rounded-lg text-xs text-ink-secondary hover:text-ink cursor-pointer transition"
        >
          {{ t("common.cancel") }}
        </button>

        <div class="flex items-center gap-2">
          <!-- Jump to Interactive Rebase button -->
          <button
            type="button"
            @click="handleStartInteractive"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised text-xs font-semibold text-cyan border border-cyan/40 transition cursor-pointer"
            :title="t('rebase.interactive.title')"
          >
            <span>{{ t("rebase.interactive") }}</span>
          </button>

          <!-- Standard Rebase Submit -->
          <button
            type="button"
            @click="handleExecuteRebase"
            :disabled="!selectedUpstream || isRebasing"
            class="flex items-center gap-1.5 px-5 py-2 rounded-xl bg-warning-strong text-warning-contrast text-xs font-bold transition shadow hover:bg-warning-strong-hover cursor-pointer disabled:opacity-50"
          >
            <Loader2 v-if="isRebasing" class="w-3.5 h-3.5 animate-spin" />
            <GitPullRequest v-else class="w-3.5 h-3.5" />
            <span>{{ isRebasing ? t("rebase.running") : t("rebase.run") }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
