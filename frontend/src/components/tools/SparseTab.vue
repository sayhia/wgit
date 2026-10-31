<script setup lang="ts">
import { reportError } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch, onMounted, watchEffect } from "vue";
import { Filter, CheckCircle2, FolderTree, Folder, CloudDownload, Database, Loader2, Sparkles, HardDrive, Check } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { toolsStatus } from "../../composables/toolsStatus";
import { t } from "../../i18n";
import type { PartialCloneStatus, ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, SparsePreset, SceneHydrateResult } from "../../types";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
  mode?: "remotes" | "worktrees";
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
  (e: "open-repo", path: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

// -------------------------------------------------------------
// 6. SPARSE CHECKOUT (稀疏检出 / Monorepo 部分拉取)
// -------------------------------------------------------------
const sparseFolders = ref<string[]>([]);
const newSparseFolder = ref("");
const isLoadingSparse = ref(false);
const isSavingSparse = ref(false);
const partialCloneStatus = ref<PartialCloneStatus | null>(null);
const hydratePath = ref("");
const isHydrating = ref(false);

const sparsePresets = ref<SparsePreset[]>([]);
const activePresetId = ref<string>("");
const isLoadingPresets = ref(false);
const isApplyingPreset = ref("");
const scenePath = ref("");
const isHydratingScene = ref(false);
const sceneResult = ref<SceneHydrateResult | null>(null);

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

async function loadSparsePresets() {
  if (!props.currentRepoPath) return;
  isLoadingPresets.value = true;
  try {
    sparsePresets.value = await gitService.getSparsePresets(props.currentRepoPath);
  } catch (err) {
    reportError("Failed to load sparse presets", err);
  } finally {
    isLoadingPresets.value = false;
  }
}

async function handleApplyPreset(preset: SparsePreset) {
  if (!props.currentRepoPath || isApplyingPreset.value) return;
  isApplyingPreset.value = preset.id;
  try {
    const res = await gitService.applyRoleSparsePreset(props.currentRepoPath, preset.id, []);
    activePresetId.value = preset.id;
    notify(t("sparse.role.applied", { role: preset.title }));
    await loadSparseCheckout();
  } catch (err: any) {
    notify(err?.message || String(err));
  } finally {
    isApplyingPreset.value = "";
  }
}

async function handleHydrateScene() {
  const p = scenePath.value.trim();
  if (!p || !props.currentRepoPath || isHydratingScene.value) return;
  isHydratingScene.value = true;
  try {
    const res = await gitService.hydrateSceneLfsAssets(props.currentRepoPath, p);
    sceneResult.value = res;
    if (res.hydratedCount > 0) {
      notify(t("sparse.scene.success", { count: res.hydratedCount, size: formatBytes(res.reclaimedBytes), duration: res.durationMs }));
    } else {
      notify(t("sparse.scene.empty"));
    }
  } catch (err: any) {
    notify(err?.message || String(err));
  } finally {
    isHydratingScene.value = false;
  }
}

async function loadPartialCloneStatus() {
  if (!props.currentRepoPath) return;
  try {
    partialCloneStatus.value = await gitService.getPartialCloneStatus(props.currentRepoPath);
  } catch (err) {
    // State marking kept: still degrades to the standard-clone badge, but the
    // failure is no longer silent — the repo may in fact be a partial clone.
    reportError("Failed to load partial clone status", err);
    partialCloneStatus.value = null;
  }
}

async function handleHydrate() {
  const p = hydratePath.value.trim();
  if (!p || !props.currentRepoPath || isHydrating.value) return;
  isHydrating.value = true;
  try {
    const res = await gitService.hydrateBlobs(props.currentRepoPath, [p]);
    notify(t("sparse.partial.hydrated", { count: res.hydratedPaths.length }));
    hydratePath.value = "";
  } catch (err: any) {
    notify(err?.message || String(err));
  } finally {
    isHydrating.value = false;
  }
}

async function loadSparseCheckout() {
  if (!props.currentRepoPath) return;
  isLoadingSparse.value = true;
  try {
    const list = await gitService.getSparseCheckoutFolders(props.currentRepoPath);
    sparseFolders.value = list || [];
  } catch (err) {
    reportError("Failed to load sparse checkout", err);
  } finally {
    isLoadingSparse.value = false;
  }
}

function addSparseFolder(path?: string) {
  const target = (path ?? newSparseFolder.value).trim();
  if (!target) return;
  if (!sparseFolders.value.includes(target)) {
    sparseFolders.value.push(target);
  }
  newSparseFolder.value = "";
}

function removeSparseFolder(index: number) {
  sparseFolders.value.splice(index, 1);
}

async function handleApplySparseCheckout() {
  if (!props.currentRepoPath) return;
  isSavingSparse.value = true;
  try {
    await gitService.setSparseCheckoutFolders(props.currentRepoPath, sparseFolders.value);
    notify(t("sparse.toast.applied", { count: sparseFolders.value.length }));
    await loadSparseCheckout();
  } catch (err: any) {
    notify(t("sparse.toast.apply.failed", { error: err?.message || err }));
  } finally {
    isSavingSparse.value = false;
  }
}

async function handleResetSparseCheckout() {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction(t("sparse.confirm.reset")))) return;
  isSavingSparse.value = true;
  try {
    await gitService.setSparseCheckoutFolders(props.currentRepoPath, []);
    notify(t("sparse.toast.reset"));
    await loadSparseCheckout();
  } catch (err: any) {
    notify(t("sparse.toast.reset.failed", { error: err?.message || err }));
  } finally {
    isSavingSparse.value = false;
  }
}


