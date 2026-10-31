<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import {
  Check,
  Copy,
  GitPullRequest,
  GitMerge,
  GitCompare,
  RotateCcw,
  GitBranch,
  Tag,
  AlertTriangle,
  ExternalLink,
  StickyNote,
  Send,
  FileText,
  RefreshCw,
  ChevronDown,
  ShieldCheck,
  ShieldAlert,
  ShieldX,
  Wand2,
  ScanEye,
  Link,
  Clock,
  Sparkles,
} from "lucide-vue-next";
import type { CommitItem, CommitDetail, CommitCIStatus } from "../../types";
import { gitService } from "../../services/gitService";
import { isAIConfigured } from "../../services/aiService";
import { openAiSetup } from "../../composables/useAiSetup";
import { startAIStream, type AIStreamSession } from "../../services/aiStreamService";
import { pushToast } from "../../composables/useToast";
import FeatureGate from "../ui/FeatureGate.vue";
import AiReasoningPanel from "../ui/AiReasoningPanel.vue";
import AiReviewModal from "../workspace/AiReviewModal.vue";
import { t } from "../../i18n";
import { relativeTime } from "../../utils/relativeTime";
import { generateDeepLink } from "../../utils/deepLink";

type CommitDetailWithPatch = CommitDetail & { diffTruncated?: boolean };

const props = defineProps<{
  selectedCommit: CommitItem | null;
  copiedHash: string | null;
  commitDetail: CommitDetailWithPatch | null;
  isLoadingDetail: boolean;
  currentCommitNote: string;
  isPushingNotes: boolean;
  repoPath?: string;
  error?: string | null;
}>();

const emit = defineEmits<{
  (e: "copy-hash", hash: string): void;
  (e: "rebase-onto"): void;
  (e: "rebase-dialog", hash: string): void;
  (e: "cherry-pick"): void;
  (e: "create-fixup"): void;
  (e: "revert"): void;
  (e: "revert-no-commit"): void;
  (e: "checkout-detached"): void;
  (e: "new-branch"): void;
  (e: "new-tag"): void;
  (e: "reset"): void;
  (e: "open-web", hash: string): void;
  (e: "signature-details", hash: string): void;
  (e: "edit-note"): void;
  (e: "delete-note"): void;
  (e: "push-notes"): void;
  (e: "open-asset-diff", filePath: string): void;
  (e: "open-timelapse", filePath: string): void;
}>();

const copiedMessage = ref(false);
let copyMessageTimer: ReturnType<typeof setTimeout> | null = null;

// ---- AI commit summary (advisor plan F2, flag aiSummary) --------------------
const aiSummary = ref("");
const aiSummaryReasoning = ref("");
const aiSummaryError = ref("");
const aiSummarizing = ref(false);
const aiSummaryExpanded = ref(true);
let aiSummarySeq = 0;
let summarySession: AIStreamSession | null = null;
const showAiReview = ref(false);

async function openAiReview(): Promise<void> {
  if (!props.selectedCommit?.hash) return;
  if (!(await isAIConfigured())) {
    openAiSetup();
    return;
  }
  showAiReview.value = true;
}

function stopSummaryStream(): void {
  const session = summarySession;
  summarySession = null;
  if (session) void session.cancel().finally(() => session.dispose());
}

// 切换提交即丢弃上一条摘要：摘要永远对应检查器当前展示的提交。
watch(
  () => [props.selectedCommit?.hash, props.repoPath] as const,
  () => {
    aiSummarySeq++;
    stopSummaryStream();
    aiSummary.value = "";
    aiSummaryReasoning.value = "";
    aiSummaryError.value = "";
    aiSummarizing.value = false;
    aiSummaryExpanded.value = true;
  }
);

onUnmounted(stopSummaryStream);

async function cancelAiSummary(): Promise<void> {
  const session = summarySession;
  if (!session) return;
  await session.cancel();
}

