import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type {
  LibraryDetail,
  LibraryListItem,
  LibraryMeta,
  LibraryVariant,
  MediaLibraryStatus,
  TaskState,
} from "@/lib/api";

import {
  cloneMeta,
  getCardImage,
  getInitialCopyMode,
  getInitialMessage,
  getInitialSelectedPath,
  getInitialVariantKey,
  getMoveButtonLabel,
  getRefreshButtonLabel,
  getUploadMessage,
  getVariantCoverPath,
  getVariantPosterPath,
  handleMoveToMediaLibraryError,
  hasTranslatedCopy,
  itemActors,
  markMoveRunning,
  markMoveStarting,
  normalizeMeta,
  pickNextCopyMode,
  pickNextVariantKey,
  pickVariant,
  resolveSelectedCover,
  resolveSelectedPoster,
  serializeMeta,
  taskPercent,
  toErrorMessage,
  toMoveToMediaLibraryMessage,
} from "../utils";

// library-shell/utils.ts: 纯工具集, 在 §2.2 B-2 从 library-shell.tsx 拆出.
// 本套单测把每个 export 的正常 / 异常 / 边缘 case 全部冻结.
// 唯一有副作用的 handleMoveToMediaLibraryError 用 vi.mock 打掉 API.

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getMediaLibraryStatus: vi.fn(),
  };
});

const apiModule = await import("@/lib/api");
const mockedGetMediaLibraryStatus = vi.mocked(apiModule.getMediaLibraryStatus);

function baseMeta(overrides: Partial<LibraryMeta> = {}): LibraryMeta {
  return {
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
    ...overrides,
  };
}

function baseItem(overrides: Partial<LibraryListItem> = {}): LibraryListItem {
  return {
    rel_path: "",
    name: "",
    title: "",
    number: "",
    release_date: "",
    actors: [],
    created_at: 0,
    updated_at: 0,
    has_nfo: false,
    poster_path: "",
    cover_path: "",
    total_size: 0,
    file_count: 0,
    video_count: 0,
    variant_count: 1,
    conflict: false,
    ...overrides,
  };
}

function baseVariant(overrides: Partial<LibraryVariant> = {}): LibraryVariant {
  return {
    key: "v1",
    label: "V1",
    base_name: "",
    suffix: "",
    is_primary: true,
    video_path: "",
    nfo_path: "",
    poster_path: "",
    cover_path: "",
    meta: baseMeta(),
    files: [],
    file_count: 0,
    ...overrides,
  };
}

function baseDetail(overrides: Partial<LibraryDetail> = {}): LibraryDetail {
  return {
    item: baseItem(),
    meta: baseMeta(),
    variants: [baseVariant()],
    primary_variant_key: "v1",
    files: [],
    ...overrides,
  };
}

function baseTaskState(overrides: Partial<TaskState> = {}): TaskState {
  return {
    task_key: "",
    status: "idle",
    total: 0,
    processed: 0,
    success_count: 0,
    conflict_count: 0,
    error_count: 0,
    current: "",
    message: "",
    started_at: 0,
    finished_at: 0,
    updated_at: 0,
    ...overrides,
  };
}

function baseStatus(overrides: Partial<MediaLibraryStatus> = {}): MediaLibraryStatus {
  return {
    configured: true,
    sync: baseTaskState(),
    move: baseTaskState(),
    ...overrides,
  };
}

describe("cloneMeta", () => {
  it("creates a fully-populated empty meta when input is null", () => {
    const result = cloneMeta(null);
    expect(result.actors).toEqual([]);
    expect(result.genres).toEqual([]);
    expect(result.title).toBe("");
    expect(result.runtime).toBe(0);
  });

  it("treats undefined like null", () => {
    expect(cloneMeta(undefined)).toEqual(cloneMeta(null));
  });

  it("deep-copies actors/genres arrays so mutations don't leak", () => {
    const src = baseMeta({ actors: ["a"], genres: ["g"] });
    const clone = cloneMeta(src);
    clone.actors.push("b");
    clone.genres.push("h");
    expect(src.actors).toEqual(["a"]);
    expect(src.genres).toEqual(["g"]);
  });

  it("preserves all scalar fields", () => {
    const src = baseMeta({ title: "T", runtime: 42, source: "demo" });
    expect(cloneMeta(src)).toMatchObject({ title: "T", runtime: 42, source: "demo" });
  });
});

