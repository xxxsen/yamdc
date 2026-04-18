import { Eye, Pencil, RotateCcw, Sparkles, Trash2, X } from "lucide-react";
import type { ChangeEvent } from "react";

import { Button } from "@/components/ui/button";
import type { JobItem } from "@/lib/api";
import { formatBytes, formatUnixMillis } from "@/lib/utils";

import { StatusBadge } from "../status-badge";
import {
  canSelectJob,
  getNumberHint,
  getPathSegments,
  requiresManualNumberReview,
} from "./helpers";
import { NumberStatusIcon } from "./number-status-icon";

// JobRow 负责渲染单行任务, 不持有状态, 所有交互通过 props 回调透传给父层.
// 拆出来的主要动机是让外层主文件摆脱 ~160 行行内渲染, 便于单独读懂这一行的
// 显示/交互语义 (选择框 / 行内编辑番号 / 状态 / 日志 / 动作).
interface Props {
  job: JobItem;
  isSelected: boolean;
  isEditing: boolean;
  editingNumber: string;
  hasHydrated: boolean;
  isPending: boolean;
  onToggleSelect: (jobID: number) => void;
  onStartEdit: (job: JobItem) => void;
  onCancelEdit: () => void;
  onCommitEdit: (job: JobItem) => void;
  onEditingNumberChange: (value: string) => void;
  onRun: (job: JobItem) => void;
  onRerun: (job: JobItem) => void;
  onDelete: (job: JobItem) => void;
  onOpenLogs: (job: JobItem) => void;
}

export function JobRow({
  job,
  isSelected,
  isEditing,
  editingNumber,
  hasHydrated,
  isPending,
  onToggleSelect,
  onStartEdit,
  onCancelEdit,
  onCommitEdit,
  onEditingNumberChange,
  onRun,
  onRerun,
  onDelete,
  onOpenLogs,
}: Props) {
  const canRun = job.status === "init";
  const canRerun = job.status === "failed";
  const canDelete = job.status === "init" || job.status === "failed" || job.status === "reviewing";
  const canEditNumber = job.status === "init" || job.status === "failed";
  const hasConflict = Boolean(job.conflict_reason);
  const needsManualNumberReview = requiresManualNumberReview(job);
  const runDisabled = isPending || !canRun || needsManualNumberReview || hasConflict;
  const rerunDisabled = isPending || needsManualNumberReview || hasConflict;
  const runTitle = hasConflict
    ? `${job.conflict_reason}${job.conflict_target ? `: ${job.conflict_target}` : ""}`
    : needsManualNumberReview
      ? "清洗失败或低置信度，需先手动编辑影片 ID 后才能抓取"
      : undefined;
  const { folder, name } = getPathSegments(job);
  const selectable = canSelectJob(job);

  return (
    <tr data-selected={isSelected} data-status={job.status}>
      <td data-label="文件" style={{ minWidth: 280 }}>
        <div className="file-path-cell">
          <input
            type="checkbox"
            checked={isSelected}
            disabled={!hasHydrated || !selectable || isPending}
            title={!selectable ? "中高置信度或手动编辑后的影片 ID 可加入批量提交" : "选择任务"}
            onChange={() => onToggleSelect(job.id)}
          />
          <div className="file-path-copy">
            <div className="file-path-title-row">
              <span className="file-path-name" title={name}>
                {name}
              </span>
              {hasConflict ? <span className="file-path-flag">目标冲突</span> : null}
              {needsManualNumberReview ? <span className="file-path-flag">需校正影片 ID</span> : null}
            </div>
            <div className="file-path-folder" title={job.rel_path}>
              {folder}
            </div>
          </div>
        </div>
      </td>
      <td data-label="影片 ID" style={{ width: 340 }}>
        <div className="file-number-cell">
          <div style={{ minWidth: 0, flex: 1 }}>
            {isEditing ? (
              <div className="file-number-editing">
                <NumberStatusIcon job={job} />
                <input
                  className="input"
                  style={{ width: "100%", minWidth: 0, height: 40, boxSizing: "border-box", padding: "0 12px" }}
                  value={editingNumber}
                  placeholder="请输入确认后的影片 ID"
                  autoFocus
                  onChange={(e: ChangeEvent<HTMLInputElement>) => onEditingNumberChange(e.target.value)}
                  onBlur={() => onCommitEdit(job)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      onCommitEdit(job);
                    }
                    if (e.key === "Escape") {
                      e.preventDefault();
                      onCancelEdit();
                    }
                  }}
                />
              </div>
            ) : (
              <div className="file-number-display">
                <NumberStatusIcon job={job} />
                <div className="file-number-copy">
                  <span
                    className="file-number-value"
                    title={needsManualNumberReview ? "待手动确认" : job.number}
                  >
                    {needsManualNumberReview ? "待确认" : job.number}
                  </span>
                  <span className="file-number-note">{getNumberHint(job)}</span>
                </div>
              </div>
            )}
          </div>
          {canEditNumber ? (
            isEditing ? (
              <Button
                onMouseDown={(e) => e.preventDefault()}
                onClick={onCancelEdit}
                disabled={isPending}
              >
                <X size={16} />
              </Button>
            ) : (
              <Button onClick={() => onStartEdit(job)} disabled={isPending}>
                <Pencil size={16} />
              </Button>
            )
          ) : null}
        </div>
      </td>
      <td data-label="大小">
        <div className="file-meta-cell">{formatBytes(job.file_size)}</div>
      </td>
      <td data-label="状态" style={{ width: 210 }}>
        <div className="file-status-cell">
          <div className="file-status-copy">
            <StatusBadge status={job.status} />
          </div>
          {job.status !== "init" ? (
            <button
              className={`file-log-btn ${job.status === "failed" || job.error_msg ? "file-log-btn-danger" : ""}`}
              onClick={() => onOpenLogs(job)}
              disabled={isPending}
              aria-label="查看日志"
              title="查看日志"
            >
              <Eye size={14} />
            </button>
          ) : (
            <span className="file-log-placeholder" aria-hidden="true">-</span>
          )}
        </div>
      </td>
      <td data-label="更新时间" style={{ width: 180 }}>
        <div className="file-time-cell">
          <span>{formatUnixMillis(job.updated_at)}</span>
          <span className="file-inline-muted">
            {job.status === "processing" ? "运行中自动刷新" : "最近一次状态变更"}
          </span>
        </div>
      </td>
      <td data-label="操作" style={{ width: 230 }}>
        <div className="file-actions-cell">
          {canRerun ? (
            <Button
              className="file-action-btn"
              onClick={() => onRerun(job)}
              disabled={rerunDisabled}
              title={runTitle}
              leftIcon={<RotateCcw size={16} />}
            >
              重试
            </Button>
          ) : (
            <Button
              className="file-action-btn"
              onClick={() => onRun(job)}
              disabled={runDisabled}
              title={runTitle}
              leftIcon={<Sparkles size={16} />}
            >
              提交
            </Button>
          )}
          <Button
            className="file-action-btn file-action-btn-ghost"
            onClick={() => onDelete(job)}
            disabled={!canDelete || isPending}
            leftIcon={<Trash2 size={16} />}
          >
            删除
          </Button>
        </div>
      </td>
    </tr>
  );
}
