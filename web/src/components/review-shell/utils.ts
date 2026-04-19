import type { ReviewMeta, ScrapeDataItem } from "@/lib/api";

export function parseMeta(data: ScrapeDataItem | null): ReviewMeta | null {
  if (!data) {
    return null;
  }
  const raw = data.review_data || data.raw_data;
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as ReviewMeta;
  } catch {
    return null;
  }
}

// parseRawMeta: 只认 raw_data (原始刮削快照). 用来支撑 "恢复原始刮削" 按钮
// 的 hasRawMeta 判定. parseMeta 会 fallback 到 raw_data, 但 raw_data 未必存在,
// 两个不能复用.
export function parseRawMeta(data: ScrapeDataItem | null): ReviewMeta | null {
  if (!data?.raw_data) return null;
  try {
    return JSON.parse(data.raw_data) as ReviewMeta;
  } catch {
    return null;
  }
}

export function normalizeList(items?: string[]) {
  return (items ?? []).map((item) => item.trim()).filter(Boolean);
}

export function buildPayload(meta: ReviewMeta | null) {
  if (!meta) {
    return "";
  }
  return JSON.stringify(
    {
      ...meta,
      actors: normalizeList(meta.actors),
      genres: normalizeList(meta.genres),
    },
    null,
    2,
  );
}

export function imageTitle(type: string) {
  if (type === "cover") {
    return "封面";
  }
  if (type === "poster") {
    return "海报";
  }
  return "Extrafanart";
}
