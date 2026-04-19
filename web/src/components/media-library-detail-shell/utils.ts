import type { LibraryMeta, MediaLibraryDetail } from "@/lib/api";

const EMPTY_META: LibraryMeta = {
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
};

export function cloneMeta(meta: LibraryMeta | null): LibraryMeta {
  return {
    ...EMPTY_META,
    ...(meta ?? {}),
    actors: [...(meta?.actors ?? [])],
    genres: [...(meta?.genres ?? [])],
  };
}

export function pickVariant(detail: MediaLibraryDetail | null, key: string) {
  if (!detail) {
    return null;
  }
  return detail.variants.find((item) => item.key === key) ?? detail.variants.at(0) ?? null;
}

export function serializeMeta(meta: LibraryMeta) {
  return JSON.stringify({
    ...meta,
    actors: meta.actors.map((item) => item.trim()).filter(Boolean),
    genres: meta.genres.map((item) => item.trim()).filter(Boolean),
  });
}

export function normalizeMeta(meta: LibraryMeta): LibraryMeta {
  return {
    ...meta,
    actors: meta.actors.map((item) => item.trim()).filter(Boolean),
    genres: meta.genres.map((item) => item.trim()).filter(Boolean),
  };
}

// getVariantCoverPath walks a priority chain to resolve the best
// available cover image for the stage backdrop. The ordering matters:
// variant-level cover/meta paths come first so per-variant overrides
// win, then detail-level meta fallbacks, and finally detail.item's
// denormalised card-grid cover as a last resort.
export function getVariantCoverPath(detail: MediaLibraryDetail | null, variantKey: string) {
  const variant = pickVariant(detail, variantKey);
  const candidates = [
    variant?.cover_path,
    variant?.meta.cover_path,
    variant?.meta.fanart_path,
    variant?.meta.thumb_path,
    detail?.meta.cover_path,
    detail?.meta.fanart_path,
    detail?.meta.thumb_path,
    detail?.item.cover_path,
  ];
  return candidates.find((path) => !!path) ?? "";
}
