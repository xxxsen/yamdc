// @vitest-environment jsdom

import { act } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Modal } from "@/components/ui/modal";

import { mount, rerender } from "./test-helpers";

describe("Modal", () => {
  afterEach(() => {
    // 防御: 某个用例如果 assert 失败跳过了 unmount, 确保下一个用例
    // 不会看到残留的 body.style.overflow / portal 节点。
    document.body.style.overflow = "";
    document
      .querySelectorAll(".plugin-editor-modal-backdrop")
      .forEach((el) => el.remove());
    // bare 模式用自定义 backdrop class, 也要兜底清掉。
    document
      .querySelectorAll("[data-bare-backdrop]")
      .forEach((el) => el.remove());
  });

  // ---- 正常 case ----
  it("open=true 时通过 portal 渲染到 document.body, 显示 title / children", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal open onClose={onClose} title="导入">
        <p data-testid="body-text">hello</p>
      </Modal>,
    );
    const backdrop = document.querySelector(".plugin-editor-modal-backdrop");
    expect(backdrop).not.toBeNull();
    const dialog = document.querySelector('[role="dialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog!.getAttribute("aria-modal")).toBe("true");
    expect(dialog!.querySelector("h3")!.textContent).toBe("导入");
    expect(
      document.querySelector('[data-testid="body-text"]')!.textContent,
    ).toBe("hello");
    unmount();
  });

  it("subtitle 有值时渲染 head span, 无值时不渲染", () => {
    const { unmount } = mount(
      <Modal open onClose={vi.fn()} title="T" subtitle="副标题">
        <div />
      </Modal>,
    );
    const head = document.querySelector(".plugin-editor-modal-title-copy")!;
    expect(head.querySelector("span")!.textContent).toBe("副标题");
    unmount();

    const { unmount: u2 } = mount(
      <Modal open onClose={vi.fn()} title="T">
        <div />
      </Modal>,
    );
    const head2 = document.querySelector(".plugin-editor-modal-title-copy")!;
    expect(head2.querySelector("span")).toBeNull();
    u2();
  });

  it("badge + actions 渲染到对应插槽", () => {
    const { unmount } = mount(
      <Modal
        open
        onClose={vi.fn()}
        title="T"
        badge={{ icon: <i data-testid="bi" />, label: "BADGE" }}
        actions={<button data-testid="ok">ok</button>}
      >
        <div />
      </Modal>,
    );
    expect(document.querySelector('[data-testid="bi"]')).not.toBeNull();
    expect(
      document.querySelector(".plugin-editor-modal-badge")!.textContent,
    ).toContain("BADGE");
    expect(document.querySelector('[data-testid="ok"]')).not.toBeNull();
    unmount();
  });

  it("ESC 键触发 onClose", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal open onClose={onClose} title="T">
        <div />
      </Modal>,
    );
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    expect(onClose).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("backdrop 点击触发 onClose, 内容区点击不触发", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal open onClose={onClose} title="T">
        <div data-testid="inner" />
      </Modal>,
    );
    const backdrop = document.querySelector(
      ".plugin-editor-modal-backdrop",
    ) as HTMLDivElement;
    act(() => {
      backdrop.click();
    });
    expect(onClose).toHaveBeenCalledTimes(1);

    const inner = document.querySelector(
      "[data-testid='inner']",
    ) as HTMLDivElement;
    act(() => {
      inner.click();
    });
    // inner click 冒泡到 dialog 被 stopPropagation 拦下, 不再触发
    expect(onClose).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("close 按钮点击触发 onClose", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal open onClose={onClose} title="T">
        <div />
      </Modal>,
    );
    const closeBtn = document.querySelector(
      ".plugin-editor-modal-close",
    ) as HTMLButtonElement;
    expect(closeBtn).not.toBeNull();
    act(() => {
      closeBtn.click();
    });
    expect(onClose).toHaveBeenCalledTimes(1);
    unmount();
  });

  // ---- 异常 case ----
  it("open=false 时不渲染任何 backdrop / dialog", () => {
    const { unmount } = mount(
      <Modal open={false} onClose={vi.fn()} title="T">
        <div />
      </Modal>,
    );
    expect(
      document.querySelector(".plugin-editor-modal-backdrop"),
    ).toBeNull();
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    unmount();
  });

  it("disableClose=true 时不渲染关闭按钮, ESC 和 backdrop 点击都不触发 onClose", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal open onClose={onClose} title="T" disableClose>
        <div />
      </Modal>,
    );
    expect(document.querySelector(".plugin-editor-modal-close")).toBeNull();

    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    const backdrop = document.querySelector(
      ".plugin-editor-modal-backdrop",
    ) as HTMLDivElement;
    act(() => {
      backdrop.click();
    });
    expect(onClose).not.toHaveBeenCalled();
    unmount();
  });

  it("closeOnEscape=false 时 ESC 不触发 onClose, 但 backdrop 仍可关闭", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal open onClose={onClose} title="T" closeOnEscape={false}>
        <div />
      </Modal>,
    );
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    expect(onClose).not.toHaveBeenCalled();

    const backdrop = document.querySelector(
      ".plugin-editor-modal-backdrop",
    ) as HTMLDivElement;
    act(() => {
      backdrop.click();
    });
    expect(onClose).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("closeOnBackdrop=false 时点击背景不触发 onClose", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal open onClose={onClose} title="T" closeOnBackdrop={false}>
        <div />
      </Modal>,
    );
    const backdrop = document.querySelector(
      ".plugin-editor-modal-backdrop",
    ) as HTMLDivElement;
    act(() => {
      backdrop.click();
    });
    expect(onClose).not.toHaveBeenCalled();
    unmount();
  });

  it("非 Escape 键不触发 onClose", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal open onClose={onClose} title="T">
        <div />
      </Modal>,
    );
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
    });
    expect(onClose).not.toHaveBeenCalled();
    unmount();
  });

  // ---- 边缘 case ----
  it("open=true 时 body.style.overflow='hidden', unmount 后恢复原值", () => {
    document.body.style.overflow = "scroll";
    const { unmount } = mount(
      <Modal open onClose={vi.fn()} title="T">
        <div />
      </Modal>,
    );
    expect(document.body.style.overflow).toBe("hidden");
    unmount();
    expect(document.body.style.overflow).toBe("scroll");
    document.body.style.overflow = "";
  });

  it("open 从 false 切到 true 时才挂载 portal", () => {
    const onClose = vi.fn();
    const { root, unmount } = mount(
      <Modal open={false} onClose={onClose} title="T">
        <div />
      </Modal>,
    );
    expect(
      document.querySelector(".plugin-editor-modal-backdrop"),
    ).toBeNull();
    rerender(
      root,
      <Modal open onClose={onClose} title="T">
        <div />
      </Modal>,
    );
    expect(
      document.querySelector(".plugin-editor-modal-backdrop"),
    ).not.toBeNull();
    unmount();
  });

  it("className 追加到 dialog 容器上, 不覆盖内置 .plugin-editor-modal", () => {
    const { unmount } = mount(
      <Modal open onClose={vi.fn()} title="T" className="custom-dlg">
        <div />
      </Modal>,
    );
    const dialog = document.querySelector('[role="dialog"]')!;
    const cls = dialog.className.split(" ");
    expect(cls).toContain("panel");
    expect(cls).toContain("plugin-editor-modal");
    expect(cls).toContain("custom-dlg");
    unmount();
  });

  it("ariaLabel 提供时覆盖 title 作为 aria-label", () => {
    const { unmount } = mount(
      <Modal open onClose={vi.fn()} title="T" ariaLabel="custom-label">
        <div />
      </Modal>,
    );
    const dialog = document.querySelector('[role="dialog"]')!;
    expect(dialog.getAttribute("aria-label")).toBe("custom-label");
    unmount();
  });

  it("actions 未传时不渲染 actions 容器", () => {
    const { unmount } = mount(
      <Modal open onClose={vi.fn()} title="T">
        <div />
      </Modal>,
    );
    expect(
      document.querySelector(".plugin-editor-modal-actions"),
    ).toBeNull();
    unmount();
  });

  // ---- bare 模式 ----
  it("bare=true 时不渲染 plugin-editor-modal-head/body 骨架, children 直接挂到 frame", () => {
    const { unmount } = mount(
      <Modal
        open
        onClose={vi.fn()}
        bare
        backdropClassName="custom-backdrop"
        frameClassName="custom-frame"
        ariaLabel="no-header-modal"
      >
        <div data-testid="bare-content">raw-body</div>
      </Modal>,
    );
    expect(document.querySelector(".plugin-editor-modal-backdrop")).toBeNull();
    expect(document.querySelector(".plugin-editor-modal-head")).toBeNull();
    expect(document.querySelector(".plugin-editor-modal-body")).toBeNull();
    const backdrop = document.querySelector(".custom-backdrop")!;
    expect(backdrop).not.toBeNull();
    const dialog = backdrop.querySelector('[role="dialog"]')!;
    expect(dialog.className).toContain("custom-frame");
    expect(dialog.getAttribute("aria-label")).toBe("no-header-modal");
    expect(dialog.querySelector('[data-testid="bare-content"]')!.textContent).toBe("raw-body");
    unmount();
  });

  it("bare 模式下 ESC / backdrop 点击行为仍然生效", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <Modal
        open
        onClose={onClose}
        bare
        backdropClassName="bare-test-backdrop"
        frameClassName="bare-test-frame"
      >
        <div />
      </Modal>,
    );
    const backdrop = document.querySelector(".bare-test-backdrop") as HTMLElement;
    act(() => {
      backdrop.click();
    });
    expect(onClose).toHaveBeenCalledTimes(1);
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    expect(onClose).toHaveBeenCalledTimes(2);
    unmount();
  });

  it("非 bare 模式额外传 backdropClassName 时, 以追加方式与默认 class 叠加", () => {
    const { unmount } = mount(
      <Modal open onClose={vi.fn()} title="T" backdropClassName="extra-class">
        <div />
      </Modal>,
    );
    const backdrop = document.querySelector(".plugin-editor-modal-backdrop")!;
    expect(backdrop.className).toContain("extra-class");
    unmount();
  });
});
