// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LibraryBottomActions } from "../bottom-actions";

afterEach(() => {
  cleanup();
});

function baseProps(overrides: Partial<React.ComponentProps<typeof LibraryBottomActions>> = {}) {
  return {
    refreshBusy: false,
    moveBusy: false,
    mediaSyncRunning: false,
    configured: true,
    refreshButtonLabel: "重新扫描",
    moveButtonLabel: "移动到媒体库",
    moveProgressVisible: false,
    moveState: null,
    moveProgress: 0,
    onRefresh: vi.fn(),
    onMove: vi.fn(),
    ...overrides,
  };
}

describe("LibraryBottomActions", () => {
  it("正常路径: 两个按钮可点; refresh/move 各自触发对应 callback", () => {
    const onRefresh = vi.fn();
    const onMove = vi.fn();
    render(<LibraryBottomActions {...baseProps({ onRefresh, onMove })} />);
    fireEvent.click(screen.getByRole("button", { name: /重新扫描/ }));
    expect(onRefresh).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: /移动到媒体库/ }));
    expect(onMove).toHaveBeenCalledTimes(1);
  });

  it("异常路径: configured=false 或 mediaSyncRunning=true → 移动按钮 disabled", () => {
    const onMove = vi.fn();
    const { rerender } = render(
      <LibraryBottomActions {...baseProps({ configured: false, onMove })} />,
    );
    let moveBtn = screen.getByRole("button", { name: /移动到媒体库/ });
    expect(moveBtn.hasAttribute("disabled")).toBe(true);
    fireEvent.click(moveBtn);
    expect(onMove).not.toHaveBeenCalled();

    rerender(<LibraryBottomActions {...baseProps({ mediaSyncRunning: true, onMove })} />);
    moveBtn = screen.getByRole("button", { name: /移动到媒体库/ });
    expect(moveBtn.hasAttribute("disabled")).toBe(true);
  });

  it("边缘路径: moveProgressVisible=true + moveState 渲染进度条; refreshBusy=true → refresh icon 加 spinning class", () => {
    const { container } = render(
      <LibraryBottomActions
        {...baseProps({
          moveProgressVisible: true,
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          moveState: { task_key: "k", status: "running" } as any,
          moveProgress: 42,
          refreshBusy: true,
        })}
      />,
    );
    const fill = container.querySelector(".library-action-progress-fill") as HTMLElement;
    expect(fill).toBeTruthy();
    expect(fill.style.width).toBe("42%");
    // refresh icon 应该带 spinning class.
    const spinning = container.querySelector(".media-library-sync-icon-spinning");
    expect(spinning).toBeTruthy();
  });
});
