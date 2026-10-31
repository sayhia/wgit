import { createApp } from "vue";
import App from "./App.vue";
import { initTheme } from "./composables/useTheme";
import { initAccent } from "./composables/useAccent";
import { initLocale, t } from "./i18n";
import { installAriaLabelFallback } from "./composables/ariaLabelFallback";
import { recordCrash } from "./utils/crashReporter";
import "./style.css";

initTheme();
initAccent();
initLocale();
installAriaLabelFallback();

const app = createApp(App);

app.config.errorHandler = (err, instance, info) => {
  console.error("[Vue Error Handler]", err, info);
  // Vue swallows render/lifecycle errors before the window listeners can see
  // them, so persist them explicitly or they never reach the crash report UI.
  const type = instance?.$?.type as { name?: string; __name?: string } | undefined;
  recordCrash(err, "vue", { info, component: type?.__name || type?.name || "app" });
};

// Resource lifecycle note: these two window-level error listeners are
// intentional app-lifetime singletons — main.ts runs exactly once per process
// and never unmounts, so no teardown is required (unlike setupGlobalCrashReporting()
// in App.vue, which is paired with teardownCrashReporter() in onUnmounted).
window.addEventListener("error", (event) => {
  // A ResizeObserver callback that changes layout re-enters the observer and the
  // browser reports "loop completed with undelivered notifications". Nothing is
  // broken — WebKit and Chromium both recommend ignoring it — but on a
  // virtualised diff list it fires continuously and buries real errors. Swallow
  // this one message only; everything else still reaches the console.
  const message = typeof event.message === "string" ? event.message : "";
  if (message.includes("ResizeObserver loop")) {
    event.preventDefault();
    return;
  }
  console.error("[Window Error]", event.error || event.message);
});

window.addEventListener("unhandledrejection", (event) => {
  console.error("[Unhandled Promise Rejection]", event.reason);
});

try {
  app.mount("#app");
} catch (e: any) {
  console.error("[main.ts] Fatal mount exception:", e);
  const appEl = document.getElementById("app");
  if (appEl) {
    // The app tree never mounted, so this screen renders outside Vue. It still
    // goes through t() — the locale was initialised before mount — otherwise a
    // Chinese user's only clue would be an English heading.
    appEl.innerHTML = `
      <div style="padding: 24px; color: #ff9592; background: #19191b; font-family: monospace;">
        <h2>${t("app.mount.failed.title")}</h2>
        <p style="font-size: 12px; opacity: 0.8; margin: 8px 0 0;">${t("app.mount.failed.hint")}</p>
        <pre id="mount-error" style="white-space: pre-wrap; font-size: 12px; margin-top: 12px;"></pre>
      </div>
    `;
    const detail = document.getElementById("mount-error");
    if (detail) detail.textContent = String(e?.stack || e?.message || e);
  }
}
