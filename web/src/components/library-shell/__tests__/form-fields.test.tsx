// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { LibraryMeta } from "@/lib/api";

import { LibraryFormFields } from "../form-fields";

afterEach(() => {
  cleanup();
});

function makeMeta(overrides: Partial<LibraryMeta> = {}): LibraryMeta {
  return {
    title: "原T",
    title_translated: "翻T",
    plot: "原P",
    plot_translated: "翻P",
    director: "D",
    studio: "S",
    label: "L",
    series: "Sr",
    number: "ABC-001",
    release_date: "2024-01-01",
    runtime: 120,
    source: "src",
    actors: [],
    genres: [],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(overrides as any),
  } as LibraryMeta;
}

// 注意: 组件内部 onChange 的 lambda 形如
//   updateDraftMeta((prev) => ({ ...prev, [key]: e.target.value }));
// e.target.value 是惰性读取的, 在 fireEvent.change 之后由于
// controlled component 的 props 没有跟着变, 同步 re-render 会把 input.value
// 还原回 props.activeTitleValue. 所以用 mock.calls[N][0](currentMeta)
// 这种"事后拿 lambda 重放"的写法去校验 patch 时, e.target.value 已经被
// React 还原回旧值. 改成在调 updater 之前先 stub 一份 fake event:
function captureLastUpdater(spy: ReturnType<typeof vi.fn>) {
  const lambda = spy.mock.calls[spy.mock.calls.length - 1][0];
  spy.mockClear();
  return lambda;
}

describe("LibraryFormFields", () => {
  it("正常路径 (translated): 标题输入触发 updateDraftMeta, patch 写入 title_translated", () => {
    const updateDraftMeta = vi.fn();
    render(
      <LibraryFormFields
        draftMeta={makeMeta()}
        copyMode="translated"
        activeTitleValue="翻T"
        activePlotValue="翻P"
        updateDraftMeta={updateDraftMeta}
        onBlurSave={vi.fn()}
      />,
    );
    const titleInput = screen.getByDisplayValue("翻T");
    fireEvent.change(titleInput, { target: { value: "翻T2" } });
    // updater 一旦取出, 立刻拿当下输入框的 value 跑一次 — 这里模拟 React
    // setState reducer 执行时的状态.
    const updater = captureLastUpdater(updateDraftMeta);
    titleInput.value = "翻T2";
    const next = updater(makeMeta());
    expect(next.title_translated).toBe("翻T2");
    // 旧字段不应被覆盖.
    expect(next.title).toBe("原T");
  });

  it("异常路径 (original): 标题输入只更新 title, 不动 title_translated", () => {
    const updateDraftMeta = vi.fn();
    render(
      <LibraryFormFields
        draftMeta={makeMeta()}
        copyMode="original"
        activeTitleValue="原T"
        activePlotValue="原P"
        updateDraftMeta={updateDraftMeta}
        onBlurSave={vi.fn()}
      />,
    );
    const titleInput = screen.getByDisplayValue("原T");
    fireEvent.change(titleInput, { target: { value: "原T2" } });
    const updater = captureLastUpdater(updateDraftMeta);
    titleInput.value = "原T2";
    const next = updater(makeMeta());
    expect(next.title).toBe("原T2");
    expect(next.title_translated).toBe("翻T");
  });

  it("边缘路径: runtime 解析成数字 / 非数字字符兜底 0 / onBlur 触发 onBlurSave", () => {
    const updateDraftMeta = vi.fn();
    const onBlurSave = vi.fn();
    render(
      <LibraryFormFields
        draftMeta={makeMeta()}
        copyMode="translated"
        activeTitleValue="翻T"
        activePlotValue="翻P"
        updateDraftMeta={updateDraftMeta}
        onBlurSave={onBlurSave}
      />,
    );
    const runtimeInput = screen.getByDisplayValue("120");
    fireEvent.change(runtimeInput, { target: { value: "240" } });
    let updater = captureLastUpdater(updateDraftMeta);
    runtimeInput.value = "240";
    expect(updater(makeMeta()).runtime).toBe(240);

    fireEvent.change(runtimeInput, { target: { value: "abc" } });
    updater = captureLastUpdater(updateDraftMeta);
    runtimeInput.value = "abc";
    expect(updater(makeMeta()).runtime).toBe(0);

    fireEvent.blur(runtimeInput);
    expect(onBlurSave).toHaveBeenCalled();
  });

  it("剩余字段: director / studio / label / series / number / release_date / source / 简介 onChange + onBlur 全部接通 (function coverage)", () => {
    // 每个 input 的 onChange 是一份独立的箭头函数, 必须各自触发一次才能
    // 让 v8 function-coverage 识别为已执行. 这条测试的目标就是把 11 个
    // input/textarea 的 onChange + onBlur 都跑过, 让单文件 functions 阈值
    // ≥ 95%, 否则会拖累全局 coverage threshold.
    const updateDraftMeta = vi.fn();
    const onBlurSave = vi.fn();
    render(
      <LibraryFormFields
        draftMeta={makeMeta()}
        copyMode="translated"
        activeTitleValue="翻T"
        activePlotValue="翻P"
        updateDraftMeta={updateDraftMeta}
        onBlurSave={onBlurSave}
      />,
    );
    const fields: Array<[string, string, keyof ReturnType<typeof makeMeta>]> = [
      ["D", "D2", "director"],
      ["S", "S2", "studio"],
      ["L", "L2", "label"],
      ["Sr", "Sr2", "series"],
      ["ABC-001", "ZZZ-999", "number"],
      ["2024-01-01", "2025-02-02", "release_date"],
      ["src", "src2", "source"],
    ];
    for (const [from, to, key] of fields) {
      const el = screen.getByDisplayValue(from);
      fireEvent.change(el, { target: { value: to } });
      const updater = captureLastUpdater(updateDraftMeta);
      el.value = to;
      expect((updater(makeMeta()) as Record<string, unknown>)[key as string]).toBe(to);
      fireEvent.blur(el);
    }

    // 简介 textarea (translated 模式 → plot_translated).
    const plotArea = screen.getByDisplayValue("翻P");
    fireEvent.change(plotArea, { target: { value: "翻P2" } });
    const u = captureLastUpdater(updateDraftMeta);
    plotArea.value = "翻P2";
    expect(u(makeMeta()).plot_translated).toBe("翻P2");
    fireEvent.blur(plotArea);

    expect(onBlurSave.mock.calls.length).toBeGreaterThanOrEqual(8);
  });
});
