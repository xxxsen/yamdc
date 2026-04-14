"use client";

import dynamic from "next/dynamic";

import type { JobListResponse } from "@/lib/api";

const JobTable = dynamic(() => import("@/components/job-table").then((mod) => mod.JobTable), {
  ssr: false,
  loading: () => (
    <div className="panel file-list-panel">
      <div className="file-list-hero">
        <div className="file-list-hero-copy">
          <div className="file-list-eyebrow">Processing Queue</div>
          <h2 className="file-list-title">文件列表</h2>
          <p className="file-list-subtitle">正在加载任务列表...</p>
        </div>
      </div>
    </div>
  ),
});

export function ProcessingShell({ initialData }: { initialData: JobListResponse }) {
  return <JobTable initialData={initialData} />;
}
