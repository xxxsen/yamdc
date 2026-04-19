"use client";

import { Plus, RefreshCw } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { TaskState } from "@/lib/api";

// LibraryBottomActions: library-shell 底部的两颗主操作按钮 (重新扫描库 +
// 移动到媒体库)。"移动" 按钮内部叠了一个进度条覆层 (.library-action-progress),
// 通过 moveProgressVisible + moveProgress 受控展示。
//
// 从 library-shell.tsx 整块搬过来, 语义零改动。被依赖的 CSS class
// (.library-bottom-actions / .library-action-btn / .library-action-progress*)
// 仍在 globals.css 里, §2.1 Tailwind 迁移时会一并处理。
//
// 详见 td/022-frontend-optimization-roadmap.md §2.2 B-2。

export interface LibraryBottomActionsProps {
  refreshBusy: boolean;
  moveBusy: boolean;
  mediaSyncRunning: boolean;
  configured: boolean;
  refreshButtonLabel: string;
  moveButtonLabel: string;
  moveProgressVisible: boolean;
  moveState: TaskState | null;
  moveProgress: number;
  onRefresh: () => void;
  onMove: () => void;
}

export function LibraryBottomActions({
  refreshBusy,
  moveBusy,
  mediaSyncRunning,
  configured,
  refreshButtonLabel,
  moveButtonLabel,
  moveProgressVisible,
  moveState,
  moveProgress,
  onRefresh,
  onMove,
}: LibraryBottomActionsProps) {
  return (
    <div className="library-bottom-actions">
      <Button
        variant="primary"
        className="media-library-sync-btn library-action-btn"
        onClick={onRefresh}
        disabled={refreshBusy || moveBusy}
        leftIcon={
          <RefreshCw size={16} className={refreshBusy ? "media-library-sync-icon-spinning" : ""} />
        }
      >
        {refreshButtonLabel}
      </Button>
      <Button
        variant="primary"
        className="media-library-sync-btn library-action-btn library-action-btn-progress"
        onClick={onMove}
        disabled={refreshBusy || moveBusy || mediaSyncRunning || !configured}
      >
        {moveProgressVisible && moveState ? (
          <span className="library-action-progress" aria-hidden="true">
            <span className="library-action-progress-fill" style={{ width: `${moveProgress}%` }} />
          </span>
        ) : null}
        <span className="library-action-btn-content">
          {moveBusy ? <RefreshCw size={16} className="media-library-sync-icon-spinning" /> : <Plus size={16} />}
          <span>{moveButtonLabel}</span>
        </span>
      </Button>
    </div>
  );
}
