import { ref, watch, type Ref } from "vue";

/**
 * Roving tabindex for row-based lists.
 *
 * Why not `role="listbox"`? These rows contain their own buttons (checkout /
 * merge / delete). Nesting interactive controls inside `role="option"` is worse
 * for assistive tech than leaving the rows as labelled groups. So this keeps
 * the existing semantics and only fixes keyboard reachability:
 *
 *   · exactly one row is in the tab order at a time (roving tabindex)
 *   · ↑/↓/Home/End move focus between rows
 *   · Enter is left to the list (each list decides its primary action)
 *
 * `rovingTargetIndex` is a pure function so the index math is unit-tested.
 */

export function rovingTargetIndex(current: number, total: number, key: string): number | null {
  if (total <= 0) return null;
  const last = total - 1;
  switch (key) {
    case "ArrowDown":
      return current < 0 ? 0 : Math.min(current + 1, last);
    case "ArrowUp":
      return current < 0 ? last : Math.max(current - 1, 0);
    case "Home":
      return 0;
    case "End":
      return last;
    default:
      return null;
  }
}

export interface RovingList<T> {
  activeKey: Ref<string>;
  setItemRef: (item: T, el: unknown) => void;
  tabindexFor: (item: T, index: number) => number;
  onKeydown: (event: KeyboardEvent, index: number) => void;
  onFocus: (item: T) => void;
  focusIndex: (index: number) => void;
}

export function useRovingList<T>(items: () => T[], keyOf: (item: T) => string): RovingList<T> {
  const activeKey = ref("");
  const elements = new Map<string, HTMLElement>();

  // Deleting the active row must not leave every remaining row at tabindex -1.
  watch(
    items,
    (list) => {
      if (activeKey.value && !list.some((item) => keyOf(item) === activeKey.value)) {
        activeKey.value = "";
      }
    },
    { flush: "sync" }
  );

  function setItemRef(item: T, el: unknown) {
    const key = keyOf(item);
    if (el instanceof HTMLElement) elements.set(key, el);
    else elements.delete(key);
  }

  function tabindexFor(item: T, index: number): number {
    if (activeKey.value) return activeKey.value === keyOf(item) ? 0 : -1;
    // Before the first interaction only the first row is tabbable, so Tab
    // enters the list once instead of walking every row.
    return index === 0 ? 0 : -1;
  }

  function focusIndex(index: number) {
    const list = items();
    const item = list[index];
    if (!item) return;
    activeKey.value = keyOf(item);
    elements.get(keyOf(item))?.focus();
  }

  function onKeydown(event: KeyboardEvent, index: number) {
    // Let inputs inside a row keep their own arrow-key behaviour.
    const target = event.target as HTMLElement | null;
    const tag = target?.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;

    const next = rovingTargetIndex(index, items().length, event.key);
    if (next == null) return;
    event.preventDefault();
    focusIndex(next);
  }

  function onFocus(item: T) {
    activeKey.value = keyOf(item);
  }

  return { activeKey, setItemRef, tabindexFor, onKeydown, onFocus, focusIndex };
}
