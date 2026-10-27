/**
 * High-performance, zero-dependency generic Least Recently Used (LRU) Cache.
 * Used for caching immutable Git commits, diffs, and SVG DAG graph paths.
 */
export class LruCache<K, V> {
  private capacity: number;
  private map: Map<K, V>;

  constructor(capacity = 128) {
    if (capacity <= 0) {
      throw new Error("LruCache capacity must be greater than 0");
    }
    this.capacity = capacity;
    this.map = new Map<K, V>();
  }

  get(key: K): V | undefined {
    if (!this.map.has(key)) return undefined;
    const value = this.map.get(key)!;
    // Refresh position to mark as recently used
    this.map.delete(key);
    this.map.set(key, value);
    return value;
  }

  set(key: K, value: V): void {
    if (this.map.has(key)) {
      this.map.delete(key);
    } else if (this.map.size >= this.capacity) {
      // Evict oldest item (first item in Map iterator)
      const oldestKey = this.map.keys().next().value;
      if (oldestKey !== undefined) {
        this.map.delete(oldestKey);
      }
    }
    this.map.set(key, value);
  }

  has(key: K): boolean {
    return this.map.has(key);
  }

  delete(key: K): boolean {
    return this.map.delete(key);
  }

  clear(): void {
    this.map.clear();
  }

  size(): number {
    return this.map.size;
  }

  getCapacity(): number {
    return this.capacity;
  }
}
