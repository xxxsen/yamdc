"use client";

import { Check, ChevronLeft, Pencil, X } from "lucide-react";
import Link from "next/link";
import { type SetStateAction, useEffect, useEffectEvent, useRef, useState, useTransition } from "react";

import type { LibraryMeta, MediaLibraryDetail } from "@/lib/api";
import { getMediaLibraryFileURL, getMediaLibraryItem, updateMediaLibraryItem } from "@/lib/api";

interface Props {
  initialDetail: MediaLibraryDetail;
  stageOnly?: boolean;
  onDetailChange?: (detail: MediaLibraryDetail) => void;
}

function cloneMeta(meta: LibraryMeta | null): LibraryMeta {
  return {
    title: meta?.title ?? "",
    title_translated: meta?.title_translated ?? "",
    original_title: meta?.original_title ?? "",
    plot: meta?.plot ?? "",
    plot_translated: meta?.plot_translated ?? "",
    number: meta?.number ?? "",
    release_date: meta?.release_date ?? "",
    runtime: meta?.runtime ?? 0,
    studio: meta?.studio ?? "",
    label: meta?.label ?? "",
    series: meta?.series ?? "",
    director: meta?.director ?? "",
    actors: [...(meta?.actors ?? [])],
    genres: [...(meta?.genres ?? [])],
    poster_path: meta?.poster_path ?? "",
    cover_path: meta?.cover_path ?? "",
    fanart_path: meta?.fanart_path ?? "",
    thumb_path: meta?.thumb_path ?? "",
    source: meta?.source ?? "",
    scraped_at: meta?.scraped_at ?? "",
  };
}

function pickVariant(detail: MediaLibraryDetail | null, key: string) {
  if (!detail) {
    return null;
  }
  return detail.variants.find((item) => item.key === key) ?? detail.variants[0] ?? null;
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

function getVariantCoverPath(detail: MediaLibraryDetail | null, variantKey: string) {
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

function TokenEditor({
  label,
  placeholder,
  value,
  onChange,
  singleLine = false,
  readOnly = false,
}: {
  label: string;
  placeholder: string;
  value: string[];
  onChange: (next: string[]) => void;
  singleLine?: boolean;
  readOnly?: boolean;
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
  };

  return (
    <div className="review-field review-field-tokens">
      <span className="review-label review-label-side">{label}</span>
      <div
        className={`token-editor${singleLine ? " token-editor-single-line" : ""}${readOnly ? " token-editor-readonly" : ""}`}
        onClick={() => {
          if (!readOnly) {
            document.getElementById(`media-library-token-${label}`)?.focus();
          }
        }}
      >
        {value.map((item, idx) => (
          <span key={`${item}-${idx}`} className="token-chip">
            {item}
            {!readOnly ? (
              <button type="button" className="token-chip-remove" aria-label={`删除${item}`} onClick={() => removeAt(idx)}>
                <X size={11} />
              </button>
            ) : null}
          </span>
        ))}
        {!readOnly ? (
          <input
            id={`media-library-token-${label}`}
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
              } else if (e.key === "Backspace" && draft === "" && value.length > 0) {
                onChange(value.slice(0, -1));
              }
            }}
            onBlur={() => {
              commitDraft();
            }}
          />
        ) : value.length === 0 ? (
          <span className="library-inline-muted">{placeholder}</span>
        ) : null}
      </div>
    </div>
  );
}

