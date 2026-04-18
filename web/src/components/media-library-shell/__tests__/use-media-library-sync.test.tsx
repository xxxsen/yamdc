// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { MediaLibraryStatus } from "@/lib/api";

import { useMediaLibrarySync } from "../use-media-library-sync";

// useMediaLibrarySync: 媒体库页的 "同步任务" 生命周期 + "按 filter 拉 item
// 列表" 的编排. 结构和 use-library-move-refresh 同构 — polling + flash +
// 首次 running -> completed 观测守门 (observedSyncRunningRef).
//
// 测试重点:
//   1. filter 变化会触发 refreshItems, year=all 时会 merge 新的 year options
//   2. 同步正常全周期: running -> idle + previously observed -> flash + 拉 items
//   3. error: "already running" 推 syncRunning=true, 其它 error 退到 idle
//   4. polling interval 在 syncBusy 切 false 后被 clear
//   5. 首次 mount 的 refreshStatus(), 当 initial status 已经是 running 时
//      观察到完成事件能正确触发 flash

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getMediaLibraryStatus: vi.fn(),
    listMediaLibraryItems: vi.fn(),
    triggerMediaLibrarySync: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockGetStatus = vi.mocked(api.getMediaLibraryStatus);
const mockListItems = vi.mocked(api.listMediaLibraryItems);
const mockTriggerSync = vi.mocked(api.triggerMediaLibrarySync);

function makeStatus(overrides: {
  configured?: boolean;
  sync?: Partial<MediaLibraryStatus["sync"]>;
  move?: Partial<MediaLibraryStatus["move"]>;
} = {}): MediaLibraryStatus {
  return {
    configured: overrides.configured ?? true,
    sync: {
      task_key: "media_library_sync",
      status: "idle",
      total: 0,
      processed: 0,
      success_count: 0,
      conflict_count: 0,
      error_count: 0,
      current: "",
      message: "",
      started_at: 0,
      finished_at: 0,
      updated_at: 0,
      ...overrides.sync,
    },
    move: {
      task_key: "media_library_move",
      status: "idle",
      total: 0,
      processed: 0,
      success_count: 0,
      conflict_count: 0,
      error_count: 0,
      current: "",
      message: "",
      started_at: 0,
      finished_at: 0,
      updated_at: 0,
      ...overrides.move,
    },
  };
}

interface RenderOpts {
  initialStatus?: MediaLibraryStatus | null;
  deferredKeyword?: string;
  yearFilter?: string;
  sizeFilter?: "all" | "small" | "large";
  sortMode?: "recent" | "name";
  sortOrder?: "asc" | "desc";
  setItems?: ReturnType<typeof vi.fn>;
  setYearOptions?: ReturnType<typeof vi.fn>;
}

function renderSync(opts: RenderOpts = {}) {
  const setItems = opts.setItems ?? vi.fn();
  const setYearOptions = opts.setYearOptions ?? vi.fn();
  const hook = renderHook(() =>
    useMediaLibrarySync({
      initialStatus: opts.initialStatus ?? null,
      deferredKeyword: opts.deferredKeyword ?? "",
      yearFilter: opts.yearFilter ?? "all",
      sizeFilter: (opts.sizeFilter ?? "all") as never,
      sortMode: (opts.sortMode ?? "recent") as never,
      sortOrder: (opts.sortOrder ?? "desc") as never,
      setItems,
      setYearOptions,
    }),
  );
  return { hook, setItems, setYearOptions };
}

