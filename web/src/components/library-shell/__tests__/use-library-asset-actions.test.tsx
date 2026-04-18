// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { createRef, startTransition as reactStartTransition, type RefObject } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LibraryDetail, LibraryVariant } from "@/lib/api";

import type { LibraryPreviewState } from "../asset-gallery";
import { useLibraryAssetActions } from "../use-library-asset-actions";

// useLibraryAssetActions: 库详情页的资产操作 hook (封面 / 海报 / fanart 的
// 上传、删除、裁剪). 测试目标:
//   1. resolveImage 三分支: override > versioned > bare.
//   2. 上传流程: 点 upload -> 模拟 <input> 文件选择 -> replaceLibraryAsset ->
//      syncDetail + setItems + assetOverride 注入.
//   3. 删除 fanart: 清 override + version, 如果正在预览这张图则 setPreview(null).
//   4. openCropper 守卫: 没选中 cover 时直接 return.
//   5. handleConfirmCrop: 走 cropLibraryPosterFromCover + bumpAssetVersion.
//   6. 错误路径: setMessage 显示错误, override 不泄漏.
//   7. URL.createObjectURL / revokeObjectURL 的生命周期 (unmount cleanup).

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getLibraryFileURL: vi.fn((path: string) => `/api/library/file?path=${encodeURIComponent(path)}`),
    replaceLibraryAsset: vi.fn(),
    deleteLibraryFile: vi.fn(),
    cropLibraryPosterFromCover: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockReplaceAsset = vi.mocked(api.replaceLibraryAsset);
const mockDeleteFile = vi.mocked(api.deleteLibraryFile);
const mockCropPoster = vi.mocked(api.cropLibraryPosterFromCover);

// jsdom 默认不实现 createObjectURL / revokeObjectURL, 自己塞进去并计数.
let objectURLCounter = 0;
const createdURLs = new Set<string>();
const revokedURLs: string[] = [];

function installURLStubs() {
  objectURLCounter = 0;
  createdURLs.clear();
  revokedURLs.length = 0;
  Object.defineProperty(URL, "createObjectURL", {
    value: vi.fn(() => {
      objectURLCounter += 1;
      const url = `blob:test/${objectURLCounter}`;
      createdURLs.add(url);
      return url;
    }),
    configurable: true,
    writable: true,
  });
  Object.defineProperty(URL, "revokeObjectURL", {
    value: vi.fn((url: string) => {
      revokedURLs.push(url);
      createdURLs.delete(url);
    }),
    configurable: true,
    writable: true,
  });
}

function makeVariant(overrides: Partial<LibraryVariant> = {}): LibraryVariant {
  return {
    key: "v1",
    label: "Main",
    base_name: "main",
    suffix: "",
    is_primary: true,
    video_path: "/lib/m.mp4",
    nfo_path: "/lib/m.nfo",
    poster_path: "/lib/m-poster.jpg",
    cover_path: "/lib/m-cover.jpg",
    meta: {
      title: "",
      title_translated: "",
      original_title: "",
      plot: "",
      plot_translated: "",
      number: "",
      release_date: "",
      runtime: 0,
      studio: "",
      label: "",
      series: "",
      director: "",
      actors: [],
      genres: [],
      poster_path: "",
      cover_path: "",
      fanart_path: "",
      thumb_path: "",
      source: "",
      scraped_at: "",
    } as never,
    files: [],
    file_count: 1,
    ...overrides,
  };
}

function makeDetail(overrides: Partial<LibraryDetail> = {}): LibraryDetail {
  const variant = makeVariant();
  return {
    item: {
      rel_path: "movies/demo/",
      name: "demo",
      title: "demo",
      number: "ABC-001",
      release_date: "2024-01-01",
      actors: [],
      created_at: 0,
      updated_at: 0,
      has_nfo: true,
      poster_path: "/lib/m-poster.jpg",
      cover_path: "/lib/m-cover.jpg",
      total_size: 0,
      file_count: 1,
      video_count: 1,
      variant_count: 1,
    } as never,
    meta: variant.meta,
    variants: [variant],
    primary_variant_key: "v1",
    files: [],
    ...overrides,
  };
}

