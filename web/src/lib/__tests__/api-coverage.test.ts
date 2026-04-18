import { afterEach, describe, expect, it, vi } from "vitest";

import type { HandlerDebugRequest, LibraryMeta, PluginEditorCaseSpec, PluginEditorDraft } from "@/lib/api";
import {
  compilePluginDraft,
  cropLibraryPosterFromCover,
  cropPosterFromCover,
  debugHandler,
  debugPluginDraftCase,
  debugPluginDraftRequest,
  debugPluginDraftScrape,
  debugPluginDraftWorkflow,
  debugSearcher,
  deleteLibraryFile,
  deleteLibraryItem,
  deleteMediaLibraryFile,
  getAPIBaseURL,
  getAssetURL,
  getHandlerDebugHandlers,
  getLibraryFileURL,
  getMediaLibraryFileURL,
  getMediaLibraryStatus,
  getReviewJob,
  importPluginDraftYAML,
  importReviewJob,
  listJobLogs,
  listJobs,
  listMediaLibraryItems,
  listMediaLibrarySyncLogs,
  replaceMediaLibraryAsset,
  rerunJob,
  runJob,
  saveReviewJob,
  triggerMediaLibrarySync,
  triggerMoveToMediaLibrary,
  updateLibraryItem,
  updateMediaLibraryItem,
} from "@/lib/api";

function mockSuccess<T>(data: T) {
  const fn = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ code: 0, message: "ok", data }),
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

