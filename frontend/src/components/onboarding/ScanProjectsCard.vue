<script setup lang="ts">
import { ref } from "vue";
import { FolderOpen, FolderGit2 } from "lucide-vue-next";
import { t } from "../../i18n";
import { gitService } from "../../services/gitService";
import type { ProjectDetailItem } from "../../types";

/**
 * Scan a directory tree for Git repositories.
 *
 * Owns its own query, results and loading flag. The parent only needs to
 * focus the input (shortcuts jump here) and to hear which result was opened,
 * so it gets one exposed method and one event and keeps no scanning state.
 */
defineProps<{ highlighted?: boolean }>();
const emit = defineEmits<{
  (e: "opened", path: string): void;
  (e: "failed", message: string): void;
  (e: "started"): void;
}>();

const scanRoot = ref("");
const inputRef = ref<HTMLInputElement | null>(null);
const scannedProjects = ref<ProjectDetailItem[]>([]);
const isScanning = ref(false);

async function runScan() {
  if (isScanning.value) return;
  const root = scanRoot.value.trim();
  // Clearing the banner on start matches the behaviour before the split.
  emit("started");
  isScanning.value = true;
  try {
    const paths = await gitService.scanLocalProjects(root);
    scannedProjects.value = (await gitService.getProjectsDetails(paths || [], "")) || [];
    if (scannedProjects.value.length === 0) {
      // Same surface as before the extraction: the welcome page owns the
      // error banner, so failures are reported upward rather than swallowed here.
      emit("failed", t("onboard.error.scan.none", { root: root || "~" }));
    }
  } catch (err: any) {
    emit("failed", t("onboard.error.scan.failed", { error: err?.message || err }));
  } finally {
    isScanning.value = false;
  }
}

/** Called by the parent when a shortcut jumps straight to the scan form. */
async function focusInput() {
  inputRef.value?.scrollIntoView({ block: "center", behavior: "smooth" });
  inputRef.value?.focus();
}

defineExpose({ focusInput });
</script>

<template>
<div
  class="mt-1 rounded-2xl bg-canvas border overflow-hidden transition"
  :class="highlighted ? 'border-accent/60 ring-1 ring-accent/30' : 'border-line'"
>
  <div class="px-3.5 py-2 border-b border-line/70 flex items-center gap-1.5">
    <FolderOpen class="w-3.5 h-3.5 text-ink-muted" />
    <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">{{ t("onboard.scan") }}</span>
  </div>
  <div class="p-2 space-y-2">
    <div class="flex items-center gap-1.5">
      <input
        ref="inputRef"
        v-model="scanRoot"
        type="text"
        :aria-label="t('onboard.scan.placeholder')"
        :placeholder="t('onboard.scan.placeholder')"
        class="flex-1 min-w-0 px-2.5 py-1.5 rounded-lg bg-surface border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
      />
      <button
        @click="runScan"
        :disabled="isScanning"
        class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
      >
        {{ isScanning ? t("onboard.scanning") : t("onboard.scan.run") }}
      </button>
    </div>
    <div v-if="scannedProjects.length > 0" class="max-h-56 overflow-y-auto space-y-1">
      <button
        v-for="project in scannedProjects"
        :key="project.path"
        @click="emit('opened', project.path)"
        class="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-raised transition text-left cursor-pointer"
        :title="project.path"
      >
        <FolderGit2 class="w-3.5 h-3.5 text-ink-muted shrink-0" />
        <span class="text-xs text-ink font-semibold truncate">{{ project.name }}</span>
        <span class="px-1.5 py-0.2 rounded bg-accent-bg text-accent text-[9px] font-bold shrink-0">
          {{ project.engineType }}
        </span>
        <span class="text-[10px] text-ink-muted font-mono truncate">{{ project.currentBranch }}</span>
        <span v-if="project.isDirty" class="text-[10px] text-warning shrink-0">{{ t("onboard.scan.dirty", { count: project.dirtyCount }) }}</span>
        <span
          v-if="project.statusWarning"
          class="text-[10px] font-bold text-warning shrink-0 cursor-help"
          role="status"
          :title="project.statusWarning"
          :aria-label="project.statusWarning"
        >!</span>
        <span class="ml-auto text-[10px] text-ink-muted font-mono truncate max-w-[140px]">{{ project.path }}</span>
      </button>
    </div>
    <div v-else-if="!isScanning" class="py-2 text-center text-[11px] text-ink-muted">
      {{ t("onboard.scan.hint") }}
    </div>
  </div>
</div>
</template>
