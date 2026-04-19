"use client";

import { ChainResultView } from "@/components/handler-debug-shell/chain-result-view";
import { JsonDiffView } from "@/components/handler-debug-shell/json-diff-view";
import {
  PicDiffView,
  type PicDiffState,
} from "@/components/handler-debug-shell/pic-diff-view";
import type { DiffRow } from "@/components/handler-debug-shell/utils";
import type { HandlerDebugResult } from "@/lib/api";

export type ResultTab = "json" | "pic" | "chain";

export interface ResultPanelProps {
  activeTab: ResultTab;
  onTabChange: (next: ResultTab) => void;
  result: HandlerDebugResult | null;
  diffRows: DiffRow[];
  picDiffState: PicDiffState | null;
}

// ResultPanel: 结果区域总容器. 上方 tabs bar (3 个按钮 + error 状态
// 徽标), 下方按 activeTab 分发到对应 view. 三个 view 之间完全独立,
// 所以直接用 switch 分支, 不需要 framer motion 做 tab 切换动画 --
// CSS 已经有 opacity / translate transition.
export function ResultPanel({ activeTab, onTabChange, result, diffRows, picDiffState }: ResultPanelProps) {
  return (
    <div className="handler-debug-results">
      <section className="panel handler-debug-panel">
        <div className="ruleset-debug-panel-head">
          <div className="handler-debug-tabs">
            <button
              type="button"
              className={`handler-debug-tab ${activeTab === "json" ? "handler-debug-tab-active" : ""}`}
              onClick={() => onTabChange("json")}
            >
              Json Diff
            </button>
            <button
              type="button"
              className={`handler-debug-tab ${activeTab === "pic" ? "handler-debug-tab-active" : ""}`}
              onClick={() => onTabChange("pic")}
            >
              Pic Diff
            </button>
            <button
              type="button"
              className={`handler-debug-tab ${activeTab === "chain" ? "handler-debug-tab-active" : ""}`}
              onClick={() => onTabChange("chain")}
            >
              Chain Result
            </button>
          </div>
          {result?.error ? <span className="ruleset-debug-status ruleset-debug-status-no_match">error</span> : null}
        </div>
        {activeTab === "json" ? (
          <JsonDiffView result={result} diffRows={diffRows} />
        ) : activeTab === "pic" ? (
          <PicDiffView result={result} picDiffState={picDiffState} />
        ) : (
          <ChainResultView result={result} />
        )}
      </section>
    </div>
  );
}
