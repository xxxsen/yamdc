"use client";

import { Crop, Plus, RefreshCw, Search, Trash2, X } from "lucide-react";
import { type SetStateAction, useDeferredValue, useEffect, useEffectEvent, useRef, useState, useTransition } from "react";

import { ImageCropper, type CropRect } from "@/components/image-cropper";
import { Button } from "@/components/ui/button";
import { TokenEditor } from "@/components/ui/token-editor";
import type { LibraryDetail, LibraryListItem, LibraryMeta, MediaLibraryStatus, TaskState } from "@/lib/api";
import { cropLibraryPosterFromCover, deleteLibraryFile, deleteLibraryItem, getLibraryFileURL, getLibraryItem, getMediaLibraryStatus, listLibraryItems, replaceLibraryAsset, triggerMoveToMediaLibrary, updateLibraryItem } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

interface Props {
  items: LibraryListItem[];
  initialDetail: LibraryDetail | null;
  initialMediaStatus: MediaLibraryStatus | null;
}

function cloneMeta(meta: LibraryMeta | null | undefined): LibraryMeta {
  if (!meta) {
    return {
      title: "", title_translated: "", original_title: "", plot: "", plot_translated: "",
      number: "", release_date: "", runtime: 0, studio: "", label: "", series: "",
      director: "", actors: [], genres: [], poster_path: "", cover_path: "",
      fanart_path: "", thumb_path: "", source: "", scraped_at: "",
    };
  }
  return {
    ...meta,
    actors: [...meta.actors],
    genres: [...meta.genres],
  };
}

function pickVariant(detail: LibraryDetail | null, key: string) {
  if (!detail) {
    return null;
  }
  return detail.variants.find((item) => item.key === key) ?? detail.variants.at(0) ?? null;
}

function serializeMeta(meta: LibraryMeta) {
  return JSON.stringify({
    ...meta,
    actors: meta.actors.map((item) => item.trim()).filter(Boolean),
    genres: meta.genres.map((item) => item.trim()).filter(Boolean),
  });
}

function normalizeMeta(meta: LibraryMeta): LibraryMeta {
  return {
    ...meta,
    actors: meta.actors.map((item) => item.trim()).filter(Boolean),
    genres: meta.genres.map((item) => item.trim()).filter(Boolean),
  };
}

function getCardImage(item: LibraryListItem) {
  return item.poster_path || item.cover_path;
}

function itemActors(item: LibraryListItem) {
  return Array.isArray(item.actors) ? item.actors : [];
}

function getVariantPosterPath(detail: LibraryDetail | null, variantKey: string) {
  const variant = pickVariant(detail, variantKey);
  return variant?.poster_path || variant?.meta.poster_path || detail?.meta.poster_path || detail?.item.poster_path || "";
}

function getVariantCoverPath(detail: LibraryDetail | null, variantKey: string) {
  const variant = pickVariant(detail, variantKey);
  return (
    variant?.cover_path ||
    variant?.meta.cover_path ||
    variant?.meta.fanart_path ||
    variant?.meta.thumb_path ||
    detail?.meta.cover_path ||
    detail?.meta.fanart_path ||
    detail?.meta.thumb_path ||
    detail?.item.cover_path ||
    ""
  );
}

function hasTranslatedCopy(meta: LibraryMeta | null) {
  if (!meta) {
    return false;
  }
  return Boolean(meta.title_translated.trim() || meta.plot_translated.trim());
}

function taskPercent(state: TaskState | null) {
  if (!state || state.total <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(100, Math.round((state.processed / state.total) * 100)));
}

function toMoveToMediaLibraryMessage(error: unknown) {
  const raw = error instanceof Error ? error.message : "启动移动到媒体库失败";
  const text = raw.trim();
  if (text.includes("move to media library is already running")) {
    return "媒体库移动任务已在进行中";
  }
  if (text.includes("media library sync is running")) {
    return "媒体库同步进行中，暂时无法移动";
  }
  if (text.includes("library dir is not configured")) {
    return "未配置媒体库目录";
  }
  if (text.includes("save dir is not configured")) {
    return "未配置保存目录";
  }
  return raw;
}

function getRefreshButtonLabel(refreshRunning: boolean, refreshCompletedFlash: boolean) {
  if (refreshRunning) return "扫描中...";
  if (refreshCompletedFlash) return "扫描完成";
  return "重新扫描库";
}

function getMoveButtonLabel(moveBusy: boolean, moveRunning: boolean, moveState: TaskState | null, moveCompletedFlash: boolean) {
  if (moveBusy) {
    if (moveRunning && moveState) return `移动中 ${moveState.processed}/${moveState.total}`;
    return "移动中...";
  }
  if (moveCompletedFlash) return "移动完成";
  return "移动到媒体库";
}

