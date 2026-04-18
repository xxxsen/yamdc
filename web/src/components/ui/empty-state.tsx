import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

// EmptyState: "当前没有数据" 的统一视觉原子.
//
// 三种变体应对项目里实际出现的场景:
//   block   → 默认, 带虚线边框 + 居中文案, 用在 panel 内部留白.
//             对齐现有 .review-empty-state 视觉.
//   inline  → 无边框无填充, 只是灰色文本, 用在 token chip 列表 / 表头等
//             "上下文已经有容器" 的位置. 对齐 .library-inline-muted.
//   compact → 紧凑块, 居中文案但去掉边框, 用在 Modal 内的小区域 (比如
//             同步日志无数据). 对齐 .review-empty-state 但 padding 更小.
//
// icon / action 都是可选, 没传就只渲染文本; 最低使用成本:
//   <EmptyState title="暂无数据" />

export type EmptyStateVariant = "block" | "inline" | "compact";

export interface EmptyStateProps {
  title: string;
  hint?: string;
  icon?: ReactNode;
  action?: ReactNode;
  variant?: EmptyStateVariant;
  className?: string;
}

const VARIANT_CLASSES: Record<EmptyStateVariant, string> = {
  // block: 复用 globals.css .review-empty-state 的虚线边框 + centered layout
  block: "review-empty-state",
  // inline: 复用 .library-inline-muted (只是灰色文本, 无边框)
  inline: "library-inline-muted",
  // compact: 手工组合 — 虚线弱化 + 小 padding; 走 utility 让 Tailwind 层胜出.
  // 具体尺寸用 padding 12px, 其它随 review-empty-state 的边框/圆角复用.
  compact: "review-empty-state",
};

export function EmptyState({
  title,
  hint,
  icon,
  action,
  variant = "block",
  className,
}: EmptyStateProps) {
  if (variant === "inline") {
    return (
      <span className={cn(VARIANT_CLASSES.inline, className)} role="status">
        {title}
      </span>
    );
  }

  return (
    <div
      className={cn(VARIANT_CLASSES[variant], className)}
      role="status"
      aria-live="polite"
      style={variant === "compact" ? { padding: "12px 16px" } : undefined}
    >
      {icon ? <div className="empty-state-icon">{icon}</div> : null}
      <div>{title}</div>
      {hint ? <div className="empty-state-hint">{hint}</div> : null}
      {action ? <div className="empty-state-action">{action}</div> : null}
    </div>
  );
}
