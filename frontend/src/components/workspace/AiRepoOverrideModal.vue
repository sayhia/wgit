<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { AlertTriangle, Save, Trash2 } from "lucide-vue-next";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import {
  deleteAIRepoOverride,
  getAIConfig,
  getAIRepoOverride,
  saveAIRepoOverride,
  type AIRepoOverride,
} from "../../services/aiService";

const props = defineProps<{ isOpen: boolean; repoPath?: string }>();
const emit = defineEmits<{ (e: "close"): void; (e: "saved"): void }>();

const TASK_KINDS = ["commit", "summary", "conflict", "explain", "chat", "review", "release"] as const;

const form = reactive({
  model: "",
  language: "" as "" | "auto" | "zh" | "en",
  includeBody: "inherit" as "inherit" | "on" | "off",
  instructions: "",
  taskModels: {} as Record<string, string>,
});
const globalModel = ref("");
const isLoading = ref(false);
const isSaving = ref(false);
const errorMessage = ref("");
const configured = computed(
  () =>
    form.model.trim() !== "" ||
    form.language !== "" ||
    form.includeBody !== "inherit" ||
    form.instructions.trim() !== "" ||
    Object.values(form.taskModels).some((value) => value.trim() !== "")
);

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

async function load(): Promise<void> {
  if (!props.repoPath) return;
  isLoading.value = true;
  errorMessage.value = "";
  try {
    const [override, config] = await Promise.all([getAIRepoOverride(props.repoPath), getAIConfig()]);
    form.model = override.model || "";
    form.language = (override.commitLanguage as "" | "auto" | "zh" | "en") || "";
    form.includeBody =
      override.commitIncludeBody === true ? "on" : override.commitIncludeBody === false ? "off" : "inherit";
    form.instructions = override.commitInstructions || "";
    form.taskModels = { ...(override.taskModels || {}) };
    globalModel.value = config?.model || "—";
  } catch (err) {
    errorMessage.value = errorText(err);
  } finally {
    isLoading.value = false;
  }
}

function payload(): AIRepoOverride {
  const taskModels: Record<string, string> = {};
  for (const kind of TASK_KINDS) {
    const value = (form.taskModels[kind] || "").trim();
    if (value) taskModels[kind] = value;
  }
  return {
    model: form.model.trim(),
    commitLanguage: form.language,
    commitIncludeBody: form.includeBody === "inherit" ? null : form.includeBody === "on",
    commitInstructions: form.instructions.trim(),
    taskModels,
  };
}

async function save(): Promise<void> {
  if (!props.repoPath || isSaving.value) return;
  isSaving.value = true;
  try {
    await saveAIRepoOverride(props.repoPath, payload());
    pushToast(t("ai.repo.saved"), { tone: "success" });
    emit("saved");
    emit("close");
  } catch (err) {
    pushToast(t("ai.repo.error", { error: errorText(err) }), { tone: "danger" });
  } finally {
    isSaving.value = false;
  }
}

async function restoreGlobal(): Promise<void> {
  if (!props.repoPath || isSaving.value) return;
  isSaving.value = true;
  try {
    await deleteAIRepoOverride(props.repoPath);
    pushToast(t("ai.repo.removed"), { tone: "success" });
    emit("saved");
    emit("close");
  } catch (err) {
    pushToast(t("ai.repo.error", { error: errorText(err) }), { tone: "danger" });
  } finally {
    isSaving.value = false;
  }
}

watch(
  () => [props.isOpen, props.repoPath] as const,
  ([open]) => {
    if (open) void load();
  }
);
</script>

