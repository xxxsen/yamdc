// @vitest-environment jsdom

import { act } from "react";
import { describe, expect, it, vi } from "vitest";

import { TokenEditor } from "@/components/ui/token-editor";

import { mount, rerender } from "./test-helpers";

// 小工具: 手动触发 input 的 change event。React onChange 拦截的是 React
// 自己代理的 value setter, 直接 input.value = "x" 会被 React 当作未变化
// (内部对比 lastKnownValue)。正确做法是用 prototype 的原生 setter 绕过
// React 的 shim, 再派发 input 事件 - 这是 react 社区测试 input 的惯用法。
function typeInto(input: HTMLInputElement, value: string) {
  const descriptor = Object.getOwnPropertyDescriptor(
    window.HTMLInputElement.prototype,
    "value",
  );
  if (!descriptor?.set) throw new Error("native input setter missing");
  act(() => {
    // 解引用 descriptor.set 以调用 native setter, 通过 .call(input, ...)
    // 显式绑定 this, 是安全的惯用法。
    descriptor.set.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function keyDown(input: HTMLInputElement, key: string) {
  act(() => {
    input.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

describe("TokenEditor", () => {
  // ---- 正常 case ----
  it("渲染 label / 已有 tokens / 空 input", () => {
    const onChange = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="演员"
        placeholder="请输入"
        value={["a", "b"]}
        onChange={onChange}
      />,
    );
    expect(container.querySelector(".review-label")!.textContent).toBe("演员");
    const chips = container.querySelectorAll(".token-chip");
    expect(chips).toHaveLength(2);
    expect(chips[0].textContent).toContain("a");
    expect(chips[1].textContent).toContain("b");
    const input = container.querySelector("input") as HTMLInputElement;
    expect(input.id).toBe("t-演员");
    expect(input.placeholder).toBe("");
    unmount();
  });

  it("value 为空时 input.placeholder 显示 placeholder prop", () => {
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="演员"
        placeholder="请输入"
        value={[]}
        onChange={() => {}}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    expect(input.placeholder).toBe("请输入");
    unmount();
  });

  it("Enter 键提交 draft, 触发 onChange + onCommit", () => {
    const onChange = vi.fn();
    const onCommit = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={["old"]}
        onChange={onChange}
        onCommit={onCommit}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    typeInto(input, "new");
    keyDown(input, "Enter");
    expect(onChange).toHaveBeenCalledWith(["old", "new"]);
    expect(onCommit).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("blur 时提交当前 draft 并触发 onCommit", () => {
    const onChange = vi.fn();
    const onCommit = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={[]}
        onChange={onChange}
        onCommit={onCommit}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    typeInto(input, "x");
    // React 18+ 把 blur 委托挂在 root 级别监听 focusout (blur 本身不冒泡,
    // 需要用 focusout 或调用 input.blur() 才能走到 React handler)。
    act(() => {
      input.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
    });
    expect(onChange).toHaveBeenCalledWith(["x"]);
    expect(onCommit).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("逗号分隔输入: 一次可切出多个 token, 最后一段留在 draft", () => {
    const onChange = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={["a"]}
        onChange={onChange}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    typeInto(input, "b,c,d");
    // 切出 b / c (trim后), d 作为 draft 留在 input
    expect(onChange).toHaveBeenCalledWith(["a", "b", "c"]);
    expect(input.value).toBe("d");
    unmount();
  });

  it("Backspace 空 draft + 非空 value: 删除最后一个 token", () => {
    const onChange = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={["a", "b", "c"]}
        onChange={onChange}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    keyDown(input, "Backspace");
    expect(onChange).toHaveBeenCalledWith(["a", "b"]);
    unmount();
  });

  it("chip 的删除按钮: 触发 onChange (去掉该项) + onCommit", () => {
    const onChange = vi.fn();
    const onCommit = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={["a", "b", "c"]}
        onChange={onChange}
        onCommit={onCommit}
      />,
    );
    const removeBtns = container.querySelectorAll<HTMLButtonElement>(
      ".token-chip-remove",
    );
    act(() => {
      removeBtns[1].click();
    });
    expect(onChange).toHaveBeenCalledWith(["a", "c"]);
    expect(onCommit).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("未传 onCommit 时, Enter / blur / remove 都不报错", () => {
    const onChange = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={["a"]}
        onChange={onChange}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    typeInto(input, "b");
    keyDown(input, "Enter");
    expect(onChange).toHaveBeenCalledWith(["a", "b"]);
    const removeBtn = container.querySelector(
      ".token-chip-remove",
    ) as HTMLButtonElement;
    act(() => {
      removeBtn.click();
    });
    // 两次 onChange: 一次 Enter 提交, 一次 remove
    expect(onChange).toHaveBeenCalledTimes(2);
    unmount();
  });

  // ---- 异常 case ----
  it("空 draft 按 Enter 不产生新 token, 但仍触发 onCommit", () => {
    const onChange = vi.fn();
    const onCommit = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={["a"]}
        onChange={onChange}
        onCommit={onCommit}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    keyDown(input, "Enter");
    expect(onChange).not.toHaveBeenCalled();
    // 保持和旧版行为一致: onCommit 在 Enter 时总会被调用一次 (即便 draft 空),
    // 因为外层 commitDraft 返回后逻辑不区分是否真提交了 token。
    expect(onCommit).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("Backspace 空 value + 空 draft: 不 crash, 不触发 onChange", () => {
    const onChange = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={[]}
        onChange={onChange}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    keyDown(input, "Backspace");
    expect(onChange).not.toHaveBeenCalled();
    unmount();
  });

  it("非 Enter / Backspace 键不触发 onCommit", () => {
    const onChange = vi.fn();
    const onCommit = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={[]}
        onChange={onChange}
        onCommit={onCommit}
      />,
    );
    const input = container.querySelector("input") as HTMLInputElement;
    keyDown(input, "a");
    keyDown(input, "Tab");
    expect(onCommit).not.toHaveBeenCalled();
    unmount();
  });

  // ---- 边缘 case ----
  it("readOnly=true: 不渲染 input / delete 按钮, value 空时显示 placeholder span", () => {
    const onChange = vi.fn();
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="暂无数据"
        value={[]}
        onChange={onChange}
        readOnly
      />,
    );
    expect(container.querySelector("input")).toBeNull();
    expect(container.querySelector(".token-chip-remove")).toBeNull();
    const muted = container.querySelector(".library-inline-muted")!;
    expect(muted.textContent).toBe("暂无数据");
    unmount();
  });

  it("readOnly=true 且 value 非空: 渲染 chip 但不渲染 remove 按钮", () => {
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={["a", "b"]}
        onChange={() => {}}
        readOnly
      />,
    );
    const chips = container.querySelectorAll(".token-chip");
    expect(chips).toHaveLength(2);
    expect(container.querySelector(".token-chip-remove")).toBeNull();
    expect(container.querySelector(".library-inline-muted")).toBeNull();
    unmount();
  });

  it("singleLine=true: token-editor 容器追加 token-editor-single-line class", () => {
    const { container, unmount } = mount(
      <TokenEditor
        idPrefix="t"
        label="l"
        placeholder="p"
        value={[]}
        onChange={() => {}}
        singleLine
      />,
    );
    const tokenEditor = container.querySelector(".token-editor")!;
    expect(tokenEditor.className).toContain("token-editor-single-line");
    unmount();
  });

  it("同 label 但不同 idPrefix: input.id 不冲突", () => {
    const { container: c1, unmount: u1 } = mount(
      <TokenEditor idPrefix="a" label="演员" placeholder="p" value={[]} onChange={() => {}} />,
    );
    const { container: c2, unmount: u2 } = mount(
      <TokenEditor idPrefix="b" label="演员" placeholder="p" value={[]} onChange={() => {}} />,
    );
    expect((c1.querySelector("input") as HTMLInputElement).id).toBe("a-演员");
    expect((c2.querySelector("input") as HTMLInputElement).id).toBe("b-演员");
    u1();
    u2();
  });

  it("props value 变化后重新渲染, chips 数量跟随更新", () => {
    const { container, root, unmount } = mount(
      <TokenEditor idPrefix="t" label="l" placeholder="p" value={["a"]} onChange={() => {}} />,
    );
    expect(container.querySelectorAll(".token-chip")).toHaveLength(1);
    rerender(
      root,
      <TokenEditor idPrefix="t" label="l" placeholder="p" value={["a", "b", "c"]} onChange={() => {}} />,
    );
    expect(container.querySelectorAll(".token-chip")).toHaveLength(3);
    unmount();
  });
});
