// @vitest-environment jsdom

import { describe, expect, it } from "vitest";

import { Badge } from "@/components/ui/badge";

import { mount } from "./test-helpers";

describe("Badge", () => {
  // ---- 正常 case ----
  it("默认 variant=neutral, 渲染 .badge 和 .badge-neutral, 显示 children", () => {
    const { container, unmount } = mount(<Badge>标签</Badge>);
    const span = container.querySelector("span.badge");
    expect(span).not.toBeNull();
    expect(span!.className).toContain("badge-neutral");
    expect(span!.textContent).toBe("标签");
    unmount();
  });

  it("variant=info/success/warning/danger 分别映射到对应 alias class", () => {
    const variants = ["info", "success", "warning", "danger"] as const;
    for (const v of variants) {
      const { container, unmount } = mount(<Badge variant={v}>x</Badge>);
      const span = container.querySelector("span.badge")!;
      expect(span.className).toContain(`badge-${v}`);
      unmount();
    }
  });

  it("dot=true 时渲染 .badge-dot", () => {
    const { container, unmount } = mount(<Badge dot>label</Badge>);
    expect(container.querySelector(".badge-dot")).not.toBeNull();
    unmount();
  });

  // ---- 异常 case ----
  it("children=空字符串时仍正常渲染, 不 crash", () => {
    const { container, unmount } = mount(<Badge>{""}</Badge>);
    const span = container.querySelector("span.badge");
    expect(span).not.toBeNull();
    expect(span!.textContent).toBe("");
    unmount();
  });

  it("dot=false (默认) 时不渲染 .badge-dot", () => {
    const { container, unmount } = mount(<Badge>x</Badge>);
    expect(container.querySelector(".badge-dot")).toBeNull();
    unmount();
  });

  // ---- 边缘 case ----
  it("className 透传, 和默认 class 叠加", () => {
    const { container, unmount } = mount(
      <Badge variant="info" className="custom-extra">
        x
      </Badge>,
    );
    const span = container.querySelector("span.badge")!;
    const classes = span.className.split(" ");
    expect(classes).toContain("badge");
    expect(classes).toContain("badge-info");
    expect(classes).toContain("custom-extra");
    unmount();
  });

  it("额外 HTML 属性 (title / data-*) 透传到 span", () => {
    const { container, unmount } = mount(
      <Badge title="mouse-tip" data-testid="b1">
        x
      </Badge>,
    );
    const span = container.querySelector("span.badge")!;
    expect(span.getAttribute("title")).toBe("mouse-tip");
    expect(span.getAttribute("data-testid")).toBe("b1");
    unmount();
  });

  it("dot + children 同时出现时, dot 在前, children 紧随其后", () => {
    const { container, unmount } = mount(
      <Badge dot>
        <span data-testid="inner">label</span>
      </Badge>,
    );
    const badge = container.querySelector(".badge")!;
    const children = Array.from(badge.childNodes);
    expect(children.length).toBeGreaterThanOrEqual(2);
    expect((children[0] as HTMLElement).classList?.contains("badge-dot")).toBe(
      true,
    );
    unmount();
  });
});
