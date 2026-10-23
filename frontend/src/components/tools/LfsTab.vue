<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted, watchEffect } from "vue";
import {
  Trash2,
  HardDrive,
  Search,
  Plus,
  Copy,
  Check,
  ShieldCheck,
  Lock,
  AlertTriangle,
  Filter,
  Activity,
  Database,
  Rocket,
  History,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { toolsStatus } from "../../composables/toolsStatus";
import { useModalA11y } from "../../composables/useModalA11y";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, LfsCacheStats, LfsTuningConfig, LfsFileRevision, LfsLockActivityResult, LfsBulkUnlockResult, LfsMigratePreview, LfsMigrateResult, LockableSyncResult, PrePushVerificationResult, FileStatus } from "../../types";

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
// 4. GIT LFS STORAGE
// -------------------------------------------------------------
const lfsLocks = ref<LfsLockInfo[]>([]);
const lfsLocksError = ref("");
const lfsPatterns = ref<LfsTrackPattern[]>([]);
const isLoadingLfs = ref(false);
const newPatternInput = ref("");
const isLfsSyncing = ref<"" | "pull" | "push">("");

// Local cache stats + selective download filters
const lfsCacheStats = ref<LfsCacheStats | null>(null);
const fetchIncludeInput = ref("");
const fetchExcludeInput = ref("");
const isSavingLfsFilters = ref(false);

// Raw .gitattributes editor
const showAttributesEditor = ref(false);
const attributesContent = ref("");
const isLoadingAttributes = ref(false);
const isSavingAttributes = ref(false);

const lfsTuning = ref<LfsTuningConfig | null>(null);
const isApplyingTuning = ref("");
const lfsHistoryPath = ref("");
const lfsHistory = ref<LfsFileRevision[]>([]);
const isInspectingHistory = ref(false);
const lockActivity = ref<LfsLockActivityResult | null>(null);
const lfsDiskUsage = ref("0 B");

// ---- 历史迁移向导（git lfs migrate）--------------------------------------
const migratePatterns = ref("*.psd,*.psb,*.fbx,*.uasset,*.umap,*.tga,*.exr,*.wav,*.mp3");
const migrateEverything = ref(true);
const migrateRevRange = ref("");
const migrateCreateBundle = ref(true);
const migratePreview = ref<LfsMigratePreview | null>(null);
const isPreviewingMigrate = ref(false);
const isExecutingMigrate = ref(false);
const migrateResult = ref<LfsMigrateResult | null>(null);
const migrateExpanded = ref(false);

function parseMigratePatterns(input: string): string[] {
  return input
    .split(/[,\n]/)
    .map((p) => p.trim())
    .filter(Boolean);
}

async function handlePreviewMigrate() {
  if (!props.currentRepoPath || isPreviewingMigrate.value) return;
  const patterns = parseMigratePatterns(migratePatterns.value);
  if (patterns.length === 0) {
    notify("请先填写要迁移的文件模式，例如 *.psd");
    return;
  }
  isPreviewingMigrate.value = true;
  migrateResult.value = null;
  try {
    migratePreview.value = await gitService.previewLfsMigration(
      props.currentRepoPath,
      patterns,
      migrateEverything.value,
      migrateEverything.value ? "" : migrateRevRange.value.trim()
    );
    notify(migratePreview.value.message);
  } catch (err: any) {
    notify(`迁移预览失败: ${err?.message || err}`);
  } finally {
    isPreviewingMigrate.value = false;
  }
}

async function handleExecuteMigrate() {
  if (!props.currentRepoPath || isExecutingMigrate.value) return;
  const patterns = parseMigratePatterns(migratePatterns.value);
  if (patterns.length === 0) {
    notify("请先填写要迁移的文件模式");
    return;
  }

  const totalFiles = (migratePreview.value?.rows || []).reduce((sum, r) => sum + r.files, 0);
  const first = await confirmAction(
    { message: `即将重写历史：把 ${patterns.join("、")} 命中的 ${totalFiles || "全部"} 个历史文件转为 LFS 指针。\n\n` +
      `· 所有提交的 hash 都会改变\n` +
      `· 协作者必须重新 clone（force-push 后的旧历史无法直接 pull）\n` +
      `· 迁移期间请勿中断进程`, tone: "danger" }
  );
  if (!first) return;

  const second = await confirmAction(
    { message: `最后一次确认。\n\n${migrateCreateBundle.value ? "将先创建 .git/wgit-prefetch-*.bundle 作为回滚点，然后" : "未勾选备份，"}` +
      `执行不可逆的历史重写。\n\n继续？`, tone: "danger", requireText: "MIGRATE", requireTextHint: "输入 MIGRATE 以确认执行不可逆的历史重写" }
  );
  if (!second) return;

  isExecutingMigrate.value = true;
  try {
    const res = await gitService.executeLfsMigration(props.currentRepoPath, {
      patterns,
      everything: migrateEverything.value,
      revRange: migrateEverything.value ? "" : migrateRevRange.value.trim(),
      createBundle: migrateCreateBundle.value,
    });
    migrateResult.value = res;
    notify(res.message);
    await loadLfsData();
    emit("refresh");
  } catch (err: any) {
    notify(`迁移失败: ${err?.message || err}`);
  } finally {
    isExecutingMigrate.value = false;
  }
}

// Stale lock remediation
const zombieLocks = computed(() =>
  (lockActivity.value?.locks || []).filter((l) => l.isZombie)
);
const isUnlockingStale = ref(false);
const staleUnlockResult = ref<LfsBulkUnlockResult | null>(null);

// Blocked by a teammate's lock? Hand the user a ready-to-paste request instead
// of making them retype the path and owner into chat.
async function copyLockRequest(lock: { path: string; owner: string; durationStr: string }) {
  const text =
    `【LFS 锁释放请求】\n` +
    `文件：${lock.path}\n` +
    `持有人：${lock.owner || "未知"}\n` +
    `已锁定：${lock.durationStr || "未知"}\n` +
    `我需要修改该资产，方便时请释放锁，谢谢。`;
  try {
    await navigator.clipboard.writeText(text);
    notify("已复制锁释放请求，可直接粘贴给持有人");
  } catch {
    notify(`复制失败，请手动转达：${lock.path}（${lock.owner}）`);
  }
}

async function handleUnlockStaleLocks() {  if (!props.currentRepoPath || zombieLocks.value.length === 0 || isUnlockingStale.value) return;
  const paths = zombieLocks.value.map((l) => l.path);
  const owners = Array.from(new Set(zombieLocks.value.map((l) => l.owner).filter(Boolean)));
  const confirmed = await confirmAction(
    { message: `强制破除 ${paths.length} 把僵尸锁？\n\n` +
      `持有人：${owners.join("、") || "未知"}\n\n` +
      `· 这会把远端锁释放，其他人可以立即编辑这些资产\n` +
      `· 如果对方其实仍在编辑，可能造成资产互相覆盖`, tone: "danger" }
  );
  if (!confirmed) return;

  isUnlockingStale.value = true;
  try {
    const res = await gitService.unlockStaleLfsLocks(props.currentRepoPath, paths, true);
    staleUnlockResult.value = res;
    notify(res.message);
    await loadLfsData();
    emit("refresh");
  } catch (err: any) {
    notify(`破除僵尸锁失败: ${err?.message || err}`);
  } finally {
    isUnlockingStale.value = false;
  }
}

