import { type SetStateAction } from "react";

import type { LibraryDetail, LibraryListItem, LibraryMeta, MediaLibraryStatus, TaskState } from "@/lib/api";
import { getMediaLibraryStatus } from "@/lib/api";

// library-shell 内部共享的纯工具函数。整块从 library-shell.tsx 头部搬过来,
// 语义零改动 —— 目的是让主文件只专注于 LibraryShell 组件本身 (state /
// effect / JSX), 把可复用 / 可单测的逻辑层独立出来。
//
// 设计原则:
//   - 优先保持 pure: 只有 handleMoveToMediaLibraryError 和调用 API 的实参
//     有副作用 (触发 status refresh), 其它全部 pure。
//   - 不引入 React / DOM 依赖 — 保证 utils 以后可以直接单测。
//   - 不要在这里组合 state/effect, 那些交给主文件。
//
// 详见 td/022-frontend-optimization-roadmap.md §2.2 B-2。

export function cloneMeta(meta: LibraryMeta | null | undefined): LibraryMeta {
  if (!meta) {
    return {
      title: "", title_translated: "", original_title: "", plot: "", plot_translated: "",
      number: "", release_date: "", runtime: 0, studio: "", label: "", series: "",
      director: "", actors: [], genres: [], poster_path: "", cover_path: "",
      fanart_path: "", thumb_path: "", source: "", scraped_at: "",
    };
  }
  return {
    ...meta,
    actors: [...meta.actors],
    genres: [...meta.genres],
  };
}

export function pickVariant(detail: LibraryDetail | null, key: string) {
  if (!detail) {
    return null;
  }
  return detail.variants.find((item) => item.key === key) ?? detail.variants.at(0) ?? null;
}

export function serializeMeta(meta: LibraryMeta) {
  return JSON.stringify({
    ...meta,
    actors: meta.actors.map((item) => item.trim()).filter(Boolean),
    genres: meta.genres.map((item) => item.trim()).filter(Boolean),
  });
}

export function normalizeMeta(meta: LibraryMeta): LibraryMeta {
  return {
    ...meta,
    actors: meta.actors.map((item) => item.trim()).filter(Boolean),
    genres: meta.genres.map((item) => item.trim()).filter(Boolean),
  };
}

export function getCardImage(item: LibraryListItem) {
  return item.poster_path || item.cover_path;
}

export function itemActors(item: LibraryListItem) {
  return Array.isArray(item.actors) ? item.actors : [];
}

export function getVariantPosterPath(detail: LibraryDetail | null, variantKey: string) {
  const variant = pickVariant(detail, variantKey);
  return variant?.poster_path || variant?.meta.poster_path || detail?.meta.poster_path || detail?.item.poster_path || "";
}

export function getVariantCoverPath(detail: LibraryDetail | null, variantKey: string) {
  const variant = pickVariant(detail, variantKey);
  const candidates = [
    variant?.cover_path,
    variant?.meta.cover_path,
    variant?.meta.fanart_path,
    variant?.meta.thumb_path,
    detail?.meta.cover_path,
    detail?.meta.fanart_path,
    detail?.meta.thumb_path,
    detail?.item.cover_path,
  ];
  return candidates.find((path) => !!path) ?? "";
}

export function hasTranslatedCopy(meta: LibraryMeta | null) {
  if (!meta) {
    return false;
  }
  return Boolean(meta.title_translated.trim() || meta.plot_translated.trim());
}

export function taskPercent(state: TaskState | null) {
  if (!state || state.total <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(100, Math.round((state.processed / state.total) * 100)));
}

export function toMoveToMediaLibraryMessage(error: unknown) {
  const raw = error instanceof Error ? error.message : "启动移动到媒体库失败";
  const text = raw.trim();
  if (text.includes("move to media library is already running")) {
    return "媒体库移动任务已在进行中";
  }
  if (text.includes("media library sync is running")) {
    return "媒体库同步进行中，暂时无法移动";
  }
  if (text.includes("library dir is not configured")) {
    return "未配置媒体库目录";
  }
  if (text.includes("save dir is not configured")) {
    return "未配置保存目录";
  }
  return raw;
}

