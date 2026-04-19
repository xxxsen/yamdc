"use client";

import { type Dispatch, type SetStateAction, type TransitionStartFunction, useEffect, useEffectEvent, useReducer, useRef, useState } from "react";

import {
  getInitialMoveRefreshState,
  moveRefreshReducer,
  type MoveRefreshAction,
  type MoveRefreshState,
} from "@/components/library-shell/move-refresh-reducer";
import {
  getMoveButtonLabel,
  getRefreshButtonLabel,
  taskPercent,
  toErrorMessage,
  toMoveToMediaLibraryMessage,
} from "@/components/library-shell/utils";
import type { MediaLibraryStatus, TaskState } from "@/lib/api";
import { getMediaLibraryStatus, triggerMoveToMediaLibrary } from "@/lib/api";

// use-library-move-refresh: library-shell "移动到媒体库" + "重新扫描库"
// 生命周期托管. 替换掉原来散布在主组件里的一堆 bool + ref + effect.
//
// ── 责任 ──
// 1. 拥有 mediaStatus (来自后端 polling)
// 2. 拥有 moveRefreshReducer 状态机, 根据 user 动作 / server 事件 / 定时器
//    派发 action
// 3. 暴露 UI 所需的派生态 (moveBusy / moveProgressVisible / *ButtonLabel ...)
// 4. 在 state.move 进入 completed-flash 时触发 refreshLibrary (上层注入), 修
//    掉原代码 "没观察到 running 就不刷新" 的 0KB fast-path bug
//
// ── 跟原实现的差异 ──
// - 旧: moveStarting / moveRunning / moveCompletedFlash / moveProgressVisible
//       / refreshRunning / refreshCompletedFlash / observedMoveRunningRef /
//       prevMoveRunningRef 八个 state/ref 相互 useEffect 同步, 少一个就挂
// - 新: 单一 MoveRefreshState (move + refresh 两条 status), 类型层消除非法
//       组合, 所有迁移有单测
//
// 详见 td/022-frontend-optimization-roadmap.md §3.4.

export interface UseLibraryMoveRefreshDeps {
  initialMediaStatus: MediaLibraryStatus | null;
  refreshLibrary: () => Promise<void>;
  setMessage: Dispatch<SetStateAction<string>>;
  startTransition: TransitionStartFunction;
}

export interface UseLibraryMoveRefreshResult {
  mediaStatus: MediaLibraryStatus | null;
  moveState: TaskState | null;
  mediaSyncRunning: boolean;
  configured: boolean;
  moveBusy: boolean;
  moveProgressVisible: boolean;
  moveProgress: number;
  refreshBusy: boolean;
  refreshButtonLabel: string;
  moveButtonLabel: string;
  handleRefreshLibrary: () => void;
  handleMoveToMediaLibrary: () => void;
}

function deriveView(state: MoveRefreshState, mediaStatus: MediaLibraryStatus | null) {
  const moveState = mediaStatus?.move ?? null;
  const moveRunning = state.move === "running";
  const moveBusy = state.move === "starting" || state.move === "running";
  const moveCompletedFlash = state.move === "completed-flash";
  const moveProgressVisible = moveBusy;
  const moveProgress = state.move === "starting" ? 0 : moveState ? taskPercent(moveState) : 0;
  const refreshBusy = state.refresh === "running";
  const refreshCompletedFlash = state.refresh === "completed-flash";
  return {
    moveState,
    moveRunning,
    moveBusy,
    moveProgressVisible,
    moveProgress,
    moveCompletedFlash,
    refreshBusy,
    refreshCompletedFlash,
  };
}

