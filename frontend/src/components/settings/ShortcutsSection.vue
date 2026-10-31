<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { AlertTriangle, Check, Copy, Keyboard, Pencil, RotateCcw, ShieldCheck, Upload, X, Zap } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import { t } from "../../i18n";
import type { GitVersionInfo } from "../../types";
import { isVimModeEnabled, toggleVimMode } from "../../utils/vimNavigation";
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
  shortcutDescription,
  shortcutLabel,
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
  statusMessage.value = t("settings.shortcuts.status.recording");
  statusKind.value = "ok";
}

function cancelRecording() {
  if (!recordingId.value) return;
  recordingId.value = null;
  livePreview.value = "";
  statusMessage.value = t("settings.shortcuts.status.recording.canceled");
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
      problems.push(t("settings.shortcuts.problem.unparsable", { action: shortcutLabel(action), binding }));
      continue;
    }
    const normalized = binding.toUpperCase();
    const existing = seen.get(normalized);
    if (existing)
      problems.push(
        t("settings.shortcuts.problem.duplicate", {
          action: shortcutLabel(action),
          other: existing,
          binding: formatBinding(binding),
        })
      );
    else seen.set(normalized, shortcutLabel(action));
  }
  for (const [actionId, binding] of Object.entries(candidate)) {
    const action = SHORTCUT_ACTIONS.find((a) => a.id === actionId);
    if (!action) {
      problems.push(t("settings.shortcuts.problem.unknown.action", { id: actionId }));
      continue;
    }
    if (!action.customizable)
      problems.push(t("settings.shortcuts.problem.not.customizable", { action: shortcutLabel(action), id: actionId }));
    if (!parseBinding(binding))
      problems.push(t("settings.shortcuts.problem.custom.unparsable", { id: actionId, binding }));
  }
  return problems;
}

/** Validate every effective binding: parseable, unique, complete. */
function runSelfCheck() {
  const problems = validateOverrideConfig(overrides.value);
  if (problems.length > 0) {
    statusKind.value = "error";
    statusMessage.value = t("settings.shortcuts.status.selfcheck.failed", {
      count: problems.length,
      detail: problems.slice(0, 2).join(t("common.list.semicolon")),
    });
    return;
  }
  statusKind.value = "ok";
  statusMessage.value = t("settings.shortcuts.status.selfcheck.ok", { count: SHORTCUT_ACTIONS.length });
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
    statusMessage.value = t("settings.shortcuts.status.copied");
  } catch {
    statusKind.value = "error";
    statusMessage.value = t("settings.shortcuts.status.copy.failed");
  }
}

function importTransferConfig() {
  transferError.value = "";
  let parsed: any;
  try {
    parsed = JSON.parse(transferText.value);
  } catch {
    transferError.value = t("settings.shortcuts.error.json.parse");
    return;
  }
  const candidate: ShortcutOverrides = {};
  const source = parsed && typeof parsed === "object" && parsed.overrides ? parsed.overrides : parsed;
  if (!source || typeof source !== "object" || Array.isArray(source)) {
    transferError.value = t("settings.shortcuts.error.json.shape");
    return;
  }
  for (const [actionId, binding] of Object.entries(source)) {
    if (typeof binding !== "string") {
      transferError.value = t("settings.shortcuts.error.binding.type", { id: actionId });
      return;
    }
    candidate[actionId] = binding;
  }
  const problems = validateOverrideConfig(candidate);
  if (problems.length > 0) {
    transferError.value = t("settings.shortcuts.error.import", {
      detail: problems.slice(0, 2).join(t("common.list.semicolon")),
    });
    return;
  }
  overrides.value = candidate;
  saveShortcutOverrides(candidate);
  showTransferPanel.value = false;
  statusKind.value = "ok";
  statusMessage.value = t("settings.shortcuts.status.imported", { count: Object.keys(candidate).length });
}

function resetOne(id: string) {
  conflictActionId.value = null;
  const next = { ...overrides.value };
  delete next[id];
  overrides.value = next;
  saveShortcutOverrides(next);
  const target = SHORTCUT_ACTIONS.find((a) => a.id === id);
  statusMessage.value = t("settings.shortcuts.status.reset.one", {
    action: target ? shortcutLabel(target) : id,
  });
  statusKind.value = "ok";
}

