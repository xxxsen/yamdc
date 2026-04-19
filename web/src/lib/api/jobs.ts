import { apiRequest, buildPath } from "./core";

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
  conflict_reason: string;
  conflict_target: string;
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

export interface JobListResponse {
  items: JobItem[];
  total: number;
  page: number;
  page_size: number;
}

export async function listJobs(params?: {
  status?: string;
  keyword?: string;
  page?: number;
  pageSize?: number;
  all?: boolean;
}, signal?: AbortSignal) {
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
  const data = await apiRequest<JobListResponse>(buildPath("/api/jobs", query), {
    cache: "no-store",
    signal,
  });
  return data.data;
}

export async function runJob(id: number, signal?: AbortSignal) {
  const data = await apiRequest<unknown>(`/api/jobs/${id}/run`, { method: "POST", signal });
  return data;
}

export async function rerunJob(id: number, signal?: AbortSignal) {
  const data = await apiRequest<unknown>(`/api/jobs/${id}/rerun`, { method: "POST", signal });
  return data;
}

export async function deleteJob(id: number, signal?: AbortSignal) {
  const data = await apiRequest<unknown>(`/api/jobs/${id}`, { method: "DELETE", signal });
  return data;
}

export async function updateJobNumber(id: number, number: string, signal?: AbortSignal) {
  const data = await apiRequest<JobItem>(`/api/jobs/${id}/number`, {
    method: "PATCH",
    body: { number },
    signal,
  });
  return data.data;
}

export async function listJobLogs(id: number, signal?: AbortSignal) {
  const data = await apiRequest<JobLogItem[]>(`/api/jobs/${id}/logs`, { cache: "no-store", signal });
  return data.data;
}
