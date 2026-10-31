<script setup lang="ts">
import { ref } from "vue";
import {
  Send,
  CheckCircle2,
  XCircle,
  KeyRound,
  Globe,
  Loader2,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import SettingsCard from "./SettingsCard.vue";
import type { WebhookDispatchResult } from "../../types";

const webhookUrl = ref("");
const eventType = ref("git.push");
const secret = ref("");
const customPayload = ref('{"event":"git.push","branch":"main","author":"Developer"}');
const isSending = ref(false);
const dispatchResult = ref<WebhookDispatchResult | null>(null);

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

async function handleTestDispatch() {
  const url = webhookUrl.value.trim();
  if (!url || isSending.value) return;

  isSending.value = true;
  dispatchResult.value = null;

  try {
    const res = await gitService.dispatchTeamWebhook(url, eventType.value, customPayload.value, secret.value);
    dispatchResult.value = res;
    if (res.success) {
      emit("notify", t("webhook.toast.success", { code: res.statusCode, duration: res.durationMs }));
    } else {
      emit("notify", t("webhook.toast.failed", { error: `HTTP ${res.statusCode} ${res.responseBody}` }));
    }
  } catch (err: any) {
    emit("notify", t("webhook.toast.failed", { error: err?.message || String(err) }));
  } finally {
    isSending.value = false;
  }
}
</script>

<template>
  <SettingsCard class="@container" data-testid="team-webhook-section">
    <template #header><h3 class="text-xs font-semibold text-ink">{{ t("webhook.section.title") }}</h3></template>
    <div class="space-y-4">
      <p class="text-xs text-ink-secondary leading-relaxed">{{ t("webhook.section.desc") }}</p>

      <!-- URL Input -->
      <div class="space-y-1">
        <label for="settings-webhook-url" class="text-xs font-medium text-ink flex items-center gap-1.5">
          <Globe class="w-3.5 h-3.5 text-ink-muted" aria-hidden="true" />
          <span>{{ t("webhook.url.label") }}</span>
        </label>
        <input
          id="settings-webhook-url"
          v-model="webhookUrl"
          type="text"
          :placeholder="t('webhook.url.placeholder')"
          class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line font-mono text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          data-testid="webhook-url-input"
        />
      </div>

      <!-- Event & Secret Grid -->
      <div class="grid grid-cols-1 @min-[36rem]:grid-cols-2 gap-3">
        <div class="space-y-1">
          <label for="settings-webhook-event" class="text-xs font-medium text-ink">{{ t("webhook.event.label") }}</label>
          <select
            id="settings-webhook-event"
            v-model="eventType"
            class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink focus:outline-none focus:border-accent cursor-pointer"
          >
            <option value="git.push">{{ t("webhook.event.push") }}</option>
            <option value="git.tag">{{ t("webhook.event.tag") }}</option>
            <option value="git.merge">{{ t("webhook.event.merge") }}</option>
            <option value="ci.completed">{{ t("webhook.event.ci") }}</option>
          </select>
        </div>

        <div class="space-y-1">
          <label for="settings-webhook-secret" class="text-xs font-medium text-ink flex items-center gap-1.5">
            <KeyRound class="w-3 h-3 text-ink-muted" aria-hidden="true" />
            <span>{{ t("webhook.secret.label") }}</span>
          </label>
          <input
            id="settings-webhook-secret"
            v-model="secret"
            type="password"
            :placeholder="t('webhook.secret.placeholder')"
            class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line font-mono text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <!-- Dispatch Action -->
      <div class="flex flex-wrap items-center justify-between gap-3 pt-1">
        <span class="text-[10px] text-ink-muted">HMAC-SHA256 signature auto-computed</span>
        <button
          @click="handleTestDispatch"
          :disabled="!webhookUrl.trim() || isSending"
          class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-accent text-accent-contrast hover:bg-accent-strong text-xs font-semibold transition cursor-pointer disabled:opacity-40 shadow-xs"
          data-testid="dispatch-webhook-btn"
        >
          <Loader2 v-if="isSending" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
          <Send v-else class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ isSending ? t("webhook.btn.sending") : t("webhook.btn.test") }}</span>
        </button>
      </div>

      <!-- Dispatch Result Banner -->
      <div
        v-if="dispatchResult"
        class="p-3 rounded-lg border text-xs space-y-1 font-mono"
        :class="dispatchResult.success ? 'bg-emerald/10 border-emerald/30 text-emerald' : 'bg-rose/10 border-rose/30 text-rose'"
        data-testid="webhook-result"
      >
        <div class="flex items-center justify-between font-bold">
          <span class="flex items-center gap-1.5">
            <CheckCircle2 v-if="dispatchResult.success" class="w-3.5 h-3.5" />
            <XCircle v-else class="w-3.5 h-3.5" />
            <span>HTTP {{ dispatchResult.statusCode }}</span>
          </span>
          <span>{{ dispatchResult.durationMs }}ms</span>
        </div>
      <div v-if="dispatchResult.signatureHeader" class="text-[10px] text-ink-muted truncate">
        Sig: {{ dispatchResult.signatureHeader }}
      </div>
      <div class="text-[11px] text-ink truncate">
        Resp: {{ dispatchResult.responseBody || "OK" }}
      </div>
      </div>
    </div>
  </SettingsCard>
</template>
