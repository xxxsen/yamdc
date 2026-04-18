"use client";

import { Button } from "@/components/ui/button";

export interface RestoreConfirmOverlayProps {
  open: boolean;
  selectedRelPath: string | undefined;
  onCancel: () => void;
  onConfirm: () => void;
  isPending: boolean;
}

export function RestoreConfirmOverlay({
  open,
  selectedRelPath,
  onCancel,
  onConfirm,
  isPending,
}: RestoreConfirmOverlayProps) {
  if (!open) return null;
  return (
    <div className="review-preview-overlay" onClick={onCancel}>
      <div className="panel review-confirm-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="review-confirm-title">恢复原始内容</div>
        <div className="review-confirm-body">
          这会用最初刮削得到的原始内容覆盖当前修改。
          <br />
          <span className="review-confirm-path">{selectedRelPath}</span>
        </div>
        <div className="review-confirm-actions">
          <Button onClick={onCancel}>取消</Button>
          <Button variant="primary" onClick={onConfirm} disabled={isPending}>
            恢复
          </Button>
        </div>
      </div>
    </div>
  );
}
