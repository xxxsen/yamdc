"use client";

import { type SetStateAction } from "react";

import { TokenEditor } from "@/components/ui/token-editor";
import type { LibraryMeta } from "@/lib/api";

export interface MediaLibraryFormFieldsProps {
  draftMeta: LibraryMeta;
  updateDraftMeta: (updater: SetStateAction<LibraryMeta>) => void;
}

// MediaLibraryFormFields: 详情 stage 内部 "编辑模式" 下的整个字段网格.
// 与 review-shell / library-shell 的 form-fields 语义对齐, 但使用
// media-library-fields-grid + media-library-inline-editor 这一套专门
//为 stage 行内编辑器设计的布局类名 (单列窄, 2 列跨度用 field-span-2
// 标记). 字段顺序与只读态 display-view 对齐, 以便用户在切换模式时
// 视觉位置不跳跃.
//
// runtime 字段做 number <-> string 转换: 空输入 / 非法输入 -> 0;
// 空字符串而非 "0" 被渲染, 避免 "0 分钟" 这样的误导显示.
export function MediaLibraryFormFields({ draftMeta, updateDraftMeta }: MediaLibraryFormFieldsProps) {
  const bind =
    (key: keyof LibraryMeta) =>
    (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      const value = event.target.value;
      updateDraftMeta((prev) => ({ ...prev, [key]: value }));
    };

  const setActors = (next: string[]) => {
    updateDraftMeta((prev) => ({ ...prev, actors: next }));
  };
  const setGenres = (next: string[]) => {
    updateDraftMeta((prev) => ({ ...prev, genres: next }));
  };

  return (
    <div className="media-library-fields-grid media-library-inline-editor">
      <div className="review-field media-library-field-span-2">
        <span className="review-label review-label-side">原始标题</span>
        <input className="input review-input-strong" value={draftMeta.title} onChange={bind("title")} />
      </div>
      <div className="review-field media-library-field-span-2">
        <span className="review-label review-label-side">翻译标题</span>
        <input className="input" value={draftMeta.title_translated} onChange={bind("title_translated")} />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">影片 ID</span>
        <input className="input" value={draftMeta.number} onChange={bind("number")} />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">发行日期</span>
        <input className="input" placeholder="YYYY-MM-DD" value={draftMeta.release_date} onChange={bind("release_date")} />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">时长</span>
        <input
          className="input"
          inputMode="numeric"
          value={draftMeta.runtime ? String(draftMeta.runtime) : ""}
          onChange={(e) =>
            updateDraftMeta((prev) => ({
              ...prev,
              runtime: Number.parseInt(e.target.value || "0", 10) || 0,
            }))
          }
        />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">来源</span>
        <input className="input" value={draftMeta.source} onChange={bind("source")} />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">导演</span>
        <input className="input" value={draftMeta.director} onChange={bind("director")} />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">片商</span>
        <input className="input" value={draftMeta.studio} onChange={bind("studio")} />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">发行商</span>
        <input className="input" value={draftMeta.label} onChange={bind("label")} />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">系列</span>
        <input className="input" value={draftMeta.series} onChange={bind("series")} />
      </div>
      <div className="review-field review-field-area media-library-inline-plot-field">
        <span className="review-label review-label-side">原始简介</span>
        <textarea className="input review-textarea library-textarea media-library-inline-plot-textarea" value={draftMeta.plot} onChange={bind("plot")} />
      </div>
      <div className="review-field review-field-area media-library-inline-plot-field">
        <span className="review-label review-label-side">翻译简介</span>
        <textarea className="input review-textarea library-textarea media-library-inline-plot-textarea" value={draftMeta.plot_translated} onChange={bind("plot_translated")} />
      </div>
      <div className="media-library-field-span-2">
        <TokenEditor
          idPrefix="media-library-token"
          label="演员"
          placeholder="输入后回车或逗号确认"
          value={draftMeta.actors}
          onChange={setActors}
          singleLine
          readOnly={false}
        />
      </div>
      <div className="media-library-field-span-2">
        <TokenEditor
          idPrefix="media-library-token"
          label="标签"
          placeholder="输入后回车或逗号确认"
          value={draftMeta.genres}
          onChange={setGenres}
          readOnly={false}
        />
      </div>
    </div>
  );
}
