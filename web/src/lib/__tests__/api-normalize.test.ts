import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getLibraryItem,
  getMediaLibraryItem,
  listLibraryItems,
  listMediaLibraryItems,
} from "@/lib/api";

function mockAPIResponse(data: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        code: 0,
        message: "ok",
        data,
      }),
    }),
  );
}

describe("library api normalization", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("normalizes nullable library list actors", async () => {
    mockAPIResponse([
      {
        rel_path: "savedir/a",
        name: "a",
        title: "A",
        number: "ABC-123",
        release_date: "",
        actors: null,
        updated_at: 1,
        has_nfo: true,
        poster_path: "",
        cover_path: "",
        file_count: 1,
        video_count: 1,
        variant_count: 1,
        conflict: false,
      },
    ]);

    const result = await listLibraryItems();
    expect(result[0]?.actors).toEqual([]);
  });

  it("normalizes nullable library detail arrays", async () => {
    mockAPIResponse({
      item: {
        rel_path: "savedir/a",
        name: "a",
        title: "A",
        number: "ABC-123",
        release_date: "",
        actors: null,
        updated_at: 1,
        has_nfo: true,
        poster_path: "",
        cover_path: "",
        file_count: 1,
        video_count: 1,
        variant_count: 1,
        conflict: false,
      },
      meta: {
        title: "A",
        title_translated: "",
        original_title: "",
        plot: "",
        plot_translated: "",
        number: "ABC-123",
        release_date: "",
        runtime: 0,
        studio: "",
        label: "",
        series: "",
        director: "",
        actors: null,
        genres: null,
        poster_path: "",
        cover_path: "",
        fanart_path: "",
        thumb_path: "",
        source: "",
        scraped_at: "",
      },
      variants: null,
      primary_variant_key: "",
      files: null,
    });

    const result = await getLibraryItem("savedir/a");
    expect(result.item.actors).toEqual([]);
    expect(result.meta.actors).toEqual([]);
    expect(result.meta.genres).toEqual([]);
    expect(result.variants).toEqual([]);
    expect(result.files).toEqual([]);
  });

  it("normalizes nullable media library list actors", async () => {
    mockAPIResponse([
      {
        id: 1,
        rel_path: "library/a",
        name: "a",
        title: "A",
        number: "ABC-123",
        release_date: "",
        actors: null,
        updated_at: 1,
        has_nfo: true,
        poster_path: "",
        cover_path: "",
        file_count: 1,
        video_count: 1,
        variant_count: 1,
        conflict: false,
      },
    ]);

    const result = await listMediaLibraryItems();
    expect(result[0]?.actors).toEqual([]);
  });

  it("normalizes nullable media library detail arrays", async () => {
    mockAPIResponse({
      item: {
        id: 1,
        rel_path: "library/a",
        name: "a",
        title: "A",
        number: "ABC-123",
        release_date: "",
        actors: null,
        updated_at: 1,
        has_nfo: true,
        poster_path: "",
        cover_path: "",
        file_count: 1,
        video_count: 1,
        variant_count: 1,
        conflict: false,
      },
      meta: {
        title: "A",
        title_translated: "",
        original_title: "",
        plot: "",
        plot_translated: "",
        number: "ABC-123",
        release_date: "",
        runtime: 0,
        studio: "",
        label: "",
        series: "",
        director: "",
        actors: null,
        genres: null,
        poster_path: "",
        cover_path: "",
        fanart_path: "",
        thumb_path: "",
        source: "",
        scraped_at: "",
      },
      variants: [
        {
          key: "default",
          label: "Default",
          base_name: "a",
          suffix: "",
          is_primary: true,
          video_path: "",
          nfo_path: "",
          poster_path: "",
          cover_path: "",
          meta: {
            title: "A",
            title_translated: "",
            original_title: "",
            plot: "",
            plot_translated: "",
            number: "ABC-123",
            release_date: "",
            runtime: 0,
            studio: "",
            label: "",
            series: "",
            director: "",
            actors: null,
            genres: null,
            poster_path: "",
            cover_path: "",
            fanart_path: "",
            thumb_path: "",
            source: "",
            scraped_at: "",
          },
          files: null,
          file_count: 0,
        },
      ],
      primary_variant_key: "default",
      files: null,
    });

    const result = await getMediaLibraryItem(1);
    expect(result.item.actors).toEqual([]);
    expect(result.meta.actors).toEqual([]);
    expect(result.meta.genres).toEqual([]);
    expect(result.variants[0]?.meta.actors).toEqual([]);
    expect(result.variants[0]?.meta.genres).toEqual([]);
    expect(result.variants[0]?.files).toEqual([]);
    expect(result.files).toEqual([]);
  });
});
