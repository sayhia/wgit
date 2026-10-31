<script setup lang="ts">
import { computed } from "vue";
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
    /** Extra classes for the panel itself (animation, select behavior). */
    panelClass?: string;
    /**
     * "bare" renders the overlay and panel only — no header, footer, padding
     * or scroll container. For search palettes and full-bleed editors that
     * own their entire layout. The focus trap, Esc and overlay stay.
     */
    variant?: "default" | "bare";
    /** Close when the user clicks the dimmed backdrop (mouse only). */
    closeOnBackdrop?: boolean;
    /** "top" keeps search palettes near the top; default centers the panel. */
    align?: "center" | "top";
    /** Overlay tint for full-page and dashboard surfaces. */
    overlay?: "dim" | "canvas" | "canvas-strong";
    /** "plain" drops the glass material for full-page cards. */
    surface?: "glass" | "plain";
  }>(),
  {
    width: "max-w-lg",
    hideClose: false,
    bodyClass: "",
    panelClass: "",
    variant: "default",
    closeOnBackdrop: false,
    align: "center",
    overlay: "dim",
    surface: "glass",
  }
);

const emit = defineEmits<{ (e: "update:open", value: boolean): void }>();

// Kept out of the template literal so the glass guard can see that a plain
// surface never carries both the material and an opaque background utility.
const panelSurfaceClass = computed(() =>
  props.surface === "glass" ? "rounded-2xl glass glass-panel" : "rounded-2xl bg-surface"
);

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
    class="fixed inset-0 z-50 flex justify-center"
    :class="[
      overlay === 'canvas-strong'
        ? 'bg-canvas/95 backdrop-blur-2xl'
        : overlay === 'canvas'
          ? 'bg-canvas/70 backdrop-blur-md'
          : 'bg-black/60 backdrop-blur-sm',
      align === 'top' ? 'items-start px-4 pb-4 pt-24' : 'items-center p-4',
    ]"
    @mousedown.self="closeOnBackdrop && emit('update:open', false)"
  >
    <div
      data-glass
      class="w-full max-h-[92vh] flex flex-col overflow-hidden border"
      :class="[width, panelClass, panelSurfaceClass]"
    >
      <template v-if="variant === 'default'">
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
      </template>

      <slot v-else />
    </div>
  </div>
</template>
