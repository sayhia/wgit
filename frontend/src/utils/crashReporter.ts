/**
 * Local Crash & Error Reporter (P4.5)
 *
 * Persists runtime exceptions, unhandled promise rejections, and fatal git failures
 * to local storage (`wgit.crash_log.v1`). Enables users to inspect and export
 * forensic crash bundles even after an unexpected reload.
 */

export interface CrashRecord {
  id: string;
  timestamp: string;
  message: string;
  stack?: string;
  scope: string;
  context?: Record<string, unknown>;
}

const STORAGE_KEY = "wgit.crash_log.v1";
const MAX_LOGS = 25;

let memoryStore: Record<string, string> = {};

function safeGetStorage(): {
  getItem(k: string): string | null;
  setItem(k: string, v: string): void;
  removeItem(k: string): void;
} {
  try {
    if (typeof localStorage !== "undefined" && localStorage) {
      return localStorage;
    }
    if (typeof window !== "undefined" && window.localStorage) {
      return window.localStorage;
    }
  } catch {
    // best-effort: security or quota restrictions
  }
  return {
    getItem(k: string) {
      return memoryStore[k] ?? null;
    },
    setItem(k: string, v: string) {
      memoryStore[k] = v;
    },
    removeItem(k: string) {
      delete memoryStore[k];
    },
  };
}

export function getCrashLogs(): CrashRecord[] {
  const storage = safeGetStorage();
  if (!storage) return [];
  try {
    const raw = storage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function saveCrashLogs(logs: CrashRecord[]): void {
  const storage = safeGetStorage();
  if (!storage) return;
  try {
    storage.setItem(STORAGE_KEY, JSON.stringify(logs.slice(-MAX_LOGS)));
  } catch (err) {
    // best-effort: quota exceeded
    console.warn("failed to persist crash logs:", err);
  }
}

export function clearCrashLogs(): void {
  memoryStore = {};
  const storage = safeGetStorage();
  try {
    storage.removeItem(STORAGE_KEY);
  } catch {
    // best-effort
  }
}

export function recordCrash(
  err: unknown,
  scope = "runtime",
  context: Record<string, unknown> = {}
): CrashRecord {
  const message = err instanceof Error ? err.message : String(err ?? "Unknown error");
  const stack = err instanceof Error ? err.stack : undefined;
  const id = `crash_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
  const record: CrashRecord = {
    id,
    timestamp: new Date().toISOString(),
    message,
    stack,
    scope,
    context: Object.keys(context).length > 0 ? context : undefined,
  };

  const logs = getCrashLogs();
  logs.push(record);
  saveCrashLogs(logs);

  return record;
}

export function hasPendingCrashes(): boolean {
  return getCrashLogs().length > 0;
}

export function exportCrashLogsJson(): string {
  const logs = getCrashLogs();
  return JSON.stringify(
    {
      exportedAt: new Date().toISOString(),
      userAgent: typeof navigator !== "undefined" ? navigator.userAgent : "unknown",
      total: logs.length,
      logs,
    },
    null,
    2
  );
}

/**
 * Initializes global uncaught error listeners for the desktop webview window.
 * Returns a teardown function.
 */
export function setupGlobalCrashReporting(): () => void {
  if (typeof window === "undefined") return () => {};

  const errorHandler = (event: ErrorEvent) => {
    recordCrash(event.error || event.message, "uncaught_exception", {
      filename: event.filename,
      lineno: event.lineno,
      colno: event.colno,
    });
  };

  const rejectionHandler = (event: PromiseRejectionEvent) => {
    recordCrash(event.reason, "unhandled_rejection");
  };

  window.addEventListener("error", errorHandler);
  window.addEventListener("unhandledrejection", rejectionHandler);

  return () => {
    window.removeEventListener("error", errorHandler);
    window.removeEventListener("unhandledrejection", rejectionHandler);
  };
}
