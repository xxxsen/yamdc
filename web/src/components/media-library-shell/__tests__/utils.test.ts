import { describe, expect, it } from "vitest";

import type { MediaLibraryItem } from "@/lib/api";

import {
  extractYearOptions,
  formatSyncLogTime,
  getReleaseYear,
  mergeYearOptions,
  toMediaLibrarySyncMessage,
} from "../utils";

// media-library-shell/utils: 年份派生 / 合并 / 错误文案 / 时间格式化.

function makeItem(overrides: Partial<MediaLibraryItem> = {}): MediaLibraryItem {
  return {
    id: 0,
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

describe("getReleaseYear", () => {
  it("extracts the first 4-digit run", () => {
    expect(getReleaseYear("2022-01-31")).toBe("2022");
    expect(getReleaseYear("released in 1999, re-released 2020")).toBe("1999");
  });

  it("returns '' when no 4-digit run present", () => {
    expect(getReleaseYear("no-date")).toBe("");
    expect(getReleaseYear("")).toBe("");
    expect(getReleaseYear("12-3-45")).toBe("");
  });

  it("picks digits even in weird formats", () => {
    expect(getReleaseYear("2021年10月")).toBe("2021");
  });
});

describe("extractYearOptions", () => {
  it("returns unique years sorted descending", () => {
    const items = [
      makeItem({ release_date: "2022-01-01" }),
      makeItem({ release_date: "2020-05-01" }),
      makeItem({ release_date: "2022-07-01" }),
      makeItem({ release_date: "2018-12-31" }),
    ];
    expect(extractYearOptions(items)).toEqual(["2022", "2020", "2018"]);
  });

  it("skips items without a valid year", () => {
    const items = [
      makeItem({ release_date: "no-date" }),
      makeItem({ release_date: "" }),
      makeItem({ release_date: "2019-01-01" }),
    ];
    expect(extractYearOptions(items)).toEqual(["2019"]);
  });

  it("empty input returns []", () => {
    expect(extractYearOptions([])).toEqual([]);
  });
});

describe("mergeYearOptions", () => {
  it("dedupes and sorts descending", () => {
    expect(mergeYearOptions(["2020", "2022"], ["2021", "2022"])).toEqual(["2022", "2021", "2020"]);
  });

  it("current-only pass-through (still sorted)", () => {
    expect(mergeYearOptions(["2018", "2021"], [])).toEqual(["2021", "2018"]);
  });

  it("both empty returns []", () => {
    expect(mergeYearOptions([], [])).toEqual([]);
  });
});

describe("toMediaLibrarySyncMessage", () => {
  it("non-Error returns default", () => {
    expect(toMediaLibrarySyncMessage("bad")).toBe("启动媒体库同步失败");
    expect(toMediaLibrarySyncMessage(null)).toBe("启动媒体库同步失败");
  });

  it("maps known server errors", () => {
    expect(toMediaLibrarySyncMessage(new Error("media library sync is already running"))).toBe("媒体库正在同步中");
    expect(toMediaLibrarySyncMessage(new Error("move to media library is running"))).toBe("媒体库移动任务进行中，暂时无法同步");
    expect(toMediaLibrarySyncMessage(new Error("library dir is not configured"))).toBe("未配置媒体库目录");
  });

  it("unknown messages are returned raw", () => {
    expect(toMediaLibrarySyncMessage(new Error("boom"))).toBe("boom");
  });
});

describe("formatSyncLogTime", () => {
  it("non-finite / non-positive returns '--'", () => {
    expect(formatSyncLogTime(Number.NaN)).toBe("--");
    expect(formatSyncLogTime(Number.POSITIVE_INFINITY)).toBe("--");
    expect(formatSyncLogTime(0)).toBe("--");
    expect(formatSyncLogTime(-1)).toBe("--");
  });

  it("formats as YYYY-MM-DD HH:mm:ss with zero-padding", () => {
    // 选一个已知时间戳, 验证 pad 形式 (不依赖具体时区, 只查格式).
    const ts = new Date(2023, 0, 5, 9, 3, 7).getTime();
    const out = formatSyncLogTime(ts);
    expect(out).toBe("2023-01-05 09:03:07");
  });

  it("keeps 4-digit year and 2-digit components consistently", () => {
    const ts = new Date(2099, 11, 31, 23, 59, 59).getTime();
    expect(formatSyncLogTime(ts)).toBe("2099-12-31 23:59:59");
  });
});
