// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { DeleteConfirmOverlay } from "../delete-confirm-overlay";

// DeleteConfirmOverlay 本身很薄, 但它是"最后一道 UI 闸门" ——
// 一旦点了 "删除" 就会立刻触发 confirmDelete → deleteJob → os.Remove。
// 媒体库 move 中途起来的情况下, 对话框可能已经在前面打开了, 所以即便
// 外层按钮先 disabled 了, 这里也必须挡一层。本文件主要覆盖 moveRunning
// 场景下的 UI 锁 (按钮 disabled + 提示 banner)。

afterEach(() => {
  cleanup();
});

function baseProps(overrides: Partial<React.ComponentProps<typeof DeleteConfirmOverlay>> = {}) {
  return {
    targetIds: [1] as number[] | null,
    selectedRelPath: "a/b.mp4",
    onCancel: vi.fn(),
    onConfirm: vi.fn(),
    isPending: false,
    moveRunning: false,
    ...overrides,
  };
}

describe("DeleteConfirmOverlay", () => {
  it("targetIds 为 null 时不渲染", () => {
    const { container } = render(<DeleteConfirmOverlay {...baseProps({ targetIds: null })} />);
    expect(container.firstChild).toBeNull();
  });

  it("单条: 展示路径和标题, 按钮可点击 onConfirm", () => {
    const onConfirm = vi.fn();
    render(<DeleteConfirmOverlay {...baseProps({ onConfirm })} />);
    expect(screen.getByText("确认删除")).toBeTruthy();
    expect(screen.getByText("a/b.mp4")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("moveRunning=true: 渲染 warning banner + '删除' 按钮 disabled + title 提示", () => {
    const onConfirm = vi.fn();
    render(
      <DeleteConfirmOverlay {...baseProps({ onConfirm, moveRunning: true })} />,
    );
    const confirm = screen.getByRole("button", { name: "删除" });
    expect(confirm.hasAttribute("disabled")).toBe(true);
    expect(confirm.getAttribute("title")).toContain("媒体库移动进行中");
    // banner 文本
    expect(screen.getByRole("alert").textContent).toContain("媒体库移动进行中");
    // disabled 情况下 JSDOM 会屏蔽 click handler, 不会调 onConfirm
    fireEvent.click(confirm);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("moveRunning=true: 取消按钮仍可用, 保证用户可以关掉对话框", () => {
    const onCancel = vi.fn();
    render(
      <DeleteConfirmOverlay {...baseProps({ onCancel, moveRunning: true })} />,
    );
    const cancel = screen.getByRole("button", { name: "取消" });
    expect(cancel.hasAttribute("disabled")).toBe(false);
    fireEvent.click(cancel);
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("isPending=true 也 disabled 删除 (和原有语义兼容, moveRunning 不覆盖 isPending)", () => {
    render(<DeleteConfirmOverlay {...baseProps({ isPending: true })} />);
    const confirm = screen.getByRole("button", { name: "删除" });
    expect(confirm.hasAttribute("disabled")).toBe(true);
  });
});
