import { afterEach, describe, expect, it, vi } from "vitest";

import {
  deleteJob,
  explainMovieIDCleaner,
  getAPIBaseURL,
  getSearcherDebugPlugins,
  listJobs,
  listLibraryItems,
  replaceLibraryAsset,
  triggerScan,
  updateJobNumber,
  uploadAsset,
  uploadReviewAsset,
} from "@/lib/api";

function mockFetch(response: {
  ok: boolean;
  status: number;
  json?: () => Promise<unknown>;
  text?: () => Promise<string>;
}) {
  const fn = vi.fn().mockResolvedValue(response);
  vi.stubGlobal("fetch", fn);
  return fn;
}

function mockSuccess<T>(data: T) {
  return mockFetch({
    ok: true,
    status: 200,
    json: async () => ({ code: 0, message: "ok", data }),
  });
}

function mockBusinessError(message: string, code = 1) {
  return mockFetch({
    ok: true,
    status: 200,
    json: async () => ({ code, message, data: null }),
  });
}

function mockHTTPErrorJSON(status: number, message: string) {
  return mockFetch({
    ok: false,
    status,
    json: async () => ({ message }),
  });
}

function mockHTTPErrorHTML(status: number) {
  return mockFetch({
    ok: false,
    status,
    json: async () => { throw new SyntaxError("Unexpected token '<'"); },
  });
}

function mockNetworkError() {
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
}

describe("readAPIResponse via public API", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns data on success", async () => {
    mockSuccess({ items: [], total: 0, page: 1, page_size: 20 });
    const result = await listJobs();
    expect(result).toEqual({ items: [], total: 0, page: 1, page_size: 20 });
  });

  it("throws business error with server message", async () => {
    mockBusinessError("number already exists");
    await expect(listJobs()).rejects.toThrow("number already exists");
  });

  it("throws business error with fallback when server message is empty", async () => {
    mockBusinessError("");
    await expect(listJobs()).rejects.toThrow("request /api/jobs failed");
  });

  it("throws on HTTP error with JSON body containing message", async () => {
    mockHTTPErrorJSON(500, "internal server error");
    await expect(listJobs()).rejects.toThrow("internal server error");
  });

  it("throws on HTTP error with fallback when JSON message is empty", async () => {
    mockHTTPErrorJSON(404, "");
    await expect(listJobs()).rejects.toThrow(/HTTP 404/);
  });

  it("throws on HTTP error with JSON body missing message field", async () => {
    mockFetch({
      ok: false,
      status: 422,
      json: async () => ({ error: "validation" }),
    });
    await expect(listJobs()).rejects.toThrow(/HTTP 422/);
  });

  it("throws on HTTP error with non-JSON response body (HTML 502)", async () => {
    mockHTTPErrorHTML(502);
    await expect(listJobs()).rejects.toThrow(/HTTP 502/);
  });

  it("propagates network errors", async () => {
    mockNetworkError();
    await expect(listJobs()).rejects.toThrow("Failed to fetch");
  });
});

