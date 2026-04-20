import type { JobItem } from "@/lib/api";

export const STATUS_FILTER = "init,processing,failed,reviewing";

// jobSortKey 取一个最能代表 "影片 ID 前缀" 的字段: 优先用清洗后的 number,
// 没清洗过 (init) 就拿 raw_number 兜底。localeCompare 的 numeric:true 会把
// "ABC-2" / "ABC-10" 当数字比较, 相同前缀会天然聚到一起, 这是刻意选的行为
// — 用户反馈想按影片 ID 前缀分组而不是时间序。
export function jobSortKey(job: JobItem) {
  return (job.number || job.raw_number || job.cleaned_number || "").trim();
}

export function compareJobsByNumber(a: JobItem, b: JobItem) {
  const aKey = jobSortKey(a);
  const bKey = jobSortKey(b);
  // 都没 number 的 (比如刚扫进来又清洗失败) 按 id DESC 兜底, 保留 "最新在前"
  // 的直觉; 有 number 的永远排在没 number 的前面, 避免空影片 ID 把列表头占满。
  if (!aKey && !bKey) {
    return b.id - a.id;
  }
  if (!aKey) {
    return 1;
  }
  if (!bKey) {
    return -1;
  }
  const cmp = aKey.localeCompare(bKey, "zh-Hans-CN", { numeric: true, sensitivity: "base" });
  if (cmp !== 0) {
    return cmp;
  }
  // 同 number 的 (极少见, 一般是 conflict_reason 场景) 按 id DESC 作稳定兜底,
  // 免得用户每次刷新顺序都飘。
  return b.id - a.id;
}

// canSelectJob 判断一个任务是否可加入批量提交. 两个条件都必须满足:
//   1) 影片 ID 已确认 — manual 源, 或自动清洗得到 high/medium 置信度.
//   2) 处于可运行态 — init (未提交) 或 failed (可重试).
// conflict_reason 不空直接排除, 避免批量提交触发目标冲突.
export function canSelectJob(job: JobItem) {
  if (job.conflict_reason) {
    return false;
  }
  const hasApprovedNumber =
    job.number_source === "manual" ||
    (job.number_clean_status === "success" &&
      (job.number_clean_confidence === "high" || job.number_clean_confidence === "medium"));
  const isRunnable = job.status === "init" || job.status === "failed";
  return hasApprovedNumber && isRunnable;
}

export interface NumberMeta {
  tone: string;
  kind: "success" | "manual" | "warn" | "danger";
  warning: string;
}

export function getNumberMeta(job: JobItem): NumberMeta {
  if (job.number_source === "manual") {
    return {
      tone: "var(--info)",
      kind: "manual",
      warning: "用户已手动编辑影片 ID",
    };
  }
  if (job.number_clean_status === "success" && job.number_clean_confidence === "high") {
    return {
      tone: "var(--ok)",
      kind: "success",
      warning: "",
    };
  }
  if (job.number_clean_status === "success" && job.number_clean_confidence === "medium") {
    return {
      tone: "var(--warn)",
      kind: "warn",
      warning: job.number_clean_warnings || "",
    };
  }
  if (
    job.number_clean_status === "no_match" ||
    job.number_clean_status === "low_quality" ||
    job.number_clean_confidence === "low"
  ) {
    return {
      tone: "var(--danger)",
      kind: "danger",
      warning: job.number_clean_warnings || "清洗失败，当前使用原始值",
    };
  }
  return {
    tone: "var(--warn)",
    kind: "warn",
    warning: job.number_clean_warnings || "",
  };
}

export function requiresManualNumberReview(job: JobItem) {
  if (job.number_source === "manual") {
    return false;
  }
  if (job.number_clean_status === "no_match" || job.number_clean_status === "low_quality") {
    return true;
  }
  return job.number_clean_confidence === "low";
}

export function getNumberHint(job: JobItem) {
  if (job.conflict_reason) {
    return "目标文件名冲突，需先处理";
  }
  if (job.number_source === "manual") {
    return "已手动确认";
  }
  if (job.number_clean_status === "success" && job.number_clean_confidence === "high") {
    return "高置信度，可直接提交";
  }
  if (job.number_clean_status === "success" && job.number_clean_confidence === "medium") {
    return job.number_clean_warnings || "中等置信度，建议检查";
  }
  return job.number_clean_warnings || "需先手动修正影片 ID";
}

export interface PathSegments {
  folder: string;
  name: string;
}

export function getPathSegments(job: JobItem): PathSegments {
  const segments = job.rel_path.split("/").filter(Boolean);
  return {
    folder: segments.length > 1 ? segments.slice(0, -1).join(" / ") : "根目录",
    name: segments.length > 0 ? segments[segments.length - 1] : job.rel_path,
  };
}
