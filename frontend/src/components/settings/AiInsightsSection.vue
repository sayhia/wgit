<script setup lang="ts">
/**
 * AiInsightsSection — settings leaf for usage and diagnostics.
 *
 * Order is intentional: usage (read-only reporting), diagnostics (local-only
 * read-only checks) and finally config transfer. The leaf owns no draft state;
 * children reload on `refreshKey` and the transfer action emits `changed` after
 * a successful import so the shell can refresh every surface.
 */
import AiUsageCard from "./AiUsageCard.vue";
import AiDiagnosticsCard from "./AiDiagnosticsCard.vue";
import AiTransferActions from "./AiTransferActions.vue";
import SettingsPageHeader from "./SettingsPageHeader.vue";
import { t } from "../../i18n";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });
const emit = defineEmits<{ (e: "changed"): void }>();
</script>

<template>
  <div class="space-y-4" data-testid="ai-insights-section">
    <SettingsPageHeader
      :title="t('settings.ai.insights.title')"
      :description="t('settings.ai.insights.subtitle')"
    />
    <AiUsageCard :refresh-key="refreshKey" />
    <AiDiagnosticsCard :refresh-key="refreshKey" />
    <AiTransferActions :refresh-key="refreshKey" @changed="emit('changed')" />
  </div>
</template>
