import { beforeEach, describe, expect, test } from "bun:test";
import {
  clearCrashLogs,
  exportCrashLogsJson,
  getCrashLogs,
  hasPendingCrashes,
  recordCrash,
} from "../src/utils/crashReporter";

describe("P4.5: Local Crash Reporter", () => {
  beforeEach(() => {
    clearCrashLogs();
  });

  test("records an Error with stack and scope", () => {
    const err = new Error("fatal: index corrupted");
    const record = recordCrash(err, "git_index", { file: "repo/.git/index" });

    expect(record.message).toBe("fatal: index corrupted");
    expect(record.scope).toBe("git_index");
    expect(record.stack).toBeDefined();
    expect(record.context).toEqual({ file: "repo/.git/index" });
    expect(hasPendingCrashes()).toBe(true);

    const logs = getCrashLogs();
    expect(logs).toHaveLength(1);
    expect(logs[0]!.message).toBe("fatal: index corrupted");
  });

  test("records non-Error primitives safely", () => {
    recordCrash("string exception", "eval");
    const logs = getCrashLogs();
    expect(logs).toHaveLength(1);
    expect(logs[0]!.message).toBe("string exception");
    expect(logs[0]!.scope).toBe("eval");
  });

  test("clearCrashLogs empties stored records", () => {
    recordCrash(new Error("err 1"));
    recordCrash(new Error("err 2"));
    expect(getCrashLogs()).toHaveLength(2);

    clearCrashLogs();
    expect(getCrashLogs()).toHaveLength(0);
    expect(hasPendingCrashes()).toBe(false);
  });

  test("exportCrashLogsJson produces valid json with metadata", () => {
    recordCrash(new Error("critical error"), "core");
    const jsonStr = exportCrashLogsJson();
    const data = JSON.parse(jsonStr);

    expect(data.total).toBe(1);
    expect(data.exportedAt).toBeDefined();
    expect(data.logs).toHaveLength(1);
    expect(data.logs[0].message).toBe("critical error");
  });
});
