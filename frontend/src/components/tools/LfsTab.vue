<script setup lang="ts">
import { reportError } from "../../composables/useToast";
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
  MessageSquare,
  Wifi,
  Radio,
  RefreshCw,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import { toolsStatus } from "../../composables/toolsStatus";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import LockNotificationModal from "../lfs/LockNotificationModal.vue";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, LfsCacheStats, LfsTuningConfig, LfsFileRevision, LfsLockActivityResult, LfsBulkUnlockResult, LfsMigratePreview, LfsMigrateResult, LockableSyncResult, PrePushVerificationResult, FileStatus, LanLfsPeerStatus, LanLfsPeer, LanLfsTransferResult, LfsStorageAnalysis } from "../../types";

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

// LFS Cache Prune
const isPruningLfs = ref(false);
const lfsPruneResult = ref<LfsPruneResult | null>(null);

async function handleLfsPrune(dryRun: boolean) {
  if (!props.currentRepoPath || isPruningLfs.value) return;
  if (!dryRun) {
    const ok = await confirmAction({
      message: t("lfs.prune.confirm"),
      tone: "danger",
    });
    if (!ok) return;
  }
  isPruningLfs.value = true;
  try {
    const res = await gitService.lfsPrune(props.currentRepoPath, dryRun, false);
    lfsPruneResult.value = res;
    if (res) {
      if (dryRun) {
        notify(t("lfs.toast.prune.preview", { count: res.prunedFiles, size: res.reclaimedStr }));
      } else {
        notify(t("lfs.toast.prune.success", { count: res.prunedFiles, size: res.reclaimedStr }));
        await loadLfsData();
      }
    }
  } catch (err: any) {
    notify(t("lfs.toast.prune.failed", { error: err?.message || err }));
  } finally {
    isPruningLfs.value = false;
  }
}

// LFS Pointer Verification (git lfs fsck)
const isVerifyingLfs = ref(false);
const lfsFsckResult = ref<GitFsckResult | null>(null);

async function handleVerifyLfs() {
  if (!props.currentRepoPath || isVerifyingLfs.value) return;
  isVerifyingLfs.value = true;
  try {
    const res = await gitService.verifyLfsPointers(props.currentRepoPath);
    lfsFsckResult.value = res;
    if (res.isClean) {
      notify(t("lfs.toast.fsck.clean"));
    } else {
      notify(t("lfs.toast.fsck.issues", { count: res.rawErrors?.length || 0 }));
    }
  } catch (err: any) {
    notify(t("lfs.toast.fsck.failed", { error: err?.message || err }));
  } finally {
    isVerifyingLfs.value = false;
  }
}

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

// Local LFS Storage Profiler & Extension Breakdown
const storageAnalysis = ref<LfsStorageAnalysis | null>(null);
const isLoadingStorageAnalysis = ref(false);

async function loadStorageAnalysis() {
  if (!props.currentRepoPath) return;
  isLoadingStorageAnalysis.value = true;
  try {
    const res = await gitService.analyzeLfsStorage(props.currentRepoPath);
    storageAnalysis.value = res;
  } catch (err: any) {
    reportError("Failed to analyze LFS storage", err);
  } finally {
    isLoadingStorageAnalysis.value = false;
  }
}

// ---- 局域网 P2P LFS 加速 --------------------------------------
const lanLfsStatus = ref<LanLfsPeerStatus | null>(null);
const isScanningLanPeers = ref(false);
const isTogglingLanLfs = ref(false);

async function scanLanPeers() {
  if (!props.currentRepoPath || isScanningLanPeers.value) return;
  isScanningLanPeers.value = true;
  try {
    const res = await gitService.detectLanLfsPeers(props.currentRepoPath);
    lanLfsStatus.value = res;
    notify(t("lfs.toast.lan.scanned", { count: res.peers.length }));
  } catch (err: any) {
    notify(t("lfs.toast.lan.failed", { error: err?.message || err }));
  } finally {
    isScanningLanPeers.value = false;
  }
}

async function handleToggleLanLfs() {
  if (!props.currentRepoPath || isTogglingLanLfs.value) return;
  isTogglingLanLfs.value = true;
  try {
    const willEnable = !lanLfsStatus.value?.localLfsServing;
    const res = await gitService.configureLanLfsSharing(props.currentRepoPath, willEnable, 8080);
    if (res.enabled) {
      notify(t("lfs.toast.lan.started", { port: res.port }));
    } else {
      notify(t("lfs.toast.lan.stopped"));
    }
    await scanLanPeers();
  } catch (err: any) {
    notify(t("lfs.toast.lan.failed", { error: err?.message || err }));
  } finally {
    isTogglingLanLfs.value = false;
  }
}

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
    notify(t("lfs.toast.migrate.pattern.required"));
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
    notify(t("lfs.toast.migrate.preview.failed", { error: err?.message || err }));
  } finally {
    isPreviewingMigrate.value = false;
  }
}