function resetAll() {
  conflictActionId.value = null;
  overrides.value = {};
  saveShortcutOverrides({});
  statusMessage.value = t("settings.shortcuts.status.reset.all");
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
    statusMessage.value = t("settings.shortcuts.status.need.modifier");
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
    statusMessage.value = t("settings.shortcuts.status.conflict", {
      binding: formatBinding(binding),
      action: shortcutLabel(conflict),
    });
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
  statusMessage.value = t("settings.shortcuts.status.saved", {
    action: shortcutLabel(action),
    binding: formatBinding(binding),
  });
  statusKind.value = "ok";
}

onMounted(async () => {
  window.addEventListener("keydown", handleRecordKeydown, true);
  try {
    gitVersion.value = await gitService.getGitVersion();
  } catch (err) {
    // best-effort: the version badge hides when unknown.
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
      <h2 class="text-sm font-bold text-ink">{{ t("settings.shortcuts.title") }}</h2>
      <p class="text-xs text-ink-secondary mt-1">
        {{ t("settings.shortcuts.intro") }}
      </p>
    </div>

    <!-- Vim Geek Mode Configuration Card -->
    <div class="p-4 rounded-2xl bg-surface border border-line flex items-center justify-between gap-4">
      <div class="flex items-start gap-3 min-w-0">
        <div class="w-9 h-9 rounded-xl bg-accent/10 border border-accent/20 flex items-center justify-center text-accent font-mono font-black text-sm shrink-0">
          V
        </div>
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h3 class="text-xs font-bold text-ink">{{ t("settings.shortcuts.vim.title") }}</h3>
            <span
              class="px-1.5 py-0.5 rounded text-[9px] font-mono font-bold"
              :class="isVimModeEnabled ? 'bg-accent/20 text-accent border border-accent/30' : 'bg-canvas text-ink-muted border border-line'"
            >
              {{ isVimModeEnabled ? t("settings.shortcuts.vim.enabled") : t("settings.shortcuts.vim.disabled") }}
            </span>
          </div>
          <p class="text-[11px] text-ink-muted mt-0.5 leading-relaxed">
            {{ t("settings.shortcuts.vim.desc") }}
          </p>
          <div class="flex flex-wrap items-center gap-1.5 mt-2 text-[10px] text-ink-secondary font-mono">
            <span class="px-1.5 py-0.5 rounded bg-raised border border-line">j/k: {{ t("settings.shortcuts.vim.keys.jk") }}</span>
            <span class="px-1.5 py-0.5 rounded bg-raised border border-line">x/Space: {{ t("settings.shortcuts.vim.keys.stage") }}</span>
            <span class="px-1.5 py-0.5 rounded bg-raised border border-line">s: {{ t("settings.shortcuts.vim.keys.hunk") }}</span>
            <span class="px-1.5 py-0.5 rounded bg-raised border border-line">d: {{ t("settings.shortcuts.vim.keys.discard") }}</span>
            <span class="px-1.5 py-0.5 rounded bg-raised border border-line">c: {{ t("settings.shortcuts.vim.keys.commit") }}</span>
            <span class="px-1.5 py-0.5 rounded bg-raised border border-line">gt/gT: {{ t("settings.shortcuts.vim.keys.tab") }}</span>
            <span class="px-1.5 py-0.5 rounded bg-raised border border-line">Esc: {{ t("settings.shortcuts.vim.keys.esc") }}</span>
          </div>
        </div>
      </div>
      <button
        type="button"
        role="switch"
        :aria-checked="isVimModeEnabled"
        @click="toggleVimMode"
        class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent/50"
        :class="isVimModeEnabled ? 'bg-accent' : 'bg-line'"
      >
        <span
          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out"
          :class="isVimModeEnabled ? 'translate-x-5' : 'translate-x-0'"
        />
      </button>
    </div>

    <div class="p-5 rounded-2xl bg-surface border border-line space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-xs font-bold text-ink flex items-center gap-2">
          <Keyboard class="w-3.5 h-3.5 text-accent" />
          {{ t("settings.shortcuts.customizable") }}
        </h3>
        <div class="flex flex-wrap items-center gap-2">
          <button
            @click="openExportPanel"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer"
            :title="t('settings.shortcuts.export.hint')"
          >
            <Copy class="w-3 h-3" aria-hidden="true" /> {{ t("settings.shortcuts.export") }}
          </button>
          <button
            @click="openImportPanel"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer"
            :title="t('settings.shortcuts.import.hint')"
          >
            <Upload class="w-3 h-3" aria-hidden="true" /> {{ t("settings.shortcuts.import") }}
          </button>
          <button
            @click="runSelfCheck"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer"
            :title="t('settings.shortcuts.selfcheck.hint')"
          >
            <ShieldCheck class="w-3 h-3" aria-hidden="true" /> {{ t("settings.shortcuts.selfcheck") }}
          </button>
          <button
            v-if="hasCustomizations"
            @click="resetAll"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-muted hover:text-ink transition cursor-pointer"
          >
            <RotateCcw class="w-3 h-3" aria-hidden="true" /> {{ t("settings.shortcuts.reset.all") }}
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
          <div class="text-xs text-ink font-medium truncate">{{ shortcutLabel(action) }}</div>
          <div class="text-[10px] text-ink-muted truncate">{{ shortcutDescription(action) }}</div>
        </div>

        <div v-if="isRecording(action.id)" class="flex items-center gap-2 text-[11px] text-accent shrink-0">
          <span class="w-1.5 h-1.5 rounded-full bg-accent animate-pulse"></span>
          <span>{{ t("settings.shortcuts.recording") }}</span>
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
            {{ t("settings.shortcuts.recording.cancel") }}
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
            <span v-if="isCustomized(action.id)" class="text-[9px] text-accent/80 font-semibold">{{
              t("settings.shortcuts.customized")
            }}</span>
          </div>
          <button
            @click="startRecording(action.id)"
            class="flex items-center gap-1 px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] font-semibold text-ink-secondary hover:text-ink transition cursor-pointer shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
            :title="t('settings.shortcuts.record.hint')"
          >
            <Pencil class="w-3 h-3" aria-hidden="true" /> {{ t("settings.shortcuts.record") }}
          </button>
          <button :aria-label="t('settings.shortcuts.reset.one.hint')"
            v-if="isCustomized(action.id)"
            @click="resetOne(action.id)"
            class="p-1 rounded-lg border border-line bg-surface hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
            :title="t('settings.shortcuts.reset.one.hint')"
          >
            <RotateCcw class="w-3 h-3" aria-hidden="true" />
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
            {{
              transferMode === "export"
                ? t("settings.shortcuts.transfer.export.title")
                : t("settings.shortcuts.transfer.import.title")
            }}
          </span>
          <button :aria-label="t('settings.shortcuts.close')"
            @click="closeTransferPanel"
            class="p-1 rounded hover:bg-raised text-ink-muted hover:text-ink transition cursor-pointer"
            :title="t('settings.shortcuts.close')"
          >
            <X class="w-3 h-3" aria-hidden="true" />
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
            class="flex items-center gap-1 px-2 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-[10px] font-semibold transition cursor-pointer"
          >
            <Copy class="w-3 h-3" aria-hidden="true" /> {{ t("settings.shortcuts.transfer.copy") }}
          </button>
          <button
            v-else
            @click="importTransferConfig"
            :disabled="!transferText.trim()"
            class="flex items-center gap-1 px-2 py-1 rounded-lg bg-accent-strong hover:bg-accent-strong-hover text-accent-contrast text-[10px] font-semibold transition cursor-pointer disabled:opacity-40"
          >
            <Upload class="w-3 h-3" aria-hidden="true" /> {{ t("settings.shortcuts.transfer.import") }}
          </button>
          <button
            @click="closeTransferPanel"
            class="px-2 py-1 rounded-lg border border-line bg-surface hover:bg-raised text-[10px] text-ink-muted transition cursor-pointer"
          >
            {{ t("settings.shortcuts.close") }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="fixedActions.length > 0" class="p-5 rounded-2xl bg-surface border border-line space-y-2">
      <h3 class="text-xs font-bold text-ink flex items-center gap-2">
        <Zap class="w-3.5 h-3.5 text-success" />
        {{ t("settings.shortcuts.fixed.title") }}
      </h3>
      <div
        v-for="action in fixedActions"
        :key="action.id"
        class="flex items-center justify-between gap-3 py-1.5 border-b border-line last:border-b-0"
      >
        <div class="min-w-0">
          <span class="text-xs text-ink-secondary">{{ shortcutLabel(action) }}</span>
          <span class="ml-2 text-[9px] text-ink-muted">{{ t("settings.shortcuts.fixed.badge") }}</span>
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
        <span>{{ t("settings.shortcuts.arch.title") }}</span>
      </div>
      <p class="text-xs text-ink-secondary leading-relaxed">
        {{ t("settings.shortcuts.arch.body") }}
      </p>
      <div class="flex items-center justify-between gap-3 pt-2 border-t border-accent/20 text-xs">
        <span class="text-ink-secondary">{{ t("settings.shortcuts.arch.git.version") }}</span>
        <code class="font-mono text-[10px] text-ink truncate">{{
          gitVersion?.version || t("settings.shortcuts.arch.git.detecting")
        }}</code>
      </div>
    </div>
  </section>
</template>
