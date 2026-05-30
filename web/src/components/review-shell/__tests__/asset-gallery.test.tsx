// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ReviewCoverCard, ReviewFanartStrip, ReviewPosterCard } from "../asset-gallery";

afterEach(() => {
  cleanup();
});

const fakeImage = { key: "k1", name: "p.jpg", rel_path: "" };

describe("ReviewPosterCard", () => {
  it("正常路径: 有 poster → 渲染 image hit + 上传按钮 + 裁剪按钮; 点击图区调 onOpenPreview", () => {
    const onOpenPreview = vi.fn();
    const onOpenCropper = vi.fn();
    const onOpenUploadPicker = vi.fn();
    const ref = createRef<boolean>();
    render(
      <ReviewPosterCard
        poster={fakeImage}
        uploadActiveRef={ref as React.RefObject<boolean>}
        onOpenCropper={onOpenCropper}
        onOpenUploadPicker={onOpenUploadPicker}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(screen.getAllByRole("button").find((b) => b.className.includes("review-image-hit"))!);
    expect(onOpenPreview).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "上传海报" }));
    expect(onOpenUploadPicker).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "从封面截取海报" }));
    expect(onOpenCropper).toHaveBeenCalledTimes(1);
  });

  it("异常路径: uploadActiveRef.current=true → 点图区不弹预览 (上传过程中防误触)", () => {
    const onOpenPreview = vi.fn();
    const activeRef = { current: true } as React.RefObject<boolean>;
    render(
      <ReviewPosterCard
        poster={fakeImage}
        uploadActiveRef={activeRef}
        onOpenCropper={vi.fn()}
        onOpenUploadPicker={vi.fn()}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(screen.getAllByRole("button").find((b) => b.className.includes("review-image-hit"))!);
    expect(onOpenPreview).not.toHaveBeenCalled();
  });

  it("边缘路径: poster 为 null → 渲染空槽 + 上传按钮 + 裁剪按钮", () => {
    const onOpenUploadPicker = vi.fn();
    render(
      <ReviewPosterCard
        poster={null}
        uploadActiveRef={{ current: false } as React.RefObject<boolean>}
        onOpenCropper={vi.fn()}
        onOpenUploadPicker={onOpenUploadPicker}
        onOpenPreview={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "上传海报" }));
    expect(onOpenUploadPicker).toHaveBeenCalledTimes(1);
    expect(screen.getByText("海报")).toBeTruthy();
  });
});

describe("ReviewCoverCard", () => {
  it("正常路径: 有 cover → 渲染图区 + 上传按钮; 点图区调 onOpenPreview", () => {
    const onOpenPreview = vi.fn();
    render(
      <ReviewCoverCard
        cover={fakeImage}
        uploadActiveRef={{ current: false } as React.RefObject<boolean>}
        onOpenUploadPicker={vi.fn()}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(screen.getAllByRole("button").find((b) => b.className.includes("review-image-hit"))!);
    expect(onOpenPreview).toHaveBeenCalledTimes(1);
  });

  it("异常路径: 上传按钮 click 触发 onOpenUploadPicker 但不冒泡到图片预览", () => {
    const onOpenUploadPicker = vi.fn();
    const onOpenPreview = vi.fn();
    render(
      <ReviewCoverCard
        cover={fakeImage}
        uploadActiveRef={{ current: false } as React.RefObject<boolean>}
        onOpenUploadPicker={onOpenUploadPicker}
        onOpenPreview={onOpenPreview}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "上传封面" }));
    expect(onOpenUploadPicker).toHaveBeenCalledTimes(1);
    expect(onOpenPreview).not.toHaveBeenCalled();
  });

  it("边缘路径: cover 为 undefined → 渲染空槽, 上传按钮仍可用", () => {
    const onOpenUploadPicker = vi.fn();
    render(
      <ReviewCoverCard
        cover={undefined}
        uploadActiveRef={{ current: false } as React.RefObject<boolean>}
        onOpenUploadPicker={onOpenUploadPicker}
        onOpenPreview={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "上传封面" }));
    expect(onOpenUploadPicker).toHaveBeenCalledTimes(1);
  });
});

