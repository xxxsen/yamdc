"use client";

import { type RefObject } from "react";

import { getReleaseYear } from "@/components/media-library-shell/utils";
import type { MediaLibraryItem } from "@/lib/api";
import { getMediaLibraryFileURL } from "@/lib/api";

export interface MediaLibraryCardGridProps {
  visibleItems: MediaLibraryItem[];
  itemsTotal: number;
  filteredTotal: number;
  browserRef: RefObject<HTMLDivElement | null>;
  loadMoreRef: RefObject<HTMLDivElement | null>;
  showLoadMoreSentinel: boolean;
  onOpenDetail: (id: number) => void;
}

export function MediaLibraryCardGrid({
  visibleItems,
  itemsTotal,
  filteredTotal,
  browserRef,
  loadMoreRef,
  showLoadMoreSentinel,
  onOpenDetail,
}: MediaLibraryCardGridProps) {
  return (
    <div className="media-library-browser-content" ref={browserRef}>
      <div className="media-library-card-grid media-library-card-grid-wide">
        {visibleItems.map((item) => {
          const posterPath = item.poster_path || item.cover_path;
          return (
            <button
              key={item.id}
              type="button"
              className="media-library-card media-library-card-wide media-library-card-button"
              onClick={() => onOpenDetail(item.id)}
            >
              <div className="media-library-card-poster">
                {posterPath ? (
                  <img src={getMediaLibraryFileURL(posterPath)} alt={item.title || item.name} className="media-library-card-image" />
                ) : (
                  <div className="library-thumb-fallback">{(item.number || item.title || item.name).slice(0, 2).toUpperCase()}</div>
                )}
              </div>
              <div className="media-library-card-copy">
                <div className="media-library-card-title media-library-card-title-only">{item.title || item.name}</div>
                <div className="media-library-card-meta">
                  <div className="library-item-number">{item.number || "未命名影片"}</div>
                  <div className="media-library-card-year">{getReleaseYear(item.release_date) || "----"}</div>
                </div>
              </div>
            </button>
          );
        })}

        {itemsTotal === 0 ? <div className="review-empty-state media-library-grid-empty">当前媒体库里还没有项目</div> : null}
        {itemsTotal > 0 && filteredTotal === 0 ? <div className="review-empty-state media-library-grid-empty">没有匹配的媒体库项目</div> : null}
        {showLoadMoreSentinel ? <div ref={loadMoreRef} className="media-library-load-sentinel" aria-hidden="true" /> : null}
      </div>
    </div>
  );
}