async function flushAsync() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  mockGetStatus.mockReset();
  mockListItems.mockReset();
  mockTriggerSync.mockReset();
  // 缺省给 listItems / getStatus 安全默认值, 避免未注入 mock 的测试直接 undefined.
  mockListItems.mockResolvedValue([]);
  mockGetStatus.mockResolvedValue(makeStatus());
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("initial mount", () => {
  it("with null initialStatus: configured=false, no items fetch (gated by configured)", async () => {
    const setItems = vi.fn();
    mockGetStatus.mockResolvedValue(makeStatus({ configured: false }));

    const { hook } = renderSync({ initialStatus: null, setItems });

    await flushAsync();
    expect(hook.result.current.configured).toBe(false);
    expect(mockListItems).not.toHaveBeenCalled();
  });

  it("with configured=true: refreshItems fires once with current filter params", async () => {
    const setItems = vi.fn();
    mockGetStatus.mockResolvedValue(makeStatus({ configured: true }));
    mockListItems.mockResolvedValue([
      { id: 1, path: "/movies/movie-2022-1.mp4", meta: null, updated_at: 0 } as never,
    ]);

    renderSync({
      initialStatus: makeStatus({ configured: true }),
      deferredKeyword: "abc",
      yearFilter: "2023",
      setItems,
    });
    await flushAsync();

    expect(mockListItems).toHaveBeenCalledWith(
      expect.objectContaining({ keyword: "abc", year: "2023", size: "all", sort: "recent", order: "desc" }),
    );
    expect(setItems).toHaveBeenCalled();
  });

  it("yearFilter !== 'all': does NOT merge yearOptions (avoids polluting year list with filtered-in year)", async () => {
    const setYearOptions = vi.fn();
    mockListItems.mockResolvedValue([
      { id: 1, path: "/media/movie-2018-1.mp4", meta: { release_date: "2018-01-01" }, updated_at: 0 } as never,
    ]);

    renderSync({
      initialStatus: makeStatus({ configured: true }),
      yearFilter: "2022",
      setYearOptions,
    });
    await flushAsync();
    expect(setYearOptions).not.toHaveBeenCalled();
  });

  it("yearFilter === 'all': merges newly-seen years into yearOptions", async () => {
    const setYearOptions = vi.fn();
    mockListItems.mockResolvedValue([
      { id: 1, path: "/media/movie-2018.mp4", meta: null, updated_at: 0 } as never,
      { id: 2, path: "/media/movie-2020.mp4", meta: null, updated_at: 0 } as never,
    ]);

    renderSync({
      initialStatus: makeStatus({ configured: true }),
      yearFilter: "all",
      setYearOptions,
    });
    await flushAsync();
    expect(setYearOptions).toHaveBeenCalled();
  });
});

describe("handleTriggerSync - success", () => {
  it("click: starts syncing, sets syncStarting then promotes to syncRunning", async () => {
    mockTriggerSync.mockResolvedValue(undefined);

    const { hook } = renderSync({ initialStatus: makeStatus({ configured: true }) });
    await flushAsync();

    act(() => {
      hook.result.current.handleTriggerSync();
    });
    expect(hook.result.current.syncStarting).toBe(true);
    expect(hook.result.current.syncButtonLabel).toBe("同步中...");
    expect(hook.result.current.syncMessage).toBe("媒体库同步已启动");

    await flushAsync();
    expect(hook.result.current.syncRunning).toBe(true);
    expect(hook.result.current.syncStarting).toBe(false);
    expect(hook.result.current.syncBusy).toBe(true);
  });

  it("running -> completed via polling: shows 同步完成 flash and refetches items", async () => {
    mockTriggerSync.mockResolvedValue(undefined);
    // 全流程 getStatus 都返回 idle — 关键是 handleTriggerSync 的成功 IIFE
    // 把 observedRef / prevRef 硬写 true, 下一次 polling 看到 idle 就满足
    // "observedRef && prevRef && !nextSyncRunning" 的 flash 触发条件.
    mockGetStatus.mockResolvedValue(makeStatus({ configured: true, sync: { status: "idle" } }));

    const setItems = vi.fn();
    const { hook } = renderSync({ initialStatus: makeStatus({ configured: true }), setItems });
    await flushAsync();

    act(() => {
      hook.result.current.handleTriggerSync();
    });
    await flushAsync();
    expect(hook.result.current.syncRunning).toBe(true);

    setItems.mockClear();
    await act(async () => {
      vi.advanceTimersByTime(3000);
      await Promise.resolve();
      await Promise.resolve();
    });
    await flushAsync();

    expect(hook.result.current.syncCompletedFlash).toBe(true);
    expect(hook.result.current.syncButtonLabel).toBe("同步完成");
    // completed flash 会主动再拉一次列表
    expect(setItems).toHaveBeenCalled();

    // 1s 后 flash 消失
    await act(async () => {
      vi.advanceTimersByTime(1000);
      await Promise.resolve();
    });
    expect(hook.result.current.syncCompletedFlash).toBe(false);
    expect(hook.result.current.syncButtonLabel).toBe("同步媒体库");
  });
});

