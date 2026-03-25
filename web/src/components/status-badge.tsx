import type { JobStatus } from "@/lib/api";
import { cn } from "@/lib/utils";

const LABELS: Record<JobStatus, string> = {
  init: "待提交",
  processing: "处理中",
  reviewing: "待复核",
  done: "已完成",
  failed: "失败",
};

export function StatusBadge({ status }: { status: JobStatus }) {
  return (
    <span className={cn("badge", `badge-${status}`)} data-view-enabled={status !== "init"}>
      <span className="badge-dot" aria-hidden="true" />
      {LABELS[status]}
    </span>
  );
}
