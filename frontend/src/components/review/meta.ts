export interface StatusMeta {
  label: string;
  chip: string;
}

const PENDING_STATUS_META: StatusMeta = {
  label: "待评审",
  chip: "bg-raised text-ink-muted border-line",
};

const FILE_STATUS_MAP: Record<string, StatusMeta> = {
  approved: { label: "通过", chip: "bg-success-bg text-success border-success-border" },
  changes: { label: "需修改", chip: "bg-danger-bg text-danger border-danger-border" },
  pending: PENDING_STATUS_META,
};

export function fileStatus(status: string): StatusMeta {
  return FILE_STATUS_MAP[status] ?? PENDING_STATUS_META;
}

export function changeTypeMeta(changeType: string): StatusMeta {
  switch ((changeType || "").toUpperCase()) {
    case "A":
      return { label: "新增", chip: "bg-success-bg text-success border-success-border" };
    case "D":
      return { label: "删除", chip: "bg-danger-bg text-danger border-danger-border" };
    case "R":
      return { label: "重命名", chip: "bg-purple-bg text-purple border-purple-border" };
    case "C":
      return { label: "复制", chip: "bg-info-bg text-info border-info-border" };
    default:
      return { label: "修改", chip: "bg-warning-bg text-warning border-warning-border" };
  }
}

const VERDICT_ALIASES: Record<string, string> = {
  ready: "可以合并",
  mergeable: "可以合并",
  changes_requested: "需修改",
  changes: "需修改",
  in_progress: "评审中",
  reviewing: "评审中",
  pending: "评审中",
  no_changes: "无改动",
};

export function verdictTextOf(raw: string): string {
  const text = (raw || "").trim();
  if (!text) return "未评审";
  return VERDICT_ALIASES[text] ?? text;
}

export function verdictChipOf(raw: string): string {
  switch (verdictTextOf(raw)) {
    case "可以合并":
      return "bg-success-bg text-success border-success-border";
    case "需修改":
      return "bg-danger-bg text-danger border-danger-border";
    case "评审中":
      return "bg-accent-bg text-accent border-accent-border";
    default:
      return "bg-raised text-ink-muted border-line";
  }
}
