<script setup lang="ts">
import type { Component } from "vue";
import { Inbox } from "lucide-vue-next";

withDefaults(
  defineProps<{
    title: string;
    hint?: string;
    icon?: Component;
    /** Adds a dashed border when the empty state fills a card body. */
    bordered?: boolean;
  }>(),
  { hint: "", icon: () => Inbox, bordered: false }
);
</script>

<template>
  <div
    class="flex flex-col items-center justify-center gap-2 py-8 px-4 text-center rounded-xl"
    :class="bordered ? 'border border-dashed border-line bg-surface/20' : ''"
  >
    <component :is="icon" class="w-7 h-7 text-ink-muted" />
    <p class="text-xs font-semibold text-ink">{{ title }}</p>
    <p v-if="hint" class="text-[11px] text-ink-muted max-w-sm leading-relaxed">{{ hint }}</p>
    <slot />
  </div>
</template>
