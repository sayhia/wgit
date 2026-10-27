<script setup lang="ts">
import { ref, watch } from "vue";
import { User, RefreshCw } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";

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
      <!-- 1. Identity -->
      <section class="space-y-6">
        <div>
          <h2 class="text-sm font-bold text-ink">{{ t("settings.identity.title") }}</h2>
          <p class="text-xs text-ink-secondary mt-1">{{ t("settings.identity.intro") }}</p>
        </div>

        <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
          <div class="flex flex-col gap-1.5">
            <label class="text-xs text-ink font-medium">{{ t("settings.identity.name") }}</label>
            <input
              v-model="userName"
              type="text"
              placeholder="sunmking"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
            />
          </div>

          <div class="flex flex-col gap-1.5">
            <label class="text-xs text-ink font-medium">{{ t("settings.identity.email") }}</label>
            <input
              v-model="userEmail"
              type="email"
              placeholder="user@example.com"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
            />
          </div>

          <div class="flex items-center justify-between pt-2 border-t border-line">
            <label class="flex items-center gap-2 text-xs text-ink-secondary cursor-pointer">
              <input
                v-model="isGlobalIdentity"
                type="checkbox"
                class="rounded border-line bg-canvas text-accent focus:ring-0"
              />
              <span>{{ t("settings.identity.global") }}</span>
            </label>

            <button
              @click="saveIdentity"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer shadow-sm hover:bg-accent-strong-hover"
              :disabled="isSavingIdentity"
            >
              <RefreshCw v-if="isSavingIdentity" class="w-3.5 h-3.5 animate-spin" />
              <span>{{
                isSavingIdentity ? t("settings.identity.saving") : t("settings.identity.save")
              }}</span>
            </button>
          </div>
        </div>
      </section>

</template>
