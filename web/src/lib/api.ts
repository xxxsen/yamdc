export type JobStatus = "init" | "processing" | "reviewing" | "done" | "failed";

export interface JobItem {
  id: number;
  job_uid: string;
  file_name: string;
  file_ext: string;
  rel_path: string;
  abs_path: string;
  number: string;
  raw_number: string;
  cleaned_number: string;
  number_source: string;
  number_clean_status: string;
  number_clean_confidence: string;
  number_clean_warnings: string;
  file_size: number;
  status: JobStatus;
  error_msg: string;
  created_at: number;
  updated_at: number;
}

export interface JobLogItem {
  id: number;
  job_id: number;
  level: string;
  stage: string;
  message: string;
  detail: string;
  created_at: number;
}

export interface ScrapeDataItem {
  id: number;
  job_id: number;
  source: string;
  version: number;
  raw_data: string;
  review_data: string;
  final_data: string;
  status: string;
  created_at: number;
  updated_at: number;
}

export interface MediaFileRef {
  name: string;
  key: string;
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

interface APIResponse<T> {
  code: number;
  message: string;
  data: T;
}

export interface JobListResponse {
  items: JobItem[];
  total: number;
  page: number;
  page_size: number;
}

function getBaseURL() {
  if (typeof window !== "undefined") {
    return "";
  }
  return process.env.YAMDC_API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";
}

export function getAPIBaseURL() {
  return getBaseURL();
}

export async function listJobs(params?: {
  status?: string;
  keyword?: string;
  page?: number;
  pageSize?: number;
  all?: boolean;
}) {
  const query = new URLSearchParams();
  if (params?.status) {
    query.set("status", params.status);
  }
  if (params?.keyword) {
    query.set("keyword", params.keyword);
  }
  if (params?.page) {
    query.set("page", String(params.page));
  }
  if (params?.pageSize) {
    query.set("page_size", String(params.pageSize));
  }
  if (params?.all) {
    query.set("all", "true");
  }
  const suffix = query.toString() ? `?${query.toString()}` : "";
  const resp = await fetch(`${getBaseURL()}/api/jobs${suffix}`, {
    cache: "no-store",
  });
  if (!resp.ok) {
    throw new Error(`list jobs failed: ${resp.status}`);
  }
  const data = (await resp.json()) as APIResponse<JobListResponse>;
  return data.data;
}

export async function triggerScan() {
  const resp = await fetch(`${getBaseURL()}/api/scan`, {
    method: "POST",
  });
  if (!resp.ok) {
    throw new Error(`scan failed: ${resp.status}`);
  }
  return (await resp.json()) as APIResponse<unknown>;
}

export async function runJob(id: number) {
  const resp = await fetch(`${getBaseURL()}/api/jobs/${id}/run`, {
    method: "POST",
  });
  const data = (await resp.json()) as APIResponse<unknown>;
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `run job failed: ${resp.status}`);
  }
  return data;
}

export async function rerunJob(id: number) {
  const resp = await fetch(`${getBaseURL()}/api/jobs/${id}/rerun`, {
    method: "POST",
  });
  const data = (await resp.json()) as APIResponse<unknown>;
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `rerun job failed: ${resp.status}`);
  }
  return data;
}

export async function deleteJob(id: number) {
  const resp = await fetch(`${getBaseURL()}/api/jobs/${id}`, {
    method: "DELETE",
  });
  const data = (await resp.json()) as APIResponse<unknown>;
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `delete job failed: ${resp.status}`);
  }
  return data;
}

export async function updateJobNumber(id: number, number: string) {
  const resp = await fetch(`${getBaseURL()}/api/jobs/${id}/number`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ number }),
  });
  const data = (await resp.json()) as APIResponse<JobItem>;
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `update job number failed: ${resp.status}`);
  }
  return data.data;
}

export async function listJobLogs(id: number) {
  const resp = await fetch(`${getBaseURL()}/api/jobs/${id}/logs`, {
    cache: "no-store",
  });
  if (!resp.ok) {
    throw new Error(`list logs failed: ${resp.status}`);
  }
  const data = (await resp.json()) as APIResponse<JobLogItem[]>;
  return data.data;
}

export async function getReviewJob(id: number) {
  const resp = await fetch(`${getBaseURL()}/api/review/jobs/${id}`, {
    cache: "no-store",
  });
  if (!resp.ok) {
    throw new Error(`get review job failed: ${resp.status}`);
  }
  const data = (await resp.json()) as APIResponse<ScrapeDataItem | null>;
  return data.data;
}

export async function saveReviewJob(id: number, reviewData: string) {
  const resp = await fetch(`${getBaseURL()}/api/review/jobs/${id}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ review_data: reviewData }),
  });
  const data = (await resp.json()) as APIResponse<unknown>;
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `save review job failed: ${resp.status}`);
  }
  return data;
}

export async function importReviewJob(id: number) {
  const resp = await fetch(`${getBaseURL()}/api/review/jobs/${id}/import`, {
    method: "POST",
  });
  const data = (await resp.json()) as APIResponse<unknown>;
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `import review job failed: ${resp.status}`);
  }
  return data;
}

export async function cropPosterFromCover(
  id: number,
  rect: { x: number; y: number; width: number; height: number },
) {
  const resp = await fetch(`${getBaseURL()}/api/review/jobs/${id}/poster-crop`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(rect),
  });
  const data = (await resp.json()) as APIResponse<MediaFileRef>;
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `crop poster failed: ${resp.status}`);
  }
  return data.data;
}

export async function uploadAsset(file: File) {
  const form = new FormData();
  form.append("file", file);
  const resp = await fetch(`${getBaseURL()}/api/assets/upload`, {
    method: "POST",
    body: form,
  });
  const data = (await resp.json()) as APIResponse<MediaFileRef>;
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `upload asset failed: ${resp.status}`);
  }
  return data.data;
}

export function getAssetURL(key: string) {
  return `/api/assets/${encodeURIComponent(key)}`;
}
