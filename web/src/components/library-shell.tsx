"use client";

import { useState } from "react";

import { LibraryShellMain } from "@/components/library-shell-main";
import { ErrorState } from "@/components/ui/error-state";
import type { LibraryDetail, LibraryListItem, MediaLibraryStatus } from "@/lib/api";
import { getLibraryItem, listLibraryItems } from "@/lib/api";

interface Props {
  items: LibraryListItem[];
  initialDetail: LibraryDetail | null;
  initialMediaStatus: MediaLibraryStatus | null;
  initialError?: string | null;
}

// LibraryShell: /library 顶层壳. 只承担 "Server Component 注入的
// (initialData, initialError) → ErrorState / Main 路由切换" 这一职责.
// 主体内容编排在 ./library-shell-main.tsx, 由 useLibraryAssetActions /
// useLibraryMoveRefresh / utils 单测 + E2E 共同覆盖.
//
// 重试不再走 window.location.reload() (会丢滚动 / 筛选 / 浏览器 history),
// 改成客户端拉新数据然后清 error, 让 Main 以新 props mount. 与
// ProcessingShell / ReviewShell / MediaLibraryShell 行为对齐.
export function LibraryShell({ items: initialItems, initialDetail, initialMediaStatus, initialError }: Props) {
  const [data, setData] = useState({
    items: initialItems,
    initialDetail,
    initialMediaStatus,
  });
  const [error, setError] = useState<string | null>(initialError ?? null);
  const [retrying, setRetrying] = useState(false);

  if (error) {
    const handleRetry = async () => {
      setRetrying(true);
      try {
        const items = await listLibraryItems();
        let detail: LibraryDetail | null = null;
        if (items.length > 0) {
          try {
            detail = await getLibraryItem(items[0].rel_path);
          } catch {
            detail = null;
          }
        }
        setData({ items, initialDetail: detail, initialMediaStatus: data.initialMediaStatus });
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "重新加载已入库列表失败");
      } finally {
        setRetrying(false);
      }
    };
    return (
      <div className="panel library-shell">
        <ErrorState
          title="加载已入库列表失败"
          detail={error}
          onRetry={handleRetry}
          retryLabel={retrying ? "重试中..." : "重试"}
        />
      </div>
    );
  }

  return (
    <LibraryShellMain
      items={data.items}
      initialDetail={data.initialDetail}
      initialMediaStatus={data.initialMediaStatus}
    />
  );
}
