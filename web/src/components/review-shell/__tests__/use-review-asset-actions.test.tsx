// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { JobItem, MediaFileRef, ReviewMeta } from "@/lib/api";

import { useReviewAssetActions, type UseReviewAssetActionsDeps } from "../use-review-asset-actions";

// useReviewAssetActions: review 页面的资产操作 (上传封面/海报/fanart,
// 从封面截取海报, 删除 fanart). 覆盖:
// - openCropper: meta.cover 空 -> 不打开; 有 cover 时 setCropOpen(true)
// - handleCropResult: selected 空直接 return; 成功 -> updateMeta + lastSavedPayloadRef
//   + 关闭裁剪; 失败 -> setMessage
// - handleRemoveFanart: selected/meta 任一空 no-op; 成功保存过滤后的 sample_images
// - openUploadPicker: 触发 file input, 选择文件后调 uploadAsset + saveReviewJob,
//   cancel 事件只释放 uploadActiveRef
// - uploadActiveRef 在上传结束后 300ms 才解锁

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    cropPosterFromCover: vi.fn(),
    saveReviewJob: vi.fn(),
    uploadAsset: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockCrop = vi.mocked(api.cropPosterFromCover);
const mockSave = vi.mocked(api.saveReviewJob);
const mockUpload = vi.mocked(api.uploadAsset);

function makeJob(overrides: Partial<JobItem> = {}): JobItem {
  return {
    id: 7,
    rel_path: "a/b.mp4",
    number: "ABC-001",
    status: "reviewing",
    created_at: 0,
    updated_at: 0,
    error_message: "",
    conflict_reason: "",
    raw_number: "",
    cleaned_number: "",
    number_source: "manual",
    number_clean_status: "success",
    number_clean_confidence: "high",
    number_clean_warnings: "",
    ...overrides,
  };
}

function makeMeta(overrides: Partial<ReviewMeta> = {}): ReviewMeta {
  return { number: "ABC-001", title: "t", ...overrides };
}

function makeAsset(key = "new-k"): MediaFileRef {
  return { key, url: `https://cdn/${key}.jpg` } as MediaFileRef;
}

async function flushAsync(ticks = 8) {
  await act(async () => {
    for (let i = 0; i < ticks; i += 1) {
      await Promise.resolve();
    }
  });
}

function renderAsset(
  opts: Partial<UseReviewAssetActionsDeps> & { initialMeta?: ReviewMeta | null } = {},
) {
  const initialMeta = "initialMeta" in opts ? opts.initialMeta ?? null : makeMeta();
  const metaRef = { current: initialMeta } as { current: ReviewMeta | null };
  const lastSavedPayloadRef = { current: "" };
  const setMeta = vi.fn();
  const updateMeta = vi.fn((patch: Partial<ReviewMeta>) => {
    metaRef.current = { ...(metaRef.current ?? {}), ...patch } as ReviewMeta;
  });
  const setMessage = vi.fn();
  const setCropOpen = vi.fn();
  const startTransition = (cb: () => void) => cb();

  const deps: UseReviewAssetActionsDeps = {
    selected: "selected" in opts ? (opts.selected as JobItem | null) : makeJob(),
    meta: "meta" in opts ? (opts.meta as ReviewMeta | null) : initialMeta,
    metaRef,
    lastSavedPayloadRef,
    setMeta,
    updateMeta,
    setMessage,
    setCropOpen,
    startTransition,
  };

  const hook = renderHook(() => useReviewAssetActions(deps));
  return { hook, deps, metaRef, lastSavedPayloadRef, setMeta, updateMeta, setMessage, setCropOpen };
}

