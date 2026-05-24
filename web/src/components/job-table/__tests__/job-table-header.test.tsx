// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { FilterChip, SummaryCard } from "../job-table-header";
import { JobTableHeader } from "../job-table-header";

afterEach(() => {
  cleanup();
});

const summary: readonly SummaryCard[] = [
  { label: "总任务", value: 12, hint: "all", tone: "default", filter: "all" },
  { label: "失败", value: 2, hint: "failed", tone: "danger", filter: "failed" },
];

const chips: readonly FilterChip[] = [
  { value: "all", label: "全部", count: 12 },
  { value: "init", label: "未提交", count: 5 },
];

function baseProps(overrides: Partial<React.ComponentProps<typeof JobTableHeader>> = {}) {
  return {
    jobsCount: 12,
    total: 50,
    keyword: "",
    statusFilter: "all",
    isPending: false,
    isScanning: false,
    summaryCards: summary,
    filterChips: chips,
    onKeywordChange: vi.fn(),
    onStatusFilterChange: vi.fn(),
    onScan: vi.fn(),
    ...overrides,
  };
}

describe("JobTableHeader", () => {
  it("正常路径: 渲染 hero 文案 + summary cards + chip 行 + 立即扫描按钮", () => {
    render(<JobTableHeader {...baseProps()} />);
    expect(screen.getByText("Processing Queue")).toBeTruthy();
    expect(screen.getByText("文件列表")).toBeTruthy();
    // 子句包含 "12" / "50"
    expect(screen.getByText(/12 条记录/)).toBeTruthy();
    expect(screen.getByText(/50 条任务/)).toBeTruthy();
    expect(screen.getByText("总任务")).toBeTruthy();
    expect(screen.getByText("失败")).toBeTruthy();
    expect(screen.getByText("全部")).toBeTruthy();
    expect(screen.getByText("未提交")).toBeTruthy();
    expect(screen.getByRole("button", { name: /立即扫描/ })).toBeTruthy();
  });

  it("异常路径: 点击 summary card / chip 都触发 onStatusFilterChange 且参数不同", () => {
    const onStatusFilterChange = vi.fn();
    render(<JobTableHeader {...baseProps({ onStatusFilterChange })} />);
    fireEvent.click(screen.getByText("失败"));
    expect(onStatusFilterChange).toHaveBeenLastCalledWith("failed");
    fireEvent.click(screen.getByText("未提交"));
    expect(onStatusFilterChange).toHaveBeenLastCalledWith("init");
  });

  it("边缘路径: keyword onChange 触发 onKeywordChange, isScanning=true 按钮 disabled + label 改", () => {
    const onKeywordChange = vi.fn();
    const onScan = vi.fn();
    render(
      <JobTableHeader
        {...baseProps({ keyword: "abc", onKeywordChange, isScanning: true, onScan })}
      />,
    );
    const input = screen.getByPlaceholderText(/按文件名/);
    fireEvent.change(input, { target: { value: "xyz" } });
    expect(onKeywordChange).toHaveBeenCalledWith("xyz");
    const scanBtn = screen.getByRole("button", { name: /扫描中/ });
    expect(scanBtn.hasAttribute("disabled")).toBe(true);
    fireEvent.click(scanBtn);
    expect(onScan).not.toHaveBeenCalled();
  });
});