function mockHTTPError(status: number) {
  const fn = vi.fn().mockResolvedValue({
    ok: false,
    status,
    json: async () => {
      throw new SyntaxError("not json");
    },
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

function getLastFetchInit(): RequestInit | undefined {
  const fn = vi.mocked(fetch);
  return fn.mock.calls.at(-1)?.[1] as RequestInit;
}

function lastFetchCall(): [string, RequestInit] {
  const fn = vi.mocked(fetch);
  return fn.mock.calls[0] as [string, RequestInit];
}

const minItem = {
  rel_path: "a",
  name: "a",
  title: "A",
  number: "X-1",
  release_date: "",
  actors: null as string[] | null,
  created_at: 1,
  updated_at: 1,
  has_nfo: true,
  poster_path: "",
  cover_path: "",
  total_size: 0,
  file_count: 1,
  video_count: 1,
  variant_count: 1,
  conflict: false,
};

const minMeta = {
  title: "A",
  title_translated: "",
  original_title: "",
  plot: "",
  plot_translated: "",
  number: "X-1",
  release_date: "",
  runtime: 0,
  studio: "",
  label: "",
  series: "",
  director: "",
  actors: null as string[] | null,
  genres: null as string[] | null,
  poster_path: "",
  cover_path: "",
  fanart_path: "",
  thumb_path: "",
  source: "",
  scraped_at: "",
};

const libraryDetail = {
  item: minItem,
  meta: minMeta,
  variants: [] as never[],
  primary_variant_key: "",
  files: null as never[] | null,
};

const mediaDetail = {
  item: { id: 1, ...minItem },
  meta: minMeta,
  variants: [] as never[],
  primary_variant_key: "",
  files: null as never[] | null,
};

const minDraft: PluginEditorDraft = {
  version: 1,
  name: "t",
  type: "one-step",
  hosts: ["https://x.com"],
  request: { method: "GET", path: "/s" },
  scrape: {
    format: "html",
    fields: {
      title: { selector: { kind: "xpath", expr: "//t" }, parser: "string" },
    },
  },
};

const minLibMeta: LibraryMeta = {
  title: "A",
  title_translated: "",
  original_title: "",
  plot: "",
  plot_translated: "",
  number: "X-1",
  release_date: "",
  runtime: 0,
  studio: "",
  label: "",
  series: "",
  director: "",
  actors: ["A"],
  genres: ["D"],
  poster_path: "",
  cover_path: "",
  fanart_path: "",
  thumb_path: "",
  source: "",
  scraped_at: "",
};

describe("URL helper functions", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("getAPIBaseURL returns empty string in browser-like environment", () => {
    vi.stubGlobal("window", {});
    expect(getAPIBaseURL()).toBe("");
  });

  it("getAPIBaseURL always returns a string", () => {
    vi.stubGlobal("window", {});
    expect(typeof getAPIBaseURL()).toBe("string");
  });

  it("getAPIBaseURL returns server base URL when window is undefined", () => {
    vi.stubGlobal("window", undefined);
    const base = getAPIBaseURL();
    expect(typeof base).toBe("string");
    expect(base.length).toBeGreaterThan(0);
  });

  it("getAssetURL returns correct URL for a plain key", () => {
    expect(getAssetURL("abc")).toBe("/api/assets/abc");
  });

  it("getAssetURL encodes keys with special characters", () => {
    expect(getAssetURL("a/b")).toBe(`/api/assets/${encodeURIComponent("a/b")}`);
  });

  it("getLibraryFileURL returns correct URL for a simple path", () => {
    expect(getLibraryFileURL("movies/foo.mkv")).toBe(`/api/library/file?path=${encodeURIComponent("movies/foo.mkv")}`);
  });

  it("getLibraryFileURL encodes paths with spaces and special characters", () => {
    expect(getLibraryFileURL("p?q")).toBe(`/api/library/file?path=${encodeURIComponent("p?q")}`);
    expect(getLibraryFileURL("a b")).toBe(`/api/library/file?path=${encodeURIComponent("a b")}`);
  });

  it("getMediaLibraryFileURL returns correct URL for a simple path", () => {
    expect(getMediaLibraryFileURL("lib/x.mkv")).toBe(`/api/media-library/file?path=${encodeURIComponent("lib/x.mkv")}`);
  });

  it("getMediaLibraryFileURL encodes paths with spaces and special characters", () => {
    expect(getMediaLibraryFileURL("x y")).toBe(`/api/media-library/file?path=${encodeURIComponent("x y")}`);
  });
});

describe("media library API", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("listMediaLibraryItems includes every non-default filter in the query string", async () => {
    mockSuccess([]);
    await listMediaLibraryItems({
      keyword: "  foo  ",
      year: "2020",
      size: "large",
      sort: "title",
      order: "asc",
    });
    const [url] = lastFetchCall();
    const qs = new URL(url, "http://localhost").searchParams;
    expect(qs.get("keyword")).toBe("foo");
    expect(qs.get("year")).toBe("2020");
    expect(qs.get("size")).toBe("large");
    expect(qs.get("sort")).toBe("title");
    expect(qs.get("order")).toBe("asc");
  });

  it("listMediaLibraryItems omits default filter values from the query string", async () => {
    mockSuccess([]);
    await listMediaLibraryItems({
      keyword: "bar",
      year: "all",
      size: "all",
      sort: "ingested",
      order: "desc",
    });
    const [url] = lastFetchCall();
    const qs = new URL(url, "http://localhost").searchParams;
    expect(qs.get("keyword")).toBe("bar");
    expect(qs.has("year")).toBe(false);
    expect(qs.has("size")).toBe(false);
    expect(qs.has("sort")).toBe(false);
    expect(qs.has("order")).toBe(false);
  });

  it("listMediaLibraryItems omits query params when params are empty or undefined", async () => {
    mockSuccess([]);
    await listMediaLibraryItems({});
    const [url1] = lastFetchCall();
    const u1 = new URL(url1, "http://local.test");
    expect(u1.pathname).toBe("/api/media-library");
    expect(u1.search).toBe("");
    mockSuccess([]);
    await listMediaLibraryItems(undefined);
    const [url2] = lastFetchCall();
    const u2 = new URL(url2, "http://local.test");
    expect(u2.pathname).toBe("/api/media-library");
    expect(u2.search).toBe("");
  });

  it("listMediaLibraryItems ignores whitespace-only keyword", async () => {
    mockSuccess([]);
    await listMediaLibraryItems({ keyword: "   \t  " });
    const [url] = lastFetchCall();
    const u = new URL(url, "http://local.test");
    expect(u.pathname).toBe("/api/media-library");
    expect(u.search).toBe("");
  });

  it("updateMediaLibraryItem sends PATCH with meta body and returns normalized detail", async () => {
    mockSuccess(mediaDetail);
    const result = await updateMediaLibraryItem(1, minLibMeta);
    expect(vi.mocked(fetch)).toHaveBeenCalledOnce();
    const [, init] = lastFetchCall();
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body as string)).toEqual({ meta: minLibMeta });
    expect(result.meta.actors).toEqual([]);
    expect(result.meta.genres).toEqual([]);
  });

  it("updateMediaLibraryItem throws on HTTP 500", async () => {
    mockHTTPError(500);
    await expect(updateMediaLibraryItem(1, minLibMeta)).rejects.toThrow(/HTTP 500/);
  });

  it("updateMediaLibraryItem normalizes null variant files and null meta arrays", async () => {
    const raw = {
      item: { id: 9, ...minItem },
      meta: minMeta,
      variants: [
        {
          key: "vk",
          label: "L",
          base_name: "b",
          suffix: "",
          is_primary: true,
          video_path: "",
          nfo_path: "",
          poster_path: "",
          cover_path: "",
          meta: { ...minMeta, actors: null, genres: null },
          files: null,
          file_count: 0,
        },
      ],
      primary_variant_key: "vk",
      files: null,
    };
    mockSuccess(raw);
    const result = await updateMediaLibraryItem(9, minLibMeta);
    expect(result.variants[0]?.files).toEqual([]);
    expect(result.variants[0]?.meta.actors).toEqual([]);
    expect(result.variants[0]?.meta.genres).toEqual([]);
    expect(result.files).toEqual([]);
  });

  it("replaceMediaLibraryAsset POSTs FormData and puts variant in the query when set", async () => {
    mockSuccess(mediaDetail);
    const file = new File([""], "p.jpg", { type: "image/jpeg" });
    await replaceMediaLibraryAsset(2, "v1", "poster", file);
    const [url, init] = lastFetchCall();
    expect(url).toContain("variant=v1");
    expect(url).toContain("kind=poster");
    expect(init.method).toBe("POST");
    expect(init.body).toBeInstanceOf(FormData);
  });

  it("replaceMediaLibraryAsset omits variant when variant is empty", async () => {
    mockSuccess(mediaDetail);
    const file = new File([""], "c.jpg");
    await replaceMediaLibraryAsset(3, "", "cover", file);
    const [url] = lastFetchCall();
    expect(url).not.toContain("variant=");
  });

  it("deleteMediaLibraryFile sends DELETE and returns normalized detail", async () => {
    mockSuccess(mediaDetail);
    const result = await deleteMediaLibraryFile(4, "rel/sub.mkv");
    expect(vi.mocked(fetch)).toHaveBeenCalledOnce();
    const [url, init] = lastFetchCall();
    expect(init.method).toBe("DELETE");
    expect(url).toContain("id=4");
    expect(url).toContain(encodeURIComponent("rel/sub.mkv"));
    expect(result.meta.genres).toEqual([]);
  });

  it("deleteMediaLibraryFile throws on HTTP error", async () => {
    mockHTTPError(503);
    await expect(deleteMediaLibraryFile(1, "x")).rejects.toThrow(/HTTP 503/);
  });

  it("getMediaLibraryStatus returns data.data", async () => {
    const status = {
      configured: true,
      sync: {} as never,
      move: {} as never,
    };
    mockSuccess(status);
    await expect(getMediaLibraryStatus()).resolves.toEqual(status);
  });

  it("triggerMediaLibrarySync returns full API response", async () => {
    const envelope = { code: 0, message: "queued", data: { ok: true } };
    const fn = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => envelope,
    });
    vi.stubGlobal("fetch", fn);
    await expect(triggerMediaLibrarySync()).resolves.toEqual(envelope);
  });

  it("triggerMoveToMediaLibrary returns full API response", async () => {
    const envelope = { code: 0, message: "moved", data: null };
    const fn = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => envelope,
    });
    vi.stubGlobal("fetch", fn);
    await expect(triggerMoveToMediaLibrary()).resolves.toEqual(envelope);
  });

  // 正常 case: limit 合法时必须作为 query 参数透传给后端, 避免前端口径和
  // 后端默认值漂移。
  it("listMediaLibrarySyncLogs passes limit as query and returns array", async () => {
    const entries = [
      { id: 1, run_id: "r1", level: "info", rel_path: "", message: "start", created_at: 100 },
      { id: 2, run_id: "r1", level: "warn", rel_path: "a", message: "warn", created_at: 200 },
    ];
    const fn = mockSuccess(entries);
    await expect(listMediaLibrarySyncLogs(50)).resolves.toEqual(entries);
    const [url] = fn.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/api/media-library/sync/logs");
    expect(url).toContain("limit=50");
  });

  // 边缘 case: limit 省略 / 非法 (NaN、负数、0) 时不能把 bad value 拼进
  // URL, 否则后端 strconv.Atoi 会失败回 0 浪费一次解析。
  it("listMediaLibrarySyncLogs omits limit query when invalid or missing", async () => {
    const fn = mockSuccess([]);
    await expect(listMediaLibrarySyncLogs()).resolves.toEqual([]);
    await expect(listMediaLibrarySyncLogs(0)).resolves.toEqual([]);
    await expect(listMediaLibrarySyncLogs(-5)).resolves.toEqual([]);
    await expect(listMediaLibrarySyncLogs(Number.NaN)).resolves.toEqual([]);
    for (const call of fn.mock.calls) {
      const [url] = call as [string, RequestInit];
      expect(url).not.toContain("limit=");
    }
  });

  // 异常 case: 后端返回 data=null (配置未就绪时的空响应) 必须归一化到
  // []; HTTP 500 要抛 HTTP 5xx 让上层弹窗捕获。
  it("listMediaLibrarySyncLogs normalizes nullish data to empty array", async () => {
    mockSuccess(null);
    await expect(listMediaLibrarySyncLogs()).resolves.toEqual([]);
  });

  it("listMediaLibrarySyncLogs throws on HTTP error", async () => {
    mockHTTPError(500);
    await expect(listMediaLibrarySyncLogs()).rejects.toThrow(/HTTP 500/);
  });
});

