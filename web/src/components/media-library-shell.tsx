"use client";

import { useState } from "react";

import { MediaLibraryShellMain } from "@/components/media-library-shell-main";
import { ErrorState } from "@/components/ui/error-state";
import type { MediaLibraryItem, MediaLibraryStatus } from "@/lib/api";
import { listMediaLibraryItems } from "@/lib/api";

interface Props {
  items: MediaLibraryItem[];
  initialStatus: MediaLibraryStatus | null;
  initialError?: string | null;
}

// MediaLibraryShell: /media-library 顶层壳. 只承担 "Server Component 注入
// 的 (initialData, initialError) → ErrorState / Main 路由切换" 这一职责.
// 主体内容编排在 ./media-library-shell-main.tsx (太大无法纳入 vitest
// coverage include, 由用户路径 / E2E 覆盖).
//
// 重试不再走 window.location.reload() (会丢滚动 / 筛选 / 浏览器 history),
// 改成客户端拉新数据然后清 error, 让 Main 以新 props mount. 与
// ProcessingShell / ReviewShell / LibraryShell 行为对齐.
export function MediaLibraryShell({ items: initialItems, initialStatus, initialError }: Props) {
  const [data, setData] = useState({ items: initialItems, initialStatus });
  const [error, setError] = useState<string | null>(initialError ?? null);
  const [retrying, setRetrying] = useState(false);

  if (error) {
    const handleRetry = async () => {
      setRetrying(true);
      try {
        const items = await listMediaLibraryItems({ sort: "ingested", order: "desc" });
        setData({ items, initialStatus: data.initialStatus });
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "重新加载媒体库失败");
      } finally {
        setRetrying(false);
      }
    };
    return (
      <div className="media-library-page media-library-page-wide">
        <section className="panel media-library-overview media-library-overview-wide">
          <ErrorState
            title="加载媒体库失败"
            detail={error}
            onRetry={handleRetry}
            retryLabel={retrying ? "重试中..." : "重试"}
          />
        </section>
      </div>
    );
  }

  return <MediaLibraryShellMain items={data.items} initialStatus={data.initialStatus} />;
}
