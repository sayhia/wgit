<script setup lang="ts">
import { computed } from "vue";
import { t } from "../i18n";
import {
  SHORTCUT_ACTIONS,
  effectiveBinding,
  formatBinding,
  loadShortcutOverrides,
} from "../utils/shortcuts";
import UiModal from "./ui/UiModal.vue";
import { Keyboard, X } from "lucide-vue-next";

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
</script>

<template>
  <UiModal :open="open" @close="emit('close')">
    <div class="p-5 max-w-2xl w-full flex flex-col gap-4 max-h-[85vh]">
      <div class="flex items-center justify-between pb-3 border-b border-line">
        <div class="flex items-center gap-2">
          <Keyboard class="w-5 h-5 text-accent" />
          <h2 class="text-base font-bold text-ink">{{ t("shortcuts.cheatsheet.title") }}</h2>
        </div>
        <button
          @click="emit('close')"
          class="p-1 rounded-md text-ink-muted hover:text-ink hover:bg-raised transition cursor-pointer"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="overflow-y-auto space-y-4 pr-1">
        <!-- Global Actions -->
        <div>
          <h3 class="text-xs font-bold uppercase tracking-wider text-ink-muted mb-2">
            {{ t("shortcuts.cheatsheet.section.global") }}
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
            <div
              v-for="action in SHORTCUT_ACTIONS"
              :key="action.id"
              class="flex items-center justify-between p-2.5 rounded-lg border border-line/60 bg-surface/40 text-xs"
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
        <div>
          <h3 class="text-xs font-bold uppercase tracking-wider text-ink-muted mb-2">
            {{ t("shortcuts.cheatsheet.section.builtin") }}
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
            <div
              v-for="item in fixedShortcuts"
              :key="item.labelKey"
              class="flex items-center justify-between p-2.5 rounded-lg border border-line/60 bg-surface/40 text-xs"
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
