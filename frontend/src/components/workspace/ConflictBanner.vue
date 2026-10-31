<script setup lang="ts">
import { computed } from "vue";
import {
  AlertTriangle,
  CheckCircle2,
  RotateCcw,
  GitPullRequest,
  GitMerge,
  Layers,
  HelpCircle,
} from "lucide-vue-next";
import { t } from "../../i18n";
import { confirmAction } from "../../composables/useConfirm";

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

const currentBadge = computed(() => {
  if (props.isRebasing) {
    return {
      label: t("workspace.conflict.badge.rebasing"),
      classes: "bg-warning/20 text-warning border-warning/40",
      icon: GitPullRequest,
    };
  }
  if (props.isMerging) {
    return {
      label: t("workspace.conflict.badge.merging"),
      classes: "bg-accent/20 text-accent border-accent/40",
      icon: GitMerge,
    };
  }
  if (props.isCherryPicking) {
    return {
      label: t("workspace.conflict.badge.cherry_picking"),
      classes: "bg-purple-500/20 text-purple-400 border-purple-500/40",
      icon: Layers,
    };
  }
  return null;
});

const stepProgressionText = computed(() => {
  if (!props.isRebasing) return "";
  return t("rebase.progress.step", {
    step: props.rebaseStep || 1,
    total: props.rebaseTotal || "?",
  });
});

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

const helpText = computed(() => {
  if (props.conflictedFiles.length > 0) {
    return t("workspace.conflict.help.unresolved");
  }
  return t("workspace.conflict.help.resolved");
});

async function handleAbort() {
  const message = props.isRebasing
    ? t("rebase.abort.confirm")
    : t("workspace.conflict.abort.confirm");
  const confirmed = await confirmAction({ message, tone: "danger" });
  if (confirmed) {
    emit("abort");
  }
}
</script>

<template>
  <div
    class="px-4 py-2.5 bg-danger/10 border-b border-danger/30 text-xs flex flex-wrap items-center justify-between gap-3 shrink-0 select-none shadow-xs"
  >
    <!-- Left: Distinct Badges, Step Progression, Headline, Conflicted Count & Help Text -->
    <div class="flex items-center gap-2.5 flex-wrap min-w-0">
      <!-- Distinct Badges for Rebasing / Merging / Cherry-Picking -->
      <span
        v-if="currentBadge"
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full border text-[11px] font-bold shrink-0"
        :class="currentBadge.classes"
      >
        <component :is="currentBadge.icon" class="w-3 h-3 shrink-0" />
        <span>{{ currentBadge.label }}</span>
      </span>

      <!-- Step progression for rebasing (步骤 X / Y) -->
      <span
        v-if="isRebasing && stepProgressionText"
        class="inline-flex items-center px-2 py-0.5 rounded-md bg-warning/20 text-warning font-mono font-bold text-[11px] border border-warning/30 shrink-0"
      >
        {{ stepProgressionText }}
      </span>

      <div class="flex items-center gap-1.5 text-danger font-medium flex-wrap">
        <AlertTriangle class="w-4 h-4 shrink-0 animate-pulse text-danger" />
        <span>
          <strong class="font-bold text-ink">
            {{ headline }}
          </strong>:
          <span v-if="conflictedFiles.length > 0" class="text-danger font-semibold">
            {{ t("workspace.conflict.count", { count: conflictedFiles.length }) }}
          </span>
          <span v-else class="text-success font-semibold">
            {{ t("workspace.conflict.resolved") }}
          </span>
        </span>
      </div>

      <!-- Localized Help Text -->
      <span class="text-[11px] text-ink-muted hidden lg:inline-flex items-center gap-1">
        <HelpCircle class="w-3 h-3 text-ink-muted shrink-0" />
        <span>{{ helpText }}</span>
      </span>
    </div>

    <!-- Right: Operations (Continue / Skip / Abort) -->
    <div class="flex items-center gap-2 shrink-0">
      <button
        @click="emit('continue')"
        :disabled="conflictedFiles.length > 0"
        class="flex items-center gap-1 px-3 py-1 rounded-lg text-xs font-bold transition cursor-pointer shadow-xs disabled:opacity-40 disabled:cursor-not-allowed"
        :class="conflictedFiles.length === 0 ? 'bg-success-strong text-white hover:bg-success-strong-hover' : 'bg-surface border border-line text-ink-muted'"
        :title="t('workspace.conflict.continue.hint')"
      >
        <CheckCircle2 class="w-3.5 h-3.5" aria-hidden="true" />
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
        @click="handleAbort"
        class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-danger/20 hover:bg-danger/30 text-danger font-semibold text-xs transition cursor-pointer border border-danger/40"
        :title="t('workspace.conflict.abort.hint')"
      >
        <RotateCcw class="w-3.5 h-3.5" aria-hidden="true" />
        <span>{{ t("workspace.conflict.abort") }}</span>
      </button>
    </div>
  </div>
</template>
