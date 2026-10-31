<script setup lang="ts">
import { ref, computed } from "vue";
import { t } from "../i18n";
import {
  SHORTCUT_ACTIONS,
  effectiveBinding,
  formatBinding,
  loadShortcutOverrides,
} from "../utils/shortcuts";
import UiModal from "./ui/UiModal.vue";
import { Search } from "lucide-vue-next";

defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
}>();

const overrides = computed(() => loadShortcutOverrides());

const fixedShortcuts = [
  { labelKey: "shortcuts.fixed.repo.tabs.label", descKey: "shortcuts.fixed.repo.tabs.desc", binding: "⌘/Ctrl 1..9" },
  { labelKey: "shortcuts.fixed.escape.label", descKey: "shortcuts.fixed.escape.desc", binding: "Esc" },
  { labelKey: "shortcuts.fixed.help.label", descKey: "shortcuts.fixed.help.desc", binding: "⌘/Ctrl / , ?" },
];

const searchQuery = ref("");
const selectedCategory = ref<"all" | "global" | "builtin">("all");

const filteredGlobalActions = computed(() => {
  if (selectedCategory.value === "builtin") return [];
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return SHORTCUT_ACTIONS;
  return SHORTCUT_ACTIONS.filter((action) => {
    const label = t(action.labelKey).toLowerCase();
    const desc = t(action.descriptionKey).toLowerCase();
    const binding = formatBinding(effectiveBinding(action, overrides.value)).toLowerCase();
    return label.includes(q) || desc.includes(q) || binding.includes(q);
  });
});

const filteredFixedShortcuts = computed(() => {
  if (selectedCategory.value === "global") return [];
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return fixedShortcuts;
  return fixedShortcuts.filter((item) => {
    const label = t(item.labelKey).toLowerCase();
    const desc = t(item.descKey).toLowerCase();
    const binding = item.binding.toLowerCase();
    return label.includes(q) || desc.includes(q) || binding.includes(q);
  });
});

const totalMatches = computed(() => filteredGlobalActions.value.length + filteredFixedShortcuts.value.length);
</script>

<template>
  <!-- UiModal only emits update:open (built-in ✕ and Esc both route through
       it); it never emits "close", so the built-in close paths were dead. -->
  <UiModal
    :open="open"
    :title="t('shortcuts.cheatsheet.title')"
  width="max-w-2xl"
    @update:open="(value: boolean) => !value && emit('close')"
  >
    <div class="flex flex-col gap-4">
      <!-- Search & Category Filters (Phase 52) -->
      <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2.5">
        <div class="relative flex-1">
          <Search class="w-4 h-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-muted" aria-hidden="true" />
          <input
            v-model="searchQuery"
            type="text"
            data-testid="shortcut-search-input"
            :aria-label="t('shortcuts.cheatsheet.search.placeholder')"
            :placeholder="t('shortcuts.cheatsheet.search.placeholder')"
            class="w-full pl-8 pr-3 py-1.5 rounded-lg bg-surface border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            v-if="searchQuery"
            type="button"
            @click="searchQuery = ''"
            :aria-label="t('explorer.search.clear')"
            class="absolute right-2.5 top-1/2 -translate-y-1/2 text-ink-muted hover:text-ink text-xs"
          >
            ✕
          </button>
        </div>

        <div class="flex items-center gap-1 shrink-0 bg-surface p-0.5 rounded-lg border border-line">
          <button
            type="button"
            data-testid="shortcut-category-pill"
            @click="selectedCategory = 'all'"
            class="px-2.5 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="selectedCategory === 'all' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
          >
            {{ t("shortcuts.cheatsheet.category.all") }}
          </button>
          <button
            type="button"
            data-testid="shortcut-category-pill"
            @click="selectedCategory = 'global'"
            class="px-2.5 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="selectedCategory === 'global' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
          >
            {{ t("shortcuts.cheatsheet.section.global") }}
          </button>
          <button
            type="button"
            data-testid="shortcut-category-pill"
            @click="selectedCategory = 'builtin'"
            class="px-2.5 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="selectedCategory === 'builtin' ? 'bg-raised text-accent shadow-xs' : 'text-ink-secondary hover:text-ink'"
          >
            {{ t("shortcuts.cheatsheet.section.builtin") }}
          </button>
        </div>
      </div>

      <div class="space-y-4">
        <!-- Empty State -->
        <div
          v-if="totalMatches === 0"
          data-testid="shortcut-empty-state"
          class="py-12 text-center text-xs text-ink-muted"
        >
          {{ t("shortcuts.cheatsheet.empty") }}
        </div>

        <!-- Global Actions -->
        <div v-if="filteredGlobalActions.length > 0">
          <h3 class="text-xs font-bold uppercase tracking-wider text-ink-muted mb-2">
            {{ t("shortcuts.cheatsheet.section.global") }}
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
            <div
              v-for="action in filteredGlobalActions"
              :key="action.id"
              class="flex items-center justify-between p-2.5 rounded-lg border border-line/60 bg-surface text-xs"
            >
              <div class="flex flex-col min-w-0 pr-2">
                <span class="font-semibold text-ink truncate">{{ t(action.labelKey) }}</span>
                <span class="text-[10px] text-ink-muted truncate">{{ t(action.descriptionKey) }}</span>
              </div>
              <kbd class="px-2 py-1 rounded bg-canvas border border-line font-mono text-[11px] font-bold text-accent shrink-0 shadow-xs">
                {{ formatBinding(effectiveBinding(action, overrides)) }}
              </kbd>
            </div>
          </div>
        </div>

        <!-- Fixed / Built-in Navigation Shortcuts -->
        <div v-if="filteredFixedShortcuts.length > 0">
          <h3 class="text-xs font-bold uppercase tracking-wider text-ink-muted mb-2">
            {{ t("shortcuts.cheatsheet.section.builtin") }}
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
            <div
              v-for="item in filteredFixedShortcuts"
              :key="item.labelKey"
              class="flex items-center justify-between p-2.5 rounded-lg border border-line/60 bg-surface text-xs"
            >
              <div class="flex flex-col min-w-0 pr-2">
                <span class="font-semibold text-ink truncate">{{ t(item.labelKey) }}</span>
                <span class="text-[10px] text-ink-muted truncate">{{ t(item.descKey) }}</span>
              </div>
              <kbd class="px-2 py-1 rounded bg-canvas border border-line font-mono text-[11px] font-bold text-accent shrink-0 shadow-xs">
                {{ item.binding }}
              </kbd>
            </div>
          </div>
        </div>
      </div>
    </div>
  </UiModal>
</template>
