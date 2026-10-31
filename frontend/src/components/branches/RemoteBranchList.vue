<script setup lang="ts">
import { ref } from "vue";
import {
  GitFork,
  GitBranch,
  GitMerge,
  GitPullRequest,
  Copy,
  ExternalLink,
  Trash2,
  ArrowDownCircle,
} from "lucide-vue-next";
import type { BranchInfo } from "../../types";
import { useContextMenu, type ContextMenuItem } from "../../composables/useContextMenu";
import { useRovingList } from "../../composables/useRovingList";
import UiContextMenu from "../ui/UiContextMenu.vue";
import { t } from "../../i18n";

const props = defineProps<{
  total: number;
  visible: BranchInfo[];
  moreCount: number;
}>();

const emit = defineEmits<{
  (e: "checkout", name: string): void;
  (e: "merge", name: string): void;
  (e: "rebase", name: string): void;
  (e: "open-web", name: string): void;
  (e: "delete-remote", name: string): void;
  (e: "show-more"): void;
}>();

const { setItemRef, tabindexFor, onKeydown: onRowKeydown, onFocus: onRowFocus } = useRovingList(
  () => props.visible,
  (branch) => branch.name
);

const {
  isOpen: branchMenuOpen,
  position: branchMenuPos,
  items: branchMenuList,
  open: openMenuAt,
  close: closeBranchMenu,
} = useContextMenu();

const menuBranch = ref<BranchInfo | null>(null);

function branchMenuItems(branch: BranchInfo): ContextMenuItem[] {
  return [
    { id: "checkout", label: t("branches.remote.menu.checkout"), icon: GitBranch },
    { id: "merge", label: t("branches.remote.menu.merge"), icon: GitMerge },
    { id: "rebase", label: t("branches.remote.menu.rebase"), icon: GitPullRequest },
    { id: "copy", label: t("branches.remote.menu.copy"), icon: Copy, hint: branch.name },
    { id: "open-web", label: t("branches.remote.menu.open.web"), icon: ExternalLink },
    {
      id: "delete-remote",
      label: t("branches.remote.menu.delete"),
      icon: Trash2,
      danger: true,
      separatorBefore: true,
    },
  ];
}

function openBranchMenu(branch: BranchInfo, event: MouseEvent) {
  menuBranch.value = branch;
  openMenuAt(event, branchMenuItems(branch));
}

async function handleBranchMenuSelect(id: string) {
  const branch = menuBranch.value;
  if (!branch) return;
  switch (id) {
    case "checkout":
      emit("checkout", branch.name);
      break;
    case "merge":
      emit("merge", branch.name);
      break;
    case "rebase":
      emit("rebase", branch.name);
      break;
    case "copy":
      await navigator.clipboard.writeText(branch.name);
      break;
    case "open-web":
      emit("open-web", branch.name);
      break;
    case "delete-remote":
      emit("delete-remote", branch.name);
      break;
  }
}
</script>

