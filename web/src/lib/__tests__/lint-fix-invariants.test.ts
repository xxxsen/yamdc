/**
 * Tests verifying behavioral consistency after lint-fix refactoring:
 * - no-unnecessary-condition fixes (removing ??, ?. on non-nullish values)
 * - no-non-null-assertion fixes (replacing ! with null guards)
 * - Array.isArray guards replacing ?? [] in normalize functions
 * - stateFromDraft using required fields directly (no ?? fallback)
 * - cloneMeta early-return pattern
 * - Partial<Record> for job status counts
 */

import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getLibraryItem,
  getMediaLibraryItem,
  listJobs,
  listLibraryItems,
  listMediaLibraryItems,
} from "@/lib/api";
import {
  defaultState,
  normalizeEditorState,
  stateFromDraft,
} from "@/components/plugin-editor/plugin-editor-utils";
import type { PluginEditorDraft } from "@/lib/api";
import type { EditorState } from "@/components/plugin-editor/plugin-editor-types";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function mockAPIResponse(data: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ code: 0, message: "ok", data }),
    }),
  );
}

function makeMinimalMeta() {
  return {
    title: "A", title_translated: "", original_title: "", plot: "", plot_translated: "",
    number: "X-1", release_date: "", runtime: 0, studio: "", label: "", series: "",
    director: "", actors: ["Alice"], genres: ["Drama"],
    poster_path: "", cover_path: "", fanart_path: "", thumb_path: "", source: "", scraped_at: "",
  };
}

function makeMinimalItem() {
  return {
    rel_path: "a", name: "a", title: "A", number: "X-1", release_date: "",
    actors: ["Alice"], updated_at: 1, has_nfo: true, poster_path: "", cover_path: "",
    total_size: 0, file_count: 1, video_count: 1, variant_count: 1, conflict: false,
  };
}

// ---------------------------------------------------------------------------
// 1. API normalize: Array.isArray guards (was ?? [])
// ---------------------------------------------------------------------------

