"use client";

import { Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";

export interface LibraryDetailHeaderProps {
  subtitle: string;
  copyMode: "translated" | "original";
  onCopyModeChange: (mode: "translated" | "original") => void;
  conflict: boolean;
  isPending: boolean;
  onDelete: () => void;
}

export function LibraryDetailHeader({
  subtitle,
  copyMode,
  onCopyModeChange,
  conflict,
  isPending,
  onDelete,
}: LibraryDetailHeaderProps) {
  return (
    <div className="review-header library-detail-header">
      <div>
        <div className="review-list-kicker">Library Editor</div>
        <h2 className="review-detail-title">已入库内容</h2>
        <div className="review-subtitle">{subtitle}</div>
      </div>
      <div className="review-actions library-detail-actions">
        <div className="library-copy-toggle" role="tablist" aria-label="标题与简介语言切换">
          <button
            type="button"
            className="library-copy-toggle-btn"
            data-active={copyMode === "translated"}
            onClick={() => onCopyModeChange("translated")}
          >
            中文
          </button>
          <button
            type="button"
            className="library-copy-toggle-btn"
            data-active={copyMode === "original"}
            onClick={() => onCopyModeChange("original")}
          >
            原文
          </button>
        </div>
        {conflict ? <span className="badge library-conflict-badge">已存在(冲突)</span> : null}
        <Button
          className="file-action-btn file-action-btn-ghost"
          onClick={onDelete}
          disabled={isPending}
          leftIcon={<Trash2 size={16} />}
        >
          删除
        </Button>
      </div>
    </div>
  );
}
