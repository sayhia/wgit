<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { Activity, AlertTriangle, Download, RefreshCw } from "lucide-vue-next";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import {
  exportAICallLogs,
  getAIAdvancedConfig,
  getAIUsageSummary,
  saveAIAdvancedConfig,
  type AIUsageSummary,
} from "../../services/aiService";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });

const ALL_REPOS = "__all__";
const NO_REPO = "__none__";
const usage = ref<AIUsageSummary | null>(null);
const selectedRepo = ref(ALL_REPOS);
const budgetDraft = ref(0);
const isLoading = ref(false);
const isSaving = ref(false);
const isExporting = ref(false);
const errorMessage = ref("");

const numberFormat = new Intl.NumberFormat();
const costText = (value: number): string => `$${(value || 0).toFixed(4)}`;
const budgetWidth = computed(() => `${Math.min(100, Math.max(0, usage.value?.budgetPercent ?? 0))}%`);
const showBudgetWarning = computed(
  () => Boolean(usage.value && usage.value.monthlyBudgetUsd > 0 && usage.value.budgetPercent >= 80)
);
const knownTokens = computed(() => Number(usage.value?.tokensIn || 0) + Number(usage.value?.tokensOut || 0));
const repoBuckets = computed(() => usage.value?.byRepo || []);
const scopedRepo = computed(() => {
  if (selectedRepo.value === ALL_REPOS || selectedRepo.value === "") return null;
  return (
    repoBuckets.value.find((bucket) => (bucket.key || NO_REPO) === selectedRepo.value) || null
  );
});
const displayCalls = computed(() => scopedRepo.value?.calls ?? usage.value?.calls ?? 0);
const displayTokens = computed<number | null>(() => {
  if (scopedRepo.value) {
    return scopedRepo.value.usageKnownCalls > 0
      ? Number(scopedRepo.value.tokensIn || 0) + Number(scopedRepo.value.tokensOut || 0)
      : null;
  }
  return usage.value && usage.value.usageKnownCalls > 0 ? knownTokens.value : null;
});
const displayCost = computed(() =>
  scopedRepo.value ? scopedRepo.value.estimatedCostUsd : usage.value?.estimatedCostUsd || 0
);
const unknownCostCalls = computed(() =>
  scopedRepo.value
    ? Math.max(0, scopedRepo.value.calls - scopedRepo.value.costKnownCalls)
    : usage.value?.costUnknownCalls || 0
);
const showRepoFilter = computed(() => repoBuckets.value.length > 1);

function repoLabel(path: string): string {
  if (!path) return t("ai.usage.repo.none");
  const parts = path.split("/").filter(Boolean);
  if (parts.length <= 2) return path;
  return `${parts[parts.length - 2]}/${parts[parts.length - 1]}`;
}

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

async function loadUsage(): Promise<void> {
  isLoading.value = true;
  errorMessage.value = "";
  try {
    usage.value = await getAIUsageSummary(30);
    budgetDraft.value = usage.value.monthlyBudgetUsd || 0;
  } catch (err) {
    errorMessage.value = errorText(err);
  } finally {
    isLoading.value = false;
  }
}

async function saveBudget(): Promise<void> {
  if (isSaving.value) return;
  isSaving.value = true;
  try {
    const advanced = await getAIAdvancedConfig();
    await saveAIAdvancedConfig({
      ...advanced,
      monthlyBudgetUsd: Math.max(0, Number(budgetDraft.value) || 0),
      taskModels: advanced.taskModels || {},
      repoOverrides: advanced.repoOverrides || {},
    });
    await loadUsage();
    pushToast(t("ai.usage.budget.saved"), { tone: "success" });
  } catch (err) {
    pushToast(errorText(err), { tone: "danger" });
  } finally {
    isSaving.value = false;
  }
}

async function exportLogs(): Promise<void> {
  if (isExporting.value) return;
  isExporting.value = true;
  try {
    const path = await exportAICallLogs("csv");
    pushToast(t("ai.usage.exported", { path }), { tone: "success" });
  } catch (err) {
    pushToast(errorText(err), { tone: "danger" });
  } finally {
    isExporting.value = false;
  }
}

watch(
  () => props.refreshKey,
  (next, previous) => {
    if (next !== previous) void loadUsage();
  }
);

watch(usage, (next) => {
  if (selectedRepo.value === ALL_REPOS || selectedRepo.value === "") return;
  if (!(next?.byRepo || []).some((bucket) => (bucket.key || NO_REPO) === selectedRepo.value)) {
    selectedRepo.value = ALL_REPOS;
  }
});

onMounted(loadUsage);
</script>

