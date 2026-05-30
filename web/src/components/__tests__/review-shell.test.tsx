// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { JobItem, JobListResponse, MediaLibraryStatus, ScrapeDataItem } from "@/lib/api";

// next/dynamic 在 jsdom 下会异步加载组件; ReviewShell 内部用 dynamic 拉
// ImageCropper, 这里 stub 成空, 测试只关心顶层的 (data, error) 切换 +
// 重试逻辑, 不复测 ImageCropper.
vi.mock("next/dynamic", () => ({
  default: () => function NoopDynamic() {
    return null;
  },
}));

const listJobsMock = vi.fn();
const getReviewJobMock = vi.fn();
const getMediaLibraryStatusMock = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    listJobs: (...args: Parameters<typeof actual.listJobs>) => listJobsMock(...args),
    getReviewJob: (...args: Parameters<typeof actual.getReviewJob>) =>
      getReviewJobMock(...args),
    getMediaLibraryStatus: (...args: Parameters<typeof actual.getMediaLibraryStatus>) =>
      getMediaLibraryStatusMock(...args),
  };
});

import { ReviewShell } from "../review-shell";

const reviewJob: JobItem = {
  id: 42,
  rel_path: "a/b.mp4",
  number: "ABC-001",
  status: "reviewing",
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as any;

const reviewListResponse: JobListResponse = {
  items: [reviewJob],
  total: 1,
  page: 1,
  page_size: 200,
};

const idleTask = {
  task_key: "k",
  status: "idle",
  total: 0,
  processed: 0,
  success_count: 0,
  conflict_count: 0,
  error_count: 0,
  current: "",
  message: "",
  started_at: 0,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as any;

const fakeStatus: MediaLibraryStatus = {
  configured: false,
  sync: idleTask,
  move: idleTask,
};

beforeEach(() => {
  listJobsMock.mockReset();
  getReviewJobMock.mockReset();
  getMediaLibraryStatusMock.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("ReviewShell - 正常路径", () => {
  it("initialError=null 时不渲染 ErrorState, 渲染 review 主体壳", () => {
    render(
      <ReviewShell
        jobs={[]}
        initialScrapeData={null}
        initialMediaStatus={null}
        initialError={null}
      />,
    );
    expect(screen.queryByText("加载待 review 列表失败")).toBeNull();
    // 空列表态: review 主体多处文案都会提示 "当前没有待 review 的任务",
    // 这里只验证至少有一处, 不锁渲染层级 (放置位置由 Main 编排, 改版常变).
    expect(screen.getAllByText(/当前没有待 review 的任务/).length).toBeGreaterThan(0);
  });
});

describe("ReviewShell - 异常路径", () => {
  it("initialError 非空时渲染 ErrorState + 错误文案 + 重试按钮", () => {
    render(
      <ReviewShell
        jobs={[]}
        initialScrapeData={null}
        initialMediaStatus={null}
        initialError="加载失败"
      />,
    );
    expect(screen.getByText("加载待 review 列表失败")).toBeTruthy();
    expect(screen.getByText("加载失败")).toBeTruthy();
    expect(screen.getByRole("button", { name: "重试" })).toBeTruthy();
  });
});

describe("ReviewShell - 边缘路径 (重试)", () => {
  it("重试成功: listJobs + getReviewJob + getMediaLibraryStatus 都被调用, ErrorState 消失", async () => {
    listJobsMock.mockResolvedValueOnce(reviewListResponse);
    getReviewJobMock.mockResolvedValueOnce({} as ScrapeDataItem);
    getMediaLibraryStatusMock.mockResolvedValueOnce(fakeStatus);

    render(
      <ReviewShell
        jobs={[]}
        initialScrapeData={null}
        initialMediaStatus={null}
        initialError="一开始挂了"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => {
      expect(screen.queryByText("加载待 review 列表失败")).toBeNull();
    });
    expect(listJobsMock).toHaveBeenCalledTimes(1);
    expect(getReviewJobMock).toHaveBeenCalledWith(42);
    expect(getMediaLibraryStatusMock).toHaveBeenCalled();
  });

  it("重试再次失败: ErrorState 仍显示并刷新文案", async () => {
    listJobsMock.mockRejectedValueOnce(new Error("第二次也挂"));
    render(
      <ReviewShell
        jobs={[]}
        initialScrapeData={null}
        initialMediaStatus={null}
        initialError="一开始挂了"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(screen.getByText("第二次也挂")).toBeTruthy());
    expect(screen.getByText("加载待 review 列表失败")).toBeTruthy();
  });

  it("重试: getReviewJob 失败但 listJobs 成功 → 仍切到主体壳 (scrape 降级 null, 不阻塞列表)", async () => {
    listJobsMock.mockResolvedValueOnce(reviewListResponse);
    getReviewJobMock.mockRejectedValueOnce(new Error("scrape 抛错"));
    getMediaLibraryStatusMock.mockResolvedValueOnce(fakeStatus);

    render(
      <ReviewShell
        jobs={[]}
        initialScrapeData={null}
        initialMediaStatus={null}
        initialError="一开始挂了"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => {
      expect(screen.queryByText("加载待 review 列表失败")).toBeNull();
    });
  });
});
