// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LibraryDetail, LibraryListItem, MediaLibraryStatus } from "@/lib/api";

// next/dynamic: LibraryShell 内部用 dynamic 拉 ImageCropper, 测试只验
// 顶层 (data, error) 切换 + 重试逻辑, 这里 stub 成空.
vi.mock("next/dynamic", () => ({
  default: () => function NoopDynamic() {
    return null;
  },
}));

const listLibraryItemsMock = vi.fn();
const getLibraryItemMock = vi.fn();
const getMediaLibraryStatusMock = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    listLibraryItems: (...args: Parameters<typeof actual.listLibraryItems>) => listLibraryItemsMock(...args),
    getLibraryItem: (...args: Parameters<typeof actual.getLibraryItem>) => getLibraryItemMock(...args),
    getMediaLibraryStatus: (...args: Parameters<typeof actual.getMediaLibraryStatus>) => getMediaLibraryStatusMock(...args),
  };
});

import { LibraryShell } from "../library-shell";

const sampleItem: LibraryListItem = {
  rel_path: "movies/foo",
  name: "foo",
  title: "Foo Movie",
  number: "ABC-001",
  release_date: "",
  actors: ["Alice"],
  updated_at: 0,
  has_nfo: true,
  poster_path: "",
  cover_path: "",
  file_count: 1,
  video_count: 1,
  total_size: 0,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as any;

const sampleDetail: LibraryDetail = {
  item: sampleItem,
  meta: {
    title: "Foo Movie",
    title_translated: "",
    plot: "",
    plot_translated: "",
    actors: ["Alice"],
    genres: [],
    studio: "",
    series: "",
    director: "",
    release_date: "",
    runtime: 0,
    rating: 0,
    number: "ABC-001",
    poster_path: "",
    cover_path: "",
  },
  variants: [],
  files: [],
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

const fakeStatus: MediaLibraryStatus = {
  configured: false,
  sync: idleTask,
  move: idleTask,
};

beforeEach(() => {
  listLibraryItemsMock.mockReset();
  getLibraryItemMock.mockReset();
  getMediaLibraryStatusMock.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("LibraryShell - 正常路径", () => {
  it("initialError=null 时不渲染 ErrorState, 渲染主体壳 (列表 + 详情)", () => {
    render(
      <LibraryShell
        items={[]}
        initialDetail={null}
        initialMediaStatus={fakeStatus}
        initialError={null}
      />,
    );
    expect(screen.queryByText("加载已入库列表失败")).toBeNull();
    // 空列表态时 LibraryListPanel 至少会渲出一个搜索框 / 列表容器,
    // 这里只验关键文案"已入库"或"列表"是否进入 DOM.
    expect(document.body.textContent).toMatch(/已入库|empty|library/i);
  });
});

describe("LibraryShell - 异常路径", () => {
  it("initialError 非空时渲染 ErrorState + 文案 + 重试按钮", () => {
    render(
      <LibraryShell
        items={[]}
        initialDetail={null}
        initialMediaStatus={null}
        initialError="服务端拉列表挂了"
      />,
    );
    expect(screen.getByText("加载已入库列表失败")).toBeTruthy();
    expect(screen.getByText("服务端拉列表挂了")).toBeTruthy();
    expect(screen.getByRole("button", { name: "重试" })).toBeTruthy();
  });
});

describe("LibraryShell - 边缘路径 (重试)", () => {
  it("重试成功 (列表非空 → getLibraryItem 也成功) → ErrorState 消失, Main 挂起", async () => {
    listLibraryItemsMock.mockResolvedValueOnce([sampleItem]);
    getLibraryItemMock.mockResolvedValueOnce(sampleDetail);
    render(
      <LibraryShell
        items={[]}
        initialDetail={null}
        initialMediaStatus={null}
        initialError="一开始挂了"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => {
      expect(screen.queryByText("加载已入库列表失败")).toBeNull();
    });
    expect(listLibraryItemsMock).toHaveBeenCalledTimes(1);
    expect(getLibraryItemMock).toHaveBeenCalledWith(sampleItem.rel_path);
  });

  it("重试: 列表成功但 getLibraryItem 抛错 → 仍切到 Main (detail 降级 null)", async () => {
    listLibraryItemsMock.mockResolvedValueOnce([sampleItem]);
    getLibraryItemMock.mockRejectedValueOnce(new Error("detail boom"));
    render(
      <LibraryShell
        items={[]}
        initialDetail={null}
        initialMediaStatus={null}
        initialError="一开始挂了"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => {
      expect(screen.queryByText("加载已入库列表失败")).toBeNull();
    });
    // detail 失败被吞掉, 列表仍然把 sampleItem 挂上 Main; 不应再有 ErrorState.
  });

  it("重试再次失败 (listLibraryItems reject) → ErrorState 仍显示并刷新文案", async () => {
    listLibraryItemsMock.mockRejectedValueOnce(new Error("第二次也挂"));
    render(
      <LibraryShell
        items={[]}
        initialDetail={null}
        initialMediaStatus={null}
        initialError="一开始挂了"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(screen.getByText("第二次也挂")).toBeTruthy());
    expect(screen.getByText("加载已入库列表失败")).toBeTruthy();
  });
});
