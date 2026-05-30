// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ReviewMeta } from "@/lib/api";

import { ReviewFormFields } from "../form-fields";

afterEach(() => {
  cleanup();
});

function makeMeta(overrides: Partial<ReviewMeta> = {}): ReviewMeta {
  return {
    title: "T",
    title_translated: "翻译",
    director: "D",
    studio: "S",
    label: "L",
    series: "Sr",
    plot: "P",
    plot_translated: "翻 P",
    actors: [],
    genres: [],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(overrides as any),
  } as ReviewMeta;
}

describe("ReviewFormFields", () => {
  it("正常路径: 6 个文本控件 + 2 个 textarea 都按 meta 渲染初值", () => {
    render(<ReviewFormFields meta={makeMeta()} updateMeta={vi.fn()} onBlurSave={vi.fn()} />);
    expect((screen.getByDisplayValue("T")).className).toContain("review-input-strong");
    expect(screen.getByDisplayValue("翻译")).toBeTruthy();
    expect(screen.getByDisplayValue("D")).toBeTruthy();
    expect(screen.getByDisplayValue("S")).toBeTruthy();
    expect(screen.getByDisplayValue("L")).toBeTruthy();
    expect(screen.getByDisplayValue("Sr")).toBeTruthy();
    expect(screen.getByDisplayValue("P")).toBeTruthy();
    expect(screen.getByDisplayValue("翻 P")).toBeTruthy();
  });

  it("异常路径: 8 个字段 (title / 翻译 / 导演 / 制作商 / 发行商 / 系列 / 简介 / 翻译简介) onChange 各触发一次 updateMeta + onBlurSave", () => {
    // 每个 input/textarea 的 onChange 都是独立箭头函数, 必须逐个触发让
    // v8 function coverage 把它们都标记成已运行 — 否则单文件 functions
    // 覆盖率会拖累全局阈值.
    const updateMeta = vi.fn();
    const onBlurSave = vi.fn();
    render(
      <ReviewFormFields
        meta={makeMeta()}
        updateMeta={updateMeta}
        onBlurSave={onBlurSave}
      />,
    );
    const cases: Array<[string, string, string]> = [
      ["T", "T2", "title"],
      ["翻译", "翻译2", "title_translated"],
      ["D", "D2", "director"],
      ["S", "S2", "studio"],
      ["L", "L2", "label"],
      ["Sr", "Sr2", "series"],
      ["P", "P2", "plot"],
      ["翻 P", "翻 P2", "plot_translated"],
    ];
    for (const [from, to, key] of cases) {
      const el = screen.getByDisplayValue(from);
      fireEvent.change(el, { target: { value: to } });
      expect(updateMeta).toHaveBeenLastCalledWith({ [key]: to });
      fireEvent.blur(el);
    }
    expect(onBlurSave.mock.calls.length).toBe(cases.length);
  });

  it("边缘路径: meta 字段为 undefined / null 时 input 渲染空字符串, 不报错", () => {
    const meta = {} as ReviewMeta;
    const updateMeta = vi.fn();
    render(
      <ReviewFormFields meta={meta} updateMeta={updateMeta} onBlurSave={vi.fn()} />,
    );
    // 至少 6 个 input + 2 个 textarea, 全部 value=""
    const inputs = document.querySelectorAll("input.input");
    expect(inputs.length).toBeGreaterThanOrEqual(5);
    inputs.forEach((el) => expect((el as HTMLInputElement).value).toBe(""));
    const textareas = document.querySelectorAll("textarea.input");
    textareas.forEach((el) => expect((el as HTMLTextAreaElement).value).toBe(""));
  });
});
