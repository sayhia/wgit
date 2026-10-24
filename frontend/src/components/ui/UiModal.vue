<script setup lang="ts">
import { X } from "lucide-vue-next";
import { useModalA11y } from "../../composables/useModalA11y";
import { t } from "../../i18n";

const props = withDefaults(
  defineProps<{
    open: boolean;
    /** Visible heading; also the fallback accessible name. */
    title?: string;
    /** Explicit accessible name when the heading is not plain text. */
    label?: string;
    /** Tailwind max-width token, e.g. "max-w-md". */
    width?: string;
    /** Hide the built-in close button when the body owns its own actions. */
    hideClose?: boolean;
    /** Extra classes for the scroll container (spacing, full-bleed bodies). */
    bodyClass?: string;
  }>(),
  { width: "max-w-lg", hideClose: false, bodyClass: "" }
);

const emit = defineEmits<{ (e: "update:open", value: boolean): void }>();

const { containerRef } = useModalA11y(
  () => props.open,
  () => emit("update:open", false)
);
</script>

<template>
  <div
    v-if="open"
    ref="containerRef"
    role="dialog"
    aria-modal="true"
    :aria-label="label || title || t('common.confirm')"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
  >
    <div
      data-glass
      class="w-full max-h-[92vh] rounded-2xl glass glass-panel border flex flex-col overflow-hidden"
      :class="width"
    >
      <div class="flex items-start justify-between gap-3 px-5 py-3.5 border-b border-line shrink-0">
        <div class="min-w-0 flex-1">
          <slot name="header">
            <h3 class="text-sm font-bold text-ink truncate">{{ title }}</h3>
          </slot>
        </div>
        <button
          v-if="!hideClose"
          type="button"
          :aria-label="t('common.close')"
          @click="emit('update:open', false)"
          class="p-1 rounded-lg text-ink-muted hover:text-ink hover:bg-raised transition cursor-pointer shrink-0"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="flex-1 overflow-y-auto px-5 py-4" :class="bodyClass">
        <slot />
      </div>

      <div
        v-if="$slots.footer"
        class="px-5 py-3 border-t border-line flex items-center justify-end gap-2 shrink-0"
      >
        <slot name="footer" />
      </div>
    </div>
  </div>
</template>
