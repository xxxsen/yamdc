import { describe, expect, it } from "vitest";

import type { ReviewMeta, ScrapeDataItem } from "@/lib/api";

import { buildPayload, imageTitle, normalizeList, parseMeta, parseRawMeta } from "../utils";

// review-shell/utils: meta 解析与 payload 构造. 覆盖 data 为 null / 没
// review_data 也没 raw_data / JSON parse 失败 / 只有 raw_data 等路径.

function makeDataItem(overrides: Partial<ScrapeDataItem> = {}): ScrapeDataItem {
  return {
    id: 0,
    job_id: 0,
    raw_data: "",
    review_data: "",
    final_data: "",
    status: "",
    created_at: 0,
    updated_at: 0,
    ...overrides,
  };
}

describe("parseMeta", () => {
  it("returns null when data is null", () => {
    expect(parseMeta(null)).toBeNull();
  });

  it("prefers review_data over raw_data", () => {
    const meta = parseMeta(makeDataItem({
      review_data: JSON.stringify({ title: "reviewed" }),
      raw_data: JSON.stringify({ title: "raw" }),
    }));
    expect(meta?.title).toBe("reviewed");
  });

  it("falls back to raw_data when review_data is empty", () => {
    const meta = parseMeta(makeDataItem({ raw_data: JSON.stringify({ title: "raw" }) }));
    expect(meta?.title).toBe("raw");
  });

  it("returns null when both review_data and raw_data are empty", () => {
    expect(parseMeta(makeDataItem())).toBeNull();
  });

  it("returns null when JSON parse fails", () => {
    expect(parseMeta(makeDataItem({ review_data: "{not json" }))).toBeNull();
  });
});

describe("parseRawMeta", () => {
  it("returns null when data is null", () => {
    expect(parseRawMeta(null)).toBeNull();
  });

  it("returns null when raw_data is empty (ignores review_data)", () => {
    expect(parseRawMeta(makeDataItem({ review_data: JSON.stringify({ title: "x" }) }))).toBeNull();
  });

  it("parses raw_data when present", () => {
    const meta = parseRawMeta(makeDataItem({ raw_data: JSON.stringify({ title: "raw" }) }));
    expect(meta?.title).toBe("raw");
  });

  it("returns null when raw_data is malformed", () => {
    expect(parseRawMeta(makeDataItem({ raw_data: "{garbage" }))).toBeNull();
  });
});

describe("normalizeList", () => {
  it("undefined returns []", () => {
    expect(normalizeList()).toEqual([]);
    expect(normalizeList(undefined)).toEqual([]);
  });

  it("trims and drops empty entries", () => {
    expect(normalizeList([" a ", "", "\n", "b\t"])).toEqual(["a", "b"]);
  });

  it("keeps non-empty in order", () => {
    expect(normalizeList(["x", "y", "z"])).toEqual(["x", "y", "z"]);
  });
});

describe("buildPayload", () => {
  it("returns empty string when meta is null", () => {
    expect(buildPayload(null)).toBe("");
  });

  it("produces pretty JSON with normalised actors/genres", () => {
    const meta: ReviewMeta = {
      title: "T",
      actors: [" a ", "", "b"],
      genres: ["drama", " "],
    };
    const json = buildPayload(meta);
    expect(json).toContain("\n");
    const parsed = JSON.parse(json);
    expect(parsed.actors).toEqual(["a", "b"]);
    expect(parsed.genres).toEqual(["drama"]);
    expect(parsed.title).toBe("T");
  });

  it("handles meta without actors/genres (undefined gets normalised to [])", () => {
    const json = buildPayload({ title: "T" });
    const parsed = JSON.parse(json);
    expect(parsed.actors).toEqual([]);
    expect(parsed.genres).toEqual([]);
  });
});

describe("imageTitle", () => {
  it("maps known kinds to Chinese labels", () => {
    expect(imageTitle("cover")).toBe("封面");
    expect(imageTitle("poster")).toBe("海报");
  });

  it("falls back to Extrafanart for anything else", () => {
    expect(imageTitle("fanart")).toBe("Extrafanart");
    expect(imageTitle("")).toBe("Extrafanart");
    expect(imageTitle("unknown")).toBe("Extrafanart");
  });
});
