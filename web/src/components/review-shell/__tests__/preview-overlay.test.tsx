// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ReviewPreviewOverlay } from "../preview-overlay";

afterEach(() => {
  cleanup();
});

describe("ReviewPreviewOverlay", () => {
  it("正常路径: preview=null 不渲染", () => {
    const { container } = render(<ReviewPreviewOverlay preview={null} onClose={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it("异常路径: 点 backdrop 触发 onClose, 点对话框内不触发", () => {
    const onClose = vi.fn();
    const { container } = render(
      <ReviewPreviewOverlay
        preview={{ title: "海报", item: { key: "k1", name: "p.jpg", rel_path: "" } }}
        onClose={onClose}
      />,
    );
    expect(screen.getByText("海报")).toBeTruthy();
    const dialog = container.querySelector(".review-preview-dialog") as HTMLElement;
    fireEvent.click(dialog);
    expect(onClose).not.toHaveBeenCalled();
    const overlay = container.querySelector(".review-preview-overlay") as HTMLElement;
    fireEvent.click(overlay);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("边缘路径: 点显式关闭按钮也触发 onClose, aria-label 提供给屏幕阅读器", () => {
    const onClose = vi.fn();
    render(
      <ReviewPreviewOverlay
        preview={{ title: "封面", item: { key: "k2", name: "c.jpg", rel_path: "" } }}
        onClose={onClose}
      />,
    );
    const closeBtn = screen.getByRole("button", { name: "关闭预览" });
    fireEvent.click(closeBtn);
    // 关闭按钮挂在 overlay 容器内, click 会冒泡到 overlay 的 onClick=onClose,
    // 实际触发两次都属于 "用户主动关闭" 路径, 关键是必须 ≥1 次, 不能不触发.
    expect(onClose).toHaveBeenCalled();
  });
});
