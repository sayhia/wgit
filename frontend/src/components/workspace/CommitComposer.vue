<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from "vue";
import { isActionCustomized } from "../../utils/shortcuts";
import { GitCommit, Sparkles, Send, FileText, Save, History, Zap, AlertCircle, ShieldCheck, Clock, ChevronDown, MoreHorizontal, Eye } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { isAIConfigured, previewAIRequest } from "../../services/aiService";
import { openAiSetup } from "../../composables/useAiSetup";
import { startAIStream, type AIStreamSession } from "../../services/aiStreamService";
import { confirmAction } from "../../composables/useConfirm";
import { ensureFlagsLoaded, isFeatureEnabled } from "../../composables/useFeatureFlags";
import FeatureGate from "../ui/FeatureGate.vue";
import AiReasoningPanel from "../ui/AiReasoningPanel.vue";
import AiPayloadPreviewModal from "./AiPayloadPreviewModal.vue";
import type { CommitLintResult, CommitLintReport, GpgHealthReport } from "../../types";
import { t } from "../../i18n";

const props = withDefaults(
  defineProps<{
    subject: string;
    body: string;
    amend: boolean;
    autoPush: boolean;
    noVerify?: boolean;
    committing: boolean;
    isNewRepo?: boolean;
    repoPath?: string;
  }>(),
  {
    noVerify: false,
    isNewRepo: false,
  }
);

const emit = defineEmits<{
  (e: "update:subject", value: string): void;
  (e: "update:body", value: string): void;
  (e: "update:amend", value: boolean): void;
  (e: "update:autoPush", value: boolean): void;
  (e: "update:noVerify", value: boolean): void;
  (e: "commit", noVerify: boolean): void;
  (e: "fast-commit"): void;
  (e: "generate"): void;
  (e: "audit-sentinel"): void;
  (e: "load-template"): void;
  (e: "load-last"): void;
  (e: "save-template"): void;
  (e: "notify", message: string): void;
}>();

const subjectModel = computed({
  get: () => props.subject,
  set: (value: string) => emit("update:subject", value),
});
const bodyModel = computed({
  get: () => props.body,
  set: (value: string) => emit("update:body", value),
});
const amendModel = computed({
  get: () => props.amend,
  set: (value: boolean) => emit("update:amend", value),
});
const autoPushModel = computed({
  get: () => props.autoPush,
  set: (value: boolean) => emit("update:autoPush", value),
});
const internalNoVerify = ref(props.noVerify ?? false);
watch(
  () => props.noVerify,
  (val) => {
    if (val !== undefined) internalNoVerify.value = val;
  }
);
const noVerifyModel = computed({
  get: () => (props.noVerify !== undefined ? props.noVerify : internalNoVerify.value),
  set: (value: boolean) => {
    internalNoVerify.value = value;
    emit("update:noVerify", value);
  },
});

const lintResult = ref<CommitLintResult | null>(null);
const commitLintReport = ref<CommitLintReport | null>(null);
const gpgHealth = ref<GpgHealthReport | null>(null);
let lintTimer: ReturnType<typeof setTimeout> | null = null;

async function checkGpg() {
  if (!props.repoPath) return;
  try {
    gpgHealth.value = await gitService.checkCommitGpgHealth(props.repoPath);
  } catch (err) {
    // No fitting i18n key for a probe failure: log it so the silently missing
    // GPG badge stays diagnosable instead of reading as "no signing issues".
    console.error("Failed to check commit GPG health:", err);
  }
}

watch(
  () => props.repoPath,
  () => {
    void checkGpg();
  },
  { immediate: true }
);

async function handleAutoFormat() {
  const full = props.body.trim() ? `${props.subject.trim()}\n\n${props.body.trim()}` : props.subject.trim();
  if (!full) return;
  try {
    const res = await gitService.formatCommitMessageWithTemplate(full, "conventional");
    if (res && res.formattedMessage) {
      const parts = res.formattedMessage.split("\n\n");
      emit("update:subject", parts[0] || "");
      emit("update:body", parts.slice(1).join("\n\n"));
      emit("notify", t("workspace.composer.auto.formatted"));
    }
  } catch (err: any) {
    emit("notify", String(err?.message || err));
  }
}

