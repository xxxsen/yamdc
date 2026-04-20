// @vitest-environment jsdom

import { act } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { JobItem, NumberVariantDescriptor, NumberVariantSelection } from "@/lib/api";

import { mount } from "@/components/ui/__tests__/test-helpers";

import { NumberEditModal } from "../number-edit-modal";

// NumberEditModal 的目标是让用户用 "base + 变体按钮" 结构化地填番号, 而不是
// 拼字符串。本测试覆盖:
//   1. 从已有 job.number (含 variant 后缀) 正确预填 base 与 variant 勾选状态;
//   2. 预览是跟随 base / variant 状态实时重算的;
//   3. flag chip 点击可切换, indexed 类型支持启用/禁用 + index 输入;
//   4. 保存按钮最终提交结构化 payload, 取消按钮直接关闭;
//   5. 边缘: 空 base 时不可提交, descriptorsError 会显示告警。

const DEFAULT_DESCRIPTORS: NumberVariantDescriptor[] = [
  { id: "chinese-subtitle", suffix: "C", label: "中字", description: "中文字幕", kind: "flag" },
  { id: "resolution-4k", suffix: "4K", label: "4K", description: "4K 画质", kind: "flag" },
  { id: "multi-cd", suffix: "CD", label: "多盘", description: "多盘 CD", kind: "indexed", min: 1, max: 10 },
];

// 测试里只用到 JobItem 的 id / number 两个字段, cast 以避开 JobItem 其它
// 大量无关字段的造数; 保持测试本身 focused。
function makeJob(number: string, id = 1): JobItem {
  return { id, number } as unknown as JobItem;
}

function renderModal(props: {
  job?: JobItem;
  descriptors?: NumberVariantDescriptor[];
  isSubmitting?: boolean;
  descriptorsError?: string;
  onClose?: () => void;
  onSubmit?: (base: string, selections: NumberVariantSelection[]) => void;
}) {
  const onClose = props.onClose ?? vi.fn();
  const onSubmit = props.onSubmit ?? vi.fn();
  const { unmount } = mount(
    <NumberEditModal
      job={props.job ?? makeJob("")}
      descriptors={props.descriptors ?? DEFAULT_DESCRIPTORS}
      isSubmitting={props.isSubmitting ?? false}
      descriptorsError={props.descriptorsError}
      onClose={onClose}
      onSubmit={onSubmit}
    />,
  );
  return { unmount, onClose, onSubmit };
}

afterEach(() => {
  document
    .querySelectorAll(".plugin-editor-modal-backdrop")
    .forEach((el) => el.remove());
  document.body.style.overflow = "";
});

function findModalButtonByText(text: string): HTMLButtonElement {
  const buttons = Array.from(document.querySelectorAll("button"));
  const matched = buttons.find((b) => b.textContent?.includes(text));
  if (!matched) {
    throw new Error(`button not found: ${text}`);
  }
  return matched;
}

