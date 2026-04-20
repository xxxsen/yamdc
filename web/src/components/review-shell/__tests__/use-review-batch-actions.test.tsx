// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { JobItem, ReviewMeta } from "@/lib/api";

import { useReviewBatchActions, type UseReviewBatchActionsDeps } from "../use-review-batch-actions";

// useReviewBatchActions 包了 review 页面的 "单条/批量 入库" + "单条/批量
// 删除" action. 覆盖:
// - handleImport: selected/meta 任一空时 no-op, moveRunning 时只报 message
// - handleImport: persistReview 失败时不发 import 请求, 成功后调用 import + removeJobFromList
// - handleImportSelected: 无选中 no-op, moveRunning 报 message,
//   当 selected 在批量里时会走一次 persistReview; 部分失败 / 全失败消息不同
// - handleDelete / handleDeleteSelected: 只是写 setDeleteTargetIds
// - confirmDelete: deleteTargetIds 空 / null 时 no-op, 否则依次 deleteJob
//   成功+失败混合时消息不同

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    importReviewJob: vi.fn(),
    deleteJob: vi.fn(),
    rejectReviewJob: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockImport = vi.mocked(api.importReviewJob);
const mockDelete = vi.mocked(api.deleteJob);
const mockReject = vi.mocked(api.rejectReviewJob);

function makeJob(overrides: Partial<JobItem> = {}): JobItem {
  return {
    id: 1,
    rel_path: "a/b.mp4",
    number: "ABC-001",
    status: "reviewing",
    created_at: 0,
    updated_at: 0,
    error_message: "",
    conflict_reason: "",
    raw_number: "",
    cleaned_number: "",
    number_source: "manual",
    number_clean_status: "success",
    number_clean_confidence: "high",
    number_clean_warnings: "",
    ...overrides,
  };
}

function makeMeta(overrides: Partial<ReviewMeta> = {}): ReviewMeta {
  return { number: "ABC-001", title: "t", ...overrides };
}

async function flushAsync(ticks = 6) {
  await act(async () => {
    for (let i = 0; i < ticks; i += 1) {
      await Promise.resolve();
    }
  });
}

function renderBatch(deps: Partial<UseReviewBatchActionsDeps> = {}) {
  const setMessage = vi.fn();
  const setDeleteTargetIds = vi.fn();
  const startTransition = (cb: () => void) => cb();
  const persistReview = vi.fn(async () => true);
  const removeJobFromList = vi.fn();
  const removeJobsFromList = vi.fn();

  const fullDeps: UseReviewBatchActionsDeps = {
    selected: deps.selected ?? null,
    meta: deps.meta ?? null,
    moveRunning: deps.moveRunning ?? false,
    selectedJobIds: deps.selectedJobIds ?? new Set(),
    deleteTargetIds: deps.deleteTargetIds ?? null,
    setDeleteTargetIds: deps.setDeleteTargetIds ?? setDeleteTargetIds,
    setMessage: deps.setMessage ?? setMessage,
    startTransition: deps.startTransition ?? startTransition,
    persistReview: deps.persistReview ?? persistReview,
    removeJobFromList: deps.removeJobFromList ?? removeJobFromList,
    removeJobsFromList: deps.removeJobsFromList ?? removeJobsFromList,
  };

  const hook = renderHook(() => useReviewBatchActions(fullDeps));
  return {
    hook,
    setMessage: fullDeps.setMessage as ReturnType<typeof vi.fn>,
    setDeleteTargetIds: fullDeps.setDeleteTargetIds as ReturnType<typeof vi.fn>,
    persistReview: fullDeps.persistReview as ReturnType<typeof vi.fn>,
    removeJobFromList: fullDeps.removeJobFromList as ReturnType<typeof vi.fn>,
    removeJobsFromList: fullDeps.removeJobsFromList as ReturnType<typeof vi.fn>,
  };
}

