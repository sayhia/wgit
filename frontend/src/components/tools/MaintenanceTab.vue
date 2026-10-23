<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted, onUnmounted, watchEffect } from "vue";
import {
  Wrench,
  Trash2,
  HardDrive,
  RefreshCw,
  Copy,
  Check,
  ShieldCheck,
  AlertTriangle,
  Filter,
  Activity,
  Database,
  CheckCircle2,
  AlertCircle,
  Shield,
  FileBox,
  Rocket,
  Terminal,
  FolderSearch,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import FixCenterSection from "./FixCenterSection.vue";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, GitHookInfo, RepoHealthReport, HealthDimension, HealthFixResult, ObjectStoreOptimizeResult, CrashStackDemangleResult, HotfixManifestResult } from "../../types";

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
// 8. REPOSITORY MAINTENANCE & HEALTH (维护与健康)
// -------------------------------------------------------------
const maintenanceStats = ref<RepoMaintenanceStats | null>(null);
const isLoadingMaintenance = ref(false);
const isRunningMaintenance = ref(false);

const lfsPruneResult = ref<LfsPruneResult | null>(null);
const isPruningLfs = ref(false);
const lfsPruneDryRun = ref(true);

const fsckResult = ref<GitFsckResult | null>(null);
const isCheckingHealth = ref(false);
const healthReport = ref<RepoHealthReport | null>(null);
const isRunningHealthCheck = ref(false);
const squashCount = ref(2);
const squashMessage = ref("");
const squashResult = ref("");
const isSquashing = ref(false);

const crashDump = ref("");
const crashTargetCommit = ref("");
const crashResult = ref<CrashStackDemangleResult | null>(null);
const isDemangling = ref(false);

const hotfixBaseRef = ref("");
const hotfixTargetRef = ref("HEAD");
const hotfixAppVersion = ref("1.0.0");
const hotfixResult = ref<HotfixManifestResult | null>(null);
const isGeneratingHotfix = ref(false);

async function handleDemangleCrash() {
  if (!props.currentRepoPath || !crashDump.value.trim() || isDemangling.value) return;
  isDemangling.value = true;
  try {
    crashResult.value = await gitService.demangleCrashStack(
      props.currentRepoPath,
      crashDump.value,
      crashTargetCommit.value.trim()
    );
    if (crashResult.value) {
      notify(
        `崩溃栈反解完成：${crashResult.value.demangledCount}/${crashResult.value.totalFrames} 帧已解析`
      );
    }
  } catch (err: any) {
    notify(`崩溃栈反解失败: ${err?.message || err}`);
  } finally {
    isDemangling.value = false;
  }
}

async function handleGenerateHotfix() {
  if (!props.currentRepoPath || isGeneratingHotfix.value) return;
  isGeneratingHotfix.value = true;
  try {
    hotfixResult.value = await gitService.generateHotfixManifest(
      props.currentRepoPath,
      hotfixBaseRef.value.trim(),
      hotfixTargetRef.value.trim() || "HEAD",
      hotfixAppVersion.value.trim() || "1.0.0"
    );
    if (hotfixResult.value) notify(hotfixResult.value.summary);
  } catch (err: any) {
    notify(`热更清单生成失败: ${err?.message || err}`);
  } finally {
    isGeneratingHotfix.value = false;
  }
}

async function copyHotfixManifest() {
  if (!hotfixResult.value) return;
  const text = hotfixResult.value.manifestJson;
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand("copy");
    textarea.remove();
  }
  notify("热更清单 JSON 已复制");
}

async function handleSquashCommits() {
  if (!props.currentRepoPath || isSquashing.value) return;
  if (squashCount.value < 2) {
    notify("至少需要压缩 2 条提交");
    return;
  }
  if (!squashMessage.value.trim()) {
    notify("请填写压缩后的提交说明");
    return;
  }
  if (
    !(await confirmAction(
      `确定把最近 ${squashCount.value} 条提交压缩为一条吗？\n\n将执行 git reset --soft HEAD~${squashCount.value} 后重新提交，会改写提交历史。`
    ))
  )
    return;
  isSquashing.value = true;
  try {
    squashResult.value = await gitService.squashCommits(
      props.currentRepoPath,
      squashCount.value,
      squashMessage.value.trim()
    );
    notify(`已压缩最近 ${squashCount.value} 条提交`);
    squashMessage.value = "";
    emit("refresh");
  } catch (err: any) {
    notify(`压缩提交失败: ${err?.message || err}`);
  } finally {
    isSquashing.value = false;
  }
}

// Archive / patch / bundle export & import
const repoName = computed(() => {
  const parts = props.currentRepoPath.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] || "repo";
});
const archiveRef = ref("HEAD");
const archiveDir = ref("");
const archiveFile = ref("");
const archiveMessage = ref("");
const archiveBusy = ref("");
const patchContent = ref("");
const patchContentResult = ref("");
const isApplyingPatchContent = ref(false);

async function handleCheckPatchContent() {
  if (!props.currentRepoPath || !patchContent.value.trim() || isApplyingPatchContent.value) return;
  isApplyingPatchContent.value = true;
  try {
    patchContentResult.value = await gitService.applyPatchContent(
      props.currentRepoPath,
      patchContent.value,
      true
    );
    notify("补丁内容检查通过（未写入工作区）");
  } catch (err: any) {
    patchContentResult.value = "";
    notify(`补丁内容检查失败: ${err?.message || err}`);
  } finally {
    isApplyingPatchContent.value = false;
  }
}

async function handleApplyPatchContent() {
  if (!props.currentRepoPath || !patchContent.value.trim() || isApplyingPatchContent.value) return;
  if (!(await confirmAction("确定将粘贴的补丁内容应用到工作区吗？"))) return;
  isApplyingPatchContent.value = true;
  try {
    patchContentResult.value = await gitService.applyPatchContent(
      props.currentRepoPath,
      patchContent.value,
      false
    );
    notify("补丁内容已应用到工作区");
    emit("refresh");
  } catch (err: any) {
    notify(`应用补丁内容失败: ${err?.message || err}`);
  } finally {
    isApplyingPatchContent.value = false;
  }
}

function joinArchivePath(dir: string, filename: string) {
  return `${dir.replace(/[\\/]+$/, "")}/${filename}`;
}

function safeRefName(ref: string) {
  return ref.replace(/[^\w.-]+/g, "_") || "HEAD";
}

async function handlePickArchiveDir() {
  try {
    const dir = await gitService.pickFolderDialog();
    if (dir) archiveDir.value = dir;
  } catch {
    notify("无法打开目录选择器，请手动输入输出目录");
  }
}

