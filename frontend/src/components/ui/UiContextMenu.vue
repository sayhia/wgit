<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from "vue";
import { edgeEnabledIndex, nextEnabledIndex, type ContextMenuItem } from "../../composables/useContextMenu";
import { t } from "../../i18n";

const props = defineProps<{
  open: boolean;
  x: number;
  y: number;
  items: ContextMenuItem[];
}>();

const emit = defineEmits<{
  (e: "select", id: string): void;
  (e: "close"): void;
}>();

const menuRef = ref<HTMLDivElement | null>(null);
const offset = ref({ x: 0, y: 0 });
/** Keyboard cursor; -1 means "nothing highlighted yet". */
const activeIndex = ref(-1);

/** Re-measure after mount: the estimate in `open()` can be off for long menus. */
async function clampToViewport() {
  await nextTick();
  const el = menuRef.value;
  if (!el) return;

  const rect = el.getBoundingClientRect();
  let dx = 0;
  let dy = 0;
  if (rect.right > window.innerWidth - 8) {
    dx = window.innerWidth - 8 - rect.right;
  }
  if (rect.bottom > window.innerHeight - 8) {
    dy = window.innerHeight - 8 - rect.bottom;
  }
  offset.value = { x: dx, y: dy };
}

function handlePointerDown(event: PointerEvent) {
  const el = menuRef.value;
  if (el && !el.contains(event.target as Node)) {
    emit("close");
  }
}

function handleKeyDown(event: KeyboardEvent) {
  switch (event.key) {
    case "Escape":
      event.preventDefault();
      emit("close");
      return;
    case "ArrowDown":
      event.preventDefault();
      activeIndex.value = nextEnabledIndex(props.items, activeIndex.value, 1);
      return;
    case "ArrowUp":
      event.preventDefault();
      activeIndex.value = nextEnabledIndex(props.items, activeIndex.value, -1);
      return;
    case "Home":
      event.preventDefault();
      activeIndex.value = edgeEnabledIndex(props.items, 1);
      return;
    case "End":
      event.preventDefault();
      activeIndex.value = edgeEnabledIndex(props.items, -1);
      return;
    case "Enter":
    case " ": {
      const item = props.items[activeIndex.value];
      if (item && !item.disabled) {
        event.preventDefault();
        select(item);
      }
      return;
    }
  }
}

function handleWindowChange() {
  emit("close");
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      offset.value = { x: 0, y: 0 };
      activeIndex.value = edgeEnabledIndex(props.items, 1);
      void clampToViewport();
      // Capture phase so a menu does not outlive the element it belongs to.
      window.addEventListener("pointerdown", handlePointerDown, true);
      window.addEventListener("keydown", handleKeyDown, true);
      window.addEventListener("resize", handleWindowChange);
      window.addEventListener("scroll", handleWindowChange, true);
    } else {
      window.removeEventListener("pointerdown", handlePointerDown, true);
      window.removeEventListener("keydown", handleKeyDown, true);
      window.removeEventListener("resize", handleWindowChange);
      window.removeEventListener("scroll", handleWindowChange, true);
    }
  }
);

onUnmounted(() => {
  window.removeEventListener("pointerdown", handlePointerDown, true);
  window.removeEventListener("keydown", handleKeyDown, true);
  window.removeEventListener("resize", handleWindowChange);
  window.removeEventListener("scroll", handleWindowChange, true);
});

function select(item: ContextMenuItem) {
  if (item.disabled) return;
  emit("select", item.id);
  emit("close");
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      ref="menuRef"
      role="menu"
      :aria-label="t('ui.context.menu')"
      data-glass
      class="fixed z-[80] w-58 py-1 rounded-xl glass glass-panel glass-popover border"
      :style="{ left: `${x + offset.x}px`, top: `${y + offset.y}px`, width: '232px' }"
      @contextmenu.prevent
    >
      <template v-for="(item, idx) in items" :key="item.id">
        <div v-if="item.separatorBefore" class="my-1 border-t border-line/70" role="separator" />
        <button
          :id="`ctx-menu-item-${item.id}`"
          type="button"
          role="menuitem"
          :disabled="item.disabled"
          :aria-disabled="item.disabled ? 'true' : undefined"
          class="w-full flex items-center gap-2 px-3 py-1.5 text-left text-[11px] transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          :class="[
            item.danger ? 'text-danger' : 'text-ink',
            idx === activeIndex ? (item.danger ? 'bg-danger/15' : 'bg-raised') : '',
            !item.danger && idx !== activeIndex ? 'hover:bg-raised' : '',
            item.danger && idx !== activeIndex ? 'hover:bg-danger/10' : '',
          ]"
          @mouseenter="activeIndex = idx"
          @click="select(item)"
        >
          <component :is="item.icon" v-if="item.icon" class="w-3.5 h-3.5 shrink-0" />
          <span class="flex-1 truncate">{{ item.label }}</span>
          <span v-if="item.hint" class="text-[10px] font-mono text-ink-muted shrink-0">{{ item.hint }}</span>
        </button>
      </template>
    </div>
  </Teleport>
</template>