// React onChange 不认 "直接改 value.propery 再 dispatch input" 的路径,
// 必须通过原生 prototype setter 绕过 React 的 shim. 这是 react 社区测试
// 受控 input 的惯用写法, 参见 token-editor.test.tsx。
function typeInto(input: HTMLInputElement, value: string) {
  const descriptor = Object.getOwnPropertyDescriptor(
    window.HTMLInputElement.prototype,
    "value",
  );
  if (!descriptor?.set) throw new Error("native input setter missing");
  act(() => {
    descriptor.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

describe("NumberEditModal", () => {
  it("预填: job.number='PXVR-406-4K-CD2' 时 base 与 variant 勾选状态正确", () => {
    const { unmount } = renderModal({
      job: makeJob("PXVR-406-4K-CD2"),
    });

    const baseInput = document.querySelector(".number-edit-modal .input") as HTMLInputElement;
    expect(baseInput.value).toBe("PXVR-406");

    const preview = document.querySelector(".number-edit-preview-value")!;
    expect(preview.textContent).toBe("PXVR-406-4K-CD2");

    const chips = Array.from(document.querySelectorAll(".number-edit-variant-chip"));
    // "4K" chip 应为 active
    const fourK = chips.find((c) => (c as HTMLElement).textContent?.includes("4K")) as HTMLElement;
    expect(fourK.getAttribute("data-active")).toBe("true");

    // CD indexed chip active, index=2
    const cd = chips.find((c) => (c as HTMLElement).textContent?.includes("多盘")) as HTMLElement;
    expect(cd.getAttribute("data-active")).toBe("true");
    const cdIndex = cd.querySelector(".number-edit-variant-index-input") as HTMLInputElement;
    expect(cdIndex.value).toBe("2");

    unmount();
  });

  it("toggle flag: 点击 4K chip 切换 active + preview 同步更新", () => {
    const { unmount } = renderModal({
      job: makeJob("ABC-001"),
    });

    const chips = Array.from(document.querySelectorAll(".number-edit-variant-chip"));
    const fourK = chips.find((c) => (c as HTMLElement).textContent?.includes("4K")) as HTMLElement;
    expect(fourK.getAttribute("data-active")).toBe("false");

    act(() => {
      (fourK as HTMLButtonElement).click();
    });
    expect(fourK.getAttribute("data-active")).toBe("true");
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("ABC-001-4K");

    act(() => {
      (fourK as HTMLButtonElement).click();
    });
    expect(fourK.getAttribute("data-active")).toBe("false");
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("ABC-001");

    unmount();
  });

  it("indexed: 勾选 CD + 修改 index 会反映到预览; 取消勾选则预览回到 base", () => {
    const { unmount } = renderModal({
      job: makeJob("ABC-001"),
    });

    const chips = Array.from(document.querySelectorAll(".number-edit-variant-chip"));
    const cd = chips.find((c) => (c as HTMLElement).textContent?.includes("多盘")) as HTMLElement;
    const checkbox = cd.querySelector('input[type="checkbox"]') as HTMLInputElement;
    const numberInput = cd.querySelector(".number-edit-variant-index-input") as HTMLInputElement;

    act(() => {
      checkbox.click();
    });
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("ABC-001-CD1");

    typeInto(numberInput, "3");
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("ABC-001-CD3");

    act(() => {
      checkbox.click();
    });
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("ABC-001");

    unmount();
  });

  it("保存: 按当前 state 提交结构化 payload (base + 选中的 variants)", () => {
    const onSubmit = vi.fn();
    const { unmount } = renderModal({
      job: makeJob("ABC-001-C-CD2"),
      onSubmit,
    });

    const saveBtn = findModalButtonByText("保存");
    act(() => {
      saveBtn.click();
    });

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const [base, selections] = onSubmit.mock.calls[0];
    expect(base).toBe("ABC-001");
    expect(selections).toEqual(
      expect.arrayContaining([
        { id: "chinese-subtitle" },
        { id: "multi-cd", index: 2 },
      ]),
    );
    expect(selections.length).toBe(2);

    unmount();
  });

  it("取消: 点击取消按钮触发 onClose, 不调用 onSubmit", () => {
    const onSubmit = vi.fn();
    const onClose = vi.fn();
    const { unmount } = renderModal({
      job: makeJob("ABC-001"),
      onClose,
      onSubmit,
    });

    const cancelBtn = findModalButtonByText("取消");
    act(() => {
      cancelBtn.click();
    });
    expect(onClose).toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();

    unmount();
  });

  it("空 base: 保存按钮 disabled, preview 给占位文案", () => {
    const onSubmit = vi.fn();
    const { unmount } = renderModal({
      job: makeJob(""),
      onSubmit,
    });
    const saveBtn = findModalButtonByText("保存");
    expect(saveBtn.disabled).toBe(true);
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("(请填写基础番号)");

    act(() => {
      saveBtn.click();
    });
    expect(onSubmit).not.toHaveBeenCalled();

    unmount();
  });

  it("descriptorsError 传值时显示告警条", () => {
    const { unmount } = renderModal({
      job: makeJob("ABC-001"),
      descriptorsError: "network down",
    });
    const err = document.querySelector(".number-edit-error");
    expect(err).not.toBeNull();
    expect(err!.textContent).toContain("network down");
    unmount();
  });

  it("Enter 键: 在 base 输入框按回车触发 onSubmit", () => {
    const onSubmit = vi.fn();
    const { unmount } = renderModal({
      job: makeJob("ABC-001"),
      onSubmit,
    });
    const baseInput = document.querySelector(".number-edit-modal .input") as HTMLInputElement;
    act(() => {
      baseInput.focus();
      baseInput.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    });
    expect(onSubmit).toHaveBeenCalled();
    unmount();
  });
});
