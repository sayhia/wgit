<script setup lang="ts">
/**
 * AiDiagnosticsCard — local-only configuration diagnostics.
 *
 * Read-only by contract: it reads the saved connection, provider catalog,
 * advanced config and detected CLIs, then derives ok/warn/fail rows. It never
 * starts a stream, previews a prompt or sends a connection test; the quick
 * setup surface owns the only explicit connectivity probe.
 */
import { computed, onMounted, ref, watch } from "vue";
import { AlertTriangle, CheckCircle2, Cpu, XCircle } from "lucide-vue-next";
import SettingsCard from "./SettingsCard.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import {
  apiOrigin,
  detectAICLIs,
  findModelPrice,
  getAIAdvancedConfig,
  getAIConfig,
  getAIProviderCatalog,
  isSameApiOrigin,
  type AICLIOption,
  type AIAdvancedConfig,
  type AIProviderCatalog,
  type AiConfigNoSecret,
} from "../../services/aiService";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });

type DiagnosticLevel = "ok" | "warn" | "fail";
interface DiagnosticRow {
  id: string;
  level: DiagnosticLevel;
  label: string;
  detail: string;
}

const config = ref<AiConfigNoSecret | null>(null);
const advanced = ref<AIAdvancedConfig | null>(null);
const catalog = ref<AIProviderCatalog | null>(null);
const cliOptions = ref<AICLIOption[]>([]);
const isLoading = ref(false);
const loadError = ref("");

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

/** Fallback draft used only until the first advanced-config read resolves. */
function emptyAdvanced(): AIAdvancedConfig {
  return {
    taskModels: {},
    repoOverrides: {},
    profileNames: {},
    inputUsdPerMTok: 0,
    outputUsdPerMTok: 0,
    monthlyBudgetUsd: 0,
    retryCount: 0,
    rateLimitPerMinute: 0,
    fallbackBaseUrl: "",
    fallbackModel: "",
    fallbackProtocol: "openai",
  };
}

const diagnostics = computed<DiagnosticRow[]>(() => {
  const current = config.value;
  if (!current) return [];
  const adv = advanced.value || emptyAdvanced();
  const rows: DiagnosticRow[] = [];
  const isCLI = current.mode === "cli";
  const command = current.cliCommand.trim();

  if (isCLI) {
  const option = cliOptions.value.find(
      (item) => item.command === command || item.id === command
    );
    const found = Boolean(command && option?.available);
    rows.push({
      id: "endpoint",
      level: found ? "ok" : "fail",
      label: t("settings.ai.diagnostics.cli"),
      detail: !command
        ? t("settings.ai.diagnostics.cli.missing", { command: "—" })
        : found
          ? t("settings.ai.diagnostics.cli.ok", { command, version: option?.version || "" })
          : t("settings.ai.diagnostics.cli.missing", { command }),
    });
  } else {
    let urlOK = false;
    try {
      const parsed = new URL(current.baseUrl);
      urlOK = parsed.protocol === "http:" || parsed.protocol === "https:";
    } catch {
      urlOK = false;
    }
    rows.push({
      id: "endpoint",
      level: urlOK ? "ok" : "fail",
      label: t("settings.ai.diagnostics.endpoint"),
      detail: urlOK
        ? t("settings.ai.diagnostics.endpoint.ok", {
            protocol: current.protocol || "openai",
            url: current.baseUrl,
          })
        : t("settings.ai.diagnostics.endpoint.invalid"),
    });

  const provider = (catalog.value?.providers || []).find(
      (item) => item.baseUrl && isSameApiOrigin(item.baseUrl, current.baseUrl)
    );
    if (provider?.keyRequired === false) {
      rows.push({
        id: "key",
        level: "ok",
        label: t("settings.ai.diagnostics.key"),
        detail: t("settings.ai.diagnostics.key.notrequired"),
      });
    } else if (current.keyAvailable) {
      rows.push({
        id: "key",
        level: "ok",
        label: t("settings.ai.diagnostics.key"),
        detail: t("settings.ai.diagnostics.key.ok"),
      });
    } else {
      rows.push({
        id: "key",
        level: "warn",
        label: t("settings.ai.diagnostics.key"),
        detail: t("settings.ai.diagnostics.key.missing"),
      });
    }

    if (!current.model.trim()) {
      rows.push({
        id: "model",
        level: "fail",
        label: t("settings.ai.diagnostics.model"),
        detail: t("settings.ai.diagnostics.model.missing"),
      });
    } else {
      const price = findModelPrice(catalog.value, current.model);
      rows.push({
        id: "model",
        level: price ? "ok" : "warn",
        label: t("settings.ai.diagnostics.model"),
        detail: price
          ? price.provider === "local"
            ? t("settings.ai.diagnostics.model.local", { model: current.model })
            : t("settings.ai.diagnostics.model.ok", { model: current.model })
          : t("settings.ai.diagnostics.model.noprice", { model: current.model }),
      });
    }
  }

  const fallbackBase = adv.fallbackBaseUrl.trim();
  const fallbackModel = adv.fallbackModel.trim();
  if (!fallbackBase && !fallbackModel) {
    rows.push({
      id: "fallback",
      level: "ok",
      label: t("settings.ai.diagnostics.fallback"),
      detail: t("settings.ai.diagnostics.fallback.off"),
    });
  } else if (!fallbackBase || !fallbackModel) {
    rows.push({
      id: "fallback",
      level: "fail",
      label: t("settings.ai.diagnostics.fallback"),
      detail: t("settings.ai.diagnostics.fallback.partial"),
    });
  } else {
    let fallbackOK = false;
    try {
      fallbackOK = Boolean(apiOrigin(fallbackBase));
    } catch {
      fallbackOK = false;
    }
    rows.push({
      id: "fallback",
      level: fallbackOK ? "ok" : "fail",
      label: t("settings.ai.diagnostics.fallback"),
      detail: fallbackOK
        ? t("settings.ai.diagnostics.fallback.ok", {
            url: fallbackBase,
            model: fallbackModel,
          })
        : t("settings.ai.diagnostics.endpoint.invalid"),
    });
  }

  if (current.notice.trim()) {
    rows.push({
      id: "notice",
      level: "warn",
      label: t("settings.ai.diagnostics.notice"),
      detail: current.notice,
    });
  }
  return rows;
});

