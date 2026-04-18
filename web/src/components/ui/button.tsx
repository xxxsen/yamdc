import * as React from "react";
import { LoaderCircle } from "lucide-react";

import { cn } from "@/lib/utils";

// Button: 项目公共按钮原子。第一版只支持 primary / secondary 两档,
// 对应 globals.css 现有的 .btn / .btn-primary。其它散装变体 (ghost/
// danger/sm/icon) 暂不内置, 走 className 透传兜底 — 等真实 consumer
// 出现明确需求再提升为 variant, 避免 API 过早扩张。
// 详见 td/022-frontend-optimization-roadmap.md §3.1。

export type ButtonVariant = "primary" | "secondary";

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  loading?: boolean;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  function Button(
    {
      variant = "secondary",
      loading = false,
      leftIcon,
      rightIcon,
      disabled,
      className,
      children,
      type,
      ...rest
    },
    ref,
  ) {
    // variant=secondary 映射为纯 .btn (无 modifier), 和现有代码库里
    // "className={btn btn-secondary}" 但 CSS 无 .btn-secondary 规则的
    // 历史约定对齐 — 视觉零回退。
    const variantClass = variant === "primary" ? "btn-primary" : null;

    // loading 态语义上等同 disabled, 但保留区分: loading 显示 spinner,
    // disabled 不显示 — 避免"置灰但看不出原因"。
    const leading = loading ? (
      <LoaderCircle
        size={16}
        className="ui-button-spinner"
        aria-hidden="true"
      />
    ) : (
      leftIcon
    );

    return (
      <button
        ref={ref}
        type={type ?? "button"}
        className={cn("btn", variantClass, className)}
        disabled={disabled || loading}
        aria-busy={loading || undefined}
        {...rest}
      >
        {leading}
        {children}
        {rightIcon}
      </button>
    );
  },
);
