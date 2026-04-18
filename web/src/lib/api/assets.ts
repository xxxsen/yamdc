import { logUploadDebug } from "@/lib/upload-debug";

import { apiRequest, type MediaFileRef } from "./core";

export async function uploadAsset(file: File, signal?: AbortSignal) {
  const startedAt = typeof performance !== "undefined" ? performance.now() : Date.now();
  logUploadDebug("api", "upload-asset-start", {
    fileName: file.name,
    size: file.size,
    type: file.type,
  });
  const form = new FormData();
  form.append("file", file);
  try {
    const data = await apiRequest<MediaFileRef>("/api/assets/upload", {
      method: "POST",
      formData: form,
      signal,
    });
    const durationMs = Math.round((typeof performance !== "undefined" ? performance.now() : Date.now()) - startedAt);
    logUploadDebug("api", "upload-asset-response", { ok: true, durationMs });
    return data.data;
  } catch (error) {
    const durationMs = Math.round((typeof performance !== "undefined" ? performance.now() : Date.now()) - startedAt);
    logUploadDebug("api", "upload-asset-response", {
      ok: false, durationMs,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

export function getAssetURL(key: string): string {
  return `/api/assets/${encodeURIComponent(key)}`;
}
