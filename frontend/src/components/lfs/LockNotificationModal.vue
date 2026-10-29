<script setup lang="ts">
import { reportError } from "../../composables/useToast";
import { ref, watch } from "vue";
import { Copy, Check, MessageSquare, Mail, Terminal, RefreshCw } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { LfsLockNotificationTemplate } from "../../types";
import { t } from "../../i18n";
import UiButton from "../ui/UiButton.vue";
import UiModal from "../ui/UiModal.vue";

const props = defineProps<{
  open: boolean;
  repoPath: string;
  filePath: string;
  owner?: string;
}>();

const emit = defineEmits<{ (e: "update:open", value: boolean): void }>();

const note = ref("");
const activeTab = ref<"markdown" | "email" | "webhook">("markdown");
const loading = ref(false);
const error = ref("");
const template = ref<LfsLockNotificationTemplate | null>(null);
const copied = ref(false);

async function generate() {
  if (!props.repoPath || !props.filePath) return;
  loading.value = true;
  error.value = "";
  try {
    template.value = await gitService.generateLfsLockNotification(
      props.repoPath,
      props.filePath,
      note.value
    );
  } catch (err: any) {
    error.value = err?.message || String(err);
  } finally {
    loading.value = false;
  }
}

watch(
  () => [props.open, props.filePath],
  () => {
    if (props.open && props.filePath) {
      copied.value = false;
      void generate();
    }
  },
  { immediate: true }
);

async function copyCurrentContent() {
  if (!template.value) return;
  let text = "";
  if (activeTab.value === "markdown") {
    text = template.value.markdown;
  } else if (activeTab.value === "email") {
    text = `Subject: ${template.value.emailSubject}\n\n${template.value.emailBody}`;
  } else {
    text = template.value.webhookJson;
  }

  try {
    await navigator.clipboard.writeText(text);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch (err) {
    reportError("Failed to copy lock notice", err);
  }
}
</script>

<template>
  <UiModal
    :open="open"
    width="max-w-[680px]"
    :label="t('lfs.notify.modal.title')"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <div>
        <div class="flex items-center gap-2">
          <MessageSquare class="w-4 h-4 text-accent" />
          <h3 class="text-sm font-bold text-ink">{{ t("lfs.notify.modal.title") }}</h3>
        </div>
        <p class="text-xs text-ink-muted mt-1 font-mono truncate max-w-[500px]" :title="filePath">
          {{ filePath }}
        </p>
      </div>
    </template>

    <!-- Body -->
    <div class="space-y-4">
      <!-- Custom note input -->
      <div class="space-y-1.5">
        <label class="text-xs font-semibold text-ink-secondary">{{ t("lfs.notify.modal.note.label") }}</label>
        <div class="flex gap-2">
          <input
            v-model="note"
            type="text"
            :placeholder="t('lfs.notify.modal.note.placeholder')"
            @keydown.enter="generate"
            class="flex-1 px-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink focus:outline-none focus:border-accent"
          />
          <UiButton variant="secondary" size="sm" :disabled="loading" @click="generate" class="flex items-center gap-1.5 shrink-0">
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
            <span>{{ t("common.refresh") }}</span>
          </UiButton>
        </div>
      </div>

      <div v-if="loading" class="py-12 flex flex-col items-center gap-2 text-ink-muted">
        <RefreshCw class="w-5 h-5 animate-spin" />
        <span class="text-xs">{{ t("common.loading") }}</span>
      </div>

      <p v-else-if="error" class="py-6 text-center text-xs text-danger">{{ error }}</p>

      <template v-else-if="template">
        <!-- Format Tabs -->
        <div class="flex items-center gap-1 p-0.5 rounded-lg border border-line bg-canvas w-fit">
          <button
            @click="activeTab = 'markdown'"
            class="flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="activeTab === 'markdown' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <MessageSquare class="w-3.5 h-3.5" />
            <span>Markdown</span>
          </button>
          <button
            @click="activeTab = 'email'"
            class="flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="activeTab === 'email' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Mail class="w-3.5 h-3.5" />
            <span>{{ t("lfs.notify.modal.tab.email") }}</span>
          </button>
          <button
            @click="activeTab = 'webhook'"
            class="flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition cursor-pointer"
            :class="activeTab === 'webhook' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
          >
            <Terminal class="w-3.5 h-3.5" />
            <span>Webhook JSON</span>
          </button>
        </div>

        <!-- Preview Content -->
        <div class="rounded-xl border border-line bg-canvas/70 p-3.5 font-mono text-xs leading-relaxed max-h-64 overflow-auto whitespace-pre-wrap select-all">
          <template v-if="activeTab === 'markdown'">{{ template.markdown }}</template>
          <template v-else-if="activeTab === 'email'">Subject: {{ template.emailSubject }}&#10;&#10;{{ template.emailBody }}</template>
          <template v-else>{{ template.webhookJson }}</template>
        </div>
      </template>
    </div>

    <template #footer>
      <div class="w-full flex items-center justify-between">
        <span class="text-xs text-ink-muted">{{ t("lfs.notify.modal.hint") }}</span>
        <UiButton variant="primary" size="sm" :disabled="!template || loading" @click="copyCurrentContent" class="flex items-center gap-1.5">
          <component :is="copied ? Check : Copy" class="w-4 h-4" />
          <span>{{ copied ? t("common.copied") : t("lfs.notify.modal.copy") }}</span>
        </UiButton>
      </div>
    </template>
  </UiModal>
</template>
