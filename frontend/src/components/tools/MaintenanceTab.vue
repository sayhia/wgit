<script setup lang="ts">
import { reportError } from "../../composables/useToast";
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
  Play,
  RotateCcw,
  FileCode,
  Zap,
} from "lucide-vue-next";
import { gitService, commitDetailCache } from "../../services/gitService";
import { backendText, t } from "../../i18n";
import FixCenterSection from "./FixCenterSection.vue";
import GitBundleModal from "./GitBundleModal.vue";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { toolsStatus } from "../../composables/toolsStatus";
import type { ReflogItem, RemoteDetail, WorktreeInfo, LfsLockInfo, LfsTrackPattern, SubmoduleInfo, DiskBreakdownResult, LargeFileInfo, RepoMaintenanceStats, LfsPruneResult, GitFsckResult, GitHookInfo, GitHookRunResult, GitHooksFrameworkInfo, RepoHealthReport, HealthDimension, HealthFixResult, ObjectStoreOptimizeResult, CrashStackDemangleResult, HotfixManifestResult, RepoBloatReport, SlimmingPlanResult, WorkspaceJunkReport, SafeCleanResult } from "../../types";

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

/**
 * Health dimensions ship rendered text plus catalog keys. The detail line may
 * also carry conditional notes (`detailSuffixKeys`) and, for the in-progress
 * workspace state, an operation slug the UI resolves itself.
 */
function healthTitle(item: HealthDimension): string {
  return backendText(item.titleKey, item.title);
}

function healthDetail(item: HealthDimension): string {
  const params = item.params ? { ...item.params } : undefined;
  if (params && params.operation) {
    params.operation = backendText(`health.workspace.op.${params.operation}`, params.operation);
  }
  const base = backendText(item.detailKey, item.detail, params);
  if (!item.detailKey) return base;
  const suffixes = (item.detailSuffixKeys ?? [])
    .map((key) => backendText(key, "", params))
    .join("");
  return base + suffixes;
}

function healthSummary(): string {
  const report = healthReport.value;
  if (!report) return "";
  return backendText(report.summaryKey, report.summary, report.summaryParams);
}
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
        t("maint.toast.demangle.done", {
          done: crashResult.value.demangledCount,
          total: crashResult.value.totalFrames,
        })
      );
    }
  } catch (err: any) {
    notify(t("maint.toast.demangle.failed", { error: err?.message || err }));
  } finally {
    isDemangling.value = false;
  }
}

// -------------------------------------------------------------
// Repo Bloat Inspector & Slimming (Phase 47)
// -------------------------------------------------------------
const bloatReport = ref<RepoBloatReport | null>(null);
const isScanningBloat = ref(false);
const slimmingPlan = ref<SlimmingPlanResult | null>(null);
const isGeneratingPlan = ref(false);

async function handleScanBloat() {
  if (!props.currentRepoPath || isScanningBloat.value) return;
  isScanningBloat.value = true;
  try {
    bloatReport.value = await gitService.inspectRepoBloat(props.currentRepoPath, 15);
  } catch (err: any) {
    reportError("Failed to inspect repo bloat", err);
  } finally {
    isScanningBloat.value = false;
  }
}

async function handleGenerateSlimmingPlan() {
  if (!props.currentRepoPath || !bloatReport.value?.topBloatObjects?.length || isGeneratingPlan.value) return;
  isGeneratingPlan.value = true;
  try {
    const paths = bloatReport.value.topBloatObjects.map((o) => o.path).filter(Boolean);
    slimmingPlan.value = await gitService.generateSlimmingPlan(props.currentRepoPath, paths);
  } catch (err: any) {
    reportError("Failed to generate slimming plan", err);
  } finally {
    isGeneratingPlan.value = false;
  }
}

// -------------------------------------------------------------
// Workspace Hygiene & Safe Cleaner (Phase 54)
// -------------------------------------------------------------
const junkReport = ref<WorkspaceJunkReport | null>(null);
const isScanningJunk = ref(false);
const isCleaningJunk = ref(false);
const selectedJunkCategories = ref<string[]>(["osArtifacts", "logAndTemp"]);
const isAppendingGitIgnore = ref(false);

async function handleScanWorkspaceJunk() {
  if (!props.currentRepoPath || isScanningJunk.value) return;
  isScanningJunk.value = true;
  try {
    junkReport.value = await gitService.scanWorkspaceJunk(props.currentRepoPath, true);
  } catch (err: any) {
    reportError("Failed to scan workspace junk", err);
  } finally {
    isScanningJunk.value = false;
  }
}

async function handleExecuteSafeClean() {
  if (!props.currentRepoPath || selectedJunkCategories.value.length === 0 || isCleaningJunk.value) return;
  const confirmed = await confirmAction({
    message: t("maint.hygiene.clean.confirm", { count: selectedJunkCategories.value.length }),
    tone: "warning",
  });
  if (!confirmed) return;

  isCleaningJunk.value = true;
  try {
    const res = await gitService.executeSafeClean(props.currentRepoPath, selectedJunkCategories.value, false);
    if (res) {
      notify(res.summary);
      await handleScanWorkspaceJunk();
      emit("refresh");
    }
  } catch (err: any) {
    notify(String(err?.message || err));
  } finally {
    isCleaningJunk.value = false;
  }
}

async function handleAppendGitIgnore() {
  if (!props.currentRepoPath || !junkReport.value?.recommendedGitIgnore?.length || isAppendingGitIgnore.value) return;
  isAppendingGitIgnore.value = true;
  try {
    const ok = await gitService.appendRecommendedGitIgnore(props.currentRepoPath, junkReport.value.recommendedGitIgnore);
    if (ok) {
      notify(t("maint.hygiene.ignore.appended"));
      await handleScanWorkspaceJunk();
    }
  } catch (err: any) {
    notify(String(err?.message || err));
  } finally {
    isAppendingGitIgnore.value = false;
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
    notify(t("maint.toast.hotfix.failed", { error: err?.message || err }));
  } finally {
    isGeneratingHotfix.value = false;
  }
}

async function copyToClipboard(text: string) {
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
  notify(t("maint.toast.errors.copied"));
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
  notify(t("maint.toast.hotfix.copied"));
}