async function loadDiagnostics(): Promise<void> {
  isLoading.value = true;
  loadError.value = "";
  try {
    const [configResult, advancedResult, catalogResult, cliResult] = await Promise.all([
      getAIConfig(),
      getAIAdvancedConfig(),
      getAIProviderCatalog().catch(() => null),
      detectAICLIs().catch(() => []),
    ]);
    config.value = configResult;
    advanced.value = advancedResult;
    catalog.value = catalogResult;
    cliOptions.value = cliResult || [];
  } catch (err) {
    loadError.value = errorText(err);
  } finally {
    isLoading.value = false;
  }
}

watch(
  () => props.refreshKey,
  (next, previous) => {
    if (next !== previous) void loadDiagnostics();
  }
);

onMounted(loadDiagnostics);
</script>

<template>
  <div class="space-y-4" :aria-busy="isLoading">
    <div
      v-if="loadError"
      data-testid="ai-diagnostics-error"
      role="alert"
      class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger-bg text-[11px] text-danger"
    >
      <span class="min-w-0 flex-1 break-words">
        {{ t("settings.ai.control.loadfailed", { error: loadError }) }}
      </span>
      <UiButton variant="secondary" data-testid="ai-diagnostics-retry" @click="loadDiagnostics">
        {{ t("common.retry") }}
      </UiButton>
    </div>

    <!-- Local-only diagnostics: never sends a network request. -->
    <SettingsCard
  v-if="config && diagnostics.length"
  id="ai-section-diagnostics"
  data-testid="ai-diagnostics-card"
    >
      <template #header>
        <div class="flex items-center gap-2">
          <Cpu class="w-4 h-4 text-accent" aria-hidden="true" />
          <div>
            <h3 class="ai-card-title">{{ t("settings.ai.diagnostics.title") }}</h3>
            <p class="ai-card-desc">{{ t("settings.ai.diagnostics.desc") }}</p>
          </div>
        </div>
      </template>
      <ul class="space-y-1.5">
        <li
  v-for="row in diagnostics"
          :key="row.id"
  class="flex items-start gap-2 text-[11px]"
          :data-testid="`ai-diagnostic-${row.id}`"
        >
          <CheckCircle2 v-if="row.level === 'ok'" class="w-3.5 h-3.5 text-success shrink-0 mt-0.5" />
          <AlertTriangle v-else-if="row.level === 'warn'" class="w-3.5 h-3.5 text-warning shrink-0 mt-0.5" />
          <XCircle v-else class="w-3.5 h-3.5 text-danger shrink-0 mt-0.5" />
          <span class="text-ink-muted shrink-0">{{ row.label }}</span>
          <span class="min-w-0 text-ink break-all">{{ row.detail }}</span>
        </li>
      </ul>
    </SettingsCard>
  </div>
</template>
