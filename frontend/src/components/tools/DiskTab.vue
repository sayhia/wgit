<script setup lang="ts">
import { reportError } from "../../composables/useToast";
import { t } from "../../i18n";
import { ref, computed, watch, onMounted, watchEffect } from "vue";
import {
  HardDrive,
  Search,
  Copy,
  FileCode,
  Sparkles,
  Filter,
  PieChart,
  CheckCircle2,
  FileWarning,
  Folder,
  History,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { toolsStatus } from "../../composables/toolsStatus";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, StorageBreakdownItem, HistoryLargeBlob, PackageBudgetResult } from "../../types";

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

function copyToClipboard(text: string, label: string) {
  navigator.clipboard.writeText(text);
  notify(t("disk.toast.copied", { label }));
}

// -------------------------------------------------------------
// 7. DISK BREAKDOWN & LARGE FILE SCANNER (体积透视与大文件)
// -------------------------------------------------------------
const diskBreakdown = ref<DiskBreakdownResult | null>(null);
const storageBreakdown = ref<StorageBreakdownItem[]>([]);
const isLoadingDisk = ref(false);
const maxStorageSize = computed(() =>
  storageBreakdown.value.reduce((max, item) => Math.max(max, item.sizeBytes), 0)
);
const largeFiles = ref<LargeFileInfo[]>([]);
const isLoadingLargeFiles = ref(false);
const largeFileMinSize = ref<number>(10);
const historyBlobs = ref<HistoryLargeBlob[]>([]);
const historyBlobLimit = ref(20);
const historyBlobsScanned = ref(false);
const isScanningHistoryBlobs = ref(false);
const packageBudget = ref<PackageBudgetResult | null>(null);
const isAnalyzingBudget = ref(false);

async function loadDiskBreakdown() {
  if (!props.currentRepoPath) return;
  isLoadingDisk.value = true;
  try {
    const [res, storage] = await Promise.allSettled([
      gitService.getRepositoryDiskBreakdown(props.currentRepoPath),
      gitService.getRepoStorageBreakdown(props.currentRepoPath),
    ]);
    const errors: string[] = [];
    if (res.status === "fulfilled") {
      diskBreakdown.value = res.value;
    } else {
      errors.push(String((res.reason as any)?.message || res.reason));
    }
    if (storage.status === "fulfilled") {
      storageBreakdown.value = storage.value || [];
    } else {
      errors.push(String((storage.reason as any)?.message || storage.reason));
    }
    if (errors.length > 0) {
      notify(t("disk.toast.load.failed", { error: errors.join("; ") }));
    }
  } finally {
    isLoadingDisk.value = false;
  }
}

async function scanLargeFiles(minMB = largeFileMinSize.value) {
  if (!props.currentRepoPath) return;
  largeFileMinSize.value = minMB;
  isLoadingLargeFiles.value = true;
  try {
    const files = await gitService.scanLargeFiles(props.currentRepoPath, minMB);
    largeFiles.value = files || [];
  } catch (err) {
    reportError("Failed to scan large files", err);
  } finally {
    isLoadingLargeFiles.value = false;
  }
}

async function scanHistoryBlobs(limit = historyBlobLimit.value) {
  if (!props.currentRepoPath || isScanningHistoryBlobs.value) return;
  historyBlobLimit.value = limit;
  isScanningHistoryBlobs.value = true;
  try {
    historyBlobs.value = (await gitService.scanHistoryLargeBlobs(props.currentRepoPath, limit)) || [];
    historyBlobsScanned.value = true;
    notify(t("disk.toast.history.scanned", { count: historyBlobs.value.length }));
  } catch (err: any) {
    notify(t("disk.toast.history.failed", { error: err?.message || err }));
  } finally {
    isScanningHistoryBlobs.value = false;
  }
}

async function analyzePackageBudget() {
  if (!props.currentRepoPath || isAnalyzingBudget.value) return;
  isAnalyzingBudget.value = true;
  try {
    packageBudget.value = await gitService.analyzePackageSizeBudget(props.currentRepoPath);
    if (packageBudget.value) {
      notify(
        t("disk.toast.budget.done", { size: packageBudget.value.totalSizeStr, status: packageBudget.value.hasOverrun ? t("disk.budget.overrun.platform", { platform: packageBudget.value.worstPlatform }) : t("disk.budget.within") })
      );
    }
  } catch (err: any) {
    notify(t("disk.toast.budget.failed", { error: err?.message || err }));
  } finally {
    isAnalyzingBudget.value = false;
  }
}

