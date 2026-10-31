<script setup lang="ts">
import { reportError } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import { t } from "../../i18n";
import { ref, watch, onMounted, watchEffect } from "vue";
import {
  Trash2,
  RefreshCw,
  ShieldCheck,
  ShieldAlert,
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
const isRemediatingSecrets = ref(false);

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
    notify(t("clean.toast.empty.dirs.scanned", { count: emptyDirs.value.length }));
  } catch (err: any) {
    notify(t("clean.toast.empty.dirs.failed", { error: err?.message || err }));
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
    notify(t("clean.toast.keep.selection.required"));
    return;
  }
  if (!(await confirmAction(t("clean.confirm.gitkeep", { count: dirs.length })))) return;
  isGeneratingGitkeeps.value = true;
  try {
    const count = await gitService.generateGitkeeps(props.currentRepoPath, dirs);
    notify(t("clean.toast.gitkeep.created", { count }));
    await scanEmptyDirs();
    emit("refresh");
  } catch (err: any) {
    notify(t("clean.toast.gitkeep.failed", { error: err?.message || err }));
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
        t("clean.toast.audit.done", {
          score: ignoreAudit.value.healthScore,
          count: ignoreAudit.value.missingRules.length,
        })
      );
    }
  } catch (err: any) {
    if (announce) notify(t("clean.toast.audit.failed", { error: err?.message || err }));
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
      notify(t("clean.toast.ignore.hit", { file: res.sourceFile, line: res.lineNumber, pattern: res.pattern }));
    } else {
      notify(t("clean.toast.ignore.miss"));
    }
  } catch (err: any) {
    notify(t("clean.toast.ignore.check.failed", { error: err?.message || err }));
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
    notify(t("clean.toast.pattern.added", { pattern }));
    ignorePatternInput.value = "";
    ignoreCheckResult.value = null;
    await loadIgnoreAudit();
    emit("refresh");
  } catch (err: any) {
    notify(t("clean.toast.pattern.add.failed", { error: err?.message || err }));
  } finally {
    isAddingIgnorePattern.value = false;
  }
}

async function handleGenerateIgnoreTemplate() {
  if (!props.currentRepoPath || isGeneratingIgnoreTemplate.value) return;
  if (!(await confirmAction({ message: t("clean.confirm.template", { type: ignoreTemplateType.value }), tone: "warning" }))) return;
  isGeneratingIgnoreTemplate.value = true;
  try {
    await gitService.generateGitignoreTemplate(props.currentRepoPath, ignoreTemplateType.value);
    notify(t("clean.toast.template.added", { type: ignoreTemplateType.value }));
    await loadIgnoreAudit();
    emit("refresh");
  } catch (err: any) {
    notify(t("clean.toast.template.failed", { error: err?.message || err }));
  } finally {
    isGeneratingIgnoreTemplate.value = false;
  }
}

async function handleApplyIgnoreFix() {
  if (!props.currentRepoPath || !ignoreAudit.value || isApplyingIgnoreFix.value) return;
  const engine = ignoreAudit.value.engineType || "";
  const count = ignoreAudit.value.trackedArtifacts.length;
  const suffix = untrackIgnoredArtifacts.value
    ? t("clean.fix.suffix.untrack", { count })
    : t("clean.fix.suffix.rules.only");
  if (!(await confirmAction(t("clean.confirm.fix", { count: ignoreAudit.value.missingRules.length, suffix })))) return;
  isApplyingIgnoreFix.value = true;
  try {
    const res = await gitService.applyIgnoreSanitizer(
      props.currentRepoPath,
      engine,
      untrackIgnoredArtifacts.value
    );
    const failed = res?.failedFiles?.length ?? 0;
    if (res && failed > 0) {
      notify(
        t("clean.toast.fix.partial", {
          files: res.untrackedFiles.length,
          failed,
        })
      );
    } else {
      notify(
        res
          ? t("clean.toast.fix.done", { rules: res.rulesAdded.length, files: res.untrackedFiles.length })
          : t("clean.toast.fix.done.plain")
      );
    }
    await loadIgnoreAudit();
    await scanCleanableFiles();
    emit("refresh");
  } catch (err: any) {
    notify(t("clean.toast.fix.failed", { error: err?.message || err }));
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
        ? t("clean.toast.staged.large.found", { count: stagedLargeFiles.value.length, size: stagedLargeThresholdMB.value })
        : t("clean.toast.staged.large.none", { size: stagedLargeThresholdMB.value })
    );
  } catch (err: any) {
    notify(t("clean.toast.staged.large.failed", { error: err?.message || err }));
  } finally {
    isCheckingStagedLarge.value = false;
  }
}

