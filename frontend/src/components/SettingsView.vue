<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from "vue";
import type { Component } from "vue";
import {
  User,
  ShieldCheck,
  Globe,
  GitCompare,
  Keyboard,
  Wrench,
  Palette,
  Search,
} from "lucide-vue-next";
import AppearanceSection from "./settings/AppearanceSection.vue";
import IdentitySection from "./settings/IdentitySection.vue";
import SigningSection from "./settings/SigningSection.vue";
import ProxySection from "./settings/ProxySection.vue";
import ToolsSection from "./settings/ToolsSection.vue";
import ShortcutsSection from "./settings/ShortcutsSection.vue";
import AdvancedConfigSection from "./settings/AdvancedConfigSection.vue";
import Toast from "./ui/Toast.vue";
import { useToast } from "../composables/useToast";
import {
  SETTINGS_CATEGORY_LABELS,
  SETTINGS_ITEM_INDEX,
  type SettingsIndexItem,
} from "../utils/settingsIndex";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

type SettingsCategory = "appearance" | "identity" | "signing" | "proxy" | "tools" | "config" | "shortcuts";

const activeCategory = ref<SettingsCategory>("appearance");

const SETTINGS_CATEGORIES: readonly SettingsCategory[] = [
  "appearance",
  "identity",
  "signing",
  "proxy",
  "tools",
  "config",
  "shortcuts",
];

interface SettingsCategoryMeta {
  id: SettingsCategory;
  label: string;
  icon: Component;
  keywords: string[];
}

const SETTINGS_CATEGORY_META: SettingsCategoryMeta[] = [
  { id: "appearance", label: "外观与主题", icon: Palette, keywords: ["主题", "配色", "字体", "外观", "appearance", "theme", "color"] },
  { id: "identity", label: "Git 身份认证", icon: User, keywords: ["身份", "用户名", "邮箱", "认证", "identity", "user", "email", "name"] },
  { id: "signing", label: "GPG / SSH 防伪签名", icon: ShieldCheck, keywords: ["签名", "密钥", "秘钥", "gpg", "ssh", "signing", "sign"] },
  { id: "proxy", label: "网络与加速代理", icon: Globe, keywords: ["代理", "网络", "加速", "proxy", "http", "socks"] },
  { id: "tools", label: "外部对比与合并工具", icon: GitCompare, keywords: ["对比", "合并", "外部工具", "beyond", "compare", "merge", "diff", "tool"] },
  { id: "config", label: "高级 Git 配置", icon: Wrench, keywords: ["配置", "高级", "config", "core", "设置"] },
  { id: "shortcuts", label: "快捷键与架构信息", icon: Keyboard, keywords: ["快捷键", "键盘", "键位", "shortcut", "keybinding", "架构", "版本", "keys"] },
];

const settingsSearch = ref("");

// Recently used settings queries (persisted).
const SETTINGS_SEARCH_HISTORY_KEY = "wgit.settings.searchHistory.v1";

function loadSettingsSearchHistory(): string[] {
  try {
    const raw = localStorage.getItem(SETTINGS_SEARCH_HISTORY_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed)
      ? parsed.filter((item): item is string => typeof item === "string").slice(0, 5)
      : [];
  } catch {
    return [];
  }
}

const settingsSearchHistory = ref<string[]>(loadSettingsSearchHistory());
const settingsSearchFocused = ref(false);

