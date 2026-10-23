import { createApp } from "vue";
import App from "./App.vue";
import { initTheme } from "./composables/useTheme";
import { initAccent } from "./composables/useAccent";
import { initLocale } from "./i18n";
import { installAriaLabelFallback } from "./composables/ariaLabelFallback";
import "./style.css";

initTheme();
initAccent();
initLocale();
installAriaLabelFallback();

const app = createApp(App);

app.config.errorHandler = (err, instance, info) => {
  console.error("[Vue Error Handler]", err, info);
};

window.addEventListener("error", (event) => {
  console.error("[Window Error]", event.error || event.message);
});

window.addEventListener("unhandledrejection", (event) => {
  console.error("[Unhandled Promise Rejection]", event.reason);
});

try {
  console.log("[main.ts] Mounting Vue app to #app");
  app.mount("#app");
  console.log("[main.ts] Vue app successfully mounted!");
} catch (e: any) {
  console.error("[main.ts] Fatal mount exception:", e);
  const appEl = document.getElementById("app");
  if (appEl) {
    appEl.innerHTML = `
      <div style="padding: 24px; color: #ff9592; background: #19191b; font-family: monospace;">
        <h2>⚠️ Vue Mount Failed</h2>
        <pre style="white-space: pre-wrap; font-size: 12px; margin-top: 12px;">${e?.stack || e?.message || e}</pre>
      </div>
    `;
  }
}
