// @vitest-environment jsdom

import { act } from "react";
import { describe, expect, it, vi } from "vitest";

import { Button } from "@/components/ui/button";

import { mount } from "./test-helpers";

describe("Button", () => {
  // ---- 正常 case ----
  it("默认渲染 secondary variant: 仅 .btn, 不带 modifier", () => {
    const { container, unmount } = mount(<Button>保存</Button>);
    const btn = container.querySelector("button");
    expect(btn).not.toBeNull();
    expect(btn!.className.split(" ")).toContain("btn");
    expect(btn!.className).not.toContain("btn-primary");
    expect(btn!.textContent).toBe("保存");
    expect(btn!.type).toBe("button");
    unmount();
  });

  it("variant=primary 追加 .btn-primary", () => {
    const { container, unmount } = mount(<Button variant="primary">提交</Button>);
    const btn = container.querySelector("button")!;
    expect(btn.className).toContain("btn-primary");
    unmount();
  });

  it("click 触发 onClick handler", () => {
    const onClick = vi.fn();
    const { container, unmount } = mount(<Button onClick={onClick}>click</Button>);
    const btn = container.querySelector("button")!;
    act(() => {
      btn.click();
    });
    expect(onClick).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("leftIcon / rightIcon 渲染在文本两侧", () => {
    const { container, unmount } = mount(
      <Button
        leftIcon={<span data-testid="l" />}
        rightIcon={<span data-testid="r" />}
      >
        text
      </Button>,
    );
    expect(container.querySelector("[data-testid='l']")).not.toBeNull();
    expect(container.querySelector("[data-testid='r']")).not.toBeNull();
    unmount();
  });

  // ---- 异常 case ----
  it("disabled=true 时 click 不触发 handler, 且 HTML disabled 标志置位", () => {
    const onClick = vi.fn();
    const { container, unmount } = mount(
      <Button onClick={onClick} disabled>
        x
      </Button>,
    );
    const btn = container.querySelector("button")!;
    expect(btn.disabled).toBe(true);
    act(() => {
      btn.click();
    });
    expect(onClick).not.toHaveBeenCalled();
    unmount();
  });

  it("loading=true 时 button 置 disabled, 渲染 spinner, aria-busy='true'", () => {
    const onClick = vi.fn();
    const { container, unmount } = mount(
      <Button loading onClick={onClick}>
        x
      </Button>,
    );
    const btn = container.querySelector("button")!;
    expect(btn.disabled).toBe(true);
    expect(btn.getAttribute("aria-busy")).toBe("true");
    expect(container.querySelector(".ui-button-spinner")).not.toBeNull();
    act(() => {
      btn.click();
    });
    expect(onClick).not.toHaveBeenCalled();
    unmount();
  });

  it("loading=true 时 leftIcon 被 spinner 替换 (不同时渲染)", () => {
    const { container, unmount } = mount(
      <Button loading leftIcon={<span data-testid="left" />}>
        x
      </Button>,
    );
    expect(container.querySelector("[data-testid='left']")).toBeNull();
    expect(container.querySelector(".ui-button-spinner")).not.toBeNull();
    unmount();
  });

  // ---- 边缘 case ----
  it("className 透传, 不破坏 .btn 基础类", () => {
    const { container, unmount } = mount(
      <Button className="custom-a custom-b" variant="primary">
        x
      </Button>,
    );
    const btn = container.querySelector("button")!;
    const classes = btn.className.split(" ");
    expect(classes).toContain("btn");
    expect(classes).toContain("btn-primary");
    expect(classes).toContain("custom-a");
    expect(classes).toContain("custom-b");
    unmount();
  });

  it("type 可覆盖为 submit", () => {
    const { container, unmount } = mount(
      <Button type="submit">提交</Button>,
    );
    const btn = container.querySelector("button")!;
    expect(btn.type).toBe("submit");
    unmount();
  });

  it("aria-label 等 HTML 属性透传到底层 button", () => {
    const { container, unmount } = mount(
      <Button aria-label="关闭" data-testid="x">
        <span />
      </Button>,
    );
    const btn = container.querySelector("button")!;
    expect(btn.getAttribute("aria-label")).toBe("关闭");
    expect(btn.getAttribute("data-testid")).toBe("x");
    unmount();
  });
});
