"use client";

import { RotateCcw } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { JobItem } from "@/lib/api";

// ReviewDetailHeader: review-shell 主面板顶部的 "Review Editor / Review 内容
// / 消息 / 恢复原始刮削" 四件套. 拆出来目的是让 review-shell.tsx 专注 state
// 编排, 不被 JSX 拖到 400+ 行. 这里保持纯展示 + callback, 不拥有任何 state.

export interface ReviewDetailHeaderProps {
  selected: JobItem | null;
  message: string;
  messageTone: "info" | "danger";
  hasRawMeta: boolean;
  isPending: boolean;
  onRestoreRaw: () => void;
}

export function ReviewDetailHeader({
  selected,
  message,
  messageTone,
  hasRawMeta,
  isPending,
  onRestoreRaw,
}: ReviewDetailHeaderProps) {
  return (
    <div className="review-header">
      <div>
        <div className="review-list-kicker">Review Editor</div>
        <h2 className="review-detail-title">Review 内容</h2>
        {selected ? <div className="review-subtitle">当前任务 #{selected.id} / {selected.rel_path}</div> : null}
      </div>
      <div className="review-actions">
        {message ? <span className="review-message" data-tone={messageTone}>{message}</span> : null}
        <Button
          className="review-inline-icon-btn"
          onClick={onRestoreRaw}
          disabled={!selected || isPending || !hasRawMeta}
          aria-label="恢复原始刮削内容"
          title="恢复原始刮削内容"
        >
          <RotateCcw size={14} />
        </Button>
      </div>
    </div>
  );
}
