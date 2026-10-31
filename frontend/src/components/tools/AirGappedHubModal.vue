<script setup lang="ts">
import { ref, watch } from "vue";
import {
  Radio,
  Download,
  Upload,
  ShieldCheck,
  CheckCircle2,
  AlertTriangle,
  FileBox,
  RefreshCw,
  FileCode,
  Tag,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { DeltaBundleManifest, DeltaBundleResult, DeltaApplyResult } from "../../types";

const props = defineProps<{
  open: boolean;
  repoPath: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "notify", msg: string): void;
}>();

const activeTab = ref<"export" | "import">("export");

// Export state
const exportBaseRef = ref("HEAD~1");
const exportTargetRef = ref("HEAD");
const exportOutputPath = ref("");
const isExporting = ref(false);
const exportResult = ref<DeltaBundleResult | null>(null);
const exportError = ref<string | null>(null);

// Import state
const importFilePath = ref("");
const isInspecting = ref(false);
const manifest = ref<DeltaBundleManifest | null>(null);
const inspectError = ref<string | null>(null);
const importTargetBranch = ref("");
const isApplying = ref(false);
const applyResult = ref<DeltaApplyResult | null>(null);
const applyError = ref<string | null>(null);

async function handleExport() {
  if (!props.repoPath) return;
  isExporting.value = true;
  exportError.value = null;
  exportResult.value = null;
  try {
    const res = await gitService.createDeltaBundle(
      props.repoPath,
      exportBaseRef.value,
      exportTargetRef.value,
      exportOutputPath.value
    );
    exportResult.value = res;
    if (res.success) {
      emit(
        "notify",
        t("air_gapped.export.success", {
          count: res.commitCount,
          size: `${res.totalSize} B`,
        })
      );
    } else {
      exportError.value = res.message;
    }
  } catch (err: unknown) {
    exportError.value = err instanceof Error ? err.message : String(err);
  } finally {
    isExporting.value = false;
  }
}

