// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RestoreConfirmOverlay } from "../restore-confirm-overlay";

afterEach(() => {
  cleanup();
});

describe("RestoreConfirmOverlay", () => {
  it("正常路径: open=false 不渲染", () => {
    const { container } = render(
      <RestoreConfirmOverlay
        open={false}
        selectedRelPath="x.mp4"
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
        isPending={false}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("异常路径: open=true 渲染标题/路径/按钮; 点击恢复触发 onConfirm", () => {
    const onConfirm = vi.fn();
    render(
      <RestoreConfirmOverlay
        open={true}
        selectedRelPath="movies/old.mp4"
        onCancel={vi.fn()}
        onConfirm={onConfirm}
        isPending={false}
      />,
    );
    expect(screen.getByText("恢复原始内容")).toBeTruthy();
    expect(screen.getByText("movies/old.mp4")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "恢复" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("边缘路径: isPending=true → 恢复按钮 disabled; 取消和 backdrop 仍调 onCancel", () => {
    const onCancel = vi.fn();
    const { container } = render(
      <RestoreConfirmOverlay
        open={true}
        selectedRelPath={undefined}
        onCancel={onCancel}
        onConfirm={vi.fn()}
        isPending={true}
      />,
    );
    expect(screen.getByRole("button", { name: "恢复" }).hasAttribute("disabled")).toBe(true);
    // 对话框内点击 stopPropagation, 不触发 onCancel.
    const dialog = container.querySelector(".review-confirm-dialog") as HTMLElement;
    fireEvent.click(dialog);
    expect(onCancel).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
    fireEvent.click(container.querySelector(".review-preview-overlay") as HTMLElement);
    expect(onCancel).toHaveBeenCalledTimes(2);
  });
});
