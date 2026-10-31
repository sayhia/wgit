<script setup lang="ts">
import { computed } from "vue";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";
import { GitMerge, ArrowRight, RefreshCw, CheckCircle2, AlertTriangle, Users } from "lucide-vue-next";
import type { MergePreviewResult, MergeOverlapResult } from "../../../types";
import { backendText, t } from "../../../i18n";

const props = withDefaults(
  defineProps<{
    open: boolean;
    source: string;
    currentBranch: string;
    preview: MergePreviewResult | null;
    loadingPreview: boolean;
    overlap: MergeOverlapResult | null;
    loadingOverlap: boolean;
    mode: "default" | "no-ff" | "squash" | "ff-only";
    strategy: "" | "ours" | "theirs";
    autostash: boolean;
    message: string;
    merging: boolean;
    /** Pre-merge probes failed; warn in-modal so nobody merges blind. */
    previewError?: string;
  }>(),
  { previewError: "" },
);

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:mode", value: "default" | "no-ff" | "squash" | "ff-only"): void;
  (e: "update:strategy", value: "" | "ours" | "theirs"): void;
  (e: "update:autostash", value: boolean): void;
  (e: "update:message", value: string): void;
  (e: "confirm"): void;
}>();

const modeModel = computed({
  get: () => props.mode,
  set: (value: "default" | "no-ff" | "squash" | "ff-only") => emit("update:mode", value),
});
const strategyModel = computed({
  get: () => props.strategy,
  set: (value: "" | "ours" | "theirs") => emit("update:strategy", value),
});
const autostashModel = computed({
  get: () => props.autostash,
  set: (value: boolean) => emit("update:autostash", value),
});
/** The overlap summary ships a catalog key + params; prefer it over the text. */
function overlapMessage(): string {
  const o = props.overlap;
  return o ? backendText(o.messageKey, o.message, o.messageParams) : "";
}

const messageModel = computed({
  get: () => props.message,
  set: (value: string) => emit("update:message", value),
});
</script>

