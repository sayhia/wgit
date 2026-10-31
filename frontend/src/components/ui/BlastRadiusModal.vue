<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { AlertTriangle, ShieldAlert, CheckCircle2, Flame, Loader2, ArrowRight } from "lucide-vue-next";
import UiModal from "./UiModal.vue";
import UiButton from "./UiButton.vue";
import { gitService } from "../../services/gitService";
import type { BlastRadiusReport } from "../../types";
import { t } from "../../i18n";

const props = withDefaults(
  defineProps<{
    open: boolean;
    repoPath: string;
    actionType: string;
    targetRef?: string;
    title?: string;
  }>(),
  {
    targetRef: "HEAD",
    title: "",
  }
);

const emit = defineEmits<{
  (e: "close"): void;
  (e: "confirm"): void;
}>();

const loading = ref(false);
const report = ref<BlastRadiusReport | null>(null);

async function evaluate() {
  if (!props.open || !props.repoPath || !props.actionType) return;
  loading.value = true;
  try {
    const res = await gitService.calculateBlastRadius(
      props.repoPath,
      props.actionType,
      props.targetRef || "HEAD"
    );
    report.value = res ?? null;
  } catch (err) {
    /* best-effort preview */
    report.value = null;
  } finally {
    loading.value = false;
  }
}

watch(
  () => [props.open, props.repoPath, props.actionType, props.targetRef],
  () => {
    if (props.open) {
      evaluate();
    } else {
      report.value = null;
    }
  },
  { immediate: true }
);

const riskLevel = computed(() => report.value?.riskLevel || "low");

const riskBadgeClass = computed(() => {
  switch (riskLevel.value) {
    case "critical":
      return "bg-danger text-danger-contrast border-danger shadow-xs";
    case "high":
      return "bg-danger-bg text-danger border-danger/30";
    case "medium":
      return "bg-warning-bg text-warning border-warning/30";
    default:
      return "bg-success-bg text-success border-success/30";
  }
});

const riskIcon = computed(() => {
  switch (riskLevel.value) {
    case "critical":
    case "high":
      return Flame;
    case "medium":
      return AlertTriangle;
    default:
      return CheckCircle2;
  }
});
</script>

<template>
  <UiModal
    :open="open"
    :title="title || t('blast.radius.title')"
    width="max-w-md"
    data-testid="blast-radius-modal"
    @update:open="(v) => !v && emit('close')"
  >
    <div v-if="loading" class="py-8 flex flex-col items-center justify-center gap-2 text-ink-muted">
      <Loader2 class="w-6 h-6 animate-spin text-accent" />
      <span class="text-xs">{{ t("blast.radius.evaluating") }}</span>
    </div>

    <div v-else class="space-y-4">
      <!-- Risk Header Banner -->
      <div class="flex items-center justify-between p-3 rounded-xl bg-canvas border border-line">
        <div class="flex items-center gap-2">
          <component :is="riskIcon" class="w-4 h-4 shrink-0" :class="riskLevel === 'critical' ? 'text-danger animate-pulse' : 'text-accent'" />
          <span class="text-xs font-semibold text-ink">{{ t("blast.radius.title") }}</span>
        </div>
        <span
          data-testid="blast-risk-badge"
          class="px-2 py-0.5 rounded text-[11px] font-bold uppercase tracking-wider border"
          :class="riskBadgeClass"
        >
          {{ t(`blast.radius.risk.${riskLevel}`) || riskLevel }}
        </span>
      </div>

      <!-- Warning Message -->
      <p v-if="report?.warningMessage" class="text-xs text-ink leading-relaxed break-words">
        {{ report.warningMessage }}
      </p>

      <!-- Metric Stats Cards -->
      <div class="grid grid-cols-2 gap-2 text-xs">
        <div class="p-2.5 rounded-lg bg-canvas border border-line flex flex-col gap-0.5">
          <span class="text-[11px] text-ink-muted">{{ t("blast.radius.uncommitted") }}</span>
          <span class="text-base font-bold text-ink">
            {{ report?.uncommittedFilesCount ?? 0 }}
          </span>
        </div>
        <div class="p-2.5 rounded-lg bg-canvas border border-line flex flex-col gap-0.5">
          <span class="text-[11px] text-ink-muted">{{ t("blast.radius.dangling") }}</span>
          <span class="text-base font-bold text-ink" :class="(report?.danglingCommitsCount ?? 0) > 0 ? 'text-danger font-mono' : ''">
            {{ report?.danglingCommitsCount ?? 0 }}
          </span>
        </div>
      </div>

      <!-- Suggested Alternative -->
      <div
        v-if="report?.suggestedAlternative"
        class="p-2.5 rounded-lg bg-accent-bg border border-accent/20 text-xs text-ink flex items-start gap-2"
      >
        <ArrowRight class="w-3.5 h-3.5 text-accent shrink-0 mt-0.5" />
        <div class="min-w-0">
          <span class="font-semibold text-accent block text-[11px]">{{ t("blast.radius.alternative") }}</span>
          <span class="text-[11px] text-ink-muted">{{ report.suggestedAlternative }}</span>
        </div>
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="emit('close')">
        {{ t("blast.radius.cancel") }}
      </UiButton>
      <UiButton
        :variant="riskLevel === 'critical' || riskLevel === 'high' ? 'danger' : 'primary'"
        @click="emit('confirm')"
      >
        {{ t("blast.radius.proceed") }}
      </UiButton>
    </template>
  </UiModal>
</template>
