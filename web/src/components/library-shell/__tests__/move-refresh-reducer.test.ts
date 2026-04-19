import { describe, expect, it } from "vitest";

import type { MediaLibraryStatus } from "@/lib/api";
import {
  getInitialMoveRefreshState,
  moveRefreshReducer,
  type MoveRefreshAction,
  type MoveRefreshState,
} from "../move-refresh-reducer";

// reducer 单测覆盖全部合法迁移 + 非法迁移保持 no-op + 初始化分支.
// 配合 td/022 §3.4: "每个迁移后的 state machine 有单测覆盖全部合法迁移".

function makeTask(status: string): MediaLibraryStatus["move"] {
  return {
    task_key: "",
    status,
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
  };
}

function makeStatus(move: string): MediaLibraryStatus {
  return { configured: true, sync: makeTask("idle"), move: makeTask(move) };
}

function reduce(state: MoveRefreshState, ...actions: MoveRefreshAction[]): MoveRefreshState {
  return actions.reduce((acc, action) => moveRefreshReducer(acc, action), state);
}

describe("getInitialMoveRefreshState", () => {
  it("returns idle when initialMediaStatus is null", () => {
    expect(getInitialMoveRefreshState(null)).toEqual({ move: "idle", refresh: "idle" });
  });

  it("returns running when initialMediaStatus.move.status === running (page reload during active move)", () => {
    expect(getInitialMoveRefreshState(makeStatus("running"))).toEqual({ move: "running", refresh: "idle" });
  });

  it("returns idle when initialMediaStatus has non-running move status", () => {
    expect(getInitialMoveRefreshState(makeStatus("idle"))).toEqual({ move: "idle", refresh: "idle" });
    expect(getInitialMoveRefreshState(makeStatus("completed"))).toEqual({ move: "idle", refresh: "idle" });
    expect(getInitialMoveRefreshState(makeStatus("starting"))).toEqual({ move: "idle", refresh: "idle" });
  });
});

