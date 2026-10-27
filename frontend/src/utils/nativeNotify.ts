/**
 * Native system notifications helper with graceful web Notification API fallback.
 * Allows long-running operations (like local CI sandbox runs, air-gapped bundle exports,
 * and background repo pre-warming) to notify developers even when the app is in background.
 */
export async function sendNativeNotification(title: string, body: string): Promise<boolean> {
  try {
    if (typeof window === "undefined" || !("Notification" in window)) {
      return false;
    }
    if (Notification.permission === "granted") {
      new Notification(title, { body });
      return true;
    }
    if (Notification.permission !== "denied") {
      const permission = await Notification.requestPermission();
      if (permission === "granted") {
        new Notification(title, { body });
        return true;
      }
    }
    return false;
  } catch {
    return false;
  }
}
