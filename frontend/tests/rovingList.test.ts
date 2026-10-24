import { describe, expect, test } from "bun:test";
import { ref } from "vue";
import { useRovingList } from "../src/composables/useRovingList";

describe("useRovingList", () => {
  test("keeps the first row tabbable before any focus", () => {
    const rows = ref([{ key: "a" }, { key: "b" }]);
    const roving = useRovingList(() => rows.value, (row) => row.key);
    expect(roving.tabindexFor(rows.value[0]!, 0)).toBe(0);
    expect(roving.tabindexFor(rows.value[1]!, 1)).toBe(-1);
  });

  test("resets the active key when the focused row disappears", () => {
    const rows = ref([{ key: "a" }, { key: "b" }]);
    const roving = useRovingList(() => rows.value, (row) => row.key);
    roving.onFocus(rows.value[1]!);
    expect(roving.tabindexFor(rows.value[0]!, 0)).toBe(-1);

    rows.value = [rows.value[0]!];
    expect(roving.tabindexFor(rows.value[0]!, 0)).toBe(0);
  });
});
