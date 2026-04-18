"use client";

import { Crop, Plus, X } from "lucide-react";
import Image from "next/image";
import type { RefObject } from "react";

import type { ReviewPreviewState } from "@/components/review-shell/preview-overlay";
import { imageTitle } from "@/components/review-shell/utils";
import { Button } from "@/components/ui/button";
import type { MediaFileRef } from "@/lib/api";
import { getAssetURL } from "@/lib/api";

const THUMB_IMAGE_STYLE = { objectFit: "cover", objectPosition: "center" } as const;

export interface ReviewPosterCardProps {
  poster: MediaFileRef | null | undefined;
  uploadActiveRef: RefObject<boolean>;
  onOpenCropper: () => void;
  onOpenUploadPicker: () => void;
  onOpenPreview: (next: ReviewPreviewState) => void;
}

export function ReviewPosterCard({
  poster,
  uploadActiveRef,
  onOpenCropper,
  onOpenUploadPicker,
  onOpenPreview,
}: ReviewPosterCardProps) {
  if (poster) {
    return (
      <div className="panel review-image-card review-image-card-poster review-top-poster review-main-poster">
        <span className="review-image-title">海报</span>
        <Button className="review-inline-icon-btn review-image-crop-btn" onClick={onOpenCropper} aria-label="从封面截取海报" title="从封面截取海报">
          <Crop size={14} />
        </Button>
        <div className="review-image-box review-image-box-poster">
          <button
            type="button"
            className="review-image-hit"
            onClick={() => {
              if (!uploadActiveRef.current) {
                onOpenPreview({ title: imageTitle("poster"), item: poster });
              }
            }}
          >
            <Image src={getAssetURL(poster.key)} alt="poster" fill style={THUMB_IMAGE_STYLE} unoptimized />
          </button>
          <button
            type="button"
            className="review-upload-overlay"
            onClick={onOpenUploadPicker}
            aria-label="上传海报"
            title="上传海报"
          >
            <Plus size={18} />
          </button>
        </div>
      </div>
    );
  }
  return (
    <div className="panel review-image-card review-image-card-poster review-top-poster review-main-poster review-image-empty">
      <span className="review-image-title">海报</span>
      <Button className="review-inline-icon-btn review-image-crop-btn" onClick={onOpenCropper} aria-label="从封面截取海报" title="从封面截取海报">
        <Crop size={14} />
      </Button>
      <div className="review-image-box review-image-box-poster review-upload-empty">
        <button
          type="button"
          className="review-upload-overlay"
          onClick={onOpenUploadPicker}
          aria-label="上传海报"
          title="上传海报"
        >
          <Plus size={18} />
        </button>
      </div>
    </div>
  );
}

export interface ReviewCoverCardProps {
  cover: MediaFileRef | null | undefined;
  uploadActiveRef: RefObject<boolean>;
  onOpenUploadPicker: () => void;
  onOpenPreview: (next: ReviewPreviewState) => void;
}

export function ReviewCoverCard({
  cover,
  uploadActiveRef,
  onOpenUploadPicker,
  onOpenPreview,
}: ReviewCoverCardProps) {
  if (cover) {
    return (
      <div className="review-media-offset review-cover-slot">
        <div className="panel review-image-card review-image-card-cover">
          <span className="review-image-title">封面</span>
          <div className="review-image-box review-image-box-cover">
            <button
              type="button"
              className="review-image-hit"
              onClick={() => {
                if (!uploadActiveRef.current) {
                  onOpenPreview({ title: imageTitle("cover"), item: cover });
                }
              }}
            >
              <Image src={getAssetURL(cover.key)} alt="cover" fill style={THUMB_IMAGE_STYLE} unoptimized />
            </button>
            <button
              type="button"
              className="review-upload-overlay"
              onClick={(e) => {
                e.stopPropagation();
                onOpenUploadPicker();
              }}
              aria-label="上传封面"
              title="上传封面"
            >
              <Plus size={18} />
            </button>
          </div>
        </div>
      </div>
    );
  }
  return (
    <div className="review-media-offset review-cover-slot">
      <div className="panel review-image-card review-image-card-cover review-image-empty">
        <div className="review-image-box review-image-box-cover review-upload-empty">
          <button
            type="button"
            className="review-upload-overlay"
            onClick={onOpenUploadPicker}
            aria-label="上传封面"
            title="上传封面"
          >
            <Plus size={18} />
          </button>
        </div>
      </div>
    </div>
  );
}

export interface ReviewFanartStripProps {
  sampleImages: MediaFileRef[] | undefined;
  uploadActiveRef: RefObject<boolean>;
  onOpenUploadPicker: () => void;
  onRemoveFanart: (key: string) => void;
  onOpenPreview: (next: ReviewPreviewState) => void;
}

export function ReviewFanartStrip({
  sampleImages,
  uploadActiveRef,
  onOpenUploadPicker,
  onRemoveFanart,
  onOpenPreview,
}: ReviewFanartStripProps) {
  return (
    <div className="review-media-offset review-fanart-slot">
      <div className="panel review-fanart-panel">
        <div
          className="review-fanart-strip"
          onWheel={(e) => {
            if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) {
              return;
            }
            e.currentTarget.scrollLeft += e.deltaY;
            e.preventDefault();
          }}
        >
          {(sampleImages ?? []).map((item) => (
            <div key={item.key} className="review-fanart-item">
              <button
                type="button"
                className="review-image-hit"
                onClick={() => {
                  if (!uploadActiveRef.current) {
                    onOpenPreview({ title: imageTitle("fanart"), item });
                  }
                }}
              >
                <Image src={getAssetURL(item.key)} alt={item.name} fill style={THUMB_IMAGE_STYLE} unoptimized />
              </button>
              <Button
                className="review-inline-icon-btn review-fanart-delete"
                onClick={() => onRemoveFanart(item.key)}
                aria-label="删除 fanart"
                title="删除 fanart"
              >
                <X size={12} />
              </Button>
            </div>
          ))}
          <button type="button" className="review-fanart-item review-upload-empty" onClick={onOpenUploadPicker}>
            <span className="review-upload-overlay review-upload-overlay-static" aria-hidden="true">
              <Plus size={18} />
            </span>
          </button>
        </div>
      </div>
    </div>
  );
}