describe("moveRefreshReducer — MOVE_CLICK", () => {
  const initial: MoveRefreshState = { move: "idle", refresh: "idle" };

  it("idle → starting (normal click)", () => {
    expect(reduce(initial, { type: "MOVE_CLICK" })).toEqual({ move: "starting", refresh: "idle" });
  });

  it("completed-flash → starting (click during flash window)", () => {
    const state: MoveRefreshState = { move: "completed-flash", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_CLICK" })).toEqual({ move: "starting", refresh: "idle" });
  });

  it("starting → starting (ignore duplicate click)", () => {
    const state: MoveRefreshState = { move: "starting", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_CLICK" })).toBe(state);
  });

  it("running → running (ignore click mid-flight)", () => {
    const state: MoveRefreshState = { move: "running", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_CLICK" })).toBe(state);
  });
});

describe("moveRefreshReducer — MOVE_SERVER_RUNNING", () => {
  it("idle → running (externally initiated move observed via polling)", () => {
    const state: MoveRefreshState = { move: "idle", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_SERVER_RUNNING" })).toEqual({ move: "running", refresh: "idle" });
  });

  it("starting → running (normal path after user click)", () => {
    const state: MoveRefreshState = { move: "starting", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_SERVER_RUNNING" })).toEqual({ move: "running", refresh: "idle" });
  });

  it("running → running (idempotent while server keeps reporting running)", () => {
    const state: MoveRefreshState = { move: "running", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_SERVER_RUNNING" })).toBe(state);
  });

  it("completed-flash → completed-flash (don't hijack the flash)", () => {
    const state: MoveRefreshState = { move: "completed-flash", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_SERVER_RUNNING" })).toBe(state);
  });
});

describe("moveRefreshReducer — MOVE_SERVER_NOT_RUNNING", () => {
  it("starting → completed-flash (FAST PATH: fixes the 0KB move-debug bug)", () => {
    const state: MoveRefreshState = { move: "starting", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_SERVER_NOT_RUNNING" })).toEqual({
      move: "completed-flash",
      refresh: "idle",
    });
  });

  it("running → completed-flash (normal completion path)", () => {
    const state: MoveRefreshState = { move: "running", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_SERVER_NOT_RUNNING" })).toEqual({
      move: "completed-flash",
      refresh: "idle",
    });
  });

  it("idle → idle (polling reports not-running while no active session)", () => {
    const state: MoveRefreshState = { move: "idle", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_SERVER_NOT_RUNNING" })).toBe(state);
  });

  it("completed-flash → completed-flash (don't retrigger flash on repeated not-running polls)", () => {
    const state: MoveRefreshState = { move: "completed-flash", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_SERVER_NOT_RUNNING" })).toBe(state);
  });
});

describe("moveRefreshReducer — MOVE_ERROR", () => {
  it("starting → idle", () => {
    const state: MoveRefreshState = { move: "starting", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_ERROR" })).toEqual({ move: "idle", refresh: "idle" });
  });

  it("running → idle", () => {
    const state: MoveRefreshState = { move: "running", refresh: "running" };
    expect(reduce(state, { type: "MOVE_ERROR" })).toEqual({ move: "idle", refresh: "running" });
  });

  it("idle → idle (no-op, refresh untouched)", () => {
    const state: MoveRefreshState = { move: "idle", refresh: "completed-flash" };
    expect(reduce(state, { type: "MOVE_ERROR" })).toEqual({ move: "idle", refresh: "completed-flash" });
  });
});

describe("moveRefreshReducer — MOVE_FLASH_DONE", () => {
  it("completed-flash → idle after flash timer", () => {
    const state: MoveRefreshState = { move: "completed-flash", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_FLASH_DONE" })).toEqual({ move: "idle", refresh: "idle" });
  });

  it("idle → idle (timer fires late after external reset)", () => {
    const state: MoveRefreshState = { move: "idle", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_FLASH_DONE" })).toBe(state);
  });

  it("running → running (should not kill an active move if timer somehow leaks)", () => {
    const state: MoveRefreshState = { move: "running", refresh: "idle" };
    expect(reduce(state, { type: "MOVE_FLASH_DONE" })).toBe(state);
  });
});

describe("moveRefreshReducer — REFRESH_START", () => {
  it("idle → running", () => {
    expect(reduce({ move: "idle", refresh: "idle" }, { type: "REFRESH_START" })).toEqual({
      move: "idle",
      refresh: "running",
    });
  });

  it("completed-flash → running (重新扫描 while previous flash still showing)", () => {
    expect(reduce({ move: "idle", refresh: "completed-flash" }, { type: "REFRESH_START" })).toEqual({
      move: "idle",
      refresh: "running",
    });
  });

  it("running → running (idempotent re-start)", () => {
    const state: MoveRefreshState = { move: "idle", refresh: "running" };
    expect(reduce(state, { type: "REFRESH_START" })).toEqual({ move: "idle", refresh: "running" });
  });
});

describe("moveRefreshReducer — REFRESH_SUCCESS / REFRESH_ERROR / REFRESH_FLASH_DONE", () => {
  it("REFRESH_SUCCESS from running → completed-flash", () => {
    expect(reduce({ move: "idle", refresh: "running" }, { type: "REFRESH_SUCCESS" })).toEqual({
      move: "idle",
      refresh: "completed-flash",
    });
  });

  it("REFRESH_ERROR from running → idle", () => {
    expect(reduce({ move: "idle", refresh: "running" }, { type: "REFRESH_ERROR" })).toEqual({
      move: "idle",
      refresh: "idle",
    });
  });

  it("REFRESH_FLASH_DONE from completed-flash → idle", () => {
    expect(reduce({ move: "idle", refresh: "completed-flash" }, { type: "REFRESH_FLASH_DONE" })).toEqual({
      move: "idle",
      refresh: "idle",
    });
  });

  it("REFRESH_FLASH_DONE from idle is no-op (stale timer)", () => {
    const state: MoveRefreshState = { move: "idle", refresh: "idle" };
    expect(reduce(state, { type: "REFRESH_FLASH_DONE" })).toBe(state);
  });
});

describe("moveRefreshReducer — full-path integration", () => {
  it("normal path: idle → starting → running → completed-flash → idle", () => {
    const s0 = getInitialMoveRefreshState(null);
    const s1 = moveRefreshReducer(s0, { type: "MOVE_CLICK" });
    expect(s1.move).toBe("starting");
    const s2 = moveRefreshReducer(s1, { type: "MOVE_SERVER_RUNNING" });
    expect(s2.move).toBe("running");
    const s3 = moveRefreshReducer(s2, { type: "MOVE_SERVER_NOT_RUNNING" });
    expect(s3.move).toBe("completed-flash");
    const s4 = moveRefreshReducer(s3, { type: "MOVE_FLASH_DONE" });
    expect(s4.move).toBe("idle");
  });

  it("fast path (0KB bug fix): idle → starting → completed-flash → idle, skipping running", () => {
    const s0 = getInitialMoveRefreshState(null);
    const s1 = moveRefreshReducer(s0, { type: "MOVE_CLICK" });
    const s2 = moveRefreshReducer(s1, { type: "MOVE_SERVER_NOT_RUNNING" });
    expect(s2.move).toBe("completed-flash");
    const s3 = moveRefreshReducer(s2, { type: "MOVE_FLASH_DONE" });
    expect(s3.move).toBe("idle");
  });

  it("page reload during active move: init running → completed-flash → idle", () => {
    const s0 = getInitialMoveRefreshState(makeStatus("running"));
    expect(s0.move).toBe("running");
    const s1 = moveRefreshReducer(s0, { type: "MOVE_SERVER_NOT_RUNNING" });
    expect(s1.move).toBe("completed-flash");
  });

  it("externally initiated move: idle → running via polling, not via click", () => {
    const s0 = getInitialMoveRefreshState(null);
    const s1 = moveRefreshReducer(s0, { type: "MOVE_SERVER_RUNNING" });
    expect(s1.move).toBe("running");
  });

  it("manual refresh concurrent with move-triggered refresh: refresh lifecycle is independent", () => {
    let s: MoveRefreshState = { move: "completed-flash", refresh: "running" };
    s = moveRefreshReducer(s, { type: "MOVE_FLASH_DONE" });
    expect(s).toEqual({ move: "idle", refresh: "running" });
    s = moveRefreshReducer(s, { type: "REFRESH_SUCCESS" });
    expect(s).toEqual({ move: "idle", refresh: "completed-flash" });
    s = moveRefreshReducer(s, { type: "REFRESH_FLASH_DONE" });
    expect(s).toEqual({ move: "idle", refresh: "idle" });
  });
});