export function getRefreshButtonLabel(refreshRunning: boolean, refreshCompletedFlash: boolean) {
  if (refreshRunning) return "扫描中...";
  if (refreshCompletedFlash) return "扫描完成";
  return "重新扫描库";
}

export function getMoveButtonLabel(moveBusy: boolean, moveRunning: boolean, moveState: TaskState | null, moveCompletedFlash: boolean) {
  if (moveBusy) {
    if (moveRunning && moveState) return `移动中 ${moveState.processed}/${moveState.total}`;
    return "移动中...";
  }
  if (moveCompletedFlash) return "移动完成";
  return "移动到媒体库";
}

export function markMoveStarting(current: MediaLibraryStatus | null): MediaLibraryStatus | null {
  if (!current) return current;
  return {
    ...current,
    move: {
      ...current.move,
      status: "starting",
      message: "移动到媒体库中",
      updated_at: Date.now(),
      started_at: current.move.started_at || Date.now(),
    },
  };
}

export function markMoveRunning(current: MediaLibraryStatus | null): MediaLibraryStatus | null {
  if (!current) return current;
  return {
    ...current,
    move: {
      ...current.move,
      status: "running",
      message: "移动到媒体库中",
      updated_at: Date.now(),
    },
  };
}

export function handleMoveToMediaLibraryError(
  error: unknown,
  setMessage: (msg: string) => void,
  setMoveProgressVisible: (v: boolean) => void,
  setMediaStatus: (updater: SetStateAction<MediaLibraryStatus | null>) => void,
  observedMoveRunningRef: { current: boolean },
) {
  const msg = toMoveToMediaLibraryMessage(error);
  setMessage(msg);
  if (msg === "媒体库移动任务已在进行中") {
    setMoveProgressVisible(true);
    setMediaStatus(markMoveRunning);
    observedMoveRunningRef.current = true;
  } else {
    setMoveProgressVisible(false);
    void getMediaLibraryStatus().then((next) => setMediaStatus(next)).catch(() => {});
  }
}

export function resolveSelectedPoster(variant: ReturnType<typeof pickVariant>, meta: LibraryMeta, detail: LibraryDetail | null) {
  return variant?.poster_path || variant?.meta.poster_path || meta.poster_path || detail?.item.poster_path || "";
}

export function resolveSelectedCover(variant: ReturnType<typeof pickVariant>, meta: LibraryMeta, detail: LibraryDetail | null) {
  return (
    variant?.cover_path ||
    variant?.meta.cover_path ||
    variant?.meta.fanart_path ||
    variant?.meta.thumb_path ||
    meta.cover_path ||
    meta.fanart_path ||
    detail?.item.cover_path ||
    ""
  );
}

export function getUploadMessage(kind: "poster" | "cover" | "fanart", phase: "start" | "done"): string {
  if (kind === "poster") return phase === "start" ? "替换当前实例海报..." : "当前实例海报已更新";
  if (kind === "cover") return phase === "start" ? "替换当前实例封面..." : "当前实例封面已更新";
  return phase === "start" ? "上传 extrafanart..." : "Extrafanart 已上传";
}

export function pickNextVariantKey(current: string, detail: LibraryDetail): string {
  if (current && detail.variants.some((item) => item.key === current)) return current;
  return detail.primary_variant_key || detail.variants.at(0)?.key || "";
}

export function pickNextCopyMode(current: "translated" | "original", meta: LibraryMeta): "translated" | "original" {
  return current === "translated" && !hasTranslatedCopy(meta) ? "original" : current;
}

export function toErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}

export function getInitialSelectedPath(detail: LibraryDetail | null, items: LibraryListItem[]): string {
  return detail?.item.rel_path ?? items.at(0)?.rel_path ?? "";
}

export function getInitialVariantKey(detail: LibraryDetail | null): string {
  return detail?.primary_variant_key ?? detail?.variants.at(0)?.key ?? "";
}

export function getInitialCopyMode(detail: LibraryDetail | null): "translated" | "original" {
  return hasTranslatedCopy(detail?.meta ?? null) ? "translated" : "original";
}

export function getInitialMessage(items: LibraryListItem[]): string {
  return items.length === 0 ? "当前 savedir 里还没有已入库内容" : "";
}
