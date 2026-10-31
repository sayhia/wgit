<script setup lang="ts">
import { GitCompare, GitBranch, X, Copy, Download, Layers } from "lucide-vue-next";
import { t } from "../../i18n";

defineProps<{ selectionCount: number }>();

const emit = defineEmits<{
  (e: "compare-two"): void;
  (e: "export-patch"): void;
  (e: "batch-cherry-pick"): void;
  (e: "copy-details"): void;
  (e: "squash"): void;
  (e: "rebase"): void;
  (e: "clear"): void;
}>();
</script>

<template>
  <div
    v-if="selectionCount > 0"
    data-testid="history-batch-actions"
    class="border-b border-line/60 px-4 py-2 flex flex-col gap-2 bg-selected shrink-0"
  >
    <div class="flex flex-wrap items-center gap-2 min-w-0">
      <span class="text-[11px] font-semibold text-ink whitespace-nowrap">
        {{ t("history.batch.selected", { count: selectionCount }) }}
      </span>
      <button
        type="button"
        @click="emit('clear')"
        class="ml-auto flex shrink-0 items-center gap-1 whitespace-nowrap px-2.5 py-1 rounded-md bg-raised hover:bg-hover-strong text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
        :title="t('history.batch.clear.hint')"
      >
        <X class="w-3 h-3" aria-hidden="true" />
        <span>{{ t("history.batch.clear") }}</span>
      </button>
    </div>
    <div class="flex flex-wrap items-center gap-1.5 min-w-0 max-w-full">
      <button
        type="button"
        v-if="selectionCount >= 2"
        @click="emit('compare-two')"
        class="flex shrink-0 min-h-7 items-center gap-1 whitespace-nowrap px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
        :title="selectionCount === 2 ? t('history.batch.compare.two.hint') : t('history.batch.compare.range.hint')"
      >
        <GitCompare class="w-3.5 h-3.5 text-cyan" aria-hidden="true" />
        <span>{{ selectionCount === 2 ? t("history.batch.compare.two") : t("history.batch.compare.range") }}</span>
      </button>
      <button
        type="button"
        v-if="selectionCount >= 2"
        @click="emit('export-patch')"
        class="flex shrink-0 min-h-7 items-center gap-1 whitespace-nowrap px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
        :title="t('history.batch.export.patch.hint')"
      >
        <Download class="w-3.5 h-3.5 text-success" aria-hidden="true" />
        <span>{{ t("history.batch.export.patch") }}</span>
      </button>
      <button
        type="button"
        @click="emit('batch-cherry-pick')"
        class="flex shrink-0 min-h-7 items-center gap-1 whitespace-nowrap px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
        :title="t('history.batch.cherry.pick.hint')"
      >
        <GitCompare class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
        <span>{{ t("history.batch.cherry.pick") }}</span>
      </button>
      <button
        type="button"
        v-if="selectionCount >= 2"
        @click="emit('squash')"
        class="flex shrink-0 min-h-7 items-center gap-1 whitespace-nowrap px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
        :title="t('history.batch.squash.hint')"
      >
        <Layers class="w-3.5 h-3.5 text-warning" aria-hidden="true" />
        <span>{{ t("history.batch.squash") }}</span>
      </button>
      <button
        type="button"
        @click="emit('copy-details')"
        class="flex shrink-0 min-h-7 items-center gap-1 whitespace-nowrap px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
        :title="t('history.batch.copy.hint')"
      >
        <Copy class="w-3.5 h-3.5 text-purple" aria-hidden="true" />
        <span>{{ t("history.batch.copy") }}</span>
      </button>
      <button
        type="button"
        @click="emit('rebase')"
        class="flex shrink-0 min-h-7 items-center gap-1 whitespace-nowrap px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
        :title="t('history.batch.rebase.hint')"
      >
        <GitBranch class="w-3.5 h-3.5 text-cyan" aria-hidden="true" />
        <span>{{ t("history.batch.rebase") }}</span>
      </button>
    </div>
  </div>
</template>