describe("library API", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("updateLibraryItem sends PATCH with meta body and normalizes null arrays", async () => {
    mockSuccess(libraryDetail);
    const result = await updateLibraryItem("movies/a", minLibMeta);
    expect(vi.mocked(fetch)).toHaveBeenCalledOnce();
    const [, init] = lastFetchCall();
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body as string)).toEqual({ meta: minLibMeta });
    expect(result.meta.actors).toEqual([]);
    expect(result.meta.genres).toEqual([]);
    expect(result.item.actors).toEqual([]);
  });

  it("updateLibraryItem throws on HTTP error", async () => {
    mockHTTPError(500);
    await expect(updateLibraryItem("movies/a", minLibMeta)).rejects.toThrow(/HTTP 500/);
  });

  it("deleteLibraryItem sends DELETE and returns full response", async () => {
    const envelope = { code: 0, message: "ok", data: null };
    const fn = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => envelope,
    });
    vi.stubGlobal("fetch", fn);
    await expect(deleteLibraryItem("movies/b")).resolves.toEqual(envelope);
    const [, init] = lastFetchCall();
    expect(init.method).toBe("DELETE");
  });

  it("deleteLibraryItem throws on HTTP error", async () => {
    mockHTTPError(404);
    await expect(deleteLibraryItem("movies/b")).rejects.toThrow(/HTTP 404/);
  });

  it("cropLibraryPosterFromCover POSTs rect body and includes variant when non-empty", async () => {
    mockSuccess(libraryDetail);
    const rect = { x: 1, y: 2, width: 3, height: 4 };
    const result = await cropLibraryPosterFromCover("movies/c", "vk", rect);
    const [url, init] = lastFetchCall();
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual(rect);
    expect(url).toContain("variant=vk");
    expect(result.meta.actors).toEqual([]);
    expect(result.meta.genres).toEqual([]);
  });

  it("cropLibraryPosterFromCover omits variant when variant is empty", async () => {
    mockSuccess(libraryDetail);
    const rect = { x: 0, y: 0, width: 10, height: 10 };
    await cropLibraryPosterFromCover("movies/d", "", rect);
    const [url] = lastFetchCall();
    expect(url).not.toContain("variant=");
  });

  it("deleteLibraryFile sends DELETE and returns normalized detail", async () => {
    mockSuccess(libraryDetail);
    const result = await deleteLibraryFile("movies/e");
    expect(getLastFetchInit()?.method).toBe("DELETE");
    expect(result.item.actors).toEqual([]);
    expect(result.files).toEqual([]);
  });
});

