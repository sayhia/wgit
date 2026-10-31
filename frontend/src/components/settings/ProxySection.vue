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

const proxyUrl = ref("");
const isGlobalProxy = ref(false);
const isSavingProxy = ref(false);

async function loadProxy() {
  if (!props.currentRepoPath) return;
  try {
    const proxy = await gitService.getGitProxy(props.currentRepoPath);
    if (!proxy) return;
    proxyUrl.value = proxy.httpProxy || proxy.httpsProxy || "";
    isGlobalProxy.value = proxy.isGlobal;
  } catch (err) {
    // best-effort: the section renders its empty state.
    console.debug("Failed to load git proxy:", err);
  }
}

async function saveProxy() {
  if (!props.currentRepoPath) return;
  isSavingProxy.value = true;
  try {
    const target = proxyUrl.value.trim();
    if (target) {
      await gitService.setGitProxy(props.currentRepoPath, target, isGlobalProxy.value);
      notify(t("settings.proxy.saved"));
    } else {
      await gitService.clearGitProxy(props.currentRepoPath, isGlobalProxy.value);
      notify(t("settings.proxy.cleared"));
    }
  } catch (err: any) {
    notify(t("settings.proxy.failed", { error: err?.message || err }));
  } finally {
    isSavingProxy.value = false;
  }
}

watch(() => props.currentRepoPath, loadProxy, { immediate: true });
</script>

<template>
  <section class="space-y-4">
    <SettingsPageHeader :title="t('settings.proxy.title')" :description="t('settings.proxy.intro')" />

    <SettingsCard>
      <form class="space-y-4" @submit.prevent="saveProxy">
        <label class="flex min-w-0 flex-col gap-1.5">
          <span class="text-xs font-medium text-ink">{{ t("settings.proxy.url") }}</span>
          <input
            v-model="proxyUrl"
            type="text"
            :aria-label="t('settings.proxy.url')"
            :placeholder="t('settings.proxy.url.placeholder')"
            spellcheck="false"
            class="w-full min-w-0 px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
          />
        </label>

        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-line/60 pt-4">
          <label class="flex items-center gap-2 text-xs text-ink-secondary cursor-pointer">
            <input v-model="isGlobalProxy" type="checkbox" class="shrink-0 rounded border-line bg-canvas text-accent focus:ring-0" />
            <span>{{ t("settings.proxy.global") }}</span>
          </label>
          <button
            type="submit"
            class="ml-auto flex shrink-0 items-center gap-1.5 px-4 py-2 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer hover:bg-accent-strong-hover disabled:opacity-50 disabled:cursor-not-allowed"
            :disabled="isSavingProxy"
          >
            <RefreshCw v-if="isSavingProxy" class="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
            {{ t("settings.proxy.save") }}
          </button>
        </div>
      </form>
    </SettingsCard>
  </section>
</template>
