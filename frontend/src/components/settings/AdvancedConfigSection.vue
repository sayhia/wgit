<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted } from "vue";
import { RefreshCw, Search, Save, ShieldCheck, Trash2 } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { ConfigEntry, TeamGovernanceReport, ApplyGovernancePresetResult } from "../../types";
import { t } from "../../i18n";
import SettingsPageHeader from "./SettingsPageHeader.vue";
import SettingsCard from "./SettingsCard.vue";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

const entries = ref<ConfigEntry[]>([]);
const filter = ref("");
const isLoading = ref(false);
const isSaving = ref(false);
const configKey = ref("");
const configValue = ref("");
const configGlobal = ref(false);

const safeDirectories = ref<string[]>([]);
const isCurrentRepoSafe = ref(true);
const isAddingSafe = ref(false);

async function loadSafeDirectories() {
  if (!props.currentRepoPath) return;
  try {
    const [dirs, safe] = await Promise.all([
      gitService.getSafeDirectories(),
      gitService.checkSafeDirectory(props.currentRepoPath),
    ]);
    safeDirectories.value = dirs || [];
    isCurrentRepoSafe.value = safe;
  } catch (err) {
    // best-effort: the section renders its empty state.
    console.debug("Failed to load safe directories:", err);
  }
}

async function handleAddCurrentToSafe() {
  if (!props.currentRepoPath || isAddingSafe.value) return;
  isAddingSafe.value = true;
  try {
    await gitService.addSafeDirectory(props.currentRepoPath);
    notify(t("settings.safe.added", { path: props.currentRepoPath }));
    await loadSafeDirectories();
  } catch (err: any) {
    notify(t("settings.safe.add.failed", { error: err?.message || err }));
  } finally {
    isAddingSafe.value = false;
  }
}

async function handleRemoveSafe(dir: string) {
  if (!dir) return;
  if (!(await confirmAction(t("settings.safe.remove.confirm", { path: dir })))) return;
  try {
    await gitService.removeSafeDirectory(dir);
    notify(t("settings.safe.removed", { path: dir }));
    await loadSafeDirectories();
  } catch (err: any) {
    notify(t("settings.safe.remove.failed", { error: err?.message || err }));
  }
}

// Phase 56: Team Governance & Best Practices Hub
const governanceReport = ref<TeamGovernanceReport | null>(null);
const isInspectingGov = ref(false);
const isApplyingPreset = ref(false);

async function handleInspectGovernance() {
  if (!props.currentRepoPath || isInspectingGov.value) return;
  isInspectingGov.value = true;
  try {
    governanceReport.value = await gitService.inspectRepoGitConfigs(props.currentRepoPath);
  } catch (err: any) {
    notify(String(err?.message || err));
  } finally {
    isInspectingGov.value = false;
  }
}

async function handleApplyPreset(preset: string) {
  if (!props.currentRepoPath || isApplyingPreset.value) return;
  isApplyingPreset.value = true;
  try {
    const res = await gitService.applyRecommendedTeamPreset(props.currentRepoPath, preset);
    if (res && res.success) {
      notify(res.message);
      await Promise.all([handleInspectGovernance(), loadConfig()]);
    }
  } catch (err: any) {
    notify(String(err?.message || err));
  } finally {
    isApplyingPreset.value = false;
  }
}

const filteredEntries = computed(() => {
  const q = filter.value.trim().toLowerCase();
  if (!q) return entries.value;
  return entries.value.filter(
    (entry) =>
      entry.key.toLowerCase().includes(q) || entry.value.toLowerCase().includes(q)
  );
});

const localCount = computed(() => entries.value.filter((entry) => entry.scope === "local").length);
const globalCount = computed(() => entries.value.filter((entry) => entry.scope === "global").length);

async function loadConfig() {
  if (!props.currentRepoPath || isLoading.value) return;
  isLoading.value = true;
  try {
    entries.value = (await gitService.getDetailedGitConfig(props.currentRepoPath)) || [];
  } catch (err: any) {
    notify(t("settings.config.load.failed", { error: err?.message || err }));
  } finally {
    isLoading.value = false;
  }
}

