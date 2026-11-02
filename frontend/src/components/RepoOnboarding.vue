<script setup lang="ts">
import { t } from "../i18n";
import { ref, computed, onMounted, watch, nextTick } from "vue";
import UiModal from "./ui/UiModal.vue";
import OnboardingCapabilities from "./onboarding/OnboardingCapabilities.vue";
import ScanProjectsCard from "./onboarding/ScanProjectsCard.vue";
import RecentReposCard from "./onboarding/RecentReposCard.vue";
import {
  FolderGit2,
  FolderOpen,
  Download,
  FolderPlus,
  GitBranch,
  ArrowRight,
  Check,
  X,
  Loader2,
  ChevronDown,
  HardDrive,
  Sparkles,
  AlertTriangle,
  Clock,
} from "lucide-vue-next";
import { pushToast } from "../composables/useToast";
import { gitService } from "../services/gitService";
import type { ProjectDetailItem } from "../types";

const props = defineProps<{
  mode?: "welcome" | "add" | "clone" | "init";
  canClose?: boolean;
}>();

const emit = defineEmits<{
  (e: "opened", path: string): void;
  (e: "close"): void;
}>();

const activeMode = ref<"welcome" | "add" | "clone" | "init">(props.mode || "welcome");
const errorMessage = ref("");

// ---- Clone form ----
const cloneURL = ref("");
const cloneTarget = ref("");
const cloneType = ref<"full" | "blobless" | "shallow">("blobless");
const cloneDepth = ref(1);
const cloneBranch = ref("");
const cloneSubmodules = ref(false);
const showAdvancedClone = ref(false);
const isCloning = ref(false);
const cloneProgressHint = ref("");

// ---- Init form ----
const initPath = ref("");
const isIniting = ref(false);

const canSubmitClone = computed(() => cloneURL.value.trim() !== "" && cloneTarget.value.trim() !== "");
const canSubmitInit = computed(() => initPath.value.trim() !== "");

const cloneTypeLabel = computed(() => {
  switch (cloneType.value) {
    case "blobless":
      return t("onboard.clone.blobless.label");
    case "shallow":
      return t("onboard.clone.shallow.label");
    default:
      return t("onboard.clone.full.label");
  }
});



async function handleOpenDialog() {
  errorMessage.value = "";
  try {
    const info = await gitService.openRepoDialog();
    if (info?.path) {
      emit("opened", info.path);
    }
  } catch (err: any) {
    const msg = err?.message || String(err);
    // Backend wire values (English + both historical Chinese forms) — matching
    // literals, never displayed.
    if (msg.includes("not a valid git repository") || msg.includes("不是一个有效的 Git 仓库") || msg.includes("不是有效的 Git 仓库")) {
      const match = msg.match(/['"「]([^'"」]+)['"」]/);
      const folder = match ? match[1] : "";
      if (folder) {
        initPath.value = folder;
        activeMode.value = "init";
        errorMessage.value = t("onboard.error.not.repo", { folder });
        return;
      }
    }
    errorMessage.value = t("onboard.error.open", { message: msg });
  }
}

const scanCardRef = ref<InstanceType<typeof ScanProjectsCard> | null>(null);

// "Scan local projects" entry point (command palette / repo rail): switch to
// scan mode and let the card scroll its own input into view and focus it.

// Batch add writes the recent list only, so there is nothing on this page to
// refresh — confirm with a toast instead of leaving the click silent.
function onReposAdded(count: number) {
  errorMessage.value = "";
  pushToast(t("onboard.scan.added", { count }));
}

async function focusScanInput() {
  activeMode.value = "add";
  await nextTick();
  await scanCardRef.value?.focusInput();
}

async function handleOpenRecent(path: string) {
  errorMessage.value = "";
  try {
    const info = await gitService.openRepo(path);
    if (info?.path) {
      emit("opened", info.path);
    }
  } catch (err: any) {
    errorMessage.value = t("onboard.error.open", { message: err?.message || err });
  }
}

async function browseCloneTarget() {
  try {
    const folder = await gitService.pickFolderDialog();
    if (folder) {
      cloneTarget.value = folder;
    }
  } catch (err) {
    // best-effort: the user dismissed the native dialog.
    console.debug("Pick folder cancelled/failed:", err);
  }
}

