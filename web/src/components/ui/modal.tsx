"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

// Modal: 项目公共弹层原子。不引入动画/focus-trap, 聚焦于把散落在各
// shell 里的 Modal 行为 (portal / ESC / backdrop 点击 / body 滚动锁)
// 统一化, 把"每个 shell 自己 new 一份"的重复代码收敛。
//
// 两种变体:
//   1. shell (默认, bare=false): 带 plugin-editor-modal-* 一套 head/body
//      包装, 用 title/subtitle/badge/actions 做声明式 slots。
//      覆盖 ImportModal / ExampleModal 这类"标准对话框"。
//   2. bare (bare=true): 不渲染任何 head/body 骨架, 调用方在 children
//      里自己摆布局, Modal 只负责 portal + 三件行为 + 把
//      frameClassName 套在内框上。覆盖 media-library-shell 里那些
//      有独特 backdrop 样式和不规则 header 的对话框。
//
// 详见 td/022-frontend-optimization-roadmap.md §3.1。

export interface ModalBadge {
  icon: React.ReactNode;
  label: string;
}

// 通用 props: 两种变体都用得到。title 在 shell 模式下必填, bare
// 模式下无意义; 由 consumer 自觉遵守 (我们在运行期放弃校验, 保持
// 类型声明简洁)。
export interface ModalProps {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;

  // shell 模式独有 (bare=true 时被忽略)
  title?: string;
  subtitle?: string;
  badge?: ModalBadge;
  actions?: React.ReactNode;
  closeAriaLabel?: string;

  // 外观定制
  bare?: boolean;
  backdropClassName?: string;
  frameClassName?: string;
  className?: string;

  // 行为 / 可访问性
  closeOnBackdrop?: boolean;
  closeOnEscape?: boolean;
  ariaLabel?: string;
  disableClose?: boolean;
}

const DEFAULT_BACKDROP_CLASS = "plugin-editor-modal-backdrop";
const DEFAULT_FRAME_CLASS = "panel plugin-editor-modal";

interface ModalChromeProps {
  title?: string;
  subtitle?: string;
  badge?: ModalBadge;
  actions?: React.ReactNode;
  children: React.ReactNode;
  disableClose: boolean;
  closeAriaLabel: string;
  onClose: () => void;
}

function ModalChrome({
  title,
  subtitle,
  badge,
  actions,
  children,
  disableClose,
  closeAriaLabel,
  onClose,
}: ModalChromeProps) {
  return (
    <>
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
      {actions ? <div className="plugin-editor-modal-actions">{actions}</div> : null}
    </>
  );
}

export function Modal({
  open,
  onClose,
  title,
  subtitle,
  badge,
  actions,
  children,
  bare = false,
  backdropClassName,
  frameClassName,
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

  // bare 模式: 调用方自己给 backdrop/frame 上类; 我们不追加任何默认,
  // 避免和调用方的自定义样式相互打架 (例如 .media-library-detail-modal
  // 的 flex 布局不想叠加 .plugin-editor-modal-backdrop 的 grid 居中)。
  const resolvedBackdropClass = bare
    ? backdropClassName
    : cn(DEFAULT_BACKDROP_CLASS, backdropClassName);
  const resolvedFrameClass = bare
    ? cn(frameClassName, className)
    : cn(DEFAULT_FRAME_CLASS, frameClassName, className);

  return createPortal(
    <div
      className={resolvedBackdropClass}
      role="presentation"
      onClick={handleBackdropClick}
    >
      <div
        className={resolvedFrameClass}
        role="dialog"
        aria-modal="true"
        aria-label={ariaLabel ?? title}
        onClick={(event) => event.stopPropagation()}
      >
        {bare ? (
          children
        ) : (
          <ModalChrome
            title={title}
            subtitle={subtitle}
            badge={badge}
            actions={actions}
            disableClose={disableClose}
            closeAriaLabel={closeAriaLabel}
            onClose={onClose}
          >
            {children}
          </ModalChrome>
        )}
      </div>
    </div>,
    document.body,
  );
}
