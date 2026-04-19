// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { startTransition as reactStartTransition } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { MediaLibraryStatus } from "@/lib/api";

import { useLibraryMoveRefresh } from "../use-library-move-refresh";

// use-library-move-refresh: 库 shell 里 "移动到媒体库" + "重新扫描库" 的
// 生命周期托管 hook. 这套单测的核心目标:
//
//   1. 冻结 watermark 逻辑 (lastAckedMoveStartedAtRef) — 就是 commit 1d2f24d
//      修掉的"第二次点击移动直接闪完成"的 race 防线. 以后任何重构改到
//      这条路径都必须先把测试改掉, 不让行为悄悄回滚.
//   2. 覆盖 reducer 外部所有副作用: polling interval, flash setTimeout, auto
//      refresh (走 startTransition 不走 REFRESH_* action), 错误路径.
//   3. 锁 "auto-refresh 不驱动 refresh 按钮文案" 这个契约 (commit 7bce4bd).
//
// 策略: vi.useFakeTimers() 控制 setInterval/setTimeout, 用 vi.mock 把
// @/lib/api 里 2 个真实请求 (getMediaLibraryStatus / triggerMoveToMediaLibrary)
// 换成可控 mock, 不碰网络.

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getMediaLibraryStatus: vi.fn(),
    triggerMoveToMediaLibrary: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockGetStatus = vi.mocked(api.getMediaLibraryStatus);
const mockTriggerMove = vi.mocked(api.triggerMoveToMediaLibrary);

function makeStatus(move: Partial<MediaLibraryStatus["move"]>, sync?: Partial<MediaLibraryStatus["sync"]>): MediaLibraryStatus {
  return {
    configured: true,
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
      ...sync,
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
      ...move,
    },
  };
}

interface RenderOpts {
  initialMediaStatus?: MediaLibraryStatus | null;
  refreshLibrary?: () => Promise<void>;
}

function renderMoveRefresh(opts: RenderOpts = {}) {
  const setMessage = vi.fn();
  const refreshLibrary = opts.refreshLibrary ?? vi.fn().mockResolvedValue(undefined);
  const hook = renderHook(() =>
    useLibraryMoveRefresh({
      initialMediaStatus: opts.initialMediaStatus ?? null,
      refreshLibrary,
      setMessage,
      startTransition: reactStartTransition,
    }),
  );
  return { hook, setMessage, refreshLibrary };
}

// flushMicrotasks: 让 hook 里 fire-and-forget 的 `void pollStatusOnce()` /
// `.then(...)` 跑完一轮 microtask. 外层再裹一层 act 保证所有 setState 都
// 被 React 合并 commit, 避免 "state update not wrapped in act" 警告.
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
  mockTriggerMove.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("initial mount", () => {
  it("null initialMediaStatus: move idle, no polling, buttons at default label", () => {
    const { hook } = renderMoveRefresh();
    expect(hook.result.current.moveBusy).toBe(false);
    expect(hook.result.current.moveProgressVisible).toBe(false);
    expect(hook.result.current.refreshBusy).toBe(false);
    expect(hook.result.current.moveButtonLabel).toBe("移动到媒体库");
    expect(hook.result.current.refreshButtonLabel).toBe("重新扫描库");
    expect(hook.result.current.configured).toBe(false);
    expect(mockGetStatus).not.toHaveBeenCalled();
  });

  it("initial status move=running: hook boots directly into running state", () => {
    const initial = makeStatus({ status: "running", started_at: 100, total: 10, processed: 3 });
    const { hook } = renderMoveRefresh({ initialMediaStatus: initial });
    expect(hook.result.current.moveBusy).toBe(true);
    expect(hook.result.current.moveButtonLabel).toBe("移动中 3/10");
    expect(hook.result.current.configured).toBe(true);
  });

  it("initial status completed (started_at=T0): watermark advances silently, button stays idle", async () => {
    // 这条断言住了 page reload 后 useEffect[mediaStatus] 首轮 fire 的
    // 幂等性: 我们会 ack 一次 MOVE_SERVER_NOT_RUNNING, 但 reducer 在 idle
    // 态下是 no-op, 也会把 watermark 推到 T0. 后续同一 snapshot 不再 dispatch.
    const initial = makeStatus({ status: "completed", started_at: 500 });
    const { hook } = renderMoveRefresh({ initialMediaStatus: initial });
    await flushAsync();
    expect(hook.result.current.moveBusy).toBe(false);
    expect(hook.result.current.moveButtonLabel).toBe("移动到媒体库");
  });
});

