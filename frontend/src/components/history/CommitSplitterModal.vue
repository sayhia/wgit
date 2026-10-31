<script setup lang="ts">
import { ref, watch } from "vue";
import { GitCommit, Split, Plus, Trash2, CheckCircle2, AlertTriangle, RefreshCw } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import type { SplitCommitPlan, SplitCommitResult } from "../../types";
import { t } from "../../i18n";

const props = withDefaults(
  defineProps<{
    open: boolean;
    repoPath: string;
    commitSha: string;
    commitSubject?: string;
  }>(),
  {
    commitSubject: "",
  }
);

const emit = defineEmits<{
  (e: "close"): void;
  (e: "split"): void;
}>();

const plans = ref<SplitCommitPlan[]>([
  { message: "fix: first part of atomic split", files: [] },
  { message: "refactor: second part of atomic split", files: [] },
]);

const fileInputTexts = ref<string[]>(["", ""]);

const isExecuting = ref(false);
const result = ref<SplitCommitResult | null>(null);
const errorMessage = ref<string | null>(null);

watch(
  () => props.open,
  (val) => {
    if (val) {
      result.value = null;
      errorMessage.value = null;
      plans.value = [
        { message: "fix: first part of atomic split", files: [] },
        { message: "refactor: second part of atomic split", files: [] },
      ];
      fileInputTexts.value = ["", ""];
    }
  }
);

function addPlan() {
  plans.value.push({
    message: `split: part ${plans.value.length + 1}`,
    files: [],
  });
  fileInputTexts.value.push("");
}

function removePlan(idx: number) {
  if (plans.value.length <= 1) return;
  plans.value.splice(idx, 1);
  fileInputTexts.value.splice(idx, 1);
}

async function handleExecuteSplit() {
  if (!props.repoPath || isExecuting.value) return;

  // Sync file inputs into plans
  plans.value.forEach((p, idx) => {
    const raw = fileInputTexts.value[idx] || "";
    const list = raw
      .split(/[,\n]/)
      .map((s) => s.trim())
      .filter(Boolean);
    p.files = list;
  });

  isExecuting.value = true;
  errorMessage.value = null;
  result.value = null;

  try {
    const res = await gitService.deconstructCommit(
      props.repoPath,
      props.commitSha || "HEAD",
      plans.value
    );
    result.value = res;
    if (res && res.success) {
      emit("split");
    } else if (res && !res.success) {
      errorMessage.value = res.summary;
    }
  } catch (err: any) {
    errorMessage.value = err?.message || String(err);
  } finally {
    isExecuting.value = false;
  }
}
</script>

<template>
  <UiModal
    :open="open"
    :title="t('history.splitter.title')"
    width="max-w-xl"
    data-testid="commit-splitter-modal"
    @update:open="(v) => !v && !isExecuting && emit('close')"
  >
    <div class="space-y-4">
      <!-- Target Commit Info -->
      <div class="p-3 rounded-xl bg-canvas border border-line flex items-center justify-between text-xs">
        <div class="flex items-center gap-2 truncate">
          <Split class="w-4 h-4 text-accent shrink-0" />
          <span class="font-mono font-bold text-accent">{{ commitSha ? commitSha.slice(0, 7) : 'HEAD' }}</span>
          <span class="font-semibold text-ink truncate">{{ commitSubject || t('history.splitter.target') }}</span>
        </div>
      </div>

      <p class="text-xs text-ink-muted leading-relaxed">
        {{ t('history.splitter.desc') }}
      </p>

      <!-- Plans Buckets -->
      <div class="space-y-3">
        <div
          v-for="(plan, idx) in plans"
          :key="idx"
          data-testid="split-plan-bucket"
          class="p-3 rounded-xl border border-line bg-surface/40 space-y-2 text-xs"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="font-bold text-ink">#{{ idx + 1 }} {{ t('history.splitter.bucket') }}</span>
            <button
              v-if="plans.length > 1"
              type="button"
              @click="removePlan(idx)"
              class="p-1 rounded hover:bg-danger/15 text-ink-muted hover:text-danger transition cursor-pointer"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>

          <input
            v-model="plan.message"
            type="text"
            :placeholder="t('history.splitter.msg.placeholder')"
            class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
          />

          <textarea
            v-model="fileInputTexts[idx]"
            rows="2"
            :placeholder="t('history.splitter.files.placeholder')"
            class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
          />
        </div>

        <button
          type="button"
          data-testid="add-split-plan-btn"
          @click="addPlan"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-line bg-surface hover:bg-raised text-xs font-semibold text-ink transition cursor-pointer w-full justify-center"
        >
          <Plus class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
          <span>{{ t('history.splitter.add.bucket') }}</span>
        </button>
      </div>

      <!-- Execution Result -->
      <div
        v-if="result"
        class="p-3 rounded-xl border text-xs flex flex-col gap-1.5"
        :class="result.success ? 'bg-success/10 border-success/30 text-success' : 'bg-warning/10 border-warning/30 text-warning'"
      >
        <div class="flex items-center gap-2 font-semibold">
          <CheckCircle2 v-if="result.success" class="w-4 h-4 text-success" />
          <AlertTriangle v-else class="w-4 h-4 text-warning" />
          <span>{{ result.summary }}</span>
        </div>
      </div>

      <div v-if="errorMessage" class="p-3 rounded-xl border border-danger/30 bg-danger/10 text-xs text-danger">
        {{ errorMessage }}
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" :disabled="isExecuting" @click="emit('close')">
        {{ t('common.close') }}
      </UiButton>
      <UiButton
        variant="primary"
        data-testid="split-commit-execute-btn"
        :disabled="isExecuting || plans.length === 0"
        @click="handleExecuteSplit"
      >
        <RefreshCw v-if="isExecuting" class="w-3.5 h-3.5 animate-spin" />
        <span>{{ isExecuting ? t('history.splitter.executing') : t('history.splitter.execute') }}</span>
      </UiButton>
    </template>
  </UiModal>
</template>
