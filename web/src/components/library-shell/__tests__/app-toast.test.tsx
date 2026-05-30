// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { AppToast } from "../app-toast";

afterEach(() => {
  cleanup();
});

describe("AppToast", () => {
  it("正常路径: message 非空 → 渲染 role=status + aria-live=polite", () => {
    render(<AppToast message="已自动保存" />);
    const el = screen.getByRole("status");
    expect(el.getAttribute("aria-live")).toBe("polite");
    expect(el.textContent).toContain("已自动保存");
    // tone 默认 undefined (info 语义).
    expect(el.getAttribute("data-tone")).toBeNull();
  });

  it("异常路径: 含 '失败' / 'error' → data-tone='danger'", () => {
    const { rerender } = render(<AppToast message="保存 NFO 失败" />);
    expect(screen.getByRole("status").getAttribute("data-tone")).toBe("danger");
    rerender(<AppToast message="Upload error" />);
    expect(screen.getByRole("status").getAttribute("data-tone")).toBe("danger");
  });

  it("边缘路径: message 为空字符串 → 不渲染任何 DOM", () => {
    const { container } = render(<AppToast message="" />);
    expect(container.firstChild).toBeNull();
  });
});