describe("job API", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("listJobs adds all=true when all is true", async () => {
    mockSuccess({ items: [], total: 0, page: 1, page_size: 20 });
    await listJobs({ all: true });
    const [url] = lastFetchCall();
    expect(new URL(url, "http://localhost").searchParams.get("all")).toBe("true");
  });

  it("runJob POSTs to /api/jobs/:id/run", async () => {
    mockSuccess({});
    await expect(runJob(7)).resolves.toEqual({ code: 0, message: "ok", data: {} });
    const [url, init] = lastFetchCall();
    expect(url).toMatch(/\/api\/jobs\/7\/run$/);
    expect(init.method).toBe("POST");
  });

  it("runJob throws on HTTP error", async () => {
    mockHTTPError(500);
    await expect(runJob(7)).rejects.toThrow(/HTTP 500/);
  });

  it("rerunJob POSTs to /api/jobs/:id/rerun", async () => {
    const envelope = { code: 0, message: "ok", data: null };
    const fn = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => envelope,
    });
    vi.stubGlobal("fetch", fn);
    await expect(rerunJob(8)).resolves.toEqual(envelope);
    const [url] = lastFetchCall();
    expect(url).toMatch(/\/api\/jobs\/8\/rerun$/);
  });

  it("listJobLogs GETs with cache no-store", async () => {
    mockSuccess([]);
    await listJobLogs(9);
    const [url, init] = lastFetchCall();
    expect(url).toContain("/api/jobs/9/logs");
    expect(init.cache).toBe("no-store");
  });

  it("listJobLogs returns an empty array when the server sends no logs", async () => {
    mockSuccess([]);
    await expect(listJobLogs(10)).resolves.toEqual([]);
  });
});

