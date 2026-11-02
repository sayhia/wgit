<script setup lang="ts">
import { Sparkles, HardDrive, GitBranch, FolderGit2 } from "lucide-vue-next";
import { t } from "../../i18n";

/**
 * The capability list shown on the right while the welcome page is idle.
 *
 * Pure presentation: no props, no state, no emits. It used to be inline in
 * RepoOnboarding, which carried four modes (welcome / clone / init / scan) plus
 * the recent list and the scanner in one 605-line file.
 */
</script>

<template>
<div class="flex items-center gap-2 mb-4">
  <Sparkles class="w-4 h-4 text-accent" />
  <h2 class="text-sm font-bold text-ink">{{ t("onboard.features.title") }}</h2>
</div>
<ul class="flex flex-col gap-2.5 text-[11px] text-ink-secondary">
  <li class="flex items-start gap-2">
    <HardDrive class="w-3.5 h-3.5 text-cyan shrink-0 mt-0.5" />
    <span><strong class="text-ink">{{ t("onboard.feature.clone.title") }}</strong>{{ t("onboard.feature.clone.desc") }}</span>
  </li>
  <li class="flex items-start gap-2">
    <GitBranch class="w-3.5 h-3.5 text-accent shrink-0 mt-0.5" />
    <span><strong class="text-ink">{{ t("onboard.feature.branch.title") }}</strong>{{ t("onboard.feature.branch.desc") }}</span>
  </li>
  <li class="flex items-start gap-2">
    <Sparkles class="w-3.5 h-3.5 text-purple shrink-0 mt-0.5" />
    <span><strong class="text-ink">⌘P / ⌘⇧F</strong>{{ t("onboard.feature.search.desc") }}</span>
  </li>
  <li class="flex items-start gap-2">
    <FolderGit2 class="w-3.5 h-3.5 text-success shrink-0 mt-0.5" />
    <span><strong class="text-ink">{{ t("onboard.feature.tools.title") }}</strong>{{ t("onboard.feature.tools.desc") }}</span>
  </li>
</ul>
<div class="mt-auto pt-4 text-[10px] text-ink-muted leading-relaxed">
  {{ t("onboard.tip") }}
</div>
          
</template>
