<script setup lang="ts">
import { ref, watch } from "vue";
import {
  Package,
  Check,
  Copy,
  Loader2,
  FolderArchive,
  CheckCircle2,
  AlertCircle,
  ShieldCheck,
  Terminal,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import type { GitBundleResult } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  open: boolean;
  currentRepoPath: string;
  defaultRev?: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
}>();

const revSpec = ref("HEAD");
const outputPath = ref("");
const isExporting = ref(false);
const exportResult = ref<GitBundleResult | null>(null);
const errorMessage = ref("");
const isVerifying = ref(false);
const verifySuccess = ref(false);
const copiedField = ref<string | null>(null);

watch(
  () => props.open,
  (val) => {
    if (val) {
      if (props.defaultRev) revSpec.value = props.defaultRev;
      errorMessage.value = "";
      verifySuccess.value = false;
    }
  }
);

async function copyText(text: string, field: string) {
  try {
    await navigator.clipboard.writeText(text);
    copiedField.value = field;
    setTimeout(() => {
      if (copiedField.value === field) copiedField.value = null;
    }, 2000);
  } catch {
    /* ignore clipboard error */
  }
}

async function handleExport() {
  if (!props.currentRepoPath || isExporting.value) return;
  isExporting.value = true;
  errorMessage.value = "";
  verifySuccess.value = false;

  try {
    const res = await gitService.createGitBundleAdvanced(
      props.currentRepoPath,
      revSpec.value.trim() || "HEAD",
      outputPath.value.trim()
    );
    exportResult.value = res;
  } catch (err: any) {
    errorMessage.value = err?.message || String(err);
  } finally {
    isExporting.value = false;
  }
}

async function handleVerify() {
  if (!props.currentRepoPath || !exportResult.value?.outputPath || isVerifying.value) return;
  isVerifying.value = true;
  errorMessage.value = "";

  try {
    await gitService.verifyGitBundle(props.currentRepoPath, exportResult.value.outputPath);
    verifySuccess.value = true;
  } catch (err: any) {
    errorMessage.value = err?.message || String(err);
  } finally {
    isVerifying.value = false;
  }
}
</script>