describe("apiRequest wrapper", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("sends GET with cache: no-store for list endpoints", async () => {
    const fn = mockSuccess([]);
    await listLibraryItems();
    expect(fn).toHaveBeenCalledOnce();
    const [url, init] = fn.mock.calls[0];
    expect(url).toMatch(/\/api\/library$/);
    expect(init.cache).toBe("no-store");
    expect(init.method).toBeUndefined();
  });

  it("sends POST with JSON content-type and body", async () => {
    const fn = mockSuccess({ input: "test", input_no_ext: "test", steps: [], final: {} });
    await explainMovieIDCleaner("test-input");
    expect(fn).toHaveBeenCalledOnce();
    const [url, init] = fn.mock.calls[0];
    expect(url).toMatch(/\/api\/debug\/movieid-cleaner\/explain$/);
    expect(init.method).toBe("POST");
    expect(init.headers).toEqual({ "Content-Type": "application/json" });
    expect(JSON.parse(init.body as string)).toEqual({ input: "test-input" });
  });

  it("sends PATCH with JSON content-type and body", async () => {
    const fn = mockSuccess({ id: 1, number: "ABC-123" });
    await updateJobNumber(1, "ABC-123");
    expect(fn).toHaveBeenCalledOnce();
    const [url, init] = fn.mock.calls[0];
    expect(url).toMatch(/\/api\/jobs\/1\/number$/);
    expect(init.method).toBe("PATCH");
    expect(init.headers).toEqual({ "Content-Type": "application/json" });
    expect(JSON.parse(init.body as string)).toEqual({ number: "ABC-123" });
  });

  it("sends DELETE without body", async () => {
    const fn = mockSuccess(null);
    await deleteJob(42);
    expect(fn).toHaveBeenCalledOnce();
    const [url, init] = fn.mock.calls[0];
    expect(url).toMatch(/\/api\/jobs\/42$/);
    expect(init.method).toBe("DELETE");
    expect(init.body).toBeUndefined();
  });

  it("sends POST with no Content-Type header when no body", async () => {
    const fn = mockSuccess({ code: 0, message: "ok", data: null });
    await triggerScan();
    expect(fn).toHaveBeenCalledOnce();
    const [, init] = fn.mock.calls[0];
    expect(init.method).toBe("POST");
    expect(init.headers).toBeUndefined();
  });

  it("builds query string for GET endpoints with params", async () => {
    const fn = mockSuccess({ items: [], total: 0, page: 1, page_size: 20 });
    await listJobs({ status: "init", keyword: "test", page: 2, pageSize: 10 });
    expect(fn).toHaveBeenCalledOnce();
    const [url] = fn.mock.calls[0];
    expect(url).toContain("status=init");
    expect(url).toContain("keyword=test");
    expect(url).toContain("page=2");
    expect(url).toContain("page_size=10");
  });

  it("builds query string only for provided params", async () => {
    const fn = mockSuccess({ items: [], total: 0, page: 1, page_size: 20 });
    await listJobs({ status: "init" });
    const [url] = fn.mock.calls[0];
    expect(url).toContain("status=init");
    expect(url).not.toContain("keyword");
    expect(url).not.toContain("page=");
  });

  it("omits query string when no params provided", async () => {
    const fn = mockSuccess({ items: [], total: 0, page: 1, page_size: 20 });
    await listJobs();
    const [url] = fn.mock.calls[0];
    expect(url).toMatch(/\/api\/jobs$/);
  });
});

describe("AbortController signal support", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("passes signal to fetch for GET requests", async () => {
    const fn = mockSuccess([]);
    const controller = new AbortController();
    await getSearcherDebugPlugins(controller.signal);
    const [, init] = fn.mock.calls[0];
    expect(init.signal).toBe(controller.signal);
  });

  it("passes signal to fetch for POST requests", async () => {
    const fn = mockSuccess({ input: "x", input_no_ext: "x", steps: [], final: {} });
    const controller = new AbortController();
    await explainMovieIDCleaner("x", controller.signal);
    const [, init] = fn.mock.calls[0];
    expect(init.signal).toBe(controller.signal);
  });

  it("throws AbortError when signal is already aborted", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new DOMException("The operation was aborted.", "AbortError")));
    const controller = new AbortController();
    controller.abort();
    await expect(listJobs(undefined, controller.signal)).rejects.toThrow("aborted");
  });
});

