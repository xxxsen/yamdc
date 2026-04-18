import { apiRequest, buildPath } from "./core";
import {
  type LibraryFileItem,
  type LibraryListItem,
  type LibraryMeta,
  type LibraryVariant,
  normalizeLibraryMeta,
} from "./library";

export interface MediaLibraryItem extends LibraryListItem {
  id: number;
}

export interface MediaLibraryDetail {
  item: MediaLibraryItem;
  meta: LibraryMeta;
  variants: LibraryVariant[];
  primary_variant_key: string;
  files: LibraryFileItem[];
}

export interface TaskState {
  task_key: string;
  status: string;
  total: number;
  processed: number;
  success_count: number;
  conflict_count: number;
  error_count: number;
  current: string;
  message: string;
  started_at: number;
  finished_at: number;
  updated_at: number;
}

export interface MediaLibraryStatus {
  configured: boolean;
  sync: TaskState;
  move: TaskState;
}

export interface MediaLibrarySyncLogEntry {
  id: number;
  run_id: string;
  level: string;
  rel_path: string;
  message: string;
  created_at: number;
}

function normalizeMediaLibraryItem(item: MediaLibraryItem): MediaLibraryItem {
  return {
    ...item,
    actors: Array.isArray(item.actors) ? item.actors : [],
  };
}

function normalizeMediaLibraryDetail(detail: MediaLibraryDetail): MediaLibraryDetail {
  return {
    ...detail,
    item: normalizeMediaLibraryItem(detail.item),
    meta: normalizeLibraryMeta(detail.meta),
    variants: (Array.isArray(detail.variants) ? detail.variants : []).map((variant) => ({
      ...variant,
      meta: normalizeLibraryMeta(variant.meta),
      files: Array.isArray(variant.files) ? variant.files : [],
    })),
    files: Array.isArray(detail.files) ? detail.files : [],
  };
}

export async function listMediaLibraryItems(params?: {
  keyword?: string;
  year?: string;
  size?: string;
  sort?: string;
  order?: string;
}, signal?: AbortSignal) {
  const query = new URLSearchParams();
  if (params?.keyword?.trim()) {
    query.set("keyword", params.keyword.trim());
  }
  if (params?.year?.trim() && params.year !== "all") {
    query.set("year", params.year.trim());
  }
  if (params?.size?.trim() && params.size !== "all") {
    query.set("size", params.size.trim());
  }
  if (params?.sort?.trim() && params.sort !== "ingested") {
    query.set("sort", params.sort.trim());
  }
  if (params?.order?.trim() && params.order !== "desc") {
    query.set("order", params.order.trim());
  }
  const data = await apiRequest<MediaLibraryItem[]>(buildPath("/api/media-library", query), {
    cache: "no-store",
    signal,
  });
  return data.data.map(normalizeMediaLibraryItem);
}

export async function getMediaLibraryItem(id: number, signal?: AbortSignal) {
  const query = new URLSearchParams({ id: String(id) });
  const data = await apiRequest<MediaLibraryDetail>(buildPath("/api/media-library/item", query), {
    cache: "no-store",
    signal,
  });
  return normalizeMediaLibraryDetail(data.data);
}

export async function updateMediaLibraryItem(id: number, meta: LibraryMeta, signal?: AbortSignal) {
  const query = new URLSearchParams({ id: String(id) });
  const data = await apiRequest<MediaLibraryDetail>(buildPath("/api/media-library/item", query), {
    method: "PATCH",
    body: { meta },
    signal,
  });
  return normalizeMediaLibraryDetail(data.data);
}

export async function replaceMediaLibraryAsset(id: number, variant: string, kind: "poster" | "cover" | "fanart", file: File, signal?: AbortSignal) {
  const query = new URLSearchParams({ id: String(id), kind });
  if (variant) {
    query.set("variant", variant);
  }
  const form = new FormData();
  form.append("file", file);
  const data = await apiRequest<MediaLibraryDetail>(buildPath("/api/media-library/asset", query), {
    method: "POST",
    formData: form,
    signal,
  });
  return normalizeMediaLibraryDetail(data.data);
}

export async function deleteMediaLibraryFile(id: number, path: string, signal?: AbortSignal) {
  const query = new URLSearchParams({ id: String(id), path });
  const data = await apiRequest<MediaLibraryDetail>(buildPath("/api/media-library/file", query), {
    method: "DELETE",
    signal,
  });
  return normalizeMediaLibraryDetail(data.data);
}

export async function getMediaLibraryStatus(signal?: AbortSignal) {
  const data = await apiRequest<MediaLibraryStatus>("/api/media-library/status", {
    cache: "no-store",
    signal,
  });
  return data.data;
}

export async function triggerMediaLibrarySync(signal?: AbortSignal) {
  const data = await apiRequest<unknown>("/api/media-library/sync", { method: "POST", signal });
  return data;
}

export async function listMediaLibrarySyncLogs(limit?: number, signal?: AbortSignal) {
  const query = new URLSearchParams();
  if (typeof limit === "number" && Number.isFinite(limit) && limit > 0) {
    query.set("limit", String(Math.floor(limit)));
  }
  const path = query.toString()
    ? buildPath("/api/media-library/sync/logs", query)
    : "/api/media-library/sync/logs";
  const data = await apiRequest<MediaLibrarySyncLogEntry[] | null>(path, {
    cache: "no-store",
    signal,
  });
  return Array.isArray(data.data) ? data.data : [];
}

export async function triggerMoveToMediaLibrary(signal?: AbortSignal) {
  const data = await apiRequest<unknown>("/api/media-library/move", { method: "POST", signal });
  return data;
}

export function getMediaLibraryFileURL(path: string): string {
  return `/api/media-library/file?path=${encodeURIComponent(path)}`;
}