// ---- AI commit-message generation (F1, flag aiCommitMsg) --------------------
// The button is wrapped in <FeatureGate flag="aiCommitMsg"> so it stays dark
// until the flag rolls out. On click: availability probe first (no error
// stack when AI is unconfigured — a friendly notify instead), then a confirm
// before overwriting a non-empty draft, then staged diff collection through
// the existing per-file diff RPCs, and finally the backend AI RPC.
const aiGenerating = ref(false);
const showAiPreview = ref(false);
const aiPreviewText = ref("");
const aiPreviewBusy = ref(false);
const aiStreamText = ref("");
const aiReasoning = ref("");
const aiStreamActive = ref(false);
let aiStreamSession: AIStreamSession | null = null;

async function collectStagedDiff(): Promise<string> {
  if (!props.repoPath) return "";
  const status = await gitService.getRepoStatus(props.repoPath);
  const stagedFiles = status?.stagedFiles ?? [];
  const parts: string[] = [];
  for (const file of stagedFiles) {
    try {
      const diff = await gitService.getWorkingDiffAdvanced(props.repoPath, file.path, true, false);
      if (diff && diff.trim()) parts.push(diff);
    } catch {
      /* skip an unreadable single file; the rest still describes the change */
    }
  }
  return parts.join("\n");
}

type GenerateOutcome =
  | { kind: "applied" }
  | { kind: "cancelled" }
  | { kind: "busy" }
  | { kind: "unconfigured" }
  | { kind: "disabled" }
  | { kind: "no-staged-diff" }
  | { kind: "failed"; message: string };

async function runAiGeneration(): Promise<GenerateOutcome> {
  if (aiGenerating.value) return { kind: "busy" };
  aiGenerating.value = true;
  aiStreamText.value = "";
  aiReasoning.value = "";
  try {
    await ensureFlagsLoaded();
    if (!isFeatureEnabled("aiCommitMsg")) return { kind: "disabled" };
    if (!props.repoPath) return { kind: "unconfigured" };
    if (!(await isAIConfigured())) {
      openAiSetup();
      return { kind: "unconfigured" };
    }
    // Only fill directly into an empty draft; confirm before overwriting one.
    if (props.subject.trim() || props.body.trim()) {
      const ok = await confirmAction({ message: t("ai.generate.confirmOverwrite"), tone: "warning" });
      if (!ok) return { kind: "cancelled" };
    }
    const diff = await collectStagedDiff();
    if (!diff.trim()) return { kind: "no-staged-diff" };

    const session = await startAIStream(
      { kind: "commit", repoPath: props.repoPath, diff },
      {
        onDelta: (delta) => { aiStreamText.value += delta; },
        onReasoning: (delta) => { aiReasoning.value += delta; },
      }
    );
    aiStreamSession = session;
    aiStreamActive.value = true;
    const outcome = await session.completion;
    aiStreamSession = null;
    aiStreamActive.value = false;
    if (outcome.kind === "done") {
      const message = outcome.text.trim();
      if (!message) return { kind: "failed", message: t("ai.generate.empty") };
      const separator = message.indexOf("\n\n");
      const subject = (separator >= 0 ? message.slice(0, separator) : message).trim();
      const body = separator >= 0 ? message.slice(separator + 2).trim() : "";
      if (!subject) return { kind: "failed", message: t("ai.generate.empty") };
      emit("update:subject", subject);
      if (body) emit("update:body", body);
      return { kind: "applied" };
    }
    if (outcome.kind === "cancelled") return { kind: "cancelled" };
    return { kind: "failed", message: outcome.message };
  } catch (err: any) {
    const detail = String(err?.message || err || "");
    if (/feature disabled/i.test(detail)) return { kind: "disabled" };
    return { kind: "failed", message: detail };
  } finally {
    aiGenerating.value = false;
    aiStreamActive.value = false;
    aiStreamText.value = "";
    aiReasoning.value = "";
    aiStreamSession = null;
  }
}

async function previewPayload(): Promise<void> {
  if (aiPreviewBusy.value) return;
  if (!props.repoPath) {
    emit("notify", t("ai.notConfigured"));
    return;
  }
  if (!(await isAIConfigured())) {
    openAiSetup();
    return;
  }
  aiPreviewBusy.value = true;
  try {
    const diff = await collectStagedDiff();
    if (!diff.trim()) {
      emit("notify", t("ai.generate.no.staged"));
      return;
    }
    aiPreviewText.value = await previewAIRequest({ kind: "commit", repoPath: props.repoPath, diff });
    showAiPreview.value = true;
  } catch (err: any) {
    emit("notify", t("ai.preview.failed", { message: err?.message || err }));
  } finally {
    aiPreviewBusy.value = false;
  }
}

async function cancelAIGeneration(): Promise<void> {
  const session = aiStreamSession;
  if (!session) return;
  await session.cancel();
}

