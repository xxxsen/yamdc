// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { MediaLibraryItem, MediaLibraryStatus } from "@/lib/api";

// next/dynamic: stub MediaLibraryDetailShell, 我们只验顶层 (data, error)
// 切换 + 重试逻辑.
vi.mock("next/dynamic", () => ({
  default: () => function NoopDynamic() {
    return null;
  },
}));

const listMediaLibraryItemsMock = vi.fn();
const listMediaLibrarySyncLogsMock = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    listMediaLibraryItems: (...args: Parameters<typeof actual.listMediaLibraryItems>) =>
      listMediaLibraryItemsMock(...args),
    listMediaLibrarySyncLogs: (...args: Parameters<typeof actual.listMediaLibrarySyncLogs>) =>
      listMediaLibrarySyncLogsMock(...args),
  };
});

import { MediaLibraryShell } from "../media-library-shell";

const sampleItem: MediaLibraryItem = {
  id: "abc",
  rel_path: "abc",
  title: "Sample Title",
  number: "ABC-001",
  release_date: "2024-01-01",
  total_size: 1024,
  poster_path: "",
  cover_path: "",
  ingested_at: 0,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as any;

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

const configuredStatus: MediaLibraryStatus = {
  configured: true,
  sync: idleTask,
  move: idleTask,
};

const unconfiguredStatus: MediaLibraryStatus = {
  configured: false,
  sync: idleTask,
  move: idleTask,
};

beforeEach(() => {
  listMediaLibraryItemsMock.mockReset();
  listMediaLibrarySyncLogsMock.mockReset();
  // 安全默认: 任何未显式注入的 mock 调用都返回合法空数组. 避免 Main mount
  // 后 useMediaLibrarySync 的 filter useEffect 二次拉列表撞到 undefined ->
  // extractYearOptions(undefined) 抛 TypeError 的 flaky 路径.
  listMediaLibraryItemsMock.mockResolvedValue([]);
  listMediaLibrarySyncLogsMock.mockResolvedValue([]);
});

afterEach(() => {
  cleanup();
});

describe("MediaLibraryShell - 正常路径", () => {
  it("initialError=null 时不渲染 ErrorState, configured=false 显示 EmptyState", () => {
    render(
      <MediaLibraryShell
        items={[]}
        initialStatus={unconfiguredStatus}
        initialError={null}
      />,
    );
    expect(screen.queryByText("加载媒体库失败")).toBeNull();
    // configured=false → EmptyState 提示 "library_dir" 未配置.
    expect(document.body.textContent).toContain("library_dir");
  });

  it("initialError=null 且 configured=true → 渲染浏览器壳 (filter rail + grid)", () => {
    render(
      <MediaLibraryShell
        items={[sampleItem]}
        initialStatus={configuredStatus}
        initialError={null}
      />,
    );
    expect(screen.queryByText("加载媒体库失败")).toBeNull();
    // configured=true 时不再显示 EmptyState 占位文案.
    expect(document.body.textContent).not.toContain("library_dir");
  });
});

describe("MediaLibraryShell - 异常路径", () => {
  it("initialError 非空 → ErrorState + 错误文案 + 重试按钮", () => {
    render(
      <MediaLibraryShell
        items={[]}
        initialStatus={null}
        initialError="后端 500 了"
      />,
    );
    expect(screen.getByText("加载媒体库失败")).toBeTruthy();
    expect(screen.getByText("后端 500 了")).toBeTruthy();
    expect(screen.getByRole("button", { name: "重试" })).toBeTruthy();
  });
});

describe("MediaLibraryShell - 边缘路径 (重试)", () => {
  it("重试成功: listMediaLibraryItems 被调用, ErrorState 消失", async () => {
    // 用 mockResolvedValue (持久) 而非 mockResolvedValueOnce: Main mount
    // 后 useMediaLibrarySync 还会因 [configured, ...filters] useEffect 再
    // 触发一次 listMediaLibraryItems, 一次性 mock 会让二次调用返回 undefined,
    // 进而被 extractYearOptions 当 iterable 解构出 TypeError, 表现为 ~57%
    // flaky 失败. 安全默认在 beforeEach 已注入, 这里覆盖为 [sampleItem] 仍
    // 仅锁定首次调用形状.
    listMediaLibraryItemsMock.mockResolvedValue([sampleItem]);
    render(
      <MediaLibraryShell
        items={[]}
        initialStatus={configuredStatus}
        initialError="一开始挂了"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => {
      expect(screen.queryByText("加载媒体库失败")).toBeNull();
    });
    // 重试至少触发一次 listMediaLibraryItems; Main mount 后 useMediaLibrarySync
    // 自身也会拉一次, 这里不锁次数, 仅锁第一次的调用形状 (ingested+desc),
    // 防止后续重构悄悄改默认排序.
    expect(listMediaLibraryItemsMock).toHaveBeenCalled();
    const firstCallArgs = listMediaLibraryItemsMock.mock.calls[0][0];
    expect(firstCallArgs).toMatchObject({ sort: "ingested", order: "desc" });
  });

  it("重试再次失败: ErrorState 仍显示, 文案更新为新错误", async () => {
    listMediaLibraryItemsMock.mockRejectedValueOnce(new Error("第二次也挂"));
    render(
      <MediaLibraryShell
        items={[]}
        initialStatus={null}
        initialError="一开始挂了"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(screen.getByText("第二次也挂")).toBeTruthy());
    expect(screen.getByText("加载媒体库失败")).toBeTruthy();
  });
});
