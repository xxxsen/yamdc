"use client";

import { useDeferredValue, useEffect, useRef, useState } from "react";

import { MediaLibraryDetailShell } from "@/components/media-library-detail-shell";
import { MediaLibraryCardGrid } from "@/components/media-library-shell/card-grid";
import {
  MediaLibraryFilterRail,
  type SizeFilter,
  type SortMode,
  type SortOrder,
} from "@/components/media-library-shell/filter-rail";
import { MediaLibrarySyncLogsModal } from "@/components/media-library-shell/sync-logs-modal";
import { useMediaLibraryDetail } from "@/components/media-library-shell/use-media-library-detail";
import { useMediaLibrarySync } from "@/components/media-library-shell/use-media-library-sync";
import { extractYearOptions } from "@/components/media-library-shell/utils";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Modal } from "@/components/ui/modal";
import { Spinner } from "@/components/ui/spinner";
import type { MediaLibraryItem, MediaLibraryStatus, MediaLibrarySyncLogEntry } from "@/lib/api";
import { listMediaLibrarySyncLogs } from "@/lib/api";

interface Props {
  items: MediaLibraryItem[];
  initialStatus: MediaLibraryStatus | null;
}

const PAGE_SIZE = 30;

export function MediaLibraryShell({ items: initialItems, initialStatus }: Props) {
  const [items, setItems] = useState(initialItems);
  const [yearOptions, setYearOptions] = useState(() => extractYearOptions(initialItems));
  const [keyword, setKeyword] = useState("");
  const [yearFilter, setYearFilter] = useState("all");
  const [sizeFilter, setSizeFilter] = useState<SizeFilter>("all");
  const [sortMode, setSortMode] = useState<SortMode>("ingested");
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc");
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);
  const [yearPickerOpen, setYearPickerOpen] = useState(false);
  const [syncMenuOpen, setSyncMenuOpen] = useState(false);
  const [syncLogsOpen, setSyncLogsOpen] = useState(false);
  const [syncLogs, setSyncLogs] = useState<MediaLibrarySyncLogEntry[]>([]);
  const [syncLogsLoading, setSyncLogsLoading] = useState(false);
  const [syncLogsError, setSyncLogsError] = useState("");

  const browserRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const yearPickerRef = useRef<HTMLDivElement | null>(null);
  const syncMenuRef = useRef<HTMLDivElement | null>(null);
  const deferredKeyword = useDeferredValue(keyword);

  const {
    configured,
    syncBusy,
    syncButtonLabel,
    syncMessage,
    handleTriggerSync: triggerSync,
  } = useMediaLibrarySync({
    initialStatus,
    deferredKeyword,
    yearFilter,
    sizeFilter,
    sortMode,
    sortOrder,
    setItems,
    setYearOptions,
  });

  const {
    activeDetail,
    activeDetailID,
    detailLoading,
    detailError,
    openDetailModal,
    closeDetailModal,
    applyDetailChange,
  } = useMediaLibraryDetail({ setItems });

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
    if (!syncMenuOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (!syncMenuRef.current?.contains(event.target as Node)) {
        setSyncMenuOpen(false);
      }
    };
    window.addEventListener("mousedown", handlePointerDown);
    return () => window.removeEventListener("mousedown", handlePointerDown);
  }, [syncMenuOpen]);

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

  // 每次打开日志弹窗都重新拉最新数据, 而不是 "一次加载一直复用"。理由:
  // sync 是异步后台跑的, 用户可能在 sync 运行中点开弹窗看进度; 缓存数据
  // 只会让用户看到过时的状态。200 条默认量级下一次拉取 < 30KB, 没必要省。
  const openSyncLogs = () => {
    setSyncMenuOpen(false);
    setSyncLogsOpen(true);
    setSyncLogsError("");
    setSyncLogsLoading(true);
    void (async () => {
      try {
        const entries = await listMediaLibrarySyncLogs(200);
        setSyncLogs(entries);
      } catch (error) {
        setSyncLogsError(error instanceof Error ? error.message : "加载同步日志失败");
        setSyncLogs([]);
      } finally {
        setSyncLogsLoading(false);
      }
    })();
  };

  const closeSyncLogs = () => {
    setSyncLogsOpen(false);
    setSyncLogsError("");
  };

  const handleTriggerSync = () => {
    setSyncMenuOpen(false);
    triggerSync();
  };

  return (
    <div className="media-library-page media-library-page-wide">
      <section className="panel media-library-overview media-library-overview-wide">
        {!configured ? (
          <EmptyState title="当前还没有配置 `library_dir`，媒体库页面暂不可用。" />
        ) : (
          <div className="media-library-browser-shell">
            <MediaLibraryFilterRail
              keyword={keyword}
              onKeywordChange={(value) => {
                setKeyword(value);
                resetViewport();
              }}
              yearFilter={yearFilter}
              onYearFilterChange={(value) => {
                setYearFilter(value);
                resetViewport();
              }}
              visibleYearOptions={visibleYearOptions}
              overflowYearOptions={overflowYearOptions}
              isOverflowYearSelected={isOverflowYearSelected}
              yearPickerOpen={yearPickerOpen}
              onYearPickerToggle={() => setYearPickerOpen((current) => !current)}
              onYearPickerClose={() => setYearPickerOpen(false)}
              yearPickerRef={yearPickerRef}
              sizeFilter={sizeFilter}
              onSizeFilterChange={(value) => {
                setSizeFilter(value);
                resetViewport();
              }}
              sortMode={sortMode}
              onSortModeChange={(value) => {
                setSortMode(value);
                resetViewport();
              }}
              sortOrder={sortOrder}
              onSortOrderChange={(value) => {
                setSortOrder(value);
                resetViewport();
              }}
              syncBusy={syncBusy}
              syncButtonLabel={syncButtonLabel}
              syncMenuOpen={syncMenuOpen}
              onSyncMenuToggle={() => setSyncMenuOpen((current) => !current)}
              syncMenuRef={syncMenuRef}
              onTriggerSync={handleTriggerSync}
              onOpenSyncLogs={openSyncLogs}
            />

            <MediaLibraryCardGrid
              visibleItems={visibleItems}
              itemsTotal={items.length}
              filteredTotal={filteredItems.length}
              browserRef={browserRef}
              loadMoreRef={loadMoreRef}
              showLoadMoreSentinel={filteredItems.length > visibleItems.length}
              onOpenDetail={openDetailModal}
            />
          </div>
        )}
      </section>

      <Modal
        open={activeDetailID !== null}
        onClose={closeDetailModal}
        bare
        backdropClassName="media-library-detail-modal"
        frameClassName="media-library-detail-modal-frame"
        ariaLabel="媒体项详情"
      >
        {detailLoading ? (
          <div className="media-library-detail-modal-state panel">
            <Spinner />
          </div>
        ) : detailError ? (
          <div className="media-library-detail-modal-state panel">
            <ErrorState title="加载详情失败" detail={detailError} />
          </div>
        ) : activeDetail ? (
          <MediaLibraryDetailShell
            initialDetail={activeDetail}
            stageOnly
            onDetailChange={applyDetailChange}
          />
        ) : null}
      </Modal>
      <MediaLibrarySyncLogsModal
        open={syncLogsOpen}
        onClose={closeSyncLogs}
        loading={syncLogsLoading}
        error={syncLogsError}
        logs={syncLogs}
      />
      {syncMessage ? (
        <div className="app-toast app-toast-top" data-tone={/失败|error/i.test(syncMessage) ? "danger" : undefined} role="status" aria-live="polite">
          {syncMessage}
        </div>
      ) : null}
    </div>
  );
}