// One-click remediation for the staged-large-file guard: rewrite the staged
// blobs as LFS pointers without touching the working tree.
async function handleConvertStagedToLfs() {
  if (!props.currentRepoPath || stagedLargeFiles.value.length === 0 || isConvertingStaged.value) return;
  const paths = stagedLargeFiles.value.map((f) => f.path);
  const confirmed = await confirmAction({
    message: t("clean.confirm.to.lfs", { count: paths.length }),
    tone: "danger",
  });
  if (!confirmed) return;

  isConvertingStaged.value = true;
  try {
    const res = await gitService.trackStagedLargeFilesAsLfs(props.currentRepoPath, paths, true);
    lfsConvertResult.value = res;
    notify(res.message);
    await handleCheckStagedLarge();
    emit("refresh");
  } catch (err: any) {
    notify(t("clean.toast.to.lfs.failed", { error: err?.message || err }));
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
        ? t("clean.toast.secrets.found", { count: secretFindings.value.length })
        : t("clean.toast.secrets.none")
    );
  } catch (err: any) {
    notify(t("clean.toast.secrets.failed", { error: err?.message || err }));
  } finally {
    isScanningSecrets.value = false;
  }
}

async function handleRemediateSecrets() {
  if (!props.currentRepoPath || secretFindings.value.length === 0 || isRemediatingSecrets.value) return;
  const paths = Array.from(new Set(secretFindings.value.map((f) => f.filePath).filter(Boolean)));
  if (paths.length === 0) return;
  const ok = await confirmAction({
    message: t("clean.confirm.secrets.remediate", { count: paths.length }),
    tone: "danger",
  });
  if (!ok) return;
  isRemediatingSecrets.value = true;
  try {
    const res = await gitService.remediateStagedSecrets(props.currentRepoPath, paths, true, false);
    try {
      secretFindings.value = (await gitService.scanStagedSecrets(props.currentRepoPath)) || [];
      secretsScanned.value = true;
    } catch {
      // Keep the previous findings; the remediation result below still reports
      // what actually happened. A manual rescan is always available.
    }
    notify(res?.message || t("clean.toast.secrets.remediated", { count: res?.unstaged ?? paths.length }));
  } catch (err: any) {
    notify(t("clean.toast.secrets.remediate.failed", { error: err?.message || err }));
  } finally {
    isRemediatingSecrets.value = false;
  }
}

async function scanCleanableFiles() {
  if (!props.currentRepoPath) return;
  isLoadingClean.value = true;
  try {
    const list = await gitService.cleanWorkingTree(props.currentRepoPath, true);
    cleanDryRunItems.value = list || [];
  } catch (err) {
    reportError("Failed to scan cleanable files", err);
  } finally {
    isLoadingClean.value = false;
  }
}

async function handleDiscardUntracked(path: string) {
  if (!props.currentRepoPath) return;
  if (!(await confirmAction({ message: t("clean.confirm.delete.one", { path }), tone: "danger" }))) return;
  try {
    await gitService.discardUntrackedFile(props.currentRepoPath, path);
    notify(t("clean.toast.delete.one", { path }));
    await scanCleanableFiles();
    emit("refresh");
  } catch (err: any) {
    notify(t("clean.toast.delete.one.failed", { error: err?.message || err }));
  }
}

