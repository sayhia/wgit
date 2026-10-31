<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, onMounted, onUnmounted, ref, watch, type Component } from "vue";
import {
  Wrench,
  ChevronDown,
  RotateCcw,
  Trash2,
  Globe,
  FolderGit2,
  HardDrive,
  Boxes,
  Filter,
  PieChart,
  Activity,
  SplitSquareHorizontal,
  Rocket,
  RefreshCw,
  Sparkles,
  Users,
  Terminal,
  Play,
  Radio,
} from "lucide-vue-next";
import BisectPanel from "./BisectPanel.vue";
import ReleaseWizard from "./ReleaseWizard.vue";
import LocalCiModal from "./tools/LocalCiModal.vue";
import AirGappedHubModal from "./tools/AirGappedHubModal.vue";
const ReflogTab = defineAsyncComponent(() => import("./tools/ReflogTab.vue"));
const CleanTab = defineAsyncComponent(() => import("./tools/CleanTab.vue"));
const RemotesTab = defineAsyncComponent(() => import("./tools/RemotesTab.vue"));
const WorktreesTab = defineAsyncComponent(() => import("./tools/WorktreesTab.vue"));
const LfsTab = defineAsyncComponent(() => import("./tools/LfsTab.vue"));
const SubmodulesTab = defineAsyncComponent(() => import("./tools/SubmodulesTab.vue"));
const SparseTab = defineAsyncComponent(() => import("./tools/SparseTab.vue"));
const DiskTab = defineAsyncComponent(() => import("./tools/DiskTab.vue"));
const MaintenanceTab = defineAsyncComponent(() => import("./tools/MaintenanceTab.vue"));
const StudioTab = defineAsyncComponent(() => import("./tools/StudioTab.vue"));
const EngineTab = defineAsyncComponent(() => import("./tools/EngineTab.vue"));
const TerminalTab = defineAsyncComponent(() => import("./tools/TerminalTab.vue"));
import { toolsStatus } from "../composables/toolsStatus";
import Toast from "./ui/Toast.vue";
import { useToast } from "../composables/useToast";
import { t } from "../i18n";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "open-repo", path: string): void;
  (e: "notify", msg: string): void;
}>();

type ToolsSubTab =
  | "terminal"
  | "reflog"
  | "clean"
  | "remotes"
  | "worktrees"
  | "lfs"
  | "submodules"
  | "sparse"
  | "disk"
  | "maintenance"
  | "studio"
  | "engine"
  | "bisect"
  | "release";

const TOOLS_SUB_TABS: readonly ToolsSubTab[] = [
  "terminal",
  "reflog",
  "clean",
  "remotes",
  "worktrees",
  "lfs",
  "submodules",
  "sparse",
  "disk",
  "maintenance",
  "studio",
  "engine",
  "bisect",
  "release",
];

/** 子 Tab 渲染定义：一级直达 + 「更多 ▾」下拉两级。 */
interface ToolsTabDef {
  id: ToolsSubTab;
  labelKey: string;
  icon: Component;
  /** toolsStatus.counts 的键；存在且 >0 时渲染计数徽标。 */
  badgeKey?: string;
  badgeClass?: string;
}

const BADGE_WARNING = "bg-warning/20 text-warning";
const BADGE_PURPLE = "bg-purple/20 text-purple";

/** 一级 Tab：高频工具直达；其余收入「更多 ▾」，完整索引仍在命令面板。 */
const PRIMARY_TABS: readonly ToolsTabDef[] = [
  { id: "terminal", labelKey: "tools.tab.terminal", icon: Terminal },
  { id: "reflog", labelKey: "tools.tab.reflog", icon: RotateCcw },
  { id: "clean", labelKey: "tools.tab.clean", icon: Trash2, badgeKey: "clean", badgeClass: BADGE_WARNING },
  { id: "worktrees", labelKey: "tools.tab.worktrees", icon: FolderGit2, badgeKey: "worktrees", badgeClass: BADGE_PURPLE },
  { id: "lfs", labelKey: "tools.tab.lfs", icon: HardDrive },
];

