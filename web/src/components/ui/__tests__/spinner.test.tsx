// @vitest-environment jsdom

import { describe, expect, it } from "vitest";

import { Spinner } from "@/components/ui/spinner";

import { mount } from "./test-helpers";

describe("Spinner", () => {
  it("正常 case: 默认渲染 lg 尺寸, 只输出 .list-loading-spinner, 不带 overlay", () => {
    const { container, unmount } = mount(<Spinner />);
    const overlay = container.querySelector(".list-loading-overlay");
    expect(overlay).toBeNull();
    const spinner = container.querySelector(".list-loading-spinner");
    expect(spinner).not.toBeNull();
    expect(spinner!.getAttribute("role")).toBe("status");
    expect(spinner!.getAttribute("aria-label")).toBe("加载中");
    const style = (spinner as HTMLElement).style;
    expect(style.width).toBe("34px");
    expect(style.height).toBe("34px");
    unmount();
  });

  it("正常 case: size=sm 映射 16px, size=md 映射 24px", () => {
    const { container: smC, unmount: smU } = mount(<Spinner size="sm" />);
    const sm = smC.querySelector<HTMLElement>(".list-loading-spinner")!;
    expect(sm.style.width).toBe("16px");
    expect(sm.style.borderWidth).toBe("2px");
    smU();

    const { container: mdC, unmount: mdU } = mount(<Spinner size="md" />);
    const md = mdC.querySelector<HTMLElement>(".list-loading-spinner")!;
    expect(md.style.width).toBe("24px");
    mdU();
  });

  it("overlay=true 额外包一层 .list-loading-overlay, 并加 aria-live", () => {
    const { container, unmount } = mount(<Spinner overlay />);
    const overlay = container.querySelector(".list-loading-overlay");
    expect(overlay).not.toBeNull();
    expect(overlay!.getAttribute("aria-live")).toBe("polite");
    expect(overlay!.querySelector(".list-loading-spinner")).not.toBeNull();
    unmount();
  });

  it("异常 case: 自定义 label 覆盖默认 '加载中'", () => {
    const { container, unmount } = mount(<Spinner label="同步中" />);
    const spinner = container.querySelector(".list-loading-spinner")!;
    expect(spinner.getAttribute("aria-label")).toBe("同步中");
    unmount();
  });

  it("边缘 case: 传 className 不破坏 .list-loading-spinner 基础类", () => {
    const { container, unmount } = mount(<Spinner className="extra-class" />);
    const spinner = container.querySelector(".list-loading-spinner")!;
    const classes = spinner.className.split(" ");
    expect(classes).toContain("list-loading-spinner");
    expect(classes).toContain("extra-class");
    unmount();
  });
});