describe("pickVariant", () => {
  it("returns null when detail is null", () => {
    expect(pickVariant(null, "v1")).toBeNull();
  });

  it("returns matching variant by key", () => {
    const v2 = baseVariant({ key: "v2", label: "V2" });
    const detail = baseDetail({ variants: [baseVariant(), v2] });
    expect(pickVariant(detail, "v2")).toBe(v2);
  });

  it("falls back to first variant when key not found", () => {
    const first = baseVariant({ key: "v1" });
    const detail = baseDetail({ variants: [first, baseVariant({ key: "v2" })] });
    expect(pickVariant(detail, "missing")).toBe(first);
  });

  it("returns null when detail has empty variants array", () => {
    expect(pickVariant(baseDetail({ variants: [] }), "v1")).toBeNull();
  });
});

describe("serializeMeta / normalizeMeta", () => {
  it("serializeMeta trims and drops empty actors/genres", () => {
    const meta = baseMeta({ actors: ["  alice  ", "", "bob"], genres: [" drama "] });
    const json = serializeMeta(meta);
    const parsed = JSON.parse(json) as LibraryMeta;
    expect(parsed.actors).toEqual(["alice", "bob"]);
    expect(parsed.genres).toEqual(["drama"]);
  });

  it("normalizeMeta produces the same normalised list structure", () => {
    const meta = baseMeta({ actors: [" x ", "  "], genres: ["", "y"] });
    expect(normalizeMeta(meta)).toMatchObject({ actors: ["x"], genres: ["y"] });
  });

  it("normalizeMeta preserves scalar fields", () => {
    const meta = baseMeta({ title: "t", plot: "p" });
    expect(normalizeMeta(meta)).toMatchObject({ title: "t", plot: "p" });
  });
});

describe("getCardImage / itemActors", () => {
  it("prefers poster over cover", () => {
    expect(getCardImage(baseItem({ poster_path: "p", cover_path: "c" }))).toBe("p");
  });

  it("falls back to cover when poster empty", () => {
    expect(getCardImage(baseItem({ poster_path: "", cover_path: "c" }))).toBe("c");
  });

  it("returns '' when both empty", () => {
    expect(getCardImage(baseItem())).toBe("");
  });

  it("itemActors returns array or empty for non-array", () => {
    expect(itemActors(baseItem({ actors: ["a"] }))).toEqual(["a"]);
    const bad = baseItem();
    (bad as unknown as { actors: unknown }).actors = null;
    expect(itemActors(bad)).toEqual([]);
  });
});

