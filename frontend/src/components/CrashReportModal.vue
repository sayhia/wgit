<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
import { AlertTriangle, Copy, Download, Trash2, RotateCcw, Check, ChevronDown, ChevronRight, Bug, ShieldAlert } from "lucide-vue-next";
import { confirmAction } from "../composables/useConfirm";
import {
  getCrashLogs,
  clearCrashLogs,
  exportCrashLogsJson,
  type CrashRecord,
} from "../utils/crashReporter";
import { t } from "../i18n";
import UiButton from "./ui/UiButton.vue";
import UiEmpty from "./ui/UiEmpty.vue";
import UiModal from "./ui/UiModal.vue";

const props = defineProps<{
  isOpen: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "notify", msg: string): void;
  (e: "safe-reset"): void;
}>();

const crashes = ref<CrashRecord[]>([]);
const expandedIds = ref<Set<string>>(new Set());
const isCopied = ref(false);

function refreshCrashes() {
  const logs = getCrashLogs();
  crashes.value = logs.slice().reverse();
}

function toggleExpand(id: string) {
  if (expandedIds.value.has(id)) {
    expandedIds.value.delete(id);
  } else {
    expandedIds.value.add(id);
  }
}

async function handleCopy() {
  const json = exportCrashLogsJson();
  try {
    await navigator.clipboard.writeText(json);
    isCopied.value = true;
    emit("notify", t("crash.copied"));
    setTimeout(() => {
      isCopied.value = false;
    }, 2000);
  } catch {
    emit("notify", "Failed to copy crash logs");
  }
}

function handleExport() {
  const json = exportCrashLogsJson();
  const blob = new Blob([json], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `wgit-crash-report-${Date.now()}.json`;
  a.click();
  URL.revokeObjectURL(url);
  emit("notify", t("crash.export.json"));
}

function handleClear() {
  clearCrashLogs();
  refreshCrashes();
  emit("notify", t("crash.cleared"));
}

async function handleSafeReset() {
  const ok = await confirmAction({
    message: t("crash.safe.reset.confirm"),
    tone: "warning",
    confirmLabel: t("crash.safe.reset"),
  });
  if (!ok) return;
  try {
    localStorage.removeItem("wgit.session.v1");
    localStorage.removeItem("wgit.active-tab");
    localStorage.removeItem("wgit.window.v1");
  } catch {
    // best-effort
  }
  emit("safe-reset");
  emit("notify", t("crash.safe.reset.done"));
  emit("close");
}

watch(
  () => props.isOpen,
  (open) => {
    if (open) refreshCrashes();
  }
);

onMounted(() => {
  refreshCrashes();
});
</script>

<template>
  <UiModal
    :open="isOpen"
    width="max-w-2xl"
    close-on-backdrop
    :label="t('crash.modal.title')"
    @update:open="(open: boolean) => !open && emit('close')"
  >
    <template #header>
      <div class="flex items-center gap-2.5">
        <div class="w-8 h-8 rounded-lg bg-warning-bg border border-warning/30 flex items-center justify-center text-warning">
          <Bug class="w-4 h-4" />
        </div>
        <div>
          <h3 class="text-sm font-bold text-ink">{{ t("crash.modal.title") }}</h3>
          <p class="text-[11px] text-ink-muted">{{ t("crash.modal.desc") }}</p>
        </div>
      </div>
    </template>

    <div class="-mx-5 -mt-4 border-b border-line/60 bg-surface px-5 py-2.5 flex items-center justify-between shrink-0 text-xs">
      <div class="flex items-center gap-2">
        <span class="text-ink-muted font-medium">
          {{ t("crash.record.count", { n: crashes.length }) }}
        </span>
      </div>
      <div class="flex items-center gap-2">
        <UiButton
          v-if="crashes.length > 0"
          variant="secondary"
          size="sm"
          @click="handleCopy"
          class="flex items-center gap-1.5"
        >
          <component :is="isCopied ? Check : Copy" class="w-3.5 h-3.5" :class="isCopied ? 'text-success' : ''" />
          <span>{{ t("crash.copy") }}</span>
        </UiButton>
        <UiButton
          v-if="crashes.length > 0"
          variant="secondary"
          size="sm"
          @click="handleExport"
          class="flex items-center gap-1.5"
        >
          <Download class="w-3.5 h-3.5" />
          <span>{{ t("crash.export.json") }}</span>
        </UiButton>
        <UiButton
          v-if="crashes.length > 0"
          variant="danger"
          size="sm"
          @click="handleClear"
          class="flex items-center gap-1.5"
        >
          <Trash2 class="w-3.5 h-3.5" />
          <span>{{ t("crash.clear") }}</span>
        </UiButton>
      </div>
    </div>

    <!-- Body / Logs List -->
    <div class="space-y-3 font-sans">
      <UiEmpty
        v-if="crashes.length === 0"
        :title="t('crash.empty.title')"
        :hint="t('crash.empty.desc')"
        bordered
      />

      <div
        v-for="item in crashes"
        :key="item.id"
        class="rounded-xl border border-line/70 bg-raised overflow-hidden text-xs transition duration-150"
      >
        <button
          type="button"
          class="w-full text-left p-3 flex items-start justify-between gap-3 cursor-pointer select-none hover:bg-raised rounded-t-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/70"
          :aria-expanded="expandedIds.has(item.id)"
          @click="toggleExpand(item.id)"
        >
          <div class="flex items-start gap-2 min-w-0">
            <component
              :is="expandedIds.has(item.id) ? ChevronDown : ChevronRight"
              class="w-4 h-4 text-ink-muted shrink-0 mt-0.5"
  aria-hidden="true"
            />
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold uppercase bg-danger-bg text-danger border border-danger/30">
                  {{ item.scope }}
                </span>
                <span class="text-[10px] font-mono text-ink-muted">
                  {{ new Date(item.timestamp).toLocaleString() }}
                </span>
              </div>
              <p class="font-mono text-xs text-ink font-medium mt-1 break-all">
                {{ item.message }}
              </p>
            </div>
          </div>
        </button>

        <!-- Stack & Context -->
        <div
          v-if="expandedIds.has(item.id)"
          class="p-3 border-t border-line/60 bg-black/30 space-y-2 text-[11px] font-mono"
        >
          <div v-if="item.stack" class="overflow-x-auto text-ink-muted leading-relaxed whitespace-pre font-mono p-2 rounded bg-black/40">
            {{ item.stack }}
          </div>
          <div v-if="item.context" class="overflow-x-auto text-ink-muted p-2 rounded bg-black/40">
            <pre>{{ JSON.stringify(item.context, null, 2) }}</pre>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="w-full flex items-center justify-between text-xs">
        <div class="flex items-center gap-1.5 text-ink-muted text-[11px]">
          <ShieldAlert class="w-3.5 h-3.5 text-warning" />
          <span>{{ t("crash.safe.reset.desc") }}</span>
        </div>
        <UiButton variant="secondary" size="sm" @click="handleSafeReset" class="flex items-center gap-1.5 text-warning">
          <RotateCcw class="w-3 h-3" />
          <span>{{ t("crash.safe.reset") }}</span>
        </UiButton>
      </div>
    </template>
  </UiModal>
</template>
