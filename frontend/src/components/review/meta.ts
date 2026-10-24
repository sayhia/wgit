import { t } from "../../i18n";

export interface StatusMeta {
  /** i18n key — resolve with `t()` at the call site so locale switches apply. */
  labelKey: string;
  chip: string;
}

const PENDING_STATUS_META: StatusMeta = {
  labelKey: "review.status.pending",
  chip: "bg-raised text-ink-muted border-line",
};

const FILE_STATUS_MAP: Record<string, StatusMeta> = {
  approved: { labelKey: "review.status.approved", chip: "bg-success-bg text-success border-success-border" },
  changes: { labelKey: "review.status.changes", chip: "bg-danger-bg text-danger border-danger-border" },
  pending: PENDING_STATUS_META,
};

export function fileStatus(status: string): StatusMeta {
  return FILE_STATUS_MAP[status] ?? PENDING_STATUS_META;
}

export function changeTypeMeta(changeType: string): StatusMeta {
  switch ((changeType || "").toUpperCase()) {
    case "A":
      return { labelKey: "review.change.added", chip: "bg-success-bg text-success border-success-border" };
    case "D":
      return { labelKey: "review.change.deleted", chip: "bg-danger-bg text-danger border-danger-border" };
    case "R":
      return { labelKey: "review.change.renamed", chip: "bg-purple-bg text-purple border-purple-border" };
    case "C":
      return { labelKey: "review.change.copied", chip: "bg-info-bg text-info border-info-border" };
    default:
      return { labelKey: "review.change.modified", chip: "bg-warning-bg text-warning border-warning-border" };
  }
}

/** Raw verdict text → stable i18n key. Unknown verdicts fall through unchanged. */
const VERDICT_KEYS: Record<string, string> = {
  ready: "review.verdict.ready",
  mergeable: "review.verdict.ready",
  changes_requested: "review.verdict.changes",
  changes: "review.verdict.changes",
  in_progress: "review.verdict.reviewing",
  reviewing: "review.verdict.reviewing",
  pending: "review.verdict.reviewing",
  no_changes: "review.verdict.no.changes",
};

export function verdictKeyOf(raw: string): string {
  const text = (raw || "").trim();
  if (!text) return "review.verdict.none";
  return VERDICT_KEYS[text] ?? "";
}

export function verdictTextOf(raw: string): string {
  const key = verdictKeyOf(raw);
  return key ? t(key) : (raw || "").trim();
}

export function verdictChipOf(raw: string): string {
  switch (verdictKeyOf(raw)) {
    case "review.verdict.ready":
      return "bg-success-bg text-success border-success-border";
    case "review.verdict.changes":
      return "bg-danger-bg text-danger border-danger-border";
    case "review.verdict.reviewing":
      return "bg-accent-bg text-accent border-accent-border";
    default:
      return "bg-raised text-ink-muted border-line";
  }
}
