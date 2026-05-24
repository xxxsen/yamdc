// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { LibraryVariant } from "@/lib/api";

import { LibraryVariantSwitcher } from "../variant-switcher";

afterEach(() => {
  cleanup();
});

const variants: LibraryVariant[] = [
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  { key: "v1", label: "中字", base_name: "foo-v1" } as any,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  { key: "v2", label: "原版", base_name: "foo-v2" } as any,
];

describe("LibraryVariantSwitcher", () => {
  it("正常路径: 渲染所有 variants + label/meta; 点击切换调 onSelect(key)", () => {
    const onSelect = vi.fn();
    render(<LibraryVariantSwitcher variants={variants} currentKey="v1" onSelect={onSelect} />);
    expect(screen.getByText("中字")).toBeTruthy();
    expect(screen.getByText("原版")).toBeTruthy();
    fireEvent.click(screen.getByText("原版"));
    expect(onSelect).toHaveBeenCalledWith("v2");
  });

  it("异常路径: currentKey 不在 variants 中 → 所有 chip 都 data-active='false'", () => {
    render(<LibraryVariantSwitcher variants={variants} currentKey="ghost" onSelect={vi.fn()} />);
    const chips = document.querySelectorAll(".library-variant-chip");
    chips.forEach((c) => expect(c.getAttribute("data-active")).toBe("false"));
  });

  it("边缘路径: extraClassName 追加到 panel; variants 空时只渲染容器无 chip", () => {
    const { container, rerender } = render(
      <LibraryVariantSwitcher
        variants={variants}
        currentKey="v1"
        onSelect={vi.fn()}
        extraClassName="custom-class"
      />,
    );
    expect((container.querySelector(".library-variant-panel") as HTMLElement).className).toContain("custom-class");
    rerender(<LibraryVariantSwitcher variants={[]} currentKey="" onSelect={vi.fn()} />);
    expect(container.querySelectorAll(".library-variant-chip").length).toBe(0);
  });
});