describe("upload functions with debug logging", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uploadAsset returns data on success", async () => {
    const fn = mockSuccess({ name: "test.jpg", key: "abc123" });
    const file = new File(["data"], "test.jpg", { type: "image/jpeg" });
    const result = await uploadAsset(file);
    expect(result).toEqual({ name: "test.jpg", key: "abc123" });
    const [, init] = fn.mock.calls[0];
    expect(init.method).toBe("POST");
    expect(init.body).toBeInstanceOf(FormData);
    expect(init.headers).toBeUndefined();
  });

  it("uploadAsset throws on HTTP error", async () => {
    mockHTTPErrorHTML(500);
    const file = new File(["data"], "test.jpg", { type: "image/jpeg" });
    await expect(uploadAsset(file)).rejects.toThrow(/HTTP 500/);
  });

  it("uploadAsset propagates non-Error rejections from fetch", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue("offline"));
    const file = new File(["data"], "test.jpg", { type: "image/jpeg" });
    await expect(uploadAsset(file)).rejects.toBe("offline");
  });

  it("uploadAsset throws on business error", async () => {
    mockBusinessError("file too large");
    const file = new File(["data"], "test.jpg", { type: "image/jpeg" });
    await expect(uploadAsset(file)).rejects.toThrow("file too large");
  });

  it("uploadReviewAsset returns data on success", async () => {
    mockSuccess({ name: "cover.jpg", key: "def456" });
    const file = new File(["data"], "cover.jpg", { type: "image/jpeg" });
    const result = await uploadReviewAsset(1, "cover", file);
    expect(result).toEqual({ name: "cover.jpg", key: "def456" });
  });

  it("uploadReviewAsset throws on HTTP error", async () => {
    mockHTTPErrorJSON(403, "forbidden");
    const file = new File(["data"], "cover.jpg", { type: "image/jpeg" });
    await expect(uploadReviewAsset(1, "cover", file)).rejects.toThrow("forbidden");
  });

  it("uploadReviewAsset propagates non-Error rejections from fetch", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(503));
    const file = new File(["data"], "cover.jpg", { type: "image/jpeg" });
    await expect(uploadReviewAsset(2, "poster", file)).rejects.toBe(503);
  });

  it("replaceLibraryAsset returns normalized detail on success", async () => {
    mockSuccess({
      item: { rel_path: "a", name: "a", title: "A", number: "X-1", release_date: "", actors: null, updated_at: 1, has_nfo: true, poster_path: "", cover_path: "", file_count: 1, video_count: 1, variant_count: 1, conflict: false },
      meta: { title: "A", title_translated: "", original_title: "", plot: "", plot_translated: "", number: "X-1", release_date: "", runtime: 0, studio: "", label: "", series: "", director: "", actors: null, genres: null, poster_path: "", cover_path: "", fanart_path: "", thumb_path: "", source: "", scraped_at: "" },
      variants: [],
      primary_variant_key: "",
      files: null,
    });
    const file = new File(["data"], "poster.jpg", { type: "image/jpeg" });
    const result = await replaceLibraryAsset("a", "", "poster", file);
    expect(result.item.actors).toEqual([]);
    expect(result.meta.actors).toEqual([]);
    expect(result.meta.genres).toEqual([]);
    expect(result.files).toEqual([]);
  });

  it("replaceLibraryAsset includes variant in query when variant is non-empty", async () => {
    const fn = mockSuccess({
      item: { rel_path: "a", name: "a", title: "A", number: "X-1", release_date: "", actors: null, updated_at: 1, has_nfo: true, poster_path: "", cover_path: "", file_count: 1, video_count: 1, variant_count: 1, conflict: false },
      meta: { title: "A", title_translated: "", original_title: "", plot: "", plot_translated: "", number: "X-1", release_date: "", runtime: 0, studio: "", label: "", series: "", director: "", actors: null, genres: null, poster_path: "", cover_path: "", fanart_path: "", thumb_path: "", source: "", scraped_at: "" },
      variants: [],
      primary_variant_key: "",
      files: null,
    });
    const file = new File(["data"], "poster.jpg", { type: "image/jpeg" });
    await replaceLibraryAsset("movies/a", "disc1", "poster", file);
    const [url] = fn.mock.calls[0];
    expect(String(url)).toContain("variant=disc1");
  });

  it("replaceLibraryAsset throws on HTTP error", async () => {
    mockHTTPErrorHTML(502);
    const file = new File(["data"], "poster.jpg", { type: "image/jpeg" });
    await expect(replaceLibraryAsset("a", "", "poster", file)).rejects.toThrow(/HTTP 502/);
  });

  it("replaceLibraryAsset propagates non-Error rejections from fetch", async () => {
    const boom = Symbol("boom");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(boom));
    const file = new File(["data"], "poster.jpg", { type: "image/jpeg" });
    await expect(replaceLibraryAsset("b", "v", "cover", file)).rejects.toBe(boom);
  });

  it("upload functions pass signal through", async () => {
    const fn = mockSuccess({ name: "test.jpg", key: "abc" });
    const controller = new AbortController();
    const file = new File(["data"], "test.jpg", { type: "image/jpeg" });
    await uploadAsset(file, controller.signal);
    const [, init] = fn.mock.calls[0];
    expect(init.signal).toBe(controller.signal);
  });
});

