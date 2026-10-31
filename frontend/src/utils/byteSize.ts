/**
 * Byte-size formatting shared by surfaces that show raw byte counts from the
 * backend (asset board rows, summaries, …).
 *
 * There was no shared formatter — the only precedent is a component-local
 * `formatBytes` inside SparseTab.vue, which is not importable. This one keeps
 * the same compact shape (B/KB/MB/GB/TB, one decimal) but is fixed to
 * binary units (1 KB = 1024 B) to match git-side humanized sizes.
 */

const UNITS = ["B", "KB", "MB", "GB", "TB", "PB"] as const;

/** Humanize a byte count: 0 -> "0 B", 1536 -> "1.5 KB", negative -> "0 B". */
export function formatByteSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < UNITS.length - 1) {
    value /= 1024;
    unitIndex++;
  }
  const rounded = unitIndex === 0 ? String(Math.round(value)) : value.toFixed(1);
  return `${rounded} ${UNITS[unitIndex]}`;
}
