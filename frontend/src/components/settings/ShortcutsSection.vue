<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { AlertTriangle, Check, Copy, Keyboard, Pencil, RotateCcw, ShieldCheck, Upload, X, Zap } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import type { GitVersionInfo } from "../../types";
import {
  SHORTCUT_ACTIONS,
  bindingToString,
  effectiveBinding,
  eventToBinding,
  findShortcutConflict,
  formatBinding,
  parseBinding,
  loadShortcutOverrides,
  saveShortcutOverrides,
} from "../../utils/shortcuts";
import type { ShortcutOverrides } from "../../utils/shortcuts";

const gitVersion = ref<GitVersionInfo | null>(null);

const overrides = ref(loadShortcutOverrides());
const recordingId = ref<string | null>(null);
const livePreview = ref("");
const conflictActionId = ref<string | null>(null);
const statusMessage = ref("");
const statusKind = ref<"ok" | "error">("ok");
const showTransferPanel = ref(false);
const transferMode = ref<"export" | "import">("export");
const transferText = ref("");
const transferError = ref("");

const customizableActions = computed(() => SHORTCUT_ACTIONS.filter((a) => a.customizable));
const fixedActions = computed(() => SHORTCUT_ACTIONS.filter((a) => !a.customizable));
const hasCustomizations = computed(() => Object.keys(overrides.value).length > 0);

function bindingChips(binding: string): string[] {
  return formatBinding(binding).split(" ");
}

function isRecording(id: string): boolean {
  return recordingId.value === id;
}

function isCustomized(id: string): boolean {
  return !!overrides.value[id];
}

function startRecording(id: string) {
  recordingId.value = id;
  livePreview.value = "";
  conflictActionId.value = null;
  statusMessage.value = "按下新的组合键（需包含 ⌘ / ⌥ / ⇧ 之一，Esc 取消）";
  statusKind.value = "ok";
}

function cancelRecording() {
  if (!recordingId.value) return;
  recordingId.value = null;
  livePreview.value = "";
  statusMessage.value = "已取消录制";
  statusKind.value = "ok";
}

/** Recompute a live modifier preview while the user is still holding keys. */
function updateLivePreview(event: KeyboardEvent) {
  const mods: string[] = [];
  if (event.metaKey || event.ctrlKey) mods.push("⌘");
  if (event.shiftKey) mods.push("⇧");
  if (event.altKey) mods.push("⌥");
  livePreview.value = mods.length > 0 ? `${mods.join(" ")} …` : "";
}

/** Validate a candidate override map: parseable, unique, no unknown actions. */
function validateOverrideConfig(candidate: ShortcutOverrides): string[] {
  const problems: string[] = [];
  const seen = new Map<string, string>();
  for (const action of SHORTCUT_ACTIONS) {
    const binding = candidate[action.id] || action.defaultBinding;
    const parsed = parseBinding(binding);
    if (!parsed) {
      problems.push(`「${action.label}」绑定无法解析: ${binding}`);
      continue;
    }
    const normalized = binding.toUpperCase();
    const existing = seen.get(normalized);
    if (existing) problems.push(`「${action.label}」与「${existing}」绑定重复: ${formatBinding(binding)}`);
    else seen.set(normalized, action.label);
  }
  for (const [actionId, binding] of Object.entries(candidate)) {
    const action = SHORTCUT_ACTIONS.find((a) => a.id === actionId);
    if (!action) {
      problems.push(`未知动作的自定义绑定: ${actionId}`);
      continue;
    }
    if (!action.customizable) problems.push(`「${action.label}」不可自定义: ${actionId}`);
    if (!parseBinding(binding)) problems.push(`自定义绑定无法解析: ${actionId}=${binding}`);
  }
  return problems;
}

/** Validate every effective binding: parseable, unique, complete. */
function runSelfCheck() {
  const problems = validateOverrideConfig(overrides.value);
  if (problems.length > 0) {
    statusKind.value = "error";
    statusMessage.value = `快捷键自检发现 ${problems.length} 个问题：${problems.slice(0, 2).join("；")}`;
    return;
  }
  statusKind.value = "ok";
  statusMessage.value = `快捷键自检通过：${SHORTCUT_ACTIONS.length} 个动作绑定有效且无冲突`;
}

