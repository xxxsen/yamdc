"use client";

import Image from "next/image";

import { getAssetURL, type HandlerDebugResult } from "@/lib/api";

export interface PicDiffState {
  coverChanged: boolean;
  posterChanged: boolean;
  sampleChanged: boolean;
}

export interface PicDiffViewProps {
  result: HandlerDebugResult | null;
  picDiffState: PicDiffState | null;
}

// PicDiffView: "Pic Diff" tab, 比较 before/after 的 cover / poster /
// sample_images 三组图片资源.
//
// 三种态:
// 1) 未运行 -> 引导文案;
// 2) 三类都没变 -> "无图片资源差异";
// 3) 至少有一类变了 -> 渲染三个 section (cover/poster/sample),
//    每个 section 的 badge 单独反映该类是否变化; 没变的那类显示
//    "unchanged" 灰 badge 但仍渲染 before/after 原图, 方便用户肉眼
//    对比 (不是只渲染变化的那类).
//
// cover 和 poster 单图逻辑一模一样, 抽成 SingleImageCompare 避免
// 三倍重复.
interface SingleImageCompareProps {
  label: string;
  changed: boolean;
  beforeKey?: string;
  afterKey?: string;
  beforeAlt: string;
  afterAlt: string;
}

function SingleImageCompare({ label, changed, beforeKey, afterKey, beforeAlt, afterAlt }: SingleImageCompareProps) {
  return (
    <article className="handler-debug-pic-diff-section">
      <div className="handler-debug-pic-diff-head">
        <h4>{label}</h4>
        <span className={`ruleset-debug-step-badge ${changed ? "ruleset-debug-step-badge-hit" : ""}`}>
          {changed ? "changed" : "unchanged"}
        </span>
      </div>
      <div className="handler-debug-pic-diff-compare">
        <div className="handler-debug-pic-slot">
          {beforeKey ? (
            <Image src={getAssetURL(beforeKey)} alt={beforeAlt} width={220} height={320} unoptimized />
          ) : (
            <div className="ruleset-debug-empty">No Image</div>
          )}
        </div>
        <div className="handler-debug-pic-slot">
          {afterKey ? (
            <Image src={getAssetURL(afterKey)} alt={afterAlt} width={220} height={320} unoptimized />
          ) : (
            <div className="ruleset-debug-empty">No Image</div>
          )}
        </div>
      </div>
    </article>
  );
}

export function PicDiffView({ result, picDiffState }: PicDiffViewProps) {
  if (!result) {
    return <div className="ruleset-debug-empty">运行后会按 Before / After 展示图片资源差异。</div>;
  }
  if (!picDiffState || (!picDiffState.coverChanged && !picDiffState.posterChanged && !picDiffState.sampleChanged)) {
    return <div className="ruleset-debug-empty">当前 handler 没有图片资源差异。</div>;
  }

  const beforeSamples = result.before_meta.sample_images ?? [];
  const afterSamples = result.after_meta.sample_images ?? [];

  return (
    <div className="handler-debug-pic-diff">
      <SingleImageCompare
        label="Cover"
        changed={picDiffState.coverChanged}
        beforeKey={result.before_meta.cover?.key}
        afterKey={result.after_meta.cover?.key}
        beforeAlt="before cover"
        afterAlt="after cover"
      />
      <SingleImageCompare
        label="Poster"
        changed={picDiffState.posterChanged}
        beforeKey={result.before_meta.poster?.key}
        afterKey={result.after_meta.poster?.key}
        beforeAlt="before poster"
        afterAlt="after poster"
      />
      <article className="handler-debug-pic-diff-section">
        <div className="handler-debug-pic-diff-head">
          <h4>Sample Images</h4>
          <span className={`ruleset-debug-step-badge ${picDiffState.sampleChanged ? "ruleset-debug-step-badge-hit" : ""}`}>
            {picDiffState.sampleChanged ? "changed" : "unchanged"}
          </span>
        </div>
        <div className="handler-debug-pic-diff-compare">
          <div className="handler-debug-pic-grid">
            {beforeSamples.length ? (
              beforeSamples.map((item) =>
                item.key ? (
                  <Image
                    key={`before-${item.key}`}
                    src={getAssetURL(item.key)}
                    alt={item.name || "before sample"}
                    width={220}
                    height={140}
                    unoptimized
                  />
                ) : null,
              )
            ) : (
              <div className="ruleset-debug-empty">No Image</div>
            )}
          </div>
          <div className="handler-debug-pic-grid">
            {afterSamples.length ? (
              afterSamples.map((item) =>
                item.key ? (
                  <Image
                    key={`after-${item.key}`}
                    src={getAssetURL(item.key)}
                    alt={item.name || "after sample"}
                    width={220}
                    height={140}
                    unoptimized
                  />
                ) : null,
              )
            ) : (
              <div className="ruleset-debug-empty">No Image</div>
            )}
          </div>
        </div>
      </article>
    </div>
  );
}
