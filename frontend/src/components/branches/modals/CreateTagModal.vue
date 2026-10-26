<script setup lang="ts">
import { computed } from "vue";
import { Tag } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";
import { t } from "../../../i18n";

const props = defineProps<{
  open: boolean;
  name: string;
  message: string;
  sign: boolean;
  lastTag?: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "update:name", value: string): void;
  (e: "update:message", value: string): void;
  (e: "update:sign", value: boolean): void;
  (e: "confirm"): void;
}>();

const nameModel = computed({
  get: () => props.name,
  set: (value: string) => emit("update:name", value),
});
const messageModel = computed({
  get: () => props.message,
  set: (value: string) => emit("update:message", value),
});
const signModel = computed({
  get: () => props.sign,
  set: (value: boolean) => emit("update:sign", value),
});

const semverPresets = computed(() => {
  if (!props.lastTag) return null;
  const match = props.lastTag.trim().match(/^v?(\d+)\.(\d+)\.(\d+)$/);
  const prefix = props.lastTag.startsWith("v") ? "v" : "";
  if (match && match[1] && match[2] && match[3]) {
    const major = parseInt(match[1], 10);
    const minor = parseInt(match[2], 10);
    const patch = parseInt(match[3], 10);
    return {
      patch: `${prefix}${major}.${minor}.${patch + 1}`,
      minor: `${prefix}${major}.${minor + 1}.0`,
      major: `${prefix}${major + 1}.0.0`,
    };
  }
  return null;
});

const tagTypeBadge = computed(() => {
  if (signModel.value) {
    return { label: t("branches.tag.type.signed"), class: "bg-purple/20 text-purple border-purple/30" };
  }
  if (messageModel.value.trim()) {
    return { label: t("branches.tag.type.annotated"), class: "bg-accent/20 text-accent border-accent/30" };
  }
  return { label: t("branches.tag.type.lightweight"), class: "bg-line/60 text-ink-secondary border-line" };
});
</script>

<template>
  <UiModal
    :open="open"
    :label="t('branches.tag.create.title')"
    width="max-w-sm"
    @update:open="emit('update:open', $event)"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Tag class="w-4 h-4 text-purple" />
        <span>{{ t("branches.tag.create.heading") }}</span>
      </h3>
    </template>

    <div class="space-y-3">
      <!-- Tag Type Indicator Badge -->
      <div class="flex items-center justify-between">
        <span class="text-[11px] text-ink-muted">{{ t("branches.tag.type.label") }}</span>
        <span class="px-2 py-0.5 rounded text-[10px] font-medium border" :class="tagTypeBadge.class">
          {{ tagTypeBadge.label }}
        </span>
      </div>

      <div class="flex flex-col gap-1">
        <div class="flex items-center justify-between">
          <label class="text-[11px] text-ink-secondary font-medium" for="wgit-tag-name">
            {{ t("branches.tag.create.name") }}
          </label>
          <div v-if="semverPresets" class="flex items-center gap-1">
            <button
              type="button"
              @click="nameModel = semverPresets.patch"
              class="px-1.5 py-0.5 rounded text-[10px] font-mono border border-line bg-surface hover:bg-raised text-ink-secondary cursor-pointer transition"
              :class="{ 'bg-purple/20 text-purple border-purple/40 font-bold': nameModel === semverPresets.patch }"
            >
              Patch {{ semverPresets.patch }}
            </button>
            <button
              type="button"
              @click="nameModel = semverPresets.minor"
              class="px-1.5 py-0.5 rounded text-[10px] font-mono border border-line bg-surface hover:bg-raised text-ink-secondary cursor-pointer transition"
              :class="{ 'bg-purple/20 text-purple border-purple/40 font-bold': nameModel === semverPresets.minor }"
            >
              Minor {{ semverPresets.minor }}
            </button>
            <button
              type="button"
              @click="nameModel = semverPresets.major"
              class="px-1.5 py-0.5 rounded text-[10px] font-mono border border-line bg-surface hover:bg-raised text-ink-secondary cursor-pointer transition"
              :class="{ 'bg-purple/20 text-purple border-purple/40 font-bold': nameModel === semverPresets.major }"
            >
              Major {{ semverPresets.major }}
            </button>
          </div>
        </div>
        <input
          id="wgit-tag-name"
          v-model="nameModel"
          type="text"
          placeholder="v1.0.0"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
        />
      </div>
      <div class="flex flex-col gap-1">
        <label class="text-[11px] text-ink-secondary font-medium" for="wgit-tag-message">
          {{ t("branches.tag.create.annotation") }}
        </label>
        <input
          id="wgit-tag-message"
          v-model="messageModel"
          type="text"
          :placeholder="t('branches.tag.create.annotation.placeholder')"
          @keyup.enter="emit('confirm')"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-sans"
        />
      </div>
      <label class="flex items-start gap-2 text-[11px] text-ink-secondary cursor-pointer select-none">
        <input type="checkbox" v-model="signModel" class="mt-0.5 rounded accent-[var(--ug-purple-strong)]" />
        <span>{{ t("branches.tag.create.sign") }}</span>
      </label>
    </div>

    <template #footer>
      <UiButton variant="ghost" @click="emit('update:open', false)">{{ t("branches.tag.create.cancel") }}</UiButton>
      <UiButton variant="purple" @click="emit('confirm')">{{ t("branches.tag.create.confirm") }}</UiButton>
    </template>
  </UiModal>
</template>
