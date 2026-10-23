<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch } from "vue";
import {
  ChevronDown,
  ChevronRight,
  Database,
  KeyRound,
  Layers,
  Lock,
  RefreshCw,
  ShieldCheck,
  Trash2,
  Play,
  Webhook,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type {
  CredentialVaultResult,
  StoreSecretResult,
  StudioWebhookResult,
  StudioDDCResult,
  OfflineQueueResult,
  StudioSwitchboardResult,
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

const vault = ref<CredentialVaultResult | null>(null);
const isLoadingVault = ref(false);
const secretType = ref("GIT_TOKEN");
const secretIdentifier = ref("");
const secretValue = ref("");
const secretScope = ref("LOCAL_REPO");
const isStoringSecret = ref(false);

const webhooks = ref<StudioWebhookResult | null>(null);
const isLoadingWebhooks = ref(false);
const webhookType = ref("WECOM");
const webhookUrl = ref("");
const isSavingWebhook = ref(false);

const ddc = ref<StudioDDCResult | null>(null);
const isLoadingDDC = ref(false);
const isPruningDDC = ref(false);

const queue = ref<OfflineQueueResult | null>(null);
const isLoadingQueue = ref(false);
const queueBusy = ref<"" | "enqueue" | "replay" | "pause">("");
const queueOperation = ref("fetch");
const queueTarget = ref("");

const switchboard = ref<StudioSwitchboardResult | null>(null);
const isLoadingSwitchboard = ref(false);

async function loadVault() {
  if (!props.currentRepoPath || isLoadingVault.value) return;
  isLoadingVault.value = true;
  try {
    vault.value = await gitService.inspectCredentialVault(props.currentRepoPath);
  } catch (err: any) {
    notify(`凭据保险库扫描失败: ${err?.message || err}`);
  } finally {
    isLoadingVault.value = false;
  }
}

async function loadWebhooks() {
  if (!props.currentRepoPath || isLoadingWebhooks.value) return;
  isLoadingWebhooks.value = true;
  try {
    webhooks.value = await gitService.manageStudioWebhooks(
      props.currentRepoPath,
      "LIST",
      "",
      "",
      ""
    );
  } catch (err: any) {
    notify(`读取 Webhook 配置失败: ${err?.message || err}`);
  } finally {
    isLoadingWebhooks.value = false;
  }
}

async function loadDDC() {
  if (!props.currentRepoPath || isLoadingDDC.value) return;
  isLoadingDDC.value = true;
  try {
    ddc.value = await gitService.manageStudioDDC(props.currentRepoPath, "STATUS");
  } catch (err: any) {
    notify(`读取 DDC 缓存状态失败: ${err?.message || err}`);
  } finally {
    isLoadingDDC.value = false;
  }
}

async function loadQueue() {
  if (!props.currentRepoPath || isLoadingQueue.value) return;
  isLoadingQueue.value = true;
  try {
    queue.value = await gitService.manageOfflineSubmitQueue(
      props.currentRepoPath,
      "status",
      "",
      ""
    );
  } catch (err: any) {
    notify(`读取离线队列失败: ${err?.message || err}`);
  } finally {
    isLoadingQueue.value = false;
  }
}

async function loadSwitchboard() {
  if (!props.currentRepoPath || isLoadingSwitchboard.value) return;
  isLoadingSwitchboard.value = true;
  try {
    switchboard.value = await gitService.manageStudioSwitchboard(
      props.currentRepoPath,
      "LIST",
      "",
      "",
      ""
    );
  } catch (err: any) {
    notify(`读取工程拓扑失败: ${err?.message || err}`);
  } finally {
    isLoadingSwitchboard.value = false;
  }
}

function ensureLoaded() {
  void loadVault();
  void loadWebhooks();
  void loadDDC();
  void loadQueue();
  void loadSwitchboard();
}

function toggleExpanded() {
  expanded.value = !expanded.value;
  if (expanded.value && !vault.value && !webhooks.value && !ddc.value && !queue.value && !switchboard.value)
    ensureLoaded();
}

async function handleStoreSecret() {
  if (!props.currentRepoPath || isStoringSecret.value) return;
  if (!secretValue.value.trim()) return;
  if (!(await confirmAction("确定把该密钥加密写入当前仓库的 .git/wgit_vault.json 吗？密钥内容不会回显。")))
    return;
  isStoringSecret.value = true;
  try {
    const res: StoreSecretResult | null = await gitService.storeCredentialSecret(
      props.currentRepoPath,
      secretType.value,
      secretIdentifier.value.trim(),
      secretValue.value,
      secretScope.value
    );
    notify(res?.message || "密钥已加密存入本地保险库");
    secretValue.value = "";
    secretIdentifier.value = "";
    await loadVault();
  } catch (err: any) {
    notify(`存入保险库失败: ${err?.message || err}`);
  } finally {
    isStoringSecret.value = false;
  }
}

async function handleAddWebhook() {
  if (!props.currentRepoPath || isSavingWebhook.value || !webhookUrl.value.trim()) return;
  isSavingWebhook.value = true;
  try {
    const res = await gitService.manageStudioWebhooks(
      props.currentRepoPath,
      "ADD",
      webhookType.value,
      webhookUrl.value.trim(),
      "PUSH,TAG"
    );
    notify(res?.message || "Webhook 配置已登记");
    webhookUrl.value = "";
    await loadWebhooks();
  } catch (err: any) {
    notify(`添加 Webhook 失败: ${err?.message || err}`);
  } finally {
    isSavingWebhook.value = false;
  }
}

async function handlePruneDDC() {
  if (!props.currentRepoPath || isPruningDDC.value) return;
  if (!(await confirmAction("确定修剪 30 天未命中的过期 DDC 缓存块吗？"))) return;
  isPruningDDC.value = true;
  try {
    ddc.value = await gitService.manageStudioDDC(props.currentRepoPath, "PRUNE_STALE");
    notify(ddc.value?.summary || "DDC 过期缓存已修剪");
  } catch (err: any) {
    notify(`修剪 DDC 缓存失败: ${err?.message || err}`);
  } finally {
    isPruningDDC.value = false;
  }
}

async function handleEnqueue() {
  if (!props.currentRepoPath || queueBusy.value) return;
  queueBusy.value = "enqueue";
  try {
    const payload = queueTarget.value.trim()
      ? `${queueOperation.value}:${queueTarget.value.trim()}`
      : queueOperation.value;
    queue.value = await gitService.manageOfflineSubmitQueue(
      props.currentRepoPath,
      "enqueue",
      "",
      payload
    );
    notify(queue.value?.message || "操作已加入离线队列");
    queueTarget.value = "";
  } catch (err: any) {
    notify(`入队失败: ${err?.message || err}`);
  } finally {
    queueBusy.value = "";
  }
}

async function handleReplayQueue() {
  if (!props.currentRepoPath || queueBusy.value) return;
  if (
    !(await confirmAction(
      "确定重放队列中的全部操作吗？将实际执行 push / fetch / LFS 操作，可能访问远端并修改远端状态。"
    ))
  )
    return;
  queueBusy.value = "replay";
  try {
    queue.value = await gitService.manageOfflineSubmitQueue(
      props.currentRepoPath,
      "replay",
      "all",
      ""
    );
    notify(queue.value?.message || "已触发离线队列重放");
  } catch (err: any) {
    notify(`重放离线队列失败: ${err?.message || err}`);
  } finally {
    queueBusy.value = "";
  }
}

async function handlePauseQueue() {
  if (!props.currentRepoPath || queueBusy.value) return;
  if (!(await confirmAction("确定暂停队列中所有等待执行的操作吗？"))) return;
  queueBusy.value = "pause";
  try {
    queue.value = await gitService.manageOfflineSubmitQueue(
      props.currentRepoPath,
      "pause",
      "all",
      ""
    );
    notify(queue.value?.message || "离线队列已暂停");
  } catch (err: any) {
    notify(`暂停离线队列失败: ${err?.message || err}`);
  } finally {
    queueBusy.value = "";
  }
}

function securityClass(level: string) {
  if (level === "CRITICAL_EXPOSURE") return "bg-danger/15 text-danger";
  if (level === "WARNING") return "bg-warning/15 text-warning";
  return "bg-success/15 text-success";
}

watch(
  () => props.currentRepoPath,
  () => {
    vault.value = null;
    webhooks.value = null;
    ddc.value = null;
    queue.value = null;
    switchboard.value = null;
    if (expanded.value) ensureLoaded();
  }
);
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="flex items-center justify-between pb-2 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Layers class="w-4 h-4 text-cyan" />
          <span>Studio 运维与构建缓存</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          凭据暴露扫描、Webhook 登记、DDC 缓存、离线队列与多工程拓扑。
        </p>
      </div>
      <button
        @click="toggleExpanded"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
      >
        <component :is="expanded ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
        <span>{{ expanded ? "收起运维" : "展开运维" }}</span>
      </button>
    </div>

    <div v-if="expanded" class="space-y-4">
      <!-- Credentials -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <ShieldCheck class="w-4 h-4 text-success" />
            <span class="text-xs font-bold text-ink">凭据保险库与暴露扫描</span>
            <span v-if="vault" class="text-[10px] text-ink-muted font-mono">
              AES-256-GCM · {{ vault.kmsEncryptionEnabled ? "KMS 已启用" : "KMS 未启用" }}
            </span>
          </div>
          <button
            @click="loadVault"
            :disabled="isLoadingVault"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingVault }" />
            <span>{{ isLoadingVault ? "扫描中..." : "扫描凭据" }}</span>
          </button>
        </div>

        <template v-if="vault">
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">总数</div>
              <div class="text-sm font-bold font-mono text-ink">{{ vault.totalCredentials }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">安全</div>
              <div class="text-sm font-bold font-mono text-success">{{ vault.secureCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">警告</div>
              <div class="text-sm font-bold font-mono text-warning">{{ vault.warningsCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">严重暴露</div>
              <div class="text-sm font-bold font-mono text-danger">{{ vault.criticalExposures }}</div>
            </div>
          </div>
          <div class="rounded-lg border border-line bg-canvas/30 divide-y divide-line/40 max-h-48 overflow-y-auto">
            <div v-for="item in vault.items" :key="item.id" class="px-2.5 py-1.5 space-y-0.5">
              <div class="flex items-center gap-2 text-[11px]">
                <span class="px-1 py-0.2 rounded text-[9px] font-bold shrink-0" :class="securityClass(item.securityLevel)">
                  {{ item.securityLevel === "CRITICAL_EXPOSURE" ? "严重" : item.securityLevel === "WARNING" ? "警告" : "安全" }}
                </span>
                <span class="font-mono text-ink truncate flex-1" :title="item.identifier">{{ item.identifier }}</span>
                <span class="text-ink-muted font-mono shrink-0">{{ item.source }}</span>
              </div>
              <div class="flex items-center gap-2 text-[10px] text-ink-muted">
                <span class="font-mono truncate flex-1">{{ item.maskedValue }}</span>
                <span class="shrink-0">{{ item.permissions }}</span>
              </div>
            </div>
          </div>
          <p class="text-[10px] text-ink-muted">{{ vault.summary }}</p>
          <p class="text-[10px] text-ink-muted font-mono truncate" :title="vault.vaultStoragePath">{{ vault.vaultStoragePath }}</p>
        </template>

        <div class="grid grid-cols-1 md:grid-cols-5 gap-2 pt-1">
          <select
            v-model="secretType"
            class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="GIT_TOKEN">Git Token</option>
            <option value="TAPD_TOKEN">TAPD Token</option>
            <option value="TENCENT_CLOUD_AKSK">腾讯云 AKSK</option>
            <option value="AWS_AKSK">AWS AKSK</option>
            <option value="GENERIC_SECRET">通用密钥</option>
          </select>
          <input
            v-model="secretIdentifier"
            type="text"
            placeholder="标识 (例如 studio-ci)"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
          <input
            v-model="secretValue"
            type="password"
            autocomplete="new-password"
            placeholder="密钥内容（不会回显）"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
          <select
            v-model="secretScope"
            class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="LOCAL_REPO">仅本仓库</option>
            <option value="GLOBAL_SYSTEM">全局系统</option>
            <option value="ENV_VAR">环境变量</option>
          </select>
          <button
            @click="handleStoreSecret"
            :disabled="isStoringSecret || !secretValue.trim()"
            class="flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-xs font-semibold transition cursor-pointer disabled:opacity-40"
          >
            <Lock class="w-3.5 h-3.5" />
            <span>{{ isStoringSecret ? "加密中..." : "加密存入" }}</span>
          </button>
        </div>
      </div>

      <!-- Webhooks + DDC -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <Webhook class="w-4 h-4 text-accent" />
              <span class="text-xs font-bold text-ink">Webhook 通知</span>
            </div>
            <button
              @click="loadWebhooks"
              :disabled="isLoadingWebhooks"
              class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingWebhooks }" />
              <span>刷新</span>
            </button>
          </div>
          <div v-if="webhooks" class="space-y-1.5 max-h-40 overflow-y-auto">
            <p v-if="webhooks.webhooks.length === 0" class="text-[11px] text-ink-muted">尚未登记 Webhook 配置。</p>
            <div
              v-for="hook in webhooks.webhooks"
              :key="hook.id"
              class="flex items-center gap-2 px-2 py-1.5 rounded-lg border border-line bg-canvas/40 text-[11px]"
            >
              <span class="px-1 py-0.2 rounded bg-accent/15 text-accent text-[9px] font-bold shrink-0">{{ hook.webhookType }}</span>
              <span class="text-ink truncate flex-1" :title="hook.urlMasked">{{ hook.name }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ hook.eventRules.join("/") }}</span>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <select
              v-model="webhookType"
              class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
            >
              <option value="WECOM">企业微信</option>
              <option value="FEISHU">飞书</option>
              <option value="DINGTALK">钉钉</option>
              <option value="KODING">CODING</option>
            </select>
            <input
              v-model="webhookUrl"
              type="text"
              placeholder="Webhook URL（保存时脱敏）"
              class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <button
              @click="handleAddWebhook"
              :disabled="!webhookUrl.trim() || isSavingWebhook"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              登记
            </button>
          </div>
          <p class="text-[10px] text-ink-muted">仅登记配置到 .git/studio_webhooks.json，不会真实发送网络请求。</p>
        </div>

        <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <Database class="w-4 h-4 text-purple" />
              <span class="text-xs font-bold text-ink">DDC 构建缓存</span>
            </div>
            <div class="flex items-center gap-1.5">
              <button
                @click="loadDDC"
                :disabled="isLoadingDDC"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingDDC }" />
                <span>检测</span>
              </button>
              <button
                @click="handlePruneDDC"
                :disabled="isPruningDDC"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-warning transition cursor-pointer disabled:opacity-50"
              >
                <Trash2 class="w-3 h-3" />
                <span>{{ isPruningDDC ? "修剪中..." : "修剪过期" }}</span>
              </button>
            </div>
          </div>
          <template v-if="ddc">
            <div class="grid grid-cols-2 gap-2">
              <div class="p-2 rounded-lg border border-line bg-canvas/40">
                <div class="text-[10px] text-ink-muted">命中率</div>
                <div class="text-sm font-bold font-mono text-success">{{ ddc.cacheHitRatio > 0 ? ddc.cacheHitRatio + "%" : "未采集" }}</div>
              </div>
              <div class="p-2 rounded-lg border border-line bg-canvas/40">
                <div class="text-[10px] text-ink-muted">缓存体积</div>
                <div class="text-sm font-bold font-mono text-ink">{{ ddc.totalCacheSizeStr }}</div>
              </div>
              <div class="p-2 rounded-lg border border-line bg-canvas/40">
                <div class="text-[10px] text-ink-muted">过期块</div>
                <div class="text-sm font-bold font-mono text-warning">{{ ddc.staleBlobsCount }}</div>
              </div>
              <div class="p-2 rounded-lg border border-line bg-canvas/40">
                <div class="text-[10px] text-ink-muted">网络延迟</div>
                <div class="text-sm font-bold font-mono text-ink">{{ ddc.latencyMs > 0 ? ddc.latencyMs + " ms" : "—" }}</div>
              </div>
            </div>
            <p class="text-[10px] text-ink-muted truncate" :title="ddc.sharedCachePath">{{ ddc.summary }}</p>
          </template>
          <p v-else class="text-[11px] text-ink-muted">尚未检测 DDC 缓存状态。</p>
        </div>
      </div>

      <!-- Offline queue -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <KeyRound class="w-4 h-4 text-warning" />
            <span class="text-xs font-bold text-ink">离线提交队列</span>
            <span v-if="queue" class="text-[10px] font-mono" :class="queue.isOnline ? 'text-success' : 'text-warning'">
              {{ queue.networkSummary }}
            </span>
          </div>
          <div class="flex items-center gap-1.5">
            <button
              @click="loadQueue"
              :disabled="isLoadingQueue"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingQueue }" />
              <span>刷新</span>
            </button>
            <button
              @click="handlePauseQueue"
              :disabled="!!queueBusy"
              class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              全部暂停
            </button>
            <button
              @click="handleReplayQueue"
              :disabled="!!queueBusy"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-[11px] font-semibold transition cursor-pointer disabled:opacity-50"
            >
              <Play class="w-3 h-3" />
              <span>全部重放</span>
            </button>
          </div>
        </div>

        <template v-if="queue">
          <div class="grid grid-cols-5 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">排队</div>
              <div class="text-sm font-bold font-mono text-accent">{{ queue.queuedCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">失败</div>
              <div class="text-sm font-bold font-mono text-danger">{{ queue.failedCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">暂停</div>
              <div class="text-sm font-bold font-mono text-warning">{{ queue.pausedCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">成功</div>
              <div class="text-sm font-bold font-mono text-success">{{ queue.succeededCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">总计</div>
              <div class="text-sm font-bold font-mono text-ink">{{ queue.totalCount }}</div>
            </div>
          </div>
          <div class="rounded-lg border border-line bg-canvas/30 divide-y divide-line/40 max-h-40 overflow-y-auto">
            <div v-for="item in queue.items" :key="item.id" class="px-2.5 py-1.5 flex items-center gap-2 text-[11px]">
              <span class="px-1 py-0.2 rounded bg-surface border border-line text-[9px] font-mono shrink-0">{{ item.operation }}</span>
              <span class="font-mono text-ink truncate flex-1">{{ item.target || "—" }}</span>
              <span class="text-ink-muted shrink-0">{{ item.status }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ item.retryCount }}/{{ item.maxRetries }}</span>
            </div>
            <div v-if="queue.items.length === 0" class="px-2.5 py-4 text-center text-[11px] text-ink-muted">队列为空</div>
          </div>
        </template>

        <div class="grid grid-cols-1 md:grid-cols-4 gap-2">
          <select
            v-model="queueOperation"
            class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="fetch">fetch</option>
            <option value="push">push</option>
            <option value="lfs_lock">lfs_lock</option>
            <option value="lfs_unlock">lfs_unlock</option>
            <option value="lfs_push">lfs_push</option>
          </select>
          <input
            v-model="queueTarget"
            type="text"
            placeholder="目标分支 / 资产路径（可选）"
            class="md:col-span-2 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
          <button
            @click="handleEnqueue"
            :disabled="!!queueBusy"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40"
          >
            {{ queueBusy === "enqueue" ? "入队中..." : "加入队列" }}
          </button>
        </div>
      </div>

      <!-- Switchboard -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <Layers class="w-4 h-4 text-cyan" />
            <span class="text-xs font-bold text-ink">多工程 / 工作树拓扑</span>
            <span v-if="switchboard" class="text-[10px] text-ink-muted font-mono">
              {{ switchboard.totalProjects }} 工程 · {{ switchboard.linkedWorktrees }} 工作树 · {{ switchboard.adjacentRepos }} 邻接仓库
            </span>
          </div>
          <button
            @click="loadSwitchboard"
            :disabled="isLoadingSwitchboard"
            class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
          >
            <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingSwitchboard }" />
            <span>{{ isLoadingSwitchboard ? "读取中..." : "刷新拓扑" }}</span>
          </button>
        </div>
        <div v-if="switchboard" class="rounded-lg border border-line bg-canvas/30 divide-y divide-line/40 max-h-52 overflow-y-auto">
          <div v-for="project in switchboard.projects" :key="project.path" class="px-2.5 py-1.5 space-y-0.5">
            <div class="flex items-center gap-2 text-[11px]">
              <span class="px-1 py-0.2 rounded bg-surface border border-line text-[9px] shrink-0">{{ project.category }}</span>
              <span class="text-ink font-semibold truncate flex-1">{{ project.name }}</span>
              <span v-if="project.isCurrent" class="text-[9px] font-bold text-accent shrink-0">当前</span>
              <span v-if="project.isLocked" class="text-[9px] font-bold text-warning shrink-0">已锁定</span>
              <span v-if="project.isDirty" class="text-[9px] font-bold text-danger shrink-0">{{ project.dirtyFilesCount }} 改动</span>
            </div>
            <div class="flex items-center gap-2 text-[10px] text-ink-muted font-mono">
              <span class="truncate flex-1" :title="project.path">{{ project.path }}</span>
              <span class="shrink-0">{{ project.branch }}</span>
              <span v-if="project.aheadCount || project.behindCount" class="shrink-0">↑{{ project.aheadCount }} ↓{{ project.behindCount }}</span>
            </div>
          </div>
          <div v-if="switchboard.projects.length === 0" class="px-2.5 py-4 text-center text-[11px] text-ink-muted">未发现关联工程</div>
        </div>
        <p class="text-[10px] text-ink-muted">此处只读展示拓扑；分支与工作树操作请在对应页面执行。</p>
      </div>
    </div>
  </div>
</template>