export function useLibraryMoveRefresh(deps: UseLibraryMoveRefreshDeps): UseLibraryMoveRefreshResult {
  const { initialMediaStatus, refreshLibrary, setMessage, startTransition } = deps;

  const [mediaStatus, setMediaStatus] = useState<MediaLibraryStatus | null>(initialMediaStatus);
  const [state, dispatch] = useReducer(moveRefreshReducer, initialMediaStatus, getInitialMoveRefreshState);

  // lastAckedMoveStartedAtRef: 上一次已经 "ack 过" 的 move.started_at. 用来判
  // 定 polling 返回的 "completed/idle/failed" 是**新任务的完成**还是**上一次
  // 任务残留的快照**.
  //
  // ── 为什么需要这个 watermark ──
  // 用户点 "移动到媒体库" 后, dispatch MOVE_CLICK 会让 state.move=starting,
  // moveBusy=true, 触发 shouldPollMediaStatus 的 useEffect 同步 fire. 那个
  // effect 里 `void pollStatusOnce()` 是 fire-and-forget 的 fetch, 几乎总是
  // 比 `startTransition(await triggerMoveToMediaLibrary(...))` 里的 await 更
  // 早回来 (一个是立即发请求, 一个要走完整 round-trip 并等 server 处理).
  //
  // 此时 server 还没收到 TriggerMove, mediaStatus.move 仍是 **上一次 move
  // 残留的 { status: "completed", started_at: T_prev }**. setMediaStatus 会
  // 让 useEffect[mediaStatus] 立刻 fire, 翻译成 MOVE_SERVER_NOT_RUNNING, reducer
  // 走 fast path: starting -> completed-flash. 结果: 用户第二次 (及以后)
  // 点击, 直接看到 "移动完成" 闪过, 进度条一帧没出现.
  //
  // 修法: 每次 ack MOVE_SERVER_NOT_RUNNING 时把 started_at 记下. 后续 poll
  // 返回 started_at <= ref 的快照 (同一任务 / 老任务), 视为 stale 忽略; 只
  // 有 started_at 严格大于 ref 才是新任务完成信号. 由于 server 在每次新 move
  // StartedAt 都会写入新的 UnixMillis (见 internal/medialib), watermark 是
  // 单调递增的.
  //
  // ── 幂等与边界 ──
  // - 初值 0: 服务端 started_at 第一次出现就会被 ack (> 0), 不影响首次行为.
  // - 重复同值快照: ref.current = T, poll 返回 T, 不 dispatch, reducer 不前
  //   移 — 避免了页面加载后的 completed 快照反复 dispatch 的噪音 (reducer
  //   本身在 idle 态是 no-op, 但这里提前剪掉也省一份 effect 调度).
  // - 外部/其它窗口发起的 move: 页面加载时 initialMediaStatus.move.status
  //   可能是 running, started_at=T_ext. ref 初值是 0, 所以该 move 的完成信
  //   号 (started_at=T_ext > 0) 仍会被 ack, 不会误判.
  // - 服务端返回 "已在进行中" 错误: handleMoveError 里 dispatch MOVE_SERVER_
  //   RUNNING 推进状态机, 当前正在跑的任务完成时 started_at 仍会 > 原有
  //   ref.current (因为那个跑中任务 started 时就已经 > 之前的 ack 值了).
  const lastAckedMoveStartedAtRef = useRef<number>(0);

  const view = deriveView(state, mediaStatus);
  const mediaSyncRunning = mediaStatus?.sync.status === "running";
  const configured = !!mediaStatus?.configured;
  const shouldPollMediaStatus = view.moveBusy || mediaSyncRunning;

  const refreshButtonLabel = getRefreshButtonLabel(view.refreshBusy, view.refreshCompletedFlash);
  const moveButtonLabel = getMoveButtonLabel(view.moveBusy, view.moveRunning, view.moveState, view.moveCompletedFlash);

  // 翻译服务器 move.status -> reducer action. 每次 polling setMediaStatus 产生
  // 新对象, 这个 effect 会重跑一次; reducer 本身幂等 (重复同态 action 是 no-op),
  // 不会无限循环. MOVE_SERVER_NOT_RUNNING 由 watermark 把门, 只认新任务的
  // 完成信号 (细节见 lastAckedMoveStartedAtRef 上方注释).
  useEffect(() => {
    if (!mediaStatus) return;
    const status = mediaStatus.move.status;
    const startedAt = mediaStatus.move.started_at;
    if (status === "running") {
      dispatch({ type: "MOVE_SERVER_RUNNING" });
      return;
    }
    if (status === "idle" || status === "completed" || status === "failed") {
      if (startedAt > lastAckedMoveStartedAtRef.current) {
        lastAckedMoveStartedAtRef.current = startedAt;
        dispatch({ type: "MOVE_SERVER_NOT_RUNNING" });
      }
      // started_at <= ref.current: stale snapshot (新任务还没登记), 忽略.
    }
  }, [mediaStatus]);

  const pollStatusOnce = useEffectEvent(async (signal?: AbortSignal) => {
    try {
      const next = await getMediaLibraryStatus(signal);
      setMediaStatus(next);
    } catch {
      // ignore polling errors - next tick 会重试
    }
  });

  useEffect(() => {
    if (!shouldPollMediaStatus) return;
    const controller = new AbortController();
    void pollStatusOnce(controller.signal);
    const timer = window.setInterval(() => {
      void pollStatusOnce(controller.signal);
    }, 3000);
    return () => {
      window.clearInterval(timer);
      controller.abort();
    };
  }, [shouldPollMediaStatus]);

  // 进入 completed-flash 时触发一次 refresh. starting-> completed-flash
  // (fast path) / running -> completed-flash (normal path) 都走这条分支,
  // 彻底消除 0KB 漏刷新 bug.
  //
  // 注意: 这里 **故意不走** REFRESH_START / REFRESH_SUCCESS reducer 路径.
  // 那条路径是给用户点 "重新扫描库" 按钮用的, 会把按钮文案切到 "扫描中..."
  // 再闪 "扫描完成". 移动后的自动 refresh 是副作用式的后处理, 用户根本
  // 没点那个按钮, 反馈点应该是 "移动完成" (在 move 按钮上). 如果这里也
  // 走 reducer, 用户会看到自己没触发的 "扫描完成" 文案 (历史 bug).
  //
  // 并发边界: 用户若在 ~1s flash 窗口内手动点 "重新扫描库", 会并发产生
  // 两次 listLibraryItems 请求, 最后到达的响应赢. 数据不会坏, 接受这个
  // 小代价换 UX 清晰.
  const triggerRefreshAfterMove = useEffectEvent(() => {
    startTransition(async () => {
      try {
        await refreshLibrary();
      } catch (error) {
        setMessage(toErrorMessage(error, "刷新已入库目录失败"));
      }
    });
  });

  useEffect(() => {
    if (state.move !== "completed-flash") return;
    triggerRefreshAfterMove();
    const timer = window.setTimeout(() => dispatch({ type: "MOVE_FLASH_DONE" }), 1000);
    return () => window.clearTimeout(timer);
  }, [state.move]);

  useEffect(() => {
    if (state.refresh !== "completed-flash") return;
    const timer = window.setTimeout(() => dispatch({ type: "REFRESH_FLASH_DONE" }), 1000);
    return () => window.clearTimeout(timer);
  }, [state.refresh]);

  const handleRefreshLibrary = () => {
    dispatch({ type: "REFRESH_START" });
    startTransition(async () => {
      try {
        await refreshLibrary();
        dispatch({ type: "REFRESH_SUCCESS" });
      } catch (error) {
        setMessage(toErrorMessage(error, "刷新已入库目录失败"));
        dispatch({ type: "REFRESH_ERROR" });
      }
    });
  };

  const handleMoveToMediaLibrary = () => {
    dispatch({ type: "MOVE_CLICK" });
    setMessage("媒体库移动已启动");
    startTransition(async () => {
      try {
        await triggerMoveToMediaLibrary();
        const next = await getMediaLibraryStatus();
        setMediaStatus(next);
      } catch (error) {
        handleMoveError(error, setMessage, dispatch, setMediaStatus);
      }
    });
  };

  return {
    mediaStatus,
    moveState: view.moveState,
    mediaSyncRunning,
    configured,
    moveBusy: view.moveBusy,
    moveProgressVisible: view.moveProgressVisible,
    moveProgress: view.moveProgress,
    refreshBusy: view.refreshBusy,
    refreshButtonLabel,
    moveButtonLabel,
    handleRefreshLibrary,
    handleMoveToMediaLibrary,
  };
}

// handleMoveError: trigger 期间异常的专门处理. "already running" 意味着
// 服务端确实在跑 (可能是其它页面发起的), 需要把 state 推进到 running 态;
// 其它错误全部当失败处理回 idle, 同时异步拉一次 status 修正 UI.
function handleMoveError(
  error: unknown,
  setMessage: Dispatch<SetStateAction<string>>,
  dispatch: Dispatch<MoveRefreshAction>,
  setMediaStatus: Dispatch<SetStateAction<MediaLibraryStatus | null>>,
) {
  const msg = toMoveToMediaLibraryMessage(error);
  setMessage(msg);
  if (msg === "媒体库移动任务已在进行中") {
    dispatch({ type: "MOVE_SERVER_RUNNING" });
    void getMediaLibraryStatus().then((next) => setMediaStatus(next)).catch(() => {});
    return;
  }
  dispatch({ type: "MOVE_ERROR" });
  void getMediaLibraryStatus().then((next) => setMediaStatus(next)).catch(() => {});
}
