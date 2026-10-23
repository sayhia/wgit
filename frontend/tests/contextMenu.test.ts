import { beforeEach, describe, expect, test } from "bun:test";
import {
  edgeEnabledIndex,
  nextEnabledIndex,
  useContextMenu,
  type ContextMenuItem,
} from "../src/composables/useContextMenu";

function installWindow(width: number, height: number) {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: { innerWidth: width, innerHeight: height },
  });
}

function mouseAt(x: number, y: number): MouseEvent {
  const event = {
    clientX: x,
    clientY: y,
    defaultPrevented: false,
    preventDefault() {
      event.defaultPrevented = true;
    },
  };
  return event as unknown as MouseEvent;
}

const ITEMS: ContextMenuItem[] = [
  { id: "a", label: "A" },
  { id: "b", label: "B" },
];

beforeEach(() => installWindow(1200, 800));

describe("useContextMenu", () => {
  test("opens at the pointer when there is room", () => {
    const menu = useContextMenu();
    menu.open(mouseAt(100, 120), ITEMS);

    expect(menu.isOpen.value).toBe(true);
    expect(menu.position.value).toEqual({ x: 100, y: 120 });
    expect(menu.items.value).toHaveLength(2);
  });

  test("prevents the browser context menu", () => {
    const menu = useContextMenu();
    const event = mouseAt(10, 10);
    menu.open(event, ITEMS);
    expect(event.defaultPrevented).toBe(true);
  });

  test("does nothing for an empty item list", () => {
    const menu = useContextMenu();
    menu.open(mouseAt(10, 10), []);
    expect(menu.isOpen.value).toBe(false);
  });

  test("clamps to the right edge", () => {
    const menu = useContextMenu();
    // 1190 + 232 default width would overflow a 1200px viewport.
    menu.open(mouseAt(1190, 100), ITEMS);
    expect(menu.position.value.x).toBe(1200 - 232 - 8);
  });

  test("clamps to the bottom edge using the estimated height", () => {
    const menu = useContextMenu();
    menu.open(mouseAt(100, 790), ITEMS);
    // 2 items * 30 + 16 = 76
    expect(menu.position.value.y).toBe(800 - 76 - 8);
  });

  test("never clamps past the top-left padding", () => {
    const menu = useContextMenu();
    menu.open(mouseAt(0, 0), ITEMS);
    expect(menu.position.value.x).toBe(8);
    expect(menu.position.value.y).toBe(8);
  });

  test("honours an explicit width and height estimate", () => {
    const menu = useContextMenu();
    menu.open(mouseAt(1100, 700), ITEMS, { width: 300, height: 200 });
    expect(menu.position.value.x).toBe(1200 - 300 - 8);
    expect(menu.position.value.y).toBe(800 - 200 - 8);
  });

  test("close() hides the menu but keeps the items for the next open", () => {
    const menu = useContextMenu();
    menu.open(mouseAt(50, 50), ITEMS);
    menu.close();
    expect(menu.isOpen.value).toBe(false);
    expect(menu.items.value).toHaveLength(2);
  });

  test("re-opening replaces the item list", () => {
    const menu = useContextMenu();
    menu.open(mouseAt(50, 50), ITEMS);
    menu.open(mouseAt(60, 60), [{ id: "only", label: "Only" }]);
    expect(menu.items.value.map((i) => i.id)).toEqual(["only"]);
    expect(menu.position.value).toEqual({ x: 60, y: 60 });
  });
});

describe("nextEnabledIndex", () => {
  const items: ContextMenuItem[] = [
    { id: "a", label: "A" },
    { id: "b", label: "B", disabled: true },
    { id: "c", label: "C" },
  ];

  test("skips disabled entries going down", () => {
    expect(nextEnabledIndex(items, 0, 1)).toBe(2);
  });

  test("wraps around from the last item", () => {
    expect(nextEnabledIndex(items, 2, 1)).toBe(0);
  });

  test("wraps backwards from the first item", () => {
    expect(nextEnabledIndex(items, 0, -1)).toBe(2);
  });

  test("starts at the first selectable item when nothing is active", () => {
    expect(nextEnabledIndex(items, -1, 1)).toBe(0);
  });

  test("returns -1 when every item is disabled", () => {
    const allDisabled: ContextMenuItem[] = [
      { id: "x", label: "X", disabled: true },
      { id: "y", label: "Y", disabled: true },
    ];
    expect(nextEnabledIndex(allDisabled, -1, 1)).toBe(-1);
  });

  test("returns -1 for an empty list", () => {
    expect(nextEnabledIndex([], 0, 1)).toBe(-1);
  });
});

describe("edgeEnabledIndex", () => {
  const items: ContextMenuItem[] = [
    { id: "a", label: "A" },
    { id: "b", label: "B" },
    { id: "c", label: "C", disabled: true },
  ];

  test("first selectable for dir=1", () => {
    expect(edgeEnabledIndex(items, 1)).toBe(0);
  });

  test("last selectable for dir=-1 skips a disabled tail", () => {
    expect(edgeEnabledIndex(items, -1)).toBe(1);
  });

  test("returns -1 when nothing is selectable", () => {
    expect(edgeEnabledIndex([{ id: "z", label: "Z", disabled: true }], 1)).toBe(-1);
    expect(edgeEnabledIndex([], -1)).toBe(-1);
  });
});
