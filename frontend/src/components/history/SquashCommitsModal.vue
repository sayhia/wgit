<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Layers, AlertTriangle, Check, X, GitCommit } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { CommitItem } from "../../types";

const props = defineProps<{
  isOpen: boolean;
  repoPath: string;
  commits: CommitItem[];
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "squashed"): void;
  (e: "notify", message: string): void;
}>();

const isSubmitting = ref(false);
const commitMessage = ref("");
const errorMessage = ref("");

// Pre-fill combined commit message from selected commits
watch(
  () => props.isOpen,
  (open) => {
    if (open) {
      errorMessage.value = "";
      if (props.commits.length > 0) {
        const subjects = props.commits.map((c) => c.subject).filter(Boolean);
        commitMessage.value = `squash: ${subjects[0] || "combined commits"}\n\n` +
          subjects.map((s, idx) => `* ${s}`).join("\n");
      } else {
        commitMessage.value = "";
      }
    }
  },
  { immediate: true }
);

const commitHashes = computed(() => props.commits.map((c) => c.hash));

async function handleConfirmSquash() {
  if (!props.repoPath || props.commits.length < 2) return;
  const msg = commitMessage.value.trim();
  if (!msg) {
    errorMessage.value = t("history.squash.msg.required");
    return;
  }

  isSubmitting.value = true;
  errorMessage.value = "";

  try {
    const res = await gitService.squashSelectedCommits(props.repoPath, commitHashes.value, msg);
    if (res.success) {
      emit("notify", res.message || t("history.squash.success", { count: res.count }));
      emit("squashed");
      emit("close");
    } else {
      errorMessage.value = res.message || t("history.squash.error");
    }
  } catch (err: any) {
    errorMessage.value = err?.message || String(err);
  } finally {
    isSubmitting.value = false;
  }
}
</script>

<template>
  <UiModal
    :open="isOpen"
    :title="t('history.squash.modal.title')"
    width="max-w-xl"
    data-testid="squash-commits-modal"
    @update:open="emit('close')"
  >
    <div class="space-y-4 text-xs font-sans">
      <!-- Description Banner -->
      <div class="p-3 rounded-xl border border-line bg-surface space-y-1.5">
        <div class="flex items-center gap-2 text-ink font-semibold">
          <Layers class="w-4 h-4 text-warning" />
          <span>{{ t("history.squash.modal.subtitle", { count: commits.length }) }}</span>
        </div>
        <p class="text-[11px] text-ink-muted leading-relaxed">
          {{ t("history.squash.modal.desc") }}
        </p>
      </div>

      <!-- Commits List Preview -->
      <div class="space-y-1.5">
        <label class="font-bold text-ink-secondary text-[11px] uppercase tracking-wide">
          {{ t("history.squash.commits.preview") }}
        </label>
        <div class="max-h-36 overflow-y-auto rounded-lg border border-line bg-canvas divide-y divide-line/30">
          <div
            v-for="commit in commits"
            :key="commit.hash"
            class="px-2.5 py-1.5 flex items-center justify-between gap-2 text-xs"
          >
            <div class="flex items-center gap-2 min-w-0">
              <GitCommit class="w-3.5 h-3.5 text-accent shrink-0" />
              <span class="font-mono text-[10px] text-accent font-bold shrink-0">{{ commit.shortHash }}</span>
              <span class="text-ink truncate">{{ commit.subject }}</span>
            </div>
            <span class="text-[10px] text-ink-muted shrink-0 font-sans">{{ commit.authorName }}</span>
          </div>
        </div>
      </div>

      <!-- Editable Message Input -->
      <div class="space-y-1.5">
        <label class="font-bold text-ink-secondary text-[11px] uppercase tracking-wide">
          {{ t("history.squash.msg.label") }}
        </label>
        <textarea
          v-model="commitMessage"
          rows="5"
          class="w-full p-2.5 rounded-xl border border-line bg-canvas text-ink font-mono text-xs focus:outline-none focus:border-accent transition select-text"
          :placeholder="t('history.squash.msg.placeholder')"
          data-testid="squash-message-input"
        />
      </div>

      <!-- Error Message if any -->
      <div
        v-if="errorMessage"
        class="p-2.5 rounded-lg border border-danger/40 bg-danger-bg text-danger text-[11px] flex items-center gap-2"
      >
        <AlertTriangle class="w-4 h-4 shrink-0" />
        <span class="leading-relaxed">{{ errorMessage }}</span>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end gap-2 w-full">
        <UiButton variant="secondary" :disabled="isSubmitting" @click="emit('close')">
          {{ t("common.cancel") }}
        </UiButton>
        <UiButton
          variant="primary"
          :loading="isSubmitting"
          data-testid="confirm-squash-btn"
          @click="handleConfirmSquash"
        >
          <Layers class="w-3.5 h-3.5 mr-1 text-warning" />
          <span>{{ t("history.squash.confirm.btn") }}</span>
        </UiButton>
      </div>
    </template>
  </UiModal>
</template>
