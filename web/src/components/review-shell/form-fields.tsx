"use client";

import type { ReviewMeta } from "@/lib/api";

export interface ReviewFormFieldsProps {
  meta: ReviewMeta;
  updateMeta: (patch: Partial<ReviewMeta>) => void;
  onBlurSave: () => void;
}

export function ReviewFormFields({ meta, updateMeta, onBlurSave }: ReviewFormFieldsProps) {
  return (
    <div className="review-top-fields">
      <div className="review-field">
        <span className="review-label review-label-side">标题</span>
        <input
          className="input review-input-strong"
          value={meta.title ?? ""}
          onChange={(e) => updateMeta({ title: e.target.value })}
          onBlur={onBlurSave}
        />
      </div>
      <div className="review-field">
        <span className="review-label review-label-side">翻译标题</span>
        <input
          className="input"
          value={meta.title_translated ?? ""}
          onChange={(e) => updateMeta({ title_translated: e.target.value })}
          onBlur={onBlurSave}
        />
      </div>
      <div className="review-meta-row review-meta-row-2 review-meta-row-top">
        <div className="review-field">
          <span className="review-label review-label-side">导演</span>
          <input className="input" value={meta.director ?? ""} onChange={(e) => updateMeta({ director: e.target.value })} onBlur={onBlurSave} />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">制作商</span>
          <input className="input" value={meta.studio ?? ""} onChange={(e) => updateMeta({ studio: e.target.value })} onBlur={onBlurSave} />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">发行商</span>
          <input className="input" value={meta.label ?? ""} onChange={(e) => updateMeta({ label: e.target.value })} onBlur={onBlurSave} />
        </div>
        <div className="review-field">
          <span className="review-label review-label-side">系列</span>
          <input className="input" value={meta.series ?? ""} onChange={(e) => updateMeta({ series: e.target.value })} onBlur={onBlurSave} />
        </div>
      </div>
      <div className="review-meta-row review-meta-row-2">
        <div className="review-field review-field-area">
          <span className="review-label review-label-side">简介</span>
          <textarea className="input review-textarea" value={meta.plot ?? ""} onChange={(e) => updateMeta({ plot: e.target.value })} onBlur={onBlurSave} />
        </div>
        <div className="review-field review-field-area">
          <span className="review-label review-label-side">翻译简介</span>
          <textarea
            className="input review-textarea"
            value={meta.plot_translated ?? ""}
            onChange={(e) => updateMeta({ plot_translated: e.target.value })}
            onBlur={onBlurSave}
          />
        </div>
      </div>
    </div>
  );
}
