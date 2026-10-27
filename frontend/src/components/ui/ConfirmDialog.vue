<script setup lang="ts">
import { computed } from "vue";
import { AlertTriangle, ShieldAlert, HelpCircle } from "lucide-vue-next";
import UiModal from "./UiModal.vue";
import UiButton from "./UiButton.vue";
import { canConfirm, confirmState, settleConfirm } from "../../composables/useConfirm";
import { t } from "../../i18n";

/**
 * Single host for `confirmAction()`. Mounted once in App.vue so every view
 * shares the same styled, keyboard-accessible dialog instead of window.confirm.
 */
const tone = computed(() => confirmState.tone ?? "default");
const icon = computed(() =>
  tone.value === "danger" ? ShieldAlert : tone.value === "warning" ? AlertTriangle : HelpCircle
);
const iconClass = computed(() =>
  tone.value === "danger"
    ? "bg-danger/12 text-danger"
    : tone.value === "warning"
      ? "bg-warning/12 text-warning"
      : "bg-accent/12 text-accent"
);
const confirmVariant = computed(() =>
  tone.value === "danger" ? "danger" : tone.value === "warning" ? "warning" : "primary"
);
const ready = computed(() => canConfirm(confirmState));

function onOpenChange(open: boolean) {
  if (!open) settleConfirm(false);
}

function accept() {
  if (!ready.value) return;
  settleConfirm(true);
}
</script>

<template>
  <UiModal
    :open="confirmState.open"
    :title="confirmState.title || t('common.confirm')"
    :label="confirmState.title || confirmState.message"
    width="max-w-md"
    hide-close
    @update:open="onOpenChange"
  >
    <div class="flex items-start gap-3">
      <span class="w-9 h-9 rounded-xl flex items-center justify-center shrink-0" :class="iconClass">
        <component :is="icon" class="w-4.5 h-4.5" />
      </span>
      <div class="min-w-0 flex-1 space-y-2">
        <p class="text-sm font-semibold text-ink leading-relaxed break-words whitespace-pre-line">
          {{ confirmState.message }}
        </p>
        <p v-if="confirmState.detail" class="text-xs text-ink-muted leading-relaxed break-words whitespace-pre-line">
          {{ confirmState.detail }}
        </p>

        <div v-if="confirmState.affectedSummary" class="p-2 rounded bg-danger/10 border border-danger/20 text-[11px] text-danger flex items-center gap-1.5 font-medium">
          <AlertTriangle class="w-3.5 h-3.5 shrink-0" />
          <span>{{ confirmState.affectedSummary }}</span>
        </div>

        <div v-if="confirmState.requireText" class="pt-1 space-y-1.5">
          <label class="block text-[11px] text-ink-muted" for="confirm-require-text">
            {{
              confirmState.requireTextHint ||
              t("confirm.require.text", { text: confirmState.requireText })
            }}
          </label>
          <input
            id="confirm-require-text"
            v-model="confirmState.requireTextValue"
            type="text"
            autocomplete="off"
            spellcheck="false"
            class="w-full px-2.5 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent-strong"
            :placeholder="confirmState.requireText"
            @keydown.enter.prevent="accept"
          />
        </div>
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="settleConfirm(false)">
        {{ confirmState.cancelLabel || t("common.cancel") }}
      </UiButton>
      <UiButton :variant="confirmVariant" :disabled="!ready" @click="accept">
        {{ confirmState.confirmLabel || t("common.confirm") }}
      </UiButton>
    </template>
  </UiModal>
</template>
