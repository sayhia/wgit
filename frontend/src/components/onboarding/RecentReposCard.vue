<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Clock, FolderGit2 } from "lucide-vue-next";
import { t } from "../../i18n";
import { gitService } from "../../services/gitService";

/**
 * Recently opened repositories.
 *
 * Loads on mount: the list is read once when the welcome page appears, and
 * nothing else in the page depends on it, so it does not need to sit in the
 * parent's state.
 */
const emit = defineEmits<{ (e: "open", path: string): void }>();

const recentRepos = ref<string[]>([]);
const isLoadingRecents = ref(false);

async function load() {
  isLoadingRecents.value = true;
  try {
    const list = await gitService.getRecentRepos();
    if (list) recentRepos.value = list;
  } catch {
    // best-effort: an empty recent list is not worth an error banner
  } finally {
    isLoadingRecents.value = false;
  }
}

onMounted(load);
</script>

<template>
<div class="mt-1 rounded-2xl bg-canvas border border-line overflow-hidden">
  <div class="px-3.5 py-2 border-b border-line/70 flex items-center gap-1.5">
    <Clock class="w-3.5 h-3.5 text-ink-muted" />
    <span class="text-[11px] font-semibold text-ink-muted uppercase tracking-wider">{{ t("onboard.recent") }}</span>
  </div>
  <div class="max-h-40 overflow-y-auto p-1.5 space-y-0.5">
    <button
      v-for="repo in recentRepos"
      :key="repo"
      @click="emit('open', repo)"
      class="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg hover:bg-raised transition text-left cursor-pointer"
    >
      <FolderGit2 class="w-3.5 h-3.5 text-ink-muted shrink-0" />
      <span class="text-xs text-ink truncate flex-1">{{ repo.split(/[\\/]/).pop() || repo }}</span>
      <span class="text-[10px] text-ink-muted font-mono truncate max-w-[140px]">{{ repo }}</span>
    </button>
    <div v-if="!isLoadingRecents && recentRepos.length === 0" class="py-3 text-center text-[11px] text-ink-muted">
      {{ t("onboard.recent.empty") }}
    </div>
  </div>
</div>
</template>