async function handleInspect() {
  if (!props.repoPath || !importFilePath.value) return;
  isInspecting.value = true;
  inspectError.value = null;
  manifest.value = null;
  applyResult.value = null;
  applyError.value = null;
  try {
    const res = await gitService.inspectDeltaBundle(props.repoPath, importFilePath.value);
    manifest.value = res;
    if (!importTargetBranch.value && res.targetRef && res.targetRef !== "auto") {
      importTargetBranch.value = res.targetRef.replace(/^refs\/heads\//, "");
    }
  } catch (err: unknown) {
    inspectError.value = t("air_gapped.import.invalid", {
      error: err instanceof Error ? err.message : String(err),
    });
  } finally {
    isInspecting.value = false;
  }
}

async function handleApply() {
  if (!props.repoPath || !importFilePath.value) return;
  isApplying.value = true;
  applyError.value = null;
  applyResult.value = null;
  try {
    const res = await gitService.applyDeltaBundle(
      props.repoPath,
      importFilePath.value,
      importTargetBranch.value
    );
    applyResult.value = res;
    if (res.success) {
      emit("notify", t("air_gapped.import.success", { branch: res.targetBranch }));
    } else {
      applyError.value = res.message;
    }
  } catch (err: unknown) {
    applyError.value = err instanceof Error ? err.message : String(err);
  } finally {
    isApplying.value = false;
  }
}

watch(
  () => props.open,
  (val) => {
    if (val) {
      exportResult.value = null;
      exportError.value = null;
      applyResult.value = null;
      applyError.value = null;
    }
  }
);
</script>

<template>
  <UiModal
    :open="open"
    :title="t('air_gapped.title')"
    width="max-w-2xl"
    @update:open="emit('update:open', $event)"
  >
    <div class="space-y-4 text-xs text-neutral-300">
      <!-- Subtitle -->
      <div class="pb-2 border-b border-neutral-700/60">
        <p class="text-neutral-400">
          {{ t('air_gapped.subtitle') }}
        </p>
      </div>

      <!-- Tab Switcher -->
      <div class="flex items-center gap-2 border-b border-neutral-700/60 pb-2">
        <button
          type="button"
          data-testid="air-gapped-tab-export"
          class="flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition cursor-pointer"
          :class="
            activeTab === 'export'
              ? 'bg-indigo-600 text-white font-semibold'
              : 'bg-neutral-800 text-neutral-400 hover:text-neutral-200'
          "
          @click="activeTab = 'export'"
        >
          <Upload class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ t('air_gapped.tab.export') }}</span>
        </button>
        <button
          type="button"
          data-testid="air-gapped-tab-import"
          class="flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition cursor-pointer"
          :class="
            activeTab === 'import'
              ? 'bg-indigo-600 text-white font-semibold'
              : 'bg-neutral-800 text-neutral-400 hover:text-neutral-200'
          "
          @click="activeTab = 'import'"
        >
          <Download class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ t('air_gapped.tab.import') }}</span>
        </button>
      </div>

      <!-- Export Panel -->
      <div v-if="activeTab === 'export'" class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-neutral-400 mb-1 font-medium">
              {{ t('air_gapped.export.base_ref') }}
            </label>
            <input
              v-model="exportBaseRef"
              type="text"
              class="w-full bg-neutral-900 border border-neutral-700 rounded px-2.5 py-1.5 text-xs text-neutral-100 font-mono focus:outline-none focus:border-indigo-500"
            />
          </div>
          <div>
            <label class="block text-neutral-400 mb-1 font-medium">
              {{ t('air_gapped.export.target_ref') }}
            </label>
            <input
              v-model="exportTargetRef"
              type="text"
              class="w-full bg-neutral-900 border border-neutral-700 rounded px-2.5 py-1.5 text-xs text-neutral-100 font-mono focus:outline-none focus:border-indigo-500"
            />
          </div>
        </div>

        <div>
          <label class="block text-neutral-400 mb-1 font-medium">
            {{ t('air_gapped.export.output_path') }}
          </label>
          <input
            v-model="exportOutputPath"
            type="text"
            :placeholder="t('air_gapped.export.output_placeholder')"
            class="w-full bg-neutral-900 border border-neutral-700 rounded px-2.5 py-1.5 text-xs text-neutral-100 font-mono focus:outline-none focus:border-indigo-500"
          />
        </div>

        <div v-if="exportError" class="p-2.5 bg-red-950/40 border border-red-500/30 rounded-lg text-red-300 flex items-center gap-2">
          <AlertTriangle class="w-4 h-4 text-red-400 shrink-0" />
          <span>{{ exportError }}</span>
        </div>

        <div v-if="exportResult && exportResult.success" class="p-3 bg-emerald-950/30 border border-emerald-500/30 rounded-lg space-y-1">
          <div class="text-emerald-300 font-semibold flex items-center gap-1.5">
            <CheckCircle2 class="w-4 h-4 text-emerald-400" />
            <span>
              {{ t('air_gapped.export.success', { count: exportResult.commitCount, size: `${exportResult.totalSize} B` }) }}
            </span>
          </div>
          <p class="font-mono text-neutral-400 text-[11px] truncate">
            {{ exportResult.bundlePath }}
          </p>
        </div>

        <div class="flex justify-end pt-2">
          <UiButton
            size="sm"
            variant="primary"
            :disabled="isExporting"
            data-testid="create-delta-bundle-btn"
            class="flex items-center gap-1.5"
            @click="handleExport"
          >
            <Upload class="w-3.5 h-3.5" :class="{ 'animate-bounce': isExporting }" aria-hidden="true" />
            <span>
              {{ isExporting ? t('air_gapped.export.generating') : t('air_gapped.export.btn') }}
            </span>
          </UiButton>
        </div>
      </div>

      <!-- Import Panel -->
      <div v-else class="space-y-3">
        <div>
          <label class="block text-neutral-400 mb-1 font-medium">
            {{ t('air_gapped.import.file_path') }}
          </label>
          <div class="flex gap-2">
            <input
              v-model="importFilePath"
              type="text"
              :placeholder="t('air_gapped.import.file_placeholder')"
              class="flex-1 bg-neutral-900 border border-neutral-700 rounded px-2.5 py-1.5 text-xs text-neutral-100 font-mono focus:outline-none focus:border-indigo-500"
            />
            <UiButton
              size="sm"
              variant="secondary"
              :disabled="!importFilePath || isInspecting"
              data-testid="inspect-delta-bundle-btn"
              class="shrink-0 flex items-center gap-1"
              @click="handleInspect"
            >
              <ShieldCheck class="w-3.5 h-3.5" aria-hidden="true" />
              <span>{{ t('air_gapped.import.btn.inspect') }}</span>
            </UiButton>
          </div>
        </div>

        <div v-if="inspectError" class="p-2.5 bg-red-950/40 border border-red-500/30 rounded-lg text-red-300 flex items-center gap-2">
          <AlertTriangle class="w-4 h-4 text-red-400 shrink-0" />
          <span>{{ inspectError }}</span>
        </div>

        <!-- Manifest Inspection Details -->
        <div v-if="manifest" class="p-3 bg-neutral-900/80 border border-neutral-700 rounded-lg space-y-2">
          <div class="grid grid-cols-3 gap-2 pb-2 border-b border-neutral-800 text-[11px]">
            <div>
              <span class="text-neutral-500 block">{{ t('air_gapped.import.commit_count') }}</span>
              <span class="font-semibold text-neutral-200">{{ manifest.commitCount }}</span>
            </div>
            <div>
              <span class="text-neutral-500 block">{{ t('air_gapped.import.created_at') }}</span>
              <span class="text-neutral-300">{{ manifest.createdAt }}</span>
            </div>
            <div>
              <span class="text-neutral-500 block">{{ t('air_gapped.import.checksum') }}</span>
              <span class="font-mono text-[10px] text-emerald-400 truncate block">
                {{ manifest.sha256Checksum ? manifest.sha256Checksum.slice(0, 12) + '...' : 'N/A' }}
              </span>
            </div>
          </div>

          <!-- Target Branch Selection -->
          <div class="pt-1">
            <label class="block text-neutral-400 mb-1 font-medium">
              {{ t('air_gapped.import.target_branch') }}
            </label>
            <input
              v-model="importTargetBranch"
              type="text"
              :placeholder="t('air_gapped.import.target_placeholder')"
              class="w-full bg-neutral-800 border border-neutral-700 rounded px-2.5 py-1.5 text-xs text-neutral-100 font-mono focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div v-if="applyError" class="p-2 bg-red-950/40 border border-red-500/30 rounded text-red-300 text-xs">
            {{ applyError }}
          </div>

          <div v-if="applyResult && applyResult.success" class="p-2 bg-emerald-950/40 border border-emerald-500/30 rounded text-emerald-300 text-xs flex items-center gap-1.5">
            <CheckCircle2 class="w-4 h-4 text-emerald-400" />
            <span>{{ t('air_gapped.import.success', { branch: applyResult.targetBranch }) }}</span>
          </div>

          <div class="flex justify-end pt-2">
            <UiButton
              size="sm"
              variant="primary"
              :disabled="isApplying"
              data-testid="apply-delta-bundle-btn"
              class="flex items-center gap-1.5"
              @click="handleApply"
            >
              <Download class="w-3.5 h-3.5" :class="{ 'animate-pulse': isApplying }" aria-hidden="true" />
              <span>
                {{ isApplying ? t('air_gapped.import.applying') : t('air_gapped.import.btn.apply') }}
              </span>
            </UiButton>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex justify-end pt-3 border-t border-neutral-800">
        <UiButton
          size="sm"
          variant="secondary"
          @click="emit('update:open', false)"
        >
          {{ t('common.cancel') }}
        </UiButton>
      </div>
    </div>
  </UiModal>
</template>
