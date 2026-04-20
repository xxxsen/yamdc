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

// NumberVariantKind 对应后端 number.VariantKind:
//   - "flag"    : 开关式 variant (中字 / 4K / 8K / VR / 特别版 / 修复版)
//   - "indexed" : 带 index 的 variant (CD 多盘), index 必须在 [min, max] 区间
export type NumberVariantKind = "flag" | "indexed";

// NumberVariantDescriptor 对应后端 number.VariantDescriptor, 用于驱动
// "文件列表" 页结构化影片 ID 输入的 UI 渲染 (id 做 stable key, label/description
// 用于展示, kind + min/max 决定交互形式)。
//
// group 是互斥分组 key: 字段缺省 / 空串表示该 variant 独立, 非空时同
// group 内只能勾选一个 (例如 4K / 8K 同属 resolution, LEAK / UC 同属
// edition)。前端按 group 做 "点击替换" 交互, 后端也会在 variant layer
// 做兜底校验, 客户端绕过 UI 直接提交冲突组合会得到 400。
export interface NumberVariantDescriptor {
  id: string;
  suffix: string;
  label: string;
  description: string;
  kind: NumberVariantKind;
  group?: string;
  min?: number;
  max?: number;
}

export interface NumberVariantSelection {
  id: string;
  index?: number;
}

// listNumberVariants 拉取后端支持的 variant 描述符列表。后端保证:
//   - id/suffix 在同一版本内稳定 (前端可当 persistent key);
//   - 顺序即是期望的 UI 渲染顺序 (和落盘文件名拼接顺序一致)。
export async function listNumberVariants(signal?: AbortSignal) {
  const data = await apiRequest<{ variants: NumberVariantDescriptor[] }>("/api/number/variants", {
    cache: "no-store",
    signal,
  });
  return data.data.variants;
}

// updateJobNumberStructured 调 PATCH /api/jobs/:id/number 的新结构化入口,
// 后端会用 base + selections 拼出 "影片 ID-后缀1-后缀2" 形态的完整 number,
// 然后复用老校验 / 持久化链路。推荐新代码一律走此入口。老的 updateJobNumber
// 仍然保留, 给只需要传完整 number 的老调用方 / 脚本使用。
export async function updateJobNumberStructured(
  id: number,
  base: string,
  variants: NumberVariantSelection[],
  signal?: AbortSignal,
) {
  const data = await apiRequest<JobItem>(`/api/jobs/${id}/number`, {
    method: "PATCH",
    body: { base, variants },
    signal,
  });
  return data.data;
}

export async function listJobLogs(id: number, signal?: AbortSignal) {
  const data = await apiRequest<JobLogItem[]>(`/api/jobs/${id}/logs`, { cache: "no-store", signal });
  return data.data;
}