watchEffect(() => {
  toolsStatus.loading.sparse = isLoadingSparse.value || isSavingSparse.value;
});

watchEffect(() => {
  toolsStatus.counts.sparse = sparseFolders.value.length;
});

onMounted(() => {
  void loadSparseCheckout();
  void loadPartialCloneStatus();
  void loadSparsePresets();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadSparseCheckout();
    void loadPartialCloneStatus();
    void loadSparsePresets();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 6: 稀疏检出与 Monorepo (Sparse Checkout)    -->
      <!-- ============================================== -->
      <div class="space-y-5">
        <!-- Banner -->
        <div class="p-4 rounded-xl border border-purple/30 bg-purple-bg flex items-start gap-3">
          <Filter class="w-5 h-5 text-purple shrink-0 mt-0.5" />
          <div class="text-xs space-y-1">
            <div class="font-bold text-sm text-ink">{{ t("sparse.banner.title") }}</div>
            <div class="text-ink-muted leading-relaxed">
              {{ t("sparse.banner.body") }}
            </div>
          </div>
        </div>

        <!-- Role-Based Sparse Presets -->
        <div class="p-4 rounded-xl border border-line bg-surface space-y-3" data-testid="sparse-role-presets">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Sparkles class="w-4 h-4 text-purple" />
              <span class="text-xs font-bold text-ink">{{ t("sparse.role.title") }}</span>
            </div>
            <span class="text-[11px] text-ink-muted hidden sm:inline">{{ t("sparse.role.desc") }}</span>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2.5">
            <div
              v-for="preset in sparsePresets"
              :key="preset.id"
              class="p-3 rounded-lg border transition flex flex-col justify-between gap-2"
              :class="activePresetId === preset.id ? 'border-purple/50 bg-purple-bg' : 'border-line bg-surface hover:bg-raised'"
            >
              <div>
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-ink">{{ preset.title }}</span>
                  <span class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-purple-bg text-purple border border-purple/30">
                    {{ preset.estimatedDiskSaving }}
                  </span>
                </div>
                <p class="text-[11px] text-ink-muted mt-1 leading-snug">{{ preset.description }}</p>
                <div v-if="preset.patterns.length > 0" class="flex flex-wrap gap-1 mt-2">
                  <span
                    v-for="pat in preset.patterns"
                    :key="pat"
                    class="px-1.5 py-0.5 rounded bg-canvas border border-line font-mono text-[10px] text-ink-secondary"
                  >
                    {{ pat }}
                  </span>
                </div>
              </div>

              <button
                @click="handleApplyPreset(preset)"
                :disabled="isApplyingPreset === preset.id"
                class="w-full mt-2 py-1.5 px-3 rounded-md text-xs font-medium transition cursor-pointer flex items-center justify-center gap-1.5"
                :class="activePresetId === preset.id ? 'bg-purple text-white' : 'bg-surface hover:bg-raised border border-line text-ink'"
              >
                <Loader2 v-if="isApplyingPreset === preset.id" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
                <Check v-else-if="activePresetId === preset.id" class="w-3.5 h-3.5 text-white" aria-hidden="true" />
                <span>{{ isApplyingPreset === preset.id ? t("sparse.role.applying") : t("sparse.role.apply") }}</span>
              </button>
            </div>
          </div>
        </div>

        <!-- Scene LFS Asset Hydration Card -->
        <div class="p-3.5 rounded-xl border border-line bg-surface space-y-3" data-testid="scene-lfs-hydration">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <HardDrive class="w-4 h-4 text-accent" />
              <span class="text-xs font-bold text-ink">{{ t("sparse.scene.title") }}</span>
            </div>
            <span class="text-[11px] text-ink-muted hidden sm:inline">{{ t("sparse.scene.desc") }}</span>
          </div>

          <div class="flex items-center gap-2">
            <input
              v-model="scenePath"
              type="text"
              :placeholder="t('sparse.scene.placeholder')"
              class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              @keydown.enter="handleHydrateScene"
            />
            <button
              @click="handleHydrateScene"
              :disabled="!scenePath.trim() || isHydratingScene"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent text-accent-contrast hover:bg-accent-strong text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              <Loader2 v-if="isHydratingScene" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
              <CloudDownload v-else class="w-3.5 h-3.5" aria-hidden="true" />
              <span>{{ isHydratingScene ? t("sparse.scene.hydrating") : t("sparse.scene.btn") }}</span>
            </button>
          </div>

          <div v-if="sceneResult" class="p-2.5 rounded-lg bg-surface border border-line text-[11px] space-y-1">
            <div class="flex items-center justify-between font-mono">
              <span class="text-ink-secondary">{{ sceneResult.scenePath }}</span>
              <span class="text-success font-semibold">{{ formatBytes(sceneResult.reclaimedBytes) }} ({{ sceneResult.hydratedCount }} files)</span>
            </div>
            <div v-if="sceneResult.referencedBlobs && sceneResult.referencedBlobs.length > 0" class="flex flex-wrap gap-1 pt-1">
              <span
                v-for="blob in sceneResult.referencedBlobs.slice(0, 5)"
                :key="blob"
                class="px-1.5 py-0.5 rounded bg-canvas border border-line font-mono text-[10px] text-ink-muted truncate max-w-[200px]"
              >
                {{ blob }}
              </span>
              <span v-if="sceneResult.referencedBlobs.length > 5" class="text-[10px] text-ink-muted">
                +{{ sceneResult.referencedBlobs.length - 5 }} more
              </span>
            </div>
          </div>
        </div>

        <!-- Partial Clone On-Demand Hydration Card -->
        <div class="p-3.5 rounded-xl border border-line bg-surface space-y-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Database class="w-4 h-4 text-accent" />
              <span class="text-xs font-bold text-ink">{{ t("sparse.partial.title") }}</span>
            </div>
            <span
              class="px-2 py-0.5 rounded text-[10px] font-mono border"
              :class="partialCloneStatus?.isPartialClone
                ? 'bg-cyan-bg text-cyan border-cyan/30'
                : 'bg-canvas text-ink-muted border-line'"
            >
              {{ partialCloneStatus?.isPartialClone ? t("sparse.partial.active") : t("sparse.partial.standard") }}
            </span>
          </div>

          <div v-if="partialCloneStatus?.isPartialClone" class="space-y-2">
            <div class="text-[11px] text-ink-secondary">
              {{ t("sparse.partial.filter", { filter: partialCloneStatus.filterSpec, remote: partialCloneStatus.promisorRemote }) }}
            </div>
            <div class="flex items-center gap-2">
              <input
                v-model="hydratePath"
                type="text"
            :aria-label="t('sparse.partial.hydrate')"
                :placeholder="t('sparse.partial.hydrate')"
                class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              />
              <button
                @click="handleHydrate"
                :disabled="!hydratePath.trim() || isHydrating"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent text-accent-contrast hover:bg-accent-strong text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                <Loader2 v-if="isHydrating" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
                <CloudDownload v-else class="w-3.5 h-3.5" aria-hidden="true" />
                <span>{{ isHydrating ? t("sparse.partial.hydrating") : t("sparse.partial.hydrate.btn") }}</span>
              </button>
            </div>
          </div>
          <div v-else class="text-[11px] text-ink-muted">
            {{ partialCloneStatus?.summary || t("sparse.partial.standard") }}
          </div>
        </div>

        <!-- Section: Active Folders List -->
        <div class="space-y-3" data-tool-section="sparse-folders">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <FolderTree class="w-4 h-4 text-accent" />
              <span>{{ t("sparse.list.title") }}</span>
            </h3>
            <span class="text-xs text-ink-muted">{{ t("sparse.list.count", { count: sparseFolders.length }) }}</span>
          </div>

          <!-- Tags list -->
          <div class="flex flex-wrap gap-2 p-4 rounded-xl border border-line bg-surface min-h-[90px]">
            <div
              v-for="(folder, idx) in sparseFolders"
              :key="idx"
              class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-surface border border-line font-mono text-xs text-ink shadow-2xs group"
            >
              <Folder class="w-3.5 h-3.5 text-accent shrink-0" />
              <span>{{ folder }}</span>
              <button
                @click="removeSparseFolder(idx)"
                class="text-ink-muted hover:text-danger cursor-pointer ml-1"
                :title="t('sparse.remove.hint')"
              >
                ×
              </button>
            </div>

            <div
              v-if="sparseFolders.length === 0"
              class="w-full py-4 text-center text-xs text-ink-muted flex items-center justify-center gap-2"
            >
              <CheckCircle2 class="w-4 h-4 text-success" />
              <span>{{ t("sparse.list.empty") }}</span>
            </div>
          </div>

          <!-- Add Directory Form & Presets -->
          <div class="flex items-center gap-2">
            <div class="relative flex-1">
              <input
                v-model="newSparseFolder"
                type="text"
                @keydown.enter="addSparseFolder()"
            :aria-label="t('sparse.placeholder')"
                :placeholder="t('sparse.placeholder')"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
            </div>
            <button
              @click="addSparseFolder()"
              :disabled="!newSparseFolder.trim()"
              class="px-4 py-2 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              {{ t("sparse.add") }}
            </button>
          </div>

          <!-- Preset Helpers -->
          <div class="flex items-center gap-2 text-xs text-ink-muted pt-1">
            <span class="font-medium">{{ t("sparse.presets") }}</span>
            <button
              @click="addSparseFolder('frontend')"
              class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[11px] font-mono cursor-pointer"
            >
              + frontend
            </button>
            <button
              @click="addSparseFolder('src')"
              class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[11px] font-mono cursor-pointer"
            >
              + src
            </button>
            <button
              @click="addSparseFolder('docs')"
              class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[11px] font-mono cursor-pointer"
            >
              + docs
            </button>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="flex items-center justify-between pt-4 border-t border-line">
          <button
            @click="handleResetSparseCheckout"
            :disabled="isSavingSparse || sparseFolders.length === 0"
            class="px-4 py-2 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
          >
            {{ t("sparse.reset") }}
          </button>

          <button
            @click="handleApplySparseCheckout"
            :disabled="isSavingSparse || sparseFolders.length === 0"
            class="flex items-center gap-2 px-5 py-2 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50 shadow-xs"
          >
            <Filter class="w-3.5 h-3.5" :class="{ 'animate-spin': isSavingSparse }" aria-hidden="true" />
            <span>{{ isSavingSparse ? t("sparse.applying") : t("sparse.apply") }}</span>
          </button>
        </div>
      </div>

</template>
