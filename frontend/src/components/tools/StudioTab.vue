<script setup lang="ts">
import { t } from "../../i18n";
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted, watchEffect } from "vue";
import {
  Users,
  ShieldCheck,
  FileWarning,
  Terminal,
  Trash2,
  RefreshCw,
  Activity,
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  Play,
  Pencil,
  Wrench,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { toolsStatus } from "../../composables/toolsStatus";
import StudioIntegrationSection from "./StudioIntegrationSection.vue";
import StudioOpsSection from "./StudioOpsSection.vue";
import type {
  TeamActivityResult,
  PrePushChecklistResult,
  CommitSafetyFilterResult,
  GitCommandLog,
  CustomActionConfig,
  CustomActionResult,
  PrivacyComplianceResult,
  AuthDiagnosticResult,
} from "../../types";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

const teamActivity = ref<TeamActivityResult | null>(null);
const isLoadingTeam = ref(false);

const prePushBranch = ref("");
const prePushResult = ref<PrePushChecklistResult | null>(null);
const isAuditingPrePush = ref(false);

const safetyCount = ref(20);
const safetyResult = ref<CommitSafetyFilterResult | null>(null);
const isAuditingSafety = ref(false);

const activityLogs = ref<GitCommandLog[]>([]);
const isLoadingLogs = ref(false);
const isClearingLogs = ref(false);
const expandedLogId = ref<number | null>(null);

const topContributors = computed(() => teamActivity.value?.contributors.slice(0, 8) || []);
const topHotspots = computed(() => teamActivity.value?.hotspots.slice(0, 8) || []);
const visibleLogs = computed(() => activityLogs.value.slice(0, 30));

const customActions = ref<CustomActionConfig[]>([]);
const isLoadingActions = ref(false);
const actionBusyId = ref("");
const actionResult = ref<CustomActionResult | null>(null);
const showActionsEditor = ref(false);
const actionsJson = ref("");
const isSavingActions = ref(false);

const privacyResult = ref<PrivacyComplianceResult | null>(null);
const isAuditingPrivacy = ref(false);
const authHost = ref("github.com");
const authResult = ref<AuthDiagnosticResult | null>(null);
const isDiagnosingAuth = ref(false);

async function handleAuditPrivacy() {
  if (!props.currentRepoPath || isAuditingPrivacy.value) return;
  isAuditingPrivacy.value = true;
  try {
    privacyResult.value = await gitService.auditPrivacyAndCompliance(props.currentRepoPath);
    if (privacyResult.value) {
      notify(
        privacyResult.value.isStoreReady
          ? t("studio.toast.privacy.pass", { grade: privacyResult.value.storeReadinessGrade })
          : t("studio.toast.privacy.issues", { count: privacyResult.value.violations?.length ?? 0, grade: privacyResult.value.storeReadinessGrade })
      );
    }
  } catch (err: any) {
    notify(t("studio.toast.privacy.failed", { error: err?.message || err }));
  } finally {
    isAuditingPrivacy.value = false;
  }
}

async function handleDiagnoseAuth() {
  if (!props.currentRepoPath || isDiagnosingAuth.value) return;
  isDiagnosingAuth.value = true;
  try {
    authResult.value = await gitService.diagnoseGitAuth(
      props.currentRepoPath,
      authHost.value.trim() || "github.com"
    );
    if (authResult.value) {
      notify(
        authResult.value.sshAuthSuccess
          ? t("studio.toast.auth.ok", { count: authResult.value.sshKeysFound?.length ?? 0 })
          : t("studio.toast.auth.warn", { count: authResult.value.sshKeysFound?.length ?? 0 })
      );
    }
  } catch (err: any) {
    notify(t("studio.toast.auth.failed", { error: err?.message || err }));
  } finally {
    isDiagnosingAuth.value = false;
  }
}

async function loadCustomActions() {
  if (!props.currentRepoPath) return;
  isLoadingActions.value = true;
  try {
    customActions.value = (await gitService.getCustomActions(props.currentRepoPath)) || [];
  } catch (err) {
    console.error("Failed to load custom actions:", err);
  } finally {
    isLoadingActions.value = false;
  }
}

function toggleActionsEditor() {
  if (!showActionsEditor.value) {
    actionsJson.value = JSON.stringify(customActions.value, null, 2);
  }
  showActionsEditor.value = !showActionsEditor.value;
}

async function handleSaveActions() {
  if (!props.currentRepoPath || isSavingActions.value) return;
  let parsed: unknown;
  try {
    parsed = JSON.parse(actionsJson.value);
  } catch (err: any) {
    notify(t("studio.toast.json.failed", { error: err?.message || err }));
    return;
  }
  if (!Array.isArray(parsed)) {
    notify(t("studio.toast.actions.array"));
    return;
  }
  isSavingActions.value = true;
  try {
    await gitService.saveCustomActions(props.currentRepoPath, parsed as CustomActionConfig[]);
    notify(t("studio.toast.actions.saved"));
    showActionsEditor.value = false;
    await loadCustomActions();
  } catch (err: any) {
    notify(t("studio.toast.actions.save.failed", { error: err?.message || err }));
  } finally {
    isSavingActions.value = false;
  }
}

async function handleExecuteAction(action: CustomActionConfig) {
  if (!props.currentRepoPath || actionBusyId.value) return;
  const cmd = `${action.command} ${action.args}`.trim();
  if (!(await confirmAction(t("studio.confirm.action", { name: action.name, cmd })))) return;
  actionBusyId.value = action.id;
  try {
    actionResult.value = await gitService.executeCustomAction(props.currentRepoPath, action.id);
    if (actionResult.value?.success) {
      notify(t("studio.toast.action.done", { name: action.name }));
    } else {
      notify(t("studio.toast.action.exit", { name: action.name, code: actionResult.value?.exitCode ?? "?" }));
    }
  } catch (err: any) {
    notify(t("studio.toast.action.failed", { error: err?.message || err }));
  } finally {
    actionBusyId.value = "";
  }
}

async function loadTeamActivity() {
  if (!props.currentRepoPath) return;
  isLoadingTeam.value = true;
  try {
    teamActivity.value = await gitService.getTeamActivity(props.currentRepoPath, 182);
  } catch (err) {
    console.error("Failed to load team activity:", err);
  } finally {
    isLoadingTeam.value = false;
  }
}

async function handleAuditPrePush() {
  if (!props.currentRepoPath) return;
  isAuditingPrePush.value = true;
  try {
    prePushResult.value = await gitService.auditPrePushChecklist(
      props.currentRepoPath,
      prePushBranch.value.trim()
    );
    if (prePushResult.value) {
      notify(
        !prePushResult.value.analysisKnown
          ? t("studio.toast.prepush.none")
          : prePushResult.value.isBlocked
            ? t("studio.toast.prepush.issues", { count: prePushResult.value.violations.length })
            : t("studio.toast.prepush.done", { grade: prePushResult.value.readinessGrade, score: prePushResult.value.passReadinessScore })
      );
    }
  } catch (err: any) {
    notify(t("studio.toast.prepush.failed", { error: err?.message || err }));
  } finally {
    isAuditingPrePush.value = false;
  }
}

async function handleAuditSafety() {
  if (!props.currentRepoPath) return;
  isAuditingSafety.value = true;
  try {
    safetyResult.value = await gitService.auditCommitContentSafety(
      props.currentRepoPath,
      safetyCount.value
    );
    if (safetyResult.value) {
      notify(
        !safetyResult.value.analysisKnown
          ? t("studio.toast.safety.none")
          : safetyResult.value.passed
            ? t("studio.toast.safety.pass", { count: safetyResult.value.commitsAudited })
            : t("studio.toast.safety.risk", { count: safetyResult.value.violationsCount })
      );
    }
  } catch (err: any) {
    notify(t("studio.toast.safety.failed", { error: err?.message || err }));
  } finally {
    isAuditingSafety.value = false;
  }
}

async function loadActivityLogs() {
  isLoadingLogs.value = true;
  try {
    activityLogs.value = (await gitService.getActivityLogs()) || [];
  } catch (err) {
    console.error("Failed to load activity logs:", err);
  } finally {
    isLoadingLogs.value = false;
  }
}

async function handleClearLogs() {
  if (isClearingLogs.value) return;
  if (!(await confirmAction({ message: t("studio.confirm.clear.logs"), tone: "danger" }))) return;
  isClearingLogs.value = true;
  try {
    await gitService.clearActivityLogs();
    activityLogs.value = [];
    expandedLogId.value = null;
    notify(t("studio.toast.logs.cleared"));
  } catch (err: any) {
    notify(t("studio.toast.logs.clear.failed", { error: err?.message || err }));
  } finally {
    isClearingLogs.value = false;
  }
}

function toggleLog(id: number) {
  expandedLogId.value = expandedLogId.value === id ? null : id;
}

function gradeClass(grade: string) {
  if (grade === "PASS") return "bg-success/15 text-success";
  if (grade === "PASS_WITH_WARNINGS") return "bg-warning/15 text-warning";
  if (grade === "NO_COMMITS") return "bg-raised text-ink-muted";
  return "bg-danger/15 text-danger";
}

watchEffect(() => {
  toolsStatus.loading.studio =
    isLoadingTeam.value ||
    isAuditingPrePush.value ||
    isAuditingSafety.value ||
    isLoadingLogs.value ||
    isClearingLogs.value ||
    isLoadingActions.value ||
    isSavingActions.value ||
    !!actionBusyId.value ||
    isAuditingPrivacy.value ||
    isDiagnosingAuth.value;
});

onMounted(() => {
  void loadTeamActivity();
  void loadActivityLogs();
  void loadCustomActions();
});

watch(
  () => props.refreshNonce,
  () => {
    void loadTeamActivity();
    void loadActivityLogs();
    void loadCustomActions();
  }
);
</script>

<template>
  <div class="space-y-6">
    <!-- Section A: Team activity -->
    <div class="space-y-3" data-tool-section="studio-team">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Users class="w-4 h-4 text-accent" />
            <span>{{ t("studio.team.title") }}</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            {{ t("studio.team.desc") }}
          </p>
        </div>
        <button
          @click="loadTeamActivity"
          :disabled="isLoadingTeam"
          class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
        >
          {{ isLoadingTeam ? t("studio.team.loading") : t("studio.team.reload") }}
        </button>
      </div>

      <div v-if="teamActivity" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">{{ t("studio.team.commits") }}</div>
          <div class="text-lg font-bold font-mono text-ink">{{ teamActivity.totalCommits }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">{{ t("studio.team.active.days") }}</div>
          <div class="text-lg font-bold font-mono text-success">{{ teamActivity.activeDays }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">{{ t("studio.team.streak") }}</div>
          <div class="text-lg font-bold font-mono text-warning">{{ t("studio.team.streak.value", { count: teamActivity.currentStreak }) }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">{{ t("studio.team.peak") }}</div>
          <div class="text-sm font-bold text-ink">{{ teamActivity.peakHour || "—" }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">{{ t("studio.team.per.day") }}</div>
          <div class="text-lg font-bold font-mono text-purple">{{ teamActivity.avgCommitsPerDay.toFixed(1) }}</div>
        </div>
      </div>

      <div v-if="teamActivity" class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
          <div class="text-xs font-bold text-ink">{{ t("studio.team.contributors", { count: teamActivity.contributors.length }) }}</div>
          <div class="space-y-1.5 max-h-56 overflow-y-auto">
            <div
              v-for="c in topContributors"
              :key="c.email || c.name"
              class="flex items-center gap-2 text-[11px]"
            >
              <span class="w-5 h-5 rounded-full bg-accent/15 text-accent text-[9px] font-bold flex items-center justify-center shrink-0">
                {{ c.name.slice(0, 1).toUpperCase() }}
              </span>
              <span class="text-ink truncate flex-1">{{ c.name }}</span>
              <span class="font-mono text-ink-muted shrink-0">{{ c.commits }} commits</span>
              <span class="font-mono text-[10px] text-success shrink-0">+{{ c.additions }}</span>
              <span class="font-mono text-[10px] text-danger shrink-0">-{{ c.deletions }}</span>
            </div>
            <div v-if="topContributors.length === 0" class="py-6 text-center text-xs text-ink-muted">{{ t("studio.team.no.contributors") }}</div>
          </div>
        </div>

        <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
          <div class="text-xs font-bold text-ink">{{ t("studio.team.hotspots", { count: teamActivity.hotspots.length }) }}</div>
          <div class="space-y-1.5 max-h-56 overflow-y-auto">
            <div
              v-for="h in topHotspots"
              :key="h.path"
              class="flex items-center gap-2 text-[11px]"
            >
              <span
                class="w-8 text-center px-1 py-0.5 rounded text-[9px] font-bold shrink-0"
                :class="h.riskScore >= 70 ? 'bg-danger/15 text-danger' : h.riskScore >= 40 ? 'bg-warning/15 text-warning' : 'bg-surface border border-line text-ink-muted'"
              >
                {{ h.riskScore }}
              </span>
              <span class="font-mono text-ink-secondary truncate flex-1" :title="h.path">{{ h.path }}</span>
              <span class="font-mono text-ink-muted shrink-0">{{ t("studio.team.hotspot.changes", { count: h.changes }) }}</span>
            </div>
            <div v-if="topHotspots.length === 0" class="py-6 text-center text-xs text-ink-muted">{{ t("studio.team.no.hotspots") }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Section B/C: Compliance audits -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-3" data-tool-section="studio-audits">
      <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3">
        <div class="flex items-center justify-between gap-2">
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <ShieldCheck class="w-4 h-4 text-success" />
            <span>{{ t("studio.prepush.title") }}</span>
          </h3>
          <button
            @click="handleAuditPrePush"
            :disabled="isAuditingPrePush"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isAuditingPrePush ? t("studio.auditing") : t("studio.run.audit") }}
          </button>
        </div>
        <input
          v-model="prePushBranch"
          type="text"
          :placeholder="t('studio.prepush.placeholder')"
          class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
        />

        <div v-if="prePushResult" class="space-y-2">
          <div class="flex items-center gap-2">
            <span class="px-2 py-0.5 rounded text-[10px] font-bold" :class="gradeClass(prePushResult.readinessGrade)">
              {{ prePushResult.readinessGrade }} · {{ prePushResult.analysisKnown ? `${prePushResult.passReadinessScore}/100` : "—" }}
            </span>
            <span class="text-[10px] text-ink-muted font-mono">{{ prePushResult.branchName }} · {{ prePushResult.commitsAudited }} commits</span>
          </div>
          <div class="space-y-1 max-h-48 overflow-y-auto">
            <div
              v-for="rule in prePushResult.rulesChecked"
              :key="rule.ruleId"
              class="flex items-start gap-2 text-[11px]"
            >
              <CheckCircle2 v-if="rule.passed" class="w-3.5 h-3.5 text-success shrink-0 mt-0.5" />
              <AlertTriangle v-else class="w-3.5 h-3.5 text-danger shrink-0 mt-0.5" />
              <div class="min-w-0">
                <div :class="rule.passed ? 'text-ink-secondary' : 'text-danger font-semibold'">{{ rule.ruleName }}</div>
                <div v-if="!rule.passed && rule.details" class="text-[10px] text-ink-muted">{{ rule.details }}</div>
              </div>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ prePushResult.summary || prePushResult.message }}</p>
        </div>
      </div>

      <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3">
        <div class="flex items-center justify-between gap-2">
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <FileWarning class="w-4 h-4 text-warning" />
            <span>{{ t("studio.safety.title") }}</span>
          </h3>
          <div class="flex items-center gap-2 shrink-0">
            <label class="flex items-center gap-1 text-[10px] text-ink-muted">
              {{ t("studio.safety.recent") }}
              <input
                v-model.number="safetyCount"
                type="number"
                min="1"
                max="200"
                class="w-12 px-1 py-0.5 rounded bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
              />
              {{ t("studio.safety.commits") }}
            </label>
            <button
              @click="handleAuditSafety"
              :disabled="isAuditingSafety"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
            >
              {{ isAuditingSafety ? t("studio.auditing") : t("studio.run.audit") }}
            </button>
          </div>
        </div>

        <div v-if="safetyResult" class="space-y-2">
          <div class="flex items-center gap-2">
            <span
              class="px-2 py-0.5 rounded text-[10px] font-bold"
              :class="!safetyResult.analysisKnown ? 'bg-raised text-ink-muted' : safetyResult.passed ? 'bg-success/15 text-success' : 'bg-danger/15 text-danger'"
            >
              {{ !safetyResult.analysisKnown ? t("studio.safety.no.data") : safetyResult.passed ? t("studio.safety.pass") : t("studio.safety.risk", { count: safetyResult.violationsCount }) }}
            </span>
            <span class="text-[10px] text-ink-muted">{{ t("studio.safety.audited", { count: safetyResult.commitsAudited }) }}</span>
          </div>
          <div v-if="safetyResult.violations.length > 0" class="space-y-1.5 max-h-48 overflow-y-auto">
            <div
              v-for="v in safetyResult.violations"
              :key="`${v.commitHash}:${v.ruleTriggered}`"
              class="px-2 py-1.5 rounded-lg bg-danger/5 border border-danger/20 text-[11px] space-y-0.5"
            >
              <div class="flex items-center gap-2">
                <span class="font-mono text-danger text-[10px]">{{ v.commitHash.substring(0, 8) }}</span>
                <span class="font-semibold text-ink">{{ v.ruleTriggered }}</span>
              </div>
              <div class="text-ink-muted truncate">{{ v.subject }}</div>
              <div class="font-mono text-[10px] text-danger/80 truncate">{{ v.offendingText }}</div>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ safetyResult.summary }}</p>
        </div>
      </div>
    </div>

    <!-- Section D: Activity logs -->
    <div class="space-y-3" data-tool-section="studio-logs">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Terminal class="w-4 h-4 text-purple" />
            <span>{{ t("studio.logs.title", { count: activityLogs.length }) }}</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">{{ t("studio.logs.desc") }}</p>
        </div>
        <div class="flex items-center gap-2">
          <button :aria-label="t('studio.logs.refresh')"
            @click="loadActivityLogs"
            :disabled="isLoadingLogs"
            class="p-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-40"
            :title="t('studio.logs.refresh')"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingLogs }" />
          </button>
          <button :aria-label="t('studio.logs.clear')"
            @click="handleClearLogs"
            :disabled="isClearingLogs || activityLogs.length === 0"
            class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer disabled:opacity-40"
            :title="t('studio.logs.clear')"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div class="rounded-xl border border-line bg-surface/30 overflow-hidden font-mono text-[11px]">
        <div v-if="visibleLogs.length === 0" class="py-8 text-center text-xs text-ink-muted">
          {{ isLoadingLogs ? t("studio.logs.loading") : t("studio.logs.empty") }}
        </div>
        <div
          v-for="log in visibleLogs"
          :key="log.id"
          class="border-b border-line/40 last:border-b-0"
        >
          <button
            @click="toggleLog(log.id)"
            class="w-full px-3 py-1.5 flex items-center gap-2 text-left hover:bg-raised/40 transition cursor-pointer"
          >
            <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="log.success ? 'bg-success-strong' : 'bg-danger-strong'"></span>
            <span class="text-ink-muted shrink-0">{{ log.time }}</span>
            <span class="text-ink truncate flex-1">{{ log.command }}</span>
            <span class="text-ink-muted shrink-0">{{ log.durationMs }}ms</span>
            <ChevronDown class="w-3 h-3 text-ink-muted shrink-0 transition-transform" :class="{ 'rotate-180': expandedLogId === log.id }" />
          </button>
          <div v-if="expandedLogId === log.id" class="px-3 pb-2.5 space-y-1 text-[10px]">
            <div class="text-ink-muted">{{ t("studio.logs.dir", { dir: log.dir }) }}</div>
            <pre v-if="log.output" class="p-2 rounded bg-canvas/70 border border-line/60 text-ink-secondary whitespace-pre-wrap max-h-40 overflow-y-auto">{{ log.output }}</pre>
            <pre v-if="log.error" class="p-2 rounded bg-danger/10 border border-danger/20 text-danger whitespace-pre-wrap max-h-40 overflow-y-auto">{{ log.error }}</pre>
          </div>
        </div>
      </div>
    </div>

    <!-- Section E: Custom actions -->
    <div class="space-y-3" data-tool-section="studio-custom-actions">
      <div class="flex items-center justify-between pb-2 border-b border-line">
        <div>
          <h3 class="text-sm font-bold text-ink flex items-center gap-2">
            <Wrench class="w-4 h-4 text-purple" />
            <span>{{ t("studio.actions.title") }}</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            {{ t("studio.actions.desc") }}
          </p>
        </div>
        <button
          @click="toggleActionsEditor"
          class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
        >
          <Pencil class="w-3.5 h-3.5" />
          {{ showActionsEditor ? t("studio.actions.collapse") : t("studio.actions.edit") }}
        </button>
      </div>

      <div v-if="showActionsEditor" class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
        <textarea
          v-model="actionsJson"
          rows="12"
          spellcheck="false"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono leading-relaxed focus:outline-none focus:border-accent resize-y"
        ></textarea>
        <div class="flex items-center justify-between gap-3">
          <p class="text-[10px] text-ink-muted">{{ t("studio.actions.fields") }}</p>
          <button
            @click="handleSaveActions"
            :disabled="isSavingActions"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-50 shrink-0"
          >
            {{ isSavingActions ? t("studio.saving") : t("studio.actions.save") }}
          </button>
        </div>
      </div>

      <div v-if="isLoadingActions && customActions.length === 0" class="py-4 text-center text-xs text-ink-muted">
        {{ t("studio.actions.loading") }}
      </div>
      <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-2">
        <div
          v-for="action in customActions"
          :key="action.id"
          class="p-3 rounded-xl border border-line bg-surface/30 flex flex-col gap-2"
        >
          <div class="flex items-center gap-2">
            <span class="text-xs font-bold text-ink truncate">{{ action.name }}</span>
            <span class="ml-auto text-[9px] font-mono text-ink-muted shrink-0">{{ action.id }}</span>
          </div>
          <div class="text-[10px] text-ink-muted">{{ action.description }}</div>
          <div class="px-2 py-1 rounded bg-canvas/60 border border-line/60 font-mono text-[10px] text-ink-secondary truncate" :title="`${action.command} ${action.args}`">
            $ {{ action.command }} {{ action.args }}
          </div>
          <button
            @click="handleExecuteAction(action)"
            :disabled="!!actionBusyId"
            class="self-start flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-accent transition cursor-pointer disabled:opacity-40"
          >
            <Play class="w-3 h-3" />
            {{ actionBusyId === action.id ? t("studio.actions.running") : t("studio.actions.run") }}
          </button>
        </div>
      </div>

      <div v-if="actionResult" class="rounded-xl border border-line bg-canvas/50 p-3 space-y-1.5 font-mono text-[10px]">
        <div class="flex items-center gap-2">
          <span class="px-1.5 py-0.5 rounded font-bold" :class="actionResult.success ? 'bg-success/15 text-success' : 'bg-danger/15 text-danger'">
            {{ actionResult.success ? "SUCCESS" : "FAILED" }}
          </span>
          <span class="text-ink-muted">{{ actionResult.command }}</span>
          <span class="ml-auto text-ink-muted">{{ actionResult.durationMs }}ms · exit {{ actionResult.exitCode }}</span>
        </div>
        <pre v-if="actionResult.output" class="p-2 rounded bg-surface border border-line/60 text-ink-secondary whitespace-pre-wrap max-h-48 overflow-y-auto">{{ actionResult.output }}</pre>
      </div>
    </div>

    <!-- Section F: Compliance & auth diagnostics -->
    <div class="space-y-3" data-tool-section="studio-compliance">
      <div class="pb-2 border-b border-line">
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <ShieldCheck class="w-4 h-4 text-success" />
          <span>{{ t("studio.compliance.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("studio.compliance.desc") }}
        </p>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-bold text-ink">{{ t("studio.privacy.title") }}</span>
            <button
              @click="handleAuditPrivacy"
              :disabled="isAuditingPrivacy"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isAuditingPrivacy ? t("studio.auditing") : t("studio.run.audit") }}
            </button>
          </div>
          <div v-if="privacyResult" class="space-y-2">
            <div class="flex flex-wrap items-center gap-2 text-[11px]">
              <span
                class="px-2 py-0.5 rounded font-bold"
                :class="privacyResult.isStoreReady ? 'bg-success/15 text-success' : privacyResult.storeReadinessGrade === 'FAIL' ? 'bg-danger/15 text-danger' : 'bg-warning/15 text-warning'"
              >
                {{ privacyResult.storeReadinessGrade }} · {{ privacyResult.platform }}
              </span>
              <span class="text-ink-muted">{{ t("studio.privacy.permissions", { count: privacyResult.totalPermissionsDeclared }) }}</span>
              <span class="text-ink-muted">{{ t("studio.privacy.sensitive", { count: privacyResult.sensitivePermissionsCount }) }}</span>
              <span class="text-ink-muted">{{ t("studio.privacy.tracking", { count: privacyResult.trackingDomainsCount }) }}</span>
              <span :class="privacyResult.hasPrivacyManifest ? 'text-success' : 'text-warning'">
                {{ privacyResult.hasPrivacyManifest ? t("studio.privacy.manifest.ok") : t("studio.privacy.manifest.missing") }}
              </span>
            </div>
            <div v-if="(privacyResult.violations || []).length > 0" class="space-y-1 max-h-40 overflow-y-auto">
              <div
                v-for="v in privacyResult.violations || []"
                :key="`${v.ruleId}:${v.offenderFile}`"
                class="px-2 py-1.5 rounded-lg bg-warning/5 border border-warning/20 text-[10px]"
              >
                <div class="flex items-center gap-2">
                  <span class="px-1 py-0.2 rounded text-[9px] font-bold" :class="v.severity === 'BLOCKING' ? 'bg-danger/15 text-danger' : 'bg-warning/15 text-warning'">
                    {{ v.severity }}
                  </span>
                  <span class="text-ink truncate">{{ v.description }}</span>
                </div>
                <div class="text-ink-muted font-mono truncate mt-0.5">{{ v.offenderFile }}</div>
              </div>
            </div>
            <p v-else-if="privacyResult.isStoreReady" class="text-[11px] text-success flex items-center gap-1.5">
              <CheckCircle2 class="w-3.5 h-3.5" /> {{ t("studio.privacy.ok") }}
            </p>
            <p v-else class="text-[11px] text-ink-muted">
              {{ t("studio.privacy.unknown") }}
            </p>
            <p class="text-[10px] text-ink-muted">{{ privacyResult.summary }}</p>
          </div>
        </div>

        <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-bold text-ink">{{ t("studio.auth.title") }}</span>
            <button
              @click="handleDiagnoseAuth"
              :disabled="isDiagnosingAuth"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isDiagnosingAuth ? t("studio.diagnosing") : t("studio.auth.run") }}
            </button>
          </div>
          <input
            v-model="authHost"
            type="text"
            placeholder="github.com / gitlab.com / git.company.com"
            class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <div v-if="authResult" class="space-y-2 text-[11px]">
            <div class="flex items-center gap-2">
              <span
                class="px-2 py-0.5 rounded font-bold"
                :class="authResult.sshAuthSuccess ? 'bg-success/15 text-success' : 'bg-warning/15 text-warning'"
              >
                {{ authResult.sshAuthSuccess ? t("studio.auth.ssh.ok") : t("studio.auth.ssh.unknown") }}
              </span>
              <span class="text-ink-muted font-mono">{{ authResult.host }}</span>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div class="p-2 rounded-lg bg-canvas/40 border border-line">
                <div class="text-[10px] text-ink-muted">{{ t("studio.auth.ssh.keys") }}</div>
                <div class="text-ink">{{ t("studio.auth.keys.count", { count: (authResult.sshKeysFound || []).length }) }}</div>
              </div>
              <div class="p-2 rounded-lg bg-canvas/40 border border-line">
                <div class="text-[10px] text-ink-muted">Credential Helper</div>
                <div class="font-mono text-ink truncate">{{ authResult.credentialHelper || t("studio.auth.not.configured") }}</div>
              </div>
            </div>
            <div v-if="(authResult.sshKeysFound || []).length > 0" class="flex flex-wrap gap-1">
              <span
                v-for="key in authResult.sshKeysFound || []"
                :key="key"
                class="px-1.5 py-0.5 rounded bg-surface border border-line font-mono text-[10px] text-ink-secondary"
              >
                {{ key }}
              </span>
            </div>
            <pre v-if="authResult.sshAuthOutput" class="p-2 rounded bg-canvas/70 border border-line/60 text-[10px] text-ink-muted whitespace-pre-wrap max-h-32 overflow-y-auto">{{ authResult.sshAuthOutput }}</pre>
          </div>
        </div>
      </div>
    </div>

    <StudioIntegrationSection :current-repo-path="currentRepoPath" @notify="notify" />
    <StudioOpsSection :current-repo-path="currentRepoPath" @notify="notify" />
  </div>
</template>
