<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from "vue";
import { t } from "../i18n";
import UiModal from "./ui/UiModal.vue";
import type { Component } from "vue";
import { Search, Star, Clock, ChevronUp, ChevronDown, ChevronRight } from "lucide-vue-next";
import { COMMAND_REGISTRY, type CommandRunContext } from "../commands/registry";

const props = defineProps<{
  isOpen: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "action", actionId: string): void;
}>();

/**
 * Command data lives in the app-wide registry (src/commands/registry.ts); this
 * component only renders, filters, and executes it. Legacy inline entries such
 * as id: "settings-safe-directory" / title: "cmd.settings-safe-directory" and
 * id: "tools-engine-meta" / title: "cmd.tools-engine-meta" are registered there
 * now and reach this list through COMMAND_REGISTRY below.
 */

/** 命令面板条目：registry 条目的渲染投影（title = i18n key，category = 分组 key）。 */
interface PaletteCommand {
  id: string;
  title: string;
  category: string;
  icon: Component;
  run: (ctx: CommandRunContext) => void;
}

const commands: PaletteCommand[] = COMMAND_REGISTRY.map((entry) => ({
  id: entry.id,
  title: entry.labelKey,
  category: entry.group,
  icon: entry.icon,
  run: entry.run,
}));

const searchQuery = ref("");
const searchInput = ref<HTMLInputElement | null>(null);
const listRef = ref<HTMLElement | null>(null);
const selectedIndex = ref(0);

// ---- Recently used commands (MRU, persisted) ----
/**
 * Commands store i18n keys, not translated text.
 *
 * `commands` is a module-level array, so calling `t()` while building it would
 * freeze the labels in whatever language was active at import time. Resolving
 * lazily also keeps the persisted category-collapse list stable across a
 * language switch, because the stored value is the category key, not its label.
 */
function titleOf(cmd: PaletteCommand): string {
  return t(cmd.title);
}

function categoryLabel(category: string): string {
  return t(`cmd.cat.${category}`);
}

const RECENT_KEY = "wgit.palette.recent.v1";
const MAX_RECENT = 8;

function loadRecentIds(): string[] {
  try {
    const raw = localStorage.getItem(RECENT_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed)
      ? parsed.filter((id): id is string => typeof id === "string").slice(0, MAX_RECENT)
      : [];
  } catch {
    // Benign: unreadable/corrupt recents just start the list empty.
    return [];
  }
}

const recentIds = ref<string[]>(loadRecentIds());

function rememberCommand(cmd: PaletteCommand) {
  recentIds.value = [cmd.id, ...recentIds.value.filter((id) => id !== cmd.id)].slice(0, MAX_RECENT);
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(recentIds.value));
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

function commandById(id: string): PaletteCommand | undefined {
  return commands.find((c) => c.id === id);
}

// ---- Pinned commands (persisted) ----
const PIN_KEY = "wgit.palette.pinned.v1";
const MAX_PINNED = 6;

function loadPinnedIds(): string[] {
  try {
    const raw = localStorage.getItem(PIN_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed)
      ? parsed.filter((id): id is string => typeof id === "string").slice(0, MAX_PINNED)
      : [];
  } catch {
    // Benign: unreadable/corrupt pins just start the list empty.
    return [];
  }
}

const pinnedIds = ref<string[]>(loadPinnedIds());

function persistPinned() {
  try {
    localStorage.setItem(PIN_KEY, JSON.stringify(pinnedIds.value));
  } catch {
    /* ignore quota / privacy-mode errors — pins just stay unpersisted */
  }
}

function isPinned(id: string): boolean {
  return pinnedIds.value.includes(id);
}

/** Move a pinned command up/down inside the 收藏 section (persisted). */
function movePinned(id: string, delta: number) {
  const index = pinnedIds.value.indexOf(id);
  const target = index + delta;
  if (index < 0 || target < 0 || target >= pinnedIds.value.length) return;
  const next = [...pinnedIds.value];
  const [item] = next.splice(index, 1);
  next.splice(target, 0, item!);
  pinnedIds.value = next;
  persistPinned();
  selectedIndex.value = target;
  void nextTick(scrollSelectedIntoView);
}

const draggingPinId = ref<string | null>(null);
const dragOverPinId = ref<string | null>(null);

