"use client";

import { Check, ChevronLeft, Pencil, X } from "lucide-react";
import Link from "next/link";
import { type SetStateAction, useEffect, useEffectEvent, useRef, useState, useTransition } from "react";

import { LibraryVariantSwitcher } from "@/components/library-shell/variant-switcher";
import { MediaLibraryDisplayView } from "@/components/media-library-detail-shell/display-view";
import { FanartStrip } from "@/components/media-library-detail-shell/fanart-strip";
import { MediaLibraryFormFields } from "@/components/media-library-detail-shell/form-fields";
import {
  ImagePreviewOverlay,
  type MediaLibraryImagePreview,
} from "@/components/media-library-detail-shell/image-preview-overlay";
import {
  cloneMeta,
  getVariantCoverPath,
  normalizeMeta,
  pickVariant,
  serializeMeta,
} from "@/components/media-library-detail-shell/utils";
import type { LibraryMeta, MediaLibraryDetail } from "@/lib/api";
import { getMediaLibraryFileURL, getMediaLibraryItem, updateMediaLibraryItem } from "@/lib/api";

interface Props {
  initialDetail: MediaLibraryDetail;
  stageOnly?: boolean;
  onDetailChange?: (detail: MediaLibraryDetail) => void;
}

export function MediaLibraryDetailShell({ initialDetail, stageOnly = false, onDetailChange }: Props) {
  const initialDraftMeta = cloneMeta(initialDetail.meta);
  const [detail, setDetail] = useState<MediaLibraryDetail>(initialDetail);
  const [selectedVariantKey, setSelectedVariantKey] = useState(initialDetail.primary_variant_key || initialDetail.variants[0]?.key || "");
  const [draftMeta, setDraftMeta] = useState<LibraryMeta>(initialDraftMeta);
  const [message, setMessage] = useState("");
  const [preview, setPreview] = useState<MediaLibraryImagePreview | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [isPending, startTransition] = useTransition();
  const detailRef = useRef<MediaLibraryDetail>(initialDetail);
  const draftMetaRef = useRef<LibraryMeta>(initialDraftMeta);
  const lastSavedMetaRef = useRef(serializeMeta(initialDraftMeta));

  const currentVariant = pickVariant(detail, selectedVariantKey);
  const showVariantSwitch = detail.variants.length > 1;
  const fanartFiles = detail.files.filter((file) => file.rel_path.includes("/extrafanart/"));
  const selectedPoster = currentVariant?.poster_path || currentVariant?.meta.poster_path || draftMeta.poster_path || detail.item.poster_path || "";
  const selectedCover = getVariantCoverPath(detail, selectedVariantKey) || draftMeta.cover_path || draftMeta.fanart_path || draftMeta.thumb_path || detail.item.cover_path || "";
  const detailDisplayTitle =
    draftMeta.title.trim() ||
    detail.item.title ||
    detail.item.name ||
    draftMeta.title_translated.trim();
  const detailDisplayTitleSecondary =
    draftMeta.title_translated.trim() && draftMeta.title_translated.trim() !== detailDisplayTitle ? draftMeta.title_translated.trim() : "";
  const detailDisplayNumber = draftMeta.number || detail.item.number || "未命名影片";
  const detailDisplayPlot = draftMeta.plot.trim() || draftMeta.plot_translated.trim();
  const detailDisplayPlotSecondary =
    draftMeta.plot.trim() && draftMeta.plot_translated.trim() && draftMeta.plot_translated.trim() !== draftMeta.plot.trim()
      ? draftMeta.plot_translated.trim()
      : "";

  useEffect(() => {
    if (!message || /失败|error/i.test(message)) {
      return;
    }
    const timer = window.setTimeout(() => setMessage(""), 2400);
    return () => window.clearTimeout(timer);
  }, [message]);

  const syncDetail = (next: MediaLibraryDetail) => {
    setDetail(next);
    detailRef.current = next;
    const nextDraftMeta = cloneMeta(next.meta);
    setDraftMeta(nextDraftMeta);
    draftMetaRef.current = nextDraftMeta;
    setSelectedVariantKey((current) => {
      if (current && next.variants.some((item) => item.key === current)) {
        return current;
      }
      return next.primary_variant_key || next.variants[0]?.key || "";
    });
    lastSavedMetaRef.current = serializeMeta(nextDraftMeta);
    onDetailChange?.(next);
  };

  const refreshDetail = useEffectEvent(async () => {
    try {
      const next = await getMediaLibraryItem(detailRef.current.item.id);
      syncDetail(next);
    } catch {
      // ignore polling errors
    }
  });

  useEffect(() => {
    if (isEditing) {
      return;
    }
    const timer = window.setInterval(() => {
      void refreshDetail();
    }, 8000);
    return () => window.clearInterval(timer);
  }, [isEditing]);

  const updateDraftMeta = (updater: SetStateAction<LibraryMeta>) => {
    setDraftMeta((prev) => {
      const next = typeof updater === "function" ? updater(prev) : updater;
      draftMetaRef.current = next;
      return next;
    });
  };

  const resolveImageSrc = (path: string) => getMediaLibraryFileURL(path);

  const persistMeta = async (meta: LibraryMeta) => {
    const normalizedMeta = normalizeMeta(meta);
    const serialized = serializeMeta(normalizedMeta);
    if (serialized === lastSavedMetaRef.current) {
      return true;
    }
    try {
      const next = await updateMediaLibraryItem(detailRef.current.item.id, normalizedMeta);
      syncDetail(next);
      return true;
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存媒体库 NFO 失败");
      return false;
    }
  };

  const handleSaveEdit = () => {
    startTransition(async () => {
      const saved = await persistMeta(draftMetaRef.current);
      if (saved) {
        setIsEditing(false);
      }
    });
  };

  const handleCancelEdit = () => {
    const nextDraftMeta = cloneMeta(detailRef.current.meta);
    setDraftMeta(nextDraftMeta);
    draftMetaRef.current = nextDraftMeta;
    setIsEditing(false);
    setMessage("");
  };

  const stage = (
      <div className={`panel media-library-detail-stage media-library-backdrop${selectedCover ? "" : " media-library-backdrop-empty"}${stageOnly ? " media-library-detail-stage-inline" : ""}`}>
        {selectedCover ? (
          <button type="button" className="media-library-backdrop-hit" onClick={() => setPreview({ title: "封面", path: selectedCover, name: "封面" })}>
            <img src={resolveImageSrc(selectedCover)} alt="封面" className="media-library-backdrop-image" />
          </button>
        ) : (
          <div className="library-preview-empty media-library-backdrop-fallback">暂无封面</div>
        )}
        <div className="media-library-backdrop-scrim" aria-hidden="true" />
        <div className="media-library-stage-actions">
          {message ? (
            <span className="review-message" data-tone="danger">
              {message}
            </span>
          ) : null}
          {isEditing ? (
            <>
              <button className="media-library-stage-action-btn" type="button" onClick={handleCancelEdit} disabled={isPending} aria-label="取消编辑">
                <X size={16} />
              </button>
              <button className="media-library-stage-action-btn media-library-stage-action-btn-primary" type="button" onClick={handleSaveEdit} disabled={isPending} aria-label="保存编辑">
                <Check size={16} />
              </button>
            </>
          ) : (
            <button className="media-library-stage-action-btn" type="button" onClick={() => { setMessage(""); setIsEditing(true); }} disabled={isPending} aria-label="编辑">
              <Pencil size={16} />
            </button>
          )}
        </div>
        <div className="media-library-detail-workspace">
          <div className="media-library-hero">
            <aside className="media-library-hero-poster-column">
              <div className="panel review-image-card review-image-card-poster media-library-hero-poster-card">
                <div className={`review-image-box review-image-box-poster${selectedPoster ? "" : " review-upload-empty"}`}>
                  {selectedPoster ? (
                    <button type="button" className="review-image-hit" onClick={() => setPreview({ title: "海报", path: selectedPoster, name: "海报" })}>
                      <img src={resolveImageSrc(selectedPoster)} alt="海报" className="library-poster-image" />
                    </button>
                  ) : (
                    <div className="library-preview-empty">暂无海报</div>
                  )}
                </div>
              </div>
            </aside>

            <div className="media-library-hero-side">
              {showVariantSwitch ? (
                <LibraryVariantSwitcher
                  variants={detail.variants}
                  currentKey={selectedVariantKey}
                  onSelect={setSelectedVariantKey}
                  extraClassName="media-library-hero-variant-panel"
                />
              ) : null}

              <div className="panel media-library-hero-main">
                {isEditing ? (
                  <MediaLibraryFormFields draftMeta={draftMeta} updateDraftMeta={updateDraftMeta} />
                ) : (
                  <MediaLibraryDisplayView
                    draftMeta={draftMeta}
                    displayTitle={detailDisplayTitle}
                    displayTitleSecondary={detailDisplayTitleSecondary}
                    displayNumber={detailDisplayNumber}
                    displayPlot={detailDisplayPlot}
                    displayPlotSecondary={detailDisplayPlotSecondary}
                  />
                )}
              </div>

              <FanartStrip
                files={fanartFiles}
                resolveImageSrc={resolveImageSrc}
                onPreview={setPreview}
                extraClassName="media-library-stage-fanart"
              />
            </div>
          </div>
        </div>
      </div>
  );

  return (
    <>
      {!stageOnly ? (
        <section className="panel library-detail-panel media-library-detail-shell">
          <div className="media-library-detail-topbar media-library-detail-topbar-wide">
            <div className="media-library-detail-heading">
              <Link href="/media-library" className="media-library-back-link">
                <ChevronLeft size={16} />
                返回媒体库
              </Link>
              <div className="media-library-detail-copy">
                <div className="review-list-kicker">Media Library Item</div>
                <h2 className="review-detail-title">{detailDisplayTitle}</h2>
              </div>
            </div>
          </div>
          {stage}
        </section>
      ) : (
        <div className="media-library-stage-only-shell">
          {stage}
        </div>
      )}

      <ImagePreviewOverlay preview={preview} resolveImageSrc={resolveImageSrc} onClose={() => setPreview(null)} />
    </>
  );
}
