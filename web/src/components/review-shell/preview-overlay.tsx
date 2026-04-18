"use client";

import { X } from "lucide-react";
import Image from "next/image";

import type { MediaFileRef } from "@/lib/api";
import { getAssetURL } from "@/lib/api";

const PREVIEW_IMAGE_STYLE = { objectFit: "contain", objectPosition: "center" } as const;

export type ReviewPreviewState = { title: string; item: MediaFileRef } | null;

export interface ReviewPreviewOverlayProps {
  preview: ReviewPreviewState;
  onClose: () => void;
}

export function ReviewPreviewOverlay({ preview, onClose }: ReviewPreviewOverlayProps) {
  if (!preview) return null;
  return (
    <div className="review-preview-overlay" onClick={onClose}>
      <button type="button" className="review-preview-close" aria-label="关闭预览" onClick={onClose}>
        <X size={18} />
      </button>
      <div className="review-preview-dialog panel" onClick={(e) => e.stopPropagation()}>
        <div className="review-preview-title">{preview.title}</div>
        <div className="review-preview-frame">
          <Image src={getAssetURL(preview.item.key)} alt={preview.item.name} fill style={PREVIEW_IMAGE_STYLE} unoptimized />
        </div>
      </div>
    </div>
  );
}
