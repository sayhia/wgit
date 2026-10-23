<script setup lang="ts">
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
          ? `隐私合规检查通过 (${privacyResult.value.storeReadinessGrade})`
          : `隐私合规存在 ${privacyResult.value.violations?.length ?? 0} 项问题 (${privacyResult.value.storeReadinessGrade})`
      );
    }
  } catch (err: any) {
    notify(`隐私合规审计失败: ${err?.message || err}`);
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
          ? `Git 认证正常：发现 ${authResult.value.sshKeysFound?.length ?? 0} 个 SSH 公钥`
          : `Git 认证可能异常：发现 ${authResult.value.sshKeysFound?.length ?? 0} 个 SSH 公钥`
      );
    }
  } catch (err: any) {
    notify(`Git 认证诊断失败: ${err?.message || err}`);
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
    notify(`JSON 解析失败: ${err?.message || err}`);
    return;
  }
  if (!Array.isArray(parsed)) {
    notify("自定义操作配置必须是数组");
    return;
  }
  isSavingActions.value = true;
  try {
    await gitService.saveCustomActions(props.currentRepoPath, parsed as CustomActionConfig[]);
    notify("自定义操作配置已保存");
    showActionsEditor.value = false;
    await loadCustomActions();
  } catch (err: any) {
    notify(`保存自定义操作失败: ${err?.message || err}`);
  } finally {
    isSavingActions.value = false;
  }
}