async function handleExportArchive() {
  if (!props.currentRepoPath || !archiveDir.value.trim() || archiveBusy.value) return;
  const ref = archiveRef.value.trim() || "HEAD";
  const output = joinArchivePath(archiveDir.value.trim(), `${repoName.value}-${safeRefName(ref)}.zip`);
  archiveBusy.value = "archive";
  try {
    await gitService.exportArchive(props.currentRepoPath, ref, output);
    archiveMessage.value = output;
    notify(`ZIP 归档已导出: ${output}`);
  } catch (err: any) {
    notify(`导出 ZIP 失败: ${err?.message || err}`);
  } finally {
    archiveBusy.value = "";
  }
}

async function handleExportPatch() {
  if (!props.currentRepoPath || !archiveDir.value.trim() || archiveBusy.value) return;
  const ref = archiveRef.value.trim() || "HEAD";
  const output = joinArchivePath(archiveDir.value.trim(), `${safeRefName(ref)}.patch`);
  archiveBusy.value = "patch";
  try {
    await gitService.exportPatch(props.currentRepoPath, ref, output);
    archiveMessage.value = output;
    archiveFile.value = output;
    notify(`补丁已导出: ${output}`);
  } catch (err: any) {
    notify(`导出补丁失败: ${err?.message || err}`);
  } finally {
    archiveBusy.value = "";
  }
}

async function handleCreateBundle() {
  if (!props.currentRepoPath || !archiveDir.value.trim() || archiveBusy.value) return;
  const ref = archiveRef.value.trim() || "HEAD";
  const output = joinArchivePath(archiveDir.value.trim(), `${repoName.value}.bundle`);
  archiveBusy.value = "bundle";
  try {
    await gitService.createGitBundle(props.currentRepoPath, output, ref === "HEAD" ? "--all" : ref);
    archiveMessage.value = output;
    archiveFile.value = output;
    notify(`Git Bundle 已创建: ${output}`);
  } catch (err: any) {
    notify(`创建 Bundle 失败: ${err?.message || err}`);
  } finally {
    archiveBusy.value = "";
  }
}

async function handleVerifyBundle() {
  if (!props.currentRepoPath || !archiveFile.value.trim() || archiveBusy.value) return;
  archiveBusy.value = "verify";
  try {
    const out = await gitService.verifyGitBundle(props.currentRepoPath, archiveFile.value.trim());
    notify(out?.trim() ? "Bundle 校验通过" : "Bundle 校验完成");
  } catch (err: any) {
    notify(`Bundle 校验失败: ${err?.message || err}`);
  } finally {
    archiveBusy.value = "";
  }
}

async function handleApplyPatch() {
  if (!props.currentRepoPath || !archiveFile.value.trim() || archiveBusy.value) return;
  if (!(await confirmAction(`确定将补丁应用到工作区吗？\n${archiveFile.value.trim()}`))) return;
  archiveBusy.value = "apply";
  try {
    await gitService.applyPatch(props.currentRepoPath, archiveFile.value.trim());
    notify("补丁已应用到工作区");
    emit("refresh");
  } catch (err: any) {
    notify(`应用补丁失败: ${err?.message || err}`);
  } finally {
    archiveBusy.value = "";
  }
}

const gitHooks = ref<GitHookInfo[]>([]);
const isLoadingHooks = ref(false);
const hookBusy = ref("");

// Briefly highlight the hook row that was just toggled (local refresh feedback).
const recentlyToggledHook = ref<string | null>(null);
let hookHighlightTimer: ReturnType<typeof setTimeout> | null = null;

function flashToggledHook(name: string) {
  recentlyToggledHook.value = name;
  if (hookHighlightTimer) clearTimeout(hookHighlightTimer);
  hookHighlightTimer = setTimeout(() => {
    recentlyToggledHook.value = null;
    hookHighlightTimer = null;
  }, 1800);
}

onUnmounted(() => {
  if (hookHighlightTimer) clearTimeout(hookHighlightTimer);
});

/** Reveal the actual hook file in the OS file manager (disabled hooks live as *.disabled). */
async function handleRevealHook(hook: GitHookInfo) {
  if (!props.currentRepoPath) return;
  const target = hook.enabled ? hook.path : `${hook.path}.disabled`;
  try {
    await gitService.revealFileInFinder(props.currentRepoPath, target);
  } catch (err: any) {
    notify(`定位钩子文件失败: ${err?.message || err}`);
  }
}

const showHookEditor = ref(false);
const hookEditName = ref("");
const hookEditContent = ref("");
const hookEditEnable = ref(true);
const hookEditMeta = ref("");
const isLoadingHook = ref(false);
const isSavingHook = ref(false);

async function openHookEditor(hook: GitHookInfo) {
  if (!props.currentRepoPath) return;
  hookEditName.value = hook.name;
  hookEditContent.value = "";
  hookEditMeta.value = "";
  hookEditEnable.value = hook.enabled || !hook.installed;
  showHookEditor.value = true;
  isLoadingHook.value = true;
  try {
    const res = await gitService.readGitHook(props.currentRepoPath, hook.name);
    hookEditContent.value = res.content;
    hookEditMeta.value = res.message;
    hookEditEnable.value = res.exists ? res.enabled : true;
  } catch (err: any) {
    notify(`读取钩子脚本失败: ${err?.message || err}`);
  } finally {
    isLoadingHook.value = false;
  }
}

async function handleSaveHook() {
  if (!props.currentRepoPath || isSavingHook.value) return;
  if (!hookEditContent.value.trim()) {
    notify("脚本内容不能为空");
    return;
  }
  isSavingHook.value = true;
  try {
    await gitService.saveGitHook(
      props.currentRepoPath,
      hookEditName.value,
      hookEditContent.value,
      hookEditEnable.value
    );
    notify(`钩子 ${hookEditName.value} 已保存${hookEditEnable.value ? "并启用" : "（保持禁用）"}`);
    showHookEditor.value = false;
    await loadGitHooks();
  } catch (err: any) {
    notify(`保存钩子脚本失败: ${err?.message || err}`);
  } finally {
    isSavingHook.value = false;
  }
}

async function handleToggleHook(hook: GitHookInfo) {
  if (!props.currentRepoPath || hookBusy.value) return;
  const enable = !hook.enabled;
  if (!(await confirmAction(`${enable ? "启用" : "禁用"} Git 钩子 ${hook.name} 吗？`))) return;
  hookBusy.value = hook.name;
  try {
    await gitService.toggleGitHook(props.currentRepoPath, hook.name, enable, "");
    notify(`钩子 ${hook.name} 已${enable ? "启用" : "禁用"}`);
    await loadGitHooks();
    flashToggledHook(hook.name);
    emit("refresh");
  } catch (err: any) {
    notify(`切换钩子状态失败: ${err?.message || err}`);
  } finally {
    hookBusy.value = "";
  }
}
const enabledHooksCount = computed(() => gitHooks.value.filter((hook) => hook.enabled).length);
const hooksToEnableCount = computed(
  () => gitHooks.value.filter((hook) => hook.installed && !hook.enabled).length
);
const hooksToDisableCount = computed(() => gitHooks.value.filter((hook) => hook.enabled).length);

