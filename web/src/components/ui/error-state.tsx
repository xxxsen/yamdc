import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { Button } from "./button";

// ErrorState: "加载/保存失败" 的统一视觉原子.
//
// 项目里此前没有专门的错误块组件, 错误全部走 inline 文本 + 红色.
// 抽这个原子目的是: 给用户 "失败原因 + 重试入口" 的一致入口, 避免每个
// shell 各写一版 "错误信息 + 重试按钮".
//
// 不做视觉框 (border / background), 保持与 EmptyState 的对称 —— 如果
// 场景需要放在 panel 里, 由父层决定容器; 本原子只负责内容渲染.
//
// onRetry 存在就自动渲染"重试"按钮; retryLabel 可覆盖文案.

export interface ErrorStateProps {
  title?: string;
  detail?: string;
  onRetry?: () => void;
  retryLabel?: string;
  icon?: ReactNode;
  className?: string;
}

export function ErrorState({
  title = "加载失败",
  detail,
  onRetry,
  retryLabel = "重试",
  icon,
  className,
}: ErrorStateProps) {
  return (
    <div
      className={cn("error-state", className)}
      role="alert"
      style={{
        padding: "16px",
        color: "var(--danger)",
        textAlign: "center",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: 8,
      }}
    >
      {icon ? <div className="error-state-icon">{icon}</div> : null}
      <div style={{ fontWeight: 600 }}>{title}</div>
      {detail ? (
        <div style={{ color: "var(--muted)", fontSize: 14 }}>{detail}</div>
      ) : null}
      {onRetry ? (
        <Button onClick={onRetry} type="button">
          {retryLabel}
        </Button>
      ) : null}
    </div>
  );
}
