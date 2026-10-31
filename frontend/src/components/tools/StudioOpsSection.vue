<script setup lang="ts">
import { t } from "../../i18n";
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
const removingWebhookId = ref("");

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
    notify(t("ops.toast.vault.failed", { error: err?.message || err }));
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
    notify(t("ops.toast.webhook.read.failed", { error: err?.message || err }));
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
    notify(t("ops.toast.ddc.read.failed", { error: err?.message || err }));
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
    notify(t("ops.toast.queue.read.failed", { error: err?.message || err }));
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
    notify(t("ops.toast.topology.read.failed", { error: err?.message || err }));
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
  if (!(await confirmAction(t("ops.confirm.store.secret"))))
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
    notify(res?.message || t("ops.toast.secret.stored"));
    secretValue.value = "";
    secretIdentifier.value = "";
    await loadVault();
  } catch (err: any) {
    notify(t("ops.toast.secret.failed", { error: err?.message || err }));
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
    notify(res?.message || t("ops.toast.webhook.registered"));
    webhookUrl.value = "";
    await loadWebhooks();
  } catch (err: any) {
    notify(t("ops.toast.webhook.failed", { error: err?.message || err }));
  } finally {
    isSavingWebhook.value = false;
  }
}

async function handleRemoveWebhook(id: string) {
  if (!props.currentRepoPath || removingWebhookId.value) return;
  if (!(await confirmAction(t("ops.confirm.webhook.remove")))) return;
  removingWebhookId.value = id;
  try {
    const res = await gitService.manageStudioWebhooks(props.currentRepoPath, "REMOVE", id, "", "");
    notify(res?.message || t("ops.toast.webhook.removed"));
    await loadWebhooks();
  } catch (err: any) {
    notify(t("ops.toast.webhook.remove.failed", { error: err?.message || err }));
  } finally {
    removingWebhookId.value = "";
  }
}

async function handlePruneDDC() {
  if (!props.currentRepoPath || isPruningDDC.value) return;
  if (!(await confirmAction(t("ops.confirm.prune.ddc")))) return;
  isPruningDDC.value = true;
  try {
    ddc.value = await gitService.manageStudioDDC(props.currentRepoPath, "PRUNE_STALE");
    notify(ddc.value?.summary || t("ops.toast.ddc.pruned"));
  } catch (err: any) {
    notify(t("ops.toast.ddc.prune.failed", { error: err?.message || err }));
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
    notify(queue.value?.message || t("ops.toast.queue.enqueued"));
    queueTarget.value = "";
  } catch (err: any) {
    notify(t("ops.toast.queue.enqueue.failed", { error: err?.message || err }));
  } finally {
    queueBusy.value = "";
  }
}

