<script setup lang="ts">
import { ref } from "vue";
import {
  Sparkles,
  Server,
  Zap,
  CheckCircle2,
  AlertTriangle,
  RefreshCw,
  Cpu,
} from "lucide-vue-next";
import { t } from "../../i18n";
import {
  getLocalAiConfig,
  saveLocalAiConfig,
  testLocalAiConnection,
  type LocalAiConfig,
} from "../../services/localAiService";

const config = ref<LocalAiConfig>(getLocalAiConfig());
const isTesting = ref(false);
const testResult = ref<{ ok: boolean; message: string; models?: string[] } | null>(null);

function updateConfig() {
  saveLocalAiConfig(config.value);
}

function toggleEnabled() {
  config.value.enabled = !config.value.enabled;
  updateConfig();
}

async function runTest() {
  isTesting.value = true;
  testResult.value = null;
  try {
    const res = await testLocalAiConnection(config.value.endpoint, config.value.apiKey);
    testResult.value = res;
    if (res.ok && res.models && res.models.length > 0 && !config.value.model) {
      config.value.model = res.models[0]!;
      updateConfig();
    }
  } catch (err: any) {
    testResult.value = {
      ok: false,
      message: err?.message || String(err),
    };
  } finally {
    isTesting.value = false;
  }
}
</script>

<template>
  <section class="space-y-6">
    <div>
      <h2 class="text-sm font-bold text-ink">{{ t("settings.localai.title") }}</h2>
      <p class="text-xs text-ink-secondary mt-1">
        {{ t("settings.localai.intro") }}
      </p>
    </div>

    <!-- Master Switch Card -->
    <div class="p-4 rounded-2xl bg-surface border border-line flex items-center justify-between gap-4">
      <div class="flex items-start gap-3 min-w-0">
        <div class="w-9 h-9 rounded-xl bg-accent/10 border border-accent/20 flex items-center justify-center text-accent shrink-0">
          <Sparkles class="w-4 h-4 text-accent" />
        </div>
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h3 class="text-xs font-bold text-ink">{{ t("settings.localai.switch.title") }}</h3>
            <span
              class="px-1.5 py-0.5 rounded text-[9px] font-mono font-bold"
              :class="config.enabled ? 'bg-accent/20 text-accent border border-accent/30' : 'bg-canvas text-ink-muted border border-line'"
            >
              {{ config.enabled ? t("settings.localai.enabled") : t("settings.localai.disabled") }}
            </span>
          </div>
          <p class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">
            {{ t("settings.localai.switch.desc") }}
          </p>
        </div>
      </div>

      <button
        type="button"
        role="switch"
        :aria-checked="config.enabled"
        @click="toggleEnabled"
        class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent/50"
        :class="config.enabled ? 'bg-accent' : 'bg-line'"
      >
        <span
          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out"
          :class="config.enabled ? 'translate-x-5' : 'translate-x-0'"
        />
      </button>
    </div>

    <!-- Endpoint Configuration Form -->
    <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
      <div class="space-y-1">
        <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
          <Server class="w-3.5 h-3.5 text-accent" />
          <span>{{ t("settings.localai.endpoint.label") }}</span>
        </label>
        <input
          v-model="config.endpoint"
          type="text"
          placeholder="http://localhost:11434/v1"
          @change="updateConfig"
          class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
        />
        <p class="text-[10px] text-ink-muted">
          {{ t("settings.localai.endpoint.hint") }}
        </p>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
            <Cpu class="w-3.5 h-3.5 text-accent" />
            <span>{{ t("settings.localai.model.label") }}</span>
          </label>
          <input
            v-model="config.model"
            type="text"
            placeholder="llama3"
            @change="updateConfig"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>

        <div class="space-y-1">
          <label class="text-xs font-semibold text-ink flex items-center gap-1.5">
            <Zap class="w-3.5 h-3.5 text-accent" />
            <span>{{ t("settings.localai.apikey.label") }}</span>
          </label>
          <input
            v-model="config.apiKey"
            type="password"
            placeholder="Optional for Ollama"
            @change="updateConfig"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      <!-- Test Connection Button & Result -->
      <div class="pt-2 flex flex-col gap-2">
        <button
          type="button"
          @click="runTest"
          :disabled="isTesting || !config.endpoint.trim()"
          class="w-fit flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-line bg-surface hover:bg-raised text-xs font-semibold text-ink-secondary hover:text-ink transition cursor-pointer disabled:opacity-40"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isTesting }" />
          <span>{{ isTesting ? t("common.loading") : t("settings.localai.test.btn") }}</span>
        </button>

        <div
          v-if="testResult"
          class="p-3 rounded-xl border text-xs flex items-center gap-2 transition"
          :class="testResult.ok ? 'bg-success/10 border-success/30 text-success' : 'bg-danger/10 border-danger/30 text-danger'"
        >
          <CheckCircle2 v-if="testResult.ok" class="w-4 h-4 shrink-0 text-success" />
          <AlertTriangle v-else class="w-4 h-4 shrink-0 text-danger" />
          <span class="text-[11px] font-medium leading-relaxed">{{ testResult.message }}</span>
        </div>
      </div>
    </div>
  </section>
</template>