describe("getVariantPosterPath / getVariantCoverPath", () => {
  it("poster: variant.poster_path wins", () => {
    const variant = baseVariant({ poster_path: "a", meta: baseMeta({ poster_path: "b" }) });
    const detail = baseDetail({ variants: [variant], meta: baseMeta({ poster_path: "c" }) });
    expect(getVariantPosterPath(detail, "v1")).toBe("a");
  });

  it("poster: falls through variant.meta -> detail.meta -> detail.item", () => {
    const variant = baseVariant({ poster_path: "", meta: baseMeta({ poster_path: "b" }) });
    expect(
      getVariantPosterPath(baseDetail({ variants: [variant] }), "v1"),
    ).toBe("b");

    const variant2 = baseVariant({ poster_path: "", meta: baseMeta() });
    expect(
      getVariantPosterPath(
        baseDetail({ variants: [variant2], meta: baseMeta({ poster_path: "c" }) }),
        "v1",
      ),
    ).toBe("c");

    const variant3 = baseVariant({ poster_path: "", meta: baseMeta() });
    expect(
      getVariantPosterPath(
        baseDetail({
          variants: [variant3],
          meta: baseMeta(),
          item: baseItem({ poster_path: "d" }),
        }),
        "v1",
      ),
    ).toBe("d");
  });

  it("poster: returns '' when detail null", () => {
    expect(getVariantPosterPath(null, "v1")).toBe("");
  });

  it("cover: full priority chain", () => {
    const vFull = baseVariant({ cover_path: "a" });
    expect(getVariantCoverPath(baseDetail({ variants: [vFull] }), "v1")).toBe("a");

    const vMetaCover = baseVariant({ cover_path: "", meta: baseMeta({ cover_path: "b" }) });
    expect(getVariantCoverPath(baseDetail({ variants: [vMetaCover] }), "v1")).toBe("b");

    const vFanart = baseVariant({ cover_path: "", meta: baseMeta({ fanart_path: "c" }) });
    expect(getVariantCoverPath(baseDetail({ variants: [vFanart] }), "v1")).toBe("c");

    const vThumb = baseVariant({ cover_path: "", meta: baseMeta({ thumb_path: "d" }) });
    expect(getVariantCoverPath(baseDetail({ variants: [vThumb] }), "v1")).toBe("d");

    const vEmpty = baseVariant({ cover_path: "", meta: baseMeta() });
    expect(
      getVariantCoverPath(
        baseDetail({ variants: [vEmpty], meta: baseMeta({ cover_path: "e" }) }),
        "v1",
      ),
    ).toBe("e");

    expect(
      getVariantCoverPath(
        baseDetail({ variants: [vEmpty], meta: baseMeta({ fanart_path: "f" }) }),
        "v1",
      ),
    ).toBe("f");

    expect(
      getVariantCoverPath(
        baseDetail({ variants: [vEmpty], meta: baseMeta({ thumb_path: "g" }) }),
        "v1",
      ),
    ).toBe("g");

    expect(
      getVariantCoverPath(
        baseDetail({
          variants: [vEmpty],
          meta: baseMeta(),
          item: baseItem({ cover_path: "h" }),
        }),
        "v1",
      ),
    ).toBe("h");
  });

  it("cover: returns '' when everything empty", () => {
    expect(getVariantCoverPath(baseDetail({ variants: [baseVariant()] }), "v1")).toBe("");
  });
});

describe("hasTranslatedCopy", () => {
  it("null returns false", () => {
    expect(hasTranslatedCopy(null)).toBe(false);
  });

  it("both empty returns false", () => {
    expect(hasTranslatedCopy(baseMeta())).toBe(false);
  });

  it("whitespace-only returns false", () => {
    expect(hasTranslatedCopy(baseMeta({ title_translated: "   ", plot_translated: "\t\n" }))).toBe(false);
  });

  it("title_translated populated returns true", () => {
    expect(hasTranslatedCopy(baseMeta({ title_translated: "t" }))).toBe(true);
  });

  it("plot_translated populated returns true", () => {
    expect(hasTranslatedCopy(baseMeta({ plot_translated: "p" }))).toBe(true);
  });
});

describe("taskPercent", () => {
  it("null or total=0 returns 0", () => {
    expect(taskPercent(null)).toBe(0);
    expect(taskPercent(baseTaskState({ total: 0, processed: 0 }))).toBe(0);
    expect(taskPercent(baseTaskState({ total: -1, processed: 5 }))).toBe(0);
  });

  it("mid-progress rounds", () => {
    expect(taskPercent(baseTaskState({ total: 3, processed: 1 }))).toBe(33);
  });

  it("clamps to [0, 100]", () => {
    expect(taskPercent(baseTaskState({ total: 10, processed: 15 }))).toBe(100);
    expect(taskPercent(baseTaskState({ total: 10, processed: -3 }))).toBe(0);
  });

  it("returns 100 when complete", () => {
    expect(taskPercent(baseTaskState({ total: 5, processed: 5 }))).toBe(100);
  });
});