describe("handleTriggerSync - error paths", () => {
  it("'already running' error: maps to syncRunning=true and message '媒体库正在同步中'", async () => {
    mockTriggerSync.mockRejectedValue(new Error("media library sync is already running"));

    const { hook } = renderSync({ initialStatus: makeStatus({ configured: true }) });
    await flushAsync();

    act(() => {
      hook.result.current.handleTriggerSync();
    });
    await flushAsync();

    expect(hook.result.current.syncRunning).toBe(true);
    expect(hook.result.current.syncStarting).toBe(false);
    expect(hook.result.current.syncMessage).toBe("媒体库正在同步中");
  });

  it("generic error: falls back to idle, syncMessage set, no running flag lingering", async () => {
    mockTriggerSync.mockRejectedValue(new Error("library dir is not configured"));

    const { hook } = renderSync({ initialStatus: makeStatus({ configured: true }) });
    await flushAsync();

    act(() => {
      hook.result.current.handleTriggerSync();
    });
    await flushAsync();

    expect(hook.result.current.syncRunning).toBe(false);
    expect(hook.result.current.syncStarting).toBe(false);
    expect(hook.result.current.syncMessage).toBe("未配置媒体库目录");
  });
});

describe("syncMessage auto-clear", () => {
  it("syncMessage auto-clears after 2.4s", async () => {
    mockTriggerSync.mockRejectedValue(new Error("library dir is not configured"));

    const { hook } = renderSync({ initialStatus: makeStatus({ configured: true }) });
    await flushAsync();

    act(() => {
      hook.result.current.handleTriggerSync();
    });
    await flushAsync();
    expect(hook.result.current.syncMessage).toBe("未配置媒体库目录");

    await act(async () => {
      vi.advanceTimersByTime(2400);
      await Promise.resolve();
    });
    expect(hook.result.current.syncMessage).toBe("");
  });
});

describe("polling lifecycle", () => {
  it("while syncBusy: setInterval 3s calls refreshStatus", async () => {
    mockTriggerSync.mockResolvedValue(undefined);
    mockGetStatus.mockResolvedValue(makeStatus({ configured: true, sync: { status: "running" } }));

    const { hook } = renderSync({ initialStatus: makeStatus({ configured: true }) });
    await flushAsync();

    act(() => {
      hook.result.current.handleTriggerSync();
    });
    await flushAsync();

    const before = mockGetStatus.mock.calls.length;
    await act(async () => {
      vi.advanceTimersByTime(3000);
      await Promise.resolve();
    });
    const after = mockGetStatus.mock.calls.length;
    expect(after).toBeGreaterThan(before);
  });

  it("polling stops after syncBusy returns to false (interval cleanup)", async () => {
    mockTriggerSync.mockResolvedValue(undefined);
    // 全程 idle: mount call 不触发 flash (observedRef=false); click 后手工
    // 把 observedRef/prevRef 推 true; 下一次 polling 看到 idle -> flash ->
    // syncRunning=false -> syncBusy 回 false -> interval cleanup.
    mockGetStatus.mockResolvedValue(makeStatus({ configured: true, sync: { status: "idle" } }));

    const { hook } = renderSync({ initialStatus: makeStatus({ configured: true }) });
    await flushAsync();

    act(() => {
      hook.result.current.handleTriggerSync();
    });
    await flushAsync();

    await act(async () => {
      vi.advanceTimersByTime(3000);
      await Promise.resolve();
      await Promise.resolve();
    });
    await flushAsync();
    expect(hook.result.current.syncBusy).toBe(false);

    const freeze = mockGetStatus.mock.calls.length;
    await act(async () => {
      vi.advanceTimersByTime(6000);
      await Promise.resolve();
    });
    expect(mockGetStatus.mock.calls.length).toBe(freeze);
  });

  it("polling / status errors are swallowed, hook keeps running", async () => {
    mockGetStatus.mockRejectedValue(new Error("net down"));

    const { hook } = renderSync({ initialStatus: makeStatus({ configured: true, sync: { status: "running" } }) });

    await flushAsync();
    // 虽然 fetch 炸了, hook 拿得到 initial syncRunning=true 的派生值
    expect(hook.result.current.syncRunning).toBe(true);
  });
});

