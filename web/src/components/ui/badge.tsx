import * as React from "react";

import { cn } from "@/lib/utils";

// Badge: 项目公共 badge 原子。沿用 globals.css 既有的 .badge +
// .badge-dot 胶囊布局, 颜色走 currentColor, 所以 variant 只需要
// 覆盖 color 变量即可。
//
// 已经存在的 .badge-init / .badge-processing / .badge-reviewing /
// .badge-done / .badge-failed 是按 JobStatus 语义命名的, 本组件
// 不直接使用 — JobStatus 专用的 StatusBadge 仍保留现有实现。
// 新增的 neutral / info / success / warning / danger 是通用语义
// variant, 对应 CSS 在 globals.css 里新增 alias (见 §badge-variants)。

export type BadgeVariant =
  | "neutral"
  | "info"
  | "success"
  | "warning"
  | "danger";

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
  dot?: boolean;
  children: React.ReactNode;
}

export function Badge({
  variant = "neutral",
  dot = false,
  className,
  children,
  ...rest
}: BadgeProps) {
  return (
    <span className={cn("badge", `badge-${variant}`, className)} {...rest}>
      {dot ? <span className="badge-dot" aria-hidden="true" /> : null}
      {children}
    </span>
  );
}
