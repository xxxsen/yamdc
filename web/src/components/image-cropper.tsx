"use client";

import {
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type SyntheticEvent,
} from "react";

import { Modal } from "@/components/ui";

// ImageCropper: 从一张封面里截海报的固定宽高比拖拽框选器.
//
// 抽离自 review-shell / library-shell 里"逐字符相同"的两份 CropOverlay,
// 状态全部内聚: imageSrc + onConfirm 之外, 调用方不再需要持有 cropRect /
// cropImageSize / cropDragRef 这套手势 state. 这也是 td/022 §2.2 第一步
// "Crop 编辑器独立成 library/crop-dialog.tsx" 的实际落地 (放在顶层而不是
// library/ 下, 因为 review 也用; 等 §2.2 整体推进时再决定归属).
//
// 视觉行为完全复刻原 .review-crop-* 一套样式; 走 <Modal bare> 复用 portal
// + ESC + body 滚动锁, backdrop / frame 用 Tailwind utility 描述.

export type CropRect = { x: number; y: number; width: number; height: number };

// 海报标准长宽比. review/library 两边历史上都硬编码 2/3, 通过 prop 暴露
// 留个口子 (例如未来截 fanart 是 16/9), 默认值保持原行为不变.
const DEFAULT_POSTER_ASPECT = 2 / 3;

export interface ImageCropperProps {
  open: boolean;
  imageSrc: string;
  onClose: () => void;
  onConfirm: (rect: CropRect) => void;
  isPending?: boolean;
  title?: string;
  posterAspect?: number;
}

