"use client";

import { Check, MoreHorizontal, RotateCcw, Trash2 } from "lucide-react";
import { useEffect, useRef, useState, type RefObject } from "react";

import { Button } from "@/components/ui/button";
import type { JobItem } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

export interface ReviewListPanelProps {
  items: JobItem[];
  selectedId: number | undefined;
  selectedIndex: number;
  selectedJobIds: Set<number>;
  selectedCount: number;
  allSelectableChecked: boolean;
  isPending: boolean;
  moveRunning: boolean;
  selectAllRef: RefObject<HTMLInputElement | null>;
  onToggleSelectAll: () => void;
  onToggleSelectJob: (jobID: number) => void;
  onLoadDetail: (job: JobItem) => void;
  onImportSelected: () => void;
  onDeleteSelected: () => void;
  onImport: () => void;
  onDelete: () => void;
  onReject: () => void;
}

export function ReviewListPanel({
  items,
  selectedId,
  selectedIndex,
  selectedJobIds,
  selectedCount,
  allSelectableChecked,
  isPending,
  moveRunning,
  selectAllRef,
  onToggleSelectAll,
  onToggleSelectJob,
  onLoadDetail,
  onImportSelected,
  onDeleteSelected,
  onImport,
  onDelete,
  onReject,
}: ReviewListPanelProps) {
  return (
    <aside className="panel review-list-panel">
      <div className="review-list-head">
        <div>
          <div className="review-list-kicker">Review Queue</div>
          <h2 className="review-list-title">Review 列表</h2>
          <p className="review-list-subtitle">
            当前 {items.length} 条待复核任务
            {selectedIndex >= 0 ? `，正在查看第 ${selectedIndex + 1} 条` : ""}
          </p>
          {moveRunning ? <p className="review-list-subtitle">媒体库正在同步迁移，审批按钮已临时锁定。</p> : null}
        </div>
      </div>
      <div className="review-bulk-toolbar">
        <label className="review-bulk-select-all">
          <input
            ref={selectAllRef}
            type="checkbox"
            checked={allSelectableChecked}
            disabled={items.length === 0 || isPending || moveRunning}
            title="选择当前列表中的全部 review 任务"
            onChange={onToggleSelectAll}
          />
          <span>全选</span>
        </label>
        <div className="review-bulk-toolbar-actions">
          {selectedCount > 0 ? <span className="review-bulk-count">已选 {selectedCount} 项</span> : null}
          <Button
            className="review-inline-icon-btn review-bulk-approve-btn"
            onClick={onImportSelected}
            disabled={selectedCount === 0 || isPending || moveRunning}
            aria-label="批量审批"
            title={selectedCount > 0 ? `批量审批已选 ${selectedCount} 项` : "批量审批"}
          >
            <Check size={16} />
          </Button>
          <Button
            className="review-inline-icon-btn review-bulk-delete-btn"
            onClick={onDeleteSelected}
            disabled={selectedCount === 0 || isPending}
            aria-label="批量删除"
            title={selectedCount > 0 ? `删除已选 ${selectedCount} 项` : "批量删除"}
          >
            <Trash2 size={14} />
          </Button>
        </div>
      </div>
      <div className="review-job-list">
        {items.length === 0 ? <div className="review-empty-state">当前没有待 review 的任务</div> : null}
        {items.map((job, index) => (
          <div
            key={job.id}
            className="panel review-job-card"
            data-active={selectedId === job.id}
            data-selected={selectedJobIds.has(job.id)}
          >
            <div className="review-job-card-select">
              <input
                type="checkbox"
                checked={selectedJobIds.has(job.id)}
                disabled={isPending || moveRunning}
                title={moveRunning ? "媒体库移动进行中，暂不可选择" : "选择任务"}
                onChange={() => onToggleSelectJob(job.id)}
              />
            </div>
            <button className="review-job-card-main" onClick={() => onLoadDetail(job)} disabled={isPending}>
              <div className="review-job-card-topline">
                <span className="review-job-card-index">#{index + 1}</span>
                <span className="review-job-card-time">更新于 {formatUnixMillis(job.updated_at)}</span>
              </div>
              <div className="review-job-card-path">{job.rel_path}</div>
              <div className="review-job-card-number">{job.number}</div>
            </button>
            <div className="review-job-card-actions">
              <Button
                className="review-inline-icon-btn review-action-approve"
                onClick={onImport}
                disabled={isPending || selectedId !== job.id || moveRunning}
                aria-label="入库"
                title={moveRunning ? "媒体库移动进行中，暂不可审批" : "入库"}
              >
                <Check size={16} />
              </Button>
              <ReviewJobOverflowMenu
                disabled={isPending || selectedId !== job.id}
                onDelete={onDelete}
                onReject={onReject}
              />
            </div>
          </div>
        ))}
      </div>
    </aside>
  );
}

interface ReviewJobOverflowMenuProps {
  disabled: boolean;
  onDelete: () => void;
  onReject: () => void;
}

// ReviewJobOverflowMenu 把 "删除" / "打回" 两个相对低频的破坏性操作
// 折叠到 `...` 菜单里, 避免每张 review 卡片上出现 3 个按钮过于拥挤。
function ReviewJobOverflowMenu({ disabled, onDelete, onReject }: ReviewJobOverflowMenuProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handlePointer = (event: MouseEvent) => {
      if (!containerRef.current) {
        return;
      }
      if (!containerRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    window.addEventListener("mousedown", handlePointer);
    window.addEventListener("keydown", handleKey);
    return () => {
      window.removeEventListener("mousedown", handlePointer);
      window.removeEventListener("keydown", handleKey);
    };
  }, [open]);

  // 注: 不要用 useEffect 同步 "disabled -> setOpen(false)", 那会触发
  // cascading render。改成 "disabled 生效时把 open 视为 false" 的 derived
  // 展示模型, 同时禁用触发器 onClick, 保证切 disabled 那一刻菜单立即收起。
  const effectiveOpen = open && !disabled;

  return (
    <div ref={containerRef} className="review-job-overflow">
      <Button
        className="review-inline-icon-btn review-job-overflow-trigger"
        onClick={() => {
          if (disabled) return;
          setOpen((prev) => !prev);
        }}
        disabled={disabled}
        aria-label="更多操作"
        aria-haspopup="menu"
        aria-expanded={effectiveOpen}
        title="更多操作"
      >
        <MoreHorizontal size={16} />
      </Button>
      {effectiveOpen ? (
        <div className="review-job-overflow-menu" role="menu">
          <button
            type="button"
            role="menuitem"
            className="review-job-overflow-item"
            onClick={() => {
              setOpen(false);
              onReject();
            }}
          >
            <RotateCcw size={14} aria-hidden />
            <span>打回</span>
          </button>
          <button
            type="button"
            role="menuitem"
            className="review-job-overflow-item review-job-overflow-item-danger"
            onClick={() => {
              setOpen(false);
              onDelete();
            }}
          >
            <Trash2 size={14} aria-hidden />
            <span>删除</span>
          </button>
        </div>
      ) : null}
    </div>
  );
}