<template>
  <UiModal
    :open="open"
    :title="t('branches.merge.modal.title')"
    width="max-w-[500px]"
    body-class="space-y-4"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <GitMerge class="w-4 h-4 text-accent" />
        <span>{{ t("branches.merge.modal.title") }}</span>
      </h3>
    </template>

        <div class="p-3 rounded-xl bg-canvas border border-line flex items-center justify-between">
          <div class="flex flex-col">
            <span class="text-[10px] text-ink-muted">{{ t("branches.merge.modal.source") }}</span>
            <span class="font-mono text-xs font-bold text-accent">{{ source }}</span>
          </div>
          <ArrowRight class="w-4 h-4 text-ink-muted" aria-hidden="true" />
          <div class="flex flex-col items-end">
            <span class="text-[10px] text-ink-muted">{{ t("branches.merge.modal.target") }}</span>
            <span class="font-mono text-xs font-bold text-ink">{{ currentBranch }}</span>
          </div>
        </div>

        <!-- Pre-merge probes failed for one or both sides: keep the warning
             next to the preview so nobody merges blind. -->
        <div
          v-if="previewError"
          role="alert"
          class="rounded-xl border border-warning/40 bg-warning/15 px-3 py-2 text-[11px] leading-relaxed text-warning flex items-start gap-2"
        >
          <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-px" aria-hidden="true" />
          <span class="break-words">{{ t("branch.toast.merge.preview.failed", { error: previewError }) }}</span>
        </div>

        <!-- Pre-merge Conflict Preview Card -->
        <div class="rounded-xl border p-3 flex flex-col gap-2 text-xs" :class="preview?.canMergeCleanly ? 'bg-success/10 border-success/30' : preview?.hasConflicts ? 'bg-danger/10 border-danger/30' : 'bg-canvas border-line'">
          <div class="flex items-center justify-between">
            <span class="font-bold flex items-center gap-1.5" :class="preview?.canMergeCleanly ? 'text-success' : preview?.hasConflicts ? 'text-danger' : 'text-ink-secondary'">
              <span v-if="loadingPreview" class="flex items-center gap-1">
                <RefreshCw class="w-3.5 h-3.5 animate-spin" />
                {{ t("branches.merge.modal.preview.loading") }}
              </span>
              <span v-else-if="preview?.canMergeCleanly" class="flex items-center gap-1">
                <CheckCircle2 class="w-4 h-4" />
                {{ t("branches.merge.modal.preview.clean") }}
              </span>
              <span v-else-if="preview?.hasConflicts" class="flex items-center gap-1">
                <AlertTriangle class="w-4 h-4" />
                {{ t("branches.merge.modal.preview.conflicts", { count: preview.conflicts.length }) }}
              </span>
            </span>

            <span v-if="preview" class="text-[10px] text-ink-muted">
              {{ t("branches.merge.modal.preview.changed", { count: preview.changedFiles.length }) }}
            </span>
          </div>

          <!-- Conflicted files list preview -->
          <div v-if="preview?.hasConflicts && preview.conflicts.length > 0" class="max-h-24 overflow-y-auto space-y-1 font-mono text-[10px] text-danger bg-danger/10 p-2 rounded-lg">
            <div v-for="cf in preview.conflicts" :key="cf" class="truncate">• {{ cf }}</div>
          </div>
        </div>

        <!-- Team overlap: merge-tree says nothing about WHO is editing the same files -->
        <div
          v-if="loadingOverlap || (overlap && overlap.overlapFiles.length > 0)"
          class="rounded-xl border p-3 flex flex-col gap-2 text-xs"
          :class="overlap?.riskLevel === 'high' ? 'bg-danger/10 border-danger/30' : overlap?.riskLevel === 'medium' ? 'bg-warning/10 border-warning/30' : 'bg-canvas border-line'"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="font-bold flex items-center gap-1.5 text-ink">
              <Users class="w-3.5 h-3.5" />
              <span v-if="loadingOverlap" class="flex items-center gap-1">
                <RefreshCw class="w-3 h-3 animate-spin" />
                {{ t("branches.merge.modal.overlap.loading") }}
              </span>
              <span v-else>{{ t("branches.merge.modal.overlap.count", { count: overlap?.overlapFiles.length ?? 0 }) }}</span>
            </span>
            <span v-if="overlap" class="text-[10px] text-ink-muted shrink-0">
              {{ t("branches.merge.modal.overlap.split", { mine: overlap.myFileCount, theirs: overlap.teamFileCount }) }}
            </span>
          </div>

          <p v-if="overlap" class="text-[11px] text-ink-secondary leading-relaxed">{{ overlapMessage() }}</p>

          <div v-if="overlap && overlap.overlapFiles.length > 0" class="max-h-24 overflow-y-auto space-y-1">
            <div
              v-for="f in overlap.overlapFiles"
              :key="f.path"
              class="flex items-center gap-2 font-mono text-[10px] text-ink-muted"
            >
              <span class="truncate flex-1" :title="f.path">{{ f.path }}</span>
              <span class="shrink-0">{{ f.authors.join(" / ") }}</span>
            </div>
          </div>

          <p v-if="overlap && overlap.authors.length > 0" class="text-[10px] text-ink-muted">
            {{
              t("branches.merge.modal.overlap.advice", {
                authors: overlap.authors.join(t("common.list.comma")),
              })
            }}
          </p>
        </div>

        <div class="flex flex-col gap-1.5">
          <label class="text-[11px] text-ink-secondary font-medium">{{ t("branches.merge.modal.mode") }}</label>
          <!-- Toggle-button radio group: the container names the choice set,
               each button carries its pressed state for assistive tech. -->
          <div class="grid grid-cols-2 gap-2" role="group" :aria-label="t('branches.merge.modal.mode')">
            <button
              @click="modeModel = 'no-ff'"
              :aria-pressed="modeModel === 'no-ff'"
              class="p-2 rounded-lg border text-left text-xs transition cursor-pointer"
              :class="modeModel === 'no-ff' ? 'border-accent bg-accent/10 text-accent font-semibold' : 'border-line bg-canvas text-ink-secondary'"
            >
              <div>{{ t("branches.merge.modal.mode.noff") }}</div>
              <div class="text-[10px] text-ink-muted font-normal mt-0.5">{{ t("branches.merge.modal.mode.noff.desc") }}</div>
            </button>
            <button
              @click="modeModel = 'squash'"
              :aria-pressed="modeModel === 'squash'"
              class="p-2 rounded-lg border text-left text-xs transition cursor-pointer"
              :class="modeModel === 'squash' ? 'border-accent bg-accent/10 text-accent font-semibold' : 'border-line bg-canvas text-ink-secondary'"
            >
              <div>{{ t("branches.merge.modal.mode.squash") }}</div>
              <div class="text-[10px] text-ink-muted font-normal mt-0.5">{{ t("branches.merge.modal.mode.squash.desc") }}</div>
            </button>
            <button
              @click="modeModel = 'ff-only'"
              :aria-pressed="modeModel === 'ff-only'"
              class="p-2 rounded-lg border text-left text-xs transition cursor-pointer"
              :class="modeModel === 'ff-only' ? 'border-accent bg-accent/10 text-accent font-semibold' : 'border-line bg-canvas text-ink-secondary'"
            >
              <div>{{ t("branches.merge.modal.mode.ffonly") }}</div>
              <div class="text-[10px] text-ink-muted font-normal mt-0.5">{{ t("branches.merge.modal.mode.ffonly.desc") }}</div>
            </button>
            <button
              @click="modeModel = 'default'"
              :aria-pressed="modeModel === 'default'"
              class="p-2 rounded-lg border text-left text-xs transition cursor-pointer"
              :class="modeModel === 'default' ? 'border-accent bg-accent/10 text-accent font-semibold' : 'border-line bg-canvas text-ink-secondary'"
            >
              <div>{{ t("branches.merge.modal.mode.default") }}</div>
              <div class="text-[10px] text-ink-muted font-normal mt-0.5">{{ t("branches.merge.modal.mode.default.desc") }}</div>
            </button>
          </div>
        </div>

        <!-- Strategy Options (-X ours / theirs) & Autostash -->
        <div class="grid grid-cols-2 gap-2">
          <div class="flex flex-col gap-1">
            <label class="text-[11px] text-ink-secondary font-medium" for="wgit-merge-strategy">{{ t("branches.merge.modal.strategy") }}</label>
            <select
              id="wgit-merge-strategy"
              v-model="strategyModel"
              class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
            >
              <option value="">{{ t("branches.merge.modal.strategy.default") }}</option>
              <option value="ours">{{ t("branches.merge.modal.strategy.ours") }}</option>
              <option value="theirs">{{ t("branches.merge.modal.strategy.theirs") }}</option>
            </select>
          </div>

          <div class="flex flex-col justify-end">
            <label class="flex items-center gap-2 text-xs text-ink cursor-pointer select-none py-1">
              <input
                v-model="autostashModel"
                type="checkbox"
                class="rounded border-line bg-canvas text-accent focus:ring-0"
              />
              <span>{{ t("branches.merge.modal.autostash") }}</span>
            </label>
          </div>
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-[11px] text-ink-secondary font-medium" for="wgit-merge-message">{{ t("branches.merge.modal.message") }}</label>
          <input
            id="wgit-merge-message"
            v-model="messageModel"
            type="text"
            class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans"
          />
        </div>

    <template #footer>
      <UiButton variant="secondary" :disabled="merging" @click="emit('update:open', false)">
        {{ t("branches.merge.modal.cancel") }}
      </UiButton>
      <UiButton variant="primary" :disabled="merging" @click="emit('confirm')">
        <RefreshCw v-if="merging" class="w-3.5 h-3.5 animate-spin" />
        <span>{{ merging ? t("branches.merge.modal.merging") : t("branches.merge.modal.confirm") }}</span>
      </UiButton>
    </template>
  </UiModal>
</template>
