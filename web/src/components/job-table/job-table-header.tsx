import { RefreshCw, Search } from "lucide-react";
import type { ChangeEvent } from "react";

import { Button } from "@/components/ui/button";

// JobTableHeader 把原来混在主 JSX 里的 hero 区 / summary cards / toolbar /
// filter chips 打包成单一视觉块. 它只做受控渲染, 所有可变状态由父层持有并
// 通过 props 下发, 便于父组件继续做 useTransition / 轮询等编排逻辑.
export interface SummaryCard {
  label: string;
  value: number;
  hint: string;
  tone: "default" | "info" | "warn" | "danger";
  filter: string;
}

export interface FilterChip {
  value: string;
  label: string;
  count: number;
}

interface Props {
  jobsCount: number;
  total: number;
  keyword: string;
  statusFilter: string;
  isPending: boolean;
  isScanning: boolean;
  summaryCards: readonly SummaryCard[];
  filterChips: readonly FilterChip[];
  onKeywordChange: (value: string) => void;
  onStatusFilterChange: (filter: string) => void;
  onScan: () => void;
}

export function JobTableHeader({
  jobsCount,
  total,
  keyword,
  statusFilter,
  isPending,
  isScanning,
  summaryCards,
  filterChips,
  onKeywordChange,
  onStatusFilterChange,
  onScan,
}: Props) {
  return (
    <>
      <div className="file-list-hero">
        <div className="file-list-hero-copy">
          <div className="file-list-eyebrow">Processing Queue</div>
          <h2 className="file-list-title">文件列表</h2>
          <p className="file-list-subtitle">
            当前展示 {jobsCount} 条记录，共 {total} 条任务。优先处理低置信度影片 ID，运行中的状态会自动刷新。
          </p>
        </div>
        <div className="file-list-stats">
          {summaryCards.map((item) => (
            <button
              key={item.label}
              type="button"
              className="file-list-stat-card"
              data-tone={item.tone}
              data-active={statusFilter === item.filter}
              onClick={() => onStatusFilterChange(item.filter)}
            >
              <span className="file-list-stat-label">{item.label}</span>
              <strong className="file-list-stat-value">{item.value}</strong>
              <span className="file-list-stat-hint">{item.hint}</span>
            </button>
          ))}
        </div>
      </div>

      <div className="file-list-toolbar">
        <label className="file-list-search">
          <Search size={16} />
          <input
            className="input file-list-search-input"
            placeholder="按文件名 / 路径 / 影片 ID 搜索"
            value={keyword}
            onChange={(e: ChangeEvent<HTMLInputElement>) => onKeywordChange(e.target.value)}
          />
        </label>
        <div className="file-list-toolbar-actions">
          <Button
            variant="primary"
            onClick={onScan}
            disabled={isPending || isScanning}
            leftIcon={
              <RefreshCw size={16} className={isScanning ? "media-library-sync-icon-spinning" : ""} />
            }
          >
            {isScanning ? "扫描中..." : "立即扫描"}
          </Button>
        </div>
      </div>

      <div className="file-list-chip-row" aria-label="状态快捷筛选">
        {filterChips.map((item) => (
          <button
            key={item.value}
            type="button"
            className="file-list-chip"
            data-active={statusFilter === item.value}
            onClick={() => onStatusFilterChange(item.value)}
          >
            <span>{item.label}</span>
            <span className="file-list-chip-count">{item.count}</span>
          </button>
        ))}
      </div>
    </>
  );
}
