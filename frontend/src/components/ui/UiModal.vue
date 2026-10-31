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
    /**
     * Full-bleed body. The default body owns `px-5 py-4`; a caller that wants
     * its own padding must not fight it with negative margins (which widened
     * the scroll box past the panel and got clipped). With `flushBody` the
     * padding is genuinely absent.
     */
    flushBody?: boolean;
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
    flushBody: false,
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
  props.surface === "glass"
    ? "rounded-[var(--ug-radius-xl)] glass glass-panel"
    : "rounded-[var(--ug-radius-xl)] border-line bg-surface shadow-[var(--ug-shadow-modal)]"
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
    style="--ui-modal-bottom-inset: max(1rem, var(--ug-toast-reserve, 0px)); padding-bottom: var(--ui-modal-bottom-inset)"
    :class="[
      overlay === 'canvas-strong'
        ? 'bg-canvas/95'
        : overlay === 'canvas'
          ? 'bg-canvas/80'
          : 'bg-[var(--ug-bg-scrim)]',
      align === 'top' ? 'items-start px-4 pb-4 pt-24' : 'items-center p-4',
    ]"
    @mousedown.self="closeOnBackdrop && emit('update:open', false)"
  >
    <div
      data-glass
      class="w-full min-h-0 flex flex-col overflow-hidden border"
      :class="[
        align === 'top'
          ? 'max-h-[calc(100dvh-6rem-var(--ui-modal-bottom-inset))]'
          : 'max-h-[min(92dvh,calc(100dvh-1rem-var(--ui-modal-bottom-inset)))]',
        width,
        panelClass,
        panelSurfaceClass,
      ]"
    >
      <template v-if="variant === 'default'">
        <div class="flex items-start justify-between gap-3 px-5 py-3.5 border-b border-line-subtle shrink-0">
          <div class="min-w-0 flex-1">
            <slot name="header">
              <h3 class="text-sm font-semibold text-ink truncate">{{ title }}</h3>
            </slot>
          </div>
          <button
            v-if="!hideClose"
            type="button"
            :aria-label="t('common.close')"
            @click="emit('update:open', false)"
            class="flex h-7 w-7 items-center justify-center rounded-lg text-ink-muted hover:text-ink hover:bg-raised transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent cursor-pointer shrink-0"
          >
            <X class="w-4 h-4" aria-hidden="true" />
          </button>
        </div>

        <div class="flex-1 min-h-0 overflow-y-auto" :class="[flushBody ? 'p-0' : 'px-5 py-4', bodyClass]">
          <slot />
        </div>

        <div
          v-if="$slots.footer"
          class="px-5 py-3 border-t border-line-subtle flex flex-wrap items-center justify-end gap-2 shrink-0"
        >
          <slot name="footer" />
        </div>
      </template>

      <slot v-else />
    </div>
  </div>
</template>