<template>
  <div class="ai-card p-4 space-y-3" data-testid="ai-usage-card">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex items-center gap-2 min-w-0">
        <Activity class="w-4 h-4 text-accent shrink-0" />
        <div>
          <h4 class="ai-card-title">{{ t("ai.usage.title") }}</h4>
          <p class="ai-hint">{{ t("ai.usage.price.note") }}</p>
        </div>
      </div>
      <div class="flex items-center gap-1.5">
        <UiButton variant="ghost" :disabled="isLoading" :title="t('ai.usage.refresh')" :aria-label="t('ai.usage.refresh')" @click="loadUsage">
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
        </UiButton>
        <UiButton variant="secondary" :disabled="isExporting || !usage?.calls" @click="exportLogs">
          <Download class="w-3.5 h-3.5" />
          {{ t("ai.usage.export") }}
        </UiButton>
      </div>
    </div>

    <p v-if="errorMessage" role="alert" class="text-xs text-danger">{{ errorMessage }}</p>
    <p v-else-if="isLoading && !usage" class="text-xs text-ink-muted">{{ t("settings.ai.control.loading") }}</p>
    <p v-else-if="!usage" class="text-xs text-ink-muted">{{ t("ai.usage.empty") }}</p>
    <div v-if="usage && showRepoFilter" class="flex flex-wrap items-end gap-2">
      <label class="space-y-1 flex-1 min-w-[180px] max-w-[320px]">
        <span class="ai-field-label">{{ t("ai.usage.repo.filter") }}</span>
        <select
          v-model="selectedRepo"
          data-testid="ai-usage-repo-filter"
          class="ai-input"
        >
          <option :value="ALL_REPOS">{{ t("ai.usage.repo.all") }}</option>
          <option
            v-for="bucket in repoBuckets"
            :key="bucket.key || NO_REPO"
            :value="bucket.key || NO_REPO"
          >
            {{ repoLabel(bucket.key) }} · {{ numberFormat.format(bucket.calls) }}
          </option>
        </select>
      </label>
      <p class="ai-hint pb-1">{{ t("ai.usage.repo.scopehint") }}</p>
    </div>
    <div v-if="usage && usage.calls" class="grid grid-cols-2 lg:grid-cols-4 gap-2">
        <div class="p-2.5 rounded-xl bg-canvas border border-line/70">
          <div class="ai-hint">{{ t("ai.usage.calls") }}</div>
          <div class="text-sm font-bold text-ink mt-0.5">{{ numberFormat.format(displayCalls) }}</div>
        </div>
        <div class="p-2.5 rounded-xl bg-canvas border border-line/70">
          <div class="ai-hint">{{ t("ai.usage.tokens") }}</div>
          <div class="text-sm font-bold text-ink mt-0.5">
            {{ displayTokens === null ? t("ai.usage.unknown") : numberFormat.format(displayTokens) }}
          </div>
        </div>
        <div class="p-2.5 rounded-xl bg-canvas border border-line/70">
          <div class="ai-hint">{{ t("ai.usage.cost") }}</div>
          <div class="text-sm font-bold text-ink mt-0.5">{{ costText(displayCost) }}</div>
        </div>
        <div class="p-2.5 rounded-xl bg-canvas border border-line/70">
          <div class="ai-hint">{{ t("ai.usage.month") }}</div>
          <div class="text-sm font-bold text-ink mt-0.5">{{ costText(usage.monthCostUsd) }}</div>
        </div>
      </div>
      <p v-else class="text-xs text-ink-muted">{{ t("ai.usage.empty") }}</p>

      <p v-if="usage && unknownCostCalls > 0" class="ai-hint">
        {{ t("ai.usage.price.unknown", { count: unknownCostCalls }) }}
      </p>

      <div v-if="usage" class="flex flex-wrap items-end gap-2">
        <label class="space-y-1 flex-1 min-w-[140px] max-w-[220px]">
          <span class="ai-field-label">{{ t("ai.usage.budget") }} (USD)</span>
          <input
            v-model.number="budgetDraft"
            type="number"
            min="0"
            step="1"
            data-testid="ai-usage-budget-input"
            class="ai-input"
          />
        </label>
        <UiButton variant="secondary" :disabled="isSaving" data-testid="ai-usage-budget-save" @click="saveBudget">
          {{ t("ai.usage.budget.save") }}
        </UiButton>
        <div v-if="usage.monthlyBudgetUsd > 0" class="flex-1 min-w-[160px] space-y-1">
          <div class="flex items-center justify-between text-[10px]">
            <span class="text-ink-muted">{{ t("ai.usage.budget") }}</span>
            <span class="font-mono" :class="{ 'text-danger': usage.overBudget }">
              {{ costText(usage.monthCostUsd) }} / {{ costText(usage.monthlyBudgetUsd) }}
            </span>
          </div>
          <div class="h-1.5 rounded-full bg-line overflow-hidden">
            <div
              class="h-full rounded-full transition-all"
              :class="usage.overBudget ? 'bg-danger' : usage.budgetPercent >= 80 ? 'bg-warning' : 'bg-accent'"
              :style="{ width: budgetWidth }"
            ></div>
          </div>
        </div>
      </div>

      <div v-if="usage && showBudgetWarning" class="flex items-center gap-1.5 text-[10px]" :class="usage.overBudget ? 'text-danger' : 'text-warning'">
        <AlertTriangle class="w-3 h-3 shrink-0" />
        <span>
          {{ usage.overBudget ? t("ai.usage.over") : t("ai.usage.budgetwarn", { percent: Math.round(usage.budgetPercent) }) }}
        </span>
      </div>
  </div>
</template>