async function handleReplayQueue() {
  if (!props.currentRepoPath || queueBusy.value) return;
  if (
    !(await confirmAction(
      t("ops.confirm.replay.all")
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
    notify(queue.value?.message || t("ops.toast.queue.replayed"));
  } catch (err: any) {
    notify(t("ops.toast.queue.replay.failed", { error: err?.message || err }));
  } finally {
    queueBusy.value = "";
  }
}

async function handlePauseQueue() {
  if (!props.currentRepoPath || queueBusy.value) return;
  if (!(await confirmAction(t("ops.confirm.pause.all")))) return;
  queueBusy.value = "pause";
  try {
    queue.value = await gitService.manageOfflineSubmitQueue(
      props.currentRepoPath,
      "pause",
      "all",
      ""
    );
    notify(queue.value?.message || t("ops.toast.queue.paused"));
  } catch (err: any) {
    notify(t("ops.toast.queue.pause.failed", { error: err?.message || err }));
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
          <span>{{ t("ops.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("ops.desc") }}
        </p>
      </div>
      <button
        @click="toggleExpanded"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer"
      >
        <component :is="expanded ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
        <span>{{ expanded ? t("ops.collapse") : t("ops.expand") }}</span>
      </button>
    </div>

    <div v-if="expanded" class="space-y-4">
      <!-- Credentials -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <ShieldCheck class="w-4 h-4 text-success" />
            <span class="text-xs font-bold text-ink">{{ t("ops.vault.title") }}</span>
            <span v-if="vault" class="text-[10px] text-ink-muted font-mono">
              AES-256-GCM · {{ vault.kmsEncryptionEnabled ? t("ops.vault.kms.on") : t("ops.vault.kms.off") }}
            </span>
          </div>
          <button
            @click="loadVault"
            :disabled="isLoadingVault"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoadingVault }" aria-hidden="true" />
            <span>{{ isLoadingVault ? t("ops.scanning") : t("ops.vault.scan") }}</span>
          </button>
        </div>

        <template v-if="vault">
          <div class="grid grid-cols-4 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.vault.total") }}</div>
              <div class="text-sm font-bold font-mono text-ink">{{ vault.totalCredentials }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.vault.safe") }}</div>
              <div class="text-sm font-bold font-mono text-success">{{ vault.secureCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.vault.warn") }}</div>
              <div class="text-sm font-bold font-mono text-warning">{{ vault.warningsCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.vault.critical") }}</div>
              <div class="text-sm font-bold font-mono text-danger">{{ vault.criticalExposures }}</div>
            </div>
          </div>
          <div class="rounded-lg border border-line bg-canvas/30 divide-y divide-line/40 max-h-48 overflow-y-auto">
            <div v-for="item in vault.items" :key="item.id" class="px-2.5 py-1.5 space-y-0.5">
              <div class="flex items-center gap-2 text-[11px]">
                <span class="px-1 py-0.2 rounded text-[9px] font-bold shrink-0" :class="securityClass(item.securityLevel)">
                  {{ item.securityLevel === "CRITICAL_EXPOSURE" ? t("ops.level.critical") : item.securityLevel === "WARNING" ? t("ops.level.warn") : t("ops.level.safe") }}
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
            <option value="TENCENT_CLOUD_AKSK">{{ t("ops.secret.type.tencent") }}</option>
            <option value="AWS_AKSK">AWS AKSK</option>
            <option value="GENERIC_SECRET">{{ t("ops.secret.type.generic") }}</option>
          </select>
          <input
            v-model="secretIdentifier"
            type="text"
            :placeholder="t('ops.secret.id.placeholder')"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
          <input
            v-model="secretValue"
            type="password"
            autocomplete="new-password"
            :placeholder="t('ops.secret.value.placeholder')"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
          <select
            v-model="secretScope"
            class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="LOCAL_REPO">{{ t("ops.secret.scope.local") }}</option>
            <option value="GLOBAL_SYSTEM">{{ t("ops.secret.scope.global") }}</option>
            <option value="ENV_VAR">{{ t("ops.secret.scope.env") }}</option>
          </select>
          <button
            @click="handleStoreSecret"
            :disabled="isStoringSecret || !secretValue.trim()"
            class="flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
          >
            <Lock class="w-3.5 h-3.5" aria-hidden="true" />
            <span>{{ isStoringSecret ? t("ops.secret.encrypting") : t("ops.secret.encrypt") }}</span>
          </button>
        </div>
      </div>

      <!-- Webhooks + DDC -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <Webhook class="w-4 h-4 text-accent" />
              <span class="text-xs font-bold text-ink">{{ t("ops.webhook.title") }}</span>
            </div>
            <button
              @click="loadWebhooks"
              :disabled="isLoadingWebhooks"
              class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingWebhooks }" aria-hidden="true" />
              <span>{{ t("ops.refresh") }}</span>
            </button>
          </div>
          <div v-if="webhooks" class="space-y-1.5 max-h-40 overflow-y-auto">
            <p v-if="webhooks.webhooks.length === 0" class="text-[11px] text-ink-muted">{{ t("ops.webhook.empty") }}</p>
            <div
              v-for="hook in webhooks.webhooks"
              :key="hook.id"
              class="flex items-center gap-2 px-2 py-1.5 rounded-lg border border-line bg-canvas/40 text-[11px]"
            >
              <span class="px-1 py-0.2 rounded bg-accent/15 text-accent text-[9px] font-bold shrink-0">{{ hook.webhookType }}</span>
              <span class="text-ink truncate flex-1" :title="hook.urlMasked">{{ hook.name }}</span>
              <span class="text-ink-muted font-mono shrink-0">{{ hook.eventRules.join("/") }}</span>
              <button
                @click="handleRemoveWebhook(hook.id)"
                :disabled="removingWebhookId === hook.id"
                :aria-label="t('ops.webhook.remove')"
                :title="t('ops.webhook.remove')"
                class="p-1 rounded text-ink-muted hover:text-danger hover:bg-danger/10 transition cursor-pointer disabled:opacity-40 shrink-0"
              >
                <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
              </button>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <select
              v-model="webhookType"
              class="px-2 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
            >
              <option value="WECOM">{{ t("ops.webhook.type.wecom") }}</option>
              <option value="FEISHU">{{ t("ops.webhook.type.feishu") }}</option>
              <option value="DINGTALK">{{ t("ops.webhook.type.dingtalk") }}</option>
              <option value="KODING">CODING</option>
            </select>
            <input
              v-model="webhookUrl"
              type="text"
              :placeholder="t('ops.webhook.url.placeholder')"
              class="flex-1 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
            />
            <button
              @click="handleAddWebhook"
              :disabled="!webhookUrl.trim() || isSavingWebhook"
              class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40 shrink-0"
            >
              {{ t("ops.webhook.register") }}
            </button>
          </div>
          <p class="text-[10px] text-ink-muted">{{ t("ops.webhook.hint") }}</p>
        </div>

        <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <Database class="w-4 h-4 text-purple" />
              <span class="text-xs font-bold text-ink">{{ t("ops.ddc.title") }}</span>
            </div>
            <div class="flex items-center gap-1.5">
              <button
                @click="loadDDC"
                :disabled="isLoadingDDC"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
              >
                <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingDDC }" aria-hidden="true" />
                <span>{{ t("ops.ddc.detect") }}</span>
              </button>
              <button
                @click="handlePruneDDC"
                :disabled="isPruningDDC"
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-warning transition cursor-pointer disabled:opacity-50"
              >
                <Trash2 class="w-3 h-3" aria-hidden="true" />
                <span>{{ isPruningDDC ? t("ops.ddc.pruning") : t("ops.ddc.prune") }}</span>
              </button>
            </div>
          </div>
          <template v-if="ddc">
            <div class="grid grid-cols-2 gap-2">
              <div class="p-2 rounded-lg border border-line bg-canvas/40">
                <div class="text-[10px] text-ink-muted">{{ t("ops.ddc.hit.ratio") }}</div>
                <div class="text-sm font-bold font-mono text-success">{{ ddc.cacheHitRatio > 0 ? ddc.cacheHitRatio + "%" : t("studio.ops.not.collected") }}</div>
              </div>
              <div class="p-2 rounded-lg border border-line bg-canvas/40">
                <div class="text-[10px] text-ink-muted">{{ t("ops.ddc.size") }}</div>
                <div class="text-sm font-bold font-mono text-ink">{{ ddc.totalCacheSizeStr }}</div>
              </div>
              <div class="p-2 rounded-lg border border-line bg-canvas/40">
                <div class="text-[10px] text-ink-muted">{{ t("ops.ddc.expired") }}</div>
                <div class="text-sm font-bold font-mono text-warning">{{ ddc.staleBlobsCount }}</div>
              </div>
              <div class="p-2 rounded-lg border border-line bg-canvas/40">
                <div class="text-[10px] text-ink-muted">{{ t("ops.ddc.latency") }}</div>
                <div class="text-sm font-bold font-mono text-ink">{{ ddc.latencyMs > 0 ? ddc.latencyMs + " ms" : "—" }}</div>
              </div>
            </div>
            <p class="text-[10px] text-ink-muted truncate" :title="ddc.sharedCachePath">{{ ddc.summary }}</p>
          </template>
          <p v-else class="text-[11px] text-ink-muted">{{ t("ops.ddc.not.detected") }}</p>
        </div>
      </div>

      <!-- Offline queue -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <KeyRound class="w-4 h-4 text-warning" />
            <span class="text-xs font-bold text-ink">{{ t("ops.queue.title") }}</span>
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
              <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingQueue }" aria-hidden="true" />
              <span>{{ t("ops.refresh") }}</span>
            </button>
            <button
              @click="handlePauseQueue"
              :disabled="!!queueBusy"
              class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
            >
              {{ t("ops.queue.pause.all") }}
            </button>
            <button
              @click="handleReplayQueue"
              :disabled="!!queueBusy"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-[11px] font-semibold transition cursor-pointer disabled:opacity-50"
            >
              <Play class="w-3 h-3" aria-hidden="true" />
              <span>{{ t("ops.queue.replay.all") }}</span>
            </button>
          </div>
        </div>

        <template v-if="queue">
          <div class="grid grid-cols-5 gap-2">
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.queue.queued") }}</div>
              <div class="text-sm font-bold font-mono text-accent">{{ queue.queuedCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.queue.failed") }}</div>
              <div class="text-sm font-bold font-mono text-danger">{{ queue.failedCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.queue.paused") }}</div>
              <div class="text-sm font-bold font-mono text-warning">{{ queue.pausedCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.queue.done") }}</div>
              <div class="text-sm font-bold font-mono text-success">{{ queue.succeededCount }}</div>
            </div>
            <div class="p-2 rounded-lg border border-line bg-canvas/40">
              <div class="text-[10px] text-ink-muted">{{ t("ops.queue.total") }}</div>
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
            <div v-if="queue.items.length === 0" class="px-2.5 py-4 text-center text-[11px] text-ink-muted">{{ t("ops.queue.empty") }}</div>
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
            :placeholder="t('ops.queue.target.placeholder')"
            class="md:col-span-2 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
          />
          <button
            @click="handleEnqueue"
            :disabled="!!queueBusy"
            class="px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-accent transition cursor-pointer disabled:opacity-40"
          >
            {{ queueBusy === "enqueue" ? t("ops.queue.enqueueing") : t("ops.queue.enqueue") }}
          </button>
        </div>
      </div>

      <!-- Switchboard -->
      <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <Layers class="w-4 h-4 text-cyan" />
            <span class="text-xs font-bold text-ink">{{ t("ops.topology.title") }}</span>
            <span v-if="switchboard" class="text-[10px] text-ink-muted font-mono">
              {{ t("ops.topology.summary", { projects: switchboard.totalProjects, worktrees: switchboard.linkedWorktrees, adjacent: switchboard.adjacentRepos }) }}
            </span>
          </div>
          <button
            @click="loadSwitchboard"
            :disabled="isLoadingSwitchboard"
            class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised border border-line text-[11px] font-semibold text-ink transition cursor-pointer disabled:opacity-50"
          >
            <RefreshCw class="w-3 h-3" :class="{ 'animate-spin': isLoadingSwitchboard }" aria-hidden="true" />
            <span>{{ isLoadingSwitchboard ? t("ops.reading") : t("ops.topology.refresh") }}</span>
          </button>
        </div>
        <div v-if="switchboard" class="rounded-lg border border-line bg-canvas/30 divide-y divide-line/40 max-h-52 overflow-y-auto">
          <div v-for="project in switchboard.projects" :key="project.path" class="px-2.5 py-1.5 space-y-0.5">
            <div class="flex items-center gap-2 text-[11px]">
              <span class="px-1 py-0.2 rounded bg-surface border border-line text-[9px] shrink-0">{{ project.category }}</span>
              <span class="text-ink font-semibold truncate flex-1">{{ project.name }}</span>
              <span v-if="project.isCurrent" class="text-[9px] font-bold text-accent shrink-0">{{ t("ops.topology.current") }}</span>
              <span v-if="project.isLocked" class="text-[9px] font-bold text-warning shrink-0">{{ t("ops.topology.locked") }}</span>
              <span v-if="project.isDirty" class="text-[9px] font-bold text-danger shrink-0">{{ t("ops.topology.dirty", { count: project.dirtyFilesCount }) }}</span>
              <span
                v-if="project.statusWarning"
                class="text-[9px] font-bold text-warning shrink-0 cursor-help"
                role="status"
                :title="project.statusWarning"
                :aria-label="project.statusWarning"
              >!</span>
            </div>
            <div class="flex items-center gap-2 text-[10px] text-ink-muted font-mono">
              <span class="truncate flex-1" :title="project.path">{{ project.path }}</span>
              <span class="shrink-0">{{ project.branch }}</span>
              <span v-if="project.aheadCount || project.behindCount" class="shrink-0">↑{{ project.aheadCount }} ↓{{ project.behindCount }}</span>
            </div>
          </div>
          <div v-if="switchboard.projects.length === 0" class="px-2.5 py-4 text-center text-[11px] text-ink-muted">{{ t("ops.topology.empty") }}</div>
        </div>
        <p class="text-[10px] text-ink-muted">{{ t("ops.topology.hint") }}</p>
      </div>
    </div>
  </div>
</template>
