<script setup lang="ts">
import { ref } from "vue";
import { FileText, Check, Copy, Download, Upload, PackageCheck, RefreshCw } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";

const props = defineProps<{
  open: boolean;
  markdown: string;
  copied: boolean;
  currentRepoPath?: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "copy"): void;
  (e: "notify", msg: string): void;
}>();

const isExportingNotes = ref(false);
const isImportingNotes = ref(false);
const importPathInput = ref("");

async function exportGitNotes() {
  if (!props.currentRepoPath || isExportingNotes.value) return;
  isExportingNotes.value = true;
  try {
    await gitService.exportReviewNotes(props.currentRepoPath, "");
    emit("notify", t("review.notes.export.success", { count: 0, path: "refs/notes/commits" }));
  } catch (err: any) {
    emit("notify", t("review.notes.export.failed", { error: String(err?.message || err) }));
  } finally {
    isExportingNotes.value = false;
  }
}

async function importGitNotes() {
  if (!props.currentRepoPath || !importPathInput.value.trim() || isImportingNotes.value) return;
  isImportingNotes.value = true;
  try {
    const res = await gitService.importReviewNotes(props.currentRepoPath, importPathInput.value.trim());
    emit("notify", t("review.notes.import.success", { count: res.totalImported }));
  } catch (err: any) {
    emit("notify", t("review.notes.import.failed", { error: String(err?.message || err) }));
  } finally {
    isImportingNotes.value = false;
  }
}
</script>

<template>
  <UiModal
    :open="open"
    :title="t('review.export.title')"
    width="max-w-[720px]"
  flush-body
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <div class="flex items-center gap-2 min-w-0">
        <FileText class="w-4 h-4 text-accent shrink-0" />
        <span class="text-xs font-semibold text-ink whitespace-nowrap">{{ t("review.export.title") }}</span>
        <span v-if="copied" class="text-[10px] text-success flex items-center gap-1 whitespace-nowrap">
          <Check class="w-3 h-3" />{{ t("review.export.copied") }}
        </span>
        <UiButton variant="secondary" size="xs" class="ml-2" @click="emit('copy')">
          <Copy class="w-3 h-3" aria-hidden="true" />
          {{ t("review.export.copy") }}
        </UiButton>
      </div>
    </template>

    <div class="space-y-3 p-4">
      <pre class="p-3 text-[11px] leading-5 font-mono text-ink-secondary whitespace-pre-wrap break-words max-h-64 overflow-y-auto bg-canvas border border-line rounded-lg">{{ markdown }}</pre>

      <!-- Git Notes Package Export / Import Section -->
      <div class="p-3 rounded-lg border border-line bg-surface space-y-2 text-xs" data-testid="review-notes-io">
        <div class="flex items-center justify-between">
          <span class="font-bold text-ink flex items-center gap-1.5">
            <PackageCheck class="w-3.5 h-3.5 text-accent" />
            <span>{{ t("review.notes.package.title") }}</span>
          </span>
          <UiButton
            variant="secondary"
            size="xs"
            :disabled="isExportingNotes || !currentRepoPath"
            @click="exportGitNotes"
            class="flex items-center gap-1"
          >
            <RefreshCw v-if="isExportingNotes" class="w-3 h-3 animate-spin" aria-hidden="true" />
            <Download v-else class="w-3 h-3" aria-hidden="true" />
            <span>{{ t("review.notes.export.btn") }}</span>
          </UiButton>
        </div>

        <div class="flex items-center gap-2 pt-1 border-t border-line/60">
          <input
            v-model="importPathInput"
            type="text"
            :placeholder="t('review.notes.import.placeholder')"
            class="flex-1 min-w-0 px-2 py-1 rounded bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <UiButton
            variant="secondary"
            size="xs"
            :disabled="isImportingNotes || !importPathInput.trim() || !currentRepoPath"
            @click="importGitNotes"
            class="flex items-center gap-1 shrink-0"
          >
            <RefreshCw v-if="isImportingNotes" class="w-3 h-3 animate-spin" aria-hidden="true" />
            <Upload v-else class="w-3 h-3" aria-hidden="true" />
            <span>{{ t("review.notes.import.btn") }}</span>
          </UiButton>
        </div>
      </div>
    </div>
  </UiModal>
</template>