function startPinDrag(id: string, event: DragEvent) {
  draggingPinId.value = id;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", id);
  }
}

function dragOverPin(id: string) {
  if (draggingPinId.value && draggingPinId.value !== id) dragOverPinId.value = id;
}

function dropPin(targetId: string, event: DragEvent) {
  event.preventDefault();
  const sourceId = draggingPinId.value || event.dataTransfer?.getData("text/plain") || "";
  draggingPinId.value = null;
  dragOverPinId.value = null;
  if (!sourceId || sourceId === targetId) return;
  const from = pinnedIds.value.indexOf(sourceId);
  const to = pinnedIds.value.indexOf(targetId);
  if (from < 0 || to < 0) return;
  const next = [...pinnedIds.value];
  next.splice(from, 1);
  next.splice(to, 0, sourceId);
  pinnedIds.value = next;
  persistPinned();
  selectedIndex.value = to;
  void nextTick(scrollSelectedIntoView);
}

function endPinDrag() {
  draggingPinId.value = null;
  dragOverPinId.value = null;
}

const allPaletteCategories = computed(() => [...new Set(commands.map((c) => c.category))]);

const allCategoriesCollapsed = computed(
  () =>
    allPaletteCategories.value.length > 0 &&
    allPaletteCategories.value.every((category) => isCategoryCollapsed(category))
);

function toggleAllCategories() {
  collapsedCategories.value = allCategoriesCollapsed.value
    ? []
    : [...allPaletteCategories.value];
  persistCollapsed();
  selectedIndex.value = 0;
  void nextTick(scrollSelectedIntoView);
}

function togglePin(cmd: PaletteCommand) {
  if (isPinned(cmd.id)) {
    pinnedIds.value = pinnedIds.value.filter((id) => id !== cmd.id);
  } else {
    pinnedIds.value = [cmd.id, ...pinnedIds.value].slice(0, MAX_PINNED);
  }
  persistPinned();
}

// ---- Collapsible categories (persisted) ----
const COLLAPSE_KEY = "wgit.palette.collapsed.v1";

function loadCollapsedCategories(): string[] {
  try {
    const raw = localStorage.getItem(COLLAPSE_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed)
      ? parsed.filter((c): c is string => typeof c === "string")
      : [];
  } catch {
    // Benign: unreadable/corrupt collapse prefs just start everything expanded.
    return [];
  }
}

const collapsedCategories = ref<string[]>(loadCollapsedCategories());

function persistCollapsed() {
  try {
    localStorage.setItem(COLLAPSE_KEY, JSON.stringify(collapsedCategories.value));
  } catch {
    /* ignore quota / privacy-mode errors — collapse state just stays unpersisted */
  }
}

function isCategoryCollapsed(category: string): boolean {
  return collapsedCategories.value.includes(category);
}

function toggleCategory(category: string) {
  if (isCategoryCollapsed(category)) {
    collapsedCategories.value = collapsedCategories.value.filter((c) => c !== category);
  } else {
    collapsedCategories.value = [...collapsedCategories.value, category];
  }
  persistCollapsed();
  selectedIndex.value = 0;
  void nextTick(scrollSelectedIntoView);
}

function clearRecents() {
  recentIds.value = [];
  try {
    localStorage.removeItem(RECENT_KEY);
  } catch {
    // ignore storage issues — the in-memory recents list is already cleared
  }
  selectedIndex.value = 0;
}

const isSearching = computed(() => !!searchQuery.value.trim());

function fuzzySubsequenceMatch(text: string, query: string): boolean {
  let ti = 0;
  let qi = 0;
  while (ti < text.length && qi < query.length) {
    if (text[ti] === query[qi]) {
      qi++;
    }
    ti++;
  }
  return qi === query.length;
}

/** Search results grouped by category with ranking: pinned → recent → prefix match →
 *  title match → category-only match. Categories are ordered by their best hit. */
