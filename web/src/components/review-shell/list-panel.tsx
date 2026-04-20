"use client";

import { Check, MoreHorizontal, RotateCcw, Trash2 } from "lucide-react";
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";

import { Button } from "@/components/ui/button";
import type { JobItem } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

export interface ReviewListPanelProps {
  items: JobItem[];
  selectedId: number | undefined;
  selectedIndex: number;
  selectedJobIds: Set<number>;
  selectedCount: number;
  allSelectableChecked: boolean;
  isPending: boolean;
  moveRunning: boolean;
  selectAllRef: RefObject<HTMLInputElement | null>;
  onToggleSelectAll: () => void;
  onToggleSelectJob: (jobID: number) => void;
  onLoadDetail: (job: JobItem) => void;
  onImportSelected: () => void;
  onDeleteSelected: () => void;
  onImport: () => void;
  onDelete: () => void;
  onReject: () => void;
}

export function ReviewListPanel({
  items,
  selectedId,
  selectedIndex,
  selectedJobIds,
  selectedCount,
  allSelectableChecked,
  isPending,
  moveRunning,
  selectAllRef,
  onToggleSelectAll,
  onToggleSelectJob,
  onLoadDetail,
  onImportSelected,
  onDeleteSelected,
  onImport,
  onDelete,
  onReject,
}: ReviewListPanelProps) {
  return (
    <aside className="panel review-list-panel">
      <div className="review-list-head">
        <div>
          <div className="review-list-kicker">Review Queue</div>
          <h2 className="review-list-title">Review 列表</h2>
          <p className="review-list-subtitle">
            当前 {items.length} 条待复核任务
            {selectedIndex >= 0 ? `，正在查看第 ${selectedIndex + 1} 条` : ""}
          </p>
          {moveRunning ? <p className="review-list-subtitle">媒体库正在同步迁移，审批按钮已临时锁定。</p> : null}
        </div>
      </div>
      <div className="review-bulk-toolbar">
        <label className="review-bulk-select-all">
          <input
            ref={selectAllRef}
            type="checkbox"
            checked={allSelectableChecked}
            disabled={items.length === 0 || isPending || moveRunning}
            title="选择当前列表中的全部 review 任务"
            onChange={onToggleSelectAll}
          />
          <span>全选</span>
        </label>
        <div className="review-bulk-toolbar-actions">
          {selectedCount > 0 ? <span className="review-bulk-count">已选 {selectedCount} 项</span> : null}
          <Button
            className="review-inline-icon-btn review-bulk-approve-btn"
            onClick={onImportSelected}
            disabled={selectedCount === 0 || isPending || moveRunning}
            aria-label="批量审批"
            title={selectedCount > 0 ? `批量审批已选 ${selectedCount} 项` : "批量审批"}
          >
            <Check size={16} />
          </Button>
          <Button
            className="review-inline-icon-btn review-bulk-delete-btn"
            onClick={onDeleteSelected}
            // moveRunning 一并 disable: 批量删除最终走 os.Remove 源文件,
            // 媒体库迁移中如果撞到正在搬的路径会产生文件系统竞态, 与入库按钮
            // 保持同一套锁定策略。
            disabled={selectedCount === 0 || isPending || moveRunning}
            aria-label="批量删除"
            title={
              moveRunning
                ? "媒体库移动进行中，暂不可删除"
                : selectedCount > 0
                  ? `删除已选 ${selectedCount} 项`
                  : "批量删除"
            }
          >
            <Trash2 size={14} />
          </Button>
        </div>
      </div>
      <div className="review-job-list">
        {items.length === 0 ? <div className="review-empty-state">当前没有待 review 的任务</div> : null}
        {items.map((job, index) => (
          <div
            key={job.id}
            className="panel review-job-card"
            data-active={selectedId === job.id}
            data-selected={selectedJobIds.has(job.id)}
          >
            <div className="review-job-card-select">
              <input
                type="checkbox"
                checked={selectedJobIds.has(job.id)}
                disabled={isPending || moveRunning}
                title={moveRunning ? "媒体库移动进行中，暂不可选择" : "选择任务"}
                onChange={() => onToggleSelectJob(job.id)}
              />
            </div>
            <button className="review-job-card-main" onClick={() => onLoadDetail(job)} disabled={isPending}>
              <div className="review-job-card-topline">
                <span className="review-job-card-index">#{index + 1}</span>
                <span className="review-job-card-time">更新于 {formatUnixMillis(job.updated_at)}</span>
              </div>
              <div className="review-job-card-path">{job.rel_path}</div>
              <div className="review-job-card-number">{job.number}</div>
            </button>
            <div className="review-job-card-actions">
              <Button
                className="review-inline-icon-btn review-action-approve"
                onClick={onImport}
                disabled={isPending || selectedId !== job.id || moveRunning}
                aria-label="入库"
                title={moveRunning ? "媒体库移动进行中，暂不可审批" : "入库"}
              >
                <Check size={16} />
              </Button>
              <ReviewJobOverflowMenu
                // moveRunning 也列入 disabled: 菜单里的"删除"会 os.Remove 源文件,
                // 媒体库迁移进行中如果恰好碰上同一条路径会产生文件系统竞态;
                // "打回"本身只改 DB 无风险, 但为了 UI 一致性一起锁上, 避免用户
                // 看到 "入库按钮禁用、旁边的菜单却可用" 的割裂感。
                disabled={isPending || selectedId !== job.id || moveRunning}
                triggerTitle={moveRunning ? "媒体库移动进行中，暂不可操作" : "更多操作"}
                onDelete={onDelete}
                onReject={onReject}
              />
            </div>
          </div>
        ))}
      </div>
    </aside>
  );
}

