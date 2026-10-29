<script setup lang="ts">
import { ref, computed } from "vue";
import { Tag, Trash2, ArrowDownCircle, ArrowUpCircle, Globe, Copy, Info, ShieldCheck } from "lucide-vue-next";
import { useContextMenu, type ContextMenuItem } from "../../composables/useContextMenu";
import { useRovingList } from "../../composables/useRovingList";
import UiContextMenu from "../ui/UiContextMenu.vue";
import UiEmpty from "../ui/UiEmpty.vue";
import type { TagInfo } from "../../types";
import { t } from "../../i18n";

const props = defineProps<{
  total: number;
  filteredCount: number;
  visible: TagInfo[];
  moreCount: number;
}>();

const emit = defineEmits<{
  (e: "checkout", name: string): void;
  (e: "push", name: string): void;
  (e: "detail", name: string): void;
  (e: "delete", name: string): void;
  (e: "delete-remote", name: string): void;
  (e: "delete-selected", names: string[]): void;
  (e: "show-more"): void;
}>();

const { setItemRef, tabindexFor, onKeydown: onRowKeydown, onFocus: onRowFocus } = useRovingList(
  () => props.visible,
  (tag) => tag.name
);

const {
  isOpen: tagMenuOpen,
  position: tagMenuPos,
  items: tagMenuList,
  open: openMenuAt,
  close: closeTagMenu,
} = useContextMenu();
const menuTag = ref<TagInfo | null>(null);

function tagMenuItems(tag: TagInfo): ContextMenuItem[] {
  return [
    { id: "detail", label: t("branches.tags.menu.detail"), icon: Info },
    { id: "checkout", label: t("branches.tags.menu.checkout"), icon: Tag, separatorBefore: true },
    { id: "push", label: t("branches.tags.menu.push"), icon: ArrowUpCircle },
    { id: "copy", label: t("branches.tags.menu.copy"), icon: Copy, hint: tag.name },
    { id: "verify", label: t("branches.tags.menu.verify"), icon: ShieldCheck },
    {
      id: "delete-remote",
      label: t("branches.tags.menu.delete.remote"),
      icon: Globe,
      danger: true,
      separatorBefore: true,
    },
    { id: "delete", label: t("branches.tags.menu.delete"), icon: Trash2, danger: true },
  ];
}

function openTagMenu(tag: TagInfo, event: MouseEvent) {
  menuTag.value = tag;
  openMenuAt(event, tagMenuItems(tag));
}

async function handleTagMenuSelect(id: string) {
  const tag = menuTag.value;
  if (!tag) return;
  switch (id) {
    case "detail":
    case "verify":
      emit("detail", tag.name);
      break;
    case "checkout":
      emit("checkout", tag.name);
      break;
    case "push":
      emit("push", tag.name);
      break;
    case "copy":
      await navigator.clipboard.writeText(tag.name);
      break;
    case "delete-remote":
      emit("delete-remote", tag.name);
      break;
    case "delete":
      emit("delete", tag.name);
      break;
  }
}

const selected = ref<Set<string>>(new Set());

function toggleSelect(name: string) {
  const next = new Set(selected.value);
  if (next.has(name)) next.delete(name);
  else next.add(name);
  selected.value = next;
}

const allVisibleSelected = computed(
  () => props.visible.length > 0 && props.visible.every((t) => selected.value.has(t.name))
);

function toggleAllVisible() {
  if (allVisibleSelected.value) {
    selected.value = new Set();
    return;
  }
  selected.value = new Set(props.visible.map((t) => t.name));
}
</script>

