<script setup lang="ts">
import { ref, watch } from "vue";
import { RefreshCw } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";

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
      <!-- 3. Proxy -->
      <section  class="space-y-6">
        <div>
          <h2 class="text-sm font-bold text-ink">{{ t("settings.proxy.title") }}</h2>
          <p class="text-xs text-ink-secondary mt-1">{{ t("settings.proxy.intro") }}</p>
        </div>

        <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
          <div class="flex flex-col gap-1.5">
            <label class="text-xs text-ink font-medium">{{ t("settings.proxy.url") }}</label>
            <input
              v-model="proxyUrl"
              type="text"
              :placeholder="t('settings.proxy.url.placeholder')"
              class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent font-mono"
            />
          </div>

          <div class="flex items-center justify-between pt-2 border-t border-line">
            <label class="flex items-center gap-2 text-xs text-ink-secondary cursor-pointer">
              <input
                v-model="isGlobalProxy"
                type="checkbox"
                class="rounded border-line bg-canvas text-accent focus:ring-0"
              />
              <span>{{ t("settings.proxy.global") }}</span>
            </label>

            <button
              @click="saveProxy"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-accent-contrast font-semibold text-xs cursor-pointer shadow-sm hover:bg-accent-strong-hover"
              :disabled="isSavingProxy"
            >
              <RefreshCw v-if="isSavingProxy" class="w-3.5 h-3.5 animate-spin" />
              <span>{{ t("settings.proxy.save") }}</span>
            </button>
          </div>
        </div>
      </section>

</template>
