"use client";

import dynamic from "next/dynamic";
import { useEffect, useEffectEvent, useRef, useState, useTransition } from "react";

// ImageCropper 只在用户点 "裁剪封面" 按钮 (或 review 页特定裁图流程) 后才
// 真的挂上. 初次打开 /review 路由时 200+ 行的 cropper 模块 + 指针/canvas
// 交互代码都不该进首屏 JS chunk. ssr: false 保证不给没交互的用户 SSR 这
// 个组件. 详见 td/022-frontend-optimization-roadmap.md §5.2.
const ImageCropper = dynamic(
  () => import("@/components/image-cropper").then((m) => m.ImageCropper),
  { ssr: false },
);

import { ReviewCoverCard, ReviewFanartStrip, ReviewPosterCard } from "@/components/review-shell/asset-gallery";
import { DeleteConfirmOverlay } from "@/components/review-shell/delete-confirm-overlay";
import { ReviewDetailHeader } from "@/components/review-shell/detail-header";
import { ReviewFormFields } from "@/components/review-shell/form-fields";
import { ReviewListPanel } from "@/components/review-shell/list-panel";
import { ReviewPreviewOverlay, type ReviewPreviewState } from "@/components/review-shell/preview-overlay";
import { RestoreConfirmOverlay } from "@/components/review-shell/restore-confirm-overlay";
import { useReviewAssetActions } from "@/components/review-shell/use-review-asset-actions";
import { useReviewBatchActions } from "@/components/review-shell/use-review-batch-actions";
import { buildPayload, normalizeList, parseMeta, parseRawMeta } from "@/components/review-shell/utils";
import { TokenEditor } from "@/components/ui/token-editor";
import type { JobItem, MediaLibraryStatus, ReviewMeta, ScrapeDataItem } from "@/lib/api";
import { getAssetURL, getMediaLibraryStatus, getReviewJob, saveReviewJob } from "@/lib/api";

interface Props {
  jobs: JobItem[];
  initialScrapeData: ScrapeDataItem | null;
  initialMediaStatus: MediaLibraryStatus | null;
}

