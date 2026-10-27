<script setup lang="ts">
import { ref, computed, watch, nextTick } from "vue";
import { Send, RefreshCw, Layers } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import type { RepoStatus, FileStatus } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  open: boolean;
  repoPath: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "committed"): void;
  (e: "notify", msg: string): void;
}>();

const subject = ref("");
const body = ref("");
const showBody = ref(false);
const isCommitting = ref(false);
const status = ref<RepoStatus | null>(null);
const isLoadingStatus = ref(false);
const subjectInputRef = ref<HTMLInputElement | null>(null);

const stagedFiles = computed<FileStatus[]>(() => status.value?.stagedFiles || []);
const stagedCount = computed(() => stagedFiles.value.length);

async function refreshStatus() {
  if (!props.repoPath) return;
  isLoadingStatus.value = true;
  try {
    const res = await gitService.getRepoStatus(props.repoPath);
    status.value = res;
  } catch {
    /* ignore */
  } finally {
    isLoadingStatus.value = false;
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      subject.value = "";
      body.value = "";
      showBody.value = false;
      void refreshStatus();
      void nextTick(() => {
        subjectInputRef.value?.focus();
      });
    }
  }
);

async function handleStageAll() {
  if (!props.repoPath) return;
  try {
    await gitService.stageAll(props.repoPath);
    emit("notify", t("ws.hud.toast.staged.all"));
    await refreshStatus();
  } catch (err: any) {
    emit("notify", t("ws.hud.toast.failed", { error: err?.message || err }));
  }
}

async function handleCommit() {
  if (!props.repoPath || !subject.value.trim() || isCommitting.value) return;
  if (stagedCount.value === 0) {
    emit("notify", t("ws.hud.toast.no.staged"));
    return;
  }
  isCommitting.value = true;
  try {
    const fullMsg = body.value.trim()
      ? `${subject.value.trim()}\n\n${body.value.trim()}`
      : subject.value.trim();
    await gitService.commitAdvanced(
      props.repoPath,
      fullMsg,
      false,
      false
    );
    emit("notify", t("ws.hud.toast.success"));
    emit("committed");
    emit("update:open", false);
  } catch (err: any) {
    emit("notify", t("ws.hud.toast.failed", { error: err?.message || err }));
  } finally {
    isCommitting.value = false;
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === "Enter" && (e.metaKey || e.ctrlKey || !showBody.value)) {
    e.preventDefault();
    void handleCommit();
  }
}
</script>

<template>
  <UiModal
    :open="open"
    :title="t('ws.hud.title')"
    width="max-w-[480px]"
    @close="emit('update:open', false)"
  >
    <div class="space-y-3.5">
      <!-- Staged Status Bar -->
      <div class="p-2.5 rounded-lg bg-surface border border-line flex items-center justify-between gap-2 text-xs">
        <div class="flex items-center gap-2 min-w-0">
          <Layers class="w-4 h-4 text-accent shrink-0" />
          <span class="text-ink font-semibold truncate">
            {{ t("ws.hud.staged.count", { count: stagedCount }) }}
          </span>
          <span
            v-if="stagedCount > 0"
            class="px-1.5 py-0.2 rounded text-[10px] font-mono bg-success/15 text-success font-bold"
          >
            {{ stagedCount }}
          </span>
        </div>
        <button
          v-if="stagedCount === 0"
          type="button"
          @click="handleStageAll"
          class="px-2 py-1 rounded bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast font-medium text-[10px] transition cursor-pointer"
        >
          {{ t("ws.hud.stage.all") }}
        </button>
      </div>

      <!-- Quick Staged Previews -->
      <div v-if="stagedCount > 0" class="flex flex-wrap gap-1 max-h-20 overflow-y-auto">
        <span
          v-for="f in stagedFiles.slice(0, 5)"
          :key="f.path"
          class="inline-block px-1.5 py-0.5 rounded text-[10px] font-mono bg-canvas border border-line text-ink-muted truncate max-w-[200px]"
          :title="f.path"
        >
          {{ f.path.split("/").pop() }}
        </span>
        <span v-if="stagedCount > 5" class="text-[10px] text-ink-muted self-center">
          +{{ stagedCount - 5 }}
        </span>
      </div>

      <!-- Subject Input -->
      <div class="space-y-1">
        <input
          ref="subjectInputRef"
          v-model="subject"
          type="text"
          :placeholder="t('ws.hud.subject.placeholder')"
          @keydown="handleKeydown"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans"
        />
      </div>

      <!-- Body Expander -->
      <div class="space-y-1">
        <div class="flex items-center justify-between">
          <button
            type="button"
            @click="showBody = !showBody"
            class="text-[10px] text-accent hover:underline cursor-pointer"
          >
            {{ showBody ? t("ws.hud.hide.body") : t("ws.hud.add.body") }}
          </button>
        </div>
        <textarea
          v-if="showBody"
          v-model="body"
          rows="3"
          :placeholder="t('ws.hud.body.placeholder')"
          @keydown="handleKeydown"
          class="w-full p-2.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent resize-none"
        />
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-between gap-2">
        <span class="text-[10px] text-ink-muted">
          {{ t("ws.hud.shortcut.hint") }}
        </span>
        <div class="flex items-center gap-2">
          <UiButton variant="secondary" size="sm" @click="emit('update:open', false)">
            {{ t("common.cancel") }}
          </UiButton>
          <UiButton
            variant="primary"
            size="sm"
            :disabled="!subject.trim() || isCommitting || stagedCount === 0"
            @click="handleCommit"
            class="flex items-center gap-1.5"
          >
            <RefreshCw v-if="isCommitting" class="w-3.5 h-3.5 animate-spin" />
            <Send v-else class="w-3.5 h-3.5" />
            <span>{{ isCommitting ? t("ws.hud.committing") : t("ws.hud.commit") }}</span>
          </UiButton>
        </div>
      </div>
    </template>
  </UiModal>
</template>