interface ReviewJobOverflowMenuProps {
  disabled: boolean;
  triggerTitle?: string;
  onDelete: () => void;
  onReject: () => void;
}

// MENU_WIDTH / MENU_OFFSET: 菜单尺寸的近似值, 仅用于 flip/clamp 计算,
// 并不锁死实际渲染尺寸 (渲染后我们会用真实 rect 再修一次位置)。
const MENU_WIDTH = 130;
const MENU_MIN_HEIGHT = 88;
const MENU_OFFSET = 4;
const VIEWPORT_PAD = 8;

interface MenuPosition {
  top: number;
  left: number;
}

// computeMenuPosition: 根据 trigger 的 viewport rect + 菜单真实尺寸 (若已挂载)
// 计算菜单相对 viewport 的 top/left。提到 hook 之外是为了:
//   1. 保证在 useLayoutEffect 里能把 `triggerRef.current` 的读取和 setPosition
//      放在同一个函数体里, 让 react-hooks/set-state-in-effect 能识别到新 state
//      的源头来自 ref (否则会误报 "setState synchronously within an effect");
//   2. 顺便避免每次渲染重新构造 useCallback。
function computeMenuPosition(trigger: HTMLElement, menuEl: HTMLElement | null): MenuPosition {
  const rect = trigger.getBoundingClientRect();
  // 优先用真实渲染后的 menu 尺寸; 首帧还没挂上时 fallback 到近似值,
  // 首帧定位后 useLayoutEffect 会再触发一次 compute 收敛到精确位置。
  const menuWidth = menuEl?.offsetWidth ?? MENU_WIDTH;
  const menuHeight = menuEl?.offsetHeight ?? MENU_MIN_HEIGHT;

  // 水平: 右对齐 trigger, 然后向左 clamp 保证不出左侧 viewport。
  let left = rect.right - menuWidth;
  const maxLeft = window.innerWidth - menuWidth - VIEWPORT_PAD;
  if (left > maxLeft) left = maxLeft;
  if (left < VIEWPORT_PAD) left = VIEWPORT_PAD;

  // 垂直: 默认往下; trigger 下方空间不足时翻到上方; 两侧都不够就贴
  // 底部并允许原本 overflow: auto 的菜单自身滚动 (目前只有两项, 实际
  // 不会出现)。
  const spaceBelow = window.innerHeight - rect.bottom - VIEWPORT_PAD;
  const spaceAbove = rect.top - VIEWPORT_PAD;
  let top: number;
  if (spaceBelow >= menuHeight + MENU_OFFSET) {
    top = rect.bottom + MENU_OFFSET;
  } else if (spaceAbove >= menuHeight + MENU_OFFSET) {
    top = rect.top - menuHeight - MENU_OFFSET;
  } else {
    top = Math.max(VIEWPORT_PAD, window.innerHeight - menuHeight - VIEWPORT_PAD);
  }
  return { top, left };
}

