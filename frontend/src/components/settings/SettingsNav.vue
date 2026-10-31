<script setup lang="ts">
/**
 * SettingsNav — grouped settings rail plus the settings search.
 *
 * Four groups come from `SETTINGS_GROUPS`; every leaf is a real button with
 * `data-settings-leaf` and `aria-current="page"` so deep links, tests and
 * assistive tech share one source of truth. The rail is vertical on desktop
 * and a horizontal scroll strip on phones.
 */
import { computed, onMounted, ref, watch, type Component } from "vue";
import {
  BarChart3,
  FileCode2,
  GitCompare,
  Globe,
  Info,
  Keyboard,
  KeyRound,
  Palette,
  ShieldCheck,
  Sparkles,
  Terminal,
  User,
  Wrench,
} from "lucide-vue-next";
import { t } from "../../i18n";
import {
  SETTINGS_CATEGORY_LABEL_KEYS,
  SETTINGS_LEAF_KEYWORDS,
  highlightParts,
} from "../../utils/settingsIndex";
import { SETTINGS_GROUPS, type SettingsLeaf } from "../../utils/settingsStructure";
import SettingsSearch from "./SettingsSearch.vue";

const props = defineProps<{ activeLeaf: SettingsLeaf }>();
const emit = defineEmits<{ (e: "select-leaf", leaf: SettingsLeaf): void }>();

const query = ref("");
const navigationRef = ref<HTMLElement | null>(null);

function revealActiveLeaf() {
  navigationRef.value?.querySelector<HTMLElement>('[aria-current="page"]')
    ?.scrollIntoView?.({ block: "nearest", inline: "nearest" });
}

const SETTINGS_LEAF_ICONS: Record<SettingsLeaf, Component> = {
  appearance: Palette,
  identity: User,
  shortcuts: Keyboard,
  signing: ShieldCheck,
  tools: GitCompare,
  aliases: Terminal,
  hooks: FileCode2,
  "ai-connection": KeyRound,
  "ai-behavior": Sparkles,
  "ai-insights": BarChart3,
  proxy: Globe,
  config: Wrench,
  about: Info,
};

function matchesQuery(leaf: SettingsLeaf): boolean {
  const needle = query.value.trim().toLowerCase();
  if (!needle) return true;
  const labelKey = SETTINGS_CATEGORY_LABEL_KEYS[leaf];
  if (t(labelKey).toLowerCase().includes(needle)) return true;
  return SETTINGS_LEAF_KEYWORDS[leaf].some((keyword) => keyword.toLowerCase().includes(needle));
}

const visibleGroups = computed(() =>
  SETTINGS_GROUPS.map((group) => ({
    ...group,
    leaves: group.leaves.filter(matchesQuery),
  })).filter((group) => group.leaves.length > 0)
);

function selectLeaf(leaf: SettingsLeaf) {
  query.value = "";
  emit("select-leaf", leaf);
}

// Deep links (command palette, shortcuts) must reveal the target leaf even if
// the previous search filtered it out.
watch(
  () => props.activeLeaf,
  () => { query.value = ""; }
);
watch([() => props.activeLeaf, query], () => {
    if (!query.value) revealActiveLeaf();
}, { flush: "post" });
onMounted(revealActiveLeaf);
</script>

<template>
  <div class="min-h-0 flex flex-1 flex-col">
    <SettingsSearch v-model="query" @select-leaf="selectLeaf" />
    <div data-testid="settings-navigation-scroll" class="min-h-0 flex-1 overflow-y-auto pb-4">
      <nav ref="navigationRef" data-settings-nav :aria-label="t('settings.nav.label')" class="flex flex-row md:flex-col gap-wide md:gap-5 overflow-x-auto md:overflow-visible scrollbar-none px-3">
        <section
  v-for="group in visibleGroups"
          :key="group.id"
  role="group"
          :aria-labelledby="`settings-group-${group.id}`"
  class="shrink-0 md:w-full"
      >
          <h3 :id="`settings-group-${group.id}`" class="hidden md:block px-2 pb-1.5 text-[10px] font-medium text-ink-muted">{{ t(group.labelKey) }}</h3>
          <div class="flex flex-row md:flex-col gap-0.5">
            <button
              v-for="leaf in group.leaves"
              :key="leaf"
              type="button"
              :data-settings-leaf="leaf"
              :aria-current="activeLeaf === leaf ? 'page' : undefined"
              :title="t(SETTINGS_CATEGORY_LABEL_KEYS[leaf])"
              @click="selectLeaf(leaf)"
              class="flex items-center gap-2.5 min-w-0 shrink-0 md:w-full px-2.5 py-2 rounded-lg text-xs transition cursor-pointer text-left whitespace-nowrap"
              :class="activeLeaf === leaf ? 'bg-accent-bg text-accent font-medium' : 'text-ink-secondary hover:text-ink hover:bg-raised'"
            >
              <component :is="SETTINGS_LEAF_ICONS[leaf]" class="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
              <span>
                <template v-for="(part, partIndex) in highlightParts(t(SETTINGS_CATEGORY_LABEL_KEYS[leaf]), query)" :key="`leaf-${partIndex}`">
                  <mark v-if="part.hit" class="bg-accent-bg text-accent rounded px-0.5">{{ part.text }}</mark><template v-else>{{ part.text }}</template>
                </template>
              </span>
            </button>
          </div>
        </section>
      </nav>
    </div>
  </div>
</template>
