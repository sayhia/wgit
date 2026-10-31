/**
 * Clipboard writes fail on insecure contexts or when the host denies the
 * permission. Returning the error instead of rejecting keeps callers from
 * producing unhandled rejections while still letting them show feedback.
 */
export async function writeClipboardText(text: string): Promise<string | null> {
  try {
    await navigator.clipboard.writeText(text);
    return null;
  } catch (err) {
    return err instanceof Error ? err.message : String(err);
  }
}
