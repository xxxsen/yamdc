// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { JobItem } from "@/lib/api";

import { DeleteConfirmModal } from "../delete-confirm-modal";

afterEach(() => {
  cleanup();
});

const fakeJob: JobItem = {
  id: 1,
  rel_path: "movies/awful.mp4",
  number: "ABC-001",
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  status: "init" as any,
  file_size: 0,
  updated_at: 0,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as any;

describe("DeleteConfirmModal", () => {
  it("正常路径: 渲染标题/路径/两个按钮; 点击删除回调 onConfirm(job)", () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <DeleteConfirmModal
        job={fakeJob}
        isPending={false}
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    expect(screen.getByText("确认删除")).toBeTruthy();
    expect(screen.getByText("movies/awful.mp4")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    expect(onConfirm).toHaveBeenCalledWith(fakeJob);
  });

  it("异常路径: isPending=true 时删除按钮 disabled, 点击不触发 onConfirm", () => {
    const onConfirm = vi.fn();
    render(
      <DeleteConfirmModal
        job={fakeJob}
        isPending={true}
        onConfirm={onConfirm}
        onCancel={vi.fn()}
      />,
    );
    const btn = screen.getByRole("button", { name: "删除" });
    expect(btn.hasAttribute("disabled")).toBe(true);
    fireEvent.click(btn);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("边缘路径: 点 backdrop 触发 onCancel; 点对话框内不触发 (stopPropagation)", () => {
    const onCancel = vi.fn();
    const { container } = render(
      <DeleteConfirmModal
        job={fakeJob}
        isPending={false}
        onConfirm={vi.fn()}
        onCancel={onCancel}
      />,
    );
    const backdrop = container.querySelector(".review-preview-overlay") as HTMLElement;
    const dialog = container.querySelector(".review-confirm-dialog") as HTMLElement;
    fireEvent.click(dialog);
    expect(onCancel).not.toHaveBeenCalled();
    fireEvent.click(backdrop);
    expect(onCancel).toHaveBeenCalledTimes(1);
    // 取消按钮也走 onCancel.
    fireEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(onCancel).toHaveBeenCalledTimes(2);
  });
});