/** Bulk enable (only installed-but-disabled hooks) or disable (all enabled hooks). */
async function bulkToggleHooks(enable: boolean) {
  if (!props.currentRepoPath || hookBusy.value) return;
  const targets = gitHooks.value.filter((hook) =>
    enable ? hook.installed && !hook.enabled : hook.enabled
  );
  if (targets.length === 0) {
    notify(enable ? "没有已安装但被禁用的钩子" : "当前没有已启用的钩子");
    return;
  }
  if (!(await confirmAction(`确定${enable ? "启用" : "禁用"} ${targets.length} 个 Git 钩子吗？`))) return;
  let succeeded = 0;
  let failed = 0;
  let lastToggled = "";
  for (const hook of targets) {
    hookBusy.value = hook.name;
    try {
      await gitService.toggleGitHook(props.currentRepoPath, hook.name, enable, "");
      succeeded++;
      lastToggled = hook.name;
    } catch (err: any) {
      failed++;
      notify(`钩子 ${hook.name} 切换失败: ${err?.message || err}`);
    }
  }
  hookBusy.value = "";
  notify(`批量${enable ? "启用" : "禁用"}完成：成功 ${succeeded} 个${failed ? `，失败 ${failed} 个` : ""}`);
  await loadGitHooks();
  if (lastToggled) flashToggledHook(lastToggled);
  emit("refresh");
}

async function loadGitHooks() {
  if (!props.currentRepoPath) return;
  isLoadingHooks.value = true;
  try {
    gitHooks.value = (await gitService.getGitHooks(props.currentRepoPath)) || [];
  } catch (err) {
    console.error("Failed to load git hooks:", err);
  } finally {
    isLoadingHooks.value = false;
  }
}

async function loadMaintenanceStats() {
  if (!props.currentRepoPath) return;
  isLoadingMaintenance.value = true;
  try {
    const stats = await gitService.getMaintenanceStats(props.currentRepoPath);
    maintenanceStats.value = stats;
  } catch (err) {
    console.error("Failed to load maintenance stats:", err);
  } finally {
    isLoadingMaintenance.value = false;
  }
}

async function handleRunMaintenance() {
  if (!props.currentRepoPath) return;
  isRunningMaintenance.value = true;
  try {
    const res = await gitService.runMaintenance(props.currentRepoPath);
    maintenanceStats.value = res;
    if (res?.success === false) {
      notify(`维护完成，但有 ${res.warnings?.length ?? 0} 个步骤失败，请查看详情`);
    } else if (res?.remotePruneStatus === "SKIPPED") {
      notify("本地 GC / Prune 完成；仓库无远端，已跳过远端引用清理");
    } else {
      notify("Git GC 深度垃圾回收与远端清理已完成");
    }
  } catch (err: any) {
    notify(`执行维护失败: ${err?.message || err}`);
  } finally {
    isRunningMaintenance.value = false;
  }
}

async function handleLfsPrune() {
  if (!props.currentRepoPath) return;
  isPruningLfs.value = true;
  try {
    const res = await gitService.lfsPrune(props.currentRepoPath, lfsPruneDryRun.value, false);
    lfsPruneResult.value = res;
    if (res?.success) {
      notify(
        lfsPruneDryRun.value
          ? `预检演练完成: 可释放 ${res.reclaimedStr || "0 B"} (${res.prunedFiles} 个文件)`
          : `LFS 缓存清理完成: 成功释放 ${res.reclaimedStr || "0 B"} 磁盘空间`
      );
    }
  } catch (err: any) {
    notify(`LFS 清理失败: ${err?.message || err}`);
  } finally {
    isPruningLfs.value = false;
  }
}

async function handleRunHealthCheck() {
  if (!props.currentRepoPath || isRunningHealthCheck.value) return;
  isRunningHealthCheck.value = true;
  try {
    healthReport.value = await gitService.runRepoHealthCheck(props.currentRepoPath);
    if (healthReport.value) {
      notify(
        `综合体检完成：${healthReport.value.score}/100 · ${healthReport.value.grade}（通过 ${healthReport.value.passCount} / 警告 ${healthReport.value.warnCount} / 失败 ${healthReport.value.failCount}）`
      );
    }
  } catch (err: any) {
    notify(`综合体检失败: ${err?.message || err}`);
  } finally {
    isRunningHealthCheck.value = false;
  }
}

// Health-check remediation: run the fix behind a dimension, or jump to the
// panel that needs a human decision. Passing dimensions get no button.
const fixingKey = ref("");
const fixResults = ref<Record<string, HealthFixResult>>({});

function healthFixLabel(item: HealthDimension): string {
  switch (item.key) {
    case "objects":
      return "执行 fsck + gc";
    case "ignore":
      return "一键补全规则";
    case "largefiles":
      return "去处理大文件";
    case "workspace":
      return "去解决冲突";
    case "signing":
      return "去配置签名";
    default:
      return "去填写身份";
  }
}

function healthFixIsWrite(item: HealthDimension): boolean {
  return item.key === "objects" || item.key === "ignore";
}

async function handleHealthFix(item: HealthDimension) {
  if (!props.currentRepoPath || fixingKey.value) return;
  if (healthFixIsWrite(item)) {
    const verb = item.key === "objects" ? "运行 git fsck --full 与 gc --prune=now" : "补全忽略规则并取消跟踪误提交的构建产物";
    if (!(await confirmAction(`确定要${verb}吗？`))) return;
  }

  fixingKey.value = item.key;
  try {
    const res = await gitService.runHealthFix(props.currentRepoPath, item.key);
    fixResults.value = { ...fixResults.value, [item.key]: res };

    if (res.needsInput) {
      notify(res.hint || "需要你手动处理，已为你跳转。");
      if (res.target === "settings") {
        const section = item.key === "signing" ? "signing" : "identity";
        window.dispatchEvent(new CustomEvent("wgit:settings-section", { detail: { section } }));
      } else if (res.target === "lfs") {
        window.dispatchEvent(new CustomEvent("wgit:tools-tab", { detail: { tab: "lfs" } }));
      } else if (res.target === "conflict") {
        emit("refresh");
      }
      return;
    }

    notify(res.hint || res.output || "已执行修复。");
    await handleRunHealthCheck();
    emit("refresh");
  } catch (err: any) {
    notify(`自动修复失败: ${err?.message || err}`);
  } finally {
    fixingKey.value = "";
  }
}

// Object-store tuning: the passes that make a 100k-commit repo feel fast.
const OBJECT_STORE_TASKS = [
  { key: "commit-graph", label: "提交图索引", hint: "跳过提交遍历，加速 git log / blame" },
  { key: "multi-pack-index", label: "多包索引 MIDX", hint: "合并包查找表，减少 pack 扫描" },
  { key: "repack", label: "重打包对象", hint: "合并松散对象与旧包，耗时较长" },
  { key: "pack-refs", label: "压缩引用", hint: "把上千个松散 ref 合并成单文件" },
] as const;
const objectStoreTasks = ref<string[]>(["commit-graph", "multi-pack-index", "pack-refs"]);
const isOptimizingStore = ref(false);
const optimizeResult = ref<ObjectStoreOptimizeResult | null>(null);

