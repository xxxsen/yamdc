"use client";

import { RefreshCw, Search } from "lucide-react";
import { useDeferredValue, useEffect, useEffectEvent, useRef, useState } from "react";

import { MediaLibraryDetailShell } from "@/components/media-library-detail-shell";
import type { MediaLibraryDetail, MediaLibraryItem, MediaLibraryStatus } from "@/lib/api";
import { getMediaLibraryFileURL, getMediaLibraryItem, getMediaLibraryStatus, listMediaLibraryItems, triggerMediaLibrarySync } from "@/lib/api";

interface Props {
  items: MediaLibraryItem[];
  initialStatus: MediaLibraryStatus | null;
}

type SizeFilter = "all" | "lt-1" | "1-2" | "2-5" | "5-10" | "10-20" | "20-50" | "50-plus";
type SortMode = "ingested" | "year" | "size" | "title";
type SortOrder = "desc" | "asc";

const PAGE_SIZE = 30;

function getReleaseYear(value: string) {
  const match = value.match(/\d{4}/);
  return match?.[0] ?? "";
}

function extractYearOptions(items: MediaLibraryItem[]) {
  const seenYears = new Set<string>();
  for (const item of items) {
    const year = getReleaseYear(item.release_date);
    if (year) {
      seenYears.add(year);
    }
  }
  return Array.from(seenYears).sort((left, right) => Number.parseInt(right, 10) - Number.parseInt(left, 10));
}

function mergeYearOptions(current: string[], next: string[]) {
  return Array.from(new Set([...current, ...next])).sort((left, right) => Number.parseInt(right, 10) - Number.parseInt(left, 10));
}

function toMediaLibrarySyncMessage(error: unknown) {
  const raw = error instanceof Error ? error.message : "启动媒体库同步失败";
  const text = raw.trim();
  if (text.includes("media library sync is already running")) {
    return "媒体库正在同步中";
  }
  if (text.includes("move to media library is running")) {
    return "媒体库移动任务进行中，暂时无法同步";
  }
  if (text.includes("library dir is not configured")) {
    return "未配置媒体库目录";
  }
  return raw;
}

