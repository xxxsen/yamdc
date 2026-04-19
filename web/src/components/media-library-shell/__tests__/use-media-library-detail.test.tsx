// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { MediaLibraryDetail, MediaLibraryItem } from "@/lib/api";

import { useMediaLibraryDetail } from "../use-media-library-detail";

// useMediaLibraryDetail 管媒体库详情 modal:
// - openDetailModal 的 fetch 生命周期 (loading -> success / error)
// - closeDetailModal 重置四件套
// - applyDetailChange 同步到 activeDetail + items list 的子集字段
// - Escape 按键关闭 modal
// - Escape 监听器在 modal 关闭时被 cleanup

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getMediaLibraryItem: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockGetItem = vi.mocked(api.getMediaLibraryItem);

function makeItem(overrides: Partial<MediaLibraryItem> = {}): MediaLibraryItem {
  return {
    id: 1,
    rel_path: "a/b",
    name: "b",
    title: "title-orig",
    number: "NUM-1",
    release_date: "2024-01-01",
    actors: [],
    created_at: 0,
    updated_at: 0,
    has_nfo: true,
    poster_path: "p.jpg",
    cover_path: "c.jpg",
    total_size: 0,
    file_count: 1,
    video_count: 1,
    variant_count: 1,
    conflict: false,
    ...overrides,
  };
}

function makeDetail(itemOverrides: Partial<MediaLibraryItem> = {}): MediaLibraryDetail {
  const item = makeItem(itemOverrides);
  return {
    item,
    meta: {} as MediaLibraryDetail["meta"],
    variants: [],
    primary_variant_key: "",
    files: [],
  };
}

async function flushAsync(ticks = 4) {
  await act(async () => {
    for (let i = 0; i < ticks; i += 1) {
      await Promise.resolve();
    }
  });
}

beforeEach(() => {
  mockGetItem.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("openDetailModal", () => {
  it("先进入 loading, 成功后把 detail 写进 activeDetail", async () => {
    const setItems = vi.fn();
    mockGetItem.mockResolvedValue(makeDetail());
    const { result } = renderHook(() => useMediaLibraryDetail({ setItems }));

    act(() => {
      result.current.openDetailModal(42);
    });
    expect(result.current.activeDetailID).toBe(42);
    expect(result.current.detailLoading).toBe(true);
    expect(result.current.activeDetail).toBeNull();
    expect(result.current.detailError).toBe("");

    await flushAsync();
    expect(result.current.detailLoading).toBe(false);
    expect(result.current.activeDetail).not.toBeNull();
    expect(mockGetItem).toHaveBeenCalledWith(42);
  });

  it("API 抛错时把 Error.message 挂到 detailError, 不设置 activeDetail", async () => {
    const setItems = vi.fn();
    mockGetItem.mockRejectedValue(new Error("load failed"));
    const { result } = renderHook(() => useMediaLibraryDetail({ setItems }));

    act(() => {
      result.current.openDetailModal(7);
    });
    await flushAsync();
    expect(result.current.activeDetail).toBeNull();
    expect(result.current.detailError).toBe("load failed");
    expect(result.current.detailLoading).toBe(false);
    expect(result.current.activeDetailID).toBe(7);
  });

  it("API 抛非 Error 时使用兜底文案", async () => {
    const setItems = vi.fn();
    mockGetItem.mockRejectedValue("raw");
    const { result } = renderHook(() => useMediaLibraryDetail({ setItems }));

    act(() => {
      result.current.openDetailModal(8);
    });
    await flushAsync();
    expect(result.current.detailError).toBe("加载媒体详情失败");
  });
});

describe("closeDetailModal", () => {
  it("重置全部四件套", async () => {
    const setItems = vi.fn();
    mockGetItem.mockResolvedValue(makeDetail());
    const { result } = renderHook(() => useMediaLibraryDetail({ setItems }));
    act(() => {
      result.current.openDetailModal(1);
    });
    await flushAsync();

    act(() => {
      result.current.closeDetailModal();
    });
    expect(result.current.activeDetail).toBeNull();
    expect(result.current.activeDetailID).toBeNull();
    expect(result.current.detailLoading).toBe(false);
    expect(result.current.detailError).toBe("");
  });
});

describe("applyDetailChange", () => {
  it("更新 activeDetail 并把卡片可见字段写回 items list", async () => {
    const setItems = vi.fn();
    mockGetItem.mockResolvedValue(makeDetail({ id: 10, title: "old" }));
    const { result } = renderHook(() => useMediaLibraryDetail({ setItems }));
    act(() => {
      result.current.openDetailModal(10);
    });
    await flushAsync();

    const next = makeDetail({
      id: 10,
      title: "new-title",
      number: "NEW-2",
      release_date: "2024-02-02",
      actors: ["A"],
      updated_at: 123,
      poster_path: "new-p.jpg",
      cover_path: "new-c.jpg",
    });

    act(() => {
      result.current.applyDetailChange(next);
    });
    expect(result.current.activeDetail?.item.title).toBe("new-title");
    expect(setItems).toHaveBeenCalledTimes(1);
    const updater = setItems.mock.calls[0][0] as (items: MediaLibraryItem[]) => MediaLibraryItem[];
    const out = updater([makeItem({ id: 10, title: "old" }), makeItem({ id: 11, title: "other" })]);
    expect(out[0]).toMatchObject({
      id: 10,
      title: "new-title",
      number: "NEW-2",
      release_date: "2024-02-02",
      actors: ["A"],
      updated_at: 123,
      poster_path: "new-p.jpg",
      cover_path: "new-c.jpg",
    });
    // id=11 保持原状
    expect(out[1].title).toBe("other");
  });
});

describe("Escape 键监听", () => {
  it("modal 打开时按 Escape 关闭", async () => {
    const setItems = vi.fn();
    mockGetItem.mockResolvedValue(makeDetail());
    const { result } = renderHook(() => useMediaLibraryDetail({ setItems }));
    act(() => {
      result.current.openDetailModal(1);
    });
    await flushAsync();
    expect(result.current.activeDetailID).toBe(1);

    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    expect(result.current.activeDetail).toBeNull();
    expect(result.current.activeDetailID).toBeNull();
  });

  it("modal 关闭 (activeDetail/ID 都为空) 时, Escape 不影响任何状态", async () => {
    const setItems = vi.fn();
    const { result } = renderHook(() => useMediaLibraryDetail({ setItems }));
    // modal 未打开, Escape 不应报错也不应改变状态
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    expect(result.current.activeDetail).toBeNull();
    expect(result.current.activeDetailID).toBeNull();
  });

  it("非 Escape 键不关闭 modal", async () => {
    const setItems = vi.fn();
    mockGetItem.mockResolvedValue(makeDetail());
    const { result } = renderHook(() => useMediaLibraryDetail({ setItems }));
    act(() => {
      result.current.openDetailModal(1);
    });
    await flushAsync();
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
    });
    expect(result.current.activeDetailID).toBe(1);
  });

  it("unmount 后不再监听 Escape", async () => {
    const setItems = vi.fn();
    mockGetItem.mockResolvedValue(makeDetail());
    const { result, unmount } = renderHook(() => useMediaLibraryDetail({ setItems }));
    act(() => {
      result.current.openDetailModal(1);
    });
    await flushAsync();
    unmount();
    // unmount 之后 window 上的监听器应当被 cleanup, 不再抛异常.
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
  });
});
