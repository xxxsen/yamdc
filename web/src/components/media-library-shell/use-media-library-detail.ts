"use client";

import { type Dispatch, type SetStateAction, useEffect, useState } from "react";

import type { MediaLibraryDetail, MediaLibraryItem } from "@/lib/api";
import { getMediaLibraryItem } from "@/lib/api";

export interface UseMediaLibraryDetailDeps {
  setItems: Dispatch<SetStateAction<MediaLibraryItem[]>>;
}

export interface UseMediaLibraryDetailResult {
  activeDetail: MediaLibraryDetail | null;
  activeDetailID: number | null;
  detailLoading: boolean;
  detailError: string;
  openDetailModal: (id: number) => void;
  closeDetailModal: () => void;
  applyDetailChange: (next: MediaLibraryDetail) => void;
}

// useMediaLibraryDetail owns the detail modal state: which item is
// being viewed, the fetch lifecycle (loading / error / loaded), the
// Escape-to-close keyboard handler, and the "write-back into items"
// synchronisation that the detail modal triggers via onDetailChange.
//
// The modal is mounted unconditionally by the parent with
// open={activeDetailID !== null}, so closeDetailModal simply resets
// all four pieces of detail state.
export function useMediaLibraryDetail({ setItems }: UseMediaLibraryDetailDeps): UseMediaLibraryDetailResult {
  const [activeDetail, setActiveDetail] = useState<MediaLibraryDetail | null>(null);
  const [activeDetailID, setActiveDetailID] = useState<number | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState("");

  useEffect(() => {
    if (!activeDetail && activeDetailID === null) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setActiveDetail(null);
        setActiveDetailID(null);
        setDetailError("");
        setDetailLoading(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeDetail, activeDetailID]);

  const openDetailModal = (id: number) => {
    setActiveDetailID(id);
    setActiveDetail(null);
    setDetailError("");
    setDetailLoading(true);
    void (async () => {
      try {
        const next = await getMediaLibraryItem(id);
        setActiveDetail(next);
      } catch (error) {
        setDetailError(error instanceof Error ? error.message : "加载媒体详情失败");
      } finally {
        setDetailLoading(false);
      }
    })();
  };

  const closeDetailModal = () => {
    setActiveDetail(null);
    setActiveDetailID(null);
    setDetailError("");
    setDetailLoading(false);
  };

  // applyDetailChange bridges the MediaLibraryDetailShell's onDetailChange
  // callback into both the modal-local activeDetail and the parent's
  // items list. We only copy the card-relevant subset of MediaLibraryItem
  // fields, because those are what the card grid renders; anything else
  // the detail shell might mutate stays inside activeDetail and will be
  // refreshed on the next list fetch.
  const applyDetailChange = (next: MediaLibraryDetail) => {
    setActiveDetail(next);
    setItems((current) =>
      current.map((item) =>
        item.id === next.item.id
          ? {
              ...item,
              title: next.item.title,
              number: next.item.number,
              release_date: next.item.release_date,
              actors: next.item.actors,
              updated_at: next.item.updated_at,
              poster_path: next.item.poster_path,
              cover_path: next.item.cover_path,
            }
          : item,
      ),
    );
  };

  return {
    activeDetail,
    activeDetailID,
    detailLoading,
    detailError,
    openDetailModal,
    closeDetailModal,
    applyDetailChange,
  };
}
