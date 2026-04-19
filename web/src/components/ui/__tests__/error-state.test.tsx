// @vitest-environment jsdom

import { act } from "react";
import { describe, expect, it, vi } from "vitest";

import { ErrorState } from "@/components/ui/error-state";

import { mount } from "./test-helpers";

describe("ErrorState", () => {
  it("正常 case: 默认 title '加载失败', role=alert, 没传 onRetry 时无重试按钮", () => {
    const { container, unmount } = mount(<ErrorState />);
    const root = container.querySelector(".error-state")!;
    expect(root).not.toBeNull();
    expect(root.getAttribute("role")).toBe("alert");
    expect(root.textContent).toContain("加载失败");
    // 无重试按钮
    expect(container.querySelector("button")).toBeNull();
    unmount();
  });

  it("正常 case: 传 title / detail / icon 正确渲染所有 slot", () => {
    const { container, unmount } = mount(
      <ErrorState
        title="保存失败"
        detail="网络超时, 请稍后再试"
        icon={<span data-testid="err-icon">!</span>}
      />,
    );
    const root = container.querySelector(".error-state")!;
    expect(root.querySelector('[data-testid="err-icon"]')).not.toBeNull();
    expect(root.textContent).toContain("保存失败");
    expect(root.textContent).toContain("网络超时, 请稍后再试");
    unmount();
  });

  it("正常 case: onRetry 存在时渲染重试按钮, 点击触发回调", () => {
    const onRetry = vi.fn();
    const { container, unmount } = mount(<ErrorState onRetry={onRetry} />);
    const btn = container.querySelector<HTMLButtonElement>("button")!;
    expect(btn).not.toBeNull();
    expect(btn.textContent).toBe("重试");
    act(() => {
      btn.click();
    });
    expect(onRetry).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("异常 case: retryLabel 覆盖默认 '重试' 文案", () => {
    const onRetry = vi.fn();
    const { container, unmount } = mount(<ErrorState onRetry={onRetry} retryLabel="再试一次" />);
    const btn = container.querySelector<HTMLButtonElement>("button")!;
    expect(btn.textContent).toBe("再试一次");
    unmount();
  });

  it("边缘 case: 自定义 className 叠加在 .error-state 上", () => {
    const { container, unmount } = mount(<ErrorState className="my-err" />);
    const root = container.querySelector(".error-state")!;
    const classes = root.className.split(" ");
    expect(classes).toContain("error-state");
    expect(classes).toContain("my-err");
    unmount();
  });
});
