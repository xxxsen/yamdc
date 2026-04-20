// @vitest-environment jsdom

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createRef } from "react";

import type { JobItem } from "@/lib/api";

import { ReviewListPanel, type ReviewListPanelProps } from "../list-panel";

// ReviewListPanel 这里主要覆盖新增的 overflow (...) 菜单: 把 "打回" / "删除"
// 折到一个点击展开的菜单, 避免每张卡 3 个按钮。其它已有渲染逻辑 (数量展示、
// 批量按钮 disable) 在 e2e 里覆盖更直观, 这里聚焦菜单交互。

function makeJob(overrides: Partial<JobItem> = {}): JobItem {
  return {
    id: 1,
    rel_path: "a/b.mp4",
    number: "ABC-001",
    status: "reviewing",
    created_at: 0,
    updated_at: 0,
    error_message: "",
    conflict_reason: "",
    raw_number: "",
    cleaned_number: "",
    number_source: "manual",
    number_clean_status: "success",
    number_clean_confidence: "high",
    number_clean_warnings: "",
    ...overrides,
  };
}

function renderPanel(overrides: Partial<ReviewListPanelProps> = {}) {
  const job = makeJob({ id: 1 });
  const props: ReviewListPanelProps = {
    items: [job],
    selectedId: job.id,
    selectedIndex: 0,
    selectedJobIds: new Set(),
    selectedCount: 0,
    allSelectableChecked: false,
    isPending: false,
    moveRunning: false,
    selectAllRef: createRef<HTMLInputElement>(),
    onToggleSelectAll: vi.fn(),
    onToggleSelectJob: vi.fn(),
    onLoadDetail: vi.fn(),
    onImportSelected: vi.fn(),
    onDeleteSelected: vi.fn(),
    onImport: vi.fn(),
    onDelete: vi.fn(),
    onReject: vi.fn(),
    ...overrides,
  };
  const utils = render(<ReviewListPanel {...props} />);
  return { ...utils, props };
}

afterEach(() => {
  cleanup();
});

describe("ReviewListPanel overflow menu", () => {
  it("默认不显示 ... 菜单项, 点击 trigger 才出现 打回/删除", () => {
    renderPanel();
    expect(screen.queryByRole("menuitem", { name: "打回" })).toBeNull();
    expect(screen.queryByRole("menuitem", { name: "删除" })).toBeNull();

    const trigger = screen.getByRole("button", { name: "更多操作" });
    fireEvent.click(trigger);

    expect(screen.getByRole("menuitem", { name: "打回" })).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "删除" })).toBeTruthy();
  });

  it("点击 打回 菜单项: 触发 onReject, 然后菜单关闭", () => {
    const onReject = vi.fn();
    renderPanel({ onReject });

    fireEvent.click(screen.getByRole("button", { name: "更多操作" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "打回" }));

    expect(onReject).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menuitem", { name: "打回" })).toBeNull();
  });

  it("点击 删除 菜单项: 触发 onDelete, 然后菜单关闭", () => {
    const onDelete = vi.fn();
    renderPanel({ onDelete });

    fireEvent.click(screen.getByRole("button", { name: "更多操作" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "删除" }));

    expect(onDelete).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menuitem", { name: "删除" })).toBeNull();
  });

  it("点击外部 / 按 Esc 关闭菜单", () => {
    renderPanel();

    fireEvent.click(screen.getByRole("button", { name: "更多操作" }));
    expect(screen.getByRole("menuitem", { name: "打回" })).toBeTruthy();

    act(() => {
      document.body.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
    });
    expect(screen.queryByRole("menuitem", { name: "打回" })).toBeNull();

    // 再打开一次, 用 Escape 关掉
    fireEvent.click(screen.getByRole("button", { name: "更多操作" }));
    expect(screen.getByRole("menuitem", { name: "打回" })).toBeTruthy();
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    expect(screen.queryByRole("menuitem", { name: "打回" })).toBeNull();
  });

  it("selectedId 不匹配时: ... 按钮 disabled, 无法弹出菜单", () => {
    renderPanel({ selectedId: 999 });
    const trigger = screen.getByRole("button", { name: "更多操作" });
    expect(trigger.hasAttribute("disabled")).toBe(true);
    fireEvent.click(trigger);
    expect(screen.queryByRole("menuitem", { name: "打回" })).toBeNull();
  });

  it("moveRunning=true: overflow trigger 也被禁用 (和 入库 按钮保持一致), title 提示迁移中", () => {
    // 媒体库迁移进行中: 入库按钮早已 disabled, overflow 菜单里的"删除"
    // 会 os.Remove 源文件, 迁移时极易和搬文件的路径撞车; "打回"虽然只改 DB
    // 但为了 UI 不割裂一起锁上。这个 case 就是防御回归。
    renderPanel({ moveRunning: true });
    const trigger = screen.getByRole("button", { name: "更多操作" });
    expect(trigger.hasAttribute("disabled")).toBe(true);
    expect(trigger.getAttribute("title")).toContain("媒体库移动进行中");
    fireEvent.click(trigger);
    expect(screen.queryByRole("menuitem", { name: "打回" })).toBeNull();
    expect(screen.queryByRole("menuitem", { name: "删除" })).toBeNull();
  });

  // 批量删除按钮和 overflow 里的"删除"走同一套 os.Remove 源文件路径,
  // 之前只堵了 overflow 而漏掉这里 (B 修不全)。本 case 回归"批量删除入口
  // 也被 moveRunning 锁"。
  it("批量删除按钮: moveRunning=true 时 disabled + title 提示迁移中", () => {
    renderPanel({
      moveRunning: true,
      selectedJobIds: new Set([1]),
      selectedCount: 1,
    });
    const btn = screen.getByRole("button", { name: "批量删除" });
    expect(btn.hasAttribute("disabled")).toBe(true);
    expect(btn.getAttribute("title")).toContain("媒体库移动进行中");
  });

  it("批量删除按钮: selectedCount > 0 且非 moveRunning 时可用", () => {
    const onDeleteSelected = vi.fn();
    renderPanel({
      selectedJobIds: new Set([1]),
      selectedCount: 1,
      onDeleteSelected,
    });
    const btn = screen.getByRole("button", { name: "批量删除" });
    expect(btn.hasAttribute("disabled")).toBe(false);
    fireEvent.click(btn);
    expect(onDeleteSelected).toHaveBeenCalledTimes(1);
  });

  // 菜单通过 portal 渲染到 document.body 以避开 .review-job-list 的 overflow
  // 裁剪。这里回归两件事: (1) 菜单节点确实在 list-panel 外部, 证明 portal
  // 生效了; (2) 点菜单项本身不会触发"点外部"逻辑把菜单关掉 (之前 containerRef
  // 版本是靠 DOM 嵌套保证的, portal 后必须显式在 trigger 和 menu 两个 ref
  // 上都做 contains 检查)。
  it("菜单通过 portal 渲染到 panel 外部, 点菜单项仍可以触发回调", () => {
    const onReject = vi.fn();
    const { container } = renderPanel({ onReject });
    fireEvent.click(screen.getByRole("button", { name: "更多操作" }));

    const rejectItem = screen.getByRole("menuitem", { name: "打回" });
    expect(container.contains(rejectItem)).toBe(false);
    expect(document.body.contains(rejectItem)).toBe(true);

    fireEvent.mouseDown(rejectItem);
    fireEvent.click(rejectItem);
    expect(onReject).toHaveBeenCalledTimes(1);
  });
});