describe("toMoveToMediaLibraryMessage", () => {
  it("non-Error falls back to default", () => {
    expect(toMoveToMediaLibraryMessage("string-error")).toBe("启动移动到媒体库失败");
    expect(toMoveToMediaLibraryMessage(null)).toBe("启动移动到媒体库失败");
  });

  it("maps known server error strings", () => {
    expect(toMoveToMediaLibraryMessage(new Error("move to media library is already running"))).toBe("媒体库移动任务已在进行中");
    expect(toMoveToMediaLibraryMessage(new Error("media library sync is running"))).toBe("媒体库同步进行中，暂时无法移动");
    expect(toMoveToMediaLibraryMessage(new Error("library dir is not configured"))).toBe("未配置媒体库目录");
    expect(toMoveToMediaLibraryMessage(new Error("save dir is not configured"))).toBe("未配置保存目录");
  });

  it("returns the raw message for unknown errors (after trim check)", () => {
    expect(toMoveToMediaLibraryMessage(new Error("boom"))).toBe("boom");
  });
});

describe("getRefreshButtonLabel / getMoveButtonLabel", () => {
  it("refresh: running > flash > idle label", () => {
    expect(getRefreshButtonLabel(true, false)).toBe("扫描中...");
    expect(getRefreshButtonLabel(false, true)).toBe("扫描完成");
    expect(getRefreshButtonLabel(false, false)).toBe("重新扫描库");
    // running takes precedence over flash
    expect(getRefreshButtonLabel(true, true)).toBe("扫描中...");
  });

  it("move: busy+running with state shows processed/total", () => {
    const state = baseTaskState({ processed: 3, total: 10 });
    expect(getMoveButtonLabel(true, true, state, false)).toBe("移动中 3/10");
  });

  it("move: busy without running state shows generic 移动中", () => {
    expect(getMoveButtonLabel(true, false, null, false)).toBe("移动中...");
  });

  it("move: not busy, flash active", () => {
    expect(getMoveButtonLabel(false, false, null, true)).toBe("移动完成");
  });

  it("move: pure idle returns default label", () => {
    expect(getMoveButtonLabel(false, false, null, false)).toBe("移动到媒体库");
  });

  it("move: busy+running but state null falls back to generic", () => {
    expect(getMoveButtonLabel(true, true, null, false)).toBe("移动中...");
  });
});

describe("markMoveStarting / markMoveRunning", () => {
  it("null passes through", () => {
    expect(markMoveStarting(null)).toBeNull();
    expect(markMoveRunning(null)).toBeNull();
  });

  it("markMoveStarting sets status + preserves started_at if already set", () => {
    const status = baseStatus({ move: baseTaskState({ status: "idle", started_at: 42 }) });
    const next = markMoveStarting(status);
    expect(next?.move.status).toBe("starting");
    expect(next?.move.message).toBe("移动到媒体库中");
    expect(next?.move.started_at).toBe(42);
    expect(next?.move.updated_at).toBeGreaterThan(0);
  });

  it("markMoveStarting assigns new started_at when it was 0", () => {
    const now = 1_700_000_000_000;
    vi.spyOn(Date, "now").mockReturnValue(now);
    const status = baseStatus({ move: baseTaskState({ status: "idle", started_at: 0 }) });
    const next = markMoveStarting(status);
    expect(next?.move.started_at).toBe(now);
    vi.restoreAllMocks();
  });

  it("markMoveRunning sets running status and updates updated_at", () => {
    const status = baseStatus({ move: baseTaskState({ status: "starting" }) });
    const next = markMoveRunning(status);
    expect(next?.move.status).toBe("running");
    expect(next?.move.message).toBe("移动到媒体库中");
  });
});

