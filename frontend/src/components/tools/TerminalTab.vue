<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { Events } from "@wailsio/runtime";
import "@xterm/xterm/css/xterm.css";
import { Play, Square, Eraser, Terminal as TerminalIcon } from "lucide-vue-next";
import { gitService } from "../../services/gitService";
import UiButton from "../ui/UiButton.vue";
import { t } from "../../i18n";

const props = defineProps<{
  currentRepoPath: string;
  refreshNonce: number;
}>();

const emit = defineEmits<{ (e: "notify", msg: string): void }>();

const hostRef = ref<HTMLDivElement | null>(null);
const isRunning = ref(false);
const statusText = ref("");
const isBusy = ref(false);

let term: XTerm | null = null;
let fitAddon: FitAddon | null = null;
let offOutput: (() => void) | null = null;
let observer: ResizeObserver | null = null;

/** Pull the live design tokens so the terminal matches the active theme. */
function readTheme() {
  let style: CSSStyleDeclaration | null = null;
  try {
    style = getComputedStyle(document.documentElement);
  } catch {
    style = null;
  }
  const token = (name: string, fallback: string) => style?.getPropertyValue(name).trim() || fallback;
  return {
    background: token("--ug-bg-canvas", "#111113"),
    foreground: token("--ug-text-primary", "#e3e4e8"),
    cursor: token("--ug-accent", "#93b4ff"),
    cursorAccent: token("--ug-bg-canvas", "#111113"),
    selectionBackground: token("--ug-accent-border", "#375098"),
  };
}

function mountTerminal() {
  if (!hostRef.value || term) return;

  term = new XTerm({
    fontSize: 12,
    fontFamily: '"SF Mono", "JetBrains Mono", Menlo, Consolas, monospace',
    cursorBlink: true,
    scrollback: 5000,
    theme: readTheme(),
    allowProposedApi: true,
  });
  fitAddon = new FitAddon();
  term.loadAddon(fitAddon);
  term.open(hostRef.value);

  try {
    fitAddon.fit();
  } catch {
    /* the host may not be laid out yet */
  }

  term.onData((data) => {
    // Keystrokes are fire-and-forget: a failed write means the session died,
    // which the status poll picks up.
    void gitService.writeTerminal(data).catch(() => {});
  });

  term.onResize(({ cols, rows }) => {
    void gitService.resizeTerminal(cols, rows).catch(() => {});
  });
}

function refit() {
  try {
    fitAddon?.fit();
  } catch {
    /* ignore transient layout errors */
  }
}

async function refreshStatus() {
  try {
    const status = await gitService.getTerminalStatus();
    isRunning.value = status.running;
    statusText.value = status.running
      ? `${status.shell} · ${status.dir} · ${status.cols}×${status.rows}`
      : status.message || "";
  } catch {
    isRunning.value = false;
  }
}

async function startTerminal() {
  if (isBusy.value) return;
  isBusy.value = true;
  mountTerminal();
  refit();
  try {
    const status = await gitService.startTerminal(
      props.currentRepoPath,
      term?.cols || 100,
      term?.rows || 30
    );
    isRunning.value = status.running;
    statusText.value = `${status.shell} · ${status.dir}`;
    term?.clear();
    if (status.backlog) term?.write(status.backlog);
    term?.focus();
    refit();
    emit("notify", t("terminal.toast.started", { shell: status.shell }));
  } catch (err: any) {
    emit("notify", t("terminal.toast.start.failed", { error: err?.message || err }));
  } finally {
    isBusy.value = false;
  }
}

async function stopTerminal() {
  if (isBusy.value) return;
  isBusy.value = true;
  try {
    await gitService.stopTerminal();
    isRunning.value = false;
    statusText.value = t("terminal.session.ended");
    term?.write(`\r\n\x1b[2m${t("terminal.output.stopped")}\x1b[0m\r\n`);
  } catch (err: any) {
    emit("notify", t("terminal.toast.stop.failed", { error: err?.message || err }));
  } finally {
    isBusy.value = false;
  }
}

function clearScreen() {
  term?.clear();
  term?.focus();
}

onMounted(() => {
  mountTerminal();

  offOutput = Events.On("terminal:output", (event: any) => {
    if (typeof event?.data === "string") {
      term?.write(event.data);
    }
  });

  if (hostRef.value) {
    observer = new ResizeObserver(() => refit());
    observer.observe(hostRef.value);
  }
  void refreshStatus();
});

onUnmounted(() => {
  // Detach only the view: the shell keeps running so switching tool tabs does
  // not kill a long-running command.
  offOutput?.();
  observer?.disconnect();
  term?.dispose();
  term = null;
  fitAddon = null;
});

watch(
  () => props.currentRepoPath,
  () => {
    void refreshStatus();
  }
);
</script>

<template>
  <div class="space-y-3" data-tool-section="terminal">
    <div class="pb-2 border-b border-line flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-ink flex items-center gap-2">
          <TerminalIcon class="w-4 h-4 text-accent" />
          <span>{{ t("terminal.title") }}</span>
          <span
            class="px-1.5 py-0.5 rounded text-[9px] font-bold"
            :class="isRunning ? 'bg-success/15 text-success' : 'bg-surface border border-line text-ink-muted'"
          >
            {{ isRunning ? t("terminal.running") : t("terminal.stopped") }}
          </span>
        </h3>
        <p class="text-[11px] text-ink-muted mt-0.5">
          {{ t("terminal.intro") }}
        </p>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <UiButton v-if="!isRunning" variant="primary" :disabled="isBusy" @click="startTerminal">
          <Play class="w-3.5 h-3.5" />
          <span>{{ t("terminal.start") }}</span>
        </UiButton>
        <UiButton v-else variant="danger" :disabled="isBusy" @click="stopTerminal">
          <Square class="w-3.5 h-3.5" />
          <span>{{ t("terminal.stop") }}</span>
        </UiButton>
        <UiButton variant="secondary" @click="clearScreen">
          <Eraser class="w-3.5 h-3.5" />
          <span>{{ t("terminal.clear") }}</span>
        </UiButton>
      </div>
    </div>

    <p class="text-[10px] font-mono text-ink-muted truncate" :title="statusText">
      {{ statusText || t("terminal.not.started") }}
    </p>

    <div
      class="rounded-xl border border-line bg-canvas overflow-hidden p-2"
      style="height: 520px"
    >
      <div ref="hostRef" class="w-full h-full" />
    </div>

    <p class="text-[10px] text-ink-muted">
      {{ t("terminal.warning") }}
    </p>
  </div>
</template>

<style scoped>
/* xterm renders its own viewport; make it fill the card. */
:deep(.xterm) {
  height: 100%;
  padding: 2px;
}
:deep(.xterm-viewport) {
  background-color: transparent !important;
}
</style>