const MORE_TABS: readonly ToolsTabDef[] = [
  { id: "remotes", labelKey: "tools.tab.remotes", icon: Globe },
  { id: "submodules", labelKey: "tools.tab.submodules", icon: Boxes, badgeKey: "submodules", badgeClass: "bg-accent/20 text-accent" },
  { id: "sparse", labelKey: "tools.tab.sparse", icon: Filter, badgeKey: "sparse", badgeClass: BADGE_PURPLE },
  { id: "disk", labelKey: "tools.tab.disk", icon: PieChart },
  { id: "maintenance", labelKey: "tools.tab.maintenance", icon: Activity },
  { id: "studio", labelKey: "tools.tab.studio", icon: Users },
  { id: "engine", labelKey: "tools.tab.engine", icon: Sparkles },
  { id: "bisect", labelKey: "tools.tab.bisect", icon: SplitSquareHorizontal },
  { id: "release", labelKey: "tools.tab.release", icon: Rocket },
];

const showMoreMenu = ref(false);
const toolsMoreBtnRef = ref<HTMLElement | null>(null);
const moreMenuPosition = ref({ left: 0, top: 0 });
const MORE_MENU_WIDTH = 192;
const activeIsMore = computed(() => MORE_TABS.some((tab) => tab.id === activeSubTab.value));

/**
 * The 更多工具 menu lives in the scrollable tab strip (`overflow-x-auto`), so an
 * in-flow `absolute` panel was clipped away and the nine secondary tools were
 * unreachable by mouse. The menu is teleported to `<body>` with fixed
 * coordinates anchored to the trigger, mirroring the command palette pattern.
 */
function toggleMoreMenu() {
  if (showMoreMenu.value) {
    showMoreMenu.value = false;
    return;
  }
  const rect = toolsMoreBtnRef.value?.getBoundingClientRect();
  if (rect) {
    moreMenuPosition.value = {
      left: Math.max(8, Math.min(window.innerWidth - MORE_MENU_WIDTH - 8, rect.right - MORE_MENU_WIDTH)),
      top: rect.bottom + 4,
    };
  }
  showMoreMenu.value = true;
}

function handleMoreMenuKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" && showMoreMenu.value) showMoreMenu.value = false;
}

function selectTab(id: ToolsSubTab) {
  activeSubTab.value = id;
  showMoreMenu.value = false;
}

function tabBadge(def: ToolsTabDef): number {
  if (!def.badgeKey) return 0;
  return (toolsStatus.counts as Record<string, number>)[def.badgeKey] ?? 0;
}

interface ToolsTabEventDetail {
  tab?: string;
  section?: string;
  badRef?: string;
  goodRef?: string;
}

const activeSubTab = ref<ToolsSubTab>("reflog");
const showLocalCiModal = ref(false);
const showAirGappedModal = ref(false);

/** Bumped by the header refresh and repo switches; the mounted tab reloads on change. */
const refreshNonce = ref(0);

const bisectPanelRef = ref<InstanceType<typeof BisectPanel> | null>(null);
const releasePanelRef = ref<InstanceType<typeof ReleaseWizard> | null>(null);

const isActiveTabLoading = computed(() => Boolean(toolsStatus.loading[activeSubTab.value]));

// Toast
const { toastText, showToast, notify } = useToast();
function refreshActiveTab() {
  const tab = activeSubTab.value;
  if (tab === "bisect") {
    // The panel owns its data; ask it to re-read the live bisect session state.
    void bisectPanelRef.value?.loadStatus();
    return;
  }
  if (tab === "release") {
    // The release wizard owns its plan/changelog data; ask it to reload.
    void releasePanelRef.value?.loadPlan();
    return;
  }
  refreshNonce.value++;
}

// Bisect judgments check out other commits: refresh the app plus the current tool tab.
function handleBisectRefresh() {
  emit("refresh");
  refreshActiveTab();
}

// Creating a release changes tags/refs: refresh the app plus the current tool tab.
function handleReleaseRefresh() {
  emit("refresh");
  refreshActiveTab();
}

// CommandPalette (and other views) jump into a specific tool tab via this event.
/** Scroll to a section inside the (possibly async) tool tab, retrying until the
 *  panel is mounted. Used by command-palette deep links such as "热更清单". */
function scrollToToolSection(section?: string) {
  if (!section) return;
  let attempts = 0;
  const tryScroll = () => {
    const el = document.querySelector(`[data-tool-section="${section}"]`);
    if (el) {
      el.scrollIntoView({ behavior: "smooth", block: "start" });
      el.classList.add("ring-2", "ring-accent/40", "rounded-xl");
      setTimeout(() => el.classList.remove("ring-2", "ring-accent/40", "rounded-xl"), 1800);
      return;
    }
    if (++attempts < 15) setTimeout(tryScroll, 120);
  };
  void nextTick(() => setTimeout(tryScroll, 80));
}

