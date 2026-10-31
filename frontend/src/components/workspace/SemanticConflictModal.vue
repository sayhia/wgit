<script setup lang="ts">
import { ref, watch } from "vue";
import {
  GitMerge,
  Sparkles,
  Layers,
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  Loader2,
  Wand2,
  FileCode,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { YamlConflictAnalysisResult } from "../../types";
import UiModal from "../ui/UiModal.vue";

const props = defineProps<{
  open: boolean;
  filePath: string;
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "resolved", filePath: string): void;
  (e: "open-ai-batch"): void;
  (e: "notify", msg: string): void;
}>();

const analysis = ref<YamlConflictAnalysisResult | null>(null);
const isLoading = ref(false);
const isResolving = ref(false);

async function loadAnalysis() {
  if (!props.open || !props.currentRepoPath || !props.filePath) return;
  isLoading.value = true;
  analysis.value = null;
  try {
    const res = await gitService.analyzeYaml3WayConflict(props.currentRepoPath, props.filePath);
    analysis.value = res;
  } catch (err: any) {
    emit("notify", err?.message || String(err));
  } finally {
    isLoading.value = false;
  }
}

async function handleResolve(strategy: string) {
  if (!props.currentRepoPath || !props.filePath || isResolving.value) return;
  isResolving.value = true;
  try {
    const res = await gitService.autoResolveSemanticYamlConflict(props.currentRepoPath, props.filePath, strategy);
    if (res.success) {
      emit("notify", t("conflict.semantic.success", { count: res.resolvedBlocks }));
      emit("resolved", props.filePath);
      emit("update:open", false);
    } else {
      emit("notify", res.message);
    }
  } catch (err: any) {
    emit("notify", err?.message || String(err));
  } finally {
    isResolving.value = false;
  }
}

function getConflictTypeLabel(type: string): string {
  switch (type) {
    case "both_added_component":
      return t("conflict.semantic.type.component");
    case "transform_modified":
      return t("conflict.semantic.type.transform");
    default:
      return t("conflict.semantic.type.property");
  }
}

watch(
  () => props.open,
  (val) => {
    if (val) void loadAnalysis();
  }
);
</script>

<template>
  <UiModal
    :open="open"
    @update:open="emit('update:open', $event)"
    :title="t('conflict.semantic.title')"
    width="max-w-3xl"
  >
    <div class="p-4 space-y-4" data-testid="semantic-conflict-modal">
      <!-- Header Banner -->
      <div class="p-3.5 rounded-xl border border-line bg-surface flex items-start gap-3">
        <Sparkles class="w-5 h-5 text-accent shrink-0 mt-0.5" />
        <div class="text-xs space-y-1">
          <div class="flex items-center gap-2">
            <span class="font-bold text-sm text-ink">{{ t("conflict.semantic.title") }}</span>
            <span class="font-mono text-[11px] text-ink-muted">({{ filePath }})</span>
            <button
              type="button"
              class="ml-auto px-2 py-0.5 rounded-lg border border-accent/30 bg-accent-bg text-[10px] font-semibold text-accent hover:bg-accent-bg transition cursor-pointer"
              @click="emit('open-ai-batch')"
            >
              {{ t("workspace.conflict.batch.btn") }}
            </button>
          </div>
          <p class="text-ink-muted leading-relaxed">
            {{ t("conflict.semantic.desc") }}
          </p>
        </div>
      </div>

      <div v-if="isLoading" class="py-16 flex flex-col items-center justify-center gap-2 text-xs text-ink-muted">
        <Loader2 class="w-6 h-6 animate-spin text-accent" />
        <span>{{ t("conflict.semantic.resolving") }}</span>
      </div>

      <div v-else-if="!analysis || analysis.totalConflictBlocks === 0" class="py-16 text-center text-xs text-ink-muted">
        {{ t("conflict.semantic.empty") }}
      </div>

      <div v-else class="space-y-4">
        <!-- Stats Bar & Quick Strategies -->
        <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
          <div class="flex items-center gap-2">
            <span class="px-2.5 py-1 rounded-lg bg-surface border border-line font-medium text-ink">
              {{ t("conflict.semantic.blocks", { count: analysis.totalConflictBlocks }) }}
            </span>
            <span
              v-if="analysis.autoResolvableCount > 0"
              class="px-2.5 py-1 rounded-lg bg-emerald/15 border border-emerald/30 text-emerald font-semibold"
            >
              {{ t("conflict.semantic.autocount", { count: analysis.autoResolvableCount }) }}
            </span>
          </div>

          <!-- Action Buttons -->
          <div class="flex items-center gap-2">
            <button
              @click="handleResolve('prefer_ours')"
              :disabled="isResolving"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
            >
              {{ t("conflict.semantic.btn.ours") }}
            </button>
            <button
              @click="handleResolve('prefer_theirs')"
              :disabled="isResolving"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
            >
              {{ t("conflict.semantic.btn.theirs") }}
            </button>
            <button
              @click="handleResolve('merge_both')"
              :disabled="isResolving"
              class="px-3 py-1.5 rounded-lg bg-purple text-white hover:bg-purple-strong text-xs font-semibold transition cursor-pointer shadow-xs"
            >
              {{ t("conflict.semantic.btn.both") }}
            </button>
            <button
              @click="handleResolve('smart_auto')"
              :disabled="isResolving"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent text-accent-contrast hover:bg-accent-strong text-xs font-semibold transition cursor-pointer shadow-xs"
              data-testid="smart-resolve-btn"
            >
              <Loader2 v-if="isResolving" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
              <Wand2 v-else class="w-3.5 h-3.5" aria-hidden="true" />
              <span>{{ t("conflict.semantic.btn.smart") }}</span>
            </button>
          </div>
        </div>

        <!-- Conflict Blocks List -->
        <div class="space-y-3 max-h-[360px] overflow-y-auto">
          <div
            v-for="block in analysis.blocks"
            :key="block.id"
            class="p-3.5 rounded-xl border border-line bg-surface space-y-2.5"
          >
            <div class="flex items-center justify-between text-xs">
              <div class="flex items-center gap-2">
                <span class="font-bold font-mono text-ink">{{ block.objectType }}</span>
                <span v-if="block.fileId" class="text-[10px] font-mono text-ink-muted">
                  &amp;{{ block.fileId }}
                </span>
                <span class="px-2 py-0.5 rounded text-[10px] bg-canvas border border-line font-medium text-ink-secondary">
                  {{ getConflictTypeLabel(block.conflictType) }}
                </span>
              </div>
              <span
                class="px-2 py-0.5 rounded text-[10px] font-semibold"
                :class="block.autoResolved ? 'bg-emerald/15 text-emerald border border-emerald/30' : 'bg-amber/15 text-amber border border-amber/30'"
              >
                {{ block.suggestedStrategy }}
              </span>
            </div>

            <!-- Two-column diff preview -->
            <div class="grid grid-cols-1 md:grid-cols-2 gap-2 text-[11px] font-mono">
              <div class="p-2.5 rounded-lg bg-canvas border border-emerald/30 space-y-1">
                <div class="text-[10px] text-emerald font-bold">Ours (Current)</div>
                <pre class="overflow-x-auto whitespace-pre text-ink leading-relaxed">{{ block.oursContent || "(empty)" }}</pre>
              </div>

              <div class="p-2.5 rounded-lg bg-canvas border border-purple/30 space-y-1">
                <div class="text-[10px] text-purple font-bold">Theirs (Incoming)</div>
                <pre class="overflow-x-auto whitespace-pre text-ink leading-relaxed">{{ block.theirsContent || "(empty)" }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </UiModal>
</template>
