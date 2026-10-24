<script setup lang="ts">
import { computed } from "vue";
import { AlertTriangle, CheckCircle2, RotateCcw } from "lucide-vue-next";
import { t } from "../../i18n";

const props = defineProps<{
  isMerging: boolean;
  isRebasing: boolean;
  isCherryPicking: boolean;
  conflictedFiles: string[];
  mergeHeadBranch: string;
  rebaseHeadName: string;
  rebaseStep: number;
  rebaseTotal: number;
}>();

const emit = defineEmits<{
  (e: "continue"): void;
  (e: "skip"): void;
  (e: "abort"): void;
}>();

/** Headline is a computed (not a module-level string) so it follows the locale. */
const headline = computed(() => {
  if (props.isMerging) {
    return t("workspace.conflict.merging", {
      branch: props.mergeHeadBranch || t("workspace.conflict.incoming"),
    });
  }
  if (props.isRebasing) {
    return t("workspace.conflict.rebasing", {
      branch: props.rebaseHeadName || t("workspace.conflict.target"),
      step: props.rebaseStep || 0,
      total: props.rebaseTotal || 0,
    });
  }
  if (props.isCherryPicking) return t("workspace.conflict.cherry.picking");
  return t("workspace.conflict.detected");
});
</script>

<template>
  <div
    class="px-4 py-2.5 bg-danger/10 border-b border-danger/30 text-xs flex flex-wrap items-center justify-between gap-2 shrink-0 select-none"
  >
    <div class="flex items-center gap-2 text-danger font-medium">
      <AlertTriangle class="w-4 h-4 shrink-0 animate-pulse" />
      <span>
        <strong class="font-bold">
          {{ headline }}
        </strong>:
        <span v-if="conflictedFiles.length > 0">
          {{ t("workspace.conflict.count", { count: conflictedFiles.length }) }}
        </span>
        <span v-else class="text-success font-semibold">
          {{ t("workspace.conflict.resolved") }}
        </span>
      </span>
    </div>

    <div class="flex items-center gap-2">
      <button
        @click="emit('continue')"
        :disabled="conflictedFiles.length > 0"
        class="flex items-center gap-1 px-3 py-1 rounded-lg text-xs font-bold transition cursor-pointer shadow-xs disabled:opacity-40 disabled:cursor-not-allowed"
        :class="conflictedFiles.length === 0 ? 'bg-success-strong text-white hover:bg-success-strong-hover' : 'bg-surface border border-line text-ink-muted'"
        :title="t('workspace.conflict.continue.hint')"
      >
        <CheckCircle2 class="w-3.5 h-3.5" />
        <span>{{ isMerging ? t("workspace.conflict.finish.merge") : t("workspace.conflict.continue") }}</span>
      </button>

      <button
        v-if="isRebasing || isCherryPicking"
        @click="emit('skip')"
        class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
        :title="t('workspace.conflict.skip.hint')"
      >
        <span>{{ t("workspace.conflict.skip") }}</span>
      </button>

      <button
        @click="emit('abort')"
        class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-danger/20 hover:bg-danger/30 text-danger font-semibold text-xs transition cursor-pointer border border-danger/40"
        :title="t('workspace.conflict.abort.hint')"
      >
        <RotateCcw class="w-3.5 h-3.5" />
        <span>{{ t("workspace.conflict.abort") }}</span>
      </button>
    </div>
  </div>
</template>
