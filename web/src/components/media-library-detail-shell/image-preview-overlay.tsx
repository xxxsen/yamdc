"use client";

import { X } from "lucide-react";

export interface MediaLibraryImagePreview {
  title: string;
  path: string;
  name: string;
}

export interface ImagePreviewOverlayProps {
  preview: MediaLibraryImagePreview | null;
  resolveImageSrc: (path: string) => string;
  onClose: () => void;
}

// ImagePreviewOverlay: media-library-detail-shell 的图片放大预览层.
// 与 review-shell 的 preview-overlay 语义类似但使用独立的 preview
// 数据形状 (path 字段而不是 key/url), 故暂不合并. 如果后续几处
// overlay 再同步演化, 可以在 ui/ 下抽一个通用 ImageZoomOverlay.
export function ImagePreviewOverlay({ preview, resolveImageSrc, onClose }: ImagePreviewOverlayProps) {
  if (!preview) {
    return null;
  }
  return (
    <div className="review-preview-overlay" onClick={onClose}>
      <button type="button" className="review-preview-close" aria-label="关闭预览" onClick={onClose}>
        <X size={18} />
      </button>
      <div className="review-preview-dialog panel" onClick={(e) => e.stopPropagation()}>
        <div className="review-preview-title">{preview.title}</div>
        <div className="review-preview-frame">
          <img
            src={resolveImageSrc(preview.path)}
            alt={preview.name}
            style={{ width: "100%", height: "100%", objectFit: "contain", objectPosition: "center", display: "block" }}
          />
        </div>
      </div>
    </div>
  );
}