const searchGroups = computed<{ category: string; cmds: PaletteCommand[] }[]>(() => {
  if (!isSearching.value) return [];
  const q = searchQuery.value.trim().toLowerCase();
  const scores = new Map<string, number>();
  for (const cmd of commands) {
    const title = titleOf(cmd).toLowerCase();
    const category = categoryLabel(cmd.category).toLowerCase();
    const idClean = cmd.id.toLowerCase().replace(/[^a-z0-9]/g, "");
    const titleMatch = title.includes(q);
    const categoryMatch = category.includes(q);
    const fuzzyIdMatch = fuzzySubsequenceMatch(idClean, q);
    const fuzzyTitleMatch = fuzzySubsequenceMatch(title, q);

    if (!titleMatch && !categoryMatch && !fuzzyIdMatch && !fuzzyTitleMatch) continue;
    let score = 0;
    if (isPinned(cmd.id)) score -= 100;
    else if (recentIds.value.includes(cmd.id)) score -= 50;
    if (title.startsWith(q)) score -= 20;
    else if (titleMatch) score -= 10;
    else if (fuzzyIdMatch) score -= 5;
    scores.set(cmd.id, score);
  }
  const categoryOrder = [...new Set(commands.map((c) => c.category))];
  const groups = categoryOrder
    .map((category) => {
      const cmds = commands
        .filter((c) => c.category === category && scores.has(c.id))
        .sort(
          (a, b) =>
            scores.get(a.id)! - scores.get(b.id)! ||
            commands.indexOf(a) - commands.indexOf(b)
        );
      const best = cmds.length ? Math.min(...cmds.map((c) => scores.get(c.id)!)) : 0;
      return { category, cmds, best };
    })
    .filter((g) => g.cmds.length > 0);
  groups.sort(
    (a, b) => a.best - b.best || categoryOrder.indexOf(a.category) - categoryOrder.indexOf(b.category)
  );
  return groups.map((g) => ({ category: g.category, cmds: g.cmds }));
});

const searchFlat = computed(() => searchGroups.value.flatMap((group) => group.cmds));

/** Flat order used by keyboard navigation (pinned → recent → remaining categories). */
const filteredCommands = computed<PaletteCommand[]>(() => {
  if (isSearching.value) return searchFlat.value;
  const pinned = pinnedIds.value
    .map((id) => commandById(id))
    .filter((c): c is PaletteCommand => !!c);
  const pinnedSet = new Set(pinned.map((c) => c.id));
  const recents = recentIds.value
    .map((id) => commandById(id))
    .filter((c): c is PaletteCommand => !!c && !pinnedSet.has(c.id));
  const recentSet = new Set(recents.map((c) => c.id));
  const rest = commands.filter(
    (c) => !pinnedSet.has(c.id) && !recentSet.has(c.id) && !isCategoryCollapsed(c.category)
  );
  return [...pinned, ...recents, ...rest];
});

type PaletteSection = {
  category: string;
  items: { cmd: PaletteCommand; index: number }[];
  recent?: boolean;
  pinned?: boolean;
  collapsed?: boolean;
  fullCount?: number;
};

/** 按分类分组成带标题的 section；index 仍是扁平序号，保证键盘导航不断。 */
const commandSections = computed<PaletteSection[]>(() => {
  const sections: PaletteSection[] = [];
  if (isSearching.value) {
    // Search results: ranked category groups, no pinned/recent/collapse sections.
    let searchIndex = 0;
    for (const group of searchGroups.value) {
      sections.push({
        category: group.category,
        items: group.cmds.map((cmd) => ({ cmd, index: searchIndex++ })),
      });
    }
    return sections;
  }

  const pinned = pinnedIds.value
    .map((id) => commandById(id))
    .filter((c): c is PaletteCommand => !!c);
  const pinnedSet = new Set(pinned.map((c) => c.id));
  const recents = recentIds.value
    .map((id) => commandById(id))
    .filter((c): c is PaletteCommand => !!c && !pinnedSet.has(c.id));
  const recentSet = new Set(recents.map((c) => c.id));

  let index = 0;
  if (pinned.length > 0) {
    sections.push({
      category: "pinned",
      pinned: true,
      items: pinned.map((cmd) => ({ cmd, index: index++ })),
    });
  }
  if (recents.length > 0) {
    sections.push({
      category: "recent",
      recent: true,
      items: recents.map((cmd) => ({ cmd, index: index++ })),
    });
  }

  const rest = commands.filter((c) => !pinnedSet.has(c.id) && !recentSet.has(c.id));
  const categoryOrder: string[] = [];
  const grouped = new Map<string, PaletteCommand[]>();
  for (const cmd of rest) {
    if (!grouped.has(cmd.category)) {
      grouped.set(cmd.category, []);
      categoryOrder.push(cmd.category);
    }
    grouped.get(cmd.category)!.push(cmd);
  }
  for (const category of categoryOrder) {
    const cmds = grouped.get(category)!;
    if (isCategoryCollapsed(category)) {
      sections.push({ category, items: [], collapsed: true, fullCount: cmds.length });
    } else {
      sections.push({
        category,
        items: cmds.map((cmd) => ({ cmd, index: index++ })),
      });
    }
  }
  return sections;
});