async function generateAiSummary() {
  if (aiSummarizing.value) return;
  const commit = props.selectedCommit;
  if (!commit) return;
  if (!props.repoPath) {
    pushToast(t("ai.notconfigured"), { tone: "warning" });
    return;
  }
  if (!(await isAIConfigured())) {
    openAiSetup();
    return;
  }

  const seq = ++aiSummarySeq;
  const hash = commit.hash;
  const repoPath = props.repoPath;
  aiSummarizing.value = true;
  aiSummary.value = "";
  aiSummaryReasoning.value = "";
  aiSummaryError.value = "";
  try {
    const session = await startAIStream(
      { kind: "summary", repoPath, commitHash: hash },
      {
        onDelta: (delta) => {
          if (seq === aiSummarySeq && props.selectedCommit?.hash === hash) {
            aiSummary.value += delta;
          }
        },
        onReasoning: (delta) => {
          if (seq === aiSummarySeq && props.selectedCommit?.hash === hash) {
            aiSummaryReasoning.value += delta;
          }
        },
      }
    );
    summarySession = session;
    if (seq !== aiSummarySeq || props.selectedCommit?.hash !== hash) {
      summarySession = null;
      await session.cancel();
      session.dispose();
      return;
    }
    const outcome = await session.completion;
    summarySession = null;
    if (seq !== aiSummarySeq || props.selectedCommit?.hash !== hash) return;
    if (outcome.kind === "done") {
      aiSummary.value = outcome.text;
      aiSummaryExpanded.value = true;
    } else if (outcome.kind === "cancelled") {
      aiSummary.value = "";
    } else {
      aiSummary.value = "";
      aiSummaryError.value = outcome.message;
    }
  } catch (err: any) {
    if (seq !== aiSummarySeq || props.selectedCommit?.hash !== hash) return;
    aiSummary.value = "";
    aiSummaryError.value = err?.message || String(err);
  } finally {
    if (seq === aiSummarySeq) aiSummarizing.value = false;
  }
}

