// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { LibraryListItem } from "@/lib/api";

import { LibraryListPanel } from "../list-panel";

afterEach(() => {
  cleanup();
});

function makeItem(overrides: Partial<LibraryListItem> = {}): LibraryListItem {
  return {
    rel_path: "movies/foo",
    name: "foo",
    title: "Foo",
    number: "ABC-001",
    release_date: "",
    actors: ["A", "B"],
    updated_at: 0,
    has_nfo: true,
    poster_path: "movies/foo/poster.jpg",
    cover_path: "",
    file_count: 1,
    video_count: 1,
    total_size: 0,
    variant_count: 1,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(overrides as any),
  } as LibraryListItem;
}

describe("LibraryListPanel", () => {
  it("正常路径: 渲染列表项 + 标题/演员/路径; 点击 card 调 onSelectItem(rel_path)", () => {
    const onSelectItem = vi.fn();
    render(
      <LibraryListPanel
        items={[makeItem(), makeItem({ rel_path: "movies/bar", title: "Bar", number: "XYZ-002" })]}
        keyword=""
        onKeywordChange={vi.fn()}
        selectedPath="movies/foo"
        onSelectItem={onSelectItem}
        resolveImage={(p) => "/api/lib/" + p}
        bottomActions={<div data-testid="bottom-slot" />}
      />,
    );
    expect(screen.getByText("Foo")).toBeTruthy();
    expect(screen.getByText("Bar")).toBeTruthy();
    // 两个 item 各有一处 "A / B" 演员行 (相同 props), 用 getAllByText.
    expect(screen.getAllByText("A / B").length).toBe(2);
    expect(screen.getByTestId("bottom-slot")).toBeTruthy();
    // 点击 Bar 项触发 onSelectItem.
    fireEvent.click(screen.getByText("Bar"));
    expect(onSelectItem).toHaveBeenCalledWith("movies/bar");
  });

  it("异常路径: 没有 actors → '暂无演员信息'; 关键字搜索 → onKeywordChange", () => {
    const onKeywordChange = vi.fn();
    render(
      <LibraryListPanel
        items={[makeItem({ actors: [] })]}
        keyword=""
        onKeywordChange={onKeywordChange}
        selectedPath=""
        onSelectItem={vi.fn()}
        resolveImage={(p) => p}
        bottomActions={null}
      />,
    );
    expect(screen.getByText("暂无演员信息")).toBeTruthy();
    const searchInput = screen.getByPlaceholderText(/按标题/);
    fireEvent.change(searchInput, { target: { value: "ABC" } });
    expect(onKeywordChange).toHaveBeenCalledWith("ABC");
  });

  it("边缘路径: items 空 → 渲染 '没有匹配的已入库项目'; conflict=true → 渲染冲突 badge; variant_count>1 → '%d 个文件实例'", () => {
    const { rerender } = render(
      <LibraryListPanel
        items={[]}
        keyword=""
        onKeywordChange={vi.fn()}
        selectedPath=""
        onSelectItem={vi.fn()}
        resolveImage={(p) => p}
        bottomActions={null}
      />,
    );
    expect(screen.getByText("没有匹配的已入库项目")).toBeTruthy();

    rerender(
      <LibraryListPanel
        items={[
          makeItem({ rel_path: "movies/conflict", title: "C", conflict: true, variant_count: 3 }),
        ]}
        keyword=""
        onKeywordChange={vi.fn()}
        selectedPath=""
        onSelectItem={vi.fn()}
        resolveImage={(p) => p}
        bottomActions={null}
      />,
    );
    expect(screen.getByText("已存在(冲突)")).toBeTruthy();
    expect(screen.getByText("3 个文件实例")).toBeTruthy();
  });

  it("无图片路径时渲染 fallback 缩略图: number 优先, 其次 title, 最后 name", () => {
    const { rerender, container } = render(
      <LibraryListPanel
        items={[makeItem({ rel_path: "movies/no-img-1", number: "MNO-005", poster_path: "", cover_path: "" })]}
        keyword=""
        onKeywordChange={vi.fn()}
        selectedPath=""
        onSelectItem={vi.fn()}
        resolveImage={(p) => p}
        bottomActions={null}
      />,
    );
    // number 存在时取前两位大写: "MNO-005" -> "MN"
    expect(container.querySelector(".library-thumb-fallback")?.textContent).toBe("MN");
    expect(container.querySelector(".library-thumb-image")).toBeNull();

    // number 为空 → fallback 用 title 的前两位
    rerender(
      <LibraryListPanel
        items={[makeItem({ rel_path: "movies/no-img-2", number: "", title: "Hello", poster_path: "", cover_path: "" })]}
        keyword=""
        onKeywordChange={vi.fn()}
        selectedPath=""
        onSelectItem={vi.fn()}
        resolveImage={(p) => p}
        bottomActions={null}
      />,
    );
    expect(container.querySelector(".library-thumb-fallback")?.textContent).toBe("HE");

    // number / title 都为空 → fallback 用 name 的前两位
    rerender(
      <LibraryListPanel
        items={[makeItem({ rel_path: "movies/no-img-3", number: "", title: "", name: "raw", poster_path: "", cover_path: "" })]}
        keyword=""
        onKeywordChange={vi.fn()}
        selectedPath=""
        onSelectItem={vi.fn()}
        resolveImage={(p) => p}
        bottomActions={null}
      />,
    );
    expect(container.querySelector(".library-thumb-fallback")?.textContent).toBe("RA");
  });

  it("title 缺失时影片标题与 title attribute 都回退到 name", () => {
    const { container } = render(
      <LibraryListPanel
        items={[makeItem({ rel_path: "movies/title-fallback", title: "", name: "fallback-name" })]}
        keyword=""
        onKeywordChange={vi.fn()}
        selectedPath=""
        onSelectItem={vi.fn()}
        resolveImage={(p) => p}
        bottomActions={null}
      />,
    );
    const titleEl = container.querySelector(".library-item-title");
    expect(titleEl?.textContent).toBe("fallback-name");
    expect(titleEl?.getAttribute("title")).toBe("fallback-name");
  });

  it("number 缺失时影片号位置渲染 '未命名影片'", () => {
    render(
      <LibraryListPanel
        items={[makeItem({ rel_path: "movies/no-number", number: "" })]}
        keyword=""
        onKeywordChange={vi.fn()}
        selectedPath=""
        onSelectItem={vi.fn()}
        resolveImage={(p) => p}
        bottomActions={null}
      />,
    );
    expect(screen.getByText("未命名影片")).toBeTruthy();
  });
});
