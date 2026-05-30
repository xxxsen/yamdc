// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { JobItem } from "@/lib/api";

import { ReviewDetailHeader } from "../detail-header";

afterEach(() => {
  cleanup();
});

const fakeJob: JobItem = {
  id: 7,
  rel_path: "movies/foo.mp4",
  number: "ABC-001",
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  status: "reviewing" as any,
  file_size: 0,
  updated_at: 0,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as any;

describe("ReviewDetailHeader", () => {
  it("正常路径: selected + hasRawMeta=true → 渲染当前任务 + '恢复原始刮削内容' 按钮可点", () => {
    const onRestoreRaw = vi.fn();
    render(
      <ReviewDetailHeader
        selected={fakeJob}
        message=""
        messageTone="info"
        hasRawMeta={true}
        isPending={false}
        onRestoreRaw={onRestoreRaw}
      />,
    );
    expect(screen.getByText(/当前任务 #7/)).toBeTruthy();
    const btn = screen.getByRole("button", { name: "恢复原始刮削内容" });
    expect(btn.hasAttribute("disabled")).toBe(false);
    fireEvent.click(btn);
    expect(onRestoreRaw).toHaveBeenCalledTimes(1);
  });

  it("异常路径: hasRawMeta=false → 按钮 disabled, 点击不触发 onRestoreRaw", () => {
    const onRestoreRaw = vi.fn();
    render(
      <ReviewDetailHeader
        selected={fakeJob}
        message="保存失败"
        messageTone="danger"
        hasRawMeta={false}
        isPending={false}
        onRestoreRaw={onRestoreRaw}
      />,
    );
    expect(screen.getByText("保存失败").getAttribute("data-tone")).toBe("danger");
    const btn = screen.getByRole("button", { name: "恢复原始刮削内容" });
    expect(btn.hasAttribute("disabled")).toBe(true);
    fireEvent.click(btn);
    expect(onRestoreRaw).not.toHaveBeenCalled();
  });

  it("边缘路径: selected=null → 不渲染当前任务行, 按钮也 disabled", () => {
    render(
      <ReviewDetailHeader
        selected={null}
        message=""
        messageTone="info"
        hasRawMeta={true}
        isPending={false}
        onRestoreRaw={vi.fn()}
      />,
    );
    expect(screen.queryByText(/当前任务/)).toBeNull();
    expect(screen.getByRole("button", { name: "恢复原始刮削内容" }).hasAttribute("disabled")).toBe(true);
  });
});
