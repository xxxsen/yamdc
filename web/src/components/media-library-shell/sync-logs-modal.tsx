"use client";

import { formatSyncLogTime } from "@/components/media-library-shell/utils";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";
import type { MediaLibrarySyncLogEntry } from "@/lib/api";

export interface MediaLibrarySyncLogsModalProps {
  open: boolean;
  onClose: () => void;
  loading: boolean;
  error: string;
  logs: MediaLibrarySyncLogEntry[];
}

export function MediaLibrarySyncLogsModal({
  open,
  onClose,
  loading,
  error,
  logs,
}: MediaLibrarySyncLogsModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      bare
      backdropClassName="media-library-detail-modal"
      frameClassName="media-library-sync-logs-frame panel"
      ariaLabel="同步日志"
    >
      <div className="media-library-sync-logs-header">
        <div className="media-library-sync-logs-title">媒体库同步日志</div>
        <Button
          variant="ghost"
          className="media-library-sync-logs-close"
          onClick={onClose}
        >
          关闭
        </Button>
      </div>
      {loading ? (
        <div className="media-library-sync-logs-state">
          <div className="list-loading-spinner" aria-hidden="true" />
        </div>
      ) : error ? (
        <div className="media-library-sync-logs-state">
          <span className="review-message" data-tone="danger">
            {error}
          </span>
        </div>
      ) : logs.length === 0 ? (
        <div className="media-library-sync-logs-state">
          <span className="review-empty-state">暂无同步日志</span>
        </div>
      ) : (
        // 一行一条, 最新的在最上面 (后端已按 created_at DESC 返回)。
        // 不再做前端二次分组 / 折叠: 扁平列表的心智负担最低,
        // 用 run_id 列让用户肉眼区分不同 sync 轮次。
        <ul className="media-library-sync-logs-list">
          {logs.map((entry) => (
            <li key={entry.id} className="media-library-sync-logs-row" data-level={entry.level}>
              <div className="media-library-sync-logs-row-meta">
                <span className="media-library-sync-logs-row-time">{formatSyncLogTime(entry.created_at)}</span>
                <span className="media-library-sync-logs-row-level" data-level={entry.level}>
                  {entry.level.toUpperCase()}
                </span>
                <span className="media-library-sync-logs-row-run" title={entry.run_id}>
                  {entry.run_id}
                </span>
              </div>
              <div className="media-library-sync-logs-row-body">
                {entry.rel_path ? (
                  <span className="media-library-sync-logs-row-path">{entry.rel_path}</span>
                ) : null}
                <span className="media-library-sync-logs-row-message">{entry.message}</span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </Modal>
  );
}
