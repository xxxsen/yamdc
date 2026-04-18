import type { MediaLibraryStatus } from "@/lib/api";

// move-refresh-reducer: library-shell "移动到媒体库" + "重新扫描库"
// 两个并行生命周期的 state machine. 对应 td/022 §3.4 的决策: 把
// "多个并存 bool" 收口到单一 status, 类型层消除非法组合.
//
// ── 原设计 (这次重构前) ──
// 每个 shell 自己散养 moveStarting / moveRunning / moveCompletedFlash
// / moveProgressVisible 四个独立 bool, 通过 useEffect 相互同步, 加一
// 对 ref (observedMoveRunningRef / prevMoveRunningRef) 做 "至少见过
// 一次 running" 的隐式门. 只要有一个 bool 漏更新, 或者快路径跳过
// 某个态, 反馈链就断掉.
//
// 实际踩过的坑 (已记录到 §3.4): 后端处理极快时 (3 个 0KB 占位文件),
// 前端首次 poll 已经拿到 completed, 从未观察到 moveRunning=true.
// observedRef 永远是 false, 完成 useEffect 的守卫条件不满足,
//   - listLibraryItems 不调 -> 左侧列表不刷新 (❌ 正确性 bug);
//   - moveCompletedFlash 不闪 -> 无 "移动完成" 反馈 (UX);
//   - moveProgressVisible 被首次 poll 直接 hide (< 1s).
//
// ── state machine 设计 ──
//
//   move:    idle -> starting -> running -> completed-flash -> idle
//                         └── (fast path) ──┘    ← 核心修复点
//
//   refresh: idle -> running -> completed-flash -> idle
//                                  └── (error) ──> idle
//
// "fast path" 对应 MOVE_SERVER_NOT_RUNNING 在 state.move === "starting"
// 时直接跳到 completed-flash, 不再要求中间态. 进 completed-flash 的
// *任何一条路径* 都视为一次完整移动, 上层在 useEffect[state.move] 里
// 统一触发 refresh + 闪 "移动完成" 文案, 3 个 0KB 文件的 bug 从根本
// 消除.
//
// ── 边界决策 ──
//
// - MOVE_SERVER_RUNNING 来自 polling 副作用, 在 state=idle 时视为
//   "外部/其它页面启动了移动", 也合法迁移到 running (页面刷新时撞上
//   已经在跑的移动的场景需要这个).
// - MOVE_SERVER_NOT_RUNNING 在 state=idle / state=completed-flash
//   时是 no-op: 前者表示一直都没任务, 后者表示我们已经在 flash 态,
//   不要重复触发.
// - MOVE_ERROR 只清除 move 分支, 不碰 refresh, 让 refresh 的生命
//   周期独立 (错误上报由上层 setMessage 负责).
// - REFRESH_* 一族只给 **手动 "重新扫描库"** 按钮用, 用来驱动按钮
//   "扫描中..." / "扫描完成" 文案和 spinner 图标. 移动完成后的自动
//   刷新 **不走** 这组 action (走 startTransition + 直接调 refreshLibrary),
//   否则用户没点扫描按钮却看到自己没触发的 "扫描完成", 属 UX 回归.
//   详见 use-library-move-refresh.ts 里 triggerRefreshAfterMove 的注释.

export type MoveStatus = "idle" | "starting" | "running" | "completed-flash";
export type RefreshStatus = "idle" | "running" | "completed-flash";

export interface MoveRefreshState {
  move: MoveStatus;
  refresh: RefreshStatus;
}

export type MoveRefreshAction =
  | { type: "MOVE_CLICK" }
  | { type: "MOVE_SERVER_RUNNING" }
  | { type: "MOVE_SERVER_NOT_RUNNING" }
  | { type: "MOVE_ERROR" }
  | { type: "MOVE_FLASH_DONE" }
  | { type: "REFRESH_START" }
  | { type: "REFRESH_SUCCESS" }
  | { type: "REFRESH_ERROR" }
  | { type: "REFRESH_FLASH_DONE" };

export function getInitialMoveRefreshState(initial: MediaLibraryStatus | null): MoveRefreshState {
  return {
    move: initial?.move.status === "running" ? "running" : "idle",
    refresh: "idle",
  };
}

export function moveRefreshReducer(
  state: MoveRefreshState,
  action: MoveRefreshAction,
): MoveRefreshState {
  switch (action.type) {
    case "MOVE_CLICK":
      if (state.move === "idle" || state.move === "completed-flash") {
        return { ...state, move: "starting" };
      }
      return state;

    case "MOVE_SERVER_RUNNING":
      if (state.move === "idle" || state.move === "starting") {
        return { ...state, move: "running" };
      }
      return state;

    case "MOVE_SERVER_NOT_RUNNING":
      if (state.move === "starting" || state.move === "running") {
        return { ...state, move: "completed-flash" };
      }
      return state;

    case "MOVE_ERROR":
      return { ...state, move: "idle" };

    case "MOVE_FLASH_DONE":
      if (state.move === "completed-flash") {
        return { ...state, move: "idle" };
      }
      return state;

    case "REFRESH_START":
      return { ...state, refresh: "running" };

    case "REFRESH_SUCCESS":
      return { ...state, refresh: "completed-flash" };

    case "REFRESH_ERROR":
      return { ...state, refresh: "idle" };

    case "REFRESH_FLASH_DONE":
      if (state.refresh === "completed-flash") {
        return { ...state, refresh: "idle" };
      }
      return state;
  }
}
