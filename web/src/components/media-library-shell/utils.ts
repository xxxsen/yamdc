import type { MediaLibraryItem } from "@/lib/api";

// Pure helpers extracted from media-library-shell.tsx.

// getReleaseYear pulls the first 4-digit run out of a release_date
// string. The backend currently emits YYYY-MM-DD for normal records
// but also non-ISO strings for edge cases; this regex survives both.
export function getReleaseYear(value: string) {
  const match = value.match(/\d{4}/);
  return match?.[0] ?? "";
}

export function extractYearOptions(items: MediaLibraryItem[]) {
  const seenYears = new Set<string>();
  for (const item of items) {
    const year = getReleaseYear(item.release_date);
    if (year) {
      seenYears.add(year);
    }
  }
  return Array.from(seenYears).sort((left, right) => Number.parseInt(right, 10) - Number.parseInt(left, 10));
}

export function mergeYearOptions(current: string[], next: string[]) {
  return Array.from(new Set([...current, ...next])).sort((left, right) => Number.parseInt(right, 10) - Number.parseInt(left, 10));
}

// toMediaLibrarySyncMessage maps backend error strings to user-
// facing Chinese copy. We match on substring (not equal) because
// the backend prefixes some errors with a path / context string.
export function toMediaLibrarySyncMessage(error: unknown) {
  const raw = error instanceof Error ? error.message : "启动媒体库同步失败";
  const text = raw.trim();
  if (text.includes("media library sync is already running")) {
    return "媒体库正在同步中";
  }
  if (text.includes("move to media library is running")) {
    return "媒体库移动任务进行中，暂时无法同步";
  }
  if (text.includes("library dir is not configured")) {
    return "未配置媒体库目录";
  }
  return raw;
}

// formatSyncLogTime turns a backend unix-ms timestamp into
// "YYYY-MM-DD HH:mm:ss" local time. We hand-format instead of
// toLocaleString to avoid per-browser / per-OS drift and the
// timezone fragmentation inherent to the locale API.
export function formatSyncLogTime(timestampMs: number): string {
  if (!Number.isFinite(timestampMs) || timestampMs <= 0) {
    return "--";
  }
  const d = new Date(timestampMs);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
