<script setup lang="ts">
import { ref, computed, watch, onMounted } from "vue";
import {
  Rocket,
  Tag,
  GitCommitHorizontal,
  ArrowUpCircle,
  ArrowDownCircle,
  RefreshCw,
  Copy,
  Check,
  AlertTriangle,
  CheckCircle2,
  AlertCircle,
  ClipboardCheck,
  Send,
  FileText,
  ShieldCheck,
  Sparkles,
  History,
} from "lucide-vue-next";
import { gitService } from "../services/gitService";
import type {
  ReleasePlan,
  ReleaseCreateResult,
  StudioChangelogResult,
  StudioChangelogCategory,
} from "../types";

const props = defineProps<{
  repoPath: string;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

// ---------------- release plan ----------------
const plan = ref<ReleasePlan | null>(null);
const isLoading = ref(false);
const loadError = ref("");

// ---------------- tag form ----------------
const versionInput = ref("");
const messageInput = ref("");
const pushAfterCreate = ref(true);
const remoteInput = ref("");
const signTag = ref(false);
const isCreating = ref(false);
const createError = ref("");
const createResult = ref<ReleaseCreateResult | null>(null);

// ---------------- studio changelog ----------------
const changelog = ref<StudioChangelogResult | null>(null);
const isLoadingChangelog = ref(false);
const changelogError = ref("");
const fromRefInput = ref("");
const toRefInput = ref("HEAD");
const copiedDoc = ref(false);
let copiedTimer: any = null;

// ---------------- toast ----------------
const toastMessage = ref("");
const showToast = ref(false);
let toastTimer: any = null;

function notify(msg: string) {
  toastMessage.value = msg;
  showToast.value = true;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    showToast.value = false;
  }, 2600);
}

const checks = computed(() => plan.value?.checks ?? []);
const passCount = computed(() => checks.value.filter((c) => c.status === "pass").length);
const warnCount = computed(() => checks.value.filter((c) => c.status === "warn").length);
const failCount = computed(() => checks.value.filter((c) => c.status === "fail").length);
const blockingChecks = computed(() => checks.value.filter((c) => c.blocking && c.status === "fail"));

const versionLooksValid = computed(() => /^[A-Za-z0-9][A-Za-z0-9._+-]*$/.test(versionInput.value.trim()));

const canCreate = computed(
  () =>
    !!plan.value &&
    !!versionInput.value.trim() &&
    versionLooksValid.value &&
    blockingChecks.value.length === 0 &&
    !isCreating.value
);

const createDisabledReason = computed(() => {
  if (!plan.value) return "暂无发布计划";
  if (!versionInput.value.trim()) return "请先填写版本号";
  if (!versionLooksValid.value) return "版本号只能包含字母、数字、点、下划线、加号或连字符";
  if (blockingChecks.value.length > 0) {
    return `存在 ${blockingChecks.value.length} 项阻塞检查，请先修复后再发布`;
  }
  return "";
});

const releaseSucceeded = computed(
  () => !!createResult.value?.success && (!pushAfterCreate.value || !!createResult.value?.pushed)
);

const changelogCategories = computed(() => changelog.value?.categories ?? []);

function statusLabel(status: string): string {
  if (status === "pass") return "通过";
  if (status === "warn") return "警告";
  return "失败";
}

function statusClass(status: string): string {
  if (status === "pass") return "bg-success/15 text-success border-success/30";
  if (status === "warn") return "bg-warning/15 text-warning border-warning/30";
  return "bg-danger/15 text-danger border-danger/30";
}

/** Short Chinese category name without the English suffix, e.g. "游戏玩法与系统逻辑". */
function shortCategory(cat: StudioChangelogCategory): string {
  const loose = cat as StudioChangelogCategory & { name?: string };
  const label = loose.category || loose.name || "其他改动";
  return label.split(" (")[0] ?? label;
}

/** Real backend uses commits[]; tolerate dev-preview mocks that send count. */
function categoryCount(cat: StudioChangelogCategory): number {
  const loose = cat as StudioChangelogCategory & { count?: number };
  if (Array.isArray(loose.commits)) return loose.commits.length;
  return typeof loose.count === "number" ? loose.count : 0;
}

function categoryIcon(cat: StudioChangelogCategory): string {
  return cat.icon || "•";
}

function resetChangelogInputs() {
  fromRefInput.value = "";
  toRefInput.value = "HEAD";
  changelog.value = null;
  changelogError.value = "";
}

