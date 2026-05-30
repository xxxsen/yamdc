// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  LibraryCoverCard,
  LibraryFanartStrip,
  LibraryPosterCard,
  LibraryPreviewOverlay,
} from "../asset-gallery";

afterEach(() => {
  cleanup();
});

const fakeFile = {
  rel_path: "movies/foo/extrafanart/1.jpg",
  name: "1.jpg",
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
} as any;

describe("LibraryPosterCard", () => {
  it("正常路径: selectedPoster 存在 → 渲染图区, 点 hit 调 onOpenPreview", () => {
    const onOpenPreview = vi.fn();
    render(
      <LibraryPosterCard
        selectedPoster="movies/foo/poster.jpg"
        selectedCover="movies/foo/cover.jpg"
        isPending={false}
        uploadActiveRef={{ current: false }}
        resolveImage={(p) => "/r/" + p}
        onOpenCropper={vi.fn()}
        onOpenUploadPicker={vi.fn()}
        onOpenPreview={onOpenPreview}
      />,
    );
    const hit = document.querySelector(".review-image-hit") as HTMLElement;
    fireEvent.click(hit);
    expect(onOpenPreview).toHaveBeenCalledWith({ title: "海报", path: "movies/foo/poster.jpg", name: "海报" });
  });

  it("异常路径: selectedPoster 为空 → 显示 '暂无海报'; cropper 按钮 disabled (无 cover)", () => {
    render(
      <LibraryPosterCard
        selectedPoster=""
        selectedCover=""
        isPending={false}
        uploadActiveRef={{ current: false }}
        resolveImage={(p) => p}
        onOpenCropper={vi.fn()}
        onOpenUploadPicker={vi.fn()}
        onOpenPreview={vi.fn()}
      />,
    );
    expect(screen.getByText("暂无海报")).toBeTruthy();
    expect(screen.getByRole("button", { name: "从封面截取海报" }).hasAttribute("disabled")).toBe(true);
  });

  it("边缘路径: uploadActiveRef.current=true → 点 hit 不开预览; 上传按钮触发 onOpenUploadPicker", () => {
    const onOpenPreview = vi.fn();
    const onOpenUploadPicker = vi.fn();
    render(
      <LibraryPosterCard
        selectedPoster="poster.jpg"
        selectedCover="cover.jpg"
        isPending={false}
        uploadActiveRef={{ current: true }}
        resolveImage={(p) => p}
        onOpenCropper={vi.fn()}
        onOpenUploadPicker={onOpenUploadPicker}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(document.querySelector(".review-image-hit") as HTMLElement);
    expect(onOpenPreview).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "上传海报" }));
    expect(onOpenUploadPicker).toHaveBeenCalledTimes(1);
  });
});

