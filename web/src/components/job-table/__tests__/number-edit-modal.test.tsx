// @vitest-environment jsdom

import { act } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { JobItem, NumberVariantDescriptor, NumberVariantSelection } from "@/lib/api";

import { mount } from "@/components/ui/__tests__/test-helpers";

import { NumberEditModal } from "../number-edit-modal";

// NumberEditModal 的目标是让用户用 "base + 变体按钮" 结构化地填影片 ID, 而不是
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
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("(请填写基础影片 ID)");

    act(() => {
      saveBtn.click();
    });
    expect(onSubmit).not.toHaveBeenCalled();

    unmount();
  });

  it("chip 只展示 label, suffix (如 -C / -LEAK) 通过 title 提供给 hover", () => {
    const { unmount } = renderModal({
      job: makeJob("ABC-001"),
      descriptors: [
        { id: "chinese-subtitle", suffix: "C", label: "中字", description: "中文字幕", kind: "flag" },
        { id: "leak-edition", suffix: "LEAK", label: "特别版", description: "特别版泄漏", kind: "flag" },
      ],
    });
    const chips = Array.from(document.querySelectorAll(".number-edit-variant-chip"));
    const cn = chips.find((c) => c.textContent?.trim() === "中字");
    const leak = chips.find((c) => c.textContent?.trim() === "特别版");
    expect(cn).toBeDefined();
    expect(leak).toBeDefined();
    // 正文不出现 "-C" / "-LEAK", 但 title 里能看到对应 suffix
    expect(cn!.textContent).not.toContain("-C");
    expect(leak!.textContent).not.toContain("-LEAK");
    expect(cn!.getAttribute("title")).toContain("-C");
    expect(leak!.getAttribute("title")).toContain("-LEAK");
    expect(cn!.getAttribute("title")).toContain("中文字幕");
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

  it("group mutex (flag): 勾选 8K 会自动取消已勾选的 4K, 并更新 preview", () => {
    // 互斥分组 "resolution" 下: 4K / 8K 同时只能存在一个。点 8K 时 4K 自动消失。
    const descriptorsWithGroup: NumberVariantDescriptor[] = [
      {
        id: "resolution_4k",
        suffix: "4K",
        label: "4K",
        description: "4K 分辨率",
        kind: "flag",
        group: "resolution",
      },
      {
        id: "resolution_8k",
        suffix: "8K",
        label: "8K",
        description: "8K 分辨率",
        kind: "flag",
        group: "resolution",
      },
      // 同时放一个独立 variant, 验证它不会受互斥影响。
      { id: "chinese-subtitle", suffix: "C", label: "中字", description: "中文字幕", kind: "flag" },
    ];
    const { unmount } = renderModal({
      job: makeJob("ABC-001"),
      descriptors: descriptorsWithGroup,
    });

    const chips = Array.from(document.querySelectorAll<HTMLElement>(".number-edit-variant-chip"));
    const fourK = chips.find((c) => c.textContent?.trim() === "4K")!;
    const eightK = chips.find((c) => c.textContent?.trim() === "8K")!;
    const cn = chips.find((c) => c.textContent?.trim() === "中字")!;

    act(() => {
      fourK.click();
    });
    expect(fourK.getAttribute("data-active")).toBe("true");
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("ABC-001-4K");

    // 独立 variant 勾选: 不应被 resolution 组互斥牵连。
    act(() => {
      cn.click();
    });
    expect(cn.getAttribute("data-active")).toBe("true");

    // 切到 8K: 4K 必须被自动取消, 中字保持不变。
    act(() => {
      eightK.click();
    });
    expect(fourK.getAttribute("data-active")).toBe("false");
    expect(eightK.getAttribute("data-active")).toBe("true");
    expect(cn.getAttribute("data-active")).toBe("true");
    // preview 按 descriptor 声明顺序拼装 (4K / 8K 之后才是 C), 所以这里是
    // ABC-001-8K-C 而不是 ABC-001-C-8K。
    expect(document.querySelector(".number-edit-preview-value")!.textContent).toBe("ABC-001-8K-C");

    unmount();
  });

  it("group mutex (parse): 遗留数据 'ABC-001-4K-8K' 只保留其中一个, UI 不会展示双勾", () => {
    // 老数据可能存在 pre-mutex 时期写入的 "ABC-001-4K-8K"。parseExistingNumber
    // 会按 "末尾 suffix 胜出" 的规则去重, 保留 8K (更贴近用户最近一次意图),
    // 避免打开 modal 就看到两个 chip 同时 active / 保存时被后端 400。
    const descriptorsWithGroup: NumberVariantDescriptor[] = [
      {
        id: "resolution_4k",
        suffix: "4K",
        label: "4K",
        description: "4K 分辨率",
        kind: "flag",
        group: "resolution",
      },
      {
        id: "resolution_8k",
        suffix: "8K",
        label: "8K",
        description: "8K 分辨率",
        kind: "flag",
        group: "resolution",
      },
    ];
    const { unmount } = renderModal({
      job: makeJob("ABC-001-4K-8K"),
      descriptors: descriptorsWithGroup,
    });

    const chips = Array.from(document.querySelectorAll<HTMLElement>(".number-edit-variant-chip"));
    const fourK = chips.find((c) => c.textContent?.trim() === "4K")!;
    const eightK = chips.find((c) => c.textContent?.trim() === "8K")!;

    // 组内只能有一个 active。具体哪个无所谓 (去重规则是实现细节), 这里只锚
    // "二者不会同时 active" 这个不变量, 测试更稳。
    const activeCount = [fourK, eightK].filter((c) => c.getAttribute("data-active") === "true").length;
    expect(activeCount).toBe(1);
    // preview 也必须只有一个分辨率后缀, 不能出现 "-4K-8K"。
    const preview = document.querySelector(".number-edit-preview-value")!.textContent ?? "";
    expect(preview.includes("-4K") && preview.includes("-8K")).toBe(false);
    expect(preview).toMatch(/ABC-001-(4K|8K)/);

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
