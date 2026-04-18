"use client";

import { useEffect, useTransition } from "react";

import type { JobItem, JobLogItem } from "@/lib/api";
import { deleteJob, listJobs, listJobLogs, rerunJob, runJob, triggerScan, updateJobNumber } from "@/lib/api";

import { STATUS_FILTER, canSelectJob, requiresManualNumberReview } from "./helpers";

// useJobActions 把原来堆在主 JobTable 组件里的 ~15 个 handler + 轮询 /
// 防抖副作用 抽成一个 hook. 调用方通过入参交出状态与 setter, hook 再
// 把副作用 + 动作闭包一次性返还. 好处:
//   1. 主文件从 488 行降到 ≤400 行, 集中在 "状态声明 + 派生 + JSX" 三段.
//   2. hook 自带 isPending (useTransition), 消费方不用再单独持有.
//   3. handler 内部共享的 refreshJobs / handleCancelEditNumber 作为闭包
//      内私有逻辑, 对消费方无需暴露, 接口更收敛.
export interface JobActionsDeps {
  jobs: JobItem[];
  keyword: string;
  resolvedStatusFilter: string;
  selectedJobIds: Set<number>;
  editingNumber: string;
  logJob: JobItem | null;
  selectableJobs: JobItem[];
  setAllJobs: (items: JobItem[]) => void;
  setTotal: (total: number) => void;
  setMessage: (msg: string) => void;
  setIsScanning: (v: boolean) => void;
  setLogJob: (v: JobItem | null) => void;
  setLogs: (v: JobLogItem[]) => void;
  setLogMessage: (v: string) => void;
  setDeleteConfirmJob: (v: JobItem | null) => void;
  setEditingJobId: (v: number | null) => void;
  setEditingNumber: (v: string) => void;
  setSelectedJobIds: (updater: (prev: Set<number>) => Set<number>) => void;
}

export interface JobActions {
  isPending: boolean;
  handleScan: () => void;
  handleRun: (job: JobItem) => void;
  handleRerun: (job: JobItem) => void;
  handleOpenLogs: (job: JobItem) => void;
  handleDelete: (job: JobItem) => void;
  handleToggleSelectAll: () => void;
  handleToggleSelectJob: (jobID: number) => void;
  handleStartEditNumber: (job: JobItem) => void;
  handleCancelEditNumber: () => void;
  handleCommitEditNumber: (job: JobItem) => void;
  handleRunSelectedJobs: () => void;
  confirmDelete: (job: JobItem) => void;
}

export function useJobActions(deps: JobActionsDeps): JobActions {
  const {
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
  } = deps;
  const [isPending, startTransition] = useTransition();

  const refreshJobs = async (nextKeyword = keyword) => {
    const data = await listJobs({
      status: STATUS_FILTER,
      all: true,
      keyword: nextKeyword,
    });
    setAllJobs(data.items);
    setTotal(data.total);
  };

  // 8 秒轮询. 每次重新触发前 abort 上一次的 in-flight 请求,
  // 防止网络慢时旧响应覆盖新响应造成 UI 闪回.
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
    // resolvedStatusFilter 加进来是为了切 filter 时立即重置轮询基线,
    // keyword 同理. 只依赖 keyword 会导致切 filter 时下一次轮询仍走旧 keyword.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keyword, resolvedStatusFilter]);

  // 250ms 防抖的关键字搜索. 用 useTransition 包裹请求, 让输入不阻塞 UI.
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      startTransition(async () => {
        try {
          const data = await listJobs(
            { status: STATUS_FILTER, all: true, keyword },
            controller.signal,
          );
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keyword, resolvedStatusFilter]);

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
        setSelectedJobIds(() => new Set());
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

  return {
    isPending,
    handleScan,
    handleRun,
    handleRerun,
    handleOpenLogs,
    handleDelete,
    handleToggleSelectAll,
    handleToggleSelectJob,
    handleStartEditNumber,
    handleCancelEditNumber,
    handleCommitEditNumber,
    handleRunSelectedJobs,
    confirmDelete,
  };
}
