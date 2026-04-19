import { logUploadDebug } from "@/lib/upload-debug";

import { apiRequest, buildPath, type MediaFileRef } from "./core";

// 字段与 internal/repository.ScrapeDataItem 对齐:
// raw_data       - 插件抓到的原始 JSON meta
// review_data    - 人工审核后保存的修改版 (优先于 raw_data 作为展示源)
// final_data     - 审核通过并落盘的最终版
// 历史前端 type 漏了 review_data, 但 review-shell.tsx 实际在读 data.review_data,
// 以前没跑通 web-check 所以 TS 没抓到, 补齐后 `make web-check` 才能过 build。
export interface ScrapeDataItem {
  id: number;
  job_id: number;
  raw_data: string;
  review_data: string;
  final_data: string;
  status: string;
  created_at: number;
  updated_at: number;
}

export interface ReviewMeta {
  number?: string;
  title?: string;
  title_translated?: string;
  plot?: string;
  plot_translated?: string;
  actors?: string[];
  release_date?: number;
  duration?: number;
  studio?: string;
  label?: string;
  series?: string;
  genres?: string[];
  director?: string;
  cover?: MediaFileRef | null;
  poster?: MediaFileRef | null;
  sample_images?: MediaFileRef[];
}

export async function getReviewJob(id: number, signal?: AbortSignal) {
  const data = await apiRequest<ScrapeDataItem | null>(`/api/review/jobs/${id}`, { cache: "no-store", signal });
  return data.data;
}

export async function saveReviewJob(id: number, reviewData: string, signal?: AbortSignal) {
  const data = await apiRequest<unknown>(`/api/review/jobs/${id}`, {
    method: "PUT",
    body: { review_data: reviewData },
    signal,
  });
  return data;
}

export async function importReviewJob(id: number, signal?: AbortSignal) {
  const data = await apiRequest<unknown>(`/api/review/jobs/${id}/import`, { method: "POST", signal });
  return data;
}

export async function cropPosterFromCover(
  id: number,
  rect: { x: number; y: number; width: number; height: number },
  signal?: AbortSignal,
) {
  const data = await apiRequest<MediaFileRef>(`/api/review/jobs/${id}/poster-crop`, {
    method: "POST",
    body: rect,
    signal,
  });
  return data.data;
}

export async function uploadReviewAsset(id: number, target: "cover" | "poster" | "fanart", file: File, signal?: AbortSignal) {
  const startedAt = typeof performance !== "undefined" ? performance.now() : Date.now();
  logUploadDebug("api", "upload-review-asset-start", {
    id,
    target,
    fileName: file.name,
    size: file.size,
    type: file.type,
  });
  const form = new FormData();
  form.append("file", file);
  const query = new URLSearchParams({ target });
  try {
    const data = await apiRequest<MediaFileRef>(buildPath(`/api/review/jobs/${id}/asset`, query), {
      method: "POST",
      formData: form,
      signal,
    });
    const durationMs = Math.round((typeof performance !== "undefined" ? performance.now() : Date.now()) - startedAt);
    logUploadDebug("api", "upload-review-asset-response", {
      id, target, ok: true, durationMs, key: data.data.key,
    });
    return data.data;
  } catch (error) {
    const durationMs = Math.round((typeof performance !== "undefined" ? performance.now() : Date.now()) - startedAt);
    logUploadDebug("api", "upload-review-asset-response", {
      id, target, ok: false, durationMs,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}