// ReviewJobOverflowMenu 把 "删除" / "打回" 两个相对低频的破坏性操作
// 折叠到 `...` 菜单里, 避免每张 review 卡片上出现 3 个按钮过于拥挤。
//
// 定位方式说明:
//   早期版本把菜单渲染成 trigger 的绝对定位子元素, 但 .review-job-list
//   是 overflow: auto 的滚动容器, 菜单一旦超出列表 rect (例如落在最后一
//   张卡下方, 或卡片右边缘) 就会被列表裁掉。
//   现在改成通过 React portal 把菜单挂到 document.body, 并用 position:
//   fixed + 手动计算的 top/left 跟 trigger 的 getBoundingClientRect()
//   对齐。这样菜单彻底脱离列表的 overflow 裁剪上下文, 只受 viewport
//   约束, 不会被任何祖先 clip。滚动 / 窗口 resize 时重算位置, 超窗时
//   向上翻或往左 clamp。
function ReviewJobOverflowMenu({ disabled, triggerTitle, onDelete, onReject }: ReviewJobOverflowMenuProps) {
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<MenuPosition | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);

  // SSR 安全说明: createPortal 需要 document。组件不再单独维护 mounted 门闩,
  // 因为 `open` 初始为 false, 只可能在 client 的 onClick 里翻成 true ——
  // 即使 Next.js app router 把这份组件放到 server render, 也不会进到下面
  // `createPortal(..., document.body)` 分支, 所以不会 touch `document`。

  // 打开瞬间先用近似尺寸算一次, 避免菜单在 (0,0) 闪一下; 菜单挂上之后
  // useLayoutEffect 再用真实尺寸修一次位置。
  // 这里把 trigger 的 ref 读取和 setPosition 放在同一个 effect 体里, 是为了
  // 符合 react-hooks/set-state-in-effect: 规则允许 "setState 的值来源于 ref"
  // 这种写法 (典型场景就是测量 DOM 尺寸), 但不允许同步 setState 一个常量 /
  // 从 props 派生的值。如果再经过 useCallback 包一层, 规则就看不出 ref 的
  // 源头, 会误报, 所以保持内联。
  useLayoutEffect(() => {
    if (!open) return;
    const trigger = triggerRef.current;
    if (!trigger) return;
    setPosition(computeMenuPosition(trigger, menuRef.current));
  }, [open]);

  // 打开时监听 scroll/resize 重算位置, 覆盖"菜单打开 -> 用户滚列表"
  // 场景。用 capture: true 保证能接到 .review-job-list 这种内部滚动
  // 容器的 scroll 事件。setPosition 在事件回调里触发 (而不是 effect 主体
  // 同步调用), 因此不会被 set-state-in-effect 规则判定为级联渲染。
  useEffect(() => {
    if (!open) return;
    const handler = () => {
      const trigger = triggerRef.current;
      if (!trigger) return;
      setPosition(computeMenuPosition(trigger, menuRef.current));
    };
    window.addEventListener("scroll", handler, true);
    window.addEventListener("resize", handler);
    return () => {
      window.removeEventListener("scroll", handler, true);
      window.removeEventListener("resize", handler);
    };
  }, [open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handlePointer = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (!target) return;
      // 菜单已经 portal 到 body, 不再是 containerRef 的后代, 要同时
      // 检查 trigger 和 menu 两个元素, 避免点击菜单项自己把菜单关掉。
      if (triggerRef.current?.contains(target)) return;
      if (menuRef.current?.contains(target)) return;
      setOpen(false);
    };
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    window.addEventListener("mousedown", handlePointer);
    window.addEventListener("keydown", handleKey);
    return () => {
      window.removeEventListener("mousedown", handlePointer);
      window.removeEventListener("keydown", handleKey);
    };
  }, [open]);

  // 注: 不要用 useEffect 同步 "disabled -> setOpen(false)", 那会触发
  // cascading render。改成 "disabled 生效时把 open 视为 false" 的 derived
  // 展示模型, 同时禁用触发器 onClick, 保证切 disabled 那一刻菜单立即收起。
  const effectiveOpen = open && !disabled;

  return (
    <div className="review-job-overflow">
      <Button
        ref={triggerRef}
        className="review-inline-icon-btn review-job-overflow-trigger"
        onClick={() => {
          if (disabled) return;
          setOpen((prev) => !prev);
        }}
        disabled={disabled}
        aria-label="更多操作"
        aria-haspopup="menu"
        aria-expanded={effectiveOpen}
        title={triggerTitle ?? "更多操作"}
      >
        <MoreHorizontal size={16} />
      </Button>
      {effectiveOpen
        ? createPortal(
            <div
              ref={menuRef}
              className="review-job-overflow-menu"
              role="menu"
              // 首帧 position === null 时先渲到 viewport 外避免闪烁,
              // useLayoutEffect 马上会把真实位置算好。
              style={{
                position: "fixed",
                top: position?.top ?? -9999,
                left: position?.left ?? -9999,
                visibility: position ? "visible" : "hidden",
              }}
            >
              <button
                type="button"
                role="menuitem"
                className="review-job-overflow-item"
                onClick={() => {
                  setOpen(false);
                  onReject();
                }}
              >
                <RotateCcw size={14} aria-hidden />
                <span>打回</span>
              </button>
              <button
                type="button"
                role="menuitem"
                className="review-job-overflow-item review-job-overflow-item-danger"
                onClick={() => {
                  setOpen(false);
                  onDelete();
                }}
              >
                <Trash2 size={14} aria-hidden />
                <span>删除</span>
              </button>
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}
