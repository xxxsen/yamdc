"use client";

import { X } from "lucide-react";
import Image from "next/image";
import { useRef, useState, useTransition } from "react";

import type { JobItem, MediaFileRef, ReviewMeta, ScrapeDataItem } from "@/lib/api";
import { deleteJob, getAssetURL, getReviewJob, importReviewJob, saveReviewJob } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

interface Props {
  jobs: JobItem[];
  initialScrapeData: ScrapeDataItem | null;
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
    return "Poster";
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
    <label className="review-field review-field-tokens">
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
    </label>
  );
}

export function ReviewShell({ jobs, initialScrapeData }: Props) {
  const initialMeta = parseMeta(initialScrapeData);
  const [items, setItems] = useState<JobItem[]>(jobs);
  const [selected, setSelected] = useState<JobItem | null>(jobs[0] ?? null);
  const [meta, setMeta] = useState<ReviewMeta | null>(initialMeta);
  const [message, setMessage] = useState<string>(jobs.length === 0 ? "当前没有待 review 的任务" : "");
  const [preview, setPreview] = useState<{ title: string; item: MediaFileRef } | null>(null);
  const [isPending, startTransition] = useTransition();
  const lastSavedPayloadRef = useRef(buildPayload(initialMeta));
  const selectedRef = useRef<JobItem | null>(jobs[0] ?? null);
  const metaRef = useRef<ReviewMeta | null>(initialMeta);

  const syncStateWithData = (data: ScrapeDataItem | null) => {
    const nextMeta = parseMeta(data);
    const payload = buildPayload(nextMeta);
    setMeta(nextMeta);
    metaRef.current = nextMeta;
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
    const ok = window.confirm(`确认删除该任务及源文件吗？\n\n${selected.rel_path}`);
    if (!ok) {
      return;
    }
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

  return (
    <>
      <div className="two-col">
        <aside className="panel review-list-panel">
          <h2 style={{ marginTop: 0, marginBottom: 12 }}>Review 列表</h2>
          <div className="review-job-list">
            {items.length === 0 ? <div style={{ color: "var(--muted)" }}>当前没有待 review 的任务</div> : null}
            {items.map((job) => (
              <button
                key={job.id}
                className="panel review-job-card"
                style={{
                  border: selected?.id === job.id ? "1px solid var(--accent)" : undefined,
                  background: selected?.id === job.id ? "rgba(180, 79, 45, 0.08)" : undefined,
                }}
                onClick={() => loadDetail(job)}
                disabled={isPending}
              >
                <div className="review-job-card-path">{job.rel_path}</div>
                <div className="review-job-card-number">{job.number}</div>
                <div className="review-job-card-time">更新时间 {formatUnixMillis(job.updated_at)}</div>
              </button>
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
              <button className="btn btn-primary" onClick={handleImport} disabled={!selected || isPending || !meta}>
                入库
              </button>
              <button className="btn" onClick={handleDelete} disabled={!selected || isPending}>
                删除
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
                  <label className="review-field">
                    <span className="review-label review-label-side">导演</span>
                    <input className="input" value={meta.director ?? ""} onChange={(e) => updateMeta({ director: e.target.value })} onBlur={handleAutoSave} />
                  </label>
                  <label className="review-field">
                    <span className="review-label review-label-side">制作商</span>
                    <input className="input" value={meta.studio ?? ""} onChange={(e) => updateMeta({ studio: e.target.value })} onBlur={handleAutoSave} />
                  </label>
                  <label className="review-field">
                    <span className="review-label review-label-side">发行商</span>
                    <input className="input" value={meta.label ?? ""} onChange={(e) => updateMeta({ label: e.target.value })} onBlur={handleAutoSave} />
                  </label>
                  <label className="review-field">
                    <span className="review-label review-label-side">系列</span>
                    <input className="input" value={meta.series ?? ""} onChange={(e) => updateMeta({ series: e.target.value })} onBlur={handleAutoSave} />
                  </label>
                </div>
                <div className="review-meta-row review-meta-row-2">
                  <label className="review-field review-field-area">
                    <span className="review-label review-label-side">简介</span>
                    <textarea className="input review-textarea" value={meta.plot ?? ""} onChange={(e) => updateMeta({ plot: e.target.value })} onBlur={handleAutoSave} />
                  </label>
                  <label className="review-field review-field-area">
                    <span className="review-label review-label-side">翻译简介</span>
                    <textarea
                      className="input review-textarea"
                      value={meta.plot_translated ?? ""}
                      onChange={(e) => updateMeta({ plot_translated: e.target.value })}
                      onBlur={handleAutoSave}
                    />
                  </label>
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
                      <button
                        type="button"
                        className="panel review-image-card review-image-card-poster"
                        onClick={() => setPreview({ title: imageTitle("poster"), item: meta.poster! })}
                      >
                        <span className="review-image-title">Poster</span>
                        <div className="review-image-box review-image-box-poster">
                          <Image src={getAssetURL(meta.poster.key)} alt="poster" fill style={THUMB_IMAGE_STYLE} unoptimized />
                        </div>
                      </button>
                    ) : <div className="panel review-image-card review-image-card-poster review-image-empty">暂无 Poster</div>}
                  </div>
                </div>
                <div className="review-media-offset">
                  <div className="panel review-fanart-panel">
                    <div className="review-fanart-strip">
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
    </>
  );
}
