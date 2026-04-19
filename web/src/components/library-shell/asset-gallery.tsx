"use client";

import { Crop, Plus, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { LibraryFileItem } from "@/lib/api";

// asset-gallery.tsx: library-shell detail 区的 "资源展示" 子组件群.
// 四个 named export 映射到原 JSX 里不连续的四处:
//   - LibraryPosterCard    → 在 review-main-layout 内, 与 actors 并排
//   - LibraryCoverCard     → review-main-layout 之外, genres 下方
//   - LibraryFanartStrip   → cover 下方的 extrafanart 横向滚动条
//   - LibraryPreviewOverlay→ 根层的图片放大预览 (独立 modal)
// 特意拆成 4 个小 export 而不是 Fragment 合一, 因为原 DOM 里中间夹着
// genres TokenEditor, 合一会破坏结构.
//
// preview state 继续留在父层 (LibraryShell), 只是把 overlay JSX 独立出
// 来 — 下沉收益不足以抵消它与 3 张卡的跨组件 state 耦合.
//
// uploadActiveRef 由父层持有, 各 card 读 .current 判断 "上传弹窗活跃期
// 禁止打开 preview". 只读不写.
//
// JSX / class 名完全照搬 library-shell.tsx 旧版, 行为零改动.
// 详见 td/022-frontend-optimization-roadmap.md §2.2 B-3b.

export type LibraryPreviewState = { title: string; path: string; name: string } | null;

export interface LibraryPosterCardProps {
  selectedPoster: string;
  selectedCover: string;
  isPending: boolean;
  uploadActiveRef: { current: boolean };
  resolveImage: (path: string) => string;
  onOpenCropper: () => void;
  onOpenUploadPicker: () => void;
  onOpenPreview: (next: NonNullable<LibraryPreviewState>) => void;
}

export function LibraryPosterCard({
  selectedPoster,
  selectedCover,
  isPending,
  uploadActiveRef,
  resolveImage,
  onOpenCropper,
  onOpenUploadPicker,
  onOpenPreview,
}: LibraryPosterCardProps) {
  return (
    <div className="panel review-image-card review-image-card-poster review-top-poster review-main-poster">
      <div className="review-image-card-head">
        <span className="review-image-title">海报</span>
        <Button
          className="review-inline-icon-btn review-image-crop-btn"
          onClick={onOpenCropper}
          aria-label="从封面截取海报"
          title="从封面截取海报"
          disabled={!selectedCover || isPending}
        >
          <Crop size={14} />
        </Button>
      </div>
      <div className={`review-image-box review-image-box-poster${selectedPoster ? "" : " review-upload-empty"}`}>
        {selectedPoster ? (
          <button
            type="button"
            className="review-image-hit"
            onClick={() => {
              if (!uploadActiveRef.current) onOpenPreview({ title: "海报", path: selectedPoster, name: "海报" });
            }}
          >
            <img src={resolveImage(selectedPoster)} alt="海报" className="library-poster-image" />
          </button>
        ) : (
          <div className="library-preview-empty">暂无海报</div>
        )}
        <button
          type="button"
          className="review-upload-overlay"
          onClick={onOpenUploadPicker}
          aria-label="上传海报"
          title="上传海报"
          disabled={isPending}
        >
          <Plus size={18} />
        </button>
      </div>
    </div>
  );
}

export interface LibraryCoverCardProps {
  selectedCover: string;
  isPending: boolean;
  uploadActiveRef: { current: boolean };
  resolveImage: (path: string) => string;
  onOpenUploadPicker: () => void;
  onOpenPreview: (next: NonNullable<LibraryPreviewState>) => void;
}

export function LibraryCoverCard({
  selectedCover,
  isPending,
  uploadActiveRef,
  resolveImage,
  onOpenUploadPicker,
  onOpenPreview,
}: LibraryCoverCardProps) {
  return (
    <div className="review-media-offset review-cover-slot">
      <div className="panel review-image-card review-image-card-cover">
        <div className="review-image-card-head">
          <span className="review-image-title">封面</span>
        </div>
        <div className={`review-image-box review-image-box-cover${selectedCover ? "" : " review-upload-empty"}`}>
          {selectedCover ? (
            <button
              type="button"
              className="review-image-hit"
              onClick={() => {
                if (!uploadActiveRef.current) onOpenPreview({ title: "封面", path: selectedCover, name: "封面" });
              }}
            >
              <img src={resolveImage(selectedCover)} alt="封面" className="library-cover-image" />
            </button>
          ) : (
            <div className="library-preview-empty">暂无封面</div>
          )}
          <button
            type="button"
            className="review-upload-overlay"
            onClick={onOpenUploadPicker}
            aria-label="上传封面"
            title="上传封面"
            disabled={isPending}
          >
            <Plus size={18} />
          </button>
        </div>
      </div>
    </div>
  );
}

export interface LibraryFanartStripProps {
  fanartFiles: LibraryFileItem[];
  isPending: boolean;
  uploadActiveRef: { current: boolean };
  resolveImage: (path: string) => string;
  onOpenUploadPicker: () => void;
  onDeleteFanart: (path: string) => void;
  onOpenPreview: (next: NonNullable<LibraryPreviewState>) => void;
}

export function LibraryFanartStrip({
  fanartFiles,
  isPending,
  uploadActiveRef,
  resolveImage,
  onOpenUploadPicker,
  onDeleteFanart,
  onOpenPreview,
}: LibraryFanartStripProps) {
  return (
    <div className="review-media-offset library-file-offset">
      <div className="panel review-fanart-panel library-fanart-panel">
        <div className="library-file-section-head">
          <div className="library-file-section-title">Extrafanart</div>
          <div className="library-file-section-subtitle">目录里的扩展剧照资源。</div>
        </div>
        {fanartFiles.length > 0 ? (
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
            {fanartFiles.map((file) => (
              <div key={file.rel_path} className="review-fanart-item library-fanart-item">
                <button
                  type="button"
                  className="review-image-hit"
                  onClick={() => {
                    if (!uploadActiveRef.current) onOpenPreview({ title: "Extrafanart", path: file.rel_path, name: file.name });
                  }}
                >
                  <img src={resolveImage(file.rel_path)} alt={file.name} className="library-fanart-image" />
                </button>
                <Button
                  className="review-inline-icon-btn review-fanart-delete"
                  onClick={() => onDeleteFanart(file.rel_path)}
                  aria-label="删除 extrafanart"
                  title="删除 extrafanart"
                  disabled={isPending}
                >
                  <X size={12} />
                </Button>
                <div className="library-fanart-name">{file.name.split("/").pop()}</div>
              </div>
            ))}
            <button type="button" className="review-fanart-item review-upload-empty" onClick={onOpenUploadPicker} disabled={isPending}>
              <span className="review-upload-overlay review-upload-overlay-static" aria-hidden="true">
                <Plus size={18} />
              </span>
            </button>
          </div>
        ) : (
          <div className="review-fanart-strip library-fanart-strip">
            <button type="button" className="review-fanart-item review-upload-empty" onClick={onOpenUploadPicker} disabled={isPending}>
              <span className="review-upload-overlay review-upload-overlay-static" aria-hidden="true">
                <Plus size={18} />
              </span>
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

export interface LibraryPreviewOverlayProps {
  preview: LibraryPreviewState;
  resolveImage: (path: string) => string;
  onClose: () => void;
}

export function LibraryPreviewOverlay({ preview, resolveImage, onClose }: LibraryPreviewOverlayProps) {
  if (!preview) return null;
  return (
    <div className="review-preview-overlay" onClick={onClose}>
      <button type="button" className="review-preview-close" aria-label="关闭预览" onClick={onClose}>
        <X size={18} />
      </button>
      <div className="review-preview-dialog panel" onClick={(e) => e.stopPropagation()}>
        <div className="review-preview-title">{preview.title}</div>
        <div className="review-preview-frame">
          <img
            src={resolveImage(preview.path)}
            alt={preview.name}
            style={{ width: "100%", height: "100%", objectFit: "contain", objectPosition: "center", display: "block" }}
          />
        </div>
      </div>
    </div>
  );
}
