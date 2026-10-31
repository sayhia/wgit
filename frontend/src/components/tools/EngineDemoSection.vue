<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { computed, ref } from "vue";
import { AlertTriangle, CheckCircle2, ChevronDown, ChevronRight, Play } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
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

interface DemoField {
  key: string;
  /** i18n key. */
  label: string;
  placeholder: string;
  defaultValue?: string;
}

interface DemoTool {
  id: string;
  /** i18n key. */
  name: string;
  /** i18n key. */
  description: string;
  fields: DemoField[];
  real?: boolean;
  run: (args: Record<string, string>) => Promise<unknown>;
}

const tools: DemoTool[] = [
  {
    id: "fonts",
    name: "engine.demo.fonts.name",
    description: "engine.demo.fonts.description",
    real: true,
    fields: [],
    run: () => gitService.auditFontLicenses(props.currentRepoPath),
  },
  {
    id: "lightmap",
    name: "engine.demo.lightmap.name",
    description: "engine.demo.lightmap.description",
    real: true,
    fields: [{ key: "level", label: "engine.demo.lightmap.level.label", placeholder: "Assets/Levels/MainCity" }],
    run: (args) => gitService.auditLightmapBakeSettings(props.currentRepoPath, args.level || ""),
  },
  {
    id: "texturePacking",
    name: "engine.demo.texturepacking.name",
    description: "engine.demo.texturepacking.description",
    real: true,
    fields: [{ key: "dir", label: "engine.demo.field.scandir.label", placeholder: "Assets/Textures" }],
    run: (args) => gitService.auditTextureChannelPacking(props.currentRepoPath, args.dir || ""),
  },
  {
    id: "cycles",
    name: "engine.demo.cycles.name",
    description: "engine.demo.cycles.description",
    real: true,
    fields: [{ key: "dir", label: "engine.demo.field.scandir.label", placeholder: "Assets" }],
    run: (args) => gitService.detectAssetCyclicDependencies(props.currentRepoPath, args.dir || ""),
  },
  {
    id: "hotfixBackport",
    name: "engine.demo.hotfixbackport.name",
    description: "engine.demo.hotfixbackport.description",
    real: true,
    fields: [
      { key: "hotfix", label: "engine.demo.hotfixbackport.hotfix.label", placeholder: "hotfix/v1.0.1", defaultValue: "hotfix/v1.0.1" },
      { key: "target", label: "engine.demo.hotfixbackport.target.label", placeholder: "main", defaultValue: "main" },
    ],
    run: (args) =>
      gitService.traceHotfixBackportStatus(
        props.currentRepoPath,
        args.hotfix || "hotfix/v1.0.1",
        args.target || "main"
      ),
  },
  {
    id: "bundleDiff",
    name: "engine.demo.bundlediff.name",
    description: "engine.demo.bundlediff.description",
    real: true,
    fields: [
      { key: "base", label: "engine.demo.bundlediff.base.label", placeholder: "base_manifest.json", defaultValue: "base_manifest.json" },
      { key: "target", label: "engine.demo.bundlediff.target.label", placeholder: "target_manifest.json", defaultValue: "target_manifest.json" },
    ],
    run: (args) =>
      gitService.analyzeBundleDiff(
        props.currentRepoPath,
        args.base || "base_manifest.json",
        args.target || "target_manifest.json"
      ),
  },
  {
    id: "audioBank",
    name: "engine.demo.audiobank.name",
    description: "engine.demo.audiobank.description",
    real: true,
    fields: [
      { key: "path", label: "engine.demo.audiobank.path.label", placeholder: "Audio/Main.bnk", defaultValue: "Audio/Main.bnk" },
      { key: "base", label: "engine.demo.field.base.label", placeholder: "HEAD~1", defaultValue: "HEAD~1" },
      { key: "target", label: "engine.demo.field.target.label", placeholder: "HEAD", defaultValue: "HEAD" },
    ],
    run: (args) =>
      gitService.auditAudioBankDiff(
        props.currentRepoPath,
        args.path || "Audio/Main.bnk",
        args.base || "HEAD~1",
        args.target || "HEAD"
      ),
  },
  {
    id: "worldLayers",
    name: "engine.demo.worldlayers.name",
    description: "engine.demo.worldlayers.description",
    real: true,
    fields: [
      { key: "base", label: "engine.demo.field.base.label", placeholder: "HEAD~1", defaultValue: "HEAD~1" },
      { key: "target", label: "engine.demo.field.target.label", placeholder: "HEAD", defaultValue: "HEAD" },
    ],
    run: (args) =>
      gitService.diffWorldDataLayers(
        props.currentRepoPath,
        args.base || "HEAD~1",
        args.target || "HEAD"
      ),
  },
  {
    id: "offlinePatch",
    name: "engine.demo.offlinepatch.name",
    description: "engine.demo.offlinepatch.description",
    real: true,
    fields: [
      { key: "base", label: "engine.demo.field.base.label", placeholder: "HEAD~1", defaultValue: "HEAD~1" },
      { key: "target", label: "engine.demo.field.target.label", placeholder: "HEAD", defaultValue: "HEAD" },
      { key: "output", label: "engine.demo.offlinepatch.output.label", placeholder: "offline_patch.wgitbundle" },
    ],
    run: (args) =>
      gitService.generateOfflinePatchBundle(
        props.currentRepoPath,
        args.base || "HEAD~1",
        args.target || "HEAD",
        args.output || ""
      ),
  },
  {
    id: "shaderHeal",
    name: "engine.demo.shaderheal.name",
    description: "engine.demo.shaderheal.description",
    real: true,
    fields: [
      { key: "action", label: "engine.demo.shaderheal.action.label", placeholder: "HEAL_ALL / CLEAR_SHADER_CACHE", defaultValue: "HEAL_ALL" },
    ],
    run: (args) => gitService.shaderSelfHealingWizard(props.currentRepoPath, args.action || "HEAL_ALL"),
  },
];

