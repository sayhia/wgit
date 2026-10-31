/**
 * AssetBoard — F7 minimal loop, frontend half.
 *
 * Layers covered:
 *   1. service routing: the wrapper must address AssetService by full name
 *      (main.AssetService.*) via Call.ByName — gitService.callBackend hardcodes
 *      the main.GitService. prefix and would miss the surface entirely.
 *   2. EngineTab mount point: the gate lives in EngineTab (FeatureGate
 *      flag="assetBoard" above the engine sections), the child does not
 *      double-gate itself.
 *   3. mounted behavior (real component, @wailsio/runtime mocked): four
 *      summary badges + GeneratedAt, dropdown disabled for system-owned
 *      states (tracked/locked), optimistic switch with rollback + danger
 *      toast, warnings bar, empty state.
 *   4. i18n: zh-CN / en-US parity for every asset.board.* key.
 */
import { beforeEach, describe, expect, mock, test } from "bun:test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import zhCN from "../src/i18n/messages/zh-CN";
import enUS from "../src/i18n/messages/en-US";
import { locale, setWarnMissing } from "../src/i18n";
import { resetToasts, toasts } from "../src/composables/useToast";
import { formatByteSize } from "../src/utils/byteSize";

const here = (p: string) => fileURLToPath(new URL(p, import.meta.url));
const serviceSrc = readFileSync(here("../src/services/assetService.ts"), "utf8");
const engineTabSrc = readFileSync(here("../src/components/tools/EngineTab.vue"), "utf8");
const boardSrc = readFileSync(here("../src/components/tools/AssetBoard.vue"), "utf8");
const dict = (m: object) => m as Record<string, string>;

// ---------------------------------------------------------------------------
// Runtime mock: intercept @wailsio/runtime BEFORE the component graph loads so
// the REAL assetService wrapper runs and its routing can be asserted end to end.
// ---------------------------------------------------------------------------
let boardResponse: unknown = null;
let boardShouldFail = false;
let setStateShouldFail = false;
const rpcCalls: { name: string; args: unknown[] }[] = [];

function handleCall(name: string, args: unknown[]): Promise<unknown> {
  rpcCalls.push({ name, args });
  switch (name) {
    case "main.AssetService.GetAssetBoard":
      return boardShouldFail ? Promise.reject(new Error("board exploded")) : Promise.resolve(boardResponse);
    case "main.AssetService.SetAssetState":
      return setStateShouldFail ? Promise.reject(new Error("backend refused")) : Promise.resolve(undefined);
    case "main.AssetService.GetAssetStates":
      return Promise.resolve({});
    default:
      return Promise.resolve(null);
  }
}

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByName: (name: string, ...args: unknown[]) => handleCall(name, args),
    ByID: () => Promise.resolve(null),
  },
  Events: { On: () => () => {}, Emit: () => {}, Off: () => {} },
  Window: { Name: () => "test" },
  Screens: {},
}));

const { mount, flushPromises } = await import("@vue/test-utils");
const AssetBoardComp = (await import("../src/components/tools/AssetBoard.vue")).default;

const FIXTURE_BOARD = {
  items: [
    { path: "Assets/Tex/hero.png", state: "review", owner: "", sizeBytes: 1536, budgetRatio: 0, updatedAt: "2026-01-02T03:04:05Z" },
    { path: "Assets/Models/hero.fbx", state: "locked", owner: "alice", sizeBytes: 1048576, budgetRatio: 0, updatedAt: "" },
    { path: "Assets/Audio/bgm.ogg", state: "tracked", owner: "", sizeBytes: 0, budgetRatio: 0, updatedAt: "" },
    { path: "Assets/Scenes/level01.unity", state: "approved", owner: "", sizeBytes: 2048, budgetRatio: 0, updatedAt: "" },
  ],
  summary: { tracked: 3, locked: 1, review: 2, approved: 1 },
  generatedAt: "2026-10-29T08:00:00Z",
  warnings: ["未配置资产体积预算，budgetRatio 暂为 0"],
};

function cloneFixture() {
  return JSON.parse(JSON.stringify(FIXTURE_BOARD));
}

async function mountBoard() {
  const wrapper = mount(AssetBoardComp, {
    props: { currentRepoPath: "/repo", refreshNonce: 0 },
  });
  await flushPromises();
  return wrapper;
}

function rowFor(wrapper: ReturnType<typeof mount>, path: string) {
  return wrapper
    .findAll('[data-testid="asset-board-row"]')
    .find((row) => row.text().includes(path))!;
}

/**
 * Select-value helper that dispatches through the element's OWN window.
 *
 * mock.module re-seats module registries mid-suite and the parallel preloads
 * can swap globalThis.window while this file is running — @vue/test-utils'
 * setValue then builds its change event from a global window that no longer
 * matches the mounted document. The element's ownerDocument window is always
 * the right realm for dispatching, so we bypass setValue entirely.
 */
