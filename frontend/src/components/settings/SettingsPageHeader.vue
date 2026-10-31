<script setup lang="ts">
defineProps<{ title: string; description?: string }>();
</script>

<template>
  <header data-settings-page-header class="flex flex-wrap items-start justify-between gap-x-6 gap-y-3 border-b border-line-subtle pb-4 mb-5">
    <div class="min-w-0 flex-1 basis-48">
      <h2 data-testid="settings-page-title" class="text-lg leading-6 font-semibold tracking-tight text-ink break-words">{{ title }}</h2>
      <p v-if="description" class="mt-1.5 max-w-[65ch] text-xs text-ink-muted leading-5">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2 shrink-0 max-w-full">
      <slot name="actions" />
    </div>
  </header>
</template>