export function ReviewShell({ jobs, initialScrapeData, initialMediaStatus }: Props) {
  const initialMeta = parseMeta(initialScrapeData);
  const initialRawMeta = parseRawMeta(initialScrapeData);
  const [items, setItems] = useState<JobItem[]>(jobs);
  const [selected, setSelected] = useState<JobItem | null>(jobs[0] ?? null);
  const [meta, setMeta] = useState<ReviewMeta | null>(initialMeta);
  const [hasRawMeta, setHasRawMeta] = useState(initialRawMeta !== null);
  const [message, setMessage] = useState<string>(jobs.length === 0 ? "当前没有待 review 的任务" : "");
  const [preview, setPreview] = useState<ReviewPreviewState>(null);
  const [mediaStatus, setMediaStatus] = useState<MediaLibraryStatus | null>(initialMediaStatus);
  const [selectedJobIds, setSelectedJobIds] = useState<Set<number>>(new Set());
  const [cropOpen, setCropOpen] = useState(false);
  const [deleteTargetIds, setDeleteTargetIds] = useState<number[] | null>(null);
  const [restoreConfirmOpen, setRestoreConfirmOpen] = useState(false);
  const [isPending, startTransition] = useTransition();
  const lastSavedPayloadRef = useRef(buildPayload(initialMeta));
  const lastSavedJobIDRef = useRef<number | null>(jobs[0]?.id ?? null);
  const saveQueueRef = useRef<Promise<boolean>>(Promise.resolve(true));
  const selectedRef = useRef<JobItem | null>(jobs[0] ?? null);
  const metaRef = useRef<ReviewMeta | null>(initialMeta);
  const rawMetaRef = useRef<ReviewMeta | null>(initialRawMeta);
  const selectAllRef = useRef<HTMLInputElement | null>(null);

  const messageTone = /失败|error|删除|failed/i.test(message) ? "danger" : "info";
  const selectedIndex = selected ? items.findIndex((item) => item.id === selected.id) : -1;
  const moveRunning = mediaStatus?.move.status === "running";
  const syncRunning = mediaStatus?.sync.status === "running";
  const shouldPollMediaStatus = moveRunning || syncRunning;
  const selectableJobs = items;
  const selectedCount = selectedJobIds.size;
  const allSelectableChecked = selectableJobs.length > 0 && selectableJobs.every((job) => selectedJobIds.has(job.id));
  const hasPartialSelection = selectedCount > 0 && !allSelectableChecked;

  const refreshMediaStatus = useEffectEvent(async () => {
    try {
      const next = await getMediaLibraryStatus();
      setMediaStatus(next);
    } catch {
      // ignore polling errors
    }
  });

  useEffect(() => {
    if (!shouldPollMediaStatus) {
      return;
    }
    void refreshMediaStatus();
    const timer = window.setInterval(() => {
      void refreshMediaStatus();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [shouldPollMediaStatus]);

  useEffect(() => {
    if (!selectAllRef.current) {
      return;
    }
    selectAllRef.current.indeterminate = hasPartialSelection;
  }, [hasPartialSelection]);

  const syncStateWithData = (data: ScrapeDataItem | null) => {
    const nextMeta = parseMeta(data);
    const nextRawMeta = parseRawMeta(data);
    const payload = buildPayload(nextMeta);
    setMeta(nextMeta);
    metaRef.current = nextMeta;
    rawMetaRef.current = nextRawMeta;
    setHasRawMeta(nextRawMeta !== null);
    lastSavedPayloadRef.current = payload;
    lastSavedJobIDRef.current = selectedRef.current?.id ?? null;
    setMessage(data ? "" : "该任务还没有 scrape_data");
  };

  const loadDetail = (job: JobItem) => {
    setSelected(job);
    selectedRef.current = job;
    startTransition(async () => {
      try {
        setMessage("加载刮削结果...");
        const data = await getReviewJob(job.id);
        syncStateWithData(data);
      } catch (error) {
        setMeta(null);
        metaRef.current = null;
        rawMetaRef.current = null;
        setHasRawMeta(false);
        lastSavedPayloadRef.current = "";
        lastSavedJobIDRef.current = null;
        setMessage(error instanceof Error ? error.message : "加载失败");
      }
    });
  };

  const removeJobsFromList = (jobIDs: number[]) => {
    const removedIDs = new Set(jobIDs);
    const next = items.filter((item) => !removedIDs.has(item.id));
    setItems(next);
    setSelectedJobIds((prev) => {
      const nextSelectedIDs = new Set(Array.from(prev).filter((id) => !removedIDs.has(id)));
      return nextSelectedIDs;
    });
    const currentSelected = selectedRef.current;
    if (currentSelected && next.some((item) => item.id === currentSelected.id)) {
      return;
    }
    const nextSelected = next.at(0) ?? null;
    if (!nextSelected) {
      setSelected(null);
      selectedRef.current = null;
      setMeta(null);
      metaRef.current = null;
      rawMetaRef.current = null;
      setHasRawMeta(false);
      lastSavedPayloadRef.current = "";
      lastSavedJobIDRef.current = null;
      return;
    }
    loadDetail(nextSelected);
  };

  const removeJobFromList = (jobID: number) => {
    removeJobsFromList([jobID]);
  };

  const updateMeta = (patch: Partial<ReviewMeta>) => {
    setMeta((prev) => {
      const next = { ...(prev ?? {}), ...patch };
      metaRef.current = next;
      return next;
    });
  };

  const persistReview = async (options?: { silent?: boolean; successText?: string }) => {
    const selectedJob = selectedRef.current;
    const currentMeta = metaRef.current;
    if (!selectedJob || !currentMeta) {
      return true;
    }
    const jobID = selectedJob.id;
    const payload = buildPayload(currentMeta);
    if (jobID === lastSavedJobIDRef.current && payload === lastSavedPayloadRef.current) {
      return true;
    }
    const task = saveQueueRef.current.then(async () => {
      if (jobID === lastSavedJobIDRef.current && payload === lastSavedPayloadRef.current) {
        return true;
      }
      try {
        if (!options?.silent) {
          setMessage("保存 review 数据...");
        }
        await saveReviewJob(jobID, payload);
        lastSavedPayloadRef.current = payload;
        lastSavedJobIDRef.current = jobID;
        setMessage(options?.successText ?? "已自动保存");
        return true;
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "保存失败");
        return false;
      }
    });
    saveQueueRef.current = task.catch(() => true);
    return task;
  };

  const handleBlurSave = () => {
    startTransition(async () => {
      await persistReview({ silent: true });
    });
  };

  const handleToggleSelectAll = () => {
    setSelectedJobIds((prev) => {
      if (selectableJobs.length === 0) {
        return prev;
      }
      if (selectableJobs.every((job) => prev.has(job.id))) {
        return new Set<number>();
      }
      return new Set(selectableJobs.map((job) => job.id));
    });
  };

  const handleToggleSelectJob = (jobID: number) => {
    setSelectedJobIds((prev) => {
      const next = new Set(prev);
      if (next.has(jobID)) {
        next.delete(jobID);
      } else {
        next.add(jobID);
      }
      return next;
    });
  };

  const handleRestoreRaw = () => {
    if (!selected || !rawMetaRef.current) {
      return;
    }
    setRestoreConfirmOpen(true);
  };

  const confirmRestoreRaw = () => {
    if (!selected || !rawMetaRef.current) {
      return;
    }
    const restored = JSON.parse(JSON.stringify(rawMetaRef.current)) as ReviewMeta;
    setRestoreConfirmOpen(false);
    setMeta(restored);
    metaRef.current = restored;
    startTransition(async () => {
      try {
        setMessage("恢复原始刮削内容...");
        const payload = buildPayload(restored);
        await saveReviewJob(selected.id, payload);
        lastSavedPayloadRef.current = payload;
        setMessage("已恢复为原始刮削内容");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "恢复失败");
      }
    });
  };

  const {
    handleImport,
    handleImportSelected,
    handleDelete,
    handleDeleteSelected,
    confirmDelete,
  } = useReviewBatchActions({
    selected,
    meta,
    moveRunning,
    selectedJobIds,
    deleteTargetIds,
    setDeleteTargetIds,
    setMessage,
    startTransition,
    persistReview,
    removeJobFromList,
    removeJobsFromList,
  });

  const {
    uploadActiveRef,
    openCropper,
    handleCropResult,
    handleRemoveFanart,
    openUploadPicker,
  } = useReviewAssetActions({
    selected,
    meta,
    metaRef,
    lastSavedPayloadRef,
    setMeta,
    updateMeta,
    setMessage,
    setCropOpen,
    startTransition,
  });

  return (
    <>
      <div className="two-col">
        <ReviewListPanel
          items={items}
          selectedId={selected?.id}
          selectedIndex={selectedIndex}
          selectedJobIds={selectedJobIds}
          selectedCount={selectedCount}
          allSelectableChecked={allSelectableChecked}
          isPending={isPending}
          moveRunning={moveRunning}
          selectAllRef={selectAllRef}
          onToggleSelectAll={handleToggleSelectAll}
          onToggleSelectJob={handleToggleSelectJob}
          onLoadDetail={loadDetail}
          onImportSelected={handleImportSelected}
          onDeleteSelected={handleDeleteSelected}
          onImport={handleImport}
          onDelete={handleDelete}
        />
        <section className="panel review-detail-panel">
          <ReviewDetailHeader
            selected={selected}
            message={message}
            messageTone={messageTone}
            hasRawMeta={hasRawMeta}
            isPending={isPending}
            onRestoreRaw={handleRestoreRaw}
          />
          {meta ? (
            <div className="review-content review-content-single">
              <div className="review-form">
                <div className="review-main-layout">
                  <ReviewFormFields meta={meta} updateMeta={updateMeta} onBlurSave={handleBlurSave} />
                  <div className="review-main-side">
                    <div className="review-meta-row">
                      <TokenEditor
                        idPrefix="token"
                        label="演员"
                        placeholder="输入演员名后输入逗号"
                        value={normalizeList(meta.actors)}
                        onChange={(next) => updateMeta({ actors: next })}
                        onCommit={handleBlurSave}
                      />
                    </div>
                  </div>
                  <ReviewPosterCard
                    poster={meta.poster}
                    uploadActiveRef={uploadActiveRef}
                    onOpenCropper={openCropper}
                    onOpenUploadPicker={() => openUploadPicker("poster")}
                    onOpenPreview={setPreview}
                  />
                </div>
                <div className="review-meta-row review-meta-row-full">
                  <TokenEditor
                    idPrefix="token"
                    label="标签"
                    placeholder="输入标签后输入逗号"
                    value={normalizeList(meta.genres)}
                    onChange={(next) => updateMeta({ genres: next })}
                    onCommit={handleBlurSave}
                    singleLine
                  />
                </div>
                <ReviewCoverCard
                  cover={meta.cover}
                  uploadActiveRef={uploadActiveRef}
                  onOpenUploadPicker={() => openUploadPicker("cover")}
                  onOpenPreview={setPreview}
                />
                <ReviewFanartStrip
                  sampleImages={meta.sample_images}
                  uploadActiveRef={uploadActiveRef}
                  onOpenUploadPicker={() => openUploadPicker("fanart")}
                  onRemoveFanart={handleRemoveFanart}
                  onOpenPreview={setPreview}
                />
              </div>
            </div>
          ) : (
            <div style={{ color: "var(--muted)" }}>选择左侧任务后在这里展示刮削结果</div>
          )}
        </section>
      </div>
      <ReviewPreviewOverlay preview={preview} onClose={() => setPreview(null)} />

      {cropOpen && meta?.cover ? (
        <ImageCropper
          open
          imageSrc={getAssetURL(meta.cover.key)}
          onClose={() => setCropOpen(false)}
          onConfirm={handleCropResult}
        />
      ) : null}

      <DeleteConfirmOverlay
        targetIds={deleteTargetIds}
        selectedRelPath={selected?.rel_path}
        onCancel={() => setDeleteTargetIds(null)}
        onConfirm={confirmDelete}
        isPending={isPending}
      />
      <RestoreConfirmOverlay
        open={restoreConfirmOpen}
        selectedRelPath={selected?.rel_path}
        onCancel={() => setRestoreConfirmOpen(false)}
        onConfirm={confirmRestoreRaw}
        isPending={isPending}
      />
    </>
  );
}
