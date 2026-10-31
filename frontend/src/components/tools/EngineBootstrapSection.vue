<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch } from "vue";
import { Sparkles, RefreshCw, CheckCircle2, AlertTriangle, MinusCircle, Rocket } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { EngineBootstrapResult, EngineBootstrapStep } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
  /** Detected engine display name, used only for the default hint. */
  engineType?: string;
  readiness: number;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
}>();

const writeGitattributes = ref(true);
const writeGitignore = ref(true);
const untrackArtifacts = ref(true);
const configureUnityMerge = ref(true);
const lfsPatterns = ref("*.psd,*.fbx,*.uasset,*.umap,*.tga,*.exr,*.wav");
const isRunning = ref(false);
const result = ref<EngineBootstrapResult | null>(null);

const isUnity = computed(() => (props.engineType || "").toLowerCase().includes("unity"));

function parsePatterns(input: string): string[] {
  // TrackExtensionsAsLfs expects bare extensions ("psd") or globs ("*.psd").
  return input
    .split(/[,\n]/)
    .map((p) => p.trim())
    .filter(Boolean);
}

async function runBootstrap() {
  if (!props.currentRepoPath || isRunning.value) return;
  const patterns = parsePatterns(lfsPatterns.value);
  const bullet = (key: string) => t("engine.bootstrap.confirm.bullet", { text: t(key) });
  const lines: string[] = [t("engine.bootstrap.confirm.intro"), ""];
  if (writeGitattributes.value) lines.push(bullet("engine.bootstrap.confirm.gitattributes"));
  if (writeGitignore.value) {
    lines.push(
      bullet(
        untrackArtifacts.value
          ? "engine.bootstrap.confirm.gitignore.untrack"
          : "engine.bootstrap.confirm.gitignore"
      )
    );
  }
  if (isUnity.value && configureUnityMerge.value) lines.push(bullet("engine.bootstrap.confirm.unity"));
  if (patterns.length) {
    lines.push(
      bullet(
        t("engine.bootstrap.confirm.lfs", {
          patterns: patterns.join(t("common.list.comma")),
        })
      )
    );
  }
  const confirmed = await confirmAction({ message: lines.join("\n"), tone: "danger" });
  if (!confirmed) return;

  isRunning.value = true;
  try {
    const res = await gitService.runEngineBootstrap(props.currentRepoPath, {
      engine: "",
      writeGitattributes: writeGitattributes.value,
      writeGitignore: writeGitignore.value,
      untrackArtifacts: untrackArtifacts.value,
      configureUnityMerge: isUnity.value && configureUnityMerge.value,
      lfsExtensions: patterns,
    });
    result.value = res;
    emit("notify", res.message);
    emit("refresh");
  } catch (err: any) {
    emit("notify", t("engine.bootstrap.toast.failed", { error: err?.message || err }));
  } finally {
    isRunning.value = false;
  }
}

function stepMeta(status: EngineBootstrapStep["status"]) {
  switch (status) {
    case "applied":
      return { icon: CheckCircle2, cls: "text-success", badge: t("engine.bootstrap.status.applied") };
    case "failed":
      return { icon: AlertTriangle, cls: "text-danger", badge: t("engine.bootstrap.status.failed") };
    default:
      return { icon: MinusCircle, cls: "text-ink-muted", badge: t("engine.bootstrap.status.skipped") };
  }
}

watch(
  () => [props.currentRepoPath, props.refreshNonce],
  () => {
    result.value = null;
  }
);
</script>

