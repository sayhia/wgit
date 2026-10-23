<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch } from "vue";
import { Activity, CheckCircle2, ChevronDown, ChevronRight, Play, RefreshCw, Rocket } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type {
  TAPDWorkItemListResult,
  TAPDCommitReferenceResult,
  StudioCIResult,
  FullParityStatusSummary,
  PinnacleParityStatusSummary,
} from "../../types";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

const expanded = ref(false);
const tapd = ref<TAPDWorkItemListResult | null>(null);
const isLoadingTAPD = ref(false);
const bindCommitHash = ref("");
const bindTapdID = ref("");
const bindStatus = ref("resolved");
const isBinding = ref(false);
const parseMessage = ref("");
const parseResult = ref<TAPDCommitReferenceResult | null>(null);
const isParsing = ref(false);

const ci = ref<StudioCIResult | null>(null);
const isLoadingCI = ref(false);
const isTriggeringCI = ref(false);

const parityKind = ref<"" | "full" | "pinnacle">("");
const parity = ref<FullParityStatusSummary | PinnacleParityStatusSummary | null>(null);
const isLoadingParity = ref("");

async function loadTAPD() {
  if (!props.currentRepoPath || isLoadingTAPD.value) return;
  isLoadingTAPD.value = true;
  try {
    const [list, commits] = await Promise.all([
      gitService.fetchTAPDWorkItems(props.currentRepoPath),
      gitService.getCommits(props.currentRepoPath, 1).catch(() => []),
    ]);
    tapd.value = list;
    const head = commits[0];
    if (!bindCommitHash.value && head) {
      bindCommitHash.value = head.shortHash || head.hash;
    }
  } catch (err: any) {
    notify(`同步 TAPD 工作项失败: ${err?.message || err}`);
  } finally {
    isLoadingTAPD.value = false;
  }
}

async function loadCI() {
  if (!props.currentRepoPath || isLoadingCI.value) return;
  isLoadingCI.value = true;
  try {
    ci.value = await gitService.getStudioCIPipelines(props.currentRepoPath);
  } catch (err: any) {
    notify(`读取流水线失败: ${err?.message || err}`);
  } finally {
    isLoadingCI.value = false;
  }
}

function ensureLoaded() {
  void loadTAPD();
  void loadCI();
}

function toggleExpanded() {
  expanded.value = !expanded.value;
  if (expanded.value && !tapd.value && !ci.value) ensureLoaded();
}

async function handleBind() {
  if (!props.currentRepoPath || isBinding.value) return;
  if (!bindCommitHash.value.trim() || !bindTapdID.value.trim()) return;
  if (!(await confirmAction(`确定把提交 ${bindCommitHash.value.trim()} 关联到 TAPD #${bindTapdID.value.trim()} 并标记为 ${bindStatus.value} 吗？`)))
    return;
  isBinding.value = true;
  try {
    const res = await gitService.bindCommitToTAPD(
      props.currentRepoPath,
      bindCommitHash.value.trim(),
      bindTapdID.value.trim(),
      bindStatus.value
    );
    notify(res?.message || "提交已关联到 TAPD 工作项");
    await loadTAPD();
  } catch (err: any) {
    notify(`关联 TAPD 失败: ${err?.message || err}`);
  } finally {
    isBinding.value = false;
  }
}

async function handleParse() {
  const msg = parseMessage.value.trim();
  if (!msg || isParsing.value) return;
  isParsing.value = true;
  try {
    parseResult.value = await gitService.parseTAPDCommitReferences(msg);
    const count = parseResult.value?.foundTickets.length ?? 0;
    notify(count > 0 ? `解析到 ${count} 个 TAPD 单号` : "未解析到 TAPD 单号");
  } catch (err: any) {
    notify(`解析提交信息失败: ${err?.message || err}`);
  } finally {
    isParsing.value = false;
  }
}

async function handleTriggerCI() {
  if (!props.currentRepoPath || isTriggeringCI.value) return;
  if (!(await confirmAction("确定触发一次 Studio CI 流水线构建吗？"))) return;
  isTriggeringCI.value = true;
  try {
    const res = await gitService.triggerStudioCIPipeline(
      props.currentRepoPath,
      ci.value?.latestRun?.name || "Game-Client-Daily-Build"
    );
    notify(res?.message || "已触发流水线构建");
    await loadCI();
  } catch (err: any) {
    notify(`触发流水线失败: ${err?.message || err}`);
  } finally {
    isTriggeringCI.value = false;
  }
}

async function handleLoadParity(kind: "full" | "pinnacle") {
  if (!props.currentRepoPath || isLoadingParity.value) return;
  isLoadingParity.value = kind;
  try {
    parity.value =
      kind === "full"
        ? await gitService.getFullParityStatusSummary(props.currentRepoPath)
        : await gitService.getPinnacleParityStatusSummary(props.currentRepoPath);
    parityKind.value = kind;
  } catch (err: any) {
    notify(`读取能力清单失败: ${err?.message || err}`);
  } finally {
    isLoadingParity.value = "";
  }
}