function markMoveStarting(current: MediaLibraryStatus | null): MediaLibraryStatus | null {
  if (!current) return current;
  return {
    ...current,
    move: {
      ...current.move,
      status: "starting",
      message: "移动到媒体库中",
      updated_at: Date.now(),
      started_at: current.move.started_at || Date.now(),
    },
  };
}

function markMoveRunning(current: MediaLibraryStatus | null): MediaLibraryStatus | null {
  if (!current) return current;
  return {
    ...current,
    move: {
      ...current.move,
      status: "running",
      message: "移动到媒体库中",
      updated_at: Date.now(),
    },
  };
}

function handleMoveToMediaLibraryError(
  error: unknown,
  setMessage: (msg: string) => void,
  setMoveProgressVisible: (v: boolean) => void,
  setMediaStatus: (updater: SetStateAction<MediaLibraryStatus | null>) => void,
  observedMoveRunningRef: { current: boolean },
) {
  const msg = toMoveToMediaLibraryMessage(error);
  setMessage(msg);
  if (msg === "媒体库移动任务已在进行中") {
    setMoveProgressVisible(true);
    setMediaStatus(markMoveRunning);
    observedMoveRunningRef.current = true;
  } else {
    setMoveProgressVisible(false);
    void getMediaLibraryStatus().then((next) => setMediaStatus(next)).catch(() => {});
  }
}

function resolveSelectedPoster(variant: ReturnType<typeof pickVariant>, meta: LibraryMeta, detail: LibraryDetail | null) {
  return variant?.poster_path || variant?.meta.poster_path || meta.poster_path || detail?.item.poster_path || "";
}

function resolveSelectedCover(variant: ReturnType<typeof pickVariant>, meta: LibraryMeta, detail: LibraryDetail | null) {
  return (
    variant?.cover_path ||
    variant?.meta.cover_path ||
    variant?.meta.fanart_path ||
    variant?.meta.thumb_path ||
    meta.cover_path ||
    meta.fanart_path ||
    detail?.item.cover_path ||
    ""
  );
}

function getUploadMessage(kind: "poster" | "cover" | "fanart", phase: "start" | "done"): string {
  if (kind === "poster") return phase === "start" ? "替换当前实例海报..." : "当前实例海报已更新";
  if (kind === "cover") return phase === "start" ? "替换当前实例封面..." : "当前实例封面已更新";
  return phase === "start" ? "上传 extrafanart..." : "Extrafanart 已上传";
}

function pickNextVariantKey(current: string, detail: LibraryDetail): string {
  if (current && detail.variants.some((item) => item.key === current)) return current;
  return detail.primary_variant_key || detail.variants.at(0)?.key || "";
}

function pickNextCopyMode(current: "translated" | "original", meta: LibraryMeta): "translated" | "original" {
  return current === "translated" && !hasTranslatedCopy(meta) ? "original" : current;
}

function toErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}

function getInitialSelectedPath(detail: LibraryDetail | null, items: LibraryListItem[]): string {
  return detail?.item.rel_path ?? items.at(0)?.rel_path ?? "";
}

function getInitialVariantKey(detail: LibraryDetail | null): string {
  return detail?.primary_variant_key ?? detail?.variants.at(0)?.key ?? "";
}

function getInitialCopyMode(detail: LibraryDetail | null): "translated" | "original" {
  return hasTranslatedCopy(detail?.meta ?? null) ? "translated" : "original";
}

function getInitialMessage(items: LibraryListItem[]): string {
  return items.length === 0 ? "当前 savedir 里还没有已入库内容" : "";
}

function LibraryBottomActions({
  refreshBusy,
  moveBusy,
  mediaSyncRunning,
  configured,
  refreshButtonLabel,
  moveButtonLabel,
  moveProgressVisible,
  moveState,
  moveProgress,
  onRefresh,
  onMove,
}: {
  refreshBusy: boolean;
  moveBusy: boolean;
  mediaSyncRunning: boolean;
  configured: boolean;
  refreshButtonLabel: string;
  moveButtonLabel: string;
  moveProgressVisible: boolean;
  moveState: TaskState | null;
  moveProgress: number;
  onRefresh: () => void;
  onMove: () => void;
}) {
  return (
    <div className="library-bottom-actions">
      <Button
        variant="primary"
        className="media-library-sync-btn library-action-btn"
        onClick={onRefresh}
        disabled={refreshBusy || moveBusy}
        leftIcon={
          <RefreshCw size={16} className={refreshBusy ? "media-library-sync-icon-spinning" : ""} />
        }
      >
        {refreshButtonLabel}
      </Button>
      <Button
        variant="primary"
        className="media-library-sync-btn library-action-btn library-action-btn-progress"
        onClick={onMove}
        disabled={refreshBusy || moveBusy || mediaSyncRunning || !configured}
      >
        {moveProgressVisible && moveState ? (
          <span className="library-action-progress" aria-hidden="true">
            <span className="library-action-progress-fill" style={{ width: `${moveProgress}%` }} />
          </span>
        ) : null}
        <span className="library-action-btn-content">
          {moveBusy ? <RefreshCw size={16} className="media-library-sync-icon-spinning" /> : <Plus size={16} />}
          <span>{moveButtonLabel}</span>
        </span>
      </Button>
    </div>
  );
}

