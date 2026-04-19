"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { Spinner } from "@/components/ui/spinner";
import type { JobItem, JobListResponse, JobLogItem } from "@/lib/api";

import { DeleteConfirmModal } from "./job-table/delete-confirm-modal";
import {
  STATUS_FILTER,
  canSelectJob,
  compareJobsByNumber,
} from "./job-table/helpers";
import { JobLogModal } from "./job-table/job-log-modal";
import { JobRow } from "./job-table/job-row";
import type { FilterChip, SummaryCard } from "./job-table/job-table-header";
import { JobTableHeader } from "./job-table/job-table-header";
import { useJobActions } from "./job-table/use-job-actions";

interface Props {
  initialData: JobListResponse;
}

// JobTable: 处理队列页 (/processing) 的主壳.
//   - 本文件只保留: 本地状态声明 + useMemo 派生 + 选择同步 effect + JSX 编排.
//   - 所有 handler / 轮询 / 防抖搜索 → ./job-table/use-job-actions.ts.
//   - 纯函数 (排序 / 番号 meta / 路径切分) → ./job-table/helpers.ts (带单测).
//   - 行 / 表头 / 两个 Modal → ./job-table/*.tsx.
// 详见 td/022-frontend-optimization-roadmap.md §3.4.
//
// max-lines-per-function + complexity: JobTable 是 shell 组件, 职责是本地
// 状态声明 + useMemo 派生 + 单次 effect + JSX 编排; 进一步拆分会把紧密耦合
// 的 state/derived/JSX 三者打散, 反而降低可读性. 所有可拆的部分 (handler /
// 行 / 表头 / modal) 已经拆出去了. 此处留 disable + 说明作为技术债标记.
// eslint-disable-next-line complexity, max-lines-per-function
export function JobTable({ initialData }: Props) {
  const [allJobs, setAllJobs] = useState(initialData.items);
  const [total, setTotal] = useState(initialData.total);
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [message, setMessage] = useState<string>("");
  const [isScanning, setIsScanning] = useState(false);
  const [logJob, setLogJob] = useState<JobItem | null>(null);
  const [logs, setLogs] = useState<JobLogItem[]>([]);
  const [logMessage, setLogMessage] = useState("");
  const [deleteConfirmJob, setDeleteConfirmJob] = useState<JobItem | null>(null);
  const [editingJobId, setEditingJobId] = useState<number | null>(null);
  const [editingNumber, setEditingNumber] = useState("");
  const [selectedJobIds, setSelectedJobIds] = useState<Set<number>>(new Set());
  const [hasHydrated, setHasHydrated] = useState(false);
  const selectAllRef = useRef<HTMLInputElement | null>(null);

  const resolvedStatusFilter = statusFilter === "all" ? STATUS_FILTER : statusFilter;
  const messageTone = /失败|failed|error/i.test(message) ? "danger" : "info";

  const counts = useMemo(() => {
    return allJobs.reduce<Partial<Record<string, number>>>((acc, item) => {
      return { ...acc, [item.status]: (acc[item.status] ?? 0) + 1 };
    }, {});
  }, [allJobs]);
  const processingCount = counts.processing ?? 0;
  const failedCount = counts.failed ?? 0;
  const reviewingCount = counts.reviewing ?? 0;
  const initCount = counts.init ?? 0;

  const jobs = useMemo(() => {
    const source = statusFilter === "all" ? allJobs : allJobs.filter((item) => item.status === statusFilter);
    return [...source].sort(compareJobsByNumber);
  }, [allJobs, statusFilter]);

  const selectableJobs = useMemo(() => jobs.filter(canSelectJob), [jobs]);
  const selectedCount = selectedJobIds.size;
  const allSelectableChecked = selectableJobs.length > 0 && selectableJobs.every((job) => selectedJobIds.has(job.id));
  const hasPartialSelection = selectedCount > 0 && !allSelectableChecked;
  const readyToRunCount = selectableJobs.length;

  const actions = useJobActions({
    jobs,
    keyword,
    resolvedStatusFilter,
    selectedJobIds,
    editingNumber,
    logJob,
    selectableJobs,
    setAllJobs,
    setTotal,
    setMessage,
    setIsScanning,
    setLogJob,
    setLogs,
    setLogMessage,
    setDeleteConfirmJob,
    setEditingJobId,
    setEditingNumber,
    setSelectedJobIds,
  });
  const { isPending } = actions;

  const summaryCards: readonly SummaryCard[] = [
    {
      label: "待提交",
      value: initCount,
      hint: readyToRunCount > 0 ? `其中 ${readyToRunCount} 条已满足提交条件` : "等待影片 ID 确认后可提交",
      tone: "default",
      filter: "init",
    },
    {
      label: "处理中",
      value: processingCount,
      hint: processingCount > 0 ? "列表会自动轮询刷新" : "当前没有运行中的任务",
      tone: "info",
      filter: "processing",
    },
    {
      label: "待复核",
      value: reviewingCount,
      hint: reviewingCount > 0 ? "进入 Review 列表继续处理" : "当前没有待复核任务",
      tone: "warn",
      filter: "reviewing",
    },
    {
      label: "失败",
      value: failedCount,
      hint: failedCount > 0 ? "建议优先查看日志后重试" : "当前没有失败任务",
      tone: "danger",
      filter: "failed",
    },
  ];

  const filterChips: readonly FilterChip[] = [
    { value: "all", label: "全部", count: total },
    { value: "init", label: "待提交", count: initCount },
    { value: "processing", label: "处理中", count: processingCount },
    { value: "reviewing", label: "待复核", count: reviewingCount },
    { value: "failed", label: "失败", count: failedCount },
  ];

  const bulkSelectionDisabled = !hasHydrated || selectableJobs.length === 0 || isPending;

  // hasHydrated 刻意在 effect 里翻 true: SSR 第一帧和客户端初始渲染都必须
  // 是 false (否则会水合不一致), 等水合完成后再 flip true 触发需要 DOM 能力
  // 的分支 (例如 indeterminate 属性 / 可交互 checkbox). React 的
  // set-state-in-effect 规则提示"会 cascade render" — 这里 cascade 就是目的.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setHasHydrated(true);
  }, []);

  useEffect(() => {
    if (!selectAllRef.current) {
      return;
    }
    selectAllRef.current.indeterminate = hasPartialSelection;
  }, [hasPartialSelection]);

  // 筛选切换导致可选集合变动时, 清理掉已不在列表中的选中项, 防止
  // "看不见的任务被批量提交". 需要基于 prev 做集合交集 — 既非纯派生也不是
  // 外部系统同步, set-state-in-effect 规则会误报, 按行压制.
  useEffect(() => {
    const visibleSelectableIds = new Set(selectableJobs.map((job) => job.id));
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSelectedJobIds((prev) => {
      const next = new Set(Array.from(prev).filter((id) => visibleSelectableIds.has(id)));
      if (next.size === prev.size) {
        let unchanged = true;
        for (const id of next) {
          if (!prev.has(id)) {
            unchanged = false;
            break;
          }
        }
        if (unchanged) {
          return prev;
        }
      }
      return next;
    });
  }, [selectableJobs]);

  // 成功类 toast 2.6s 自动消失; danger 留到用户主动刷新.
  useEffect(() => {
    if (!message || messageTone === "danger") {
      return;
    }
    const timer = window.setTimeout(() => {
      setMessage("");
    }, 2600);
    return () => window.clearTimeout(timer);
  }, [message, messageTone]);

  return (
    <>
      <div className="panel file-list-panel">
        <JobTableHeader
          jobsCount={jobs.length}
          total={total}
          keyword={keyword}
          statusFilter={statusFilter}
          isPending={isPending}
          isScanning={isScanning}
          summaryCards={summaryCards}
          filterChips={filterChips}
          onKeywordChange={setKeyword}
          onStatusFilterChange={setStatusFilter}
          onScan={actions.handleScan}
        />
        <div className="table-wrap" style={{ position: "relative", flex: 1, overflow: "auto" }}>
          {isScanning ? <Spinner overlay /> : null}
          <table className="table file-table">
            <thead>
              <tr>
                <th>
                  <div className="file-table-path-head">
                    <input
                      ref={selectAllRef}
                      type="checkbox"
                      checked={allSelectableChecked}
                      disabled={bulkSelectionDisabled}
                      title="选择当前列表中所有可批量提交的文件"
                      onChange={actions.handleToggleSelectAll}
                    />
                    <span>Path</span>
                    <button
                      type="button"
                      className="btn file-batch-submit-btn"
                      data-visible={selectedCount > 0}
                      onClick={actions.handleRunSelectedJobs}
                      disabled={selectedCount === 0 || isPending}
                      aria-hidden={selectedCount === 0}
                      tabIndex={selectedCount === 0 ? -1 : 0}
                    >
                      提交已选 ({selectedCount})
                    </button>
                  </div>
                </th>
                <th>Number</th>
                <th>Size</th>
                <th>Status</th>
                <th>Updated</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job) => (
                <JobRow
                  key={job.id}
                  job={job}
                  isSelected={selectedJobIds.has(job.id)}
                  isEditing={editingJobId === job.id}
                  editingNumber={editingNumber}
                  hasHydrated={hasHydrated}
                  isPending={isPending}
                  onToggleSelect={actions.handleToggleSelectJob}
                  onStartEdit={actions.handleStartEditNumber}
                  onCancelEdit={actions.handleCancelEditNumber}
                  onCommitEdit={actions.handleCommitEditNumber}
                  onEditingNumberChange={setEditingNumber}
                  onRun={actions.handleRun}
                  onRerun={actions.handleRerun}
                  onDelete={actions.handleDelete}
                  onOpenLogs={actions.handleOpenLogs}
                />
              ))}
              {jobs.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ color: "var(--muted)", textAlign: "center", padding: 28 }}>
                    当前没有待处理文件
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
        {message ? (
          <div className="file-list-toast" data-tone={messageTone} role="status" aria-live="polite">
            {message}
          </div>
        ) : null}
      </div>
      {logJob ? (
        <JobLogModal
          job={logJob}
          logs={logs}
          message={logMessage}
          onClose={() => setLogJob(null)}
        />
      ) : null}
      {deleteConfirmJob ? (
        <DeleteConfirmModal
          job={deleteConfirmJob}
          isPending={isPending}
          onCancel={() => setDeleteConfirmJob(null)}
          onConfirm={actions.confirmDelete}
        />
      ) : null}
    </>
  );
}