/** Split a label into plain/highlighted segments for the active search query. */
function titleParts(title: string): Array<{ text: string; hit: boolean }> {
  const query = searchQuery.value.trim();
  if (!query) return [{ text: title, hit: false }];
  const haystack = title.toLowerCase();
  const needle = query.toLowerCase();
  const parts: Array<{ text: string; hit: boolean }> = [];
  let cursor = 0;
  while (cursor < title.length) {
    const match = haystack.indexOf(needle, cursor);
    if (match < 0) {
      parts.push({ text: title.slice(cursor), hit: false });
      break;
    }
    if (match > cursor) parts.push({ text: title.slice(cursor, match), hit: false });
    parts.push({ text: title.slice(match, match + needle.length), hit: true });
    cursor = match + needle.length;
  }
  return parts.length > 0 ? parts : [{ text: title, hit: false }];
}

function scrollSelectedIntoView() {
  const el = listRef.value?.querySelector<HTMLElement>(`[data-idx="${selectedIndex.value}"]`);
  el?.scrollIntoView({ block: "nearest" });
}

function executeCommand(cmd: PaletteCommand) {
  rememberCommand(cmd);
  // registry 条目只复用两条既有执行通道；执行上下文在这里组装，
  // 面板本身不感知任何具体命令。
  cmd.run({
    emitAction: (actionId) => emit("action", actionId),
    dispatchWindowEvent: (eventName, detail) => {
      window.dispatchEvent(new CustomEvent(eventName, { detail }));
    },
  });
  emit("close");
}

function moveSelection(delta: number) {
  const len = filteredCommands.value.length;
  if (len === 0) return;
  selectedIndex.value = (selectedIndex.value + delta + len) % len;
  void nextTick(scrollSelectedIntoView);
}

function handleKeydown(e: KeyboardEvent) {
  if (!props.isOpen) return;
  if (e.isComposing) return;

  if (e.key === "ArrowDown") {
    e.preventDefault();
    moveSelection(1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    moveSelection(-1);
  } else if (e.key === "PageDown") {
    e.preventDefault();
    moveSelection(5);
  } else if (e.key === "PageUp") {
    e.preventDefault();
    moveSelection(-5);
  } else if (e.key === "Home" && !searchQuery.value) {
    e.preventDefault();
    selectedIndex.value = 0;
    void nextTick(scrollSelectedIntoView);
  } else if (e.key === "End" && !searchQuery.value) {
    e.preventDefault();
    selectedIndex.value = Math.max(0, filteredCommands.value.length - 1);
    void nextTick(scrollSelectedIntoView);
  } else if (e.key === "Enter") {
    e.preventDefault();
    const cmd = filteredCommands.value[selectedIndex.value];
    if (cmd) executeCommand(cmd);
  } else if (e.key === "Escape") {
    emit("close");
  }
}

// 输入即过滤：选中项回到首条并滚回顶部。
watch(searchQuery, () => {
  selectedIndex.value = 0;
  void nextTick(scrollSelectedIntoView);
});

// 每次打开都重置搜索与选中，并聚焦输入框。
watch(
  () => props.isOpen,
  (open) => {
    if (!open) return;
    searchQuery.value = "";
    selectedIndex.value = 0;
    void nextTick(() => {
      searchInput.value?.focus();
      scrollSelectedIntoView();
    });
  }
);

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleKeydown);
});
</script>

