/**
 * Run an async mapper over a list with a bounded number of in-flight calls.
 *
 * Why not `Promise.all`: the backend here is a single Wails bridge, not a fleet
 * of servers. Firing 20 disk-size scans at once queues 20 goroutines behind one
 * pipe and makes the whole panel feel stuck; four at a time keeps the UI
 * responsive and still cuts wall time by roughly the limit.
 *
 * Failures are isolated: one rejection resolves to `undefined` for that item
 * instead of rejecting the batch, so a single unreadable path cannot blank a
 * column of otherwise valid numbers.
 */
export async function mapWithConcurrency<T, R>(
  items: readonly T[],
  limit: number,
  mapper: (item: T, index: number) => Promise<R>,
  onResult?: (result: R | undefined, item: T, index: number) => void
): Promise<(R | undefined)[]> {
  const size = Math.max(1, Math.floor(limit) || 1);
  const results: (R | undefined)[] = new Array(items.length).fill(undefined);
  let cursor = 0;

  async function worker(): Promise<void> {
    for (;;) {
      const index = cursor++;
      if (index >= items.length) return;
      const item = items[index]!;
      try {
        const value = await mapper(item, index);
        results[index] = value;
        onResult?.(value, item, index);
      } catch {
        results[index] = undefined;
        onResult?.(undefined, item, index);
      }
    }
  }

  const workers = Array.from({ length: Math.min(size, items.length) }, () => worker());
  await Promise.all(workers);
  return results;
}