async function setSelectValue(select: { element: Element }, value: string) {
  const el = select.element as HTMLSelectElement;
  const realm = el.ownerDocument.defaultView as { Event?: new (type: string, init?: { bubbles?: boolean }) => Event } | null;
  const EventCtor = realm?.Event ?? (globalThis as unknown as { Event: new (type: string, init?: { bubbles?: boolean }) => Event }).Event;
  el.value = value;
  el.dispatchEvent(new EventCtor("change", { bubbles: true }));
  await flushPromises();
}

beforeEach(() => {
  boardResponse = cloneFixture();
  boardShouldFail = false;
  setStateShouldFail = false;
  rpcCalls.length = 0;
  resetToasts();
  setWarnMissing(false);
  locale.value = "zh-CN";
});

describe("assetService routing (full-name Call.ByName)", () => {
  test("addresses main.AssetService.* by name, never through callBackend", () => {
    expect(serviceSrc).toContain('Call.ByName("main.AssetService.GetAssetBoard"');
    expect(serviceSrc).toContain('Call.ByName("main.AssetService.SetAssetState"');
    expect(serviceSrc).toContain('Call.ByName("main.AssetService.GetAssetStates"');
    // No callBackend helper usage and no gitService import (the comment may
    // still mention callBackend to explain the routing choice).
    expect(serviceSrc).not.toContain("callBackend(");
    expect(serviceSrc).not.toContain('from "./gitService"');
  });

  test("documents why callBackend cannot be used (main.GitService. prefix)", () => {
    expect(serviceSrc).toContain("callBackend hardcodes the");
    expect(serviceSrc).toContain("main.GitService.");
  });

  test("exposes the three wrappers and the state vocabulary", () => {
    expect(serviceSrc).toContain("export function getAssetBoard(");
    expect(serviceSrc).toContain("export function setAssetState(");
    expect(serviceSrc).toContain("export function getAssetStates(");
    expect(serviceSrc).toContain('["tracked", "locked", "review", "approved"]');
    expect(serviceSrc).toContain('USER_SETTABLE_ASSET_STATES: readonly AssetState[] = ["review", "approved"]');
  });

  test("mounted rows really route through the full names", async () => {
    await mountBoard();
    expect(rpcCalls.some((c) => c.name === "main.AssetService.GetAssetBoard")).toBe(true);
  });
});

describe("FeatureGate mount point lives in EngineTab", () => {
  test("EngineTab imports FeatureGate and wraps AssetBoard with flag=assetBoard", () => {
    expect(engineTabSrc).toContain('import FeatureGate from "../ui/FeatureGate.vue"');
    expect(engineTabSrc).toContain('import AssetBoard from "./AssetBoard.vue"');
    expect(engineTabSrc).toContain('<FeatureGate flag="assetBoard"');
    expect(engineTabSrc).toContain("<AssetBoard");
  });

  test("the gate sits above the existing engine sections (top mount point)", () => {
    const gateAt = engineTabSrc.indexOf('<FeatureGate flag="assetBoard"');
    const firstEngineSection = engineTabSrc.indexOf('data-tool-section="engine-detect"');
    expect(gateAt).toBeGreaterThan(-1);
    expect(firstEngineSection).toBeGreaterThan(-1);
    expect(gateAt).toBeLessThan(firstEngineSection);
  });

  test("AssetBoard.vue does not double-gate itself", () => {
    // The comment explains the gate lives in EngineTab; the element and the
    // import must be absent.
    expect(boardSrc).not.toContain("<FeatureGate");
    expect(boardSrc).not.toContain("ui/FeatureGate.vue");
    expect(boardSrc).toContain('data-testid="asset-board"');
  });
});

