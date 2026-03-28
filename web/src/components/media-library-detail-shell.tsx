"use client";

import { ChevronLeft, Plus, RefreshCw, X } from "lucide-react";
import Link from "next/link";
import { type SetStateAction, useEffect, useEffectEvent, useRef, useState, useTransition } from "react";

import type { LibraryMeta, MediaLibraryDetail } from "@/lib/api";
import {
  deleteMediaLibraryFile,
  getMediaLibraryFileURL,
  getMediaLibraryItem,
  replaceMediaLibraryAsset,
  updateMediaLibraryItem,
} from "@/lib/api";

interface Props {
  initialDetail: MediaLibraryDetail;
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

function hasTranslatedCopy(meta: LibraryMeta | null) {
  if (!meta) {
    return false;
  }
  return Boolean(meta.title_translated.trim() || meta.plot_translated.trim());
}

function getVariantPosterPath(detail: MediaLibraryDetail | null, variantKey: string) {
  const variant = pickVariant(detail, variantKey);
  return variant?.poster_path || variant?.meta.poster_path || detail?.meta.poster_path || detail?.item.poster_path || "";
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
      <div className={`token-editor${singleLine ? " token-editor-single-line" : ""}`} onClick={() => document.getElementById(`media-library-token-${label}`)?.focus()}>
        {value.map((item, idx) => (
          <span key={`${item}-${idx}`} className="token-chip">
            {item}
            <button type="button" className="token-chip-remove" aria-label={`删除${item}`} onClick={() => removeAt(idx)}>
              <X size={11} />
            </button>
          </span>
        ))}
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

export function MediaLibraryDetailShell({ initialDetail }: Props) {
  const initialDraftMeta = cloneMeta(initialDetail.meta);
  const [detail, setDetail] = useState<MediaLibraryDetail>(initialDetail);
  const [selectedVariantKey, setSelectedVariantKey] = useState(initialDetail.primary_variant_key || initialDetail.variants[0]?.key || "");
  const [copyMode, setCopyMode] = useState<"translated" | "original">(hasTranslatedCopy(initialDetail.meta) ? "translated" : "original");
  const [draftMeta, setDraftMeta] = useState<LibraryMeta>(initialDraftMeta);
  const [message, setMessage] = useState("");
  const [preview, setPreview] = useState<{ title: string; path: string; name: string } | null>(null);
  const [assetOverrides, setAssetOverrides] = useState<Record<string, string>>({});
  const [isPending, startTransition] = useTransition();
  const detailRef = useRef<MediaLibraryDetail>(initialDetail);
  const draftMetaRef = useRef<LibraryMeta>(initialDraftMeta);
  const lastSavedMetaRef = useRef(serializeMeta(initialDraftMeta));
  const saveQueueRef = useRef<Promise<boolean>>(Promise.resolve(true));
  const uploadActiveRef = useRef(false);
  const assetOverridesRef = useRef<Record<string, string>>({});

  const currentVariant = pickVariant(detail, selectedVariantKey);
  const showVariantSwitch = detail.variants.length > 1;
  const activeTitleValue = copyMode === "translated" ? draftMeta.title_translated : draftMeta.title;
  const activePlotValue = copyMode === "translated" ? draftMeta.plot_translated : draftMeta.plot;
  const fanartFiles = detail.files.filter((file) => file.rel_path.includes("/extrafanart/"));
  const selectedPoster = currentVariant?.poster_path || currentVariant?.meta.poster_path || draftMeta.poster_path || detail.item.poster_path || "";
  const selectedCover =
    currentVariant?.cover_path ||
    currentVariant?.meta.cover_path ||
    currentVariant?.meta.fanart_path ||
    currentVariant?.meta.thumb_path ||
    draftMeta.cover_path ||
    draftMeta.fanart_path ||
    detail.item.cover_path ||
    "";

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

  const syncDetail = (next: MediaLibraryDetail) => {
    setDetail(next);
    detailRef.current = next;
    const nextDraftMeta = cloneMeta(next.meta);
    setDraftMeta(nextDraftMeta);
    draftMetaRef.current = nextDraftMeta;
    setCopyMode((current) => (current === "translated" && !hasTranslatedCopy(next.meta) ? "original" : current || "original"));
    setSelectedVariantKey((current) => {
      if (current && next.variants.some((item) => item.key === current)) {
        return current;
      }
      return next.primary_variant_key || next.variants[0]?.key || "";
    });
    lastSavedMetaRef.current = serializeMeta(nextDraftMeta);
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
    const timer = window.setInterval(() => {
      void refreshDetail();
    }, 8000);
    return () => window.clearInterval(timer);
  }, []);

  const updateDraftMeta = (updater: SetStateAction<LibraryMeta>) => {
    setDraftMeta((prev) => {
      const next = typeof updater === "function" ? updater(prev) : updater;
      draftMetaRef.current = next;
      return next;
    });
  };

  const resolveImageSrc = (path: string) => assetOverrides[path] ?? getMediaLibraryFileURL(path);

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

  const persistMeta = (meta: LibraryMeta, messageText: string, options?: { silent?: boolean }) => {
    const normalizedMeta = normalizeMeta(meta);
    const serialized = serializeMeta(normalizedMeta);
    if (serialized === lastSavedMetaRef.current) {
      return Promise.resolve(true);
    }
    const task = saveQueueRef.current.then(async () => {
      if (serialized === lastSavedMetaRef.current) {
        return true;
      }
      try {
        if (!options?.silent) {
          setMessage("保存媒体库 NFO...");
        }
        const next = await updateMediaLibraryItem(detailRef.current.item.id, normalizedMeta);
        syncDetail(next);
        setMessage(messageText);
        return true;
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "保存媒体库 NFO 失败");
        return false;
      }
    });
    saveQueueRef.current = task.catch(() => true);
    return task;
  };

