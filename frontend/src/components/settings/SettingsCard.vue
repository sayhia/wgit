<script setup lang="ts">
withDefaults(defineProps<{ padded?: boolean }>(), { padded: true });
</script>

<template>
  <section data-settings-card class="min-w-0 rounded-[var(--ug-radius-xl)] border border-line bg-surface">
    <div v-if="$slots.header || $slots.actions" class="flex flex-wrap items-start justify-between gap-3 border-b border-line-subtle px-4 py-3">
      <div class="min-w-0 flex-1"><slot name="header" /></div>
      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2 max-w-full"><slot name="actions" /></div>
    </div>
    <div :class="padded === false ? '' : 'p-4'"><slot /></div>
    <div v-if="$slots.footer" class="flex flex-wrap items-center justify-end gap-2 border-t border-line-subtle px-4 py-3"><slot name="footer" /></div>
  </section>
</template>