const hasDemoTools = computed(() => tools.some((tool) => !tool.real));
const expanded = ref(false);
const selectedId = ref(tools[0]?.id || "");
const values = ref<Record<string, string>>({});
const isRunning = ref(false);
const resultJson = ref("");
const lastToolName = ref("");

const selectedTool = computed(() => tools.find((tool) => tool.id === selectedId.value) || tools[0]);

function fieldValue(field: DemoField) {
  const key = `${selectedId.value}.${field.key}`;
  return values.value[key] ?? field.defaultValue ?? "";
}

function setFieldValue(field: DemoField, value: string) {
  values.value[`${selectedId.value}.${field.key}`] = value;
}

async function handleRun() {
  const tool = selectedTool.value;
  if (!tool || !props.currentRepoPath || isRunning.value) return;
  if (tool.id === "shaderHeal") {
    const actionField = tool.fields[0];
    const action = (actionField ? fieldValue(actionField).trim() : "HEAL_ALL").toUpperCase() || "HEAL_ALL";
    if (
      (action === "HEAL_ALL" || action === "CLEAR_SHADER_CACHE") &&
      !(await confirmAction(t("engine.demo.confirm.shader.cache")))
    ) {
      return;
    }
  }
  isRunning.value = true;
  resultJson.value = "";
  lastToolName.value = t(tool.name);
  try {
    const args: Record<string, string> = {};
    tool.fields.forEach((field) => {
      args[field.key] = fieldValue(field).trim();
    });
    const result = await tool.run(args);
    resultJson.value = JSON.stringify(result ?? null, null, 2);
    notify(
      tool.real
        ? t("engine.demo.toast.ran", { tool: t(tool.name) })
        : t("engine.demo.toast.ran.demo", { tool: t(tool.name) })
    );
  } catch (err: any) {
    resultJson.value = JSON.stringify({ error: err?.message || String(err) }, null, 2);
    notify(t("engine.demo.toast.failed", { tool: t(tool.name), error: err?.message || err }));
  } finally {
    isRunning.value = false;
  }
}
</script>