// ---- Import / export shortcut configuration (JSON) ----
function openExportPanel() {
  transferMode.value = "export";
  transferError.value = "";
  transferText.value = JSON.stringify({ version: 1, overrides: overrides.value }, null, 2);
  showTransferPanel.value = true;
}

function openImportPanel() {
  transferMode.value = "import";
  transferError.value = "";
  transferText.value = "";
  showTransferPanel.value = true;
}

function closeTransferPanel() {
  showTransferPanel.value = false;
  transferError.value = "";
}

async function copyTransferText() {
  try {
    await navigator.clipboard.writeText(transferText.value);
    statusKind.value = "ok";
    statusMessage.value = "快捷键配置已复制到剪贴板";
  } catch {
    statusKind.value = "error";
    statusMessage.value = "复制失败：剪贴板不可用，请手动选择文本复制";
  }
}

function importTransferConfig() {
  transferError.value = "";
  let parsed: any;
  try {
    parsed = JSON.parse(transferText.value);
  } catch {
    transferError.value = "JSON 解析失败，请检查格式";
    return;
  }
  const candidate: ShortcutOverrides = {};
  const source = parsed && typeof parsed === "object" && parsed.overrides ? parsed.overrides : parsed;
  if (!source || typeof source !== "object" || Array.isArray(source)) {
    transferError.value = "配置格式无效：需要 { overrides: { actionId: binding } } 或 { actionId: binding }";
    return;
  }
  for (const [actionId, binding] of Object.entries(source)) {
    if (typeof binding !== "string") {
      transferError.value = `动作 ${actionId} 的绑定必须是字符串`;
      return;
    }
    candidate[actionId] = binding;
  }
  const problems = validateOverrideConfig(candidate);
  if (problems.length > 0) {
    transferError.value = `导入失败：${problems.slice(0, 2).join("；")}`;
    return;
  }
  overrides.value = candidate;
  saveShortcutOverrides(candidate);
  showTransferPanel.value = false;
  statusKind.value = "ok";
  statusMessage.value = `已导入快捷键配置（${Object.keys(candidate).length} 个自定义绑定）`;
}

function resetOne(id: string) {
  conflictActionId.value = null;
  const next = { ...overrides.value };
  delete next[id];
  overrides.value = next;
  saveShortcutOverrides(next);
  statusMessage.value = `已恢复默认快捷键：${SHORTCUT_ACTIONS.find((a) => a.id === id)?.label ?? id}`;
  statusKind.value = "ok";
}

function resetAll() {
  conflictActionId.value = null;
  overrides.value = {};
  saveShortcutOverrides({});
  statusMessage.value = "已恢复全部默认快捷键";
  statusKind.value = "ok";
}

function handleRecordKeydown(event: KeyboardEvent) {
  if (!recordingId.value) return;
  event.preventDefault();
  event.stopImmediatePropagation();

  if (event.key === "Escape") {
    cancelRecording();
    return;
  }

  // Modifier-only presses update the live preview chips instead of committing.
  if (["Meta", "Control", "Alt", "Shift"].includes(event.key)) {
    updateLivePreview(event);
    return;
  }

  const parsed = eventToBinding(event);
  if (!parsed) {
    statusMessage.value = "请按下包含至少一个修饰键（⌘ / ⌥ / ⇧）的组合键";
    statusKind.value = "error";
    return;
  }
  const binding = bindingToString(parsed);
  const action = SHORTCUT_ACTIONS.find((a) => a.id === recordingId.value);
  if (!action) {
    recordingId.value = null;
    return;
  }
  const conflict = findShortcutConflict(binding, overrides.value, action.id);
  if (conflict) {
    conflictActionId.value = conflict.id;
    livePreview.value = "";
    statusMessage.value = `快捷键冲突：「${formatBinding(binding)}」已被「${conflict.label}」占用`;
    statusKind.value = "error";
    return;
  }

  const next = { ...overrides.value };
  if (binding === action.defaultBinding) delete next[action.id];
  else next[action.id] = binding;
  overrides.value = next;
  saveShortcutOverrides(next);
  recordingId.value = null;
  livePreview.value = "";
  conflictActionId.value = null;
  statusMessage.value = `已保存：「${action.label}」→ ${formatBinding(binding)}`;
  statusKind.value = "ok";
}

