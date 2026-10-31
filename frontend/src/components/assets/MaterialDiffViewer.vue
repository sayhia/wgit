<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import {
  Sparkles,
  Layers,
  Palette,
  Sliders,
  Image,
  RefreshCw,
  Filter,
  Check,
  ArrowRight,
} from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { MaterialDiffResult, MaterialPropertyItem } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  repoPath: string;
  path: string;
  oldRef: string;
  newRef: string;
}>();

const isLoading = ref(false);
const error = ref("");
const diffData = ref<MaterialDiffResult | null>(null);
const onlyChanged = ref(true);
const filterQuery = ref("");

async function loadMaterialDiff() {
  if (!props.repoPath || !props.path) return;
  isLoading.value = true;
  error.value = "";
  try {
    const res = await gitService.compareMaterialProperties(
      props.repoPath,
      props.path,
      props.oldRef,
      props.newRef
    );
    diffData.value = res;
  } catch (err: any) {
    error.value = err?.message || String(err);
  } finally {
    isLoading.value = false;
  }
}

watch(
  () => [props.repoPath, props.path, props.oldRef, props.newRef],
  () => {
    void loadMaterialDiff();
  },
  { immediate: true }
);

const visibleProperties = computed(() => {
  if (!diffData.value?.properties) return [];
  let list = diffData.value.properties;
  if (onlyChanged.value) {
    list = list.filter((p) => p.changed);
  }
  if (filterQuery.value.trim()) {
    const q = filterQuery.value.toLowerCase();
    list = list.filter((p) => p.name.toLowerCase().includes(q));
  }
  return list;
});
</script>