async function handleClone() {
  if (!canSubmitClone.value || isCloning.value) return;
  errorMessage.value = "";
  isCloning.value = true;
  cloneProgressHint.value =
    cloneType.value === "blobless"
      ? t("onboard.progress.blobless")
      : cloneType.value === "shallow"
      ? t("onboard.progress.shallow", { depth: cloneDepth.value })
      : t("onboard.progress.full");
  try {
    const info = await gitService.cloneRepoAdvanced({
      url: cloneURL.value.trim(),
      targetPath: cloneTarget.value.trim(),
      cloneType: cloneType.value,
      depth: cloneDepth.value,
      branch: cloneBranch.value.trim(),
      recurseSubmodules: cloneSubmodules.value,
    });
    if (info?.path) {
      emit("opened", info.path);
    } else {
      errorMessage.value = t("onboard.error.clone.opened");
    }
  } catch (err: any) {
    errorMessage.value = t("onboard.error.clone", { error: err?.message || err });
  } finally {
    isCloning.value = false;
    cloneProgressHint.value = "";
  }
}

async function browseInitPath() {
  try {
    const folder = await gitService.pickFolderDialog();
    if (folder) {
      initPath.value = folder;
    }
  } catch (err) {
    // best-effort: the user dismissed the native dialog.
    console.debug("Pick folder cancelled/failed:", err);
  }
}

async function handleInit() {
  if (!canSubmitInit.value || isIniting.value) return;
  errorMessage.value = "";
  isIniting.value = true;
  try {
    const info = await gitService.initRepo(initPath.value.trim());
    if (info?.path) {
      emit("opened", info.path);
    }
  } catch (err: any) {
    errorMessage.value = t("onboard.error.init", { error: err?.message || err });
  } finally {
    isIniting.value = false;
  }
}

onMounted(() => {
  if (props.mode === "add") void focusScanInput();
});

watch(
  () => props.mode,
  (mode) => {
    if (mode === "add") void focusScanInput();
  }
);
</script>

