<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted } from "vue";
import { RefreshCw, Search, Save, Wrench } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { ConfigEntry } from "../../types";
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
  }
);

onMounted(() => {
  void loadConfig();
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
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
        <span>{{ isLoading ? t("settings.config.loading") : t("settings.config.reload") }}</span>
      </button>
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
          <Save class="w-3.5 h-3.5" />
          <span>{{ isSaving ? t("settings.config.writing") : t("settings.config.write") }}</span>
        </button>
      </div>
      <label class="flex items-center gap-1.5 text-[11px] text-ink-muted cursor-pointer select-none">
        <input v-model="configGlobal" type="checkbox" class="rounded accent-danger" />
        <span>{{ t("settings.config.write.global") }}</span>
      </label>
      <p class="text-[10px] text-ink-muted">{{ t("settings.config.write.footnote") }}</p>
    </div>
  </div>
</template>
