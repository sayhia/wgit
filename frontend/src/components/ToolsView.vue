<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import {
  Wrench,
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
} from "lucide-vue-next";
import BisectPanel from "./BisectPanel.vue";
import ReleaseWizard from "./ReleaseWizard.vue";
import LocalCiModal from "./tools/LocalCiModal.vue";
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

interface ToolsTabEventDetail {
  tab?: string;
  section?: string;
  badRef?: string;
  goodRef?: string;
}

const activeSubTab = ref<ToolsSubTab>("reflog");
const showLocalCiModal = ref(false);

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
});

onUnmounted(() => {
  window.removeEventListener("wgit:tools-tab", handleToolsTabEvent);
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
            @click="activeSubTab = 'terminal'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'terminal' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Terminal class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.terminal") }}</span>
          </button>
          <button
            @click="activeSubTab = 'reflog'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'reflog' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <RotateCcw class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.reflog") }}</span>
          </button>
          <button
            @click="activeSubTab = 'clean'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'clean' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Trash2 class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.clean") }}</span>
            <span
              v-if="(toolsStatus.counts.clean ?? 0) > 0"
              class="px-1.5 py-0.2 rounded-full bg-warning/20 text-warning text-[10px] font-bold"
            >
              {{ (toolsStatus.counts.clean ?? 0) }}
            </span>
          </button>
          <button
            @click="activeSubTab = 'remotes'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'remotes' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Globe class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.remotes") }}</span>
          </button>
          <button
            @click="activeSubTab = 'worktrees'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'worktrees' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <FolderGit2 class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.worktrees") }}</span>
            <span
              v-if="(toolsStatus.counts.worktrees ?? 0) > 0"
              class="px-1.5 py-0.2 rounded-full bg-purple/20 text-purple text-[10px] font-bold"
            >
              {{ (toolsStatus.counts.worktrees ?? 0) }}
            </span>
          </button>
          <button
            @click="activeSubTab = 'lfs'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'lfs' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <HardDrive class="w-3.5 h-3.5" />
            <span>Git LFS</span>
          </button>
          <button
            @click="activeSubTab = 'submodules'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'submodules' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Boxes class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.submodules") }}</span>
            <span
              v-if="(toolsStatus.counts.submodules ?? 0) > 0"
              class="px-1.5 py-0.2 rounded-full bg-accent/20 text-accent text-[10px] font-bold"
            >
              {{ (toolsStatus.counts.submodules ?? 0) }}
            </span>
          </button>
          <button
            @click="activeSubTab = 'sparse'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'sparse' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Filter class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.sparse") }}</span>
            <span
              v-if="(toolsStatus.counts.sparse ?? 0) > 0"
              class="px-1.5 py-0.2 rounded-full bg-purple/20 text-purple text-[10px] font-bold"
            >
              {{ (toolsStatus.counts.sparse ?? 0) }}
            </span>
          </button>
          <button
            @click="activeSubTab = 'disk'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'disk' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <PieChart class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.disk") }}</span>
          </button>
          <button
            @click="activeSubTab = 'maintenance'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'maintenance' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Activity class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.maintenance") }}</span>
          </button>
          <button
            @click="activeSubTab = 'studio'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'studio' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Users class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.studio") }}</span>
          </button>
          <button
            @click="activeSubTab = 'engine'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'engine' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Sparkles class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.engine") }}</span>
          </button>
          <button
            @click="activeSubTab = 'bisect'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'bisect' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <SplitSquareHorizontal class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.bisect") }}</span>
          </button>
          <button
            @click="activeSubTab = 'release'"
            class="px-2.5 py-1 rounded-md text-xs font-semibold transition cursor-pointer flex items-center gap-1.5 shrink-0 whitespace-nowrap"
            :class="activeSubTab === 'release' ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Rocket class="w-3.5 h-3.5" />
            <span>{{ t("tools.tab.release") }}</span>
          </button>
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

    <!-- Main Content Canvas -->
    <div :key="currentRepoPath" class="flex-1 overflow-y-auto p-6 max-w-6xl mx-auto w-full">

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

    <!-- Modal: Local CI Sandbox -->
    <LocalCiModal
      v-model:open="showLocalCiModal"
      :current-repo-path="currentRepoPath"
      @notify="notify"
    />

    <Toast :text="toastText" :visible="showToast" />
  </div>
</template>

