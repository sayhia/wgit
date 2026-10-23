<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch, onMounted, watchEffect } from "vue";
import {
  Trash2,
  RefreshCw,
  ShieldCheck,
  FileCode,
  FolderPlus,
  FileWarning,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import GitignoreEditor from "../GitignoreEditor.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, SecretFinding, IgnoreSanitizerResult, CheckIgnoreResult, LfsConvertResult } from "../../types";

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
// 2. WORKSPACE CLEAN & HOUSEKEEPING
// -------------------------------------------------------------
const cleanDryRunItems = ref<string[]>([]);
const isLoadingClean = ref(false);
const isCleaning = ref(false);
const secretFindings = ref<SecretFinding[]>([]);
const secretsScanned = ref(false);
const isScanningSecrets = ref(false);

const stagedLargeFiles = ref<LargeFileInfo[]>([]);
const stagedLargeThresholdMB = ref(20);
const stagedLargeScanned = ref(false);
const isCheckingStagedLarge = ref(false);
const isConvertingStaged = ref(false);
const lfsConvertResult = ref<LfsConvertResult | null>(null);

// .gitignore audit & repair
const ignoreAudit = ref<IgnoreSanitizerResult | null>(null);
const isLoadingIgnoreAudit = ref(false);
const ignorePatternInput = ref("");
const showGitignoreEditor = ref(false);
const ignoreCheckResult = ref<CheckIgnoreResult | null>(null);
const ignoreTemplateType = ref("go");
const untrackIgnoredArtifacts = ref(true);
const isApplyingIgnoreFix = ref(false);
const isAddingIgnorePattern = ref(false);
const isGeneratingIgnoreTemplate = ref(false);
const isCheckingIgnoreRule = ref(false);

// Empty directories & .gitkeep
const emptyDirs = ref<string[]>([]);
const selectedEmptyDirs = ref<Set<string>>(new Set());
const isEmptyDirsScanned = ref(false);
const isScanningEmptyDirs = ref(false);
const isGeneratingGitkeeps = ref(false);

async function scanEmptyDirs() {
  if (!props.currentRepoPath || isScanningEmptyDirs.value) return;
  isScanningEmptyDirs.value = true;
  try {
    emptyDirs.value = (await gitService.scanEmptyDirectories(props.currentRepoPath)) || [];
    selectedEmptyDirs.value = new Set();
    isEmptyDirsScanned.value = true;
    notify(`空目录扫描完成：发现 ${emptyDirs.value.length} 个空目录`);
  } catch (err: any) {
    notify(`空目录扫描失败: ${err?.message || err}`);
  } finally {
    isScanningEmptyDirs.value = false;
  }
}

function toggleEmptyDir(dir: string) {
  const next = new Set(selectedEmptyDirs.value);
  if (next.has(dir)) next.delete(dir);
  else next.add(dir);
  selectedEmptyDirs.value = next;
}

function toggleAllEmptyDirs() {
  selectedEmptyDirs.value =
    selectedEmptyDirs.value.size === emptyDirs.value.length
      ? new Set()
      : new Set(emptyDirs.value);
}

async function handleGenerateGitkeeps() {
  if (!props.currentRepoPath || isGeneratingGitkeeps.value) return;
  const dirs = [...selectedEmptyDirs.value];
  if (dirs.length === 0) {
    notify("请先勾选需要保留的空目录");
    return;
  }
  if (!(await confirmAction(`确定在 ${dirs.length} 个空目录中生成 .gitkeep 并暂存吗？`))) return;
  isGeneratingGitkeeps.value = true;
  try {
    const count = await gitService.generateGitkeeps(props.currentRepoPath, dirs);
    notify(`已生成并暂存 ${count} 个 .gitkeep`);
    await scanEmptyDirs();
    emit("refresh");
  } catch (err: any) {
    notify(`生成 .gitkeep 失败: ${err?.message || err}`);
    // Partial writes may have created files before staging failed.
    await scanEmptyDirs();
    emit("refresh");
  } finally {
    isGeneratingGitkeeps.value = false;
  }
}

async function loadIgnoreAudit(announce = false) {
  if (!props.currentRepoPath) return;
  isLoadingIgnoreAudit.value = true;
  try {
    ignoreAudit.value = await gitService.auditIgnoreRules(props.currentRepoPath);
    if (announce && ignoreAudit.value) {
      notify(
        `忽略规则体检完成：健康度 ${ignoreAudit.value.healthScore} 分，缺失 ${ignoreAudit.value.missingRules.length} 条规则`
      );
    }
  } catch (err: any) {
    if (announce) notify(`忽略规则体检失败: ${err?.message || err}`);
    else console.error("Failed to audit ignore rules:", err);
  } finally {
    isLoadingIgnoreAudit.value = false;
  }
}

async function handleCheckIgnorePath() {
  if (!props.currentRepoPath || !ignorePatternInput.value.trim()) return;
  isCheckingIgnoreRule.value = true;
  try {
    const res = await gitService.checkIgnoreRule(
      props.currentRepoPath,
      ignorePatternInput.value.trim()
    );
    ignoreCheckResult.value = res;
    if (res?.isIgnored) {
      notify(`该路径已被忽略：${res.sourceFile}:${res.lineNumber} (${res.pattern})`);
    } else {
      notify("该路径当前没有被任何忽略规则匹配");
    }
  } catch (err: any) {
    notify(`忽略规则检查失败: ${err?.message || err}`);
  } finally {
    isCheckingIgnoreRule.value = false;
  }
}

async function handleAddIgnorePattern() {
  const pattern = ignorePatternInput.value.trim();
  if (!props.currentRepoPath || !pattern) return;
  isAddingIgnorePattern.value = true;
  try {
    await gitService.addToGitignore(props.currentRepoPath, pattern);
    notify(`已追加到 .gitignore: ${pattern}`);
    ignorePatternInput.value = "";
    ignoreCheckResult.value = null;
    await loadIgnoreAudit();
    emit("refresh");
  } catch (err: any) {
    notify(`写入 .gitignore 失败: ${err?.message || err}`);
  } finally {
    isAddingIgnorePattern.value = false;
  }
}

async function handleGenerateIgnoreTemplate() {
  if (!props.currentRepoPath || isGeneratingIgnoreTemplate.value) return;
  if (!(await confirmAction({ message: `确定向 .gitignore 追加「${ignoreTemplateType.value}」模板规则吗？已有规则不会删除。`, tone: "warning" }))) return;
  isGeneratingIgnoreTemplate.value = true;
  try {
    await gitService.generateGitignoreTemplate(props.currentRepoPath, ignoreTemplateType.value);
    notify(`已追加 ${ignoreTemplateType.value} 忽略模板`);
    await loadIgnoreAudit();
    emit("refresh");
  } catch (err: any) {
    notify(`追加忽略模板失败: ${err?.message || err}`);
  } finally {
    isGeneratingIgnoreTemplate.value = false;
  }
}

async function handleApplyIgnoreFix() {
  if (!props.currentRepoPath || !ignoreAudit.value || isApplyingIgnoreFix.value) return;
  const engine = ignoreAudit.value.engineType || "";
  const count = ignoreAudit.value.trackedArtifacts.length;
  const suffix = untrackIgnoredArtifacts.value
    ? `并取消跟踪 ${count} 个已跟踪的构建产物`
    : "（仅补全规则，不改动已跟踪文件）";
  if (!(await confirmAction(`确定一键修复忽略规则吗？将补全 ${ignoreAudit.value.missingRules.length} 条规则${suffix}。`))) return;
  isApplyingIgnoreFix.value = true;
  try {
    const res = await gitService.applyIgnoreSanitizer(
      props.currentRepoPath,
      engine,
      untrackIgnoredArtifacts.value
    );
    notify(
      res
        ? `忽略规则修复完成：新增 ${res.rulesAdded.length} 条规则，取消跟踪 ${res.untrackedFiles.length} 个文件`
        : "忽略规则修复完成"
    );
    await loadIgnoreAudit();
    await scanCleanableFiles();
    emit("refresh");
  } catch (err: any) {
    notify(`忽略规则修复失败: ${err?.message || err}`);
  } finally {
    isApplyingIgnoreFix.value = false;
  }
}

async function handleCheckStagedLarge() {
  if (!props.currentRepoPath) return;
  isCheckingStagedLarge.value = true;
  try {
    const list = await gitService.checkStagedNonLfsLargeFiles(
      props.currentRepoPath,
      stagedLargeThresholdMB.value
    );
    stagedLargeFiles.value = list || [];
    stagedLargeScanned.value = true;
    notify(
      stagedLargeFiles.value.length > 0
        ? `发现 ${stagedLargeFiles.value.length} 个超过 ${stagedLargeThresholdMB.value} MB 且未使用 LFS 的暂存文件`
        : `暂存区没有超过 ${stagedLargeThresholdMB.value} MB 的非 LFS 大文件`
    );
  } catch (err: any) {
    notify(`暂存区大文件检查失败: ${err?.message || err}`);
  } finally {
    isCheckingStagedLarge.value = false;
  }
}

// One-click remediation for the staged-large-file guard: rewrite the staged
// blobs as LFS pointers without touching the working tree.
async function handleConvertStagedToLfs() {
  if (!props.currentRepoPath || stagedLargeFiles.value.length === 0 || isConvertingStaged.value) return;
  const paths = stagedLargeFiles.value.map((f) => f.path);
  const confirmed = await confirmAction(
    { message: `将 ${paths.length} 个文件转为 Git LFS 跟踪？\n\n` +
      `· 会新增对应的 .gitattributes 规则\n` +
      `· 暂存区中的对象会重写为 LFS 指针\n` +
      `· 工作区文件本身不受影响`, tone: "danger" }
  );
  if (!confirmed) return;

  isConvertingStaged.value = true;
  try {
    const res = await gitService.trackStagedLargeFilesAsLfs(props.currentRepoPath, paths, true);
    lfsConvertResult.value = res;
    notify(res.message);
    await handleCheckStagedLarge();
    emit("refresh");
  } catch (err: any) {
    notify(`转为 LFS 失败: ${err?.message || err}`);
  } finally {
    isConvertingStaged.value = false;
  }
}

async function handleScanSecrets() {  if (!props.currentRepoPath) return;
  isScanningSecrets.value = true;
  try {
    const list = await gitService.scanStagedSecrets(props.currentRepoPath);
    secretFindings.value = list || [];
    secretsScanned.value = true;
    notify(
      secretFindings.value.length > 0
        ? `发现 ${secretFindings.value.length} 处潜在敏感信息`
        : "暂存区未发现敏感信息"
    );
  } catch (err: any) {
    notify(`敏感信息扫描失败: ${err?.message || err}`);
  } finally {
    isScanningSecrets.value = false;
  }
}

async function scanCleanableFiles() {
  if (!props.currentRepoPath) return;
  isLoadingClean.value = true;
  try {
    const list = await gitService.cleanWorkingTree(props.currentRepoPath, true);
    cleanDryRunItems.value = list || [];
  } catch (err) {
    console.error("Failed to scan cleanable files:", err);
  } finally {
    isLoadingClean.value = false;
  }
}

async function handleDiscardUntracked(path: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: `确定仅删除未跟踪文件「${path}」吗？此操作无法撤销。`, tone: "danger" }))) return;
  try {
    await gitService.discardUntrackedFile(props.currentRepoPath, path);
    notify(`已删除未跟踪文件: ${path}`);
    await scanCleanableFiles();
    emit("refresh");
  } catch (err: any) {
    notify(`删除未跟踪文件失败: ${err?.message || err}`);
  }
}

