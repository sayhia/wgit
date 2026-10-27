<script setup lang="ts">
import { ref } from "vue";
import {
  Play,
  Terminal,
  CheckCircle2,
  XCircle,
  Clock,
  Loader2,
  Shield,
  Layers,
  ChevronDown,
  ChevronRight,
  Sparkles,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { LocalCiResult } from "../../types";
import UiModal from "../ui/UiModal.vue";

const props = defineProps<{
  open: boolean;
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "notify", msg: string): void;
}>();

const rawCommands = ref("bun test\nbun run build");
const timeoutSec = ref(60);
const isRunning = ref(false);
const ciResult = ref<LocalCiResult | null>(null);
const expandedSteps = ref<Record<number, boolean>>({});

function applyPreset(preset: "typecheck" | "test" | "build") {
  if (preset === "typecheck") {
    rawCommands.value = "bun x vue-tsc --noEmit";
  } else if (preset === "test") {
    rawCommands.value = "go test -short ./...\nbun test";
  } else {
    rawCommands.value = "bun run build";
  }
}

function toggleStep(idx: number) {
  expandedSteps.value[idx] = !expandedSteps.value[idx];
}

async function handleRunCi() {
  if (!props.currentRepoPath || isRunning.value) return;
  const lines = rawCommands.value
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean);

  if (lines.length === 0) return;

  isRunning.value = true;
  ciResult.value = null;
  expandedSteps.value = {};

  try {
    const res = await gitService.runLocalCiSandbox(props.currentRepoPath, lines, timeoutSec.value);
    ciResult.value = res;
    // Auto-expand failed steps or all if fewer than 3
    res.steps.forEach((s, idx) => {
      if (!s.success || res.steps.length <= 2) {
        expandedSteps.value[idx] = true;
      }
    });
    if (res.success) {
      emit("notify", t("ci.status.all.passed", { duration: res.totalDurationMs }));
    } else {
      const failedIdx = res.steps.findIndex((s) => !s.success);
      const code = failedIdx >= 0 ? (res.steps[failedIdx]?.exitCode ?? 1) : 1;
      emit("notify", t("ci.status.failed", { step: failedIdx + 1, code }));
    }
  } catch (err: any) {
    emit("notify", err?.message || String(err));
  } finally {
    isRunning.value = false;
  }
}
</script>

<template>
  <UiModal
    :open="open"
    @update:open="emit('update:open', $event)"
    :title="t('ci.modal.title')"
    width="max-w-3xl"
  >
    <div class="p-4 space-y-4" data-testid="local-ci-modal">
      <!-- Isolation Banner -->
      <div class="p-3.5 rounded-xl border border-line bg-surface/50 flex items-start gap-3">
        <Shield class="w-5 h-5 text-accent shrink-0 mt-0.5" />
        <div class="text-xs space-y-1">
          <div class="font-bold text-sm text-ink">{{ t("ci.modal.title") }}</div>
          <p class="text-ink-muted leading-relaxed">
            {{ t("ci.modal.desc") }}
          </p>
        </div>
      </div>

      <!-- Presets Toolbar -->
      <div class="flex items-center gap-2">
        <span class="text-xs text-ink-muted flex items-center gap-1 font-medium">
          <Sparkles class="w-3.5 h-3.5 text-accent" />
          <span>Presets:</span>
        </span>
        <button
          @click="applyPreset('typecheck')"
          class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-mono text-ink transition cursor-pointer"
        >
          {{ t("ci.preset.typecheck") }}
        </button>
        <button
          @click="applyPreset('test')"
          class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-mono text-ink transition cursor-pointer"
        >
          {{ t("ci.preset.test") }}
        </button>
        <button
          @click="applyPreset('build')"
          class="px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-mono text-ink transition cursor-pointer"
        >
          {{ t("ci.preset.build") }}
        </button>
      </div>

      <!-- Command Pipeline Input -->
      <div class="space-y-1.5">
        <label class="text-xs font-semibold text-ink flex items-center justify-between">
          <span>{{ t("ci.commands.label") }}</span>
          <span class="text-[11px] font-mono text-ink-muted">Timeout: {{ timeoutSec }}s</span>
        </label>
        <textarea
          v-model="rawCommands"
          rows="4"
          :placeholder="t('ci.commands.placeholder')"
          class="w-full px-3 py-2 rounded-xl bg-canvas border border-line font-mono text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
        />
      </div>

      <!-- Action Button -->
      <div class="flex items-center justify-between pt-1">
        <span class="text-[11px] text-ink-muted">
          Detached worktree at HEAD
        </span>
        <button
          @click="handleRunCi"
          :disabled="!rawCommands.trim() || isRunning"
          class="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-accent text-accent-contrast hover:bg-accent-strong text-xs font-semibold transition cursor-pointer disabled:opacity-40 shadow-xs"
          data-testid="run-ci-btn"
        >
          <Loader2 v-if="isRunning" class="w-3.5 h-3.5 animate-spin" />
          <Play v-else class="w-3.5 h-3.5" />
          <span>{{ isRunning ? t("ci.btn.running") : t("ci.btn.run") }}</span>
        </button>
      </div>

      <!-- Results View -->
      <div v-if="ciResult" class="space-y-3 pt-3 border-t border-line" data-testid="ci-results">
        <div
          class="p-3 rounded-xl border flex items-center justify-between"
          :class="ciResult.success ? 'bg-emerald/10 border-emerald/30 text-emerald' : 'bg-rose/10 border-rose/30 text-rose'"
        >
          <div class="flex items-center gap-2 font-bold text-xs">
            <CheckCircle2 v-if="ciResult.success" class="w-4 h-4" />
            <XCircle v-else class="w-4 h-4" />
            <span>{{ ciResult.summary }}</span>
          </div>
          <span class="font-mono text-[11px]">{{ ciResult.totalDurationMs }}ms</span>
        </div>

        <!-- Step list -->
        <div class="space-y-2 max-h-[300px] overflow-y-auto">
          <div
            v-for="(step, idx) in ciResult.steps"
            :key="idx"
            class="rounded-lg border border-line bg-surface/70 overflow-hidden"
          >
            <div
              @click="toggleStep(idx)"
              class="p-2.5 flex items-center justify-between gap-2 cursor-pointer hover:bg-raised transition"
            >
              <div class="flex items-center gap-2 min-w-0">
                <CheckCircle2 v-if="step.success" class="w-3.5 h-3.5 text-emerald shrink-0" />
                <XCircle v-else class="w-3.5 h-3.5 text-rose shrink-0" />
                <span class="font-mono font-bold text-xs text-ink truncate">{{ step.command }}</span>
              </div>
              <div class="flex items-center gap-2 text-[11px] font-mono shrink-0">
                <span class="text-ink-muted">{{ step.durationMs }}ms</span>
                <span
                  class="px-1.5 py-0.2 rounded font-semibold text-[10px]"
                  :class="step.success ? 'bg-emerald/15 text-emerald' : 'bg-rose/15 text-rose'"
                >
                  Exit: {{ step.exitCode }}
                </span>
                <ChevronDown v-if="expandedSteps[idx]" class="w-3.5 h-3.5 text-ink-muted" />
                <ChevronRight v-else class="w-3.5 h-3.5 text-ink-muted" />
              </div>
            </div>

            <!-- Console Output -->
            <div
              v-if="expandedSteps[idx]"
              class="p-3 border-t border-line bg-canvas font-mono text-[11px] text-ink max-h-48 overflow-auto whitespace-pre-wrap break-all leading-relaxed"
            >
              {{ step.output || "No output." }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </UiModal>
</template>
