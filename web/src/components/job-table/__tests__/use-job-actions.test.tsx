// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { JobItem, JobLogItem } from "@/lib/api";

import { useJobActions, type JobActionsDeps } from "../use-job-actions";

// useJobActions: JobTable 的 13 个 handler + 两条副作用 (8s 轮询 + 250ms
// 防抖搜索). 测试目标:
//   1. handleScan / handleRun / handleRerun / handleOpenLogs / handleDelete /
//      confirmDelete 每条 happy + error 两条路径.
//   2. handleToggleSelectAll 三分支 (empty selectable / toggle on / toggle off).
//   3. handleToggleSelectJob add / remove.
//   4. handleStartEditNumber 两条分支 (manual review -> 空字符串, 否则 job.number).
//   5. handleCommitEditNumber 三条分支 (空 -> cancel / 原值 -> cancel / new -> save).
//   6. handleRunSelectedJobs 的 failed/init 分派 + 全失败 / 部分失败消息.
//   7. 轮询 effect 8s 间隔, filter 变化重置基线.
//   8. 搜索 debounce 250ms; rapid input 取消前一次请求.
//   9. logJob 删除时自动清空 log 面板.

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    listJobs: vi.fn(),
    listJobLogs: vi.fn(),
    runJob: vi.fn(),
    rerunJob: vi.fn(),
    deleteJob: vi.fn(),
    triggerScan: vi.fn(),
    updateJobNumber: vi.fn(),
    updateJobNumberStructured: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockListJobs = vi.mocked(api.listJobs);
const mockListLogs = vi.mocked(api.listJobLogs);
const mockRunJob = vi.mocked(api.runJob);
const mockRerunJob = vi.mocked(api.rerunJob);
const mockDeleteJob = vi.mocked(api.deleteJob);
const mockScan = vi.mocked(api.triggerScan);
const mockUpdateNumber = vi.mocked(api.updateJobNumber);
const mockUpdateNumberStructured = vi.mocked(api.updateJobNumberStructured);

function makeJob(overrides: Partial<JobItem> = {}): JobItem {
  return {
    id: 1,
    path: "/movies/demo.mp4",
    number: "ABC-001",
    raw_number: "ABC-001",
    status: "init",
    error: "",
    progress: 0,
    conflict_reason: "",
    number_source: "auto",
    number_clean_status: "success",
    number_clean_confidence: "high",
    created_at: 0,
    updated_at: 0,
    ...overrides,
  } as never;
}

function makeLog(overrides: Partial<JobLogItem> = {}): JobLogItem {
  return {
    id: 1,
    job_id: 1,
    level: "info",
    message: "hello",
    created_at: 0,
    ...overrides,
  } as never;
}

interface RenderOpts {
  jobs?: JobItem[];
  keyword?: string;
  resolvedStatusFilter?: string;
  selectedJobIds?: Set<number>;
  editingNumber?: string;
  logJob?: JobItem | null;
  selectableJobs?: JobItem[];
}