beforeEach(() => {
  mockImport.mockReset();
  mockDelete.mockReset();
  mockReject.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("handleImport", () => {
  it("selected/meta 缺任一: no-op", async () => {
    const { hook, setMessage, persistReview } = renderBatch({ selected: null, meta: makeMeta() });
    act(() => {
      hook.result.current.handleImport();
    });
    await flushAsync();
    expect(persistReview).not.toHaveBeenCalled();
    expect(setMessage).not.toHaveBeenCalled();
  });

  it("moveRunning 时只 setMessage, 不 persist/import", async () => {
    const { hook, setMessage, persistReview } = renderBatch({
      selected: makeJob(),
      meta: makeMeta(),
      moveRunning: true,
    });
    act(() => {
      hook.result.current.handleImport();
    });
    await flushAsync();
    expect(setMessage).toHaveBeenCalledWith("媒体库移动进行中，暂不可审批入库");
    expect(persistReview).not.toHaveBeenCalled();
    expect(mockImport).not.toHaveBeenCalled();
  });

  it("persistReview 失败时不发 import", async () => {
    const persistReview = vi.fn(async () => false);
    const { hook, setMessage } = renderBatch({
      selected: makeJob(),
      meta: makeMeta(),
      persistReview,
    });
    act(() => {
      hook.result.current.handleImport();
    });
    await flushAsync();
    expect(persistReview).toHaveBeenCalled();
    expect(mockImport).not.toHaveBeenCalled();
    // "执行入库..." 不应被调用, 因为 persist 失败就 return 了.
    expect(setMessage.mock.calls.flatMap((c) => c as string[])).not.toContain("执行入库...");
  });

  it("import 成功: 调用 removeJobFromList + 最终 message", async () => {
    const selected = makeJob({ id: 42 });
    mockImport.mockResolvedValue({});
    const { hook, setMessage, removeJobFromList } = renderBatch({
      selected,
      meta: makeMeta(),
    });
    act(() => {
      hook.result.current.handleImport();
    });
    await flushAsync();
    expect(mockImport).toHaveBeenCalledWith(42);
    expect(removeJobFromList).toHaveBeenCalledWith(42);
    expect(setMessage).toHaveBeenCalledWith("入库完成，任务已移出 review 列表");
  });

  it("import 抛错时 setMessage 用 error.message", async () => {
    mockImport.mockRejectedValue(new Error("import boom"));
    const { hook, setMessage, removeJobFromList } = renderBatch({
      selected: makeJob(),
      meta: makeMeta(),
    });
    act(() => {
      hook.result.current.handleImport();
    });
    await flushAsync();
    expect(setMessage).toHaveBeenCalledWith("import boom");
    expect(removeJobFromList).not.toHaveBeenCalled();
  });
});

describe("handleImportSelected", () => {
  it("selectedJobIds 空: no-op", async () => {
    const { hook, persistReview } = renderBatch({ selectedJobIds: new Set() });
    act(() => {
      hook.result.current.handleImportSelected();
    });
    await flushAsync();
    expect(persistReview).not.toHaveBeenCalled();
    expect(mockImport).not.toHaveBeenCalled();
  });

  it("moveRunning 时只 setMessage, 不触发 persist/import", async () => {
    const { hook, setMessage, persistReview } = renderBatch({
      selectedJobIds: new Set([1, 2]),
      moveRunning: true,
    });
    act(() => {
      hook.result.current.handleImportSelected();
    });
    await flushAsync();
    expect(setMessage).toHaveBeenCalledWith("媒体库移动进行中，暂不可批量审批入库");
    expect(persistReview).not.toHaveBeenCalled();
  });

  it("selected 在选中里: 先走一次 persistReview, 成功后批量 import", async () => {
    mockImport.mockResolvedValue({});
    const selected = makeJob({ id: 1 });
    const persistReview = vi.fn(async () => true);
    const { hook, removeJobsFromList, setMessage } = renderBatch({
      selected,
      meta: makeMeta(),
      selectedJobIds: new Set([1, 2]),
      persistReview,
    });
    act(() => {
      hook.result.current.handleImportSelected();
    });
    await flushAsync();
    expect(persistReview).toHaveBeenCalledTimes(1);
    expect(mockImport).toHaveBeenCalledTimes(2);
    expect(removeJobsFromList).toHaveBeenCalledWith([1, 2]);
    expect(setMessage).toHaveBeenCalledWith("批量审批完成，已入库 2 项");
  });

  it("selected 在选中里但 persistReview 失败: 直接 return, 不走 import", async () => {
    const persistReview = vi.fn(async () => false);
    const { hook, removeJobsFromList } = renderBatch({
      selected: makeJob({ id: 1 }),
      meta: makeMeta(),
      selectedJobIds: new Set([1, 2]),
      persistReview,
    });
    act(() => {
      hook.result.current.handleImportSelected();
    });
    await flushAsync();
    expect(persistReview).toHaveBeenCalled();
    expect(mockImport).not.toHaveBeenCalled();
    expect(removeJobsFromList).not.toHaveBeenCalled();
  });

  it("全部失败: setMessage 取第一条错误", async () => {
    mockImport.mockRejectedValue(new Error("E1"));
    const { hook, setMessage, removeJobsFromList } = renderBatch({
      selected: null,
      meta: null,
      selectedJobIds: new Set([1, 2]),
    });
    act(() => {
      hook.result.current.handleImportSelected();
    });
    await flushAsync();
    expect(setMessage).toHaveBeenLastCalledWith("E1");
    expect(removeJobsFromList).not.toHaveBeenCalled();
  });

  it("部分失败: setMessage 汇总成功数 + 失败数", async () => {
    mockImport.mockImplementationOnce(async () => ({}));
    mockImport.mockImplementationOnce(async () => { throw new Error("boom"); });
    const { hook, setMessage, removeJobsFromList } = renderBatch({
      selected: null,
      meta: null,
      selectedJobIds: new Set([1, 2]),
    });
    act(() => {
      hook.result.current.handleImportSelected();
    });
    await flushAsync();
    expect(removeJobsFromList).toHaveBeenCalledWith([1]);
    expect(setMessage).toHaveBeenLastCalledWith("已入库 1 项，1 项失败");
  });
});

describe("handleDelete / handleDeleteSelected", () => {
  it("handleDelete: selected 为空 no-op", () => {
    const { hook, setDeleteTargetIds } = renderBatch({ selected: null });
    act(() => {
      hook.result.current.handleDelete();
    });
    expect(setDeleteTargetIds).not.toHaveBeenCalled();
  });

  it("handleDelete: 写入 [selected.id]", () => {
    const { hook, setDeleteTargetIds } = renderBatch({ selected: makeJob({ id: 99 }) });
    act(() => {
      hook.result.current.handleDelete();
    });
    expect(setDeleteTargetIds).toHaveBeenCalledWith([99]);
  });

  it("handleDeleteSelected: 空集合 no-op", () => {
    const { hook, setDeleteTargetIds } = renderBatch({ selectedJobIds: new Set() });
    act(() => {
      hook.result.current.handleDeleteSelected();
    });
    expect(setDeleteTargetIds).not.toHaveBeenCalled();
  });

  it("handleDeleteSelected: 写入 Array.from(selectedJobIds)", () => {
    const { hook, setDeleteTargetIds } = renderBatch({ selectedJobIds: new Set([1, 2, 3]) });
    act(() => {
      hook.result.current.handleDeleteSelected();
    });
    expect(setDeleteTargetIds).toHaveBeenCalledTimes(1);
    expect(Array.from(setDeleteTargetIds.mock.calls[0][0] as number[]).sort()).toEqual([1, 2, 3]);
  });

  // moveRunning 守卫: 这两个 handler 是删除链路的入口, 如果只靠 UI 的
  // disabled 去挡, "对话框已经开着, moveRunning 中途变 true" 场景仍然漏掉。
  // 所以 hook 层自己也要短路 + 给出一致 message, 保持和 handleImport* 同款
  // 策略。
  it("handleDelete: moveRunning 时只 setMessage, 不写 targetIds", () => {
    const { hook, setMessage, setDeleteTargetIds } = renderBatch({
      selected: makeJob({ id: 99 }),
      moveRunning: true,
    });
    act(() => {
      hook.result.current.handleDelete();
    });
    expect(setMessage).toHaveBeenCalledWith("媒体库移动进行中，暂不可删除任务");
    expect(setDeleteTargetIds).not.toHaveBeenCalled();
  });

  it("handleDeleteSelected: moveRunning 时只 setMessage, 不写 targetIds", () => {
    const { hook, setMessage, setDeleteTargetIds } = renderBatch({
      selectedJobIds: new Set([1, 2]),
      moveRunning: true,
    });
    act(() => {
      hook.result.current.handleDeleteSelected();
    });
    expect(setMessage).toHaveBeenCalledWith("媒体库移动进行中，暂不可批量删除");
    expect(setDeleteTargetIds).not.toHaveBeenCalled();
  });
});

describe("handleReject", () => {
  it("selected 为 null 时 no-op, 不调 rejectReviewJob", async () => {
    const { hook, setMessage, removeJobFromList } = renderBatch({ selected: null });
    act(() => {
      hook.result.current.handleReject();
    });
    await flushAsync();
    expect(mockReject).not.toHaveBeenCalled();
    expect(setMessage).not.toHaveBeenCalled();
    expect(removeJobFromList).not.toHaveBeenCalled();
  });

  it("打回成功: 调 rejectReviewJob + removeJobFromList + 成功 message", async () => {
    mockReject.mockResolvedValue({});
    const selected = makeJob({ id: 77 });
    const { hook, setMessage, removeJobFromList } = renderBatch({ selected });
    act(() => {
      hook.result.current.handleReject();
    });
    await flushAsync();
    expect(mockReject).toHaveBeenCalledWith(77);
    expect(removeJobFromList).toHaveBeenCalledWith(77);
    expect(setMessage).toHaveBeenCalledWith("打回任务...");
    expect(setMessage).toHaveBeenLastCalledWith("任务已打回，可到文件列表修改影片 ID 后重新 run");
  });

  it("reject 抛错: setMessage 用 error.message, 不 removeJob", async () => {
    mockReject.mockRejectedValue(new Error("reject boom"));
    const selected = makeJob({ id: 77 });
    const { hook, setMessage, removeJobFromList } = renderBatch({ selected });
    act(() => {
      hook.result.current.handleReject();
    });
    await flushAsync();
    expect(setMessage).toHaveBeenLastCalledWith("reject boom");
    expect(removeJobFromList).not.toHaveBeenCalled();
  });
});

describe("confirmDelete", () => {
  it("deleteTargetIds 为 null 时 no-op", async () => {
    const { hook } = renderBatch({ deleteTargetIds: null });
    act(() => {
      hook.result.current.confirmDelete();
    });
    await flushAsync();
    expect(mockDelete).not.toHaveBeenCalled();
  });

  it("deleteTargetIds 空数组 no-op", async () => {
    const { hook, setDeleteTargetIds } = renderBatch({ deleteTargetIds: [] });
    act(() => {
      hook.result.current.confirmDelete();
    });
    await flushAsync();
    expect(mockDelete).not.toHaveBeenCalled();
    expect(setDeleteTargetIds).not.toHaveBeenCalled();
  });

  it("单条成功: setMessage '任务已删除' + removeJobsFromList", async () => {
    mockDelete.mockResolvedValue({});
    const { hook, setMessage, removeJobsFromList, setDeleteTargetIds } = renderBatch({
      deleteTargetIds: [5],
    });
    act(() => {
      hook.result.current.confirmDelete();
    });
    await flushAsync();
    expect(setDeleteTargetIds).toHaveBeenCalledWith(null);
    expect(mockDelete).toHaveBeenCalledWith(5);
    expect(removeJobsFromList).toHaveBeenCalledWith([5]);
    expect(setMessage).toHaveBeenLastCalledWith("任务已删除");
  });

  it("批量全部成功: setMessage '已删除 N 项'", async () => {
    mockDelete.mockResolvedValue({});
    const { hook, setMessage, removeJobsFromList } = renderBatch({
      deleteTargetIds: [1, 2, 3],
    });
    act(() => {
      hook.result.current.confirmDelete();
    });
    await flushAsync();
    expect(removeJobsFromList).toHaveBeenCalledWith([1, 2, 3]);
    expect(setMessage).toHaveBeenLastCalledWith("已删除 3 项");
  });

  it("全部失败: setMessage 使用第一条错误", async () => {
    mockDelete.mockRejectedValue(new Error("no-permission"));
    const { hook, setMessage, removeJobsFromList } = renderBatch({
      deleteTargetIds: [1, 2],
    });
    act(() => {
      hook.result.current.confirmDelete();
    });
    await flushAsync();
    expect(removeJobsFromList).not.toHaveBeenCalled();
    expect(setMessage).toHaveBeenLastCalledWith("no-permission");
  });

  it("部分失败: setMessage 汇总", async () => {
    mockDelete.mockImplementationOnce(async () => ({}));
    mockDelete.mockImplementationOnce(async () => { throw new Error("boom"); });
    const { hook, setMessage, removeJobsFromList } = renderBatch({
      deleteTargetIds: [1, 2],
    });
    act(() => {
      hook.result.current.confirmDelete();
    });
    await flushAsync();
    expect(removeJobsFromList).toHaveBeenCalledWith([1]);
    expect(setMessage).toHaveBeenLastCalledWith("已删除 1 项，1 项失败");
  });

  // moveRunning 在"对话框已经打开、随后迁移启动"时的兜底: 点确认会走到
  // confirmDelete, 这里必须把 targetIds 清空 (关对话框) 并发消息, 拒绝
  // 进入 deleteJob/os.Remove 的实际删除路径。
  it("moveRunning 时: 清空 targetIds + setMessage, 不调 deleteJob", async () => {
    const { hook, setMessage, setDeleteTargetIds, removeJobsFromList } = renderBatch({
      deleteTargetIds: [1, 2],
      moveRunning: true,
    });
    act(() => {
      hook.result.current.confirmDelete();
    });
    await flushAsync();
    expect(setDeleteTargetIds).toHaveBeenCalledWith(null);
    expect(setMessage).toHaveBeenCalledWith("媒体库移动进行中，删除已取消");
    expect(mockDelete).not.toHaveBeenCalled();
    expect(removeJobsFromList).not.toHaveBeenCalled();
  });
});