function notifyGenerateOutcome(outcome: GenerateOutcome): void {
  switch (outcome.kind) {
    case "unconfigured":
      emit("notify", t("ai.notConfigured"));
      break;
    case "disabled":
      emit("notify", t("ai.generate.disabled"));
      break;
    case "no-staged-diff":
      emit("notify", t("ai.generate.no.staged"));
      break;
    case "failed":
      emit(
        "notify",
        outcome.message
          ? `${t("ai.generate.failed")}: ${outcome.message}`
          : t("ai.generate.failed")
      );
      break;
    default:
      break;
  }
}

/** The local fallback also overwrites the draft, so it asks the same question. */
async function runLocalGeneration(): Promise<void> {
  if (props.subject.trim() || props.body.trim()) {
    const ok = await confirmAction({ message: t("ai.generate.confirmOverwrite"), tone: "warning" });
    if (!ok) return;
  }
  emit("generate");
}

/** AI first; local rules only when AI is genuinely unavailable. */
async function generateSmart(): Promise<void> {
  const outcome = await runAiGeneration();
  if (outcome.kind === "unconfigured" || outcome.kind === "disabled") {
    notifyGenerateOutcome(outcome);
    await runLocalGeneration();
    return;
  }
  notifyGenerateOutcome(outcome);
}

/** Tools-menu entry: AI only, with the menu staying open until it settles. */
async function handleMenuAIGenerate(): Promise<void> {
  const outcome = await runAiGeneration();
  notifyGenerateOutcome(outcome);
  showToolsMenu.value = false;
}

// Commit Message Draft Box State (Persisted)
const DRAFT_KEY = "wgit.composer.draft.v1";
const showDraftMenu = ref(false);
const showToolsMenu = ref(false);
const showTypeMenu = ref(false);

function closeAllMenus() {
  showDraftMenu.value = false;
  showToolsMenu.value = false;
  showTypeMenu.value = false;
}
const recentDrafts = ref<string[]>([]);

function loadDrafts() {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (raw) {
      recentDrafts.value = JSON.parse(raw);
    }
  } catch {
    // Benign: unreadable/corrupt draft history just starts empty.
    recentDrafts.value = [];
  }
}

function saveCurrentDraft() {
  const text = props.subject.trim();
  if (!text) return;
  try {
    const drafts = recentDrafts.value.filter((d) => d !== text);
    drafts.unshift(text);
    recentDrafts.value = drafts.slice(0, 10);
    localStorage.setItem(DRAFT_KEY, JSON.stringify(recentDrafts.value));
  } catch {
    // ignore storage issues
  }
}

function applyDraft(d: string) {
  emit("update:subject", d);
  showDraftMenu.value = false;
  showSubjectWarning.value = false;
}

// Empty subject feedback state (shake animation + warning tooltip)
const isShaking = ref(false);
const showSubjectWarning = ref(false);
let shakeTimer: ReturnType<typeof setTimeout> | null = null;
let warningTimer: ReturnType<typeof setTimeout> | null = null;

function triggerEmptySubjectWarning() {
  isShaking.value = true;
  showSubjectWarning.value = true;
  if (shakeTimer) clearTimeout(shakeTimer);
  shakeTimer = setTimeout(() => {
    isShaking.value = false;
  }, 450);

  if (warningTimer) clearTimeout(warningTimer);
  warningTimer = setTimeout(() => {
    showSubjectWarning.value = false;
  }, 4000);

  emit("notify", t("workspace.composer.empty.subject.warning"));
}

function applyInitialPreset(text: string) {
  emit("update:subject", text);
  showSubjectWarning.value = false;
}

// Conventional Commits helper & types
const CONVENTIONAL_TYPES = [
  { type: "feat", label: "feat", descKey: "workspace.composer.conventional.feat", tone: "text-accent bg-accent/15 border-accent/30" },
  { type: "fix", label: "fix", descKey: "workspace.composer.conventional.fix", tone: "text-warning bg-warning/15 border-warning/30" },
  { type: "docs", label: "docs", descKey: "workspace.composer.conventional.docs", tone: "text-purple bg-purple/15 border-purple/30" },
  { type: "style", label: "style", descKey: "workspace.composer.conventional.style", tone: "text-ink-secondary bg-surface border-line" },
  { type: "refactor", label: "refactor", descKey: "workspace.composer.conventional.refactor", tone: "text-purple bg-purple/15 border-purple/30" },
  { type: "perf", label: "perf", descKey: "workspace.composer.conventional.perf", tone: "text-success bg-success/15 border-success/30" },
  { type: "test", label: "test", descKey: "workspace.composer.conventional.test", tone: "text-ink-secondary bg-surface border-line" },
  { type: "chore", label: "chore", descKey: "workspace.composer.conventional.chore", tone: "text-ink-secondary bg-surface border-line" },
  { type: "revert", label: "revert", descKey: "workspace.composer.conventional.revert", tone: "text-danger bg-danger/15 border-danger/30" },
] as const;