<template>
  <UiModal
    :open="open"
    :title="t('bundle.modal.title')"
    width="max-w-[620px]"
    @update:open="emit('update:open', $event)"
  >
    <div class="space-y-4">
      <!-- Input Controls -->
      <div class="space-y-3">
        <div>
          <label class="block text-xs font-semibold text-ink mb-1">
            {{ t("bundle.modal.rev.label") }}
          </label>
          <div class="flex items-center gap-2">
            <input
              v-model="revSpec"
              type="text"
              :placeholder="t('bundle.modal.rev.hint')"
              class="flex-1 bg-canvas border border-line rounded-lg px-3 py-1.5 text-xs text-ink focus:outline-none focus:border-accent"
            />
            <div class="flex items-center gap-1">
              <button
                type="button"
                @click="revSpec = 'HEAD'"
                class="px-2 py-1 rounded bg-surface hover:bg-raised text-[11px] font-mono border border-line text-ink-secondary hover:text-ink cursor-pointer"
              >
                HEAD
              </button>
              <button
                type="button"
                @click="revSpec = '--all'"
                class="px-2 py-1 rounded bg-surface hover:bg-raised text-[11px] font-mono border border-line text-ink-secondary hover:text-ink cursor-pointer"
              >
                --all
              </button>
            </div>
          </div>
          <span class="text-[10px] text-ink-muted mt-1 block">
            {{ t("bundle.modal.rev.hint") }}
          </span>
        </div>

        <div>
          <label class="block text-xs font-semibold text-ink mb-1">
            {{ t("bundle.modal.output.label") }}
          </label>
          <input
            v-model="outputPath"
            type="text"
            :placeholder="t('bundle.modal.output.placeholder')"
            class="w-full bg-canvas border border-line rounded-lg px-3 py-1.5 text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <UiButton
          variant="primary"
          class="w-full justify-center"
          :disabled="isExporting"
          @click="handleExport"
        >
          <Loader2 v-if="isExporting" class="w-4 h-4 animate-spin mr-1.5" />
          <FolderArchive v-else class="w-4 h-4 mr-1.5" />
          <span>{{ isExporting ? t("bundle.modal.exporting") : t("bundle.modal.export") }}</span>
        </UiButton>
      </div>

      <!-- Error banner -->
      <div
        v-if="errorMessage"
        class="p-3 rounded-xl border border-danger/40 bg-danger/10 text-xs text-danger flex items-start gap-2"
      >
        <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
        <span class="break-words">{{ errorMessage }}</span>
      </div>

      <!-- Result Success Card -->
      <div
        v-if="exportResult"
        class="p-4 rounded-xl border border-line bg-canvas/70 space-y-3"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2 text-success font-semibold text-xs">
            <CheckCircle2 class="w-4 h-4" />
            <span>{{ t("bundle.modal.success") }}</span>
          </div>
          <span class="text-xs font-mono font-bold text-accent bg-accent/10 px-2 py-0.5 rounded border border-accent/20">
            {{ exportResult.sizeHuman }}
          </span>
        </div>

        <div class="p-2.5 rounded-lg bg-surface/60 border border-line flex items-center justify-between text-xs font-mono break-all text-ink">
          <span class="truncate mr-2">{{ exportResult.outputPath }}</span>
          <button
            @click="copyText(exportResult.outputPath, 'path')"
            class="p-1 rounded text-ink-muted hover:text-ink cursor-pointer"
          >
            <Check v-if="copiedField === 'path'" class="w-3.5 h-3.5 text-success" />
            <Copy v-else class="w-3.5 h-3.5" />
          </button>
        </div>

        <!-- CLI Instructions -->
        <div class="space-y-2 pt-1">
          <div class="flex items-center gap-1.5 text-xs font-semibold text-ink-muted">
            <Terminal class="w-3.5 h-3.5" />
            <span>{{ t("bundle.modal.instructions") }}</span>
          </div>

          <!-- Clone Instruction -->
          <div class="space-y-1">
            <span class="text-[10px] text-ink-muted">{{ t("bundle.modal.instructions.clone") }}</span>
            <div class="p-2 rounded bg-surface/80 border border-line flex items-center justify-between text-[11px] font-mono text-ink">
              <span class="truncate mr-2">{{ exportResult.cloneHelp }}</span>
              <button
                @click="copyText(exportResult.cloneHelp, 'clone')"
                class="p-1 rounded text-ink-muted hover:text-ink cursor-pointer shrink-0"
              >
                <Check v-if="copiedField === 'clone'" class="w-3 h-3 text-success" />
                <Copy v-else class="w-3 h-3" />
              </button>
            </div>
          </div>

          <!-- Verify Instruction -->
          <div class="space-y-1">
            <span class="text-[10px] text-ink-muted">{{ t("bundle.modal.instructions.verify") }}</span>
            <div class="p-2 rounded bg-surface/80 border border-line flex items-center justify-between text-[11px] font-mono text-ink">
              <span class="truncate mr-2">{{ exportResult.verifyHelp }}</span>
              <button
                @click="copyText(exportResult.verifyHelp, 'verify')"
                class="p-1 rounded text-ink-muted hover:text-ink cursor-pointer shrink-0"
              >
                <Check v-if="copiedField === 'verify'" class="w-3 h-3 text-success" />
                <Copy v-else class="w-3 h-3" />
              </button>
            </div>
          </div>

          <!-- Local Verify Action Button -->
          <div class="pt-2 flex items-center justify-between">
            <button
              @click="handleVerify"
              :disabled="isVerifying"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-line bg-surface hover:bg-raised text-xs font-medium text-ink transition cursor-pointer disabled:opacity-40"
            >
              <Loader2 v-if="isVerifying" class="w-3.5 h-3.5 animate-spin" />
              <ShieldCheck v-else class="w-3.5 h-3.5 text-accent" />
              <span>{{ isVerifying ? t("bundle.modal.verifying") : t("bundle.modal.verify.btn") }}</span>
            </button>
            <span v-if="verifySuccess" class="text-xs text-success flex items-center gap-1 font-medium">
              <Check class="w-3.5 h-3.5" />
              {{ t("bundle.modal.verified") }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </UiModal>
</template>
