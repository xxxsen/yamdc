"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

// Modal: 项目公共弹层原子。第一版不引入动画/focus-trap, 聚焦于把
// 散落在各 shell 里的 Modal 行为 (portal / ESC / backdrop 点击 /
// body 滚动锁) 统一化, 把"每个 shell 自己 new 一份"的重复代码收敛。
//
// 样式上直接沿用 globals.css 既有的 .plugin-editor-modal-* 一套类
// (当前项目里最完整的一套 Modal 样式)。类名带 plugin-editor- 前缀
// 属历史遗留; 后续在 §class-rename 任务里再做 alias, 本轮不动。
// 详见 td/022-frontend-optimization-roadmap.md §3.1。

export interface ModalBadge {
  icon: React.ReactNode;
  label: string;
}

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle?: string;
  badge?: ModalBadge;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  closeOnBackdrop?: boolean;
  closeOnEscape?: boolean;
  closeAriaLabel?: string;
  ariaLabel?: string;
  disableClose?: boolean;
}

export function Modal({
  open,
  onClose,
  title,
  subtitle,
  badge,
  actions,
  children,
  className,
  closeOnBackdrop = true,
  closeOnEscape = true,
  closeAriaLabel = "关闭弹窗",
  ariaLabel,
  disableClose = false,
}: ModalProps) {
  // SSR 安全: createPortal 需要 document, 用 mounted 门闩避免
  // 服务端渲染阶段访问 document 报错。next.js app router 下组件
  // 默认会在 hydration 阶段重新运行, useEffect 触发 setMounted,
  // 随后首次可以正常走 portal。
  const [mounted, setMounted] = React.useState(false);
  React.useEffect(() => {
    setMounted(true);
  }, []);

  // ESC 关闭: 仅在 open=true 且 closeOnEscape 开启时挂监听, 避免
  // 关闭态仍占着全局事件。
  React.useEffect(() => {
    if (!open || !closeOnEscape || disableClose) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, closeOnEscape, disableClose, onClose]);

  // body 滚动锁: 打开期间禁用页面滚动, 关闭时恢复。记录原 overflow
  // 值再恢复, 兼容外部已经手动设过 overflow 的场景。
  React.useEffect(() => {
    if (!open) return;
    const original = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = original;
    };
  }, [open]);

  if (!mounted || !open) return null;

  const handleBackdropClick = () => {
    if (!closeOnBackdrop || disableClose) return;
    onClose();
  };

  return createPortal(
    <div
      className="plugin-editor-modal-backdrop"
      role="presentation"
      onClick={handleBackdropClick}
    >
      <div
        className={cn("panel plugin-editor-modal", className)}
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel ?? title}
        onClick={(event) => event.stopPropagation()}
      >
        <div className="plugin-editor-modal-head">
          <div className="plugin-editor-modal-title-group">
            {badge ? (
              <div className="plugin-editor-modal-badge">
                {badge.icon}
                <span>{badge.label}</span>
              </div>
            ) : null}
            <div className="plugin-editor-modal-title-copy">
              <h3>{title}</h3>
              {subtitle ? <span>{subtitle}</span> : null}
            </div>
          </div>
          {disableClose ? null : (
            <button
              className="btn plugin-editor-modal-close"
              type="button"
              aria-label={closeAriaLabel}
              title={closeAriaLabel}
              onClick={onClose}
            >
              <X size={16} />
            </button>
          )}
        </div>
        <div className="plugin-editor-modal-body">{children}</div>
        {actions ? (
          <div className="plugin-editor-modal-actions">{actions}</div>
        ) : null}
      </div>
    </div>,
    document.body,
  );
}