describe("upload functions with performance unavailable (Date.now fallback)", () => {
  const origPerf = globalThis.performance;

  afterEach(() => {
    globalThis.performance = origPerf;
    vi.unstubAllGlobals();
  });

  function stubNoPerformance() {
    // @ts-expect-error - intentionally removing performance to exercise Date.now fallback
    delete globalThis.performance;
  }

  it("uploadAsset succeeds with Date.now fallback", async () => {
    const file = new File(["data"], "test.jpg", { type: "image/jpeg" });
    mockSuccess({ name: "test.jpg", key: "abc" });
    stubNoPerformance();
    const result = await uploadAsset(file);
    expect(result).toEqual({ name: "test.jpg", key: "abc" });
  });

  it("uploadAsset catch path uses Date.now fallback", async () => {
    const file = new File(["data"], "test.jpg", { type: "image/jpeg" });
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("fail")));
    stubNoPerformance();
    await expect(uploadAsset(file)).rejects.toThrow("fail");
  });

  it("uploadReviewAsset succeeds with Date.now fallback", async () => {
    const file = new File(["data"], "c.jpg", { type: "image/jpeg" });
    mockSuccess({ name: "c.jpg", key: "k1" });
    stubNoPerformance();
    const result = await uploadReviewAsset(1, "cover", file);
    expect(result).toEqual({ name: "c.jpg", key: "k1" });
  });

  it("uploadReviewAsset catch path uses Date.now fallback", async () => {
    const file = new File(["data"], "c.jpg", { type: "image/jpeg" });
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("net")));
    stubNoPerformance();
    await expect(uploadReviewAsset(1, "cover", file)).rejects.toThrow("net");
  });

  it("replaceLibraryAsset succeeds with Date.now fallback", async () => {
    const file = new File(["data"], "p.jpg", { type: "image/jpeg" });
    mockSuccess({
      item: { rel_path: "a", name: "a", title: "A", number: "X-1", release_date: "", actors: [], updated_at: 1, has_nfo: true, poster_path: "", cover_path: "", file_count: 1, video_count: 1, variant_count: 1, conflict: false },
      meta: { title: "A", title_translated: "", original_title: "", plot: "", plot_translated: "", number: "X-1", release_date: "", runtime: 0, studio: "", label: "", series: "", director: "", actors: [], genres: [], poster_path: "", cover_path: "", fanart_path: "", thumb_path: "", source: "", scraped_at: "" },
      variants: [], primary_variant_key: "", files: [],
    });
    stubNoPerformance();
    const result = await replaceLibraryAsset("a", "", "poster", file);
    expect(result.item.actors).toEqual([]);
  });

  it("replaceLibraryAsset catch path uses Date.now fallback", async () => {
    const file = new File(["data"], "p.jpg", { type: "image/jpeg" });
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("err")));
    stubNoPerformance();
    await expect(replaceLibraryAsset("a", "", "poster", file)).rejects.toThrow("err");
  });
});

describe("getBaseURL SSR path", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns empty string when window is defined (browser)", () => {
    vi.stubGlobal("window", {});
    expect(getAPIBaseURL()).toBe("");
  });

  it("returns YAMDC_API_BASE_URL when window is undefined (SSR)", () => {
    const origEnv = process.env.YAMDC_API_BASE_URL;
    process.env.YAMDC_API_BASE_URL = "http://custom:9090";
    try {
      expect(getAPIBaseURL()).toBe("http://custom:9090");
    } finally {
      if (origEnv === undefined) {
        delete process.env.YAMDC_API_BASE_URL;
      } else {
        process.env.YAMDC_API_BASE_URL = origEnv;
      }
    }
  });

  it("falls back to NEXT_PUBLIC_API_BASE_URL when YAMDC_API_BASE_URL is unset", () => {
    const origPrimary = process.env.YAMDC_API_BASE_URL;
    const origFallback = process.env.NEXT_PUBLIC_API_BASE_URL;
    delete process.env.YAMDC_API_BASE_URL;
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://fallback:8080";
    try {
      expect(getAPIBaseURL()).toBe("http://fallback:8080");
    } finally {
      if (origPrimary !== undefined) process.env.YAMDC_API_BASE_URL = origPrimary;
      if (origFallback === undefined) delete process.env.NEXT_PUBLIC_API_BASE_URL;
      else process.env.NEXT_PUBLIC_API_BASE_URL = origFallback;
    }
  });

  it("falls back to default localhost when both env vars unset", () => {
    const origPrimary = process.env.YAMDC_API_BASE_URL;
    const origFallback = process.env.NEXT_PUBLIC_API_BASE_URL;
    delete process.env.YAMDC_API_BASE_URL;
    delete process.env.NEXT_PUBLIC_API_BASE_URL;
    try {
      expect(getAPIBaseURL()).toBe("http://127.0.0.1:8080");
    } finally {
      if (origPrimary !== undefined) process.env.YAMDC_API_BASE_URL = origPrimary;
      if (origFallback !== undefined) process.env.NEXT_PUBLIC_API_BASE_URL = origFallback;
    }
  });
});