export function ImageCropper({
  open,
  imageSrc,
  onClose,
  onConfirm,
  isPending = false,
  title = "从封面截取海报",
  posterAspect = DEFAULT_POSTER_ASPECT,
}: ImageCropperProps) {
  const [rect, setRect] = useState({ x: 0, y: 0, width: 0, height: 0 });
  const [imgSize, setImgSize] = useState({
    displayWidth: 0,
    displayHeight: 0,
    naturalWidth: 0,
    naturalHeight: 0,
  });
  const dragRef = useRef<{
    startX: number;
    startY: number;
    originX: number;
    originY: number;
  } | null>(null);

  // 横图: 高度顶满, 宽度按 aspect 算, 水平居中.
  // 竖图: 宽度顶满, 高度按 aspect 算, 垂直居中.
  // 之所以横竖分支不同, 是因为后面拖拽也只允许沿主导方向 (横图横向 /
  // 竖图纵向) 调位, 初始位置就给到默认居中, 用户从中心出发往两侧调.
  const handleImageLoad = (event: SyntheticEvent<HTMLImageElement>) => {
    const img = event.currentTarget;
    const naturalWidth = img.naturalWidth;
    const naturalHeight = img.naturalHeight;
    const displayWidth = img.clientWidth;
    const displayHeight = img.clientHeight;
    let width = 0;
    let height = 0;
    let x = 0;
    let y = 0;
    if (naturalWidth >= naturalHeight) {
      height = displayHeight;
      width = height * posterAspect;
      x = Math.max(0, (displayWidth - width) / 2);
    } else {
      width = displayWidth;
      height = width / posterAspect;
      y = Math.max(0, (displayHeight - height) / 2);
    }
    setImgSize({ displayWidth, displayHeight, naturalWidth, naturalHeight });
    setRect({ x, y, width, height });
  };

  const beginDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    dragRef.current = {
      startX: event.clientX,
      startY: event.clientY,
      originX: rect.x,
      originY: rect.y,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const moveDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    const ds = dragRef.current;
    if (!ds) return;
    const dx = event.clientX - ds.startX;
    const dy = event.clientY - ds.startY;
    setRect((prev) => {
      const next = { ...prev };
      if (imgSize.naturalWidth >= imgSize.naturalHeight) {
        next.x = Math.min(
          Math.max(0, ds.originX + dx),
          imgSize.displayWidth - prev.width,
        );
      } else {
        next.y = Math.min(
          Math.max(0, ds.originY + dy),
          imgSize.displayHeight - prev.height,
        );
      }
      return next;
    });
  };

  const endDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!dragRef.current) return;
    dragRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  const handleConfirm = () => {
    if (imgSize.displayWidth === 0 || imgSize.displayHeight === 0) return;
    const sx = imgSize.naturalWidth / imgSize.displayWidth;
    const sy = imgSize.naturalHeight / imgSize.displayHeight;
    onConfirm({
      x: Math.round(rect.x * sx),
      y: Math.round(rect.y * sy),
      width: Math.round(rect.width * sx),
      height: Math.round(rect.height * sy),
    });
  };

  const showSelection = rect.width > 0 && rect.height > 0;

  return (
    <Modal
      open={open}
      onClose={onClose}
      bare
      ariaLabel={title}
      // 复刻旧 .review-preview-overlay: 全屏覆盖 + 暗色半透明 + 居中.
      backdropClassName="fixed inset-0 z-50 flex items-center justify-center p-8 bg-[rgba(26,18,14,0.6)]"
      // 复刻旧 .review-preview-dialog + .panel + .review-crop-dialog 叠加效果:
      // panel 卡片底 + crop 专用宽度 (min(86vw, 1200px) 比通用 dialog 宽 6vw).
      frameClassName={
        "relative w-[min(86vw,1200px)] p-4 rounded-[20px] " +
        "bg-panel backdrop-blur-md border border-line " +
        "shadow-[0_12px_40px_rgba(58,35,20,0.08)]"
      }
    >
      <div className="flex items-center justify-start mb-3">
        <div className="text-[14px] text-muted">{title}</div>
      </div>
      <div className="relative w-full h-[min(78vh,760px)] flex items-center justify-center overflow-hidden">
        <div className="relative inline-block leading-[0]">
          <img
            src={imageSrc}
            alt="cover crop preview"
            className="max-w-full max-h-full block select-none"
            onLoad={handleImageLoad}
            draggable={false}
          />
          {showSelection ? (
            <div
              // 选区: 白色描边 + 14px 圆角 + 9999px 外阴影模拟"挖洞"暗化框外区域.
              className="absolute border-2 border-[rgba(255,248,244,0.98)] rounded-[14px] cursor-grab active:cursor-grabbing touch-none shadow-[0_0_0_9999px_rgba(26,18,14,0.36)]"
              style={{
                left: rect.x,
                top: rect.y,
                width: rect.width,
                height: rect.height,
              }}
              onPointerDown={beginDrag}
              onPointerMove={moveDrag}
              onPointerUp={endDrag}
              onPointerCancel={endDrag}
            />
          ) : null}
          {showSelection ? (
            // 截取按钮浮在选区右上角, 暗底白字 + backdrop blur. 这里刻意
            // 用裸 <button> 而非 ui/Button — Button 套着的 .btn 全局规则
            // (padding: 10px 16px) 会覆盖我们的 padding utility, 把按钮
            // 撑成 50px 高溢出选区. 旧 .review-crop-confirm 用 padding: 0
            // 10px 显式压制, 这里直接绕过 .btn 体系干净一些.
            <button
              type="button"
              className="absolute z-[3] inline-flex items-center justify-center min-w-[42px] h-[26px] px-2.5 py-0 rounded-full border border-[rgba(255,248,244,0.28)] bg-[rgba(31,26,20,0.7)] text-[#fff8f4] backdrop-blur-md text-[12px] leading-none shadow-[0_8px_18px_rgba(26,18,14,0.18)] cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              style={{ left: rect.x + rect.width - 54, top: rect.y + 8 }}
              onClick={handleConfirm}
              disabled={isPending}
            >
              截取
            </button>
          ) : null}
        </div>
      </div>
    </Modal>
  );
}
