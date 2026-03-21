import type { JobStatus } from "@/lib/api";
import { cn } from "@/lib/utils";

const LABELS: Record<JobStatus, string> = {
  init: "Init",
  processing: "Processing",
  reviewing: "Reviewing",
  done: "Done",
  failed: "Failed",
};

export function StatusBadge({ status }: { status: JobStatus }) {
  return (
    <span className={cn("badge", `badge-${status}`)} data-view-enabled={status !== "init"}>
      {LABELS[status]}
    </span>
  );
}
