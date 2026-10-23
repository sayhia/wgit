<script setup lang="ts">
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
  notify(`已复制 ${label}`);
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
    const [res, storage] = await Promise.all([
      gitService.getRepositoryDiskBreakdown(props.currentRepoPath),
      gitService.getRepoStorageBreakdown(props.currentRepoPath).catch(() => []),
    ]);
    diskBreakdown.value = res;
    storageBreakdown.value = storage || [];
  } catch (err) {
    console.error("Failed to load disk breakdown:", err);
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
    console.error("Failed to scan large files:", err);
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
    notify(`历史大对象扫描完成：发现 ${historyBlobs.value.length} 个对象`);
  } catch (err: any) {
    notify(`历史大对象扫描失败: ${err?.message || err}`);
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
        `包体预算分析完成：${packageBudget.value.totalSizeStr}，${packageBudget.value.hasOverrun ? `存在超限 (${packageBudget.value.worstPlatform})` : "全部平台在预算内"}`
      );
    }
  } catch (err: any) {
    notify(`包体预算分析失败: ${err?.message || err}`);
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
    notify(`已成功将 ${pattern} 加入 Git LFS 托管规则`);
    await scanLargeFiles();
  } catch (err: any) {
    notify(`加入 LFS 托管失败: ${err?.message || err}`);
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
              <span>仓库物理空间占用透视 (Repository Disk Breakdown)</span>
            </h3>
            <span class="text-xs text-ink-muted font-mono">总大小: {{ diskBreakdown?.totalSizeStr || '统计中...' }}</span>
          </div>

          <div class="grid grid-cols-5 gap-3">
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">总物理占用</span>
              <span class="text-lg font-bold font-mono text-ink">{{ diskBreakdown?.totalSizeStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">磁盘实际占用总量</span>
            </div>
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">工作区文件 (Worktree)</span>
              <span class="text-lg font-bold font-mono text-accent">{{ diskBreakdown?.workingTreeStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">当前检出代码及资产</span>
            </div>
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">Git 内部数据库 (.git)</span>
              <span class="text-lg font-bold font-mono text-purple">{{ diskBreakdown?.gitInternalStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">历史提交、Tree及Pack</span>
            </div>
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">Git LFS 本地缓存</span>
              <span class="text-lg font-bold font-mono text-warning">{{ diskBreakdown?.lfsStoreStr || '0 B' }}</span>
              <span class="text-[10px] text-ink-muted">已缓存的二进制大文件</span>
            </div>
            <div class="p-3.5 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-[11px] text-ink-muted font-medium">引擎及构建缓存</span>
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
              <div class="font-bold text-accent">仓库瘦身与性能优化建议:</div>
              <ul class="list-disc list-inside space-y-0.5 text-ink-muted">
                <li v-for="(rec, i) in diskBreakdown.recommendations" :key="i">{{ rec }}</li>
              </ul>
            </div>
          </div>

          <!-- Top Folders Ranking -->
          <div v-if="diskBreakdown?.topFolders && diskBreakdown.topFolders.length > 0" class="space-y-2 pt-2">
            <h4 class="text-xs font-bold text-ink-secondary">根目录物理体积排行榜 (Top Directories)</h4>
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
            <h4 class="text-xs font-bold text-ink-secondary">工作区扩展名体积分布 (Top Extensions)</h4>
            <div class="grid grid-cols-2 gap-2">
              <div
                v-for="item in storageBreakdown.slice(0, 10)"
                :key="item.extension"
                class="p-2.5 rounded-lg border border-line bg-surface/20 space-y-1"
              >
                <div class="flex items-center justify-between gap-2 text-[11px] font-mono">
                  <span class="text-ink font-semibold truncate">{{ item.extension }}</span>
                  <span class="text-ink-muted shrink-0">{{ item.count }} 个</span>
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
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <FileWarning class="w-4 h-4 text-warning" />
                <span>大文件体检扫描 (Large File Scanner)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                实时扫描工作区中超出阈值的大文件，及时纳入 Git LFS 托管，防止仓库历史永久膨胀。
              </p>
            </div>

            <!-- Size threshold filter & Scan trigger -->
            <div class="flex items-center gap-2">
              <div class="flex items-center bg-surface rounded-lg p-0.5 border border-line text-xs font-semibold">
                <button
                  v-for="size in [5, 10, 50, 100]"
                  :key="size"
                  @click="scanLargeFiles(size)"
                  class="px-2.5 py-1 rounded-md transition cursor-pointer font-mono"
                  :class="largeFileMinSize === size ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
                >
                  &gt; {{ size }}MB
                </button>
              </div>

              <button
                @click="scanLargeFiles(largeFileMinSize)"
                :disabled="isLoadingLargeFiles"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer shadow-xs disabled:opacity-50"
              >
                <Search class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingLargeFiles }" />
                <span>{{ isLoadingLargeFiles ? "扫描中..." : "重新扫描" }}</span>
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
                    {{ file.isLfs ? 'LFS 托管中' : '⚠️ 未纳入 LFS' }}
                  </span>
                </div>

                <div class="flex items-center gap-2 shrink-0">
                  <button
                    v-if="!file.isLfs"
                    @click="handleTrackFilePattern(file.path)"
                    class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-warning/20 hover:bg-warning/30 text-warning text-xs font-semibold transition cursor-pointer border border-warning/30"
                    title="将此扩展名加入 .gitattributes LFS 规则"
                  >
                    <HardDrive class="w-3 h-3" />
                    <span>纳入 LFS 跟踪</span>
                  </button>
                  <button aria-label="复制文件路径"
                    @click="copyToClipboard(file.path, '文件路径')"
                    class="p-1 rounded text-ink-muted hover:text-ink cursor-pointer"
                    title="复制文件路径"
                  >
                    <Copy class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            </div>

            <div v-else class="py-12 text-center text-xs text-ink-muted flex flex-col items-center gap-2">
              <CheckCircle2 class="w-6 h-6 text-success" />
              <span>当前工作区未发现大于 {{ largeFileMinSize }}MB 的大文件</span>
            </div>
          </div>
        </div>

        <!-- Section C: Historical large blobs -->
        <div class="space-y-3 pt-3" data-tool-section="disk-history-blobs">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <History class="w-4 h-4 text-purple" />
                <span>历史大对象扫描 (History Large Blobs)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                扫描全部历史提交中的大 Blob，定位即使已删除仍占用 .git 体积的对象。
              </p>
            </div>
            <div class="flex items-center gap-2">
              <div class="flex items-center bg-surface rounded-lg p-0.5 border border-line text-xs font-semibold">
                <button
                  v-for="size in [10, 20, 50, 100]"
                  :key="size"
                  @click="scanHistoryBlobs(size)"
                  class="px-2.5 py-1 rounded-md transition cursor-pointer font-mono"
                  :class="historyBlobLimit === size ? 'bg-raised text-accent shadow-xs' : 'text-ink-muted hover:text-ink'"
                >
                  Top {{ size }}
                </button>
              </div>
              <button
                @click="scanHistoryBlobs(historyBlobLimit)"
                :disabled="isScanningHistoryBlobs"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                <Search class="w-3.5 h-3.5" :class="{ 'animate-spin': isScanningHistoryBlobs }" />
                <span>{{ isScanningHistoryBlobs ? "扫描中..." : "扫描历史" }}</span>
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
                <div class="min-w-0 flex-1">
                  <div class="text-ink font-semibold truncate" :title="blob.path">{{ blob.path || "(未知路径)" }}</div>
                  <div class="text-[10px] text-ink-muted truncate" :title="blob.subject">
                    {{ blob.commitHash.substring(0, 8) }} · {{ blob.author }} · {{ blob.commitDate }} · {{ blob.subject }}
                  </div>
                </div>
                <span class="text-[10px] text-ink-muted shrink-0">{{ blob.oid.substring(0, 10) }}</span>
              </div>
            </div>
            <div v-else-if="historyBlobsScanned" class="py-6 text-center text-xs text-success">
              历史中未发现可统计的大对象
            </div>
            <div v-else class="py-6 text-center text-xs text-ink-muted">
              点击「扫描历史」分析全部提交中的大 Blob
            </div>
          </div>
        </div>

        <!-- Section D: Package size budget -->
        <div class="space-y-3 pt-3" data-tool-section="disk-package-budget">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <PieChart class="w-4 h-4 text-accent" />
                <span>包体预算与平台占用 (Package Size Budget)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                按资产系统归因，对比 iOS / Android / PC / 小游戏平台的包体预算与超限风险。
              </p>
            </div>
            <button
              @click="analyzePackageBudget"
              :disabled="isAnalyzingBudget"
              class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50"
            >
              {{ isAnalyzingBudget ? "分析中..." : "分析包体预算" }}
            </button>
          </div>

          <template v-if="packageBudget">
            <div class="grid grid-cols-4 gap-2">
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">资产总量</div>
                <div class="text-sm font-bold font-mono text-ink">{{ packageBudget.totalSizeStr }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">LFS 占用</div>
                <div class="text-sm font-bold font-mono text-warning">{{ packageBudget.totalLfsSizeStr }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">资产文件</div>
                <div class="text-sm font-bold font-mono text-purple">{{ packageBudget.totalAssetsCount }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">预算状态</div>
                <div class="text-sm font-bold" :class="packageBudget.hasOverrun ? 'text-danger' : 'text-success'">
                  {{ packageBudget.hasOverrun ? `超限 · ${packageBudget.worstPlatform}` : "全部在预算内" }}
                </div>
              </div>
            </div>

            <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
              <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
                <div class="text-xs font-bold text-ink">平台预算</div>
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
                <div class="text-xs font-bold text-ink">资产系统归因</div>
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
                <div v-if="(packageBudget.systems || []).length === 0" class="text-[11px] text-success">未发现可归因资产</div>
              </div>
            </div>

            <div v-if="(packageBudget.optimizationTips || []).length > 0" class="rounded-xl border border-warning/20 bg-warning/5 p-3 space-y-1">
              <div class="text-xs font-bold text-warning">优化建议</div>
              <div
                v-for="tip in (packageBudget.optimizationTips || []).slice(0, 5)"
                :key="`${tip.category}:${tip.targetFile}`"
                class="text-[10px] text-ink-muted"
              >
                [{{ tip.priority }}] {{ tip.category }} · {{ tip.targetFile }} · 可节省 {{ tip.estimatedSavingStr }}
              </div>
            </div>
            <p class="text-[10px] text-ink-muted">{{ packageBudget.summary }}</p>
          </template>
          <p v-else class="text-[11px] text-ink-muted">尚未分析包体预算。</p>
        </div>
      </div>

</template>
