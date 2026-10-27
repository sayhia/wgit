/**
 * Adaptive background poller for repository status, locks, and health checks.
 * Automatically throttles when the browser/webview tab is hidden or offline to conserve CPU and battery.
 */
export interface AdaptivePollerOptions {
  activeIntervalMs: number;
  backgroundIntervalMs?: number;
  pauseWhenHidden?: boolean;
}

export interface AdaptivePoller {
  start(): void;
  stop(): void;
  triggerNow(): Promise<void>;
  isRunning(): boolean;
}

export function createAdaptivePoller(
  action: () => void | Promise<void>,
  options: AdaptivePollerOptions
): AdaptivePoller {
  const {
    activeIntervalMs,
    backgroundIntervalMs = activeIntervalMs * 4,
    pauseWhenHidden = false,
  } = options;

  let timer: ReturnType<typeof setTimeout> | null = null;
  let running = false;
  let isExecuting = false;

  async function execute() {
    if (!running || isExecuting) return;
    isExecuting = true;
    try {
      await action();
    } catch (err) {
      // best-effort: the poller retries on the next tick; one failed action is
      // not worth interrupting the user.
      console.warn("[AdaptivePoller] Action error:", err);
    } finally {
      isExecuting = false;
      scheduleNext();
    }
  }

  function scheduleNext() {
    if (!running) return;
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }

    const isHidden = typeof document !== "undefined" && document.hidden;
    const isOnline = typeof navigator !== "undefined" ? navigator.onLine : true;

    if (!isOnline) {
      // Offline: check infrequently
      timer = setTimeout(execute, backgroundIntervalMs * 2);
      return;
    }

    if (isHidden) {
      if (pauseWhenHidden) {
        // Paused when completely hidden; will resume on visibilitychange
        return;
      }
      timer = setTimeout(execute, backgroundIntervalMs);
    } else {
      timer = setTimeout(execute, activeIntervalMs);
    }
  }

  function handleVisibilityChange() {
    if (!running) return;
    if (typeof document !== "undefined" && !document.hidden) {
      // Woke up from background: immediately run or schedule active interval
      scheduleNext();
    } else {
      scheduleNext();
    }
  }

  function handleOnline() {
    if (!running) return;
    scheduleNext();
  }

  return {
    start() {
      if (running) return;
      running = true;
      if (typeof document !== "undefined" && typeof document.addEventListener === "function") {
        document.addEventListener("visibilitychange", handleVisibilityChange);
      }
      if (typeof window !== "undefined" && typeof window.addEventListener === "function") {
        window.addEventListener("online", handleOnline);
      }
      scheduleNext();
    },

    stop() {
      running = false;
      if (timer) {
        clearTimeout(timer);
        timer = null;
      }
      if (typeof document !== "undefined" && typeof document.removeEventListener === "function") {
        document.removeEventListener("visibilitychange", handleVisibilityChange);
      }
      if (typeof window !== "undefined" && typeof window.removeEventListener === "function") {
        window.removeEventListener("online", handleOnline);
      }
    },

    async triggerNow() {
      if (timer) {
        clearTimeout(timer);
        timer = null;
      }
      await execute();
    },

    isRunning() {
      return running;
    },
  };
}