describe("review API", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("getReviewJob returns null when there is no review payload", async () => {
    mockSuccess(null);
    await expect(getReviewJob(11)).resolves.toBeNull();
  });

  it("getReviewJob returns scrape data when present", async () => {
    const row = {
      id: 1,
      job_id: 11,
      source: "s",
      version: 1,
      raw_data: "",
      review_data: "",
      final_data: "",
      status: "x",
      created_at: 1,
      updated_at: 2,
    };
    mockSuccess(row);
    await expect(getReviewJob(11)).resolves.toEqual(row);
  });

  it("saveReviewJob PUTs review_data", async () => {
    const envelope = { code: 0, message: "saved", data: {} };
    const fn = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => envelope,
    });
    vi.stubGlobal("fetch", fn);
    await expect(saveReviewJob(12, '{"x":1}')).resolves.toEqual(envelope);
    const [, init] = lastFetchCall();
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toEqual({ review_data: '{"x":1}' });
  });

  it("importReviewJob POSTs", async () => {
    const envelope = { code: 0, message: "imported", data: {} };
    const fn = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => envelope,
    });
    vi.stubGlobal("fetch", fn);
    await expect(importReviewJob(13)).resolves.toEqual(envelope);
    const [, init] = lastFetchCall();
    expect(init.method).toBe("POST");
  });

  it("cropPosterFromCover POSTs rect and returns data.data", async () => {
    const ref = { name: "p.jpg", key: "k1" };
    mockSuccess(ref);
    const rect = { x: 0, y: 0, width: 100, height: 200 };
    await expect(cropPosterFromCover(14, rect)).resolves.toEqual(ref);
    const [, init] = lastFetchCall();
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual(rect);
  });

  it("cropPosterFromCover throws on HTTP error", async () => {
    mockHTTPError(502);
    await expect(cropPosterFromCover(14, { x: 0, y: 0, width: 1, height: 1 })).rejects.toThrow(/HTTP 502/);
  });
});

