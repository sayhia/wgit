import { ref, type Component } from "vue";

/**
 * Right-click menu state shared by list views.
 *
 * Only owns positioning and lifecycle; the items and their actions stay with
 * the list component, because every list means something different by
 * "delete" or "compare".
 */

export interface ContextMenuItem {
  id: string;
  label: string;
  icon?: Component;
  /** Renders in the danger colour and sits after a separator by convention. */
  danger?: boolean;
  disabled?: boolean;
  separatorBefore?: boolean;
  /** Optional right-aligned hint, e.g. "⌘C". */
  hint?: string;
}

export interface ContextMenuOpenOptions {
  /** Estimated size, used to keep the menu inside the viewport before mount. */
  width?: number;
  height?: number;
}

const VIEWPORT_PADDING = 8;

export function useContextMenu() {
  const isOpen = ref(false);
  const position = ref({ x: 0, y: 0 });
  const items = ref<ContextMenuItem[]>([]);

  function open(event: MouseEvent, nextItems: ContextMenuItem[], opts: ContextMenuOpenOptions = {}) {
    if (nextItems.length === 0) return;
    event.preventDefault();

    const width = opts.width ?? 232;
    const height = opts.height ?? nextItems.length * 30 + 16;
    const maxX = window.innerWidth - width - VIEWPORT_PADDING;
    const maxY = window.innerHeight - height - VIEWPORT_PADDING;

    position.value = {
      x: Math.max(VIEWPORT_PADDING, Math.min(event.clientX, maxX)),
      y: Math.max(VIEWPORT_PADDING, Math.min(event.clientY, maxY)),
    };
    items.value = nextItems;
    isOpen.value = true;
  }

  function close() {
    isOpen.value = false;
  }

  return { isOpen, position, items, open, close };
}

/**
 * Moves to the next selectable item, wrapping around and skipping disabled
 * entries. Returns -1 when nothing is selectable.
 */
export function nextEnabledIndex(items: ContextMenuItem[], from: number, delta: 1 | -1): number {
  const total = items.length;
  if (total === 0) return -1;

  let cursor = from;
  for (let step = 0; step < total; step++) {
    cursor = (cursor + delta + total) % total;
    if (!items[cursor]?.disabled) return cursor;
  }
  return -1;
}

/** First (dir=1) or last (dir=-1) selectable index. */
export function edgeEnabledIndex(items: ContextMenuItem[], dir: 1 | -1): number {
  if (items.length === 0) return -1;
  return dir === 1 ? nextEnabledIndex(items, -1, 1) : nextEnabledIndex(items, 0, -1);
}
