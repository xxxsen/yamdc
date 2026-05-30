import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  loadLibraryInitialData,
  loadMediaLibraryInitialData,
  loadProcessingInitialData,
  loadReviewInitialData,
  toLoaderMessage,
} from "@/lib/server/initial-loaders";

vi.mock("@/lib/api", () => ({
  listJobs: vi.fn(),
  getReviewJob: vi.fn(),
  getMediaLibraryStatus: vi.fn(),
  listLibraryItems: vi.fn(),
  getLibraryItem: vi.fn(),
  listMediaLibraryItems: vi.fn(),
}));

const api = await import("@/lib/api");
const mockListJobs = vi.mocked(api.listJobs);
const mockGetReviewJob = vi.mocked(api.getReviewJob);
const mockGetMediaStatus = vi.mocked(api.getMediaLibraryStatus);
const mockListLibrary = vi.mocked(api.listLibraryItems);
const mockGetLibraryItem = vi.mocked(api.getLibraryItem);
const mockListMediaLibrary = vi.mocked(api.listMediaLibraryItems);

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.resetAllMocks();
});

describe("toLoaderMessage", () => {
  it("正常路径: 透传 Error.message", () => {
    expect(toLoaderMessage(new Error("oops"), "fallback")).toBe("oops");
  });
  it("异常路径: 非 Error 时使用 fallback", () => {
    expect(toLoaderMessage("string error", "fallback")).toBe("fallback");
    expect(toLoaderMessage(null, "fallback")).toBe("fallback");
  });
  it("边缘路径: Error message 为空时使用 fallback", () => {
    expect(toLoaderMessage(new Error(""), "fallback")).toBe("fallback");
  });
});

describe("loadProcessingInitialData", () => {
  it("正常路径: API 成功返回 data", async () => {
    mockListJobs.mockResolvedValue({ items: [{ id: 1 } as never], total: 1, page: 1, page_size: 50 });
    const result = await loadProcessingInitialData();
    expect(result.errorMessage).toBeNull();
    expect(result.data.jobs.items).toHaveLength(1);
  });

  it("异常路径: API 抛错返回 errorMessage 和空列表", async () => {
    mockListJobs.mockRejectedValue(new Error("connection refused"));
    const result = await loadProcessingInitialData();
    expect(result.errorMessage).toBe("connection refused");
    expect(result.data.jobs.items).toEqual([]);
  });

  it("边缘路径: 抛非 Error 类型时使用 fallback", async () => {
    mockListJobs.mockRejectedValue("just a string");
    const result = await loadProcessingInitialData();
    expect(result.errorMessage).toBe("加载处理队列失败");
  });
});

describe("loadReviewInitialData", () => {
  it("正常路径: list + detail + status 全部成功", async () => {
    mockListJobs.mockResolvedValue({ items: [{ id: 1 } as never], total: 1, page: 1, page_size: 200 });
    mockGetReviewJob.mockResolvedValue({ review_data: "{}" } as never);
    mockGetMediaStatus.mockResolvedValue({ configured: true } as never);
    const result = await loadReviewInitialData();
    expect(result.errorMessage).toBeNull();
    expect(result.data.jobs).toHaveLength(1);
    expect(result.data.initialScrapeData).toBeTruthy();
    expect(result.data.initialMediaStatus).toBeTruthy();
  });

  it("异常路径: list 失败时整体失败, scrape 不被调", async () => {
    mockListJobs.mockRejectedValue(new Error("listJobs fail"));
    mockGetMediaStatus.mockResolvedValue(null as never);
    const result = await loadReviewInitialData();
    expect(result.errorMessage).toBe("listJobs fail");
    expect(result.data.jobs).toEqual([]);
    expect(mockGetReviewJob).not.toHaveBeenCalled();
  });

  it("边缘路径: list 成功但首条 detail 失败, errorMessage 携带 detail 错误", async () => {
    mockListJobs.mockResolvedValue({ items: [{ id: 7 } as never], total: 1, page: 1, page_size: 200 });
    mockGetReviewJob.mockRejectedValue(new Error("scrape down"));
    mockGetMediaStatus.mockRejectedValue(new Error("ignored"));
    const result = await loadReviewInitialData();
    expect(result.errorMessage).toBe("scrape down");
    expect(result.data.jobs).toHaveLength(1);
    expect(result.data.initialMediaStatus).toBeNull();
  });

  it("正常路径: 列表为空时 detail/status 均不影响 errorMessage", async () => {
    mockListJobs.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 200 });
    mockGetMediaStatus.mockResolvedValue({ configured: false } as never);
    const result = await loadReviewInitialData();
    expect(result.errorMessage).toBeNull();
    expect(mockGetReviewJob).not.toHaveBeenCalled();
  });
});

