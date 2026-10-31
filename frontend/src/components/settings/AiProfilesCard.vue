<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { Check, CheckCircle2, Layers, Pencil, Trash2, X } from "lucide-vue-next";
import SettingsCard from "./SettingsCard.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";
import { pushToast } from "../../composables/useToast";
import { confirmAction } from "../../composables/useConfirm";
import {
  activateAIProfile,
  deleteAIProfile,
  getAIAdvancedConfig,
  listAIProfiles,
  saveAIAdvancedConfig,
  type AIProfile,
} from "../../services/aiService";

const props = withDefaults(defineProps<{ refreshKey?: number }>(), { refreshKey: 0 });
const emit = defineEmits<{ (e: "changed"): void }>();

const profiles = ref<AIProfile[]>([]);
const isLoading = ref(false);
const loadError = ref("");
const renamingProfileId = ref("");
const renameDraft = ref("");
const isRenamingProfile = ref(false);

const errorText = (err: unknown): string => (err instanceof Error ? err.message : String(err));

async function loadProfiles(): Promise<void> {
  isLoading.value = true;
  loadError.value = "";
  try {
    profiles.value = (await listAIProfiles()) || [];
  } catch (err) {
    loadError.value = errorText(err);
  } finally {
    isLoading.value = false;
  }
}

async function switchProfile(profile: AIProfile): Promise<void> {
  if (profile.active) return;
  try {
    await activateAIProfile(profile.id);
    await loadProfiles();
    pushToast(t("ai.quick.saved"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(t("ai.quick.profile.switchfailed", { error: errorText(err) }), { tone: "danger" });
  }
}

function startRenameProfile(profile: AIProfile): void {
  renamingProfileId.value = profile.id;
  renameDraft.value = profile.name;
}

function cancelRenameProfile(): void {
  renamingProfileId.value = "";
  renameDraft.value = "";
}

async function saveProfileName(profile: AIProfile): Promise<void> {
  if (isRenamingProfile.value) return;
  const name = renameDraft.value.trim();
  if (name.length > 80) {
    pushToast(t("ai.profile.rename.toolong"), { tone: "warning" });
    return;
  }
  isRenamingProfile.value = true;
  try {
    const current = await getAIAdvancedConfig();
    const labels = { ...(current.profileNames || {}) };
    if (name) labels[profile.id] = name;
    else delete labels[profile.id];
    await saveAIAdvancedConfig({ ...current, profileNames: labels });
    await loadProfiles();
    cancelRenameProfile();
    pushToast(t("ai.profile.renamed"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(t("settings.ai.control.failed", { error: errorText(err) }), { tone: "danger" });
  } finally {
    isRenamingProfile.value = false;
  }
}

async function removeProfile(profile: AIProfile): Promise<void> {
  if (!(await confirmAction({ message: `${t("ai.profile.delete")} ${profile.name}?`, tone: "danger" }))) return;
  try {
    await deleteAIProfile(profile.id);
    await loadProfiles();
    pushToast(t("ai.profile.deleted"), { tone: "success" });
    emit("changed");
  } catch (err) {
    pushToast(errorText(err), { tone: "danger" });
  }
}

watch(
  () => props.refreshKey,
  (next, previous) => {
    if (next !== previous) void loadProfiles();
  }
);

onMounted(loadProfiles);
</script>

<template>
  <SettingsCard
  id="ai-section-profiles"
    :aria-busy="isLoading"
  data-testid="ai-profiles-card"
  >
    <template #header>
      <div class="flex items-center gap-2">
        <Layers class="w-4 h-4 text-accent shrink-0" />
        <div>
          <h3 class="ai-card-title">{{ t("ai.profile.title") }}</h3>
          <p class="ai-card-desc">{{ t("ai.profile.desc") }}</p>
        </div>
      </div>
    </template>

    <div class="space-y-3">
      <div
  v-if="loadError"
  role="alert"
  class="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border border-danger/30 bg-danger-bg text-[11px] text-danger"
      >
        <span class="min-w-0 flex-1 break-words">
          {{ t("settings.ai.control.loadfailed", { error: loadError }) }}
        </span>
        <UiButton variant="secondary" data-testid="ai-profiles-retry" @click="loadProfiles">
          {{ t("ai.quick.load.retry") }}
        </UiButton>
      </div>

      <p v-if="isLoading && !profiles.length" class="text-xs text-ink-muted" role="status">
        {{ t("settings.ai.control.loading") }}
      </p>
      <p v-else-if="!profiles.length && !loadError" class="text-xs text-ink-muted">
        {{ t("ai.profile.empty") }}
      </p>
      <div v-else class="space-y-1.5">
        <div
  v-for="profile in profiles"
          :key="profile.id"
  class="flex flex-wrap items-center gap-2 p-2 rounded-xl bg-canvas border border-line"
        >
          <div v-if="renamingProfileId === profile.id" class="flex flex-wrap items-center gap-1.5 min-w-0 flex-1">
            <input
              v-model="renameDraft"
              type="text"
              maxlength="80"
              data-testid="ai-profile-rename-input"
              :aria-label="t('ai.profile.rename')"
              :placeholder="t('ai.profile.rename.placeholder')"
              class="min-w-0 flex-1 px-2 py-1.5 rounded-lg bg-surface border border-accent/40 text-[11px] text-ink placeholder-ink-muted focus:outline-none focus:border-accent"
              @keydown.enter.prevent="saveProfileName(profile)"
              @keydown.esc="cancelRenameProfile"
            />
            <button
              type="button"
              class="p-1.5 rounded-lg text-success hover:bg-success-bg disabled:opacity-40"
              :disabled="isRenamingProfile"
              :aria-label="t('common.save')"
              @click="saveProfileName(profile)"
            >
              <Check class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
            <button
  type="button"
  class="p-1.5 rounded-lg text-ink-muted hover:text-ink hover:bg-raised"
              :aria-label="t('common.cancel')"
  @click="cancelRenameProfile"
            >
              <X class="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          </div>
          <button
  v-else
  type="button"
            :data-profile-id="profile.id"
            :aria-pressed="profile.active"
  class="min-w-0 flex-1 text-left cursor-pointer"
  @click="switchProfile(profile)"
          >
            <div class="text-[11px] font-semibold text-ink truncate">{{ profile.name }}</div>
            <div class="text-[10px] text-ink-muted truncate">
              {{ profile.mode === "cli" ? profile.cliCommand : profile.baseUrl }} · {{ profile.model || "—" }}
            </div>
          </button>
          <span v-if="profile.active" class="px-1.5 py-0.5 rounded bg-accent-bg text-accent text-[9px] font-bold flex items-center gap-1">
            <CheckCircle2 class="w-3 h-3" />
            {{ t("ai.profile.active") }}
          </span>
          <button
            v-if="renamingProfileId !== profile.id"
            type="button"
            class="p-1.5 rounded-lg text-ink-muted hover:text-accent hover:bg-accent-bg"
            :title="t('ai.profile.rename')"
            :aria-label="t('ai.profile.rename')"
            data-testid="ai-profile-rename"
            @click="startRenameProfile(profile)"
          >
            <Pencil class="w-3.5 h-3.5" aria-hidden="true" />
          </button>
          <button
            type="button"
            class="p-1.5 rounded-lg text-ink-muted hover:text-danger hover:bg-danger-bg"
            :title="t('ai.profile.delete')"
            :aria-label="t('ai.profile.delete')"
            @click="removeProfile(profile)"
          >
            <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>
  </SettingsCard>
</template>