async function handleTrackFilePattern(filePath: string) {
  if (!props.currentRepoPath || !filePath) return;
  const parts = filePath.split(".");
  if (parts.length < 2) return;
  const ext = parts.pop();
  const pattern = `*.${ext}`;
  try {
    await gitService.trackLfsPattern(props.currentRepoPath, pattern);
    notify(t("disk.toast.lfs.added", { pattern }));
    await scanLargeFiles();
  } catch (err: any) {
    notify(t("disk.toast.lfs.failed", { error: err?.message || err }));
  }
}


watchEffect(() => {
  toolsStatus.loading.disk =
    isLoadingDisk.value ||
    isLoadingLargeFiles.value ||
    isScanningHistoryBlobs.value ||
    isAnalyzingBudget.value;
});

onMounted(() => {
  void loadDiskBreakdown();
  void scanLargeFiles();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadDiskBreakdown();
    void scanLargeFiles();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 7: 仓库体积透视与大文件 (Disk & Large Files) -->
      <!-- ============================================== -->
      <div class="space-y-6">
        <!-- Section A: 存储指标透视卡片 -->
        <div class="space-y-3" data-tool-section="disk-metrics">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <PieChart class="w-4 h-4 text-accent" />
              <span>{{ t("disk.breakdown.title") }}</span>
            </h3>
            <span class="text-xs text-ink-muted font-mono">{{ t("disk.breakdown.total", { size: diskBreakdown?.totalSizeStr || t("disk.counting") }) }}</span>
          </div>

          <div class="grid grid-cols-5 gap-3">
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">{{ t("disk.total.physical") }}</span>
              <span class="text-lg font-bold font-mono text-ink">{{ diskBreakdown?.totalSizeStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("disk.total.physical.hint") }}</span>
            </div>
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">{{ t("disk.worktree") }}</span>
              <span class="text-lg font-bold font-mono text-accent">{{ diskBreakdown?.workingTreeStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("disk.worktree.hint") }}</span>
            </div>
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">{{ t("disk.git.db") }}</span>
              <span class="text-lg font-bold font-mono text-purple">{{ diskBreakdown?.gitInternalStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("disk.git.db.hint") }}</span>
            </div>
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">{{ t("disk.lfs.cache") }}</span>
              <span class="text-lg font-bold font-mono text-warning">{{ diskBreakdown?.lfsStoreStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("disk.lfs.cache.hint") }}</span>
            </div>
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">{{ t("disk.engine.cache") }}</span>
              <span class="text-lg font-bold font-mono text-ink-secondary">{{ diskBreakdown?.engineCacheStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">Derived / Artifacts</span>
            </div>
          </div>

          <!-- Recommendations if any -->
          <div
            v-if="diskBreakdown?.recommendations && diskBreakdown.recommendations.length > 0"
            class="p-3 rounded-xl border border-accent/20 bg-accent/5 flex items-start gap-2.5"
          >
            <Sparkles class="w-4 h-4 text-accent shrink-0 mt-0.5" />
            <div class="text-xs space-y-1">
              <div class="font-bold text-accent">{{ t("disk.suggestions") }}</div>
              <ul class="list-disc list-inside space-y-0.5 text-ink-muted">
                <li v-for="(rec, i) in diskBreakdown.recommendations" :key="i">{{ rec }}</li>
              </ul>
            </div>
          </div>

          <!-- Top Folders Ranking -->
          <div v-if="diskBreakdown?.topFolders && diskBreakdown.topFolders.length > 0" class="space-y-2 pt-2">
            <h4 class="text-xs font-bold text-ink-secondary">{{ t("disk.top.dirs") }}</h4>
            <div class="grid grid-cols-2 gap-2">
              <div
                v-for="folder in diskBreakdown.topFolders"
                :key="folder.path"
                class="p-2.5 rounded-lg border border-line bg-surface/20 flex items-center justify-between font-mono text-xs"
              >
                <div class="flex items-center gap-2 truncate min-w-0">
                  <Folder class="w-3.5 h-3.5 text-ink-muted shrink-0" />
                  <span class="font-semibold text-ink truncate">{{ folder.path }}</span>
                </div>
                <span class="text-ink-secondary font-bold shrink-0 ml-2">{{ folder.sizeStr }}</span>
              </div>
            </div>
          </div>
          <div v-if="storageBreakdown.length > 0" class="space-y-2 pt-2">
            <h4 class="text-xs font-bold text-ink-secondary">{{ t("disk.top.exts") }}</h4>
            <div class="grid grid-cols-2 gap-2">
              <div
                v-for="item in storageBreakdown.slice(0, 10)"
                :key="item.extension"
                class="p-2.5 rounded-lg border border-line bg-surface/20 space-y-1"
              >
                <div class="flex items-center justify-between gap-2 text-[11px] font-mono">
                  <span class="text-ink font-semibold truncate">{{ item.extension }}</span>
                  <span class="text-ink-muted shrink-0">{{ t("disk.files.count", { count: item.count }) }}</span>
                  <span class="text-ink-secondary font-bold shrink-0">{{ item.sizeHuman }}</span>
                </div>
                <div class="h-1 rounded bg-canvas overflow-hidden">
                  <div
                    class="h-full rounded bg-accent/70"
                    :style="{ width: `${maxStorageSize > 0 ? Math.max(2, (item.sizeBytes / maxStorageSize) * 100) : 0}%` }"
                  ></div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Section B: 大文件体检扫描 -->
        <div class="space-y-3 pt-3" data-tool-section="disk-large-files">
          <!--
            Action cluster is `shrink-0` and the row wraps: with the longer
            English description the threshold buttons used to be squeezed until
            「> 5MB」 / 「Top 10」 / 「Scan history」 broke across two lines.
          -->
          <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 pb-2 border-b border-line">
            <div class="min-w-0 flex-1">
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <FileWarning class="w-4 h-4 text-warning" />
                <span>{{ t("disk.large.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("disk.large.desc") }}
              </p>
            </div>

            <!-- Size threshold filter & Scan trigger -->
            <div class="ml-auto flex flex-wrap items-center gap-2 shrink-0">
              <div class="flex items-center bg-surface rounded-lg p-0.5 border border-line text-xs font-semibold shrink-0">
                <button
                  v-for="size in [5, 10, 50, 100]"
                  :key="size"
                  @click="scanLargeFiles(size)"
                  class="px-2.5 py-1 rounded-md transition cursor-pointer font-mono whitespace-nowrap"
                  :class="largeFileMinSize === size ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
                >
                  &gt; {{ size }}MB
                </button>
              </div>

              <button
                @click="scanLargeFiles(largeFileMinSize)"
                :disabled="isLoadingLargeFiles"
                class="flex items-center gap-1.5 whitespace-nowrap px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer shadow-xs disabled:opacity-50"
              >
                <Search class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingLargeFiles }" />
                <span>{{ isLoadingLargeFiles ? t("disk.scanning") : t("disk.rescan") }}</span>
              </button>
            </div>
          </div>

          <!-- Large files table -->
          <div class="border border-line rounded-xl bg-surface/30 overflow-hidden shadow-xs">
            <div v-if="largeFiles.length > 0" class="divide-y divide-line/40 font-mono text-xs">
              <div
                v-for="file in largeFiles"
                :key="file.path"
                class="p-3 flex items-center justify-between hover:bg-surface/60 transition group"
              >
                <div class="flex items-center gap-3 truncate min-w-0 pr-3">
                  <FileCode class="w-4 h-4 text-ink-muted shrink-0" />
                  <span class="text-ink font-semibold truncate">{{ file.path }}</span>
                  <span class="px-2 py-0.5 rounded bg-surface border border-line text-ink-muted text-[10px] shrink-0">
                    {{ file.sizeHuman }}
                  </span>
                  <span
                    class="px-2 py-0.5 rounded text-[10px] font-sans font-bold shrink-0"
                    :class="file.isLfs ? 'bg-success/15 text-success border border-success/30' : 'bg-warning/15 text-warning border border-warning/30'"
                  >
                    {{ file.isLfs ? t('disk.lfs.tracked') : t('disk.lfs.untracked') }}
                  </span>
                </div>

                <div class="flex items-center gap-2 shrink-0">
                  <button
                    v-if="!file.isLfs"
                    @click="handleTrackFilePattern(file.path)"
                    class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-warning/20 hover:bg-warning/30 text-warning text-xs font-semibold transition cursor-pointer border border-warning/30"
                    :title="t('disk.lfs.add.title')"
                  >
                    <HardDrive class="w-3 h-3" />
                    <span>{{ t("disk.lfs.add") }}</span>
                  </button>
                  <button :aria-label="t('disk.copy.path')"
                    @click="copyToClipboard(file.path, t('disk.file.path'))"
                    class="p-1 rounded text-ink-muted hover:text-ink cursor-pointer"
                    :title="t('disk.copy.path')"
                  >
                    <Copy class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            </div>

            <div v-else class="py-12 text-center text-xs text-ink-muted flex flex-col items-center gap-2">
              <CheckCircle2 class="w-6 h-6 text-success" />
              <span>{{ t("disk.large.empty", { size: largeFileMinSize }) }}</span>
            </div>
          </div>
        </div>

        <!-- Section C: Historical large blobs -->
        <div class="space-y-3 pt-3" data-tool-section="disk-history-blobs">
          <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 pb-2 border-b border-line">
            <div class="min-w-0 flex-1">
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <History class="w-4 h-4 text-purple" />
                <span>{{ t("disk.history.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("disk.history.desc") }}
              </p>
            </div>
            <div class="ml-auto flex flex-wrap items-center gap-2 shrink-0">
              <div class="flex items-center bg-surface rounded-lg p-0.5 border border-line text-xs font-semibold shrink-0">
                <button
                  v-for="size in [10, 20, 50, 100]"
                  :key="size"
                  @click="scanHistoryBlobs(size)"
                  class="px-2.5 py-1 rounded-md transition cursor-pointer font-mono whitespace-nowrap"
                  :class="historyBlobLimit === size ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
                >
                  Top {{ size }}
                </button>
              </div>
              <button
                @click="scanHistoryBlobs(historyBlobLimit)"
                :disabled="isScanningHistoryBlobs"
                class="flex items-center gap-1.5 whitespace-nowrap px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                <Search class="w-3.5 h-3.5" :class="{ 'animate-spin': isScanningHistoryBlobs }" />
                <span>{{ isScanningHistoryBlobs ? t("disk.scanning") : t("disk.history.run") }}</span>
              </button>
            </div>
          </div>

          <div class="border border-line rounded-xl bg-surface/30 overflow-hidden">
            <div v-if="historyBlobs.length > 0" class="divide-y divide-line/40 font-mono text-xs">
              <div
                v-for="blob in historyBlobs"
                :key="blob.oid"
                class="p-2.5 flex items-center gap-3 hover:bg-surface/60 transition"
              >
                <span class="text-warning font-bold shrink-0 w-16 text-right">{{ blob.sizeStr }}</span>
                <div class="min-w-0">
                  <div class="text-ink font-semibold truncate" :title="blob.path">{{ blob.path || t("disk.unknown.path") }}</div>
                  <div class="text-[10px] text-ink-muted truncate" :title="blob.subject">
                    {{ blob.commitHash.substring(0, 8) }} · {{ blob.author }} · {{ blob.commitDate }} · {{ blob.subject }}
                  </div>
                </div>
                <span class="text-[10px] text-ink-muted shrink-0">{{ blob.oid.substring(0, 10) }}</span>
              </div>
            </div>
            <div v-else-if="historyBlobsScanned" class="py-6 text-center text-xs text-success">
              {{ t("disk.history.empty") }}
            </div>
            <div v-else class="py-6 text-center text-xs text-ink-muted">
              {{ t("disk.history.prompt") }}
            </div>
          </div>
        </div>

        <!-- Section D: Package size budget -->
        <div class="space-y-3 pt-3" data-tool-section="disk-package-budget">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <PieChart class="w-4 h-4 text-accent" />
                <span>{{ t("disk.budget.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("disk.budget.desc") }}
              </p>
            </div>
            <button
              @click="analyzePackageBudget"
              :disabled="isAnalyzingBudget"
              class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-50"
            >
              {{ isAnalyzingBudget ? t("disk.analyzing") : t("disk.budget.run") }}
            </button>
          </div>

          <template v-if="packageBudget">
            <div class="grid grid-cols-4 gap-2">
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("disk.budget.assets") }}</div>
                <div class="text-sm font-bold font-mono text-ink">{{ packageBudget.totalSizeStr }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("disk.budget.lfs") }}</div>
                <div class="text-sm font-bold font-mono text-warning">{{ packageBudget.totalLfsSizeStr }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("disk.budget.files") }}</div>
                <div class="text-sm font-bold font-mono text-purple">{{ packageBudget.totalAssetsCount }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("disk.budget.status") }}</div>
                <div class="text-sm font-bold" :class="packageBudget.hasOverrun ? 'text-danger' : 'text-success'">
                  {{ packageBudget.hasOverrun ? t("disk.budget.overrun", { platform: packageBudget.worstPlatform }) : t("disk.budget.within") }}
                </div>
              </div>
            </div>

            <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
              <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
                <div class="text-xs font-bold text-ink">{{ t("disk.budget.platforms") }}</div>
                <div
                  v-for="platform in packageBudget.platformBudgets || []"
                  :key="platform.platform"
                  class="space-y-1"
                >
                  <div class="flex items-center gap-2 text-[10px] font-mono">
                    <span class="text-ink truncate flex-1">{{ platform.platform }}</span>
                    <span class="text-ink-muted shrink-0">{{ platform.currentSizeStr }} / {{ platform.maxBudgetLimitStr }}</span>
                    <span
                      class="shrink-0 font-bold"
                      :class="platform.status === 'OVERRUN' ? 'text-danger' : platform.status === 'WARNING' ? 'text-warning' : 'text-success'"
                    >
                      {{ platform.usageRatio }}%
                    </span>
                  </div>
                  <div class="h-1.5 rounded bg-canvas overflow-hidden">
                    <div
                      class="h-full rounded"
                      :class="platform.status === 'OVERRUN' ? 'bg-danger-strong' : platform.status === 'WARNING' ? 'bg-warning-strong' : 'bg-success-strong'"
                      :style="{ width: `${Math.min(100, platform.usageRatio)}%` }"
                    ></div>
                  </div>
                </div>
              </div>

              <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
                <div class="text-xs font-bold text-ink">{{ t("disk.budget.systems") }}</div>
                <div
                  v-for="system in (packageBudget.systems || []).slice(0, 6)"
                  :key="system.systemName"
                  class="flex items-center gap-2 text-[10px] font-mono"
                >
                  <span class="text-ink w-24 shrink-0 truncate">{{ system.systemName }}</span>
                  <div class="flex-1 h-1.5 rounded bg-canvas overflow-hidden">
                    <div class="h-full rounded bg-accent/70" :style="{ width: `${Math.min(100, system.percentage)}%` }"></div>
                  </div>
                  <span class="text-ink-muted w-24 text-right shrink-0">{{ system.totalSizeStr }} ({{ system.percentage }}%)</span>
                </div>
                <div v-if="(packageBudget.systems || []).length === 0" class="text-[11px] text-success">{{ t("disk.budget.systems.empty") }}</div>
              </div>
            </div>

            <div v-if="(packageBudget.optimizationTips || []).length > 0" class="rounded-xl border border-warning/20 bg-warning/5 p-3 space-y-1">
              <div class="text-xs font-bold text-warning">{{ t("disk.budget.tips") }}</div>
              <div
                v-for="tip in (packageBudget.optimizationTips || []).slice(0, 5)"
                :key="`${tip.category}:${tip.targetFile}`"
                class="text-[10px] text-ink-muted"
              >
                [{{ tip.priority }}] {{ tip.category }} · {{ tip.targetFile }} · {{ t("disk.budget.saving", { size: tip.estimatedSavingStr }) }}
              </div>
            </div>
            <p class="text-[10px] text-ink-muted">{{ packageBudget.summary }}</p>
          </template>
          <p v-else class="text-[11px] text-ink-muted">{{ t("disk.budget.not.run") }}</p>
        </div>
      </div>

</template>