<template>
  <div class="flex-1 flex flex-col h-full bg-canvas overflow-hidden">
    <!-- Header Controls -->
    <div class="px-4 py-2.5 border-b border-line bg-surface flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <div class="w-7 h-7 rounded-lg bg-accent-bg text-accent flex items-center justify-center">
          <Palette class="w-4 h-4" />
        </div>
        <div>
          <span class="text-xs font-bold text-ink">{{ t("assets.material.title") }}</span>
          <span class="text-[10px] text-ink-muted ml-2">
            {{ diffData ? t("assets.material.changes.count", { count: diffData.totalChanges }) : "" }}
          </span>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <input
          v-model="filterQuery"
          type="text"
          :placeholder="t('assets.material.search')"
          class="px-2.5 py-1 rounded-md bg-canvas border border-line text-xs text-ink placeholder:text-ink-muted focus:outline-none focus:border-accent w-40"
        />

        <button
          type="button"
          @click="onlyChanged = !onlyChanged"
          class="px-2.5 py-1 rounded-md border text-xs font-medium transition cursor-pointer flex items-center gap-1.5"
          :class="
            onlyChanged
              ? 'bg-accent-bg border-accent text-accent'
              : 'bg-surface border-line text-ink-secondary hover:bg-raised'
          "
        >
          <Filter class="w-3 h-3" />
          <span>{{ onlyChanged ? t("assets.material.filter.changed") : t("assets.material.filter.all") }}</span>
        </button>

        <button
          type="button"
          @click="loadMaterialDiff"
          :disabled="isLoading"
          class="p-1.5 rounded-md bg-surface border border-line text-ink-secondary hover:text-ink hover:bg-raised transition cursor-pointer"
          :title="t('review.refresh')"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
        </button>
      </div>
    </div>

    <!-- Shader Comparison Header -->
    <div
      v-if="diffData"
      class="px-4 py-2 bg-raised border-b border-line flex items-center justify-between text-xs font-mono"
    >
      <div class="flex items-center gap-2">
        <span class="text-ink-muted text-[11px] font-sans font-medium">{{ t("assets.material.shader") }}:</span>
        <span class="text-ink-secondary px-1.5 py-0.5 rounded bg-surface border border-line/60">
          {{ diffData.oldShader || "None" }}
        </span>
        <ArrowRight class="w-3 h-3 text-ink-muted shrink-0" />
        <span
          class="px-1.5 py-0.5 rounded border"
          :class="diffData.shaderChanged ? 'bg-warning-bg border-warning/40 text-warning font-bold' : 'bg-surface border-line/60 text-ink'"
        >
          {{ diffData.newShader || "None" }}
        </span>
      </div>
    </div>

    <!-- Loading / Error states -->
    <div v-if="isLoading" class="flex-1 flex items-center justify-center text-ink-muted text-xs">
      <RefreshCw class="w-5 h-5 animate-spin mr-2" />
      <span>{{ t("ui.loading") }}</span>
    </div>

    <div v-else-if="error" class="flex-1 flex items-center justify-center p-6 text-danger text-xs text-center">
      <span>{{ error }}</span>
    </div>

    <!-- Properties Grid -->
    <div v-else class="flex-1 overflow-y-auto p-4 space-y-2">
      <div
        v-if="visibleProperties.length === 0"
        class="text-center py-12 text-ink-muted text-xs"
      >
        {{ t("assets.material.empty") }}
      </div>

      <div
        v-for="prop in visibleProperties"
        :key="prop.name"
        class="p-3 rounded-xl border transition-all flex flex-col gap-2"
        :class="
          prop.changed
            ? 'bg-accent-bg border-accent/40 shadow-xs'
            : 'bg-surface border-line hover:border-line-hover'
        "
      >
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <span class="font-mono text-xs font-bold text-ink">{{ prop.name }}</span>
            <span
              class="px-1.5 py-0.2 rounded text-[9px] font-mono uppercase"
              :class="{
                'bg-purple-bg text-purple': prop.type === 'color',
                'bg-blue-500/15 text-blue-500': prop.type === 'float',
                'bg-emerald-500/15 text-emerald-500': prop.type === 'texture',
                'bg-raised text-ink-muted': prop.type === 'vector',
              }"
            >
              {{ prop.type }}
            </span>
          </div>

          <span
            v-if="prop.changed"
            class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-warning-bg text-warning border border-warning/30"
          >
            {{ t("assets.material.badge.modified") }}
          </span>
          <span
            v-else
            class="text-[10px] text-ink-muted"
          >
            {{ t("assets.material.badge.unchanged") }}
          </span>
        </div>

        <!-- Value Renderers depending on Type -->
        <!-- 1. COLOR PROPERTY WITH COLOR SWATCH -->
        <div v-if="prop.type === 'color'" class="flex items-center gap-3 pt-1">
          <div class="flex items-center gap-2">
            <div
              v-if="prop.colorHexOld"
              class="w-6 h-6 rounded-md border border-line shadow-inner shrink-0"
              :style="{ backgroundColor: prop.colorHexOld }"
              :title="prop.colorHexOld"
            ></div>
            <span class="font-mono text-xs text-ink-muted">{{ prop.colorHexOld || prop.oldValue || "None" }}</span>
          </div>

          <ArrowRight class="w-3.5 h-3.5 text-ink-muted shrink-0" />

          <div class="flex items-center gap-2">
            <div
              v-if="prop.colorHexNew"
              class="w-6 h-6 rounded-md border border-line shadow-inner shrink-0 ring-1 ring-accent"
              :style="{ backgroundColor: prop.colorHexNew }"
              :title="prop.colorHexNew"
            ></div>
            <span class="font-mono text-xs font-bold text-ink">{{ prop.colorHexNew || prop.newValue }}</span>
          </div>
        </div>

        <!-- 2. FLOAT PROPERTY WITH DELTA -->
        <div v-else-if="prop.type === 'float'" class="flex items-center gap-3 pt-1 font-mono text-xs">
          <span class="text-ink-muted">{{ prop.oldValue || "0" }}</span>
          <ArrowRight class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span class="font-bold text-ink">{{ prop.newValue }}</span>
          <span
            v-if="prop.deltaFloat !== undefined && prop.deltaFloat !== 0"
            class="px-1.5 py-0.2 rounded text-[10px] font-bold"
            :class="prop.deltaFloat > 0 ? 'bg-success-bg text-success' : 'bg-danger-bg text-danger'"
          >
            {{ prop.deltaFloat > 0 ? `+${prop.deltaFloat.toFixed(3)}` : prop.deltaFloat.toFixed(3) }}
          </span>
        </div>

        <!-- 3. TEXTURE / VECTOR PROPERTY -->
        <div v-else class="flex items-center gap-2 pt-1 font-mono text-xs">
          <span class="text-ink-muted truncate max-w-xs">{{ prop.oldValue || "None" }}</span>
          <ArrowRight class="w-3.5 h-3.5 text-ink-muted shrink-0" />
          <span class="font-bold text-ink truncate max-w-xs">{{ prop.newValue || "None" }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