interface RenderOpts {
  detail?: LibraryDetail | null;
  detailRef?: RefObject<LibraryDetail | null>;
  currentVariant?: LibraryVariant | null;
  selectedCover?: string;
  preview?: LibraryPreviewState;
}

function renderAssetActions(opts: RenderOpts = {}) {
  const detail = opts.detail === undefined ? makeDetail() : opts.detail;
  const detailRef = opts.detailRef ?? { current: detail };
  const currentVariant =
    opts.currentVariant === undefined ? (detail?.variants[0] ?? null) : opts.currentVariant;
  const syncDetail = vi.fn();
  const setItems = vi.fn();
  const setMessage = vi.fn();
  const setPreview = vi.fn();
  const setCropOpen = vi.fn();
  const hook = renderHook(() =>
    useLibraryAssetActions({
      detail,
      detailRef,
      currentVariant,
      selectedVariantKey: currentVariant?.key ?? "",
      selectedCover: opts.selectedCover ?? detail?.item.cover_path ?? "",
      syncDetail,
      setItems,
      setMessage,
      startTransition: reactStartTransition,
      preview: opts.preview ?? null,
      setPreview,
      setCropOpen,
    }),
  );
  return { hook, syncDetail, setItems, setMessage, setPreview, setCropOpen };
}

async function flushAsync() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });
}

// simulatePickFile: 模拟用户点击 input -> 选文件 -> change 事件. 我们把
// 原来的 createElement 打了个桩, 记下最近一个 <input>, 然后手动触发 change.
let lastInput: HTMLInputElement | null = null;
function installInputStub() {
  lastInput = null;
  const originalCreate = document.createElement.bind(document);
  vi.spyOn(document, "createElement").mockImplementation((tag: string) => {
    const el = originalCreate(tag) as HTMLElement;
    if (tag === "input") {
      lastInput = el as HTMLInputElement;
      // 覆盖 .click() 让它不要真的打开文件对话框 (jsdom 是 no-op, 这里只是保险).
      (el as HTMLInputElement).click = vi.fn();
    }
    return el;
  });
}

function firePickedFile(name = "test.png", type = "image/png") {
  if (!lastInput) throw new Error("no input captured");
  const file = new File(["x"], name, { type });
  Object.defineProperty(lastInput, "files", {
    value: {
      0: file,
      length: 1,
      item(i: number) {
        return i === 0 ? file : null;
      },
    },
    configurable: true,
  });
  lastInput.dispatchEvent(new Event("change"));
  return file;
}

function fireCancel() {
  if (!lastInput) throw new Error("no input captured");
  Object.defineProperty(lastInput, "files", { value: { length: 0, item: () => null }, configurable: true });
  lastInput.dispatchEvent(new Event("cancel"));
}

