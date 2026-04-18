"use client";

import {
  type Dispatch,
  type SetStateAction,
  useEffect,
  useEffectEvent,
  useRef,
  useState,
  useTransition,
} from "react";

import { cloneMeta, normalizeMeta, serializeMeta } from "@/components/media-library-detail-shell/utils";
import type { LibraryMeta, MediaLibraryDetail } from "@/lib/api";
import { getMediaLibraryItem, updateMediaLibraryItem } from "@/lib/api";

export interface UseMediaLibraryDetailStateDeps {
  initialDetail: MediaLibraryDetail;
  onDetailChange?: (detail: MediaLibraryDetail) => void;
}

export interface UseMediaLibraryDetailStateResult {
  detail: MediaLibraryDetail;
  draftMeta: LibraryMeta;
  selectedVariantKey: string;
  setSelectedVariantKey: Dispatch<SetStateAction<string>>;
  message: string;
  isEditing: boolean;
  isPending: boolean;
  updateDraftMeta: (updater: SetStateAction<LibraryMeta>) => void;
  handleStartEdit: () => void;
  handleSaveEdit: () => void;
  handleCancelEdit: () => void;
}

// useMediaLibraryDetailState owns the detail view's editable-metadata
// lifecycle: the authoritative detail snapshot, the per-edit draft,
// the "last saved" serialization fingerprint used for dirty-checking,
// the 8s background polling, the success/error toast, and the
// save/cancel/start-edit handlers that tie it all together.
//
// Kept in the parent on purpose:
// * preview state - pure UI concern, only used by the image overlay
//   and the click handlers in the stage JSX, no overlap with the
//   edit lifecycle.
// * resolveImageSrc - a single-line wrapper around
//   getMediaLibraryFileURL, used only in render.
//
// detailRef / draftMetaRef are refs rather than state because
// persistMeta / handleSaveEdit run inside startTransition and need
// the latest value WITHOUT waiting for the scheduled state update
// to commit (which would only land after the transition finishes).
export function useMediaLibraryDetailState({
  initialDetail,
  onDetailChange,
}: UseMediaLibraryDetailStateDeps): UseMediaLibraryDetailStateResult {
  const initialDraftMeta = cloneMeta(initialDetail.meta);
  const [detail, setDetail] = useState<MediaLibraryDetail>(initialDetail);
  const [selectedVariantKey, setSelectedVariantKey] = useState(
    initialDetail.primary_variant_key || initialDetail.variants[0]?.key || "",
  );
  const [draftMeta, setDraftMeta] = useState<LibraryMeta>(initialDraftMeta);
  const [message, setMessage] = useState("");
  const [isEditing, setIsEditing] = useState(false);
  const [isPending, startTransition] = useTransition();
  const detailRef = useRef<MediaLibraryDetail>(initialDetail);
  const draftMetaRef = useRef<LibraryMeta>(initialDraftMeta);
  const lastSavedMetaRef = useRef(serializeMeta(initialDraftMeta));

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

  const handleStartEdit = () => {
    setMessage("");
    setIsEditing(true);
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

  return {
    detail,
    draftMeta,
    selectedVariantKey,
    setSelectedVariantKey,
    message,
    isEditing,
    isPending,
    updateDraftMeta,
    handleStartEdit,
    handleSaveEdit,
    handleCancelEdit,
  };
}
