<script setup lang="ts">
import { ref, watch } from "vue";
import { RefreshCw } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import SettingsPageHeader from "./SettingsPageHeader.vue";
import SettingsCard from "./SettingsCard.vue";

const props = defineProps<{ currentRepoPath: string }>();
const emit = defineEmits<{ (e: "notify", msg: string): void }>();

function notify(msg: string) {
  emit("notify", msg);
}

const userName = ref("");
const userEmail = ref("");
const isGlobalIdentity = ref(false);
const isSavingIdentity = ref(false);

async function loadIdentity() {
  if (!props.currentRepoPath) return;
  try {
    const cfg = await gitService.getGitConfig(props.currentRepoPath);
    if (!cfg) return;
    userName.value = cfg.userName || "";
    userEmail.value = cfg.userEmail || "";
    isGlobalIdentity.value = cfg.isGlobal;
  } catch (err) {
    // best-effort: the section renders its empty state.
    console.debug("Failed to load git identity:", err);
  }
}

async function saveIdentity() {
  if (!props.currentRepoPath) return;
  isSavingIdentity.value = true;
  try {
    await gitService.setGitConfig(
      props.currentRepoPath,
      userName.value.trim(),
      userEmail.value.trim(),
      isGlobalIdentity.value
    );
    notify(
      t(
        isGlobalIdentity.value
          ? "settings.identity.saved.global"
          : "settings.identity.saved.local"
      )
    );
  } catch (err: any) {
    notify(t("settings.identity.save.failed", { error: err?.message || err }));
  } finally {
    isSavingIdentity.value = false;
  }
}

watch(() => props.currentRepoPath, loadIdentity, { immediate: true });
</script>

<template>
  <section class="@container space-y-4">
    <SettingsPageHeader :title="t('settings.identity.title')" :description="t('settings.identity.intro')" />

    <SettingsCard>
      <form class="space-y-4" novalidate @submit.prevent="saveIdentity">
        <div class="grid grid-cols-1 @min-[520px]:grid-cols-2 gap-4">
          <label class="flex min-w-0 flex-col gap-1.5">
            <span class="text-xs font-medium text-ink">{{ t("settings.identity.name") }}</span>
            <input
              v-model="userName"
              type="text"
              :aria-label="t('settings.identity.name')"
              placeholder="sunmking"
              class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
            />
          </label>
          <label class="flex min-w-0 flex-col gap-1.5">
            <span class="text-xs font-medium text-ink">{{ t("settings.identity.email") }}</span>
            <input
              v-model="userEmail"
              type="email"
              :aria-label="t('settings.identity.email')"
              placeholder="user@example.com"
              class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
            />
          </label>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-line/60 pt-4">
          <label class="flex items-center gap-2 text-xs text-ink-secondary cursor-pointer">
            <input v-model="isGlobalIdentity" type="checkbox" class="shrink-0 rounded border-line bg-canvas text-accent focus:ring-0" />
            <span>{{ t("settings.identity.global") }}</span>
          </label>
          <button
            type="submit"
            class="ml-auto flex shrink-0 items-center gap-1.5 px-4 py-2 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer hover:bg-accent-strong-hover disabled:opacity-50 disabled:cursor-not-allowed"
            :disabled="isSavingIdentity"
          >
            <RefreshCw v-if="isSavingIdentity" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            {{ isSavingIdentity ? t("settings.identity.saving") : t("settings.identity.save") }}
          </button>
        </div>
      </form>
    </SettingsCard>
  </section>
</template>
