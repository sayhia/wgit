<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from "vue";
import { isActionCustomized } from "../../utils/shortcuts";
import { GitCommit, Sparkles, Send, FileText, Save, History, Zap } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { CommitLintResult } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  subject: string;
  body: string;
  amend: boolean;
  autoPush: boolean;
  committing: boolean;
}>();

const emit = defineEmits<{
  (e: "update:subject", value: string): void;
  (e: "update:body", value: string): void;
  (e: "update:amend", value: boolean): void;
  (e: "update:autoPush", value: boolean): void;
  (e: "commit"): void;
  (e: "fast-commit"): void;
  (e: "generate"): void;
  (e: "load-template"): void;
  (e: "load-last"): void;
  (e: "save-template"): void;
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

const lintResult = ref<CommitLintResult | null>(null);
let lintTimer: ReturnType<typeof setTimeout> | null = null;

// Commit composer lives in a user-resizable pane (240–550px). Below ~420px the
// five action buttons cannot share one row with the title, so switch to a
// wrapping two-row layout instead of letting flexbox squeeze labels into
// vertical text (the "变形" bug).
const rootRef = ref<HTMLElement | null>(null);
const isCompact = ref(false);
let resizeObserver: ResizeObserver | null = null;

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
  resizeObserver?.disconnect();
  resizeObserver = null;
});

const commitShortcutCustomized = isActionCustomized("commit");

function handleComposerCommitShortcut(event: KeyboardEvent) {
  if (commitShortcutCustomized) {
    event.preventDefault();
    return;
  }
  emit("commit");
}
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
      </div>
    </div>

    <input
      v-model="subjectModel"
      type="text"
      :aria-label="t('workspace.composer.subject.aria')"
      :placeholder="t('workspace.composer.subject.placeholder')"
      @keydown.meta.enter="handleComposerCommitShortcut"
      @keydown.ctrl.enter="handleComposerCommitShortcut"
      class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans transition"
    />

    <textarea
      v-model="bodyModel"
      rows="2"
      :aria-label="t('workspace.composer.body.aria')"
      :placeholder="t('workspace.composer.body.placeholder')"
      class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans transition resize-y"
    ></textarea>

    <div class="flex flex-wrap items-center justify-between gap-1.5 text-[10px] text-ink-muted">
      <div class="flex items-center gap-2 shrink-0">
        <label class="flex items-center gap-1 cursor-pointer hover:text-ink whitespace-nowrap">
          <input type="checkbox" v-model="amendModel" class="rounded accent-accent scale-90" />
          <span>Amend</span>
        </label>
        <label class="flex items-center gap-1 cursor-pointer hover:text-ink">
          <input type="checkbox" v-model="autoPushModel" class="rounded accent-accent scale-90" />
          <span>Push</span>
        </label>
      </div>
      <button
        @click="emit('commit')"
        :disabled="committing || !subject.trim()"
        class="flex items-center gap-1 shrink-0 whitespace-nowrap px-3 py-1 rounded-md bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast font-semibold text-xs transition cursor-pointer disabled:opacity-40 shadow-xs"
      >
        <Send class="w-3 h-3" />
        <span>{{ committing ? t("workspace.composer.committing") : t("workspace.composer.commit") }}</span>
        <kbd class="ml-0.5 px-1 py-0.2 rounded ring-1 ring-inset ring-accent-contrast/40 text-[8px] font-mono">⌘↵</kbd>
      </button>
    </div>
  </div>
</template>
