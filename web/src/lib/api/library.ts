import { logUploadDebug } from "@/lib/upload-debug";

import { apiRequest, buildPath } from "./core";

export interface LibraryListItem {
  rel_path: string;
  name: string;
  title: string;
  number: string;
  release_date: string;
  actors: string[];
  created_at: number;
  updated_at: number;
  has_nfo: boolean;
  poster_path: string;
  cover_path: string;
  total_size: number;
  file_count: number;
  video_count: number;
  variant_count: number;
  conflict: boolean;
}

export interface LibraryMeta {
  title: string;
  title_translated: string;
  original_title: string;
  plot: string;
  plot_translated: string;
  number: string;
  release_date: string;
  runtime: number;
  studio: string;
  label: string;
  series: string;
  director: string;
  actors: string[];
  genres: string[];
  poster_path: string;
  cover_path: string;
  fanart_path: string;
  thumb_path: string;
  source: string;
  scraped_at: string;
}

export interface LibraryFileItem {
  name: string;
  rel_path: string;
  kind: string;
  size: number;
  updated_at: number;
  variant_key?: string;
  variant_label?: string;
}

export interface LibraryVariant {
  key: string;
  label: string;
  base_name: string;
  suffix: string;
  is_primary: boolean;
  video_path: string;
  nfo_path: string;
  poster_path: string;
  cover_path: string;
  meta: LibraryMeta;
  files: LibraryFileItem[];
  file_count: number;
}

export interface LibraryDetail {
  item: LibraryListItem;
  meta: LibraryMeta;
  variants: LibraryVariant[];
  primary_variant_key: string;
  files: LibraryFileItem[];
}

// 以下 normalize 函数对外也有用: media-library.ts 共享 LibraryMeta/LibraryFileItem
// 的归一化逻辑, 跨文件 import 时通过 /lib/api (index.ts re-export) 走不干净,
// 所以保持直接 export, 让 media-library.ts 从本模块拿。
export function normalizeLibraryListItem(item: LibraryListItem): LibraryListItem {
  return {
    ...item,
    actors: Array.isArray(item.actors) ? item.actors : [],
  };
}

export function normalizeLibraryMeta(meta: LibraryMeta): LibraryMeta {
  return {
    ...meta,
    actors: Array.isArray(meta.actors) ? meta.actors : [],
    genres: Array.isArray(meta.genres) ? meta.genres : [],
  };
}

export function normalizeLibraryDetail(detail: LibraryDetail): LibraryDetail {
  return {
    ...detail,
    item: normalizeLibraryListItem(detail.item),
    meta: normalizeLibraryMeta(detail.meta),
    variants: (Array.isArray(detail.variants) ? detail.variants : []).map((variant) => ({
      ...variant,
      meta: normalizeLibraryMeta(variant.meta),
      files: Array.isArray(variant.files) ? variant.files : [],
    })),
    files: Array.isArray(detail.files) ? detail.files : [],
  };
}

export async function listLibraryItems(signal?: AbortSignal) {
  const data = await apiRequest<LibraryListItem[]>("/api/library", { cache: "no-store", signal });
  return data.data.map(normalizeLibraryListItem);
}

export async function getLibraryItem(path: string, signal?: AbortSignal) {
  const query = new URLSearchParams({ path });
  const data = await apiRequest<LibraryDetail>(buildPath("/api/library/item", query), {
    cache: "no-store",
    signal,
  });
  return normalizeLibraryDetail(data.data);
}

export async function updateLibraryItem(path: string, meta: LibraryMeta, signal?: AbortSignal) {
  const query = new URLSearchParams({ path });
  const data = await apiRequest<LibraryDetail>(buildPath("/api/library/item", query), {
    method: "PATCH",
    body: { meta },
    signal,
  });
  return normalizeLibraryDetail(data.data);
}

export async function deleteLibraryItem(path: string, signal?: AbortSignal) {
  const query = new URLSearchParams({ path });
  const data = await apiRequest<unknown>(buildPath("/api/library/item", query), {
    method: "DELETE",
    signal,
  });
  return data;
}

export async function replaceLibraryAsset(path: string, variant: string, kind: "poster" | "cover" | "fanart", file: File, signal?: AbortSignal) {
  const startedAt = typeof performance !== "undefined" ? performance.now() : Date.now();
  logUploadDebug("api", "replace-library-asset-start", {
    path,
    variant,
    kind,
    fileName: file.name,
    size: file.size,
    type: file.type,
  });
  const query = new URLSearchParams({ path, kind });
  if (variant) {
    query.set("variant", variant);
  }
  const form = new FormData();
  form.append("file", file);
  try {
    const data = await apiRequest<LibraryDetail>(buildPath("/api/library/asset", query), {
      method: "POST",
      formData: form,
      signal,
    });
    const durationMs = Math.round((typeof performance !== "undefined" ? performance.now() : Date.now()) - startedAt);
    logUploadDebug("api", "replace-library-asset-response", { path, variant, kind, ok: true, durationMs });
    return normalizeLibraryDetail(data.data);
  } catch (error) {
    const durationMs = Math.round((typeof performance !== "undefined" ? performance.now() : Date.now()) - startedAt);
    logUploadDebug("api", "replace-library-asset-response", {
      path, variant, kind, ok: false, durationMs,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

export async function cropLibraryPosterFromCover(
  path: string,
  variant: string,
  rect: { x: number; y: number; width: number; height: number },
  signal?: AbortSignal,
) {
  const query = new URLSearchParams({ path });
  if (variant) {
    query.set("variant", variant);
  }
  const data = await apiRequest<LibraryDetail>(buildPath("/api/library/poster-crop", query), {
    method: "POST",
    body: rect,
    signal,
  });
  return normalizeLibraryDetail(data.data);
}

export async function deleteLibraryFile(path: string, signal?: AbortSignal) {
  const query = new URLSearchParams({ path });
  const data = await apiRequest<LibraryDetail>(buildPath("/api/library/file", query), {
    method: "DELETE",
    signal,
  });
  return normalizeLibraryDetail(data.data);
}

export function getLibraryFileURL(path: string): string {
  return `/api/library/file?path=${encodeURIComponent(path)}`;
}
