"use client";

import { type Dispatch, type RefObject, type SetStateAction, type TransitionStartFunction, useEffect, useRef, useState } from "react";

import type { LibraryPreviewState } from "@/components/library-shell/asset-gallery";
import { getUploadMessage, getVariantCoverPath, getVariantPosterPath, toErrorMessage } from "@/components/library-shell/utils";
import type { LibraryDetail, LibraryListItem, LibraryVariant } from "@/lib/api";
import { cropLibraryPosterFromCover, deleteLibraryFile, getLibraryFileURL, replaceLibraryAsset } from "@/lib/api";
import type { CropRect } from "@/components/image-cropper";

export interface UseLibraryAssetActionsDeps {
  detail: LibraryDetail | null;
  detailRef: RefObject<LibraryDetail | null>;
  currentVariant: LibraryVariant | null;
  selectedVariantKey: string;
  selectedCover: string;
  syncDetail: (next: LibraryDetail) => void;
  setItems: Dispatch<SetStateAction<LibraryListItem[]>>;
  setMessage: Dispatch<SetStateAction<string>>;
  startTransition: TransitionStartFunction;
  preview: LibraryPreviewState;
  setPreview: Dispatch<SetStateAction<LibraryPreviewState>>;
  setCropOpen: Dispatch<SetStateAction<boolean>>;
}

export interface LibraryAssetActions {
  uploadActiveRef: RefObject<boolean>;
  resolveImage: (path: string) => string;
  openUploadPicker: (kind: "poster" | "cover" | "fanart") => void;
  handleDeleteFanart: (path: string) => void;
  openCropper: () => void;
  handleConfirmCrop: (rect: CropRect) => void;
}

export function useLibraryAssetActions(deps: UseLibraryAssetActionsDeps): LibraryAssetActions {
  const {
    detail,
    detailRef,
    currentVariant,
    selectedVariantKey,
    selectedCover,
    syncDetail,
    setItems,
    setMessage,
    startTransition,
    preview,
    setPreview,
    setCropOpen,
  } = deps;

  const apiVariantKey = currentVariant?.key ?? "";

  const [assetOverrides, setAssetOverrides] = useState<Record<string, string>>({});
  const [assetVersions, setAssetVersions] = useState<Record<string, number>>({});
  const uploadActiveRef = useRef(false);
  const assetOverridesRef = useRef<Record<string, string>>({});

  useEffect(() => {
    assetOverridesRef.current = assetOverrides;
  }, [assetOverrides]);

  useEffect(() => () => {
    for (const url of Object.values(assetOverridesRef.current)) {
      URL.revokeObjectURL(url);
    }
  }, []);

  const resolveImage = (path: string) => {
    const overrideURL = assetOverrides[path];
    if (overrideURL) {
      return overrideURL;
    }
    const version = assetVersions[path];
    if (!version) {
      return getLibraryFileURL(path);
    }
    return `${getLibraryFileURL(path)}&v=${version}`;
  };

  const bumpAssetVersion = (path: string) => {
    if (!path) {
      return;
    }
    setAssetVersions((prev) => ({ ...prev, [path]: Date.now() }));
  };

  const setAssetOverride = (path: string, file: File) => {
    if (!path) {
      return;
    }
    const nextURL = URL.createObjectURL(file);
    setAssetOverrides((prev) => {
      const existing = prev[path];
      if (existing) {
        URL.revokeObjectURL(existing);
      }
      return { ...prev, [path]: nextURL };
    });
  };

  const clearAssetOverride = (path: string) => {
    setAssetOverrides((prev) => {
      const existing = prev[path];
      if (!existing) {
        return prev;
      }
      URL.revokeObjectURL(existing);
      const next = { ...prev };
      delete next[path];
      return next;
    });
  };

  const openUploadPicker = (kind: "poster" | "cover" | "fanart") => {
    if (!detail) {
      return;
    }
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*";
    uploadActiveRef.current = true;
    const unlock = () => {
      setTimeout(() => { uploadActiveRef.current = false; }, 300);
    };
    input.addEventListener("change", () => {
      const file = input.files?.[0] ?? null;
      unlock();
      if (!file) {
        return;
      }
      startTransition(async () => {
        try {
          setMessage(getUploadMessage(kind, "start"));
          const next = await replaceLibraryAsset(detail.item.rel_path, apiVariantKey, kind, file);
          syncDetail(next);
          setItems((prev) => prev.map((item) => (item.rel_path === next.item.rel_path ? next.item : item)));
          if (kind === "poster") {
            setAssetOverride(getVariantPosterPath(next, apiVariantKey), file);
          } else if (kind === "cover") {
            setAssetOverride(getVariantCoverPath(next, apiVariantKey), file);
          }
          setMessage(getUploadMessage(kind, "done"));
        } catch (error) {
          setMessage(toErrorMessage(error, "替换图片失败"));
        }
      });
    }, { once: true });
    input.addEventListener("cancel", () => {
      unlock();
    }, { once: true });
    input.click();
  };

  const handleDeleteFanart = (path: string) => {
    startTransition(async () => {
      try {
        setMessage("删除 extrafanart...");
        const next = await deleteLibraryFile(path);
        clearAssetOverride(path);
        setAssetVersions((prev) => {
          if (!(path in prev)) {
            return prev;
          }
          const nextVersions = { ...prev };
          delete nextVersions[path];
          return nextVersions;
        });
        if (preview?.path === path) {
          setPreview(null);
        }
        syncDetail(next);
        setItems((prev) => prev.map((item) => (item.rel_path === next.item.rel_path ? next.item : item)));
        setMessage("Extrafanart 已删除");
      } catch (error) {
        setMessage(toErrorMessage(error, "删除 extrafanart 失败"));
      }
    });
  };

  const openCropper = () => {
    if (!selectedCover) {
      return;
    }
    setCropOpen(true);
  };

  // 截取按钮回调: ImageCropper 已经把 display→natural 缩放算完, 我们这里
  // 只做 detail 守卫和 API 调用 + 缓存失效, 不重复手势/计算逻辑.
  const handleConfirmCrop = (rect: CropRect) => {
    if (!detail || !selectedCover) {
      return;
    }
    const posterPathVariantKey = currentVariant?.key ?? selectedVariantKey;
    startTransition(async () => {
      try {
        setMessage("从封面截取海报...");
        const currentPosterPath = getVariantPosterPath(detailRef.current, posterPathVariantKey);
        const next = await cropLibraryPosterFromCover(detail.item.rel_path, apiVariantKey, rect);
        clearAssetOverride(currentPosterPath);
        syncDetail(next);
        setItems((prev) => prev.map((item) => (item.rel_path === next.item.rel_path ? next.item : item)));
        bumpAssetVersion(getVariantPosterPath(next, posterPathVariantKey));
        setCropOpen(false);
        setMessage("海报已更新");
      } catch (error) {
        setMessage(toErrorMessage(error, "海报截取失败"));
      }
    });
  };

  return {
    uploadActiveRef,
    resolveImage,
    openUploadPicker,
    handleDeleteFanart,
    openCropper,
    handleConfirmCrop,
  };
}
