"use client";

import { useEffect } from "react";

import { ErrorState } from "@/components/ui/error-state";

// app-level error boundary: route-level Server Component 抛出 / 渲染期
// 抛错 / Promise reject 等场景的最后防线. Server Component initial-loaders
// 已经把绝大多数预期错误降级成 ErrorState; 这里只兜不可预期的崩溃,
// 仍然保留 warm serif 视觉壳, 不让用户看到 Next 默认错误页.
//
// reset() 是 Next 提供的回调, 调用后 Next 会在客户端重新尝试渲染当前
// segment; 我们再额外允许用户回主页, 防止反复 reset 仍崩溃时陷在死循环.
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    if (typeof console !== "undefined") {
      console.error("[yamdc] global error boundary:", error);
    }
  }, [error]);

  return (
    <div className="panel" style={{ margin: 24, padding: 24 }}>
      <ErrorState
        title="页面渲染失败"
        detail={error.message || "未知错误, 请重试或返回主页"}
        onRetry={reset}
        retryLabel="重新尝试"
      />
    </div>
  );
}