<template>
  <UiModal
    :open="true"
    variant="bare"
    width="max-w-3xl"
    overlay="canvas-strong"
    surface="plain"
    :label="t('onboard.aria')"
    panel-class="max-h-[calc(100vh-3rem)] overflow-y-auto rounded-xl shadow-modal select-none"
    @update:open="(open) => { if (!open && props.canClose) emit('close'); }"
  >
      <!-- Hero header -->
      <div class="relative px-8 pt-4 pb-3.5 border-b border-line bg-surface">
        <button :aria-label="t('common.close')"
          v-if="props.canClose"
          @click="emit('close')"
          class="absolute right-4 top-4 p-1.5 rounded-lg text-ink-muted hover:text-ink hover:bg-raised transition cursor-pointer"
          :title="t('common.close')"
        >
          <X class="w-4 h-4" />
        </button>
        <div class="flex items-center gap-3">
          <img
            src="/brand/wgit-icon.svg"
            alt=""
            width="44"
            height="44"
            :draggable="false"
            class="w-11 h-11 shrink-0"
          />
          <div class="flex flex-col">
            <h1 class="text-lg font-bold text-ink tracking-tight">{{ t("onboard.app.title") }}</h1>
            <p class="text-xs text-ink-muted">{{ t("onboard.app.subtitle") }}</p>
          </div>
        </div>
      </div>

      <!-- Error banner -->
      <div
        v-if="errorMessage"
        class="mx-8 mt-5 px-3 py-2.5 rounded-xl bg-danger-bg border border-danger/30 text-danger text-xs flex items-start gap-2"
      >
        <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
        <span class="break-all">{{ errorMessage }}</span>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 p-6">
        <!-- Left: actions -->
        <div class="flex flex-col gap-3">
          <button
            @click="activeMode = 'add'"
            class="group flex items-center gap-3 p-3.5 rounded-2xl border text-left transition cursor-pointer"
            :class="activeMode === 'add' ? 'bg-accent-bg border-accent/50' : 'bg-canvas border-line hover:border-accent/50 hover:bg-raised'"
          >
            <div class="w-9 h-9 rounded-xl bg-accent-bg text-accent flex items-center justify-center shrink-0">
              <FolderOpen class="w-4.5 h-4.5" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-ink">{{ t("onboard.open.title") }}</div>
              <div class="text-[11px] text-ink-muted truncate">{{ t("onboard.open.desc") }}</div>
            </div>
            <ArrowRight class="w-4 h-4 text-ink-muted group-hover:text-accent transition shrink-0" />
          </button>

          <button
            @click="activeMode = 'clone'"
            class="group flex items-center gap-3 p-3.5 rounded-2xl border text-left transition cursor-pointer"
            :class="activeMode === 'clone' ? 'bg-accent-bg border-accent/50' : 'bg-canvas border-line hover:border-accent/50 hover:bg-raised'"
          >
            <div class="w-9 h-9 rounded-xl bg-cyan-bg text-cyan flex items-center justify-center shrink-0">
              <Download class="w-4.5 h-4.5" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-ink">{{ t("onboard.clone.title") }}</div>
              <div class="text-[11px] text-ink-muted truncate">{{ t("onboard.clone.desc") }}</div>
            </div>
            <ArrowRight class="w-4 h-4 text-ink-muted group-hover:text-cyan transition shrink-0" />
          </button>

          <button
            @click="activeMode = 'init'"
            class="group flex items-center gap-3 p-3.5 rounded-2xl border text-left transition cursor-pointer"
            :class="activeMode === 'init' ? 'bg-success-bg border-success/50' : 'bg-canvas border-line hover:border-success/50 hover:bg-raised'"
          >
            <div class="w-9 h-9 rounded-xl bg-success-bg text-success flex items-center justify-center shrink-0">
              <FolderPlus class="w-4.5 h-4.5" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-ink">{{ t("onboard.init.title") }}</div>
              <div class="text-[11px] text-ink-muted truncate">{{ t("onboard.init.desc") }}</div>
            </div>
            <ArrowRight class="w-4 h-4 text-ink-muted group-hover:text-success transition shrink-0" />
          </button>

          <!-- Recent repos -->
          <RecentReposCard
            class="mt-1"
            @open="handleOpenRecent"
          />

        </div>

        <!-- Right: selected action form -->
        <div class="rounded-2xl bg-canvas border border-line p-5 flex flex-col">
          <!-- Add a local repository: one folder, or a directory scan -->
          <template v-if="activeMode === 'add'">
            <div class="flex items-center gap-2 mb-4">
              <FolderOpen class="w-4 h-4 text-accent" />
              <h2 class="text-sm font-bold text-ink">{{ t("onboard.add.panel") }}</h2>
            </div>

            <button
              @click="handleOpenDialog"
              class="w-full flex items-center justify-center gap-2 px-3 py-2.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer"
            >
              <FolderOpen class="w-3.5 h-3.5" />
              {{ t("onboard.add.pick") }}
            </button>

            <div class="flex items-center gap-2 my-3 text-[10px] text-ink-muted">
              <span class="flex-1 h-px bg-line"></span>
              <span>{{ t("onboard.add.or") }}</span>
              <span class="flex-1 h-px bg-line"></span>
            </div>

            <ScanProjectsCard
              ref="scanCardRef"
              @opened="handleOpenRecent"
              @failed="errorMessage = $event"
              @started="errorMessage = ''"
              @added="onReposAdded"
            />
          </template>

          <!-- Clone form -->
          <template v-else-if="activeMode === 'clone'">
            <div class="flex items-center gap-2 mb-4">
              <Download class="w-4 h-4 text-cyan" />
              <h2 class="text-sm font-bold text-ink">{{ t("onboard.clone.panel") }}</h2>
            </div>

            <div class="flex flex-col gap-3">
              <label class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">{{ t("onboard.clone.url") }}</span>
                <input
                  v-model="cloneURL"
                  type="text"
                  placeholder="git@github.com:org/big-monorepo.git"
                  class="w-full px-3 py-2 rounded-lg bg-surface border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
                />
              </label>

              <label class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">{{ t("onboard.clone.target") }}</span>
                <div class="flex items-center gap-1.5">
                  <input
                    v-model="cloneTarget"
                    type="text"
                    placeholder="/Users/you/repos/big-monorepo"
                    class="flex-1 min-w-0 px-3 py-2 rounded-lg bg-surface border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
                  />
                  <button :aria-label="t('onboard.choose.dir')"
                    @click="browseCloneTarget"
                    class="px-2.5 py-2 rounded-lg bg-surface hover:bg-raised border border-line text-xs text-ink transition cursor-pointer shrink-0"
                    :title="t('onboard.choose.dir')"
                  >
                    <FolderOpen class="w-3.5 h-3.5" />
                  </button>
                </div>
              </label>

              <label class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">{{ t("onboard.clone.mode") }}</span>
                <div class="relative">
                  <select
                    v-model="cloneType"
                    class="w-full appearance-none px-3 py-2 pr-8 rounded-lg bg-surface border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
                  >
                    <option value="blobless">{{ t("onboard.clone.blobless.label") }}</option>
                    <option value="shallow">{{ t("onboard.clone.shallow.label") }}</option>
                    <option value="full">{{ t("onboard.clone.full.label") }}</option>
                  </select>
                  <ChevronDown class="w-3.5 h-3.5 text-ink-muted absolute right-2.5 top-2.5 pointer-events-none" />
                </div>
                <span class="text-[10px] text-ink-muted leading-relaxed">
                  {{ t("onboard.clone.mode.hint") }}
                </span>
              </label>

              <div v-if="cloneType === 'shallow'" class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">{{ t("onboard.clone.depth") }}</span>
                <input
                  v-model.number="cloneDepth"
                  type="number"
                  min="1"
                  class="w-full px-3 py-2 rounded-lg bg-surface border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
                />
              </div>

              <button
                @click="showAdvancedClone = !showAdvancedClone"
                class="flex items-center gap-1 text-[11px] text-accent hover:text-accent-strong transition cursor-pointer self-start"
              >
                <ChevronDown class="w-3 h-3 transition-transform" :class="{ 'rotate-180': showAdvancedClone }" />
                <span>{{ t("onboard.clone.advanced") }}</span>
              </button>

              <div v-if="showAdvancedClone" class="flex flex-col gap-3 p-3 rounded-xl bg-surface border border-line/70">
                <label class="flex flex-col gap-1">
                  <span class="text-[11px] font-medium text-ink-secondary">{{ t("onboard.clone.branch") }}</span>
                  <input
                    v-model="cloneBranch"
                    type="text"
                    placeholder="main / release/v2"
                    class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
                  />
                </label>
                <label class="flex items-center gap-2 cursor-pointer">
                  <input v-model="cloneSubmodules" type="checkbox" class="accent-accent" />
                  <span class="text-[11px] text-ink-secondary">{{ t("onboard.clone.recurse") }}</span>
                </label>
              </div>

              <div
                v-if="cloneProgressHint"
                class="flex items-start gap-2 px-3 py-2 rounded-lg bg-cyan-bg border border-cyan/30 text-[11px] text-cyan"
              >
                <Loader2 class="w-3.5 h-3.5 animate-spin shrink-0 mt-0.5" />
                <span>{{ cloneProgressHint }}</span>
              </div>

              <button
                @click="handleClone"
                :disabled="!canSubmitClone || isCloning"
                class="flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-cyan-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Loader2 v-if="isCloning" class="w-4 h-4 animate-spin" />
                <Download v-else class="w-4 h-4" />
                <span>{{ isCloning ? t("onboard.cloning") : t("onboard.clone.run") }}</span>
              </button>
            </div>
          </template>

          <!-- Init form -->
          <template v-else-if="activeMode === 'init'">
            <div class="flex items-center gap-2 mb-4">
              <FolderPlus class="w-4 h-4 text-success" />
              <h2 class="text-sm font-bold text-ink">{{ t("onboard.init.panel") }}</h2>
            </div>
            <div class="flex flex-col gap-3">
              <label class="flex flex-col gap-1">
                <span class="text-[11px] font-medium text-ink-secondary">{{ t("onboard.init.dir") }}</span>
                <div class="flex items-center gap-1.5">
                  <input
                    v-model="initPath"
                    type="text"
                    placeholder="/Users/you/projects/new-project"
                    class="flex-1 min-w-0 px-3 py-2 rounded-lg bg-surface border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
                  />
                  <button :aria-label="t('onboard.choose.dir')"
                    @click="browseInitPath"
                    class="px-2.5 py-2 rounded-lg bg-surface hover:bg-raised border border-line text-xs text-ink transition cursor-pointer shrink-0"
                    :title="t('onboard.choose.dir')"
                  >
                    <FolderOpen class="w-3.5 h-3.5" />
                  </button>
                </div>
              </label>
              <p class="text-[11px] text-ink-muted leading-relaxed">
                {{ t("onboard.init.hint.before") }} <code class="font-mono text-ink-secondary">git init</code>{{ t("onboard.init.hint.after") }}
              </p>
              <button
                @click="handleInit"
                :disabled="!canSubmitInit || isIniting"
                class="flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-success-strong text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Loader2 v-if="isIniting" class="w-4 h-4 animate-spin" />
                <Check v-else class="w-4 h-4" />
                <span>{{ isIniting ? t("onboard.initing") : t("onboard.init.run") }}</span>
              </button>
            </div>
          </template>

          <!-- Welcome hints -->
          <template v-else>
            <OnboardingCapabilities />
          </template>
        </div>
      </div>

      <div v-if="activeMode !== 'welcome'" class="px-8 pb-6">
        <button
          @click="activeMode = 'welcome'"
          class="text-[11px] text-ink-muted hover:text-ink transition cursor-pointer"
        >
          {{ t("onboard.back") }}
        </button>
      </div>
  </UiModal>
</template>
