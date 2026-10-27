<script setup lang="ts">
import { ref, computed, watch } from "vue";
import {
  Wifi,
  Copy,
  Check,
  Power,
  RefreshCw,
  Globe,
  Radio,
  ExternalLink,
  Laptop,
} from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { gitService } from "../../services/gitService";
import type { LanServerInfo } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  open: boolean;
  repoPath: string;
  base?: string;
  head?: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "notify", msg: string): void;
}>();

const isBusy = ref(false);
const port = ref(8080);
const copied = ref(false);
const serverInfo = ref<LanServerInfo | null>(null);
const activePeerCount = ref(0);
let eventSource: EventSource | null = null;

const isRunning = computed(() => !!serverInfo.value?.running);

function connectEventSource() {
  if (!isRunning.value || !serverInfo.value?.url || typeof EventSource === "undefined") return;
  disconnectEventSource();
  try {
    eventSource = new EventSource(`${serverInfo.value.url}/api/events`);
    eventSource.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);
        if (data.type === "peer_count" && data.payload?.count !== undefined) {
          activePeerCount.value = data.payload.count;
        } else if (data.type === "connected" && data.payload?.activePeers !== undefined) {
          activePeerCount.value = data.payload.activePeers;
        } else if (data.type === "comment") {
          emit("notify", t("review.lan.event.comment", { author: data.sender || "Peer" }));
        }
      } catch {}
    };
    eventSource.onerror = () => {
      disconnectEventSource();
    };
  } catch {}
}

function disconnectEventSource() {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
}

watch(
  [() => props.open, isRunning],
  ([open, running]) => {
    if (open && running) {
      connectEventSource();
    } else {
      disconnectEventSource();
      activePeerCount.value = 0;
    }
  },
  { immediate: true }
);

async function handleBroadcastPing() {
  try {
    await gitService.broadcastLanReviewEvent("ping", "ping_from_host");
    emit("notify", t("review.lan.toast.broadcast.sent"));
  } catch (err: any) {
    emit("notify", t("review.lan.toast.failed", { error: err?.message || err }));
  }
}

const shareUrl = computed(() => {
  if (!serverInfo.value?.url) return "";
  if (props.base && props.head) {
    return `${serverInfo.value.url}/?base=${encodeURIComponent(props.base)}&head=${encodeURIComponent(props.head)}`;
  }
  return serverInfo.value.url;
});

async function fetchStatus() {
  try {
    const info = await gitService.getLanReviewServerStatus();
    serverInfo.value = info;
    if (info?.port) {
      port.value = info.port;
    }
  } catch (err) {
    console.debug("Failed to query LAN review server status:", err);
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      void fetchStatus();
    }
  },
  { immediate: true }
);

async function toggleServer() {
  if (!props.repoPath) return;
  isBusy.value = true;
  try {
    if (isRunning.value) {
      const res = await gitService.stopLanReviewServer();
      serverInfo.value = res;
      emit("notify", t("review.lan.toast.stopped"));
    } else {
      const res = await gitService.startLanReviewServer(props.repoPath, port.value);
      serverInfo.value = res;
      emit("notify", t("review.lan.toast.started", { port: res.port }));
    }
  } catch (err: any) {
    emit("notify", t("review.lan.toast.failed", { error: err?.message || err }));
  } finally {
    isBusy.value = false;
  }
}

async function handleCopyUrl() {
  if (!shareUrl.value) return;
  try {
    await navigator.clipboard.writeText(shareUrl.value);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
    emit("notify", t("review.lan.toast.copied"));
  } catch {
    /* ignore clipboard failures */
  }
}
</script>