async function handleExecuteClean() {
  if (!props.currentRepoPath) return;
  if (cleanDryRunItems.value.length === 0) return;
  if (!(await confirmAction({ message: `⚠️ 警告: 确定要彻底删除扫描出的 ${cleanDryRunItems.value.length} 个未跟踪孤立文件吗？此操作无法撤销！`, tone: "danger" }))) return;
  isCleaning.value = true;
  try {
    await gitService.cleanWorkingTree(props.currentRepoPath, false);
    notify(`已彻底清理 ${cleanDryRunItems.value.length} 个未跟踪垃圾文件`);
    emit("refresh");
    await scanCleanableFiles();
  } catch (err: any) {
    notify(`清理失败: ${err?.message || err}`);
  } finally {
    isCleaning.value = false;
  }
}


watchEffect(() => {
  toolsStatus.loading.clean =
    isLoadingClean.value ||
    isCleaning.value ||
    isScanningSecrets.value ||
    isCheckingStagedLarge.value ||
    isLoadingIgnoreAudit.value ||
    isApplyingIgnoreFix.value ||
    isAddingIgnorePattern.value ||
    isGeneratingIgnoreTemplate.value ||
    isScanningEmptyDirs.value ||
    isGeneratingGitkeeps.value ||
    isCheckingIgnoreRule.value;
});

