/**
 * Turn an audit result into a Markdown brief the user can take elsewhere.
 *
 * Several audits report a problem and offer no fix — the engine-asset ones in
 * particular scan by extension and size, never parsing `.meta`, so they cannot
 * know the correct target value and must not rewrite files. What they can do is
 * hand the user a description complete enough to act on outside the app.
 *
 * Deliberately generic: the audits return differently shaped payloads, and
 * writing nine field-by-field extractors would be both brittle and stale the
 * moment a struct changes. Callers pass what they know — summary, advice,
 * per-category counts — plus the raw payload, which is appended verbatim inside
 * a collapsed block so the exact numbers survive.
 */

import { t } from "../i18n";

export interface AuditPlanInput {
  /** Heading, also used as the document title. */
  title: string;
  /** One-line verdict from the audit itself. */
  summary: string;
  /** Concrete next steps the audit already computed. */
  advice?: string[];
  /** Category → count, e.g. `{ 网格: 120, 纹理: 40 }`. */
  counts?: Record<string, number>;
  /** Raw audit payload, appended inside a `<details>` block. */
  raw?: unknown;
  /** Where the audit ran, shown so a reader can reproduce it. */
  scope?: string;
}

/** Pipes and newlines break Markdown tables, so escape them in cell content. */
function cell(value: string): string {
  return value.replace(/\|/g, "\\|").replace(/\r?\n/g, " ");
}

/**
 * Normalize the differently shaped audit payloads into one brief section.
 * Counts stay generic: numeric fields and array lengths are useful context even
 * when an audit has no dedicated exporter.
 */
export function auditSectionFromResult(
  title: string,
  result: unknown,
  scope?: string
): AuditPlanInput {
  const record = result && typeof result === "object" ? (result as Record<string, unknown>) : {};
  const summaryCandidates = [record.summary, record.message, record.statusWarning];
  const summary = summaryCandidates.find(
    (value): value is string => typeof value === "string" && value.trim() !== ""
  ) ?? "";
  const adviceCandidates = [record.advice, record.recommendations, record.warnings, record.issues];
  const adviceSource = adviceCandidates.find(Array.isArray);
  const advice = Array.isArray(adviceSource)
    ? adviceSource
        .filter((value): value is string => typeof value === "string" && value.trim() !== "")
        .slice(0, 12)
    : [];
  const counts: Record<string, number> = {};
  for (const [key, value] of Object.entries(record)) {
    if (Object.keys(counts).length >= 12) break;
    if (typeof value === "number" && Number.isFinite(value)) counts[key] = value;
    else if (Array.isArray(value)) counts[key] = value.length;
  }
  return { title, summary, advice, counts, raw: result, scope };
}

export function buildAuditBundle(sections: AuditPlanInput[], generatedAt: Date = new Date()): string {
  return sections.map((section) => buildAuditPlan(section, generatedAt)).join("\n\n---\n\n");
}

export function buildAuditPlan(input: AuditPlanInput, generatedAt: Date = new Date()): string {
  const lines: string[] = [];

  lines.push(`## ${input.title}`);
  lines.push("");

  if (input.scope) {
    lines.push(t("audit.plan.scope", { scope: input.scope }));
    lines.push("");
  }

  const summary = input.summary.trim();
  if (summary) {
    lines.push(`> ${summary.replace(/\r?\n/g, " ")}`);
    lines.push("");
  }

  const counts = Object.entries(input.counts ?? {}).filter(([, n]) => Number.isFinite(n));
  if (counts.length > 0) {
    lines.push(t("audit.plan.count.header"));
    lines.push("| --- | --- |");
    for (const [label, n] of counts) {
      lines.push(`| ${cell(label)} | ${n} |`);
    }
    lines.push("");
  }

  const advice = (input.advice ?? []).map((a) => a.trim()).filter(Boolean);
  if (advice.length > 0) {
    lines.push(t("audit.plan.advice"));
    lines.push("");
    for (const item of advice) {
      lines.push(`- ${item.replace(/\r?\n/g, " ")}`);
    }
    lines.push("");
  }

  if (input.raw !== undefined) {
    lines.push("<details>");
    lines.push(t("audit.plan.raw"));
    lines.push("");
    lines.push("```json");
    try {
      lines.push(JSON.stringify(input.raw, null, 2));
    } catch {
      // Cyclic or otherwise unserialisable: say so rather than emit broken JSON.
      lines.push(t("audit.plan.raw.failed"));
    }
    lines.push("```");
    lines.push("");
    lines.push("</details>");
    lines.push("");
  }

  lines.push(t("audit.plan.generated", { time: generatedAt.toISOString() }));
  lines.push("");
  return lines.join("\n");
}
