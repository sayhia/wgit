<script setup lang="ts">
/**
 * SettingsSearch — query, history and item-level combobox for settings.
 *
 * Owns every bit of search state that used to live inline in `SettingsView`:
 * the query, the persisted recent-query history, keyboard selection, result
 * highlighting and the item jump toast. The navigation filters its leaves from
 * the same `modelValue`, so the search box and the grouped rail always agree.
 */
import { computed, ref, watch } from "vue";
import { Search, X } from "lucide-vue-next";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import {
  SETTINGS_CATEGORY_LABEL_KEYS,
  SETTINGS_ITEM_INDEX,
  SETTINGS_LEAF_KEYWORDS,
  highlightParts,
  type SettingsIndexItem,
} from "../../utils/settingsIndex";
import { SETTINGS_LEAVES, type SettingsLeaf } from "../../utils/settingsStructure";

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
  (e: "select-leaf", leaf: SettingsLeaf): void;
}>();

const query = computed({
  get: () => props.modelValue,
  set: (value: string) => emit("update:modelValue", value),
});

const SEARCH_HISTORY_KEY = "wgit.settings.searchHistory.v1";

function loadSearchHistory(): string[] {
  try {
    const raw = localStorage.getItem(SEARCH_HISTORY_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
  return Array.isArray(parsed)
      ? parsed.filter((item): item is string => typeof item === "string").slice(0, 5)
      : [];
  } catch {
    return [];
  }
}

const searchHistory = ref<string[]>(loadSearchHistory());
const searchFocused = ref(false);
const searchResultsRef = ref<HTMLElement | null>(null);

function revealSearchSelection() {
  searchResultsRef.value?.querySelector<HTMLElement>('[data-search-selected="true"]')
    ?.scrollIntoView?.({ block: "nearest" });
}

function rememberQuery(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return;
  const next = [trimmed, ...searchHistory.value.filter((item) => item !== trimmed)].slice(0, 5);
  searchHistory.value = next;
  try {
    localStorage.setItem(SEARCH_HISTORY_KEY, JSON.stringify(next));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

function applyHistoryQuery(value: string) {
  query.value = value;
  searchFocused.value = true;
}

function clearSearchHistory() {
  searchHistory.value = [];
  try {
    localStorage.removeItem(SEARCH_HISTORY_KEY);
  } catch {
    /* ignore */
  }
}

const filteredLeaves = computed<SettingsLeaf[]>(() => {
  const needle = query.value.trim().toLowerCase();
  if (!needle) return [...SETTINGS_LEAVES];
  return SETTINGS_LEAVES.filter((leaf) => {
    const labelKey = SETTINGS_CATEGORY_LABEL_KEYS[leaf];
    if (t(labelKey).toLowerCase().includes(needle)) return true;
    return SETTINGS_LEAF_KEYWORDS[leaf].some((keyword) => keyword.toLowerCase().includes(needle));
  });
});

/** Item-level search results (e.g. "邮箱" → Git 身份认证 · Git 邮箱). */
const matchedSettingItems = computed<SettingsIndexItem[]>(() => {
  const needle = query.value.trim().toLowerCase();
  if (!needle) return [];
  return SETTINGS_ITEM_INDEX.filter(
    (item) =>
  t(item.labelKey).toLowerCase().includes(needle) ||
  item.keywords.some((keyword) => keyword.toLowerCase().includes(needle))
  ).slice(0, 8);
});

function categoryLabel(category: SettingsLeaf): string {
  const key = SETTINGS_CATEGORY_LABEL_KEYS[category];
  return key ? t(key) : category;
}

function openSettingItem(item: SettingsIndexItem) {
  rememberQuery(query.value);
  query.value = "";
  emit("select-leaf", item.category);
  pushToast(
    t("settings.search.jumped", {
      item: t(item.labelKey),
      category: categoryLabel(item.category),
    })
  );
}

function selectFirstFilteredLeaf() {
  const first = filteredLeaves.value[0];
  if (!first) return;
  rememberQuery(query.value);
  emit("select-leaf", first);
}

const searchItemIndex = ref(0);
const searchHistoryIndex = ref(0);

const searchHistoryVisible = computed(
  () => !query.value.trim() && searchFocused.value && searchHistory.value.length > 0
);

const noMatches = computed(
  () => Boolean(query.value.trim()) && filteredLeaves.value.length === 0 && matchedSettingItems.value.length === 0
);

watch(query, () => {
  searchItemIndex.value = 0;
  searchHistoryIndex.value = 0;
});

watch([searchItemIndex, searchHistoryIndex, query], revealSearchSelection, { flush: "post" });

watch(searchHistory, (history) => {
  if (searchHistoryIndex.value >= history.length) searchHistoryIndex.value = 0;
});

watch(matchedSettingItems, (items) => {
  if (searchItemIndex.value >= items.length) searchItemIndex.value = 0;
});

function moveSelection(delta: number) {
  if (searchHistoryVisible.value) {
    const history = searchHistory.value;
    searchHistoryIndex.value = (searchHistoryIndex.value + delta + history.length) % history.length;
    return;
  }
  const items = matchedSettingItems.value;
  if (items.length === 0) return;
  searchItemIndex.value = (searchItemIndex.value + delta + items.length) % items.length;
}

function activateSelection() {
  if (searchHistoryVisible.value) {
    const recent = searchHistory.value[searchHistoryIndex.value];
    if (recent) applyHistoryQuery(recent);
    return;
  }
  const items = matchedSettingItems.value;
  if (items.length > 0) {
    openSettingItem(items[searchItemIndex.value]!);
    return;
  }
  selectFirstFilteredLeaf();
}
</script>

<template>
  <div role="search" data-testid="settings-search" class="shrink-0 mx-3 mt-3 md:mt-0 mb-3 space-y-2">
    <div class="relative">
      <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-ink-muted pointer-events-none" aria-hidden="true" />
      <input
        v-model="query"
        type="text"
        spellcheck="false"
        :aria-label="t('settings.search.placeholder')"
        :placeholder="t('settings.search.placeholder')"
        class="w-full pl-8 pr-8 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
        @keydown.enter.prevent="activateSelection"
        @keydown.down.prevent="moveSelection(1)"
        @keydown.up.prevent="moveSelection(-1)"
        @focus="searchFocused = true"
        @blur="searchFocused = false"
      />
      <button v-if="query" type="button" class="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 rounded text-ink-muted hover:text-ink" :aria-label="t('settings.search.reset')" @click="query = ''">
        <X class="w-3 h-3" aria-hidden="true" />
      </button>
    </div>

    <div
      v-if="searchHistoryVisible || (query.trim() && matchedSettingItems.length) || noMatches"
      ref="searchResultsRef"
      data-testid="settings-search-results"
      class="max-h-[min(16rem,30dvh)] overflow-y-auto space-y-2"
    >
    <div v-if="searchHistoryVisible" class="rounded-lg border border-line bg-canvas overflow-hidden">
      <div class="flex items-center justify-between px-2.5 pt-2 pb-1">
        <span class="text-[10px] text-ink-muted">{{ t("settings.search.recent") }}</span>
        <button @mousedown.prevent @click="clearSearchHistory" class="text-[10px] text-ink-muted hover:text-danger cursor-pointer" :title="t('settings.search.clear.hint')">{{ t("settings.search.clear") }}</button>
      </div>
      <button v-for="(recent, historyIndex) in searchHistory" :key="recent" :data-search-selected="searchHistoryIndex === historyIndex" @mousedown.prevent @click="applyHistoryQuery(recent)" @mouseenter="searchHistoryIndex = historyIndex"
        class="w-full flex items-center gap-base px-2.5 py-1.5 text-left text-xs cursor-pointer"
        :class="searchHistoryIndex === historyIndex ? 'bg-raised text-accent' : 'text-ink-secondary hover:bg-raised'">
        <Search class="w-3 h-3 shrink-0 text-ink-muted" aria-hidden="true" /><span class="truncate">{{ recent }}</span>
      </button>
    </div>

    <div v-if="query.trim() && matchedSettingItems.length" class="rounded-lg border border-line bg-canvas overflow-hidden divide-y divide-line/50">
      <button v-for="(item, itemIndex) in matchedSettingItems" :key="`${item.category}-${item.labelKey}`" :data-search-selected="searchItemIndex === itemIndex" @click="openSettingItem(item)" @mouseenter="searchItemIndex = itemIndex"
        class="w-full flex flex-col gap-0.5 px-2.5 py-2 text-left cursor-pointer"
        :class="searchItemIndex === itemIndex ? 'bg-raised text-accent' : 'hover:bg-raised'" :title="t(item.labelKey)">
        <span class="text-xs truncate">
          <template v-for="(part, partIndex) in highlightParts(t(item.labelKey), query)" :key="`l-${partIndex}`">
            <mark v-if="part.hit" class="bg-accent-bg text-accent rounded px-0.5">{{ part.text }}</mark><template v-else>{{ part.text }}</template>
          </template>
        </span>
        <span class="text-[10px] text-ink-muted truncate">{{ categoryLabel(item.category) }}</span>
      </button>
    </div>

    <p v-if="noMatches" class="px-2 py-5 text-center text-xs text-ink-muted leading-5">
      {{ t("settings.search.empty") }}<br /><span class="text-[11px]">{{ t("settings.search.empty.hint") }}</span>
    </p>
    </div>
  </div>
</template>
