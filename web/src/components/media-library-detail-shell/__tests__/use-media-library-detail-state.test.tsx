// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LibraryMeta, MediaLibraryDetail } from "@/lib/api";

import { useMediaLibraryDetailState } from "../use-media-library-detail-state";

// useMediaLibraryDetailState 覆盖:
// - initial draftMeta 来自 initialDetail.meta 的 clone
// - selectedVariantKey 取 primary_variant_key 或首个 variant
// - 8s polling: 编辑态下不启动, 非编辑态下每 8s 调一次 getMediaLibraryItem
// - handleStartEdit: 清 message, 切到编辑态
// - handleSaveEdit: dirty 校验 + 成功 sync + 失败 setMessage 不退出编辑态
// - handleSaveEdit 未改动直接退出编辑态, 不发请求
// - handleCancelEdit: 回滚 draftMeta, 退出编辑态
// - message 自动清除 (非失败类文案 2.4s 后清掉)

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getMediaLibraryItem: vi.fn(),
    updateMediaLibraryItem: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockGetItem = vi.mocked(api.getMediaLibraryItem);
const mockUpdateItem = vi.mocked(api.updateMediaLibraryItem);

function makeMeta(overrides: Partial<LibraryMeta> = {}): LibraryMeta {
  return {
    title: "orig",
    title_translated: "",
    original_title: "",
    plot: "",
    plot_translated: "",
    number: "ABC-001",
    release_date: "",
    runtime: 0,
    studio: "",
    label: "",
    series: "",
    director: "",
    actors: [],
    genres: [],
    poster_path: "",
    cover_path: "",
    fanart_path: "",
    thumb_path: "",
    source: "",
    scraped_at: "",
    ...overrides,
  };
}

function makeDetail(overrides: Partial<MediaLibraryDetail> = {}): MediaLibraryDetail {
  return {
    item: {
      id: 10,
      rel_path: "a",
      name: "a",
      title: "orig",
      number: "ABC-001",
      release_date: "",
      actors: [],
      created_at: 0,
      updated_at: 0,
      has_nfo: true,
      poster_path: "",
      cover_path: "",
      total_size: 0,
      file_count: 1,
      video_count: 1,
      variant_count: 1,
      conflict: false,
    },
    meta: makeMeta(),
    variants: [
      { key: "v1", label: "v1", base_name: "", suffix: "", is_primary: true, video_path: "", nfo_path: "", poster_path: "", cover_path: "", meta: makeMeta(), files: [], file_count: 0 },
      { key: "v2", label: "v2", base_name: "", suffix: "", is_primary: false, video_path: "", nfo_path: "", poster_path: "", cover_path: "", meta: makeMeta(), files: [], file_count: 0 },
    ],
    primary_variant_key: "v1",
    files: [],
    ...overrides,
  };
}

async function flushAsync(ticks = 6) {
  await act(async () => {
    for (let i = 0; i < ticks; i += 1) {
      await Promise.resolve();
    }
  });
}

beforeEach(() => {
  mockGetItem.mockReset();
  mockUpdateItem.mockReset();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("initial state", () => {
  it("draftMeta 是 initialDetail.meta 的 clone (actors/genres 独立)", () => {
    const initial = makeDetail({ meta: makeMeta({ actors: ["A"], genres: ["G"] }) });
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: initial }),
    );
    expect(result.current.draftMeta.actors).toEqual(["A"]);
    initial.meta.actors.push("mutated");
    expect(result.current.draftMeta.actors).toEqual(["A"]);
  });

  it("selectedVariantKey 优先 primary_variant_key", () => {
    const initial = makeDetail({ primary_variant_key: "v2" });
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: initial }),
    );
    expect(result.current.selectedVariantKey).toBe("v2");
  });

  it("primary_variant_key 为空时回落到 variants[0].key", () => {
    const initial = makeDetail({ primary_variant_key: "" });
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: initial }),
    );
    expect(result.current.selectedVariantKey).toBe("v1");
  });

  it("primary_variant_key 和 variants 都为空时 selectedVariantKey 为空串", () => {
    const initial = makeDetail({ primary_variant_key: "", variants: [] });
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: initial }),
    );
    expect(result.current.selectedVariantKey).toBe("");
  });
});

