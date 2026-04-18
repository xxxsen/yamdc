"use client";

import { RotateCcw } from "lucide-react";
import { useEffect, useEffectEvent, useRef, useState, useTransition } from "react";

import { ImageCropper } from "@/components/image-cropper";
import { ReviewCoverCard, ReviewFanartStrip, ReviewPosterCard } from "@/components/review-shell/asset-gallery";
import { DeleteConfirmOverlay } from "@/components/review-shell/delete-confirm-overlay";
import { ReviewFormFields } from "@/components/review-shell/form-fields";
import { ReviewListPanel } from "@/components/review-shell/list-panel";
import { ReviewPreviewOverlay, type ReviewPreviewState } from "@/components/review-shell/preview-overlay";
import { RestoreConfirmOverlay } from "@/components/review-shell/restore-confirm-overlay";
import { buildPayload, normalizeList, parseMeta } from "@/components/review-shell/utils";
import { Button } from "@/components/ui/button";
import { TokenEditor } from "@/components/ui/token-editor";
import type { JobItem, MediaLibraryStatus, ReviewMeta, ScrapeDataItem } from "@/lib/api";
import { cropPosterFromCover, deleteJob, getAssetURL, getMediaLibraryStatus, getReviewJob, importReviewJob, saveReviewJob, uploadAsset } from "@/lib/api";

interface Props {
  jobs: JobItem[];
  initialScrapeData: ScrapeDataItem | null;
  initialMediaStatus: MediaLibraryStatus | null;
}

export function ReviewShell({ jobs, initialScrapeData, initialMediaStatus }: Props) {
  const initialMeta = parseMeta(initialScrapeData);
  const initialRawMeta = initialScrapeData?.raw_data ? (() => {
    try {
      return JSON.parse(initialScrapeData.raw_data) as ReviewMeta;
    } catch {
      return null;
    }
  })() : null;
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
  const uploadActiveRef = useRef(false);
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
    let nextRawMeta: ReviewMeta | null = null;
    if (data?.raw_data) {
      try {
        nextRawMeta = JSON.parse(data.raw_data) as ReviewMeta;
      } catch {
        nextRawMeta = null;
      }
    }
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

  const handleImport = () => {
    if (!selected || !meta) {
      return;
    }
    if (moveRunning) {
      setMessage("媒体库移动进行中，暂不可审批入库");
      return;
    }
    startTransition(async () => {
      const ok = await persistReview({ silent: true });
      if (!ok) {
        return;
      }
      try {
        setMessage("执行入库...");
        await importReviewJob(selected.id);
        removeJobFromList(selected.id);
        setMessage("入库完成，任务已移出 review 列表");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "入库失败");
      }
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

  const handleImportSelected = () => {
    if (selectedCount === 0) {
      return;
    }
    if (moveRunning) {
      setMessage("媒体库移动进行中，暂不可批量审批入库");
      return;
    }
    startTransition(async () => {
      const targetIDs = Array.from(selectedJobIds);
      if (targetIDs.length === 0) {
        return;
      }
      if (selected && meta && selectedJobIds.has(selected.id)) {
        const ok = await persistReview({ silent: true });
        if (!ok) {
          return;
        }
      }
      const successIDs: number[] = [];
      const failures: string[] = [];
      for (let index = 0; index < targetIDs.length; index += 1) {
        const id = targetIDs[index];
        try {
          setMessage(`批量审批中 ${index + 1}/${targetIDs.length}...`);
          await importReviewJob(id);
          successIDs.push(id);
        } catch (error) {
          failures.push(error instanceof Error ? error.message : `任务 #${id} 入库失败`);
        }
      }
      if (successIDs.length > 0) {
        removeJobsFromList(successIDs);
      }
      if (failures.length === 0) {
        setMessage(`批量审批完成，已入库 ${successIDs.length} 项`);
        return;
      }
      if (successIDs.length === 0) {
        setMessage(failures[0] ?? "批量审批失败");
        return;
      }
      setMessage(`已入库 ${successIDs.length} 项，${failures.length} 项失败`);
    });
  };

  const handleDelete = () => {
    if (!selected) {
      return;
    }
    setDeleteTargetIds([selected.id]);
  };

  const handleDeleteSelected = () => {
    if (selectedCount === 0) {
      return;
    }
    setDeleteTargetIds(Array.from(selectedJobIds));
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

  const confirmDelete = () => {
    if (!deleteTargetIds || deleteTargetIds.length === 0) {
      return;
    }
    const targetIDs = deleteTargetIds;
    setDeleteTargetIds(null);
    startTransition(async () => {
      const successIDs: number[] = [];
      const failures: string[] = [];
      for (let index = 0; index < targetIDs.length; index += 1) {
        const id = targetIDs[index];
        try {
          setMessage(targetIDs.length > 1 ? `批量删除中 ${index + 1}/${targetIDs.length}...` : "删除任务...");
          await deleteJob(id);
          successIDs.push(id);
        } catch (error) {
          failures.push(error instanceof Error ? error.message : `任务 #${id} 删除失败`);
        }
      }
      if (successIDs.length > 0) {
        removeJobsFromList(successIDs);
      }
      if (failures.length === 0) {
        setMessage(targetIDs.length > 1 ? `已删除 ${successIDs.length} 项` : "任务已删除");
        return;
      }
      if (successIDs.length === 0) {
        setMessage(failures[0] ?? "删除失败");
        return;
      }
      setMessage(`已删除 ${successIDs.length} 项，${failures.length} 项失败`);
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
          <div className="review-header">
            <div>
              <div className="review-list-kicker">Review Editor</div>
              <h2 className="review-detail-title">Review 内容</h2>
              {selected ? <div className="review-subtitle">当前任务 #{selected.id} / {selected.rel_path}</div> : null}
            </div>
            <div className="review-actions">
              {message ? <span className="review-message" data-tone={messageTone}>{message}</span> : null}
              <Button
                className="review-inline-icon-btn"
                onClick={handleRestoreRaw}
                disabled={!selected || isPending || !hasRawMeta}
                aria-label="恢复原始刮削内容"
                title="恢复原始刮削内容"
              >
                <RotateCcw size={14} />
              </Button>
            </div>
          </div>
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
