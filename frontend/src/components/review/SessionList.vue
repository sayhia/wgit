<script setup lang="ts">
import { computed } from "vue";
import { GitPullRequest, ArrowRight, Clock, AlertCircle, Loader2 } from "lucide-vue-next";
import type { ReviewSessionSummary } from "../../types";
import { verdictChipOf, verdictTextOf } from "./meta";
import { t } from "../../i18n";

const props = defineProps<{
  base: string;
  head: string;
  sessions: ReviewSessionSummary[];
  sessionsError: string;
  isLoadingSessions: boolean;
  isLoadingReview: boolean;
  currentRepoPath: string;
  currentReviewKey: string;
}>();

const emit = defineEmits<{
  (e: "update:base", value: string): void;
  (e: "update:head", value: string): void;
  (e: "start"): void;
  (e: "open", session: ReviewSessionSummary): void;
}>();

const baseModel = computed({
  get: () => props.base,
  set: (value: string) => emit("update:base", value),
});
const headModel = computed({
  get: () => props.head,
  set: (value: string) => emit("update:head", value),
});
</script>

<template>
      <!-- Column 1: review sessions -->
      <aside class="w-48 xl:w-60 shrink-0 border-r border-line flex flex-col bg-surface min-h-0">
        <div class="p-2.5 border-b border-line space-y-1.5 shrink-0">
          <div class="flex items-center gap-1.5 text-[11px] font-semibold text-ink">
            <GitPullRequest class="w-3.5 h-3.5 text-accent" />
            {{ t("review.session.range") }}
          </div>
          <label class="block">
            <span class="text-[10px] text-ink-muted">{{ t("review.session.base") }}</span>
            <input
              v-model="baseModel"
              :disabled="!currentRepoPath"
              :placeholder="t('review.session.base.placeholder')"
              spellcheck="false"
              class="mt-0.5 w-full bg-canvas border border-line rounded-lg px-2 py-1 text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent-border disabled:opacity-50"
              @keyup.enter="emit('start')"
            />
          </label>
          <label class="block">
            <span class="text-[10px] text-ink-muted">{{ t("review.session.head") }}</span>
            <input
              v-model="headModel"
              :disabled="!currentRepoPath"
              placeholder="HEAD"
              spellcheck="false"
              class="mt-0.5 w-full bg-canvas border border-line rounded-lg px-2 py-1 text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent-border disabled:opacity-50"
              @keyup.enter="emit('start')"
            />
          </label>
          <button
            @click="emit('start')"
            :disabled="isLoadingReview || !currentRepoPath"
            class="w-full flex items-center justify-center gap-1.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-[11px] font-semibold transition cursor-pointer shadow-sm disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Loader2 v-if="isLoadingReview" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            <GitPullRequest v-else class="w-3.5 h-3.5" aria-hidden="true" />
            {{ t("review.session.start") }}
          </button>
          <p class="text-[9px] leading-4 text-ink-muted">{{ t("review.session.auto.detect") }}</p>
        </div>

        <div class="h-8 shrink-0 px-3 border-b border-line flex items-center gap-1.5">
          <Clock class="w-3.5 h-3.5 text-ink-muted" />
          <span class="text-[11px] font-semibold text-ink">{{ t("review.session.history.title") }}</span>
          <span class="ml-auto text-[10px] font-mono text-ink-muted">{{ sessions.length }}</span>
        </div>

        <div class="flex-1 overflow-y-auto p-1.5 space-y-1 min-h-0">
          <div
            v-if="sessionsError"
            class="mx-1 mt-1 px-2 py-1.5 rounded-lg bg-danger-bg border border-danger-border text-[10px] text-danger flex items-start gap-1.5"
          >
            <AlertCircle class="w-3 h-3 shrink-0 mt-0.5" />
            <span class="break-words">{{ sessionsError }}</span>
          </div>
          <div v-else-if="isLoadingSessions && sessions.length === 0" class="py-6 text-center text-[11px] text-ink-muted">
            {{ t("review.session.loading") }}
          </div>
          <div v-else-if="sessions.length === 0" class="py-6 px-2 text-center text-[11px] text-ink-muted">
            {{ t("review.session.empty") }}
            <div class="mt-1 text-[10px]">{{ t("review.session.empty.hint") }}</div>
          </div>
          <button
            v-for="session in sessions"
            :key="session.key"
            @click="emit('open', session)"
            class="w-full text-left px-2 py-1.5 rounded-lg border transition cursor-pointer min-w-0"
            :class="
              currentReviewKey === session.key
                ? 'bg-raised border-accent/40 shadow-xs'
                : 'bg-surface border-line hover:bg-raised'
            "
          >
            <!--
              The verdict chip used to sit on the range row inside a ~190px
              card, shrinking the base/head labels to 「orig… → H…」. The chip
              now shares the metadata row, which wraps as a unit: the session
              identity keeps the full card width.
            -->
            <div class="flex items-center gap-1 min-w-0">
              <span class="text-[10px] font-mono text-ink truncate">{{ session.base }}</span>
              <ArrowRight class="w-2.5 h-2.5 text-ink-muted shrink-0" aria-hidden="true" />
              <span class="text-[10px] font-mono text-ink truncate">{{ session.head }}</span>
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-[9px] font-mono text-ink-muted">
              <span
                class="shrink-0 px-1 py-0.5 rounded border text-[9px]"
                :class="verdictChipOf(session.verdict)"
                :title="session.verdictDetail"
              >
                {{ verdictTextOf(session.verdict) }}
              </span>
              <span class="truncate">{{ session.updatedAt || "—" }}</span>
              <span class="text-success shrink-0">+{{ session.additions }}</span>
              <span class="text-danger shrink-0">-{{ session.deletions }}</span>
              <span class="shrink-0">{{ session.approved }}/{{ session.total }}</span>
              <span v-if="session.openComments > 0" class="text-warning shrink-0">
                {{ t("review.session.open.comments", { count: session.openComments }) }}
              </span>
            </div>
          </button>
        </div>
      </aside>

</template>