describe("mounted board: badges, dropdown, optimistic updates", () => {
  test("renders the four state badges with counts plus GeneratedAt", async () => {
    const wrapper = await mountBoard();
    for (const [state, count] of [
      ["tracked", "3"],
      ["locked", "1"],
      ["review", "2"],
      ["approved", "1"],
    ] as const) {
      const badge = wrapper.find(`[data-testid="asset-board-badge-${state}"]`);
      expect(badge.exists()).toBe(true);
      expect(badge.text()).toContain(count);
    }
    expect(wrapper.find('[data-testid="asset-board-generated-at"]').exists()).toBe(true);
  });

  test("locks/tracked rows have a disabled dropdown, review/approved stay switchable", async () => {
    const wrapper = await mountBoard();
    const lockedSelect = rowFor(wrapper, "Assets/Models/hero.fbx").find("select");
    const trackedSelect = rowFor(wrapper, "Assets/Audio/bgm.ogg").find("select");
    const reviewSelect = rowFor(wrapper, "Assets/Tex/hero.png").find("select");
    const approvedSelect = rowFor(wrapper, "Assets/Scenes/level01.unity").find("select");
    expect(lockedSelect!.attributes("disabled")).toBeDefined();
    expect(trackedSelect!.attributes("disabled")).toBeDefined();
    expect(reviewSelect!.attributes("disabled")).toBeUndefined();
    expect(approvedSelect!.attributes("disabled")).toBeUndefined();
    // Only the two user-settable states are offered.
    expect(reviewSelect!.findAll("option")).toHaveLength(2);
  });

  test("optimistic switch: badge flips first, RPC carries repo+path+state, success is silent", async () => {
    const wrapper = await mountBoard();
    await setSelectValue(rowFor(wrapper, "Assets/Tex/hero.png").find("select")!, "approved");
    await flushPromises();

    // Optimistic flip happened before/without an error path.
    expect(rowFor(wrapper, "Assets/Tex/hero.png").find('[data-testid="asset-board-row-state-approved"]').exists()).toBe(true);
    expect(
      rpcCalls.some(
        (c) =>
          c.name === "main.AssetService.SetAssetState" &&
          JSON.stringify(c.args) === JSON.stringify(["/repo", "Assets/Tex/hero.png", "approved"])
      )
    ).toBe(true);
    // Success stays silent: no toast of any tone.
    expect(toasts.value).toHaveLength(0);
  });

  test("failed switch rolls the row back and raises a danger toast", async () => {
    const wrapper = await mountBoard();
    setStateShouldFail = true;
    await setSelectValue(rowFor(wrapper, "Assets/Tex/hero.png").find("select")!, "approved");
    await flushPromises();

    expect(rowFor(wrapper, "Assets/Tex/hero.png").find('[data-testid="asset-board-row-state-review"]').exists()).toBe(true);
    expect(rowFor(wrapper, "Assets/Tex/hero.png").find('[data-testid="asset-board-row-state-approved"]').exists()).toBe(false);
    const danger = toasts.value.find((toast) => toast.tone === "danger");
    expect(danger).toBeDefined();
    expect(danger!.text).toContain("已回滚");
    expect(danger!.text).toContain("backend refused");
  });
});

describe("warnings bar and empty state", () => {
  test("renders the yellow warnings bar when warnings are non-empty", async () => {
    const wrapper = await mountBoard();
    const bar = wrapper.find('[data-testid="asset-board-warnings"]');
    expect(bar.exists()).toBe(true);
    expect(bar.text()).toContain("budgetRatio");
  });

  test("no warnings bar when the board carries none", async () => {
    boardResponse = { ...cloneFixture(), warnings: [] };
    const wrapper = await mountBoard();
    expect(wrapper.find('[data-testid="asset-board-warnings"]').exists()).toBe(false);
  });

  test("shows the guided empty state when the repo has no assets", async () => {
    boardResponse = {
      items: [],
      summary: { tracked: 0, locked: 0, review: 0, approved: 0 },
      generatedAt: "2026-10-29T08:00:00Z",
      warnings: [],
    };
    const wrapper = await mountBoard();
    const empty = wrapper.find('[data-testid="asset-board-empty"]');
    expect(empty.exists()).toBe(true);
    expect(empty.text()).toContain(dict(zhCN)["asset.board.empty.title"]);
    expect(wrapper.find('[data-testid="asset-board-row"]').exists()).toBe(false);
  });

  test("load failure surfaces the error line instead of rows", async () => {
    boardShouldFail = true;
    const wrapper = await mountBoard();
    expect(wrapper.text()).toContain(dict(zhCN)["asset.board.load.failed"].replace("{error}", "board exploded"));
    expect(wrapper.find('[data-testid="asset-board-row"]').exists()).toBe(false);
  });
});

describe("byteSize util", () => {
  test("formats binary units compactly", () => {
    expect(formatByteSize(0)).toBe("0 B");
    expect(formatByteSize(-5)).toBe("0 B");
    expect(formatByteSize(512)).toBe("512 B");
    expect(formatByteSize(1536)).toBe("1.5 KB");
    expect(formatByteSize(1048576)).toBe("1.0 MB");
  });
});

describe("i18n parity for asset.board.*", () => {
  const requiredKeys = [
    "asset.board.title",
    "asset.board.desc",
    "asset.board.generated.at",
    "asset.board.state.tracked",
    "asset.board.state.locked",
    "asset.board.state.review",
    "asset.board.state.approved",
    "asset.board.state.tracked.hint",
    "asset.board.state.locked.hint",
    "asset.board.state.select",
    "asset.board.owner",
    "asset.board.truncated",
    "asset.board.empty.title",
    "asset.board.empty.hint",
    "asset.board.load.failed",
    "asset.board.toast.state.failed",
  ];

  test("every new key exists in both locales", () => {
    for (const key of requiredKeys) {
      expect(dict(zhCN)[key]).toBeTruthy();
      expect(dict(enUS)[key]).toBeTruthy();
    }
  });

  test("reused existing keys are still present", () => {
    // Refresh button and the size column label reuse shared/engine keys.
    expect(dict(zhCN)["common.refresh"]).toBeTruthy();
    expect(dict(enUS)["common.refresh"]).toBeTruthy();
    expect(dict(zhCN)["engine.asset.size"]).toBeTruthy();
    expect(dict(enUS)["engine.asset.size"]).toBeTruthy();
  });
});
