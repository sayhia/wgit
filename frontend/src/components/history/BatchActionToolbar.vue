<script setup lang="ts">
import { GitCompare, GitBranch, X, Copy, Download } from "lucide-vue-next";
import { t } from "../../i18n";

defineProps<{ selectionCount: number }>();

const emit = defineEmits<{
  (e: "compare-two"): void;
  (e: "export-patch"): void;
  (e: "batch-cherry-pick"): void;
  (e: "copy-details"): void;
  (e: "rebase"): void;
  (e: "clear"): void;
}>();
</script>

<template>
      <!-- Multi-select batch actions toolbar -->
      <div
        v-if="selectionCount > 0"
        class="border-b border-line/60 px-4 py-1.5 flex items-center gap-2 bg-accent/5 shrink-0 flex-wrap"
      >
        <span class="text-[11px] font-semibold text-accent whitespace-nowrap">
          {{ t("history.batch.selected", { count: selectionCount }) }}
        </span>
        <span class="text-[10px] text-ink-muted font-mono whitespace-nowrap">
          {{ t("history.batch.hint") }}
        </span>
        <div class="flex items-center gap-1.5 ml-auto">
          <button
            v-if="selectionCount >= 2"
            @click="emit('compare-two')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
            :title="selectionCount === 2 ? t('history.batch.compare.two.hint') : t('history.batch.compare.range.hint')"
          >
            <GitCompare class="w-3.5 h-3.5 text-cyan" />
            <span>{{ selectionCount === 2 ? t("history.batch.compare.two") : t("history.batch.compare.range") }}</span>
          </button>
          <button
            v-if="selectionCount >= 2"
            @click="emit('export-patch')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.batch.export.patch.hint')"
          >
            <Download class="w-3.5 h-3.5 text-success" />
            <span>{{ t("history.batch.export.patch") }}</span>
          </button>
          <button
            @click="emit('batch-cherry-pick')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.batch.cherry.pick.hint')"
          >
            <GitCompare class="w-3.5 h-3.5 text-accent" />
            <span>{{ t("history.batch.cherry.pick") }}</span>
          </button>
          <button
            @click="emit('copy-details')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.batch.copy.hint')"
          >
            <Copy class="w-3.5 h-3.5 text-purple" />
            <span>{{ t("history.batch.copy") }}</span>
          </button>
          <button
            @click="emit('rebase')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-canvas hover:bg-raised text-[11px] font-medium text-ink transition cursor-pointer border border-line/60"
            :title="t('history.batch.rebase.hint')"
          >
            <GitBranch class="w-3.5 h-3.5 text-cyan" />
            <span>{{ t("history.batch.rebase") }}</span>
          </button>
          <button
            @click="emit('clear')"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-raised hover:bg-hover-strong text-[11px] text-ink-secondary hover:text-ink transition cursor-pointer border border-line"
            :title="t('history.batch.clear.hint')"
          >
            <X class="w-3 h-3" />
            <span>{{ t("history.batch.clear") }}</span>
          </button>
        </div>
      </div>

</template>
