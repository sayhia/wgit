<script setup lang="ts">
import { ref, watch } from "vue";
import { ChevronDown } from "lucide-vue-next";
import { t } from "../../i18n";

const props = defineProps<{ text?: string }>();

const open = ref(false);
watch(
  () => props.text,
  (next) => {
    if (!next) open.value = false;
  }
);
</script>

<template>
  <div v-if="text" data-testid="ai-reasoning">
    <button
      type="button"
      class="flex items-center gap-1 text-[10px] text-ink-muted hover:text-ink cursor-pointer"
      :aria-expanded="open"
      data-testid="ai-reasoning-toggle"
      @click="open = !open"
    >
      <ChevronDown class="w-3 h-3 transition-transform" :class="{ 'rotate-180': open }" aria-hidden="true" />
      {{ t("ai.chat.reasoning") }}
    </button>
    <pre
      v-if="open"
      data-testid="ai-reasoning-text"
      class="max-h-32 overflow-y-auto p-2 rounded-xl bg-surface border border-line text-[10px] font-mono text-ink-muted whitespace-pre-wrap select-text mt-1"
    >{{ text }}</pre>
  </div>
</template>
