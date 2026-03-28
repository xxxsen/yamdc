"use client";

import { Check, RefreshCw, Search, Upload, X } from "lucide-react";
import { useDeferredValue, useEffect, useState, useTransition } from "react";

import type { LibraryDetail, LibraryListItem, LibraryMeta } from "@/lib/api";
import { getLibraryFileURL, getLibraryItem, listLibraryItems, replaceLibraryAsset, updateLibraryItem } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

interface Props {
  items: LibraryListItem[];
  initialDetail: LibraryDetail | null;
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

function pickVariant(detail: LibraryDetail | null, key: string) {
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

function getCardImage(item: LibraryListItem) {
  return item.poster_path || item.cover_path;
}

function hasTranslatedCopy(meta: LibraryMeta | null) {
  if (!meta) {
    return false;
  }
  return Boolean(meta.title_translated.trim() || meta.plot_translated.trim());
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
      <div className={`token-editor${singleLine ? " token-editor-single-line" : ""}`} onClick={() => document.getElementById(`library-token-${label}`)?.focus()}>
        {value.map((item, idx) => (
          <span key={`${item}-${idx}`} className="token-chip">
            {item}
            <button type="button" className="token-chip-remove" aria-label={`删除${item}`} onClick={() => removeAt(idx)}>
              <X size={11} />
            </button>
          </span>
        ))}
        <input
          id={`library-token-${label}`}
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

export function LibraryShell({ items: initialItems, initialDetail }: Props) {
  const [items, setItems] = useState(initialItems);
  const [selectedPath, setSelectedPath] = useState(initialDetail?.item.rel_path ?? initialItems[0]?.rel_path ?? "");
  const [detail, setDetail] = useState<LibraryDetail | null>(initialDetail);
  const [selectedVariantKey, setSelectedVariantKey] = useState(
    initialDetail?.primary_variant_key ?? initialDetail?.variants[0]?.key ?? "",
  );
  const [copyMode, setCopyMode] = useState<"translated" | "original">(hasTranslatedCopy(initialDetail?.meta ?? null) ? "translated" : "original");
  const [draftMeta, setDraftMeta] = useState<LibraryMeta>(cloneMeta(initialDetail?.meta ?? null));
  const [keyword, setKeyword] = useState("");
  const [message, setMessage] = useState(initialItems.length === 0 ? "当前 savedir 里还没有已入库内容" : "");
  const [preview, setPreview] = useState<{ title: string; path: string; name: string } | null>(null);
  const [isPending, startTransition] = useTransition();
  const deferredKeyword = useDeferredValue(keyword);

  const query = deferredKeyword.trim().toLowerCase();
  const filteredItems = !query
    ? items
    : items.filter((item) => {
      const haystack = [
        item.title,
        item.number,
        item.rel_path,
        item.name,
        item.release_date,
        item.actors.join(" "),
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(query);
    });

  const totalCount = items.length;
  const nfoCount = items.filter((item) => item.has_nfo).length;
  const artworkCount = items.filter((item) => item.poster_path || item.cover_path).length;
  const dirty = detail ? serializeMeta(draftMeta) !== serializeMeta(cloneMeta(detail.meta)) : false;
  const currentVariant = pickVariant(detail, selectedVariantKey);
  const showVariantSwitch = (detail?.variants.length ?? 0) > 1;
  const activeTitleValue = copyMode === "translated" ? draftMeta.title_translated : draftMeta.title;
  const activePlotValue = copyMode === "translated" ? draftMeta.plot_translated : draftMeta.plot;
  const fanartFiles = detail?.files.filter((file) => file.rel_path.includes("/extrafanart/")) ?? [];
  const selectedPoster = currentVariant?.poster_path || currentVariant?.meta.poster_path || draftMeta.poster_path || detail?.item.poster_path || "";
  const selectedCover =
    currentVariant?.cover_path ||
    currentVariant?.meta.cover_path ||
    currentVariant?.meta.fanart_path ||
    currentVariant?.meta.thumb_path ||
    draftMeta.cover_path ||
    draftMeta.fanart_path ||
    detail?.item.cover_path ||
    "";

  useEffect(() => {
    if (!message || /失败|error/i.test(message)) {
      return;
    }
    const timer = window.setTimeout(() => setMessage(""), 2400);
    return () => window.clearTimeout(timer);
  }, [message]);

  useEffect(() => {
    if (!detail && items.length > 0 && selectedPath) {
      loadDetail(selectedPath);
    }
  }, [detail, items.length, selectedPath]);

  const syncDetail = (next: LibraryDetail) => {
    setDetail(next);
    setSelectedPath(next.item.rel_path);
    setDraftMeta(cloneMeta(next.meta));
    setCopyMode((current) => (current === "translated" && !hasTranslatedCopy(next.meta) ? "original" : current || "original"));
    setSelectedVariantKey((current) => {
      if (current && next.variants.some((item) => item.key === current)) {
        return current;
      }
      return next.primary_variant_key || next.variants[0]?.key || "";
    });
  };

  const persistMeta = (meta: LibraryMeta, messageText: string) => {
    if (!detail) {
      return;
    }
    startTransition(async () => {
      try {
        const next = await updateLibraryItem(detail.item.rel_path, normalizeMeta(meta));
        syncDetail(next);
        setItems((prev) => prev.map((item) => (item.rel_path === next.item.rel_path ? next.item : item)));
        setMessage(messageText);
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "保存 NFO 失败");
      }
    });
  };

  const loadDetail = (path: string) => {
    setSelectedPath(path);
    startTransition(async () => {
      try {
        setMessage("加载已入库详情...");
        const next = await getLibraryItem(path);
        syncDetail(next);
        setMessage("");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "加载已入库详情失败");
      }
    });
  };

  const handleRefreshLibrary = () => {
    startTransition(async () => {
      try {
        setMessage("重新扫描已入库目录...");
        const nextItems = await listLibraryItems();
        setItems(nextItems);
        if (nextItems.length === 0) {
          setDetail(null);
          setDraftMeta(cloneMeta(null));
          setSelectedPath("");
          setSelectedVariantKey("");
          setMessage("当前 savedir 里还没有已入库内容");
          return;
        }
        const nextSelected = nextItems.some((item) => item.rel_path === selectedPath) ? selectedPath : nextItems[0].rel_path;
        const nextDetail = await getLibraryItem(nextSelected);
        syncDetail(nextDetail);
        setMessage("已刷新 savedir");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "刷新已入库目录失败");
      }
    });
  };

  const handleManualSave = () => {
    if (!detail || !dirty || isPending) {
      return;
    }
    persistMeta(draftMeta, "已保存");
  };

  const openUploadPicker = (kind: "poster" | "cover") => {
    if (!detail) {
      return;
    }
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*";
    input.addEventListener("change", () => {
      const file = input.files?.[0] ?? null;
      if (!file) {
        return;
      }
      startTransition(async () => {
        try {
          setMessage(kind === "poster" ? "替换当前实例海报..." : "替换当前实例封面...");
          const next = await replaceLibraryAsset(detail.item.rel_path, currentVariant?.key ?? "", kind, file);
          syncDetail(next);
          setItems((prev) => prev.map((item) => (item.rel_path === next.item.rel_path ? next.item : item)));
          setMessage(kind === "poster" ? "当前实例海报已更新" : "当前实例封面已更新");
        } catch (error) {
          setMessage(error instanceof Error ? error.message : "替换图片失败");
        }
      });
    }, { once: true });
    input.click();
  };

  return (
    <div className="library-shell">
      <section className="panel library-list-panel">
        <div className="library-list-head">
          <div className="library-list-kicker">Saved Library</div>
          <h2 className="library-list-title">已入库</h2>
          <p className="library-list-subtitle">浏览 `savedir` 下的媒体目录，并直接修改目录内全部 NFO 的共享元数据。</p>
        </div>

        <div className="library-list-actions">
          <button className="btn" type="button" onClick={handleRefreshLibrary} disabled={isPending}>
            <RefreshCw size={16} />
            重新扫描库
          </button>
        </div>

        <div className="library-stat-row">
          <div className="library-stat-card">
            <span className="library-stat-label">目录总数</span>
            <strong className="library-stat-value">{totalCount}</strong>
            <span className="library-stat-hint">按电影目录聚合</span>
          </div>
          <div className="library-stat-card">
            <span className="library-stat-label">NFO 完整</span>
            <strong className="library-stat-value">{nfoCount}</strong>
            <span className="library-stat-hint">可直接编辑元数据</span>
          </div>
          <div className="library-stat-card">
            <span className="library-stat-label">有封面</span>
            <strong className="library-stat-value">{artworkCount}</strong>
            <span className="library-stat-hint">支持缩略图预览</span>
          </div>
        </div>

        <label className="file-list-search library-search">
          <Search size={16} />
          <input
            className="input file-list-search-input"
            placeholder="按标题 / 番号 / 演员 / 路径搜索"
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
                    <img src={getLibraryFileURL(imagePath)} alt={item.title} className="library-thumb-image" />
                  ) : (
                    <div className="library-thumb-fallback">{(item.number || item.title || item.name).slice(0, 2).toUpperCase()}</div>
                  )}
                </div>
                <div className="library-item-copy">
                  <div className="library-item-topline">
                    <span className="library-item-number">{item.number || "未命名番号"}</span>
                    <span className="library-item-time">{formatUnixMillis(item.updated_at)}</span>
                  </div>
                  <div className="library-item-title">{item.title || item.name}</div>
                  <div className="library-item-meta">{item.actors.length > 0 ? item.actors.join(" / ") : "暂无演员信息"}</div>
                  <div className="library-item-path">{item.rel_path}</div>
                  <div className="library-item-footnote">{item.variant_count > 1 ? `${item.variant_count} 个文件实例` : "单实例目录"}</div>
                </div>
              </button>
            );
          })}
          {filteredItems.length === 0 ? <div className="review-empty-state">没有匹配的已入库项目</div> : null}
        </div>
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
                {message ? (
                  <span className="review-message" data-tone={/失败|error/i.test(message) ? "danger" : "info"}>
                    {message}
                  </span>
                ) : null}
                <button
                  type="button"
                  className="btn review-inline-btn"
                  onClick={handleManualSave}
                  disabled={!dirty || isPending}
                >
                  <Check size={14} />
                  保存
                </button>
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
                  <div className="library-info-chip">目录级字段保存时会同步写入全部实例 NFO</div>
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
                          setDraftMeta((prev) => ({
                            ...prev,
                            [copyMode === "translated" ? "title_translated" : "title"]: e.target.value,
                          }))
                        }
                      />
                    </div>
                    <div className="review-meta-row review-meta-row-2 review-meta-row-top">
                      <div className="review-field">
                        <span className="review-label review-label-side">导演</span>
                        <input
                          className="input"
                          value={draftMeta.director}

                          onChange={(e) => setDraftMeta((prev) => ({ ...prev, director: e.target.value }))}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">片商</span>
                        <input
                          className="input"
                          value={draftMeta.studio}

                          onChange={(e) => setDraftMeta((prev) => ({ ...prev, studio: e.target.value }))}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">发行商</span>
                        <input
                          className="input"
                          value={draftMeta.label}

                          onChange={(e) => setDraftMeta((prev) => ({ ...prev, label: e.target.value }))}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">系列</span>
                        <input
                          className="input"
                          value={draftMeta.series}

                          onChange={(e) => setDraftMeta((prev) => ({ ...prev, series: e.target.value }))}
                        />
                      </div>
                    </div>
                    <div className="review-meta-row review-meta-row-2 library-meta-grid">
                      <div className="review-field">
                        <span className="review-label review-label-side">番号</span>
                        <input
                          className="input"
                          value={draftMeta.number}

                          onChange={(e) => setDraftMeta((prev) => ({ ...prev, number: e.target.value }))}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">发行日期</span>
                        <input
                          className="input"
                          placeholder="YYYY-MM-DD"
                          value={draftMeta.release_date}

                          onChange={(e) => setDraftMeta((prev) => ({ ...prev, release_date: e.target.value }))}
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">时长</span>
                        <input
                          className="input"
                          inputMode="numeric"
                          value={draftMeta.runtime ? String(draftMeta.runtime) : ""}

                          onChange={(e) =>
                            setDraftMeta((prev) => ({ ...prev, runtime: Number.parseInt(e.target.value || "0", 10) || 0 }))
                          }
                        />
                      </div>
                      <div className="review-field">
                        <span className="review-label review-label-side">来源</span>
                        <input
                          className="input"
                          value={draftMeta.source}

                          onChange={(e) => setDraftMeta((prev) => ({ ...prev, source: e.target.value }))}
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
                            setDraftMeta((prev) => ({
                              ...prev,
                              [copyMode === "translated" ? "plot_translated" : "plot"]: e.target.value,
                            }))
                          }
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
                        onChange={(next) => setDraftMeta((prev) => ({ ...prev, actors: next }))}
                        onBlurSave={() => { }} singleLine
                      />
                    </div>
                  </div>
                  <div className="panel review-image-card review-image-card-poster review-top-poster review-main-poster">
                    <div className="review-image-card-head">
                      <span className="review-image-title">海报</span>
                      <button className="btn review-inline-btn" type="button" onClick={() => openUploadPicker("poster")} disabled={isPending}>
                        <Upload size={14} />
                        替换当前实例
                      </button>
                    </div>
                    <div className="review-image-box review-image-box-poster">
                      {selectedPoster ? (
                        <button type="button" className="review-image-hit" onClick={() => setPreview({ title: "海报", path: selectedPoster, name: "海报" })}>
                          <img src={getLibraryFileURL(selectedPoster)} alt="海报" className="library-poster-image" />
                        </button>
                      ) : (
                        <div className="library-preview-empty">暂无海报</div>
                      )}
                    </div>
                  </div>
                </div>

                <div className="review-meta-row review-meta-row-full">
                  <TokenEditor
                    label="标签"
                    placeholder="输入后回车或逗号确认"
                    value={draftMeta.genres}
                    onChange={(next) => setDraftMeta((prev) => ({ ...prev, genres: next }))}
                    onBlurSave={() => { }}
                  />
                </div>

                <div className="review-media-offset review-cover-slot">
                  <div className="panel review-image-card review-image-card-cover">
                    <div className="review-image-card-head">
                      <span className="review-image-title">封面</span>
                      <button className="btn review-inline-btn" type="button" onClick={() => openUploadPicker("cover")} disabled={isPending}>
                        <Upload size={14} />
                        替换当前实例
                      </button>
                    </div>
                    <div className="review-image-box review-image-box-cover">
                      {selectedCover ? (
                        <button type="button" className="review-image-hit" onClick={() => setPreview({ title: "封面", path: selectedCover, name: "封面" })}>
                          <img src={getLibraryFileURL(selectedCover)} alt="封面" className="library-cover-image" />
                        </button>
                      ) : (
                        <div className="library-preview-empty">暂无封面</div>
                      )}
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
                              onClick={() => setPreview({ title: "Extrafanart", path: file.rel_path, name: file.name })}
                            >
                              <img src={getLibraryFileURL(file.rel_path)} alt={file.name} className="library-fanart-image" />
                            </button>
                            <div className="library-fanart-name">{file.name.split("/").pop()}</div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="review-empty-state library-inline-empty">当前目录没有 extrafanart</div>
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
                src={getLibraryFileURL(preview.path)}
                alt={preview.name}
                style={{ width: "100%", height: "100%", objectFit: "contain", objectPosition: "center", display: "block" }}
              />
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