// Lockable asset protection / pre-push guard / lock inspection
const lockableSync = ref<LockableSyncResult | null>(null);
const isSyncingLockable = ref(false);
const isTogglingLockable = ref(false);
const bulkExtensionsInput = ref("");
const isTrackingExtensions = ref(false);
const lockFileStatusPath = ref("");
const lockFileOwner = ref<string | null>(null);
const lockStatusChecked = ref(false);
const isCheckingFileLock = ref(false);
const lockConflicts = ref<FileStatus[]>([]);
const isCheckingLockConflicts = ref(false);
const lockReportFormat = ref<"markdown" | "csv">("markdown");
const lockReportContent = ref("");
const showLockReport = ref(false);
const isExportingLockReport = ref(false);
const prePushRemote = ref("");
const prePushBranch = ref("");
const prePushResult = ref<PrePushVerificationResult | null>(null);
const isVerifyingPrePush = ref(false);

async function handleApplyTuning(preset: string) {
  if (!props.currentRepoPath || isApplyingTuning.value) return;
  const label =
    preset === "studio_high_speed" ? "工作室高速" : preset === "standard" ? "标准" : "恢复默认";
  if (!(await confirmAction(`确定应用「${label}」LFS 传输参数吗？将写入当前仓库 Git 配置。`))) return;
  isApplyingTuning.value = preset;
  try {
    const res = await gitService.applyLfsTuningOptimization(props.currentRepoPath, preset);
    notify(res?.message || `LFS 传输参数已切换为「${label}」`);
    await loadLfsData();
  } catch (err: any) {
    notify(`应用 LFS 调优失败: ${err?.message || err}`);
  } finally {
    isApplyingTuning.value = "";
  }
}

async function handleInspectLfsHistory() {
  const path = lfsHistoryPath.value.trim();
  if (!props.currentRepoPath || !path || isInspectingHistory.value) return;
  isInspectingHistory.value = true;
  try {
    lfsHistory.value = (await gitService.inspectLfsFileHistory(props.currentRepoPath, path)) || [];
    notify(`已读取 ${lfsHistory.value.length} 条文件修订记录`);
  } catch (err: any) {
    notify(`读取 LFS 文件历史失败: ${err?.message || err}`);
  } finally {
    isInspectingHistory.value = false;
  }
}

async function handleSelectivePull() {
  if (!props.currentRepoPath || isLfsSyncing.value) return;
  isLfsSyncing.value = "pull";
  try {
    await gitService.executeLfsSelectivePull(props.currentRepoPath);
    notify("已按 fetchinclude / fetchexclude 过滤规则拉取 LFS 对象");
    await loadLfsData();
  } catch (err: any) {
    notify(`选择性拉取失败: ${err?.message || err}`);
  } finally {
    isLfsSyncing.value = "";
  }
}

async function handleSyncLockable() {
  if (!props.currentRepoPath || isSyncingLockable.value) return;
  isSyncingLockable.value = true;
  try {
    const res = await gitService.syncLockableFilePermissions(props.currentRepoPath);
    lockableSync.value = res;
    notify(
      res
        ? `权限已同步：${res.totalLockableFiles} 个可锁文件，只读 ${res.readOnlyCount} / 可写 ${res.writableCount}`
        : "可锁文件权限已同步"
    );
  } catch (err: any) {
    notify(`同步可锁文件权限失败: ${err?.message || err}`);
  } finally {
    isSyncingLockable.value = false;
  }
}

async function handleToggleLockable() {
  if (!props.currentRepoPath || isTogglingLockable.value) return;
  const enable = !(lockableSync.value?.isProtectionActive ?? false);
  if (
    !(await confirmAction(
      enable
        ? "确定启用可锁资产只读保护吗？未由你锁定的资产将变为只读 (0444)。"
        : "确定关闭可锁资产只读保护吗？现有资产将恢复为可写 (0644)。"
    ))
  )
    return;
  isTogglingLockable.value = true;
  try {
    const res = await gitService.toggleLockableReadOnlyProtection(props.currentRepoPath, enable);
    lockableSync.value = res;
    notify(enable ? "已启用可锁资产只读保护" : "已关闭可锁资产只读保护");
  } catch (err: any) {
    notify(`切换只读保护失败: ${err?.message || err}`);
  } finally {
    isTogglingLockable.value = false;
  }
}

async function handleTrackExtensions() {
  const exts = parseLfsPatterns(bulkExtensionsInput.value);
  if (!props.currentRepoPath || exts.length === 0 || isTrackingExtensions.value) return;
  if (!(await confirmAction(`确定把 ${exts.join("、")} 纳入 Git LFS 跟踪吗？常见可锁资产会自动附加 --lockable。`)))
    return;
  isTrackingExtensions.value = true;
  try {
    await gitService.trackExtensionsAsLfs(props.currentRepoPath, exts);
    notify(`已纳入 LFS 跟踪: ${exts.join("、")}`);
    bulkExtensionsInput.value = "";
    await loadLfsData();
  } catch (err: any) {
    notify(`批量跟踪失败: ${err?.message || err}`);
  } finally {
    isTrackingExtensions.value = false;
  }
}

async function handleCheckFileLock() {
  const path = lockFileStatusPath.value.trim();
  if (!props.currentRepoPath || !path || isCheckingFileLock.value) return;
  isCheckingFileLock.value = true;
  try {
    const owner = await gitService.checkFileLockStatus(props.currentRepoPath, path);
    lockFileOwner.value = owner || "";
    lockStatusChecked.value = true;
  } catch (err: any) {
    notify(`查询文件锁状态失败: ${err?.message || err}`);
  } finally {
    isCheckingFileLock.value = false;
  }
}

async function handleCheckLockConflicts() {
  if (!props.currentRepoPath || isCheckingLockConflicts.value) return;
  isCheckingLockConflicts.value = true;
  try {
    lockConflicts.value = (await gitService.checkRemoteLocksConflict(props.currentRepoPath)) || [];
    notify(
      lockConflicts.value.length > 0
        ? `发现 ${lockConflicts.value.length} 个本地修改与团队锁冲突`
        : "未发现本地修改与团队锁冲突"
    );
  } catch (err: any) {
    notify(`检测锁冲突失败: ${err?.message || err}`);
  } finally {
    isCheckingLockConflicts.value = false;
  }
}

async function handleExportLockReport() {
  if (!props.currentRepoPath || isExportingLockReport.value) return;
  isExportingLockReport.value = true;
  try {
    lockReportContent.value =
      (await gitService.exportLfsLocksReport(props.currentRepoPath, lockReportFormat.value)) || "";
    showLockReport.value = true;
    notify(`已生成 ${lockReportFormat.value === "csv" ? "CSV" : "Markdown"} 锁审计报表`);
  } catch (err: any) {
    notify(`导出锁报表失败: ${err?.message || err}`);
  } finally {
    isExportingLockReport.value = false;
  }
}