<template>
  <div class="space-y-3" data-tool-section="engine-bootstrap">
    <div class="pb-2 border-b border-line flex items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Sparkles class="w-4 h-4 text-accent" />
          <span>{{ t("engine.bootstrap.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          {{ t("engine.bootstrap.intro") }}
        </p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <div class="text-right">
          <div class="text-[10px] text-ink-muted">{{ t("engine.bootstrap.readiness") }}</div>
          <div
            class="text-sm font-bold font-mono"
            :class="readiness >= 85 ? 'text-success' : readiness >= 50 ? 'text-warning' : 'text-danger'"
          >{{ readiness }}%</div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-1.5">
      <label class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition">
        <input v-model="writeGitattributes" type="checkbox" class="mt-0.5 accent-[var(--ug-accent-strong)]" />
        <span class="min-w-0">
          <span class="block text-[11px] font-semibold text-ink">{{ t("engine.bootstrap.gitattributes") }}</span>
          <span class="block text-[10px] text-ink-muted">{{ t("engine.bootstrap.gitattributes.desc") }}</span>
        </span>
      </label>

      <label class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition">
        <input v-model="writeGitignore" type="checkbox" class="mt-0.5 accent-[var(--ug-accent-strong)]" />
        <span class="min-w-0">
          <span class="block text-[11px] font-semibold text-ink">{{ t("engine.bootstrap.gitignore") }}</span>
          <span class="block text-[10px] text-ink-muted">{{ t("engine.bootstrap.gitignore.desc") }}</span>
        </span>
      </label>

      <label
        v-if="writeGitignore"
        class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition"
      >
        <input v-model="untrackArtifacts" type="checkbox" class="mt-0.5 accent-[var(--ug-accent-strong)]" />
        <span class="min-w-0">
          <span class="block text-[11px] font-semibold text-ink">{{ t("engine.bootstrap.untrack") }}</span>
          <span class="block text-[10px] text-ink-muted">{{ t("engine.bootstrap.untrack.desc") }}</span>
        </span>
      </label>

      <label
        v-if="isUnity"
        class="flex items-start gap-2 px-2.5 py-1.5 rounded-lg border border-line bg-surface/30 cursor-pointer hover:bg-raised/50 transition"
      >
        <input v-model="configureUnityMerge" type="checkbox" class="mt-0.5 accent-[var(--ug-accent-strong)]" />
        <span class="min-w-0">
          <span class="block text-[11px] font-semibold text-ink">{{ t("engine.bootstrap.unity") }}</span>
          <span class="block text-[10px] text-ink-muted">{{ t("engine.bootstrap.unity.desc") }}</span>
        </span>
      </label>
    </div>

    <div class="space-y-1.5">
      <label class="text-[11px] font-semibold text-ink-secondary">{{ t("engine.bootstrap.lfs") }}</label>
      <input
        v-model="lfsPatterns"
        spellcheck="false"
        placeholder="*.psd,*.fbx,*.uasset"
        class="w-full px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs font-mono text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
      />
    </div>

    <button
      @click="runBootstrap"
      :disabled="isRunning"
      class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
    >
      <Rocket class="w-3.5 h-3.5" :class="{ 'animate-pulse': isRunning }" aria-hidden="true" />
      <span>{{ isRunning ? t("engine.bootstrap.running") : t("engine.bootstrap.run") }}</span>
    </button>

    <div v-if="result" class="space-y-1.5">
      <div
        class="rounded-lg border px-2.5 py-1.5 text-[11px]"
        :class="result.failed > 0 ? 'border-warning/30 bg-warning/10 text-warning' : 'border-success/30 bg-success/10 text-success'"
      >
        {{ t("engine.bootstrap.result", { message: result.message, engine: result.engine }) }}
      </div>
      <div
        v-for="step in result.steps"
        :key="step.id"
        class="flex items-start gap-2 px-2.5 py-1 rounded-lg border border-line bg-surface/30 text-[11px]"
      >
        <component :is="stepMeta(step.status).icon" class="w-3.5 h-3.5 shrink-0 mt-0.5" :class="stepMeta(step.status).cls" />
        <span class="font-semibold text-ink shrink-0">{{ step.label }}</span>
        <span class="text-ink-muted flex-1">{{ step.detail }}</span>
      </div>
    </div>
  </div>
</template>