<template>
  <div v-if="total === 0" class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl">
    {{ t("branches.remote.empty") }}
  </div>

  <div v-else class="flex items-center justify-between px-1 text-[11px] text-ink-muted">
    <span>{{ t("branches.remote.total", { count: total }) }}</span>
    <span v-if="moreCount > 0">{{ t("branches.remote.showing", { count: visible.length }) }}</span>
  </div>

  <div
    v-for="(b, index) in visible"
    :key="b.name"
    :ref="(el) => setItemRef(b, el)"
    :tabindex="tabindexFor(b, index)"
    :aria-label="t('branches.remote.row.aria', { name: b.name })"
    class="flex items-center justify-between gap-2 p-3 rounded-xl bg-surface/60 border border-line hover:border-line-strong transition duration-150 group focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
    @focus="onRowFocus(b)"
    @keydown="onRowKeydown($event, index)"
    @keydown.enter.self="emit('checkout', b.name)"
    @contextmenu.prevent="openBranchMenu(b, $event)"
  >
    <div class="flex items-center gap-3 min-w-0">
      <div class="w-8 h-8 rounded-lg bg-cyan/10 text-cyan flex items-center justify-center shrink-0">
        <GitFork class="w-4 h-4" />
      </div>
      <div class="flex flex-col min-w-0">
        <div class="flex items-center gap-2 flex-wrap">
          <span class="font-mono text-xs font-bold text-ink">{{ b.name }}</span>
          <span
            v-if="b.commitHash"
            class="font-mono text-[10px] text-ink-muted bg-raised px-1.5 py-0.5 rounded border border-line"
            :title="t('branches.remote.latest.hint')"
          >
            {{ b.commitHash.substring(0, 8) }}
          </span>
          <span
            v-if="b.ahead"
            class="px-1.5 py-0.5 rounded border border-accent/30 bg-accent/10 text-[10px] text-accent font-bold font-mono"
            :title="t('branches.local.ahead.hint', { count: b.ahead })"
          >↑{{ b.ahead }}</span>
          <span
            v-if="b.behind"
            class="px-1.5 py-0.5 rounded border border-cyan/30 bg-cyan/10 text-[10px] text-cyan font-bold font-mono"
            :title="t('branches.local.behind.hint', { count: b.behind })"
          >↓{{ b.behind }}</span>
        </div>
        <span class="text-[11px] text-ink-muted mt-0.5">{{ t("branches.remote.kind") }}</span>
      </div>
    </div>

    <!-- Actions -->
    <div class="flex items-center gap-1.5 shrink-0">
      <!-- Secondary hover action buttons -->
      <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity duration-150">
        <button
          @click.stop="emit('merge', b.name)"
          class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-accent text-xs font-medium transition cursor-pointer border border-line"
          :title="t('branches.remote.merge.hint')"
        >
          <GitMerge class="w-3 h-3" aria-hidden="true" />
          <span>{{ t("branches.remote.merge") }}</span>
        </button>

        <button
          @click.stop="emit('rebase', b.name)"
          class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink-secondary hover:text-ink text-xs font-medium transition cursor-pointer border border-line"
          :title="t('branches.remote.rebase.hint')"
        >
          <GitPullRequest class="w-3 h-3" aria-hidden="true" />
          <span>{{ t("branches.remote.rebase") }}</span>
        </button>

        <button
          :aria-label="t('branches.remote.open.web.hint')"
          @click.stop="emit('open-web', b.name)"
          class="p-1.5 rounded-lg hover:bg-hover text-ink-muted hover:text-ink transition cursor-pointer"
          :title="t('branches.remote.open.web.hint')"
        >
          <ExternalLink class="w-3.5 h-3.5 text-accent" aria-hidden="true" />
        </button>

        <button
          :aria-label="t('branches.remote.delete.hint')"
          @click.stop="emit('delete-remote', b.name)"
          class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
          :title="t('branches.remote.delete.hint')"
        >
          <Trash2 class="w-3.5 h-3.5" aria-hidden="true" />
        </button>
      </div>

      <!-- Primary Checkout action -->
      <button
        @click.stop="emit('checkout', b.name)"
        class="flex items-center gap-1 px-3 py-1 rounded-lg bg-raised hover:bg-hover text-cyan text-xs font-medium transition cursor-pointer border border-line shrink-0"
      >
        <GitBranch class="w-3 h-3" aria-hidden="true" />
        <span>{{ t("branches.remote.checkout") }}</span>
      </button>
    </div>
  </div>

  <button
    v-if="moreCount > 0"
    @click="emit('show-more')"
    class="w-full py-2 rounded-xl border border-dashed border-line text-xs text-ink-secondary hover:text-ink hover:border-line-strong transition cursor-pointer flex items-center justify-center gap-1.5"
  >
    <ArrowDownCircle class="w-3.5 h-3.5" aria-hidden="true" />
    <span>{{ t("branches.remote.show.more", { count: moreCount }) }}</span>
  </button>

  <UiContextMenu
    :open="branchMenuOpen"
    :x="branchMenuPos.x"
    :y="branchMenuPos.y"
    :items="branchMenuList"
    @select="handleBranchMenuSelect"
    @close="closeBranchMenu"
  />
</template>
