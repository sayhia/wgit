<script setup lang="ts">
/**
 * AiReliabilityCard — retry/rate-limit plus the fallback provider.
 *
 * Presentational: the parent behavior section reads and writes the advanced
 * config, so this card only edits the fields it owns.
 */
import { computed } from "vue";
import { Server } from "lucide-vue-next";
import SettingsCard from "./SettingsCard.vue";
import { t } from "../../i18n";

const props = defineProps<{
  retryCount: number;
  rateLimitPerMinute: number;
  fallbackBaseUrl: string;
  fallbackModel: string;
  fallbackProtocol: string;
}>();
const emit = defineEmits<{
  (e: "update:retryCount", value: number): void;
  (e: "update:rateLimitPerMinute", value: number): void;
  (e: "update:fallbackBaseUrl", value: string): void;
  (e: "update:fallbackModel", value: string): void;
  (e: "update:fallbackProtocol", value: string): void;
}>();

const retryCount = computed({
  get: () => props.retryCount,
  set: (value: number) => emit("update:retryCount", Number(value) || 0),
});
const rateLimitPerMinute = computed({
  get: () => props.rateLimitPerMinute,
  set: (value: number) => emit("update:rateLimitPerMinute", Number(value) || 0),
});
const fallbackBaseUrl = computed({
  get: () => props.fallbackBaseUrl,
  set: (value: string) => emit("update:fallbackBaseUrl", value),
});
const fallbackModel = computed({
  get: () => props.fallbackModel,
  set: (value: string) => emit("update:fallbackModel", value),
});
const fallbackProtocol = computed({
  get: () => props.fallbackProtocol,
  set: (value: string) => emit("update:fallbackProtocol", value),
});
</script>

<template>
  <SettingsCard id="ai-section-reliability" data-testid="ai-reliability-card">
    <template #header>
      <div class="flex items-center gap-2">
        <Server class="w-4 h-4 text-accent" aria-hidden="true" />
        <div>
          <h3 class="ai-card-title">{{ t("settings.ai.reliability.title") }}</h3>
          <p class="ai-card-desc">{{ t("settings.ai.reliability.desc") }}</p>
        </div>
      </div>
    </template>

    <div class="space-y-3">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <label class="space-y-1">
          <span class="ai-field-label">{{ t("settings.ai.reliability.retry") }}</span>
          <select v-model.number="retryCount" class="ai-input">
            <option :value="0">{{ t("settings.ai.reliability.retry.off") }}</option>
            <option :value="1">1</option>
            <option :value="2">2</option>
            <option :value="3">3</option>
          </select>
        </label>
        <label class="space-y-1">
          <span class="ai-field-label">{{ t("settings.ai.reliability.rate") }}</span>
          <input
  v-model.number="rateLimitPerMinute"
  type="number"
  min="0"
  max="600"
  step="1"
            :placeholder="t('settings.ai.reliability.rate.off')"
  class="ai-input"
          />
        </label>
      </div>

      <div class="pt-3 border-t border-line/60 space-y-2">
        <div>
          <h5 class="text-[11px] font-bold text-ink">{{ t("settings.ai.fallback.title") }}</h5>
          <p class="ai-card-desc">{{ t("settings.ai.fallback.desc") }}</p>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
          <label class="space-y-1 sm:col-span-2">
            <span class="ai-field-label">{{ t("settings.ai.fallback.base") }}</span>
            <input
  v-model="fallbackBaseUrl"
  type="url"
  placeholder="https://api.example.com/v1"
  class="ai-input"
            />
          </label>
          <label class="space-y-1">
            <span class="ai-field-label">{{ t("settings.ai.fallback.protocol") }}</span>
            <select v-model="fallbackProtocol" class="ai-input">
              <option value="openai">OpenAI</option>
              <option value="azure">Azure</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Gemini</option>
            </select>
          </label>
          <label class="space-y-1 sm:col-span-2">
            <span class="ai-field-label">{{ t("settings.ai.fallback.model") }}</span>
            <input
  v-model="fallbackModel"
  type="text"
  placeholder="fallback-model"
  class="ai-input"
            />
          </label>
        </div>
        <p class="ai-card-desc">{{ t("settings.ai.fallback.hint") }}</p>
      </div>
    </div>
  </SettingsCard>
</template>
