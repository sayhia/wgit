import { describe, expect, test } from "bun:test";
import { mapWithConcurrency } from "../src/utils/concurrency";

const tick = (ms = 1) => new Promise((resolve) => setTimeout(resolve, ms));

describe("mapWithConcurrency", () => {
  test("keeps results in input order regardless of completion order", async () => {
    const delays = [30, 5, 15, 1];
    const out = await mapWithConcurrency([0, 1, 2, 3], 2, async (i) => {
      await tick(delays[i]);
      return i * 10;
    });
    expect(out).toEqual([0, 10, 20, 30]);
  });

  test("never exceeds the concurrency limit", async () => {
    let inFlight = 0;
    let peak = 0;
    await mapWithConcurrency(Array.from({ length: 12 }, (_, i) => i), 3, async () => {
      inFlight++;
      peak = Math.max(peak, inFlight);
      await tick(3);
      inFlight--;
      return true;
    });
    expect(peak).toBe(3);
  });

  test("is faster than serial when the limit is above one", async () => {
    const start = Date.now();
    await mapWithConcurrency([0, 1, 2, 3], 4, async () => {
      await tick(20);
    });
    const elapsed = Date.now() - start;
    // Serial would be >= 80ms. Allow generous slack for CI, but it must be
    // clearly below the serial time.
    expect(elapsed).toBeLessThan(70);
  });

  test("a rejected item resolves to undefined without failing the batch", async () => {
    const out = await mapWithConcurrency([1, 2, 3], 2, async (n) => {
      if (n === 2) throw new Error("boom");
      return n;
    });
    expect(out).toEqual([1, undefined, 3]);
  });

  test("reports each result as it arrives", async () => {
    const seen: Array<number | undefined> = [];
    await mapWithConcurrency(
      [1, 2, 3],
      1,
      async (n) => {
        if (n === 3) throw new Error("nope");
        return n;
      },
      (value) => seen.push(value)
    );
    expect(seen).toEqual([1, 2, undefined]);
  });

  test("handles an empty list without spawning workers", async () => {
    let calls = 0;
    const out = await mapWithConcurrency([], 4, async () => {
      calls++;
      return 1;
    });
    expect(out).toEqual([]);
    expect(calls).toBe(0);
  });

  test("treats a non-positive limit as one", async () => {
    let inFlight = 0;
    let peak = 0;
    await mapWithConcurrency([1, 2, 3], 0, async () => {
      inFlight++;
      peak = Math.max(peak, inFlight);
      await tick(2);
      inFlight--;
    });
    expect(peak).toBe(1);
  });
});
