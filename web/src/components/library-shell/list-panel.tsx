"use client";

import { Search } from "lucide-react";

import { getCardImage, itemActors } from "@/components/library-shell/utils";
import type { LibraryListItem } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

// LibraryListPanel: library-shell 左侧整块 — 标题卡片 + 搜索框 +
// 已入库项列表 + 底部 actions 槽位 (bottomActions prop, 由父组件注入,
// 保持 refresh/move 的业务逻辑留在 LibraryShell 内部, 这里只管布局).
//
// 从 library-shell.tsx 搬迁, JSX 与 class 名完全保持不变, 只是把筛选
// 前 (items) 和筛选实例图片解析 (resolveImage) 作为 props 外提, 以便
// 父层继续持有 assetOverrides / assetVersions 之类的状态.
//
// 详见 td/022-frontend-optimization-roadmap.md §2.2 B-3a.

export interface LibraryListPanelProps {
  items: LibraryListItem[];
  keyword: string;
  onKeywordChange: (value: string) => void;
  selectedPath: string;
  onSelectItem: (path: string) => void;
  resolveImage: (path: string) => string;
  bottomActions: React.ReactNode;
}

export function LibraryListPanel({
  items,
  keyword,
  onKeywordChange,
  selectedPath,
  onSelectItem,
  resolveImage,
  bottomActions,
}: LibraryListPanelProps) {
  return (
    <section className="panel library-list-panel">
      <div className="library-list-head">
        <div className="library-list-kicker">Saved Library</div>
        <h2 className="library-list-title">已入库</h2>
        <p className="library-list-subtitle">浏览 `savedir` 下的媒体目录，并直接修改目录内全部 NFO 的共享元数据。</p>
      </div>

      <label className="file-list-search library-search">
        <Search size={16} />
        <input
          className="input file-list-search-input"
          placeholder="按标题 / 影片 ID / 演员搜索"
          value={keyword}
          onChange={(e) => onKeywordChange(e.target.value)}
        />
      </label>

      <div className="library-item-list">
        {items.map((item) => {
          const imagePath = getCardImage(item);
          return (
            <button
              key={item.rel_path}
              type="button"
              className="library-item-card"
              data-active={selectedPath === item.rel_path}
              onClick={() => onSelectItem(item.rel_path)}
            >
              <div className="library-item-thumb">
                {imagePath ? (
                  <img src={resolveImage(imagePath)} alt={item.title} className="library-thumb-image" />
                ) : (
                  <div className="library-thumb-fallback">{(item.number || item.title || item.name).slice(0, 2).toUpperCase()}</div>
                )}
              </div>
              <div className="library-item-copy">
                <div className="library-item-topline">
                  <span className="library-item-number">{item.number || "未命名影片"}</span>
                  <span className="library-item-time">{formatUnixMillis(item.updated_at)}</span>
                </div>
                {item.conflict ? (
                  <div className="library-item-badge-row">
                    <span className="badge library-conflict-badge">已存在(冲突)</span>
                  </div>
                ) : null}
                <div className="library-item-title" title={item.title || item.name}>{item.title || item.name}</div>
                <div className="library-item-meta">{itemActors(item).length > 0 ? itemActors(item).join(" / ") : "暂无演员信息"}</div>
                <div className="library-item-path">{item.rel_path}</div>
                <div className="library-item-footnote">{item.variant_count > 1 ? `${item.variant_count} 个文件实例` : "单实例目录"}</div>
              </div>
            </button>
          );
        })}
        {items.length === 0 ? <div className="review-empty-state">没有匹配的已入库项目</div> : null}
      </div>

      {bottomActions}
    </section>
  );
}