async function handleVerifyPrePush() {
  if (!props.currentRepoPath || isVerifyingPrePush.value) return;
  isVerifyingPrePush.value = true;
  try {
    prePushResult.value = await gitService.verifyPrePushLfs(
      props.currentRepoPath,
      prePushRemote.value.trim(),
      prePushBranch.value.trim()
    );
    if (prePushResult.value) {
      notify(prePushResult.value.statusSummary || "推送前 LFS 校验完成");
    }
  } catch (err: any) {
    notify(`推送前 LFS 校验失败: ${err?.message || err}`);
  } finally {
    isVerifyingPrePush.value = false;
  }
}

async function toggleAttributesEditor() {
  showAttributesEditor.value = !showAttributesEditor.value;
  if (!showAttributesEditor.value || !props.currentRepoPath) return;
  isLoadingAttributes.value = true;
  try {
    attributesContent.value = (await gitService.getGitAttributes(props.currentRepoPath)) || "";
  } catch (err: any) {
    notify(`读取 .gitattributes 失败: ${err?.message || err}`);
  } finally {
    isLoadingAttributes.value = false;
  }
}

async function handleSaveAttributes() {
  if (!props.currentRepoPath || isSavingAttributes.value) return;
  isSavingAttributes.value = true;
  try {
    await gitService.saveGitAttributes(props.currentRepoPath, attributesContent.value);
    notify(".gitattributes 已保存");
    showAttributesEditor.value = false;
    await loadLfsData();
    emit("refresh");
  } catch (err: any) {
    notify(`保存 .gitattributes 失败: ${err?.message || err}`);
  } finally {
    isSavingAttributes.value = false;
  }
}

function parseLfsPatterns(input: string): string[] {
  return input
    .split(/[,\n]/)
    .map((p) => p.trim())
    .filter(Boolean);
}

async function loadLfsData() {
  if (!props.currentRepoPath) return;
  isLoadingLfs.value = true;
  lfsLocksError.value = "";
  try {
    const locksPromise = gitService
      .getLfsLocks(props.currentRepoPath)
      .then((locks) => locks || [])
      .catch((err: any) => {
        lfsLocksError.value = err?.message || String(err);
        return [];
      });
    const [locks, patterns, cache, filters, tuning, activity, diskUsage] = await Promise.all([
      locksPromise,
      gitService.getLfsTrackedPatterns(props.currentRepoPath).catch(() => []),
      gitService.getLfsCacheStats(props.currentRepoPath).catch(() => null),
      gitService.getLfsFilterConfig(props.currentRepoPath).catch(() => null),
      gitService.getLfsTuningConfig(props.currentRepoPath).catch(() => null),
      gitService.getLfsLockActivity(props.currentRepoPath).catch(() => null),
      gitService.getLfsDiskUsage(props.currentRepoPath).catch(() => "0 B"),
    ]);
    lfsLocks.value = locks || [];
    lfsPatterns.value = patterns || [];
    lfsCacheStats.value = cache;
    lfsTuning.value = tuning;
    lockActivity.value = activity;
    lfsDiskUsage.value = diskUsage || "0 B";
    fetchIncludeInput.value = (filters?.fetchInclude || []).join(", ");
    fetchExcludeInput.value = (filters?.fetchExclude || []).join(", ");
  } catch (err) {
    console.error("Failed to load LFS data:", err);
  } finally {
    isLoadingLfs.value = false;
  }
}

async function handleSaveLfsFilters() {
  if (!props.currentRepoPath || isSavingLfsFilters.value) return;
  isSavingLfsFilters.value = true;
  try {
    await gitService.setLfsFilterConfig(
      props.currentRepoPath,
      parseLfsPatterns(fetchIncludeInput.value),
      parseLfsPatterns(fetchExcludeInput.value)
    );
    notify("LFS 选择性下载过滤已保存，将在后续 fetch / pull 时生效");
    await loadLfsData();
  } catch (err: any) {
    notify(`保存 LFS 下载过滤失败: ${err?.message || err}`);
  } finally {
    isSavingLfsFilters.value = false;
  }
}

async function handleLfsPull() {
  if (!props.currentRepoPath || isLfsSyncing.value) return;
  isLfsSyncing.value = "pull";
  try {
    await gitService.lfsPull(props.currentRepoPath);
    notify("已拉取 Git LFS 对象");
    await loadLfsData();
  } catch (err: any) {
    notify(`LFS 拉取失败: ${err?.message || err}`);
  } finally {
    isLfsSyncing.value = "";
  }
}

async function handleLfsPush() {
  if (!props.currentRepoPath || isLfsSyncing.value) return;
  if (!(await confirmAction("确定将全部 Git LFS 对象推送到远端吗？"))) return;
  isLfsSyncing.value = "push";
  try {
    await gitService.lfsPush(props.currentRepoPath);
    notify("已将 Git LFS 对象推送到远端");
  } catch (err: any) {
    notify(`LFS 推送失败: ${err?.message || err}`);
  } finally {
    isLfsSyncing.value = "";
  }
}

async function handleTrackPattern() {
  if (!props.currentRepoPath || !newPatternInput.value.trim()) return;
  try {
    await gitService.trackLfsPattern(props.currentRepoPath, newPatternInput.value.trim());
    notify(`已将 ${newPatternInput.value.trim()} 加入 LFS 跟踪`);
    newPatternInput.value = "";
    await loadLfsData();
  } catch (err: any) {
    notify(`跟踪失败: ${err?.message || err}`);
  }
}

async function handleUntrackPattern(pattern: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction(`确定要取消 LFS 对「${pattern}」的跟踪吗？`))) return;
  try {
    await gitService.untrackLfsPattern(props.currentRepoPath, pattern);
    notify(`已取消跟踪: ${pattern}`);
    await loadLfsData();
  } catch (err: any) {
    notify(`取消失败: ${err?.message || err}`);
  }
}

const isLockingAllLfs = ref(false);
const showLockFileModal = ref(false);
const lockFilePathInput = ref("");

const { containerRef: lockFileDialogRef } = useModalA11y(
  () => showLockFileModal.value,
  () => { showLockFileModal.value = false; }
);

async function handleLockAllModifiedLfs() {
  if (!props.currentRepoPath) return;
  isLockingAllLfs.value = true;
  try {
    const res = await gitService.lockAllModifiedLfs(props.currentRepoPath);
    if (res) {
      notify(res.message || `成功锁定 ${res.totalCount} 个改动的大文件`);
      await loadLfsData();
    }
  } catch (err: any) {
    notify(`批量锁定失败: ${err?.message || err}`);
  } finally {
    isLockingAllLfs.value = false;
  }
}

async function handleLockFile() {
  if (!props.currentRepoPath || !lockFilePathInput.value.trim()) return;
  try {
    await gitService.lockFile(props.currentRepoPath, lockFilePathInput.value.trim());
    notify(`文件已成功加锁: ${lockFilePathInput.value}`);
    lockFilePathInput.value = "";
    showLockFileModal.value = false;
    await loadLfsData();
  } catch (err: any) {
    notify(`加锁失败: ${err?.message || err}`);
  }
}