function handleToolsTabEvent(event: Event) {
  const detail = (event as CustomEvent<ToolsTabEventDetail>).detail;
  const tab = detail?.tab;
  if (!tab) return;
  if ((TOOLS_SUB_TABS as readonly string[]).includes(tab)) {
    activeSubTab.value = tab as ToolsSubTab;
    scrollToToolSection(detail?.section);
    if (tab === "bisect" && (detail?.badRef || detail?.goodRef)) {
      nextTick(() => {
        bisectPanelRef.value?.setRefs(detail.badRef, detail.goodRef);
      });
    }
  }
}

onMounted(() => {
  window.addEventListener("wgit:tools-tab", handleToolsTabEvent);
  window.addEventListener("keydown", handleMoreMenuKeydown);
});

onUnmounted(() => {
  window.removeEventListener("wgit:tools-tab", handleToolsTabEvent);
  window.removeEventListener("keydown", handleMoreMenuKeydown);
});

// Initial load / tab switches: children load on mount, a nonce bump reloads them.
watch(activeSubTab, () => refreshActiveTab(), { immediate: true });

watch(
  () => props.currentRepoPath,
  () => refreshActiveTab()
);
</script>

<template>
  <div class="flex-1 flex flex-col bg-canvas text-ink overflow-hidden select-none font-sans">
    <!-- Top Sub-Navigation Header -->
    <div class="h-12 border-b border-line px-4 flex items-center justify-between bg-surface/40 backdrop-blur-sm shrink-0 gap-3">
      <div class="flex items-center gap-2 min-w-0 overflow-x-auto py-1">
        <div class="w-8 h-8 rounded-xl bg-accent/15 flex items-center justify-center text-accent shrink-0">
          <Wrench class="w-4 h-4" />
        </div>
        <div class="flex items-center gap-1 bg-surface rounded-lg p-0.5 border border-line shrink-0">
          <button
            v-for="def in PRIMARY_TABS"
            :key="def.id"
            @click="selectTab(def.id)"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === def.id ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
            :title="t(def.labelKey)"
            :aria-label="t(def.labelKey)"
          >
            <component :is="def.icon" class="w-3.5 h-3.5" />
            <!--
              Labels collapse below xl so the five primary tabs + 更多 never
              push Git LFS / 更多工具 out of the scrollable strip (they used to
              be cut mid-glyph at 1024px with no scroll affordance).
            -->
            <span class="hidden xl:inline">{{ t(def.labelKey) }}</span>
            <span
              v-if="tabBadge(def) > 0"
              class="px-1.5 py-0.2 rounded-full text-[10px] font-bold"
              :class="def.badgeClass"
            >
              {{ tabBadge(def) }}
            </span>
          </button>
          <div class="relative shrink-0" data-testid="tools-more-wrap">
            <button
              ref="toolsMoreBtnRef"
              :aria-expanded="showMoreMenu"
              @click="toggleMoreMenu"
              class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
              :class="activeIsMore ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
              data-testid="tools-more-btn"
            >
              <Wrench class="w-3.5 h-3.5" />
              <span>{{ t("tools.more") }}</span>
              <ChevronDown class="w-3 h-3 transition-transform" :class="showMoreMenu ? 'rotate-180' : ''" />
            </button>
            <Teleport to="body">
            <div v-if="showMoreMenu" class="fixed inset-0 z-40" @click="showMoreMenu = false"></div>
            <div
              v-if="showMoreMenu"
              class="fixed z-50 w-48 rounded-xl border border-line bg-surface shadow-lg p-1.5 space-y-0.5"
              :style="{ left: `${moreMenuPosition.left}px`, top: `${moreMenuPosition.top}px` }"
              data-testid="tools-more-menu"
            >
              <button
                v-for="def in MORE_TABS"
                :key="def.id"
                @click="selectTab(def.id)"
                class="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-md text-xs font-medium transition cursor-pointer text-left"
                :class="activeSubTab === def.id ? 'bg-raised text-accent' : 'text-ink-secondary hover:bg-raised hover:text-ink'"
              >
                <component :is="def.icon" class="w-3.5 h-3.5 shrink-0" />
                <span class="flex-1">{{ t(def.labelKey) }}</span>
                <span
                  v-if="tabBadge(def) > 0"
                  class="px-1.5 py-0.2 rounded-full text-[10px] font-bold shrink-0"
                  :class="def.badgeClass"
                >
                  {{ tabBadge(def) }}
                </span>
              </button>
            </div>
            </Teleport>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <button
          @click="showLocalCiModal = true"
          class="flex items-center gap-1.5 px-3 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
          data-testid="open-local-ci-btn"
        >
          <Play class="w-3.5 h-3.5 text-accent" />
          <span>{{ t("ci.btn.run") }}</span>
        </button>

        <button
          @click="showAirGappedModal = true"
          class="flex items-center gap-1.5 px-3 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer"
          data-testid="open-air-gapped-hub-btn"
          :title="t('air_gapped.title')"
        >
          <Radio class="w-3.5 h-3.5 text-accent" />
          <span>{{ t("air_gapped.title") }}</span>
        </button>

        <!-- Action Refresh Button -->
        <button
          @click="refreshActiveTab"
          class="flex items-center gap-1.5 px-3 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer shrink-0"
        >
          <RefreshCw
            class="w-3.5 h-3.5"
            :class="{ 'animate-spin': isActiveTabLoading }"
          />
          <span>{{ t("tools.refresh") }}</span>
        </button>
      </div>
    </div>

    <!--
      Main Content Canvas. The scroll container spans the full pane so the
      scrollbar stays on the window edge; the readable column is capped inside.
    -->
    <div :key="currentRepoPath" class="flex-1 overflow-y-auto">
      <div class="p-6 max-w-6xl mx-auto w-full">

      <TerminalTab
        v-if="activeSubTab === 'terminal'"
        :current-repo-path="currentRepoPath"
        :refresh-nonce="refreshNonce"
        @notify="notify"
      />
      <ReflogTab v-if="activeSubTab === 'reflog'" :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />
      <CleanTab v-if="activeSubTab === 'clean'" :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />
      <RemotesTab
        v-if="activeSubTab === 'remotes'"
        :current-repo-path="currentRepoPath"
        :refresh-nonce="refreshNonce"
        @refresh="emit('refresh')"
        @notify="notify"
      />
      <WorktreesTab
        v-if="activeSubTab === 'worktrees'"
        :current-repo-path="currentRepoPath"
        :refresh-nonce="refreshNonce"
        @refresh="emit('refresh')"
        @notify="notify($event); emit('notify', $event)"
        @open-repo="emit('open-repo', $event)"
      />
      <LfsTab v-if="activeSubTab === 'lfs'" :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />
      <SubmodulesTab
        v-if="activeSubTab === 'submodules'"
        :current-repo-path="currentRepoPath"
        :refresh-nonce="refreshNonce"
        @refresh="emit('refresh')"
        @notify="notify"
        @open-repo="emit('open-repo', $event)"
      />
      <SparseTab v-if="activeSubTab === 'sparse'" :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />
      <DiskTab v-if="activeSubTab === 'disk'" :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />
      <MaintenanceTab v-if="activeSubTab === 'maintenance'" :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />
      <StudioTab v-if="activeSubTab === 'studio'" :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />
      <EngineTab v-if="activeSubTab === 'engine'" :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />

      <!-- ============================================== -->
      <!-- TAB 9: 二分定位 (Git Bisect Regression Hunt)    -->
      <!-- ============================================== -->
      <div v-else-if="activeSubTab === 'bisect'" class="space-y-6">
        <BisectPanel ref="bisectPanelRef" :repo-path="currentRepoPath" @refresh="handleBisectRefresh" />
      </div>

      <!-- ============================================== -->
      <!-- TAB 10: 发布向导 (Release Wizard)               -->
      <!-- ============================================== -->
      <div v-else-if="activeSubTab === 'release'" class="space-y-6">
        <ReleaseWizard ref="releasePanelRef" :repo-path="currentRepoPath" @refresh="handleReleaseRefresh" />
      </div>
      </div>
    </div>

    <!-- Modal: Local CI Sandbox -->
    <LocalCiModal
      v-model:open="showLocalCiModal"
      :current-repo-path="currentRepoPath"
      @notify="notify"
    />

    <!-- Modal: Air-Gapped Sync & Delta Pack Hub -->
    <AirGappedHubModal
      v-model:open="showAirGappedModal"
      :repo-path="currentRepoPath"
      @notify="notify"
    />

  </div>
</template>

