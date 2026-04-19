// @vitest-environment jsdom

import { describe, expect, it } from "vitest";

import { EmptyState } from "@/components/ui/empty-state";

import { mount } from "./test-helpers";

describe("EmptyState", () => {
  it("正常 case: 默认 block variant 走 .review-empty-state, 有 role=status", () => {
    const { container, unmount } = mount(<EmptyState title="暂无数据" />);
    const root = container.querySelector(".review-empty-state")!;
    expect(root).not.toBeNull();
    expect(root.getAttribute("role")).toBe("status");
    expect(root.getAttribute("aria-live")).toBe("polite");
    expect(root.textContent).toContain("暂无数据");
    unmount();
  });

  it("正常 case: hint / icon / action 三个可选槽按传入顺序渲染", () => {
    const { container, unmount } = mount(
      <EmptyState
        title="无结果"
        hint="试试切换筛选"
        icon={<span data-testid="icon">i</span>}
        action={<button type="button" data-testid="action">重置</button>}
      />,
    );
    const root = container.querySelector(".review-empty-state")!;
    expect(root.querySelector('[data-testid="icon"]')).not.toBeNull();
    expect(root.querySelector('[data-testid="action"]')).not.toBeNull();
    expect(root.textContent).toContain("无结果");
    expect(root.textContent).toContain("试试切换筛选");
    unmount();
  });

  it("正常 case: variant=inline 走 span + .library-inline-muted, 只渲染 title", () => {
    const { container, unmount } = mount(
      <EmptyState variant="inline" title="暂无演员" hint="will-be-ignored" icon={<span data-testid="x" />} />,
    );
    const span = container.querySelector("span.library-inline-muted")!;
    expect(span).not.toBeNull();
    expect(span.textContent).toBe("暂无演员");
    // inline 模式不渲染 hint / icon / action
    expect(container.querySelector('[data-testid="x"]')).toBeNull();
    unmount();
  });

  it("异常 case: variant=compact 用 review-empty-state 基础样式但 padding 被改成 12px 16px", () => {
    const { container, unmount } = mount(<EmptyState variant="compact" title="紧凑空态" />);
    const root = container.querySelector<HTMLElement>(".review-empty-state")!;
    expect(root).not.toBeNull();
    expect(root.style.padding).toBe("12px 16px");
    unmount();
  });

  it("边缘 case: 自定义 className 并不吃掉默认类", () => {
    const { container, unmount } = mount(<EmptyState title="x" className="my-empty" />);
    const root = container.querySelector(".review-empty-state")!;
    const classes = root.className.split(" ");
    expect(classes).toContain("review-empty-state");
    expect(classes).toContain("my-empty");
    unmount();
  });
});
