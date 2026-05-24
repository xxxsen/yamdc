// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { JobItem } from "@/lib/api";

import { JobRow } from "../job-row";

afterEach(() => {
  cleanup();
});

function makeJob(overrides: Partial<JobItem> = {}): JobItem {
  return {
    id: 1,
    rel_path: "movies/awful.mp4",
    number: "ABC-001",
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    status: "init" as any,
    file_size: 1024,
    updated_at: 0,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    number_clean_status: "success" as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    number_clean_confidence: "high" as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(overrides as any),
  } as JobItem;
}

function baseProps(overrides: Partial<React.ComponentProps<typeof JobRow>> = {}) {
  return {
    job: makeJob(),
    isSelected: false,
    hasHydrated: true,
    isPending: false,
    onToggleSelect: vi.fn(),
    onStartEdit: vi.fn(),
    onRun: vi.fn(),
    onRerun: vi.fn(),
    onDelete: vi.fn(),
    onOpenLogs: vi.fn(),
    ...overrides,
  };
}

// JobRow 内嵌的所有 disabled 决策都基于 (job.status, conflict_reason,
// number cleaning) 三个维度. 三类用例:
//   - 正常: init + 高置信度 → 可勾选 / 可提交 / 可删除 / 可编辑
//   - 异常: failed + low confidence → 重试 disabled / 编辑器仍开放
//   - 边缘: processing 不可删除 + 无日志按钮 disabled flag

describe("JobRow", () => {
  it("正常: init 高置信度可勾选, 点击 checkbox 触发 onToggleSelect; 提交按钮可用", () => {
    const onToggleSelect = vi.fn();
    const onRun = vi.fn();
    render(
      <table><tbody>
        <JobRow {...baseProps({ onToggleSelect, onRun })} />
      </tbody></table>,
    );
    const checkbox = screen.getByRole("checkbox");
    expect(checkbox.disabled).toBe(false);
    fireEvent.click(checkbox);
    expect(onToggleSelect).toHaveBeenCalledWith(1);

    const runBtn = screen.getByRole("button", { name: /提交/ });
    expect(runBtn.hasAttribute("disabled")).toBe(false);
    fireEvent.click(runBtn);
    expect(onRun).toHaveBeenCalledTimes(1);
  });

  it("异常: 有 conflict_reason → 提交按钮 disabled + title 含冲突原因; 删除仍可点", () => {
    const onRun = vi.fn();
    const onDelete = vi.fn();
    const job = makeJob({
      conflict_reason: "目标已存在",
      conflict_target: "/library/foo.mp4",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <table><tbody>
        <JobRow {...baseProps({ job, onRun, onDelete })} />
      </tbody></table>,
    );
    const runBtn = screen.getByRole("button", { name: /提交/ });
    expect(runBtn.hasAttribute("disabled")).toBe(true);
    expect(runBtn.getAttribute("title")).toContain("目标已存在");

    fireEvent.click(runBtn);
    expect(onRun).not.toHaveBeenCalled();

    const delBtn = screen.getByRole("button", { name: /删除/ });
    fireEvent.click(delBtn);
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it("边缘: status=failed 渲染 '重试' 按钮 + 编辑铅笔仍可点, processing 状态时 '查看日志' 按钮挂出", () => {
    const onRerun = vi.fn();
    const onStartEdit = vi.fn();
    // failed 任务渲染重试 + 仍可编辑 number.
    const failedJob = makeJob({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      status: "failed" as any,
      number_source: "manual",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    const { rerender } = render(
      <table><tbody>
        <JobRow {...baseProps({ job: failedJob, onRerun, onStartEdit })} />
      </tbody></table>,
    );
    fireEvent.click(screen.getByRole("button", { name: /重试/ }));
    expect(onRerun).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "编辑影片 ID" }));
    expect(onStartEdit).toHaveBeenCalledTimes(1);

    // processing 任务: 不能删除 + 有 "查看日志" 按钮.
    const onOpenLogs = vi.fn();
    const procJob = makeJob({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      status: "processing" as any,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    rerender(
      <table><tbody>
        <JobRow {...baseProps({ job: procJob, onOpenLogs })} />
      </tbody></table>,
    );
    expect(screen.getByRole("button", { name: /删除/ }).hasAttribute("disabled")).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "查看日志" }));
    expect(onOpenLogs).toHaveBeenCalledTimes(1);
  });
});