async function handleSquashCommits() {
  if (!props.currentRepoPath || isSquashing.value) return;
  if (squashCount.value < 2) {
    notify(t("maint.toast.squash.min"));
    return;
  }
  if (!squashMessage.value.trim()) {
    notify(t("maint.toast.squash.message.required"));
    return;
  }
  if (
    !(await confirmAction(
      t("maint.confirm.squash", { count: squashCount.value })
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
    notify(t("maint.toast.squash.done", { count: squashCount.value }));
    squashMessage.value = "";
    emit("refresh");
  } catch (err: any) {
    notify(t("maint.toast.squash.failed", { error: err?.message || err }));
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
const showBundleModal = ref(false);
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
    notify(t("maint.toast.patch.checked"));
  } catch (err: any) {
    patchContentResult.value = "";
    notify(t("maint.toast.patch.check.failed", { error: err?.message || err }));
  } finally {
    isApplyingPatchContent.value = false;
  }
}

async function handleApplyPatchContent() {
  if (!props.currentRepoPath || !patchContent.value.trim() || isApplyingPatchContent.value) return;
  if (!(await confirmAction(t("maint.confirm.apply.pasted")))) return;
  isApplyingPatchContent.value = true;
  try {
    patchContentResult.value = await gitService.applyPatchContent(
      props.currentRepoPath,
      patchContent.value,
      false
    );
    notify(t("maint.toast.patch.applied"));
    emit("refresh");
  } catch (err: any) {
    notify(t("maint.toast.patch.apply.failed", { error: err?.message || err }));
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
    notify(t("maint.toast.dir.picker.failed"));
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
    notify(t("maint.toast.zip.done", { output }));
  } catch (err: any) {
    notify(t("maint.toast.zip.failed", { error: err?.message || err }));
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
    notify(t("maint.toast.patch.exported", { output }));
  } catch (err: any) {
    notify(t("maint.toast.patch.export.failed", { error: err?.message || err }));
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
    notify(t("maint.toast.bundle.created", { output }));
  } catch (err: any) {
    notify(t("maint.toast.bundle.failed", { error: err?.message || err }));
  } finally {
    archiveBusy.value = "";
  }
}

async function handleVerifyBundle() {
  if (!props.currentRepoPath || !archiveFile.value.trim() || archiveBusy.value) return;
  archiveBusy.value = "verify";
  try {
    const out = await gitService.verifyGitBundle(props.currentRepoPath, archiveFile.value.trim());
    notify(out?.trim() ? t("maint.toast.bundle.verified") : t("maint.toast.bundle.verify.done"));
  } catch (err: any) {
    notify(t("maint.toast.bundle.verify.failed", { error: err?.message || err }));
  } finally {
    archiveBusy.value = "";
  }
}

async function handleApplyPatch() {
  if (!props.currentRepoPath || !archiveFile.value.trim() || archiveBusy.value) return;
  if (!(await confirmAction(t("maint.confirm.apply.patch", { file: archiveFile.value.trim() })))) return;
  archiveBusy.value = "apply";
  try {
    await gitService.applyPatch(props.currentRepoPath, archiveFile.value.trim());
    notify(t("maint.toast.patch.applied"));
    emit("refresh");
  } catch (err: any) {
    notify(t("maint.toast.patch.apply.file.failed", { error: err?.message || err }));
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
    notify(t("maint.toast.hook.reveal.failed", { error: err?.message || err }));
  }
}

const showHookEditor = ref(false);
const hookEditName = ref("");
const hookEditContent = ref("");
const hookEditEnable = ref(true);
const hookEditMeta = ref("");
const isLoadingHook = ref(false);
const isSavingHook = ref(false);

const hooksFramework = ref<GitHooksFrameworkInfo | null>(null);
const hookRunning = ref("");
const isTestingInModal = ref(false);
const showTestTerminal = ref(false);
const modalTestResult = ref<GitHookRunResult | null>(null);
const selectedHookTemplate = ref("");

interface HookTemplate {
  id: string;
  nameKey: string;
  targetHook: string;
  script: string;
}

const HOOK_TEMPLATES: HookTemplate[] = [
  {
    id: "conventional-commits",
    nameKey: "hooks.template.conventional",
    targetHook: "commit-msg",
    script: `#!/bin/sh
# Conventional Commits specification check
commit_msg=$(cat "$1")
pattern="^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\\([a-zA-Z0-9_.-]+\\))?!?: .+"

if ! echo "$commit_msg" | grep -qE "$pattern"; then
  echo "Error: Commit message does not follow Conventional Commits format!" >&2
  echo "Format: <type>(<scope>): <subject>" >&2
  echo "Example: feat(auth): add google sso login" >&2
  exit 1
fi
`,
  },
  {
    id: "secret-scan",
    nameKey: "hooks.template.secrets",
    targetHook: "pre-commit",
    script: `#!/bin/sh
# Sensitive secret leak scan
patterns="AKIA[0-9A-Z]{16}|ghp_[0-9a-zA-Z]{36}|-----BEGIN (RSA |EC )?PRIVATE KEY-----|xox[baprs]-[0-9a-zA-Z]{10,48}"

files=$(git diff --cached --name-only --diff-filter=ACM)
found=0

for file in $files; do
  if [ -f "$file" ]; then
    matches=$(grep -E -n "$patterns" "$file" 2>/dev/null)
    if [ -n "$matches" ]; then
      echo "Error: Potential secret found in $file:" >&2
      echo "$matches" >&2
      found=1
    fi
  fi
done

if [ $found -ne 0 ]; then
  echo "Commit blocked: Sensitive secrets detected. Use --no-verify to bypass if intentional." >&2
  exit 1
fi
`,
  },
  {
    id: "large-files",
    nameKey: "hooks.template.largefiles",
    targetHook: "pre-commit",
    script: `#!/bin/sh
# Large file staging block (>5MB)
max_bytes=5242880 # 5MB

files=$(git diff --cached --name-only --diff-filter=ACM)
blocked=0

for file in $files; do
  if [ -f "$file" ]; then
    size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || wc -c < "$file")
    if [ "$size" -gt "$max_bytes" ]; then
      echo "Error: File $file exceeds 5MB limit ($((size / 1024 / 1024))MB)." >&2
      blocked=1
    fi
  fi
done

if [ $blocked -ne 0 ]; then
  echo "Commit blocked: Please use Git LFS or reduce file size." >&2
  exit 1
fi
`,
  },
  {
    id: "protect-main",
    nameKey: "hooks.template.protectmain",
    targetHook: "pre-push",
    script: `#!/bin/sh
# Main branch force-push protection
protected_branch="main"

while read local_ref local_sha remote_ref remote_sha; do
  if [ "$remote_ref" = "refs/heads/$protected_branch" ]; then
    if [ "$local_sha" = "0000000000000000000000000000000000000000" ]; then
      echo "Error: Deletion of $protected_branch is prohibited." >&2
      exit 1
    fi
    if [ "$remote_sha" != "0000000000000000000000000000000000000000" ]; then
      if ! git merge-base --is-ancestor "$remote_sha" "$local_sha" 2>/dev/null; then
        echo "Error: Force-push to $protected_branch is prohibited!" >&2
        exit 1
      fi
    fi
  fi
done

exit 0
`,
  },
];

function frameworkDisplayName(type: string) {
  switch (type.toLowerCase()) {
    case "husky":
      return "Husky";
    case "lefthook":
      return "Lefthook";
    case "pre-commit":
      return "pre-commit";
    case "custom":
      return t("hooks.framework.custom");
    default:
      return type;
  }
}

async function handleRunHookCard(hook: GitHookInfo) {
  if (!props.currentRepoPath || hookRunning.value) return;
  hookRunning.value = hook.name;
  try {
    const res = await gitService.runGitHook(props.currentRepoPath, hook.name);
    if (res.success || res.exitCode === 0) {
      notify(t("hooks.toast.run.success", { name: hook.name, code: res.exitCode, ms: res.durationMs }));
    } else {
      notify(t("hooks.toast.run.failed", { name: hook.name, code: res.exitCode, ms: res.durationMs }));
    }
  } catch (err: any) {
    notify(t("hooks.toast.run.error", { name: hook.name, error: err?.message || err }));
  } finally {
    hookRunning.value = "";
  }
}

async function handleInstallSampleHook(hook: GitHookInfo) {
  if (!props.currentRepoPath || hookBusy.value) return;
  hookBusy.value = hook.name;
  try {
    await gitService.installSampleHook(props.currentRepoPath, hook.name);
    notify(t("hooks.toast.sample.installed", { name: hook.name }));
    await loadGitHooks();
    flashToggledHook(hook.name);
    emit("refresh");
  } catch (err: any) {
    notify(t("hooks.toast.sample.failed", { name: hook.name, error: err?.message || err }));
  } finally {
    hookBusy.value = "";
  }
}

function handleApplyHookTemplate(e: Event) {
  const target = e.target as HTMLSelectElement;
  const val = target.value;
  const tpl = HOOK_TEMPLATES.find((item) => item.id === val);
  if (tpl) {
    hookEditContent.value = tpl.script;
    notify(t("hooks.toast.template.applied", { name: t(tpl.nameKey) }));
  }
  selectedHookTemplate.value = "";
}

async function handleRunHookInModal() {
  if (!props.currentRepoPath || isTestingInModal.value) return;
  isTestingInModal.value = true;
  showTestTerminal.value = true;
  try {
    const res = await gitService.runGitHook(props.currentRepoPath, hookEditName.value);
    modalTestResult.value = res;
  } catch (err: any) {
    modalTestResult.value = {
      exitCode: -1,
      stdout: "",
      stderr: err?.message || String(err),
      durationMs: 0,
      success: false,
    };
  } finally {
    isTestingInModal.value = false;
  }
}

async function openHookEditor(hook: GitHookInfo) {
  if (!props.currentRepoPath) return;
  hookEditName.value = hook.name;
  hookEditContent.value = "";
  hookEditMeta.value = "";
  hookEditEnable.value = hook.enabled || !hook.installed;
  showTestTerminal.value = false;
  modalTestResult.value = null;
  selectedHookTemplate.value = "";
  showHookEditor.value = true;
  isLoadingHook.value = true;
  try {
    const res = await gitService.readGitHook(props.currentRepoPath, hook.name);
    hookEditContent.value = res.content;
    hookEditMeta.value = res.message;
    hookEditEnable.value = res.exists ? res.enabled : true;
  } catch (err: any) {
    notify(t("maint.toast.hook.read.failed", { error: err?.message || err }));
  } finally {
    isLoadingHook.value = false;
  }
}

async function handleSaveHook() {
  if (!props.currentRepoPath || isSavingHook.value) return;
  if (!hookEditContent.value.trim()) {
    notify(t("maint.toast.hook.empty"));
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
    notify(t("maint.toast.hook.saved", { name: hookEditName.value, state: hookEditEnable.value ? t("maint.hook.state.enabled") : t("maint.hook.state.disabled") }));
    showHookEditor.value = false;
    await loadGitHooks();
  } catch (err: any) {
    notify(t("maint.toast.hook.save.failed", { error: err?.message || err }));
  } finally {
    isSavingHook.value = false;
  }
}

async function handleToggleHook(hook: GitHookInfo) {
  if (!props.currentRepoPath || hookBusy.value) return;
  const enable = !hook.enabled;
  if (!(await confirmAction(t(enable ? "maint.confirm.hook.enable" : "maint.confirm.hook.disable", { name: hook.name })))) return;
  hookBusy.value = hook.name;
  try {
    await gitService.toggleGitHook(props.currentRepoPath, hook.name, enable, "");
    notify(t(enable ? "maint.toast.hook.enabled" : "maint.toast.hook.disabled", { name: hook.name }));
    await loadGitHooks();
    flashToggledHook(hook.name);
    emit("refresh");
  } catch (err: any) {
    notify(t("maint.toast.hook.toggle.failed", { error: err?.message || err }));
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
    notify(enable ? t("maint.toast.hook.none.disabled") : t("maint.toast.hook.none.enabled"));
    return;
  }
  if (!(await confirmAction(t(enable ? "maint.confirm.hooks.enable" : "maint.confirm.hooks.disable", { count: targets.length })))) return;
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
      notify(t("maint.toast.hook.toggle.one.failed", { name: hook.name, error: err?.message || err }));
    }
  }
  hookBusy.value = "";
  notify(t(enable ? "maint.toast.hooks.enabled" : "maint.toast.hooks.disabled", { ok: succeeded, failed: failed ? t("maint.toast.hooks.failed.part", { count: failed }) : "" }));
  await loadGitHooks();
  if (lastToggled) flashToggledHook(lastToggled);
  emit("refresh");
}

