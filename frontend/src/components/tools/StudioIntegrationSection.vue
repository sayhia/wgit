<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch } from "vue";
import { Activity, CheckCircle2, ChevronDown, ChevronRight, Play, RefreshCw, Rocket } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type {
  StudioCIResult,
  FullParityStatusSummary,
  PinnacleParityStatusSummary,
} from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

const expanded = ref(false);

const ci = ref<StudioCIResult | null>(null);
const isLoadingCI = ref(false);
const isTriggeringCI = ref(false);

const parityKind = ref<"" | "full" | "pinnacle">("");
const parity = ref<FullParityStatusSummary | PinnacleParityStatusSummary | null>(null);
const isLoadingParity = ref("");

async function loadCI() {
  if (!props.currentRepoPath || isLoadingCI.value) return;
  isLoadingCI.value = true;
  try {
    ci.value = await gitService.getStudioCIPipelines(props.currentRepoPath);
  } catch (err: any) {
    notify(t("studio.toast.ci.load.failed", { error: err?.message || err }));
  } finally {
    isLoadingCI.value = false;
  }
}

function ensureLoaded() {
  void loadCI();
}

function toggleExpanded() {
  expanded.value = !expanded.value;
  if (expanded.value && !ci.value) ensureLoaded();
}

async function handleTriggerCI() {
  if (!props.currentRepoPath || isTriggeringCI.value) return;
  if (!(await confirmAction(t("studio.confirm.trigger.ci")))) return;
  isTriggeringCI.value = true;
  try {
    const res = await gitService.triggerStudioCIPipeline(
      props.currentRepoPath,
      ci.value?.latestRun?.name || "Game-Client-Daily-Build"
    );
    notify(res?.message || t("studio.toast.ci.triggered"));
    await loadCI();
  } catch (err: any) {
    notify(t("studio.toast.ci.trigger.failed", { error: err?.message || err }));
  } finally {
    isTriggeringCI.value = false;
  }
}

async function handleLoadParity(kind: "full" | "pinnacle") {
  if (!props.currentRepoPath || isLoadingParity.value) return;
  isLoadingParity.value = kind;
  try {
    parity.value =
      kind === "full"
        ? await gitService.getFullParityStatusSummary(props.currentRepoPath)
        : await gitService.getPinnacleParityStatusSummary(props.currentRepoPath);
    parityKind.value = kind;
  } catch (err: any) {
    notify(t("studio.toast.parity.load.failed", { error: err?.message || err }));
  } finally {
    isLoadingParity.value = "";
  }
}

watch(
  () => props.currentRepoPath,
  () => {
    ci.value = null;
    parity.value = null;
    parityKind.value = "";
    if (expanded.value) ensureLoaded();
  }
);
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="flex items-center justify-between pb-2 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Rocket class="w-4 h-4 text-purple" />
          <span>{{ t("studio.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("studio.intro") }}
        </p>
      </div>
      <button
        @click="toggleExpanded"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
      >
        <component :is="expanded ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
        <span>{{ expanded ? t("studio.collapse") : t("studio.expand") }}</span>
      </button>
    </div>

    <div v-if="expanded" class="space-y-4">
      <!-- CI -->
      <div class="rounded-xl border border-line bg-surface p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <Activity class="w-4 h-4 text-accent" />
            <span class="text-xs font-bold text-ink">{{ t("studio.ci.title") }}</span>
            <span v-if="ci" class="text-[10px] text-ink-muted font-mono">{{ ci.provider }} · {{ ci.activeBranch }}</span>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="loadCI"
              :disabled="isLoadingCI"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingCI }" aria-hidden="true" />
              <span>{{ t("studio.ci.refresh") }}</span>
            </button>
            <button
              @click="handleTriggerCI"
              :disabled="isTriggeringCI || !ci?.latestRun"
              :title="ci?.latestRun ? t('studio.ci.trigger.hint') : t('studio.ci.trigger.disabled')"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Play class="w-3.5 h-3.5" aria-hidden="true" />
              <span>{{ isTriggeringCI ? t("studio.ci.triggering") : t("studio.ci.trigger") }}</span>
            </button>
          </div>
        </div>

        <template v-if="ci?.latestRun">
          <div class="flex flex-wrap items-center gap-2 text-[11px]">
            <span
              class="px-1.5 py-0.5 rounded font-bold"
              :class="ci.latestRun.status === 'SUCCESS' ? 'bg-success-bg text-success' : ci.latestRun.status === 'FAILED' ? 'bg-danger-bg text-danger' : 'bg-warning-bg text-warning'"
            >
              {{ ci.latestRun.status }}
            </span>
            <span class="font-mono text-ink">{{ ci.latestRun.id }}</span>
            <span class="text-ink-muted font-mono">{{ ci.latestRun.shortHash }}</span>
            <span class="text-ink-muted">{{ ci.latestRun.author }}</span>
            <span class="text-ink-muted ml-auto">{{ ci.latestRun.duration }}</span>
          </div>
          <div class="space-y-1">
            <div
              v-for="stage in ci.latestRun.stages"
              :key="stage.name"
              class="flex items-center gap-2 px-2 py-1 rounded bg-canvas border border-line text-[11px]"
            >
              <CheckCircle2 class="w-3 h-3 shrink-0" :class="stage.status === 'SUCCESS' ? 'text-success' : 'text-warning'" />
              <span class="text-ink truncate flex-1">{{ stage.name }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ stage.duration }}</span>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ ci.message }}</p>
        </template>
        <p v-else-if="!isLoadingCI" class="text-[11px] text-ink-muted">{{ ci?.message || t("studio.ci.empty") }}</p>
      </div>

      <!-- Backend capability inventory -->
      <div class="rounded-xl border border-line bg-surface p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div class="text-xs font-bold text-ink">{{ t("studio.parity.title") }}</div>
            <div class="text-[10px] text-ink-muted">{{ t("studio.parity.subtitle") }}</div>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="handleLoadParity('full')"
              :disabled="!!isLoadingParity"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              {{ isLoadingParity === "full" ? t("studio.loading") : t("studio.parity.full") }}
            </button>
            <button
              @click="handleLoadParity('pinnacle')"
              :disabled="!!isLoadingParity"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              {{ isLoadingParity === "pinnacle" ? t("studio.loading") : t("studio.parity.pinnacle") }}
            </button>
          </div>
        </div>

        <div v-if="parity" class="space-y-2">
          <div class="flex flex-wrap items-center gap-2 text-[11px]">
            <span class="px-1.5 py-0.5 rounded bg-accent-bg text-accent font-bold">
              {{ parity.completedRounds }}/{{ parity.totalRounds }} · {{ parity.parityPercentage }}%
            </span>
            <span class="text-ink-muted font-mono truncate">{{ parity.systemReadiness }}</span>
            <span class="text-ink-muted ml-auto font-mono">{{ parity.clientPlatform }}</span>
          </div>
          <div class="space-y-1">
            <div
              v-for="domain in parity.domains"
              :key="domain.domainId"
              class="flex items-center gap-2 px-2 py-1 rounded bg-canvas border border-line text-[11px]"
            >
              <span class="text-ink truncate flex-1">{{ domain.domainName }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ domain.completedCount }}/{{ domain.totalModules }}</span>
              <span class="text-success font-mono shrink-0">{{ domain.completionRate }}%</span>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{
            t("studio.parity.footnote", { kind: parityKind === "pinnacle" ? "Pinnacle" : "Full" })
          }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