describe("handleMoveToMediaLibrary - normal full cycle", () => {
  it("click -> starting -> running -> completed-flash -> idle, triggers refreshLibrary silently", async () => {
    mockTriggerMove.mockResolvedValue(undefined);
    // running 先覆盖开头所有 poll / startTransition 内的 getStatus 调用, 避免测
    // 试被隐藏调用次序耦合. 等要进 completed 阶段再通过手动更换 mock 实现.
    mockGetStatus.mockResolvedValue(makeStatus({ status: "running", started_at: 1000, total: 5, processed: 2 }));

    const refreshLibrary = vi.fn().mockResolvedValue(undefined);
    const { hook } = renderMoveRefresh({ refreshLibrary });

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });
    expect(hook.result.current.moveBusy).toBe(true);
    expect(hook.result.current.moveButtonLabel).toBe("移动中...");

    await flushAsync();
    await flushAsync();
    expect(mockTriggerMove).toHaveBeenCalledTimes(1);
    // 拿到 running 快照, 进 running 态
    expect(hook.result.current.moveButtonLabel).toBe("移动中 2/5");

    // 换 mock 成 completed, 推 3s 让 polling 拉下一次.
    mockGetStatus.mockResolvedValue(makeStatus({ status: "completed", started_at: 1000, total: 5, processed: 5 }));
    await act(async () => {
      vi.advanceTimersByTime(3000);
      await Promise.resolve();
      await Promise.resolve();
    });
    await flushAsync();

    // 进 completed-flash 的副作用: refreshLibrary 被调用, move 按钮进入
    // "移动完成" 闪烁窗口. refresh 按钮文案完全没被触动 (commit 7bce4bd 契约).
    expect(refreshLibrary).toHaveBeenCalled();
    expect(hook.result.current.moveButtonLabel).toBe("移动完成");
    expect(hook.result.current.refreshBusy).toBe(false);
    expect(hook.result.current.refreshButtonLabel).toBe("重新扫描库");

    // 1s 后 MOVE_FLASH_DONE -> idle
    await act(async () => {
      vi.advanceTimersByTime(1000);
      await Promise.resolve();
    });
    expect(hook.result.current.moveBusy).toBe(false);
    expect(hook.result.current.moveButtonLabel).toBe("移动到媒体库");
  });
});

describe("handleMoveToMediaLibrary - stale snapshot race (regression test for commit 1d2f24d)", () => {
  it("second click after a completed move: stale 'completed' poll with same started_at is ignored, does NOT fast-path to flash", async () => {
    // 模拟 bug 复现条件: 初始已有一次历史 completed 快照 (started_at=500).
    // 点移动后, polling 立刻回来的是 *相同 started_at* 的 "completed" (因
    // server 还没处理我们的 TriggerMove). 修前会把 starting 直接 fast-path
    // 到 completed-flash; 修后 watermark 把这条 stale 信号吃掉.
    const initial = makeStatus({ status: "completed", started_at: 500 });
    mockTriggerMove.mockResolvedValue(undefined);
    mockGetStatus
      // polling 立即触发的首次请求 - 返回老快照
      .mockResolvedValueOnce(makeStatus({ status: "completed", started_at: 500 }))
      // triggerMove 之后 handleMoveToMediaLibrary 里手工拉的这一次 - 也还是老快照
      .mockResolvedValueOnce(makeStatus({ status: "completed", started_at: 500 }));

    const { hook } = renderMoveRefresh({ initialMediaStatus: initial });
    // 让初始 useEffect[mediaStatus] 把 watermark 推到 500
    await flushAsync();

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });
    expect(hook.result.current.moveBusy).toBe(true);

    // 让 polling 和 handleMoveToMediaLibrary 的 async body 都跑完
    await flushAsync();
    await flushAsync();

    // ✅ 关键断言: 即便 polling 回来是 "completed", 因为 started_at=500 <=
    //   watermark 500, 被 watermark 守门过滤. state.move 停在 starting,
    //   UI 继续显示 "移动中..." 而不是 "移动完成".
    expect(hook.result.current.moveBusy).toBe(true);
    expect(hook.result.current.moveButtonLabel).toBe("移动中...");
  });

  it("new move eventually completes with higher started_at: watermark advances and flash fires", async () => {
    const initial = makeStatus({ status: "completed", started_at: 500 });
    mockTriggerMove.mockResolvedValue(undefined);
    mockGetStatus
      .mockResolvedValueOnce(makeStatus({ status: "completed", started_at: 500 })) // stale 首 poll
      .mockResolvedValueOnce(makeStatus({ status: "running", started_at: 1000 })) // handleMove 里手工拉
      .mockResolvedValueOnce(makeStatus({ status: "completed", started_at: 1000 })); // 下一轮 polling

    const refreshLibrary = vi.fn().mockResolvedValue(undefined);
    const { hook } = renderMoveRefresh({ initialMediaStatus: initial, refreshLibrary });
    await flushAsync();

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });

    // Stale 首 poll 不推进状态
    await flushAsync();
    expect(hook.result.current.moveBusy).toBe(true);

    // handleMove async body 返回带 running 的新 status -> MOVE_SERVER_RUNNING
    await flushAsync();
    // polling 3s 后拉到 completed (started_at=1000 > watermark 500)
    await act(async () => {
      vi.advanceTimersByTime(3000);
      await Promise.resolve();
      await Promise.resolve();
    });
    await flushAsync();

    expect(hook.result.current.moveButtonLabel).toBe("移动完成");
    expect(refreshLibrary).toHaveBeenCalled();
  });
});