<template>
  <UiModal :open="isOpen" :title="t('ai.repo.title')" width="max-w-xl" @update:open="emit('close')">
    <div class="space-y-4" data-testid="ai-repo-modal">
      <div class="flex items-start gap-2 p-2.5 rounded-xl border border-line bg-canvas">
        <div class="min-w-0">
          <p class="text-[11px] font-semibold text-ink">{{ t("ai.repo.desc") }}</p>
          <p class="text-[10px] text-ink-muted font-mono truncate mt-0.5">{{ repoPath }}</p>
        </div>
        <span
          class="ml-auto px-1.5 py-0.5 rounded text-[9px] font-bold border shrink-0"
          :class="configured ? 'bg-accent-bg text-accent border-accent/30' : 'bg-canvas text-ink-muted border-line'"
        >
          {{ configured ? t("settings.localai.status.configured") : t("settings.localai.status.unconfigured") }}
        </span>
      </div>

      <p v-if="errorMessage" class="text-xs text-danger flex items-center gap-1">
        <AlertTriangle class="w-3.5 h-3.5" />{{ errorMessage }}
      </p>
      <p v-if="isLoading" class="text-xs text-ink-muted">{{ t("settings.ai.control.loading") }}</p>

      <div class="space-y-3">
        <label class="space-y-1 block">
          <span class="text-[10px] font-semibold text-ink-muted">{{ t("ai.repo.model") }}</span>
          <input
            v-model="form.model"
            type="text"
            data-testid="ai-repo-model"
            :disabled="isLoading"
            :placeholder="t('ai.repo.model.inherit', { model: globalModel })"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent disabled:opacity-50"
          />
        </label>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <label class="space-y-1">
            <span class="text-[10px] font-semibold text-ink-muted">{{ t("ai.repo.language") }}</span>
            <select v-model="form.language" data-testid="ai-repo-language" class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent">
              <option value="">{{ t("settings.ai.repos.inherit") }}</option>
              <option value="auto">{{ t("settings.localai.commit.auto") }}</option>
              <option value="zh">{{ t("settings.localai.commit.zh") }}</option>
              <option value="en">{{ t("settings.localai.commit.en") }}</option>
            </select>
          </label>
          <label class="space-y-1">
            <span class="text-[10px] font-semibold text-ink-muted">{{ t("ai.repo.body") }}</span>
            <select v-model="form.includeBody" data-testid="ai-repo-body" class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent">
              <option value="inherit">{{ t("settings.ai.repos.body.inherit") }}</option>
              <option value="on">{{ t("settings.ai.repos.body.on") }}</option>
              <option value="off">{{ t("settings.ai.repos.body.off") }}</option>
            </select>
          </label>
        </div>

        <label class="space-y-1 block">
          <span class="text-[10px] font-semibold text-ink-muted">{{ t("ai.repo.instructions") }}</span>
          <textarea
            v-model="form.instructions"
            rows="3"
            maxlength="2000"
            data-testid="ai-repo-instructions"
            class="w-full px-3 py-2 rounded-xl bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent resize-y"
          ></textarea>
        </label>

        <details class="rounded-xl border border-line bg-canvas" data-testid="ai-repo-tasks">
          <summary class="px-3 py-2 text-[11px] font-semibold text-ink-muted cursor-pointer select-none">
            {{ t("ai.repo.tasks") }}
          </summary>
          <div class="p-3 pt-0 grid grid-cols-1 sm:grid-cols-2 gap-2">
            <label v-for="kind in TASK_KINDS" :key="kind" class="space-y-1">
              <span class="text-[10px] font-semibold text-ink-muted">{{ t(`settings.ai.tasks.${kind}`) }}</span>
              <input
                v-model="form.taskModels[kind]"
                type="text"
                :data-testid="`ai-repo-task-${kind}`"
                :placeholder="t('settings.ai.tasks.inherit')"
                class="w-full px-3 py-2 rounded-xl bg-surface border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              />
            </label>
          </div>
        </details>
      </div>
    </div>

    <template #footer>
      <UiButton variant="ghost" :disabled="isSaving" data-testid="ai-repo-remove" @click="restoreGlobal">
        <Trash2 class="w-3.5 h-3.5" />
        {{ t("ai.repo.remove") }}
      </UiButton>
      <UiButton variant="primary" :disabled="isSaving || isLoading || !repoPath" data-testid="ai-repo-save" @click="save">
        <Save class="w-3.5 h-3.5" />
        {{ t("ai.repo.save") }}
      </UiButton>
    </template>
  </UiModal>
</template>
