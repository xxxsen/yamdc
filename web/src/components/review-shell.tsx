"use client";

import { useState } from "react";

import { ReviewShellMain } from "@/components/review-shell-main";
import { ErrorState } from "@/components/ui/error-state";
import type { JobItem, MediaLibraryStatus, ScrapeDataItem } from "@/lib/api";
import { getMediaLibraryStatus, getReviewJob, listJobs } from "@/lib/api";

interface Props {
  jobs: JobItem[];
  initialScrapeData: ScrapeDataItem | null;
  initialMediaStatus: MediaLibraryStatus | null;
  initialError?: string | null;
}

// ReviewShell: /review 顶层壳. 只承担 "Server Component 注入的
// (initialData, initialError) → ErrorState / Main 路由切换" 这一职责.
// 主体内容编排在 ./review-shell-main.tsx, 由 useReviewBatchActions /
// useReviewAssetActions / utils 单测 + E2E 共同覆盖.
//
// 重试不再走 window.location.reload() (会丢滚动 / 浏览器 history),
// 改成客户端拉新数据然后清 error, 让 Main 以新 props mount. 与
// ProcessingShell / LibraryShell / MediaLibraryShell 行为对齐.
export function ReviewShell({ jobs: initialJobs, initialScrapeData, initialMediaStatus, initialError }: Props) {
  const [data, setData] = useState({
    jobs: initialJobs,
    initialScrapeData,
    initialMediaStatus,
  });
  const [error, setError] = useState<string | null>(initialError ?? null);
  const [retrying, setRetrying] = useState(false);

  if (error) {
    const handleRetry = async () => {
      setRetrying(true);
      try {
        const list = await listJobs({ status: "reviewing", page: 1, pageSize: 200 });
        let scrape: ScrapeDataItem | null = null;
        if (list.items.length > 0) {
          try {
            scrape = await getReviewJob(list.items[0].id);
          } catch {
            scrape = null;
          }
        }
        let mediaStatus: MediaLibraryStatus | null = null;
        try {
          mediaStatus = await getMediaLibraryStatus();
        } catch {
          mediaStatus = null;
        }
        setData({ jobs: list.items, initialScrapeData: scrape, initialMediaStatus: mediaStatus });
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "重新加载待 review 列表失败");
      } finally {
        setRetrying(false);
      }
    };
    return (
      <div className="panel review-shell">
        <ErrorState
          title="加载待 review 列表失败"
          detail={error}
          onRetry={handleRetry}
          retryLabel={retrying ? "重试中..." : "重试"}
        />
      </div>
    );
  }

  return (
    <ReviewShellMain
      jobs={data.jobs}
      initialScrapeData={data.initialScrapeData}
      initialMediaStatus={data.initialMediaStatus}
    />
  );
}
