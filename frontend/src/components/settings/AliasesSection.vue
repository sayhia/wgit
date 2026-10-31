<script setup lang="ts">
import { confirmAction } from "../../composables/useConfirm";
import { ref, computed, watch, onMounted } from "vue";
import { RefreshCw, Search, Plus, Terminal, Trash2, Edit2, Copy, Check, X, Shield } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { GitAliasItem } from "../../types";
import { t } from "../../i18n";
import UiModal from "../ui/UiModal.vue";
import UiButton from "../ui/UiButton.vue";

const props = defineProps<{
  currentRepoPath: string;
}>();

const emit = defineEmits<{
  (e: "notify", msg: string): void;
}>();

function notify(msg: string) {
  emit("notify", msg);
}

const aliases = ref<GitAliasItem[]>([]);
const isLoading = ref(false);
const filter = ref("");
const scopeFilter = ref<"all" | "local" | "global">("all");

// Add / Edit Modal state
const showModal = ref(false);
const isEditing = ref(false);
const formKey = ref("");
const formCommand = ref("");
const formScope = ref<"local" | "global">("global");
const isSaving = ref(false);
const copiedKey = ref("");

const filteredAliases = computed(() => {
  const q = filter.value.trim().toLowerCase();
  return aliases.value.filter((a) => {
    if (scopeFilter.value !== "all" && a.scope !== scopeFilter.value) {
      return false;
    }
    if (!q) return true;
    return a.key.toLowerCase().includes(q) || a.command.toLowerCase().includes(q);
  });
});

async function loadAliases() {
  isLoading.value = true;
  try {
    const list = await gitService.getGitAliases(props.currentRepoPath);
    aliases.value = list || [];
  } catch (err: any) {
    notify(t("settings.aliases.load.failed", { error: err?.message || err }));
  } finally {
    isLoading.value = false;
  }
}

function openAddModal() {
  isEditing.value = false;
  formKey.value = "";
  formCommand.value = "";
  formScope.value = props.currentRepoPath ? "local" : "global";
  showModal.value = true;
}

function openEditModal(item: GitAliasItem) {
  isEditing.value = true;
  formKey.value = item.key;
  formCommand.value = item.command;
  formScope.value = item.scope;
  showModal.value = true;
}

async function handleSaveAlias() {
  const key = formKey.value.trim();
  const cmd = formCommand.value.trim();
  if (!key) {
    notify(t("settings.aliases.key.required"));
    return;
  }
  if (!cmd) {
    notify(t("settings.aliases.command.required"));
    return;
  }
  isSaving.value = true;
  try {
    await gitService.setGitAlias(props.currentRepoPath, formScope.value, key, cmd);
    notify(t(isEditing.value ? "settings.aliases.updated" : "settings.aliases.created", { key }));
    showModal.value = false;
    await loadAliases();
  } catch (err: any) {
    notify(t("settings.aliases.save.failed", { error: err?.message || err }));
  } finally {
    isSaving.value = false;
  }
}

async function handleDeleteAlias(item: GitAliasItem) {
  const confirmed = await confirmAction({
    message: t("settings.aliases.confirm.delete", { key: item.key, scope: item.scope }),
    tone: "danger",
  });
  if (!confirmed) return;

  try {
    await gitService.deleteGitAlias(props.currentRepoPath, item.scope, item.key);
    notify(t("settings.aliases.deleted", { key: item.key }));
    await loadAliases();
  } catch (err: any) {
    notify(t("settings.aliases.delete.failed", { error: err?.message || err }));
  }
}

async function copyAliasInvocation(item: GitAliasItem) {
  try {
    await navigator.clipboard.writeText(`git ${item.key}`);
    copiedKey.value = item.key;
    setTimeout(() => {
      copiedKey.value = "";
    }, 2000);
  } catch {
    // best-effort
  }
}

watch(() => props.currentRepoPath, () => {
  void loadAliases();
});

