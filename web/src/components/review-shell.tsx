"use client";

import { Check, Crop, RotateCcw, Trash2, X } from "lucide-react";
import Image from "next/image";
import { type PointerEvent as ReactPointerEvent, type SyntheticEvent, useRef, useState, useTransition } from "react";

import type { JobItem, MediaFileRef, ReviewMeta, ScrapeDataItem } from "@/lib/api";
import { cropPosterFromCover, deleteJob, getAssetURL, getReviewJob, importReviewJob, saveReviewJob } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

interface Props {
  jobs: JobItem[];
  initialScrapeData: ScrapeDataItem | null;
}

const THUMB_IMAGE_STYLE = { objectFit: "cover", objectPosition: "center" } as const;
const PREVIEW_IMAGE_STYLE = { objectFit: "contain", objectPosition: "center" } as const;
const POSTER_ASPECT = 2 / 3;

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
}: {
  label: string;
  placeholder: string;
  value: string[];
  onChange: (next: string[]) => void;
  onBlurSave: () => void;
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
      <div className="token-editor" onClick={() => document.getElementById(`token-${label}`)?.focus()}>
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

export function ReviewShell({ jobs, initialScrapeData }: Props) {
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
  const [cropOpen, setCropOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [restoreConfirmOpen, setRestoreConfirmOpen] = useState(false);
  const [cropRect, setCropRect] = useState({ x: 0, y: 0, width: 0, height: 0 });
  const [cropImageSize, setCropImageSize] = useState({ displayWidth: 0, displayHeight: 0, naturalWidth: 0, naturalHeight: 0 });
  const [isPending, startTransition] = useTransition();
  const lastSavedPayloadRef = useRef(buildPayload(initialMeta));
  const selectedRef = useRef<JobItem | null>(jobs[0] ?? null);
  const metaRef = useRef<ReviewMeta | null>(initialMeta);
  const rawMetaRef = useRef<ReviewMeta | null>(initialRawMeta);
  const cropDragRef = useRef<{ startX: number; startY: number; originX: number; originY: number } | null>(null);

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
        setMessage(error instanceof Error ? error.message : "加载失败");
      }
    });
  };

  const removeJobFromList = (jobID: number) => {
    const next = items.filter((item) => item.id !== jobID);
    setItems(next);
    const nextSelected = next[0] ?? null;
    if (!nextSelected) {
      setSelected(null);
      selectedRef.current = null;
      setMeta(null);
      metaRef.current = null;
      rawMetaRef.current = null;
      setHasRawMeta(false);
      lastSavedPayloadRef.current = "";
      return;
    }
    loadDetail(nextSelected);
  };

  const updateMeta = (patch: Partial<ReviewMeta>) => {
    setMeta((prev) => {
      const next = { ...(prev ?? {}), ...patch };
      metaRef.current = next;
      return next;
    });
  };

  const persistReview = async (options?: { silent?: boolean }) => {
    if (!selectedRef.current || !metaRef.current) {
      return true;
    }
    const payload = buildPayload(metaRef.current);
    if (payload === lastSavedPayloadRef.current) {
      return true;
    }
    try {
      if (!options?.silent) {
        setMessage("保存 review 数据...");
      }
      await saveReviewJob(selectedRef.current.id, payload);
      lastSavedPayloadRef.current = payload;
      setMessage("已自动保存");
      return true;
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存失败");
      return false;
    }
  };

  const handleAutoSave = () => {
    startTransition(async () => {
      await persistReview();
    });
  };

  const handleImport = () => {
    if (!selected || !meta) {
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

  const handleDelete = () => {
    if (!selected) {
      return;
    }
    setDeleteConfirmOpen(true);
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
    if (!selected) {
      return;
    }
    setDeleteConfirmOpen(false);
    startTransition(async () => {
      try {
        setMessage("删除任务...");
        await deleteJob(selected.id);
        removeJobFromList(selected.id);
        setMessage("任务已删除");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "删除失败");
      }
    });
  };

  const openCropper = () => {
    if (!meta?.cover) {
      return;
    }
    setCropOpen(true);
  };

  const handleCropImageLoad = (event: SyntheticEvent<HTMLImageElement>) => {
    const img = event.currentTarget;
    const naturalWidth = img.naturalWidth;
    const naturalHeight = img.naturalHeight;
    const displayWidth = img.clientWidth;
    const displayHeight = img.clientHeight;
    let width = 0;
    let height = 0;
    let x = 0;
    let y = 0;
    if (naturalWidth >= naturalHeight) {
      height = displayHeight;
      width = height * POSTER_ASPECT;
      x = Math.max(0, (displayWidth - width) / 2);
      y = 0;
    } else {
      width = displayWidth;
      height = width / POSTER_ASPECT;
      x = 0;
      y = Math.max(0, (displayHeight - height) / 2);
    }
    setCropImageSize({ displayWidth, displayHeight, naturalWidth, naturalHeight });
    setCropRect({ x, y, width, height });
  };

  const beginCropDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    cropDragRef.current = {
      startX: event.clientX,
      startY: event.clientY,
      originX: cropRect.x,
      originY: cropRect.y,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handleCropDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    const dragState = cropDragRef.current;
    if (!dragState) {
      return;
    }
    const deltaX = event.clientX - dragState.startX;
    const deltaY = event.clientY - dragState.startY;
    setCropRect((prev) => {
      const next = { ...prev };
      if (cropImageSize.naturalWidth >= cropImageSize.naturalHeight) {
        next.x = Math.min(Math.max(0, dragState.originX + deltaX), cropImageSize.displayWidth - prev.width);
      } else {
        next.y = Math.min(Math.max(0, dragState.originY + deltaY), cropImageSize.displayHeight - prev.height);
      }
      return next;
    });
  };

  const endCropDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (cropDragRef.current) {
      cropDragRef.current = null;
      if (event.currentTarget.hasPointerCapture(event.pointerId)) {
        event.currentTarget.releasePointerCapture(event.pointerId);
      }
    }
  };

  const handleConfirmCrop = () => {
    if (!selected || !meta?.cover || cropImageSize.displayWidth === 0 || cropImageSize.displayHeight === 0) {
      return;
    }
    const scaleX = cropImageSize.naturalWidth / cropImageSize.displayWidth;
    const scaleY = cropImageSize.naturalHeight / cropImageSize.displayHeight;
    const payload = {
      x: Math.round(cropRect.x * scaleX),
      y: Math.round(cropRect.y * scaleY),
      width: Math.round(cropRect.width * scaleX),
      height: Math.round(cropRect.height * scaleY),
    };
    startTransition(async () => {
      try {
        setMessage("从封面截取海报...");
        const poster = await cropPosterFromCover(selected.id, payload);
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

  return (
    <>
      <div className="two-col">
        <aside className="panel review-list-panel">
          <h2 style={{ marginTop: 0, marginBottom: 12 }}>Review 列表</h2>
          <div className="review-job-list">
            {items.length === 0 ? <div style={{ color: "var(--muted)" }}>当前没有待 review 的任务</div> : null}
            {items.map((job) => (
              <div
                key={job.id}
                className="panel review-job-card"
                style={{
                  border: selected?.id === job.id ? "1px solid var(--accent)" : undefined,
                  background: selected?.id === job.id ? "rgba(180, 79, 45, 0.08)" : undefined,
                }}
              >
                <button className="review-job-card-main" onClick={() => loadDetail(job)} disabled={isPending}>
                  <div className="review-job-card-path">{job.rel_path}</div>
                  <div className="review-job-card-number">{job.number}</div>
                  <div className="review-job-card-time">更新时间 {formatUnixMillis(job.updated_at)}</div>
                </button>
                <div className="review-job-card-actions">
                  <button
                    type="button"
                    className="btn review-inline-icon-btn review-action-approve"
                    onClick={handleImport}
                    disabled={isPending || selected?.id !== job.id}
                    aria-label="入库"
                    title="入库"
                  >
                    <Check size={16} />
                  </button>
                  <button
                    type="button"
                    className="btn review-inline-icon-btn"
                    onClick={handleDelete}
                    disabled={isPending || selected?.id !== job.id}
                    aria-label="删除"
                    title="删除"
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </aside>
        <section className="panel review-detail-panel">
          <div className="review-header">
            <div>
              <h2 style={{ margin: 0 }}>Review 内容</h2>
              {selected ? <div className="review-subtitle">当前任务 #{selected.id} / {selected.rel_path}</div> : null}
            </div>
            <div className="review-actions">
              {message ? <span className="review-message">{message}</span> : null}
              <button
                type="button"
                className="btn review-inline-icon-btn"
                onClick={handleRestoreRaw}
                disabled={!selected || isPending || !hasRawMeta}
                aria-label="恢复原始刮削内容"
                title="恢复原始刮削内容"
              >
                <RotateCcw size={14} />
              </button>
            </div>
          </div>
          {meta ? (
            <div className="review-content review-content-single">
              <div className="review-form">
                <div className="review-field">
                  <span className="review-label review-label-side">标题</span>
                  <input
                    className="input review-input-strong"
                    value={meta.title ?? ""}
                    onChange={(e) => updateMeta({ title: e.target.value })}
                    onBlur={handleAutoSave}
                  />
                </div>
                <div className="review-field">
                  <span className="review-label review-label-side">翻译标题</span>
                  <input
                    className="input"
                    value={meta.title_translated ?? ""}
                    onChange={(e) => updateMeta({ title_translated: e.target.value })}
                    onBlur={handleAutoSave}
                  />
                </div>
                <div className="review-meta-row review-meta-row-4">
                  <div className="review-field">
                    <span className="review-label review-label-side">导演</span>
                    <input className="input" value={meta.director ?? ""} onChange={(e) => updateMeta({ director: e.target.value })} onBlur={handleAutoSave} />
                  </div>
                  <div className="review-field">
                    <span className="review-label review-label-side">制作商</span>
                    <input className="input" value={meta.studio ?? ""} onChange={(e) => updateMeta({ studio: e.target.value })} onBlur={handleAutoSave} />
                  </div>
                  <div className="review-field">
                    <span className="review-label review-label-side">发行商</span>
                    <input className="input" value={meta.label ?? ""} onChange={(e) => updateMeta({ label: e.target.value })} onBlur={handleAutoSave} />
                  </div>
                  <div className="review-field">
                    <span className="review-label review-label-side">系列</span>
                    <input className="input" value={meta.series ?? ""} onChange={(e) => updateMeta({ series: e.target.value })} onBlur={handleAutoSave} />
                  </div>
                </div>
                <div className="review-meta-row review-meta-row-2">
                  <div className="review-field review-field-area">
                    <span className="review-label review-label-side">简介</span>
                    <textarea className="input review-textarea" value={meta.plot ?? ""} onChange={(e) => updateMeta({ plot: e.target.value })} onBlur={handleAutoSave} />
                  </div>
                  <div className="review-field review-field-area">
                    <span className="review-label review-label-side">翻译简介</span>
                    <textarea
                      className="input review-textarea"
                      value={meta.plot_translated ?? ""}
                      onChange={(e) => updateMeta({ plot_translated: e.target.value })}
                      onBlur={handleAutoSave}
                    />
                  </div>
                </div>
                <div className="review-meta-row">
                  <TokenEditor
                    label="演员"
                    placeholder="输入演员名后输入逗号"
                    value={normalizeList(meta.actors)}
                    onChange={(next) => updateMeta({ actors: next })}
                    onBlurSave={handleAutoSave}
                  />
                </div>
                <div className="review-meta-row">
                  <TokenEditor
                    label="标签"
                    placeholder="输入标签后输入逗号"
                    value={normalizeList(meta.genres)}
                    onChange={(next) => updateMeta({ genres: next })}
                    onBlurSave={handleAutoSave}
                  />
                </div>
                <div className="review-media-offset">
                  <div className="review-artwork-row">
                    {meta.cover ? (
                      <button
                        type="button"
                        className="panel review-image-card review-image-card-cover"
                        onClick={() => setPreview({ title: imageTitle("cover"), item: meta.cover! })}
                      >
                        <span className="review-image-title">封面</span>
                        <div className="review-image-box review-image-box-cover">
                          <Image src={getAssetURL(meta.cover.key)} alt="cover" fill style={THUMB_IMAGE_STYLE} unoptimized />
                        </div>
                      </button>
                    ) : <div className="panel review-image-card review-image-card-cover review-image-empty">暂无封面</div>}
                    {meta.poster ? (
                      <div className="panel review-image-card review-image-card-poster">
                        <span className="review-image-title">海报</span>
                        <button type="button" className="btn review-inline-icon-btn review-image-crop-btn" onClick={openCropper} aria-label="从封面截取海报" title="从封面截取海报">
                          <Crop size={14} />
                        </button>
                        <div className="review-image-box review-image-box-poster">
                          <button type="button" className="review-image-hit" onClick={() => setPreview({ title: imageTitle("poster"), item: meta.poster! })}>
                            <Image src={getAssetURL(meta.poster.key)} alt="poster" fill style={THUMB_IMAGE_STYLE} unoptimized />
                          </button>
                        </div>
                      </div>
                    ) : (
                      <div className="panel review-image-card review-image-card-poster review-image-empty">
                        <span className="review-image-title">海报</span>
                        <button type="button" className="btn review-inline-icon-btn review-image-crop-btn" onClick={openCropper} aria-label="从封面截取海报" title="从封面截取海报">
                          <Crop size={14} />
                        </button>
                        暂无海报
                      </div>
                    )}
                  </div>
                </div>
                <div className="review-media-offset">
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
                        <button key={item.key} type="button" className="review-fanart-item" onClick={() => setPreview({ title: imageTitle("fanart"), item })}>
                          <Image src={getAssetURL(item.key)} alt={item.name} fill style={THUMB_IMAGE_STYLE} unoptimized />
                        </button>
                      ))}
                      {(meta.sample_images ?? []).length === 0 ? <div className="review-fanart-empty">暂无 fanart</div> : null}
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
        <div className="review-preview-overlay" onClick={() => setCropOpen(false)}>
          <div className="review-preview-dialog panel review-crop-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="review-crop-head">
              <div className="review-preview-title">从封面截取海报</div>
            </div>
            <div className="review-crop-stage">
              <div className="review-crop-canvas">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={getAssetURL(meta.cover.key)}
                  alt="cover crop preview"
                  className="review-crop-image"
                  onLoad={handleCropImageLoad}
                />
                {cropRect.width > 0 && cropRect.height > 0 ? (
                  <div
                    className="review-crop-selection"
                    style={{
                      left: cropRect.x,
                      top: cropRect.y,
                      width: cropRect.width,
                      height: cropRect.height,
                    }}
                    onPointerDown={beginCropDrag}
                    onPointerMove={handleCropDrag}
                    onPointerUp={endCropDrag}
                    onPointerCancel={endCropDrag}
                  />
                ) : null}
                {cropRect.width > 0 && cropRect.height > 0 ? (
                  <button
                    type="button"
                    className="btn review-crop-confirm"
                    style={{
                      left: cropRect.x + cropRect.width - 54,
                      top: cropRect.y + 8,
                    }}
                    onClick={handleConfirmCrop}
                  >
                    截取
                  </button>
                ) : null}
              </div>
            </div>
          </div>
        </div>
      ) : null}
      {deleteConfirmOpen && selected ? (
        <div className="review-preview-overlay" onClick={() => setDeleteConfirmOpen(false)}>
          <div className="panel review-confirm-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="review-confirm-title">确认删除</div>
            <div className="review-confirm-body">
              这会删除当前任务以及对应的源文件。
              <br />
              <span className="review-confirm-path">{selected.rel_path}</span>
            </div>
            <div className="review-confirm-actions">
              <button type="button" className="btn" onClick={() => setDeleteConfirmOpen(false)}>
                取消
              </button>
              <button type="button" className="btn btn-primary" onClick={confirmDelete} disabled={isPending}>
                删除
              </button>
            </div>
          </div>
        </div>
      ) : null}
      {restoreConfirmOpen && selected ? (
        <div className="review-preview-overlay" onClick={() => setRestoreConfirmOpen(false)}>
          <div className="panel review-confirm-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="review-confirm-title">恢复原始内容</div>
            <div className="review-confirm-body">
              这会用最初刮削得到的原始内容覆盖当前修改。
              <br />
              <span className="review-confirm-path">{selected.rel_path}</span>
            </div>
            <div className="review-confirm-actions">
              <button type="button" className="btn" onClick={() => setRestoreConfirmOpen(false)}>
                取消
              </button>
              <button type="button" className="btn btn-primary" onClick={confirmRestoreRaw} disabled={isPending}>
                恢复
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
