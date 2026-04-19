import { cn } from "@/lib/utils";

// Spinner: 项目公共加载圈, 统一 .list-loading-spinner 及 overlay 两种形态.
// size 映射 globals.css 里已有的视觉规格:
//   sm  → 16px (内联在 row / badge 旁)
//   md  → 24px (卡片内部 / modal 中等空间)
//   lg  → 34px (默认, 覆盖层 / 空 panel)
// overlay=true 时额外包一层 .list-loading-overlay, 绝对定位覆盖父容器 —
// 父必须是 position:relative/absolute/fixed, 否则会冒出去. 详见 §4.5.

export type SpinnerSize = "sm" | "md" | "lg";

export interface SpinnerProps {
  size?: SpinnerSize;
  overlay?: boolean;
  className?: string;
  // aria-label: 默认读作 "加载中". 场景需要更精确 (如 "同步中")
  // 时通过 label 覆盖, 配合屏幕阅读器提示更具体的忙碌原因.
  label?: string;
}

const SIZE_STYLE: Record<SpinnerSize, { width: number; height: number; border: number }> = {
  sm: { width: 16, height: 16, border: 2 },
  md: { width: 24, height: 24, border: 2.5 },
  lg: { width: 34, height: 34, border: 3 },
};

export function Spinner({ size = "lg", overlay = false, className, label = "加载中" }: SpinnerProps) {
  const sizeStyle = SIZE_STYLE[size];
  const spinner = (
    <div
      className={cn("list-loading-spinner", className)}
      style={{
        width: sizeStyle.width,
        height: sizeStyle.height,
        borderWidth: sizeStyle.border,
      }}
      role="status"
      aria-label={label}
    />
  );

  if (!overlay) {
    return spinner;
  }

  return (
    <div className="list-loading-overlay" aria-live="polite">
      {spinner}
    </div>
  );
}