function applyConventionalType(type: string) {
  const cur = props.subject.trim();
  const match = cur.match(/^([a-z]+)(\([a-zA-Z0-9_\-\./]+\))?(!)?:\s*(.*)$/);
  if (match) {
    const scope = match[2] || "";
    const bang = match[3] || "";
    const rest = match[4] || "";
    emit("update:subject", `${type}${scope}${bang}: ${rest}`);
  } else if (cur) {
    emit("update:subject", `${type}: ${cur}`);
  } else {
    emit("update:subject", `${type}: `);
  }
  showSubjectWarning.value = false;
  subjectInputRef.value?.focus();
}

const isConventionalCommit = computed(() => {
  return /^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-zA-Z0-9_\-\./]+\))?!?: .+/i.test(props.subject.trim());
});

const activeConventionalType = computed(() => {
  const match = props.subject.trim().match(/^([a-z]+)(\([a-zA-Z0-9_\-\./]+\))?!?:/i);
  return match ? match[1]?.toLowerCase() : null;
});

// Character counter and conventions (50 recommended / 72 limit)
const subjectLength = computed(() => props.subject.length);
const subjectCounterClass = computed(() => {
  if (subjectLength.value > 72) return "text-warning font-semibold";
  if (subjectLength.value > 50) return "text-ink-secondary";
  return "text-ink-muted";
});

// Commit composer lives in a user-resizable pane (240–550px). Below ~420px the
// five action buttons cannot share one row with the title, so switch to a
// wrapping two-row layout instead of letting flexbox squeeze labels into
// vertical text (the "变形" bug).
const rootRef = ref<HTMLElement | null>(null);
const isCompact = ref(false);
let resizeObserver: ResizeObserver | null = null;

watch(
  () => props.subject,
  (val) => {
    if (val.trim()) {
      showSubjectWarning.value = false;
    }
  }
);

watch(
  [() => props.subject, () => props.body],
  ([subject, body]) => {
    if (lintTimer) clearTimeout(lintTimer);
    if (!subject.trim() && !body.trim()) {
      lintResult.value = null;
      commitLintReport.value = null;
      return;
    }
    lintTimer = setTimeout(async () => {
      const full = body.trim() ? `${subject.trim()}\n\n${body.trim()}` : subject.trim();
      try {
        const [lr, cr] = await Promise.all([
          gitService.lintCommitMessage(full),
          gitService.validateCommitMessageFormat(full, {
            maxHeaderLength: 50,
            maxBodyLineLength: 72,
            requireConventional: true,
            disallowTrailingPeriod: true,
          }),
        ]);
        // The Go zero value for a slice reaches the renderer as null; the
        // badge templates call .length/.join directly.
        lintResult.value = lr ? { ...lr, issues: lr.issues ?? [] } : null;
        commitLintReport.value = cr
          ? {
              ...cr,
              errors: cr.errors ?? [],
              warnings: cr.warnings ?? [],
              suggestions: cr.suggestions ?? [],
            }
          : null;
      } catch (err) {
        // Advisory lint on a debounced keystroke: no fitting i18n key, so mark
        // the state (badges cleared) and log instead of toasting on every edit.
        console.error("Failed to lint commit message:", err);
        lintResult.value = null;
        commitLintReport.value = null;
      }
    }, 350);
  },
  { immediate: true }
);

function handleComposerPointerDown(event: PointerEvent) {
  const target = event.target as HTMLElement | null;
  if (target?.closest("[data-composer-menu-root]")) return;
  closeAllMenus();
}

onMounted(() => {
  loadDrafts();
  document.addEventListener("pointerdown", handleComposerPointerDown, true);
  if (typeof ResizeObserver === "undefined" || !rootRef.value) return;
  resizeObserver = new ResizeObserver((entries) => {
    const entry = entries[0];
    if (!entry) return;
    isCompact.value = entry.contentRect.width < 420;
  });
  resizeObserver.observe(rootRef.value);
});

