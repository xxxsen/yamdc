"use client";

import { Check, Edit3, Eye, Pencil, RefreshCw, RotateCcw, Sparkles, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState, useTransition } from "react";

import type { JobItem, JobListResponse, JobLogItem } from "@/lib/api";
import { deleteJob, listJobs, listJobLogs, rerunJob, runJob, triggerScan, updateJobNumber } from "@/lib/api";
import { formatBytes, formatUnixMillis } from "@/lib/utils";
import { StatusBadge } from "./status-badge";

interface Props {
  initialData: JobListResponse;
}

const STATUS_FILTER = "init,processing,failed,reviewing";

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
  const selectAllRef = useRef<HTMLInputElement | null>(null);

  const resolvedStatusFilter = statusFilter === "all" ? STATUS_FILTER : statusFilter;

  const counts = useMemo(() => {
    return allJobs.reduce<Record<string, number>>((acc, item) => {
      acc[item.status] = (acc[item.status] ?? 0) + 1;
      return acc;
    }, {});
  }, [allJobs]);

  const jobs = useMemo(() => {
    if (statusFilter === "all") {
      return allJobs;
    }
    return allJobs.filter((item) => item.status === statusFilter);
  }, [allJobs, statusFilter]);

  const canSelectJob = (job: JobItem) => {
    const hasApprovedNumber =
      job.number_source === "manual" || (job.number_clean_status === "success" && job.number_clean_confidence === "high");
    const isRunnable = job.status === "init" || job.status === "failed";
    return hasApprovedNumber && isRunnable;
  };

  const selectableJobs = useMemo(() => jobs.filter(canSelectJob), [jobs]);
  const selectedCount = selectedJobIds.size;
  const allSelectableChecked = selectableJobs.length > 0 && selectableJobs.every((job) => selectedJobIds.has(job.id));
  const hasPartialSelection = selectedCount > 0 && !allSelectableChecked;

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
    const timer = window.setInterval(() => {
      void listJobs({
        status: STATUS_FILTER,
        all: true,
        keyword,
      })
        .then((data) => {
          setAllJobs(data.items);
          setTotal(data.total);
        })
        .catch(() => undefined);
    }, 8000);
    return () => window.clearInterval(timer);
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
    const timer = window.setTimeout(() => {
      startTransition(async () => {
        try {
          const data = await listJobs({
            status: STATUS_FILTER,
            all: true,
            keyword,
          });
          setAllJobs(data.items);
          setTotal(data.total);
        } catch (error) {
          setMessage(error instanceof Error ? error.message : "查询失败");
        }
      });
    }, 250);
    return () => window.clearTimeout(timer);
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
    setEditingNumber(job.number);
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
        setMessage(error instanceof Error ? error.message : "更新番号失败");
      }
    });
  };

  const handleCommitEditNumber = (job: JobItem) => {
    if (!editingNumber.trim()) {
      handleCancelEditNumber();
      return;
    }
    if (editingNumber.trim() === job.number.trim()) {
      handleCancelEditNumber();
      return;
    }
    handleSaveNumber(job);
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

  const getNumberMeta = (job: JobItem) => {
    if (job.number_source === "manual") {
      return {
        tone: "var(--info)",
        kind: "manual",
        warning: "用户已手动编辑番号",
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
      <div className="panel" style={{ padding: 18, height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            gap: 16,
            flexWrap: "wrap",
            marginBottom: 16,
          }}
        >
          <div>
            <h2 style={{ margin: 0, fontSize: 24 }}>当前需处理的文件</h2>
            <p style={{ margin: "6px 0 0", color: "var(--muted)" }}>当前展示 {jobs.length} 条，共 {total} 条任务</p>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
            <input
              className="input"
              style={{ width: 240 }}
              placeholder="按文件名 / 路径 / 番号搜索"
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
            />
            <select
              className="input"
              style={{ width: 160 }}
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
            >
              <option value="all">全部状态 ({total})</option>
              <option value="init">Init ({counts.init ?? 0})</option>
              <option value="processing">Processing ({counts.processing ?? 0})</option>
              <option value="reviewing">Reviewing ({counts.reviewing ?? 0})</option>
              <option value="failed">Failed ({counts.failed ?? 0})</option>
            </select>
            {message ? <span style={{ color: "var(--danger)", fontSize: 14 }}>{message}</span> : null}
            <button className="btn" onClick={handleRunSelectedJobs} disabled={selectedCount === 0 || isPending}>
              提交已选 ({selectedCount})
            </button>
            <button className="btn btn-primary" onClick={handleScan} disabled={isPending}>
              <RefreshCw size={16} />
              立即扫描
            </button>
          </div>
        </div>
        <div className="table-wrap" style={{ position: "relative", flex: 1, overflow: "auto" }}>
          {isScanning ? (
            <div className="list-loading-overlay">
              <div className="list-loading-spinner" />
            </div>
          ) : null}
          <table className="table">
            <thead>
              <tr>
                <th>
                  <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                    <input
                      ref={selectAllRef}
                      type="checkbox"
                      checked={allSelectableChecked}
                      disabled={selectableJobs.length === 0 || isPending}
                      title="选择当前列表中所有可批量提交的文件"
                      onChange={handleToggleSelectAll}
                    />
                    <span>Path</span>
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
                const needsManualNumberReview = requiresManualNumberReview(job);
                const runDisabled = isPending || !canRun || needsManualNumberReview;
                const rerunDisabled = isPending || needsManualNumberReview;
                const runTitle = needsManualNumberReview ? "清洗失败或低置信度，需先手动编辑番号后才能刮削" : undefined;
                return (
                  <tr key={job.id}>
                    <td style={{ minWidth: 260, color: "var(--muted)" }}>
                      <div className="cell-center" style={{ gap: 10 }}>
                        <input
                          type="checkbox"
                          checked={selectedJobIds.has(job.id)}
                          disabled={!canSelectJob(job) || isPending}
                          title={!canSelectJob(job) ? "仅高置信度或手动编辑后的番号可加入批量提交" : "选择任务"}
                          onChange={() => handleToggleSelectJob(job.id)}
                        />
                        <span>{job.rel_path}</span>
                      </div>
                    </td>
                    <td style={{ width: 320 }}>
                      <div className="cell-center" style={{ justifyContent: "space-between", gap: 10, alignItems: "center" }}>
                        <div style={{ minWidth: 0, flex: 1 }}>
                          {editingJobId === job.id ? (
                            <div style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0, minHeight: 40 }}>
                              {renderNumberStatusIcon(job)}
                              <input
                                className="input"
                                style={{ width: "100%", minWidth: 0, height: 40, boxSizing: "border-box", padding: "0 12px" }}
                                value={editingNumber}
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
                            <div style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0 }}>
                              {renderNumberStatusIcon(job)}
                              <span
                                style={{
                                  minWidth: 0,
                                  overflow: "hidden",
                                  textOverflow: "ellipsis",
                                  whiteSpace: "nowrap",
                                }}
                                title={job.number}
                              >
                                {job.number}
                              </span>
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
                    <td>
                      <div className="cell-center">{formatBytes(job.file_size)}</div>
                    </td>
                    <td style={{ width: 120 }}>
                      <div className="cell-center">
                        <div className="status-chip">
                          <StatusBadge status={job.status} />
                          <button
                            className={`status-chip-view ${job.status === "failed" || job.error_msg ? "icon-btn-danger" : ""}`}
                            data-enabled={job.status !== "init"}
                            onClick={() => handleOpenLogs(job)}
                            disabled={isPending || job.status === "init"}
                            aria-label="查看日志"
                          >
                            <Eye size={14} />
                          </button>
                        </div>
                      </div>
                    </td>
                    <td style={{ width: 150 }}>
                      <div className="cell-center">{formatUnixMillis(job.updated_at)}</div>
                    </td>
                    <td style={{ width: 176 }}>
                      <div style={{ display: "flex", gap: 8, flexWrap: "nowrap", alignItems: "center" }}>
                        {canRerun ? (
                          <button className="btn" onClick={() => handleRerun(job)} disabled={rerunDisabled} title={runTitle}>
                            <RotateCcw size={16} />
                          </button>
                        ) : (
                          <button className="btn" onClick={() => handleRun(job)} disabled={runDisabled} title={runTitle}>
                            <Sparkles size={16} />
                          </button>
                        )}
                        <button className="btn" onClick={() => handleDelete(job)} disabled={!canDelete || isPending}>
                          <Trash2 size={16} />
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
      </div>
      {logJob ? (
        <div
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(24, 18, 12, 0.42)",
            display: "grid",
            placeItems: "center",
            padding: 20,
          }}
        >
          <div className="panel" style={{ width: "min(960px, 100%)", maxHeight: "80vh", overflow: "auto", padding: 18 }}>
            <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "center", marginBottom: 12 }}>
              <div>
                <h3 style={{ margin: 0 }}>任务日志 #{logJob.id}</h3>
                <div style={{ color: "var(--muted)", marginTop: 4 }}>{logJob.rel_path}</div>
              </div>
              <button className="btn" onClick={() => setLogJob(null)}>
                <X size={16} />
              </button>
            </div>
            {logMessage ? <div style={{ color: "var(--muted)", marginBottom: 12 }}>{logMessage}</div> : null}
            <div style={{ display: "grid", gap: 10 }}>
              {logs.map((item) => (
                <div key={item.id} style={{ border: "1px solid var(--line)", borderRadius: 14, padding: 12, background: "rgba(255,255,255,0.5)" }}>
                  <div style={{ display: "flex", gap: 10, flexWrap: "wrap", marginBottom: 6, color: "var(--muted)", fontSize: 13 }}>
                    <span>{formatUnixMillis(item.created_at)}</span>
                    <span>{item.level}</span>
                    <span>{item.stage}</span>
                  </div>
                  <div>{item.message}</div>
                  {item.detail ? <div style={{ marginTop: 6, color: "var(--muted)", fontSize: 13 }}>{item.detail}</div> : null}
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