describe("polling", () => {
  it("非编辑态每 8s 调一次 getMediaLibraryItem", async () => {
    mockGetItem.mockResolvedValue(makeDetail());
    const initial = makeDetail();
    renderHook(() => useMediaLibraryDetailState({ initialDetail: initial }));
    expect(mockGetItem).not.toHaveBeenCalled();
    await act(async () => {
      vi.advanceTimersByTime(8000);
    });
    expect(mockGetItem).toHaveBeenCalledTimes(1);
    expect(mockGetItem).toHaveBeenCalledWith(10);

    await act(async () => {
      vi.advanceTimersByTime(8000);
    });
    expect(mockGetItem).toHaveBeenCalledTimes(2);
  });

  it("进入编辑态后停止 polling", async () => {
    mockGetItem.mockResolvedValue(makeDetail());
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: makeDetail() }),
    );
    act(() => {
      result.current.handleStartEdit();
    });
    await act(async () => {
      vi.advanceTimersByTime(30000);
    });
    expect(mockGetItem).not.toHaveBeenCalled();
  });

  it("polling API 抛错被吞掉, hook 状态不变", async () => {
    mockGetItem.mockRejectedValue(new Error("x"));
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }) }),
    );
    await act(async () => {
      vi.advanceTimersByTime(8000);
    });
    await flushAsync();
    expect(result.current.detail.meta.title).toBe("orig");
  });
});

describe("handleStartEdit / handleCancelEdit", () => {
  it("handleStartEdit 清 message 并进入编辑态", () => {
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: makeDetail() }),
    );
    act(() => {
      result.current.handleStartEdit();
    });
    expect(result.current.isEditing).toBe(true);
    expect(result.current.message).toBe("");
  });

  it("handleCancelEdit 回滚 draftMeta + 退出编辑态", () => {
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }) }),
    );
    act(() => {
      result.current.handleStartEdit();
      result.current.updateDraftMeta((prev) => ({ ...prev, title: "changed" }));
    });
    expect(result.current.draftMeta.title).toBe("changed");
    act(() => {
      result.current.handleCancelEdit();
    });
    expect(result.current.draftMeta.title).toBe("orig");
    expect(result.current.isEditing).toBe(false);
  });
});

