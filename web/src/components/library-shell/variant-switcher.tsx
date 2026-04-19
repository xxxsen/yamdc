"use client";

import type { LibraryVariant } from "@/lib/api";

// LibraryVariantSwitcher: library-shell 右侧 detail 面板顶部的 "变体切换"
// chips 面板. 仅在 detail.variants.length > 1 时被父层渲染.
//
// 从 library-shell.tsx 搬迁, 语义和样式 (class 名 .library-variant-*) 零改动.
//
// 详见 td/022-frontend-optimization-roadmap.md §2.2 B-3a.

export interface LibraryVariantSwitcherProps {
  variants: LibraryVariant[];
  currentKey: string;
  onSelect: (key: string) => void;
  // extraClassName: 给外层 panel 追加的 class, 用于不同页面的布局微调.
  // media-library-detail-shell 需要 "media-library-hero-variant-panel"
  // 来放到 hero 右列; library-shell 则不需要.
  extraClassName?: string;
}

export function LibraryVariantSwitcher({
  variants,
  currentKey,
  onSelect,
  extraClassName = "",
}: LibraryVariantSwitcherProps) {
  return (
    <div className={`panel library-variant-panel ${extraClassName}`.trim()}>
      <div className="library-variant-list">
        {variants.map((variant) => (
          <button
            key={variant.key}
            type="button"
            className="library-variant-chip"
            data-active={currentKey === variant.key}
            onClick={() => onSelect(variant.key)}
          >
            <span className="library-variant-chip-title">{variant.label}</span>
            <span className="library-variant-chip-meta">{variant.base_name}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