onUnmounted(() => {
  const session = aiStreamSession;
  aiStreamSession = null;
  if (session) void session.cancel().finally(() => session.dispose());
  document.removeEventListener("pointerdown", handleComposerPointerDown, true);
  if (lintTimer) clearTimeout(lintTimer);
  if (shakeTimer) clearTimeout(shakeTimer);
  if (warningTimer) clearTimeout(warningTimer);
  resizeObserver?.disconnect();
  resizeObserver = null;
});

const commitShortcutCustomized = isActionCustomized("commit");

function handleComposerCommitShortcut(event: KeyboardEvent) {
  if (commitShortcutCustomized) {
    event.preventDefault();
    return;
  }
  event.preventDefault();
  if (props.committing) return;
  if (!props.subject.trim()) {
    triggerEmptySubjectWarning();
    return;
  }
  emit("commit", noVerifyModel.value);
}

function handleCommitButtonClick() {
  if (props.committing) return;
  if (!props.subject.trim()) {
    triggerEmptySubjectWarning();
    return;
  }
  saveCurrentDraft();
  emit("commit", noVerifyModel.value);
}

const subjectInputRef = ref<HTMLInputElement | null>(null);

function focusSubject() {
  subjectInputRef.value?.focus();
}

defineExpose({
  focus: focusSubject,
  focusSubject,
});
</script>