async function loadGitHooks() {
  if (!props.currentRepoPath) return;
  isLoadingHooks.value = true;
  try {
    const [hooks, framework] = await Promise.all([
      gitService.getGitHooks(props.currentRepoPath),
      gitService.detectHooksFramework(props.currentRepoPath).catch((err) => {
        console.warn("Failed to detect hooks framework:", err);
        return null;
      }),
    ]);
    gitHooks.value = hooks || [];
    if (framework) {
      hooksFramework.value = framework;
    }
  } catch (err) {
    reportError("Failed to load git hooks", err);
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
    reportError("Failed to load maintenance stats", err);
  } finally {
    isLoadingMaintenance.value = false;
  }
}

const cacheStats = ref({
  detailCount: commitDetailCache.size,
  detailCapacity: commitDetailCache.getCapacity(),
});

function refreshCacheStats() {
  cacheStats.value = {
    detailCount: commitDetailCache.size,
    detailCapacity: commitDetailCache.getCapacity(),
  };
}

function handleClearFrontendCache() {
  gitService.clearCommitDetailCache();
  window.dispatchEvent(new CustomEvent("wgit:clear-render-cache"));
  refreshCacheStats();
  notify(t("maint.cache.cleared"));
}

async function handleRunMaintenance() {
  if (!props.currentRepoPath) return;
  isRunningMaintenance.value = true;
  try {
    const res = await gitService.runMaintenance(props.currentRepoPath);
    maintenanceStats.value = res;
    if (res?.success === false) {
      notify(t("maint.toast.maintenance.warn", { count: res.warnings?.length ?? 0 }));
    } else if (res?.remotePruneStatus === "SKIPPED") {
      notify(t("maint.toast.maintenance.no.remote"));
    } else {
      notify(t("maint.toast.maintenance.done"));
    }
  } catch (err: any) {
    notify(t("maint.toast.maintenance.failed", { error: err?.message || err }));
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
          ? t("maint.toast.lfs.prune.dry", { size: res.reclaimedStr || "0 B", count: res.prunedFiles })
          : t("maint.toast.lfs.prune.done", { size: res.reclaimedStr || "0 B" })
      );
    }
  } catch (err: any) {
    notify(t("maint.toast.lfs.prune.failed", { error: err?.message || err }));
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
        t("maint.toast.health.done", {
          score: healthReport.value.score,
          grade: healthReport.value.grade,
          pass: healthReport.value.passCount,
          warn: healthReport.value.warnCount,
          fail: healthReport.value.failCount,
        })
      );
    }
  } catch (err: any) {
    notify(t("maint.toast.health.failed", { error: err?.message || err }));
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
      return t("maint.fix.objects.action");
    case "ignore":
      return t("maint.fix.ignore.action");
    case "largefiles":
      return t("maint.fix.large.action");
    case "workspace":
      return t("maint.fix.conflict.action");
    case "signing":
      return t("maint.fix.signing.action");
    default:
      return t("maint.fix.identity.action");
  }
}

function healthFixIsWrite(item: HealthDimension): boolean {
  return item.key === "objects" || item.key === "ignore";
}

async function handleHealthFix(item: HealthDimension) {
  if (!props.currentRepoPath || fixingKey.value) return;
  if (healthFixIsWrite(item)) {
    const verb = item.key === "objects" ? t("maint.fix.objects.verb") : t("maint.fix.ignore.verb");
    if (!(await confirmAction(t("maint.confirm.fix", { verb })))) return;
  }

  fixingKey.value = item.key;
  try {
    const res = await gitService.runHealthFix(props.currentRepoPath, item.key);
    fixResults.value = { ...fixResults.value, [item.key]: res };

    if (res.needsInput) {
      notify(res.hint || t("maint.toast.fix.manual"));
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

    notify(res.hint || res.output || t("maint.toast.fix.done"));
    await handleRunHealthCheck();
    emit("refresh");
  } catch (err: any) {
    notify(t("maint.toast.fix.failed", { error: err?.message || err }));
  } finally {
    fixingKey.value = "";
  }
}

// Object-store tuning: the passes that make a 100k-commit repo feel fast.
const OBJECT_STORE_TASKS = [
  { key: "commit-graph", label: t("maint.store.task.commitgraph"), hint: t("maint.store.task.commitgraph.hint") },
  { key: "multi-pack-index", label: t("maint.store.task.midx"), hint: t("maint.store.task.midx.hint") },
  { key: "repack", label: t("maint.store.task.repack"), hint: t("maint.store.task.repack.hint") },
  { key: "pack-refs", label: t("maint.store.task.packrefs"), hint: t("maint.store.task.packrefs.hint") },
  { key: "prune", label: t("maint.store.task.prune"), hint: t("maint.store.task.prune.hint") },
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
    notify(t("maint.toast.optimize.none"));
    return;
  }
  const hasRepack = objectStoreTasks.value.includes("repack");
  const confirmed = await confirmAction({
    message:
      t("maint.confirm.optimize", {
        count: objectStoreTasks.value.length,
        tasks: objectStoreTasks.value.join(", "),
      }) + (hasRepack ? t("maint.confirm.optimize.repack") : ""),
    tone: "danger",
  });
  if (!confirmed) return;

  isOptimizingStore.value = true;
  try {
    const res = await gitService.optimizeObjectStore(props.currentRepoPath, objectStoreTasks.value);
    optimizeResult.value = res;
    notify(backendText(res.messageKey, res.message, res.params));
    emit("refresh");
  } catch (err: any) {
    notify(t("maint.toast.optimize.failed", { error: err?.message || err }));
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
      notify(t("maint.toast.fsck.clean"));
    } else {
      notify(t("maint.toast.fsck.warn"));
    }
  } catch (err: any) {
    notify(t("maint.toast.fsck.failed", { error: err?.message || err }));
  } finally {
    isCheckingHealth.value = false;
  }
}

const isPruningDangling = ref(false);