describe("loadLibraryInitialData", () => {
  it("正常路径: list + detail + status 都成功", async () => {
    mockListLibrary.mockResolvedValue([{ rel_path: "x" } as never]);
    mockGetLibraryItem.mockResolvedValue({ item: { rel_path: "x" } } as never);
    mockGetMediaStatus.mockResolvedValue({ configured: true } as never);
    const result = await loadLibraryInitialData();
    expect(result.errorMessage).toBeNull();
    expect(result.data.items).toHaveLength(1);
    expect(result.data.initialDetail).toBeTruthy();
    expect(result.data.initialMediaStatus).toBeTruthy();
  });

  it("异常路径: list 失败时整体失败", async () => {
    mockListLibrary.mockRejectedValue(new Error("library down"));
    mockGetMediaStatus.mockResolvedValue(null as never);
    const result = await loadLibraryInitialData();
    expect(result.errorMessage).toBe("library down");
    expect(result.data.items).toEqual([]);
    expect(mockGetLibraryItem).not.toHaveBeenCalled();
  });

  it("边缘路径: 列表成功但首条 detail 失败时, 列表保留, detail 为 null, 仍无 errorMessage", async () => {
    mockListLibrary.mockResolvedValue([{ rel_path: "x" } as never]);
    mockGetLibraryItem.mockRejectedValue(new Error("detail down"));
    mockGetMediaStatus.mockResolvedValue(null as never);
    const result = await loadLibraryInitialData();
    expect(result.data.items).toHaveLength(1);
    expect(result.data.initialDetail).toBeNull();
    // detail 失败按文档语义降级为 null, 不向上抛 errorMessage; 上层会
    // 在用户切换详情时重新拉取并提示.
    expect(result.errorMessage).toBeNull();
  });
});

describe("loadMediaLibraryInitialData", () => {
  it("正常路径: 列表 + 状态都成功", async () => {
    mockListMediaLibrary.mockResolvedValue([{ id: 1 } as never]);
    mockGetMediaStatus.mockResolvedValue({ configured: true } as never);
    const result = await loadMediaLibraryInitialData();
    expect(result.errorMessage).toBeNull();
    expect(result.data.items).toHaveLength(1);
    expect(result.data.initialStatus).toBeTruthy();
  });

  it("异常路径: 列表失败时记录 errorMessage, 状态 fallback null", async () => {
    mockListMediaLibrary.mockRejectedValue(new Error("ml down"));
    mockGetMediaStatus.mockResolvedValue(null as never);
    const result = await loadMediaLibraryInitialData();
    expect(result.errorMessage).toBe("ml down");
    expect(result.data.items).toEqual([]);
  });

  it("边缘路径: 列表成功但状态失败, errorMessage 来自状态", async () => {
    mockListMediaLibrary.mockResolvedValue([{ id: 1 } as never]);
    mockGetMediaStatus.mockRejectedValue(new Error("status down"));
    const result = await loadMediaLibraryInitialData();
    expect(result.errorMessage).toBe("status down");
    expect(result.data.items).toHaveLength(1);
  });
});