describe("LibraryCoverCard", () => {
  it("正常路径: 渲染图 + 点 hit 调 onOpenPreview('封面')", () => {
    const onOpenPreview = vi.fn();
    render(
      <LibraryCoverCard
        selectedCover="cover.jpg"
        isPending={false}
        uploadActiveRef={{ current: false }}
        resolveImage={(p) => p}
        onOpenUploadPicker={vi.fn()}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(document.querySelector(".review-image-hit") as HTMLElement);
    expect(onOpenPreview).toHaveBeenCalledWith({ title: "封面", path: "cover.jpg", name: "封面" });
  });

  it("异常路径: selectedCover=空 → '暂无封面'; isPending=true → 上传按钮 disabled", () => {
    const onOpenUploadPicker = vi.fn();
    render(
      <LibraryCoverCard
        selectedCover=""
        isPending={true}
        uploadActiveRef={{ current: false }}
        resolveImage={(p) => p}
        onOpenUploadPicker={onOpenUploadPicker}
        onOpenPreview={vi.fn()}
      />,
    );
    expect(screen.getByText("暂无封面")).toBeTruthy();
    const btn = screen.getByRole("button", { name: "上传封面" });
    expect(btn.hasAttribute("disabled")).toBe(true);
    fireEvent.click(btn);
    expect(onOpenUploadPicker).not.toHaveBeenCalled();
  });

  it("边缘路径: uploadActiveRef.current=true → click hit 不开预览", () => {
    const onOpenPreview = vi.fn();
    render(
      <LibraryCoverCard
        selectedCover="cover.jpg"
        isPending={false}
        uploadActiveRef={{ current: true }}
        resolveImage={(p) => p}
        onOpenUploadPicker={vi.fn()}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(document.querySelector(".review-image-hit") as HTMLElement);
    expect(onOpenPreview).not.toHaveBeenCalled();
  });
});

describe("LibraryFanartStrip", () => {
  it("正常路径: 渲染 fanart files + 末尾上传槽; 点 X 触发 onDeleteFanart(rel_path)", () => {
    const onDeleteFanart = vi.fn();
    render(
      <LibraryFanartStrip
        fanartFiles={[fakeFile]}
        isPending={false}
        uploadActiveRef={{ current: false }}
        resolveImage={(p) => p}
        onOpenUploadPicker={vi.fn()}
        onDeleteFanart={onDeleteFanart}
        onOpenPreview={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "删除 extrafanart" }));
    expect(onDeleteFanart).toHaveBeenCalledWith(fakeFile.rel_path);
  });

  it("异常路径: fanartFiles 空 → 只渲染上传槽; 点击触发 onOpenUploadPicker", () => {
    const onOpenUploadPicker = vi.fn();
    render(
      <LibraryFanartStrip
        fanartFiles={[]}
        isPending={false}
        uploadActiveRef={{ current: false }}
        resolveImage={(p) => p}
        onOpenUploadPicker={onOpenUploadPicker}
        onDeleteFanart={vi.fn()}
        onOpenPreview={vi.fn()}
      />,
    );
    expect(screen.getAllByRole("button").length).toBe(1);
    fireEvent.click(screen.getByRole("button"));
    expect(onOpenUploadPicker).toHaveBeenCalledTimes(1);
  });

  it("边缘路径: 点 thumb hit → onOpenPreview('Extrafanart'); uploadActiveRef.current=true 不开", () => {
    const onOpenPreview = vi.fn();
    const { rerender } = render(
      <LibraryFanartStrip
        fanartFiles={[fakeFile]}
        isPending={false}
        uploadActiveRef={{ current: false }}
        resolveImage={(p) => p}
        onOpenUploadPicker={vi.fn()}
        onDeleteFanart={vi.fn()}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(document.querySelector(".review-image-hit") as HTMLElement);
    expect(onOpenPreview).toHaveBeenCalledTimes(1);

    rerender(
      <LibraryFanartStrip
        fanartFiles={[fakeFile]}
        isPending={false}
        uploadActiveRef={{ current: true }}
        resolveImage={(p) => p}
        onOpenUploadPicker={vi.fn()}
        onDeleteFanart={vi.fn()}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(document.querySelector(".review-image-hit") as HTMLElement);
    expect(onOpenPreview).toHaveBeenCalledTimes(1);
  });
});

describe("LibraryPreviewOverlay", () => {
  it("正常路径: preview=null 不渲染", () => {
    const { container } = render(
      <LibraryPreviewOverlay preview={null} resolveImage={(p) => p} onClose={vi.fn()} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("异常路径: 点 backdrop / 关闭按钮触发 onClose; 点对话框内不触发", () => {
    const onClose = vi.fn();
    const { container } = render(
      <LibraryPreviewOverlay
        preview={{ title: "海报", path: "p.jpg", name: "海报" }}
        resolveImage={(p) => p}
        onClose={onClose}
      />,
    );
    fireEvent.click(container.querySelector(".review-preview-dialog") as HTMLElement);
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(container.querySelector(".review-preview-overlay") as HTMLElement);
    expect(onClose).toHaveBeenCalled();
  });

  it("边缘路径: 渲染 preview.title 和 alt 来自 preview.name", () => {
    render(
      <LibraryPreviewOverlay
        preview={{ title: "Extrafanart", path: "f.jpg", name: "fanart-1" }}
        resolveImage={(p) => p}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText("Extrafanart")).toBeTruthy();
    expect((screen.getByAltText("fanart-1")).src).toContain("f.jpg");
  });
});

describe("LibraryFanartStrip onWheel", () => {
  // 把垂直滚轮转换成水平滚动 (用户用滚轮浏览横向缩略图条带).
  // 主要方向是水平时早返回, 不接管; 主要方向是垂直时 scrollLeft += deltaY
  // + preventDefault. 这条 case 把两条分支都跑过.
  it("垂直滚轮翻成横向; 水平滚轮放行", () => {
    render(
      <LibraryFanartStrip
        fanartFiles={[fakeFile]}
        isPending={false}
        uploadActiveRef={{ current: false }}
        resolveImage={(p) => p}
        onOpenUploadPicker={vi.fn()}
        onDeleteFanart={vi.fn()}
        onOpenPreview={vi.fn()}
      />,
    );
    const strip = document.querySelector(".library-fanart-strip") as HTMLDivElement;
    strip.scrollLeft = 0;
    fireEvent.wheel(strip, { deltaX: 5, deltaY: 80 });
    expect(strip.scrollLeft).toBe(80);
    fireEvent.wheel(strip, { deltaX: 200, deltaY: 10 });
    expect(strip.scrollLeft).toBe(80);
  });
});