describe("filter reactivity", () => {
  it("changing deferredKeyword triggers refreshItems with new params", async () => {
    const setItems = vi.fn();
    mockListItems.mockResolvedValue([]);

    const hook = renderHook(({ keyword }) =>
      useMediaLibrarySync({
        initialStatus: makeStatus({ configured: true }),
        deferredKeyword: keyword,
        yearFilter: "all",
        sizeFilter: "all" as never,
        sortMode: "recent" as never,
        sortOrder: "desc" as never,
        setItems,
        setYearOptions: vi.fn(),
      }),
    {
      initialProps: { keyword: "foo" },
    });
    await flushAsync();
    const firstCalls = mockListItems.mock.calls.length;

    hook.rerender({ keyword: "bar" });
    await flushAsync();
    const afterCalls = mockListItems.mock.calls.length;

    expect(afterCalls).toBeGreaterThan(firstCalls);
    expect(mockListItems).toHaveBeenLastCalledWith(expect.objectContaining({ keyword: "bar" }));
  });

  it("not configured: filter changes do NOT trigger refreshItems", async () => {
    const setItems = vi.fn();
    mockGetStatus.mockResolvedValue(makeStatus({ configured: false }));

    const hook = renderHook(({ keyword }) =>
      useMediaLibrarySync({
        initialStatus: makeStatus({ configured: false }),
        deferredKeyword: keyword,
        yearFilter: "all",
        sizeFilter: "all" as never,
        sortMode: "recent" as never,
        sortOrder: "desc" as never,
        setItems,
        setYearOptions: vi.fn(),
      }),
    {
      initialProps: { keyword: "foo" },
    });
    await flushAsync();
    const firstCalls = mockListItems.mock.calls.length;

    hook.rerender({ keyword: "bar" });
    await flushAsync();

    // configured=false 时 useEffect 直接 return, listItems 不会被 filter 变化触发.
    expect(mockListItems.mock.calls.length).toBe(firstCalls);
  });
});

describe("observedSyncRunningRef guard (initial-running edge case)", () => {
  it("boots with sync already running: 后续 polling 看到 idle 时也能触发 flash", async () => {
    mockGetStatus
      .mockResolvedValueOnce(makeStatus({ configured: true, sync: { status: "idle" } }));

    const setItems = vi.fn();
    const { hook } = renderSync({
      initialStatus: makeStatus({ configured: true, sync: { status: "running" } }),
      setItems,
    });
    // initial: observedRef=true, prevRef=true; mount 的 refreshStatus 拉到 idle
    // 应直接观察到 "running->idle" 的边沿.
    await flushAsync();
    await flushAsync();

    expect(hook.result.current.syncCompletedFlash).toBe(true);
    expect(setItems).toHaveBeenCalled();
  });
});