function toggleObjectStoreTask(key: string) {
  objectStoreTasks.value = objectStoreTasks.value.includes(key)
    ? objectStoreTasks.value.filter((t) => t !== key)
    : [...objectStoreTasks.value, key];
}

async function handleOptimizeObjectStore() {
  if (!props.currentRepoPath || isOptimizingStore.value) return;
  if (objectStoreTasks.value.length === 0) {
    notify("请至少选择一项优化");
    return;
  }
  const hasRepack = objectStoreTasks.value.includes("repack");
  const confirmed = await confirmAction(
    { message: `将执行 ${objectStoreTasks.value.length} 项对象库优化：${objectStoreTasks.value.join("、")}。` +
      (hasRepack ? "\n\n重打包会重写全部 pack，大仓库可能需要数分钟，期间请勿关闭窗口。" : ""), tone: "danger" }
  );
  if (!confirmed) return;

  isOptimizingStore.value = true;
  try {
    const res = await gitService.optimizeObjectStore(props.currentRepoPath, objectStoreTasks.value);
    optimizeResult.value = res;
    notify(res.message);
    emit("refresh");
  } catch (err: any) {
    notify(`对象库优化失败: ${err?.message || err}`);
  } finally {
    isOptimizingStore.value = false;
  }
}

async function handleVerifyRepoHealth() {  if (!props.currentRepoPath) return;
  isCheckingHealth.value = true;
  try {
    const res = await gitService.verifyRepoHealth(props.currentRepoPath);
    fsckResult.value = res;
    if (res?.isClean) {
      notify("仓库数据库完整性检查通过，状态健康！");
    } else {
      notify("检测到仓库数据库可能存在异常，请查看详情报告");
    }
  } catch (err: any) {
    notify(`健康检查失败: ${err?.message || err}`);
  } finally {
    isCheckingHealth.value = false;
  }
}


watchEffect(() => {
  toolsStatus.loading.maintenance =
    isLoadingMaintenance.value ||
    isRunningMaintenance.value ||
    isCheckingHealth.value ||
    isRunningHealthCheck.value ||
    isSquashing.value ||
    isDemangling.value ||
    isGeneratingHotfix.value ||
    isPruningLfs.value ||
    isLoadingHooks.value ||
    !!hookBusy.value ||
    !!archiveBusy.value ||
    isApplyingPatchContent.value;
});

