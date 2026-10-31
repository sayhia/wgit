<script setup lang="ts">
/**
 * AssetBoard — 方案 §3.7 / F7 最小资产看板（frontend half）.
 *
 * Reads the merged board (tracked-asset scan + DCC locks + on-disk review
 * states) from AssetService and renders summary badges plus one row per asset.
 * Only review/approved are user-settable: tracked is the default state and
 * locked is forced while a DCC lock exists, so those selects stay disabled.
 *
 * Gating: the component itself carries no gate — EngineTab.vue mounts it
 * under the assetBoard flag so the mount point stays the single place that
 * knows about the flag.
 *
 * State changes are optimistic: the row flips immediately, a failure rolls the
 * badge back and surfaces a danger toast; success stays silent.
 */
import { computed, onMounted, ref, watch } from "vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import { formatByteSize } from "../../utils/byteSize";
import { relativeTime } from "../../utils/relativeTime";
import {
  ASSET_STATES,
  USER_SETTABLE_ASSET_STATES,
  getAssetBoard,
  setAssetState,
  type AssetBoard as AssetBoardData,
  type AssetItem,
} from "../../services/assetService";
import { RefreshCw, Package } from "lucide-vue-next";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
}>();

/** Ask for the backend maximum; the cap (500) keeps the list non-virtualized. */
const BOARD_LIMIT = 500;

const board = ref<AssetBoardData | null>(null);
const isLoading = ref(false);
const loadError = ref("");
/** Paths with an in-flight setAssetState call; their selects are disabled. */
const pendingPaths = ref(new Set<string>());

const items = computed<AssetItem[]>(() => board.value?.items ?? []);
const warnings = computed<string[]>(() => board.value?.warnings ?? []);

const summary = computed<Record<string, number>>(() => {
  const raw = board.value?.summary ?? {};
  const out: Record<string, number> = {};
  for (const state of ASSET_STATES) out[state] = raw[state] ?? 0;
  return out;
});

/** Total assets across the repo (summary is never limit-truncated). */
const totalCount = computed(() =>
  ASSET_STATES.reduce((sum, state) => sum + (summary.value[state] ?? 0), 0)
);

const truncated = computed(() => totalCount.value > items.value.length);

const generatedAtText = computed(() => {
  const raw = board.value?.generatedAt || "";
  if (!raw) return "";
  const parsed = Date.parse(raw);
  if (Number.isNaN(parsed)) return raw;
  return new Date(parsed).toLocaleString();
});

async function loadBoard() {
  if (!props.currentRepoPath || isLoading.value) return;
  isLoading.value = true;
  loadError.value = "";
  try {
    board.value = await getAssetBoard(props.currentRepoPath, BOARD_LIMIT);
  } catch (err: any) {
    board.value = null;
    loadError.value = String(err?.message || err);
  } finally {
    isLoading.value = false;
  }
}

/**
 * Optimistic state switch. The row badge flips before the RPC; on failure the
 * previous state is restored and a danger toast explains the rollback.
 */
async function handleStateChange(item: AssetItem, next: string) {
  if (next === item.state || pendingPaths.value.has(item.path)) return;
  const previous = item.state;
  item.state = next;
  pendingPaths.value = new Set(pendingPaths.value).add(item.path);
  try {
    // Success stays silent — the flipped badge IS the feedback.
    await setAssetState(props.currentRepoPath, item.path, next);
  } catch (err: any) {
    item.state = previous;
    pushToast(t("asset.board.toast.state.failed", { error: err?.message || err }), {
      tone: "danger",
    });
  } finally {
    const rest = new Set(pendingPaths.value);
    rest.delete(item.path);
    pendingPaths.value = rest;
  }
}

function isUserSettable(state: string): boolean {
  return (USER_SETTABLE_ASSET_STATES as readonly string[]).includes(state);
}

function stateHint(state: string): string {
  if (state === "locked") return t("asset.board.state.locked.hint");
  if (state === "tracked") return t("asset.board.state.tracked.hint");
  return t("asset.board.state.select");
}

function updatedAtText(item: AssetItem): string {
  return relativeTime(item.updatedAt, item.updatedAt);
}

onMounted(() => {
  void loadBoard();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadBoard();
  }
);

watch(
  () => props.currentRepoPath,
  () => {
    void loadBoard();
  }
);
</script>