async function generateChangelog() {
  if (!props.repoPath) return;
  isLoadingChangelog.value = true;
  changelogError.value = "";
  try {
    const res = await gitService.generateStudioChangelog(
      props.repoPath,
      fromRefInput.value.trim(),
      toRefInput.value.trim() || "HEAD"
    );
    changelog.value = res;
    if (res) {
      // Reflect the range actually used (backend auto-detects an empty fromRef).
      if (res.fromRef) fromRefInput.value = res.fromRef;
      if (res.toRef) toRefInput.value = res.toRef;
    }
  } catch (err: any) {
    changelog.value = null;
    changelogError.value = `生成变更日志失败: ${err?.message || err}`;
  } finally {
    isLoadingChangelog.value = false;
  }
}

async function loadPlan(preserveResult = true) {
  if (!props.repoPath) {
    plan.value = null;
    return;
  }
  isLoading.value = true;
  loadError.value = "";
  if (!preserveResult) createResult.value = null;
  createError.value = "";
  try {
    const res = await gitService.getReleasePlan(props.repoPath);
    plan.value = res ?? null;
    if (res) {
      versionInput.value = res.suggestedVersion || "";
      messageInput.value = res.suggestedVersion ? `Release ${res.suggestedVersion}` : "";
      fromRefInput.value = res.lastTag || "";
      toRefInput.value = "HEAD";
      await generateChangelog();
    } else {
      resetChangelogInputs();
    }
  } catch (err: any) {
    plan.value = null;
    loadError.value = `读取发布计划失败: ${err?.message || err}`;
    resetChangelogInputs();
  } finally {
    isLoading.value = false;
  }
}

async function handleCreateRelease() {
  if (!props.repoPath || !canCreate.value) return;
  isCreating.value = true;
  createError.value = "";
  createResult.value = null;
  try {
    const res = await gitService.createRelease(
      props.repoPath,
      versionInput.value.trim(),
      messageInput.value.trim(),
      pushAfterCreate.value,
      remoteInput.value.trim(),
      signTag.value
    );
    createResult.value = res;
    if (res?.success) {
      notify(res.pushed ? `标签 ${res.tag} 已创建并推送远端` : `标签 ${res.tag} 已创建`);
      // Emitting refresh lets the parent reload the plan (and the rest of the app)
      // without clearing the result banner shown below.
      emit("refresh");
    } else if (res) {
      createError.value = res.message || "创建发布标签失败";
    } else {
      createError.value = "创建发布标签失败：后端未返回结果";
    }
  } catch (err: any) {
    createError.value = `创建发布标签失败: ${err?.message || err}`;
  } finally {
    isCreating.value = false;
  }
}

async function copyChangelog() {
  const doc = changelog.value?.markdownDoc ?? "";
  if (!doc) return;
  try {
    await navigator.clipboard.writeText(doc);
    notify("发版变更日志 Markdown 已复制");
  } catch {
    notify("复制失败，请手动选中日志内容复制");
  }
  copiedDoc.value = true;
  if (copiedTimer) clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => {
    copiedDoc.value = false;
  }, 1600);
}

// Keep the default tag message in sync while the user has not customized it.
watch(versionInput, (value, previous) => {
  const autoPrevious = previous ? `Release ${previous}` : "";
  if (!messageInput.value.trim() || messageInput.value === autoPrevious) {
    messageInput.value = value ? `Release ${value}` : "";
  }
});

watch(
  () => props.repoPath,
  () => {
    plan.value = null;
    createResult.value = null;
    createError.value = "";
    loadError.value = "";
    resetChangelogInputs();
    void loadPlan();
  }
);

onMounted(() => {
  void loadPlan();
});

defineExpose({ loadPlan });
</script>