<template>
        <UiEmpty
          v-if="filteredCount === 0"
          :icon="Tag"
          :title="total === 0 ? t('branches.tags.empty') : t('branches.tags.empty.filtered')"
        />

        <div
          v-if="selected.size > 0"
          class="flex items-center justify-between px-3 py-2 rounded-xl bg-raised/70 border border-line text-xs"
        >
          <div class="flex items-center gap-2">
            <span class="font-semibold text-ink">{{ t("branches.tags.selected", { count: selected.size }) }}</span>
            <button @click="selected = new Set()" class="text-[10px] text-ink-muted hover:text-ink cursor-pointer">
              {{ t("branches.tags.clear") }}
            </button>
          </div>
          <button
            @click="emit('delete-selected', [...selected])"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-danger/15 hover:bg-danger/25 text-danger text-[11px] font-semibold transition cursor-pointer"
          >
            <Trash2 class="w-3 h-3" />
            {{ t("branches.tags.delete.selected") }}
          </button>
        </div>

        <div
          v-for="(tag, index) in visible"
          :key="tag.name"
          :ref="(el) => setItemRef(tag, el)"
          :tabindex="tabindexFor(tag, index)"
          :aria-label="t('branches.tags.row.aria', { name: tag.name })"
          class="flex items-center justify-between p-3 rounded-xl bg-surface/60 border border-line hover:border-line-strong transition focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
          @focus="onRowFocus(tag)"
          @keydown="onRowKeydown($event, index)"
          @keydown.enter.self="emit('detail', tag.name)"
          @contextmenu="openTagMenu(tag, $event)"
        >
          <div class="flex items-center gap-3 min-w-0">
            <input
              type="checkbox"
              :checked="selected.has(tag.name)"
              @change="toggleSelect(tag.name)"
              class="w-3.5 h-3.5 rounded border-line accent-accent cursor-pointer shrink-0"
              :aria-label="t('branches.tags.select.aria', { name: tag.name })"
              :title="t('branches.tags.select.aria', { name: tag.name })"
            />
            <div class="w-8 h-8 rounded-lg bg-purple/10 text-purple flex items-center justify-center shrink-0">
              <Tag class="w-4 h-4" />
            </div>
            <div class="flex flex-col min-w-0">
              <div class="flex items-center gap-2">
                <span class="font-mono text-xs font-bold text-purple truncate">{{ tag.name }}</span>
                <span class="font-mono text-[10px] text-ink-muted bg-raised px-1.5 py-0.2 rounded border border-line shrink-0">
                  {{ tag.commitHash?.substring(0, 8) }}
                </span>
              </div>
              <span class="text-xs text-ink-secondary mt-0.5 truncate">{{
                tag.message || t("branches.tags.lightweight")
              }}</span>
            </div>
          </div>

          <div class="flex items-center gap-2 shrink-0">
            <button
              @click="toggleAllVisible"
              class="text-[10px] text-ink-muted hover:text-ink hover:underline cursor-pointer px-1"
              :title="
                allVisibleSelected
                  ? t('branches.tags.deselect.all.hint')
                  : t('branches.tags.select.all.hint')
              "
            >
              {{ allVisibleSelected ? t("branches.tags.deselect.all") : t("branches.tags.select.all") }}
            </button>
            <button
              @click="emit('checkout', tag.name)"
              class="px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
            >
              {{ t("branches.tags.checkout") }}
            </button>
            <button
              @click="emit('detail', tag.name)"
              class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised text-ink-secondary hover:text-ink text-xs font-medium transition cursor-pointer border border-line"
              :title="t('branches.tags.detail.hint')"
            >
              {{ t("branches.tags.detail") }}
            </button>
            <button
              @click="emit('push', tag.name)"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised text-accent text-xs font-medium transition cursor-pointer border border-line"
              :title="t('branches.tags.push.hint')"
            >
              <ArrowUpCircle class="w-3 h-3" />
              <span>{{ t("branches.tags.push") }}</span>
            </button>
            <button
              @click="emit('delete-remote', tag.name)"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised text-warning text-xs font-medium transition cursor-pointer border border-line"
              :title="t('branches.tags.delete.remote.hint')"
            >
              <Globe class="w-3 h-3" />
              <span>{{ t("branches.tags.delete.remote") }}</span>
            </button>
            <button :aria-label="t('branches.tags.delete.hint')"
              @click="emit('delete', tag.name)"
              class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
              :title="t('branches.tags.delete.hint')"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <button
          v-if="moreCount > 0"
          @click="emit('show-more')"
          class="w-full py-2 rounded-xl border border-dashed border-line text-xs text-ink-secondary hover:text-ink hover:border-line-strong transition cursor-pointer flex items-center justify-center gap-1.5"
        >
          <ArrowDownCircle class="w-3.5 h-3.5" />
          <span>{{ t("branches.tags.show.more", { count: moreCount }) }}</span>
        </button>

    <UiContextMenu
      :open="tagMenuOpen"
      :x="tagMenuPos.x"
      :y="tagMenuPos.y"
      :items="tagMenuList"
      @select="handleTagMenuSelect"
      @close="closeTagMenu"
    />
</template>
