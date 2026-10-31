<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted } from "vue";
import { RefreshCw, Search, Save, Wrench, ShieldCheck, Trash2, Sparkles } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { ConfigEntry, TeamGovernanceReport, ApplyGovernancePresetResult } from "../../types";
import { t } from "../../i18n";

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
  <div class="space-y-5">
    <div class="flex items-start justify-between gap-3 pb-2 border-b border-line">
      <div>
        <h2 class="text-sm font-bold text-ink flex items-center gap-2">
          <Wrench class="w-4 h-4 text-accent" />
          <span>{{ t("settings.config.title") }}</span>
        </h2>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("settings.config.intro") }}
        </p>
      </div>
      <button
        @click="loadConfig"
        :disabled="isLoading"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer disabled:opacity-50 shrink-0"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" aria-hidden="true" />
        <span>{{ isLoading ? t("settings.config.loading") : t("settings.config.reload") }}</span>
      </button>
    </div>

    <!-- Phase 56: Team Config Governance Hub -->
    <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3" data-testid="team-governance-card">
      <div class="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <div class="flex items-center gap-2">
            <Sparkles class="w-4 h-4 text-accent" />
            <span class="text-xs font-bold text-ink">{{ t("settings.gov.title") }}</span>
            <span
              v-if="governanceReport"
              data-testid="governance-score-badge"
              class="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold"
              :class="governanceReport.complianceScore >= 80 ? 'bg-success/15 text-success' : governanceReport.complianceScore >= 50 ? 'bg-warning/15 text-warning' : 'bg-danger/15 text-danger'"
            >
              {{ governanceReport.complianceScore }}/100
            </span>
          </div>
          <p class="text-[11px] text-ink-muted mt-1 leading-relaxed">
            {{ t("settings.gov.desc") }}
          </p>
        </div>

        <div class="flex items-center gap-2">
          <button
            type="button"
            data-testid="inspect-team-governance-btn"
            :disabled="isInspectingGov"
            @click="handleInspectGovernance"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised text-ink text-xs font-semibold cursor-pointer transition border border-line disabled:opacity-50"
          >
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isInspectingGov }" aria-hidden="true" />
            <span>{{ isInspectingGov ? t("settings.gov.inspecting") : t("settings.gov.inspect.btn") }}</span>
          </button>
        </div>
      </div>

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
          class="flex items-center justify-between p-2 rounded-lg bg-canvas/40 border border-line text-xs font-mono"
        >
          <div class="flex items-center gap-2 truncate">
            <span
              class="w-2 h-2 rounded-full shrink-0"
              :class="item.status === 'compliant' ? 'bg-success' : item.status === 'warning' ? 'bg-warning' : 'bg-danger'"
            />
            <span class="font-bold text-ink">{{ item.key }}</span>
            <span class="text-ink-muted text-[10px] truncate font-sans">{{ item.description }}</span>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <span class="text-accent text-[11px] truncate max-w-[120px]">{{ item.value || "unset" }}</span>
            <span
              class="px-1.5 py-0.2 rounded text-[9px] uppercase font-bold"
              :class="item.status === 'compliant' ? 'bg-success/15 text-success' : item.status === 'warning' ? 'bg-warning/15 text-warning' : 'bg-danger/15 text-danger'"
            >
              {{ item.status }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-3 gap-2">
      <div class="p-3 rounded-xl border border-line bg-surface/30">
        <div class="text-[10px] text-ink-muted">{{ t("settings.config.total") }}</div>
        <div class="text-lg font-bold font-mono text-ink">{{ entries.length }}</div>
      </div>
      <div class="p-3 rounded-xl border border-line bg-surface/30">
        <div class="text-[10px] text-ink-muted">{{ t("settings.config.local") }}</div>
        <div class="text-lg font-bold font-mono text-accent">{{ localCount }}</div>
      </div>
      <div class="p-3 rounded-xl border border-line bg-surface/30">
        <div class="text-[10px] text-ink-muted">{{ t("settings.config.global") }}</div>
        <div class="text-lg font-bold font-mono text-purple">{{ globalCount }}</div>
      </div>
    </div>

    <div class="flex items-center gap-2">
      <div class="relative flex-1">
        <Search class="w-3.5 h-3.5 text-ink-muted absolute left-2.5 top-1/2 -translate-y-1/2" />
        <input
          v-model="filter"
          type="text"
          :placeholder="t('settings.config.filter.placeholder')"
          class="w-full pl-8 pr-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
        />
      </div>
      <span class="text-[10px] text-ink-muted font-mono shrink-0">{{
        t("settings.config.filter.count", { count: filteredEntries.length })
      }}</span>
    </div>

    <div class="rounded-xl border border-line bg-surface/30 overflow-hidden">
      <div v-if="isLoading" class="py-8 text-center text-xs text-ink-muted">{{ t("settings.config.loading.long") }}</div>
      <div v-else-if="filteredEntries.length > 0" class="divide-y divide-line/40 max-h-[360px] overflow-y-auto">
        <div
          v-for="entry in filteredEntries"
          :key="`${entry.scope}:${entry.key}:${entry.value}`"
          class="px-3 py-1.5 flex items-center gap-2 text-[11px] font-mono hover:bg-surface/50"
        >
          <span
            class="px-1 py-0.2 rounded text-[9px] font-bold shrink-0"
            :class="entry.scope === 'global' ? 'bg-purple/15 text-purple' : 'bg-accent/15 text-accent'"
          >
            {{ entry.scope }}
          </span>
          <span class="text-ink truncate min-w-0 flex-1" :title="entry.key">{{ entry.key }}</span>
          <span class="text-ink-muted truncate max-w-[45%]" :title="entry.value">{{ entry.value }}</span>
        </div>
      </div>
      <div v-else class="py-8 text-center text-xs text-ink-muted">{{ t("settings.config.empty") }}</div>
    </div>

    <div class="rounded-xl border border-line bg-surface/30 p-3 space-y-2">
      <div class="text-xs font-bold text-ink">{{ t("settings.config.write.title") }}</div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-2">
        <input
          v-model="configKey"
          type="text"
          :placeholder="t('settings.config.write.key')"
          class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
        />
        <input
          v-model="configValue"
          type="text"
          :placeholder="t('settings.config.write.value')"
          @keydown.enter="handleSave"
          class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent"
        />
        <button
          @click="handleSave"
          :disabled="isSaving || !configKey.trim()"
          class="flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
        >
          <Save class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ isSaving ? t("settings.config.writing") : t("settings.config.write") }}</span>
        </button>
      </div>
      <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
        <input v-model="configGlobal" type="checkbox" class="rounded accent-danger" />
        <span>{{ t("settings.config.write.global") }}</span>
      </label>
      <p class="text-[10px] text-ink-muted">{{ t("settings.config.write.footnote") }}</p>
    </div>

    <!-- safe.directory management -->
    <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3">
      <div class="flex items-start justify-between gap-3 flex-wrap">
        <div>
          <div class="flex items-center gap-2">
            <ShieldCheck class="w-4 h-4 text-accent" />
            <span class="text-xs font-bold text-ink">{{ t("settings.safe.title") }}</span>
            <span
              class="px-2 py-0.5 rounded-full text-[10px] font-semibold"
              :class="isCurrentRepoSafe ? 'bg-success/15 text-success' : 'bg-warning/15 text-warning'"
            >
              {{ isCurrentRepoSafe ? t("settings.safe.trusted") : t("settings.safe.untrusted") }}
            </span>
          </div>
          <p class="text-[11px] text-ink-muted mt-1 leading-relaxed">
            {{ t("settings.safe.desc") }}
          </p>
        </div>
        <button
          v-if="!isCurrentRepoSafe"
          @click="handleAddCurrentToSafe"
          :disabled="isAddingSafe"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40 shrink-0"
        >
          <ShieldCheck class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{ isAddingSafe ? t("settings.safe.adding") : t("settings.safe.fix.action") }}</span>
        </button>
      </div>

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
              class="p-0.5 text-ink-muted hover:text-danger transition cursor-pointer shrink-0 opacity-0 group-hover:opacity-100"
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
  </div>
</template>
