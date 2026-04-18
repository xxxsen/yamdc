"use client";

import { Check, Edit3, Eye, Pencil, RefreshCw, RotateCcw, Search, Sparkles, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState, useTransition } from "react";

import type { JobItem, JobListResponse, JobLogItem } from "@/lib/api";
import { deleteJob, listJobs, listJobLogs, rerunJob, runJob, triggerScan, updateJobNumber } from "@/lib/api";
import { formatBytes, formatUnixMillis } from "@/lib/utils";
import { StatusBadge } from "./status-badge";

interface Props {
  initialData: JobListResponse;
}

const STATUS_FILTER = "init,processing,failed,reviewing";

// jobSortKey 取一个最能代表 "番号前缀" 的字段: 优先用清洗后的 number,
// 没清洗过 (init) 就拿 raw_number 兜底。localeCompare 的 numeric:true 会把
// "ABC-2" / "ABC-10" 当数字比较, 相同前缀会天然聚到一起, 这是刻意选的行为
// — 用户反馈想按番号前缀分组而不是时间序。
function jobSortKey(job: JobItem) {
  return (job.number || job.raw_number || job.cleaned_number || "").trim();
}

function compareJobsByNumber(a: JobItem, b: JobItem) {
  const aKey = jobSortKey(a);
  const bKey = jobSortKey(b);
  // 都没 number 的 (比如刚扫进来又清洗失败) 按 id DESC 兜底, 保留 "最新在前"
  // 的直觉; 有 number 的永远排在没 number 的前面, 避免空番号把列表头占满。
  if (!aKey && !bKey) {
    return b.id - a.id;
  }
  if (!aKey) {
    return 1;
  }
  if (!bKey) {
    return -1;
  }
  const cmp = aKey.localeCompare(bKey, "zh-Hans-CN", { numeric: true, sensitivity: "base" });
  if (cmp !== 0) {
    return cmp;
  }
  // 同 number 的 (极少见, 一般是 conflict_reason 场景) 按 id DESC 作稳定兜底,
  // 免得用户每次刷新顺序都飘。
  return b.id - a.id;
}