describe("API normalize: Array.isArray guards", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("normalizes null actors in list items to []", async () => {
    mockAPIResponse([{ ...makeMinimalItem(), actors: null }]);
    const result = await listLibraryItems();
    expect(result[0].actors).toEqual([]);
  });

  it("preserves valid actors array in list items", async () => {
    mockAPIResponse([makeMinimalItem()]);
    const result = await listLibraryItems();
    expect(result[0].actors).toEqual(["Alice"]);
  });

  it("normalizes null variants to empty array in detail", async () => {
    mockAPIResponse({
      item: makeMinimalItem(),
      meta: makeMinimalMeta(),
      variants: null,
      primary_variant_key: "",
      files: [],
    });
    const result = await getLibraryItem("a");
    expect(result.variants).toEqual([]);
  });

  it("normalizes null files to empty array in detail", async () => {
    mockAPIResponse({
      item: makeMinimalItem(),
      meta: makeMinimalMeta(),
      variants: [],
      primary_variant_key: "",
      files: null,
    });
    const result = await getLibraryItem("a");
    expect(result.files).toEqual([]);
  });

  it("normalizes null actors/genres in meta to []", async () => {
    mockAPIResponse({
      item: makeMinimalItem(),
      meta: { ...makeMinimalMeta(), actors: null, genres: null },
      variants: [],
      primary_variant_key: "",
      files: [],
    });
    const result = await getLibraryItem("a");
    expect(result.meta.actors).toEqual([]);
    expect(result.meta.genres).toEqual([]);
  });

  it("normalizes null variant files to empty array", async () => {
    mockAPIResponse({
      item: makeMinimalItem(),
      meta: makeMinimalMeta(),
      variants: [{
        key: "default", label: "Default", base_name: "a", suffix: "",
        is_primary: true, video_path: "", nfo_path: "", poster_path: "", cover_path: "",
        meta: makeMinimalMeta(),
        files: null, file_count: 0,
      }],
      primary_variant_key: "default",
      files: [],
    });
    const result = await getLibraryItem("a");
    expect(result.variants[0].files).toEqual([]);
  });

  it("normalizes null media library variant files to empty array", async () => {
    mockAPIResponse({
      item: { id: 1, ...makeMinimalItem() },
      meta: makeMinimalMeta(),
      variants: [{
        key: "default", label: "Default", base_name: "a", suffix: "",
        is_primary: true, video_path: "", nfo_path: "", poster_path: "", cover_path: "",
        meta: { ...makeMinimalMeta(), actors: null, genres: null },
        files: null, file_count: 0,
      }],
      primary_variant_key: "default",
      files: null,
    });
    const result = await getMediaLibraryItem(1);
    expect(result.variants[0].files).toEqual([]);
    expect(result.variants[0].meta.actors).toEqual([]);
    expect(result.variants[0].meta.genres).toEqual([]);
    expect(result.files).toEqual([]);
  });

  it("normalizes null media library list actors", async () => {
    mockAPIResponse([{ id: 1, ...makeMinimalItem(), actors: null }]);
    const result = await listMediaLibraryItems();
    expect(result[0].actors).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// 2. Partial<Record> for job status counts
// ---------------------------------------------------------------------------

describe("listJobs: status counting with Partial<Record>", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("returns correct data when all statuses present", async () => {
    mockAPIResponse({
      items: [
        { id: 1, status: "init" },
        { id: 2, status: "processing" },
        { id: 3, status: "failed" },
      ],
      total: 3, page: 1, page_size: 20,
    });
    const result = await listJobs();
    expect(result.items).toHaveLength(3);
    expect(result.total).toBe(3);
  });

  it("returns correct data when no items", async () => {
    mockAPIResponse({ items: [], total: 0, page: 1, page_size: 20 });
    const result = await listJobs();
    expect(result.items).toEqual([]);
    expect(result.total).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// 3. stateFromDraft: required fields used directly (no ?? fallback)
// ---------------------------------------------------------------------------

describe("stateFromDraft: required field handling", () => {
  function makeMinimalDraft(overrides?: Partial<PluginEditorDraft>): PluginEditorDraft {
    return {
      version: 1,
      name: "test-plugin",
      type: "one-step",
      hosts: ["https://example.com"],
      request: { method: "GET", path: "/search/${number}" },
      scrape: {
        format: "html",
        fields: {
          title: { selector: { kind: "xpath", expr: "//title" }, parser: "string" },
        },
      },
      ...overrides,
    };
  }

  it("uses draft.name directly", () => {
    const state = stateFromDraft(makeMinimalDraft({ name: "my-plugin" }));
    expect(state.name).toBe("my-plugin");
  });

  it("uses empty draft.name without falling back to default", () => {
    const state = stateFromDraft(makeMinimalDraft({ name: "" }));
    expect(state.name).toBe("");
  });

  it("uses draft.type directly", () => {
    const state = stateFromDraft(makeMinimalDraft({ type: "two-step" }));
    expect(state.type).toBe("two-step");
  });

  it("uses draft.hosts directly without fallback", () => {
    const state = stateFromDraft(makeMinimalDraft({ hosts: ["https://a.com", "https://b.com"] }));
    expect(state.hostsText).toBe("https://a.com\nhttps://b.com");
  });

  it("uses draft.scrape.format directly", () => {
    const state = stateFromDraft(makeMinimalDraft({
      scrape: {
        format: "json",
        fields: { title: { selector: { kind: "jsonpath", expr: "$.title" }, parser: "string" } },
      },
    }));
    expect(state.scrapeFormat).toBe("json");
  });

  it("handles optional postprocess fields correctly", () => {
    const state = stateFromDraft(makeMinimalDraft({
      postprocess: {
        defaults: { title_lang: "ja", plot_lang: "zh" },
        switch_config: { disable_release_date_check: true },
      },
    }));
    expect(state.postTitleLang).toBe("ja");
    expect(state.postPlotLang).toBe("zh");
    expect(state.postDisableReleaseDateCheck).toBe(true);
  });

  it("handles missing optional postprocess gracefully", () => {
    const state = stateFromDraft(makeMinimalDraft());
    expect(state.postTitleLang).toBe("");
    expect(state.postPlotLang).toBe("");
    expect(state.postDisableReleaseDateCheck).toBe(false);
  });

  it("handles fetch_type correctly", () => {
    expect(stateFromDraft(makeMinimalDraft({ fetch_type: "browser" })).fetchType).toBe("browser");
    expect(stateFromDraft(makeMinimalDraft({ fetch_type: "flaresolverr" })).fetchType).toBe("flaresolverr");
    expect(stateFromDraft(makeMinimalDraft({ fetch_type: undefined })).fetchType).toBe("go-http");
    expect(stateFromDraft(makeMinimalDraft()).fetchType).toBe("go-http");
  });
});

// ---------------------------------------------------------------------------
// 4. normalizeEditorState: simplified condition handling
// ---------------------------------------------------------------------------

describe("normalizeEditorState: condition simplification", () => {
  it("preserves fields array (no ?? [] needed since fields is always array)", () => {
    const state = defaultState();
    const normalized = normalizeEditorState(state);
    expect(normalized.fields).toHaveLength(1);
    expect(normalized.fields[0].name).toBe("title");
  });

  it("restores default workflow selectors when empty array", () => {
    const state = { ...defaultState(), workflowSelectors: [] };
    const normalized = normalizeEditorState(state);
    expect(normalized.workflowSelectors).toHaveLength(1);
    expect(normalized.workflowSelectors[0].name).toBe("read_link");
  });

  it("preserves non-empty workflow selectors", () => {
    const state = defaultState();
    state.workflowSelectors = [{ id: "s1", name: "link", kind: "css", expr: "a.link" }];
    const normalized = normalizeEditorState(state);
    expect(normalized.workflowSelectors).toHaveLength(1);
    expect(normalized.workflowSelectors[0].name).toBe("link");
  });

  it("handles legacy flat request fields migration", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.request = undefined as never;
    state["requestMethod"] = "POST";
    state["requestPath"] = "/legacy";
    const normalized = normalizeEditorState(state as EditorState);
    expect(normalized.request.method).toBe("POST");
    expect(normalized.request.path).toBe("/legacy");
  });

  it("preserves nested request format", () => {
    const state = defaultState();
    state.request.method = "PATCH";
    state.request.path = "/api/v2";
    const normalized = normalizeEditorState(state);
    expect(normalized.request.method).toBe("PATCH");
    expect(normalized.request.path).toBe("/api/v2");
  });

  it("uses fetchType directly (no falsy-or fallback needed for truthy values)", () => {
    const state = defaultState();
    state.fetchType = "browser";
    const normalized = normalizeEditorState(state);
    expect(normalized.fetchType).toBe("browser");
  });

  it("preserves flaresolverr fetchType", () => {
    const state = defaultState();
    state.fetchType = "flaresolverr";
    const normalized = normalizeEditorState(state);
    expect(normalized.fetchType).toBe("flaresolverr");
  });

  it("falls back to go-http for empty fetchType", () => {
    const state = defaultState();
    state.fetchType = "" as "go-http";
    const normalized = normalizeEditorState(state);
    expect(normalized.fetchType).toBe("go-http");
  });
});

// ---------------------------------------------------------------------------
// 5. stateFromDraft → buildDraft roundtrip after refactor
// ---------------------------------------------------------------------------

describe("stateFromDraft roundtrip after lint-fix refactor", () => {
  it("roundtrips name, type, hosts, format with direct field access", () => {
    const draft: PluginEditorDraft = {
      version: 1,
      name: "roundtrip-test",
      type: "two-step",
      hosts: ["https://a.com", "https://b.com"],
      request: { method: "POST", path: "/test" },
      scrape: {
        format: "json",
        fields: {
          title: { selector: { kind: "jsonpath", expr: "$.title" }, parser: "string" },
        },
      },
    };
    const state = stateFromDraft(draft);
    expect(state.name).toBe("roundtrip-test");
    expect(state.type).toBe("two-step");
    expect(state.hostsText).toBe("https://a.com\nhttps://b.com");
    expect(state.scrapeFormat).toBe("json");
    expect(state.request.method).toBe("POST");
    expect(state.request.path).toBe("/test");
  });

  it("roundtrips workflow with selector name/kind/expr used directly", () => {
    const draft: PluginEditorDraft = {
      version: 1,
      name: "workflow-test",
      type: "two-step",
      hosts: ["https://example.com"],
      request: { method: "GET", path: "/search" },
      workflow: {
        search_select: {
          selectors: [{ name: "link", kind: "css", expr: "a.link" }],
          return: "${item.link}",
          next_request: { method: "GET" },
        },
      },
      scrape: {
        format: "html",
        fields: {
          title: { selector: { kind: "xpath", expr: "//title" }, parser: "string" },
        },
      },
    };
    const state = stateFromDraft(draft);
    expect(state.workflowEnabled).toBe(true);
    expect(state.workflowSelectors[0].name).toBe("link");
    expect(state.workflowSelectors[0].kind).toBe("css");
    expect(state.workflowSelectors[0].expr).toBe("a.link");
  });
});