async function copyAiSummary() {
  if (!aiSummary.value) return;
  try {
    if (typeof navigator !== "undefined" && navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(aiSummary.value);
    } else if (typeof document !== "undefined") {
      const textarea = document.createElement("textarea");
      textarea.value = aiSummary.value;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    pushToast(t("ws.copy.done"), { tone: "success", duration: 1500 });
  } catch {
    /* clipboard unavailable */
  }
}

async function copyCommitMessage() {
  if (!props.selectedCommit) return;
  const message =
    props.selectedCommit.subject +
    (props.selectedCommit.body ? "\n\n" + props.selectedCommit.body : "");
  try {
    if (typeof navigator !== "undefined" && navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(message);
    } else if (typeof document !== "undefined") {
      const textarea = document.createElement("textarea");
      textarea.value = message;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    }
    copiedMessage.value = true;
    if (copyMessageTimer) clearTimeout(copyMessageTimer);
    copyMessageTimer = setTimeout(() => {
      copiedMessage.value = false;
      copyMessageTimer = null;
    }, 1500);
  } catch {
    // Clipboard unavailable
  }
}

const copiedDeepLink = ref(false);
let copyDeepLinkTimer: ReturnType<typeof setTimeout> | null = null;

async function copyDeepLink() {
  if (!props.selectedCommit) return;
  const link = generateDeepLink({
    action: "open",
    repo: props.repoPath || undefined,
    tab: "history",
    ref: props.selectedCommit.hash,
  });
  try {
    if (typeof navigator !== "undefined" && navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(link);
    } else if (typeof document !== "undefined") {
      const textarea = document.createElement("textarea");
      textarea.value = link;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    }
    copiedDeepLink.value = true;
    if (copyDeepLinkTimer) clearTimeout(copyDeepLinkTimer);
    copyDeepLinkTimer = setTimeout(() => {
      copiedDeepLink.value = false;
      copyDeepLinkTimer = null;
    }, 1500);
  } catch {
    // Clipboard unavailable
  }
}

const ciStatus = ref<CommitCIStatus | null>(null);
const isLoadingCI = ref(false);

watch(
  () => [props.repoPath, props.selectedCommit?.hash],
  async () => {
    ciStatus.value = null;
    const repo = props.repoPath;
    const hash = props.selectedCommit?.hash;
    if (!repo || !hash) return;
    isLoadingCI.value = true;
    try {
      ciStatus.value = await gitService.getCommitCIStatus(repo, hash);
    } catch {
      ciStatus.value = null;
    } finally {
      isLoadingCI.value = false;
    }
  },
  { immediate: true }
);

const gpgStatus = computed<"G" | "U" | "B" | null>(() => {
  const raw =
    props.selectedCommit?.gpgStatus ||
    props.commitDetail?.gpgStatus ||
    props.selectedCommit?.signatureStatus ||
    props.commitDetail?.commit?.gpgStatus ||
    props.commitDetail?.commit?.signatureStatus;
  if (!raw) return null;
  const upper = raw.toUpperCase();
  if (upper === "G" || upper === "U" || upper === "B") {
    return upper;
  }
  return null;
});

const gpgStatusInfo = computed(() => {
  const status = gpgStatus.value;
  if (!status) return null;
  switch (status) {
    case "G":
      return {
        status: "G" as const,
        label: t("history.inspector.gpg.verified"),
        cls: "bg-success-bg text-success border-success/30 hover:bg-success-bg-strong",
      };
    case "U":
      return {
        status: "U" as const,
        label: t("history.inspector.gpg.untrusted"),
        cls: "bg-warning-bg text-warning border-warning/30 hover:bg-warning-bg-strong",
      };
    case "B":
      return {
        status: "B" as const,
        label: t("history.inspector.gpg.invalid"),
        cls: "bg-danger-bg text-danger border-danger/30 hover:bg-danger-bg-strong",
      };
    default:
      return null;
  }
});

const PATCH_RENDER_LIMIT = 4000;
const showPatch = ref(false);
const patchWrap = ref(false);
const patchShowAll = ref(false);

watch(
  () => props.selectedCommit?.hash,
  () => {
    showPatch.value = false;
    patchWrap.value = false;
    patchShowAll.value = false;
    if (copyMessageTimer) {
      clearTimeout(copyMessageTimer);
      copyMessageTimer = null;
    }
    copiedMessage.value = false;
  }
);

function togglePatchPreview() {
  showPatch.value = !showPatch.value;
}

defineExpose({
  togglePatchPreview,
  showPatch,
});

type PatchLineKind = "add" | "del" | "hunk" | "meta" | "context";

interface PatchLine {
  text: string;
  kind: PatchLineKind;
}

const PATCH_META_PREFIXES = [
  "diff --git",
  "index ",
  "new file mode",
  "deleted file mode",
  "old mode",
  "new mode",
  "similarity index",
  "dissimilarity index",
  "rename from",
  "rename to",
  "copy from",
  "copy to",
  "Binary files ",
  "\\ No newline",
];

function classifyPatchLine(text: string): PatchLineKind {
  if (text.startsWith("@@")) return "hunk";
  if (text.startsWith("+++") || text.startsWith("---")) return "meta";
  if (PATCH_META_PREFIXES.some((prefix) => text.startsWith(prefix))) return "meta";
  if (text.startsWith("+")) return "add";
  if (text.startsWith("-")) return "del";
  return "context";
}

const patchLines = computed<PatchLine[]>(() => {
  const diff = props.commitDetail?.diff ?? "";
  if (!diff) return [];
  const raw = diff.split("\n");
  if (raw.length > 0 && raw[raw.length - 1] === "") raw.pop();
  return raw.map((text) => ({ text, kind: classifyPatchLine(text) }));
});

const patchLineCount = computed(() => patchLines.value.length);
const patchOverflow = computed(
  () => !patchShowAll.value && patchLineCount.value > PATCH_RENDER_LIMIT
);
const visiblePatchLines = computed(() =>
  patchOverflow.value ? patchLines.value.slice(0, PATCH_RENDER_LIMIT) : patchLines.value
);
const patchTruncatedByServer = computed(() => Boolean(props.commitDetail?.diffTruncated));
const patchIsBinary = computed(() => {
  const diff = props.commitDetail?.diff ?? "";
  return /^Binary files /m.test(diff) || diff.includes("GIT binary patch");
});

const ASSET_EXTENSIONS = new Set([
  ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".ico", ".tiff", ".psd", ".psb", ".tga",
  ".fbx", ".obj", ".glb", ".gltf", ".ma", ".mb", ".blend", ".max", ".uasset", ".umap", ".unity", ".prefab",
]);

function isAssetFilePath(path: string): boolean {
  const idx = path.lastIndexOf(".");
  return idx >= 0 && ASSET_EXTENSIONS.has(path.slice(idx).toLowerCase());
}

const firstAssetFile = computed(() => {
  return props.commitDetail?.files?.find((f) => isAssetFilePath(f.path))?.path ?? "";
});

function patchLineClass(kind: PatchLineKind): string {
  switch (kind) {
    case "add":
      return "bg-success-bg text-success";
    case "del":
      return "bg-danger-bg text-danger";
    case "hunk":
      return "bg-accent-bg text-accent border-y border-accent/20";
    case "meta":
      return "text-ink-muted";
    default:
      return "text-ink-secondary";
  }
}

</script>

<template>
    <!-- Right: Commit Inspector Drawer -->
    <aside class="w-80 xl:w-96 bg-surface flex flex-col shrink-0 overflow-y-auto select-none border-l border-line/40">
      <div v-if="selectedCommit" class="p-4 flex flex-col gap-4">
        <!-- Hash & Copy & Quick Actions -->
        <div class="flex items-center justify-between pb-3 border-b border-line">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="font-mono text-xs font-semibold text-accent">{{ selectedCommit.shortHash }}</span>
            <button
              :aria-label="t('history.inspector.copy.hash')"
              @click="emit('copy-hash', selectedCommit.hash)"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
              :title="t('history.inspector.copy.hash')"
            >
              <Check v-if="copiedHash === selectedCommit.hash" class="w-3.5 h-3.5 text-success" aria-hidden="true" />
              <Copy v-else class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
            <button
              :aria-label="t('history.inspector.copy.message')"
              @click="copyCommitMessage"
              class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
              :title="copiedMessage ? t('history.inspector.copy.message.done') : t('history.inspector.copy.message')"
            >
              <Check v-if="copiedMessage" class="w-3.5 h-3.5 text-success" aria-hidden="true" />
              <Copy v-else class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
            <button
              v-if="gpgStatusInfo"
              type="button"
              data-testid="gpg-status-badge"
              @click="emit('signature-details', selectedCommit.hash)"
              class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-medium transition cursor-pointer border select-none"
              :class="gpgStatusInfo.cls"
              :title="t('history.inspector.signature.hint')"
            >
            <ShieldCheck v-if="gpgStatusInfo.status === 'G'" class="w-3.5 h-3.5 text-success" aria-hidden="true" />
            <ShieldAlert v-else-if="gpgStatusInfo.status === 'U'" class="w-3.5 h-3.5 text-warning" aria-hidden="true" />
            <ShieldX v-else-if="gpgStatusInfo.status === 'B'" class="w-3.5 h-3.5 text-danger" aria-hidden="true" />
              <span>{{ gpgStatusInfo.label }}</span>
            </button>
          </div>
          <span class="text-[10px] text-ink-muted">{{ relativeTime(selectedCommit.date, selectedCommit.relativeDate) }}</span>
        </div>

        <!-- Subject & Body -->
        <div class="flex flex-col gap-1.5">
          <h2 class="text-sm font-bold text-ink leading-snug">{{ selectedCommit.subject }}</h2>
          <p v-if="selectedCommit.body" class="text-xs text-ink-secondary whitespace-pre-wrap font-sans leading-relaxed bg-canvas p-2.5 rounded-lg border border-line/60">
            {{ selectedCommit.body }}
          </p>
        </div>

        <!-- AI summary card (flag aiSummary) -->
        <FeatureGate flag="aiSummary">
          <div
            v-if="aiSummarizing || aiSummary || aiSummaryError"
            data-testid="ai-summary-card"
            class="flex flex-col gap-1.5 p-2.5 rounded-xl bg-canvas border border-accent/30"
          >
            <div class="flex items-center justify-between gap-2">
              <button
                type="button"
                class="flex items-center gap-1.5 text-[11px] font-semibold text-accent cursor-pointer select-none w-fit"
                :aria-expanded="aiSummaryExpanded"
                aria-controls="ai-summary-body"
                @click="aiSummaryExpanded = !aiSummaryExpanded"
              >
                <Sparkles class="w-3 h-3" aria-hidden="true" />
                <span>{{ t("ai.summary.card.title") }}</span>
                <ChevronDown class="w-3 h-3 transition-transform" :class="aiSummaryExpanded ? '' : '-rotate-90'" aria-hidden="true" />
              </button>
              <button
                v-if="aiSummarizing"
                type="button"
                data-testid="ai-summary-cancel"
                class="flex items-center gap-1 text-[10px] text-ink-muted hover:text-danger transition cursor-pointer select-none"
                @click="cancelAiSummary"
              >
                {{ t("ai.stream.cancel") }}
              </button>
              <button
                v-else-if="aiSummary"
                type="button"
                class="flex items-center gap-1 text-[10px] text-ink-muted hover:text-ink transition cursor-pointer select-none"
                :title="t('ws.copy')"
                @click="copyAiSummary"
              >
                <Copy class="w-3 h-3" aria-hidden="true" />
                <span>{{ t("ws.copy") }}</span>
              </button>
            </div>
            <p
              v-if="aiSummaryExpanded && aiSummary"
              id="ai-summary-body"
              class="text-xs text-ink-secondary whitespace-pre-wrap font-sans leading-relaxed select-text"
            >{{ aiSummary }}</p>
            <p v-else-if="aiSummarizing" id="ai-summary-body" class="text-xs text-ink-muted flex items-center gap-1.5">
              <RefreshCw class="w-3 h-3 animate-spin" />
              <span>{{ t("ai.summarizing") }}</span>
            </p>
            <div v-else-if="aiSummaryError" id="ai-summary-body" class="space-y-1.5" role="alert">
              <p class="text-[11px] text-danger leading-relaxed">{{ t("ai.summary.failed", { message: aiSummaryError }) }}</p>
              <button
                type="button"
                class="text-[10px] font-semibold text-accent hover:text-accent-strong transition cursor-pointer"
                @click="generateAiSummary"
              >
                {{ t("ai.summary.retry") }}
              </button>
            </div>
            <AiReasoningPanel :text="aiSummaryReasoning" />
          </div>
        </FeatureGate>

        <!-- Quick Operation Buttons (Cherry-pick, Revert, Tag, Branch, Reset, Rebase) -->
        <div class="flex flex-wrap gap-1.5 p-2 rounded-xl bg-surface border border-line">
          <button
            @click="emit('rebase-onto')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-warning-bg hover:bg-warning-bg-strong text-xs font-semibold text-warning transition cursor-pointer border border-warning/30"
            :title="t('history.inspector.rebase.onto.hint')"
          >
            <GitPullRequest class="w-3.5 h-3.5" aria-hidden="true" />
            <span>{{ t("history.inspector.rebase.onto") }}</span>
          </button>

          <button
            @click="emit('rebase-dialog', selectedCommit.hash)"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-cyan-bg hover:bg-cyan-bg-strong text-xs font-semibold text-cyan transition cursor-pointer border border-cyan/30"
            :title="t('history.inspector.rebase.interactive.hint')"
          >
            <GitMerge class="w-3.5 h-3.5" aria-hidden="true" />
            <span>{{ t("history.inspector.rebase.interactive") }}</span>
          </button>

          <button
            @click="emit('cherry-pick')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.cherry.pick.hint')"
          >
            <GitCompare class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
            <span>{{ t("history.inspector.cherry.pick") }}</span>
          </button>

          <FeatureGate flag="aiSummary">
            <button
              @click="generateAiSummary"
              :disabled="aiSummarizing"
              data-testid="ai-summary-btn"
              class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60 disabled:opacity-50 disabled:cursor-not-allowed"
              :title="t('ai.summarize')"
            >
              <Sparkles class="w-3.5 h-3.5 text-accent" :class="{ 'animate-spin': aiSummarizing }" aria-hidden="true" />
              <span>{{ aiSummarizing ? t("ai.summarizing") : t("ai.summarize") }}</span>
            </button>
          </FeatureGate>

          <FeatureGate flag="aiSummary">
            <button
              @click="openAiReview"
              data-testid="ai-review-commit-btn"
              class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
              :title="t('ai.review.desc')"
            >
              <ShieldCheck class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
              <span>{{ t("ai.review.title") }}</span>
            </button>
          </FeatureGate>

          <button
            @click="emit('create-fixup')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.fixup.hint')"
          >
            <Wand2 class="w-3.5 h-3.5 text-purple" aria-hidden="true" />
            <span>{{ t("history.inspector.fixup") }}</span>
          </button>

          <button
            @click="emit('revert')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.revert.hint')"
          >
            <RotateCcw class="w-3.5 h-3.5 text-warning" aria-hidden="true" />
            <span>{{ t("history.inspector.revert") }}</span>
          </button>

          <button
            @click="emit('revert-no-commit')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.revert.no.commit.hint')"
          >
            <RotateCcw class="w-3.5 h-3.5 text-warning" aria-hidden="true" />
            <span>{{ t("history.inspector.revert.no.commit") }}</span>
          </button>

          <button
            @click="emit('checkout-detached')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.detach.hint')"
          >
            <GitBranch class="w-3.5 h-3.5 text-cyan" aria-hidden="true" />
            <span>{{ t("history.inspector.detach") }}</span>
          </button>

          <button
            @click="emit('new-branch')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.branch.hint')"
          >
            <GitBranch class="w-3.5 h-3.5 text-cyan" aria-hidden="true" />
            <span>{{ t("history.inspector.branch") }}</span>
          </button>

          <button
            @click="emit('new-tag')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.tag.hint')"
          >
            <Tag class="w-3.5 h-3.5 text-purple" aria-hidden="true" />
            <span>{{ t("history.inspector.tag") }}</span>
          </button>

          <button
            @click="emit('reset')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-danger-bg text-xs font-medium text-danger transition cursor-pointer border border-danger/30"
            :title="t('history.inspector.reset.hint')"
          >
            <AlertTriangle class="w-3.5 h-3.5" aria-hidden="true" />
            <span>{{ t("history.inspector.reset") }}</span>
          </button>

          <button
            @click="emit('signature-details', selectedCommit.hash)"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.signature.hint')"
          >
            <ShieldCheck class="w-3.5 h-3.5 text-success" aria-hidden="true" />
            <span>{{ t("history.inspector.signature") }}</span>
          </button>

          <button
            @click="emit('open-web', selectedCommit.hash)"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.open.web.hint')"
          >
            <ExternalLink class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
            <span>{{ t("history.inspector.open.web") }}</span>
          </button>

          <button
            @click="copyDeepLink"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-xs font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.inspector.deeplink.hint')"
          >
            <Check v-if="copiedDeepLink" class="w-3.5 h-3.5 text-success" aria-hidden="true" />
            <Link v-else class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
            <span>{{ copiedDeepLink ? t("common.copied") : t("history.inspector.deeplink") }}</span>
          </button>
        </div>

        <!-- Author Meta -->
        <div class="grid grid-cols-2 gap-2 text-xs bg-canvas p-2.5 rounded-lg border border-line/60">
          <div class="flex flex-col gap-0.5">
            <span class="text-[10px] text-ink-muted uppercase tracking-wider">{{ t("history.inspector.author") }}</span>
            <span class="font-medium text-ink truncate">{{ selectedCommit.authorName }}</span>
          </div>
          <div class="flex flex-col gap-0.5">
            <span class="text-[10px] text-ink-muted uppercase tracking-wider">{{ t("history.inspector.email") }}</span>
            <span class="text-ink-secondary font-mono text-[10px] truncate">{{ selectedCommit.authorEmail }}</span>
          </div>
        </div>

        <!-- CI Pipeline Status (via Git Notes / refs/notes/ci) -->
        <div
          v-if="ciStatus && ciStatus.status !== 'none'"
          class="p-2.5 rounded-xl border flex items-center justify-between text-xs transition"
          :class="{
            'bg-success-bg border-success/30 text-success': ciStatus.status === 'success',
            'bg-danger-bg border-danger/30 text-danger': ciStatus.status === 'failure',
            'bg-warning-bg border-warning/30 text-warning': ciStatus.status === 'pending'
          }"
        >
          <div class="flex items-center gap-2">
            <ShieldCheck v-if="ciStatus.status === 'success'" class="w-4 h-4 text-success" />
            <ShieldAlert v-else-if="ciStatus.status === 'failure'" class="w-4 h-4 text-danger" />
            <RefreshCw v-else class="w-4 h-4 text-warning animate-spin" />

            <div class="flex flex-col">
              <span class="font-bold text-[11px]">
                {{ ciStatus.pipeline || t("history.inspector.ci.title") }}
              </span>
              <span class="text-[10px] opacity-80">
                <template v-if="ciStatus.status === 'success'">{{ t("history.inspector.ci.passed") }}</template>
                <template v-else-if="ciStatus.status === 'failure'">{{ t("history.inspector.ci.failed") }}</template>
                <template v-else>{{ t("history.inspector.ci.pending") }}</template>
                <template v-if="ciStatus.duration"> · {{ ciStatus.duration }}</template>
              </span>
            </div>
          </div>

          <a
            v-if="ciStatus.url"
            :href="ciStatus.url"
            target="_blank"
          rel="noopener noreferrer"
            class="flex items-center gap-1 text-[10px] px-2 py-0.5 rounded bg-surface border border-line hover:bg-raised transition cursor-pointer"
          >
            <ExternalLink class="w-3 h-3" aria-hidden="true" />
            <span>CI</span>
          </a>
        </div>

        <!-- Git Notes Card -->
      <div class="p-3 rounded-xl border border-line bg-surface flex flex-col gap-2">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1.5 text-xs font-bold text-ink">
            <StickyNote class="w-3.5 h-3.5 text-ink-muted" />
              <span>{{ t("history.inspector.notes.title") }}</span>
            </div>
            <div class="flex items-center gap-1.5">
              <button
                v-if="currentCommitNote"
                @click="emit('edit-note')"
                class="px-2 py-0.5 rounded hover:bg-raised text-ink text-[11px] font-medium transition cursor-pointer"
              >
                {{ t("history.inspector.notes.edit") }}
              </button>
              <button
                v-if="currentCommitNote"
                @click="emit('delete-note')"
                class="px-2 py-0.5 rounded hover:bg-danger-bg text-danger text-[11px] font-medium transition cursor-pointer"
              >
                {{ t("history.inspector.notes.delete") }}
              </button>
              <button
                v-if="currentCommitNote"
                @click="emit('push-notes')"
                :disabled="isPushingNotes"
                class="flex items-center gap-1 px-2 py-0.5 rounded bg-surface hover:bg-raised text-[11px] text-ink font-mono border border-line transition cursor-pointer"
                :title="t('history.inspector.notes.push.hint')"
              >
                <Send class="w-2.5 h-2.5 text-accent" aria-hidden="true" />
                <span>{{ isPushingNotes ? t("history.inspector.notes.pushing") : t("history.inspector.notes.push") }}</span>
              </button>
              <button
                v-else
                @click="emit('edit-note')"
                class="flex items-center gap-1 px-2 py-0.5 rounded bg-warning-bg hover:bg-warning-bg-strong text-warning text-xs font-semibold transition cursor-pointer"
              >
                {{ t("history.inspector.notes.add") }}
              </button>
            </div>
          </div>

          <div v-if="currentCommitNote" class="text-xs text-ink whitespace-pre-wrap font-sans bg-canvas p-2.5 rounded-lg border border-line/40 leading-relaxed select-text">
            {{ currentCommitNote }}
          </div>
          <div v-else class="text-[11px] text-ink-muted italic">
            {{ t("history.inspector.notes.empty") }}
          </div>
        </div>

        <!-- Changed Files in Commit -->
        <div class="flex flex-col gap-2">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">{{
              t("history.inspector.files.title")
            }}</span>
            <span v-if="commitDetail" class="text-[10px] text-ink-muted font-mono">
              +{{ commitDetail.additions }} / -{{ commitDetail.deletions }}
            </span>
          </div>

          <div v-if="isLoadingDetail" class="py-4 text-center text-xs text-ink-muted">
            <RefreshCw class="w-4 h-4 animate-spin mx-auto mb-1" /> {{ t("history.inspector.files.loading") }}
          </div>

          <div v-else-if="commitDetail" class="space-y-1 max-h-72 overflow-y-auto">
            <div
              v-for="f in commitDetail.files"
              :key="f.path"
              class="flex items-center justify-between p-2 rounded-lg bg-surface border border-line/60 text-xs font-mono"
            >
              <div class="flex items-center gap-1.5 truncate min-w-0">
                <FileText class="w-3.5 h-3.5 text-ink-muted shrink-0" />
                <span class="truncate text-ink-secondary">{{ f.path }}</span>
              </div>
              <div class="flex items-center gap-2 shrink-0 text-[10px]">
                <button
                  v-if="isAssetFilePath(f.path)"
                  @click.stop="emit('open-asset-diff', f.path)"
                  class="p-1 rounded hover:bg-raised text-ink-muted hover:text-accent cursor-pointer transition"
                  :title="t('ws.asset.compare.title')"
                >
                  <ScanEye class="w-3.5 h-3.5" aria-hidden="true" />
                </button>
                <button
                  @click.stop="emit('open-timelapse', f.path)"
                  class="p-1 rounded hover:bg-raised text-ink-muted hover:text-accent cursor-pointer transition"
                  :title="t('ownership.btn.inspect')"
                >
                  <Clock class="w-3.5 h-3.5" aria-hidden="true" />
                </button>
                <span v-if="f.additions" class="text-success font-bold">+{{ f.additions }}</span>
                <span v-if="f.deletions" class="text-danger font-bold">-{{ f.deletions }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Commit Patch Preview (collapsible, collapsed by default) -->
        <div v-if="commitDetail" class="flex flex-col gap-2">
          <button
            @click="showPatch = !showPatch"
            class="flex items-center justify-between w-full px-2.5 py-1.5 rounded-lg bg-surface border border-line hover:bg-raised transition cursor-pointer"
            :title="showPatch ? t('history.inspector.patch.collapse') : t('history.inspector.patch.expand')"
          >
            <span class="flex items-center gap-1.5 text-[11px] font-semibold text-ink-secondary uppercase tracking-wider">
              <ChevronDown
                class="w-3.5 h-3.5 text-ink-muted transition-transform"
                :class="showPatch ? 'rotate-0' : '-rotate-90'" aria-hidden="true"
              />
              {{ t("history.inspector.patch.title") }}
            </span>
            <span class="flex items-center gap-2 text-[10px] font-mono">
              <span class="text-success font-bold">+{{ commitDetail.additions }}</span>
              <span class="text-danger font-bold">-{{ commitDetail.deletions }}</span>
              <span v-if="patchLineCount > 0" class="text-ink-muted">{{
                t("history.inspector.patch.lines", { count: patchLineCount })
              }}</span>
            </span>
          </button>

          <template v-if="showPatch">
            <div class="flex items-center justify-between gap-2">
              <span
                v-if="patchTruncatedByServer"
                class="flex items-center gap-1 text-[10px] text-warning leading-tight"
                :title="t('history.inspector.patch.truncated.hint')"
              >
                <AlertTriangle class="w-3 h-3 shrink-0" />
                {{ t("history.inspector.patch.truncated", { count: patchLineCount }) }}
              </span>
              <span v-else class="text-[10px] text-ink-muted font-mono truncate">{{ t("history.inspector.patch.unified") }}</span>
              <button
                @click="patchWrap = !patchWrap"
                class="px-2 py-0.5 rounded-md bg-canvas hover:bg-raised border border-line/60 text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer shrink-0"
                :title="patchWrap ? t('history.inspector.wrap.off') : t('history.inspector.wrap.on')"
              >
                {{ patchWrap ? t("history.inspector.wrap.off.label") : t("history.inspector.wrap.on.label") }}
              </button>
            </div>

            <div
              v-if="!commitDetail.diff"
              class="py-6 text-center text-[11px] text-ink-muted rounded-lg border border-line/60 bg-canvas"
            >
              {{ t("history.inspector.patch.empty") }}
            </div>

            <div
              v-else-if="patchIsBinary"
              class="py-6 flex flex-col items-center justify-center gap-2.5 text-[11px] text-ink-muted rounded-lg border border-line/60 bg-canvas"
            >
              <span>{{ t("history.inspector.patch.binary") }}</span>
              <button
                v-if="firstAssetFile"
                @click="emit('open-asset-diff', firstAssetFile)"
                class="px-2.5 py-1 rounded-md bg-accent text-accent-contrast hover:bg-accent-strong text-xs font-semibold cursor-pointer transition flex items-center gap-1.5 shadow-xs"
                :title="t('ws.asset.compare.title')"
              >
                <ScanEye class="w-3.5 h-3.5" aria-hidden="true" />
                <span>{{ t("ws.asset.compare") }} ({{ firstAssetFile.split('/').pop() }})</span>
              </button>
            </div>

            <div
              v-else
              class="max-h-[420px] overflow-auto rounded-lg border border-line/60 bg-canvas font-mono text-[10px] leading-relaxed"
            >
              <div
                v-for="(line, lineIndex) in visiblePatchLines"
                :key="lineIndex"
                class="flex items-start px-2 py-px"
                :class="patchLineClass(line.kind)"
              >
                <span class="w-8 shrink-0 select-none text-right pr-2 text-ink-muted">
                  {{ lineIndex + 1 }}
                </span>
                <span
                  class="flex-1 min-w-0"
                  :class="patchWrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre'"
                >{{ line.text || " " }}</span>
              </div>

              <div v-if="patchOverflow" class="p-2 text-center border-t border-line/40 bg-surface">
                <button
                  @click="patchShowAll = true"
                  class="px-2.5 py-1 rounded-md bg-raised hover:bg-hover-strong border border-line text-[10px] text-ink-secondary hover:text-ink transition cursor-pointer"
                >
                  {{
                    t("history.inspector.patch.show.all", {
                      limit: PATCH_RENDER_LIMIT,
                      count: patchLineCount,
                    })
                  }}
                </button>
              </div>
              <div
                v-else-if="patchShowAll && patchLineCount > PATCH_RENDER_LIMIT"
                class="p-1.5 text-center text-[10px] text-ink-muted"
              >
                {{ t("history.inspector.patch.all.shown", { count: patchLineCount }) }}
              </div>
            </div>
          </template>
        </div>
      </div>

      <div v-else-if="error" class="flex-1 flex flex-col items-center justify-center p-6 text-xs text-danger text-center gap-2">
        <AlertTriangle class="w-6 h-6 text-danger" />
        <span class="font-semibold">{{ t("common.error") }}</span>
        <span class="text-[11px] text-ink-muted max-w-xs break-words">{{ error }}</span>
      </div>

      <div v-else class="flex-1 flex items-center justify-center text-xs text-ink-muted">
        {{ t("history.inspector.empty") }}
      </div>

      <AiReviewModal
        :is-open="showAiReview"
        :repo-path="repoPath"
        :commit-hash="selectedCommit?.hash || ''"
        @close="showAiReview = false"
      />
    </aside>
</template>
