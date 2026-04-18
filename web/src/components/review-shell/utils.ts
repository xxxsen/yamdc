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
