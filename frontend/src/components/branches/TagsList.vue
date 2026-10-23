<script setup lang="ts">
import { ref, computed } from "vue";
import { Tag, Trash2, ArrowDownCircle, ArrowUpCircle, Globe, Copy, Info, ShieldCheck } from "lucide-vue-next";
import { useContextMenu, type ContextMenuItem } from "../../composables/useContextMenu";
import UiContextMenu from "../ui/UiContextMenu.vue";
import type { TagInfo } from "../../types";

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
    { id: "detail", label: "查看标签详情与验签", icon: Info },
    { id: "checkout", label: "检出该标签（分离 HEAD）", icon: Tag, separatorBefore: true },
    { id: "push", label: "推送标签到远端", icon: ArrowUpCircle },
    { id: "copy", label: "复制标签名", icon: Copy, hint: tag.name },
    { id: "verify", label: "校验签名", icon: ShieldCheck },
    { id: "delete-remote", label: "删除远端标签…", icon: Globe, danger: true, separatorBefore: true },
    { id: "delete", label: "删除本地标签…", icon: Trash2, danger: true },
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
        <div
          v-if="filteredCount === 0"
          class="py-12 text-center text-xs text-ink-muted border border-dashed border-line rounded-2xl"
        >
          {{ total === 0 ? '暂无标签。点击右上角 "新建标签" 为当前版本发布打标。' : '没有匹配的标签。' }}
        </div>

        <div
          v-if="selected.size > 0"
          class="flex items-center justify-between px-3 py-2 rounded-xl bg-raised/70 border border-line text-xs"
        >
          <div class="flex items-center gap-2">
            <span class="font-semibold text-ink">已选 {{ selected.size }} 个标签</span>
            <button @click="selected = new Set()" class="text-[10px] text-ink-muted hover:text-ink cursor-pointer">
              清空
            </button>
          </div>
          <button
            @click="emit('delete-selected', [...selected])"
            class="flex items-center gap-1 px-2.5 py-1 rounded-md bg-danger/15 hover:bg-danger/25 text-danger text-[11px] font-semibold transition cursor-pointer"
          >
            <Trash2 class="w-3 h-3" />
            删除所选
          </button>
        </div>

        <div
          v-for="t in visible"
          :key="t.name"
          :aria-label="`标签 ${t.name}`"
          class="flex items-center justify-between p-3 rounded-xl bg-surface/60 border border-line hover:border-line-strong transition"
          @contextmenu="openTagMenu(t, $event)"
        >
          <div class="flex items-center gap-3 min-w-0">
            <input
              type="checkbox"
              :checked="selected.has(t.name)"
              @change="toggleSelect(t.name)"
              class="w-3.5 h-3.5 rounded border-line accent-accent cursor-pointer shrink-0"
              :aria-label="`选择标签 ${t.name}`"
              :title="`选择标签 ${t.name}`"
            />
            <div class="w-8 h-8 rounded-lg bg-purple/10 text-purple flex items-center justify-center shrink-0">
              <Tag class="w-4 h-4" />
            </div>
            <div class="flex flex-col min-w-0">
              <div class="flex items-center gap-2">
                <span class="font-mono text-xs font-bold text-purple truncate">{{ t.name }}</span>
                <span class="font-mono text-[10px] text-ink-muted bg-raised px-1.5 py-0.2 rounded border border-line shrink-0">
                  {{ t.commitHash?.substring(0, 8) }}
                </span>
              </div>
              <span class="text-xs text-ink-secondary mt-0.5 truncate">{{ t.message || '轻量标签 (Lightweight Tag)' }}</span>
            </div>
          </div>

          <div class="flex items-center gap-2 shrink-0">
            <button
              @click="toggleAllVisible"
              class="text-[10px] text-ink-muted hover:text-ink hover:underline cursor-pointer px-1"
              :title="allVisibleSelected ? '取消选择当前列表' : '选择当前列表全部标签'"
            >
              {{ allVisibleSelected ? "取消" : "全选" }}
            </button>
            <button
              @click="emit('checkout', t.name)"
              class="px-2.5 py-1 rounded-lg bg-raised hover:bg-hover text-ink text-xs font-medium transition cursor-pointer border border-line"
            >
              检出标签
            </button>
            <button
              @click="emit('detail', t.name)"
              class="px-2.5 py-1 rounded-lg bg-surface hover:bg-raised text-ink-secondary hover:text-ink text-xs font-medium transition cursor-pointer border border-line"
              title="查看标签类型、打标者与附注"
            >
              详情
            </button>
            <button
              @click="emit('push', t.name)"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised text-accent text-xs font-medium transition cursor-pointer border border-line"
              title="推送该标签到 origin"
            >
              <ArrowUpCircle class="w-3 h-3" />
              <span>推送</span>
            </button>
            <button
              @click="emit('delete-remote', t.name)"
              class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-surface hover:bg-raised text-warning text-xs font-medium transition cursor-pointer border border-line"
              title="从 origin 删除该远程标签（保留本地）"
            >
              <Globe class="w-3 h-3" />
              <span>删远程</span>
            </button>
            <button aria-label="删除本地标签"
              @click="emit('delete', t.name)"
              class="p-1.5 rounded-lg hover:bg-danger/20 text-ink-muted hover:text-danger transition cursor-pointer"
              title="删除本地标签"
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
          <span>显示更多 (+{{ moreCount }})</span>
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