export function MediaLibraryShell({ items: initialItems, initialStatus }: Props) {
  const [items, setItems] = useState(initialItems);
  const [yearOptions, setYearOptions] = useState(() => extractYearOptions(initialItems));
  const [configured, setConfigured] = useState(Boolean(initialStatus?.configured));
  const [syncRunning, setSyncRunning] = useState(initialStatus?.sync.status === "running");
  const [syncStarting, setSyncStarting] = useState(false);
  const [keyword, setKeyword] = useState("");
  const [yearFilter, setYearFilter] = useState("all");
  const [sizeFilter, setSizeFilter] = useState<SizeFilter>("all");
  const [sortMode, setSortMode] = useState<SortMode>("ingested");
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc");
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);
  const [yearPickerOpen, setYearPickerOpen] = useState(false);
  const [syncMessage, setSyncMessage] = useState("");
  const [syncCompletedFlash, setSyncCompletedFlash] = useState(false);
  const [activeDetail, setActiveDetail] = useState<MediaLibraryDetail | null>(null);
  const [activeDetailID, setActiveDetailID] = useState<number | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState("");

  const browserRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const yearPickerRef = useRef<HTMLDivElement | null>(null);
  const prevSyncRunningRef = useRef(initialStatus?.sync.status === "running");
  const observedSyncRunningRef = useRef(initialStatus?.sync.status === "running");
  const deferredKeyword = useDeferredValue(keyword);

  const resetViewport = () => {
    setVisibleCount(PAGE_SIZE);
    browserRef.current?.scrollTo({ top: 0 });
  };

  useEffect(() => {
    if (!yearPickerOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (!yearPickerRef.current?.contains(event.target as Node)) {
        setYearPickerOpen(false);
      }
    };
    window.addEventListener("mousedown", handlePointerDown);
    return () => window.removeEventListener("mousedown", handlePointerDown);
  }, [yearPickerOpen]);

  useEffect(() => {
    if (!activeDetail && activeDetailID === null) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setActiveDetail(null);
        setActiveDetailID(null);
        setDetailError("");
        setDetailLoading(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeDetail, activeDetailID]);

  const refreshItems = useEffectEvent(async (nextParams?: {
    keyword?: string;
    year?: string;
    size?: string;
    sort?: string;
    order?: string;
  }) => {
    try {
      const next = await listMediaLibraryItems(nextParams);
      setItems(next);
      if (!nextParams?.year || nextParams.year === "all") {
        setYearOptions((current) => mergeYearOptions(current, extractYearOptions(next)));
      }
    } catch {
      // ignore polling errors
    }
  });

  const refreshStatus = useEffectEvent(async () => {
    try {
      const next = await getMediaLibraryStatus();
      setConfigured(Boolean(next.configured));
      const nextSyncRunning = next.sync.status === "running";
      setSyncRunning(nextSyncRunning);
      if (nextSyncRunning) {
        setSyncStarting(false);
        observedSyncRunningRef.current = true;
      }
      if (observedSyncRunningRef.current && prevSyncRunningRef.current && !nextSyncRunning) {
        setSyncCompletedFlash(true);
        observedSyncRunningRef.current = false;
        const nextItems = await listMediaLibraryItems({
          keyword: deferredKeyword,
          year: yearFilter,
          size: sizeFilter,
          sort: sortMode,
          order: sortOrder,
        });
        setItems(nextItems);
      }
      prevSyncRunningRef.current = nextSyncRunning;
    } catch {
      // ignore polling errors
    }
  });

  useEffect(() => {
    if (!syncMessage) {
      return;
    }
    const timer = window.setTimeout(() => setSyncMessage(""), 2400);
    return () => window.clearTimeout(timer);
  }, [syncMessage]);

  useEffect(() => {
    if (!syncCompletedFlash) {
      return;
    }
    const timer = window.setTimeout(() => setSyncCompletedFlash(false), 1000);
    return () => window.clearTimeout(timer);
  }, [syncCompletedFlash]);

  useEffect(() => {
    void refreshStatus();
  }, []);

  useEffect(() => {
    if (!configured) {
      return;
    }
    const params = {
      keyword: deferredKeyword,
      year: yearFilter,
      size: sizeFilter,
      sort: sortMode,
      order: sortOrder,
    };
    void refreshItems(params);
  }, [configured, deferredKeyword, yearFilter, sizeFilter, sortMode, sortOrder]);

  const syncBusy = syncStarting || syncRunning;
  const syncButtonLabel = syncBusy ? "同步中..." : syncCompletedFlash ? "同步完成" : "同步媒体库";

  useEffect(() => {
    if (!syncBusy) {
      return;
    }
    const timer = window.setInterval(() => {
      void refreshStatus();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [syncBusy]);

  const filteredItems = items;
  const visibleYearOptions = yearOptions.slice(0, 13);
  const overflowYearOptions = yearOptions.slice(13);
  const isOverflowYearSelected = yearFilter !== "all" && overflowYearOptions.includes(yearFilter);

  useEffect(() => {
    const root = browserRef.current;
    const target = loadMoreRef.current;
    if (!root || !target || visibleCount >= filteredItems.length) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries[0]?.isIntersecting) {
          return;
        }
        setVisibleCount((current) => Math.min(current + PAGE_SIZE, filteredItems.length));
      },
      {
        root,
        rootMargin: "560px 0px",
      },
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [filteredItems.length, visibleCount]);

  const visibleItems = filteredItems.slice(0, visibleCount);

  const closeDetailModal = () => {
    setActiveDetail(null);
    setActiveDetailID(null);
    setDetailError("");
    setDetailLoading(false);
  };

  const openDetailModal = (id: number) => {
    setActiveDetailID(id);
    setActiveDetail(null);
    setDetailError("");
    setDetailLoading(true);
    void (async () => {
      try {
        const next = await getMediaLibraryItem(id);
        setActiveDetail(next);
      } catch (error) {
        setDetailError(error instanceof Error ? error.message : "加载媒体详情失败");
      } finally {
        setDetailLoading(false);
      }
    })();
  };

  return (
    <div className="media-library-page media-library-page-wide">
      <section className="panel media-library-overview media-library-overview-wide">
        {!configured ? (
          <div className="review-empty-state">当前还没有配置 `library_dir`，媒体库页面暂不可用。</div>
        ) : (
          <div className="media-library-browser-shell">
            <aside className="media-library-filter-rail">
              <div className="media-library-filter-stack">
                <div className="media-library-filter-body">
                  <label className="media-library-search-bar media-library-search-bar-rail">
                    <Search size={16} />
                    <input
                      className="media-library-search-input"
                      placeholder="搜索标题 / 影片 ID"
                      value={keyword}
                      onChange={(e) => {
                        setKeyword(e.target.value);
                        resetViewport();
                      }}
                    />
                  </label>

                  <div className="media-library-filter-group">
                    <div className="media-library-filter-title">年份</div>
                    <div className="media-library-filter-chips media-library-filter-chips-years" ref={yearPickerRef}>
                      <button type="button" className="media-library-filter-chip" data-active={yearFilter === "all"} onClick={() => { setYearFilter("all"); resetViewport(); }}>
                        全部
                      </button>
                      {visibleYearOptions.map((year) => (
                        <button key={year} type="button" className="media-library-filter-chip" data-active={yearFilter === year} onClick={() => { setYearFilter(year); resetViewport(); }}>
                          {year}
                        </button>
                      ))}
                      {overflowYearOptions.length > 0 ? (
                        <div className="media-library-year-overflow">
                          <button
                            type="button"
                            className="media-library-filter-chip"
                            data-active={isOverflowYearSelected || yearPickerOpen}
                            onClick={() => setYearPickerOpen((current) => !current)}
                          >
                            其他
                          </button>
                          {yearPickerOpen ? (
                            <div className="media-library-year-popover panel">
                              <div className="media-library-year-popover-grid">
                                {overflowYearOptions.map((year) => (
                                  <button
                                    key={year}
                                    type="button"
                                    className="media-library-filter-chip"
                                    data-active={yearFilter === year}
                                    onClick={() => {
                                      setYearFilter(year);
                                      setYearPickerOpen(false);
                                      resetViewport();
                                    }}
                                  >
                                    {year}
                                  </button>
                                ))}
                              </div>
                            </div>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                  </div>

                  <div className="media-library-filter-group">
                    <div className="media-library-filter-title">文件大小</div>
                    <div className="media-library-filter-chips">
                      <button type="button" className="media-library-filter-chip" data-active={sizeFilter === "all"} onClick={() => { setSizeFilter("all"); resetViewport(); }}>
                        全部
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sizeFilter === "lt-1"} onClick={() => { setSizeFilter("lt-1"); resetViewport(); }}>
                        &lt; 1 GB
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sizeFilter === "1-2"} onClick={() => { setSizeFilter("1-2"); resetViewport(); }}>
                        1-2 GB
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sizeFilter === "2-5"} onClick={() => { setSizeFilter("2-5"); resetViewport(); }}>
                        2-5 GB
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sizeFilter === "5-10"} onClick={() => { setSizeFilter("5-10"); resetViewport(); }}>
                        5-10 GB
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sizeFilter === "10-20"} onClick={() => { setSizeFilter("10-20"); resetViewport(); }}>
                        10-20 GB
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sizeFilter === "20-50"} onClick={() => { setSizeFilter("20-50"); resetViewport(); }}>
                        20-50 GB
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sizeFilter === "50-plus"} onClick={() => { setSizeFilter("50-plus"); resetViewport(); }}>
                        50+ GB
                      </button>
                    </div>
                  </div>

                  <div className="media-library-filter-group">
                    <div className="media-library-filter-title">排序</div>
                    <div className="media-library-filter-chips">
                      <button type="button" className="media-library-filter-chip" data-active={sortMode === "ingested"} onClick={() => { setSortMode("ingested"); resetViewport(); }}>
                        入库时间
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sortMode === "year"} onClick={() => { setSortMode("year"); resetViewport(); }}>
                        年份
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sortMode === "size"} onClick={() => { setSortMode("size"); resetViewport(); }}>
                        大小
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sortMode === "title"} onClick={() => { setSortMode("title"); resetViewport(); }}>
                        标题
                      </button>
                    </div>
                  </div>

                  <div className="media-library-filter-group">
                    <div className="media-library-filter-title">排序顺序</div>
                    <div className="media-library-filter-chips">
                      <button type="button" className="media-library-filter-chip" data-active={sortOrder === "desc"} onClick={() => { setSortOrder("desc"); resetViewport(); }}>
                        逆序
                      </button>
                      <button type="button" className="media-library-filter-chip" data-active={sortOrder === "asc"} onClick={() => { setSortOrder("asc"); resetViewport(); }}>
                        顺序
                      </button>
                    </div>
                  </div>
                </div>

                <div className="media-library-filter-footer">
                  <button
                    type="button"
                    className="btn btn-primary media-library-sync-btn"
                    disabled={syncBusy}
                    onClick={() => {
                      setSyncCompletedFlash(false);
                      setSyncMessage("媒体库同步已启动");
                      setSyncStarting(true);
                      void (async () => {
                        try {
                          await triggerMediaLibrarySync();
                          setSyncRunning(true);
                          setSyncStarting(false);
                          observedSyncRunningRef.current = true;
                          prevSyncRunningRef.current = true;
                        } catch (error) {
                          const message = toMediaLibrarySyncMessage(error);
                          setSyncMessage(message);
                          if (message === "媒体库正在同步中") {
                            setSyncStarting(false);
                            setSyncRunning(true);
                            prevSyncRunningRef.current = true;
                            observedSyncRunningRef.current = true;
                            return;
                          }
                          setSyncStarting(false);
                          setSyncRunning(false);
                          prevSyncRunningRef.current = false;
                        }
                      })();
                    }}
                  >
                    <RefreshCw size={16} className={syncBusy ? "media-library-sync-icon-spinning" : ""} />
                    {syncButtonLabel}
                  </button>
                </div>
              </div>
            </aside>

            <div className="media-library-browser-content" ref={browserRef}>
              <div className="media-library-card-grid media-library-card-grid-wide">
                {visibleItems.map((item) => {
                  const posterPath = item.poster_path || item.cover_path;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      className="media-library-card media-library-card-wide media-library-card-button"
                      onClick={() => openDetailModal(item.id)}
                    >
                      <div className="media-library-card-poster">
                        {posterPath ? (
                          <img src={getMediaLibraryFileURL(posterPath)} alt={item.title || item.name} className="media-library-card-image" />
                        ) : (
                          <div className="library-thumb-fallback">{(item.number || item.title || item.name).slice(0, 2).toUpperCase()}</div>
                        )}
                      </div>
                      <div className="media-library-card-copy">
                        <div className="media-library-card-title media-library-card-title-only">{item.title || item.name}</div>
                        <div className="media-library-card-meta">
                          <div className="library-item-number">{item.number || "未命名影片"}</div>
                          <div className="media-library-card-year">{getReleaseYear(item.release_date) || "----"}</div>
                        </div>
                      </div>
                    </button>
                  );
                })}

                {items.length === 0 ? <div className="review-empty-state media-library-grid-empty">当前媒体库里还没有项目</div> : null}
                {items.length > 0 && filteredItems.length === 0 ? <div className="review-empty-state media-library-grid-empty">没有匹配的媒体库项目</div> : null}
                {filteredItems.length > visibleItems.length ? <div ref={loadMoreRef} className="media-library-load-sentinel" aria-hidden="true" /> : null}
              </div>
            </div>
          </div>
        )}
      </section>

      {activeDetailID !== null ? (
        <div className="media-library-detail-modal" onClick={closeDetailModal}>
          <div className="media-library-detail-modal-frame" onClick={(event) => event.stopPropagation()}>
            {detailLoading ? (
              <div className="media-library-detail-modal-state panel">
                <div className="list-loading-spinner" aria-hidden="true" />
              </div>
            ) : detailError ? (
              <div className="media-library-detail-modal-state panel">
                <span className="review-message" data-tone="danger">
                  {detailError}
                </span>
              </div>
            ) : activeDetail ? (
              <MediaLibraryDetailShell
                initialDetail={activeDetail}
                stageOnly
                onDetailChange={(next) => {
                  setActiveDetail(next);
                  setItems((current) =>
                    current.map((item) =>
                      item.id === next.item.id
                        ? {
                            ...item,
                            title: next.item.title,
                            number: next.item.number,
                            release_date: next.item.release_date,
                            actors: next.item.actors,
                            updated_at: next.item.updated_at,
                            poster_path: next.item.poster_path,
                            cover_path: next.item.cover_path,
                          }
                        : item,
                    ),
                  );
                }}
              />
            ) : null}
          </div>
        </div>
      ) : null}
      {syncMessage ? (
        <div className="app-toast app-toast-top" data-tone={/失败|error/i.test(syncMessage) ? "danger" : undefined} role="status" aria-live="polite">
          {syncMessage}
        </div>
      ) : null}
    </div>
  );
}