beforeEach(() => {
  vi.useFakeTimers();
  installURLStubs();
  installInputStub();
  mockReplaceAsset.mockReset();
  mockDeleteFile.mockReset();
  mockCropPoster.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("resolveImage", () => {
  it("no override, no version: bare getLibraryFileURL", () => {
    const { hook } = renderAssetActions();
    expect(hook.result.current.resolveImage("a/b.jpg")).toBe("/api/library/file?path=a%2Fb.jpg");
  });
});

describe("openUploadPicker - poster success", () => {
  it("replaces poster: calls API, syncs detail + items, adds override", async () => {
    const next = makeDetail({ primary_variant_key: "v1" });
    mockReplaceAsset.mockResolvedValue(next);

    const { hook, syncDetail, setItems, setMessage } = renderAssetActions();
    act(() => {
      hook.result.current.openUploadPicker("poster");
    });
    act(() => {
      firePickedFile();
    });
    await flushAsync();

    expect(mockReplaceAsset).toHaveBeenCalledWith(
      "movies/demo/",
      "v1",
      "poster",
      expect.any(File),
    );
    expect(syncDetail).toHaveBeenCalledWith(next);
    expect(setItems).toHaveBeenCalled();
    expect(setMessage).toHaveBeenCalledWith("替换当前实例海报...");
    expect(setMessage).toHaveBeenCalledWith("当前实例海报已更新");
    // poster override 写入 (poster 路径非空)
    expect(hook.result.current.resolveImage("/lib/m-poster.jpg")).toMatch(/^blob:test\//);
  });
});

describe("openUploadPicker - cover / fanart / cancel paths", () => {
  it("cover: sets override on cover path", async () => {
    const next = makeDetail();
    mockReplaceAsset.mockResolvedValue(next);

    const { hook } = renderAssetActions();
    act(() => hook.result.current.openUploadPicker("cover"));
    act(() => firePickedFile());
    await flushAsync();

    expect(hook.result.current.resolveImage("/lib/m-cover.jpg")).toMatch(/^blob:test\//);
  });

  it("fanart: does NOT add override (fanart url-version 走 bumpAssetVersion 的是 crop/删除分支)", async () => {
    const next = makeDetail();
    mockReplaceAsset.mockResolvedValue(next);

    const { hook } = renderAssetActions();
    act(() => hook.result.current.openUploadPicker("fanart"));
    act(() => firePickedFile());
    await flushAsync();

    // fanart 分支在成功路径里既不走 setAssetOverride 也不走 bumpAssetVersion.
    expect(hook.result.current.resolveImage("/lib/m-cover.jpg")).toBe(
      "/api/library/file?path=%2Flib%2Fm-cover.jpg",
    );
  });

  it("user cancels file dialog: no API call", async () => {
    const { hook } = renderAssetActions();
    act(() => hook.result.current.openUploadPicker("poster"));
    act(() => fireCancel());
    await flushAsync();

    expect(mockReplaceAsset).not.toHaveBeenCalled();
  });

  it("empty file list on change: no API call", async () => {
    const { hook } = renderAssetActions();
    act(() => hook.result.current.openUploadPicker("poster"));

    // 把 files 手动设成空
    Object.defineProperty(lastInput!, "files", { value: { length: 0, item: () => null }, configurable: true });
    act(() => {
      lastInput!.dispatchEvent(new Event("change"));
    });
    await flushAsync();

    expect(mockReplaceAsset).not.toHaveBeenCalled();
  });

  it("detail is null: openUploadPicker is a no-op", () => {
    const { hook } = renderAssetActions({ detail: null });
    hook.result.current.openUploadPicker("poster");
    // 没有 input 被创建 (lastInput 仍然是 null, 或不是此次调用的)
    expect(mockReplaceAsset).not.toHaveBeenCalled();
  });

  it("API error: setMessage carries server error text, sync/setItems not called", async () => {
    mockReplaceAsset.mockRejectedValue(new Error("server exploded"));

    const { hook, syncDetail, setItems, setMessage } = renderAssetActions();
    act(() => hook.result.current.openUploadPicker("poster"));
    act(() => firePickedFile());
    await flushAsync();

    expect(setMessage).toHaveBeenCalledWith("server exploded");
    expect(syncDetail).not.toHaveBeenCalled();
    expect(setItems).not.toHaveBeenCalled();
  });
});

describe("handleDeleteFanart", () => {
  it("normal delete: syncDetail, setItems, setMessage", async () => {
    const next = makeDetail();
    mockDeleteFile.mockResolvedValue(next);

    const { hook, syncDetail, setItems, setMessage } = renderAssetActions();
    act(() => hook.result.current.handleDeleteFanart("/lib/fan.jpg"));
    await flushAsync();

    expect(mockDeleteFile).toHaveBeenCalledWith("/lib/fan.jpg");
    expect(syncDetail).toHaveBeenCalledWith(next);
    expect(setItems).toHaveBeenCalled();
    expect(setMessage).toHaveBeenCalledWith("删除 extrafanart...");
    expect(setMessage).toHaveBeenCalledWith("Extrafanart 已删除");
  });

  it("resets preview if it was pointing to the deleted path", async () => {
    const next = makeDetail();
    mockDeleteFile.mockResolvedValue(next);

    const { hook, setPreview } = renderAssetActions({
      preview: { path: "/lib/fan.jpg", url: "blob:orig" } as never,
    });
    act(() => hook.result.current.handleDeleteFanart("/lib/fan.jpg"));
    await flushAsync();

    expect(setPreview).toHaveBeenCalledWith(null);
  });

  it("keeps preview if preview.path != deleted path", async () => {
    const next = makeDetail();
    mockDeleteFile.mockResolvedValue(next);

    const { hook, setPreview } = renderAssetActions({
      preview: { path: "/lib/other.jpg", url: "blob:orig" } as never,
    });
    act(() => hook.result.current.handleDeleteFanart("/lib/fan.jpg"));
    await flushAsync();

    expect(setPreview).not.toHaveBeenCalled();
  });

  it("delete error: setMessage error text, state unchanged", async () => {
    mockDeleteFile.mockRejectedValue(new Error("no perm"));

    const { hook, syncDetail, setMessage } = renderAssetActions();
    act(() => hook.result.current.handleDeleteFanart("/lib/fan.jpg"));
    await flushAsync();

    expect(setMessage).toHaveBeenCalledWith("no perm");
    expect(syncDetail).not.toHaveBeenCalled();
  });
});

describe("openCropper", () => {
  it("no selectedCover: noop, cropOpen stays closed", () => {
    const { hook, setCropOpen } = renderAssetActions({ selectedCover: "" });
    hook.result.current.openCropper();
    expect(setCropOpen).not.toHaveBeenCalled();
  });

  it("with cover selected: flips cropOpen true", () => {
    const { hook, setCropOpen } = renderAssetActions({ selectedCover: "/lib/m-cover.jpg" });
    hook.result.current.openCropper();
    expect(setCropOpen).toHaveBeenCalledWith(true);
  });
});

describe("handleConfirmCrop", () => {
  it("success: crops poster, syncs, closes modal, bumps poster version", async () => {
    const next = makeDetail();
    mockCropPoster.mockResolvedValue(next);

    const { hook, syncDetail, setItems, setCropOpen, setMessage } = renderAssetActions({
      selectedCover: "/lib/m-cover.jpg",
    });
    act(() => {
      hook.result.current.handleConfirmCrop({ x: 0, y: 0, width: 10, height: 10 } as never);
    });
    await flushAsync();

    expect(mockCropPoster).toHaveBeenCalledWith(
      "movies/demo/",
      "v1",
      expect.objectContaining({ x: 0, y: 0, width: 10, height: 10 }),
    );
    expect(syncDetail).toHaveBeenCalledWith(next);
    expect(setItems).toHaveBeenCalled();
    expect(setCropOpen).toHaveBeenCalledWith(false);
    expect(setMessage).toHaveBeenCalledWith("从封面截取海报...");
    expect(setMessage).toHaveBeenCalledWith("海报已更新");

    // bumpAssetVersion -> 再 resolveImage 带上 &v=...
    expect(hook.result.current.resolveImage("/lib/m-poster.jpg")).toMatch(/&v=\d+/);
  });

  it("no detail: noop", () => {
    const { hook, setMessage } = renderAssetActions({ detail: null, selectedCover: "/lib/cover.jpg" });
    hook.result.current.handleConfirmCrop({ x: 0, y: 0, width: 10, height: 10 } as never);
    expect(setMessage).not.toHaveBeenCalled();
  });

  it("no selectedCover: noop", () => {
    const { hook, setMessage } = renderAssetActions({ selectedCover: "" });
    hook.result.current.handleConfirmCrop({ x: 0, y: 0, width: 10, height: 10 } as never);
    expect(setMessage).not.toHaveBeenCalled();
  });

  it("API error: setMessage error text, modal stays open", async () => {
    mockCropPoster.mockRejectedValue(new Error("bad rect"));

    const { hook, setMessage, setCropOpen } = renderAssetActions({ selectedCover: "/lib/m-cover.jpg" });
    act(() => {
      hook.result.current.handleConfirmCrop({ x: 0, y: 0, width: 10, height: 10 } as never);
    });
    await flushAsync();

    expect(setMessage).toHaveBeenCalledWith("bad rect");
    // setCropOpen(false) 仅在成功分支里调用
    expect(setCropOpen).not.toHaveBeenCalled();
  });
});

describe("URL lifecycle", () => {
  it("replaces an existing override: revokes old blob URL", async () => {
    const next = makeDetail();
    mockReplaceAsset.mockResolvedValue(next);

    const { hook } = renderAssetActions();
    // 连续上传两次相同 kind, 第二次会 replace override -> 触发 revoke.
    act(() => hook.result.current.openUploadPicker("poster"));
    act(() => firePickedFile("first.png"));
    await flushAsync();

    act(() => hook.result.current.openUploadPicker("poster"));
    act(() => firePickedFile("second.png"));
    await flushAsync();

    expect(revokedURLs.length).toBeGreaterThan(0);
  });

  it("unmount: revokes all remaining overrides (cleanup effect)", async () => {
    const next = makeDetail();
    mockReplaceAsset.mockResolvedValue(next);

    const { hook } = renderAssetActions();
    act(() => hook.result.current.openUploadPicker("poster"));
    act(() => firePickedFile());
    await flushAsync();

    const before = revokedURLs.length;
    hook.unmount();
    expect(revokedURLs.length).toBeGreaterThan(before);
  });
});

describe("uploadActiveRef gating", () => {
  it("set true during picker flow, reset to false after unlock timeout", async () => {
    const next = makeDetail();
    mockReplaceAsset.mockResolvedValue(next);

    const { hook } = renderAssetActions();
    act(() => hook.result.current.openUploadPicker("poster"));
    expect(hook.result.current.uploadActiveRef.current).toBe(true);

    act(() => firePickedFile());
    await flushAsync();

    // unlock 有 300ms setTimeout
    await act(async () => {
      vi.advanceTimersByTime(400);
      await Promise.resolve();
    });
    expect(hook.result.current.uploadActiveRef.current).toBe(false);
  });
});

describe("resolveImage - version cache bust", () => {
  it("bumps version on crop: resolveImage appends &v=... after crop", async () => {
    const next = makeDetail();
    mockCropPoster.mockResolvedValue(next);

    const { hook } = renderAssetActions({ selectedCover: "/lib/m-cover.jpg" });
    expect(hook.result.current.resolveImage("/lib/m-poster.jpg")).toBe(
      "/api/library/file?path=%2Flib%2Fm-poster.jpg",
    );

    act(() => {
      hook.result.current.handleConfirmCrop({ x: 0, y: 0, width: 1, height: 1 } as never);
    });
    await flushAsync();

    expect(hook.result.current.resolveImage("/lib/m-poster.jpg")).toMatch(/&v=\d+$/);
  });
});

describe("misc", () => {
  it("detailRef is read via ref.current for the CURRENT-poster-before-crop path", async () => {
    // 这条断住实现里那个微妙点: handleConfirmCrop 清 override 时读的是
    // detailRef.current 而不是 closure 里的 detail (防止用户在 crop 打开期间
    // 详情切换到另一份的潜在竞争). 用不同的 detail/detailRef 让它走 ref.
    const detail = makeDetail();
    const altDetail = makeDetail({ item: { ...detail.item, poster_path: "/lib/alt-poster.jpg" } as never });
    const detailRef = createRef<LibraryDetail | null>() as RefObject<LibraryDetail | null>;
    detailRef.current = altDetail;
    mockCropPoster.mockResolvedValue(detail);

    const { hook } = renderAssetActions({ detail, detailRef, selectedCover: "/lib/m-cover.jpg" });
    act(() => {
      hook.result.current.handleConfirmCrop({ x: 0, y: 0, width: 1, height: 1 } as never);
    });
    await flushAsync();

    // 最终会 bumpAssetVersion 的路径来自 next (= detail), resolveImage 应看到 version.
    expect(hook.result.current.resolveImage("/lib/m-poster.jpg")).toMatch(/&v=\d+/);
  });
});
