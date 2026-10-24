<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { KeyRound, Loader2, Trash2 } from "lucide-vue-next";
import type { PrProviderInfo } from "../../types";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";

const props = defineProps<{
  open: boolean;
  provider: PrProviderInfo | null;
  busy: boolean;
}>();

const emit = defineEmits<{
  (e: "save", token: string): void;
  (e: "clear"): void;
  (e: "update:open", value: boolean): void;
}>();

const token = ref("");
const envNames = computed(() =>
  props.provider?.provider === "gitlab" ? "GITLAB_TOKEN / GL_TOKEN" : "GITHUB_TOKEN / GH_TOKEN"
);

watch(
  () => props.open,
  () => {
    // Never keep a typed token in memory after the dialog closes.
    token.value = "";
  }
);

function submit() {
  const value = token.value.trim();
  if (!value || props.busy) return;
  emit("save", value);
}
</script>

<template>
  <UiModal
    :open="open"
    :title="t('pr.token.setup')"
    width="max-w-md"
    @update:open="emit('update:open', $event)"
  >
    <div class="space-y-3">
      <div class="flex items-start gap-2.5">
        <span class="w-8 h-8 rounded-lg bg-accent/12 text-accent flex items-center justify-center shrink-0">
          <KeyRound class="w-4 h-4" />
        </span>
        <div class="min-w-0 text-[11px] text-ink-muted leading-relaxed">
          <p class="font-mono text-ink truncate" :title="provider?.host">{{ provider?.host || "—" }}</p>
          <p v-if="provider?.tokenHint">{{ provider.tokenHint }}</p>
        </div>
      </div>

      <div class="space-y-1.5">
        <label for="pr-token-input" class="block text-[11px] font-semibold text-ink">
          {{ t("pr.token.label") }}
        </label>
        <input
          id="pr-token-input"
          v-model="token"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :placeholder="t('pr.token.placeholder')"
          class="w-full px-2.5 py-2 rounded-lg bg-canvas border border-line text-xs text-ink font-mono focus:outline-none focus:border-accent-strong"
          @keydown.enter.prevent="submit"
        />
        <p class="text-[10px] text-ink-muted leading-relaxed">{{ t("pr.token.hint") }}</p>
        <p class="text-[10px] text-ink-muted">
          {{ t("pr.token.source", { source: provider?.tokenSource || t("pr.token.missing") }) }}
          <template v-if="provider?.account">· {{ t("pr.token.account", { account: provider.account }) }}</template>
        </p>
        <p class="text-[10px] text-ink-muted font-mono">env: {{ envNames }}</p>
      </div>
    </div>

    <template #footer>
      <UiButton
        v-if="provider?.hasToken"
        variant="danger"
        :disabled="busy"
        @click="emit('clear')"
      >
        <Trash2 class="w-3 h-3" />
        {{ t("pr.token.clear") }}
      </UiButton>
      <span class="flex-1"></span>
      <UiButton variant="secondary" @click="emit('update:open', false)">{{ t("common.cancel") }}</UiButton>
      <UiButton variant="primary" :disabled="busy || !token.trim()" @click="submit">
        <Loader2 v-if="busy" class="w-3 h-3 animate-spin" />
        {{ t("pr.token.setup") }}
      </UiButton>
    </template>
  </UiModal>
</template>