<template>
  <div class="space-y-3" data-tool-section="asset-board" data-testid="asset-board">
    <div class="flex items-center justify-between pb-2 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Package class="w-4 h-4 text-accent" />
          <span>{{ t("asset.board.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("asset.board.desc") }}
        </p>
      </div>
      <button
        data-testid="asset-board-refresh"
        @click="loadBoard"
        :disabled="isLoading"
        class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
      >
        {{ isLoading ? t("common.loading") : t("common.refresh") }}
      </button>
    </div>

    <div
      v-if="warnings.length > 0"
      data-testid="asset-board-warnings"
      class="px-3 py-2 rounded-lg bg-warning-bg border border-warning/30 text-[11px] text-warning space-y-0.5"
    >
      <div v-for="warning in warnings" :key="warning" class="flex items-start gap-1.5">
        <span class="shrink-0">•</span>
        <span class="min-w-0">{{ warning }}</span>
      </div>
    </div>

    <!-- Summary badges: counts cover the whole repo, not just the shown rows. -->
    <div
      v-if="board"
      data-testid="asset-board-summary"
      class="flex flex-wrap items-center gap-2"
    >
      <span
        v-for="state in ASSET_STATES"
        :key="state"
        :data-testid="`asset-board-badge-${state}`"
        class="px-2 py-0.5 rounded text-[11px] font-bold"
        :class="{
          'bg-accent-bg text-accent': state === 'tracked',
          'bg-warning-bg text-warning': state === 'locked',
          'bg-purple-bg text-purple': state === 'review',
          'bg-success-bg text-success': state === 'approved',
        }"
      >
        {{ t(`asset.board.state.${state}`) }} {{ summary[state] ?? 0 }}
      </span>
      <span
        v-if="generatedAtText"
        data-testid="asset-board-generated-at"
        class="ml-auto text-[10px] text-ink-muted font-mono"
      >
        {{ t("asset.board.generated.at", { time: generatedAtText }) }}
      </span>
    </div>

    <div v-if="loadError" class="px-3 py-2 rounded-lg bg-danger-bg border border-danger/30 text-[11px] text-danger">
      {{ t("asset.board.load.failed", { error: loadError }) }}
    </div>

    <div v-if="board && items.length > 0" class="space-y-1">
      <div
        v-for="item in items"
        :key="item.path"
        data-testid="asset-board-row"
        class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg bg-surface border border-line text-[11px]"
      >
        <span class="font-mono text-ink truncate flex-1 min-w-0" :title="item.path">{{ item.path }}</span>
        <span
          :data-testid="`asset-board-row-state-${item.state}`"
          class="px-1.5 py-0.5 rounded text-[9px] font-bold shrink-0"
          :class="{
            'bg-accent-bg text-accent': item.state === 'tracked',
            'bg-warning-bg text-warning': item.state === 'locked',
            'bg-purple-bg text-purple': item.state === 'review',
            'bg-success-bg text-success': item.state === 'approved',
          }"
        >
          {{ t(`asset.board.state.${item.state}`) }}
        </span>
        <span
          class="text-ink-muted shrink-0 max-w-[120px] truncate"
          :title="item.owner ? t('asset.board.owner', { owner: item.owner }) : ''"
        >
          {{ item.owner || "—" }}
        </span>
        <span class="font-mono text-ink-muted shrink-0" :title="t('engine.asset.size')">
          {{ formatByteSize(item.sizeBytes) }}
        </span>
        <span
          v-if="item.updatedAt"
          class="text-[10px] text-ink-muted shrink-0 hidden lg:inline"
          :title="item.updatedAt"
        >
          {{ updatedAtText(item) }}
        </span>
        <select
          :data-testid="`asset-board-select-${item.path}`"
          :value="item.state"
          :disabled="!isUserSettable(item.state) || pendingPaths.has(item.path)"
          :title="stateHint(item.state)"
          :aria-label="t('asset.board.state.select')"
          class="shrink-0 px-1.5 py-0.5 rounded bg-canvas border border-line text-[10px] text-ink focus:outline-none focus:border-accent disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
          @change="handleStateChange(item, ($event.target as HTMLSelectElement).value)"
        >
          <option v-for="s in USER_SETTABLE_ASSET_STATES" :key="s" :value="s">
            {{ t(`asset.board.state.${s}`) }}
          </option>
        </select>
      </div>
      <div v-if="truncated" class="pt-1 text-[10px] text-ink-muted text-center">
        {{ t("asset.board.truncated", { shown: items.length, total: totalCount }) }}
      </div>
    </div>

    <!-- Empty state: no tracked engine assets at all. -->
    <div
      v-if="board && !loadError && items.length === 0"
      data-testid="asset-board-empty"
      class="py-10 text-center"
    >
      <Package class="w-8 h-8 text-ink-muted mx-auto mb-2" />
      <div class="text-xs text-ink-muted">{{ t("asset.board.empty.title") }}</div>
      <div class="text-[10px] text-ink-muted mt-1">{{ t("asset.board.empty.hint") }}</div>
    </div>
  </div>
</template>
