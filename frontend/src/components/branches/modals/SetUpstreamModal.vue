<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Link2, Search, Check } from "lucide-vue-next";
import UiModal from "../../ui/UiModal.vue";
import UiButton from "../../ui/UiButton.vue";
import { t } from "../../../i18n";
import type { BranchInfo } from "../../../types";

const props = defineProps<{
  isOpen: boolean;
  branch: BranchInfo | null;
  availableRemoteBranches: string[];
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "submit", upstream: string): void;
}>();

const upstreamInput = ref("");
const searchQuery = ref("");

function findSuggestedUpstream(): string {
  if (props.branch?.upstream) {
    return props.branch.upstream;
  }
  const branchName = props.branch?.name || "";
  if (!branchName) return "";

  // 1. Exact match for origin/<branchName>
  const exactOrigin = `origin/${branchName}`;
  if (props.availableRemoteBranches.includes(exactOrigin)) {
    return exactOrigin;
  }

  // 2. Any remote matching /<branchName>
  const matched = props.availableRemoteBranches.find((rb) =>
    rb.endsWith(`/${branchName}`)
  );
  if (matched) {
    return matched;
  }

  // 3. Fallback to origin/<branchName>
  return exactOrigin;
}

watch(
  () => props.isOpen,
  (open) => {
    if (open) {
      upstreamInput.value = findSuggestedUpstream();
      searchQuery.value = "";
    }
  },
  { immediate: true }
);

const filteredBranches = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return props.availableRemoteBranches;
  return props.availableRemoteBranches.filter((b) =>
    b.toLowerCase().includes(q)
  );
});

const isValid = computed(() => {
  const val = upstreamInput.value.trim();
  if (!val) return false;
  if (/\s/.test(val)) return false;
  if (val.startsWith("/") || val.endsWith("/")) return false;
  return true;
});

function handleSubmit() {
  if (!isValid.value) return;
  emit("submit", upstreamInput.value.trim());
}
</script>

<template>
  <UiModal
    :open="isOpen"
    :label="t('branches.upstream.modal.title')"
    width="max-w-md"
    @update:open="emit('close')"
  >
    <template #header>
      <h3 class="text-sm font-bold text-ink flex items-center gap-2">
        <Link2 class="w-4 h-4 text-accent" />
        <span>{{ t("branches.upstream.modal.title") }}</span>
      </h3>
    </template>

    <div class="space-y-3.5">
      <p class="text-xs text-ink-muted">
        {{ t("branches.upstream.modal.intro", { branch: branch?.name || "" }) }}
      </p>

      <div
        v-if="branch?.upstream"
        class="flex items-center justify-between p-2.5 rounded-xl bg-canvas border border-line text-xs font-mono"
      >
        <span class="text-ink-muted text-[11px]">{{ t("branches.upstream.modal.current") }}</span>
        <span class="font-bold text-accent">{{ branch.upstream }}</span>
      </div>

      <div class="space-y-1">
        <label class="block text-xs font-medium text-ink-secondary" for="wgit-upstream-input">
          {{ t("branches.upstream.modal.target") }}
        </label>
        <input
          id="wgit-upstream-input"
          v-model="upstreamInput"
          type="text"
          :placeholder="t('branches.upstream.modal.target.placeholder')"
          @keyup.enter="handleSubmit"
          class="w-full px-3 py-2 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
        />
        <p v-if="upstreamInput && !isValid" class="text-[11px] text-danger">
          {{ t("branches.upstream.modal.invalid") }}
        </p>
      </div>

      <div
        v-if="availableRemoteBranches.length > 0"
        class="space-y-2 pt-1"
        role="group"
        aria-labelledby="wgit-upstream-select-label"
      >
        <div class="flex items-center justify-between">
          <label id="wgit-upstream-select-label" class="text-xs font-medium text-ink-secondary">
            {{ t("branches.upstream.modal.select.label") }}
          </label>
          <span class="text-[10px] text-ink-muted font-mono">
            {{ filteredBranches.length }}
          </span>
        </div>

        <div class="relative">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-ink-muted" />
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('branches.upstream.modal.search.placeholder')"
            class="w-full pl-8 pr-3 py-1.5 rounded-lg bg-canvas border border-line text-xs text-ink placeholder-ink-muted focus:outline-none focus:border-accent font-mono"
          />
        </div>

        <div class="max-h-40 overflow-y-auto rounded-xl border border-line bg-surface divide-y divide-line/40">
          <button
            v-for="rb in filteredBranches"
            :key="rb"
            type="button"
            @click="upstreamInput = rb"
            class="w-full text-left px-3 py-2 text-xs font-mono flex items-center justify-between hover:bg-hover transition cursor-pointer"
            :class="{
              'bg-accent-bg text-accent font-bold': upstreamInput === rb,
              'text-ink': upstreamInput !== rb,
            }"
          >
            <span class="truncate">{{ rb }}</span>
            <Check v-if="upstreamInput === rb" class="w-3.5 h-3.5 text-accent shrink-0 ml-2" />
          </button>
          <div v-if="filteredBranches.length === 0" class="p-4 text-center text-xs text-ink-muted">
            {{ t("branches.upstream.modal.empty") }}
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <UiButton variant="ghost" @click="emit('close')">
        {{ t("branches.upstream.modal.cancel") }}
      </UiButton>
      <UiButton variant="primary" :disabled="!isValid" @click="handleSubmit">
        {{ t("branches.upstream.modal.confirm") }}
      </UiButton>
    </template>
  </UiModal>
</template>