<template>
  <div class="space-y-5 relative">
    <!-- Header -->
    <div class="flex items-start justify-between gap-3 pb-3 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Rocket class="w-4 h-4 text-accent" />
          <span>发布向导与工作室变更日志</span>
          <span class="text-xs text-ink-muted font-normal">(发布前检查 · 版本标签 · Studio Changelog)</span>
        </h3>
        <p class="text-xs text-ink-muted mt-1">
          打标签前逐项完成发布前检查；版本号默认按提交类型自动建议，可创建附注标签并可选推送远端。
        </p>
      </div>

      <button
        @click="loadPlan(false)"
        :disabled="isLoading || !repoPath"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer shrink-0 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
        <span>刷新计划</span>
      </button>
    </div>

    <!-- No repo -->
    <div
      v-if="!repoPath"
      class="py-16 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl"
    >
      请先在左侧打开一个 Git 仓库后再使用发布向导。
    </div>

    <!-- Loading (first load) -->
    <div
      v-else-if="isLoading && !plan"
      class="py-16 flex flex-col items-center gap-3 text-xs text-ink-muted"
    >
      <RefreshCw class="w-5 h-5 animate-spin text-accent" />
      <span>正在生成发布计划与变更日志...</span>
    </div>

    <!-- Load error -->
    <div
      v-else-if="loadError"
      class="p-3 rounded-xl bg-danger/10 border border-danger/30 text-xs text-danger flex items-start gap-2"
    >
      <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
      <span class="flex-1 leading-relaxed">{{ loadError }}</span>
      <button
        @click="loadPlan(false)"
        class="shrink-0 px-2.5 py-1 rounded-md bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer"
      >
        重试
      </button>
    </div>

    <!-- Empty plan (empty repo / no history) -->
    <div
      v-else-if="!plan"
      class="py-16 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl flex flex-col items-center gap-2"
    >
      <History class="w-6 h-6 text-ink-muted/60" />
      <span>当前仓库暂无提交历史，无法生成发布计划。</span>
    </div>

    <!-- ============================= -->
    <!-- MAIN: checks (left) + form / changelog (right) -->
    <!-- ============================= -->
    <div v-else class="grid grid-cols-1 lg:grid-cols-5 gap-5">
      <!-- LEFT: plan overview + pre-flight checks -->
      <div class="lg:col-span-2 space-y-4">
        <!-- Overview -->
        <div class="p-4 rounded-xl border border-line bg-surface/30 flex flex-col gap-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs font-bold text-ink flex items-center gap-1.5">
              <GitCommitHorizontal class="w-3.5 h-3.5 text-accent" />
              <span>发布概况</span>
            </span>
            <span
              class="px-2 py-0.5 rounded-full border text-[10px] font-bold"
              :class="plan.ready
                ? 'bg-success/15 text-success border-success/30'
                : 'bg-danger/15 text-danger border-danger/30'"
            >
              {{ plan.ready ? "可以发布" : "存在阻塞项" }}
            </span>
          </div>

          <div class="grid grid-cols-2 gap-2 text-xs">
            <div class="p-2.5 rounded-lg bg-surface/50 border border-line flex flex-col gap-0.5">
              <span class="text-[10px] text-ink-muted">当前分支</span>
              <span class="font-mono font-bold text-ink truncate" :title="plan.branch">{{ plan.branch }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-surface/50 border border-line flex flex-col gap-0.5">
              <span class="text-[10px] text-ink-muted">上次发布 tag</span>
              <span class="font-mono font-bold text-ink truncate">{{ plan.lastTag || "无（首次发布）" }}</span>
            </div>
            <div class="p-2.5 rounded-lg bg-surface/50 border border-line flex flex-col gap-0.5">
              <span class="text-[10px] text-ink-muted">待发布提交</span>
              <span class="font-mono font-bold text-accent">{{ plan.commitCount }} 个</span>
            </div>
            <div class="p-2.5 rounded-lg bg-surface/50 border border-line flex flex-col gap-0.5">
              <span class="text-[10px] text-ink-muted">建议版本</span>
              <span class="font-mono font-bold text-success">
                {{ plan.suggestedVersion }}
                <span class="text-[10px] font-sans font-medium text-ink-muted">({{ plan.nextMinor ? "Minor" : "Patch" }})</span>
              </span>
            </div>
          </div>

          <div class="flex items-center gap-2 flex-wrap text-[11px]">
            <span
              v-if="plan.aheadOfRemote > 0"
              class="flex items-center gap-1 px-2 py-0.5 rounded-full bg-accent/10 border border-accent/30 text-accent font-mono font-bold"
              title="本地领先上游的提交数"
            >
              <ArrowUpCircle class="w-3 h-3" /> ↑{{ plan.aheadOfRemote }}
            </span>
            <span
              v-if="plan.behindRemote > 0"
              class="flex items-center gap-1 px-2 py-0.5 rounded-full bg-cyan/10 border border-cyan/30 text-cyan font-mono font-bold"
              title="本地落后上游的提交数"
            >
              <ArrowDownCircle class="w-3 h-3" /> ↓{{ plan.behindRemote }}
            </span>
            <span
              v-if="!plan.aheadOfRemote && !plan.behindRemote"
              class="flex items-center gap-1 px-2 py-0.5 rounded-full bg-surface border border-line text-ink-muted"
            >
              <CheckCircle2 class="w-3 h-3 text-success" /> 与远端同步
            </span>
            <span class="px-2 py-0.5 rounded-full bg-surface border border-line text-ink-muted">
              {{ failCount }} 失败 · {{ warnCount }} 警告
            </span>
          </div>

          <p class="text-[11px] text-ink-muted leading-relaxed">{{ plan.summary }}</p>
        </div>

        <!-- Pre-flight checks -->
        <div class="rounded-xl border border-line bg-surface/30 overflow-hidden">
          <div class="px-4 py-2.5 border-b border-line flex items-center justify-between gap-2">
            <span class="text-xs font-bold text-ink flex items-center gap-1.5">
              <ClipboardCheck class="w-3.5 h-3.5 text-accent" />
              <span>发布前检查</span>
            </span>
            <span class="text-[10px] text-ink-muted font-mono">
              {{ passCount }} 通过 / {{ warnCount }} 警告 / {{ failCount }} 失败
            </span>
          </div>

          <div class="divide-y divide-line/40">
            <div v-for="check in checks" :key="check.key" class="p-3 flex items-start gap-2.5">
              <CheckCircle2 v-if="check.status === 'pass'" class="w-4 h-4 text-success shrink-0 mt-0.5" />
              <AlertTriangle v-else-if="check.status === 'warn'" class="w-4 h-4 text-warning shrink-0 mt-0.5" />
              <AlertCircle v-else class="w-4 h-4 text-danger shrink-0 mt-0.5" />

              <div class="flex flex-col gap-0.5 min-w-0">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span class="text-xs font-semibold text-ink">{{ check.title }}</span>
                  <span class="px-1.5 py-0.2 rounded text-[9px] font-bold border" :class="statusClass(check.status)">
                    {{ statusLabel(check.status) }}
                  </span>
                  <span
                    v-if="check.blocking"
                    class="px-1.5 py-0.2 rounded text-[9px] font-bold bg-danger/15 text-danger border border-danger/30"
                    title="阻塞项未通过时不允许创建发布标签"
                  >
                    阻塞
                  </span>
                </div>
                <span class="text-[11px] text-ink-muted leading-relaxed">{{ check.detail }}</span>
              </div>
            </div>

            <div v-if="checks.length === 0" class="py-8 text-center text-xs text-ink-muted">
              暂无发布前检查结果
            </div>
          </div>
        </div>
      </div>

      <!-- RIGHT: version form + changelog -->
      <div class="lg:col-span-3 space-y-4">
        <!-- Version & tag form -->
        <div class="p-4 rounded-xl border border-line bg-surface/30 space-y-3">
          <div class="flex items-center justify-between gap-2 pb-2 border-b border-line">
            <span class="text-sm font-bold text-ink flex items-center gap-1.5">
              <Tag class="w-4 h-4 text-accent" />
              <span>版本与标签</span>
            </span>
            <span class="text-[10px] text-ink-muted font-mono">git tag -a &lt;version&gt; -m &lt;message&gt;</span>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="flex flex-col gap-1">
              <label class="text-xs text-ink-secondary font-medium">版本号 (Tag)</label>
              <input
                v-model="versionInput"
                type="text"
                placeholder="v2.5.0"
                class="px-3 py-2 rounded-lg bg-canvas border text-xs text-ink placeholder-ink-muted focus:outline-none font-mono"
                :class="versionLooksValid ? 'border-line focus:border-accent' : 'border-danger/60 focus:border-danger'"
              />
              <span class="text-[10px]" :class="versionLooksValid ? 'text-ink-muted' : 'text-danger'">
                建议 {{ plan.suggestedVersion }}（{{ plan.nextMinor ? "检测到新功能，建议升 Minor" : "以修复为主，建议升 Patch" }}）
              </span>
            </div>

            <div class="flex flex-col gap-1">
              <label class="text-xs text-ink-secondary font-medium">推送远程名称</label>
              <input
                v-model="remoteInput"
                type="text"
                placeholder="留空自动选择"
                :disabled="!pushAfterCreate"
                class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono disabled:opacity-50"
              />
              <span class="text-[10px] text-ink-muted">勾选「创建后推送」时使用；留空自动选择当前跟踪远程（回退 origin）。</span>
            </div>
          </div>

          <div class="flex flex-col gap-1">
            <label class="text-xs text-ink-secondary font-medium">标签说明 (Message)</label>
            <textarea
              v-model="messageInput"
              rows="2"
              placeholder="Release v2.5.0"
              class="px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans resize-y"
            />
          </div>

          <div class="flex items-center gap-4 flex-wrap">
            <label class="flex items-center gap-1.5 text-xs text-ink cursor-pointer select-none">
              <input v-model="pushAfterCreate" type="checkbox" class="rounded accent-accent" />
              <span class="flex items-center gap-1"><Send class="w-3 h-3 text-accent" />创建后推送标签到远端</span>
            </label>
            <label class="flex items-center gap-1.5 text-xs text-ink cursor-pointer select-none">
              <input v-model="signTag" type="checkbox" class="rounded accent-accent" />
              <span class="flex items-center gap-1"><ShieldCheck class="w-3 h-3 text-purple" />GPG / SSH 签名标签 (-s)</span>
            </label>
          </div>

          <div
            v-if="signTag"
            class="p-2.5 rounded-lg bg-purple/10 border border-purple/25 text-[11px] text-purple flex items-start gap-2"
          >
            <ShieldCheck class="w-3.5 h-3.5 shrink-0 mt-0.5" />
            <span>签名标签需要本机已配置 GPG 或 SSH 签名密钥，否则创建会失败。</span>
          </div>

          <!-- Create error -->
          <div
            v-if="createError"
            class="p-3 rounded-xl bg-danger/10 border border-danger/30 text-xs text-danger flex items-start gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
            <span class="flex-1 leading-relaxed">{{ createError }}</span>
          </div>

          <!-- Create result banner -->
          <div
            v-if="createResult"
            class="p-3 rounded-xl border flex flex-col gap-2"
            :class="releaseSucceeded ? 'bg-success/10 border-success/30' : 'bg-warning/10 border-warning/30'"
          >
            <div
              class="flex items-center gap-2 text-xs font-bold"
              :class="releaseSucceeded ? 'text-success' : 'text-warning'"
            >
              <CheckCircle2 v-if="releaseSucceeded" class="w-4 h-4 shrink-0" />
              <AlertTriangle v-else class="w-4 h-4 shrink-0" />
              <span>{{ createResult.message || "发布标签已创建" }}</span>
            </div>
            <div class="flex items-center gap-2 flex-wrap text-[11px]">
              <span class="px-2 py-0.5 rounded bg-canvas border border-line font-mono text-ink">
                tag: {{ createResult.tag }}
              </span>
              <span
                class="px-2 py-0.5 rounded border font-medium"
                :class="createResult.pushed
                  ? 'bg-success/15 text-success border-success/30'
                  : 'bg-surface text-ink-muted border-line'"
              >
                {{ createResult.pushed ? "已推送远端" : "仅本地标签" }}
              </span>
            </div>
            <pre
              v-if="createResult.pushOutput"
              class="max-h-32 overflow-auto rounded-lg bg-canvas border border-line p-2 font-mono text-[10px] leading-4 text-ink-secondary whitespace-pre-wrap"
            >{{ createResult.pushOutput }}</pre>
          </div>

          <!-- Create button -->
          <button
            @click="handleCreateRelease"
            :disabled="!canCreate"
            class="w-full flex items-center justify-center gap-2 py-2.5 rounded-xl bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer shadow-sm disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <RefreshCw v-if="isCreating" class="w-4 h-4 animate-spin" />
            <Rocket v-else class="w-4 h-4" />
            <span>{{ isCreating ? "正在创建发布标签..." : `创建发布标签${versionInput.trim() ? " " + versionInput.trim() : ""}` }}</span>
          </button>

          <p
            v-if="createDisabledReason"
            class="text-[11px] flex items-center gap-1"
            :class="blockingChecks.length > 0 || !versionLooksValid ? 'text-danger' : 'text-ink-muted'"
          >
            <AlertTriangle class="w-3 h-3 shrink-0" />
            <span>{{ createDisabledReason }}</span>
          </p>
        </div>

        <!-- Studio changelog -->
        <div class="p-4 rounded-xl border border-line bg-surface/30 space-y-3">
          <div class="flex items-center justify-between gap-2 pb-2 border-b border-line flex-wrap">
            <span class="text-sm font-bold text-ink flex items-center gap-1.5">
              <FileText class="w-4 h-4 text-purple" />
              <span>工作室变更日志</span>
              <span class="text-[11px] text-ink-muted font-normal">(Studio Changelog)</span>
            </span>
            <div class="flex items-center gap-2">
              <span
                v-if="changelog"
                class="px-1.5 py-0.5 rounded-full bg-purple/15 text-purple border border-purple/30 text-[10px] font-bold"
              >
                {{ changelog.totalCount }} 个提交
              </span>
              <button
                @click="copyChangelog"
                :disabled="!changelog?.markdownDoc"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-medium text-ink transition cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Check v-if="copiedDoc" class="w-3 h-3 text-success" />
                <Copy v-else class="w-3 h-3" />
                <span>{{ copiedDoc ? "已复制" : "复制 Markdown" }}</span>
              </button>
            </div>
          </div>

          <!-- Ref range controls -->
          <div class="flex items-end gap-2 flex-wrap">
            <div class="flex flex-col gap-1 flex-1 min-w-[150px]">
              <label class="text-[10px] text-ink-muted">起始 Ref (from)</label>
              <input
                v-model="fromRefInput"
                type="text"
                placeholder="留空自动探测上一个 tag"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-purple font-mono"
              />
            </div>
            <div class="flex flex-col gap-1 flex-1 min-w-[130px]">
              <label class="text-[10px] text-ink-muted">结束 Ref (to)</label>
              <input
                v-model="toRefInput"
                type="text"
                placeholder="HEAD"
                class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-purple font-mono"
              />
            </div>
            <button
              @click="generateChangelog"
              :disabled="isLoadingChangelog || !repoPath"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-strong hover:bg-purple-dark text-white text-[11px] font-semibold transition cursor-pointer shadow-xs disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingChangelog }" />
              <span>{{ isLoadingChangelog ? "生成中..." : "重新生成" }}</span>
            </button>
          </div>

          <!-- Category summary chips -->
          <div v-if="changelogCategories.length > 0" class="flex items-center gap-1.5 flex-wrap">
            <span
              v-for="(cat, idx) in changelogCategories"
              :key="`${shortCategory(cat)}-${idx}`"
              class="px-2 py-0.5 rounded-full border border-line bg-surface text-[10px] text-ink-secondary"
            >
              <span class="mr-0.5">{{ categoryIcon(cat) }}</span>{{ shortCategory(cat) }} · {{ categoryCount(cat) }}
            </span>
          </div>

          <!-- Markdown preview -->
          <div
            v-if="isLoadingChangelog"
            class="py-12 flex flex-col items-center gap-2 text-xs text-ink-muted"
          >
            <RefreshCw class="w-4 h-4 animate-spin text-purple" />
            <span>正在生成变更日志...</span>
          </div>

          <div
            v-else-if="changelogError"
            class="p-3 rounded-lg bg-danger/10 border border-danger/30 text-xs text-danger flex items-start gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
            <span class="flex-1 leading-relaxed">{{ changelogError }}</span>
          </div>

          <pre
            v-else-if="changelog?.markdownDoc"
            class="max-h-96 overflow-auto rounded-lg bg-canvas border border-line p-3 font-mono text-[11px] leading-5 text-ink-secondary whitespace-pre-wrap"
          >{{ changelog.markdownDoc }}</pre>

          <div
            v-else
            class="py-10 text-center text-xs text-ink-muted border border-dashed border-line rounded-lg flex flex-col items-center gap-2"
          >
            <FileText class="w-5 h-5 text-ink-muted/60" />
            <span>该区间暂无提交，或尚未生成变更日志。</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Toast -->
    <transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="transform translate-y-4 opacity-0"
      enter-to-class="transform translate-y-0 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="transform translate-y-0 opacity-100"
      leave-to-class="transform translate-y-4 opacity-0"
    >
      <div
        v-if="showToast"
        class="fixed bottom-6 left-1/2 -translate-x-1/2 bg-ink text-canvas text-xs px-4 py-2 rounded-full shadow-2xl flex items-center gap-2 z-50 font-medium select-none pointer-events-none"
      >
        <Sparkles class="w-3.5 h-3.5 text-accent" />
        <span>{{ toastMessage }}</span>
      </div>
    </transition>
  </div>
</template>
