"use client";

import { Check, Crop, Plus, RotateCcw, Trash2, X } from "lucide-react";
import Image from "next/image";
import { useEffect, useEffectEvent, useRef, useState, useTransition } from "react";

import { ImageCropper } from "@/components/image-cropper";
import { Button } from "@/components/ui/button";
import type { JobItem, MediaFileRef, MediaLibraryStatus, ReviewMeta, ScrapeDataItem } from "@/lib/api";
import { cropPosterFromCover, deleteJob, getAssetURL, getMediaLibraryStatus, getReviewJob, importReviewJob, saveReviewJob, uploadAsset } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

interface Props {
  jobs: JobItem[];
  initialScrapeData: ScrapeDataItem | null;
  initialMediaStatus: MediaLibraryStatus | null;
}

const THUMB_IMAGE_STYLE = { objectFit: "cover", objectPosition: "center" } as const;
const PREVIEW_IMAGE_STYLE = { objectFit: "contain", objectPosition: "center" } as const;

function parseMeta(data: ScrapeDataItem | null): ReviewMeta | null {
  if (!data) {
    return null;
  }
  const raw = data.review_data || data.raw_data;
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as ReviewMeta;
  } catch {
    return null;
  }
}

function normalizeList(items?: string[]) {
  return (items ?? []).map((item) => item.trim()).filter(Boolean);
}

function buildPayload(meta: ReviewMeta | null) {
  if (!meta) {
    return "";
  }
  return JSON.stringify(
    {
      ...meta,
      actors: normalizeList(meta.actors),
      genres: normalizeList(meta.genres),
    },
    null,
    2,
  );
}

function imageTitle(type: string) {
  if (type === "cover") {
    return "封面";
  }
  if (type === "poster") {
    return "海报";
  }
  return "Extrafanart";
}

function TokenEditor({
  label,
  placeholder,
  value,
  onChange,
  onBlurSave,
  singleLine = false,
}: {
  label: string;
  placeholder: string;
  value: string[];
  onChange: (next: string[]) => void;
  onBlurSave: () => void;
  singleLine?: boolean;
}) {
  const [draft, setDraft] = useState("");

  const commitDraft = () => {
    const next = draft.trim();
    if (!next) {
      setDraft("");
      return;
    }
    onChange([...value, next]);
    setDraft("");
  };

  const removeAt = (idx: number) => {
    onChange(value.filter((_, index) => index !== idx));
    onBlurSave();
  };

  return (
    <div className="review-field review-field-tokens">
      <span className="review-label review-label-side">{label}</span>
      <div className={`token-editor${singleLine ? " token-editor-single-line" : ""}`} onClick={() => document.getElementById(`token-${label}`)?.focus()}>
        {value.map((item, idx) => (
          <span key={`${item}-${idx}`} className="token-chip">
            {item}
            <button
              type="button"
              className="token-chip-remove"
              aria-label={`删除${item}`}
              onClick={() => removeAt(idx)}
            >
              <X size={11} />
            </button>
          </span>
        ))}
        <input
          id={`token-${label}`}
          className="token-input"
          placeholder={value.length === 0 ? placeholder : ""}
          value={draft}
          onChange={(e) => {
            const next = e.target.value;
            if (next.includes(",")) {
              const parts = next.split(",");
              const ready = parts.slice(0, -1).map((item) => item.trim()).filter(Boolean);
              if (ready.length > 0) {
                onChange([...value, ...ready]);
              }
              setDraft(parts[parts.length - 1] ?? "");
              return;
            }
            setDraft(next);
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              commitDraft();
              onBlurSave();
            } else if (e.key === "Backspace" && draft === "" && value.length > 0) {
              onChange(value.slice(0, -1));
            }
          }}
          onBlur={() => {
            commitDraft();
            onBlurSave();
          }}
        />
      </div>
    </div>
  );
}

