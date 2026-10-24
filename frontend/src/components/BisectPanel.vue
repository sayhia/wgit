<script setup lang="ts">
import { t } from "../i18n";
import { confirmAction } from "../composables/useConfirm";
import { ref, computed, watch, onMounted } from "vue";
import {
  Binary,
  AlertTriangle,
  AlertCircle,
  CheckCircle2,
  SkipForward,
  Copy,
  Check,
  ChevronDown,
  RotateCcw,
  Play,
  RefreshCw,
  GitCommitHorizontal,
  ListChecks,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import type { BisectStatus } from "../types";

const props = defineProps<{
  repoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

const status = ref<BisectStatus | null>(null);
const isLoading = ref(false);
const isStarting = ref(false);
const isJudging = ref(false);
const isResetting = ref(false);
const errorMsg = ref("");

// Start form
const badRefInput = ref("HEAD");
const goodRefInput = ref("");

// Collapsible log
const showLog = ref(false);

// Copy feedback
const copiedTarget = ref<"" | "current" | "result">("");
let copiedTimer: any = null;

function setError(message: string) {
  errorMsg.value = message;
}

function clearError() {
  errorMsg.value = "";
}

const isActive = computed(() => !!status.value?.isActive);
const logText = computed(() => status.value?.log ?? "");
const logLines = computed(() => logText.value.split("\n").filter((line) => line.trim().length > 0));

const judgementCount = computed(() =>
  logLines.value.filter((line) => /^git bisect (good|bad|skip)\b/.test(line.trim())).length
);

const goodRefsPreview = computed(() => (status.value?.goodRefs ?? []).slice(0, 8));
const goodRefsOverflow = computed(() => Math.max(0, (status.value?.goodRefs ?? []).length - 8));

/**
 * A converged bisect session either reports no current checkout hash, or its
 * log carries git's "# first bad commit: [...]" verdict line. Some Git versions
 * quote "bad" (first 'bad' commit), so both forms must match.
 */
const LOCATED_RESULT_RE = /first\s+['"]?bad['"]?\s+commit:\s*\[([0-9a-fA-F]{7,40})\]/i;
const hasLocatedResult = computed(() => {
  if (!isActive.value) return false;
  return !status.value?.currentHash || LOCATED_RESULT_RE.test(logText.value);
});

const locatedHash = computed(() => {
  const match = logText.value.match(LOCATED_RESULT_RE);
  if (match?.[1]) return match[1];
  return status.value?.currentHash || status.value?.badRef || "";
});

async function loadStatus() {
  if (!props.repoPath) return;
  isLoading.value = true;
  clearError();
  try {
    status.value = await gitService.bisectStatus(props.repoPath);
  } catch (err: any) {
    status.value = null;
    setError(t("bisect.error.read.status", { error: err?.message || err }));
  } finally {
    isLoading.value = false;
  }
}

async function handleStart() {
  if (!props.repoPath || isStarting.value) return;
  const badRef = badRefInput.value.trim() || "HEAD";
  const goodRef = goodRefInput.value.trim();
  if (!goodRef) {
    setError(t("bisect.error.good.required"));
    return;
  }
  isStarting.value = true;
  clearError();
  try {
    const next = await gitService.bisectStart(props.repoPath, badRef, goodRef);
    status.value = next ?? null;
    // bisect start checks out a middle commit: let the whole app refresh.
    emit("refresh");
  } catch (err: any) {
    setError(t("bisect.error.start.failed", { error: err?.message || err }));
  } finally {
    isStarting.value = false;
  }
}

/** Resolved per call so the labels follow the active locale. */
function judgeLabel(kind: "good" | "bad" | "skip"): string {
  return t(`bisect.judge.${kind}`);
}

async function handleJudge(kind: "good" | "bad" | "skip") {
  if (!props.repoPath || isJudging.value || !isActive.value) return;
  isJudging.value = true;
  clearError();
  try {
    const next =
      kind === "good"
        ? await gitService.bisectGood(props.repoPath)
        : kind === "bad"
          ? await gitService.bisectBad(props.repoPath)
          : await gitService.bisectSkip(props.repoPath);
    if (next) {
      status.value = next;
    } else {
      await loadStatus();
    }
    // Each judgement checks out another commit, so refresh the workspace views.
    emit("refresh");
  } catch (err: any) {
    setError(t("bisect.error.judge.failed", { kind: judgeLabel(kind), error: err?.message || err }));
  } finally {
    isJudging.value = false;
  }
}

async function handleReset() {
  if (!props.repoPath || isResetting.value) return;
  if (
    !(await confirmAction(
      { message: t("bisect.confirm.reset"), tone: "danger" }
    ))
  ) {
    return;
  }
  isResetting.value = true;
  clearError();
  try {
    await gitService.bisectReset(props.repoPath);
    status.value = {
      isActive: false,
      currentHash: "",
      currentStep: "",
      badRef: "",
      goodRefs: [],
      log: "",
    };
    showLog.value = false;
    emit("refresh");
  } catch (err: any) {
    setError(t("bisect.error.reset.failed", { error: err?.message || err }));
  } finally {
    isResetting.value = false;
  }
}

async function copyHash(value: string, target: "current" | "result") {
  if (!value) return;
  try {
    await navigator.clipboard.writeText(value);
  } catch {
    // Clipboard may be unavailable in some webviews; the hash remains selectable.
  }
  copiedTarget.value = target;
  if (copiedTimer) clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => {
    copiedTarget.value = "";
  }, 1600);
}

watch(
  () => props.repoPath,
  () => {
    badRefInput.value = "HEAD";
    goodRefInput.value = "";
    showLog.value = false;
    status.value = null;
    void loadStatus();
  }
);

onMounted(() => {
  void loadStatus();
});

defineExpose({ loadStatus });
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex items-start justify-between gap-3 pb-3 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Binary class="w-4 h-4 text-accent" />
          <span>{{ t("bisect.title") }}</span>
          <span class="text-xs text-ink-muted font-normal">{{ t("bisect.subtitle") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-1">
          {{ t("bisect.desc") }}
        </p>
      </div>

      <button
        @click="loadStatus"
        :disabled="isLoading || !repoPath"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer shrink-0 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
        <span>{{ t("bisect.refresh") }}</span>
      </button>
    </div>

    <!-- No repo -->
    <div
      v-if="!repoPath"
      class="py-16 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl"
    >
      {{ t("bisect.no.repo") }}
    </div>

    <!-- Loading (first load) -->
    <div
      v-else-if="isLoading && !status"
      class="py-16 flex flex-col items-center gap-3 text-xs text-ink-muted"
    >
      <RefreshCw class="w-5 h-5 animate-spin text-accent" />
      <span>{{ t("bisect.reading") }}</span>
    </div>

    <template v-else>
      <!-- Error banner -->
      <div
        v-if="errorMsg"
        class="p-3 rounded-xl bg-danger/10 border border-danger/30 text-xs text-danger flex items-start gap-2"
      >
        <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
        <span class="flex-1 leading-relaxed">{{ errorMsg }}</span>
        <button
          @click="clearError"
          class="text-ink-muted hover:text-danger transition cursor-pointer shrink-0 text-[11px] font-semibold"
        >
          {{ t("bisect.close") }}
        </button>
      </div>

      <!-- ============================= -->
      <!-- ACTIVE SESSION                -->
      <!-- ============================= -->
      <div v-if="isActive" class="space-y-4">
        <!-- Located result banner -->
        <div
          v-if="hasLocatedResult"
          class="p-4 rounded-xl border border-success/40 bg-success/10 flex flex-col gap-3"
        >
          <div class="flex items-center gap-2.5">
            <CheckCircle2 class="w-5 h-5 text-success shrink-0" />
            <div class="flex flex-col">
              <span class="text-sm font-bold text-success">{{ t("bisect.found.title") }}</span>
              <span class="text-[11px] text-ink-muted">
                {{ t("bisect.found.desc") }}
              </span>
            </div>
          </div>
          <div class="flex items-center gap-2 flex-wrap">
            <code class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line font-mono text-xs font-bold text-ink">
              {{ locatedHash || t("bisect.unknown.commit") }}
            </code>
            <button
              @click="copyHash(locatedHash, 'result')"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-medium text-ink transition cursor-pointer"
            >
              <Check v-if="copiedTarget === 'result'" class="w-3 h-3 text-success" />
              <Copy v-else class="w-3 h-3" />
              <span>{{ copiedTarget === "result" ? t("bisect.copied") : t("bisect.copy.hash") }}</span>
            </button>
            <button
              @click="handleReset"
              :disabled="isResetting"
              class="flex items-center gap-1 px-3 py-1 rounded-lg bg-danger-strong hover:bg-danger-strong-hover text-white text-[11px] font-semibold transition cursor-pointer disabled:opacity-50"
            >
              <RotateCcw class="w-3 h-3" :class="{ 'animate-spin': isResetting }" />
              <span>{{ isResetting ? t("bisect.resetting") : t("bisect.reset.end") }}</span>
            </button>
          </div>
          <div class="flex items-center gap-2 flex-wrap text-[11px]">
            <span class="px-2 py-0.5 rounded-full bg-danger/15 text-danger border border-danger/30 font-mono font-bold">
              bad: {{ status?.badRef || "-" }}
            </span>
            <span class="px-2 py-0.5 rounded-full bg-surface border border-line text-ink-secondary font-mono">
              {{ t("bisect.good.refs", { count: status?.goodRefs?.length ?? 0 }) }}
            </span>
            <span class="px-2 py-0.5 rounded-full bg-accent/10 border border-accent/25 text-accent font-medium">
              {{ t("bisect.judgements", { count: judgementCount }) }}
            </span>
          </div>
        </div>

        <!-- Judging UI -->
        <template v-else>
          <!-- Working tree hint -->
          <div
            class="p-3 rounded-xl bg-warning/10 border border-warning/25 text-xs text-warning flex items-start gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
            <span class="leading-relaxed">
              {{ t("bisect.judge.hint") }}
            </span>
          </div>

          <!-- Current candidate commit -->
          <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-3">
            <div class="flex items-center justify-between gap-3 flex-wrap">
              <span class="text-xs text-ink-muted font-medium flex items-center gap-1.5">
                <GitCommitHorizontal class="w-3.5 h-3.5" />
                <span>{{ t("bisect.current.commit") }}</span>
              </span>
              <div class="flex items-center gap-2">
                <code class="font-mono text-sm font-bold text-accent">{{ status?.currentHash || t("bisect.unknown") }}</code>
                <button :aria-label="t('bisect.copy.current.aria')"
                  @click="copyHash(status?.currentHash || '', 'current')"
                  :disabled="!status?.currentHash"
                  class="p-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                  :title="t('bisect.copy.current.aria')"
                >
                  <Check v-if="copiedTarget === 'current'" class="w-3.5 h-3.5 text-success" />
                  <Copy v-else class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div class="flex items-center gap-2 flex-wrap text-[11px]">
              <span
                class="px-2 py-0.5 rounded-full bg-danger/15 text-danger border border-danger/30 font-mono font-bold"
                :title="t('bisect.bad.commit.title')"
              >
                bad: {{ status?.badRef || "-" }}
              </span>
              <span class="text-ink-muted">good:</span>
              <span
                v-for="(good, idx) in goodRefsPreview"
                :key="`${good}-${idx}`"
                class="px-1.5 py-0.5 rounded bg-success/10 border border-success/25 text-success font-mono text-[10px]"
              >
                {{ good }}
              </span>
              <span v-if="goodRefsOverflow > 0" class="text-ink-muted font-mono">+{{ goodRefsOverflow }}</span>
              <span v-if="goodRefsPreview.length === 0" class="text-ink-muted font-mono">{{ t("bisect.none") }}</span>
              <span
                class="px-2 py-0.5 rounded-full bg-surface border border-line text-ink-secondary"
                :title="t('bisect.progress.title')"
              >
                {{ status?.currentStep || t("bisect.waiting.first") }}
              </span>
              <span class="px-2 py-0.5 rounded-full bg-accent/10 border border-accent/25 text-accent font-medium">
                {{ t("bisect.judgements", { count: judgementCount }) }}
              </span>
            </div>
          </div>

          <!-- Big Good / Bad buttons -->
          <div class="grid grid-cols-2 gap-4">
            <button
              @click="handleJudge('good')"
              :disabled="isJudging || isResetting"
              class="p-5 rounded-2xl border-2 border-success/40 bg-success/10 hover:bg-success/20 transition cursor-pointer flex flex-col items-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <CheckCircle2 class="w-6 h-6 text-success" />
              <span class="text-sm font-bold text-success">{{ t("bisect.good.title") }}</span>
              <span class="text-[11px] text-ink-muted">{{ t("bisect.good.desc") }}</span>
            </button>
            <button
              @click="handleJudge('bad')"
              :disabled="isJudging || isResetting"
              class="p-5 rounded-2xl border-2 border-danger/40 bg-danger/10 hover:bg-danger/20 transition cursor-pointer flex flex-col items-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <AlertCircle class="w-6 h-6 text-danger" />
              <span class="text-sm font-bold text-danger">{{ t("bisect.bad.title") }}</span>
              <span class="text-[11px] text-ink-muted">{{ t("bisect.bad.desc") }}</span>
            </button>
          </div>

          <button
            @click="handleJudge('skip')"
            :disabled="isJudging || isResetting"
            class="w-full p-2.5 rounded-xl border border-warning/40 bg-warning/10 hover:bg-warning/20 transition cursor-pointer flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            :title="t('bisect.skip.title')"
          >
            <SkipForward class="w-4 h-4 text-warning" />
            <span class="text-xs font-semibold text-warning">{{ t("bisect.skip.label") }}</span>
          </button>

          <div class="flex items-center justify-between gap-3 flex-wrap">
            <span class="text-[11px] text-ink-muted">
              {{ t("bisect.skip.hint") }}
            </span>
            <button
              @click="handleReset"
              :disabled="isResetting || isJudging"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-danger transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <RotateCcw class="w-3.5 h-3.5" :class="{ 'animate-spin': isResetting }" />
              <span>{{ isResetting ? t("bisect.resetting") : t("bisect.reset.session") }}</span>
            </button>
          </div>
        </template>

        <!-- Collapsible git bisect log -->
        <div class="rounded-xl border border-line bg-surface/20 overflow-hidden">
          <button
            @click="showLog = !showLog"
            class="w-full px-4 py-2.5 flex items-center justify-between text-xs font-semibold text-ink-secondary hover:text-ink hover:bg-surface/40 transition cursor-pointer"
          >
            <span class="flex items-center gap-2">
              <ListChecks class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("bisect.log.title") }}</span>
              <span class="text-[10px] text-ink-muted font-normal">{{ t("bisect.log.lines", { count: logLines.length }) }}</span>
            </span>
            <ChevronDown class="w-3.5 h-3.5 transition-transform" :class="{ 'rotate-180': showLog }" />
          </button>
          <div v-if="showLog" class="border-t border-line px-4 py-3 max-h-64 overflow-y-auto bg-canvas">
            <pre class="font-mono text-[11px] leading-5 text-ink-secondary whitespace-pre-wrap">{{ logText || t("bisect.log.empty") }}</pre>
          </div>
        </div>
      </div>

      <!-- ============================= -->
      <!-- START FORM (not bisecting)    -->
      <!-- ============================= -->
      <div v-else class="grid grid-cols-1 gap-4">
        <div class="p-5 rounded-2xl border border-line bg-surface/30 space-y-4">
          <div class="flex items-center gap-2 pb-2 border-b border-line">
            <Play class="w-4 h-4 text-accent" />
            <span class="text-sm font-bold text-ink">{{ t("bisect.start.title") }}</span>
            <span class="text-[11px] text-ink-muted font-mono">git bisect start &lt;bad&gt; &lt;good&gt;</span>
            <span class="ml-auto px-2 py-0.5 rounded-full bg-surface border border-line text-[10px] text-ink-muted">
              {{ t("bisect.not.active") }}
            </span>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-1.5">
              <label class="text-xs text-ink-secondary font-medium">{{ t("bisect.bad.label") }}</label>
              <input
                v-model="badRefInput"
                type="text"
                placeholder="HEAD / v2.0.0 / main"
                @keydown.enter="handleStart"
                class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
              />
              <span class="text-[10px] text-ink-muted">{{ t("bisect.bad.hint") }}</span>
            </div>

            <div class="flex flex-col gap-1.5">
              <label class="text-xs text-ink-secondary font-medium">{{ t("bisect.good.label") }}</label>
              <input
                v-model="goodRefInput"
                type="text"
                :placeholder="t('bisect.good.placeholder')"
                @keydown.enter="handleStart"
                class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
              />
              <span class="text-[10px] text-ink-muted">{{ t("bisect.good.hint") }}</span>
            </div>
          </div>

          <div
            class="p-3 rounded-xl bg-warning/10 border border-warning/25 text-[11px] text-warning flex items-start gap-2"
          >
            <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
            <span class="leading-relaxed">
              {{ t("bisect.clean.hint") }}
            </span>
          </div>

          <button
            @click="handleStart"
            :disabled="isStarting || !goodRefInput.trim()"
            class="w-full flex items-center justify-center gap-2 py-2.5 rounded-xl bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer shadow-sm disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <RefreshCw v-if="isStarting" class="w-4 h-4 animate-spin" />
            <Play v-else class="w-4 h-4" />
            <span>{{ isStarting ? t("bisect.starting") : t("bisect.start.run") }}</span>
          </button>
        </div>

        <!-- Quick tips -->
        <div class="grid grid-cols-3 gap-3">
          <div class="p-3.5 rounded-xl border border-line bg-surface/20 flex flex-col gap-1">
            <span class="text-xs font-bold text-ink flex items-center gap-1.5">
              <Binary class="w-3.5 h-3.5 text-accent" />
              <span>{{ t("bisect.how.title") }}</span>
            </span>
            <span class="text-[11px] text-ink-muted leading-relaxed">
              {{ t("bisect.how.desc") }}
            </span>
          </div>
          <div class="p-3.5 rounded-xl border border-line bg-surface/20 flex flex-col gap-1">
            <span class="text-xs font-bold text-ink flex items-center gap-1.5">
              <AlertTriangle class="w-3.5 h-3.5 text-warning" />
              <span>{{ t("bisect.require.title") }}</span>
            </span>
            <span class="text-[11px] text-ink-muted leading-relaxed">
              {{ t("bisect.require.desc") }}
            </span>
          </div>
          <div class="p-3.5 rounded-xl border border-line bg-surface/20 flex flex-col gap-1">
            <span class="text-xs font-bold text-ink flex items-center gap-1.5">
              <RotateCcw class="w-3.5 h-3.5 text-cyan" />
              <span>{{ t("bisect.finish.title") }}</span>
            </span>
            <span class="text-[11px] text-ink-muted leading-relaxed">
              {{ t("bisect.finish.desc") }}
            </span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