async function handleSave() {
  if (!props.currentRepoPath || isSaving.value) return;
  const key = configKey.value.trim();
  if (!key) {
    notify(t("settings.config.key.required"));
    return;
  }
  if (
    configGlobal.value &&
    !(await confirmAction(t("settings.config.confirm.global", { key, value: configValue.value })))
  )
    return;
  isSaving.value = true;
  try {
    await gitService.setCustomConfigKey(
      props.currentRepoPath,
      key,
      configValue.value,
      configGlobal.value
    );
    notify(
      t(
        configGlobal.value
          ? "settings.config.saved.global"
          : "settings.config.saved.local",
        { key }
      )
    );
    configKey.value = "";
    configValue.value = "";
    await loadConfig();
  } catch (err: any) {
    notify(t("settings.config.save.failed", { error: err?.message || err }));
  } finally {
    isSaving.value = false;
  }
}

watch(
  () => props.currentRepoPath,
  () => {
    entries.value = [];
    void loadConfig();
    void loadSafeDirectories();
    void handleInspectGovernance();
  }
);

onMounted(() => {
  void loadConfig();
  void loadSafeDirectories();
  void handleInspectGovernance();
});
</script>

<template>
  <div class="space-y-4 min-w-0 @container">
    <SettingsPageHeader :title="t('settings.config.title')" :description="t('settings.config.intro')">
      <template #actions>
        <button type="button" @click="loadConfig" :disabled="isLoading" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-medium text-ink transition cursor-pointer disabled:opacity-50 shrink-0">
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" aria-hidden="true" />
          <span>{{ isLoading ? t("settings.config.loading") : t("settings.config.reload") }}</span>
        </button>
      </template>
    </SettingsPageHeader>

    <!-- Phase 56: Team Config Governance Hub -->
    <SettingsCard data-testid="team-governance-card">
      <template #header>
        <div class="flex flex-wrap items-center gap-2">
          <h3 class="text-xs font-semibold text-ink">{{ t("settings.gov.title") }}</h3>
          <span v-if="governanceReport" data-testid="governance-score-badge" class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold" :class="governanceReport.complianceScore >= 80 ? 'bg-success-bg text-success' : governanceReport.complianceScore >= 50 ? 'bg-warning-bg text-warning' : 'bg-danger-bg text-danger'">{{ governanceReport.complianceScore }}/100</span>
        </div>
      </template>
      <template #actions>
        <button type="button" data-testid="inspect-team-governance-btn" :disabled="isInspectingGov" @click="handleInspectGovernance" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised text-ink text-xs font-medium cursor-pointer transition border border-line disabled:opacity-50">
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isInspectingGov }" aria-hidden="true" />
          <span>{{ isInspectingGov ? t("settings.gov.inspecting") : t("settings.gov.inspect.btn") }}</span>
        </button>
      </template>
      <div class="space-y-3">
        <p class="text-xs text-ink-secondary leading-relaxed">{{ t("settings.gov.desc") }}</p>
        <!-- Presets Quick Bar -->
        <div class="flex flex-wrap items-center gap-2 pt-2 border-t border-line/40">
          <span class="text-[10px] text-ink-muted shrink-0">{{ t("settings.gov.preset.title") }}:</span>
          <button
            type="button"
            data-testid="apply-governance-preset-btn"
            :disabled="isApplyingPreset"
            @click="handleApplyPreset('cross-platform')"
            class="px-2.5 py-1 rounded-lg border border-line bg-canvas hover:bg-raised text-xs text-ink font-medium transition cursor-pointer disabled:opacity-50"
            :title="t('settings.gov.preset.cross.desc')"
          >
            <span>{{ t("settings.gov.preset.cross") }}</span>
          </button>
          <button
            type="button"
            data-testid="apply-governance-preset-btn"
            :disabled="isApplyingPreset"
            @click="handleApplyPreset('standard-dev')"
            class="px-2.5 py-1 rounded-lg border border-line bg-canvas hover:bg-raised text-xs text-ink font-medium transition cursor-pointer disabled:opacity-50"
            :title="t('settings.gov.preset.standard.desc')"
          >
            <span>{{ t("settings.gov.preset.standard") }}</span>
          </button>
          <button
            type="button"
            data-testid="apply-governance-preset-btn"
            :disabled="isApplyingPreset"
            @click="handleApplyPreset('security-strict')"
            class="px-2.5 py-1 rounded-lg border border-line bg-canvas hover:bg-raised text-xs text-ink font-medium transition cursor-pointer disabled:opacity-50"
            :title="t('settings.gov.preset.security.desc')"
          >
            <span>{{ t("settings.gov.preset.security") }}</span>
          </button>
        </div>

        <!-- Checklist -->
        <div v-if="governanceReport && governanceReport.configs.length > 0" class="space-y-1 pt-1 max-h-48 overflow-y-auto">
          <div
            v-for="item in governanceReport.configs"
            :key="item.key"
            class="flex flex-wrap items-center justify-between gap-2 p-2 rounded-lg bg-canvas border border-line text-xs font-mono"
          >
            <div class="flex flex-wrap items-center gap-2 min-w-0 flex-1 basis-48">
            <span
              class="w-2 h-2 rounded-full shrink-0"
              :class="item.status === 'compliant' ? 'bg-success' : item.status === 'warning' ? 'bg-warning' : 'bg-danger'"
            />
              <span class="font-medium text-ink break-all">{{ item.key }}</span>
              <span class="text-ink-muted text-[11px] break-words font-sans">{{ item.description }}</span>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <span class="text-accent text-[11px] truncate max-w-[120px]">{{ item.value || "unset" }}</span>
              <span
                class="px-1.5 py-0.2 rounded text-[9px] uppercase font-bold"
                :class="item.status === 'compliant' ? 'bg-success-bg text-success' : item.status === 'warning' ? 'bg-warning-bg text-warning' : 'bg-danger-bg text-danger'"
              >
                {{ item.status }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </SettingsCard>

    <div class="grid grid-cols-3 gap-2">
      <div class="p-3 rounded-xl border border-line bg-surface">
        <div class="text-[10px] text-ink-muted">{{ t("settings.config.total") }}</div>
        <div class="text-lg font-bold font-mono text-ink">{{ entries.length }}</div>
      </div>
      <div class="p-3 rounded-xl border border-line bg-surface">
        <div class="text-[10px] text-ink-muted">{{ t("settings.config.local") }}</div>
        <div class="text-lg font-bold font-mono text-accent">{{ localCount }}</div>
      </div>
      <div class="p-3 rounded-xl border border-line bg-surface">
        <div class="text-[10px] text-ink-muted">{{ t("settings.config.global") }}</div>
        <div class="text-lg font-bold font-mono text-purple">{{ globalCount }}</div>
      </div>
    </div>

    <SettingsCard :padded="false">
      <div class="flex flex-wrap items-center gap-2 p-4 border-b border-line/60">
        <div class="relative min-w-0 flex-1 basis-40">
          <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2.5 top-1/2 -translate-y-1/2" />
          <input
            v-model="filter"
            type="text"
            :placeholder="t('settings.config.filter.placeholder')"
            :aria-label="t('settings.config.filter.placeholder')"
            class="w-full pl-8 pr-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </div>
      <span class="text-[10px] text-ink-muted font-mono shrink-0">{{
        t("settings.config.filter.count", { count: filteredEntries.length })
      }}</span>
      </div>

      <div class="overflow-hidden">
        <div v-if="isLoading" class="py-8 text-center text-xs text-ink-muted">{{ t("settings.config.loading.long") }}</div>
        <div v-else-if="filteredEntries.length > 0" class="divide-y divide-line/40 max-h-[360px] overflow-y-auto">
          <div
            v-for="entry in filteredEntries"
            :key="`${entry.scope}:${entry.key}:${entry.value}`"
            class="px-3 py-1.5 flex items-center gap-2 text-[11px] font-mono hover:bg-surface"
          >
          <span
            class="px-1 py-0.2 rounded text-[9px] font-bold shrink-0"
            :class="entry.scope === 'global' ? 'bg-purple-bg text-purple' : 'bg-accent-bg text-accent'"
          >
              {{ entry.scope }}
            </span>
            <span class="text-ink truncate min-w-0 flex-1" :title="entry.key">{{ entry.key }}</span>
            <span class="text-ink-muted truncate max-w-[45%]" :title="entry.value">{{ entry.value }}</span>
          </div>
        </div>
        <div v-else class="py-8 text-center text-xs text-ink-muted">{{ t("settings.config.empty") }}</div>
      </div>

    </SettingsCard>

    <SettingsCard class="@container">
      <template #header><h3 class="text-xs font-semibold text-ink">{{ t("settings.config.write.title") }}</h3></template>
      <div class="space-y-3">
        <div class="grid grid-cols-1 @min-[36rem]:grid-cols-2 gap-3">
          <label class="min-w-0 space-y-1.5">
            <span class="block text-xs text-ink font-medium">{{ t("settings.config.write.key") }}</span>
            <input v-model="configKey" type="text" :aria-label="t('settings.config.write.key')" :placeholder="t('settings.config.write.key')" class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent" />
          </label>
          <label class="min-w-0 space-y-1.5">
            <span class="block text-xs text-ink font-medium">{{ t("settings.config.write.value") }}</span>
            <input v-model="configValue" type="text" :aria-label="t('settings.config.write.value')" :placeholder="t('settings.config.write.value')" @keydown.enter="handleSave" class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent" />
          </label>
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <label class="flex items-start gap-2 text-xs text-ink-secondary cursor-pointer select-none">
            <input v-model="configGlobal" type="checkbox" class="mt-0.5 shrink-0 rounded accent-danger" />
            <span>{{ t("settings.config.write.global") }}</span>
          </label>
          <button type="button" @click="handleSave" :disabled="isSaving || !configKey.trim()" class="flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0">
            <Save class="w-3.5 h-3.5" aria-hidden="true" />
            <span>{{ isSaving ? t("settings.config.writing") : t("settings.config.write") }}</span>
          </button>
        </div>
        <p class="text-[11px] text-ink-muted leading-relaxed">{{ t("settings.config.write.footnote") }}</p>
      </div>
    </SettingsCard>

    <!-- safe.directory management -->
    <SettingsCard>
      <template #header>
        <div class="flex flex-wrap items-center gap-2">
          <h3 class="text-xs font-semibold text-ink">{{ t("settings.safe.title") }}</h3>
          <span class="px-2 py-0.5 rounded-full text-[10px] font-medium" :class="isCurrentRepoSafe ? 'bg-success-bg text-success' : 'bg-warning-bg text-warning'">{{ isCurrentRepoSafe ? t("settings.safe.trusted") : t("settings.safe.untrusted") }}</span>
        </div>
      </template>
      <template v-if="!isCurrentRepoSafe" #actions>
        <button type="button" @click="handleAddCurrentToSafe" :disabled="isAddingSafe" class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0">
          <ShieldCheck class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ isAddingSafe ? t("settings.safe.adding") : t("settings.safe.fix.action") }}</span>
        </button>
      </template>
      <div class="space-y-3">
        <p class="text-xs text-ink-secondary leading-relaxed">{{ t("settings.safe.desc") }}</p>
        <div v-if="safeDirectories.length > 0" class="space-y-1.5 pt-2 border-t border-line/40">
          <span class="text-[10px] text-ink-muted">{{ t("settings.safe.configured.title") }}:</span>
          <div class="space-y-1 max-h-36 overflow-y-auto">
            <div
              v-for="dir in safeDirectories"
              :key="dir"
              class="flex items-center justify-between gap-2 px-2.5 py-1 rounded bg-canvas border border-line text-[11px] font-mono group"
            >
              <span class="text-ink truncate min-w-0" :title="dir">{{ dir }}</span>
              <button
                @click="handleRemoveSafe(dir)"
                class="p-0.5 text-ink-muted hover:text-danger transition cursor-pointer shrink-0 opacity-70 hover:opacity-100 focus-visible:opacity-100"
                :title="t('settings.safe.remove.title')"
              >
                <Trash2 class="w-3 h-3" aria-hidden="true" />
              </button>
            </div>
          </div>
        </div>
        <div v-else class="text-[10px] text-ink-muted">
          {{ t("settings.safe.none") }}
        </div>
      </div>
    </SettingsCard>
  </div>
</template>