describe("handleMoveToMediaLibrary - error paths", () => {
  it("'already running' error: dispatches MOVE_SERVER_RUNNING, reflects 'in progress' message", async () => {
    mockTriggerMove.mockRejectedValue(new Error("move to media library is already running"));
    mockGetStatus.mockResolvedValue(makeStatus({ status: "running", started_at: 1000 }));

    const { hook, setMessage } = renderMoveRefresh();

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });
    await flushAsync();
    await flushAsync();

    expect(setMessage).toHaveBeenCalledWith("媒体库移动任务已在进行中");
    expect(hook.result.current.moveBusy).toBe(true);
  });

  it("other error: dispatches MOVE_ERROR, falls back to idle", async () => {
    mockTriggerMove.mockRejectedValue(new Error("save dir is not configured"));
    mockGetStatus.mockResolvedValue(makeStatus({ status: "idle" }));

    const { hook, setMessage } = renderMoveRefresh();

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });
    await flushAsync();
    await flushAsync();

    expect(setMessage).toHaveBeenCalledWith("未配置保存目录");
    expect(hook.result.current.moveBusy).toBe(false);
  });
});

describe("handleRefreshLibrary - manual rescan button", () => {
  it("click -> running label + spinner -> completed-flash -> idle after 1s", async () => {
    const refreshLibrary = vi.fn().mockResolvedValue(undefined);
    const { hook } = renderMoveRefresh({ refreshLibrary });

    act(() => {
      hook.result.current.handleRefreshLibrary();
    });
    expect(hook.result.current.refreshBusy).toBe(true);
    expect(hook.result.current.refreshButtonLabel).toBe("扫描中...");

    await flushAsync();

    expect(refreshLibrary).toHaveBeenCalledTimes(1);
    expect(hook.result.current.refreshBusy).toBe(false);
    expect(hook.result.current.refreshButtonLabel).toBe("扫描完成");

    await act(async () => {
      vi.advanceTimersByTime(1000);
      await Promise.resolve();
    });

    expect(hook.result.current.refreshButtonLabel).toBe("重新扫描库");
  });

  it("refresh error: dispatches REFRESH_ERROR, state returns to idle and message is set", async () => {
    const refreshLibrary = vi.fn().mockRejectedValue(new Error("boom"));
    const { hook, setMessage } = renderMoveRefresh({ refreshLibrary });

    act(() => {
      hook.result.current.handleRefreshLibrary();
    });
    await flushAsync();

    expect(hook.result.current.refreshBusy).toBe(false);
    expect(hook.result.current.refreshButtonLabel).toBe("重新扫描库");
    expect(setMessage).toHaveBeenCalledWith("boom");
  });
});

describe("auto-refresh after move does NOT touch refresh button UI (regression for commit 7bce4bd)", () => {
  it("move flash window: refreshLibrary runs, but refreshBusy / refreshButtonLabel stay idle throughout", async () => {
    mockTriggerMove.mockResolvedValue(undefined);
    mockGetStatus
      .mockResolvedValueOnce(makeStatus({ status: "running", started_at: 2000 }))
      .mockResolvedValueOnce(makeStatus({ status: "completed", started_at: 2000 }));

    const refreshLibrary = vi.fn().mockResolvedValue(undefined);
    const { hook } = renderMoveRefresh({ refreshLibrary });

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });
    await flushAsync();
    await flushAsync();
    await act(async () => {
      vi.advanceTimersByTime(3000);
      await Promise.resolve();
      await Promise.resolve();
    });
    await flushAsync();

    // auto-refresh 执行了
    expect(refreshLibrary).toHaveBeenCalled();
    // 但 refresh 按钮分支完全没被触动
    expect(hook.result.current.refreshBusy).toBe(false);
    expect(hook.result.current.refreshButtonLabel).toBe("重新扫描库");
  });

  it("auto-refresh rejection is captured via setMessage, refresh button still idle", async () => {
    mockTriggerMove.mockResolvedValue(undefined);
    // 用 mockResolvedValue (非 Once) 保证 polling 首 fire + startTransition
    // 体内的 getStatus 两处调用都能拿到同一个 completed 快照, 否则其中一处
    // 拿到 undefined 会让 handleMoveError 误入错误分支.
    mockGetStatus.mockResolvedValue(makeStatus({ status: "completed", started_at: 3000 }));

    const refreshLibrary = vi.fn().mockRejectedValue(new Error("bad net"));
    const { hook, setMessage } = renderMoveRefresh({ refreshLibrary });

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });
    await flushAsync();
    await flushAsync();

    expect(refreshLibrary).toHaveBeenCalled();
    expect(setMessage).toHaveBeenCalledWith("bad net");
    expect(hook.result.current.refreshBusy).toBe(false);
  });
});

