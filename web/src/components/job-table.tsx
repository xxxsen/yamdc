"use client";

import { Play, RefreshCw, RotateCcw, ScrollText, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useState, useTransition } from "react";

import type { JobItem, JobListResponse, JobLogItem } from "@/lib/api";
import { deleteJob, listJobs, listJobLogs, rerunJob, runJob, triggerScan } from "@/lib/api";
import { formatBytes, formatUnixMillis } from "@/lib/utils";
import { StatusBadge } from "./status-badge";

interface Props {
  initialData: JobListResponse;
}

const STATUS_FILTER = "init,processing,failed,reviewing";

export function JobTable({ initialData }: Props) {
  const [jobs, setJobs] = useState(initialData.items);
  const [total, setTotal] = useState(initialData.total);
  const [keyword, setKeyword] = useState("");
  const [message, setMessage] = useState<string>("");
  const [isPending, startTransition] = useTransition();
  const [logJob, setLogJob] = useState<JobItem | null>(null);
  const [logs, setLogs] = useState<JobLogItem[]>([]);
  const [logMessage, setLogMessage] = useState("");

  const counts = useMemo(() => {
    return jobs.reduce<Record<string, number>>((acc, item) => {
      acc[item.status] = (acc[item.status] ?? 0) + 1;
      return acc;
    }, {});
  }, [jobs]);

  const refreshJobs = async (nextKeyword = keyword) => {
    const data = await listJobs({
      status: STATUS_FILTER,
      all: true,
      keyword: nextKeyword,
    });
    setJobs(data.items);
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
          setJobs(data.items);
          setTotal(data.total);
        })
        .catch(() => undefined);
    }, 8000);
    return () => window.clearInterval(timer);
  }, [keyword]);

  const handleScan = () => {
    startTransition(async () => {
      try {
        setMessage("扫描中...");
        await triggerScan();
        await refreshJobs();
        setMessage("扫描完成");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "扫描失败");
      }
    });
  };

  const handleSearch = () => {
    startTransition(async () => {
      try {
        setMessage("查询中...");
        await refreshJobs(keyword);
        setMessage("");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "查询失败");
      }
    });
  };

  const handleRun = (job: JobItem) => {
    startTransition(async () => {
      try {
        setMessage(`启动任务 #${job.id}...`);
        await runJob(job.id);
        await refreshJobs();
        setMessage(`任务 #${job.id} 已进入 processing`);
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "启动任务失败");
      }
    });
  };

  const handleRerun = (job: JobItem) => {
    startTransition(async () => {
      try {
        setMessage(`重试任务 #${job.id}...`);
        await rerunJob(job.id);
        await refreshJobs();
        setMessage(`任务 #${job.id} 已重新进入 processing`);
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
    const ok = window.confirm(`确认删除文件并移除任务吗？\n\n${job.rel_path}`);
    if (!ok) {
      return;
    }
    startTransition(async () => {
      try {
        setMessage(`删除任务 #${job.id}...`);
        await deleteJob(job.id);
        await refreshJobs();
        if (logJob?.id === job.id) {
          setLogJob(null);
          setLogs([]);
          setLogMessage("");
        }
        setMessage(`任务 #${job.id} 已删除`);
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
            <p style={{ margin: "6px 0 0", color: "var(--muted)" }}>
              当前展示 {jobs.length} 条，共 {total} 条任务，init {counts.init ?? 0}，processing {counts.processing ?? 0}，failed {counts.failed ?? 0}，
              reviewing {counts.reviewing ?? 0}
            </p>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
            <input
              className="input"
              style={{ width: 240 }}
              placeholder="按文件名 / 路径 / 番号搜索"
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
            />
            <button className="btn" onClick={handleSearch} disabled={isPending}>
              查询
            </button>
            {message ? <span style={{ color: "var(--muted)", fontSize: 14 }}>{message}</span> : null}
            <button className="btn btn-primary" onClick={handleScan} disabled={isPending}>
              <RefreshCw size={16} />
              立即扫描
            </button>
          </div>
        </div>
        <div className="table-wrap" style={{ flex: 1, overflow: "auto" }}>
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Path</th>
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
                return (
                  <tr key={job.id}>
                    <td style={{ minWidth: 180 }}>
                      <div className="cell-center">{job.file_name}</div>
                    </td>
                    <td style={{ minWidth: 260, color: "var(--muted)" }}>
                      <div className="cell-center">{job.rel_path}</div>
                    </td>
                    <td>
                      <div className="cell-center">{job.number}</div>
                    </td>
                    <td>
                      <div className="cell-center">{formatBytes(job.file_size)}</div>
                    </td>
                    <td style={{ width: 120 }}>
                      <div className="cell-center">
                        <StatusBadge status={job.status} />
                      </div>
                    </td>
                    <td style={{ width: 150 }}>
                      <div className="cell-center">{formatUnixMillis(job.updated_at)}</div>
                    </td>
                    <td style={{ width: 176 }}>
                      <div style={{ display: "flex", gap: 8, flexWrap: "nowrap", alignItems: "center" }}>
                        {canRerun ? (
                          <button className="btn" onClick={() => handleRerun(job)} disabled={isPending}>
                            <RotateCcw size={16} />
                          </button>
                        ) : (
                          <button className="btn" onClick={() => handleRun(job)} disabled={!canRun || isPending}>
                            <Play size={16} />
                          </button>
                        )}
                        <button
                          className={`btn ${job.status === "failed" || job.error_msg ? "icon-btn-danger" : ""}`}
                          onClick={() => handleOpenLogs(job)}
                          disabled={isPending}
                        >
                          <ScrollText size={16} />
                        </button>
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
                  <td colSpan={7} style={{ color: "var(--muted)", textAlign: "center", padding: 28 }}>
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
    </>
  );
}
