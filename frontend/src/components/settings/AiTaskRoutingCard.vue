<script setup lang="ts">
/**
 * AiTaskRoutingCard — per-task model overrides.
 *
 * Presentational on purpose: `AiBehaviorSection` owns the single load/save
 * path (`GetAIAdvancedConfig` / `SaveAIAdvancedConfig`) so routing, reliability
 * and commit preferences cannot drift into three competing save actions.
 */
import { Cpu } from "lucide-vue-next";
import SettingsCard from "./SettingsCard.vue";
import { t } from "../../i18n";
import type { AITaskKind } from "../../services/aiService";

const TASK_KINDS: AITaskKind[] = [
  "commit",
  "summary",
  "conflict",
  "explain",
  "chat",
  "review",
  "release",
];

const props = defineProps<{ taskModels: Record<string, string> }>();
const emit = defineEmits<{
  (e: "update:taskModel", kind: AITaskKind, value: string): void;
}>();

function handleInput(kind: AITaskKind, event: Event): void {
  emit("update:taskModel", kind, (event.target as HTMLInputElement).value);
}
</script>

<template>
  <SettingsCard id="ai-section-routing" data-testid="ai-task-models-card">
    <template #header>
      <div class="flex items-center gap-2">
        <Cpu class="w-4 h-4 text-accent" aria-hidden="true" />
        <div>
          <h3 class="ai-card-title">{{ t("settings.ai.tasks.title") }}</h3>
          <p class="ai-card-desc">{{ t("settings.ai.tasks.desc") }}</p>
        </div>
      </div>
    </template>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <label v-for="kind in TASK_KINDS" :key="kind" class="space-y-1">
        <span class="ai-field-label">{{ t(`settings.ai.tasks.${kind}`) }}</span>
        <input
          :value="props.taskModels[kind] ?? ''"
          type="text"
          :placeholder="t('settings.ai.tasks.inherit')"
          :data-testid="`ai-task-model-${kind}`"
          class="ai-input"
          @input="handleInput(kind, $event)"
        />
      </label>
    </div>
  </SettingsCard>
</template>