describe("handleMoveToMediaLibraryError", () => {
  beforeEach(() => {
    mockedGetMediaLibraryStatus.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("branch: 已在进行中 -> marks running without fetching status", () => {
    const setMessage = vi.fn();
    const setVisible = vi.fn();
    const setMedia = vi.fn();
    const obs = { current: false };
    handleMoveToMediaLibraryError(
      new Error("move to media library is already running"),
      setMessage,
      setVisible,
      setMedia,
      obs,
    );
    expect(setMessage).toHaveBeenCalledWith("媒体库移动任务已在进行中");
    expect(setVisible).toHaveBeenCalledWith(true);
    expect(setMedia).toHaveBeenCalledWith(markMoveRunning);
    expect(obs.current).toBe(true);
    expect(mockedGetMediaLibraryStatus).not.toHaveBeenCalled();
  });

  it("branch: other errors -> hide progress and refresh status", async () => {
    mockedGetMediaLibraryStatus.mockResolvedValueOnce(baseStatus());
    const setMessage = vi.fn();
    const setVisible = vi.fn();
    const setMedia = vi.fn();
    const obs = { current: false };
    handleMoveToMediaLibraryError(
      new Error("save dir is not configured"),
      setMessage,
      setVisible,
      setMedia,
      obs,
    );
    expect(setMessage).toHaveBeenCalledWith("未配置保存目录");
    expect(setVisible).toHaveBeenCalledWith(false);
    expect(obs.current).toBe(false);
    await Promise.resolve();
    await Promise.resolve();
    expect(mockedGetMediaLibraryStatus).toHaveBeenCalled();
  });

  it("branch: getMediaLibraryStatus rejection is swallowed", async () => {
    mockedGetMediaLibraryStatus.mockRejectedValueOnce(new Error("network"));
    const setMessage = vi.fn();
    const setVisible = vi.fn();
    const setMedia = vi.fn();
    const obs = { current: false };
    expect(() => handleMoveToMediaLibraryError(
      new Error("boom"),
      setMessage,
      setVisible,
      setMedia,
      obs,
    )).not.toThrow();
    await Promise.resolve();
    await Promise.resolve();
    expect(setMedia).not.toHaveBeenCalled();
  });
});

describe("resolveSelectedPoster / resolveSelectedCover", () => {
  it("poster: variant > meta > detail.item", () => {
    const variant = baseVariant({ poster_path: "a" });
    expect(resolveSelectedPoster(variant, baseMeta(), null)).toBe("a");

    const variant2 = baseVariant({ poster_path: "", meta: baseMeta({ poster_path: "b" }) });
    expect(resolveSelectedPoster(variant2, baseMeta(), null)).toBe("b");

    expect(resolveSelectedPoster(null, baseMeta({ poster_path: "c" }), null)).toBe("c");

    expect(
      resolveSelectedPoster(null, baseMeta(), baseDetail({ item: baseItem({ poster_path: "d" }) })),
    ).toBe("d");

    expect(resolveSelectedPoster(null, baseMeta(), null)).toBe("");
  });

  it("cover: full fallback chain", () => {
    expect(resolveSelectedCover(baseVariant({ cover_path: "a" }), baseMeta(), null)).toBe("a");
    expect(
      resolveSelectedCover(baseVariant({ cover_path: "", meta: baseMeta({ cover_path: "b" }) }), baseMeta(), null),
    ).toBe("b");
    expect(
      resolveSelectedCover(baseVariant({ cover_path: "", meta: baseMeta({ fanart_path: "c" }) }), baseMeta(), null),
    ).toBe("c");
    expect(
      resolveSelectedCover(baseVariant({ cover_path: "", meta: baseMeta({ thumb_path: "d" }) }), baseMeta(), null),
    ).toBe("d");
    expect(resolveSelectedCover(null, baseMeta({ cover_path: "e" }), null)).toBe("e");
    expect(resolveSelectedCover(null, baseMeta({ fanart_path: "f" }), null)).toBe("f");
    expect(
      resolveSelectedCover(null, baseMeta(), baseDetail({ item: baseItem({ cover_path: "g" }) })),
    ).toBe("g");
    expect(resolveSelectedCover(null, baseMeta(), null)).toBe("");
  });
});

describe("getUploadMessage", () => {
  it("returns kind+phase combinations", () => {
    expect(getUploadMessage("poster", "start")).toContain("替换当前实例海报");
    expect(getUploadMessage("poster", "done")).toContain("当前实例海报已更新");
    expect(getUploadMessage("cover", "start")).toContain("替换当前实例封面");
    expect(getUploadMessage("cover", "done")).toContain("当前实例封面已更新");
    expect(getUploadMessage("fanart", "start")).toContain("上传 extrafanart");
    expect(getUploadMessage("fanart", "done")).toContain("Extrafanart 已上传");
  });
});

describe("pickNextVariantKey", () => {
  it("keeps current when it exists", () => {
    const detail = baseDetail({ variants: [baseVariant({ key: "v1" }), baseVariant({ key: "v2" })] });
    expect(pickNextVariantKey("v1", detail)).toBe("v1");
  });

  it("falls back to primary_variant_key when current missing", () => {
    const detail = baseDetail({
      variants: [baseVariant({ key: "va" })],
      primary_variant_key: "va",
    });
    expect(pickNextVariantKey("missing", detail)).toBe("va");
  });

  it("falls back to first variant key when primary missing", () => {
    const detail = baseDetail({
      variants: [baseVariant({ key: "va" })],
      primary_variant_key: "",
    });
    expect(pickNextVariantKey("", detail)).toBe("va");
  });

  it("returns '' when no variants", () => {
    const detail = baseDetail({ variants: [], primary_variant_key: "" });
    expect(pickNextVariantKey("", detail)).toBe("");
  });
});

describe("pickNextCopyMode", () => {
  it("current translated + has translated -> keep translated", () => {
    expect(pickNextCopyMode("translated", baseMeta({ title_translated: "t" }))).toBe("translated");
  });

  it("current translated + no translated -> switch to original", () => {
    expect(pickNextCopyMode("translated", baseMeta())).toBe("original");
  });

  it("current original always stays original", () => {
    expect(pickNextCopyMode("original", baseMeta({ title_translated: "t" }))).toBe("original");
    expect(pickNextCopyMode("original", baseMeta())).toBe("original");
  });
});

describe("toErrorMessage", () => {
  it("returns message when error is Error", () => {
    expect(toErrorMessage(new Error("boom"), "fallback")).toBe("boom");
  });

  it("returns fallback for non-Error", () => {
    expect(toErrorMessage("string", "fallback")).toBe("fallback");
    expect(toErrorMessage(null, "fallback")).toBe("fallback");
    expect(toErrorMessage(undefined, "fallback")).toBe("fallback");
  });
});

describe("getInitial* helpers", () => {
  it("getInitialSelectedPath: detail > first item > ''", () => {
    expect(getInitialSelectedPath(baseDetail({ item: baseItem({ rel_path: "d" }) }), [])).toBe("d");
    expect(getInitialSelectedPath(null, [baseItem({ rel_path: "a" }), baseItem({ rel_path: "b" })])).toBe("a");
    expect(getInitialSelectedPath(null, [])).toBe("");
  });

  it("getInitialVariantKey uses ?? chain so empty string primary is NOT treated as missing", () => {
    // 注意: 实现用的是 ?? (nullish coalescing), 所以 primary_variant_key="" 会
    // 直接返回 "" 而不会 fallback 到 variants[0].key. 这里把这个行为冻住 —
    // 如果将来想让空串也 fallback, 应该同步改实现和这条断言.
    expect(getInitialVariantKey(baseDetail({ primary_variant_key: "p" }))).toBe("p");
    expect(
      getInitialVariantKey(
        baseDetail({ primary_variant_key: "", variants: [baseVariant({ key: "v9" })] }),
      ),
    ).toBe("");
    expect(getInitialVariantKey(null)).toBe("");
  });

  it("getInitialVariantKey falls back when detail is null", () => {
    expect(getInitialVariantKey(null)).toBe("");
  });

  it("getInitialCopyMode: translated when available", () => {
    expect(
      getInitialCopyMode(baseDetail({ meta: baseMeta({ title_translated: "t" }) })),
    ).toBe("translated");
    expect(getInitialCopyMode(baseDetail({ meta: baseMeta() }))).toBe("original");
    expect(getInitialCopyMode(null)).toBe("original");
  });

  it("getInitialMessage: empty items returns onboarding copy", () => {
    expect(getInitialMessage([])).toContain("当前 savedir");
    expect(getInitialMessage([baseItem()])).toBe("");
  });
});