onMounted(() => {
  void loadAliases();
});
</script>

<template>
  <div class="space-y-6 max-w-4xl">
    <!-- Header banner -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 rounded-xl border border-line bg-surface/50">
      <div>
        <h2 class="text-base font-bold text-ink flex items-center gap-2">
          <Terminal class="w-5 h-5 text-accent" />
          <span>{{ t("settings.aliases.title") }}</span>
        </h2>
        <p class="text-xs text-ink-muted mt-1 leading-relaxed">
          {{ t("settings.aliases.desc") }}
        </p>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <button
          type="button"
          @click="loadAliases"
          :disabled="isLoading"
          class="p-2 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer disabled:opacity-50"
          :title="t('settings.aliases.refresh')"
        >
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': isLoading }" aria-hidden="true" />
        </button>

        <UiButton variant="primary" @click="openAddModal" data-testid="new-alias-btn">
          <Plus class="w-4 h-4 mr-1" aria-hidden="true" />
          <span>{{ t("settings.aliases.new.btn") }}</span>
        </UiButton>
      </div>
    </div>

    <!-- Filter and Search Toolbar -->
    <div class="flex flex-col sm:flex-row items-center justify-between gap-3">
      <div class="relative w-full sm:w-72">
        <Search class="w-3.5 h-3.5 text-ink-muted absolute left-3 top-1/2 -translate-y-1/2" />
        <input
          v-model="filter"
          type="text"
          :placeholder="t('settings.aliases.search.placeholder')"
          class="w-full pl-9 pr-3 py-1.5 rounded-lg border border-line bg-canvas text-xs text-ink placeholder:text-ink-muted focus:outline-none focus:border-accent"
        />
      </div>

      <!-- Scope Filter Pills -->
      <div class="flex items-center gap-1 bg-surface p-1 rounded-lg border border-line text-xs">
        <button
          type="button"
          @click="scopeFilter = 'all'"
          class="px-2.5 py-1 rounded-md transition cursor-pointer font-medium"
          :class="scopeFilter === 'all' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
        >
          {{ t("settings.aliases.filter.all", { count: aliases.length }) }}
        </button>
        <button
          type="button"
          @click="scopeFilter = 'local'"
          class="px-2.5 py-1 rounded-md transition cursor-pointer font-medium"
          :class="scopeFilter === 'local' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
        >
          {{ t("settings.aliases.filter.local") }}
        </button>
        <button
          type="button"
          @click="scopeFilter = 'global'"
          class="px-2.5 py-1 rounded-md transition cursor-pointer font-medium"
          :class="scopeFilter === 'global' ? 'bg-raised text-accent font-semibold shadow-xs' : 'text-ink-muted hover:text-ink'"
        >
          {{ t("settings.aliases.filter.global") }}
        </button>
      </div>
    </div>

    <!-- Aliases List -->
    <div v-if="filteredAliases.length === 0" class="py-12 text-center rounded-xl border border-dashed border-line bg-surface/20">
      <Terminal class="w-8 h-8 text-ink-muted mx-auto mb-2 opacity-50" />
      <div class="text-xs text-ink-muted">{{ t("settings.aliases.empty") }}</div>
    </div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-3" data-testid="aliases-grid">
      <div
        v-for="item in filteredAliases"
        :key="`${item.scope}:${item.key}`"
        class="p-3 rounded-xl border border-line bg-surface/40 hover:bg-surface/70 transition flex flex-col justify-between gap-2 shadow-xs"
        data-testid="alias-card"
      >
        <div class="space-y-1.5 min-w-0">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0">
              <span class="font-mono text-sm font-bold text-accent truncate">git {{ item.key }}</span>
              <span
                class="px-1.5 py-0.5 rounded text-[10px] font-mono font-medium border"
                :class="item.scope === 'local' ? 'bg-accent/15 border-accent/30 text-accent' : 'bg-purple/15 border-purple/30 text-purple'"
              >
                {{ item.scope }}
              </span>
            </div>

            <!-- Action buttons -->
            <div class="flex items-center gap-1 shrink-0">
              <button
                type="button"
                @click="copyAliasInvocation(item)"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
                :title="t('settings.aliases.copy.hint')"
              >
                <Check v-if="copiedKey === item.key" class="w-3.5 h-3.5 text-success" aria-hidden="true" />
                <Copy v-else class="w-3.5 h-3.5" aria-hidden="true" />
              </button>
              <button
                type="button"
                @click="openEditModal(item)"
                class="p-1 rounded hover:bg-raised text-ink-muted hover:text-accent transition cursor-pointer"
                :title="t('settings.aliases.edit')"
              >
                <Edit2 class="w-3.5 h-3.5" aria-hidden="true" />
              </button>
              <button
                type="button"
                @click="handleDeleteAlias(item)"
                class="p-1 rounded hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
                :title="t('settings.aliases.delete')"
              >
                <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
              </button>
            </div>
          </div>

          <!-- Command content -->
          <div class="p-2 rounded-lg bg-canvas border border-line font-mono text-xs text-ink leading-relaxed break-all select-all">
            {{ item.command }}
          </div>
        </div>
      </div>
    </div>

    <!-- Create / Edit Modal -->
    <UiModal
      :open="showModal"
      :title="t(isEditing ? 'settings.aliases.modal.edit.title' : 'settings.aliases.modal.new.title')"
      @close="showModal = false"
    >
      <div class="space-y-4">
        <div>
          <label class="block text-xs font-semibold text-ink mb-1">
            {{ t("settings.aliases.field.key") }}
          </label>
          <div class="flex items-center gap-1">
            <span class="text-xs font-mono text-ink-muted select-none">git</span>
            <input
              v-model="formKey"
              type="text"
              :disabled="isEditing"
              :placeholder="t('settings.aliases.field.key.placeholder')"
              class="flex-1 px-3 py-1.5 rounded-lg border border-line bg-canvas text-xs font-mono text-ink focus:outline-none focus:border-accent disabled:opacity-50"
            />
          </div>
        </div>

        <div>
          <label class="block text-xs font-semibold text-ink mb-1">
            {{ t("settings.aliases.field.command") }}
          </label>
          <input
            v-model="formCommand"
            type="text"
            :placeholder="t('settings.aliases.field.command.placeholder')"
            class="w-full px-3 py-1.5 rounded-lg border border-line bg-canvas text-xs font-mono text-ink focus:outline-none focus:border-accent"
          />
        </div>

        <div>
          <label class="block text-xs font-semibold text-ink mb-1">
            {{ t("settings.aliases.field.scope") }}
          </label>
          <div class="flex items-center gap-3 text-xs text-ink">
            <label class="flex items-center gap-1.5 cursor-pointer">
              <input
                v-model="formScope"
                type="radio"
                value="global"
                :disabled="isEditing"
                class="accent-accent"
              />
              <span>{{ t("settings.aliases.scope.global") }} (~/.gitconfig)</span>
            </label>
            <label class="flex items-center gap-1.5 cursor-pointer" :class="{ 'opacity-50 cursor-not-allowed': !currentRepoPath }">
              <input
                v-model="formScope"
                type="radio"
                value="local"
                :disabled="isEditing || !currentRepoPath"
                class="accent-accent"
              />
              <span>{{ t("settings.aliases.scope.local") }} (.git/config)</span>
            </label>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex items-center justify-end gap-2">
          <UiButton variant="secondary" @click="showModal = false">
            {{ t("common.cancel") }}
          </UiButton>
          <UiButton variant="primary" :disabled="isSaving" @click="handleSaveAlias">
            <span>{{ isSaving ? t("settings.aliases.modal.saving") : t("settings.aliases.modal.save") }}</span>
          </UiButton>
        </div>
      </template>
    </UiModal>
  </div>
</template>