export function JobTable({ initialData }: Props) {
  const [allJobs, setAllJobs] = useState(initialData.items);
  const [total, setTotal] = useState(initialData.total);
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [message, setMessage] = useState<string>("");
  const [isScanning, setIsScanning] = useState(false);
  const [isPending, startTransition] = useTransition();
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
      acc[item.status] = (acc[item.status] ?? 0) + 1;
      return acc;
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

  const canSelectJob = (job: JobItem) => {
    if (job.conflict_reason) {
      return false;
    }
    const hasApprovedNumber =
      job.number_source === "manual" ||
      (job.number_clean_status === "success" &&
        (job.number_clean_confidence === "high" || job.number_clean_confidence === "medium"));
    const isRunnable = job.status === "init" || job.status === "failed";
    return hasApprovedNumber && isRunnable;
  };

  const selectableJobs = useMemo(() => jobs.filter(canSelectJob), [jobs]);
  const selectedCount = selectedJobIds.size;
  const allSelectableChecked = selectableJobs.length > 0 && selectableJobs.every((job) => selectedJobIds.has(job.id));
  const hasPartialSelection = selectedCount > 0 && !allSelectableChecked;
  const readyToRunCount = selectableJobs.length;

  const summaryCards = [
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
  ] as const;

  const filterChips = [
    { value: "all", label: "全部", count: total },
    { value: "init", label: "待提交", count: initCount },
    { value: "processing", label: "处理中", count: processingCount },
    { value: "reviewing", label: "待复核", count: reviewingCount },
    { value: "failed", label: "失败", count: failedCount },
  ] as const;

  const bulkSelectionDisabled = !hasHydrated || selectableJobs.length === 0 || isPending;

  useEffect(() => {
    setHasHydrated(true);
  }, []);

  useEffect(() => {
    if (!selectAllRef.current) {
      return;
    }
    selectAllRef.current.indeterminate = hasPartialSelection;
  }, [hasPartialSelection]);

  useEffect(() => {
    const visibleSelectableIds = new Set(selectableJobs.map((job) => job.id));
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

  useEffect(() => {
    if (!message || messageTone === "danger") {
      return;
    }
    const timer = window.setTimeout(() => {
      setMessage("");
    }, 2600);
    return () => window.clearTimeout(timer);
  }, [message, messageTone]);

  const refreshJobs = async (nextKeyword = keyword) => {
    const data = await listJobs({
      status: STATUS_FILTER,
      all: true,
      keyword: nextKeyword,
    });
    setAllJobs(data.items);
    setTotal(data.total);
  };

  useEffect(() => {
    let controller: AbortController | null = null;
    const timer = window.setInterval(() => {
      controller?.abort();
      controller = new AbortController();
      void listJobs({ status: STATUS_FILTER, all: true, keyword }, controller.signal)
        .then((data) => {
          setAllJobs(data.items);
          setTotal(data.total);
        })
        .catch(() => undefined);
    }, 8000);
    return () => {
      window.clearInterval(timer);
      controller?.abort();
    };
  }, [keyword, resolvedStatusFilter]);

  const handleScan = () => {
    startTransition(async () => {
      try {
        setIsScanning(true);
        setMessage("");
        await triggerScan();
        await refreshJobs();
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "扫描失败");
      } finally {
        setIsScanning(false);
      }
    });
  };

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      startTransition(async () => {
        try {
          const data = await listJobs({
            status: STATUS_FILTER,
            all: true,
            keyword,
          }, controller.signal);
          setAllJobs(data.items);
          setTotal(data.total);
        } catch (error) {
          if (controller.signal.aborted) return;
          setMessage(error instanceof Error ? error.message : "查询失败");
        }
      });
    }, 250);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [keyword, resolvedStatusFilter]);

  const handleRun = (job: JobItem) => {
    startTransition(async () => {
      try {
        setMessage("");
        await runJob(job.id);
        await refreshJobs();
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "启动任务失败");
      }
    });
  };

  const handleRerun = (job: JobItem) => {
    startTransition(async () => {
      try {
        setMessage("");
        await rerunJob(job.id);
        await refreshJobs();
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "重试任务失败");
      }
    });
  };

  const handleOpenLogs = (job: JobItem) => {
    startTransition(async () => {
      try {
        setLogJob(job);
        setLogMessage("日志加载中...");
        const items = await listJobLogs(job.id);
        setLogs(items);
        setLogMessage(items.length === 0 ? "当前没有日志" : "");
      } catch (error) {
        setLogs([]);
        setLogMessage(error instanceof Error ? error.message : "加载日志失败");
      }
    });
  };

  const handleDelete = (job: JobItem) => {
    setDeleteConfirmJob(job);
  };

  const handleToggleSelectAll = () => {
    setSelectedJobIds((prev) => {
      if (selectableJobs.length === 0) {
        return prev;
      }
      if (selectableJobs.every((job) => prev.has(job.id))) {
        return new Set<number>();
      }
      return new Set(selectableJobs.map((job) => job.id));
    });
  };

  const handleToggleSelectJob = (jobID: number) => {
    setSelectedJobIds((prev) => {
      const next = new Set(prev);
      if (next.has(jobID)) {
        next.delete(jobID);
      } else {
        next.add(jobID);
      }
      return next;
    });
  };

  const handleStartEditNumber = (job: JobItem) => {
    setEditingJobId(job.id);
    setEditingNumber(requiresManualNumberReview(job) ? "" : job.number);
  };

  const handleCancelEditNumber = () => {
    setEditingJobId(null);
    setEditingNumber("");
  };

  const handleSaveNumber = (job: JobItem) => {
    startTransition(async () => {
      try {
        setMessage("");
        await updateJobNumber(job.id, editingNumber);
        handleCancelEditNumber();
        await refreshJobs();
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "更新影片 ID 失败");
      }
    });
  };

  const handleRunSelectedJobs = () => {
    const pendingJobs = jobs.filter((job) => selectedJobIds.has(job.id) && canSelectJob(job));
    if (pendingJobs.length === 0) {
      return;
    }
    startTransition(async () => {
      let success = 0;
      let failed = 0;
      try {
        setMessage("");
        for (const job of pendingJobs) {
          try {
            if (job.status === "failed") {
              await rerunJob(job.id);
            } else {
              await runJob(job.id);
            }
            success += 1;
          } catch {
            failed += 1;
          }
        }
        setSelectedJobIds(new Set());
        await refreshJobs();
        if (failed > 0) {
          setMessage(`已提交 ${success} 条，失败 ${failed} 条`);
          return;
        }
        setMessage(`已提交 ${success} 条任务`);
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "批量提交失败");
      }
    });
  };

  const handleCommitEditNumber = (job: JobItem) => {
    const nextNumber = editingNumber.trim();
    if (!nextNumber) {
      handleCancelEditNumber();
      return;
    }
    if (nextNumber === job.number.trim() && !requiresManualNumberReview(job)) {
      handleCancelEditNumber();
      return;
    }
    handleSaveNumber(job);
  };

  const getNumberMeta = (job: JobItem) => {
    if (job.number_source === "manual") {
      return {
        tone: "var(--info)",
        kind: "manual",
        warning: "用户已手动编辑影片 ID",
      };
    }
    if (job.number_clean_status === "success" && job.number_clean_confidence === "high") {
      return {
        tone: "var(--ok)",
        kind: "success",
        warning: "",
      };
    }
    if (job.number_clean_status === "success" && job.number_clean_confidence === "medium") {
      return {
        tone: "var(--warn)",
        kind: "warn",
        warning: job.number_clean_warnings || "",
      };
    }
    if (job.number_clean_status === "no_match" || job.number_clean_status === "low_quality" || job.number_clean_confidence === "low") {
      return {
        tone: "var(--danger)",
        kind: "danger",
        warning: job.number_clean_warnings || "清洗失败，当前使用原始值",
      };
    }
    return {
      tone: "var(--warn)",
      kind: "warn",
      warning: job.number_clean_warnings || "",
    };
  };

  const requiresManualNumberReview = (job: JobItem) => {
    if (job.number_source === "manual") {
      return false;
    }
    if (job.number_clean_status === "no_match" || job.number_clean_status === "low_quality") {
      return true;
    }
    return job.number_clean_confidence === "low";
  };

  const getNumberHint = (job: JobItem) => {
    if (job.conflict_reason) {
      return "目标文件名冲突，需先处理";
    }
    if (job.number_source === "manual") {
      return "已手动确认";
    }
    if (job.number_clean_status === "success" && job.number_clean_confidence === "high") {
      return "高置信度，可直接提交";
    }
    if (job.number_clean_status === "success" && job.number_clean_confidence === "medium") {
      return job.number_clean_warnings || "中等置信度，建议检查";
    }
    return job.number_clean_warnings || "需先手动修正影片 ID";
  };

  const getPathSegments = (job: JobItem) => {
    const segments = job.rel_path.split("/").filter(Boolean);
    return {
      folder: segments.length > 1 ? segments.slice(0, -1).join(" / ") : "根目录",
      name: segments.length > 0 ? segments[segments.length - 1] : job.rel_path,
    };
  };

  const renderNumberStatusIcon = (job: JobItem) => {
    const meta = getNumberMeta(job);
    const baseStyle = {
      width: 24,
      height: 24,
      minWidth: 24,
      borderRadius: 999,
      border: `1.5px solid ${meta.tone}`,
      color: meta.tone,
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      flexShrink: 0,
    } as const;
    if (meta.kind === "success") {
      return (
        <span style={baseStyle} title="清洗成功，高置信度">
          <Check size={14} strokeWidth={2.4} />
        </span>
      );
    }
    if (meta.kind === "manual") {
      return (
        <span style={baseStyle} title={meta.warning}>
          <Edit3 size={13} strokeWidth={2.2} />
        </span>
      );
    }
    if (meta.kind === "warn") {
      return (
        <span style={baseStyle} title="清洗成功，中等置信度">
          <span style={{ fontSize: 14, fontWeight: 700, lineHeight: 1 }}>!</span>
        </span>
      );
    }
    return (
      <span style={baseStyle} title={meta.warning || "清洗失败或低置信度"}>
        <X size={14} strokeWidth={2.4} />
      </span>
    );
  };

  const confirmDelete = (job: JobItem) => {
    setDeleteConfirmJob(null);
    startTransition(async () => {
      try {
        setMessage("");
        await deleteJob(job.id);
        await refreshJobs();
        if (logJob?.id === job.id) {
          setLogJob(null);
          setLogs([]);
          setLogMessage("");
        }
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "删除任务失败");
      }
    });
  };

  return (
    <>
      <div className="panel file-list-panel">
        <div className="file-list-hero">
          <div className="file-list-hero-copy">
            <div className="file-list-eyebrow">Processing Queue</div>
            <h2 className="file-list-title">文件列表</h2>
            <p className="file-list-subtitle">
              当前展示 {jobs.length} 条记录，共 {total} 条任务。优先处理低置信度影片 ID，运行中的状态会自动刷新。
            </p>
          </div>
          <div className="file-list-stats">
            {summaryCards.map((item) => (
              <button
                key={item.label}
                type="button"
                className="file-list-stat-card"
                data-tone={item.tone}
                data-active={statusFilter === item.filter}
                onClick={() => setStatusFilter(item.filter)}
              >
                <span className="file-list-stat-label">{item.label}</span>
                <strong className="file-list-stat-value">{item.value}</strong>
                <span className="file-list-stat-hint">{item.hint}</span>
              </button>
            ))}
          </div>
        </div>

        <div className="file-list-toolbar">
          <label className="file-list-search">
            <Search size={16} />
            <input
              className="input file-list-search-input"
              placeholder="按文件名 / 路径 / 影片 ID 搜索"
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
            />
          </label>
          <div className="file-list-toolbar-actions">
            <button className="btn btn-primary" onClick={handleScan} disabled={isPending || isScanning}>
              <RefreshCw size={16} className={isScanning ? "media-library-sync-icon-spinning" : ""} />
              {isScanning ? "扫描中..." : "立即扫描"}
            </button>
          </div>
        </div>
        <div className="file-list-chip-row" aria-label="状态快捷筛选">
          {filterChips.map((item) => (
            <button
              key={item.value}
              type="button"
              className="file-list-chip"
              data-active={statusFilter === item.value}
              onClick={() => setStatusFilter(item.value)}
            >
              <span>{item.label}</span>
              <span className="file-list-chip-count">{item.count}</span>
            </button>
          ))}
        </div>
        <div className="table-wrap" style={{ position: "relative", flex: 1, overflow: "auto" }}>
          {isScanning ? (
            <div className="list-loading-overlay">
              <div className="list-loading-spinner" />
            </div>
          ) : null}
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
                      onChange={handleToggleSelectAll}
                    />
                    <span>Path</span>
                    <button
                      className="btn file-batch-submit-btn"
                      data-visible={selectedCount > 0}
                      onClick={handleRunSelectedJobs}
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
              {jobs.map((job) => {
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
                return (
                  <tr key={job.id} data-selected={selectedJobIds.has(job.id)} data-status={job.status}>
                    <td data-label="文件" style={{ minWidth: 280 }}>
                      <div className="file-path-cell">
                        <input
                          type="checkbox"
                          checked={selectedJobIds.has(job.id)}
                          disabled={!hasHydrated || !canSelectJob(job) || isPending}
                          title={!canSelectJob(job) ? "中高置信度或手动编辑后的影片 ID 可加入批量提交" : "选择任务"}
                          onChange={() => handleToggleSelectJob(job.id)}
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
                          {editingJobId === job.id ? (
                            <div className="file-number-editing">
                              {renderNumberStatusIcon(job)}
                              <input
                                className="input"
                                style={{ width: "100%", minWidth: 0, height: 40, boxSizing: "border-box", padding: "0 12px" }}
                                value={editingNumber}
                                placeholder="请输入确认后的影片 ID"
                                autoFocus
                                onChange={(e) => setEditingNumber(e.target.value)}
                                onBlur={() => handleCommitEditNumber(job)}
                                onKeyDown={(e) => {
                                  if (e.key === "Enter") {
                                    e.preventDefault();
                                    handleCommitEditNumber(job);
                                  }
                                  if (e.key === "Escape") {
                                    e.preventDefault();
                                    handleCancelEditNumber();
                                  }
                                }}
                              />
                            </div>
                          ) : (
                            <div className="file-number-display">
                              {renderNumberStatusIcon(job)}
                              <div className="file-number-copy">
                                <span className="file-number-value" title={requiresManualNumberReview(job) ? "待手动确认" : job.number}>
                                  {requiresManualNumberReview(job) ? "待确认" : job.number}
                                </span>
                                <span className="file-number-note">{getNumberHint(job)}</span>
                              </div>
                            </div>
                          )}
                        </div>
                        {canEditNumber ? (
                          editingJobId === job.id ? (
                            <button
                              className="btn"
                              onMouseDown={(e) => e.preventDefault()}
                              onClick={handleCancelEditNumber}
                              disabled={isPending}
                            >
                              <X size={16} />
                            </button>
                          ) : (
                            <button className="btn" onClick={() => handleStartEditNumber(job)} disabled={isPending}>
                              <Pencil size={16} />
                            </button>
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
                            onClick={() => handleOpenLogs(job)}
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
                        <span className="file-inline-muted">{job.status === "processing" ? "运行中自动刷新" : "最近一次状态变更"}</span>
                      </div>
                    </td>
                    <td data-label="操作" style={{ width: 230 }}>
                      <div className="file-actions-cell">
                        {canRerun ? (
                          <button className="btn file-action-btn" onClick={() => handleRerun(job)} disabled={rerunDisabled} title={runTitle}>
                            <RotateCcw size={16} />
                            重试
                          </button>
                        ) : (
                          <button className="btn file-action-btn" onClick={() => handleRun(job)} disabled={runDisabled} title={runTitle}>
                            <Sparkles size={16} />
                            提交
                          </button>
                        )}
                        <button className="btn file-action-btn file-action-btn-ghost" onClick={() => handleDelete(job)} disabled={!canDelete || isPending}>
                          <Trash2 size={16} />
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
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
        <div className="review-preview-overlay" onClick={() => setLogJob(null)}>
          <div className="panel file-log-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="file-log-head">
              <div>
                <div className="file-log-kicker">Task Trace</div>
                <h3 className="file-log-title">任务日志 #{logJob.id}</h3>
                <div className="file-log-path">{logJob.rel_path}</div>
              </div>
              <button className="btn" onClick={() => setLogJob(null)}>
                <X size={16} />
              </button>
            </div>
            {logMessage ? <div className="file-log-message">{logMessage}</div> : null}
            <div className="file-log-list">
              {logs.map((item) => (
                <div key={item.id} className="file-log-item">
                  <div className="file-log-meta">
                    <span>{formatUnixMillis(item.created_at)}</span>
                    <span className="file-log-pill">{item.level}</span>
                    <span className="file-log-pill">{item.stage}</span>
                  </div>
                  <div className="file-log-text">{item.message}</div>
                  {item.detail ? <div className="file-log-detail">{item.detail}</div> : null}
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : null}
      {deleteConfirmJob ? (
        <div className="review-preview-overlay" onClick={() => setDeleteConfirmJob(null)}>
          <div className="panel review-confirm-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="review-confirm-title">确认删除</div>
            <div className="review-confirm-body">
              这会删除当前任务以及对应的源文件。
              <br />
              <span className="review-confirm-path">{deleteConfirmJob.rel_path}</span>
            </div>
            <div className="review-confirm-actions">
              <button type="button" className="btn" onClick={() => setDeleteConfirmJob(null)}>
                取消
              </button>
              <button type="button" className="btn btn-primary" onClick={() => confirmDelete(deleteConfirmJob)} disabled={isPending}>
                删除
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