describe("polling lifecycle", () => {
  it("stops polling interval when becomes idle (unmount-safe)", async () => {
    mockTriggerMove.mockResolvedValue(undefined);
    // 点击后 polling / startTransition 先看到 completed + started_at>0, watermark
    // 前进, 进 completed-flash -> 1s 后 idle. 进 idle 后 interval 应该被清理.
    mockGetStatus.mockResolvedValue(makeStatus({ status: "completed", started_at: 9000 }));

    const { hook } = renderMoveRefresh();

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });
    await flushAsync();
    await flushAsync();

    // completed-flash -> 1s 后 MOVE_FLASH_DONE -> idle -> polling useEffect
    // cleanup 清 interval.
    await act(async () => {
      vi.advanceTimersByTime(1000);
      await Promise.resolve();
    });
    await flushAsync();
    expect(hook.result.current.moveBusy).toBe(false);

    const callsAfterIdle = mockGetStatus.mock.calls.length;

    // idle 下再放 6s, 不应该再有 getStatus 调用 (interval 已 clear).
    await act(async () => {
      vi.advanceTimersByTime(6000);
      await Promise.resolve();
    });
    const callsAfterWait = mockGetStatus.mock.calls.length;
    expect(callsAfterWait).toBe(callsAfterIdle);
  });

  it("polling keeps running while sync task is running (shouldPollMediaStatus covers sync too)", async () => {
    mockGetStatus.mockResolvedValue(makeStatus({ status: "idle" }, { status: "running" }));
    const initial = makeStatus({ status: "idle" }, { status: "running" });

    renderMoveRefresh({ initialMediaStatus: initial });

    await flushAsync();
    const firstCall = mockGetStatus.mock.calls.length;
    await act(async () => {
      vi.advanceTimersByTime(3000);
      await Promise.resolve();
    });
    const secondCall = mockGetStatus.mock.calls.length;
    expect(secondCall).toBeGreaterThan(firstCall);
  });

  it("polling errors are swallowed: interval stays alive and hook survives", async () => {
    // 起一个 initial status 是 sync=running, 触发 polling. polling 全炸 ->
    // pollStatusOnce 内 try/catch 吞掉. 我们推进 3s 看 interval 是不是还活着,
    // 以证明 polling error 没把 effect 链打断.
    const initial = makeStatus({ status: "idle" }, { status: "running" });
    mockGetStatus.mockRejectedValue(new Error("netdown"));

    const { hook } = renderMoveRefresh({ initialMediaStatus: initial });

    await flushAsync();
    const firstCallCount = mockGetStatus.mock.calls.length;

    // 再 tick 一次, interval 仍然应在跑 -> 调用次数 > firstCallCount.
    await act(async () => {
      vi.advanceTimersByTime(3000);
      await Promise.resolve();
    });
    const secondCallCount = mockGetStatus.mock.calls.length;
    expect(secondCallCount).toBeGreaterThan(firstCallCount);

    // hook 自身状态读写仍正常.
    expect(hook.result.current.mediaSyncRunning).toBe(true);
    expect(typeof hook.result.current.handleRefreshLibrary).toBe("function");
  });
});

describe("derived view", () => {
  it("exposes taskPercent via moveProgress when running", async () => {
    const initial = makeStatus({ status: "running", started_at: 100, total: 10, processed: 7 });
    const { hook } = renderMoveRefresh({ initialMediaStatus: initial });
    expect(hook.result.current.moveProgress).toBe(70);
    expect(hook.result.current.moveState?.total).toBe(10);
  });

  it("moveProgress is 0 during 'starting' even with populated move state", () => {
    // 这条断言冻住 deriveView 的 starting 特判: 即便 mediaStatus 已有 processed,
    // starting 状态 (刚点击, 还没确认 server 进 running) 进度条永远显示 0.
    const initial = makeStatus({ status: "idle", total: 10, processed: 3 });
    const { hook } = renderMoveRefresh({ initialMediaStatus: initial });

    act(() => {
      hook.result.current.handleMoveToMediaLibrary();
    });
    expect(hook.result.current.moveProgress).toBe(0);
  });

  it("mediaSyncRunning is derived from mediaStatus.sync.status === 'running'", () => {
    const initial = makeStatus({ status: "idle" }, { status: "running" });
    const { hook } = renderMoveRefresh({ initialMediaStatus: initial });
    expect(hook.result.current.mediaSyncRunning).toBe(true);
  });
});