async function handleExecuteMigrate() {
  if (!props.currentRepoPath || isExecutingMigrate.value) return;
  const patterns = parseMigratePatterns(migratePatterns.value);
  if (patterns.length === 0) {
    notify(t("lfs.toast.migrate.pattern.required.short"));
    return;
  }

  const totalFiles = (migratePreview.value?.rows || []).reduce((sum, r) => sum + r.files, 0);
  const first = await confirmAction({
    message: t("lfs.confirm.migrate", {
      patterns: patterns.join(", "),
      count: totalFiles || t("lfs.migrate.all"),
    }),
    tone: "danger",
  });
  if (!first) return;

  const second = await confirmAction({
    message: t(
      migrateCreateBundle.value ? "lfs.confirm.migrate.final.bundle" : "lfs.confirm.migrate.final.nobundle"
    ),
    tone: "danger",
    requireText: t("lfs.confirm.migrate.require"),
    requireTextHint: t("lfs.confirm.migrate.hint"),
  });
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
    notify(t("lfs.toast.migrate.failed", { error: err?.message || err }));
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

const showLockNotifyModal = ref(false);
const notifyFilePath = ref("");
const notifyFileOwner = ref("");

function openLockNotification(path: string, owner: string) {
  notifyFilePath.value = path;
  notifyFileOwner.value = owner;
  showLockNotifyModal.value = true;
}

// Blocked by a teammate's lock? Hand the user a ready-to-paste request instead
// of making them retype the path and owner into chat.
async function copyLockRequest(lock: { path: string; owner: string; durationStr: string }) {
  const text = t("lfs.lock.request.text", {
    path: lock.path,
    owner: lock.owner || t("lfs.unknown"),
    duration: lock.durationStr || t("lfs.unknown"),
  });
  try {
    await navigator.clipboard.writeText(text);
    notify(t("lfs.toast.lock.request.copied"));
  } catch {
    notify(t("lfs.toast.lock.request.copy.failed", { path: lock.path, owner: lock.owner }));
  }
}

async function handleUnlockStaleLocks() {  if (!props.currentRepoPath || zombieLocks.value.length === 0 || isUnlockingStale.value) return;
  const paths = zombieLocks.value.map((l) => l.path);
  const owners = Array.from(new Set(zombieLocks.value.map((l) => l.owner).filter(Boolean)));
  const confirmed = await confirmAction({
    message: t("lfs.confirm.zombie", {
      count: paths.length,
      owners: owners.join(", ") || t("lfs.unknown"),
    }),
    tone: "danger",
  });
  if (!confirmed) return;

  isUnlockingStale.value = true;
  try {
    const res = await gitService.unlockStaleLfsLocks(props.currentRepoPath, paths, true);
    staleUnlockResult.value = res;
    notify(res.message);
    await loadLfsData();
    emit("refresh");
  } catch (err: any) {
    notify(t("lfs.toast.zombie.unlock.failed", { error: err?.message || err }));
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
    preset === "studio_high_speed" ? t("lfs.tuning.preset.studio") : preset === "standard" ? t("lfs.tuning.preset.standard") : t("lfs.tuning.preset.reset");
  if (!(await confirmAction(t("lfs.confirm.tuning", { label })))) return;
  isApplyingTuning.value = preset;
  try {
    const res = await gitService.applyLfsTuningOptimization(props.currentRepoPath, preset);
    notify(res?.message || t("lfs.toast.tuning.applied", { label }));
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.tuning.failed", { error: err?.message || err }));
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
    notify(t("lfs.toast.history.loaded", { count: lfsHistory.value.length }));
  } catch (err: any) {
    notify(t("lfs.toast.history.failed", { error: err?.message || err }));
  } finally {
    isInspectingHistory.value = false;
  }
}

async function handleSelectivePull() {
  if (!props.currentRepoPath || isLfsSyncing.value) return;
  isLfsSyncing.value = "pull";
  try {
    await gitService.executeLfsSelectivePull(props.currentRepoPath);
    notify(t("lfs.toast.selective.pulled"));
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.selective.failed", { error: err?.message || err }));
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
        ? t("lfs.toast.lockable.synced.detail", { total: res.totalLockableFiles, readOnly: res.readOnlyCount, writable: res.writableCount })
        : t("lfs.toast.lockable.synced")
    );
  } catch (err: any) {
    notify(t("lfs.toast.lockable.sync.failed", { error: err?.message || err }));
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
        ? t("lfs.confirm.protection.enable")
        : t("lfs.confirm.protection.disable")
    ))
  )
    return;
  isTogglingLockable.value = true;
  try {
    const res = await gitService.toggleLockableReadOnlyProtection(props.currentRepoPath, enable);
    lockableSync.value = res;
    notify(enable ? t("lfs.toast.protection.enabled") : t("lfs.toast.protection.disabled"));
  } catch (err: any) {
    notify(t("lfs.toast.protection.failed", { error: err?.message || err }));
  } finally {
    isTogglingLockable.value = false;
  }
}

async function handleTrackExtensions() {
  const exts = parseLfsPatterns(bulkExtensionsInput.value);
  if (!props.currentRepoPath || exts.length === 0 || isTrackingExtensions.value) return;
  if (!(await confirmAction(t("lfs.confirm.track.extensions", { exts: exts.join(", ") }))))
    return;
  isTrackingExtensions.value = true;
  try {
    await gitService.trackExtensionsAsLfs(props.currentRepoPath, exts);
    notify(t("lfs.toast.track.extensions.done", { exts: exts.join(", ") }));
    bulkExtensionsInput.value = "";
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.track.extensions.failed", { error: err?.message || err }));
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
    notify(t("lfs.toast.lock.query.failed", { error: err?.message || err }));
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
        ? t("lfs.toast.lock.conflicts.found", { count: lockConflicts.value.length })
        : t("lfs.toast.lock.conflicts.none")
    );
  } catch (err: any) {
    notify(t("lfs.toast.lock.conflicts.failed", { error: err?.message || err }));
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
    notify(t("lfs.toast.report.exported", { format: lockReportFormat.value === "csv" ? "CSV" : "Markdown" }));
  } catch (err: any) {
    notify(t("lfs.toast.report.failed", { error: err?.message || err }));
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
      notify(prePushResult.value.statusSummary || t("lfs.toast.prepush.done"));
    }
  } catch (err: any) {
    notify(t("lfs.toast.prepush.failed", { error: err?.message || err }));
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
    notify(t("lfs.toast.attributes.read.failed", { error: err?.message || err }));
  } finally {
    isLoadingAttributes.value = false;
  }
}

async function handleSaveAttributes() {
  if (!props.currentRepoPath || isSavingAttributes.value) return;
  isSavingAttributes.value = true;
  try {
    await gitService.saveGitAttributes(props.currentRepoPath, attributesContent.value);
    notify(t("lfs.toast.attributes.saved"));
    showAttributesEditor.value = false;
    await loadLfsData();
    emit("refresh");
  } catch (err: any) {
    notify(t("lfs.toast.attributes.save.failed", { error: err?.message || err }));
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
    const loadErrors: string[] = [];
    const [locks, patterns, cache, filters, tuning, activity, diskUsage] = await Promise.all([
      locksPromise,
      gitService.getLfsTrackedPatterns(props.currentRepoPath).catch((err: any) => {
        loadErrors.push(err?.message || String(err));
        return [];
      }),
      gitService.getLfsCacheStats(props.currentRepoPath).catch((err: any) => {
        loadErrors.push(err?.message || String(err));
        return null;
      }),
      gitService.getLfsFilterConfig(props.currentRepoPath).catch((err: any) => {
        loadErrors.push(err?.message || String(err));
        return null;
      }),
      gitService.getLfsTuningConfig(props.currentRepoPath).catch((err: any) => {
        loadErrors.push(err?.message || String(err));
        return null;
      }),
      gitService.getLfsLockActivity(props.currentRepoPath).catch((err: any) => {
        loadErrors.push(err?.message || String(err));
        return null;
      }),
      gitService.getLfsDiskUsage(props.currentRepoPath).catch((err: any) => {
        loadErrors.push(err?.message || String(err));
        return "0 B";
      }),
    ]);
    lfsLocks.value = locks || [];
    lfsPatterns.value = patterns || [];
    lfsCacheStats.value = cache;
    lfsTuning.value = tuning;
    lockActivity.value = activity;
    lfsDiskUsage.value = diskUsage || "0 B";
    fetchIncludeInput.value = (filters?.fetchInclude || []).join(", ");
    fetchExcludeInput.value = (filters?.fetchExclude || []).join(", ");
    void loadStorageAnalysis();
    gitService.detectLanLfsPeers(props.currentRepoPath).then(res => { lanLfsStatus.value = res; }).catch(() => {});
    if (loadErrors.length > 0) {
      notify(t("lfs.toast.load.failed", { error: loadErrors.join("; ") }));
    }
  } catch (err) {
    reportError("Failed to load LFS data", err);
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
    notify(t("lfs.toast.filters.saved"));
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.filters.save.failed", { error: err?.message || err }));
  } finally {
    isSavingLfsFilters.value = false;
  }
}

