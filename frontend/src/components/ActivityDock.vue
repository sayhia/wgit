<script setup lang="ts">
import { computed } from "vue";
import { t } from "../i18n";
import {
  FileDiff,
  GitGraph,
  GitBranch,
  Archive,
  Wrench,
  Settings,
  ClipboardCheck,
} from "lucide-vue-next";

defineProps<{
  activeTab: string;
  hasDirtyFiles?: boolean;
}>();

const emit = defineEmits<{
  (e: "select-tab", tab: string): void;
}>();

const tabs = computed(() => [
  { id: "workspace", label: t("nav.workspace"), icon: FileDiff },
  { id: "history", label: t("nav.history"), icon: GitGraph },
  { id: "branches", label: t("nav.branches"), icon: GitBranch },
  { id: "stashes", label: t("nav.stashes"), icon: Archive },
  { id: "review", label: t("nav.review"), icon: ClipboardCheck },
  { id: "tools", label: t("nav.tools"), icon: Wrench },
]);
</script>

<template>
  <aside data-glass class="w-13 border-r border-line glass glass-bar flex flex-col items-center justify-between py-3 shrink-0 z-20 select-none">
    <!-- Top Main Navigation -->
    <div class="flex flex-col items-center gap-2">
      <!-- App Brand Mark -->
      <img
        src="/brand/wgit-icon.svg"
        alt="wgit"
        width="32"
        height="32"
        :draggable="false"
        class="w-8 h-8 mb-2 shrink-0 select-none"
      />

      <!-- Navigation Icons -->
      <button
        v-for="tab in tabs"
        :key="tab.id"
        @click="emit('select-tab', tab.id)"
        class="relative w-9 h-9 rounded-xl flex items-center justify-center transition duration-150 group cursor-pointer"
        :class="
          activeTab === tab.id
            ? 'bg-[var(--glass-raise-strong)] text-accent shadow-sm'
            : 'text-ink-muted hover:text-ink hover:bg-[var(--glass-raise)]'
        "
        :title="tab.label"
        :aria-label="tab.label"
        :aria-current="activeTab === tab.id ? 'page' : undefined"
      >
        <component :is="tab.icon" class="w-4.5 h-4.5 transition-transform duration-150 group-hover:scale-105" />
        
        <!-- Active indicator pill on left -->
        <span
          v-if="activeTab === tab.id"
          class="absolute -left-1.5 w-1 h-4.5 rounded-r bg-accent-strong"
        ></span>

        <!-- Dirty files badge indicator -->
        <span
          v-if="tab.id === 'workspace' && hasDirtyFiles"
          class="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-warning-strong border-2 border-surface"
        ></span>
      </button>
    </div>

    <!-- Bottom Settings & Utilities -->
    <div class="flex flex-col items-center gap-1.5">
      <button
        @click="emit('select-tab', 'settings')"
        class="w-9 h-9 rounded-xl flex items-center justify-center text-ink-muted hover:text-ink hover:bg-raised/50 transition cursor-pointer"
        :class="{ 'bg-raised text-accent': activeTab === 'settings' }"
        :title="t('nav.settings')"
        :aria-label="t('nav.settings')"
        :aria-current="activeTab === 'settings' ? 'page' : undefined"
      >
        <Settings class="w-4 h-4" />
      </button>
    </div>
  </aside>
</template>