function DeleteConfirmOverlay({
  targetIds,
  selectedRelPath,
  onCancel,
  onConfirm,
  isPending,
}: {
  targetIds: number[] | null;
  selectedRelPath: string | undefined;
  onCancel: () => void;
  onConfirm: () => void;
  isPending: boolean;
}) {
  if (!targetIds || targetIds.length === 0) return null;
  return (
    <div className="review-preview-overlay" onClick={onCancel}>
      <div className="panel review-confirm-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="review-confirm-title">确认删除</div>
        <div className="review-confirm-body">
          {targetIds.length > 1 ? (
            <>
              这会删除已选中的 {targetIds.length} 个任务以及各自对应的源文件。
            </>
          ) : (
            <>
              这会删除当前任务以及对应的源文件。
              <br />
              <span className="review-confirm-path">{selectedRelPath}</span>
            </>
          )}
        </div>
        <div className="review-confirm-actions">
          <Button onClick={onCancel}>取消</Button>
          <Button variant="primary" onClick={onConfirm} disabled={isPending}>
            删除
          </Button>
        </div>
      </div>
    </div>
  );
}

function RestoreConfirmOverlay({
  open,
  selectedRelPath,
  onCancel,
  onConfirm,
  isPending,
}: {
  open: boolean;
  selectedRelPath: string | undefined;
  onCancel: () => void;
  onConfirm: () => void;
  isPending: boolean;
}) {
  if (!open) return null;
  return (
    <div className="review-preview-overlay" onClick={onCancel}>
      <div className="panel review-confirm-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="review-confirm-title">恢复原始内容</div>
        <div className="review-confirm-body">
          这会用最初刮削得到的原始内容覆盖当前修改。
          <br />
          <span className="review-confirm-path">{selectedRelPath}</span>
        </div>
        <div className="review-confirm-actions">
          <Button onClick={onCancel}>取消</Button>
          <Button variant="primary" onClick={onConfirm} disabled={isPending}>
            恢复
          </Button>
        </div>
      </div>
    </div>
  );
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
  const [preview, setPreview] = useState<{ title: string; item: MediaFileRef } | null>(null);
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
        <aside className="panel review-list-panel">
          <div className="review-list-head">
            <div>
              <div className="review-list-kicker">Review Queue</div>
              <h2 className="review-list-title">Review 列表</h2>
              <p className="review-list-subtitle">
                当前 {items.length} 条待复核任务
                {selectedIndex >= 0 ? `，正在查看第 ${selectedIndex + 1} 条` : ""}
              </p>
              {moveRunning ? <p className="review-list-subtitle">媒体库正在同步迁移，审批按钮已临时锁定。</p> : null}
            </div>
          </div>
          <div className="review-bulk-toolbar">
            <label className="review-bulk-select-all">
              <input
                ref={selectAllRef}
                type="checkbox"
                checked={allSelectableChecked}
                disabled={items.length === 0 || isPending || moveRunning}
                title="选择当前列表中的全部 review 任务"
                onChange={handleToggleSelectAll}
              />
              <span>全选</span>
            </label>
            <div className="review-bulk-toolbar-actions">
              {selectedCount > 0 ? <span className="review-bulk-count">已选 {selectedCount} 项</span> : null}
              <Button
                className="review-inline-icon-btn review-bulk-approve-btn"
                onClick={handleImportSelected}
                disabled={selectedCount === 0 || isPending || moveRunning}
                aria-label="批量审批"
                title={selectedCount > 0 ? `批量审批已选 ${selectedCount} 项` : "批量审批"}
              >
                <Check size={16} />
              </Button>
              <Button
                className="review-inline-icon-btn review-bulk-delete-btn"
                onClick={handleDeleteSelected}
                disabled={selectedCount === 0 || isPending}
                aria-label="批量删除"
                title={selectedCount > 0 ? `删除已选 ${selectedCount} 项` : "批量删除"}
              >
                <Trash2 size={14} />
              </Button>
            </div>
          </div>
          <div className="review-job-list">
            {items.length === 0 ? <div className="review-empty-state">当前没有待 review 的任务</div> : null}
            {items.map((job, index) => (
              <div
                key={job.id}
                className="panel review-job-card"
                data-active={selected?.id === job.id}
                data-selected={selectedJobIds.has(job.id)}
              >
                <div className="review-job-card-select">
                  <input
                    type="checkbox"
                    checked={selectedJobIds.has(job.id)}
                    disabled={isPending || moveRunning}
                    title={moveRunning ? "媒体库移动进行中，暂不可选择" : "选择任务"}
                    onChange={() => handleToggleSelectJob(job.id)}
                  />
                </div>
                <button className="review-job-card-main" onClick={() => loadDetail(job)} disabled={isPending}>
                  <div className="review-job-card-topline">
                    <span className="review-job-card-index">#{index + 1}</span>
                    <span className="review-job-card-time">更新于 {formatUnixMillis(job.updated_at)}</span>
                  </div>
                  <div className="review-job-card-path">{job.rel_path}</div>
                  <div className="review-job-card-number">{job.number}</div>
                </button>
                <div className="review-job-card-actions">
                  <Button
                    className="review-inline-icon-btn review-action-approve"
                    onClick={handleImport}
                    disabled={isPending || selected?.id !== job.id || moveRunning}
                    aria-label="入库"
                    title={moveRunning ? "媒体库移动进行中，暂不可审批" : "入库"}
                  >
                    <Check size={16} />
                  </Button>
                  <Button
                    className="review-inline-icon-btn"
                    onClick={handleDelete}
                    disabled={isPending || selected?.id !== job.id}
                    aria-label="删除"
                    title="删除"
                  >
                    <Trash2 size={14} />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </aside>
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
                  <div className="review-top-fields">
                    <div className="review-field">
                      <span className="review-label review-label-side">标题</span>
                      <input
                        className="input review-input-strong"
                        value={meta.title ?? ""}
                        onChange={(e) => updateMeta({ title: e.target.value })}
                        onBlur={handleBlurSave}
                      />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">翻译标题</span>
                      <input
                        className="input"
                        value={meta.title_translated ?? ""}
                        onChange={(e) => updateMeta({ title_translated: e.target.value })}
                        onBlur={handleBlurSave}
                      />
                    </div>
                    <div className="review-meta-row review-meta-row-2 review-meta-row-top">
                      <div className="review-field">
                        <span className="review-label review-label-side">导演</span>
                        <input className="input" value={meta.director ?? ""} onChange={(e) => updateMeta({ director: e.target.value })} onBlur={handleBlurSave} />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">制作商</span>
                        <input className="input" value={meta.studio ?? ""} onChange={(e) => updateMeta({ studio: e.target.value })} onBlur={handleBlurSave} />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">发行商</span>
                        <input className="input" value={meta.label ?? ""} onChange={(e) => updateMeta({ label: e.target.value })} onBlur={handleBlurSave} />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">系列</span>
                        <input className="input" value={meta.series ?? ""} onChange={(e) => updateMeta({ series: e.target.value })} onBlur={handleBlurSave} />
                      </div>
                    </div>
                    <div className="review-meta-row review-meta-row-2">
                      <div className="review-field review-field-area">
                        <span className="review-label review-label-side">简介</span>
                        <textarea className="input review-textarea" value={meta.plot ?? ""} onChange={(e) => updateMeta({ plot: e.target.value })} onBlur={handleBlurSave} />
                      </div>
                      <div className="review-field review-field-area">
                        <span className="review-label review-label-side">翻译简介</span>
                        <textarea
                          className="input review-textarea"
                          value={meta.plot_translated ?? ""}
                          onChange={(e) => updateMeta({ plot_translated: e.target.value })}
                          onBlur={handleBlurSave}
                        />
                      </div>
                    </div>
                  </div>
                  <div className="review-main-side">
                    <div className="review-meta-row">
                      <TokenEditor
                        label="演员"
                        placeholder="输入演员名后输入逗号"
                        value={normalizeList(meta.actors)}
                        onChange={(next) => updateMeta({ actors: next })}
                        onBlurSave={handleBlurSave}
                      />
                    </div>
                  </div>
                  {meta.poster ? (
                    <div className="panel review-image-card review-image-card-poster review-top-poster review-main-poster">
                      <span className="review-image-title">海报</span>
                      <Button className="review-inline-icon-btn review-image-crop-btn" onClick={openCropper} aria-label="从封面截取海报" title="从封面截取海报">
                        <Crop size={14} />
                      </Button>
                      <div className="review-image-box review-image-box-poster">
                        <button type="button" className="review-image-hit" onClick={() => { if (!uploadActiveRef.current && meta.poster) setPreview({ title: imageTitle("poster"), item: meta.poster }); }}>
                          <Image src={getAssetURL(meta.poster.key)} alt="poster" fill style={THUMB_IMAGE_STYLE} unoptimized />
                        </button>
                        <button
                          type="button"
                          className="review-upload-overlay"
                          onClick={() => openUploadPicker("poster")}
                          aria-label="上传海报"
                          title="上传海报"
                        >
                          <Plus size={18} />
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="panel review-image-card review-image-card-poster review-top-poster review-main-poster review-image-empty">
                      <span className="review-image-title">海报</span>
                      <Button className="review-inline-icon-btn review-image-crop-btn" onClick={openCropper} aria-label="从封面截取海报" title="从封面截取海报">
                        <Crop size={14} />
                      </Button>
                      <div className="review-image-box review-image-box-poster review-upload-empty">
                        <button
                          type="button"
                          className="review-upload-overlay"
                          onClick={() => openUploadPicker("poster")}
                          aria-label="上传海报"
                          title="上传海报"
                        >
                          <Plus size={18} />
                        </button>
                      </div>
                    </div>
                  )}
                </div>
                <div className="review-meta-row review-meta-row-full">
                  <TokenEditor
                    label="标签"
                    placeholder="输入标签后输入逗号"
                    value={normalizeList(meta.genres)}
                    onChange={(next) => updateMeta({ genres: next })}
                    onBlurSave={handleBlurSave}
                    singleLine
                  />
                </div>
                <div className="review-media-offset review-cover-slot">
                  {meta.cover ? (
                    <div className="panel review-image-card review-image-card-cover">
                      <span className="review-image-title">封面</span>
                      <div className="review-image-box review-image-box-cover">
                        <button type="button" className="review-image-hit" onClick={() => { if (!uploadActiveRef.current && meta.cover) setPreview({ title: imageTitle("cover"), item: meta.cover }); }}>
                          <Image src={getAssetURL(meta.cover.key)} alt="cover" fill style={THUMB_IMAGE_STYLE} unoptimized />
                        </button>
                        <button
                          type="button"
                          className="review-upload-overlay"
                          onClick={(e) => {
                            e.stopPropagation();
                            openUploadPicker("cover");
                          }}
                          aria-label="上传封面"
                          title="上传封面"
                        >
                          <Plus size={18} />
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="panel review-image-card review-image-card-cover review-image-empty">
                      <div className="review-image-box review-image-box-cover review-upload-empty">
                        <button
                          type="button"
                          className="review-upload-overlay"
                          onClick={() => openUploadPicker("cover")}
                          aria-label="上传封面"
                          title="上传封面"
                        >
                          <Plus size={18} />
                        </button>
                      </div>
                    </div>
                  )}
                </div>
                <div className="review-media-offset review-fanart-slot">
                  <div className="panel review-fanart-panel">
                    <div
                      className="review-fanart-strip"
                      onWheel={(e) => {
                        if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) {
                          return;
                        }
                        e.currentTarget.scrollLeft += e.deltaY;
                        e.preventDefault();
                      }}
                    >
                      {(meta.sample_images ?? []).map((item) => (
                        <div key={item.key} className="review-fanart-item">
                          <button type="button" className="review-image-hit" onClick={() => { if (!uploadActiveRef.current) setPreview({ title: imageTitle("fanart"), item }); }}>
                            <Image src={getAssetURL(item.key)} alt={item.name} fill style={THUMB_IMAGE_STYLE} unoptimized />
                          </button>
                          <Button
                            className="review-inline-icon-btn review-fanart-delete"
                            onClick={() => handleRemoveFanart(item.key)}
                            aria-label="删除 fanart"
                            title="删除 fanart"
                          >
                            <X size={12} />
                          </Button>
                        </div>
                      ))}
                      <button type="button" className="review-fanart-item review-upload-empty" onClick={() => openUploadPicker("fanart")}>
                        <span className="review-upload-overlay review-upload-overlay-static" aria-hidden="true">
                          <Plus size={18} />
                        </span>
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div style={{ color: "var(--muted)" }}>选择左侧任务后在这里展示刮削结果</div>
          )}
        </section>
      </div>
      {preview ? (
        <div className="review-preview-overlay" onClick={() => setPreview(null)}>
          <button type="button" className="review-preview-close" aria-label="关闭预览" onClick={() => setPreview(null)}>
            <X size={18} />
          </button>
          <div className="review-preview-dialog panel" onClick={(e) => e.stopPropagation()}>
            <div className="review-preview-title">{preview.title}</div>
            <div className="review-preview-frame">
              <Image src={getAssetURL(preview.item.key)} alt={preview.item.name} fill style={PREVIEW_IMAGE_STYLE} unoptimized />
            </div>
          </div>
        </div>
      ) : null}

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
