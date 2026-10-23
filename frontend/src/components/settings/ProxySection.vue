<script setup lang="ts">
import { ref, watch } from "vue";
import { RefreshCw } from "lucide-vue-next";
import { gitService } from "../../services/gitService";

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
      notify("Git 网络代理已生效");
    } else {
      await gitService.clearGitProxy(props.currentRepoPath, isGlobalProxy.value);
      notify("Git 网络代理已清除");
    }
  } catch (err: any) {
    notify("代理设置失败: " + (err?.message || err));
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
          <h2 class="text-sm font-bold text-ink">Git 网络与加速代理</h2>
          <p class="text-xs text-ink-secondary mt-1">配置克隆、拉取、推送及 LFS 大文件传输代理。</p>
        </div>

        <div class="p-5 rounded-2xl bg-surface border border-line space-y-4">
          <div class="flex flex-col gap-1.5">
            <label class="text-xs text-ink font-medium">代理服务器 URL (http.proxy / https.proxy)</label>
            <input
              v-model="proxyUrl"
              type="text"
              placeholder="例如: http://127.0.0.1:7890 或 socks5://127.0.0.1:1080 (留空为清除)"
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
              <span>应用于全局 Git 配置 (--global)</span>
            </label>

            <button
              @click="saveProxy"
              class="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-accent-strong text-white font-semibold text-xs cursor-pointer shadow-sm hover:bg-accent-strong-hover"
              :disabled="isSavingProxy"
            >
              <RefreshCw v-if="isSavingProxy" class="w-3.5 h-3.5 animate-spin" />
              <span>保存代理设置</span>
            </button>
          </div>
        </div>
      </section>

</template>
