"use client";

import { Button } from "@/components/ui/button";

export interface DeleteConfirmOverlayProps {
  targetIds: number[] | null;
  selectedRelPath: string | undefined;
  onCancel: () => void;
  onConfirm: () => void;
  isPending: boolean;
  // moveRunning: 媒体库迁移进行中。为 true 时"删除"按钮被禁用 + 展示提示,
  // 确保"对话框已经打开、中途 moveRunning 才变 true"这种时序也不会放过
  // 误点。具体竞态锁在 confirmDelete 里还会兜底 early-return, 这里主要
  // 是让用户肉眼看到"为什么按不动"。
  moveRunning?: boolean;
}

export function DeleteConfirmOverlay({
  targetIds,
  selectedRelPath,
  onCancel,
  onConfirm,
  isPending,
  moveRunning = false,
}: DeleteConfirmOverlayProps) {
  if (!targetIds || targetIds.length === 0) return null;
  return (
    <div className="review-preview-overlay" onClick={onCancel}>
      <div className="panel review-confirm-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="review-confirm-title">确认删除</div>
        <div className="review-confirm-body">
          {targetIds.length > 1 ? (
            <>
              这会删除已选中的 {targetIds.length} 个任务以及各自对应的源文件。
            </>
          ) : (
            <>
              这会删除当前任务以及对应的源文件。
              <br />
              <span className="review-confirm-path">{selectedRelPath}</span>
            </>
          )}
          {moveRunning ? (
            <div className="review-confirm-warning" role="alert">
              媒体库移动进行中，暂不可删除。请等同步完成后重试。
            </div>
          ) : null}
        </div>
        <div className="review-confirm-actions">
          <Button onClick={onCancel}>取消</Button>
          <Button
            variant="primary"
            onClick={onConfirm}
            disabled={isPending || moveRunning}
            title={moveRunning ? "媒体库移动进行中，暂不可删除" : undefined}
          >
            删除
          </Button>
        </div>
      </div>
    </div>
  );
}
