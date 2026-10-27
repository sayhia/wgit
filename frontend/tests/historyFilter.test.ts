import { describe, expect, test } from "bun:test";
import {
  parseHistoryQuery,
  matchesHistoryQuery,
  filterCommitsByQuery,
} from "../src/utils/historyFilter";
import type { CommitItem } from "../src/types";

describe("P1: Advanced History Search & Query Parser", () => {
  test("parses author, grep, since, and free text tokens", () => {
    const raw = 'author:Alice grep:"fix crash" since:2026-01-01 refactor';
    const parsed = parseHistoryQuery(raw);

    expect(parsed.author).toBe("alice");
    expect(parsed.grep).toBe("fix crash");
    expect(parsed.since).toBe("2026-01-01");
    expect(parsed.text).toBe("refactor");
  });

  test("parses single quotes and hash prefix", () => {
    const raw = "hash:a1b2c3d author:'Bob Builder'";
    const parsed = parseHistoryQuery(raw);

    expect(parsed.hash).toBe("a1b2c3d");
    expect(parsed.author).toBe("bob builder");
  });

  test("matches commit by author and subject grep", () => {
    const commit: CommitItem = {
      hash: "a1b2c3d4e5f6",
      shortHash: "a1b2c3d",
      subject: "fix: handle crash on exit",
      body: "",
      authorName: "Alice Wonderland",
      authorEmail: "alice@example.com",
      authorDate: "2026-05-01T12:00:00Z",
      commitDate: "2026-05-01T12:00:00Z",
      parentHashes: [],
      branchLines: [],
    };

    const query1 = parseHistoryQuery("author:Alice grep:crash");
    expect(matchesHistoryQuery(commit, query1)).toBe(true);

    const query2 = parseHistoryQuery("author:Bob");
    expect(matchesHistoryQuery(commit, query2)).toBe(false);

    const query3 = parseHistoryQuery("hash:a1b2");
    expect(matchesHistoryQuery(commit, query3)).toBe(true);
  });

  test("filterCommitsByQuery filters an array of commits", () => {
    const commits: CommitItem[] = [
      {
        hash: "1111111",
        shortHash: "1111111",
        subject: "feat: add user login",
        body: "",
        authorName: "Alice",
        authorEmail: "alice@test.com",
        authorDate: "2026-01-10",
        commitDate: "2026-01-10",
        parentHashes: [],
        branchLines: [],
      },
      {
        hash: "2222222",
        shortHash: "2222222",
        subject: "fix: solve memory leak",
        body: "",
        authorName: "Bob",
        authorEmail: "bob@test.com",
        authorDate: "2026-02-15",
        commitDate: "2026-02-15",
        parentHashes: [],
        branchLines: [],
      },
    ];

    expect(filterCommitsByQuery(commits, "author:bob")).toHaveLength(1);
    expect(filterCommitsByQuery(commits, "grep:login")).toHaveLength(1);
    expect(filterCommitsByQuery(commits, "grep:missing")).toHaveLength(0);
    expect(filterCommitsByQuery(commits, "")).toHaveLength(2);
  });
});