async function handleLfsPull() {
  if (!props.currentRepoPath || isLfsSyncing.value) return;
  isLfsSyncing.value = "pull";
  try {
    await gitService.lfsPull(props.currentRepoPath);
    notify(t("lfs.toast.pulled"));
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.pull.failed", { error: err?.message || err }));
  } finally {
    isLfsSyncing.value = "";
  }
}

async function handleLfsPush() {
  if (!props.currentRepoPath || isLfsSyncing.value) return;
  if (!(await confirmAction(t("lfs.confirm.push")))) return;
  isLfsSyncing.value = "push";
  try {
    await gitService.lfsPush(props.currentRepoPath);
    notify(t("lfs.toast.pushed"));
  } catch (err: any) {
    notify(t("lfs.toast.push.failed", { error: err?.message || err }));
  } finally {
    isLfsSyncing.value = "";
  }
}

async function handleTrackPattern() {
  if (!props.currentRepoPath || !newPatternInput.value.trim()) return;
  try {
    await gitService.trackLfsPattern(props.currentRepoPath, newPatternInput.value.trim());
    notify(t("lfs.toast.pattern.added", { pattern: newPatternInput.value.trim() }));
    newPatternInput.value = "";
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.pattern.failed", { error: err?.message || err }));
  }
}

async function handleUntrackPattern(pattern: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction(t("lfs.confirm.untrack", { pattern })))) return;
  try {
    await gitService.untrackLfsPattern(props.currentRepoPath, pattern);
    notify(t("lfs.toast.untracked", { pattern }));
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.untrack.failed", { error: err?.message || err }));
  }
}

const isLockingAllLfs = ref(false);
const showLockFileModal = ref(false);
const lockFilePathInput = ref("");

async function handleLockAllModifiedLfs() {
  if (!props.currentRepoPath) return;
  isLockingAllLfs.value = true;
  try {
    const res = await gitService.lockAllModifiedLfs(props.currentRepoPath);
    if (res) {
      notify(res.message || t("lfs.toast.bulk.locked", { count: res.totalCount }));
      await loadLfsData();
    }
  } catch (err: any) {
    notify(t("lfs.toast.bulk.lock.failed", { error: err?.message || err }));
  } finally {
    isLockingAllLfs.value = false;
  }
}

async function handleLockFile() {
  if (!props.currentRepoPath || !lockFilePathInput.value.trim()) return;
  try {
    await gitService.lockFile(props.currentRepoPath, lockFilePathInput.value.trim());
    notify(t("lfs.toast.locked", { file: lockFilePathInput.value }));
    lockFilePathInput.value = "";
    showLockFileModal.value = false;
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.lock.failed", { error: err?.message || err }));
  }
}

async function handleUnlockFile(path: string, force = false) {
  if (!props.currentRepoPath || !path) return;
  if (!(await confirmAction({ message: t(force ? "lfs.confirm.unlock.force" : "lfs.confirm.unlock", { path }), tone: "danger" }))) return;
  try {
    await gitService.unlockFile(props.currentRepoPath, path, force);
    notify(t("lfs.toast.unlocked", { path }));
    await loadLfsData();
  } catch (err: any) {
    notify(t("lfs.toast.unlock.failed", { error: err?.message || err }));
  }
}