watchEffect(() => {
  toolsStatus.counts.clean = cleanDryRunItems.value.length;
});

onMounted(() => {
  void scanCleanableFiles();
  void loadIgnoreAudit();
});

watch(
  () => props.refreshNonce,
  () => {
    void scanCleanableFiles();
    void loadIgnoreAudit();
  }
);
</script>

<template>
      <!-- ============================================== -->
      <!-- TAB 2: 工作区清理 (Clean & Housekeeping)        -->
      <!-- ============================================== -->
      <div class="space-y-4">
        <div class="flex items-center justify-between pb-3 border-b border-line">
          <div>
            <h3 class="text-sm font-bold text-ink">工作区孤立文件扫描与一键清理 (git clean)</h3>
            <p class="text-xs text-ink-muted mt-0.5">
              安全扫描未加入版本控制的垃圾文件、临时文件、macOS .DS_Store 及未跟踪产物。
            </p>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="scanCleanableFiles"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
            >
              重新扫描
            </button>
            <button
              @click="handleExecuteClean"
              :disabled="cleanDryRunItems.length === 0 || isCleaning"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-danger-strong hover:bg-danger-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shadow-xs"
            >
              <Trash2 class="w-3.5 h-3.5" />
              <span>{{ isCleaning ? "正在清理..." : `彻底清理 (${cleanDryRunItems.length})` }}</span>
            </button>
          </div>
        </div>

        <!-- Clean Items List -->
        <div class="border border-line rounded-xl bg-surface/30 p-4">
          <div v-if="cleanDryRunItems.length > 0" class="space-y-1.5">
            <div
              v-for="f in cleanDryRunItems"
              :key="f"
              class="flex items-center justify-between p-2 rounded-lg bg-canvas/60 border border-line/40 text-xs font-mono"
            >
              <div class="flex items-center gap-2 truncate">
                <FileCode class="w-3.5 h-3.5 text-warning shrink-0" />
                <span class="truncate text-ink font-medium">{{ f }}</span>
              </div>
              <div class="flex items-center gap-2 shrink-0">
                <span class="text-[10px] text-warning bg-warning/15 px-2 py-0.5 rounded font-sans font-bold">
                  未跟踪 (Untracked)
                </span>
                <button aria-label="仅删除此未跟踪文件"
                  @click="handleDiscardUntracked(f)"
                  class="p-1 rounded hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                  title="仅删除此未跟踪文件"
                >
                  <Trash2 class="w-3 h-3" />
                </button>
              </div>
            </div>
          </div>

          <div v-else class="py-12 text-center text-xs text-ink-muted flex flex-col items-center gap-2">
            <ShieldCheck class="w-8 h-8 text-success" />
            <span class="font-bold text-ink">工作区干干净净</span>
            <span>当前没有检测到任何孤立或多余的未跟踪垃圾文件。</span>
          </div>
        </div>

        <!-- Staged secret scan -->
        <div class="border border-line rounded-xl bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <ShieldCheck class="w-4 h-4 text-accent" />
                <span>暂存区敏感信息扫描</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                检查已暂存文件中的访问密钥、私钥、令牌等敏感信息，避免误提交。
              </p>
            </div>
            <button
              @click="handleScanSecrets"
              :disabled="isScanningSecrets"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isScanningSecrets ? "扫描中..." : "开始扫描" }}
            </button>
          </div>

          <div v-if="secretFindings.length > 0" class="space-y-1.5">
            <div
              v-for="f in secretFindings"
              :key="`${f.filePath}:${f.lineNumber}:${f.ruleName}`"
              class="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg bg-danger/10 border border-danger/25 text-[11px]"
            >
              <div class="flex items-center gap-2 min-w-0">
                <span class="px-1.5 py-0.5 rounded bg-danger-strong text-white text-[9px] font-bold shrink-0">{{ f.severity }}</span>
                <span class="font-mono text-ink truncate">{{ f.filePath }}:{{ f.lineNumber }}</span>
                <span class="text-ink-muted truncate">{{ f.ruleName }}</span>
              </div>
              <code class="text-[10px] text-danger/80 font-mono shrink-0">{{ f.redactedSnippet }}</code>
            </div>
          </div>
          <div v-else-if="secretsScanned" class="py-3 text-center text-xs text-success">
            未在暂存区发现敏感信息
          </div>
        </div>

        <!-- Staged large file guard -->
        <div class="border border-line rounded-xl bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <FileWarning class="w-4 h-4 text-warning" />
                <span>暂存区大文件检查</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                检查已暂存但未走 Git LFS 的大文件，避免仓库体积膨胀。
              </p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <label class="flex items-center gap-1 text-[11px] text-ink-muted">
                阈值
                <input
                  v-model.number="stagedLargeThresholdMB"
                  type="number"
                  min="1"
                  max="500"
                  class="w-14 px-1.5 py-1 rounded-md bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
                />
                MB
              </label>
              <button
                @click="handleCheckStagedLarge"
                :disabled="isCheckingStagedLarge"
                class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              >
                {{ isCheckingStagedLarge ? "检查中..." : "开始检查" }}
              </button>
            </div>
          </div>

          <div v-if="stagedLargeFiles.length > 0" class="space-y-1.5">
            <div
              v-for="f in stagedLargeFiles"
              :key="f.path"
              class="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg bg-warning/10 border border-warning/25 text-[11px]"
            >
              <div class="flex items-center gap-2 min-w-0">
                <span class="px-1.5 py-0.5 rounded bg-warning-strong text-warning-contrast text-[9px] font-bold shrink-0">LARGE</span>
                <span class="font-mono text-ink truncate" :title="f.path">{{ f.path }}</span>
              </div>
              <span class="font-mono text-warning shrink-0">{{ f.sizeHuman }}</span>
            </div>

            <div class="flex items-center justify-between gap-3 pt-1">
              <p class="text-[11px] text-ink-muted leading-relaxed">
                转换只重写暂存区对象，工作区文件保持原样；提交后远端只收指针。
              </p>
              <button
                @click="handleConvertStagedToLfs"
                :disabled="isConvertingStaged"
                class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-inverse text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
              >
                {{ isConvertingStaged ? "转换中..." : `全部转为 LFS (${stagedLargeFiles.length})` }}
              </button>
            </div>
          </div>
          <div v-else-if="stagedLargeScanned" class="py-3 text-center text-xs text-success">
            暂存区没有超过阈值的非 LFS 大文件
          </div>

          <div
            v-if="lfsConvertResult"
            class="rounded-lg border p-2.5 space-y-1 text-[11px]"
            :class="lfsConvertResult.failed > 0 ? 'border-warning/30 bg-warning/10' : 'border-success/30 bg-success/10'"
          >
            <div class="font-semibold text-ink">{{ lfsConvertResult.message }}</div>
            <div
              v-for="item in lfsConvertResult.items"
              :key="item.path"
              class="flex items-start gap-2 font-mono text-[10px]"
            >
              <span
                class="shrink-0 px-1 rounded font-bold"
                :class="item.status === 'converted' ? 'bg-success-strong text-success-contrast' : 'bg-warning-strong text-warning-contrast'"
              >{{ item.status === "converted" ? "OK" : "FAIL" }}</span>
              <span class="text-ink-muted break-all">{{ item.path }}</span>
              <span v-if="item.detail" class="text-ink-muted/70 shrink-0">{{ item.detail }}</span>
            </div>
            <div v-if="lfsConvertResult.addedPatterns.length" class="text-ink-muted">
              新增规则：<span class="font-mono text-ink">{{ lfsConvertResult.addedPatterns.join("  ") }}</span>
            </div>
          </div>
        </div>

        <!-- .gitignore audit & repair -->
        <div class="border border-line rounded-xl bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <ShieldCheck class="w-4 h-4 text-success" />
                <span>忽略规则体检与一键修复 (.gitignore)</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                检测引擎构建产物是否被误跟踪，补全缺失规则，并可按路径检查当前忽略来源。
              </p>
            </div>
            <button
              @click="loadIgnoreAudit(true)"
              :disabled="isLoadingIgnoreAudit"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isLoadingIgnoreAudit ? "体检中..." : "重新体检" }}
            </button>
          </div>

          <div v-if="ignoreAudit" class="grid grid-cols-4 gap-2">
            <div class="p-2.5 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">识别引擎</div>
              <div class="text-xs font-bold text-ink truncate">{{ ignoreAudit.engineType || "通用工程" }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">健康度</div>
              <div
                class="text-sm font-bold font-mono"
                :class="ignoreAudit.healthScore >= 85 ? 'text-success' : ignoreAudit.healthScore >= 60 ? 'text-warning' : 'text-danger'"
              >
                {{ ignoreAudit.healthScore }} / 100
              </div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">缺失规则</div>
              <div class="text-sm font-bold font-mono text-warning">{{ ignoreAudit.missingRules.length }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">误跟踪产物</div>
              <div class="text-sm font-bold font-mono" :class="ignoreAudit.trackedCount > 0 ? 'text-danger' : 'text-success'">
                {{ ignoreAudit.trackedCount }}
              </div>
            </div>
          </div>

          <div v-if="ignoreAudit && ignoreAudit.missingRules.length > 0" class="flex flex-wrap gap-1.5">
            <span
              v-for="rule in ignoreAudit.missingRules"
              :key="rule"
              class="px-1.5 py-0.5 rounded bg-warning/15 text-warning font-mono text-[10px]"
            >
              {{ rule }}
            </span>
          </div>

          <div
            v-if="ignoreAudit && ignoreAudit.trackedArtifacts.length > 0"
            class="max-h-36 overflow-y-auto space-y-1 border border-danger/20 rounded-lg p-2 bg-danger/5"
          >
            <div
              v-for="artifact in ignoreAudit.trackedArtifacts"
              :key="artifact.path"
              class="flex items-center justify-between gap-2 text-[11px]"
            >
              <div class="flex items-center gap-2 min-w-0">
                <span class="px-1.5 py-0.5 rounded bg-danger/20 text-danger text-[9px] font-bold shrink-0">
                  {{ artifact.category }}
                </span>
                <span class="font-mono text-ink truncate" :title="artifact.path">{{ artifact.path }}</span>
              </div>
              <span class="font-mono text-ink-muted shrink-0">{{ artifact.sizeHuman }}</span>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <input
              v-model="ignorePatternInput"
              type="text"
              placeholder="输入路径或忽略模式，例如 Library/ 或 Assets/Temp/"
              @keydown.enter="handleCheckIgnorePath"
              class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <button
              @click="handleCheckIgnorePath"
              :disabled="!ignorePatternInput.trim() || isCheckingIgnoreRule"
              class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isCheckingIgnoreRule ? "检查中..." : "检查是否忽略" }}
            </button>
            <button
              @click="handleAddIgnorePattern"
              :disabled="!ignorePatternInput.trim() || isAddingIgnorePattern"
              class="px-2.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isAddingIgnorePattern ? "写入中..." : "加入忽略" }}
            </button>
            <button
              @click="showGitignoreEditor = true"
              class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink-secondary hover:text-ink transition cursor-pointer shrink-0"
              title="管理 .gitignore 规则（查看 / 追加 / 移除）"
            >
              管理规则
            </button>
          </div>

          <div v-if="ignoreCheckResult" class="text-[11px] font-mono" :class="ignoreCheckResult.isIgnored ? 'text-success' : 'text-ink-muted'">
            {{ ignoreCheckResult.targetFile }}:
            {{ ignoreCheckResult.isIgnored ? `已被 ${ignoreCheckResult.sourceFile}:${ignoreCheckResult.lineNumber} (${ignoreCheckResult.pattern}) 忽略` : "未被任何规则忽略" }}
          </div>

          <div class="flex items-center justify-between gap-3 pt-1">
            <div class="flex items-center gap-2">
              <select
                v-model="ignoreTemplateType"
                aria-label="忽略规则模板类型"
                class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
              >
                <option value="go">Go</option>
                <option value="node">Node</option>
                <option value="unity">Unity</option>
                <option value="unreal">Unreal</option>
                <option value="python">Python</option>
              </select>
              <button
                @click="handleGenerateIgnoreTemplate"
                :disabled="isGeneratingIgnoreTemplate"
                class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              >
                {{ isGeneratingIgnoreTemplate ? "追加中..." : "追加模板规则" }}
              </button>
            </div>

            <div class="flex items-center gap-3">
              <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
                <input v-model="untrackIgnoredArtifacts" type="checkbox" class="rounded accent-danger" />
                <span>同时取消跟踪已误提交产物</span>
              </label>
              <button
                @click="handleApplyIgnoreFix"
                :disabled="!ignoreAudit || isApplyingIgnoreFix"
                class="px-3 py-1.5 rounded-lg text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shadow-xs"
                :class="ignoreAudit && ignoreAudit.trackedCount > 0 ? 'bg-danger-strong hover:bg-danger-strong-hover' : 'bg-success-strong hover:opacity-90'"
              >
                {{ isApplyingIgnoreFix ? "修复中..." : "一键修复忽略规则" }}
              </button>
            </div>
          </div>
        </div>

        <!-- Empty directories & .gitkeep -->
        <div class="border border-line rounded-xl bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <FolderPlus class="w-4 h-4 text-accent" />
                <span>空目录扫描与 .gitkeep 保留</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                Git 不跟踪空目录；勾选需要保留的目录后生成 .gitkeep 并自动暂存。
              </p>
            </div>
            <div class="flex items-center gap-2">
              <button
                @click="scanEmptyDirs"
                :disabled="isScanningEmptyDirs"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              >
                <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isScanningEmptyDirs }" />
                <span>{{ isScanningEmptyDirs ? "扫描中..." : "扫描空目录" }}</span>
              </button>
              <button
                @click="handleGenerateGitkeeps"
                :disabled="selectedEmptyDirs.size === 0 || isGeneratingGitkeeps"
                class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                {{ isGeneratingGitkeeps ? "生成中..." : `生成 .gitkeep (${selectedEmptyDirs.size})` }}
              </button>
            </div>
          </div>

          <div v-if="emptyDirs.length > 0" class="space-y-2">
            <button
              @click="toggleAllEmptyDirs"
              class="text-[11px] text-accent hover:underline cursor-pointer"
            >
              {{ selectedEmptyDirs.size === emptyDirs.length ? "取消全选" : "全选" }}
            </button>
            <div class="max-h-52 overflow-y-auto grid grid-cols-1 sm:grid-cols-2 gap-1">
              <label
                v-for="dir in emptyDirs"
                :key="dir"
                class="flex items-center gap-2 px-2 py-1 rounded bg-canvas/40 border border-line text-[11px] font-mono cursor-pointer hover:bg-surface/60"
              >
                <input
                  type="checkbox"
                  :checked="selectedEmptyDirs.has(dir)"
                  @change="toggleEmptyDir(dir)"
                  class="rounded accent-accent scale-90"
                />
                <span class="text-ink truncate" :title="dir">{{ dir }}</span>
              </label>
            </div>
          </div>
          <p v-else-if="isEmptyDirsScanned" class="text-[11px] text-success">
            未发现需要保留的空目录
          </p>
          <p v-else class="text-[11px] text-ink-muted">尚未扫描空目录。</p>
        </div>
      </div>


      <GitignoreEditor
        :is-open="showGitignoreEditor"
        :repo-path="currentRepoPath"
        @close="showGitignoreEditor = false"
        @notify="notify"
        @refresh="emit('refresh')"
      />
</template>