export function MediaLibraryDetailShell({ initialDetail, stageOnly = false, onDetailChange }: Props) {
  const initialDraftMeta = cloneMeta(initialDetail.meta);
  const [detail, setDetail] = useState<MediaLibraryDetail>(initialDetail);
  const [selectedVariantKey, setSelectedVariantKey] = useState(initialDetail.primary_variant_key || initialDetail.variants[0]?.key || "");
  const [draftMeta, setDraftMeta] = useState<LibraryMeta>(initialDraftMeta);
  const [message, setMessage] = useState("");
  const [preview, setPreview] = useState<{ title: string; path: string; name: string } | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [isPending, startTransition] = useTransition();
  const detailRef = useRef<MediaLibraryDetail>(initialDetail);
  const draftMetaRef = useRef<LibraryMeta>(initialDraftMeta);
  const lastSavedMetaRef = useRef(serializeMeta(initialDraftMeta));

  const currentVariant = pickVariant(detail, selectedVariantKey);
  const showVariantSwitch = detail.variants.length > 1;
  const fanartFiles = detail.files.filter((file) => file.rel_path.includes("/extrafanart/"));
  const selectedPoster = currentVariant?.poster_path || currentVariant?.meta.poster_path || draftMeta.poster_path || detail.item.poster_path || "";
  const selectedCover = getVariantCoverPath(detail, selectedVariantKey) || draftMeta.cover_path || draftMeta.fanart_path || draftMeta.thumb_path || detail.item.cover_path || "";
  const detailDisplayTitle =
    draftMeta.title.trim() ||
    detail.item.title ||
    detail.item.name ||
    draftMeta.title_translated.trim();
  const detailDisplayTitleSecondary =
    draftMeta.title_translated.trim() && draftMeta.title_translated.trim() !== detailDisplayTitle ? draftMeta.title_translated.trim() : "";
  const detailDisplayNumber = draftMeta.number || detail.item.number || "未命名影片";
  const detailDisplayPlot = draftMeta.plot.trim() || draftMeta.plot_translated.trim();
  const detailDisplayPlotSecondary =
    draftMeta.plot.trim() && draftMeta.plot_translated.trim() && draftMeta.plot_translated.trim() !== draftMeta.plot.trim()
      ? draftMeta.plot_translated.trim()
      : "";

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

  const resolveImageSrc = (path: string) => getMediaLibraryFileURL(path);

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

  const renderFanartSection = (extraClassName = "") => {
    if (fanartFiles.length === 0) {
      return null;
    }
    return (
      <div className={`panel review-fanart-panel library-fanart-panel media-library-fanart-section media-library-fanart-compact ${extraClassName}`.trim()}>
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
              <button type="button" className="review-image-hit" onClick={() => setPreview({ title: "Extrafanart", path: file.rel_path, name: file.name })}>
                <img src={resolveImageSrc(file.rel_path)} alt={file.name} className="library-fanart-image" />
              </button>
              <div className="sr-only">{file.name.split("/").pop()}</div>
            </div>
          ))}
        </div>
      </div>
    );
  };

  const stage = (
      <div className={`panel media-library-detail-stage media-library-backdrop${selectedCover ? "" : " media-library-backdrop-empty"}${stageOnly ? " media-library-detail-stage-inline" : ""}`}>
        {selectedCover ? (
          <button type="button" className="media-library-backdrop-hit" onClick={() => setPreview({ title: "封面", path: selectedCover, name: "封面" })}>
            <img src={resolveImageSrc(selectedCover)} alt="封面" className="media-library-backdrop-image" />
          </button>
        ) : (
          <div className="library-preview-empty media-library-backdrop-fallback">暂无封面</div>
        )}
        <div className="media-library-backdrop-scrim" aria-hidden="true" />
        <div className="media-library-stage-actions">
          {message ? (
            <span className="review-message" data-tone="danger">
              {message}
            </span>
          ) : null}
          {isEditing ? (
            <>
              <button className="media-library-stage-action-btn" type="button" onClick={handleCancelEdit} disabled={isPending} aria-label="取消编辑">
                <X size={16} />
              </button>
              <button className="media-library-stage-action-btn media-library-stage-action-btn-primary" type="button" onClick={handleSaveEdit} disabled={isPending} aria-label="保存编辑">
                <Check size={16} />
              </button>
            </>
          ) : (
            <button className="media-library-stage-action-btn" type="button" onClick={() => { setMessage(""); setIsEditing(true); }} disabled={isPending} aria-label="编辑">
              <Pencil size={16} />
            </button>
          )}
        </div>
        <div className="media-library-detail-workspace">
          <div className="media-library-hero">
            <aside className="media-library-hero-poster-column">
              <div className="panel review-image-card review-image-card-poster media-library-hero-poster-card">
                <div className={`review-image-box review-image-box-poster${selectedPoster ? "" : " review-upload-empty"}`}>
                  {selectedPoster ? (
                    <button type="button" className="review-image-hit" onClick={() => setPreview({ title: "海报", path: selectedPoster, name: "海报" })}>
                      <img src={resolveImageSrc(selectedPoster)} alt="海报" className="library-poster-image" />
                    </button>
                  ) : (
                    <div className="library-preview-empty">暂无海报</div>
                  )}
                </div>
              </div>
            </aside>

            <div className="media-library-hero-side">
              {showVariantSwitch ? (
                <div className="panel library-variant-panel media-library-hero-variant-panel">
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

              <div className="panel media-library-hero-main">
                {isEditing ? (
                  <div className="media-library-fields-grid media-library-inline-editor">
                    <div className="review-field media-library-field-span-2">
                      <span className="review-label review-label-side">原始标题</span>
                      <input className="input review-input-strong" value={draftMeta.title} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, title: e.target.value }))} />
                    </div>
                    <div className="review-field media-library-field-span-2">
                      <span className="review-label review-label-side">翻译标题</span>
                      <input className="input" value={draftMeta.title_translated} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, title_translated: e.target.value }))} />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">影片 ID</span>
                      <input className="input" value={draftMeta.number} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, number: e.target.value }))} />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">发行日期</span>
                      <input className="input" placeholder="YYYY-MM-DD" value={draftMeta.release_date} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, release_date: e.target.value }))} />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">时长</span>
                      <input className="input" inputMode="numeric" value={draftMeta.runtime ? String(draftMeta.runtime) : ""} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, runtime: Number.parseInt(e.target.value || "0", 10) || 0 }))} />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">来源</span>
                      <input className="input" value={draftMeta.source} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, source: e.target.value }))} />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">导演</span>
                      <input className="input" value={draftMeta.director} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, director: e.target.value }))} />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">片商</span>
                      <input className="input" value={draftMeta.studio} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, studio: e.target.value }))} />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">发行商</span>
                      <input className="input" value={draftMeta.label} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, label: e.target.value }))} />
                    </div>
                    <div className="review-field">
                      <span className="review-label review-label-side">系列</span>
                      <input className="input" value={draftMeta.series} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, series: e.target.value }))} />
                    </div>
                    <div className="review-field review-field-area media-library-inline-plot-field">
                      <span className="review-label review-label-side">原始简介</span>
                      <textarea className="input review-textarea library-textarea media-library-inline-plot-textarea" value={draftMeta.plot} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, plot: e.target.value }))} />
                    </div>
                    <div className="review-field review-field-area media-library-inline-plot-field">
                      <span className="review-label review-label-side">翻译简介</span>
                      <textarea className="input review-textarea library-textarea media-library-inline-plot-textarea" value={draftMeta.plot_translated} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, plot_translated: e.target.value }))} />
                    </div>
                    <div className="media-library-field-span-2">
                      <TokenEditor label="演员" placeholder="输入后回车或逗号确认" value={draftMeta.actors} onChange={(next) => updateDraftMeta((prev) => ({ ...prev, actors: next }))} singleLine readOnly={false} />
                    </div>
                    <div className="media-library-field-span-2">
                      <TokenEditor label="标签" placeholder="输入后回车或逗号确认" value={draftMeta.genres} onChange={(next) => updateDraftMeta((prev) => ({ ...prev, genres: next }))} readOnly={false} />
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="media-library-hero-main-head">
                      <div className="media-library-hero-title-block">
                        <div className="media-library-hero-title">{detailDisplayTitle}</div>
                        {detailDisplayTitleSecondary ? <div className="media-library-hero-title-secondary">{detailDisplayTitleSecondary}</div> : null}
                      </div>
                    </div>

                    <div className="media-library-hero-facts">
                      <div className="media-library-hero-fact"><span>影片 ID</span><strong>{detailDisplayNumber}</strong></div>
                      <div className="media-library-hero-fact"><span>发行日期</span><strong>{draftMeta.release_date || "-"}</strong></div>
                      <div className="media-library-hero-fact"><span>时长</span><strong>{draftMeta.runtime ? `${draftMeta.runtime} 分钟` : "-"}</strong></div>
                      <div className="media-library-hero-fact"><span>来源</span><strong>{draftMeta.source || "-"}</strong></div>
                      <div className="media-library-hero-fact"><span>导演</span><strong>{draftMeta.director || "-"}</strong></div>
                      <div className="media-library-hero-fact"><span>片商</span><strong>{draftMeta.studio || "-"}</strong></div>
                      <div className="media-library-hero-fact"><span>发行商</span><strong>{draftMeta.label || "-"}</strong></div>
                      <div className="media-library-hero-fact"><span>系列</span><strong>{draftMeta.series || "-"}</strong></div>
                    </div>

                    <div className="media-library-hero-plot">
                      <div>{detailDisplayPlot || "暂无简介"}</div>
                      {detailDisplayPlotSecondary ? <div className="media-library-hero-plot-secondary">{detailDisplayPlotSecondary}</div> : null}
                    </div>

                    <div className="media-library-hero-taxonomy">
                      <div className="media-library-hero-taxonomy-row">
                        <span className="media-library-hero-taxonomy-label">演员</span>
                        <div className="media-library-hero-chip-row">
                          {draftMeta.actors.length > 0 ? draftMeta.actors.map((actor) => <span key={actor} className="token-chip">{actor}</span>) : <span className="library-inline-muted">暂无演员</span>}
                        </div>
                      </div>
                      <div className="media-library-hero-taxonomy-row">
                        <span className="media-library-hero-taxonomy-label">标签</span>
                        <div className="media-library-hero-chip-row">
                          {draftMeta.genres.length > 0 ? draftMeta.genres.map((genre) => <span key={genre} className="token-chip">{genre}</span>) : <span className="library-inline-muted">暂无标签</span>}
                        </div>
                      </div>
                    </div>
                  </>
                )}
              </div>

              {renderFanartSection("media-library-stage-fanart")}
            </div>
          </div>
        </div>
      </div>
  );

  return (
    <>
      {!stageOnly ? (
        <section className="panel library-detail-panel media-library-detail-shell">
          <div className="media-library-detail-topbar media-library-detail-topbar-wide">
            <div className="media-library-detail-heading">
              <Link href="/media-library" className="media-library-back-link">
                <ChevronLeft size={16} />
                返回媒体库
              </Link>
              <div className="media-library-detail-copy">
                <div className="review-list-kicker">Media Library Item</div>
                <h2 className="review-detail-title">{detailDisplayTitle}</h2>
              </div>
            </div>
          </div>
          {stage}
        </section>
      ) : (
        <div className="media-library-stage-only-shell">
          {stage}
        </div>
      )}

      {preview ? (
        <div className="review-preview-overlay" onClick={() => setPreview(null)}>
          <button type="button" className="review-preview-close" aria-label="关闭预览" onClick={() => setPreview(null)}>
            <X size={18} />
          </button>
          <div className="review-preview-dialog panel" onClick={(e) => e.stopPropagation()}>
            <div className="review-preview-title">{preview.title}</div>
            <div className="review-preview-frame">
              <img
                src={resolveImageSrc(preview.path)}
                alt={preview.name}
                style={{ width: "100%", height: "100%", objectFit: "contain", objectPosition: "center", display: "block" }}
              />
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
