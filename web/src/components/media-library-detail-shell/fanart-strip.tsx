"use client";

import type { MediaLibraryImagePreview } from "@/components/media-library-detail-shell/image-preview-overlay";
import type { LibraryFileItem } from "@/lib/api";

export interface FanartStripProps {
  // files: 已经过滤好的 extrafanart 文件列表. 父层负责过滤 (file.rel_path
  // 中包含 "/extrafanart/"), 组件本身不做二次过滤以保持职责单一.
  files: LibraryFileItem[];
  resolveImageSrc: (path: string) => string;
  onPreview: (preview: MediaLibraryImagePreview) => void;
  extraClassName?: string;
}

// FanartStrip: 详情页下方 (或 hero 右列末尾) 的 extrafanart 横向缩略图条.
// 垂直滚动被 onWheel handler 转为水平滚动, 这样使用鼠标滚轮或触控板竖向
// 滑动都能翻页; 如果用户本来就是横向滚动 (deltaX >= deltaY), 则保留原生
// 行为不干预. 点击缩略图调用 onPreview 把大图交给父层的预览 overlay.
export function FanartStrip({ files, resolveImageSrc, onPreview, extraClassName = "" }: FanartStripProps) {
  if (files.length === 0) {
    return null;
  }
  return (
    <div
      className={`panel review-fanart-panel library-fanart-panel media-library-fanart-section media-library-fanart-compact ${extraClassName}`.trim()}
    >
      <div
        className="review-fanart-strip library-fanart-strip"
        onWheel={(e) => {
          if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) {
            return;
          }
          const target = e.currentTarget;
          target.scrollLeft += e.deltaY;
          e.preventDefault();
        }}
      >
        {files.map((file) => (
          <div key={file.rel_path} className="review-fanart-item library-fanart-item">
            <button
              type="button"
              className="review-image-hit"
              onClick={() => onPreview({ title: "Extrafanart", path: file.rel_path, name: file.name })}
            >
              <img src={resolveImageSrc(file.rel_path)} alt={file.name} className="library-fanart-image" />
            </button>
            <div className="sr-only">{file.name.split("/").pop()}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
