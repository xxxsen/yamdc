"use client";

import { Button } from "@/components/ui/button";

export interface DeleteConfirmOverlayProps {
  targetIds: number[] | null;
  selectedRelPath: string | undefined;
  onCancel: () => void;
  onConfirm: () => void;
  isPending: boolean;
}

export function DeleteConfirmOverlay({
  targetIds,
  selectedRelPath,
  onCancel,
  onConfirm,
  isPending,
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
        </div>
        <div className="review-confirm-actions">
          <Button onClick={onCancel}>取消</Button>
          <Button variant="primary" onClick={onConfirm} disabled={isPending}>
            删除
          </Button>
        </div>
      </div>
    </div>
  );
}