<template>
  <div class="space-y-3 pt-4">
    <div class="flex items-center justify-between pb-2 border-b border-line">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <AlertTriangle v-if="hasDemoTools" class="w-4 h-4 text-warning" />
          <CheckCircle2 v-else class="w-4 h-4 text-success" />
          <span>{{ t("engine.demo.title") }}</span>
          <span
            v-if="hasDemoTools"
            class="px-1.5 py-0.2 rounded bg-warning/15 text-warning text-[9px] font-bold"
          >{{ t("engine.demo.badge.demo") }}</span>
          <span v-else class="px-1.5 py-0.2 rounded bg-success/15 text-success text-[9px] font-bold">{{
            t("engine.demo.badge.all.real")
          }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          <template v-if="hasDemoTools">{{ t("engine.demo.intro.mixed") }}</template>
          <template v-else>{{ t("engine.demo.intro.real") }}</template>
        </p>
      </div>
      <button
        @click="expanded = !expanded"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface hover:bg-raised border border-line text-xs font-semibold text-ink transition cursor-pointer shrink-0"
      >
        <component :is="expanded ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
        <span>{{ expanded ? t("engine.demo.collapse") : t("engine.demo.expand") }}</span>
      </button>
    </div>

    <div
      v-if="expanded"
      class="rounded-xl border p-3 space-y-3"
      :class="hasDemoTools ? 'border-warning/25 bg-warning/5' : 'border-line bg-surface/40'"
    >
      <div class="grid grid-cols-1 md:grid-cols-3 gap-2">
        <select
          v-model="selectedId"
          class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent cursor-pointer"
        >
          <option v-for="tool in tools" :key="tool.id" :value="tool.id">{{ t(tool.name) }}</option>
        </select>
        <div class="md:col-span-2 flex items-center gap-2 text-[11px] text-ink-muted self-center">
          <span
            class="px-1.5 py-0.2 rounded text-[9px] font-bold shrink-0"
            :class="selectedTool?.real ? 'bg-success/15 text-success' : 'bg-warning/15 text-warning'"
          >
            {{ selectedTool?.real ? t("engine.demo.badge.real") : t("engine.demo.badge.demo.only") }}
          </span>
          <span>{{ selectedTool ? t(selectedTool.description) : "" }}</span>
        </div>
      </div>

      <div v-if="selectedTool && selectedTool.fields.length > 0" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
        <label v-for="field in selectedTool.fields" :key="field.key" class="flex flex-col gap-1">
          <span class="text-[10px] text-ink-muted">{{ t(field.label) }}</span>
          <input
            :value="fieldValue(field)"
            @input="setFieldValue(field, ($event.target as HTMLInputElement).value)"
            type="text"
            :placeholder="field.placeholder"
            class="px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono placeholder-ink-muted focus:outline-none focus:border-accent"
          />
        </label>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="handleRun"
          :disabled="isRunning || !currentRepoPath"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-warning-strong hover:opacity-90 text-warning-contrast text-xs font-semibold transition cursor-pointer disabled:opacity-40"
        >
          <Play class="w-3.5 h-3.5" aria-hidden="true" />
          <span>{{
            isRunning
              ? t("engine.demo.running")
              : selectedTool?.real
                ? t("engine.demo.run.real")
                : t("engine.demo.run.demo")
          }}</span>
        </button>
        <span class="text-[10px] text-ink-muted">{{ t("engine.demo.result.hint") }}</span>
      </div>

      <div v-if="resultJson" class="space-y-1">
        <div class="text-[10px] text-ink-muted">{{ t("engine.demo.result.title", { tool: lastToolName }) }}</div>
        <pre class="max-h-80 overflow-auto p-3 rounded-lg bg-canvas border border-line text-[10px] leading-relaxed font-mono whitespace-pre-wrap break-all">{{ resultJson }}</pre>
      </div>
    </div>
  </div>
</template>
