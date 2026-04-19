import { describe, expect, it } from "vitest";

import type { LibraryMeta, MediaLibraryDetail, MediaLibraryItem, LibraryVariant } from "@/lib/api";

import { cloneMeta, getVariantCoverPath, normalizeMeta, pickVariant, serializeMeta } from "../utils";

// media-library-detail-shell/utils: media-library 详情页的纯工具.
// 与 library-shell/utils 并行的一份, 这里也各自冻结.

function makeMeta(overrides: Partial<LibraryMeta> = {}): LibraryMeta {
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

function makeVariant(overrides: Partial<LibraryVariant> = {}): LibraryVariant {
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
    meta: makeMeta(),
    files: [],
    file_count: 0,
    ...overrides,
  };
}

function makeDetail(overrides: Partial<MediaLibraryDetail> = {}): MediaLibraryDetail {
  return {
    item: makeItem(),
    meta: makeMeta(),
    variants: [makeVariant()],
    primary_variant_key: "v1",
    files: [],
    ...overrides,
  };
}

describe("cloneMeta", () => {
  it("null input yields fully-defaulted meta", () => {
    const result = cloneMeta(null);
    expect(result).toMatchObject({ title: "", actors: [], genres: [], runtime: 0 });
  });

  it("copies scalar fields and clones array fields", () => {
    const src = makeMeta({ title: "t", actors: ["a"], genres: ["g"] });
    const clone = cloneMeta(src);
    expect(clone.title).toBe("t");
    clone.actors.push("b");
    clone.genres.push("h");
    expect(src.actors).toEqual(["a"]);
    expect(src.genres).toEqual(["g"]);
  });

  it("undefined fields on input fall through to defaults", () => {
    // 强制给一个缺字段的对象 (模拟前端收到不完整 JSON).
    const partial = { title: "only" } as unknown as LibraryMeta;
    const result = cloneMeta(partial);
    expect(result.title).toBe("only");
    expect(result.release_date).toBe("");
    expect(result.actors).toEqual([]);
  });
});

describe("pickVariant", () => {
  it("null detail returns null", () => {
    expect(pickVariant(null, "v1")).toBeNull();
  });

  it("finds by key when present", () => {
    const v2 = makeVariant({ key: "v2" });
    const detail = makeDetail({ variants: [makeVariant(), v2] });
    expect(pickVariant(detail, "v2")).toBe(v2);
  });

  it("falls back to first variant when key missing", () => {
    const first = makeVariant({ key: "v1" });
    const detail = makeDetail({ variants: [first] });
    expect(pickVariant(detail, "missing")).toBe(first);
  });

  it("empty variants returns null", () => {
    expect(pickVariant(makeDetail({ variants: [] }), "v1")).toBeNull();
  });
});

describe("serializeMeta / normalizeMeta", () => {
  it("serializeMeta drops empties and trims", () => {
    const meta = makeMeta({ actors: [" a ", "", "b"], genres: ["  ", "g"] });
    const parsed = JSON.parse(serializeMeta(meta)) as LibraryMeta;
    expect(parsed.actors).toEqual(["a", "b"]);
    expect(parsed.genres).toEqual(["g"]);
  });

  it("normalizeMeta returns normalised lists", () => {
    expect(
      normalizeMeta(makeMeta({ actors: [" x "], genres: ["y "] })),
    ).toMatchObject({ actors: ["x"], genres: ["y"] });
  });
});

describe("getVariantCoverPath", () => {
  it("full priority chain: variant.cover > variant.meta.cover > fanart > thumb > detail.meta.* > detail.item.cover", () => {
    expect(
      getVariantCoverPath(makeDetail({ variants: [makeVariant({ cover_path: "a" })] }), "v1"),
    ).toBe("a");

    expect(
      getVariantCoverPath(
        makeDetail({ variants: [makeVariant({ meta: makeMeta({ cover_path: "b" }) })] }),
        "v1",
      ),
    ).toBe("b");

    expect(
      getVariantCoverPath(
        makeDetail({ variants: [makeVariant({ meta: makeMeta({ fanart_path: "c" }) })] }),
        "v1",
      ),
    ).toBe("c");

    expect(
      getVariantCoverPath(
        makeDetail({ variants: [makeVariant({ meta: makeMeta({ thumb_path: "d" }) })] }),
        "v1",
      ),
    ).toBe("d");

    const variantEmpty = makeVariant();
    expect(
      getVariantCoverPath(
        makeDetail({ variants: [variantEmpty], meta: makeMeta({ cover_path: "e" }) }),
        "v1",
      ),
    ).toBe("e");

    expect(
      getVariantCoverPath(
        makeDetail({ variants: [variantEmpty], meta: makeMeta({ fanart_path: "f" }) }),
        "v1",
      ),
    ).toBe("f");

    expect(
      getVariantCoverPath(
        makeDetail({ variants: [variantEmpty], meta: makeMeta({ thumb_path: "g" }) }),
        "v1",
      ),
    ).toBe("g");

    expect(
      getVariantCoverPath(
        makeDetail({
          variants: [variantEmpty],
          item: makeItem({ cover_path: "h" }),
        }),
        "v1",
      ),
    ).toBe("h");
  });

  it("null detail returns ''", () => {
    expect(getVariantCoverPath(null, "v1")).toBe("");
  });

  it("everything empty returns ''", () => {
    expect(getVariantCoverPath(makeDetail({ variants: [makeVariant()] }), "v1")).toBe("");
  });
});