describe("handleSaveEdit", () => {
  it("draftMeta 未改动时直接退出编辑态, 不发请求", async () => {
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({ initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }) }),
    );
    act(() => {
      result.current.handleStartEdit();
    });
    act(() => {
      result.current.handleSaveEdit();
    });
    await flushAsync();
    expect(mockUpdateItem).not.toHaveBeenCalled();
    expect(result.current.isEditing).toBe(false);
  });

  it("draftMeta 有改动时调用 updateMediaLibraryItem 并 sync", async () => {
    const nextDetail = makeDetail({ meta: makeMeta({ title: "from-server" }) });
    mockUpdateItem.mockResolvedValue(nextDetail);
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({
        initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }),
      }),
    );
    act(() => {
      result.current.handleStartEdit();
      result.current.updateDraftMeta((prev) => ({ ...prev, title: "changed" }));
    });
    act(() => {
      result.current.handleSaveEdit();
    });
    await flushAsync();
    expect(mockUpdateItem).toHaveBeenCalledTimes(1);
    const [id, meta] = mockUpdateItem.mock.calls[0];
    expect(id).toBe(10);
    expect(meta.title).toBe("changed");
    expect(result.current.isEditing).toBe(false);
    expect(result.current.detail.meta.title).toBe("from-server");
  });

  it("updateMediaLibraryItem 失败时 setMessage 且保持编辑态", async () => {
    mockUpdateItem.mockRejectedValue(new Error("nfo write failed"));
    const onDetailChange = vi.fn();
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({
        initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }),
        onDetailChange,
      }),
    );
    act(() => {
      result.current.handleStartEdit();
      result.current.updateDraftMeta((prev) => ({ ...prev, title: "changed" }));
    });
    act(() => {
      result.current.handleSaveEdit();
    });
    await flushAsync();
    expect(result.current.message).toBe("nfo write failed");
    expect(result.current.isEditing).toBe(true);
    expect(onDetailChange).not.toHaveBeenCalled();
  });

  it("updateMediaLibraryItem 非 Error 错误使用兜底文案", async () => {
    mockUpdateItem.mockRejectedValue("raw");
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({
        initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }),
      }),
    );
    act(() => {
      result.current.handleStartEdit();
      result.current.updateDraftMeta((prev) => ({ ...prev, title: "changed" }));
    });
    act(() => {
      result.current.handleSaveEdit();
    });
    await flushAsync();
    expect(result.current.message).toBe("保存媒体库 NFO 失败");
  });

  it("save 成功会触发 onDetailChange 回调", async () => {
    const nextDetail = makeDetail({ meta: makeMeta({ title: "from-server" }) });
    mockUpdateItem.mockResolvedValue(nextDetail);
    const onDetailChange = vi.fn();
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({
        initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }),
        onDetailChange,
      }),
    );
    act(() => {
      result.current.handleStartEdit();
      result.current.updateDraftMeta((prev) => ({ ...prev, title: "changed" }));
    });
    act(() => {
      result.current.handleSaveEdit();
    });
    await flushAsync();
    expect(onDetailChange).toHaveBeenCalledTimes(1);
    expect(onDetailChange).toHaveBeenCalledWith(nextDetail);
  });
});

describe("message auto-clear", () => {
  it("失败类 message (含 '失败') 不会被 auto-clear", async () => {
    mockUpdateItem.mockRejectedValueOnce(new Error("保存失败"));
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({
        initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }),
      }),
    );
    act(() => {
      result.current.handleStartEdit();
      result.current.updateDraftMeta((prev) => ({ ...prev, title: "c1" }));
    });
    act(() => {
      result.current.handleSaveEdit();
    });
    await flushAsync();
    expect(result.current.message).toBe("保存失败");
    await act(async () => {
      vi.advanceTimersByTime(3000);
    });
    expect(result.current.message).toBe("保存失败");
  });

  it("英文 error 类 message 也不会被 auto-clear", async () => {
    mockUpdateItem.mockRejectedValueOnce(new Error("disk Error"));
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({
        initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }),
      }),
    );
    act(() => {
      result.current.handleStartEdit();
      result.current.updateDraftMeta((prev) => ({ ...prev, title: "c1" }));
    });
    act(() => {
      result.current.handleSaveEdit();
    });
    await flushAsync();
    expect(result.current.message).toBe("disk Error");
    await act(async () => {
      vi.advanceTimersByTime(3000);
    });
    expect(result.current.message).toBe("disk Error");
  });

  it("不含失败/error 关键字的 message 会在 2400ms 后被清空", async () => {
    mockUpdateItem.mockRejectedValueOnce(new Error("just a warning"));
    const { result } = renderHook(() =>
      useMediaLibraryDetailState({
        initialDetail: makeDetail({ meta: makeMeta({ title: "orig" }) }),
      }),
    );
    act(() => {
      result.current.handleStartEdit();
      result.current.updateDraftMeta((prev) => ({ ...prev, title: "c1" }));
    });
    act(() => {
      result.current.handleSaveEdit();
    });
    await flushAsync();
    expect(result.current.message).toBe("just a warning");
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });
    expect(result.current.message).toBe("");
  });
});