beforeEach(() => {
  mockCrop.mockReset();
  mockSave.mockReset();
  mockUpload.mockReset();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("openCropper", () => {
  it("meta.cover 空时不打开裁剪框", () => {
    const { hook, setCropOpen } = renderAsset({ meta: makeMeta({ cover: null }) });
    act(() => {
      hook.result.current.openCropper();
    });
    expect(setCropOpen).not.toHaveBeenCalled();
  });

  it("meta.cover 非空时 setCropOpen(true)", () => {
    const { hook, setCropOpen } = renderAsset({
      meta: makeMeta({ cover: makeAsset("cov") }),
    });
    act(() => {
      hook.result.current.openCropper();
    });
    expect(setCropOpen).toHaveBeenCalledWith(true);
  });
});

describe("handleCropResult", () => {
  it("selected 为空直接 return", async () => {
    const { hook } = renderAsset({ selected: null });
    act(() => {
      hook.result.current.handleCropResult({ x: 0, y: 0, width: 1, height: 1 });
    });
    await flushAsync();
    expect(mockCrop).not.toHaveBeenCalled();
  });

  it("成功: updateMeta(poster) + setCropOpen(false) + setMessage '海报已更新'", async () => {
    const posterAsset = makeAsset("pos");
    mockCrop.mockResolvedValue(posterAsset);
    const { hook, updateMeta, setCropOpen, setMessage, metaRef, lastSavedPayloadRef } = renderAsset({
      initialMeta: makeMeta({ cover: makeAsset("cov") }),
    });
    act(() => {
      hook.result.current.handleCropResult({ x: 0, y: 0, width: 10, height: 20 });
    });
    await flushAsync();
    expect(mockCrop).toHaveBeenCalledWith(7, { x: 0, y: 0, width: 10, height: 20 });
    expect(updateMeta).toHaveBeenCalledWith({ poster: posterAsset });
    expect(metaRef.current?.poster).toEqual(posterAsset);
    expect(lastSavedPayloadRef.current).not.toBe("");
    expect(setCropOpen).toHaveBeenCalledWith(false);
    expect(setMessage).toHaveBeenLastCalledWith("海报已更新");
  });

  it("crop 失败: setMessage 使用 error.message", async () => {
    mockCrop.mockRejectedValue(new Error("crop failed"));
    const { hook, setMessage, setCropOpen } = renderAsset({
      initialMeta: makeMeta({ cover: makeAsset("cov") }),
    });
    act(() => {
      hook.result.current.handleCropResult({ x: 0, y: 0, width: 1, height: 1 });
    });
    await flushAsync();
    expect(setMessage).toHaveBeenLastCalledWith("crop failed");
    expect(setCropOpen).not.toHaveBeenCalled();
  });

  it("crop 抛非 Error 时使用兜底文案", async () => {
    mockCrop.mockRejectedValue("raw");
    const { hook, setMessage } = renderAsset({
      initialMeta: makeMeta({ cover: makeAsset("cov") }),
    });
    act(() => {
      hook.result.current.handleCropResult({ x: 0, y: 0, width: 1, height: 1 });
    });
    await flushAsync();
    expect(setMessage).toHaveBeenLastCalledWith("海报截取失败");
  });
});

describe("handleRemoveFanart", () => {
  it("selected 或 metaRef.current 任一空: no-op", async () => {
    const { hook } = renderAsset({ selected: null });
    act(() => {
      hook.result.current.handleRemoveFanart("k1");
    });
    await flushAsync();
    expect(mockSave).not.toHaveBeenCalled();
  });

  it("成功: 保存过滤后的 sample_images, lastSavedPayloadRef 被更新", async () => {
    mockSave.mockResolvedValue({});
    const meta = makeMeta({
      sample_images: [makeAsset("a"), makeAsset("b"), makeAsset("c")],
    });
    const { hook, lastSavedPayloadRef, setMeta, metaRef } = renderAsset({ initialMeta: meta });
    act(() => {
      hook.result.current.handleRemoveFanart("b");
    });
    await flushAsync();
    expect(setMeta).toHaveBeenCalledTimes(1);
    const nextMeta = setMeta.mock.calls[0][0] as ReviewMeta;
    expect(nextMeta.sample_images?.map((i) => i.key)).toEqual(["a", "c"]);
    expect(metaRef.current).toBe(nextMeta);
    expect(mockSave).toHaveBeenCalledTimes(1);
    expect(lastSavedPayloadRef.current).not.toBe("");
  });

  it("saveReviewJob 失败: setMessage 用 error.message", async () => {
    mockSave.mockRejectedValue(new Error("save x"));
    const meta = makeMeta({ sample_images: [makeAsset("a")] });
    const { hook, setMessage } = renderAsset({ initialMeta: meta });
    act(() => {
      hook.result.current.handleRemoveFanart("a");
    });
    await flushAsync();
    expect(setMessage).toHaveBeenLastCalledWith("save x");
  });
});

describe("openUploadPicker", () => {
  function installInputStub() {
    const input = {
      type: "",
      accept: "",
      files: null as FileList | null,
      click: vi.fn(),
      listeners: {} as Record<string, (event: Event) => void>,
      addEventListener(event: string, cb: (event: Event) => void) {
        this.listeners[event] = cb;
      },
    };
    // spy + mockImplementation 只能拿到 createElement 的 generic string
    // overload (已被 TS lib 标记 deprecated). 这里的 originalCreate/调用都走
    // 同一个 overload, 是 mock 实现细节, 不是被测代码本身.
    /* eslint-disable @typescript-eslint/no-deprecated */
    const originalCreate = document.createElement.bind(document);
    const spy = vi.spyOn(document, "createElement").mockImplementation(((tag: string) => {
      if (tag === "input") {
        return input as unknown as HTMLElement;
      }
      return originalCreate(tag);
    }) as typeof document.createElement);
    /* eslint-enable @typescript-eslint/no-deprecated */
    return { input, spy };
  }

  it("cover 上传成功: 调用 uploadAsset + saveReviewJob, setMeta 写入 cover", async () => {
    const { input, spy } = installInputStub();
    const uploadedAsset = makeAsset("up-cover");
    mockUpload.mockResolvedValue(uploadedAsset);
    mockSave.mockResolvedValue({});

    const { hook, setMeta } = renderAsset({
      initialMeta: makeMeta(),
    });

    act(() => {
      hook.result.current.openUploadPicker("cover");
    });
    expect(input.click).toHaveBeenCalled();
    expect(hook.result.current.uploadActiveRef.current).toBe(true);

    const file = new File(["xxx"], "c.jpg", { type: "image/jpeg" });
    input.files = { 0: file, length: 1, item: (i: number) => (i === 0 ? file : null) } as unknown as FileList;
    act(() => {
      input.listeners.change(new Event("change"));
    });
    await flushAsync();

    expect(mockUpload).toHaveBeenCalledWith(file);
    expect(mockSave).toHaveBeenCalled();
    expect(setMeta).toHaveBeenCalledTimes(1);
    const next = setMeta.mock.calls[0][0] as ReviewMeta;
    expect(next.cover).toEqual(uploadedAsset);

    // 300ms 后 uploadActiveRef 自动解锁.
    await act(async () => {
      vi.advanceTimersByTime(400);
    });
    expect(hook.result.current.uploadActiveRef.current).toBe(false);
    spy.mockRestore();
  });

  it("poster 上传成功: nextMeta 把 poster 换新", async () => {
    const { input, spy } = installInputStub();
    const uploadedAsset = makeAsset("up-poster");
    mockUpload.mockResolvedValue(uploadedAsset);
    mockSave.mockResolvedValue({});

    const { hook, setMeta } = renderAsset({ initialMeta: makeMeta() });
    act(() => {
      hook.result.current.openUploadPicker("poster");
    });
    const file = new File(["xxx"], "p.jpg", { type: "image/jpeg" });
    input.files = { 0: file, length: 1, item: (i: number) => (i === 0 ? file : null) } as unknown as FileList;
    act(() => {
      input.listeners.change(new Event("change"));
    });
    await flushAsync();
    const next = setMeta.mock.calls[0][0] as ReviewMeta;
    expect(next.poster).toEqual(uploadedAsset);
    spy.mockRestore();
  });

  it("fanart 上传: merge 到 sample_images, 同 key 会被替换", async () => {
    const { input, spy } = installInputStub();
    const uploadedAsset = makeAsset("dup-key");
    mockUpload.mockResolvedValue(uploadedAsset);
    mockSave.mockResolvedValue({});

    const initial = makeMeta({
      sample_images: [makeAsset("a"), makeAsset("dup-key"), makeAsset("b")],
    });
    const { hook, setMeta } = renderAsset({ initialMeta: initial });
    act(() => {
      hook.result.current.openUploadPicker("fanart");
    });
    const file = new File(["xxx"], "f.jpg", { type: "image/jpeg" });
    input.files = { 0: file, length: 1, item: (i: number) => (i === 0 ? file : null) } as unknown as FileList;
    act(() => {
      input.listeners.change(new Event("change"));
    });
    await flushAsync();
    const next = setMeta.mock.calls[0][0] as ReviewMeta;
    expect(next.sample_images?.map((i) => i.key)).toEqual(["a", "b", "dup-key"]);
    spy.mockRestore();
  });

  it("cancel 事件: 只释放 uploadActiveRef, 不调 uploadAsset", async () => {
    const { input, spy } = installInputStub();
    const { hook } = renderAsset({ initialMeta: makeMeta() });
    act(() => {
      hook.result.current.openUploadPicker("cover");
    });
    expect(hook.result.current.uploadActiveRef.current).toBe(true);
    act(() => {
      input.listeners.cancel(new Event("cancel"));
    });
    await act(async () => {
      vi.advanceTimersByTime(400);
    });
    expect(hook.result.current.uploadActiveRef.current).toBe(false);
    expect(mockUpload).not.toHaveBeenCalled();
    spy.mockRestore();
  });

  it("change 但没选文件 (input.files 为空): 不调 uploadAsset", async () => {
    const { input, spy } = installInputStub();
    const { hook } = renderAsset({ initialMeta: makeMeta() });
    act(() => {
      hook.result.current.openUploadPicker("cover");
    });
    input.files = null;
    act(() => {
      input.listeners.change(new Event("change"));
    });
    await flushAsync();
    expect(mockUpload).not.toHaveBeenCalled();
    spy.mockRestore();
  });

  it("uploadAsset 抛错: setMessage 用 error.message, 不改 meta", async () => {
    const { input, spy } = installInputStub();
    mockUpload.mockRejectedValue(new Error("upload x"));
    const { hook, setMeta, setMessage } = renderAsset({ initialMeta: makeMeta() });
    act(() => {
      hook.result.current.openUploadPicker("cover");
    });
    const file = new File(["xxx"], "c.jpg", { type: "image/jpeg" });
    input.files = { 0: file, length: 1, item: (i: number) => (i === 0 ? file : null) } as unknown as FileList;
    act(() => {
      input.listeners.change(new Event("change"));
    });
    await flushAsync();
    expect(setMessage).toHaveBeenLastCalledWith("upload x");
    expect(setMeta).not.toHaveBeenCalled();
    spy.mockRestore();
  });
});
