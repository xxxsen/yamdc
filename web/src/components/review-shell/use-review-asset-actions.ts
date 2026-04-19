"use client";

import { type Dispatch, type RefObject, type SetStateAction, type TransitionStartFunction, useRef } from "react";

import { buildPayload } from "@/components/review-shell/utils";
import type { JobItem, ReviewMeta } from "@/lib/api";
import { cropPosterFromCover, saveReviewJob, uploadAsset } from "@/lib/api";

export interface UseReviewAssetActionsDeps {
  selected: JobItem | null;
  meta: ReviewMeta | null;
  metaRef: RefObject<ReviewMeta | null>;
  lastSavedPayloadRef: RefObject<string>;
  setMeta: Dispatch<SetStateAction<ReviewMeta | null>>;
  updateMeta: (patch: Partial<ReviewMeta>) => void;
  setMessage: Dispatch<SetStateAction<string>>;
  setCropOpen: Dispatch<SetStateAction<boolean>>;
  startTransition: TransitionStartFunction;
}

export interface ReviewAssetActions {
  uploadActiveRef: RefObject<boolean>;
  openCropper: () => void;
  handleCropResult: (rect: { x: number; y: number; width: number; height: number }) => void;
  handleRemoveFanart: (key: string) => void;
  openUploadPicker: (target: "cover" | "poster" | "fanart") => void;
}

export function useReviewAssetActions(deps: UseReviewAssetActionsDeps): ReviewAssetActions {
  const {
    selected,
    meta,
    metaRef,
    lastSavedPayloadRef,
    setMeta,
    updateMeta,
    setMessage,
    setCropOpen,
    startTransition,
  } = deps;

  const uploadActiveRef = useRef(false);

  // persistMetaPatch full-replaces meta (not a spread-merge) and
  // synchronously updates metaRef so callers can read it immediately
  // after this returns. Mirrors the parent's original inline helper.
  const persistMetaPatch = (nextMeta: ReviewMeta, successMessage: string) => {
    if (!selected) {
      return;
    }
    setMeta(nextMeta);
    metaRef.current = nextMeta;
    startTransition(async () => {
      try {
        const payload = buildPayload(nextMeta);
        await saveReviewJob(selected.id, payload);
        lastSavedPayloadRef.current = payload;
        setMessage(successMessage);
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "保存失败");
      }
    });
  };

  const openCropper = () => {
    if (!meta?.cover) {
      return;
    }
    setCropOpen(true);
  };

  const handleCropResult = (rect: { x: number; y: number; width: number; height: number }) => {
    if (!selected) return;
    startTransition(async () => {
      try {
        setMessage("从封面截取海报...");
        const poster = await cropPosterFromCover(selected.id, rect);
        updateMeta({ poster });
        // Manually sync metaRef here because the setMeta updater inside
        // updateMeta runs deferred inside this transition scope, so the
        // next line's buildPayload would otherwise see the old meta.
        metaRef.current = { ...(metaRef.current ?? {}), poster };
        lastSavedPayloadRef.current = buildPayload(metaRef.current);
        setCropOpen(false);
        setMessage("海报已更新");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "海报截取失败");
      }
    });
  };

  const handleRemoveFanart = (key: string) => {
    if (!selected || !metaRef.current) {
      return;
    }
    const nextMeta: ReviewMeta = {
      ...metaRef.current,
      sample_images: (metaRef.current.sample_images ?? []).filter((item) => item.key !== key),
    };
    persistMetaPatch(nextMeta, "已移除 fanart");
  };

  const doUpload = (file: File, target: "cover" | "poster" | "fanart") => {
    if (!meta || !selected) {
      return;
    }
    startTransition(async () => {
      const currentMeta = metaRef.current;
      if (!currentMeta) return;
      try {
        setMessage("上传图片...");
        const asset = await uploadAsset(file);
        let nextMeta: ReviewMeta;
        if (target === "cover") {
          nextMeta = { ...currentMeta, cover: asset };
        } else if (target === "poster") {
          nextMeta = { ...currentMeta, poster: asset };
        } else {
          nextMeta = {
            ...currentMeta,
            sample_images: [...(currentMeta.sample_images ?? []).filter((s) => s.key !== asset.key), asset],
          };
        }
        persistMetaPatch(nextMeta, "图片已更新");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "上传失败");
      }
    });
  };

  const openUploadPicker = (target: "cover" | "poster" | "fanart") => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*";
    uploadActiveRef.current = true;
    const unlock = () => {
      setTimeout(() => { uploadActiveRef.current = false; }, 300);
    };
    input.addEventListener("change", () => {
      const file = input.files?.[0];
      unlock();
      if (file) {
        doUpload(file, target);
      }
    }, { once: true });
    input.addEventListener("cancel", () => {
      unlock();
    }, { once: true });
    input.click();
  };

  return {
    uploadActiveRef,
    openCropper,
    handleCropResult,
    handleRemoveFanart,
    openUploadPicker,
  };
}