<template>
  <div ref="rootRef" class="relative p-2.5 border-t border-line bg-surface/70 shrink-0 flex flex-col gap-2 shadow-lg">

    <div
      class="flex flex-wrap items-center gap-x-2 gap-y-1.5"
      :class="isCompact ? '' : 'justify-between'"
    >
      <span class="text-[11px] font-semibold text-ink flex items-center gap-1 shrink-0 whitespace-nowrap">
        <GitCommit class="w-3.5 h-3.5 text-accent" />
        <span>{{ t("workspace.composer.title") }}</span>
        <span
          v-if="lintResult"
          class="px-1.5 py-0.5 rounded text-[9px] font-bold font-mono shrink-0 whitespace-nowrap"
          :class="lintResult.score >= 90 ? 'bg-success/15 text-success' : lintResult.score >= 70 ? 'bg-warning/15 text-warning' : 'bg-danger/15 text-danger'"
          :title="lintResult.issues.length > 0 ? lintResult.issues.map((issue) => `${issue.severity}: ${issue.message}`).join('\n') : lintResult.summary"
        >
          {{ lintResult.grade }} · {{ lintResult.score
          }}<template v-if="lintResult.issues.length > 0">
            · {{ t("workspace.composer.lint.issues", { count: lintResult.issues.length }) }}</template
          >
        </span>
        <span
          v-if="commitLintReport"
          data-testid="commit-lint-badge"
          class="px-1.5 py-0.5 rounded text-[9px] font-bold font-mono shrink-0 whitespace-nowrap"
          :class="commitLintReport.score >= 80 ? 'bg-success/15 text-success' : commitLintReport.score >= 50 ? 'bg-warning/15 text-warning' : 'bg-danger/15 text-danger'"
          :title="commitLintReport.suggestions.join('\n')"
        >
          {{ commitLintReport.score }}/100
        </span>
        <span
          v-if="gpgHealth"
          data-testid="gpg-health-chip"
          class="px-1.5 py-0.5 rounded text-[9px] font-mono shrink-0 whitespace-nowrap border"
          :class="gpgHealth.status === 'healthy' ? 'bg-success/10 text-success border-success/30' : 'bg-warning/10 text-warning border-warning/30'"
          :title="gpgHealth.message"
        >
          GPG: {{ gpgHealth.status }}
        </span>
      </span>
      <div class="flex flex-wrap items-center gap-1 min-w-0">
        <button
          data-testid="smart-generate-btn"
          @click="generateSmart"
          :disabled="aiGenerating"
          :aria-busy="aiGenerating"
          class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-accent/15 hover:bg-accent/25 text-accent text-[10px] font-medium transition cursor-pointer disabled:opacity-50 disabled:cursor-wait"
          :title="t('workspace.composer.generate.hint')"
        >
          <Sparkles class="w-3 h-3" :class="{ 'animate-spin': aiGenerating }" />
          {{ aiGenerating ? t("ai.generating") : t("workspace.composer.generate") }}
        </button>
        <div data-composer-menu-root>
          <button
            type="button"
            @click="showDraftMenu = !showDraftMenu; showToolsMenu = false; showTypeMenu = false"
            class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-surface hover:bg-raised text-ink-muted hover:text-ink text-[10px] font-medium transition cursor-pointer border border-line"
            :title="t('workspace.composer.drafts.title')"
            data-testid="commit-draft-history-btn"
          >
            <Clock class="w-3 h-3" /> {{ t("workspace.composer.drafts") }}
          </button>
          <div
            v-if="showDraftMenu"
            data-testid="commit-draft-menu"
            class="absolute left-2 right-2 bottom-full mb-1 p-1.5 bg-canvas border border-line rounded-lg shadow-xl z-50 text-xs space-y-1"
          >
            <div class="text-[10px] font-semibold text-ink-muted px-2 py-1 border-b border-line flex items-center justify-between">
              <span>{{ t("workspace.composer.drafts.title") }}</span>
              <button type="button" @click="showDraftMenu = false" class="text-ink-muted hover:text-ink">✕</button>
            </div>
            <div v-if="recentDrafts.length === 0" class="px-2 py-3 text-center text-ink-muted text-[11px]">
              {{ t("workspace.composer.drafts.empty") }}
            </div>
            <div v-else class="max-h-40 overflow-y-auto space-y-1">
              <button
                v-for="(d, idx) in recentDrafts"
                :key="idx"
                type="button"
                @click="applyDraft(d)"
                class="w-full text-left px-2 py-1.5 rounded hover:bg-raised text-[11px] text-ink truncate font-mono transition cursor-pointer"
                :title="d"
              >
                {{ d }}
              </button>
            </div>
          </div>
        </div>
        <div data-composer-menu-root>
          <button
            type="button"
            data-testid="commit-tools-btn"
            @click="showToolsMenu = !showToolsMenu; showDraftMenu = false; showTypeMenu = false"
            class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-surface hover:bg-raised text-ink-muted hover:text-ink text-[10px] font-medium transition cursor-pointer border border-line"
            :title="t('workspace.composer.tools')"
          >
            <MoreHorizontal class="w-3 h-3" /> {{ t("workspace.composer.tools") }}
          </button>
          <div
            v-if="showToolsMenu"
            data-testid="commit-tools-menu"
            class="absolute left-2 right-2 bottom-full mb-1 z-50 p-1.5 bg-canvas border border-line rounded-lg shadow-xl text-[11px]"
          >
            <button
              type="button"
              @click="emit('load-last'); showToolsMenu = false"
              class="w-full text-left px-2 py-1.5 rounded hover:bg-raised text-ink-muted hover:text-ink flex items-center gap-2 transition cursor-pointer"
            >
              <History class="w-3 h-3" /> {{ t("workspace.composer.last") }}
            </button>
            <button
              type="button"
              @click="emit('load-template'); showToolsMenu = false"
              class="w-full text-left px-2 py-1.5 rounded hover:bg-raised text-ink-muted hover:text-ink flex items-center gap-2 transition cursor-pointer"
            >
              <FileText class="w-3 h-3" /> {{ t("workspace.composer.template") }}
            </button>
            <button
              type="button"
              @click="emit('save-template'); showToolsMenu = false"
              class="w-full text-left px-2 py-1.5 rounded hover:bg-raised text-ink-muted hover:text-ink flex items-center gap-2 transition cursor-pointer"
            >
              <Save class="w-3 h-3" /> {{ t("workspace.composer.save.template") }}
            </button>
            <div class="my-1 h-px bg-line" />
            <button
              type="button"
              :disabled="committing || !subject.trim()"
              @click="emit('fast-commit'); showToolsMenu = false"
              class="w-full text-left px-2 py-1.5 rounded hover:bg-warning/15 text-warning flex items-center gap-2 transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Zap class="w-3 h-3" /> {{ t("workspace.composer.quick") }}
            </button>
            <button
              type="button"
              @click="emit('audit-sentinel'); showToolsMenu = false"
              class="w-full text-left px-2 py-1.5 rounded hover:bg-purple/15 text-purple flex items-center gap-2 transition cursor-pointer"
            >
              <ShieldCheck class="w-3 h-3" /> {{ t("workspace.composer.audit.sentinel") }}
            </button>
            <FeatureGate flag="aiCommitMsg">
              <button
                type="button"
                data-testid="ai-preview-payload-btn"
                @click="previewPayload"
                :disabled="aiPreviewBusy"
                class="w-full text-left px-2 py-1.5 rounded hover:bg-raised text-ink-muted hover:text-ink flex items-center gap-2 transition cursor-pointer disabled:opacity-40"
              >
                <Eye class="w-3 h-3" />
                {{ t("ai.preview.btn") }}
              </button>
              <button
                type="button"
                data-testid="ai-generate-commit-msg-btn"
                @click="handleMenuAIGenerate"
                :disabled="aiGenerating"
                class="w-full text-left px-2 py-1.5 rounded hover:bg-accent/15 text-accent flex items-center gap-2 transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <Sparkles class="w-3 h-3" :class="{ 'animate-spin': aiGenerating }" />
                {{ aiGenerating ? t("ai.generating") : t("ai.generate") }}
              </button>
            </FeatureGate>
          </div>
        </div>
      </div>
    </div>

    <!-- Fresh-repo preset: a hint line, not a colored panel. -->
    <div
      v-if="isNewRepo && !subject.trim()"
      class="flex flex-wrap items-center gap-1 px-0.5 text-[10px] text-ink-muted animate-fade-in"
    >
      <Sparkles class="w-3 h-3 text-accent shrink-0" />
      <span class="shrink-0">{{ t("ws.empty.new.repo.title") }}:</span>
      <button
        type="button"
        @click="applyInitialPreset('Initial commit')"
        class="inline-flex items-center px-1.5 py-0.5 rounded bg-surface hover:bg-raised text-ink hover:text-accent font-medium border border-line transition cursor-pointer"
        :title="t('workspace.composer.initial.commit.preset')"
      >
        {{ t("workspace.composer.initial.commit.preset") }}
      </button>
      <button
        type="button"
        @click="applyInitialPreset('feat: initial commit')"
        class="inline-flex items-center px-1.5 py-0.5 rounded bg-surface hover:bg-raised text-ink-muted hover:text-accent font-mono border border-line transition cursor-pointer"
      >
        feat: initial commit
      </button>
    </div>

    <!--
      Conventional type lives in one trigger + menu. The old pill wall (nine
      colored buttons plus a badge) spent two rows on a pane this narrow.
    -->
    <div class="flex flex-wrap items-center gap-1.5 py-0.5 select-none" data-testid="conventional-pills-bar">
      <div data-composer-menu-root>
        <button
          type="button"
          data-testid="commit-type-trigger"
          @click="showTypeMenu = !showTypeMenu; showDraftMenu = false; showToolsMenu = false"
          class="flex items-center gap-1 px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[10px] font-medium text-ink-muted hover:text-ink transition cursor-pointer"
        >
          <span>{{ t("workspace.composer.conventional.title") }}:</span>
          <span class="font-mono text-ink">{{ activeConventionalType || "—" }}</span>
          <ShieldCheck v-if="isConventionalCommit" class="w-3 h-3 text-success" />
          <ChevronDown class="w-3 h-3" />
        </button>
        <div
          v-if="showTypeMenu"
          data-testid="commit-type-menu"
          class="absolute left-2 right-2 bottom-full mb-1 z-50 p-2 bg-canvas border border-line rounded-lg shadow-xl"
        >
          <div class="grid grid-cols-3 gap-1">
            <button
              v-for="item in CONVENTIONAL_TYPES"
              :key="item.type"
              type="button"
              @click="applyConventionalType(item.type); showTypeMenu = false"
              class="px-1.5 py-1 rounded text-[10px] font-mono border transition cursor-pointer text-center"
              :class="activeConventionalType === item.type ? 'bg-accent-strong text-accent-contrast border-accent-strong font-bold' : 'bg-surface border-line text-ink-muted hover:text-ink hover:bg-raised'"
              :title="t(item.descKey)"
            >
              {{ item.label }}
            </button>
          </div>
        </div>
      </div>

      <button
        type="button"
        data-testid="commit-auto-format-btn"
        :disabled="!subject.trim()"
        @click="handleAutoFormat"
        class="px-2 py-0.5 rounded text-[10px] bg-accent/15 hover:bg-accent/25 text-accent border border-accent/30 font-medium transition cursor-pointer flex items-center gap-1 shrink-0 disabled:opacity-40"
        :title="t('workspace.composer.auto.format.hint')"
      >
        <Sparkles class="w-3 h-3" />
        <span>{{ t("workspace.composer.auto.format") }}</span>
      </button>
    </div>

    <!-- Subject input with character counter and shake feedback -->
    <div class="relative">
      <input
        ref="subjectInputRef"
        v-model="subjectModel"
        type="text"
        :aria-label="t('workspace.composer.subject.aria')"
        :placeholder="t('workspace.composer.subject.placeholder')"
        @keydown.meta.enter="handleComposerCommitShortcut"
        @keydown.ctrl.enter="handleComposerCommitShortcut"
        class="w-full pl-2.5 pr-14 py-1.5 rounded-lg bg-canvas border text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans transition"
        :class="[
          isShaking ? 'border-danger animate-composer-shake ring-1 ring-danger' : showSubjectWarning ? 'border-danger/70' : 'border-line',
        ]"
      />
      <div
        v-if="subjectLength > 0"
        class="absolute right-2 top-1/2 -translate-y-1/2 text-[10px] font-mono pointer-events-none select-none transition-colors"
        :class="subjectCounterClass"
        :title="subjectLength > 72 ? 'Subject exceeds recommended limit of 72 characters' : 'Standard git convention: 50/72'"
      >
        {{ subjectLength }}/72
      </div>
    </div>

    <div
      v-if="aiStreamActive"
      data-testid="ai-stream-preview"
      class="flex items-start gap-2 px-2 py-1.5 rounded-lg border border-accent/30 bg-accent/5"
    >
      <Sparkles class="w-3.5 h-3.5 shrink-0 mt-0.5 text-accent animate-spin" />
      <p class="flex-1 min-w-0 max-h-16 overflow-y-auto text-[11px] leading-relaxed text-ink whitespace-pre-wrap select-text">
        {{ aiStreamText || t("ai.stream.waiting") }}
      </p>
      <button
        type="button"
        data-testid="ai-stream-cancel"
        class="shrink-0 px-2 py-0.5 rounded-md border border-line bg-surface text-[10px] font-semibold text-ink-muted hover:text-danger hover:bg-raised transition cursor-pointer"
        @click="cancelAIGeneration"
      >
        {{ t("ai.stream.cancel") }}
      </button>
    </div>
    <div v-if="aiStreamActive" class="px-2">
      <AiReasoningPanel :text="aiReasoning" />
    </div>

    <!-- Empty subject inline warning feedback -->
    <div
      v-if="showSubjectWarning"
      class="text-[11px] text-danger flex items-center gap-1.5 px-2 py-1 rounded-md bg-danger/10 border border-danger/25 animate-fade-in"
    >
      <AlertCircle class="w-3.5 h-3.5 shrink-0" />
      <span>{{ t("workspace.composer.empty.subject.warning") }}</span>
    </div>

    <textarea
      v-model="bodyModel"
      rows="2"
      :aria-label="t('workspace.composer.body.aria')"
      :placeholder="t('workspace.composer.body.placeholder')"
      @keydown.meta.enter="handleComposerCommitShortcut"
      @keydown.ctrl.enter="handleComposerCommitShortcut"
      class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans transition resize-y"
    ></textarea>

    <div class="flex flex-wrap items-center justify-between gap-1.5 text-[10px] text-ink-muted">
      <div class="flex items-center gap-2 shrink-0">
        <label class="flex items-center gap-1 cursor-pointer hover:text-ink whitespace-nowrap">
          <input type="checkbox" v-model="amendModel" class="rounded accent-accent scale-90" />
          <span>Amend</span>
        </label>
        <label class="flex items-center gap-1 cursor-pointer hover:text-ink whitespace-nowrap">
          <input type="checkbox" v-model="autoPushModel" class="rounded accent-accent scale-90" />
          <span>Push</span>
        </label>
        <label
          class="flex items-center gap-1 cursor-pointer hover:text-ink whitespace-nowrap"
          :title="t('workspace.composer.noVerify.hint')"
        >
          <input
            type="checkbox"
            v-model="noVerifyModel"
            class="rounded accent-accent scale-90"
            data-testid="no-verify-checkbox"
          />
          <span class="font-mono text-[9px]">--no-verify</span>
        </label>
      </div>
      <button
        @click="handleCommitButtonClick"
        :disabled="committing"
        class="flex items-center gap-1 shrink-0 whitespace-nowrap px-3 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast font-semibold text-xs transition cursor-pointer disabled:opacity-40 shadow-xs"
        :class="{ 'opacity-65': !subject.trim() }"
      >
        <Send class="w-3 h-3" />
        <span>{{ committing ? t("workspace.composer.committing") : t("workspace.composer.commit") }}</span>
        <kbd class="ml-0.5 px-1 py-0.2 rounded ring-1 ring-inset ring-accent-contrast/40 text-[8px] font-mono">⌘↵</kbd>
      </button>
    </div>
    <AiPayloadPreviewModal
      :is-open="showAiPreview"
      :text="aiPreviewText"
      @close="showAiPreview = false"
    />
  </div>
</template>

<style scoped>
@keyframes composer-shake {
  0%, 100% {
    transform: translateX(0);
  }
  20%, 60% {
    transform: translateX(-4px);
  }
  40%, 80% {
    transform: translateX(4px);
  }
}

.animate-composer-shake {
  animation: composer-shake 0.35s ease-in-out;
}
</style>
