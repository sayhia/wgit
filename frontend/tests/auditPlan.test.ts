import { describe, expect, test } from "bun:test";
import { auditSectionFromResult, buildAuditBundle, buildAuditPlan } from "../src/utils/auditPlan";

const at = new Date("2026-10-29T12:00:00.000Z");

describe("buildAuditPlan", () => {
  test("renders title, summary, counts, advice and raw payload", () => {
    const md = buildAuditPlan(
      {
        title: "LOD 与纹理流送",
        summary: "12 个网格缺少 LOD 层级",
        advice: ["先给远景资产补 LOD1", "把 4K 贴图改为流送"],
        counts: { 网格: 120, 纹理: 40 },
        raw: { meshes: 120 },
        scope: "Assets/Env",
      },
      at
    );

    expect(md).toContain("## LOD 与纹理流送");
    expect(md).toContain("> 12 个网格缺少 LOD 层级");
    expect(md).toContain("| 网格 | 120 |");
    expect(md).toContain("| 纹理 | 40 |");
    expect(md).toContain("- 先给远景资产补 LOD1");
    expect(md).toContain("Assets/Env");
    expect(md).toContain("2026-10-29T12:00:00.000Z");
  });

  test("appends the raw payload inside a details block", () => {
    const md = buildAuditPlan({ title: "T", summary: "S", raw: { a: 1 } }, at);
    expect(md).toContain("<details>");
    expect(md).toContain("```json");
    expect(md).toContain('"a": 1');
    expect(md).toContain("</details>");
  });

  test("omits empty sections instead of leaving dangling headings", () => {
    const md = buildAuditPlan({ title: "T", summary: "S" }, at);
    expect(md).not.toContain("### 建议");
    expect(md).not.toContain("| --- |");
    expect(md).not.toContain("<details>");
  });

  test("escapes pipes so a table cell cannot break the table", () => {
    const md = buildAuditPlan({ title: "T", summary: "S", counts: { "a|b": 1 } }, at);
    expect(md).toContain("| a\\|b | 1 |");
  });

  test("flattens newlines in the summary blockquote", () => {
    const md = buildAuditPlan({ title: "T", summary: "line one\nline two" }, at);
    expect(md).toContain("> line one line two");
  });

  test("drops blank advice entries", () => {
    const md = buildAuditPlan({ title: "T", summary: "S", advice: ["", "  ", "real"] }, at);
    expect(md).toContain("- real");
    const adviceLines = md.split("\n").filter((l) => l.startsWith("- "));
    expect(adviceLines).toHaveLength(1);
  });

  test("survives a payload that cannot be serialised", () => {
    const cyclic: Record<string, unknown> = {};
    cyclic.self = cyclic;
    const md = buildAuditPlan({ title: "T", summary: "S", raw: cyclic }, at);
    expect(md).toContain("无法序列化");
    expect(md).toContain("</details>");
  });

  test("ignores non-finite counts", () => {
    const md = buildAuditPlan(
      { title: "T", summary: "S", counts: { ok: 3, bad: Number.NaN } },
      at
    );
    expect(md).toContain("| ok | 3 |");
    expect(md).not.toContain("bad");
  });
});

describe("auditSectionFromResult", () => {
  test("extracts summary, advice, numeric and array counts from a result", () => {
    const section = auditSectionFromResult(
      "LOD 与纹理流送",
      {
        summary: "12 个网格缺少 LOD 层级",
        recommendations: ["补 LOD1", "", "改流送"],
        meshes: 12,
        textures: [1, 2, 3],
        warnings: ["w1"],
      },
      "Assets/Env"
    );
    expect(section.title).toBe("LOD 与纹理流送");
    expect(section.summary).toBe("12 个网格缺少 LOD 层级");
    expect(section.advice).toEqual(["补 LOD1", "改流送"]);
    expect(section.counts?.meshes).toBe(12);
    expect(section.counts?.textures).toBe(3);
    expect(section.counts?.warnings).toBe(1);
    expect(section.scope).toBe("Assets/Env");
    expect(section.raw).toBeDefined();
  });

  test("handles null results and missing text without throwing", () => {
    const section = auditSectionFromResult("Empty", null);
    expect(section.summary).toBe("");
    expect(section.advice).toEqual([]);
    expect(section.counts).toEqual({});
    expect(section.raw).toBeNull();
  });

  test("buildAuditBundle joins sections with separators", () => {
    const bundle = buildAuditBundle(
      [
        { title: "A", summary: "first" },
        { title: "B", summary: "second" },
      ],
      at
    );
    expect(bundle).toContain("## A");
    expect(bundle).toContain("## B");
    expect(bundle).toContain("\n\n---\n\n");
    expect(bundle.match(/2026-10-29T12:00:00.000Z/g)?.length).toBe(2);
  });
});