async function handleExecuteAction(action: CustomActionConfig) {
  if (!props.currentRepoPath || actionBusyId.value) return;
  const cmd = `${action.command} ${action.args}`.trim();
  if (!(await confirmAction(`确定执行自定义操作「${action.name}」吗？\n\n${cmd}`))) return;
  actionBusyId.value = action.id;
  try {
    actionResult.value = await gitService.executeCustomAction(props.currentRepoPath, action.id);
    if (actionResult.value?.success) {
      notify(`自定义操作「${action.name}」执行成功`);
    } else {
      notify(`自定义操作「${action.name}」退出码 ${actionResult.value?.exitCode ?? "?"}`);
    }
  } catch (err: any) {
    notify(`执行自定义操作失败: ${err?.message || err}`);
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
          ? "推送前检查：当前没有待推送提交"
          : prePushResult.value.isBlocked
            ? `推送前检查未通过：${prePushResult.value.violations.length} 项问题`
            : `推送前检查完成：${prePushResult.value.readinessGrade} (${prePushResult.value.passReadinessScore}/100)`
      );
    }
  } catch (err: any) {
    notify(`推送前检查失败: ${err?.message || err}`);
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
          ? "提交内容审计：未发现可审计提交"
          : safetyResult.value.passed
            ? `最近 ${safetyResult.value.commitsAudited} 条提交内容安全检查通过`
            : `发现 ${safetyResult.value.violationsCount} 条提交内容风险`
      );
    }
  } catch (err: any) {
    notify(`提交内容审计失败: ${err?.message || err}`);
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
  if (!(await confirmAction({ message: "确定清空当前会话的 Git 操作日志吗？", tone: "danger" }))) return;
  isClearingLogs.value = true;
  try {
    await gitService.clearActivityLogs();
    activityLogs.value = [];
    expandedLogId.value = null;
    notify("操作日志已清空");
  } catch (err: any) {
    notify(`清空操作日志失败: ${err?.message || err}`);
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
            <span>团队活跃度与贡献者热力 (Team Activity)</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            汇总最近 182 天的提交、活跃天数、连续提交与文件改动热点。
          </p>
        </div>
        <button
          @click="loadTeamActivity"
          :disabled="isLoadingTeam"
          class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
        >
          {{ isLoadingTeam ? "统计中..." : "重新统计" }}
        </button>
      </div>

      <div v-if="teamActivity" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">总提交</div>
          <div class="text-lg font-bold font-mono text-ink">{{ teamActivity.totalCommits }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">活跃天数</div>
          <div class="text-lg font-bold font-mono text-success">{{ teamActivity.activeDays }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">当前连续</div>
          <div class="text-lg font-bold font-mono text-warning">{{ teamActivity.currentStreak }} 天</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">高峰时段</div>
          <div class="text-sm font-bold text-ink">{{ teamActivity.peakHour || "—" }}</div>
        </div>
        <div class="p-3 rounded-xl border border-line bg-surface/30">
          <div class="text-[10px] text-ink-muted">日均提交</div>
          <div class="text-lg font-bold font-mono text-purple">{{ teamActivity.avgCommitsPerDay.toFixed(1) }}</div>
        </div>
      </div>

      <div v-if="teamActivity" class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
          <div class="text-xs font-bold text-ink">贡献者 ({{ teamActivity.contributors.length }})</div>
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
            <div v-if="topContributors.length === 0" class="py-6 text-center text-xs text-ink-muted">暂无贡献者数据</div>
          </div>
        </div>

        <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
          <div class="text-xs font-bold text-ink">改动热点 ({{ teamActivity.hotspots.length }})</div>
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
              <span class="font-mono text-ink-muted shrink-0">{{ h.changes }} 次</span>
            </div>
            <div v-if="topHotspots.length === 0" class="py-6 text-center text-xs text-ink-muted">暂无热点文件</div>
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
            <span>推送前检查清单</span>
          </h3>
          <button
            @click="handleAuditPrePush"
            :disabled="isAuditingPrePush"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isAuditingPrePush ? "检查中..." : "开始检查" }}
          </button>
        </div>
        <input
          v-model="prePushBranch"
          type="text"
          placeholder="目标分支（留空 = 当前分支）"
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
            <span>提交内容安全审计</span>
          </h3>
          <div class="flex items-center gap-2 shrink-0">
            <label class="flex items-center gap-1 text-[10px] text-ink-muted">
              最近
              <input
                v-model.number="safetyCount"
                type="number"
                min="1"
                max="200"
                class="w-12 px-1 py-0.5 rounded bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent"
              />
              条
            </label>
            <button
              @click="handleAuditSafety"
              :disabled="isAuditingSafety"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40"
            >
              {{ isAuditingSafety ? "审计中..." : "开始审计" }}
            </button>
          </div>
        </div>

        <div v-if="safetyResult" class="space-y-2">
          <div class="flex items-center gap-2">
            <span
              class="px-2 py-0.5 rounded text-[10px] font-bold"
              :class="!safetyResult.analysisKnown ? 'bg-raised text-ink-muted' : safetyResult.passed ? 'bg-success/15 text-success' : 'bg-danger/15 text-danger'"
            >
              {{ !safetyResult.analysisKnown ? "无数据" : safetyResult.passed ? "通过" : `${safetyResult.violationsCount} 项风险` }}
            </span>
            <span class="text-[10px] text-ink-muted">已审计 {{ safetyResult.commitsAudited }} 条提交</span>
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
            <span>Git 操作日志 ({{ activityLogs.length }})</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">当前会话执行的 Git 命令、耗时与输出（点击行展开详情）。</p>
        </div>
        <div class="flex items-center gap-2">
          <button aria-label="刷新日志"
            @click="loadActivityLogs"
            :disabled="isLoadingLogs"
            class="p-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-40"
            title="刷新日志"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingLogs }" />
          </button>
          <button aria-label="清空操作日志"
            @click="handleClearLogs"
            :disabled="isClearingLogs || activityLogs.length === 0"
            class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer disabled:opacity-40"
            title="清空操作日志"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div class="rounded-xl border border-line bg-surface/30 overflow-hidden font-mono text-[11px]">
        <div v-if="visibleLogs.length === 0" class="py-8 text-center text-xs text-ink-muted">
          {{ isLoadingLogs ? "正在读取日志..." : "暂无操作日志" }}
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
            <div class="text-ink-muted">目录: {{ log.dir }}</div>
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
            <span>自定义操作 (Custom Actions)</span>
          </h3>
          <p class="text-xs text-ink-muted mt-0.5">
            执行仓库级流水线脚本，配置保存在 `.git/wgit-actions.json`；执行前会确认命令内容。
          </p>
        </div>
        <button
          @click="toggleActionsEditor"
          class="flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
        >
          <Pencil class="w-3.5 h-3.5" />
          {{ showActionsEditor ? "收起配置" : "编辑配置" }}
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
          <p class="text-[10px] text-ink-muted">字段：id / name / description / command / args / workingDir / icon / showInToolbar。</p>
          <button
            @click="handleSaveActions"
            :disabled="isSavingActions"
            class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-50 shrink-0"
          >
            {{ isSavingActions ? "保存中..." : "保存配置" }}
          </button>
        </div>
      </div>

      <div v-if="isLoadingActions && customActions.length === 0" class="py-4 text-center text-xs text-ink-muted">
        正在读取自定义操作...
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
            {{ actionBusyId === action.id ? "执行中..." : "执行" }}
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
          <span>合规与认证诊断</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          检查移动端隐私清单 / 权限声明，以及本机 Git 凭据与 SSH 认证状态。
        </p>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-bold text-ink">隐私合规审计</span>
            <button
              @click="handleAuditPrivacy"
              :disabled="isAuditingPrivacy"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isAuditingPrivacy ? "审计中..." : "开始审计" }}
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
              <span class="text-ink-muted">{{ privacyResult.totalPermissionsDeclared }} 权限</span>
              <span class="text-ink-muted">{{ privacyResult.sensitivePermissionsCount }} 敏感</span>
              <span class="text-ink-muted">{{ privacyResult.trackingDomainsCount }} 跟踪域</span>
              <span :class="privacyResult.hasPrivacyManifest ? 'text-success' : 'text-warning'">
                {{ privacyResult.hasPrivacyManifest ? "隐私清单已配置" : "缺少隐私清单" }}
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
              <CheckCircle2 class="w-3.5 h-3.5" /> 未发现隐私合规问题
            </p>
            <p v-else class="text-[11px] text-ink-muted">
              当前工程未检测到可评估的移动端隐私清单，未作合规结论。
            </p>
            <p class="text-[10px] text-ink-muted">{{ privacyResult.summary }}</p>
          </div>
        </div>

        <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-bold text-ink">Git 认证诊断</span>
            <button
              @click="handleDiagnoseAuth"
              :disabled="isDiagnosingAuth"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ isDiagnosingAuth ? "诊断中..." : "开始诊断" }}
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
                {{ authResult.sshAuthSuccess ? "SSH 认证成功" : "SSH 认证未确认" }}
              </span>
              <span class="text-ink-muted font-mono">{{ authResult.host }}</span>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div class="p-2 rounded-lg bg-canvas/40 border border-line">
                <div class="text-[10px] text-ink-muted">SSH 公钥</div>
                <div class="text-ink">{{ (authResult.sshKeysFound || []).length }} 个</div>
              </div>
              <div class="p-2 rounded-lg bg-canvas/40 border border-line">
                <div class="text-[10px] text-ink-muted">Credential Helper</div>
                <div class="font-mono text-ink truncate">{{ authResult.credentialHelper || "未配置" }}</div>
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