function rememberSettingsQuery(query: string) {
  const trimmed = query.trim();
  if (!trimmed) return;
  const next = [trimmed, ...settingsSearchHistory.value.filter((item) => item !== trimmed)].slice(0, 5);
  settingsSearchHistory.value = next;
  try {
    localStorage.setItem(SETTINGS_SEARCH_HISTORY_KEY, JSON.stringify(next));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

function applySettingsHistoryQuery(query: string) {
  settingsSearch.value = query;
  settingsSearchFocused.value = true;
}

function clearSettingsSearchHistory() {
  settingsSearchHistory.value = [];
  try {
    localStorage.removeItem(SETTINGS_SEARCH_HISTORY_KEY);
  } catch {
    /* ignore */
  }
}

const filteredCategories = computed(() => {
  const query = settingsSearch.value.trim().toLowerCase();
  if (!query) return SETTINGS_CATEGORY_META;
  return SETTINGS_CATEGORY_META.filter((category) => {
    if (category.label.toLowerCase().includes(query)) return true;
    return category.keywords.some((keyword) => keyword.toLowerCase().includes(query));
  });
});

function selectFirstFilteredCategory() {
  const first = filteredCategories.value[0];
  if (first) {
    rememberSettingsQuery(settingsSearch.value);
    activeCategory.value = first.id;
  }
}

/** Item-level search results (e.g. "邮箱" → Git 身份认证 · Git 邮箱). */
const matchedSettingItems = computed(() => {
  const query = settingsSearch.value.trim().toLowerCase();
  if (!query) return [];
  return SETTINGS_ITEM_INDEX.filter(
    (item) =>
      item.label.toLowerCase().includes(query) ||
      item.keywords.some((keyword) => keyword.toLowerCase().includes(query))
  ).slice(0, 8);
});

function openSettingItem(item: SettingsIndexItem) {
  rememberSettingsQuery(settingsSearch.value);
  activeCategory.value = item.category as SettingsCategory;
  settingsSearch.value = "";
  notify(`已定位到「${item.label}」· ${SETTINGS_CATEGORY_LABELS[item.category] ?? item.category}`);
}

// Keyboard navigation + match highlighting for the settings search.
const searchItemIndex = ref(0);
const searchHistoryIndex = ref(0);

const searchHistoryVisible = computed(
  () =>
    !settingsSearch.value.trim() &&
    settingsSearchFocused.value &&
    settingsSearchHistory.value.length > 0
);

watch(settingsSearch, () => {
  searchItemIndex.value = 0;
  searchHistoryIndex.value = 0;
});

watch(settingsSearchHistory, (history) => {
  if (searchHistoryIndex.value >= history.length) searchHistoryIndex.value = 0;
});

watch(matchedSettingItems, (items) => {
  if (searchItemIndex.value >= items.length) searchItemIndex.value = 0;
});

function moveSearchSelection(delta: number) {
  if (searchHistoryVisible.value) {
    const history = settingsSearchHistory.value;
    searchHistoryIndex.value = (searchHistoryIndex.value + delta + history.length) % history.length;
    return;
  }
  const items = matchedSettingItems.value;
  if (items.length === 0) return;
  searchItemIndex.value = (searchItemIndex.value + delta + items.length) % items.length;
}

function activateSearchSelection() {
  if (searchHistoryVisible.value) {
    const query = settingsSearchHistory.value[searchHistoryIndex.value];
    if (query) applySettingsHistoryQuery(query);
    return;
  }
  const items = matchedSettingItems.value;
  if (items.length > 0) {
    openSettingItem(items[searchItemIndex.value]!);
    return;
  }
  selectFirstFilteredCategory();
}

/** Split text into plain/highlighted segments for the active settings query. */
function highlightParts(text: string): Array<{ text: string; hit: boolean }> {
  const query = settingsSearch.value.trim();
  if (!query) return [{ text, hit: false }];
  const haystack = text.toLowerCase();
  const needle = query.toLowerCase();
  const parts: Array<{ text: string; hit: boolean }> = [];
  let cursor = 0;
  while (cursor < text.length) {
    const match = haystack.indexOf(needle, cursor);
    if (match < 0) {
      parts.push({ text: text.slice(cursor), hit: false });
      break;
    }
    if (match > cursor) parts.push({ text: text.slice(cursor, match), hit: false });
    parts.push({ text: text.slice(match, match + needle.length), hit: true });
    cursor = match + needle.length;
  }
  return parts.length > 0 ? parts : [{ text, hit: false }];
}

/** Command palette / cross-view jumps: switch to a concrete settings section. */
function handleSettingsSection(e: Event) {
  const detail = (e as CustomEvent).detail as { section?: string } | undefined;
  const section = detail?.section;
  if (section && (SETTINGS_CATEGORIES as readonly string[]).includes(section)) {
    activeCategory.value = section as SettingsCategory;
  }
}

onMounted(() => window.addEventListener("wgit:settings-section", handleSettingsSection));
onUnmounted(() => window.removeEventListener("wgit:settings-section", handleSettingsSection));

const { toastText, showToast, notify } = useToast();
</script>

<template>
  <div class="flex-1 flex overflow-hidden bg-canvas relative select-none">
    <!-- Left Navigation Categories -->
    <div class="w-56 border-r border-line bg-surface/30 p-3 flex flex-col gap-1 shrink-0">
      <div class="px-3 py-2 text-[10px] font-mono uppercase tracking-wider text-ink-muted">
        首选项设置
      </div>

      <div class="relative mb-1">
        <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
        <input
          v-model="settingsSearch"
          type="text"
          spellcheck="false"
          placeholder="搜索设置项…"
          class="w-full pl-8 pr-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          @keydown.enter.prevent="activateSearchSelection"
          @keydown.down.prevent="moveSearchSelection(1)"
          @keydown.up.prevent="moveSearchSelection(-1)"
          @focus="settingsSearchFocused = true"
          @blur="settingsSearchFocused = false"
        />
      </div>

      <div
        v-if="!settingsSearch.trim() && settingsSearchFocused && settingsSearchHistory.length > 0"
        class="mb-1 rounded-lg border border-line bg-canvas/50 overflow-hidden"
      >
        <div class="flex items-center justify-between px-2.5 pt-1.5 pb-0.5">
          <span class="text-[9px] font-semibold text-ink-muted tracking-wider">最近搜索</span>
          <button
            @mousedown.prevent
            @click="clearSettingsSearchHistory"
            class="text-[9px] text-ink-muted/70 hover:text-danger transition cursor-pointer"
            title="清空最近搜索"
          >
            清空
          </button>
        </div>
        <button
          v-for="(query, historyIndex) in settingsSearchHistory"
          :key="query"
          @mousedown.prevent
          @click="applySettingsHistoryQuery(query)"
          @mouseenter="searchHistoryIndex = historyIndex"
          class="w-full flex items-center gap-1.5 px-2.5 py-1 text-left text-[11px] transition cursor-pointer"
          :class="searchHistoryIndex === historyIndex ? 'bg-raised text-accent' : 'text-ink-secondary hover:bg-raised hover:text-ink'"
        >
          <Search class="w-3 h-3 shrink-0 text-ink-muted" />
          <span class="truncate">{{ query }}</span>
        </button>
      </div>

      <div v-if="settingsSearch.trim()" class="mb-1">
        <div
          v-if="matchedSettingItems.length > 0"
          class="rounded-lg border border-line bg-canvas/50 overflow-hidden divide-y divide-line/50"
        >
          <button
            v-for="(item, itemIndex) in matchedSettingItems"
            :key="`${item.category}-${item.label}`"
            @click="openSettingItem(item)"
            @mouseenter="searchItemIndex = itemIndex"
            class="w-full flex flex-col gap-0.5 px-2.5 py-1.5 text-left transition cursor-pointer"
            :class="searchItemIndex === itemIndex ? 'bg-raised text-accent' : 'hover:bg-raised'"
            :title="item.label"
          >
            <span class="text-[11px] truncate">
              <template v-for="(part, partIndex) in highlightParts(item.label)" :key="`l-${partIndex}`">
                <mark v-if="part.hit" class="bg-accent/25 text-accent rounded-[3px] px-0.5">{{ part.text }}</mark>
                <template v-else>{{ part.text }}</template>
              </template>
            </span>
            <span class="text-[9px] text-ink-muted truncate">{{ SETTINGS_CATEGORY_LABELS[item.category] }}</span>
          </button>
        </div>
        <p
          v-else-if="filteredCategories.length === 0"
          class="px-1 py-3 text-center text-[10px] text-ink-muted"
        >
          无匹配设置项
        </p>
      </div>

      <button
        v-for="category in filteredCategories"
        :key="category.id"
        @click="activeCategory = category.id"
        class="flex items-center gap-2.5 px-3 py-2 rounded-xl text-xs font-medium transition cursor-pointer text-left"
        :class="activeCategory === category.id ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-secondary hover:text-ink hover:bg-hover'"
      >
        <component :is="category.icon" class="w-4 h-4 shrink-0" />
        <span class="truncate">
          <template v-for="(part, partIndex) in highlightParts(category.label)" :key="`c-${partIndex}`">
            <mark v-if="part.hit" class="bg-accent/25 text-accent rounded-[3px] px-0.5">{{ part.text }}</mark>
            <template v-else>{{ part.text }}</template>
          </template>
        </span>
      </button>

      <div
        v-if="filteredCategories.length === 0 && matchedSettingItems.length === 0"
        class="px-3 py-6 text-center text-[11px] text-ink-muted leading-relaxed"
      >
        无匹配设置项<br />
        <span class="text-[10px]">试试「代理」「签名」「快捷键」</span>
      </div>
    </div>

    <!-- Right Settings Panel -->
    <div class="flex-1 overflow-y-auto p-8 max-w-3xl">

      <AppearanceSection v-if="activeCategory === 'appearance'" @notify="notify" />
      <IdentitySection v-if="activeCategory === 'identity'" :current-repo-path="currentRepoPath" @notify="notify" />
      <SigningSection v-if="activeCategory === 'signing'" :current-repo-path="currentRepoPath" @notify="notify" />
      <ProxySection v-if="activeCategory === 'proxy'" :current-repo-path="currentRepoPath" @notify="notify" />
      <ToolsSection v-if="activeCategory === 'tools'" :current-repo-path="currentRepoPath" @notify="notify" />
      <AdvancedConfigSection v-if="activeCategory === 'config'" :current-repo-path="currentRepoPath" @notify="notify" />
      <ShortcutsSection v-if="activeCategory === 'shortcuts'" />

    </div>

    <Toast :text="toastText" :visible="showToast" />
  </div>
</template>