function AppToast({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div className="app-toast app-toast-top" data-tone={/失败|error/i.test(message) ? "danger" : undefined} role="status" aria-live="polite">
      {message}
    </div>
  );
}

export function LibraryShell({ items: initialItems, initialDetail, initialMediaStatus }: Props) {
  const initialDraftMeta = cloneMeta(initialDetail?.meta);
  const [items, setItems] = useState(initialItems);
  const [selectedPath, setSelectedPath] = useState(getInitialSelectedPath(initialDetail, initialItems));
  const [detail, setDetail] = useState<LibraryDetail | null>(initialDetail);
  const [selectedVariantKey, setSelectedVariantKey] = useState(getInitialVariantKey(initialDetail));
  const [copyMode, setCopyMode] = useState<"translated" | "original">(getInitialCopyMode(initialDetail));
  const [draftMeta, setDraftMeta] = useState<LibraryMeta>(initialDraftMeta);
  const [keyword, setKeyword] = useState("");
  const [message, setMessage] = useState(getInitialMessage(initialItems));
  const [preview, setPreview] = useState<{ title: string; path: string; name: string } | null>(null);
  const [mediaStatus, setMediaStatus] = useState<MediaLibraryStatus | null>(initialMediaStatus);
  const [assetOverrides, setAssetOverrides] = useState<Record<string, string>>({});
  const [assetVersions, setAssetVersions] = useState<Record<string, number>>({});
  const [refreshRunning, setRefreshRunning] = useState(false);
  const [refreshCompletedFlash, setRefreshCompletedFlash] = useState(false);
  const [moveStarting, setMoveStarting] = useState(false);
  const [moveCompletedFlash, setMoveCompletedFlash] = useState(false);
  const [moveProgressVisible, setMoveProgressVisible] = useState(initialMediaStatus?.move.status === "running");
  const [cropOpen, setCropOpen] = useState(false);
  const [isPending, startTransition] = useTransition();
  const uploadActiveRef = useRef(false);
  const detailAbortRef = useRef<AbortController | null>(null);
  const detailRef = useRef<LibraryDetail | null>(initialDetail);
  const draftMetaRef = useRef<LibraryMeta>(initialDraftMeta);
  const lastSavedPathRef = useRef(initialDetail?.item.rel_path ?? "");
  const lastSavedMetaRef = useRef(initialDetail ? serializeMeta(initialDraftMeta) : "");
  const saveQueueRef = useRef<Promise<boolean>>(Promise.resolve(true));
  const assetOverridesRef = useRef<Record<string, string>>({});
  const observedMoveRunningRef = useRef(initialMediaStatus?.move.status === "running");
  const deferredKeyword = useDeferredValue(keyword);

  const query = deferredKeyword.trim().toLowerCase();
  const filteredItems = !query
    ? items
    : items.filter((item) => {
      const actors = itemActors(item);
      const haystack = [
        item.title,
        item.number,
        actors.join(" "),
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(query);
    });

  const currentVariant = pickVariant(detail, selectedVariantKey);
  const showVariantSwitch = (detail?.variants.length ?? 0) > 1;
  const activeTitleValue = copyMode === "translated" ? draftMeta.title_translated : draftMeta.title;
  const activePlotValue = copyMode === "translated" ? draftMeta.plot_translated : draftMeta.plot;
  const fanartFiles = detail?.files.filter((file) => file.rel_path.includes("/extrafanart/")) ?? [];
  const selectedPoster = resolveSelectedPoster(currentVariant, draftMeta, detail);
  const selectedCover = resolveSelectedCover(currentVariant, draftMeta, detail);
  const moveState = mediaStatus?.move ?? null;
  const moveRunning = moveState?.status === "running";
  const mediaSyncRunning = mediaStatus?.sync.status === "running";
  const moveBusy = moveStarting || moveRunning;
  const shouldPollMediaStatus = moveBusy || mediaSyncRunning;
  const refreshBusy = refreshRunning;
  const moveProgress = moveState ? taskPercent(moveState) : 0;
  const refreshButtonLabel = getRefreshButtonLabel(refreshRunning, refreshCompletedFlash);
  const moveButtonLabel = getMoveButtonLabel(moveBusy, moveRunning, moveState, moveCompletedFlash);

  useEffect(() => {
    assetOverridesRef.current = assetOverrides;
  }, [assetOverrides]);

  useEffect(() => () => {
    for (const url of Object.values(assetOverridesRef.current)) {
      URL.revokeObjectURL(url);
    }
  }, []);

  useEffect(() => {
    if (!message || /失败|error/i.test(message)) {
      return;
    }
    const timer = window.setTimeout(() => setMessage(""), 2400);
    return () => window.clearTimeout(timer);
  }, [message]);

  const refreshMediaStatus = useEffectEvent(async (signal?: AbortSignal) => {
    try {
      const next = await getMediaLibraryStatus(signal);
      setMediaStatus(next);
    } catch {
      // ignore polling errors
    }
  });

  useEffect(() => {
    if (!shouldPollMediaStatus) {
      return;
    }
    const controller = new AbortController();
    void refreshMediaStatus(controller.signal);
    const timer = window.setInterval(() => {
      void refreshMediaStatus(controller.signal);
    }, 3000);
    return () => {
      window.clearInterval(timer);
      controller.abort();
    };
  }, [shouldPollMediaStatus]);

  useEffect(() => {
    if (moveRunning) {
      setMoveProgressVisible(true);
      observedMoveRunningRef.current = true;
    }
  }, [moveRunning]);

  useEffect(() => {
    if (!refreshCompletedFlash) {
      return;
    }
    const timer = window.setTimeout(() => setRefreshCompletedFlash(false), 1000);
    return () => window.clearTimeout(timer);
  }, [refreshCompletedFlash]);

  useEffect(() => {
    if (!moveCompletedFlash) {
      return;
    }
    const timer = window.setTimeout(() => setMoveCompletedFlash(false), 1000);
    return () => window.clearTimeout(timer);
  }, [moveCompletedFlash]);

  const updateDraftMeta = (updater: SetStateAction<LibraryMeta>) => {
    setDraftMeta((prev) => {
      const next = typeof updater === "function" ? updater(prev) : updater;
      draftMetaRef.current = next;
      return next;
    });
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

  const resolveLibraryImageSrc = (path: string) => {
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

  const syncDetail = (next: LibraryDetail) => {
    setDetail(next);
    detailRef.current = next;
    setSelectedPath(next.item.rel_path);
    const nextDraftMeta = cloneMeta(next.meta);
    updateDraftMeta(nextDraftMeta);
    setCopyMode((current) => pickNextCopyMode(current, next.meta));
    setSelectedVariantKey((current) => pickNextVariantKey(current, next));
    lastSavedPathRef.current = next.item.rel_path;
    lastSavedMetaRef.current = serializeMeta(nextDraftMeta);
  };

  const syncDetailFromEffect = useEffectEvent((next: LibraryDetail) => {
    syncDetail(next);
  });

  const persistMeta = (meta: LibraryMeta, messageText: string, options?: { silent?: boolean }) => {
    const currentDetail = detailRef.current;
    if (!currentDetail) {
      return Promise.resolve(true);
    }
    const path = currentDetail.item.rel_path;
    const normalizedMeta = normalizeMeta(meta);
    const serialized = serializeMeta(normalizedMeta);
    if (path === lastSavedPathRef.current && serialized === lastSavedMetaRef.current) {
      return Promise.resolve(true);
    }
    const task = saveQueueRef.current.then(async () => {
      if (path === lastSavedPathRef.current && serialized === lastSavedMetaRef.current) {
        return true;
      }
      try {
        if (!options?.silent) {
          setMessage("保存 NFO...");
        }
        const next = await updateLibraryItem(path, normalizedMeta);
        syncDetail(next);
        setItems((prev) => prev.map((item) => (item.rel_path === next.item.rel_path ? next.item : item)));
        setMessage(messageText);
        return true;
      } catch (error) {
        setMessage(toErrorMessage(error, "保存 NFO 失败"));
        return false;
      }
    });
    saveQueueRef.current = task.catch(() => true);
    return task;
  };

  const loadDetail = (path: string) => {
    detailAbortRef.current?.abort();
    const controller = new AbortController();
    detailAbortRef.current = controller;
    setSelectedPath(path);
    startTransition(async () => {
      try {
        setMessage("加载已入库详情...");
        const next = await getLibraryItem(path, controller.signal);
        syncDetail(next);
        setMessage("");
      } catch (error) {
        if (controller.signal.aborted) return;
        setMessage(toErrorMessage(error, "加载已入库详情失败"));
      }
    });
  };

  const loadInitialDetail = useEffectEvent(async (path: string) => {
    detailAbortRef.current?.abort();
    const controller = new AbortController();
    detailAbortRef.current = controller;
    try {
      setMessage("加载已入库详情...");
      const next = await getLibraryItem(path, controller.signal);
      syncDetail(next);
      setMessage("");
    } catch (error) {
      if (controller.signal.aborted) return;
      setMessage(toErrorMessage(error, "加载已入库详情失败"));
    }
  });

  useEffect(() => {
    if (!detail && items.length > 0 && selectedPath) {
      startTransition(async () => {
        await loadInitialDetail(selectedPath);
      });
    }
  }, [detail, items.length, selectedPath, startTransition]);

  const refreshLibrary = async () => {
    const nextItems = await listLibraryItems();
    setItems(nextItems);
    if (nextItems.length === 0) {
      setDetail(null);
      detailRef.current = null;
      updateDraftMeta(cloneMeta(null));
      setSelectedPath("");
      setSelectedVariantKey("");
      lastSavedPathRef.current = "";
      lastSavedMetaRef.current = "";
      setMessage("当前 savedir 里还没有已入库内容");
      return;
    }
    const nextSelected = nextItems.some((item) => item.rel_path === selectedPath) ? selectedPath : nextItems[0].rel_path;
    const nextDetail = await getLibraryItem(nextSelected);
    syncDetail(nextDetail);
  };

  const handleRefreshLibrary = () => {
    setRefreshRunning(true);
    startTransition(async () => {
      try {
        await refreshLibrary();
        setRefreshCompletedFlash(true);
      } catch (error) {
        setMessage(toErrorMessage(error, "刷新已入库目录失败"));
      } finally {
        setRefreshRunning(false);
      }
    });
  };

  const handleMoveToMediaLibrary = () => {
    setMoveStarting(true);
    setMoveProgressVisible(true);
    setMoveCompletedFlash(false);
    setMessage("媒体库移动已启动");
    setMediaStatus(markMoveStarting);
    startTransition(async () => {
      try {
        await triggerMoveToMediaLibrary();
        const next = await getMediaLibraryStatus();
        setMediaStatus(next);
        setMoveProgressVisible(next.move.status === "running");
      } catch (error) {
        handleMoveToMediaLibraryError(error, setMessage, setMoveProgressVisible, setMediaStatus, observedMoveRunningRef);
      } finally {
        setMoveStarting(false);
      }
    });
  };

  const prevMoveRunningRef = useRef(moveRunning);

  useEffect(() => {
    if (observedMoveRunningRef.current && prevMoveRunningRef.current && !moveRunning) {
      observedMoveRunningRef.current = false;
      setRefreshRunning(true);
      startTransition(async () => {
        try {
          const nextItems = await listLibraryItems();
          setItems(nextItems);
          if (nextItems.length === 0) {
            setDetail(null);
            detailRef.current = null;
            updateDraftMeta(cloneMeta(null));
            setSelectedPath("");
            setSelectedVariantKey("");
            lastSavedPathRef.current = "";
            lastSavedMetaRef.current = "";
            setMessage("当前 savedir 里还没有已入库内容");
            return;
          }
          const nextSelected = nextItems.some((item) => item.rel_path === selectedPath) ? selectedPath : nextItems[0].rel_path;
          const nextDetail = await getLibraryItem(nextSelected);
          syncDetailFromEffect(nextDetail);
        } catch (error) {
          setMessage(toErrorMessage(error, "刷新已入库目录失败"));
        } finally {
          setRefreshRunning(false);
          setMoveProgressVisible(false);
          setMoveCompletedFlash(true);
        }
      });
    }
    prevMoveRunningRef.current = moveRunning;
  }, [moveRunning, selectedPath, startTransition]);

  const handleBlurSave = () => {
    startTransition(async () => {
      await persistMeta(draftMetaRef.current, "已自动保存", { silent: true });
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
          const next = await replaceLibraryAsset(detail.item.rel_path, currentVariant?.key ?? "", kind, file);
          syncDetail(next);
          setItems((prev) => prev.map((item) => (item.rel_path === next.item.rel_path ? next.item : item)));
          if (kind === "poster") {
            setAssetOverride(getVariantPosterPath(next, currentVariant?.key ?? ""), file);
          } else if (kind === "cover") {
            setAssetOverride(getVariantCoverPath(next, currentVariant?.key ?? ""), file);
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

  const handleDeleteLibraryItem = () => {
    if (!detail) {
      return;
    }
    const targetPath = detail.item.rel_path;
    startTransition(async () => {
      try {
        setMessage("删除已入库目录...");
        await deleteLibraryItem(targetPath);
        const nextItems = await listLibraryItems();
        setItems(nextItems);
        if (nextItems.length === 0) {
          setDetail(null);
          detailRef.current = null;
          updateDraftMeta(cloneMeta(null));
          setSelectedPath("");
          setSelectedVariantKey("");
          lastSavedPathRef.current = "";
          lastSavedMetaRef.current = "";
          setMessage("已入库目录已删除");
          return;
        }
        const nextSelected = nextItems.some((item) => item.rel_path === selectedPath && item.rel_path !== targetPath)
          ? selectedPath
          : nextItems[0].rel_path;
        const nextDetail = await getLibraryItem(nextSelected);
        syncDetail(nextDetail);
        setMessage("已入库目录已删除");
      } catch (error) {
        setMessage(toErrorMessage(error, "删除已入库目录失败"));
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
    startTransition(async () => {
      try {
        setMessage("从封面截取海报...");
        const currentPosterPath = getVariantPosterPath(detailRef.current, currentVariant?.key ?? selectedVariantKey);
        const next = await cropLibraryPosterFromCover(detail.item.rel_path, currentVariant?.key ?? "", rect);
        clearAssetOverride(currentPosterPath);
        syncDetail(next);
        setItems((prev) => prev.map((item) => (item.rel_path === next.item.rel_path ? next.item : item)));
        bumpAssetVersion(getVariantPosterPath(next, currentVariant?.key ?? selectedVariantKey));
        setCropOpen(false);
        setMessage("海报已更新");
      } catch (error) {
        setMessage(toErrorMessage(error, "海报截取失败"));
      }
    });
  };

  return (
    <div className="library-shell">
      <section className="panel library-list-panel">
        <div className="library-list-head">
          <div className="library-list-kicker">Saved Library</div>
          <h2 className="library-list-title">已入库</h2>
          <p className="library-list-subtitle">浏览 `savedir` 下的媒体目录，并直接修改目录内全部 NFO 的共享元数据。</p>
        </div>

        <label className="file-list-search library-search">
          <Search size={16} />
          <input
            className="input file-list-search-input"
            placeholder="按标题 / 影片 ID / 演员搜索"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
        </label>

        <div className="library-item-list">
          {filteredItems.map((item) => {
            const imagePath = getCardImage(item);
            return (
              <button
                key={item.rel_path}
                type="button"
                className="library-item-card"
                data-active={selectedPath === item.rel_path}
                onClick={() => loadDetail(item.rel_path)}
              >
                <div className="library-item-thumb">
                  {imagePath ? (
                    <img src={resolveLibraryImageSrc(imagePath)} alt={item.title} className="library-thumb-image" />
                  ) : (
                    <div className="library-thumb-fallback">{(item.number || item.title || item.name).slice(0, 2).toUpperCase()}</div>
                  )}
                </div>
                <div className="library-item-copy">
                  <div className="library-item-topline">
                    <span className="library-item-number">{item.number || "未命名影片"}</span>
                    <span className="library-item-time">{formatUnixMillis(item.updated_at)}</span>
                  </div>
                  {item.conflict ? (
                    <div className="library-item-badge-row">
                      <span className="badge library-conflict-badge">已存在(冲突)</span>
                    </div>
                  ) : null}
                  <div className="library-item-title" title={item.title || item.name}>{item.title || item.name}</div>
                  <div className="library-item-meta">{itemActors(item).length > 0 ? itemActors(item).join(" / ") : "暂无演员信息"}</div>
                  <div className="library-item-path">{item.rel_path}</div>
                  <div className="library-item-footnote">{item.variant_count > 1 ? `${item.variant_count} 个文件实例` : "单实例目录"}</div>
                </div>
              </button>
            );
          })}
          {filteredItems.length === 0 ? <div className="review-empty-state">没有匹配的已入库项目</div> : null}
        </div>

        <LibraryBottomActions
          refreshBusy={refreshBusy}
          moveBusy={moveBusy}
          mediaSyncRunning={mediaSyncRunning}
          configured={!!mediaStatus?.configured}
          refreshButtonLabel={refreshButtonLabel}
          moveButtonLabel={moveButtonLabel}
          moveProgressVisible={moveProgressVisible}
          moveState={moveState}
          moveProgress={moveProgress}
          onRefresh={handleRefreshLibrary}
          onMove={handleMoveToMediaLibrary}
        />
      </section>

      <section className="panel library-detail-panel">
        {detail ? (
          <>
            <div className="review-header library-detail-header">
              <div>
                <div className="review-list-kicker">Library Editor</div>
                <h2 className="review-detail-title">已入库内容</h2>
                <div className="review-subtitle">{detail.item.rel_path}</div>
              </div>
              <div className="review-actions library-detail-actions">
                <div className="library-copy-toggle" role="tablist" aria-label="标题与简介语言切换">
                  <button
                    type="button"
                    className="library-copy-toggle-btn"
                    data-active={copyMode === "translated"}
                    onClick={() => setCopyMode("translated")}
                  >
                    中文
                  </button>
                  <button
                    type="button"
                    className="library-copy-toggle-btn"
                    data-active={copyMode === "original"}
                    onClick={() => setCopyMode("original")}
                  >
                    原文
                  </button>
                </div>
                {detail.item.conflict ? <span className="badge library-conflict-badge">已存在(冲突)</span> : null}
                <Button
                  className="file-action-btn file-action-btn-ghost"
                  onClick={handleDeleteLibraryItem}
                  disabled={isPending}
                  leftIcon={<Trash2 size={16} />}
                >
                  删除
                </Button>
              </div>
            </div>

            {showVariantSwitch ? (
              <div className="panel library-variant-panel">
                <div className="library-variant-list">
                  {detail.variants.map((variant) => (
                    <button
                      key={variant.key}
                      type="button"
                      className="library-variant-chip"
                      data-active={currentVariant?.key === variant.key}
                      onClick={() => setSelectedVariantKey(variant.key)}
                    >
                      <span className="library-variant-chip-title">{variant.label}</span>
                      <span className="library-variant-chip-meta">{variant.base_name}</span>
                    </button>
                  ))}
                </div>
              </div>
            ) : null}

            <div className="review-content review-content-single">
              <div className="review-form library-detail-form">
                <div className="review-main-layout library-main-layout">
                  <div className="review-top-fields">
                    <div className="review-field">
                      <span className="review-label review-label-side">标题</span>
                      <input
                        className="input review-input-strong"
                        placeholder={copyMode === "translated" ? draftMeta.title || "暂无中文标题" : "输入原始标题"}
                        value={activeTitleValue}
                        onChange={(e) =>
                          updateDraftMeta((prev) => ({
                            ...prev,
                            [copyMode === "translated" ? "title_translated" : "title"]: e.target.value,
                          }))
                        }
                        onBlur={handleBlurSave}
                      />
                    </div>
                    <div className="review-meta-row review-meta-row-2 review-meta-row-top">
                      <div className="review-field">
                        <span className="review-label review-label-side">导演</span>
                        <input
                          className="input"
                          value={draftMeta.director}
                          onChange={(e) => updateDraftMeta((prev) => ({ ...prev, director: e.target.value }))}
                          onBlur={handleBlurSave}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">片商</span>
                        <input
                          className="input"
                          value={draftMeta.studio}
                          onChange={(e) => updateDraftMeta((prev) => ({ ...prev, studio: e.target.value }))}
                          onBlur={handleBlurSave}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">发行商</span>
                        <input
                          className="input"
                          value={draftMeta.label}
                          onChange={(e) => updateDraftMeta((prev) => ({ ...prev, label: e.target.value }))}
                          onBlur={handleBlurSave}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">系列</span>
                        <input
                          className="input"
                          value={draftMeta.series}
                          onChange={(e) => updateDraftMeta((prev) => ({ ...prev, series: e.target.value }))}
                          onBlur={handleBlurSave}
                        />
                      </div>
                    </div>
                    <div className="review-meta-row review-meta-row-2 library-meta-grid">
                      <div className="review-field">
                        <span className="review-label review-label-side">影片 ID</span>
                        <input
                          className="input"
                          value={draftMeta.number}
                          onChange={(e) => updateDraftMeta((prev) => ({ ...prev, number: e.target.value }))}
                          onBlur={handleBlurSave}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">发行日期</span>
                        <input
                          className="input"
                          placeholder="YYYY-MM-DD"
                          value={draftMeta.release_date}
                          onChange={(e) => updateDraftMeta((prev) => ({ ...prev, release_date: e.target.value }))}
                          onBlur={handleBlurSave}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">时长</span>
                        <input
                          className="input"
                          inputMode="numeric"
                          value={draftMeta.runtime ? String(draftMeta.runtime) : ""}
                          onChange={(e) =>
                            updateDraftMeta((prev) => ({ ...prev, runtime: Number.parseInt(e.target.value || "0", 10) || 0 }))
                          }
                          onBlur={handleBlurSave}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">来源</span>
                        <input
                          className="input"
                          value={draftMeta.source}
                          onChange={(e) => updateDraftMeta((prev) => ({ ...prev, source: e.target.value }))}
                          onBlur={handleBlurSave}
                        />
                      </div>
                    </div>
                    <div className="review-meta-row">
                      <div className="review-field review-field-area">
                        <span className="review-label review-label-side">简介</span>
                        <textarea
                          className="input review-textarea library-textarea"
                          placeholder={copyMode === "translated" ? draftMeta.plot || "暂无中文简介" : "输入原始简介"}
                          value={activePlotValue}
                          onChange={(e) =>
                            updateDraftMeta((prev) => ({
                              ...prev,
                              [copyMode === "translated" ? "plot_translated" : "plot"]: e.target.value,
                            }))
                          }
                          onBlur={handleBlurSave}
                        />
                      </div>
                    </div>
                  </div>
                  <div className="review-main-side library-actors-side">
                    <div className="review-meta-row">
                      <TokenEditor
                        idPrefix="library-token"
                        label="演员"
                        placeholder="输入后回车或逗号确认"
                        value={draftMeta.actors}
                        onChange={(next) => updateDraftMeta((prev) => ({ ...prev, actors: next }))}
                        onCommit={handleBlurSave}
                        singleLine
                      />
                    </div>
                  </div>
                  <div className="panel review-image-card review-image-card-poster review-top-poster review-main-poster">
                    <div className="review-image-card-head">
                      <span className="review-image-title">海报</span>
                      <Button
                        className="review-inline-icon-btn review-image-crop-btn"
                        onClick={openCropper}
                        aria-label="从封面截取海报"
                        title="从封面截取海报"
                        disabled={!selectedCover || isPending}
                      >
                        <Crop size={14} />
                      </Button>
                    </div>
                    <div className={`review-image-box review-image-box-poster${selectedPoster ? "" : " review-upload-empty"}`}>
                      {selectedPoster ? (
                        <button type="button" className="review-image-hit" onClick={() => { if (!uploadActiveRef.current) setPreview({ title: "海报", path: selectedPoster, name: "海报" }); }}>
                          <img src={resolveLibraryImageSrc(selectedPoster)} alt="海报" className="library-poster-image" />
                        </button>
                      ) : (
                        <div className="library-preview-empty">暂无海报</div>
                      )}
                      <button
                        type="button"
                        className="review-upload-overlay"
                        onClick={() => openUploadPicker("poster")}
                        aria-label="上传海报"
                        title="上传海报"
                        disabled={isPending}
                      >
                        <Plus size={18} />
                      </button>
                    </div>
                  </div>
                </div>

                <div className="review-meta-row review-meta-row-full">
                  <TokenEditor
                    idPrefix="library-token"
                    label="标签"
                    placeholder="输入后回车或逗号确认"
                    value={draftMeta.genres}
                    onChange={(next) => updateDraftMeta((prev) => ({ ...prev, genres: next }))}
                    onCommit={handleBlurSave}
                  />
                </div>

                <div className="review-media-offset review-cover-slot">
                  <div className="panel review-image-card review-image-card-cover">
                    <div className="review-image-card-head">
                      <span className="review-image-title">封面</span>
                    </div>
                    <div className={`review-image-box review-image-box-cover${selectedCover ? "" : " review-upload-empty"}`}>
                      {selectedCover ? (
                        <button type="button" className="review-image-hit" onClick={() => { if (!uploadActiveRef.current) setPreview({ title: "封面", path: selectedCover, name: "封面" }); }}>
                          <img src={resolveLibraryImageSrc(selectedCover)} alt="封面" className="library-cover-image" />
                        </button>
                      ) : (
                        <div className="library-preview-empty">暂无封面</div>
                      )}
                      <button
                        type="button"
                        className="review-upload-overlay"
                        onClick={() => openUploadPicker("cover")}
                        aria-label="上传封面"
                        title="上传封面"
                        disabled={isPending}
                      >
                        <Plus size={18} />
                      </button>
                    </div>
                  </div>
                </div>

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
                          e.currentTarget.scrollLeft += e.deltaY;
                          e.preventDefault();
                        }}
                      >
                        {fanartFiles.map((file) => (
                          <div key={file.rel_path} className="review-fanart-item library-fanart-item">
                            <button
                              type="button"
                              className="review-image-hit"
                              onClick={() => { if (!uploadActiveRef.current) setPreview({ title: "Extrafanart", path: file.rel_path, name: file.name }); }}
                            >
                              <img src={resolveLibraryImageSrc(file.rel_path)} alt={file.name} className="library-fanart-image" />
                            </button>
                            <Button
                              className="review-inline-icon-btn review-fanart-delete"
                              onClick={() => handleDeleteFanart(file.rel_path)}
                              aria-label="删除 extrafanart"
                              title="删除 extrafanart"
                              disabled={isPending}
                            >
                              <X size={12} />
                            </Button>
                            <div className="library-fanart-name">{file.name.split("/").pop()}</div>
                          </div>
                        ))}
                        <button type="button" className="review-fanart-item review-upload-empty" onClick={() => openUploadPicker("fanart")} disabled={isPending}>
                          <span className="review-upload-overlay review-upload-overlay-static" aria-hidden="true">
                            <Plus size={18} />
                          </span>
                        </button>
                      </div>
                    ) : (
                      <div className="review-fanart-strip library-fanart-strip">
                        <button type="button" className="review-fanart-item review-upload-empty" onClick={() => openUploadPicker("fanart")} disabled={isPending}>
                          <span className="review-upload-overlay review-upload-overlay-static" aria-hidden="true">
                            <Plus size={18} />
                          </span>
                        </button>
                      </div>
                    )}
                  </div>
                </div>


              </div>
            </div>
          </>
        ) : (
          <div className="review-empty-state">当前没有可查看的已入库目录</div>
        )}
      </section>
      {preview ? (
        <div className="review-preview-overlay" onClick={() => setPreview(null)}>
          <button type="button" className="review-preview-close" aria-label="关闭预览" onClick={() => setPreview(null)}>
            <X size={18} />
          </button>
          <div className="review-preview-dialog panel" onClick={(e) => e.stopPropagation()}>
            <div className="review-preview-title">{preview.title}</div>
            <div className="review-preview-frame">
              <img
                src={resolveLibraryImageSrc(preview.path)}
                alt={preview.name}
                style={{ width: "100%", height: "100%", objectFit: "contain", objectPosition: "center", display: "block" }}
              />
            </div>
          </div>
        </div>
      ) : null}
      {cropOpen && selectedCover ? (
        <ImageCropper
          open
          imageSrc={resolveLibraryImageSrc(selectedCover)}
          onClose={() => setCropOpen(false)}
          onConfirm={handleConfirmCrop}
          isPending={isPending}
        />
      ) : null}
      <AppToast message={message} />
    </div>
  );
}