function renderJobActions(opts: RenderOpts = {}) {
  const setAllJobs = vi.fn();
  const setTotal = vi.fn();
  const setMessage = vi.fn();
  const setIsScanning = vi.fn();
  const setLogJob = vi.fn();
  const setLogs = vi.fn();
  const setLogMessage = vi.fn();
  const setDeleteConfirmJob = vi.fn();
  const setEditingJobId = vi.fn();
  const setEditingNumber = vi.fn();
  const setSelectedJobIds = vi.fn();

  const deps: JobActionsDeps = {
    jobs: opts.jobs ?? [],
    keyword: opts.keyword ?? "",
    resolvedStatusFilter: opts.resolvedStatusFilter ?? "all",
    selectedJobIds: opts.selectedJobIds ?? new Set(),
    editingNumber: opts.editingNumber ?? "",
    logJob: opts.logJob ?? null,
    selectableJobs: opts.selectableJobs ?? [],
    setAllJobs,
    setTotal,
    setMessage,
    setIsScanning,
    setLogJob,
    setLogs,
    setLogMessage,
    setDeleteConfirmJob,
    setEditingJobId,
    setEditingNumber,
    setSelectedJobIds,
  };
  const hook = renderHook(() => useJobActions(deps));
  return {
    hook,
    setAllJobs,
    setTotal,
    setMessage,
    setIsScanning,
    setLogJob,
    setLogs,
    setLogMessage,
    setDeleteConfirmJob,
    setEditingJobId,
    setEditingNumber,
    setSelectedJobIds,
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
  vi.useFakeTimers();
  mockListJobs.mockReset();
  mockListLogs.mockReset();
  mockRunJob.mockReset();
  mockRerunJob.mockReset();
  mockDeleteJob.mockReset();
  mockScan.mockReset();
  mockUpdateNumber.mockReset();
  mockUpdateNumberStructured.mockReset();
  mockListJobs.mockResolvedValue({ items: [], total: 0 } as never);
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("polling effect - 8s interval", () => {
  it("fires listJobs every 8s with current keyword/status filter", async () => {
    const { hook } = renderJobActions({ keyword: "foo", resolvedStatusFilter: "init" });
    await flushAsync();

    const before = mockListJobs.mock.calls.length;
    await act(async () => {
      vi.advanceTimersByTime(8000);
      await Promise.resolve();
    });
    expect(mockListJobs.mock.calls.length).toBeGreaterThan(before);

    await act(async () => {
      vi.advanceTimersByTime(8000);
      await Promise.resolve();
    });
    expect(mockListJobs.mock.calls.length).toBeGreaterThan(before + 1);

    hook.unmount();
  });

  it("filter change resets polling baseline (restarts interval)", async () => {
    const hook = renderHook(
      ({ keyword }: { keyword: string }) => {
        const setters: Pick<JobActionsDeps, "setAllJobs"> & Record<string, unknown> = {
          setAllJobs: vi.fn(),
        };
        return useJobActions({
          jobs: [],
          keyword,
          resolvedStatusFilter: "all",
          selectedJobIds: new Set(),
          editingNumber: "",
          logJob: null,
          selectableJobs: [],
          setAllJobs: setters.setAllJobs as never,
          setTotal: vi.fn(),
          setMessage: vi.fn(),
          setIsScanning: vi.fn(),
          setLogJob: vi.fn(),
          setLogs: vi.fn(),
          setLogMessage: vi.fn(),
          setDeleteConfirmJob: vi.fn(),
          setEditingJobId: vi.fn(),
          setEditingNumber: vi.fn(),
          setSelectedJobIds: vi.fn(),
        });
      },
      { initialProps: { keyword: "a" } },
    );
    await flushAsync();

    hook.rerender({ keyword: "b" });
    await flushAsync();
    // 等 debounce 过期 + interval 都走一波
    await act(async () => {
      vi.advanceTimersByTime(250);
      await Promise.resolve();
    });
    const afterDebounce = mockListJobs.mock.calls.length;

    await act(async () => {
      vi.advanceTimersByTime(8000);
      await Promise.resolve();
    });
    expect(mockListJobs.mock.calls.length).toBeGreaterThan(afterDebounce);
  });
});

describe("search debounce - 250ms", () => {
  it("debounce timer fires listJobs once after 250ms", async () => {
    renderJobActions({ keyword: "searchterm" });
    await flushAsync();

    const before = mockListJobs.mock.calls.length;
    await act(async () => {
      vi.advanceTimersByTime(250);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(mockListJobs.mock.calls.length).toBeGreaterThan(before);
  });

  it("debounce error (non-abort): setMessage with error text", async () => {
    const { setMessage } = renderJobActions({ keyword: "x" });
    mockListJobs.mockRejectedValue(new Error("net err"));
    await act(async () => {
      vi.advanceTimersByTime(250);
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(setMessage).toHaveBeenCalledWith("net err");
  });
});

describe("handleScan", () => {
  it("success: triggers scan, refreshes, resets scanning", async () => {
    mockScan.mockResolvedValue(undefined);
    const { hook, setIsScanning } = renderJobActions();
    await flushAsync();

    act(() => {
      hook.result.current.handleScan();
    });
    await flushAsync();

    expect(mockScan).toHaveBeenCalled();
    expect(setIsScanning).toHaveBeenCalledWith(true);
    expect(setIsScanning).toHaveBeenCalledWith(false);
  });

  it("error: setMessage with err text, scanning state still reset", async () => {
    mockScan.mockRejectedValue(new Error("boom"));
    const { hook, setMessage, setIsScanning } = renderJobActions();
    await flushAsync();

    act(() => {
      hook.result.current.handleScan();
    });
    await flushAsync();

    expect(setMessage).toHaveBeenCalledWith("boom");
    expect(setIsScanning).toHaveBeenLastCalledWith(false);
  });
});

describe("handleRun / handleRerun", () => {
  it("handleRun success: calls runJob + refresh", async () => {
    mockRunJob.mockResolvedValue(undefined);
    const { hook } = renderJobActions();
    await flushAsync();

    act(() => {
      hook.result.current.handleRun(makeJob());
    });
    await flushAsync();
    expect(mockRunJob).toHaveBeenCalledWith(1);
  });

  it("handleRun error: setMessage with err", async () => {
    mockRunJob.mockRejectedValue(new Error("run fail"));
    const { hook, setMessage } = renderJobActions();
    await flushAsync();
    act(() => {
      hook.result.current.handleRun(makeJob());
    });
    await flushAsync();
    expect(setMessage).toHaveBeenCalledWith("run fail");
  });

  it("handleRerun success: calls rerunJob + refresh", async () => {
    mockRerunJob.mockResolvedValue(undefined);
    const { hook } = renderJobActions();
    await flushAsync();
    act(() => {
      hook.result.current.handleRerun(makeJob());
    });
    await flushAsync();
    expect(mockRerunJob).toHaveBeenCalledWith(1);
  });
});

describe("handleOpenLogs", () => {
  it("success with logs: setLogs + clears message", async () => {
    mockListLogs.mockResolvedValue([makeLog()]);
    const { hook, setLogs, setLogMessage } = renderJobActions();
    await flushAsync();
    act(() => {
      hook.result.current.handleOpenLogs(makeJob());
    });
    await flushAsync();

    expect(setLogs).toHaveBeenCalled();
    expect(setLogMessage).toHaveBeenCalledWith("");
  });

  it("success empty: setLogMessage '当前没有日志'", async () => {
    mockListLogs.mockResolvedValue([]);
    const { hook, setLogMessage } = renderJobActions();
    await flushAsync();
    act(() => {
      hook.result.current.handleOpenLogs(makeJob());
    });
    await flushAsync();
    expect(setLogMessage).toHaveBeenCalledWith("当前没有日志");
  });

  it("error: clears logs + setLogMessage with err", async () => {
    mockListLogs.mockRejectedValue(new Error("log err"));
    const { hook, setLogs, setLogMessage } = renderJobActions();
    await flushAsync();
    act(() => {
      hook.result.current.handleOpenLogs(makeJob());
    });
    await flushAsync();
    expect(setLogs).toHaveBeenLastCalledWith([]);
    expect(setLogMessage).toHaveBeenCalledWith("log err");
  });
});

describe("handleDelete + confirmDelete", () => {
  it("handleDelete: setDeleteConfirmJob(job) (does NOT call deleteJob)", () => {
    const { hook, setDeleteConfirmJob } = renderJobActions();
    const job = makeJob();
    hook.result.current.handleDelete(job);
    expect(setDeleteConfirmJob).toHaveBeenCalledWith(job);
    expect(mockDeleteJob).not.toHaveBeenCalled();
  });

  it("confirmDelete success: deleteJob + refresh, closes confirm modal", async () => {
    mockDeleteJob.mockResolvedValue(undefined);
    const { hook, setDeleteConfirmJob } = renderJobActions();
    await flushAsync();

    act(() => {
      hook.result.current.confirmDelete(makeJob());
    });
    await flushAsync();
    expect(setDeleteConfirmJob).toHaveBeenCalledWith(null);
    expect(mockDeleteJob).toHaveBeenCalledWith(1);
  });

  it("confirmDelete + matching logJob: clears log panel", async () => {
    const job = makeJob({ id: 42 });
    mockDeleteJob.mockResolvedValue(undefined);
    const { hook, setLogJob, setLogs, setLogMessage } = renderJobActions({ logJob: job });
    await flushAsync();

    act(() => {
      hook.result.current.confirmDelete(job);
    });
    await flushAsync();

    expect(setLogJob).toHaveBeenCalledWith(null);
    expect(setLogs).toHaveBeenCalledWith([]);
    expect(setLogMessage).toHaveBeenCalledWith("");
  });

  it("confirmDelete + non-matching logJob: keeps log panel", async () => {
    const job = makeJob({ id: 42 });
    const otherJob = makeJob({ id: 7 });
    mockDeleteJob.mockResolvedValue(undefined);
    const { hook, setLogJob } = renderJobActions({ logJob: otherJob });
    await flushAsync();

    act(() => {
      hook.result.current.confirmDelete(job);
    });
    await flushAsync();
    expect(setLogJob).not.toHaveBeenCalled();
  });

  it("confirmDelete error: setMessage with err", async () => {
    mockDeleteJob.mockRejectedValue(new Error("del fail"));
    const { hook, setMessage } = renderJobActions();
    await flushAsync();
    act(() => {
      hook.result.current.confirmDelete(makeJob());
    });
    await flushAsync();
    expect(setMessage).toHaveBeenCalledWith("del fail");
  });
});

describe("handleToggleSelectAll", () => {
  it("empty selectable: keeps prev Set unchanged", () => {
    const { hook, setSelectedJobIds } = renderJobActions({ selectableJobs: [] });
    hook.result.current.handleToggleSelectAll();
    expect(setSelectedJobIds).toHaveBeenCalled();

    const updater = setSelectedJobIds.mock.calls[0][0] as (prev: Set<number>) => Set<number>;
    const prev = new Set([1, 2]);
    expect(updater(prev)).toBe(prev);
  });

  it("all already selected: toggles OFF (empty Set)", () => {
    const selectable = [makeJob({ id: 1 }), makeJob({ id: 2 })];
    const { hook, setSelectedJobIds } = renderJobActions({ selectableJobs: selectable });
    hook.result.current.handleToggleSelectAll();
    const updater = setSelectedJobIds.mock.calls[0][0] as (prev: Set<number>) => Set<number>;
    const next = updater(new Set([1, 2]));
    expect(next.size).toBe(0);
  });

  it("some/none selected: toggles ON (full set)", () => {
    const selectable = [makeJob({ id: 1 }), makeJob({ id: 2 })];
    const { hook, setSelectedJobIds } = renderJobActions({ selectableJobs: selectable });
    hook.result.current.handleToggleSelectAll();
    const updater = setSelectedJobIds.mock.calls[0][0] as (prev: Set<number>) => Set<number>;
    const next = updater(new Set([1]));
    expect(Array.from(next).sort()).toEqual([1, 2]);
  });
});

describe("handleToggleSelectJob", () => {
  it("adds unseen id", () => {
    const { hook, setSelectedJobIds } = renderJobActions();
    hook.result.current.handleToggleSelectJob(5);
    const updater = setSelectedJobIds.mock.calls[0][0] as (prev: Set<number>) => Set<number>;
    const next = updater(new Set());
    expect(next.has(5)).toBe(true);
  });

  it("removes seen id", () => {
    const { hook, setSelectedJobIds } = renderJobActions();
    hook.result.current.handleToggleSelectJob(5);
    const updater = setSelectedJobIds.mock.calls[0][0] as (prev: Set<number>) => Set<number>;
    const next = updater(new Set([5]));
    expect(next.has(5)).toBe(false);
  });
});

describe("handleStartEditNumber / handleCommitEditNumber / handleCancelEditNumber", () => {
  it("start: requiresManualNumberReview false -> sets editingNumber=job.number", () => {
    const job = makeJob({ number: "XYZ-007", number_source: "auto", number_clean_status: "success" });
    const { hook, setEditingJobId, setEditingNumber } = renderJobActions();
    hook.result.current.handleStartEditNumber(job);
    expect(setEditingJobId).toHaveBeenCalledWith(1);
    expect(setEditingNumber).toHaveBeenCalledWith("XYZ-007");
  });

  it("start: requiresManualNumberReview true -> sets editingNumber=''", () => {
    const job = makeJob({ number: "XYZ-007", number_source: "auto", number_clean_status: "no_match" });
    const { hook, setEditingNumber } = renderJobActions();
    hook.result.current.handleStartEditNumber(job);
    expect(setEditingNumber).toHaveBeenCalledWith("");
  });

  it("cancel: resets editing state", () => {
    const { hook, setEditingJobId, setEditingNumber } = renderJobActions();
    hook.result.current.handleCancelEditNumber();
    expect(setEditingJobId).toHaveBeenCalledWith(null);
    expect(setEditingNumber).toHaveBeenCalledWith("");
  });

  it("commit empty number: calls cancel (does NOT call API)", () => {
    const { hook, setEditingJobId } = renderJobActions({ editingNumber: "  " });
    hook.result.current.handleCommitEditNumber(makeJob());
    expect(setEditingJobId).toHaveBeenCalledWith(null);
    expect(mockUpdateNumber).not.toHaveBeenCalled();
  });

  it("commit same value (not manual review): cancel only, no API", () => {
    const job = makeJob({ number: "ABC-001", number_source: "auto", number_clean_status: "success" });
    const { hook } = renderJobActions({ editingNumber: "ABC-001" });
    hook.result.current.handleCommitEditNumber(job);
    expect(mockUpdateNumber).not.toHaveBeenCalled();
  });

  it("commit new value: calls updateJobNumber + refresh + cancel", async () => {
    const job = makeJob({ number: "OLD-001" });
    mockUpdateNumber.mockResolvedValue(undefined);
    const { hook } = renderJobActions({ editingNumber: "NEW-002" });
    await flushAsync();

    act(() => {
      hook.result.current.handleCommitEditNumber(job);
    });
    await flushAsync();

    expect(mockUpdateNumber).toHaveBeenCalledWith(1, "NEW-002");
  });

  it("commit with API error: setMessage", async () => {
    mockUpdateNumber.mockRejectedValue(new Error("bad number"));
    const { hook, setMessage } = renderJobActions({ editingNumber: "NEW-002" });
    await flushAsync();
    act(() => {
      hook.result.current.handleCommitEditNumber(makeJob({ number: "OLD-001" }));
    });
    await flushAsync();
    expect(setMessage).toHaveBeenCalledWith("bad number");
  });
});

describe("handleSubmitStructuredNumber", () => {
  it("success: calls updateJobNumberStructured + cancel + refresh", async () => {
    mockUpdateNumberStructured.mockResolvedValue(undefined as never);
    const { hook, setEditingJobId, setEditingNumber } = renderJobActions();
    await flushAsync();

    act(() => {
      hook.result.current.handleSubmitStructuredNumber(
        makeJob({ id: 7 }),
        "PXVR-406",
        [{ id: "multi-cd", index: 2 }],
      );
    });
    await flushAsync();

    expect(mockUpdateNumberStructured).toHaveBeenCalledWith(
      7,
      "PXVR-406",
      [{ id: "multi-cd", index: 2 }],
    );
    expect(setEditingJobId).toHaveBeenCalledWith(null);
    expect(setEditingNumber).toHaveBeenCalledWith("");
  });

  it("error: setMessage with err text", async () => {
    mockUpdateNumberStructured.mockRejectedValue(new Error("invalid variant"));
    const { hook, setMessage } = renderJobActions();
    await flushAsync();

    act(() => {
      hook.result.current.handleSubmitStructuredNumber(makeJob(), "ABC-001", []);
    });
    await flushAsync();

    expect(setMessage).toHaveBeenCalledWith("invalid variant");
  });
});

describe("handleRunSelectedJobs", () => {
  it("no pending: noop", () => {
    const { hook } = renderJobActions({ jobs: [], selectedJobIds: new Set() });
    hook.result.current.handleRunSelectedJobs();
    expect(mockRunJob).not.toHaveBeenCalled();
    expect(mockRerunJob).not.toHaveBeenCalled();
  });

  it("init status uses runJob, failed status uses rerunJob", async () => {
    const j1 = makeJob({ id: 1, status: "init" });
    const j2 = makeJob({ id: 2, status: "failed" });
    mockRunJob.mockResolvedValue(undefined);
    mockRerunJob.mockResolvedValue(undefined);

    const { hook, setMessage } = renderJobActions({
      jobs: [j1, j2],
      selectedJobIds: new Set([1, 2]),
    });
    await flushAsync();
    act(() => {
      hook.result.current.handleRunSelectedJobs();
    });
    // 为每个 pendingJob await 一把 + refreshJobs, 总共 ~5 个微任务节点.
    await flushAsync(12);

    expect(mockRunJob).toHaveBeenCalledWith(1);
    expect(mockRerunJob).toHaveBeenCalledWith(2);
    expect(setMessage).toHaveBeenCalledWith("已提交 2 条任务");
  });

  it("partial failures: message shows success + failed counts", async () => {
    const j1 = makeJob({ id: 1, status: "init" });
    const j2 = makeJob({ id: 2, status: "init" });
    mockRunJob.mockResolvedValueOnce(undefined).mockRejectedValueOnce(new Error("x"));

    const { hook, setMessage } = renderJobActions({
      jobs: [j1, j2],
      selectedJobIds: new Set([1, 2]),
    });
    await flushAsync();
    act(() => {
      hook.result.current.handleRunSelectedJobs();
    });
    await flushAsync(12);

    expect(setMessage).toHaveBeenCalledWith("已提交 1 条，失败 1 条");
  });
});
