<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, watch, onMounted, computed } from "vue";
import { Wrench, RefreshCw, ChevronDown, ChevronRight, CheckCircle2, Undo2 } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import UiButton from "../ui/UiButton.vue";
import UiBadge from "../ui/UiBadge.vue";
import UiEmpty from "../ui/UiEmpty.vue";
import UiSkeleton from "../ui/UiSkeleton.vue";
import { statusTone, statusLabel } from "../../utils/uiVariants";
import { t } from "../../i18n";
import type { FixCenterItem, FixCenterResult, AuditFixResult } from "../../types";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
  (e: "notify", msg: string): void;
}>();

const result = ref<FixCenterResult | null>(null);
const isScanning = ref(false);
const runningId = ref("");
const fixResults = ref<Record<string, AuditFixResult>>({});
const expanded = ref<Record<string, boolean>>({});

const fixableItems = computed(() => (result.value?.items || []).filter((i) => i.fixable));

async function scan() {
  if (!props.currentRepoPath || isScanning.value) return;
  isScanning.value = true;
  try {
    result.value = await gitService.scanFixCenter(props.currentRepoPath);
  } catch (err: any) {
    emit("notify", `修复中心扫描失败: ${err?.message || err}`);
  } finally {
    isScanning.value = false;
  }
}

function toggleExpand(id: string) {
  expanded.value = { ...expanded.value, [id]: !expanded.value[id] };
}

async function applyFix(item: FixCenterItem) {
  if (!props.currentRepoPath || runningId.value) return;

  const targets = item.targets || [];
  const preview = targets.slice(0, 6).join("\n");
  const more = targets.length > 6 ? `\n… 等 ${targets.length} 个` : "";
  const confirmed = await confirmAction(
    `执行「${item.fixLabel}」？\n\n${item.title}：${item.summary}` +
      (targets.length ? `\n\n影响范围：\n${preview}${more}` : "")
  );
  if (!confirmed) return;

  runningId.value = item.id;
  try {
    const res = await gitService.applyAuditFix(props.currentRepoPath, item.id, targets);
    fixResults.value = { ...fixResults.value, [item.id]: res };
    emit("notify", res.message || "修复已执行");
    await scan();
    emit("refresh");
  } catch (err: any) {
    emit("notify", `修复失败: ${err?.message || err}`);
  } finally {
    runningId.value = "";
  }
}

onMounted(scan);
watch(
  () => [props.currentRepoPath, props.refreshNonce],
  () => {
    if (props.currentRepoPath) scan();
  }
);
</script>

<template>
  <div class="space-y-3" data-tool-section="fix-center">
    <div class="pb-2 border-b border-line flex items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <Wrench class="w-4 h-4 text-accent" />
          <span>{{ t("fix.title") }}</span>
        </h3>
        <p class="text-xs text-ink-muted mt-0.5">
          把各页的「只读体检」收敛成可执行动作；改写
          <span class="font-mono text-[10px]">.gitignore</span> /
          <span class="font-mono text-[10px]">.gitattributes</span> 前自动备份到
          <span class="font-mono text-[10px]">.git/wgit-fix-backup/</span>。
        </p>
      </div>
      <UiButton variant="secondary" :disabled="isScanning" class="shrink-0" @click="scan">
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isScanning }" />
        <span>{{ isScanning ? t("fix.scanning") : t("fix.scan") }}</span>
      </UiButton>
    </div>

    <p v-if="result" class="text-[11px] text-ink-muted">
      {{ result.message }}
      <span class="text-ink-muted/60">· {{ result.scannedAt }}</span>
    </p>

    <div
      v-if="result?.warnings?.length"
      class="rounded-lg border border-warning/25 bg-warning/10 px-2.5 py-2 space-y-0.5"
    >
      <p class="text-[10px] font-bold text-warning">以下检查未能完成，结果可能不完整</p>
      <p v-for="warning in result.warnings" :key="warning" class="text-[10px] text-warning/90 leading-relaxed">
        {{ warning }}
      </p>
    </div>

    <div v-if="fixableItems.length > 0" class="space-y-1.5">
      <div
        v-for="item in fixableItems"
        :key="item.id"
        class="rounded-lg border border-line bg-surface/30 overflow-hidden"
      >
        <div class="flex items-start gap-2 px-2.5 py-2">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-[11px] font-bold text-ink">{{ item.title }}</span>
              <UiBadge :tone="statusTone(item.severity)">{{ statusLabel(item.severity) }}</UiBadge>
              <span class="text-[10px] font-mono text-ink-muted">{{ item.count }}</span>
            </div>
            <p class="text-[10px] text-ink-muted leading-relaxed mt-0.5">{{ item.summary }}</p>

            <button
              v-if="item.targets.length"
              @click="toggleExpand(item.id)"
              class="mt-1 flex items-center gap-1 text-[10px] text-ink-muted hover:text-accent transition cursor-pointer"
            >
              <component :is="expanded[item.id] ? ChevronDown : ChevronRight" class="w-3 h-3" />
              <span>{{ expanded[item.id] ? "收起" : "查看影响范围" }}（{{ item.targets.length }}）</span>
            </button>
            <div v-if="expanded[item.id]" class="mt-1 max-h-28 overflow-y-auto space-y-0.5">
              <div
                v-for="t in item.targets"
                :key="t"
                class="font-mono text-[10px] text-ink-muted truncate"
                :title="t"
              >{{ t }}</div>
            </div>

            <div
              v-if="fixResults[item.id]"
              class="mt-1.5 rounded px-1.5 py-1 text-[10px] leading-relaxed"
              :class="fixResults[item.id]?.success ? 'bg-success/10 text-success' : 'bg-warning/10 text-warning'"
            >
              <div>{{ fixResults[item.id]?.message }}</div>
              <div v-if="fixResults[item.id]?.backupDir" class="text-ink-muted mt-0.5 font-mono truncate">
                备份：{{ fixResults[item.id]?.backupDir }}
              </div>
            </div>
          </div>
          <UiButton
            variant="primary"
            size="xs"
            :disabled="runningId === item.id"
            class="shrink-0"
            @click="applyFix(item)"
          >
            {{ runningId === item.id ? "处理中..." : item.fixLabel }}
          </UiButton>
        </div>
      </div>
    </div>

    <div v-else-if="result && result.analysisComplete" class="flex items-center gap-2 px-3 py-3 rounded-lg border border-success/25 bg-success/10">
      <CheckCircle2 class="w-4 h-4 text-success shrink-0" />
      <p class="text-[11px] text-ink">{{ t("fix.nothing.to.fix") }}</p>
    </div>

    <div v-else-if="result" class="rounded-lg border border-warning/25 bg-warning/10 px-3 py-3">
      <p class="text-[11px] text-warning">扫描不完整，当前无法确认仓库是否干净，请先处理上方告警。</p>
    </div>

    <UiSkeleton v-else-if="isScanning && !result" variant="card" />

    <UiEmpty
      v-else-if="!isScanning"
      :title="t('fix.empty.title')"
      :hint="t('fix.empty.hint')"
      bordered
    />

    <p class="text-[10px] text-ink-muted/70 flex items-center gap-1">
      <Undo2 class="w-3 h-3" />
      修复只做「取消暂存 / 补规则 / 转 LFS / 破锁」这类可逆动作，不重写历史。
    </p>
  </div>
</template>