async function handlePruneDangling() {
  if (!props.currentRepoPath || isPruningDangling.value) return;
  const confirmed = await confirmAction({
    message: t("maint.fsck.prune.confirm"),
    tone: "warning",
  });
  if (!confirmed) return;

  isPruningDangling.value = true;
  try {
    const res = await gitService.optimizeObjectStore(props.currentRepoPath, ["prune"]);
    notify(backendText(res.messageKey, res.message, res.params));
    await handleVerifyRepoHealth();
    emit("refresh");
  } catch (err: any) {
    notify(t("maint.toast.prune.dangling.failed", { error: err?.message || err }));
  } finally {
    isPruningDangling.value = false;
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
                <span>{{ t("maint.gc.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("maint.gc.desc") }}
              </p>
            </div>

            <button
              @click="handleRunMaintenance"
              :disabled="isRunningMaintenance"
              class="flex items-center gap-1.5 shrink-0 whitespace-nowrap px-3.5 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-xs font-semibold transition cursor-pointer shadow-xs disabled:opacity-50"
            >
              <Activity class="w-3.5 h-3.5" :class="{ 'animate-spin': isRunningMaintenance }" />
              <span>{{ isRunningMaintenance ? t("maint.gc.running") : t("maint.gc.run") }}</span>
            </button>
          </div>

          <div class="grid grid-cols-3 gap-4">
            <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-xs text-ink-muted font-medium">{{ t("maint.stat.gitdir") }}</span>
              <span class="text-xl font-bold font-mono text-ink">{{ maintenanceStats?.gitDirSize || t("maint.not.computed") }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("maint.stat.gitdir.hint") }}</span>
            </div>
            <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-xs text-ink-muted font-medium">{{ t("maint.stat.packs") }}</span>
              <span class="text-xl font-bold font-mono text-purple">{{ t("maint.stat.packs.value", { count: maintenanceStats?.packFilesCount ?? 0 }) }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("maint.stat.packs.hint") }}</span>
            </div>
            <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-xs text-ink-muted font-medium">{{ t("maint.stat.loose") }}</span>
              <span class="text-xl font-bold font-mono text-warning">{{ t("maint.stat.loose.value", { count: maintenanceStats?.looseObjects ?? 0 }) }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("maint.stat.loose.hint") }}</span>
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

        <!-- Section A2: Frontend Memory Cache -->
        <div class="space-y-3 pt-2" data-tool-section="frontend-cache">
          <div class="flex flex-wrap items-center justify-between gap-2 pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Zap class="w-4 h-4 text-warning" />
                <span>{{ t("maint.cache.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("maint.cache.desc") }}
              </p>
            </div>

            <button
              @click="handleClearFrontendCache"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised text-ink text-xs font-semibold cursor-pointer transition border border-line"
            >
              <Trash2 class="w-3.5 h-3.5 text-warning" />
              <span>{{ t("maint.cache.clear.btn") }}</span>
            </button>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-xs text-ink-muted font-medium">{{ t("maint.cache.detail.label") }}</span>
              <span class="text-xl font-bold font-mono text-ink">{{ cacheStats.detailCount }} / {{ cacheStats.detailCapacity }}</span>
              <span class="text-[10px] text-ink-muted">{{ t("maint.cache.detail.hint") }}</span>
            </div>
            <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-1">
              <span class="text-xs text-ink-muted font-medium">{{ t("maint.cache.svg.label") }}</span>
              <span class="text-xl font-bold font-mono text-purple">600 LRU</span>
              <span class="text-[10px] text-ink-muted">{{ t("maint.cache.svg.hint") }}</span>
            </div>
          </div>
        </div>

        <!-- Section A3: Repo Bloat Inspector & Slimming Studio (Phase 47) -->
        <div class="space-y-3 pt-2" data-testid="repo-bloat-card" data-tool-section="bloat-inspector">
          <div class="flex flex-wrap items-center justify-between gap-2 pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <HardDrive class="w-4 h-4 text-rose" />
                <span>{{ t("maint.bloat.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("maint.bloat.desc") }}
              </p>
            </div>

            <div class="flex items-center gap-2">
              <button
                type="button"
                data-testid="scan-bloat-btn"
                :disabled="isScanningBloat"
                @click="handleScanBloat"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised text-ink text-xs font-semibold cursor-pointer transition border border-line disabled:opacity-50"
              >
                <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isScanningBloat }" />
                <span>{{ isScanningBloat ? t("maint.bloat.scanning") : t("maint.bloat.scan.btn") }}</span>
              </button>
              <button
                v-if="bloatReport && bloatReport.topBloatObjects.length > 0"
                type="button"
                data-testid="generate-slimming-script-btn"
                :disabled="isGeneratingPlan"
                @click="handleGenerateSlimmingPlan"
                class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold cursor-pointer transition disabled:opacity-50"
              >
                <Rocket class="w-3.5 h-3.5" />
                <span>{{ isGeneratingPlan ? t("maint.bloat.plan.generating") : t("maint.bloat.plan.btn") }}</span>
              </button>
            </div>
          </div>

          <div v-if="bloatReport" class="space-y-3">
            <div class="p-3 rounded-lg bg-rose/10 border border-rose/30 flex items-center justify-between text-xs text-rose">
              <span class="font-medium">{{ bloatReport.summary }}</span>
              <span class="font-mono font-bold px-2 py-0.5 rounded bg-canvas border border-line text-ink">
                {{ bloatReport.ghostBytesHuman }}
              </span>
            </div>

            <div class="max-h-60 overflow-y-auto space-y-1 pr-1">
              <div
                v-for="obj in bloatReport.topBloatObjects"
                :key="obj.hash"
                class="p-2 rounded-lg border border-line bg-canvas/60 flex items-center justify-between text-xs font-mono"
              >
                <div class="flex items-center gap-2 min-w-0">
                  <span
                    v-if="obj.isGhostBlob"
                    class="px-1.5 py-0.2 rounded text-[9px] font-sans font-bold bg-danger/15 text-danger border border-danger/30 shrink-0"
                  >
                    {{ t("maint.bloat.ghost") }}
                  </span>
                  <span class="text-ink truncate font-semibold" :title="obj.path">{{ obj.path || obj.hash.slice(0, 10) }}</span>
                </div>
                <span class="text-ink-muted text-[11px] shrink-0">{{ obj.sizeHuman }}</span>
              </div>
            </div>

            <div v-if="slimmingPlan" class="p-3 rounded-xl border border-line bg-surface/80 space-y-2 text-xs">
              <div class="flex items-center justify-between">
                <span class="font-bold text-ink">{{ t("maint.bloat.plan.title") }}</span>
                <span class="text-success font-mono">{{ slimmingPlan.estimatedReclaimedHuman }}</span>
              </div>
              <p class="text-[11px] text-ink-muted">{{ slimmingPlan.safetyNotice }}</p>
              <div class="p-2 rounded bg-canvas border border-line font-mono text-[10px] text-ink select-all overflow-x-auto">
                <code>{{ slimmingPlan.filterRepoCommand }}</code>
              </div>
            </div>
          </div>
        </div>

        <!-- Section A4: Workspace Hygiene & Safe Cleaner (Phase 54) -->
        <div class="space-y-3 pt-2" data-testid="workspace-hygiene-card" data-tool-section="workspace-hygiene">
          <div class="flex flex-wrap items-center justify-between gap-2 pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Trash2 class="w-4 h-4 text-amber-500" />
                <span>{{ t("maint.hygiene.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("maint.hygiene.desc") }}
              </p>
            </div>

            <div class="flex items-center gap-2">
              <button
                type="button"
                data-testid="scan-workspace-junk-btn"
                :disabled="isScanningJunk"
                @click="handleScanWorkspaceJunk"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised text-ink text-xs font-semibold cursor-pointer transition border border-line disabled:opacity-50"
              >
                <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isScanningJunk }" />
                <span>{{ isScanningJunk ? t("maint.hygiene.scanning") : t("maint.hygiene.scan.btn") }}</span>
              </button>
              <button
                v-if="junkReport && junkReport.totalFiles > 0"
                type="button"
                data-testid="execute-safe-clean-btn"
                :disabled="isCleaningJunk || selectedJunkCategories.length === 0"
                @click="handleExecuteSafeClean"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-warning-strong hover:bg-warning-strong-hover text-warning-contrast text-xs font-semibold cursor-pointer transition disabled:opacity-50"
              >
                <Trash2 class="w-3.5 h-3.5" />
                <span>{{ isCleaningJunk ? t("maint.hygiene.cleaning") : t("maint.hygiene.clean.btn") }}</span>
              </button>
            </div>
          </div>

          <!-- Junk Report & Categories -->
          <div v-if="junkReport" class="space-y-3">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <label
                v-for="cat in junkReport.categories"
                :key="cat.categoryId"
                data-testid="junk-category-row"
                class="p-3 rounded-xl border border-line bg-canvas flex items-start gap-2.5 cursor-pointer hover:bg-raised/40 transition select-none"
              >
                <input
                  v-model="selectedJunkCategories"
                  type="checkbox"
                  :value="cat.categoryId"
                  class="mt-0.5 rounded border-line bg-surface text-accent focus:ring-0"
                />
                <div class="flex-1 min-w-0">
                  <div class="flex items-center justify-between text-xs font-semibold text-ink">
                    <span>{{ cat.title }}</span>
                    <span class="text-[10px] font-mono text-ink-muted">{{ cat.fileCount }} files ({{ (cat.sizeBytes / 1024).toFixed(1) }} KB)</span>
                  </div>
                  <p class="text-[10px] text-ink-muted mt-0.5">{{ cat.description }}</p>
                </div>
              </label>
            </div>

            <!-- Recommended .gitignore Appends -->
            <div
              v-if="junkReport.recommendedGitIgnore && junkReport.recommendedGitIgnore.length > 0"
              class="p-3 rounded-xl border border-line bg-accent/5 flex flex-wrap items-center justify-between gap-2"
            >
              <div class="text-xs">
                <span class="font-bold text-accent">{{ t("maint.hygiene.ignore.title") }}:</span>
                <span class="font-mono text-[11px] text-ink ml-1.5">{{ junkReport.recommendedGitIgnore.join(", ") }}</span>
              </div>
              <button
                type="button"
                data-testid="append-gitignore-btn"
                :disabled="isAppendingGitIgnore"
                @click="handleAppendGitIgnore"
                class="px-2.5 py-1 rounded text-xs font-semibold bg-accent text-accent-contrast hover:opacity-90 transition cursor-pointer disabled:opacity-50"
              >
                {{ t("maint.hygiene.ignore.append.btn") }}
              </button>
            </div>
          </div>
        </div>

        <!-- Section B: LFS Prune Cache -->
        <div class="space-y-3 pt-2" data-tool-section="lfs-prune">
          <div class="flex flex-wrap items-center justify-between gap-2 pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <HardDrive class="w-4 h-4 text-accent" />
                <span>{{ t("maint.lfs.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("maint.lfs.desc") }}
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-3 shrink-0">
              <label class="flex items-center gap-1.5 text-xs text-ink-muted cursor-pointer select-none">
                <input
                  v-model="lfsPruneDryRun"
                  type="checkbox"
                  class="rounded accent-accent"
                />
                <span>{{ t("maint.lfs.dryrun") }}</span>
              </label>

              <button
                @click="handleLfsPrune"
                :disabled="isPruningLfs"
                class="flex items-center gap-1.5 shrink-0 whitespace-nowrap px-3.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-50"
              >
                <Trash2 class="w-3.5 h-3.5" :class="{ 'animate-spin': isPruningLfs }" />
                <span>{{ isPruningLfs ? t("maint.lfs.running") : (lfsPruneDryRun ? t("maint.lfs.dryrun.action") : t("maint.lfs.run")) }}</span>
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
                  {{ lfsPruneResult.dryRun ? t('maint.lfs.dry.done') : t('maint.lfs.done') }}
                </div>
                <div class="text-ink-muted font-mono mt-0.5">
                  {{ t("maint.lfs.pruned", { count: lfsPruneResult.prunedFiles }) }}
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
                <span>{{ t("maint.fsck.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("maint.fsck.desc") }}
              </p>
            </div>

            <button
              @click="handleVerifyRepoHealth"
              :disabled="isCheckingHealth"
              class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              <ShieldCheck class="w-3.5 h-3.5 text-success" :class="{ 'animate-spin': isCheckingHealth }" />
              <span>{{ isCheckingHealth ? t("maint.fsck.running") : t("maint.fsck.run") }}</span>
            </button>
          </div>

          <div v-if="fsckResult" class="p-4 rounded-xl border border-line bg-surface/30 space-y-3">
            <div class="flex items-center gap-2">
              <CheckCircle2 v-if="fsckResult.isClean" class="w-5 h-5 text-success shrink-0" />
              <AlertCircle v-else class="w-5 h-5 text-danger shrink-0" />
              <span class="text-xs font-bold" :class="fsckResult.isClean ? 'text-success' : 'text-danger'">
                {{ fsckResult.isClean ? t('maint.fsck.clean') : t('maint.fsck.warn') }}
              </span>
            </div>

            <div class="grid grid-cols-3 gap-3 font-mono text-xs pt-1">
              <div class="p-2.5 rounded-lg bg-surface/40 border border-line flex justify-between">
                <span class="text-ink-muted">{{ t("maint.fsck.dangling.blob") }}</span>
                <span class="font-bold text-ink">{{ t("maint.fsck.count", { count: fsckResult.danglingBlob }) }}</span>
              </div>
              <div class="p-2.5 rounded-lg bg-surface/40 border border-line flex justify-between">
                <span class="text-ink-muted">{{ t("maint.fsck.dangling.tree") }}</span>
                <span class="font-bold text-ink">{{ t("maint.fsck.count", { count: fsckResult.danglingTree }) }}</span>
              </div>
              <div class="p-2.5 rounded-lg bg-surface/40 border border-line flex justify-between">
                <span class="text-ink-muted">{{ t("maint.fsck.dangling.commit") }}</span>
                <span class="font-bold text-ink">{{ t("maint.fsck.count", { count: fsckResult.danglingCommit }) }}</span>
              </div>
            </div>

            <!-- Dangling Objects Prune Action Bar -->
            <div
              v-if="fsckResult.danglingBlob > 0 || fsckResult.danglingTree > 0 || fsckResult.danglingCommit > 0"
              class="flex items-center justify-between p-3 rounded-lg bg-surface/50 border border-line gap-2 flex-wrap"
            >
              <span class="text-xs text-ink-secondary">
                {{ t("maint.fsck.dangling.hint", { count: fsckResult.danglingBlob + fsckResult.danglingTree + fsckResult.danglingCommit }) }}
              </span>
              <button
                type="button"
                @click="handlePruneDangling"
                :disabled="isPruningDangling"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-warning/40 bg-warning/10 hover:bg-warning/20 text-warning text-xs font-semibold transition cursor-pointer disabled:opacity-50"
              >
                <Trash2 class="w-3.5 h-3.5" />
                <span>{{ isPruningDangling ? t("maint.fsck.pruning") : t("maint.fsck.prune.action") }}</span>
              </button>
            </div>

            <div v-if="fsckResult.rawErrors && fsckResult.rawErrors.length > 0" class="p-3 rounded-lg bg-danger/10 border border-danger/30 text-xs text-danger font-mono space-y-1">
              <div class="flex items-center justify-between pb-1 border-b border-danger/20">
                <span class="font-bold">{{ t("maint.fsck.errors") }} ({{ fsckResult.rawErrors.length }})</span>
                <button
                  type="button"
                  @click="copyToClipboard(fsckResult.rawErrors.join('\n'))"
                  class="flex items-center gap-1 text-[11px] text-danger hover:underline cursor-pointer"
                >
                  <Copy class="w-3 h-3" />
                  <span>{{ t("maint.hotfix.copy") }}</span>
                </button>
              </div>
              <div v-for="(err, idx) in fsckResult.rawErrors" :key="idx" class="break-all">{{ err }}</div>
            </div>
          </div>
        </div>

        <!-- Section D: Git Hooks Status -->
        <div class="space-y-3 pt-2" data-tool-section="hooks">
          <!-- Third-Party Framework Detection Banner -->
          <div
            v-if="hooksFramework && hooksFramework.type && hooksFramework.type !== 'native'"
            class="flex items-start gap-3 p-3 rounded-xl border border-accent/30 bg-accent/10"
            data-testid="hooks-framework-banner"
          >
            <ShieldCheck class="w-5 h-5 text-accent shrink-0 mt-0.5" />
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-xs font-bold text-ink">
                  {{ t("hooks.framework.banner.title", { framework: frameworkDisplayName(hooksFramework.type) }) }}
                </span>
                <span class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-accent/20 text-accent uppercase">
                  {{ hooksFramework.type }}
                </span>
              </div>
              <p class="text-[11px] text-ink-muted mt-1">
                {{ t("hooks.framework.banner.desc", { framework: frameworkDisplayName(hooksFramework.type) }) }}
              </p>
              <div class="text-[10px] text-ink-muted mt-1.5 space-y-0.5 font-mono">
                <div v-if="hooksFramework.configFile" class="truncate">
                  <span class="text-ink font-semibold font-sans">{{ t("hooks.framework.config") }}:</span>
                  <span class="ml-1 text-ink-muted">{{ hooksFramework.configFile }}</span>
                </div>
                <div v-if="hooksFramework.hooksPath" class="truncate">
                  <span class="text-ink font-semibold font-sans">{{ t("hooks.framework.path") }}:</span>
                  <span class="ml-1 text-ink-muted">{{ hooksFramework.hooksPath }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="flex items-center justify-between pb-2 border-b border-line">
            <div>
              <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                <Terminal class="w-4 h-4 text-warning" />
                <span>{{ t("maint.hooks.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("maint.hooks.desc") }}
              </p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <span class="text-xs font-mono text-ink-muted">
                {{ t("maint.hooks.enabled.count", { enabled: enabledHooksCount, total: gitHooks.length }) }}
              </span>
              <button
                @click.stop="bulkToggleHooks(true)"
                :disabled="!!hookBusy || hooksToEnableCount === 0"
                class="px-2 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-success transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                :title="t('maint.hooks.enable.all.title', { count: hooksToEnableCount })"
              >
                {{ t("maint.hooks.enable.all") }}
              </button>
              <button
                @click.stop="bulkToggleHooks(false)"
                :disabled="!!hookBusy || hooksToDisableCount === 0"
                class="px-2 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-warning transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                :title="t('maint.hooks.disable.all.title', { count: hooksToDisableCount })"
              >
                {{ t("maint.hooks.disable.all") }}
              </button>
            </div>
          </div>

          <div v-if="isLoadingHooks && gitHooks.length === 0" class="py-2 text-xs text-ink-muted">
            {{ t("maint.hooks.loading") }}
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
                {{ hook.enabled ? t("maint.hook.enabled") : hook.installed ? t("maint.hook.disabled") : t("maint.hook.not.installed") }}
              </span>
              <button :aria-label="t('maint.hook.reveal')"
                v-if="hook.installed"
                @click.stop="handleRevealHook(hook)"
                class="p-1 rounded bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer shrink-0"
                :title="t('maint.hook.reveal')"
              >
                <FolderSearch class="w-3 h-3" />
              </button>
              <button
                :aria-label="t('hooks.test.run')"
                @click.stop="handleRunHookCard(hook)"
                :disabled="hookRunning === hook.name"
                class="p-1 rounded bg-surface hover:bg-raised border border-line text-ink-muted hover:text-accent transition cursor-pointer shrink-0 disabled:opacity-40"
                :title="t('hooks.test.run')"
                data-testid="hook-card-test-btn"
              >
                <Play class="w-3 h-3 fill-current" />
              </button>
              <button
                :aria-label="hook.installed ? t('hooks.sample.restore') : t('hooks.sample.init')"
                @click.stop="handleInstallSampleHook(hook)"
                :disabled="!!hookBusy"
                class="p-1 rounded bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer shrink-0 disabled:opacity-40"
                :title="hook.installed ? t('hooks.sample.restore') : t('hooks.sample.init')"
                data-testid="hook-card-sample-btn"
              >
                <RotateCcw class="w-3 h-3" />
              </button>
              <button :aria-label="t('maint.hook.edit')"
                @click.stop="openHookEditor(hook)"
                class="p-1 rounded bg-surface hover:bg-raised border border-line text-ink-muted hover:text-accent transition cursor-pointer shrink-0"
                :title="t('maint.hook.edit')"
              >
                <FileCode class="w-3 h-3" />
              </button>
              <button
                v-if="hook.installed"
                @click.stop="handleToggleHook(hook)"
                :disabled="!!hookBusy"
                class="px-1.5 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[9px] font-semibold text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
                :title="hook.enabled ? t('maint.hook.disable.title') : t('maint.hook.enable.title')"
              >
                {{ hookBusy === hook.name ? t("maint.hook.busy") : hook.enabled ? t("maint.hook.disable") : t("maint.hook.enable") }}
              </button>
            </div>
          </div>
        </div>

        <!-- Hook script editor -->
        <UiModal
          :open="showHookEditor"
          :label="t('maint.hook.modal.title', { name: hookEditName })"
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

            <!-- Template selection & Test Run header controls -->
            <div class="flex items-center justify-between gap-2 flex-wrap pb-1 border-b border-line/60">
              <div class="flex items-center gap-2">
                <label class="text-[11px] text-ink-muted font-medium whitespace-nowrap">
                  {{ t("hooks.template.select") }}:
                </label>
                <select
                  v-model="selectedHookTemplate"
                  @change="handleApplyHookTemplate"
                  class="px-2 py-1 rounded-md bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-sans"
                  data-testid="hook-template-select"
                >
                  <option value="" disabled>{{ t("hooks.template.placeholder") }}</option>
                  <option v-for="tpl in HOOK_TEMPLATES" :key="tpl.id" :value="tpl.id">
                    {{ t(tpl.nameKey) }} ({{ tpl.targetHook }})
                  </option>
                </select>
              </div>

              <button
                type="button"
                @click="handleRunHookInModal"
                :disabled="isTestingInModal"
                class="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent hover:text-accent-strong transition cursor-pointer disabled:opacity-40"
                :title="t('hooks.test.run')"
                data-testid="hook-modal-test-btn"
              >
                <Play class="w-3.5 h-3.5 fill-current" />
                <span>{{ isTestingInModal ? t("hooks.test.running") : t("hooks.test.run") }}</span>
              </button>
            </div>

            <div v-if="isLoadingHook" class="py-6 text-center text-xs text-ink-muted">{{ t("maint.hook.loading.script") }}</div>
            <template v-else>
              <textarea
                v-model="hookEditContent"
                spellcheck="false"
                rows="12"
                :aria-label="t('maint.hook.script')"
                :placeholder="t('maint.hook.placeholder')"
                class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-[11px] font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
              ></textarea>
              <label class="flex items-center gap-2 text-xs text-ink cursor-pointer select-none">
                <input v-model="hookEditEnable" type="checkbox" class="rounded accent-[var(--ug-accent-strong)]" />
                <span>{{ t("maint.hook.enable.checkbox") }}</span>
              </label>

              <!-- Live Test Run Console Panel -->
              <div
                v-if="showTestTerminal"
                class="rounded-lg border border-line bg-black/90 p-2.5 space-y-2 text-xs font-mono"
                data-testid="hook-test-terminal"
              >
                <div class="flex items-center justify-between border-b border-line/40 pb-1.5 text-[11px]">
                  <div class="flex items-center gap-2">
                    <Terminal class="w-3.5 h-3.5 text-ink-muted" />
                    <span class="text-ink font-semibold">{{ t("hooks.terminal.title") }}</span>
                    <span
                      v-if="modalTestResult"
                      class="px-1.5 py-0.2 rounded text-[10px] font-bold"
                      :class="modalTestResult.exitCode === 0 ? 'bg-success/20 text-success' : 'bg-danger/20 text-danger'"
                    >
                      Exit Code: {{ modalTestResult.exitCode }}
                    </span>
                    <span v-if="modalTestResult" class="text-[10px] text-ink-muted">
                      {{ modalTestResult.durationMs }}ms
                    </span>
                  </div>
                  <button
                    type="button"
                    @click="showTestTerminal = false"
                    class="text-[10px] text-ink-muted hover:text-ink px-1 cursor-pointer"
                  >
                    {{ t("common.close") }}
                  </button>
                </div>
                <div v-if="isTestingInModal" class="text-ink-muted py-2 text-[11px]">
                  {{ t("hooks.terminal.running") }}
                </div>
                <div v-else-if="modalTestResult" class="max-h-36 overflow-y-auto space-y-1 text-[11px] select-text">
                  <div v-if="modalTestResult.stdout" class="text-emerald-400 whitespace-pre-wrap break-all">{{ modalTestResult.stdout }}</div>
                  <div v-if="modalTestResult.stderr" class="text-danger whitespace-pre-wrap break-all">{{ modalTestResult.stderr }}</div>
                  <div v-if="!modalTestResult.stdout && !modalTestResult.stderr" class="text-ink-muted italic">
                    {{ t("hooks.terminal.no_output") }}
                  </div>
                </div>
              </div>
            </template>
          </div>

          <template #footer>
            <UiButton variant="ghost" @click="showHookEditor = false">{{ t("common.cancel") }}</UiButton>
            <UiButton variant="primary" :disabled="isSavingHook || isLoadingHook" @click="handleSaveHook">
              {{ isSavingHook ? t("maint.hook.saving") : t("maint.hook.save") }}
            </UiButton>
          </template>
        </UiModal>

        <!-- Section E: Archive, patch & bundle -->
        <div class="space-y-3 pt-2" data-tool-section="archive">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <FileBox class="w-4 h-4 text-accent" />
              <span>{{ t("maint.archive.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("maint.archive.desc") }}
            </p>
          </div>

          <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
            <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
              <div class="text-xs font-bold text-ink">{{ t("maint.archive.export") }}</div>
              <div class="grid grid-cols-2 gap-2">
                <label class="flex flex-col gap-1">
                  <span class="text-[10px] text-ink-muted">{{ t("maint.archive.ref") }}</span>
                  <input
                    v-model="archiveRef"
                    type="text"
                    placeholder="HEAD / v1.0.0 / feature/x"
                    class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                  />
                </label>
                <label class="flex flex-col gap-1">
                  <span class="text-[10px] text-ink-muted">{{ t("maint.archive.outdir") }}</span>
                  <div class="flex items-center gap-1">
                    <input
                      v-model="archiveDir"
                      type="text"
                      :placeholder="t('maint.archive.outdir.placeholder')"
                      class="flex-1 min-w-0 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                    />
                    <button
                      @click="handlePickArchiveDir"
                      class="px-2 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-[10px] font-semibold text-ink transition cursor-pointer shrink-0"
                      :title="t('maint.archive.browse.title')"
                    >
                      {{ t("maint.archive.browse") }}
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
                  {{ archiveBusy === "archive" ? t("maint.archive.exporting") : t("maint.archive.zip") }}
                </button>
                <button
                  @click="handleExportPatch"
                  :disabled="!archiveDir.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "patch" ? t("maint.archive.exporting") : t("maint.archive.patch") }}
                </button>
                <button
                  @click="handleCreateBundle"
                  :disabled="!archiveDir.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "bundle" ? t("maint.archive.creating") : t("maint.archive.bundle") }}
                </button>
                <button
                  @click="showBundleModal = true"
                  class="px-2.5 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer"
                >
                  {{ t("bundle.modal.title") }}
                </button>
              </div>
            </div>

            <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
              <div class="text-xs font-bold text-ink">{{ t("maint.archive.apply") }}</div>
              <label class="flex flex-col gap-1">
                <span class="text-[10px] text-ink-muted">{{ t("maint.archive.file") }}</span>
                <input
                  v-model="archiveFile"
                  type="text"
                  :placeholder="t('maint.archive.file.placeholder')"
                  class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
                />
              </label>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  @click="handleApplyPatch"
                  :disabled="!archiveFile.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-warning transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "apply" ? t("maint.archive.applying") : t("maint.archive.apply.workspace") }}
                </button>
                <button
                  @click="handleVerifyBundle"
                  :disabled="!archiveFile.trim() || !!archiveBusy"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-success transition cursor-pointer disabled:opacity-40"
                >
                  {{ archiveBusy === "verify" ? t("maint.archive.verifying") : t("maint.archive.verify") }}
                </button>
              </div>
              <label class="flex flex-col gap-1">
                <span class="text-[10px] text-ink-muted">{{ t("maint.archive.paste") }}</span>
                <textarea
                  v-model="patchContent"
                  rows="5"
                  spellcheck="false"
                  :placeholder="t('maint.archive.paste.placeholder')"
                  class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-[11px] text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
                ></textarea>
              </label>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  @click="handleCheckPatchContent"
                  :disabled="!patchContent.trim() || isApplyingPatchContent"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                >
                  {{ isApplyingPatchContent ? t("maint.archive.processing") : t("maint.archive.check") }}
                </button>
                <button
                  @click="handleApplyPatchContent"
                  :disabled="!patchContent.trim() || isApplyingPatchContent"
                  class="px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40"
                >
                  {{ t("maint.archive.apply.pasted") }}
                </button>
              </div>
              <pre v-if="patchContentResult" class="p-2 rounded bg-canvas/70 border border-line/60 text-[10px] text-ink-secondary whitespace-pre-wrap max-h-32 overflow-y-auto">{{ patchContentResult }}</pre>
              <p class="text-[10px] text-ink-muted">{{ t("maint.archive.hint") }}</p>
            </div>
          </div>

          <div v-if="archiveMessage" class="px-3 py-2 rounded-lg bg-canvas/60 border border-line font-mono text-[10px] text-ink-secondary truncate" :title="archiveMessage">
            {{ t("maint.archive.last.output", { message: archiveMessage }) }}
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
                <span>{{ t("maint.health.title") }}</span>
              </h3>
              <p class="text-xs text-ink-muted mt-0.5">
                {{ t("maint.health.desc") }}
              </p>
            </div>
            <button
              @click="handleRunHealthCheck"
              :disabled="isRunningHealthCheck"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-50"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isRunningHealthCheck }" />
              <span>{{ isRunningHealthCheck ? t("maint.health.running") : t("maint.health.run") }}</span>
            </button>
          </div>

          <template v-if="healthReport">
            <div class="grid grid-cols-5 gap-2">
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.health.score") }}</div>
                <div
                  class="text-lg font-bold font-mono"
                  :class="healthReport.score >= 85 ? 'text-success' : healthReport.score >= 60 ? 'text-warning' : 'text-danger'"
                >
                  {{ healthReport.score }}
                </div>
              </div>
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.health.grade") }}</div>
                <div class="text-lg font-bold text-ink">{{ healthReport.grade }}</div>
              </div>
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.health.pass") }}</div>
                <div class="text-lg font-bold font-mono text-success">{{ healthReport.passCount }}</div>
              </div>
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.health.warn") }}</div>
                <div class="text-lg font-bold font-mono text-warning">{{ healthReport.warnCount }}</div>
              </div>
              <div class="p-3 rounded-xl border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.health.fail") }}</div>
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
                  <span class="font-bold text-ink flex-1">{{ healthTitle(item) }}</span>
                  <span
                    class="px-1.5 py-0.2 rounded text-[9px] font-bold"
                    :class="item.status === 'pass' ? 'bg-success/15 text-success' : item.status === 'warn' ? 'bg-warning/15 text-warning' : 'bg-danger/15 text-danger'"
                  >
                    {{ item.status === "pass" ? t("maint.health.pass") : item.status === "warn" ? t("maint.health.warn") : t("maint.health.fail") }}
                  </span>
                  <span class="font-mono text-ink-muted">{{ item.score }}</span>
                </div>
                <p class="text-[10px] text-ink-muted leading-relaxed">{{ healthDetail(item) }}</p>
                <div v-if="item.status !== 'pass'" class="flex items-center gap-2 pt-0.5">
                  <button
                    @click="handleHealthFix(item)"
                    :disabled="fixingKey === item.key"
                    class="px-2 py-0.5 rounded-md border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink transition cursor-pointer disabled:opacity-40"
                  >
                    {{ fixingKey === item.key ? t("maint.health.fixing") : healthFixLabel(item) }}
                  </button>
                  <span class="text-[10px] text-ink-muted">{{ t("maint.health.recheck") }}</span>
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

            <p class="text-[10px] text-ink-muted">{{ healthSummary() }} · {{ t("maint.health.checked.at", { time: healthReport.checkedAt }) }}</p>
          </template>
          <p v-else class="text-[11px] text-ink-muted">{{ t("maint.health.not.run") }}</p>
        </div>

        <!-- Section F2: Object store tuning -->
        <div class="space-y-3 pt-4" data-tool-section="object-store">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Database class="w-4 h-4 text-accent" />
              <span>{{ t("maint.store.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("maint.store.desc") }}
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
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isOptimizingStore }" />
              <span>{{ isOptimizingStore ? t("maint.store.running") : t("maint.store.run", { count: objectStoreTasks.length }) }}</span>
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
              >{{ r.skipped ? t("maint.store.skipped") : r.success ? "OK" : "FAIL" }}</span>
              <span class="font-semibold text-ink shrink-0">{{ r.label }}</span>
              <span class="text-ink-muted flex-1 truncate" :title="r.output">{{ r.skipReason || r.output || t("maint.store.done") }}</span>
              <span v-if="r.durationMs > 0" class="font-mono text-ink-muted shrink-0">{{ (r.durationMs / 1000).toFixed(1) }}s</span>
            </div>
          </div>
        </div>

        <!-- Section G: Squash recent commits -->
        <div class="space-y-3 pt-4" data-tool-section="squash">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <Wrench class="w-4 h-4 text-warning" />
              <span>{{ t("maint.squash.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("maint.squash.desc") }}
            </p>
          </div>
          <div class="rounded-xl border border-warning/20 bg-warning/5 p-3 space-y-2">
            <div class="grid grid-cols-1 md:grid-cols-4 gap-2">
              <input
                v-model.number="squashCount"
                type="number"
                min="2"
                :aria-label="t('maint.squash.count')"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <input
                v-model="squashMessage"
                type="text"
                :placeholder="t('maint.squash.message')"
                @keydown.enter="handleSquashCommits"
                class="md:col-span-2 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <button
                @click="handleSquashCommits"
                :disabled="isSquashing || squashCount < 2 || !squashMessage.trim()"
                class="px-3 py-1.5 rounded-lg bg-warning-strong hover:opacity-90 text-warning-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                {{ isSquashing ? t("maint.squash.running") : t("maint.squash.run") }}
              </button>
            </div>
            <pre v-if="squashResult" class="p-2 rounded bg-canvas/70 border border-line/60 text-[10px] text-ink-secondary whitespace-pre-wrap max-h-32 overflow-y-auto">{{ squashResult }}</pre>
            <p class="text-[10px] text-ink-muted">{{ t("maint.squash.hint") }}</p>
          </div>
        </div>

        <!-- Section H: Crash stack demangle -->
        <div class="space-y-3 pt-4" data-tool-section="demangle">
          <div class="pb-2 border-b border-line">
            <h3 class="text-sm font-bold text-ink flex items-center gap-2">
              <AlertTriangle class="w-4 h-4 text-danger" />
              <span>{{ t("maint.crash.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("maint.crash.desc") }}
            </p>
          </div>
          <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
            <textarea
              v-model="crashDump"
              rows="6"
              spellcheck="false"
              :placeholder="t('maint.crash.placeholder')"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-[11px] text-ink font-mono leading-relaxed placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
            ></textarea>
            <div class="flex flex-wrap items-center gap-2">
              <input
                v-model="crashTargetCommit"
                type="text"
                :placeholder="t('maint.crash.ref')"
                class="w-56 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <button
                @click="handleDemangleCrash"
                :disabled="!crashDump.trim() || isDemangling"
                class="px-3 py-1.5 rounded-lg bg-danger-strong hover:opacity-90 text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                {{ isDemangling ? t("maint.crash.running") : t("maint.crash.run") }}
              </button>
              <span class="text-[10px] text-ink-muted">{{ t("maint.crash.hint") }}</span>
            </div>
          </div>

          <template v-if="crashResult">
            <div class="grid grid-cols-4 gap-2">
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.crash.reason") }}</div>
                <div class="text-xs font-bold text-danger truncate">{{ crashResult.crashReason }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.crash.module") }}</div>
                <div class="text-xs font-bold text-ink truncate">{{ crashResult.faultModule }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.crash.frames") }}</div>
                <div class="text-sm font-bold font-mono text-success">{{ crashResult.demangledCount }}/{{ crashResult.totalFrames }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.crash.culprit") }}</div>
                <div class="text-xs font-bold font-mono text-accent truncate">{{ crashResult.culpritCommit || "—" }}</div>
              </div>
            </div>
            <div v-if="crashResult.culpritFrame" class="p-2.5 rounded-lg border border-danger/30 bg-danger/5 text-[11px] space-y-0.5">
              <div class="text-danger font-bold">{{ t("maint.crash.culprit.frame") }}</div>
              <div class="font-mono text-ink truncate">{{ crashResult.culpritFrame.sourceFile }}:{{ crashResult.culpritFrame.lineNumber }} · {{ crashResult.culpritFrame.demangledSymbol }}</div>
              <div class="text-ink-muted">{{ crashResult.culpritAuthor || t("maint.crash.author.unknown") }} · {{ crashResult.culpritCommit || t("maint.crash.commit.unknown") }}</div>
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
                  <span v-if="frame.isCulprit" class="text-danger font-bold shrink-0">{{ t("maint.crash.culprit.frame") }}</span>
                </div>
                <div class="text-ink-muted pl-8 truncate">
                  {{ frame.sourceFile || t("maint.crash.file.unknown") }}:{{ frame.lineNumber || "?" }}
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
              <span>{{ t("maint.hotfix.title") }}</span>
            </h3>
            <p class="text-xs text-ink-muted mt-0.5">
              {{ t("maint.hotfix.desc") }}
            </p>
          </div>
          <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
            <div class="grid grid-cols-1 md:grid-cols-4 gap-2">
              <input
                v-model="hotfixBaseRef"
                type="text"
                :placeholder="t('maint.hotfix.base')"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <input
                v-model="hotfixTargetRef"
                type="text"
                :placeholder="t('maint.hotfix.target')"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <input
                v-model="hotfixAppVersion"
                type="text"
                :placeholder="t('maint.hotfix.version')"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
              />
              <button
                @click="handleGenerateHotfix"
                :disabled="isGeneratingHotfix"
                class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
              >
                {{ isGeneratingHotfix ? t("maint.hotfix.running") : t("maint.hotfix.run") }}
              </button>
            </div>
          </div>

          <template v-if="hotfixResult">
            <div class="grid grid-cols-4 gap-2">
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.hotfix.files") }}</div>
                <div class="text-sm font-bold font-mono text-ink">{{ hotfixResult.totalFilesCount }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.hotfix.size") }}</div>
                <div class="text-sm font-bold font-mono text-warning">{{ hotfixResult.totalPatchSizeStr }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.hotfix.compressed") }}</div>
                <div class="text-sm font-bold font-mono text-success">{{ hotfixResult.totalCompressedSizeStr }}</div>
              </div>
              <div class="p-2.5 rounded-lg border border-line bg-surface/30">
                <div class="text-[10px] text-ink-muted">{{ t("maint.hotfix.fullpack") }}</div>
                <div class="text-sm font-bold" :class="hotfixResult.fullPackRequired ? 'text-danger' : 'text-success'">
                  {{ hotfixResult.fullPackRequired ? t("maint.yes") : t("maint.no") }}
                </div>
              </div>
            </div>
            <div v-if="hotfixResult.fullPackRequired" class="p-2.5 rounded-lg border border-danger/30 bg-danger/5 text-[11px] text-danger">
              {{ t("maint.hotfix.fullpack.reason", { reason: hotfixResult.fullPackReason ?? "" }) }}
            </div>
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
              <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-1">
                <div class="text-xs font-bold text-ink">{{ t("maint.hotfix.tiers") }}</div>
                <div
                  v-for="tier in hotfixResult.tierBreakdown || []"
                  :key="tier.tier"
                  class="flex items-center gap-2 text-[10px] font-mono"
                >
                  <span class="text-ink flex-1 truncate">{{ tier.tier }}</span>
                  <span class="text-ink-muted shrink-0">{{ t("maint.hotfix.tier.files", { count: tier.count }) }}</span>
                  <span class="text-ink-muted shrink-0">{{ tier.totalSizeStr }}</span>
                </div>
                <div v-if="(hotfixResult.deltaPackageList || []).length > 0" class="pt-1 text-[10px] text-ink-muted">
                  {{ t("maint.hotfix.delta", { count: hotfixResult.deltaPackageList.length }) }}
                </div>
              </div>
              <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-ink">manifest JSON</span>
                  <button
                    @click="copyHotfixManifest"
                    class="px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[10px] text-accent transition cursor-pointer"
                  >
                    {{ t("maint.hotfix.copy") }}
                  </button>
                </div>
                <pre class="max-h-52 overflow-auto p-2 rounded bg-canvas border border-line/60 text-[10px] font-mono whitespace-pre-wrap break-all">{{ hotfixResult.manifestJson }}</pre>
              </div>
            </div>
            <p class="text-[10px] text-ink-muted">{{ hotfixResult.summary }}</p>
          </template>
        </div>
      </div>

    <!-- Git Bundle Delivery Wizard Modal -->
    <GitBundleModal
      v-model:open="showBundleModal"
      :current-repo-path="currentRepoPath"
    />
</template>