<template>
  <UiModal
    :open="isOpen"
    variant="bare"
    align="top"
    width="max-w-[560px]"
    :label="t('cmd.aria')"
    panel-class="select-none animate-in fade-in duration-150"
    close-on-backdrop
    @update:open="(open) => { if (!open) emit('close'); }"
  >
      <!-- Search Input -->
      <div class="p-3 border-b border-line flex shrink-0 items-center gap-2.5">
        <Search class="w-4 h-4 text-ink-muted ml-1 shrink-0" />
        <input
          ref="searchInput"
          v-model="searchQuery"
          type="text"
          :placeholder="t('cmd.placeholder')"
        :aria-label="t('cmd.placeholder')"
          autofocus
          spellcheck="false"
          autocomplete="off"
          role="combobox"
          aria-autocomplete="list"
          aria-expanded="true"
          :aria-controls="'palette-listbox'"
        :aria-activedescendant="filteredCommands.length > 0 ? 'palette-item-' + selectedIndex : undefined"
          class="flex-1 min-w-0 bg-transparent border-none text-sm text-ink placeholder-ink-muted focus:outline-none font-sans"
        />
        <span v-if="searchQuery.trim()" class="text-[10px] text-ink-muted font-mono shrink-0">
          {{ t("cmd.count", { count: filteredCommands.length }) }}
        </span>
        <kbd class="px-1.5 py-0.5 rounded bg-raised border border-line text-[10px] text-ink-muted font-mono shrink-0">
          ESC
        </kbd>
      </div>

      <!-- Grouped Command List -->
      <div
        id="palette-listbox"
        ref="listRef"
        role="listbox"
        :aria-label="t('cmd.aria.list')"
        class="max-h-[420px] overflow-y-auto p-2"
      >
        <div v-for="section in commandSections" :key="section.category" class="mb-0.5 last:mb-0">
          <!-- Category Header (粘性吸顶；普通分类可点击折叠) -->
          <div
            class="flex items-center gap-2 px-3 pt-1.5 pb-1 select-none sticky top-0 z-10 bg-surface"
            :class="!section.pinned && !section.recent ? 'cursor-pointer group/header' : ''"
            :title="
              !section.pinned && !section.recent
                ? t(section.collapsed ? 'cmd.category.expand' : 'cmd.category.collapse', {
                    category: categoryLabel(section.category),
                  })
                : ''
            "
            @click="!section.pinned && !section.recent ? toggleCategory(section.category) : undefined"
          >
            <Star v-if="section.pinned" class="w-3 h-3 text-warning shrink-0" fill="currentColor" />
            <Clock v-else-if="section.recent" class="w-3 h-3 text-accent shrink-0" />
            <ChevronRight
              v-else-if="section.collapsed"
              class="w-3 h-3 text-ink-muted shrink-0 group-hover/header:text-ink transition"
            />
            <ChevronDown
              v-else
              class="w-3 h-3 text-ink-muted shrink-0 group-hover/header:text-ink transition"
            />
            <span
              class="text-[10px] font-semibold tracking-wider"
              :class="section.pinned ? 'text-warning' : section.recent ? 'text-accent' : 'text-ink-muted group-hover/header:text-ink'"
            >
              <template v-for="(part, index) in titleParts(categoryLabel(section.category))" :key="`cat-${index}`">
                <mark v-if="part.hit" class="bg-accent-bg-strong text-accent rounded-[3px] px-0.5">{{ part.text }}</mark>
                <template v-else>{{ part.text }}</template>
              </template>
            </span>
            <span class="h-px flex-1 bg-line/60"></span>
            <button
              v-if="section.recent"
              @click.stop="clearRecents"
              class="pointer-events-auto shrink-0 px-1.5 py-0.5 rounded border border-line bg-surface text-[9px] text-ink-muted hover:text-danger hover:border-danger-border transition cursor-pointer"
              :title="t('cmd.recent.clear.title')"
            >
              {{ t("cmd.recent.clear") }}
            </button>
            <span v-if="section.recent" class="text-[9px] text-ink-muted">{{ t("cmd.recent.hint") }}</span>
            <span class="text-[9px] font-mono text-ink-muted">
              {{ section.collapsed ? section.fullCount : section.items.length }}
            </span>
          </div>

          <div class="space-y-px" role="group">
            <div
              v-for="entry in section.items"
              :key="entry.cmd.id"
              :id="'palette-item-' + entry.index"
              role="option"
              :aria-selected="selectedIndex === entry.index"
              :tabindex="selectedIndex === entry.index ? 0 : -1"
              :data-idx="entry.index"
              @click="executeCommand(entry.cmd)"
              @mouseenter="selectedIndex = entry.index"
              class="group/row flex items-center justify-between gap-2 px-3 py-1.5 rounded-lg border-l-2 transition-all duration-100 cursor-pointer"
              :class="[
                selectedIndex === entry.index
                  ? 'bg-raised text-accent font-medium shadow-xs border-accent'
                  : 'text-ink-secondary hover:text-ink border-transparent',
                draggingPinId === entry.cmd.id ? 'opacity-40' : '',
                dragOverPinId === entry.cmd.id && draggingPinId !== entry.cmd.id ? 'ring-1 ring-accent/60' : '',
              ]"
              :draggable="section.pinned ? 'true' : undefined"
              @dragstart="section.pinned ? startPinDrag(entry.cmd.id, $event) : undefined"
              @dragover.prevent="section.pinned ? dragOverPin(entry.cmd.id) : undefined"
              @dragleave="section.pinned && dragOverPinId === entry.cmd.id ? (dragOverPinId = null) : undefined"
              @drop="section.pinned ? dropPin(entry.cmd.id, $event) : undefined"
              @dragend="endPinDrag"
            >
              <div class="flex items-center gap-2.5 min-w-0">
                <component :is="entry.cmd.icon" class="w-3.5 h-3.5 shrink-0" />
                <span class="text-xs truncate">
                  <template v-for="(part, index) in titleParts(titleOf(entry.cmd))" :key="`hit-${index}`">
                    <mark v-if="part.hit" class="bg-accent-bg-strong text-accent rounded-[3px] px-0.5">{{ part.text }}</mark>
                    <template v-else>{{ part.text }}</template>
                  </template>
                </span>
              </div>
              <div class="flex items-center gap-0.5 shrink-0">
                <template v-if="section.pinned">
                  <button :aria-label="t('cmd.pin.up')"
                    @click.stop="movePinned(entry.cmd.id, -1)"
                    :disabled="entry.index === 0"
                    class="p-0.5 rounded text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-25 disabled:cursor-not-allowed"
                    :title="t('cmd.pin.up')"
                  >
                    <ChevronUp class="w-3 h-3" />
                  </button>
                  <button :aria-label="t('cmd.pin.down')"
                    @click.stop="movePinned(entry.cmd.id, 1)"
                    :disabled="entry.index === pinnedIds.length - 1"
                    class="p-0.5 rounded text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-25 disabled:cursor-not-allowed"
                    :title="t('cmd.pin.down')"
                  >
                    <ChevronDown class="w-3 h-3" />
                  </button>
                </template>
                <button
                  :aria-label="isPinned(entry.cmd.id) ? t('cmd.pin.remove') : t('cmd.pin.add')"
                  @click.stop="togglePin(entry.cmd)"
                  class="p-0.5 rounded transition cursor-pointer"
                  :class="
                    isPinned(entry.cmd.id)
                      ? 'text-warning'
                      : 'text-ink-muted/0 group-hover/row:text-ink-muted hover:text-warning'
                  "
                  :title="isPinned(entry.cmd.id) ? t('cmd.pin.remove') : t('cmd.pin.add')"
                >
                  <Star class="w-3 h-3" :fill="isPinned(entry.cmd.id) ? 'currentColor' : 'none'" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="filteredCommands.length === 0" class="py-8 text-center text-xs text-ink-muted">
          {{ t("cmd.empty") }}
        </div>
      </div>

      <!-- Bottom Hint Bar -->
      <div
        class="p-2 border-t border-line bg-canvas text-[10px] text-ink-muted flex shrink-0 items-center justify-between gap-2 px-3 font-mono"
      >
        <span class="truncate">{{ t("cmd.footer.hint") }}</span>
        <span class="flex items-center gap-2 shrink-0">
          <button
            @click.stop="toggleAllCategories"
            class="px-1.5 py-0.5 rounded border border-line bg-surface hover:bg-raised text-[9px] text-ink-muted hover:text-ink transition cursor-pointer"
            :title="allCategoriesCollapsed ? t('cmd.collapse.expand.all') : t('cmd.collapse.collapse.all')"
          >
            {{ allCategoriesCollapsed ? t("cmd.expand.all") : t("cmd.collapse.all") }}
          </button>
          <span>{{ t("cmd.footer.shortcuts") }}</span>
        </span>
      </div>
  </UiModal>
</template>
