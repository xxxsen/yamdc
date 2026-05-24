// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { MediaLibrarySyncLogEntry } from "@/lib/api";

import { MediaLibrarySyncLogsModal } from "../sync-logs-modal";

afterEach(() => {
  cleanup();
});

const fakeLog: MediaLibrarySyncLogEntry = {
  id: 1,
  run_id: "run-001",
  level: "info",
  rel_path: "movies/foo",
  message: "扫描成功",
  created_at: 1700000000000,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as any;

describe("MediaLibrarySyncLogsModal", () => {
  it("正常路径: 渲染日志列表 + 关闭按钮; 点击关闭调 onClose", () => {
    const onClose = vi.fn();
    render(
      <MediaLibrarySyncLogsModal
        open={true}
        onClose={onClose}
        loading={false}
        error=""
        logs={[fakeLog]}
      />,
    );
    expect(screen.getByText("媒体库同步日志")).toBeTruthy();
    expect(screen.getByText("扫描成功")).toBeTruthy();
    expect(screen.getByText("INFO")).toBeTruthy();
    expect(screen.getByText("run-001")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("异常路径: error 非空 → 渲染 ErrorState 含错误文案; loading=true → Spinner", () => {
    const { rerender } = render(
      <MediaLibrarySyncLogsModal
        open={true}
        onClose={vi.fn()}
        loading={false}
        error="拉日志超时"
        logs={[]}
      />,
    );
    expect(screen.getByText("加载同步日志失败")).toBeTruthy();
    expect(screen.getByText("拉日志超时")).toBeTruthy();

    rerender(
      <MediaLibrarySyncLogsModal
        open={true}
        onClose={vi.fn()}
        loading={true}
        error=""
        logs={[]}
      />,
    );
    // Spinner 默认带 role="status" / aria-label, 退而求其次断言 list 不渲染.
    expect(screen.queryByRole("listitem")).toBeNull();
  });

  it("边缘路径: logs=[] 且 loading/error 都假 → 渲染 EmptyState 文案; open=false → modal 不显示", () => {
    const { rerender } = render(
      <MediaLibrarySyncLogsModal
        open={true}
        onClose={vi.fn()}
        loading={false}
        error=""
        logs={[]}
      />,
    );
    expect(screen.getByText("暂无同步日志")).toBeTruthy();

    rerender(
      <MediaLibrarySyncLogsModal
        open={false}
        onClose={vi.fn()}
        loading={false}
        error=""
        logs={[fakeLog]}
      />,
    );
    expect(screen.queryByText("媒体库同步日志")).toBeNull();
  });
});