function copyToClipboard(text: string, label: string) {
  navigator.clipboard.writeText(text);
  notify(t("lfs.toast.copied", { label }));
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
          <span>{{ t("lfs.locks.error", { error: lfsLocksError }) }}</span>
        </div>

        <!-- Section A: Tracked Patterns -->
        <div class="space-y-3" data-tool-section="lfs-patterns">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <HardDrive class="w-4 h-4 text-accent" />
                <span>{{ t("lfs.rules.title") }}</span>
              </h3>
            </div>
            <div class="flex items-center gap-2">
              <button
                @click="toggleAttributesEditor"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer"
                :title="t('lfs.rules.edit.title')"
              >
                {{ showAttributesEditor ? t("lfs.rules.collapse") : t("lfs.rules.edit") }}
              </button>
              <button
                @click="handleLfsPull"
                :disabled="!!isLfsSyncing"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                :title="t('lfs.pull.title')"
              >
                {{ isLfsSyncing === "pull" ? t("lfs.pull.running") : t("lfs.pull") }}
              </button>
              <button
                @click="handleSelectivePull"
                :disabled="!!isLfsSyncing || (!fetchIncludeInput.trim() && !fetchExcludeInput.trim())"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                :title="
                  fetchIncludeInput.trim() || fetchExcludeInput.trim()
                    ? t('lfs.selective.title')
                    : t('lfs.selective.disabled')
                "
              >
                {{ t("lfs.selective") }}
              </button>
              <button
                @click="handleLfsPush"
                :disabled="!!isLfsSyncing"
                class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                :title="t('lfs.push.title')"
              >
                {{ isLfsSyncing === "push" ? t("lfs.push.running") : t("lfs.push") }}
              </button>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <input
              v-model="newPatternInput"
              type="text"
              :placeholder="t('lfs.pattern.placeholder')"
              @keydown.enter="handleTrackPattern"
              class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <button
              @click="handleTrackPattern"
              :disabled="!newPatternInput.trim()"
              class="px-4 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              {{ t("lfs.pattern.add") }}
            </button>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 pt-2">
            <div
              v-for="pat in lfsPatterns"
              :key="pat.pattern"
              class="p-2.5 rounded-lg bg-surface/60 border border-line flex items-center justify-between group"
            >
              <span class="font-mono text-xs text-ink font-bold truncate">{{ pat.pattern }}</span>
              <button :aria-label="t('lfs.pattern.untrack.aria')"
                @click="handleUntrackPattern(pat.pattern)"
                class="opacity-0 group-hover:opacity-100 p-1 hover:bg-danger/20 text-ink-muted hover:text-danger rounded transition"
                :title="t('lfs.pattern.untrack.aria')"
              >
                <Trash2 class="w-3 h-3" />
              </button>
            </div>

            <div v-if="lfsPatterns.length === 0" class="col-span-4 py-6 text-center text-xs text-ink-muted border border-dashed border-line rounded-xl">
              {{ t("lfs.pattern.empty") }}
            </div>
          </div>

          <div v-if="showAttributesEditor" class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-ink">{{ t("lfs.attributes.title") }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("lfs.attributes.desc") }}</span>
            </div>
            <div v-if="isLoadingAttributes" class="py-6 text-center text-xs text-ink-muted">{{ t("lfs.attributes.loading") }}</div>
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
                {{ t("common.cancel") }}
              </button>
              <button
                @click="handleSaveAttributes"
                :disabled="isSavingAttributes || isLoadingAttributes"
                class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-50"
              >
                {{ isSavingAttributes ? t("lfs.attributes.saving") : t("lfs.attributes.save") }}
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
                <span>{{ t("lfs.locks.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("lfs.locks.desc") }}
              </p>
            </div>

            <div class="flex items-center gap-2">
              <button
                @click="handleLockAllModifiedLfs"
                :disabled="isLockingAllLfs"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-50"
                :title="t('lfs.locks.lock.modified.title')"
              >
                <Lock class="w-3.5 h-3.5" :class="{ 'animate-spin': isLockingAllLfs }" />
                <span>{{ t("lfs.locks.lock.modified") }}</span>
              </button>
              <button
                @click="showLockFileModal = true"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer shadow-xs"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>{{ t("lfs.locks.lock") }}</span>
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
                    :title="t('lfs.locks.unlock')"
                  >
                    {{ t("lfs.locks.unlock.short") }}
                  </button>
                  <button :aria-label="t('lfs.locks.force.unlock.aria')"
                    @click="handleUnlockFile(lock.path, true)"
                    class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                    :title="t('lfs.locks.force.unlock.aria')"
                  >
                    <Trash2 class="w-3 h-3" />
                  </button>
                </div>
              </div>
            </div>

            <div v-else class="py-8 text-center text-xs text-ink-muted flex flex-col items-center gap-2">
              <Check class="w-6 h-6 text-success" />
              <span>{{ t("lfs.locks.empty") }}</span>
            </div>
          </div>
        </div>

        <!-- Section C: Local cache + selective download filters -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-cache">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Database class="w-4 h-4 text-purple" />
              <span>{{ t("lfs.cache.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("lfs.cache.desc") }}
            </p>
          </div>

          <div class="grid grid-cols-3 gap-3">
            <div class="p-3 rounded-xl border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.cache.size") }}</div>
              <div class="text-lg font-bold font-mono text-ink">{{ lfsCacheStats?.totalSizeStr || "0 B" }}</div>
            </div>
            <div class="p-3 rounded-xl border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.cache.objects") }}</div>
              <div class="text-lg font-bold font-mono text-purple">{{ lfsCacheStats?.fileCount ?? 0 }}</div>
            </div>
            <div class="p-3 rounded-xl border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.cache.disk") }}</div>
              <div class="text-lg font-bold font-mono text-ink">{{ lfsDiskUsage }}</div>
            </div>
          </div>
          <div class="px-3 py-2 rounded-lg border border-line bg-canvas/50 font-mono text-[10px] text-ink-muted truncate" :title="lfsCacheStats?.cachePath">
            {{ lfsCacheStats?.cachePath || ".git/lfs/objects" }}
          </div>

          <!-- LFS Prune & Pointer Integrity Verification Action Bar -->
          <div class="p-3 rounded-xl border border-line bg-surface/40 flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="handleLfsPrune(true)"
                :disabled="isPruningLfs"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-line bg-surface hover:bg-raised text-ink text-xs font-medium transition cursor-pointer disabled:opacity-50"
                :title="t('lfs.prune.dryrun.title')"
              >
                <Activity class="w-3.5 h-3.5 text-accent" />
                <span>{{ t("lfs.prune.dryrun") }}</span>
              </button>
              <button
                type="button"
                @click="handleLfsPrune(false)"
                :disabled="isPruningLfs"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-line bg-surface hover:bg-raised text-danger text-xs font-medium transition cursor-pointer disabled:opacity-50"
                :title="t('lfs.prune.exec.title')"
              >
                <Trash2 class="w-3.5 h-3.5" />
                <span>{{ isPruningLfs ? t("lfs.prune.running") : t("lfs.prune.exec") }}</span>
              </button>
              <button
                type="button"
                @click="handleVerifyLfs"
                :disabled="isVerifyingLfs"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-line bg-surface hover:bg-raised text-ink text-xs font-medium transition cursor-pointer disabled:opacity-50"
                :title="t('lfs.fsck.title')"
              >
                <ShieldCheck class="w-3.5 h-3.5 text-purple" />
                <span>{{ isVerifyingLfs ? t("lfs.fsck.running") : t("lfs.fsck.button") }}</span>
              </button>
            </div>

            <!-- FSCK Status Badge -->
            <div v-if="lfsFsckResult" class="text-xs flex items-center gap-1.5">
              <span
                class="px-2 py-0.5 rounded text-[10px] font-bold font-mono"
                :class="lfsFsckResult.isClean ? 'bg-success/15 text-success border border-success/30' : 'bg-danger/15 text-danger border border-danger/30'"
              >
                {{ lfsFsckResult.isClean ? t("lfs.fsck.status.clean") : t("lfs.fsck.status.issues", { count: lfsFsckResult.rawErrors?.length || 0 }) }}
              </span>
            </div>
          </div>

          <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2.5">
            <div class="grid grid-cols-2 gap-3">
              <label class="flex flex-col gap-1">
                <span class="text-[10px] text-ink-muted font-semibold">{{ t("lfs.cache.include") }}</span>
                <input
                  v-model="fetchIncludeInput"
                  type="text"
                  placeholder="Assets/Characters/**, Assets/UI/**"
                  class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-[10px] text-ink-muted font-semibold">{{ t("lfs.cache.exclude") }}</span>
                <input
                  v-model="fetchExcludeInput"
                  type="text"
                  placeholder="Assets/Audio/**, Assets/Videos/**"
                  class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
              </label>
            </div>
            <div class="flex items-center justify-between gap-3">
              <p class="text-[10px] text-ink-muted">{{ t("lfs.cache.hint") }}</p>
              <button
                @click="handleSaveLfsFilters"
                :disabled="isSavingLfsFilters"
                class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-50 shrink-0"
              >
                {{ isSavingLfsFilters ? t("lfs.cache.saving") : t("lfs.cache.save") }}
              </button>
            </div>
          </div>

          <!-- LFS Storage Profiler & Extension Weight Card -->
          <div class="rounded-xl border border-line bg-surface/30 p-3.5 space-y-3" data-testid="lfs-storage-profiler">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Database class="w-4 h-4 text-purple" />
                <span class="text-xs font-bold text-ink">{{ t("lfs.storage.profiler.title") }}</span>
                <span class="text-[10px] text-ink-muted">({{ t("lfs.storage.profiler.desc") }})</span>
              </div>
              <button
                type="button"
                @click="loadStorageAnalysis"
                :disabled="isLoadingStorageAnalysis"
                class="flex items-center gap-1 text-[11px] text-accent hover:underline cursor-pointer disabled:opacity-40"
              >
                <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingStorageAnalysis }" />
                <span>{{ t("common.refresh") }}</span>
              </button>
            </div>

            <div v-if="storageAnalysis" class="space-y-3">
              <!-- Advice banner -->
              <div
                class="px-3 py-2 rounded-lg border text-xs flex items-center justify-between"
                :class="storageAnalysis.unreferencedCount > 0 ? 'border-amber/30 bg-amber/10 text-amber' : 'border-success/30 bg-success/10 text-success'"
              >
                <span>{{ storageAnalysis.pruneAdvice }}</span>
                <span class="font-mono font-bold">{{ storageAnalysis.formattedTotal }} ({{ storageAnalysis.objectCount }} {{ t('lfs.storage.profiler.objects') }})</span>
              </div>

              <!-- Extension weight breakdown -->
              <div v-if="storageAnalysis.typeBreakdown && storageAnalysis.typeBreakdown.length > 0" class="space-y-2">
                <div class="text-[10px] font-semibold text-ink-muted uppercase tracking-wider">
                  {{ t("lfs.storage.weight.title") }}
                </div>
                <div class="space-y-1.5 max-h-48 overflow-y-auto">
                  <div
                    v-for="item in storageAnalysis.typeBreakdown"
                    :key="item.extension"
                    class="p-2 rounded-lg bg-canvas border border-line flex items-center justify-between gap-3 text-xs"
                  >
                    <div class="flex items-center gap-2 min-w-[100px]">
                      <span class="font-mono font-bold text-accent">{{ item.extension }}</span>
                      <span class="text-[10px] text-ink-muted">({{ item.fileCount }} {{ t('lfs.storage.profiler.files') }})</span>
                    </div>
                    <div class="flex-1 max-w-[200px] h-2 bg-raised rounded-full overflow-hidden">
                      <div
                        class="h-full bg-accent rounded-full transition-all duration-300"
                        :style="{ width: `${Math.min(100, Math.max(2, item.percentage))}%` }"
                      />
                    </div>
                    <div class="flex items-center gap-2 font-mono text-[11px] text-ink font-semibold">
                      <span>{{ item.formattedSize }}</span>
                      <span class="text-ink-muted w-14 text-right">({{ item.percentage.toFixed(1) }}%)</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="text-xs text-ink-muted py-2">
              {{ t("lfs.storage.profiler.empty") }}
            </div>
          </div>

          <!-- LAN P2P Cache Acceleration Card -->
          <div class="rounded-xl border border-line bg-surface/30 p-3.5 space-y-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Wifi class="w-4 h-4 text-accent" />
                <span class="text-xs font-bold text-ink">{{ t("lfs.lan.title") }}</span>
                <span
                  class="px-2 py-0.5 rounded-full text-[10px] font-semibold"
                  :class="lanLfsStatus?.localLfsServing ? 'bg-success/15 text-success' : 'bg-surface text-ink-muted border border-line'"
                >
                  {{ lanLfsStatus?.localLfsServing ? t("lfs.lan.serving") : t("lfs.lan.idle") }}
                </span>
              </div>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  @click="scanLanPeers"
                  :disabled="isScanningLanPeers"
                  class="px-2.5 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-xs font-medium text-ink transition cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
                  :title="t('lfs.lan.scan.title')"
                >
                  <RefreshCw class="w-3 h-3" :class="isScanningLanPeers ? 'animate-spin' : ''" />
                  <span>{{ t("lfs.lan.scan") }}</span>
                </button>
                <button
                  type="button"
                  @click="handleToggleLanLfs"
                  :disabled="isTogglingLanLfs"
                  class="px-2.5 py-1 rounded-lg text-xs font-semibold cursor-pointer transition shadow-xs"
                  :class="
                    lanLfsStatus?.localLfsServing
                      ? 'bg-danger/15 hover:bg-danger/25 text-danger border border-danger/30'
                      : 'bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast'
                  "
                >
                  {{ lanLfsStatus?.localLfsServing ? t("lfs.lan.stop") : t("lfs.lan.start") }}
                </button>
              </div>
            </div>

            <p class="text-[11px] text-ink-muted">
              {{ t("lfs.lan.desc") }}
            </p>

            <!-- Discovered Peers List -->
            <div v-if="lanLfsStatus && lanLfsStatus.peers && lanLfsStatus.peers.length > 0" class="space-y-1.5 pt-1">
              <div class="text-[10px] font-medium text-ink-muted uppercase tracking-wider">
                {{ t("lfs.lan.peers.discovered", { count: lanLfsStatus.peers.length }) }}
              </div>
              <div
                v-for="peer in lanLfsStatus.peers"
                :key="peer.url"
                class="flex items-center justify-between p-2 rounded-lg bg-canvas border border-line text-xs font-mono"
              >
                <div class="flex items-center gap-2">
                  <Radio class="w-3.5 h-3.5 text-success animate-pulse" />
                  <span class="text-ink font-semibold">{{ peer.repoName || "Peer Repo" }}</span>
                  <span class="text-ink-muted text-[10px]">({{ peer.ip }}:{{ peer.port }})</span>
                </div>
                <div class="flex items-center gap-3 text-[11px]">
                  <span class="text-purple">{{ t("lfs.lan.peer.blobs", { count: peer.cachedBlobs }) }}</span>
                  <span class="text-ink-muted">{{ peer.latencyMs }}ms</span>
                </div>
              </div>
            </div>
            <div v-else class="text-[11px] text-ink-muted italic">
              {{ t("lfs.lan.peers.empty") }}
            </div>
          </div>
        </div>

        <!-- Section D: LFS transfer tuning -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-tuning">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Activity class="w-4 h-4 text-warning" />
              <span>{{ t("lfs.tuning.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("lfs.tuning.desc") }}
            </p>
          </div>

          <div v-if="lfsTuning" class="grid grid-cols-2 lg:grid-cols-4 gap-2">
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.tuning.concurrency") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lfsTuning.concurrentTransfers }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.tuning.buffer") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lfsTuning.postBufferMb }} MB</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.tuning.timeout") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lfsTuning.dialTimeoutSec }} s</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.tuning.compression") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lfsTuning.compressionLevel }}</div>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              @click="handleApplyTuning('studio_high_speed')"
              :disabled="!!isApplyingTuning"
              class="px-2.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              {{ isApplyingTuning === "studio_high_speed" ? t("lfs.tuning.applying") : t("lfs.tuning.studio") }}
            </button>
            <button
              @click="handleApplyTuning('standard')"
              :disabled="!!isApplyingTuning"
              class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
            >
              {{ isApplyingTuning === "standard" ? t("lfs.tuning.applying") : t("lfs.tuning.standard") }}
            </button>
            <button
              @click="handleApplyTuning('reset_default')"
              :disabled="!!isApplyingTuning"
              class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink-muted transition cursor-pointer disabled:opacity-40"
            >
              {{ isApplyingTuning === "reset_default" ? t("lfs.tuning.applying") : t("lfs.tuning.reset") }}
            </button>
            <span v-if="lfsTuning" class="text-[10px] text-ink-muted truncate">{{ lfsTuning.activeSummary }}</span>
          </div>
        </div>

        <!-- Section E: LFS file revision history -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-revisions">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Search class="w-4 h-4 text-cyan" />
              <span>{{ t("lfs.history.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">{{ t("lfs.history.desc") }}</p>
          </div>
          <div class="flex items-center gap-2">
            <input
              v-model="lfsHistoryPath"
              type="text"
              :placeholder="t('lfs.history.placeholder')"
              @keydown.enter="handleInspectLfsHistory"
              class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <button
              @click="handleInspectLfsHistory"
              :disabled="!lfsHistoryPath.trim() || isInspectingHistory"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isInspectingHistory ? t("lfs.history.loading") : t("lfs.history.view") }}
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
            {{ lfsHistoryPath ? t('lfs.history.empty') : t('lfs.history.prompt') }}
          </div>
        </div>

        <!-- Section F: LFS lock activity -->
        <!-- History migration wizard -->
        <div class="space-y-3 pt-4" data-tool-section="lfs-migrate">
          <div class="pb-2 border-b border-line flex items-start justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <History class="w-4 h-4 text-purple" />
                <span>{{ t("lfs.migrate.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("lfs.migrate.desc") }}
                <span class="text-warning font-semibold">{{ t("lfs.migrate.warning") }}</span>
              </p>
            </div>
            <button
              @click="migrateExpanded = !migrateExpanded"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer shrink-0"
            >
              {{ migrateExpanded ? t("lfs.migrate.collapse") : t("lfs.migrate.expand") }}
            </button>
          </div>

          <template v-if="migrateExpanded">
            <div class="rounded-xl border border-danger/25 bg-danger/5 p-3 space-y-1.5">
              <div class="text-[11px] font-bold text-danger flex items-center gap-1.5">
                <AlertTriangle class="w-3.5 h-3.5" />
                {{ t("lfs.migrate.checklist.title") }}
              </div>
              <ul class="text-[10px] text-ink-muted leading-relaxed list-disc pl-4 space-y-0.5">
                <li>{{ t("lfs.migrate.check.1.before") }} <span class="font-mono">push --force --all --tags</span>{{ t("lfs.migrate.check.1.after") }}</li>
                <li>{{ t("lfs.migrate.check.2") }}</li>
                <li>{{ t("lfs.migrate.check.3") }}</li>
              </ul>
            </div>

            <div class="space-y-2">
              <label class="text-[11px] font-semibold text-ink-secondary">{{ t("lfs.migrate.patterns") }}</label>
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
                <span class="text-ink">{{ t("lfs.migrate.everything") }}</span>
              </label>
              <div v-if="!migrateEverything" class="flex items-center gap-1.5">
                <span class="text-ink-muted">{{ t("lfs.migrate.ref") }}</span>
                <input
                  v-model="migrateRevRange"
                  spellcheck="false"
                  placeholder="feature"
                  :title="t('lfs.migrate.ref.hint')"
                  class="w-40 px-2 py-1 rounded-md bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
                />
              </div>
              <label class="flex items-center gap-1.5 cursor-pointer">
                <input v-model="migrateCreateBundle" type="checkbox" class="accent-[var(--ug-accent-strong)]" />
                <span class="text-ink">{{ t("lfs.migrate.bundle") }}</span>
              </label>
            </div>

            <div class="flex items-center gap-2">
              <button
                @click="handlePreviewMigrate"
                :disabled="isPreviewingMigrate"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              >
                <Search class="w-3.5 h-3.5" :class="{ 'animate-pulse': isPreviewingMigrate }" />
                <span>{{ isPreviewingMigrate ? t("lfs.migrate.previewing") : t("lfs.migrate.preview") }}</span>
              </button>
              <button
                @click="handleExecuteMigrate"
                :disabled="isExecutingMigrate || !migratePreview || migratePreview.rows.length === 0"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-danger-strong hover:bg-danger-strong/80 text-white text-xs font-semibold transition cursor-pointer disabled:opacity-30"
                :title="!migratePreview ? t('lfs.migrate.need.preview') : t('lfs.migrate.execute.title')"
              >
                <Rocket class="w-3.5 h-3.5" />
                <span>{{ isExecutingMigrate ? t("lfs.migrate.running") : t("lfs.migrate.execute") }}</span>
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
                  <span class="text-ink-muted shrink-0">{{ t("lfs.migrate.files", { count: row.files }) }}</span>
                  <span class="text-warning shrink-0">{{ row.size }}</span>
                </div>
              </div>
              <p class="text-[10px] text-ink-muted font-mono truncate" :title="migratePreview.command">
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
                {{ t("lfs.migrate.rollback", { path: migrateResult.bundlePath }) }}
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
              <span>{{ t("lfs.activity.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">{{ t("lfs.activity.desc") }}</p>
          </div>
          <div v-if="lockActivity" class="grid grid-cols-2 lg:grid-cols-5 gap-2">
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.activity.total") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ lockActivity.totalLocks }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.activity.mine") }}</div>
              <div class="text-sm font-bold font-mono text-accent">{{ lockActivity.myLockCount }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.activity.team") }}</div>
              <div class="text-sm font-bold font-mono text-purple">{{ lockActivity.teamLockCount }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.activity.zombie") }}</div>
              <div class="text-sm font-bold font-mono text-warning">{{ lockActivity.zombieCount }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-surface/30">
              <div class="text-[10px] text-ink-muted">{{ t("lfs.activity.conflict") }}</div>
              <div class="text-sm font-bold font-mono text-danger">{{ lockActivity.collisionCount }}</div>
            </div>
          </div>
          <div v-if="zombieLocks.length > 0" class="flex items-center justify-between gap-3 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2">
            <div class="text-[11px] text-ink leading-relaxed min-w-0">
              <span class="font-semibold text-warning">{{ t("lfs.activity.zombie.count", { count: zombieLocks.length }) }}</span>
              {{ t("lfs.activity.zombie.desc") }}
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <button
                v-if="zombieLocks[0]"
                @click="openLockNotification(zombieLocks[0].path, zombieLocks[0].owner)"
                class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
                :title="t('lfs.activity.zombie.notify')"
              >
                <MessageSquare class="w-3.5 h-3.5 text-accent" />
                <span>{{ t("lfs.activity.zombie.notify") }}</span>
              </button>
              <button
                @click="handleUnlockStaleLocks"
                :disabled="isUnlockingStale"
                class="px-3 py-1.5 rounded-lg bg-warning-strong hover:bg-warning-strong-hover text-warning-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
              >
                {{ isUnlockingStale ? t("lfs.activity.zombie.unlocking") : t("lfs.activity.zombie.unlock") }}
              </button>
            </div>
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
              <span v-if="item.detail" class="text-ink-muted shrink-0">{{ item.detail }}</span>
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
                {{ lock.hasConflict ? t("lfs.activity.conflict") : lock.isZombie ? t("lfs.activity.zombie") : lock.category }}
              </span>
              <span class="font-mono text-ink truncate flex-1" :title="lock.path">{{ lock.path }}</span>
              <span class="text-ink-muted shrink-0">{{ lock.owner }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ lock.durationStr }}</span>
              <button
                v-if="!lock.isMe || lock.isZombie"
                @click="openLockNotification(lock.path, lock.owner)"
                class="shrink-0 p-0.5 rounded text-ink-muted hover:text-accent hover:bg-accent/10 transition cursor-pointer"
                :title="t('lfs.activity.zombie.notify')"
              >
                <MessageSquare class="w-3 h-3" />
              </button>
              <button :aria-label="t('lfs.lock.request.aria')"
                v-if="!lock.isMe || lock.isZombie"
                @click="copyLockRequest(lock)"
                class="shrink-0 p-0.5 rounded text-ink-muted hover:text-accent hover:bg-accent/10 transition cursor-pointer"
                :title="t('lfs.lock.request.aria')"
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
                <span>{{ isCheckingLockConflicts ? t("lfs.conflicts.checking") : t("lfs.conflicts.check") }}</span>
              </button>
              <div class="flex items-center gap-2 flex-1 min-w-[260px]">
                <input
                  v-model="lockFileStatusPath"
                  type="text"
                  :placeholder="t('lfs.lock.query.placeholder')"
                  @keydown.enter="handleCheckFileLock"
                  class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
                <button
                  @click="handleCheckFileLock"
                  :disabled="!lockFileStatusPath.trim() || isCheckingFileLock"
                  class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
                >
                  {{ isCheckingFileLock ? t("lfs.lock.query.running") : t("lfs.lock.query.run") }}
                </button>
              </div>
              <div class="flex items-center gap-1.5">
                <select
                  v-model="lockReportFormat"
                  :aria-label="t('lfs.report.format')"
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
                  {{ isExportingLockReport ? t("lfs.report.generating") : t("lfs.report.export") }}
                </button>
              </div>
            </div>

            <p
              v-if="lockStatusChecked"
              class="text-[11px] font-mono"
              :class="lockFileOwner ? 'text-warning' : 'text-success'"
            >
              {{ lockFileOwner ? t("lfs.lock.query.owned", { path: lockFileStatusPath, owner: lockFileOwner }) : t("lfs.lock.query.free", { path: lockFileStatusPath }) }}
            </p>

            <div v-if="lockConflicts.length > 0" class="space-y-1">
              <div
                v-for="conflict in lockConflicts"
                :key="conflict.path"
                class="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border border-danger/30 bg-danger/5 text-[11px]"
              >
                <span class="px-1 py-0.2 rounded bg-danger/15 text-danger text-[9px] font-bold shrink-0">{{ t("lfs.activity.conflict") }}</span>
                <span class="font-mono text-ink truncate flex-1" :title="conflict.path">{{ conflict.path }}</span>
                <span class="text-ink-muted shrink-0">{{ t("lfs.activity.owner", { owner: conflict.lockOwner || t("lfs.unknown") }) }}</span>
              </div>
            </div>

            <div v-if="showLockReport" class="space-y-2 pt-1">
              <div class="flex items-center justify-between">
                <span class="text-[11px] font-bold text-ink">{{ t("lfs.report.preview") }}</span>
                <div class="flex items-center gap-1.5">
                  <button
                    @click="copyToClipboard(lockReportContent, t('lfs.report.name'))"
                    class="flex items-center gap-1 px-2 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[11px] text-ink transition cursor-pointer"
                  >
                    <Copy class="w-3 h-3" />
                    <span>{{ t("lfs.report.copy") }}</span>
                  </button>
                  <button
                    @click="showLockReport = false"
                    class="px-2 py-1 rounded-md hover:bg-raised text-[11px] text-ink-muted hover:text-ink transition cursor-pointer"
                  >
                    {{ t("lfs.report.collapse") }}
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
                <span>{{ t("lfs.protection.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("lfs.protection.desc") }}
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
                  {{ lockableSync ? (lockableSync.isProtectionActive ? t("lfs.protection.on") : t("lfs.protection.off")) : t("lfs.protection.unknown") }}
                </span>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  @click="handleSyncLockable"
                  :disabled="isSyncingLockable"
                  class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
                >
                  {{ isSyncingLockable ? t("lfs.protection.syncing") : t("lfs.protection.sync") }}
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
                      ? t("lfs.protection.toggling")
                      : lockableSync?.isProtectionActive
                        ? t("lfs.protection.disable")
                        : t("lfs.protection.enable")
                  }}
                </button>
              </div>
              <div v-if="lockableSync" class="grid grid-cols-3 gap-2">
                <div class="p-2 rounded-lg border border-line bg-canvas/40">
                  <div class="text-[10px] text-ink-muted">{{ t("lfs.protection.lockable") }}</div>
                  <div class="text-sm font-bold font-mono text-ink">{{ lockableSync.totalLockableFiles }}</div>
                </div>
                <div class="p-2 rounded-lg border border-line bg-canvas/40">
                  <div class="text-[10px] text-ink-muted">{{ t("lfs.protection.readonly") }}</div>
                  <div class="text-sm font-bold font-mono text-warning">{{ lockableSync.readOnlyCount }}</div>
                </div>
                <div class="p-2 rounded-lg border border-line bg-canvas/40">
                  <div class="text-[10px] text-ink-muted">{{ t("lfs.protection.writable") }}</div>
                  <div class="text-sm font-bold font-mono text-success">{{ lockableSync.writableCount }}</div>
                </div>
              </div>
              <div class="flex items-center gap-2 pt-1">
                <input
                  v-model="bulkExtensionsInput"
                  type="text"
                  :placeholder="t('lfs.protection.extensions.placeholder')"
                  @keydown.enter="handleTrackExtensions"
                  class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
                <button
                  @click="handleTrackExtensions"
                  :disabled="!bulkExtensionsInput.trim() || isTrackingExtensions"
                  class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
                >
                  {{ isTrackingExtensions ? t("lfs.protection.tracking") : t("lfs.protection.track") }}
                </button>
              </div>
            </div>
          </div>

          <div class="space-y-3">
            <div class="pb-2 border-b border-line">
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Rocket class="w-4 h-4 text-accent" />
                <span>{{ t("lfs.prepush.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("lfs.prepush.desc") }}
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
                  :placeholder="t('lfs.prepush.ref')"
                  class="w-32 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
                />
                <button
                  @click="handleVerifyPrePush"
                  :disabled="isVerifyingPrePush"
                  class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
                >
                  {{ isVerifyingPrePush ? t("lfs.prepush.verifying") : t("lfs.prepush.verify") }}
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
                  {{ prePushResult.isSafe ? t("lfs.prepush.safe") : t("lfs.prepush.risk") }} ·
                  {{ prePushResult.statusSummary }}
                </div>
                <div class="grid grid-cols-4 gap-2">
                  <div class="p-2 rounded-lg border border-line bg-canvas/40">
                    <div class="text-[10px] text-ink-muted">{{ t("lfs.prepush.commits") }}</div>
                    <div class="text-sm font-bold font-mono text-ink">{{ prePushResult.unpushedCommits.length }}</div>
                  </div>
                  <div class="p-2 rounded-lg border border-line bg-canvas/40">
                    <div class="text-[10px] text-ink-muted">{{ t("lfs.prepush.pointers") }}</div>
                    <div class="text-sm font-bold font-mono text-purple">{{ prePushResult.totalLfsPointers }}</div>
                  </div>
                  <div class="p-2 rounded-lg border border-line bg-canvas/40">
                    <div class="text-[10px] text-ink-muted">{{ t("lfs.prepush.missing") }}</div>
                    <div class="text-sm font-bold font-mono text-danger">{{ prePushResult.missingLfsCount }}</div>
                  </div>
                  <div class="p-2 rounded-lg border border-line bg-canvas/40">
                    <div class="text-[10px] text-ink-muted">{{ t("lfs.prepush.large") }}</div>
                    <div class="text-sm font-bold font-mono text-warning">{{ prePushResult.oversizedBlobs.length }}</div>
                  </div>
                </div>
                <div v-if="prePushResult.missingLfsFiles.length > 0" class="space-y-1">
                  <div class="text-[10px] text-ink-muted font-semibold">{{ t("lfs.prepush.missing.title") }}</div>
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
                  <div class="text-[10px] text-ink-muted font-semibold">{{ t("lfs.prepush.large.title") }}</div>
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
              <p v-else class="text-[11px] text-ink-muted">{{ t("lfs.prepush.not.run") }}</p>
            </div>
          </div>
        </div>
      </div>


    <!-- Modal: Lock File (LFS) -->
    <UiModal
      :open="showLockFileModal"
      :title="t('lfs.lock.modal.title')"
      width="max-w-md"
      @update:open="showLockFileModal = $event"
    >
      <template #header>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Lock class="w-4 h-4 text-warning" />
          {{ t("lfs.lock.modal.heading") }}
        </h3>
      </template>

      <div class="flex flex-col gap-1.5">
        <label class="text-xs text-ink-secondary font-medium">{{ t("lfs.lock.modal.path") }}</label>
        <input
          v-model="lockFilePathInput"
          type="text"
          placeholder="assets/characters/hero_albedo.png"
          class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
        />
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="showLockFileModal = false">{{ t("common.cancel") }}</UiButton>
        <UiButton variant="warning" :disabled="!lockFilePathInput.trim()" @click="handleLockFile">
          {{ t("lfs.lock.modal.confirm") }}
        </UiButton>
      </template>
    </UiModal>

    <!-- Modal: LFS Lock Notification Template -->
    <LockNotificationModal
      v-model:open="showLockNotifyModal"
      :repo-path="currentRepoPath"
      :file-path="notifyFilePath"
      :owner="notifyFileOwner"
    />

</template>
