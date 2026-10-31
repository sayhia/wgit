<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import AppearanceSection from "./settings/AppearanceSection.vue";
import IdentitySection from "./settings/IdentitySection.vue";
import SigningSection from "./settings/SigningSection.vue";
import ProxySection from "./settings/ProxySection.vue";
import ToolsSection from "./settings/ToolsSection.vue";
import AliasesSection from "./settings/AliasesSection.vue";
import GitHooksSection from "./settings/GitHooksSection.vue";
import ShortcutsSection from "./settings/ShortcutsSection.vue";
import AdvancedConfigSection from "./settings/AdvancedConfigSection.vue";
import AiConnectionSection from "./settings/AiConnectionSection.vue";
import AiBehaviorSection from "./settings/AiBehaviorSection.vue";
import AiInsightsSection from "./settings/AiInsightsSection.vue";
import AboutSection from "./settings/AboutSection.vue";
import SettingsNav from "./settings/SettingsNav.vue";
import { useToast } from "../composables/useToast";
import { t } from "../i18n";
import {
  SETTINGS_GROUPS,
  SETTINGS_LEAVES,
  normalizeSettingsLeaf,
  type SettingsLeaf,
} from "../utils/settingsStructure";

defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "open-crash-report"): void;
}>();

const SETTINGS_NAV_STORAGE_KEY = "wgit.settings.active.v1";
/** Shell contract markers: four groups, thirteen leaves. */
const SETTINGS_LEAF_COUNT = SETTINGS_GROUPS.reduce((total, group) => total + group.leaves.length, 0);

function loadActiveLeaf(): SettingsLeaf {
  try {
    return normalizeSettingsLeaf(localStorage.getItem(SETTINGS_NAV_STORAGE_KEY));
  } catch {
    return "appearance";
  }
}

const activeLeaf = ref<SettingsLeaf>(loadActiveLeaf());
const settingsPanelRef = ref<HTMLElement | null>(null);
const aiRefreshKey = ref(0);

watch(activeLeaf, (leaf) => {
    settingsPanelRef.value?.scrollTo({ top: 0 });
  try { localStorage.setItem(SETTINGS_NAV_STORAGE_KEY, leaf); } catch { /* optional persistence */ }
}, { flush: "post" });

/** Command palette / cross-view jumps: switch to a concrete settings section. */
function handleSettingsSection(e: Event) {
  const detail = (e as CustomEvent).detail as { section?: string } | undefined;
  const section = detail?.section;
  if (section && (section === "localai" || (SETTINGS_LEAVES as readonly string[]).includes(section))) {
    activeLeaf.value = normalizeSettingsLeaf(section);
  }
}

onMounted(() => window.addEventListener("wgit:settings-section", handleSettingsSection));
onUnmounted(() => window.removeEventListener("wgit:settings-section", handleSettingsSection));

const { notify } = useToast();
</script>

<template>
  <div
    data-testid="settings-view"
    :data-settings-groups="SETTINGS_GROUPS.length"
    :data-settings-leaves="SETTINGS_LEAF_COUNT"
    class="flex-1 min-w-0 min-h-0 flex flex-col md:flex-row overflow-hidden bg-canvas select-none"
  >
    <aside data-testid="settings-sidebar" class="w-full md:w-52 min-h-0 max-h-[45dvh] md:max-h-none shrink-0 flex flex-col border-b md:border-b-0 md:border-r border-line bg-surface">
      <h1 class="hidden md:block px-5 pt-5 pb-4 text-sm font-semibold text-ink">{{ t("settings.title") }}</h1>
      <SettingsNav :active-leaf="activeLeaf" @select-leaf="activeLeaf = $event" />
    </aside>

    <div ref="settingsPanelRef" data-testid="settings-content" class="flex-1 min-w-0 min-h-0 overflow-y-auto p-4 md:p-6 lg:p-8">
      <div class="w-full min-w-0">
        <AppearanceSection v-if="activeLeaf === 'appearance'" @notify="notify" />
        <IdentitySection v-else-if="activeLeaf === 'identity'" :current-repo-path="currentRepoPath" @notify="notify" />
        <SigningSection v-else-if="activeLeaf === 'signing'" :current-repo-path="currentRepoPath" @notify="notify" />
        <ProxySection v-else-if="activeLeaf === 'proxy'" :current-repo-path="currentRepoPath" @notify="notify" />
        <ToolsSection v-else-if="activeLeaf === 'tools'" :current-repo-path="currentRepoPath" @notify="notify" />
        <AliasesSection v-else-if="activeLeaf === 'aliases'" :current-repo-path="currentRepoPath" @notify="notify" />
        <GitHooksSection v-else-if="activeLeaf === 'hooks'" :current-repo-path="currentRepoPath" @notify="notify" />
        <AdvancedConfigSection v-else-if="activeLeaf === 'config'" :current-repo-path="currentRepoPath" @notify="notify" />
        <ShortcutsSection v-else-if="activeLeaf === 'shortcuts'" />
        <AiConnectionSection v-else-if="activeLeaf === 'ai-connection'" :refresh-key="aiRefreshKey" @changed="aiRefreshKey++" />
        <AiBehaviorSection v-else-if="activeLeaf === 'ai-behavior'" :refresh-key="aiRefreshKey" @changed="aiRefreshKey++" />
        <AiInsightsSection v-else-if="activeLeaf === 'ai-insights'" :refresh-key="aiRefreshKey" @changed="aiRefreshKey++" />
        <AboutSection v-else-if="activeLeaf === 'about'" @notify="notify" @open-crash-report="emit('open-crash-report')" />
      </div>
    </div>
  </div>
</template>