async function handleUnlockFile(path: string, force = false) {
  if (!props.currentRepoPath || !path) return;
  if (!(await confirmAction({ message: `确定要${force ? '强制' : ''}释放该文件的 LFS 锁吗？(${path})`, tone: "danger" }))) return;
  try {
    await gitService.unlockFile(props.currentRepoPath, path, force);
    notify(`已释放锁定: ${path}`);
    await loadLfsData();
  } catch (err: any) {
    notify(`解锁失败: ${err?.message || err}`);
  }
}

function copyToClipboard(text: string, label: string) {
  navigator.clipboard.writeText(text);
  notify(`已复制 ${label}`);
}


watchEffect(() => {
  toolsStatus.loading.lfs =
    isLoadingLfs.value ||
    isLockingAllLfs.value ||
    isSavingLfsFilters.value ||
    isLoadingAttributes.value ||
    isSavingAttributes.value ||
    !!isApplyingTuning.value ||
    isInspectingHistory.value ||
    isSyncingLockable.value ||
    isTogglingLockable.value ||
    isTrackingExtensions.value ||
    isCheckingFileLock.value ||
    isCheckingLockConflicts.value ||
    isExportingLockReport.value ||
    isVerifyingPrePush.value;
});

onMounted(() => {
  void loadLfsData();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadLfsData();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 4: 大文件存储与锁定 (Git LFS)              -->
      <!-- ============================================== -->
      <div class="space-y-6">
        <div
          v-if="lfsLocksError"
          class="flex items-start gap-2 p-3 rounded-xl border border-warning/30 bg-warning/10 text-warning text-xs"
        >
          <AlertTriangle class="w-4 h-4 shrink-0" />
          <span>Git LFS 锁列表读取失败，当前无法确认文件锁定状态：{{ lfsLocksError }}</span>
        </div>

        <!-- Section A: Tracked Patterns -->
        <div class="space-y-3" data-tool-section="lfs-patterns">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <HardDrive class="w-4 h-4 text-accent" />
                <span>Git LFS 规则配置与跟踪模式 (.gitattributes)</span>
              </h3>
            </div>
            <div class="flex items-center gap-2">
              <button
                @click="toggleAttributesEditor"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer"
                title="查看 / 编辑仓库根目录的 .gitattributes"
              >
                {{ showAttributesEditor ? "收起编辑" : "编辑 .gitattributes" }}
              </button>
              <button
                @click="handleLfsPull"
                :disabled="!!isLfsSyncing"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                title="拉取当前仓库的 Git LFS 对象 (git lfs pull)"
              >
                {{ isLfsSyncing === "pull" ? "拉取中..." : "LFS 拉取" }}
              </button>
              <button
                @click="handleSelectivePull"
                :disabled="!!isLfsSyncing || (!fetchIncludeInput.trim() && !fetchExcludeInput.trim())"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                :title="
                  fetchIncludeInput.trim() || fetchExcludeInput.trim()
                    ? '按已配置的 fetchinclude / fetchexclude 规则执行 git lfs pull'
                    : '请先在下方配置包含或排除模式'
                "
              >
                选择性拉取
              </button>
              <button
                @click="handleLfsPush"
                :disabled="!!isLfsSyncing"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                title="推送全部 Git LFS 对象到远端 (git lfs push <remote> --all)"
              >
                {{ isLfsSyncing === "push" ? "推送中..." : "LFS 推送" }}
              </button>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <input
              v-model="newPatternInput"
              type="text"
              placeholder="输入大文件通配符 (例如: *.psd, *.zip, *.mp4, Assets/**.bin)..."
              @keydown.enter="handleTrackPattern"
              class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <button
              @click="handleTrackPattern"
              :disabled="!newPatternInput.trim()"
              class="px-4 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              添加跟踪规则
            </button>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 pt-2">
            <div
              v-for="pat in lfsPatterns"
              :key="pat.pattern"
              class="p-2.5 rounded-lg bg-surface/60 border border-line flex items-center justify-between group"
            >
              <span class="font-mono text-xs text-ink font-bold truncate">{{ pat.pattern }}</span>
              <button aria-label="取消跟踪该模式"
                @click="handleUntrackPattern(pat.pattern)"
                class="opacity-0 group-hover:opacity-100 p-1 hover:bg-danger/20 text-ink-muted hover:text-danger rounded transition"
                title="取消跟踪该模式"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>

            <div v-if="lfsPatterns.length === 0" class="col-span-4 py-6 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl">
              暂无配置 Git LFS 跟踪模式
            </div>
          </div>

          <div v-if="showAttributesEditor" class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-ink">直接编辑 .gitattributes</span>
              <span class="text-[10px] text-ink-muted">保存后立即影响 LFS、换行与 diff 规则</span>
            </div>
            <div v-if="isLoadingAttributes" class="py-6 text-center text-xs text-ink-muted">正在读取 .gitattributes...</div>
            <textarea
              v-else
              v-model="attributesContent"
              rows="10"
              spellcheck="false"
              placeholder="*.psd filter=lfs diff=lfs merge=lfs -text&#10;*.uasset filter=lfs diff=lfs merge=lfs -text"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono leading-relaxed placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
            ></textarea>
            <div class="flex items-center justify-end gap-2">
              <button
                @click="showAttributesEditor = false"
                class="px-3 py-1.5 rounded-lg hover:bg-raised text-xs text-ink-muted hover:text-ink transition cursor-pointer"
              >
                取消
              </button>
              <button
                @click="handleSaveAttributes"
                :disabled="isSavingAttributes || isLoadingAttributes"
                class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50"
              >
                {{ isSavingAttributes ? "保存中..." : "保存 .gitattributes" }}
              </button>
            </div>
          </div>
        </div>

        <!-- Section B: Active LFS Locks -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-locks">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Lock class="w-4 h-4 text-warning" />
                <span>活跃文件锁定 (Git LFS Locks)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                实时查看远端团队成员已锁定的二进制大文件，避免团队协作覆盖冲突。
              </p>
            </div>

            <div class="flex items-center gap-2">
              <button
                @click="handleLockAllModifiedLfs"
                :disabled="isLockingAllLfs"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-50"
                title="自动扫描并向远端锁定当前所有改动的 LFS 资产文件"
              >
                <Lock class="w-3.5 h-3.5" :class="{ 'animate-spin': isLockingAllLfs }" />
                <span>一键锁定已修改资产</span>
              </button>
              <button
                @click="showLockFileModal = true"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer shadow-xs"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>锁定文件</span>
              </button>
            </div>
          </div>

          <div class="border border-line rounded-xl bg-surface/30 overflow-hidden">
            <div v-if="lfsLocks.length > 0" class="divide-y divide-line/40 font-mono text-xs">
              <div
                v-for="lock in lfsLocks"
                :key="lock.id"
                class="p-3 flex items-center justify-between hover:bg-surface/60 transition group"
              >
                <div class="flex items-center gap-3 truncate min-w-0">
                  <Lock class="w-3.5 h-3.5 text-warning shrink-0" />
                  <span class="font-bold text-ink truncate">{{ lock.path }}</span>
                  <span class="px-1.5 py-0.2 rounded bg-warning/20 text-warning text-[10px] font-sans font-bold">
                    {{ lock.owner }}
                  </span>
                </div>

                <div class="flex items-center gap-3 shrink-0">
                  <span class="text-ink-muted text-[11px] font-sans">{{ lock.lockedAt }}</span>
                  <button
                    @click="handleUnlockFile(lock.path, false)"
                    class="opacity-0 group-hover:opacity-100 px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[11px] text-ink transition cursor-pointer"
                    title="释放锁定"
                  >
                    解锁
                  </button>
                  <button aria-label="强制释放锁定 (--force)"
                    @click="handleUnlockFile(lock.path, true)"
                    class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                    title="强制释放锁定 (--force)"
                  >
                    <Trash2 class="w-3 h-3" />
                  </button>
                </div>
              </div>
            </div>

            <div v-else class="py-8 text-center text-xs text-ink-muted flex flex-col items-center gap-2">
              <Check class="w-6 h-6 text-success" />
              <span>当前无任何被锁定的 LFS 资产文件</span>
            </div>
          </div>
        </div>

        <!-- Section C: Local cache + selective download filters -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-cache">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Database class="w-4 h-4 text-purple" />
              <span>本地 LFS 缓存与选择性下载</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              查看 .git/lfs/objects 缓存占用；用 fetchinclude / fetchexclude 控制后续 fetch / pull 实际下载的资产范围。
            </p>
          </div>

          <div class="grid grid-cols-3 gap-3">
            <div class="p-3 rounded-xl border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">本地缓存体积</div>
              <div class="text-lg font-bold font-mono text-ink">{{ lfsCacheStats?.totalSizeStr || "0 B" }}</div>
            </div>
            <div class="p-3 rounded-xl border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">缓存对象文件数</div>
              <div class="text-lg font-bold font-mono text-purple">{{ lfsCacheStats?.fileCount ?? 0 }}</div>
            </div>
            <div class="p-3 rounded-xl border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">LFS 磁盘占用</div>
              <div class="text-lg font-bold font-mono text-ink">{{ lfsDiskUsage }}</div>
            </div>
          </div>
          <div class="px-3 py-2 rounded-lg border border-line bg-canvas/50 font-mono text-[10px] text-ink-muted truncate" :title="lfsCacheStats?.cachePath">
            {{ lfsCacheStats?.cachePath || ".git/lfs/objects" }}
          </div>

          <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2.5">
            <div class="grid grid-cols-2 gap-3">
              <label class="flex flex-col gap-1">
                <span class="text-[10px] text-ink-muted font-semibold">包含模式 (lfs.fetchinclude)</span>
                <input
                  v-model="fetchIncludeInput"
                  type="text"
                  placeholder="Assets/Characters/**, Assets/UI/**"
                  class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-[10px] text-ink-muted font-semibold">排除模式 (lfs.fetchexclude)</span>
                <input
                  v-model="fetchExcludeInput"
                  type="text"
                  placeholder="Assets/Audio/**, Assets/Videos/**"
                  class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
              </label>
            </div>
            <div class="flex items-center justify-between gap-3">
              <p class="text-[10px] text-ink-muted">逗号或换行分隔，支持 glob；留空表示不启用过滤。</p>
              <button
                @click="handleSaveLfsFilters"
                :disabled="isSavingLfsFilters"
                class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-50 shrink-0"
              >
                {{ isSavingLfsFilters ? "保存中..." : "保存过滤规则" }}
              </button>
            </div>
          </div>
        </div>

        <!-- Section D: LFS transfer tuning -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-tuning">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Activity class="w-4 h-4 text-warning" />
              <span>LFS 传输调优</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              调整并发传输、HTTP 缓冲区、超时与压缩级别；适合大资产团队按网络环境切换预设。
            </p>
          </div>

          <div v-if="lfsTuning" class="grid grid-cols-2 lg:grid-cols-4 gap-2">
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">并发传输</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lfsTuning.concurrentTransfers }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">HTTP 缓冲</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lfsTuning.postBufferMb }} MB</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">拨号超时</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lfsTuning.dialTimeoutSec }} s</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">压缩级别</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lfsTuning.compressionLevel }}</div>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              @click="handleApplyTuning('studio_high_speed')"
              :disabled="!!isApplyingTuning"
              class="px-2.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              {{ isApplyingTuning === "studio_high_speed" ? "应用中..." : "工作室高速 (16 并发)" }}
            </button>
            <button
              @click="handleApplyTuning('standard')"
              :disabled="!!isApplyingTuning"
              class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
            >
              {{ isApplyingTuning === "standard" ? "应用中..." : "标准 (8 并发)" }}
            </button>
            <button
              @click="handleApplyTuning('reset_default')"
              :disabled="!!isApplyingTuning"
              class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink-muted transition cursor-pointer disabled:opacity-40"
            >
              {{ isApplyingTuning === "reset_default" ? "应用中..." : "恢复默认" }}
            </button>
            <span v-if="lfsTuning" class="text-[10px] text-ink-muted truncate">{{ lfsTuning.activeSummary }}</span>
          </div>
        </div>

        <!-- Section E: LFS file revision history -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-revisions">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Search class="w-4 h-4 text-cyan" />
              <span>LFS 文件修订历史</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">按文件路径追踪修订记录，适合排查大资产的历史版本与体积变化。</p>
          </div>
          <div class="flex items-center gap-2">
            <input
              v-model="lfsHistoryPath"
              type="text"
              placeholder="文件相对路径，例如 Assets/Characters/hero.psd"
              @keydown.enter="handleInspectLfsHistory"
              class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <button
              @click="handleInspectLfsHistory"
              :disabled="!lfsHistoryPath.trim() || isInspectingHistory"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isInspectingHistory ? "读取中..." : "查看历史" }}
            </button>
          </div>
          <div v-if="lfsHistory.length > 0" class="rounded-xl border border-line bg-surface/30 divide-y divide-line/40 max-h-64 overflow-y-auto font-mono text-[11px]">
            <div v-for="rev in lfsHistory" :key="rev.commitHash" class="px-3 py-2 flex items-center gap-2">
              <span class="text-accent font-bold shrink-0">{{ rev.shortHash }}</span>
              <span class="text-ink truncate flex-1">{{ rev.subject }}</span>
              <span class="text-ink-muted shrink-0">{{ rev.authorName }}</span>
              <span class="text-ink-muted shrink-0">{{ rev.sizeHuman || "—" }}</span>
            </div>
          </div>
          <div v-else-if="isInspectingHistory === false" class="py-2 text-center text-[11px] text-ink-muted">
            {{ lfsHistoryPath ? "暂无历史记录" : "输入文件路径后查看修订历史" }}
          </div>
        </div>

        <!-- Section F: LFS lock activity -->
        <!-- History migration wizard -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-migrate">
          <div class="pb-2 border-b border-line flex items-start justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <History class="w-4 h-4 text-purple" />
                <span>历史大文件迁移 (lfs migrate)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                把历史提交里的资产改写成 LFS 指针，清掉早期「裸 git 存资产」留下的体积包袱。
                <span class="text-warning font-semibold">会重写提交，不可逆。</span>
              </p>
            </div>
            <button
              @click="migrateExpanded = !migrateExpanded"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer shrink-0"
            >
              {{ migrateExpanded ? "收起" : "展开向导" }}
            </button>
          </div>

          <template v-if="migrateExpanded">
            <div class="rounded-xl border border-danger/25 bg-danger/5 p-3 space-y-1.5">
              <div class="text-[11px] font-bold text-danger flex items-center gap-1.5">
                <AlertTriangle class="w-3.5 h-3.5" />
                执行前请确认
              </div>
              <ul class="text-[10px] text-ink-muted leading-relaxed list-disc pl-4 space-y-0.5">
                <li>所有提交的 hash 都会改变，远端需要 <span class="font-mono">push --force --all --tags</span>。</li>
                <li>协作者必须重新 clone，直接 pull 会因为历史分叉失败。</li>
                <li>建议先勾选「创建回滚 Bundle」，出问题可用它恢复原始历史。</li>
              </ul>
            </div>

            <div class="space-y-2">
              <label class="text-[11px] font-semibold text-ink-secondary">文件模式（逗号分隔）</label>
              <input
                v-model="migratePatterns"
                spellcheck="false"
                placeholder="*.psd,*.fbx,*.uasset"
                class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              />
            </div>

            <div class="flex flex-wrap items-center gap-3 text-[11px]">
              <label class="flex items-center gap-1.5 cursor-pointer">
                <input v-model="migrateEverything" type="checkbox" class="accent-[var(--ug-accent-strong)]" />
                <span class="text-ink">全部历史（--everything）</span>
              </label>
              <div v-if="!migrateEverything" class="flex items-center gap-1.5">
                <span class="text-ink-muted">分支 / 引用</span>
                <input
                  v-model="migrateRevRange"
                  spellcheck="false"
                  placeholder="feature"
                  title="--include-ref 只接受单个分支或引用，不支持 main..feature 范围"
                  class="w-40 px-2 py-1 rounded-md bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                />
              </div>
              <label class="flex items-center gap-1.5 cursor-pointer">
                <input v-model="migrateCreateBundle" type="checkbox" class="accent-[var(--ug-accent-strong)]" />
                <span class="text-ink">创建回滚 Bundle</span>
              </label>
            </div>

            <div class="flex items-center gap-2">
              <button
                @click="handlePreviewMigrate"
                :disabled="isPreviewingMigrate"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              >
                <Search class="w-3.5 h-3.5" :class="{ 'animate-pulse': isPreviewingMigrate }" />
                <span>{{ isPreviewingMigrate ? "预览中..." : "预览影响范围" }}</span>
              </button>
              <button
                @click="handleExecuteMigrate"
                :disabled="isExecutingMigrate || !migratePreview || migratePreview.rows.length === 0"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-danger-strong hover:bg-danger-strong/80 text-white text-xs font-semibold transition cursor-pointer disabled:opacity-30"
                :title="!migratePreview ? '请先预览影响范围' : '执行历史重写'"
              >
                <Rocket class="w-3.5 h-3.5" />
                <span>{{ isExecutingMigrate ? "迁移中（请勿关闭）..." : "执行迁移" }}</span>
              </button>
            </div>

            <div v-if="migratePreview" class="rounded-lg border border-line bg-canvas/40 p-2.5 space-y-1.5">
              <p class="text-[11px] text-ink-secondary">{{ migratePreview.message }}</p>
              <div v-if="migratePreview.rows.length" class="space-y-0.5">
                <div
                  v-for="row in migratePreview.rows"
                  :key="row.pattern"
                  class="flex items-center gap-2 text-[11px] font-mono"
                >
                  <span class="text-ink truncate flex-1">{{ row.pattern }}</span>
                  <span class="text-ink-muted shrink-0">{{ row.files }} 个</span>
                  <span class="text-warning shrink-0">{{ row.size }}</span>
                </div>
              </div>
              <p class="text-[10px] text-ink-muted/70 font-mono truncate" :title="migratePreview.command">
                {{ migratePreview.command }}
              </p>
            </div>

            <div
              v-if="migrateResult"
              class="rounded-lg border p-2.5 space-y-1 text-[11px]"
              :class="migrateResult.success ? 'border-success/30 bg-success/10' : 'border-warning/30 bg-warning/10'"
            >
              <div class="font-semibold text-ink">{{ migrateResult.message }}</div>
              <div v-if="migrateResult.bundlePath" class="font-mono text-[10px] text-ink-muted truncate">
                回滚点：{{ migrateResult.bundlePath }}
              </div>
              <div v-if="migrateResult.rewrittenFrom && migrateResult.rewrittenTo" class="font-mono text-[10px] text-ink-muted">
                HEAD {{ migrateResult.rewrittenFrom.slice(0, 7) }} → {{ migrateResult.rewrittenTo.slice(0, 7) }}
              </div>
            </div>
          </template>
        </div>

        <div class="space-y-3 pt-4" data-tool-section="lfs-lock-activity">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Lock class="w-4 h-4 text-warning" />
              <span>锁活动与冲突巡检</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">汇总我的 / 团队锁、僵尸锁与本地修改冲突，避免资产覆盖。</p>
          </div>
          <div v-if="lockActivity" class="grid grid-cols-2 lg:grid-cols-5 gap-2">
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">总锁数</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lockActivity.totalLocks }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">我的锁</div>
              <div class="text-sm font-bold font-mono text-accent">{{ lockActivity.myLockCount }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">团队锁</div>
              <div class="text-sm font-bold font-mono text-purple">{{ lockActivity.teamLockCount }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">僵尸锁</div>
              <div class="text-sm font-bold font-mono text-warning">{{ lockActivity.zombieCount }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">冲突</div>
              <div class="text-sm font-bold font-mono text-danger">{{ lockActivity.collisionCount }}</div>
            </div>
          </div>
          <div v-if="zombieLocks.length > 0" class="flex items-center justify-between gap-3 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2">
            <div class="text-[11px] text-ink leading-relaxed">
              <span class="font-semibold text-warning">{{ zombieLocks.length }} 把僵尸锁</span>
              长期无人释放，会阻塞其他人编辑这些资产。
            </div>
            <button
              @click="handleUnlockStaleLocks"
              :disabled="isUnlockingStale"
              class="px-3 py-1.5 rounded-lg bg-warning-strong hover:bg-warning-strong-hover text-warning-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isUnlockingStale ? "破除中..." : "批量破除" }}
            </button>
          </div>
          <div
            v-if="staleUnlockResult"
            class="rounded-lg border p-2.5 space-y-1 text-[11px]"
            :class="staleUnlockResult.failed > 0 ? 'border-warning/30 bg-warning/10' : 'border-success/30 bg-success/10'"
          >
            <div class="font-semibold text-ink">{{ staleUnlockResult.message }}</div>
            <div
              v-for="item in staleUnlockResult.items"
              :key="item.path"
              class="flex items-start gap-2 font-mono text-[10px]"
            >
              <span
                class="shrink-0 px-1 rounded font-bold"
                :class="item.status === 'unlocked' ? 'bg-success-strong text-success-contrast' : 'bg-warning-strong text-warning-contrast'"
              >{{ item.status === "unlocked" ? "OK" : "FAIL" }}</span>
              <span class="text-ink-muted break-all">{{ item.path }}</span>
              <span v-if="item.detail" class="text-ink-muted/70 shrink-0">{{ item.detail }}</span>
            </div>
          </div>
          <div v-if="lockActivity && lockActivity.locks.length > 0" class="space-y-1 max-h-52 overflow-y-auto">
            <div
              v-for="lock in lockActivity.locks"
              :key="`${lock.path}:${lock.owner}`"
              class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 text-[11px]"
            >
              <span
                class="px-1 py-0.2 rounded text-[9px] font-bold shrink-0"
                :class="lock.hasConflict ? 'bg-danger/15 text-danger' : lock.isZombie ? 'bg-warning/15 text-warning' : 'bg-surface border border-line text-ink-muted'"
              >
                {{ lock.hasConflict ? "冲突" : lock.isZombie ? "僵尸" : lock.category }}
              </span>
              <span class="font-mono text-ink truncate flex-1" :title="lock.path">{{ lock.path }}</span>
              <span class="text-ink-muted shrink-0">{{ lock.owner }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ lock.durationStr }}</span>
              <button aria-label="复制锁释放请求，粘贴到 IM 里找人解锁"
                v-if="!lock.isMe || lock.isZombie"
                @click="copyLockRequest(lock)"
                class="shrink-0 p-0.5 rounded text-ink-muted/40 hover:text-accent hover:bg-accent/10 transition cursor-pointer"
                title="复制锁释放请求，粘贴到 IM 里找人解锁"
              >
                <Copy class="w-3 h-3" />
              </button>
            </div>
          </div>
          <p v-else-if="lockActivity" class="text-[11px] text-ink-muted">{{ lockActivity.message }}</p>

          <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2.5">
            <div class="flex flex-wrap items-center gap-2">
              <button
                @click="handleCheckLockConflicts"
                :disabled="isCheckingLockConflicts"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                <AlertTriangle class="w-3.5 h-3.5 text-warning" />
                <span>{{ isCheckingLockConflicts ? "检测中..." : "检测本地修改与团队锁冲突" }}</span>
              </button>
              <div class="flex items-center gap-2 flex-1 min-w-[260px]">
                <input
                  v-model="lockFileStatusPath"
                  type="text"
                  placeholder="查询单个文件的锁持有者，例如 Assets/hero.psd"
                  @keydown.enter="handleCheckFileLock"
                  class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
                <button
                  @click="handleCheckFileLock"
                  :disabled="!lockFileStatusPath.trim() || isCheckingFileLock"
                  class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
                >
                  {{ isCheckingFileLock ? "查询中..." : "查询锁状态" }}
                </button>
              </div>
              <div class="flex items-center gap-1.5">
                <select
                  v-model="lockReportFormat"
                  aria-label="锁审计报表格式"
                  class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
                >
                  <option value="markdown">Markdown</option>
                  <option value="csv">CSV</option>
                </select>
                <button
                  @click="handleExportLockReport"
                  :disabled="isExportingLockReport"
                  class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
                >
                  {{ isExportingLockReport ? "生成中..." : "导出锁报表" }}
                </button>
              </div>
            </div>

            <p
              v-if="lockStatusChecked"
              class="text-[11px] font-mono"
              :class="lockFileOwner ? 'text-warning' : 'text-success'"
            >
              {{ lockFileOwner ? `${lockFileStatusPath} 已被 ${lockFileOwner} 锁定` : `${lockFileStatusPath} 当前未被锁定` }}
            </p>

            <div v-if="lockConflicts.length > 0" class="space-y-1">
              <div
                v-for="conflict in lockConflicts"
                :key="conflict.path"
                class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border border-danger/30 bg-danger/5 text-[11px]"
              >
                <span class="px-1 py-0.2 rounded bg-danger/15 text-danger text-[9px] font-bold shrink-0">冲突</span>
                <span class="font-mono text-ink truncate flex-1" :title="conflict.path">{{ conflict.path }}</span>
                <span class="text-ink-muted shrink-0">持有者 {{ conflict.lockOwner || "未知" }}</span>
              </div>
            </div>

            <div v-if="showLockReport" class="space-y-2 pt-1">
              <div class="flex items-center justify-between">
                <span class="text-[11px] font-bold text-ink">锁审计报表预览</span>
                <div class="flex items-center gap-1.5">
                  <button
                    @click="copyToClipboard(lockReportContent, '锁审计报表')"
                    class="flex items-center gap-1 px-2 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[11px] text-ink transition cursor-pointer"
                  >
                    <Copy class="w-3 h-3" />
                    <span>复制</span>
                  </button>
                  <button
                    @click="showLockReport = false"
                    class="px-2 py-1 rounded-md hover:bg-raised text-[11px] text-ink-muted hover:text-ink transition cursor-pointer"
                  >
                    收起
                  </button>
                </div>
              </div>
              <textarea
                :value="lockReportContent"
                readonly
                rows="8"
                spellcheck="false"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-[11px] text-ink font-mono leading-relaxed focus:outline-none"
              ></textarea>
            </div>
          </div>
        </div>

        <!-- Section G: Lockable protection + pre-push LFS guard -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 pt-4" data-tool-section="lfs-lockable">
          <div class="space-y-3">
            <div class="pb-2 border-b border-line">
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <ShieldCheck class="w-4 h-4 text-success" />
                <span>可锁资产只读保护</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                未由你锁定的可锁资产自动置为只读，减少团队并行编辑时的覆盖风险。
              </p>
            </div>
            <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
              <div class="flex items-center justify-between gap-2">
                <span class="text-xs font-semibold text-ink">lfs.locksverify</span>
                <span
                  class="px-2 py-0.5 rounded text-[10px] font-bold"
                  :class="
                    lockableSync?.isProtectionActive
                      ? 'bg-success/15 text-success'
                      : 'bg-surface border border-line text-ink-muted'
                  "
                >
                  {{ lockableSync ? (lockableSync.isProtectionActive ? "已启用" : "未启用") : "未检测" }}
                </span>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  @click="handleSyncLockable"
                  :disabled="isSyncingLockable"
                  class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
                >
                  {{ isSyncingLockable ? "同步中..." : "同步可锁文件权限" }}
                </button>
                <button
                  @click="handleToggleLockable"
                  :disabled="isTogglingLockable"
                  class="px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer disabled:opacity-50"
                  :class="
                    lockableSync?.isProtectionActive
                      ? 'bg-surface hover:bg-raised border border-line text-ink'
                      : 'bg-success-strong hover:opacity-90 text-white'
                  "
                >
                  {{
                    isTogglingLockable
                      ? "切换中..."
                      : lockableSync?.isProtectionActive
                        ? "关闭只读保护"
                        : "启用只读保护"
                  }}
                </button>
              </div>
              <div v-if="lockableSync" class="grid grid-cols-3 gap-2">
                <div class="p-2 rounded-lg border border-line bg-canvas/40">
                  <div class="text-[10px] text-ink-muted">可锁文件</div>
                  <div class="text-sm font-bold font-mono text-ink">{{ lockableSync.totalLockableFiles }}</div>
                </div>
                <div class="p-2 rounded-lg border border-line bg-canvas/40">
                  <div class="text-[10px] text-ink-muted">只读</div>
                  <div class="text-sm font-bold font-mono text-warning">{{ lockableSync.readOnlyCount }}</div>
                </div>
                <div class="p-2 rounded-lg border border-line bg-canvas/40">
                  <div class="text-[10px] text-ink-muted">可写</div>
                  <div class="text-sm font-bold font-mono text-success">{{ lockableSync.writableCount }}</div>
                </div>
              </div>
              <div class="flex items-center gap-2 pt-1">
                <input
                  v-model="bulkExtensionsInput"
                  type="text"
                  placeholder=".psd, .fbx, .uasset（批量扩展名）"
                  @keydown.enter="handleTrackExtensions"
                  class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
                <button
                  @click="handleTrackExtensions"
                  :disabled="!bulkExtensionsInput.trim() || isTrackingExtensions"
                  class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
                >
                  {{ isTrackingExtensions ? "跟踪中..." : "批量跟踪" }}
                </button>
              </div>
            </div>
          </div>

          <div class="space-y-3">
            <div class="pb-2 border-b border-line">
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Rocket class="w-4 h-4 text-accent" />
                <span>推送前 LFS 校验</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                推送前检查待推送提交中的 LFS 指针完整性，以及未走 LFS 的超大文件。
              </p>
            </div>
            <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
              <div class="flex flex-wrap items-center gap-2">
                <input
                  v-model="prePushRemote"
                  type="text"
                  placeholder="origin"
                  class="w-24 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
                />
                <input
                  v-model="prePushBranch"
                  type="text"
                  placeholder="当前分支"
                  class="w-32 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
                />
                <button
                  @click="handleVerifyPrePush"
                  :disabled="isVerifyingPrePush"
                  class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
                >
                  {{ isVerifyingPrePush ? "校验中..." : "开始校验" }}
                </button>
              </div>

              <template v-if="prePushResult">
                <div
                  class="px-3 py-2 rounded-lg border text-xs font-semibold"
                  :class="
                    prePushResult.isSafe
                      ? 'border-success/30 bg-success/10 text-success'
                      : 'border-danger/30 bg-danger/10 text-danger'
                  "
                >
                  {{ prePushResult.isSafe ? "可安全推送" : "存在推送风险" }} ·
                  {{ prePushResult.statusSummary }}
                </div>
                <div class="grid grid-cols-4 gap-2">
                  <div class="p-2 rounded-lg border border-line bg-canvas/40">
                    <div class="text-[10px] text-ink-muted">待推送提交</div>
                    <div class="text-sm font-bold font-mono text-ink">{{ prePushResult.unpushedCommits.length }}</div>
                  </div>
                  <div class="p-2 rounded-lg border border-line bg-canvas/40">
                    <div class="text-[10px] text-ink-muted">LFS 指针</div>
                    <div class="text-sm font-bold font-mono text-purple">{{ prePushResult.totalLfsPointers }}</div>
                  </div>
                  <div class="p-2 rounded-lg border border-line bg-canvas/40">
                    <div class="text-[10px] text-ink-muted">缺失对象</div>
                    <div class="text-sm font-bold font-mono text-danger">{{ prePushResult.missingLfsCount }}</div>
                  </div>
                  <div class="p-2 rounded-lg border border-line bg-canvas/40">
                    <div class="text-[10px] text-ink-muted">超大文件</div>
                    <div class="text-sm font-bold font-mono text-warning">{{ prePushResult.oversizedBlobs.length }}</div>
                  </div>
                </div>
                <div v-if="prePushResult.missingLfsFiles.length > 0" class="space-y-1">
                  <div class="text-[10px] text-ink-muted font-semibold">缺失的 LFS 对象</div>
                  <div class="max-h-24 overflow-y-auto space-y-1">
                    <div
                      v-for="file in prePushResult.missingLfsFiles"
                      :key="file"
                      class="px-2 py-1 rounded bg-danger/5 border border-danger/20 text-[11px] font-mono text-danger truncate"
                      :title="file"
                    >
                      {{ file }}
                    </div>
                  </div>
                </div>
                <div v-if="prePushResult.oversizedBlobs.length > 0" class="space-y-1">
                  <div class="text-[10px] text-ink-muted font-semibold">未走 LFS 的超大文件 (&gt;50MB)</div>
                  <div class="max-h-24 overflow-y-auto space-y-1">
                    <div
                      v-for="file in prePushResult.oversizedBlobs"
                      :key="file.path"
                      class="flex items-center gap-2 px-2 py-1 rounded bg-warning/5 border border-warning/20 text-[11px] font-mono"
                    >
                      <span class="text-warning truncate flex-1" :title="file.path">{{ file.path }}</span>
                      <span class="text-ink-muted shrink-0">{{ file.sizeHuman }}</span>
                    </div>
                  </div>
                </div>
              </template>
              <p v-else class="text-[11px] text-ink-muted">尚未执行校验。默认检查当前分支相对远端未推送的提交。</p>
            </div>
          </div>
        </div>
      </div>


    <!-- Modal: Lock File (LFS) -->
    <div
      v-if="showLockFileModal"
      ref="lockFileDialogRef"
      role="dialog"
      aria-modal="true"
      aria-label="锁定 Git LFS 资产文件"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4"
    >
      <div class="w-full max-w-md rounded-2xl bg-surface border border-line p-5 shadow-2xl flex flex-col gap-4 animate-in fade-in zoom-in-95 duration-150">
        <div class="flex items-center gap-2 pb-2 border-b border-line">
          <Lock class="w-4 h-4 text-warning" />
          <h3 class="text-sm font-bold text-ink">锁定 Git LFS 资产文件 (git lfs lock)</h3>
        </div>

        <div class="flex flex-col gap-1.5">
          <label class="text-xs text-ink-secondary font-medium">文件相对路径 (Relative File Path):</label>
          <input
            v-model="lockFilePathInput"
            type="text"
            placeholder="assets/characters/hero_albedo.png"
            class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-line">
          <button
            @click="showLockFileModal = false"
            class="px-3 py-1.5 rounded-lg hover:bg-raised text-xs text-ink-muted hover:text-ink transition cursor-pointer"
          >
            取消
          </button>
          <button
            @click="handleLockFile"
            :disabled="!lockFilePathInput.trim()"
            class="px-4 py-1.5 rounded-lg bg-warning-strong hover:opacity-90 text-warning-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-50"
          >
            申请独占锁定
          </button>
        </div>
      </div>
    </div>

</template>
