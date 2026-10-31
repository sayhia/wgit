<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Copy, Check, ShieldCheck } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { AppVersionInfo } from "../../types";
import { t } from "../../i18n";
import SettingsPageHeader from "./SettingsPageHeader.vue";
import SettingsCard from "./SettingsCard.vue";
import UiButton from "../ui/UiButton.vue";

const emit = defineEmits<{
  (e: "notify", msg: string): void;
  (e: "open-crash-report"): void;
}>();

const appInfo = ref<AppVersionInfo>({
  version: "1.0.0",
  commitHash: "dev",
  buildDate: "unspecified",
  platform: "darwin",
  arch: "arm64",
  goVersion: "go1.27",
});

const isCopied = ref(false);

async function loadVersion() {
  try {
    const info = await gitService.getAppVersion();
    if (info) {
      appInfo.value = info;
    }
  } catch {
    // best-effort: fallback to default static values
  }
}

async function copySystemInfo() {
  const infoText = [
    `wgit Version: ${appInfo.value.version}`,
    `Commit: ${appInfo.value.commitHash}`,
    `Build Date: ${appInfo.value.buildDate}`,
    `Platform: ${appInfo.value.platform} (${appInfo.value.arch})`,
    `Go: ${appInfo.value.goVersion}`,
    `User Agent: ${navigator.userAgent}`,
  ].join("\n");

  try {
    await navigator.clipboard.writeText(infoText);
    isCopied.value = true;
    emit("notify", t("settings.about.copied"));
    setTimeout(() => {
      isCopied.value = false;
    }, 2000);
  } catch {
    emit("notify", "Failed to copy info to clipboard");
  }
}

onMounted(() => {
  void loadVersion();
});
</script>

<template>
  <div class="space-y-4 min-w-0" data-section="about">
    <SettingsPageHeader :title="t('settings.about.title')" :description="t('settings.about.desc')" />
    <SettingsCard class="@container">
      <template #header>
        <div class="flex items-center gap-3 min-w-0">
          <img src="/brand/wgit-icon.svg" alt="" width="36" height="36" class="w-9 h-9 shrink-0" draggable="false" />
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-ink">wgit</h3>
            <p class="text-[11px] font-mono text-ink-muted break-all">v{{ appInfo.version }} · {{ appInfo.commitHash }}</p>
          </div>
        </div>
      </template>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="copySystemInfo" class="flex items-center gap-1.5 text-xs">
          <component :is="isCopied ? Check : Copy" class="w-3.5 h-3.5 shrink-0" :class="isCopied ? 'text-success' : ''" aria-hidden="true" />
          <span>{{ t("settings.about.copy") }}</span>
        </UiButton>
      </template>
      <dl class="grid grid-cols-2 @min-[38rem]:grid-cols-3 gap-x-5 gap-y-4 text-xs">
        <div class="min-w-0">
          <dt class="text-[11px] text-ink-muted">{{ t("settings.about.version") }}</dt>
          <dd class="font-mono text-ink mt-1 break-all">{{ appInfo.version }}</dd>
        </div>
        <div class="min-w-0">
          <dt class="text-[11px] text-ink-muted">{{ t("settings.about.commit") }}</dt>
          <dd class="font-mono text-ink mt-1 break-all">{{ appInfo.commitHash }}</dd>
        </div>
        <div class="min-w-0">
          <dt class="text-[11px] text-ink-muted">{{ t("settings.about.platform") }}</dt>
          <dd class="font-mono text-ink mt-1 break-words">{{ appInfo.platform }} / {{ appInfo.arch }}</dd>
        </div>
        <div class="min-w-0">
          <dt class="text-[11px] text-ink-muted">{{ t("settings.about.build.date") }}</dt>
          <dd class="font-mono text-ink mt-1 break-words">{{ appInfo.buildDate }}</dd>
        </div>
        <div class="min-w-0">
          <dt class="text-[11px] text-ink-muted">{{ t("settings.about.go.version") }}</dt>
          <dd class="font-mono text-ink mt-1 break-all">{{ appInfo.goVersion }}</dd>
        </div>
        <div class="min-w-0">
          <dt class="text-[11px] text-ink-muted">{{ t("settings.about.protocol") }}</dt>
          <dd class="font-mono text-success mt-1 flex flex-wrap items-center gap-1">
            <ShieldCheck class="w-3.5 h-3.5 shrink-0" aria-hidden="true" /><span>wgit:// OK</span>
          </dd>
        </div>
      </dl>
    </SettingsCard>
    <SettingsCard>
      <template #header><h3 class="text-xs font-semibold text-ink">{{ t("settings.about.crashes") }}</h3></template>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="emit('open-crash-report')" class="text-xs">{{ t("settings.about.crashes.view") }}</UiButton>
      </template>
      <p class="text-xs text-ink-secondary leading-relaxed">{{ t("settings.about.crashes.desc") }}</p>
    </SettingsCard>
  </div>
</template>