  const handleBlurSave = () => {
    startTransition(async () => {
      await persistMeta(draftMetaRef.current, "已自动保存", { silent: true });
    });
  };

  const openUploadPicker = (kind: "poster" | "cover" | "fanart") => {
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
          setMessage(kind === "fanart" ? "上传 extrafanart..." : `替换${kind === "poster" ? "海报" : "封面"}...`);
          const next = await replaceMediaLibraryAsset(detailRef.current.item.id, currentVariant?.key ?? "", kind, file);
          syncDetail(next);
          if (kind === "poster") {
            setAssetOverride(getVariantPosterPath(next, currentVariant?.key ?? ""), file);
          } else if (kind === "cover") {
            setAssetOverride(getVariantCoverPath(next, currentVariant?.key ?? ""), file);
          }
          setMessage(kind === "fanart" ? "Extrafanart 已上传" : `${kind === "poster" ? "海报" : "封面"}已更新`);
        } catch (error) {
          setMessage(error instanceof Error ? error.message : "上传图片失败");
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
        const next = await deleteMediaLibraryFile(detailRef.current.item.id, path);
        clearAssetOverride(path);
        if (preview?.path === path) {
          setPreview(null);
        }
        syncDetail(next);
        setMessage("Extrafanart 已删除");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "删除 extrafanart 失败");
      }
    });
  };

  const handleRefresh = () => {
    startTransition(async () => {
      try {
        setMessage("刷新媒体库详情...");
        const next = await getMediaLibraryItem(detailRef.current.item.id);
        syncDetail(next);
        setMessage("媒体库详情已刷新");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "刷新媒体库详情失败");
      }
    });
  };

  return (
    <section className="panel library-detail-panel media-library-detail-shell">
      <div className="media-library-detail-topbar">
        <Link href="/media-library" className="media-library-back-link">
          <ChevronLeft size={16} />
          返回媒体库
        </Link>
        <div className="media-library-header-actions">
          {message ? (
            <span className="review-message" data-tone={/失败|error/i.test(message) ? "danger" : "info"}>
              {message}
            </span>
          ) : null}
          <button className="btn" type="button" onClick={handleRefresh} disabled={isPending}>
            <RefreshCw size={16} />
            刷新详情
          </button>
        </div>
      </div>

      <div className="review-header library-detail-header">
        <div>
          <div className="review-list-kicker">Media Library Item</div>
          <h2 className="review-detail-title">{detail.item.title || detail.item.name}</h2>
          <div className="review-subtitle">{detail.item.rel_path}</div>
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
          <div className="library-info-strip">
            <div className="library-copy-toggle" role="tablist" aria-label="标题与简介语言切换">
              <button type="button" className="library-copy-toggle-btn" data-active={copyMode === "translated"} onClick={() => setCopyMode("translated")}>
                中文
              </button>
              <button type="button" className="library-copy-toggle-btn" data-active={copyMode === "original"} onClick={() => setCopyMode("original")}>
                原文
              </button>
            </div>
          </div>

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
                  <input className="input" value={draftMeta.director} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, director: e.target.value }))} onBlur={handleBlurSave} />
                </div>
                <div className="review-field">
                  <span className="review-label review-label-side">片商</span>
                  <input className="input" value={draftMeta.studio} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, studio: e.target.value }))} onBlur={handleBlurSave} />
                </div>
                <div className="review-field">
                  <span className="review-label review-label-side">发行商</span>
                  <input className="input" value={draftMeta.label} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, label: e.target.value }))} onBlur={handleBlurSave} />
                </div>
                <div className="review-field">
                  <span className="review-label review-label-side">系列</span>
                  <input className="input" value={draftMeta.series} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, series: e.target.value }))} onBlur={handleBlurSave} />
                </div>
              </div>
              <div className="review-meta-row review-meta-row-2 library-meta-grid">
                <div className="review-field">
                  <span className="review-label review-label-side">番号</span>
                  <input className="input" value={draftMeta.number} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, number: e.target.value }))} onBlur={handleBlurSave} />
                </div>
                <div className="review-field">
                  <span className="review-label review-label-side">发行日期</span>
                  <input className="input" placeholder="YYYY-MM-DD" value={draftMeta.release_date} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, release_date: e.target.value }))} onBlur={handleBlurSave} />
                </div>
                <div className="review-field">
                  <span className="review-label review-label-side">时长</span>
                  <input
                    className="input"
                    inputMode="numeric"
                    value={draftMeta.runtime ? String(draftMeta.runtime) : ""}
                    onChange={(e) => updateDraftMeta((prev) => ({ ...prev, runtime: Number.parseInt(e.target.value || "0", 10) || 0 }))}
                    onBlur={handleBlurSave}
                  />
                </div>
                <div className="review-field">
                  <span className="review-label review-label-side">来源</span>
                  <input className="input" value={draftMeta.source} onChange={(e) => updateDraftMeta((prev) => ({ ...prev, source: e.target.value }))} onBlur={handleBlurSave} />
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
                  label="演员"
                  placeholder="输入后回车或逗号确认"
                  value={draftMeta.actors}
                  onChange={(next) => updateDraftMeta((prev) => ({ ...prev, actors: next }))}
                  onBlurSave={handleBlurSave}
                  singleLine
                />
              </div>
            </div>

            <div className="panel review-image-card review-image-card-poster review-top-poster review-main-poster">
              <div className="review-image-card-head">
                <span className="review-image-title">海报</span>
              </div>
              <div className={`review-image-box review-image-box-poster${selectedPoster ? "" : " review-upload-empty"}`}>
                {selectedPoster ? (
                  <button type="button" className="review-image-hit" onClick={() => { if (!uploadActiveRef.current) setPreview({ title: "海报", path: selectedPoster, name: "海报" }); }}>
                    <img src={resolveImageSrc(selectedPoster)} alt="海报" className="library-poster-image" />
                  </button>
                ) : (
                  <div className="library-preview-empty">暂无海报</div>
                )}
                <button type="button" className="review-upload-overlay" onClick={() => openUploadPicker("poster")} aria-label="上传海报" title="上传海报" disabled={isPending}>
                  <Plus size={18} />
                </button>
              </div>
            </div>
          </div>

          <div className="review-meta-row review-meta-row-full">
            <TokenEditor
              label="标签"
              placeholder="输入后回车或逗号确认"
              value={draftMeta.genres}
              onChange={(next) => updateDraftMeta((prev) => ({ ...prev, genres: next }))}
              onBlurSave={handleBlurSave}
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
                    <img src={resolveImageSrc(selectedCover)} alt="封面" className="library-cover-image" />
                  </button>
                ) : (
                  <div className="library-preview-empty">暂无封面</div>
                )}
                <button type="button" className="review-upload-overlay" onClick={() => openUploadPicker("cover")} aria-label="上传封面" title="上传封面" disabled={isPending}>
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
                      <button type="button" className="review-image-hit" onClick={() => { if (!uploadActiveRef.current) setPreview({ title: "Extrafanart", path: file.rel_path, name: file.name }); }}>
                        <img src={resolveImageSrc(file.rel_path)} alt={file.name} className="library-fanart-image" />
                      </button>
                      <button
                        type="button"
                        className="btn review-inline-icon-btn review-fanart-delete"
                        onClick={() => handleDeleteFanart(file.rel_path)}
                        aria-label="删除 extrafanart"
                        title="删除 extrafanart"
                        disabled={isPending}
                      >
                        <X size={12} />
                      </button>
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
    </section>
  );
}
