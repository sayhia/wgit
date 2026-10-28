<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from "vue";
import { isActionCustomized } from "../../utils/shortcuts";
import { GitCommit, Sparkles, Send, FileText, Save, History, Zap, AlertCircle, ShieldCheck, Clock } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { CommitLintResult } from "../../types";
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
let lintTimer: ReturnType<typeof setTimeout> | null = null;

// Commit Message Draft Box State (Persisted)
const DRAFT_KEY = "wgit.composer.draft.v1";
const showDraftMenu = ref(false);
const recentDrafts = ref<string[]>([]);

function loadDrafts() {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (raw) {
      recentDrafts.value = JSON.parse(raw);
    }
  } catch {
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
      return;
    }
    lintTimer = setTimeout(async () => {
      try {
        lintResult.value = await gitService.lintCommitMessage(
          body.trim() ? `${subject.trim()}\n\n${body.trim()}` : subject.trim()
        );
      } catch {
        lintResult.value = null;
      }
    }, 350);
  },
  { immediate: true }
);

onMounted(() => {
  loadDrafts();
  if (typeof ResizeObserver === "undefined" || !rootRef.value) return;
  resizeObserver = new ResizeObserver((entries) => {
    const entry = entries[0];
    if (!entry) return;
    isCompact.value = entry.contentRect.width < 420;
  });
  resizeObserver.observe(rootRef.value);
});

onUnmounted(() => {
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
  <div ref="rootRef" class="p-2.5 border-t border-line bg-surface/70 shrink-0 flex flex-col gap-2 shadow-lg">
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
      </span>
      <div class="flex flex-wrap items-center gap-1 min-w-0">
        <button
          @click="emit('load-last')"
          class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-surface hover:bg-raised text-ink-muted hover:text-ink text-[10px] font-medium transition cursor-pointer border border-line"
          :title="t('workspace.composer.last.hint')"
        >
          <History class="w-3 h-3" /> {{ t("workspace.composer.last") }}
        </button>
        <div class="relative">
          <button
            type="button"
            @click="showDraftMenu = !showDraftMenu"
            class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-surface hover:bg-raised text-ink-muted hover:text-ink text-[10px] font-medium transition cursor-pointer border border-line"
            :title="t('workspace.composer.drafts.title')"
            data-testid="commit-draft-history-btn"
          >
            <Clock class="w-3 h-3" /> {{ t("workspace.composer.drafts") }}
          </button>
          <div
            v-if="showDraftMenu"
            class="absolute left-0 top-full mt-1 w-64 p-1.5 bg-canvas border border-line rounded-lg shadow-xl z-50 text-xs space-y-1"
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
        <button
          @click="emit('load-template')"
          class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-surface hover:bg-raised text-ink-muted hover:text-ink text-[10px] font-medium transition cursor-pointer border border-line"
          :title="t('workspace.composer.template.hint')"
        >
          <FileText class="w-3 h-3" /> {{ t("workspace.composer.template") }}
        </button>
        <button
          @click="emit('save-template')"
          class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-surface hover:bg-raised text-ink-muted hover:text-ink text-[10px] font-medium transition cursor-pointer border border-line"
          :title="t('workspace.composer.save.template.hint')"
        >
          <Save class="w-3 h-3" /> {{ t("workspace.composer.save.template") }}
        </button>
        <button
          @click="emit('fast-commit')"
          :disabled="committing || !subject.trim()"
          class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-warning/15 hover:bg-warning/25 text-warning text-[10px] font-medium transition cursor-pointer disabled:opacity-40"
          :title="t('workspace.composer.quick.hint')"
        >
          <Zap class="w-3 h-3" /> {{ t("workspace.composer.quick") }}
        </button>
        <button
          @click="emit('generate')"
          class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-accent/15 hover:bg-accent/25 text-accent text-[10px] font-medium transition cursor-pointer"
          :title="t('workspace.composer.generate.hint')"
        >
          <Sparkles class="w-3 h-3" /> {{ t("workspace.composer.generate") }}
        </button>
        <button
          @click="emit('audit-sentinel')"
          class="flex items-center gap-1 shrink-0 whitespace-nowrap px-2 py-0.5 rounded bg-purple/15 hover:bg-purple/25 text-purple text-[10px] font-medium transition cursor-pointer"
          :title="t('workspace.composer.audit.sentinel.hint')"
        >
          <ShieldCheck class="w-3 h-3" /> {{ t("workspace.composer.audit.sentinel") }}
        </button>
      </div>
    </div>

    <!-- Initial Commit Quick Preset (Fresh Repo) -->
    <div
      v-if="isNewRepo && !subject.trim()"
      class="flex items-center flex-wrap gap-1.5 px-2.5 py-1.5 rounded-lg bg-accent/10 border border-accent/25 text-[11px] text-accent animate-fade-in"
    >
      <Sparkles class="w-3.5 h-3.5 text-accent shrink-0" />
      <span class="text-ink-muted text-[10px] shrink-0">{{ t("ws.empty.new.repo.title") }}:</span>
      <button
        type="button"
        @click="applyInitialPreset('Initial commit')"
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-surface hover:bg-raised text-ink hover:text-accent font-medium text-[11px] border border-line transition cursor-pointer shadow-xs"
        :title="t('workspace.composer.initial.commit.preset')"
      >
        <span>{{ t("workspace.composer.initial.commit.preset") }}</span>
      </button>
      <button
        type="button"
        @click="applyInitialPreset('feat: initial commit')"
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-surface hover:bg-raised text-ink-muted hover:text-accent font-mono text-[10px] border border-line transition cursor-pointer shadow-xs"
      >
        feat: initial commit
      </button>
    </div>

    <!-- Conventional Commits Quick Type Selector Pills -->
    <div class="flex items-center gap-1 overflow-x-auto py-0.5 no-scrollbar select-none" data-testid="conventional-pills-bar">
      <span class="text-[10px] text-ink-muted shrink-0 mr-0.5">{{ t("workspace.composer.conventional.title") }}:</span>
      <button
        v-for="item in CONVENTIONAL_TYPES"
        :key="item.type"
        type="button"
        @click="applyConventionalType(item.type)"
        class="px-1.5 py-0.5 rounded text-[10px] font-mono font-medium transition cursor-pointer shrink-0 border"
        :class="activeConventionalType === item.type ? 'bg-accent text-accent-contrast border-accent font-bold shadow-xs' : item.tone + ' hover:opacity-85'"
        :title="t(item.descKey)"
      >
        {{ item.label }}
      </button>

      <span
        v-if="isConventionalCommit"
        class="ml-auto inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[9px] font-medium bg-success/15 text-success shrink-0"
        :title="t('workspace.composer.conventional.valid')"
      >
        <ShieldCheck class="w-3 h-3 text-success" />
        <span class="hidden sm:inline">{{ t("workspace.composer.conventional.badge") }}</span>
      </span>
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