watch(
  () => props.currentRepoPath,
  () => {
    tapd.value = null;
    ci.value = null;
    parity.value = null;
    parityKind.value = "";
    if (expanded.value) ensureLoaded();
  }
);
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="flex items-center justify-between pb-2 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Rocket class="w-4 h-4 text-purple" />
          <span>Studio 集成与流水线 (TAPD / CI)</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          同步敏捷工作项、关联提交单号、查看或触发 Studio CI，并核对后端能力清单。
        </p>
      </div>
      <button
        @click="toggleExpanded"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
      >
        <component :is="expanded ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
        <span>{{ expanded ? "收起集成" : "展开集成" }}</span>
      </button>
    </div>

    <div v-if="expanded" class="space-y-4">
      <!-- TAPD -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <span class="text-xs font-bold text-ink">TAPD 敏捷工作项</span>
            <span v-if="tapd" class="px-1.5 py-0.2 rounded bg-surface border border-line text-[10px] font-mono text-ink-muted">
              {{ tapd.workspaceId }}
            </span>
          </div>
          <button
            @click="loadTAPD"
            :disabled="isLoadingTAPD"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingTAPD }" />
            <span>{{ isLoadingTAPD ? "读取中..." : "读取工作项" }}</span>
          </button>
        </div>

        <template v-if="tapd">
          <p v-if="tapd.items.length === 0" class="text-[11px] text-ink-muted">{{ tapd.message }}</p>
          <template v-else>
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">全部</div>
              <div class="text-sm font-bold font-mono text-ink">{{ tapd.totalCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">需求</div>
              <div class="text-sm font-bold font-mono text-accent">{{ tapd.storiesCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">任务</div>
              <div class="text-sm font-bold font-mono text-purple">{{ tapd.tasksCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">缺陷</div>
              <div class="text-sm font-bold font-mono text-danger">{{ tapd.bugsCount }}</div>
            </div>
          </div>

          <div class="rounded-lg border border-line bg-canvas/30 divide-y divide-line/40 max-h-64 overflow-y-auto">
            <div v-for="item in tapd.items" :key="item.id" class="p-2.5 space-y-1">
              <div class="flex items-center gap-2">
                <span
                  class="px-1 py-0.2 rounded text-[9px] font-bold shrink-0"
                  :class="item.type === 'bug' ? 'bg-danger/15 text-danger' : item.type === 'story' ? 'bg-accent/15 text-accent' : 'bg-purple/15 text-purple'"
                >
                  {{ item.type === "bug" ? "缺陷" : item.type === "story" ? "需求" : "任务" }}
                </span>
                <span class="text-xs text-ink truncate flex-1" :title="item.title">{{ item.title }}</span>
                <span class="text-[10px] text-ink-muted shrink-0">{{ item.statusLabel }}</span>
              </div>
              <div class="flex items-center gap-2 text-[10px] text-ink-muted font-mono">
                <span>#{{ item.id }}</span>
                <span>{{ item.owner }}</span>
                <span class="truncate">{{ item.iteration }}</span>
                <span v-if="item.boundCommits.length > 0" class="text-success">{{ item.boundCommits.join(" · ") }}</span>
                <button
                  @click="bindTapdID = item.id"
                  class="ml-auto px-2 py-0.5 rounded bg-surface hover:bg-raised border border-line text-[10px] text-accent transition cursor-pointer shrink-0"
                >
                  填入单号
                </button>
              </div>
            </div>
          </div>

          </template>

          <div class="grid grid-cols-1 md:grid-cols-4 gap-2">
            <input
              v-model="bindCommitHash"
              type="text"
              placeholder="提交 Hash"
              class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <input
              v-model="bindTapdID"
              type="text"
              placeholder="TAPD 单号"
              class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <select
              v-model="bindStatus"
              class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
            >
              <option value="in_progress">处理中</option>
              <option value="resolved">已解决</option>
              <option value="verified">已验证</option>
              <option value="closed">已关闭</option>
            </select>
            <button
              @click="handleBind"
              :disabled="isBinding || !bindCommitHash.trim() || !bindTapdID.trim()"
              class="px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
            >
              {{ isBinding ? "关联中..." : "关联提交" }}
            </button>
          </div>
          <p class="text-[10px] text-ink-muted">关联仅写入本地 Git Notes，不会调用 TAPD API 或修改远端工单状态。</p>
        </template>

        <div class="flex items-center gap-2">
          <input
            v-model="parseMessage"
            type="text"
            placeholder="粘贴提交信息解析单号，例如 fix: crash #1002341"
            @keydown.enter="handleParse"
            class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
          <button
            @click="handleParse"
            :disabled="!parseMessage.trim() || isParsing"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
          >
            {{ isParsing ? "解析中..." : "解析单号" }}
          </button>
        </div>
        <div v-if="parseResult" class="flex flex-wrap items-center gap-1.5">
          <span v-if="parseResult.hasTapdTag" class="px-1.5 py-0.5 rounded bg-success/15 text-success text-[10px] font-mono">
            {{ parseResult.formattedTag }}
          </span>
          <span
            v-for="ticket in parseResult.foundTickets"
            :key="ticket.ticketId"
            class="px-1.5 py-0.5 rounded bg-surface border border-line text-[10px] font-mono text-ink-secondary"
          >
            #{{ ticket.ticketId }}
          </span>
          <span v-if="!parseResult.hasTapdTag" class="text-[11px] text-ink-muted">未发现 TAPD 单号</span>
        </div>
      </div>

      <!-- CI -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <Activity class="w-4 h-4 text-accent" />
            <span class="text-xs font-bold text-ink">Studio CI 流水线</span>
            <span v-if="ci" class="text-[10px] text-ink-muted font-mono">{{ ci.provider }} · {{ ci.activeBranch }}</span>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="loadCI"
              :disabled="isLoadingCI"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingCI }" />
              <span>刷新</span>
            </button>
            <button
              @click="handleTriggerCI"
              :disabled="isTriggeringCI || !ci?.latestRun"
              :title="ci?.latestRun ? '触发 GitHub Actions workflow_dispatch' : '需要先读取到至少一次 GitHub Actions 运行记录'"
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Play class="w-3.5 h-3.5" />
              <span>{{ isTriggeringCI ? "触发中..." : "触发构建" }}</span>
            </button>
          </div>
        </div>

        <template v-if="ci?.latestRun">
          <div class="flex flex-wrap items-center gap-2 text-[11px]">
            <span
              class="px-1.5 py-0.5 rounded font-bold"
              :class="ci.latestRun.status === 'SUCCESS' ? 'bg-success/15 text-success' : ci.latestRun.status === 'FAILED' ? 'bg-danger/15 text-danger' : 'bg-warning/15 text-warning'"
            >
              {{ ci.latestRun.status }}
            </span>
            <span class="font-mono text-ink">{{ ci.latestRun.id }}</span>
            <span class="text-ink-muted font-mono">{{ ci.latestRun.shortHash }}</span>
            <span class="text-ink-muted">{{ ci.latestRun.author }}</span>
            <span class="text-ink-muted ml-auto">{{ ci.latestRun.duration }}</span>
          </div>
          <div class="space-y-1">
            <div
              v-for="stage in ci.latestRun.stages"
              :key="stage.name"
              class="flex items-center gap-2 px-2 py-1 rounded bg-canvas/40 border border-line text-[11px]"
            >
              <CheckCircle2 class="w-3 h-3 shrink-0" :class="stage.status === 'SUCCESS' ? 'text-success' : 'text-warning'" />
              <span class="text-ink truncate flex-1">{{ stage.name }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ stage.duration }}</span>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ ci.message }}</p>
        </template>
        <p v-else-if="!isLoadingCI" class="text-[11px] text-ink-muted">{{ ci?.message || "尚未读取流水线状态。" }}</p>
      </div>

      <!-- Backend capability inventory -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div class="text-xs font-bold text-ink">后端能力清单（自报）</div>
            <div class="text-[10px] text-ink-muted">查看后端内置的 UGit 对齐轮次与领域清单，仅作盘点参考。</div>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="handleLoadParity('full')"
              :disabled="!!isLoadingParity"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              {{ isLoadingParity === "full" ? "读取中..." : "120 轮清单" }}
            </button>
            <button
              @click="handleLoadParity('pinnacle')"
              :disabled="!!isLoadingParity"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              {{ isLoadingParity === "pinnacle" ? "读取中..." : "140 轮清单" }}
            </button>
          </div>
        </div>

        <div v-if="parity" class="space-y-2">
          <div class="flex flex-wrap items-center gap-2 text-[11px]">
            <span class="px-1.5 py-0.5 rounded bg-accent/15 text-accent font-bold">
              {{ parity.completedRounds }}/{{ parity.totalRounds }} · {{ parity.parityPercentage }}%
            </span>
            <span class="text-ink-muted font-mono truncate">{{ parity.systemReadiness }}</span>
            <span class="text-ink-muted ml-auto font-mono">{{ parity.clientPlatform }}</span>
          </div>
          <div class="space-y-1">
            <div
              v-for="domain in parity.domains"
              :key="domain.domainId"
              class="flex items-center gap-2 px-2 py-1 rounded bg-canvas/40 border border-line text-[11px]"
            >
              <span class="text-ink truncate flex-1">{{ domain.domainName }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ domain.completedCount }}/{{ domain.totalModules }}</span>
              <span class="text-success font-mono shrink-0">{{ domain.completionRate }}%</span>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ parityKind === "pinnacle" ? "Pinnacle" : "Full" }} 清单为后端静态自报数据，不含实时校验，不代表第三方认证或生产就绪。</p>
        </div>
      </div>
    </div>
  </div>
</template>
