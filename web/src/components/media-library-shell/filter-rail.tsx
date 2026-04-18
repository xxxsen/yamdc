"use client";

import { ChevronDown, RefreshCw, Search } from "lucide-react";
import { type RefObject } from "react";

import { Button } from "@/components/ui/button";

export type SizeFilter = "all" | "lt-1" | "1-2" | "2-5" | "5-10" | "10-20" | "20-50" | "50-plus";
export type SortMode = "ingested" | "year" | "size" | "title";
export type SortOrder = "desc" | "asc";

const SIZE_OPTIONS: ReadonlyArray<{ value: SizeFilter; label: string }> = [
  { value: "all", label: "全部" },
  { value: "lt-1", label: "< 1 GB" },
  { value: "1-2", label: "1-2 GB" },
  { value: "2-5", label: "2-5 GB" },
  { value: "5-10", label: "5-10 GB" },
  { value: "10-20", label: "10-20 GB" },
  { value: "20-50", label: "20-50 GB" },
  { value: "50-plus", label: "50+ GB" },
];

const SORT_MODE_OPTIONS: ReadonlyArray<{ value: SortMode; label: string }> = [
  { value: "ingested", label: "入库时间" },
  { value: "year", label: "年份" },
  { value: "size", label: "大小" },
  { value: "title", label: "标题" },
];

const SORT_ORDER_OPTIONS: ReadonlyArray<{ value: SortOrder; label: string }> = [
  { value: "desc", label: "逆序" },
  { value: "asc", label: "顺序" },
];

export interface MediaLibraryFilterRailProps {
  keyword: string;
  onKeywordChange: (value: string) => void;

  yearFilter: string;
  onYearFilterChange: (value: string) => void;
  visibleYearOptions: string[];
  overflowYearOptions: string[];
  isOverflowYearSelected: boolean;
  yearPickerOpen: boolean;
  onYearPickerToggle: () => void;
  onYearPickerClose: () => void;
  yearPickerRef: RefObject<HTMLDivElement | null>;

  sizeFilter: SizeFilter;
  onSizeFilterChange: (value: SizeFilter) => void;

  sortMode: SortMode;
  onSortModeChange: (value: SortMode) => void;

  sortOrder: SortOrder;
  onSortOrderChange: (value: SortOrder) => void;

  syncBusy: boolean;
  syncButtonLabel: string;
  syncMenuOpen: boolean;
  onSyncMenuToggle: () => void;
  syncMenuRef: RefObject<HTMLDivElement | null>;
  onTriggerSync: () => void;
  onOpenSyncLogs: () => void;
}

export function MediaLibraryFilterRail({
  keyword,
  onKeywordChange,
  yearFilter,
  onYearFilterChange,
  visibleYearOptions,
  overflowYearOptions,
  isOverflowYearSelected,
  yearPickerOpen,
  onYearPickerToggle,
  onYearPickerClose,
  yearPickerRef,
  sizeFilter,
  onSizeFilterChange,
  sortMode,
  onSortModeChange,
  sortOrder,
  onSortOrderChange,
  syncBusy,
  syncButtonLabel,
  syncMenuOpen,
  onSyncMenuToggle,
  syncMenuRef,
  onTriggerSync,
  onOpenSyncLogs,
}: MediaLibraryFilterRailProps) {
  return (
    <aside className="media-library-filter-rail">
      <div className="media-library-filter-stack">
        <div className="media-library-filter-body">
          <label className="media-library-search-bar media-library-search-bar-rail">
            <Search size={16} />
            <input
              className="media-library-search-input"
              placeholder="搜索标题 / 影片 ID"
              value={keyword}
              onChange={(e) => onKeywordChange(e.target.value)}
            />
          </label>

          <div className="media-library-filter-group">
            <div className="media-library-filter-title">年份</div>
            <div className="media-library-filter-chips media-library-filter-chips-years" ref={yearPickerRef}>
              <button type="button" className="media-library-filter-chip" data-active={yearFilter === "all"} onClick={() => onYearFilterChange("all")}>
                全部
              </button>
              {visibleYearOptions.map((year) => (
                <button key={year} type="button" className="media-library-filter-chip" data-active={yearFilter === year} onClick={() => onYearFilterChange(year)}>
                  {year}
                </button>
              ))}
              {overflowYearOptions.length > 0 ? (
                <div className="media-library-year-overflow">
                  <button
                    type="button"
                    className="media-library-filter-chip"
                    data-active={isOverflowYearSelected || yearPickerOpen}
                    onClick={onYearPickerToggle}
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
                              onYearFilterChange(year);
                              onYearPickerClose();
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
              {SIZE_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  className="media-library-filter-chip"
                  data-active={sizeFilter === option.value}
                  onClick={() => onSizeFilterChange(option.value)}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          <div className="media-library-filter-group">
            <div className="media-library-filter-title">排序</div>
            <div className="media-library-filter-chips">
              {SORT_MODE_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  className="media-library-filter-chip"
                  data-active={sortMode === option.value}
                  onClick={() => onSortModeChange(option.value)}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          <div className="media-library-filter-group">
            <div className="media-library-filter-title">排序顺序</div>
            <div className="media-library-filter-chips">
              {SORT_ORDER_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  className="media-library-filter-chip"
                  data-active={sortOrder === option.value}
                  onClick={() => onSortOrderChange(option.value)}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="media-library-filter-footer">
          {/* split button: 主按钮触发同步, 右侧下拉箭头打开菜单,
              目前唯一菜单项是 "查看同步日志"。把两个按钮放到同一
              视觉容器里, 避免让用户困惑 "多出来的按钮在做什么"。 */}
          <div className="media-library-sync-split" ref={syncMenuRef}>
            <Button
              variant="primary"
              className="media-library-sync-btn"
              disabled={syncBusy}
              leftIcon={
                <RefreshCw size={16} className={syncBusy ? "media-library-sync-icon-spinning" : ""} />
              }
              onClick={onTriggerSync}
            >
              {syncButtonLabel}
            </Button>
            {/* 下拉按钮不跟随 disabled: 用户可能想在同步进行中
                看历史日志 / 当前 run 进度, 不该被主按钮的 busy
                状态拦住。 */}
            <Button
              variant="primary"
              className="media-library-sync-caret"
              aria-label="同步菜单"
              aria-haspopup="menu"
              aria-expanded={syncMenuOpen}
              onClick={onSyncMenuToggle}
            >
              <ChevronDown size={14} />
            </Button>
            {syncMenuOpen ? (
              <div className="media-library-sync-menu panel" role="menu">
                <button
                  type="button"
                  role="menuitem"
                  className="media-library-sync-menu-item"
                  onClick={onOpenSyncLogs}
                >
                  查看同步日志
                </button>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </aside>
  );
}
