"use client";

import dynamic from "next/dynamic";
import { useState } from "react";

import { ErrorState } from "@/components/ui/error-state";
import type { JobListResponse } from "@/lib/api";
import { listJobs } from "@/lib/api";

// JobTableLoadingFallback: dynamic() 在 ssr:false + JS chunk 还在路上时
// 渲染的占位 UI. 抽出独立 export 让单测可以直接渲染它而不必模拟 dynamic
// 实际进入 loading 态 (jsdom 下 dynamic 多数 path 会跳过 loading callback).
export function JobTableLoadingFallback() {
  return (
    <div className="panel file-list-panel">
      <div className="file-list-hero">
        <div className="file-list-hero-copy">
          <div className="file-list-eyebrow">Processing Queue</div>
          <h2 className="file-list-title">文件列表</h2>
          <p className="file-list-subtitle">正在加载任务列表...</p>
        </div>
      </div>
    </div>
  );
}

const JobTable = dynamic(() => import("@/components/job-table").then((mod) => mod.JobTable), {
  ssr: false,
  loading: () => <JobTableLoadingFallback />,
});

interface ProcessingShellProps {
  initialData: JobListResponse;
  initialError?: string | null;
}

// ProcessingShell: /processing 顶层壳. 初始数据由 Server Component 负责
// 拉取并以 (data, errorMessage) 形式注入. 失败时 shell 显示 ErrorState +
// 重试按钮; 重试走客户端 listJobs(), 拿到数据后切回正常视图.
export function ProcessingShell({ initialData, initialError }: ProcessingShellProps) {
  const [data, setData] = useState<JobListResponse>(initialData);
  const [error, setError] = useState<string | null>(initialError ?? null);
  const [retrying, setRetrying] = useState(false);

  if (error) {
    const handleRetry = async () => {
      setRetrying(true);
      try {
        const next = await listJobs({ status: "init,processing,failed,reviewing", all: true });
        setData(next);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "重新加载处理队列失败");
      } finally {
        setRetrying(false);
      }
    };
    return (
      <div className="panel file-list-panel">
        <ErrorState
          title="加载处理队列失败"
          detail={error}
          onRetry={handleRetry}
          retryLabel={retrying ? "重试中..." : "重试"}
        />
      </div>
    );
  }
  return <JobTable initialData={data} />;
}
