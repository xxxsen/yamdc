"use client";

import { type Dispatch, type SetStateAction, type TransitionStartFunction, useEffect, useEffectEvent, useReducer, useState } from "react";

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

  const view = deriveView(state, mediaStatus);
  const mediaSyncRunning = mediaStatus?.sync.status === "running";
  const configured = !!mediaStatus?.configured;
  const shouldPollMediaStatus = view.moveBusy || mediaSyncRunning;

  const refreshButtonLabel = getRefreshButtonLabel(view.refreshBusy, view.refreshCompletedFlash);
  const moveButtonLabel = getMoveButtonLabel(view.moveBusy, view.moveRunning, view.moveState, view.moveCompletedFlash);

  // 翻译服务器 move.status -> reducer action. 每次 polling setMediaStatus 产生
  // 新对象, 这个 effect 会重跑一次; reducer 本身幂等 (重复同态 action 是 no-op),
  // 不会无限循环.
  useEffect(() => {
    if (!mediaStatus) return;
    const status = mediaStatus.move.status;
    if (status === "running") {
      dispatch({ type: "MOVE_SERVER_RUNNING" });
    } else if (status === "idle" || status === "completed" || status === "failed") {
      dispatch({ type: "MOVE_SERVER_NOT_RUNNING" });
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
