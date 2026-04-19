"use client";

import type { SetStateAction } from "react";

import type { LibraryMeta } from "@/lib/api";

export interface LibraryFormFieldsProps {
  draftMeta: LibraryMeta;
  copyMode: "translated" | "original";
  activeTitleValue: string;
  activePlotValue: string;
  updateDraftMeta: (updater: SetStateAction<LibraryMeta>) => void;
  onBlurSave: () => void;
}

export function LibraryFormFields({
  draftMeta,
  copyMode,
  activeTitleValue,
  activePlotValue,
  updateDraftMeta,
  onBlurSave,
}: LibraryFormFieldsProps) {
  return (
    <div className="review-top-fields">
      <div className="review-field">
        <span className="review-label review-label-side">标题</span>
        <input
          className="input review-input-strong"
          placeholder={copyMode === "translated" ? draftMeta.title || "暂无中文标题" : "输入原始标题"}
          value={activeTitleValue}
          onChange={(e) =>
            updateDraftMeta((prev) => ({
              ...prev,
              [copyMode === "translated" ? "title_translated" : "title"]: e.target.value,
            }))
          }
          onBlur={onBlurSave}
        />
      </div>
      <div className="review-meta-row review-meta-row-2 review-meta-row-top">
        <div className="review-field">
          <span className="review-label review-label-side">导演</span>
          <input
            className="input"
            value={draftMeta.director}
            onChange={(e) => updateDraftMeta((prev) => ({ ...prev, director: e.target.value }))}
            onBlur={onBlurSave}
          />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">片商</span>
          <input
            className="input"
            value={draftMeta.studio}
            onChange={(e) => updateDraftMeta((prev) => ({ ...prev, studio: e.target.value }))}
            onBlur={onBlurSave}
          />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">发行商</span>
          <input
            className="input"
            value={draftMeta.label}
            onChange={(e) => updateDraftMeta((prev) => ({ ...prev, label: e.target.value }))}
            onBlur={onBlurSave}
          />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">系列</span>
          <input
            className="input"
            value={draftMeta.series}
            onChange={(e) => updateDraftMeta((prev) => ({ ...prev, series: e.target.value }))}
            onBlur={onBlurSave}
          />
        </div>
      </div>
      <div className="review-meta-row review-meta-row-2 library-meta-grid">
        <div className="review-field">
          <span className="review-label review-label-side">影片 ID</span>
          <input
            className="input"
            value={draftMeta.number}
            onChange={(e) => updateDraftMeta((prev) => ({ ...prev, number: e.target.value }))}
            onBlur={onBlurSave}
          />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">发行日期</span>
          <input
            className="input"
            placeholder="YYYY-MM-DD"
            value={draftMeta.release_date}
            onChange={(e) => updateDraftMeta((prev) => ({ ...prev, release_date: e.target.value }))}
            onBlur={onBlurSave}
          />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">时长</span>
          <input
            className="input"
            inputMode="numeric"
            value={draftMeta.runtime ? String(draftMeta.runtime) : ""}
            onChange={(e) =>
              updateDraftMeta((prev) => ({ ...prev, runtime: Number.parseInt(e.target.value || "0", 10) || 0 }))
            }
            onBlur={onBlurSave}
          />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">来源</span>
          <input
            className="input"
            value={draftMeta.source}
            onChange={(e) => updateDraftMeta((prev) => ({ ...prev, source: e.target.value }))}
            onBlur={onBlurSave}
          />
        </div>
      </div>
      <div className="review-meta-row">
        <div className="review-field review-field-area">
          <span className="review-label review-label-side">简介</span>
          <textarea
            className="input review-textarea library-textarea"
            placeholder={copyMode === "translated" ? draftMeta.plot || "暂无中文简介" : "输入原始简介"}
            value={activePlotValue}
            onChange={(e) =>
              updateDraftMeta((prev) => ({
                ...prev,
                [copyMode === "translated" ? "plot_translated" : "plot"]: e.target.value,
              }))
            }
            onBlur={onBlurSave}
          />
        </div>
      </div>
    </div>
  );
}
