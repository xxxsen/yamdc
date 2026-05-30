// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { MediaLibraryItem } from "@/lib/api";

import { MediaLibraryCardGrid } from "../card-grid";

afterEach(() => {
  cleanup();
});

function makeItem(overrides: Partial<MediaLibraryItem> = {}): MediaLibraryItem {
  return {
    id: 1,
    rel_path: "abc",
    title: "Sample",
    name: "abc",
    number: "ABC-001",
    release_date: "2024-05-10",
    poster_path: "abc/poster.jpg",
    cover_path: "",
    total_size: 0,
    ingested_at: 0,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(overrides as any),
  } as MediaLibraryItem;
}

describe("MediaLibraryCardGrid", () => {
  it("正常路径: 渲染每张卡 (title + 影片 ID + 年份), 点击卡片调 onOpenDetail(id)", () => {
    const onOpenDetail = vi.fn();
    render(
      <MediaLibraryCardGrid
        visibleItems={[makeItem(), makeItem({ id: 2, title: "Another", number: "X-002", release_date: "2020-01-01" })]}
        itemsTotal={2}
        filteredTotal={2}
        browserRef={createRef<HTMLDivElement>()}
        loadMoreRef={createRef<HTMLDivElement>()}
        showLoadMoreSentinel={false}
        onOpenDetail={onOpenDetail}
      />,
    );
    expect(screen.getByText("Sample")).toBeTruthy();
    expect(screen.getByText("Another")).toBeTruthy();
    expect(screen.getByText("ABC-001")).toBeTruthy();
    expect(screen.getByText("2024")).toBeTruthy();
    expect(screen.getByText("2020")).toBeTruthy();
    fireEvent.click(screen.getByText("Another"));
    expect(onOpenDetail).toHaveBeenCalledWith(2);
  });

  it("异常路径: itemsTotal=0 → '当前媒体库里还没有项目'; itemsTotal>0 但 filteredTotal=0 → '没有匹配的媒体库项目'", () => {
    const { rerender } = render(
      <MediaLibraryCardGrid
        visibleItems={[]}
        itemsTotal={0}
        filteredTotal={0}
        browserRef={createRef<HTMLDivElement>()}
        loadMoreRef={createRef<HTMLDivElement>()}
        showLoadMoreSentinel={false}
        onOpenDetail={vi.fn()}
      />,
    );
    expect(screen.getByText("当前媒体库里还没有项目")).toBeTruthy();

    rerender(
      <MediaLibraryCardGrid
        visibleItems={[]}
        itemsTotal={5}
        filteredTotal={0}
        browserRef={createRef<HTMLDivElement>()}
        loadMoreRef={createRef<HTMLDivElement>()}
        showLoadMoreSentinel={false}
        onOpenDetail={vi.fn()}
      />,
    );
    expect(screen.getByText("没有匹配的媒体库项目")).toBeTruthy();
  });

  it("边缘路径: 没有 poster_path / cover_path → 显示首两字大写的 fallback; showLoadMoreSentinel=true 渲染 sentinel", () => {
    const { container } = render(
      <MediaLibraryCardGrid
        visibleItems={[makeItem({ poster_path: "", cover_path: "", number: "abc-001" })]}
        itemsTotal={1}
        filteredTotal={1}
        browserRef={createRef<HTMLDivElement>()}
        loadMoreRef={createRef<HTMLDivElement>()}
        showLoadMoreSentinel={true}
        onOpenDetail={vi.fn()}
      />,
    );
    expect(screen.getByText("AB")).toBeTruthy();
    expect(container.querySelector(".media-library-load-sentinel")).toBeTruthy();
  });
});
