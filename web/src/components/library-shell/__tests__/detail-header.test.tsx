// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LibraryDetailHeader } from "../detail-header";

afterEach(() => {
  cleanup();
});

describe("LibraryDetailHeader", () => {
  it("正常路径: 渲染 subtitle + 中/原 toggle; 点中文/原文切 onCopyModeChange", () => {
    const onCopyModeChange = vi.fn();
    render(
      <LibraryDetailHeader
        subtitle="movies/foo"
        copyMode="translated"
        onCopyModeChange={onCopyModeChange}
        conflict={false}
        isPending={false}
        onDelete={vi.fn()}
      />,
    );
    expect(screen.getByText("movies/foo")).toBeTruthy();
    fireEvent.click(screen.getByText("原文"));
    expect(onCopyModeChange).toHaveBeenCalledWith("original");
    fireEvent.click(screen.getByText("中文"));
    expect(onCopyModeChange).toHaveBeenCalledWith("translated");
  });

  it("异常路径: conflict=true → 渲染冲突 badge", () => {
    render(
      <LibraryDetailHeader
        subtitle="x"
        copyMode="translated"
        onCopyModeChange={vi.fn()}
        conflict={true}
        isPending={false}
        onDelete={vi.fn()}
      />,
    );
    expect(screen.getByText("已存在(冲突)")).toBeTruthy();
  });

  it("边缘路径: isPending=true → 删除按钮 disabled, 点击不触发 onDelete", () => {
    const onDelete = vi.fn();
    render(
      <LibraryDetailHeader
        subtitle="x"
        copyMode="translated"
        onCopyModeChange={vi.fn()}
        conflict={false}
        isPending={true}
        onDelete={onDelete}
      />,
    );
    const btn = screen.getByRole("button", { name: /删除/ });
    expect(btn.hasAttribute("disabled")).toBe(true);
    fireEvent.click(btn);
    expect(onDelete).not.toHaveBeenCalled();
  });
});
