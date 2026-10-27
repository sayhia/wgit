<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
import { AlertTriangle, Copy, Download, Trash2, RotateCcw, Check, ChevronDown, ChevronRight, Bug, ShieldAlert } from "lucide-vue-next";
import { useModalA11y } from "../composables/useModalA11y";
import {
  getCrashLogs,
  clearCrashLogs,
  exportCrashLogsJson,
  type CrashRecord,
} from "../utils/crashReporter";
import { t } from "../i18n";
import UiButton from "./ui/UiButton.vue";
import UiEmpty from "./ui/UiEmpty.vue";

const props = defineProps<{
  isOpen: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "notify", msg: string): void;
  (e: "safe-reset"): void;
}>();

const { containerRef: dialogRef } = useModalA11y(() => props.isOpen, () => emit("close"));

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

function handleSafeReset() {
  if (confirm(t("crash.safe.reset.confirm"))) {
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
  <div
    v-if="isOpen"
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
    @click.self="emit('close')"
  >
    <div
      ref="dialogRef"
      role="dialog"
      aria-modal="true"
      data-glass
      class="w-full max-w-2xl max-h-[85vh] rounded-2xl border glass glass-panel flex flex-col overflow-hidden"
    >
      <!-- Header -->
      <div class="px-5 py-4 border-b border-line flex items-center justify-between shrink-0">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-warning/15 border border-warning/30 flex items-center justify-center text-warning">
            <Bug class="w-4 h-4" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-ink">{{ t("crash.modal.title") }}</h3>
            <p class="text-[11px] text-ink-muted">{{ t("crash.modal.desc") }}</p>
          </div>
        </div>
        <button
          type="button"
          class="p-1 rounded-md text-ink-muted hover:text-ink hover:bg-raised transition cursor-pointer text-xs"
          @click="emit('close')"
        >
          ✕
        </button>
      </div>

      <!-- Action Toolbar -->
      <div class="px-5 py-2.5 border-b border-line/60 bg-surface/40 flex items-center justify-between shrink-0 text-xs">
        <div class="flex items-center gap-2">
          <span class="text-ink-muted font-medium">
            {{ crashes.length }} {{ crashes.length === 1 ? 'record' : 'records' }}
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
      <div class="p-5 overflow-y-auto flex-1 space-y-3 font-sans">
        <UiEmpty
          v-if="crashes.length === 0"
          :title="t('crash.empty.title')"
          :hint="t('crash.empty.desc')"
          bordered
        />

        <div
          v-for="item in crashes"
          :key="item.id"
          class="rounded-xl border border-line/70 bg-raised/40 overflow-hidden text-xs transition duration-150"
        >
          <div
            class="p-3 flex items-start justify-between gap-3 cursor-pointer select-none hover:bg-raised/70"
            @click="toggleExpand(item.id)"
          >
            <div class="flex items-start gap-2 min-w-0">
              <component
                :is="expandedIds.has(item.id) ? ChevronDown : ChevronRight"
                class="w-4 h-4 text-ink-muted shrink-0 mt-0.5"
              />
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold uppercase bg-danger/15 text-danger border border-danger/30">
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
          </div>

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

      <!-- Footer with Safe Reset Option -->
      <div class="px-5 py-3 border-t border-line bg-surface/50 flex items-center justify-between shrink-0 text-xs">
        <div class="flex items-center gap-1.5 text-ink-muted text-[11px]">
          <ShieldAlert class="w-3.5 h-3.5 text-warning" />
          <span>{{ t("crash.safe.reset.desc") }}</span>
        </div>
        <UiButton variant="secondary" size="sm" @click="handleSafeReset" class="flex items-center gap-1.5 text-warning">
          <RotateCcw class="w-3 h-3" />
          <span>{{ t("crash.safe.reset") }}</span>
        </UiButton>
      </div>
    </div>
  </div>
</template>
