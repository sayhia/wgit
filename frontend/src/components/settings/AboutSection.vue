<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Info, Copy, Check, Terminal, ExternalLink, ShieldCheck, Flame } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { AppVersionInfo } from "../../types";
import { t } from "../../i18n";
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
  <div class="space-y-6" data-section="about">
    <div>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Info class="w-4 h-4 text-accent" />
        <span>{{ t("settings.about.title") }}</span>
      </h3>
      <p class="text-xs text-ink-muted mt-1 leading-relaxed">
        {{ t("settings.about.desc") }}
      </p>
    </div>

    <!-- Metadata Card -->
    <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-3" data-glass>
      <div class="flex items-center justify-between pb-3 border-b border-line/60">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-xl bg-accent/10 border border-accent/20 flex items-center justify-center font-black text-accent text-lg shadow-sm">
            W
          </div>
          <div>
            <h4 class="text-sm font-bold text-ink">wgit</h4>
            <span class="text-[11px] font-mono text-ink-muted">v{{ appInfo.version }} · {{ appInfo.commitHash }}</span>
          </div>
        </div>
        <UiButton variant="secondary" size="sm" @click="copySystemInfo" class="flex items-center gap-1.5 text-xs">
          <component :is="isCopied ? Check : Copy" class="w-3.5 h-3.5" :class="isCopied ? 'text-success' : ''" />
          <span>{{ t("settings.about.copy") }}</span>
        </UiButton>
      </div>

      <div class="grid grid-cols-2 sm:grid-cols-3 gap-3 text-xs">
        <div class="p-2.5 rounded-lg bg-raised/50 border border-line/40">
          <span class="text-[10px] uppercase font-semibold text-ink-muted block">{{ t("settings.about.version") }}</span>
          <span class="font-mono text-ink font-medium mt-0.5 block truncate">{{ appInfo.version }}</span>
        </div>
        <div class="p-2.5 rounded-lg bg-raised/50 border border-line/40">
          <span class="text-[10px] uppercase font-semibold text-ink-muted block">{{ t("settings.about.commit") }}</span>
          <span class="font-mono text-ink font-medium mt-0.5 block truncate">{{ appInfo.commitHash }}</span>
        </div>
        <div class="p-2.5 rounded-lg bg-raised/50 border border-line/40">
          <span class="text-[10px] uppercase font-semibold text-ink-muted block">{{ t("settings.about.platform") }}</span>
          <span class="font-mono text-ink font-medium mt-0.5 block truncate">{{ appInfo.platform }} / {{ appInfo.arch }}</span>
        </div>
        <div class="p-2.5 rounded-lg bg-raised/50 border border-line/40">
          <span class="text-[10px] uppercase font-semibold text-ink-muted block">{{ t("settings.about.build.date") }}</span>
          <span class="font-mono text-ink font-medium mt-0.5 block truncate">{{ appInfo.buildDate }}</span>
        </div>
        <div class="p-2.5 rounded-lg bg-raised/50 border border-line/40">
          <span class="text-[10px] uppercase font-semibold text-ink-muted block">{{ t("settings.about.go.version") }}</span>
          <span class="font-mono text-ink font-medium mt-0.5 block truncate">{{ appInfo.goVersion }}</span>
        </div>
        <div class="p-2.5 rounded-lg bg-raised/50 border border-line/40">
          <span class="text-[10px] uppercase font-semibold text-ink-muted block">{{ t("settings.about.protocol") }}</span>
          <span class="font-mono text-success font-medium mt-0.5 flex items-center gap-1">
            <ShieldCheck class="w-3.5 h-3.5 shrink-0" />
            <span>wgit:// OK</span>
          </span>
        </div>
      </div>
    </div>

    <!-- Forensic / Diagnostics Card -->
    <div class="rounded-xl border border-line bg-surface/30 p-4 space-y-2">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <Flame class="w-4 h-4 text-warning" />
          <h4 class="text-xs font-semibold text-ink">{{ t("settings.about.crashes") }}</h4>
        </div>
        <UiButton variant="secondary" size="sm" @click="emit('open-crash-report')" class="text-xs">
          {{ t("settings.about.crashes.view") }}
        </UiButton>
      </div>
      <p class="text-[11px] text-ink-muted leading-relaxed">
        {{ t("settings.about.crashes.desc") }}
      </p>
    </div>
  </div>
</template>
