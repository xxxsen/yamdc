// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { JobListResponse } from "@/lib/api";

// next/dynamic 在 jsdom 下会异步加载组件 — 测试里用一个同步的 stub 替代,
// 避免 timer / Suspense 边界让断言 flaky. 这里只验证 ProcessingShell 自身
// 的 (data, error) 切换 + 重试逻辑, 不重复测 JobTable.
//
// 同时把 dynamic() 的两个 callback (loader / loading fallback) 捕获下来,
// 让单测能直接调用它们 — 否则 mock 会让原文件里那两个箭头函数无人调用,
// function-coverage 留下假阴影. vi.mock 的工厂会被 hoist 到文件顶部,
// 导致直接闭包引用的 const 还没初始化, 所以用 vi.hoisted() 把状态也提前.
const { dynamicCalls } = vi.hoisted(() => ({
  dynamicCalls: [] as Array<{
    loader: () => Promise<unknown>;
    opts: { ssr?: boolean; loading?: () => React.ReactNode };
  }>,
}));
vi.mock("next/dynamic", () => ({
  default: (
    loader: () => Promise<unknown>,
    opts: { ssr?: boolean; loading?: () => React.ReactNode },
  ) => {
    dynamicCalls.push({ loader, opts });
    return function MockJobTable({ initialData }: { initialData: JobListResponse }) {
      return (
        <div data-testid="job-table">
          <span data-testid="job-count">{initialData.items.length}</span>
          <span data-testid="job-total">{initialData.total}</span>
        </div>
      );
    };
  },
}));

const listJobsMock = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    listJobs: (...args: Parameters<typeof actual.listJobs>) => listJobsMock(...args),
  };
});

import { JobTableLoadingFallback, ProcessingShell } from "../processing-shell";

const emptyJobs: JobListResponse = {
  items: [],
  total: 0,
  page: 1,
  page_size: 50,
};

const populatedJobs: JobListResponse = {
  items: [
    { id: 1, rel_path: "a.mp4", number: "ABC-001", status: "init" },
    { id: 2, rel_path: "b.mp4", number: "ABC-002", status: "processing" },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ] as any,
  total: 2,
  page: 1,
  page_size: 50,
};

beforeEach(() => {
  listJobsMock.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("ProcessingShell - 正常路径", () => {
  it("initialError 为空时直接渲染 JobTable, 不显示 ErrorState", () => {
    render(<ProcessingShell initialData={populatedJobs} initialError={null} />);
    expect(screen.getByTestId("job-table")).toBeTruthy();
    expect(screen.getByTestId("job-count").textContent).toBe("2");
    expect(screen.queryByText("加载处理队列失败")).toBeNull();
  });
});

describe("ProcessingShell - 异常路径", () => {
  it("initialError 非空时渲染 ErrorState 含错误文案 + 重试按钮", () => {
    render(<ProcessingShell initialData={emptyJobs} initialError="后端 500" />);
    expect(screen.getByText("加载处理队列失败")).toBeTruthy();
    expect(screen.getByText("后端 500")).toBeTruthy();
    expect(screen.getByRole("button", { name: "重试" })).toBeTruthy();
    expect(screen.queryByTestId("job-table")).toBeNull();
  });
});

describe("JobTableLoadingFallback - 占位 UI", () => {
  // 被 dynamic({loading: () => <JobTableLoadingFallback/>}) 复用. jsdom 下
  // dynamic 多数情况下会跳过 loading callback, 直接 mount, 因此只能由这条
  // 直接 render 的单测来锁住占位 UI 的视觉合约.
  it("渲染处理队列 hero 区 + 占位文案, 不依赖任何业务数据", () => {
    render(<JobTableLoadingFallback />);
    expect(screen.getByText("Processing Queue")).toBeTruthy();
    expect(screen.getByText("文件列表")).toBeTruthy();
    expect(screen.getByText("正在加载任务列表...")).toBeTruthy();
  });
});

describe("processing-shell dynamic() 装配契约", () => {
  // 确保 ProcessingShell 给 next/dynamic 传的 ssr:false + loading 回调
  // 形状不被悄悄改变. 同时直接调用这两个被 dynamic 吞掉的回调以填补
  // function coverage (它们在 jsdom 下永远不会被 dynamic 真正触发).
  it("传入 ssr:false 与一个能渲染 JobTableLoadingFallback 的 loading 回调; loader 是 thenable", async () => {
    render(<ProcessingShell initialData={emptyJobs} initialError={null} />);
    expect(dynamicCalls.length).toBeGreaterThan(0);
    const { loader, opts } = dynamicCalls[dynamicCalls.length - 1];
    expect(opts.ssr).toBe(false);
    // 调用 loading callback, 确认其返回的就是 JobTableLoadingFallback 子树.
    const loadingEl = opts.loading?.();
    render(<>{loadingEl}</>);
    expect(screen.getAllByText("Processing Queue").length).toBeGreaterThan(0);
    // loader 是 dynamic import 的箭头函数, 调用后返回 thenable;
    // 真实 import 会触发 job-table 子树, 这里只断言 thenable 形状即可.
    const promised = loader();
    expect(typeof (promised).then).toBe("function");
  });
});

describe("ProcessingShell - 边缘路径 (重试)", () => {
  it("点击重试 → API 成功 → 切到 JobTable, 数据来自重试结果", async () => {
    listJobsMock.mockResolvedValueOnce(populatedJobs);
    render(<ProcessingShell initialData={emptyJobs} initialError="一开始挂了" />);
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(screen.getByTestId("job-table")).toBeTruthy());
    expect(screen.getByTestId("job-total").textContent).toBe("2");
    expect(listJobsMock).toHaveBeenCalledTimes(1);
  });

  it("点击重试 → API 再次失败 → 仍然显示 ErrorState 且文案更新", async () => {
    listJobsMock.mockRejectedValueOnce(new Error("第二次也挂"));
    render(<ProcessingShell initialData={emptyJobs} initialError="一开始挂了" />);
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(screen.getByText("第二次也挂")).toBeTruthy());
    expect(screen.queryByTestId("job-table")).toBeNull();
  });
});