describe("ReviewFanartStrip", () => {
  const samples = [
    { key: "k1", name: "f1.jpg", rel_path: "" },
    { key: "k2", name: "f2.jpg", rel_path: "" },
  ];

  it("正常路径: 渲染每个 sample image + 删除按钮 + 末尾上传槽; 点击 thumb 调 onOpenPreview", () => {
    const onOpenPreview = vi.fn();
    render(
      <ReviewFanartStrip
        sampleImages={samples}
        uploadActiveRef={{ current: false } as React.RefObject<boolean>}
        onOpenUploadPicker={vi.fn()}
        onRemoveFanart={vi.fn()}
        onOpenPreview={onOpenPreview}
      />,
    );
    const hits = document.querySelectorAll("button.review-image-hit");
    expect(hits.length).toBe(2);
    fireEvent.click(hits[0]);
    expect(onOpenPreview).toHaveBeenCalledTimes(1);
  });

  it("异常路径: 点 X 按钮触发 onRemoveFanart(key)", () => {
    const onRemoveFanart = vi.fn();
    render(
      <ReviewFanartStrip
        sampleImages={samples}
        uploadActiveRef={{ current: false } as React.RefObject<boolean>}
        onOpenUploadPicker={vi.fn()}
        onRemoveFanart={onRemoveFanart}
        onOpenPreview={vi.fn()}
      />,
    );
    const removeBtns = screen.getAllByRole("button", { name: "删除 fanart" });
    fireEvent.click(removeBtns[1]);
    expect(onRemoveFanart).toHaveBeenCalledWith("k2");
  });

  it("边缘路径: sampleImages=undefined 仍渲染上传槽, 不抛错", () => {
    const onOpenUploadPicker = vi.fn();
    const { container } = render(
      <ReviewFanartStrip
        sampleImages={undefined}
        uploadActiveRef={{ current: false } as React.RefObject<boolean>}
        onOpenUploadPicker={onOpenUploadPicker}
        onRemoveFanart={vi.fn()}
        onOpenPreview={vi.fn()}
      />,
    );
    expect(document.querySelectorAll(".review-image-hit").length).toBe(0);
    const uploadBtn = container.querySelector("button.review-fanart-item.review-upload-empty") as HTMLElement;
    fireEvent.click(uploadBtn);
    expect(onOpenUploadPicker).toHaveBeenCalledTimes(1);
  });

  // onWheel 把垂直滚动转换为水平滚动 (用户用滚轮浏览横向缩略图条带).
  // 两条分支: 主要方向是水平 (|deltaX| > |deltaY|) → 早返回不接管;
  // 主要方向是垂直 → 把 deltaY 加到 scrollLeft 并 preventDefault.
  it("ReviewFanartStrip onWheel: 垂直滚轮翻成横向; 水平滚轮直接放行", () => {
    render(
      <ReviewFanartStrip
        sampleImages={samples}
        uploadActiveRef={{ current: false } as React.RefObject<boolean>}
        onOpenUploadPicker={vi.fn()}
        onRemoveFanart={vi.fn()}
        onOpenPreview={vi.fn()}
      />,
    );
    const strip = document.querySelector(".review-fanart-strip") as HTMLDivElement;
    // 强制设一个初值, 否则 jsdom 默认 0, 加 0 看不出来.
    strip.scrollLeft = 0;
    fireEvent.wheel(strip, { deltaX: 5, deltaY: 80 });
    expect(strip.scrollLeft).toBe(80);
    // 水平为主时不接管 — scrollLeft 不再变.
    fireEvent.wheel(strip, { deltaX: 200, deltaY: 10 });
    expect(strip.scrollLeft).toBe(80);
  });
});