<template>
  <UiModal
    :open="open"
    :title="t('review.lan.modal.title')"
    width="max-w-[620px]"
    @close="emit('update:open', false)"
  >
    <div class="space-y-4">
      <!-- Status Card -->
      <div class="p-4 rounded-xl bg-surface border border-line flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div
            class="w-10 h-10 rounded-xl flex items-center justify-center transition-colors"
            :class="isRunning ? 'bg-success/15 text-success' : 'bg-surface text-ink-muted border border-line'"
          >
            <Radio v-if="isRunning" class="w-5 h-5 animate-pulse text-success" />
            <Wifi v-else class="w-5 h-5 text-ink-muted" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <span class="text-xs font-bold text-ink">{{ t("review.lan.status.title") }}</span>
              <span
                class="px-2 py-0.5 rounded-full text-[10px] font-semibold"
                :class="isRunning ? 'bg-success/15 text-success' : 'bg-surface text-ink-muted border border-line'"
              >
                {{ isRunning ? t("review.lan.status.running") : t("review.lan.status.stopped") }}
              </span>
            </div>
            <p class="text-[11px] text-ink-muted mt-0.5">
              {{ t("review.lan.status.desc") }}
            </p>
          </div>
        </div>

        <button
          type="button"
          @click="toggleServer"
          :disabled="isBusy"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold cursor-pointer transition shadow-sm"
          :class="
            isRunning
              ? 'bg-danger/15 hover:bg-danger/25 text-danger border border-danger/30'
              : 'bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast'
          "
        >
          <RefreshCw v-if="isBusy" class="w-3.5 h-3.5 animate-spin" />
          <Power v-else class="w-3.5 h-3.5" />
          <span>{{ isRunning ? t("review.lan.btn.stop") : t("review.lan.btn.start") }}</span>
        </button>
      </div>

      <!-- Settings & URL Card -->
      <div class="p-4 rounded-xl bg-surface border border-line space-y-3">
        <div class="flex items-center justify-between gap-4">
          <div class="flex-1 min-w-0">
            <label class="text-xs font-medium text-ink block">{{ t("review.lan.port.label") }}</label>
            <span class="text-[10px] text-ink-muted">{{ t("review.lan.port.hint") }}</span>
          </div>
          <input
            v-model.number="port"
            type="number"
            min="1024"
            max="65535"
            :disabled="isRunning || isBusy"
            class="w-24 px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink text-center focus:outline-none focus:border-accent disabled:opacity-50"
          />
        </div>

        <!-- Shareable Link (when running) -->
        <div v-if="isRunning" class="pt-3 border-t border-line space-y-2">
          <label class="text-xs font-medium text-ink flex items-center gap-1.5">
            <Globe class="w-3.5 h-3.5 text-accent" />
            <span>{{ t("review.lan.link.label") }}</span>
          </label>
          <div class="flex items-center gap-2">
            <input
              :value="shareUrl"
              readonly
              class="flex-1 min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none select-all"
            />
            <UiButton
              variant="secondary"
              size="sm"
              @click="handleCopyUrl"
              class="shrink-0 flex items-center gap-1.5"
            >
              <Check v-if="copied" class="w-3.5 h-3.5 text-success" />
              <Copy v-else class="w-3.5 h-3.5" />
              <span>{{ copied ? t("ws.copy.done") : t("ws.copy") }}</span>
            </UiButton>
          </div>

          <!-- IP Addresses list -->
          <div v-if="serverInfo && serverInfo.localIps && serverInfo.localIps.length > 0" class="pt-2">
            <span class="text-[10px] text-ink-muted block mb-1">
              {{ t("review.lan.ips.title") }}:
            </span>
            <div class="flex flex-wrap gap-1.5">
              <span
                v-for="ip in serverInfo.localIps"
                :key="ip"
                class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-raised border border-line/60 text-[10px] font-mono text-ink-secondary"
              >
                <Laptop class="w-2.5 h-2.5 text-ink-muted" />
                <span>http://{{ ip }}:{{ serverInfo.port }}</span>
              </span>
            </div>
          </div>

          <!-- Live Event Bus & Peer Counter -->
          <div class="pt-2 border-t border-line/50 flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 text-xs">
              <Radio class="w-3.5 h-3.5 text-accent animate-pulse" />
              <span class="text-ink font-medium">
                {{ t("review.lan.bus.peers", { count: activePeerCount }) }}
              </span>
            </div>
            <UiButton
              variant="secondary"
              size="xs"
              @click="handleBroadcastPing"
              class="flex items-center gap-1"
              :title="t('review.lan.bus.broadcast.title')"
            >
              <Radio class="w-3 h-3 text-accent" />
              <span>{{ t("review.lan.bus.broadcast") }}</span>
            </UiButton>
          </div>
        </div>
      </div>

      <!-- LAN Safety Notice -->
      <div class="p-3 rounded-lg bg-surface/50 border border-line/60 text-[11px] text-ink-muted leading-relaxed">
        {{ t("review.lan.notice") }}
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-end">
        <UiButton variant="secondary" size="sm" @click="emit('update:open', false)">
          {{ t("common.close") }}
        </UiButton>
      </div>
    </template>
  </UiModal>
</template>