onMounted(async () => {
  window.addEventListener("keydown", handleRecordKeydown, true);
  try {
    gitVersion.value = await gitService.getGitVersion();
  } catch (err) {
    console.debug("Failed to load git version:", err);
  }
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleRecordKeydown, true);
});
</script>

<template>
  <section class="space-y-6">
    <div>
      <h2 class="text-sm font-bold text-ink">快捷键与客户端架构</h2>
      <p class="text-xs text-ink-secondary mt-1">
        全部 9 个快捷键均可录制替换（含文件跳转、代码搜索、差分切换、源码树展开与提交），录制后立即生效并持久化。
      </p>
    </div>

    <div class="p-5 rounded-2xl bg-surface border border-line space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-xs font-bold text-ink flex items-center gap-2">
          <Keyboard class="w-3.5 h-3.5 text-accent" />
          可自定义快捷键
        </h3>
        <div class="flex flex-wrap items-center gap-2">
          <button
            @click="openExportPanel"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer"
            title="导出快捷键配置 JSON"
          >
            <Copy class="w-3 h-3" /> 导出
          </button>
          <button
            @click="openImportPanel"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer"
            title="从 JSON 导入快捷键配置"
          >
            <Upload class="w-3 h-3" /> 导入
          </button>
          <button
            @click="runSelfCheck"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer"
            title="校验全部快捷键绑定是否有效且无冲突"
          >
            <ShieldCheck class="w-3 h-3" /> 自检配置
          </button>
          <button
            v-if="hasCustomizations"
            @click="resetAll"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-muted hover:text-ink transition cursor-pointer"
          >
            <RotateCcw class="w-3 h-3" /> 全部恢复默认
          </button>
        </div>
      </div>

      <div
        v-for="action in customizableActions"
        :key="action.id"
        class="flex flex-wrap items-center gap-2 py-2 border-b border-line last:border-b-0"
        :class="[
          isRecording(action.id) ? 'bg-accent/5 rounded-lg px-2' : '',
          conflictActionId === action.id ? 'ring-1 ring-danger/40 rounded-lg px-2' : '',
        ]"
      >
        <div class="min-w-0 flex-1">
          <div class="text-xs text-ink font-medium truncate">{{ action.label }}</div>
          <div class="text-[10px] text-ink-muted truncate">{{ action.description }}</div>
        </div>

        <div v-if="isRecording(action.id)" class="flex items-center gap-2 text-[11px] text-accent shrink-0">
          <span class="w-1.5 h-1.5 rounded-full bg-accent animate-pulse"></span>
          <span>按下新的组合键…</span>
          <kbd
            v-if="livePreview"
            class="px-1.5 py-0.5 rounded bg-raised border border-accent/40 font-mono text-[10px] font-bold text-accent"
          >
            {{ livePreview }}
          </kbd>
          <button
            @click="cancelRecording"
            class="px-1.5 py-0.5 rounded border border-line bg-surface text-[10px] text-ink-muted hover:text-ink transition cursor-pointer"
          >
            Esc 取消
          </button>
        </div>
        <template v-else>
          <div class="flex items-center gap-1 shrink-0">
            <kbd
              v-for="chip in bindingChips(effectiveBinding(action, overrides))"
              :key="chip"
              class="px-1.5 py-0.5 rounded bg-raised border border-line font-mono text-[10px] font-bold"
              :class="isCustomized(action.id) ? 'text-accent' : 'text-ink'"
            >
              {{ chip }}
            </kbd>
            <span v-if="isCustomized(action.id)" class="text-[9px] text-accent/80 font-semibold">已自定义</span>
          </div>
          <button
            @click="startRecording(action.id)"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
            title="录制新的组合键"
          >
            <Pencil class="w-3 h-3" /> 录制
          </button>
          <button aria-label="恢复该动作默认快捷键"
            v-if="isCustomized(action.id)"
            @click="resetOne(action.id)"
            class="p-1 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
            title="恢复该动作默认快捷键"
          >
            <RotateCcw class="w-3 h-3" />
          </button>
        </template>
      </div>

      <p
        v-if="statusMessage"
        class="text-[10px] leading-relaxed flex items-start gap-1.5"
        :class="statusKind === 'error' ? 'text-danger' : 'text-success'"
      >
        <AlertTriangle v-if="statusKind === 'error'" class="w-3 h-3 mt-0.5 shrink-0" />
        <Check v-else class="w-3 h-3 mt-0.5 shrink-0" />
        <span>{{ statusMessage }}</span>
      </p>

      <div v-if="showTransferPanel" class="rounded-xl border border-line bg-canvas/40 p-2.5 space-y-2">
        <div class="flex items-center justify-between gap-2">
          <span class="text-[10px] font-semibold text-ink-muted">
            {{ transferMode === "export" ? "导出快捷键配置（JSON）" : "粘贴配置 JSON 后导入" }}
          </span>
          <button aria-label="关闭"
            @click="closeTransferPanel"
            class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
            title="关闭"
          >
            <X class="w-3 h-3" />
          </button>
        </div>
        <textarea
          v-model="transferText"
          :readonly="transferMode === 'export'"
          rows="6"
          spellcheck="false"
          class="w-full resize-y bg-surface border border-line rounded-lg px-2.5 py-2 text-[10px] font-mono text-ink focus:outline-none focus:border-accent"
          :placeholder="transferMode === 'import' ? '{ &quot;overrides&quot;: { &quot;open-repo&quot;: &quot;Meta+Shift+U&quot; } }' : ''"
        ></textarea>
        <p v-if="transferError" class="text-[10px] text-danger leading-relaxed">{{ transferError }}</p>
        <div class="flex items-center justify-end gap-2">
          <button
            v-if="transferMode === 'export'"
            @click="copyTransferText"
            class="flex items-center gap-1 px-2 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-[10px] font-semibold transition cursor-pointer"
          >
            <Copy class="w-3 h-3" /> 复制到剪贴板
          </button>
          <button
            v-else
            @click="importTransferConfig"
            :disabled="!transferText.trim()"
            class="flex items-center gap-1 px-2 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-white text-[10px] font-semibold transition cursor-pointer disabled:opacity-40"
          >
            <Upload class="w-3 h-3" /> 导入配置
          </button>
          <button
            @click="closeTransferPanel"
            class="px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] text-ink-muted transition cursor-pointer"
          >
            关闭
          </button>
        </div>
      </div>
    </div>

    <div v-if="fixedActions.length > 0" class="p-5 rounded-2xl bg-surface border border-line space-y-2">
      <h3 class="text-xs font-bold text-ink flex items-center gap-2">
        <Zap class="w-3.5 h-3.5 text-success" />
        组件内固定快捷键（暂不可自定义）
      </h3>
      <div
        v-for="action in fixedActions"
        :key="action.id"
        class="flex items-center justify-between gap-3 py-1.5 border-b border-line last:border-b-0"
      >
        <div class="min-w-0">
          <span class="text-xs text-ink-secondary">{{ action.label }}</span>
          <span class="ml-2 text-[9px] text-ink-muted/70">固定</span>
        </div>
        <div class="flex items-center gap-1 shrink-0">
          <kbd
            v-for="chip in bindingChips(action.defaultBinding)"
            :key="chip"
            class="px-1.5 py-0.5 rounded bg-raised border border-line font-mono text-[10px] font-bold text-ink"
          >
            {{ chip }}
          </kbd>
        </div>
      </div>
    </div>

    <div class="p-5 rounded-2xl bg-accent/5 border border-accent/20 flex flex-col gap-2">
      <div class="flex items-center gap-2 text-accent font-bold text-xs">
        <Zap class="w-4 h-4" />
        <span>通用极致工程客户端架构</span>
      </div>
      <p class="text-xs text-ink-secondary leading-relaxed">
        基于 Wails v3 + Go + Vite + Vue 3 + TypeScript 纯原生架构构建。打造集极致性能、代码比对、分支合并、储藏快照与效能工具于一体的现代化通用 Git 桌面客户端。
      </p>
      <div class="flex items-center justify-between gap-3 pt-2 border-t border-accent/20 text-xs">
        <span class="text-ink-secondary">Git 运行时版本</span>
        <code class="font-mono text-[10px] text-ink truncate">{{ gitVersion?.version || "检测中..." }}</code>
      </div>
    </div>
  </section>
</template>
