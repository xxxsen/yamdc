"use client";

import dynamic from "next/dynamic";
import { type SetStateAction, useDeferredValue, useEffect, useEffectEvent, useRef, useState, useTransition } from "react";

// ImageCropper 只在用户点 "裁剪封面/海报" 时才挂上. 把 200+ 行 cropper +
// 指针/canvas 代码从 /library 首屏 JS 里踢出去. 详见 §5.2.
const ImageCropper = dynamic(
  () => import("@/components/image-cropper").then((m) => m.ImageCropper),
  { ssr: false },
);

import { AppToast } from "@/components/library-shell/app-toast";
import {
  LibraryCoverCard,
  LibraryFanartStrip,
  LibraryPosterCard,
  LibraryPreviewOverlay,
  type LibraryPreviewState,
} from "@/components/library-shell/asset-gallery";
import { LibraryBottomActions } from "@/components/library-shell/bottom-actions";
import { LibraryDetailHeader } from "@/components/library-shell/detail-header";
import { LibraryFormFields } from "@/components/library-shell/form-fields";
import { LibraryListPanel } from "@/components/library-shell/list-panel";
import { useLibraryAssetActions } from "@/components/library-shell/use-library-asset-actions";
import { useLibraryMoveRefresh } from "@/components/library-shell/use-library-move-refresh";
import {
  cloneMeta,
  getInitialCopyMode,
  getInitialMessage,
  getInitialSelectedPath,
  getInitialVariantKey,
  itemActors,
  normalizeMeta,
  pickNextCopyMode,
  pickNextVariantKey,
  pickVariant,
  resolveSelectedCover,
  resolveSelectedPoster,
  serializeMeta,
  toErrorMessage,
} from "@/components/library-shell/utils";
import { LibraryVariantSwitcher } from "@/components/library-shell/variant-switcher";
import { TokenEditor } from "@/components/ui/token-editor";
import type { LibraryDetail, LibraryListItem, LibraryMeta, MediaLibraryStatus } from "@/lib/api";
import { deleteLibraryItem, getLibraryItem, listLibraryItems, updateLibraryItem } from "@/lib/api";

interface Props {
  items: LibraryListItem[];
  initialDetail: LibraryDetail | null;
  initialMediaStatus: MediaLibraryStatus | null;
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
  const [preview, setPreview] = useState<LibraryPreviewState>(null);
  const [cropOpen, setCropOpen] = useState(false);
  const [isPending, startTransition] = useTransition();
  const detailAbortRef = useRef<AbortController | null>(null);
  const detailRef = useRef<LibraryDetail | null>(initialDetail);
  const draftMetaRef = useRef<LibraryMeta>(initialDraftMeta);
  const lastSavedPathRef = useRef(initialDetail?.item.rel_path ?? "");
  const lastSavedMetaRef = useRef(initialDetail ? serializeMeta(initialDraftMeta) : "");
  const saveQueueRef = useRef<Promise<boolean>>(Promise.resolve(true));
  const deferredKeyword = useDeferredValue(keyword);

  const query = deferredKeyword.trim().toLowerCase();
  const filteredItems = !query ? items : items.filter((item) => {
    const haystack = [item.title, item.number, itemActors(item).join(" ")].join(" ").toLowerCase();
    return haystack.includes(query);
  });

  const currentVariant = pickVariant(detail, selectedVariantKey);
  const showVariantSwitch = (detail?.variants.length ?? 0) > 1;
  const activeTitleValue = copyMode === "translated" ? draftMeta.title_translated : draftMeta.title;
  const activePlotValue = copyMode === "translated" ? draftMeta.plot_translated : draftMeta.plot;
  const fanartFiles = detail?.files.filter((file) => file.rel_path.includes("/extrafanart/")) ?? [];
  const selectedPoster = resolveSelectedPoster(currentVariant, draftMeta, detail);
  const selectedCover = resolveSelectedCover(currentVariant, draftMeta, detail);

  useEffect(() => {
    if (!message || /失败|error/i.test(message)) {
      return;
    }
    const timer = window.setTimeout(() => setMessage(""), 2400);
    return () => window.clearTimeout(timer);
  }, [message]);