async function handleExecuteClean() {
  if (!props.currentRepoPath) return;
  if (cleanDryRunItems.value.length === 0) return;
  if (!(await confirmAction({ message: t("clean.confirm.clean.all", { count: cleanDryRunItems.value.length }), tone: "danger" }))) return;
  isCleaning.value = true;
  try {
    await gitService.cleanWorkingTree(props.currentRepoPath, false);
    notify(t("clean.toast.clean.all", { count: cleanDryRunItems.value.length }));
    emit("refresh");
    await scanCleanableFiles();
  } catch (err: any) {
    notify(t("clean.toast.clean.failed", { error: err?.message || err }));
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
            <h3 class="text-sm font-bold text-ink">{{ t("clean.title") }}</h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("clean.desc") }}
            </p>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="scanCleanableFiles"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
            >
              {{ t("clean.rescan") }}
            </button>
            <button
              @click="handleExecuteClean"
              :disabled="cleanDryRunItems.length === 0 || isCleaning"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-danger-strong hover:bg-danger-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shadow-xs"
            >
              <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
              <span>{{ isCleaning ? t("clean.cleaning") : t("clean.clean.all", { count: cleanDryRunItems.length }) }}</span>
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
                  {{ t("clean.group.untracked") }}
                </span>
                <button :aria-label="t('clean.delete.one')"
                  @click="handleDiscardUntracked(f)"
                  class="p-1 rounded hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                  :title="t('clean.delete.one')"
                >
                  <Trash2 class="w-3 h-3" aria-hidden="true" />
                </button>
              </div>
            </div>
          </div>

          <div v-else class="py-12 text-center text-xs text-ink-muted flex flex-col items-center gap-2">
            <ShieldCheck class="w-8 h-8 text-success" />
            <span class="font-bold text-ink">{{ t("clean.empty.title") }}</span>
            <span>{{ t("clean.empty.desc") }}</span>
          </div>
        </div>

        <!-- Staged secret scan -->
        <div class="border border-line rounded-xl bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <ShieldCheck class="w-4 h-4 text-accent" />
                <span>{{ t("clean.secrets.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("clean.secrets.desc") }}
              </p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <button
                v-if="secretFindings.length > 0"
                @click="handleRemediateSecrets"
                :disabled="isScanningSecrets || isRemediatingSecrets"
                class="px-3 py-1.5 rounded-lg bg-danger/15 hover:bg-danger/25 border border-danger/30 text-xs font-semibold text-danger transition cursor-pointer disabled:opacity-40"
              >
                {{ isRemediatingSecrets ? t("clean.secrets.remediating") : t("clean.secrets.remediate") }}
              </button>
              <button
                @click="handleScanSecrets"
                :disabled="isScanningSecrets"
                class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              >
                {{ isScanningSecrets ? t("clean.secrets.scanning") : t("clean.secrets.scan") }}
              </button>
            </div>
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
            {{ t("clean.secrets.empty") }}
          </div>
        </div>

        <!-- Staged large file guard -->
        <div class="border border-line rounded-xl bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <FileWarning class="w-4 h-4 text-warning" />
                <span>{{ t("clean.staged.large.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("clean.staged.large.desc") }}
              </p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <label class="flex items-center gap-1 text-[11px] text-ink-muted">
                {{ t("clean.staged.large.threshold") }}
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
                {{ isCheckingStagedLarge ? t("clean.staged.large.checking") : t("clean.staged.large.check") }}
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
                {{ t("clean.staged.large.hint") }}
              </p>
              <button
                @click="handleConvertStagedToLfs"
                :disabled="isConvertingStaged"
                class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-inverse text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
              >
                {{ isConvertingStaged ? t("clean.staged.large.converting") : t("clean.staged.large.convert.all", { count: stagedLargeFiles.length }) }}
              </button>
            </div>
          </div>
          <div v-else-if="stagedLargeScanned" class="py-3 text-center text-xs text-success">
            {{ t("clean.staged.large.empty") }}
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
              <span v-if="item.detail" class="text-ink-muted shrink-0">{{ item.detail }}</span>
            </div>
            <div v-if="lfsConvertResult.addedPatterns.length" class="text-ink-muted">
              {{ t("clean.staged.large.added.patterns") }}<span class="font-mono text-ink">{{ lfsConvertResult.addedPatterns.join("  ") }}</span>
            </div>
          </div>
        </div>

        <!-- .gitignore audit & repair -->
        <div class="border border-line rounded-xl bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <ShieldCheck class="w-4 h-4 text-success" />
                <span>{{ t("clean.ignore.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("clean.ignore.desc") }}
              </p>
            </div>
            <button
              @click="loadIgnoreAudit(true)"
              :disabled="isLoadingIgnoreAudit"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isLoadingIgnoreAudit ? t("clean.ignore.auditing") : t("clean.ignore.reaudit") }}
            </button>
          </div>

          <div v-if="ignoreAudit" class="grid grid-cols-4 gap-2">
            <div class="p-2.5 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("clean.ignore.engine") }}</div>
              <div class="text-xs font-bold text-ink truncate">{{ ignoreAudit.engineType || t("clean.ignore.engine.generic") }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("clean.ignore.health") }}</div>
              <div
                class="text-sm font-bold font-mono"
                :class="ignoreAudit.healthScore >= 85 ? 'text-success' : ignoreAudit.healthScore >= 60 ? 'text-warning' : 'text-danger'"
              >
                {{ ignoreAudit.healthScore }} / 100
              </div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("clean.ignore.missing") }}</div>
              <div class="text-sm font-bold font-mono text-warning">{{ ignoreAudit.missingRules.length }}</div>
            </div>
            <div class="p-2.5 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("clean.ignore.tracked.artifacts") }}</div>
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
              :placeholder="t('clean.ignore.path.placeholder')"
              @keydown.enter="handleCheckIgnorePath"
              class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
            />
            <button
              @click="handleCheckIgnorePath"
              :disabled="!ignorePatternInput.trim() || isCheckingIgnoreRule"
              class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isCheckingIgnoreRule ? t("clean.ignore.checking") : t("clean.ignore.check") }}
            </button>
            <button
              @click="handleAddIgnorePattern"
              :disabled="!ignorePatternInput.trim() || isAddingIgnorePattern"
              class="px-2.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isAddingIgnorePattern ? t("clean.ignore.adding") : t("clean.ignore.add") }}
            </button>
            <button
              @click="showGitignoreEditor = true"
              class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink-secondary hover:text-ink transition cursor-pointer shrink-0"
              :title="t('clean.ignore.manage.title')"
            >
              {{ t("clean.ignore.manage") }}
            </button>
          </div>

          <div v-if="ignoreCheckResult" class="text-[11px] font-mono" :class="ignoreCheckResult.isIgnored ? 'text-success' : 'text-ink-muted'">
            {{ ignoreCheckResult.targetFile }}:
            {{ ignoreCheckResult.isIgnored ? t("clean.ignore.hit", { file: ignoreCheckResult.sourceFile, line: ignoreCheckResult.lineNumber, pattern: ignoreCheckResult.pattern }) : t("clean.ignore.miss") }}
          </div>

          <div class="flex items-center justify-between gap-3 pt-1">
            <div class="flex items-center gap-2">
              <select
                v-model="ignoreTemplateType"
                :aria-label="t('clean.ignore.template.type')"
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
                {{ isGeneratingIgnoreTemplate ? t("clean.ignore.template.generating") : t("clean.ignore.template.add") }}
              </button>
            </div>

            <div class="flex items-center gap-3">
              <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
                <input v-model="untrackIgnoredArtifacts" type="checkbox" class="rounded accent-danger" />
                <span>{{ t("clean.ignore.untrack") }}</span>
              </label>
              <button
                @click="handleApplyIgnoreFix"
                :disabled="!ignoreAudit || isApplyingIgnoreFix"
                class="px-3 py-1.5 rounded-lg text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 shadow-xs"
                :class="ignoreAudit && ignoreAudit.trackedCount > 0 ? 'bg-danger-strong hover:bg-danger-strong-hover' : 'bg-success-strong hover:opacity-90'"
              >
                {{ isApplyingIgnoreFix ? t("clean.ignore.fixing") : t("clean.ignore.fix") }}
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
                <span>{{ t("clean.empty.dirs.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("clean.empty.dirs.desc") }}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <button
                @click="scanEmptyDirs"
                :disabled="isScanningEmptyDirs"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
              >
                <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isScanningEmptyDirs }" aria-hidden="true" />
                <span>{{ isScanningEmptyDirs ? t("clean.empty.dirs.scanning") : t("clean.empty.dirs.scan") }}</span>
              </button>
              <button
                @click="handleGenerateGitkeeps"
                :disabled="selectedEmptyDirs.size === 0 || isGeneratingGitkeeps"
                class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                {{ isGeneratingGitkeeps ? t("clean.empty.dirs.generating") : t("clean.empty.dirs.generate", { count: selectedEmptyDirs.size }) }}
              </button>
            </div>
          </div>

          <div v-if="emptyDirs.length > 0" class="space-y-2">
            <button
              @click="toggleAllEmptyDirs"
              class="text-[11px] text-accent hover:underline cursor-pointer"
            >
              {{ selectedEmptyDirs.size === emptyDirs.length ? t("clean.empty.dirs.unselect") : t("clean.empty.dirs.select") }}
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
            {{ t("clean.empty.dirs.empty") }}
          </p>
          <p v-else class="text-[11px] text-ink-muted">{{ t("clean.empty.dirs.not.scanned") }}</p>
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
