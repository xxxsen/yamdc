// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { MediaLibraryFilterRail } from "../filter-rail";

afterEach(() => {
  cleanup();
});

function baseProps(overrides: Partial<React.ComponentProps<typeof MediaLibraryFilterRail>> = {}) {
  return {
    keyword: "",
    onKeywordChange: vi.fn(),
    yearFilter: "all",
    onYearFilterChange: vi.fn(),
    visibleYearOptions: ["2024", "2023"],
    overflowYearOptions: ["2010"],
    isOverflowYearSelected: false,
    yearPickerOpen: false,
    onYearPickerToggle: vi.fn(),
    onYearPickerClose: vi.fn(),
    yearPickerRef: createRef<HTMLDivElement>(),
    sizeFilter: "all" as const,
    onSizeFilterChange: vi.fn(),
    sortMode: "ingested" as const,
    onSortModeChange: vi.fn(),
    sortOrder: "desc" as const,
    onSortOrderChange: vi.fn(),
    syncBusy: false,
    syncButtonLabel: "立即同步",
    syncMenuOpen: false,
    onSyncMenuToggle: vi.fn(),
    syncMenuRef: createRef<HTMLDivElement>(),
    onTriggerSync: vi.fn(),
    onOpenSyncLogs: vi.fn(),
    ...overrides,
  };
}

describe("MediaLibraryFilterRail", () => {
  it("正常路径: 渲染搜索框 + 年份/大小/排序 chips + 同步按钮; 关键交互逐个回调", () => {
    const onKeywordChange = vi.fn();
    const onYearFilterChange = vi.fn();
    const onSizeFilterChange = vi.fn();
    const onSortModeChange = vi.fn();
    const onSortOrderChange = vi.fn();
    const onTriggerSync = vi.fn();
    render(
      <MediaLibraryFilterRail
        {...baseProps({
          onKeywordChange,
          onYearFilterChange,
          onSizeFilterChange,
          onSortModeChange,
          onSortOrderChange,
          onTriggerSync,
        })}
      />,
    );
    fireEvent.change(screen.getByPlaceholderText("搜索标题 / 影片 ID"), { target: { value: "abc" } });
    expect(onKeywordChange).toHaveBeenCalledWith("abc");
    fireEvent.click(screen.getByText("2024"));
    expect(onYearFilterChange).toHaveBeenCalledWith("2024");
    fireEvent.click(screen.getByText("1-2 GB"));
    expect(onSizeFilterChange).toHaveBeenCalledWith("1-2");
    // "年份" 在标题 + sort chip 都有, 用 button role 精确锁定 sort chip.
    fireEvent.click(screen.getByRole("button", { name: "年份" }));
    expect(onSortModeChange).toHaveBeenCalledWith("year");
    fireEvent.click(screen.getByRole("button", { name: "顺序" }));
    expect(onSortOrderChange).toHaveBeenCalledWith("asc");
    fireEvent.click(screen.getByRole("button", { name: /立即同步/ }));
    expect(onTriggerSync).toHaveBeenCalled();
  });

  it("异常路径: syncBusy=true → 主同步按钮 disabled, 但下拉箭头仍可用 (允许查看进行中的日志)", () => {
    const onTriggerSync = vi.fn();
    const onSyncMenuToggle = vi.fn();
    render(
      <MediaLibraryFilterRail
        {...baseProps({
          syncBusy: true,
          syncButtonLabel: "同步中...",
          onTriggerSync,
          onSyncMenuToggle,
        })}
      />,
    );
    const syncBtn = screen.getByRole("button", { name: /同步中/ });
    expect(syncBtn.hasAttribute("disabled")).toBe(true);
    fireEvent.click(syncBtn);
    expect(onTriggerSync).not.toHaveBeenCalled();
    const caret = screen.getByRole("button", { name: "同步菜单" });
    expect(caret.hasAttribute("disabled")).toBe(false);
    fireEvent.click(caret);
    expect(onSyncMenuToggle).toHaveBeenCalled();
  });

  it("边缘路径: yearPickerOpen=true → 渲染 popover 含 overflow 年份, 点击关闭 + 切换", () => {
    const onYearFilterChange = vi.fn();
    const onYearPickerClose = vi.fn();
    render(
      <MediaLibraryFilterRail
        {...baseProps({
          overflowYearOptions: ["2010", "2009"],
          yearPickerOpen: true,
          onYearFilterChange,
          onYearPickerClose,
        })}
      />,
    );
    fireEvent.click(screen.getByText("2010"));
    expect(onYearFilterChange).toHaveBeenCalledWith("2010");
    expect(onYearPickerClose).toHaveBeenCalled();
  });

  it("边缘路径: syncMenuOpen=true → 菜单可见, '查看同步日志' 触发 onOpenSyncLogs", () => {
    const onOpenSyncLogs = vi.fn();
    render(<MediaLibraryFilterRail {...baseProps({ syncMenuOpen: true, onOpenSyncLogs })} />);
    fireEvent.click(screen.getByRole("menuitem", { name: "查看同步日志" }));
    expect(onOpenSyncLogs).toHaveBeenCalled();
  });
});
