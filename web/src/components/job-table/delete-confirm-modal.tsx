import { Button } from "@/components/ui/button";
import type { JobItem } from "@/lib/api";

interface Props {
  job: JobItem;
  isPending: boolean;
  onCancel: () => void;
  onConfirm: (job: JobItem) => void;
}

export function DeleteConfirmModal({ job, isPending, onCancel, onConfirm }: Props) {
  return (
    <div className="review-preview-overlay" onClick={onCancel}>
      <div className="panel review-confirm-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="review-confirm-title">确认删除</div>
        <div className="review-confirm-body">
          这会删除当前任务以及对应的源文件。
          <br />
          <span className="review-confirm-path">{job.rel_path}</span>
        </div>
        <div className="review-confirm-actions">
          <Button onClick={onCancel}>取消</Button>
          <Button variant="primary" onClick={() => onConfirm(job)} disabled={isPending}>
            删除
          </Button>
        </div>
      </div>
    </div>
  );
}
