"use client";

import type { LibraryMeta } from "@/lib/api";

export interface MediaLibraryDisplayViewProps {
  draftMeta: LibraryMeta;
  // 显示值都是在 parent 里推导好的 "fallback 链末端" 字符串, 不由此组件
  // 再次计算, 避免重复推导语义漂移。例如 displayTitle 可能来自
  // draftMeta.title, 也可能来自 item.name (draftMeta 为空时的兜底), 这
  // 种优先级只在 shell 一处持有.
  displayTitle: string;
  displayTitleSecondary: string;
  displayNumber: string;
  displayPlot: string;
  displayPlotSecondary: string;
}

// MediaLibraryDisplayView: 详情 stage 内 "非编辑态" 下的只读展示视图.
// 与 MediaLibraryFormFields 并列呈现, 两者视觉位置对齐. runtime 分钟
// 显示在这一侧: 0 或 undefined -> "-", 非零值追加 "分钟" 单位.
export function MediaLibraryDisplayView({
  draftMeta,
  displayTitle,
  displayTitleSecondary,
  displayNumber,
  displayPlot,
  displayPlotSecondary,
}: MediaLibraryDisplayViewProps) {
  return (
    <>
      <div className="media-library-hero-main-head">
        <div className="media-library-hero-title-block">
          <div className="media-library-hero-title">{displayTitle}</div>
          {displayTitleSecondary ? <div className="media-library-hero-title-secondary">{displayTitleSecondary}</div> : null}
        </div>
      </div>

      <div className="media-library-hero-facts">
        <div className="media-library-hero-fact"><span>影片 ID</span><strong>{displayNumber}</strong></div>
        <div className="media-library-hero-fact"><span>发行日期</span><strong>{draftMeta.release_date || "-"}</strong></div>
        <div className="media-library-hero-fact"><span>时长</span><strong>{draftMeta.runtime ? `${draftMeta.runtime} 分钟` : "-"}</strong></div>
        <div className="media-library-hero-fact"><span>来源</span><strong>{draftMeta.source || "-"}</strong></div>
        <div className="media-library-hero-fact"><span>导演</span><strong>{draftMeta.director || "-"}</strong></div>
        <div className="media-library-hero-fact"><span>片商</span><strong>{draftMeta.studio || "-"}</strong></div>
        <div className="media-library-hero-fact"><span>发行商</span><strong>{draftMeta.label || "-"}</strong></div>
        <div className="media-library-hero-fact"><span>系列</span><strong>{draftMeta.series || "-"}</strong></div>
      </div>

      <div className="media-library-hero-plot">
        <div>{displayPlot || "暂无简介"}</div>
        {displayPlotSecondary ? <div className="media-library-hero-plot-secondary">{displayPlotSecondary}</div> : null}
      </div>

      <div className="media-library-hero-taxonomy">
        <div className="media-library-hero-taxonomy-row">
          <span className="media-library-hero-taxonomy-label">演员</span>
          <div className="media-library-hero-chip-row">
            {draftMeta.actors.length > 0
              ? draftMeta.actors.map((actor) => <span key={actor} className="token-chip">{actor}</span>)
              : <span className="library-inline-muted">暂无演员</span>}
          </div>
        </div>
        <div className="media-library-hero-taxonomy-row">
          <span className="media-library-hero-taxonomy-label">标签</span>
          <div className="media-library-hero-chip-row">
            {draftMeta.genres.length > 0
              ? draftMeta.genres.map((genre) => <span key={genre} className="token-chip">{genre}</span>)
              : <span className="library-inline-muted">暂无标签</span>}
          </div>
        </div>
      </div>
    </>
  );
}