  const updateDraftMeta = (updater: SetStateAction<LibraryMeta>) => {
    setDraftMeta((prev) => {
      const next = typeof updater === "function" ? updater(prev) : updater;
      draftMetaRef.current = next;
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

  // resetDetailState / applyRefreshedItems 统一 "列表刷新后重新对齐详情" 的
  // 两条路径 (手动重扫 / 删除后回列表). 原来写了两遍几乎一模一样的 block,
  // 现在抽成本地 helper, 只保留 excludePath 这一个差异参数.
  const resetDetailState = (msg: string) => {
    setDetail(null);
    detailRef.current = null;
    updateDraftMeta(cloneMeta(null));
    setSelectedPath("");
    setSelectedVariantKey("");
    lastSavedPathRef.current = "";
    lastSavedMetaRef.current = "";
    setMessage(msg);
  };

  const applyRefreshedItems = async (nextItems: LibraryListItem[], emptyMsg: string, excludePath?: string) => {
    setItems(nextItems);
    if (nextItems.length === 0) {
      resetDetailState(emptyMsg);
      return;
    }
    const keep = nextItems.some((item) => item.rel_path === selectedPath && item.rel_path !== excludePath);
    const nextSelected = keep ? selectedPath : nextItems[0].rel_path;
    const nextDetail = await getLibraryItem(nextSelected);
    syncDetail(nextDetail);
  };

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

  // 首次渲染若已经有 items 但没 detail (比如 SSR 只拿了列表), 派发一次
  // loadDetail 把 detail 补上. 用 useEffectEvent 包装避免 loadDetail 每次
  // 渲染新引用带来的 deps 噪音.
  const triggerInitialLoad = useEffectEvent((path: string) => {
    loadDetail(path);
  });

  useEffect(() => {
    if (!detail && items.length > 0 && selectedPath) {
      triggerInitialLoad(selectedPath);
    }
  }, [detail, items.length, selectedPath]);

  const refreshLibrary = async () => {
    const nextItems = await listLibraryItems();
    await applyRefreshedItems(nextItems, "当前 savedir 里还没有已入库内容");
  };

  const {
    moveState,
    mediaSyncRunning,
    configured,
    moveBusy,
    moveProgressVisible,
    moveProgress,
    refreshBusy,
    refreshButtonLabel,
    moveButtonLabel,
    handleRefreshLibrary,
    handleMoveToMediaLibrary,
  } = useLibraryMoveRefresh({
    initialMediaStatus,
    refreshLibrary,
    setMessage,
    startTransition,
  });

  const handleBlurSave = () => {
    startTransition(async () => {
      await persistMeta(draftMetaRef.current, "已自动保存", { silent: true });
    });
  };

  const {
    uploadActiveRef,
    resolveImage: resolveLibraryImageSrc,
    openUploadPicker,
    handleDeleteFanart,
    openCropper,
    handleConfirmCrop,
  } = useLibraryAssetActions({
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
  });

  const handleDeleteLibraryItem = () => {
    if (!detail) return;
    const targetPath = detail.item.rel_path;
    startTransition(async () => {
      try {
        setMessage("删除已入库目录...");
        await deleteLibraryItem(targetPath);
        const nextItems = await listLibraryItems();
        await applyRefreshedItems(nextItems, "已入库目录已删除", targetPath);
        setMessage("已入库目录已删除");
      } catch (error) {
        setMessage(toErrorMessage(error, "删除已入库目录失败"));
      }
    });
  };

  return (
    <div className="library-shell">
      <LibraryListPanel
        items={filteredItems}
        keyword={keyword}
        onKeywordChange={setKeyword}
        selectedPath={selectedPath}
        onSelectItem={loadDetail}
        resolveImage={resolveLibraryImageSrc}
        bottomActions={
          <LibraryBottomActions
            refreshBusy={refreshBusy}
            moveBusy={moveBusy}
            mediaSyncRunning={mediaSyncRunning}
            configured={configured}
            refreshButtonLabel={refreshButtonLabel}
            moveButtonLabel={moveButtonLabel}
            moveProgressVisible={moveProgressVisible}
            moveState={moveState}
            moveProgress={moveProgress}
            onRefresh={handleRefreshLibrary}
            onMove={handleMoveToMediaLibrary}
          />
        }
      />

      <section className="panel library-detail-panel">
        {detail ? (
          <>
            <LibraryDetailHeader
              subtitle={detail.item.rel_path}
              copyMode={copyMode}
              onCopyModeChange={setCopyMode}
              conflict={detail.item.conflict}
              isPending={isPending}
              onDelete={handleDeleteLibraryItem}
            />

            {showVariantSwitch ? (
              <LibraryVariantSwitcher
                variants={detail.variants}
                currentKey={currentVariant?.key ?? ""}
                onSelect={setSelectedVariantKey}
              />
            ) : null}

            <div className="review-content review-content-single">
              <div className="review-form library-detail-form">
                <div className="review-main-layout library-main-layout">
                  <LibraryFormFields
                    draftMeta={draftMeta}
                    copyMode={copyMode}
                    activeTitleValue={activeTitleValue}
                    activePlotValue={activePlotValue}
                    updateDraftMeta={updateDraftMeta}
                    onBlurSave={handleBlurSave}
                  />
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
                  <LibraryPosterCard
                    selectedPoster={selectedPoster}
                    selectedCover={selectedCover}
                    isPending={isPending}
                    uploadActiveRef={uploadActiveRef}
                    resolveImage={resolveLibraryImageSrc}
                    onOpenCropper={openCropper}
                    onOpenUploadPicker={() => openUploadPicker("poster")}
                    onOpenPreview={setPreview}
                  />
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

                <LibraryCoverCard
                  selectedCover={selectedCover}
                  isPending={isPending}
                  uploadActiveRef={uploadActiveRef}
                  resolveImage={resolveLibraryImageSrc}
                  onOpenUploadPicker={() => openUploadPicker("cover")}
                  onOpenPreview={setPreview}
                />

                <LibraryFanartStrip
                  fanartFiles={fanartFiles}
                  isPending={isPending}
                  uploadActiveRef={uploadActiveRef}
                  resolveImage={resolveLibraryImageSrc}
                  onOpenUploadPicker={() => openUploadPicker("fanart")}
                  onDeleteFanart={handleDeleteFanart}
                  onOpenPreview={setPreview}
                />
              </div>
            </div>
          </>
        ) : (
          <div className="review-empty-state">当前没有可查看的已入库目录</div>
        )}
      </section>
      <LibraryPreviewOverlay preview={preview} resolveImage={resolveLibraryImageSrc} onClose={() => setPreview(null)} />
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