onMounted(() => {
  void loadMaintenanceStats();
  void loadGitHooks();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadMaintenanceStats();
    void loadGitHooks();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 8: 仓库维护与健康体检 (Maintenance)         -->
      <!-- ============================================== -->
      <div class="space-y-6">
        <!-- Section A: Git GC & Stats -->
        <div class="space-y-3" data-tool-section="gc">
          <div class="flex flex-wrap items-center justify-between gap-2 pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Database class="w-4 h-4 text-purple" />
                <span>Git 核心数据库指标与深度垃圾回收 (Git GC & Prune)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                清理无效引用、回收松散碎片对象，压缩 Packfile，保持仓库高性能。
              </p>
            </div>

            <button
              @click="handleRunMaintenance"
              :disabled="isRunningMaintenance"
              class="flex items-center gap-1.5 shrink-0 whitespace-nowrap px-3.5 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer shadow-xs disabled:opacity-50"
            >
              <Activity class="w-3.5 h-3.5" :class="{ 'animate-spin': isRunningMaintenance }" />
              <span>{{ isRunningMaintenance ? "优化中..." : "一键深度垃圾回收 (Git GC)" }}</span>
            </button>
          </div>

          <div class="grid grid-cols-3 gap-4">
            <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-xs text-ink-muted font-medium">.git 目录物理大小</span>
              <span class="text-xl font-bold font-mono text-ink">{{ maintenanceStats?.gitDirSize || '未计算' }}</span>
              <span class="text-[10px] text-ink-muted">包含对象数据库、Reflog 及分支引用</span>
            </div>
            <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-xs text-ink-muted font-medium">Pack 压缩包数量</span>
              <span class="text-xl font-bold font-mono text-purple">{{ maintenanceStats?.packFilesCount ?? 0 }} 个</span>
              <span class="text-[10px] text-ink-muted">已压缩封装的高效对象集</span>
            </div>
            <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-xs text-ink-muted font-medium">Loose 松散对象数量</span>
              <span class="text-xl font-bold font-mono text-warning">{{ maintenanceStats?.looseObjects ?? 0 }} 个</span>
              <span class="text-[10px] text-ink-muted">未打包的碎片对象，数量过多时建议 GC</span>
            </div>
          </div>

          <div
            v-if="maintenanceStats?.summary"
            class="rounded-lg border px-3 py-2 text-[11px]"
            :class="maintenanceStats.success
              ? 'border-line bg-surface/40 text-ink-muted'
              : 'border-warning/30 bg-warning/10 text-warning'"
          >
            <div class="flex items-start gap-1.5">
              <AlertTriangle v-if="!maintenanceStats.success" class="w-3.5 h-3.5 shrink-0 mt-0.5" />
              <CheckCircle2 v-else class="w-3.5 h-3.5 shrink-0 mt-0.5" />
              <div class="min-w-0">
                <div>{{ maintenanceStats.summary }}</div>
                <ul v-if="(maintenanceStats.warnings || []).length > 0" class="mt-1 space-y-0.5">
                  <li v-for="(warning, i) in maintenanceStats.warnings" :key="i">{{ warning }}</li>
                </ul>
              </div>
            </div>
          </div>
        </div>

        <!-- Section B: LFS Prune Cache -->
        <div class="space-y-3 pt-2" data-tool-section="lfs-prune">
          <div class="flex flex-wrap items-center justify-between gap-2 pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <HardDrive class="w-4 h-4 text-accent" />
                <span>Git LFS 本地对象缓存瘦身 (LFS Prune)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                清理本地无引用的历史 LFS 二进制文件，仅保留当前 HEAD 及分支所需的资产，安全释放 GB 级磁盘空间。
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-3 shrink-0">
              <label class="flex items-center gap-1.5 text-xs text-ink-muted cursor-pointer select-none">
                <input
                  v-model="lfsPruneDryRun"
                  type="checkbox"
                  class="rounded accent-accent"
                />
                <span>预检演练 (Dry Run，不删除文件)</span>
              </label>

              <button
                @click="handleLfsPrune"
                :disabled="isPruningLfs"
                class="flex items-center gap-1.5 shrink-0 whitespace-nowrap px-3.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-50"
              >
                <Trash2 class="w-3.5 h-3.5" :class="{ 'animate-spin': isPruningLfs }" />
                <span>{{ isPruningLfs ? "清理中..." : (lfsPruneDryRun ? "预检可释放空间" : "立即清理 LFS 缓存") }}</span>
              </button>
            </div>
          </div>

          <div
            v-if="lfsPruneResult"
            class="p-4 rounded-xl border border-line bg-surface/30 flex items-center justify-between"
          >
            <div class="flex items-center gap-3">
              <CheckCircle2 class="w-5 h-5 text-success shrink-0" />
              <div class="text-xs">
                <div class="font-bold text-ink">
                  {{ lfsPruneResult.dryRun ? '演练预检完成' : 'LFS 缓存清理完成' }}
                </div>
                <div class="text-ink-muted font-mono mt-0.5">
                  已清除 {{ lfsPruneResult.prunedFiles }} 个历史大文件，回收空间:
                  <strong class="text-accent font-bold">{{ lfsPruneResult.reclaimedStr || '0 B' }}</strong>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Section C: FSCK Health Check -->
        <div class="space-y-3 pt-2" data-tool-section="fsck">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Shield class="w-4 h-4 text-success" />
                <span>仓库数据库健康体检 (Git FSCK Integrity Check)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                全面验证 Git 对象数据库的 SHA 校验和与一致性，排查损坏的提交或悬空对象。
              </p>
            </div>

            <button
              @click="handleVerifyRepoHealth"
              :disabled="isCheckingHealth"
              class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              <ShieldCheck class="w-3.5 h-3.5 text-success" :class="{ 'animate-spin': isCheckingHealth }" />
              <span>{{ isCheckingHealth ? "体检中..." : "开始完整性体检" }}</span>
            </button>
          </div>

          <div v-if="fsckResult" class="p-4 rounded-xl border border-line bg-surface/30 space-y-3">
            <div class="flex items-center gap-2">
              <CheckCircle2 v-if="fsckResult.isClean" class="w-5 h-5 text-success shrink-0" />
              <AlertCircle v-else class="w-5 h-5 text-danger shrink-0" />
              <span class="text-xs font-bold" :class="fsckResult.isClean ? 'text-success' : 'text-danger'">
                {{ fsckResult.isClean ? '仓库数据库健康完整，未发现严重破坏与错误' : '检测到仓库数据库可能存在异常' }}
              </span>
            </div>

            <div class="grid grid-cols-3 gap-3 font-mono text-xs pt-1">
              <div class="p-2.5 rounded-lg bg-surface/40 border border-line flex justify-between">
                <span class="text-ink-muted">悬空 Blob:</span>
                <span class="font-bold text-ink">{{ fsckResult.danglingBlob }} 个</span>
              </div>
              <div class="p-2.5 rounded-lg bg-surface/40 border border-line flex justify-between">
                <span class="text-ink-muted">悬空 Tree:</span>
                <span class="font-bold text-ink">{{ fsckResult.danglingTree }} 个</span>
              </div>
              <div class="p-2.5 rounded-lg bg-surface/40 border border-line flex justify-between">
                <span class="text-ink-muted">悬空 Commit:</span>
                <span class="font-bold text-ink">{{ fsckResult.danglingCommit }} 个</span>
              </div>
            </div>

            <div v-if="fsckResult.rawErrors && fsckResult.rawErrors.length > 0" class="p-3 rounded-lg bg-danger/10 border border-danger/30 text-xs text-danger font-mono">
              <div class="font-bold pb-1">错误详情:</div>
              <div v-for="(err, idx) in fsckResult.rawErrors" :key="idx">{{ err }}</div>
            </div>
          </div>
        </div>

        <!-- Section D: Git Hooks Status -->
        <div class="space-y-3 pt-2" data-tool-section="hooks">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Terminal class="w-4 h-4 text-warning" />
                <span>Git 客户端钩子状态 (Hooks)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                检查 .git/hooks 下常用钩子是否已安装并具备可执行权限；只读检查，不修改脚本。
              </p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <span class="text-xs font-mono text-ink-muted">
                {{ enabledHooksCount }} / {{ gitHooks.length }} 已启用
              </span>
              <button
                @click.stop="bulkToggleHooks(true)"
                :disabled="!!hookBusy || hooksToEnableCount === 0"
                class="px-2 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-success transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                :title="`启用全部已安装但被禁用的钩子（${hooksToEnableCount} 个）`"
              >
                全部启用
              </button>
              <button
                @click.stop="bulkToggleHooks(false)"
                :disabled="!!hookBusy || hooksToDisableCount === 0"
                class="px-2 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-warning transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                :title="`禁用全部已启用钩子（${hooksToDisableCount} 个）`"
              >
                全部禁用
              </button>
            </div>
          </div>

          <div v-if="isLoadingHooks && gitHooks.length === 0" class="py-2 text-xs text-ink-muted">
            正在检查钩子...
          </div>
          <div v-else class="grid grid-cols-2 gap-2">
            <div
              v-for="hook in gitHooks"
              :key="hook.name"
              class="flex items-center gap-2 px-3 py-2 rounded-lg border border-line bg-surface/30 transition-all duration-300"
              :class="
                recentlyToggledHook === hook.name
                  ? hook.enabled
                    ? 'ring-1 ring-success/50 bg-success/5'
                    : 'ring-1 ring-warning/50 bg-warning/5'
                  : ''
              "
              :title="hook.path"
            >
              <span
                class="w-1.5 h-1.5 rounded-full shrink-0"
                :class="hook.enabled ? 'bg-success-strong' : 'bg-ink-muted/40'"
              ></span>
              <span class="font-mono text-xs" :class="hook.enabled ? 'text-ink' : 'text-ink-muted'">
                {{ hook.name }}
              </span>
              <span
                class="ml-auto text-[10px] font-semibold"
                :class="hook.enabled ? 'text-success' : 'text-ink-muted'"
              >
                {{ hook.enabled ? "已启用" : hook.installed ? "已禁用" : "未安装" }}
              </span>
              <button aria-label="在文件管理器中定位钩子文件"
                v-if="hook.installed"
                @click.stop="handleRevealHook(hook)"
                class="p-1 rounded bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
                title="在文件管理器中定位钩子文件"
              >
                <FolderSearch class="w-3 h-3" />
              </button>
              <button aria-label="编辑钩子脚本内容（新建或修改）"
                @click.stop="openHookEditor(hook)"
                class="p-1 rounded bg-surface hover:bg-raised border border-line text-ink-muted hover:text-accent transition cursor-pointer shrink-0"
                title="编辑钩子脚本内容（新建或修改）"
              >
                <FileCode class="w-3 h-3" />
              </button>
              <button
                v-if="hook.installed"
                @click.stop="handleToggleHook(hook)"
                :disabled="!!hookBusy"
                class="px-1.5 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[9px] font-semibold text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
                :title="hook.enabled ? '重命名为 .disabled 以禁用' : '恢复文件并赋予可执行权限'"
              >
                {{ hookBusy === hook.name ? "处理中" : hook.enabled ? "禁用" : "启用" }}
              </button>
            </div>
          </div>
        </div>

        <!-- Hook script editor -->
        <UiModal
          :open="showHookEditor"
          :label="`编辑钩子脚本：${hookEditName}`"
          width="max-w-2xl"
          @update:open="showHookEditor = $event"
        >
          <template #header>
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <FileCode class="w-4 h-4 text-warning" />
              <span class="font-mono">{{ hookEditName }}</span>
            </h3>
          </template>

          <div class="space-y-3">
            <p v-if="hookEditMeta" class="text-[11px] text-ink-muted">{{ hookEditMeta }}</p>
            <div v-if="isLoadingHook" class="py-6 text-center text-xs text-ink-muted">正在读取脚本...</div>
            <template v-else>
              <textarea
                v-model="hookEditContent"
                spellcheck="false"
                rows="14"
                aria-label="钩子脚本内容"
                placeholder="#!/bin/sh&#10;# 在这里编写钩子逻辑，例如：&#10;npm run lint"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
              ></textarea>
              <label class="flex items-center gap-2 text-xs text-ink cursor-pointer select-none">
                <input v-model="hookEditEnable" type="checkbox" class="rounded accent-[var(--ug-accent-strong)]" />
                <span>启用该钩子（写入可执行权限；取消则保存为 .disabled）</span>
              </label>
            </template>
          </div>

          <template #footer>
            <UiButton variant="ghost" @click="showHookEditor = false">取消</UiButton>
            <UiButton variant="primary" :disabled="isSavingHook || isLoadingHook" @click="handleSaveHook">
              {{ isSavingHook ? "保存中..." : "保存脚本" }}
            </UiButton>
          </template>
        </UiModal>

        <!-- Section E: Archive, patch & bundle -->
        <div class="space-y-3 pt-2" data-tool-section="archive">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <FileBox class="w-4 h-4 text-accent" />
              <span>归档、补丁与 Git Bundle</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              将指定提交/引用导出为 ZIP 或 .patch，创建/校验离线 Bundle，或将补丁应用到当前工作区。
            </p>
          </div>

          <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
            <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
              <div class="text-xs font-bold text-ink">导出 / 备份</div>
              <div class="grid grid-cols-2 gap-2">
                <label class="flex flex-col gap-1">
                  <span class="text-[10px] text-ink-muted">提交 / 引用 (ref)</span>
                  <input
                    v-model="archiveRef"
                    type="text"
                    placeholder="HEAD / v1.0.0 / feature/x"
                    class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                  />
                </label>
                <label class="flex flex-col gap-1">
                  <span class="text-[10px] text-ink-muted">输出目录</span>
                  <div class="flex items-center gap-1">
                    <input
                      v-model="archiveDir"
                      type="text"
                      placeholder="/tmp 或 ~/Downloads"
                      class="flex-1 min-w-0 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                    <button
                      @click="handlePickArchiveDir"
                      class="px-2 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-ink transition cursor-pointer shrink-0"
                      title="选择输出目录"
                    >
                      浏览
                    </button>
                  </div>
                </label>
              </div>
              <div class="flex flex-wrap items-center gap-2 pt-1">
                <button
                  @click="handleExportArchive"
                  :disabled="!archiveDir.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "archive" ? "导出中..." : "导出 ZIP" }}
                </button>
                <button
                  @click="handleExportPatch"
                  :disabled="!archiveDir.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "patch" ? "导出中..." : "导出补丁 (.patch)" }}
                </button>
                <button
                  @click="handleCreateBundle"
                  :disabled="!archiveDir.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "bundle" ? "创建中..." : "创建 Bundle" }}
                </button>
              </div>
            </div>

            <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
              <div class="text-xs font-bold text-ink">应用 / 校验</div>
              <label class="flex flex-col gap-1">
                <span class="text-[10px] text-ink-muted">补丁或 Bundle 文件路径</span>
                <input
                  v-model="archiveFile"
                  type="text"
                  placeholder="/tmp/HEAD.patch 或 /tmp/repo.bundle"
                  class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
              </label>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  @click="handleApplyPatch"
                  :disabled="!archiveFile.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "apply" ? "应用中..." : "应用补丁到工作区" }}
                </button>
                <button
                  @click="handleVerifyBundle"
                  :disabled="!archiveFile.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-success transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "verify" ? "校验中..." : "校验 Bundle" }}
                </button>
              </div>
              <label class="flex flex-col gap-1">
                <span class="text-[10px] text-ink-muted">或粘贴补丁内容</span>
                <textarea
                  v-model="patchContent"
                  rows="5"
                  spellcheck="false"
                  placeholder="粘贴 unified diff / git format-patch 内容..."
                  class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-[11px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
                ></textarea>
              </label>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  @click="handleCheckPatchContent"
                  :disabled="!patchContent.trim() || isApplyingPatchContent"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                >
                  {{ isApplyingPatchContent ? "处理中..." : "检查补丁内容" }}
                </button>
                <button
                  @click="handleApplyPatchContent"
                  :disabled="!patchContent.trim() || isApplyingPatchContent"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40"
                >
                  应用补丁内容
                </button>
              </div>
              <pre v-if="patchContentResult" class="p-2 rounded bg-canvas/70 border border-line/60 text-[10px] text-ink-secondary whitespace-pre-wrap max-h-32 overflow-y-auto">{{ patchContentResult }}</pre>
              <p class="text-[10px] text-ink-muted">补丁应用不会自动提交；Bundle 适合离线传输与完整备份。</p>
            </div>
          </div>

          <div v-if="archiveMessage" class="px-3 py-2 rounded-lg bg-canvas/60 border border-line font-mono text-[10px] text-ink-secondary truncate" :title="archiveMessage">
            最近输出: {{ archiveMessage }}
          </div>
        </div>

        <!-- Section E2: One-click fix center -->
        <div class="space-y-3 pt-4">
          <FixCenterSection :current-repo-path="currentRepoPath" :refresh-nonce="refreshNonce" @refresh="emit('refresh')" @notify="notify" />
        </div>

        <!-- Section F: Composite repository health -->
        <div class="space-y-3 pt-4" data-tool-section="health">
          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Activity class="w-4 h-4 text-success" />
                <span>综合仓库体检 (Health Score)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                汇总提交身份、对象完整性、忽略规则、大文件、凭据与冲突状态，输出 0–100 评分。
              </p>
            </div>
            <button
              @click="handleRunHealthCheck"
              :disabled="isRunningHealthCheck"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isRunningHealthCheck }" />
              <span>{{ isRunningHealthCheck ? "体检中..." : "运行综合体检" }}</span>
            </button>
          </div>

          <template v-if="healthReport">
            <div class="grid grid-cols-5 gap-2">
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">总分</div>
                <div
                  class="text-lg font-bold font-mono"
                  :class="healthReport.score >= 85 ? 'text-success' : healthReport.score >= 60 ? 'text-warning' : 'text-danger'"
                >
                  {{ healthReport.score }}
                </div>
              </div>
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">评级</div>
                <div class="text-lg font-bold text-ink">{{ healthReport.grade }}</div>
              </div>
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">通过</div>
                <div class="text-lg font-bold font-mono text-success">{{ healthReport.passCount }}</div>
              </div>
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">警告</div>
                <div class="text-lg font-bold font-mono text-warning">{{ healthReport.warnCount }}</div>
              </div>
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">失败</div>
                <div class="text-lg font-bold font-mono text-danger">{{ healthReport.failCount }}</div>
              </div>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <div
                v-for="item in healthReport.items"
                :key="item.key"
                class="p-2.5 rounded-lg border border-line bg-canvas/40 space-y-1"
              >
                <div class="flex items-center gap-2 text-[11px]">
                  <span>{{ item.icon }}</span>
                  <span class="font-bold text-ink flex-1">{{ item.title }}</span>
                  <span
                    class="px-1.5 py-0.2 rounded text-[9px] font-bold"
                    :class="item.status === 'pass' ? 'bg-success/15 text-success' : item.status === 'warn' ? 'bg-warning/15 text-warning' : 'bg-danger/15 text-danger'"
                  >
                    {{ item.status === "pass" ? "通过" : item.status === "warn" ? "警告" : "失败" }}
                  </span>
                  <span class="font-mono text-ink-muted">{{ item.score }}</span>
                </div>
                <p class="text-[10px] text-ink-muted leading-relaxed">{{ item.detail }}</p>
                <div v-if="item.status !== 'pass'" class="flex items-center gap-2 pt-0.5">
                  <button
                    @click="handleHealthFix(item)"
                    :disabled="fixingKey === item.key"
                    class="px-2 py-0.5 rounded-md border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                  >
                    {{ fixingKey === item.key ? "处理中..." : healthFixLabel(item) }}
                  </button>
                  <span class="text-[10px] text-ink-muted/70">修复后自动复检</span>
                </div>
                <p
                  v-if="fixResults[item.key]"
                  class="text-[10px] leading-relaxed rounded px-1.5 py-1"
                  :class="fixResults[item.key]?.success ? 'bg-success/10 text-success' : 'bg-warning/10 text-warning'"
                >
                  {{ fixResults[item.key]?.output || fixResults[item.key]?.hint }}
                </p>
              </div>
            </div>

            <p class="text-[10px] text-ink-muted">{{ healthReport.summary }} · 检查于 {{ healthReport.checkedAt }}</p>
          </template>
          <p v-else class="text-[11px] text-ink-muted">尚未运行综合体检。</p>
        </div>

        <!-- Section F2: Object store tuning -->
        <div class="space-y-3 pt-4" data-tool-section="object-store">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Database class="w-4 h-4 text-accent" />
              <span>对象库优化（大仓库提速）</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              写入提交图、多包索引与引用压缩，让十万级提交的 git log / blame 明显变快。只读操作，不改历史。
            </p>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-1.5">
            <label
              v-for="task in OBJECT_STORE_TASKS"
              :key="task.key"
              class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition"
            >
              <input
                type="checkbox"
                :checked="objectStoreTasks.includes(task.key)"
                @change="toggleObjectStoreTask(task.key)"
                class="mt-0.5 accent-[var(--ug-accent-strong)]"
              />
              <span class="min-w-0">
                <span class="block text-[11px] font-semibold text-ink">{{ task.label }}</span>
                <span class="block text-[10px] text-ink-muted">{{ task.hint }}</span>
              </span>
            </label>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="handleOptimizeObjectStore"
              :disabled="isOptimizingStore || objectStoreTasks.length === 0"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isOptimizingStore }" />
              <span>{{ isOptimizingStore ? "优化中..." : `执行优化 (${objectStoreTasks.length})` }}</span>
            </button>
            <span v-if="optimizeResult" class="text-[10px] text-ink-muted font-mono">
              .git {{ optimizeResult.gitDirBefore }} → {{ optimizeResult.gitDirAfter }}
            </span>
          </div>

          <div v-if="optimizeResult" class="space-y-1">
            <div
              v-for="r in optimizeResult.results"
              :key="r.task"
              class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border text-[11px]"
              :class="r.skipped ? 'border-line bg-surface/30' : r.success ? 'border-success/25 bg-success/10' : 'border-warning/25 bg-warning/10'"
            >
              <span
                class="shrink-0 px-1 py-0.2 rounded text-[9px] font-bold"
                :class="r.skipped ? 'bg-surface text-ink-muted' : r.success ? 'bg-success-strong text-success-contrast' : 'bg-warning-strong text-warning-contrast'"
              >{{ r.skipped ? "跳过" : r.success ? "OK" : "FAIL" }}</span>
              <span class="font-semibold text-ink shrink-0">{{ r.label }}</span>
              <span class="text-ink-muted flex-1 truncate" :title="r.output">{{ r.skipReason || r.output || "完成" }}</span>
              <span v-if="r.durationMs > 0" class="font-mono text-ink-muted/70 shrink-0">{{ (r.durationMs / 1000).toFixed(1) }}s</span>
            </div>
          </div>
        </div>

        <!-- Section G: Squash recent commits -->
        <div class="space-y-3 pt-4" data-tool-section="squash">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Wrench class="w-4 h-4 text-warning" />
              <span>历史整理：压缩最近提交</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              将最近 N 条提交软重置后合并为一条新提交；会改写历史，推送前请确认分支未被他人使用。
            </p>
          </div>
          <div class="rounded-xl border border-warning/20 bg-warning/5 p-3 space-y-2">
            <div class="grid grid-cols-1 md:grid-cols-4 gap-2">
              <input
                v-model.number="squashCount"
                type="number"
                min="2"
                aria-label="压缩提交数量"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <input
                v-model="squashMessage"
                type="text"
                placeholder="压缩后的提交说明"
                @keydown.enter="handleSquashCommits"
                class="md:col-span-2 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <button
                @click="handleSquashCommits"
                :disabled="isSquashing || squashCount < 2 || !squashMessage.trim()"
                class="px-3 py-1.5 rounded-lg bg-warning-strong hover:opacity-90 text-warning-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                {{ isSquashing ? "压缩中..." : "压缩提交" }}
              </button>
            </div>
            <pre v-if="squashResult" class="p-2 rounded bg-canvas/70 border border-line/60 text-[10px] text-ink-secondary whitespace-pre-wrap max-h-32 overflow-y-auto">{{ squashResult }}</pre>
            <p class="text-[10px] text-ink-muted">压缩后生成新的提交 Hash；如已推送，需要强制推送才能同步远端。</p>
          </div>
        </div>

        <!-- Section H: Crash stack demangle -->
        <div class="space-y-3 pt-4" data-tool-section="demangle">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <AlertTriangle class="w-4 h-4 text-danger" />
              <span>崩溃栈反解 (Crash Stack Demangle)</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              粘贴原始崩溃日志，解析帧地址、模块与源码位置，并尝试定位责任提交与作者。
            </p>
          </div>
          <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
            <textarea
              v-model="crashDump"
              rows="6"
              spellcheck="false"
              placeholder="粘贴崩溃日志，例如&#10;Crash Reason: SIGSEGV&#10;Thread 0 Crashed:&#10;0   MyGame  0x0000000104a2b3c0 Game::Update + 32 (game.cpp:128)"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-[11px] text-ink font-mono leading-relaxed placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
            ></textarea>
            <div class="flex flex-wrap items-center gap-2">
              <input
                v-model="crashTargetCommit"
                type="text"
                placeholder="目标提交（可选，默认 HEAD）"
                class="w-56 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <button
                @click="handleDemangleCrash"
                :disabled="!crashDump.trim() || isDemangling"
                class="px-3 py-1.5 rounded-lg bg-danger-strong hover:opacity-90 text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                {{ isDemangling ? "解析中..." : "反解崩溃栈" }}
              </button>
              <span class="text-[10px] text-ink-muted">不会修改仓库，仅解析文本并与 Git 历史比对。</span>
            </div>
          </div>

          <template v-if="crashResult">
            <div class="grid grid-cols-4 gap-2">
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">崩溃原因</div>
                <div class="text-xs font-bold text-danger truncate">{{ crashResult.crashReason }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">故障模块</div>
                <div class="text-xs font-bold text-ink truncate">{{ crashResult.faultModule }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">解析帧</div>
                <div class="text-sm font-bold font-mono text-success">{{ crashResult.demangledCount }}/{{ crashResult.totalFrames }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">责任提交</div>
                <div class="text-xs font-bold font-mono text-accent truncate">{{ crashResult.culpritCommit || "—" }}</div>
              </div>
            </div>
            <div v-if="crashResult.culpritFrame" class="p-2.5 rounded-lg border border-danger/30 bg-danger/5 text-[11px] space-y-0.5">
              <div class="text-danger font-bold">疑似责任帧</div>
              <div class="font-mono text-ink truncate">{{ crashResult.culpritFrame.sourceFile }}:{{ crashResult.culpritFrame.lineNumber }} · {{ crashResult.culpritFrame.demangledSymbol }}</div>
              <div class="text-ink-muted">{{ crashResult.culpritAuthor || "作者未知" }} · {{ crashResult.culpritCommit || "提交未知" }}</div>
            </div>
            <div class="rounded-xl border border-line bg-surface/30 divide-y divide-line/40 max-h-60 overflow-y-auto">
              <div
                v-for="frame in crashResult.frames || []"
                :key="`${frame.frameIndex}:${frame.address}`"
                class="px-2.5 py-1.5 text-[10px] font-mono space-y-0.5"
                :class="frame.isCulprit ? 'bg-danger/5' : ''"
              >
                <div class="flex items-center gap-2">
                  <span class="text-ink-muted w-6 text-right shrink-0">#{{ frame.frameIndex }}</span>
                  <span class="text-accent shrink-0">{{ frame.moduleName }}</span>
                  <span class="text-ink truncate flex-1">{{ frame.demangledSymbol || frame.mangledSymbol }}</span>
                  <span v-if="frame.isCulprit" class="text-danger font-bold shrink-0">责任帧</span>
                </div>
                <div class="text-ink-muted pl-8 truncate">
                  {{ frame.sourceFile || "未知文件" }}:{{ frame.lineNumber || "?" }}
                  <span v-if="frame.blameAuthor"> · {{ frame.blameAuthor }}</span>
                </div>
              </div>
            </div>
            <p class="text-[10px] text-ink-muted">{{ crashResult.summary }}</p>
          </template>
        </div>

        <!-- Section I: Hotfix manifest -->
        <div class="space-y-3 pt-4" data-tool-section="hotfix">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Rocket class="w-4 h-4 text-accent" />
              <span>热更清单生成 (Hotfix Manifest)</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              对比基线引用与目标引用，按代码 / 配置 / 资源分层生成补丁清单、压缩体积与 manifest JSON。
            </p>
          </div>
          <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
            <div class="grid grid-cols-1 md:grid-cols-4 gap-2">
              <input
                v-model="hotfixBaseRef"
                type="text"
                placeholder="基线引用（默认最近 Tag）"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <input
                v-model="hotfixTargetRef"
                type="text"
                placeholder="目标引用（默认 HEAD）"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <input
                v-model="hotfixAppVersion"
                type="text"
                placeholder="应用版本，例如 1.2.0"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <button
                @click="handleGenerateHotfix"
                :disabled="isGeneratingHotfix"
                class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                {{ isGeneratingHotfix ? "生成中..." : "生成热更清单" }}
              </button>
            </div>
          </div>

          <template v-if="hotfixResult">
            <div class="grid grid-cols-4 gap-2">
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">变更文件</div>
                <div class="text-sm font-bold font-mono text-ink">{{ hotfixResult.totalFilesCount }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">补丁体积</div>
                <div class="text-sm font-bold font-mono text-warning">{{ hotfixResult.totalPatchSizeStr }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">压缩后</div>
                <div class="text-sm font-bold font-mono text-success">{{ hotfixResult.totalCompressedSizeStr }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">需要整包</div>
                <div class="text-sm font-bold" :class="hotfixResult.fullPackRequired ? 'text-danger' : 'text-success'">
                  {{ hotfixResult.fullPackRequired ? "是" : "否" }}
                </div>
              </div>
            </div>
            <div v-if="hotfixResult.fullPackRequired" class="p-2.5 rounded-lg border border-danger/30 bg-danger/5 text-[11px] text-danger">
              需要整包更新：{{ hotfixResult.fullPackReason }}
            </div>
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
              <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-1">
                <div class="text-xs font-bold text-ink">分层统计</div>
                <div
                  v-for="tier in hotfixResult.tierBreakdown || []"
                  :key="tier.tier"
                  class="flex items-center gap-2 text-[10px] font-mono"
                >
                  <span class="text-ink flex-1 truncate">{{ tier.tier }}</span>
                  <span class="text-ink-muted shrink-0">{{ tier.count }} 文件</span>
                  <span class="text-ink-muted shrink-0">{{ tier.totalSizeStr }}</span>
                </div>
                <div v-if="(hotfixResult.deltaPackageList || []).length > 0" class="pt-1 text-[10px] text-ink-muted">
                  增量包 {{ hotfixResult.deltaPackageList.length }} 项
                </div>
              </div>
              <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-ink">manifest JSON</span>
                  <button
                    @click="copyHotfixManifest"
                    class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[10px] text-accent transition cursor-pointer"
                  >
                    复制
                  </button>
                </div>
                <pre class="max-h-52 overflow-auto p-2 rounded bg-canvas border border-line/60 text-[10px] font-mono whitespace-pre-wrap break-all">{{ hotfixResult.manifestJson }}</pre>
              </div>
            </div>
            <p class="text-[10px] text-ink-muted">{{ hotfixResult.summary }}</p>
          </template>
        </div>
      </div>
</template>