describe("debug API", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("debugSearcher POSTs input, plugins, and use_cleaner", async () => {
    const dbg = {
      input: "",
      number_id: "",
      requested_input: "",
      used_plugins: [],
      matched_plugin: "",
      found: false,
      category: "",
      uncensor: false,
      plugin_results: [],
      available_tools: {} as never,
    };
    mockSuccess(dbg);
    await debugSearcher("ABC-123", "p1", true);
    const [, init] = lastFetchCall();
    expect(JSON.parse(init.body as string)).toEqual({
      input: "ABC-123",
      plugins: ["p1"],
      use_cleaner: true,
    });
  });

  it("debugSearcher sends an empty plugins array when plugin is empty", async () => {
    const dbg = {
      input: "",
      number_id: "",
      requested_input: "",
      used_plugins: [],
      matched_plugin: "",
      found: false,
      category: "",
      uncensor: false,
      plugin_results: [],
      available_tools: {} as never,
    };
    mockSuccess(dbg);
    await debugSearcher("ABC-123", "", true);
    const [, init] = lastFetchCall();
    expect(JSON.parse(init.body as string)).toEqual({
      input: "ABC-123",
      plugins: [],
      use_cleaner: true,
    });
  });

  it("getHandlerDebugHandlers GETs handlers", async () => {
    mockSuccess([{ id: "h1", name: "H1" }]);
    await expect(getHandlerDebugHandlers()).resolves.toEqual([{ id: "h1", name: "H1" }]);
  });

  it("debugHandler POSTs payload", async () => {
    const payload: HandlerDebugRequest = {
      handler_id: "trim",
      meta: { title: "T" },
    };
    const resultBody = {
      mode: "single" as const,
      handler_id: "trim",
      handler_name: "Trim",
      number_id: "",
      category: "",
      uncensor: false,
      before_meta: {},
      after_meta: {},
      error: "",
      steps: [],
    };
    mockSuccess(resultBody);
    await expect(debugHandler(payload)).resolves.toEqual(resultBody);
    const [, init] = lastFetchCall();
    expect(JSON.parse(init.body as string)).toEqual(payload);
  });

  it("debugHandler throws on HTTP error", async () => {
    mockHTTPError(500);
    const payload: HandlerDebugRequest = { handler_id: "x", meta: {} };
    await expect(debugHandler(payload)).rejects.toThrow(/HTTP 500/);
  });

  it("compilePluginDraft POSTs draft and returns envelope data", async () => {
    const inner = {
      yaml: "name: test",
      summary: { has_request: true, has_multi_request: false, has_workflow: false, scrape_format: "html", field_count: 1 },
    };
    mockSuccess({ ok: true, warnings: ["w"], data: inner });
    await expect(compilePluginDraft(minDraft)).resolves.toEqual({ ok: true, warnings: ["w"], data: inner });
  });

  it("importPluginDraftYAML POSTs yaml and returns envelope data", async () => {
    const inner = { draft: minDraft };
    mockSuccess({ ok: true, warnings: [], data: inner });
    await expect(importPluginDraftYAML("name: x")).resolves.toEqual({ ok: true, warnings: [], data: inner });
  });

  it("importPluginDraftYAML sends an empty yaml string in the JSON body", async () => {
    const inner = { draft: minDraft };
    mockSuccess({ ok: true, warnings: [], data: inner });
    await importPluginDraftYAML("");
    const [, init] = lastFetchCall();
    expect(JSON.parse(init.body as string)).toEqual({ yaml: "" });
  });

  it("debugPluginDraftRequest POSTs draft and number", async () => {
    const inner = { request: { method: "GET", url: "u", headers: {}, body: "" } };
    mockSuccess({ ok: true, warnings: [], data: inner });
    await expect(debugPluginDraftRequest(minDraft, "X-1")).resolves.toEqual({ ok: true, warnings: [], data: inner });
    const [, init] = lastFetchCall();
    expect(JSON.parse(init.body as string)).toEqual({ draft: minDraft, number: "X-1" });
  });

  it("debugPluginDraftWorkflow POSTs draft and number", async () => {
    const inner = { steps: [] };
    mockSuccess({ ok: true, warnings: [], data: inner });
    await expect(debugPluginDraftWorkflow(minDraft, "N2")).resolves.toEqual({ ok: true, warnings: [], data: inner });
    const [, init] = lastFetchCall();
    expect(JSON.parse(init.body as string)).toEqual({ draft: minDraft, number: "N2" });
  });

  it("debugPluginDraftScrape POSTs draft and number", async () => {
    const inner = { fields: {} };
    mockSuccess({ ok: true, warnings: [], data: inner });
    await expect(debugPluginDraftScrape(minDraft, "N3")).resolves.toEqual({ ok: true, warnings: [], data: inner });
    const [, init] = lastFetchCall();
    expect(JSON.parse(init.body as string)).toEqual({ draft: minDraft, number: "N3" });
  });

  it("debugPluginDraftCase POSTs draft and case spec", async () => {
    const caseSpec: PluginEditorCaseSpec = {
      name: "c1",
      input: "IN",
      output: { title: "T" },
    };
    const inner = { result: { pass: true, errmsg: "", meta: null } };
    mockSuccess({ ok: true, warnings: [], data: inner });
    await expect(debugPluginDraftCase(minDraft, caseSpec)).resolves.toEqual({ ok: true, warnings: [], data: inner });
    const [, init] = lastFetchCall();
    expect(JSON.parse(init.body as string)).toEqual({ draft: minDraft, case: caseSpec });
  });
});
