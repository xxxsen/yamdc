"use client";

import type { DiffRow } from "@/components/handler-debug-shell/utils";
import type { HandlerDebugResult } from "@/lib/api";

export interface JsonDiffViewProps {
  result: HandlerDebugResult | null;
  diffRows: DiffRow[];
}

// JsonDiffView: "Json Diff" tab 完整内容, 分三种态:
// 1) 未运行 (result == null) -> 引导文案;
// 2) 运行后所有行都是 unchanged -> "当前 handler 没有改动任何字段";
// 3) 有 added/removed/changed 行 -> 双栏 before/after 表.
//
// 为什么 added 行要清空 "before" 侧的 <code>, removed 行要清空
// "after" 侧: LCS 对不齐的行在对立侧没有 line number, 直接渲染
// row.beforeLine / row.afterLine 会显示成 "只在某一侧有内容" 的
// 悬空文本. 显式 "" 保留对齐的空格 (行号列还是会留出来).
export function JsonDiffView({ result, diffRows }: JsonDiffViewProps) {
  if (!result) {
    return <div className="ruleset-debug-empty">运行后会按 JSON 文本展示前后差异。</div>;
  }
  if (!diffRows.some((row) => row.kind !== "unchanged")) {
    return <div className="ruleset-debug-empty">当前 handler 没有改动任何字段。</div>;
  }
  return (
    <div className="handler-debug-code-diff">
      <div className="handler-debug-code-diff-head">
        <div>Before</div>
        <div>After</div>
      </div>
      <div className="handler-debug-code-diff-body">
        {diffRows.map((row, index) => (
          <div
            key={`${row.kind}-${index}`}
            className={`handler-debug-code-diff-row handler-debug-code-diff-row-${row.kind}`}
          >
            <div className="handler-debug-code-diff-side">
              <span className="handler-debug-code-diff-line">{row.beforeLineNumber ?? ""}</span>
              <code>{row.kind === "added" ? "" : row.beforeLine ?? ""}</code>
            </div>
            <div className="handler-debug-code-diff-side">
              <span className="handler-debug-code-diff-line">{row.afterLineNumber ?? ""}</span>
              <code>{row.kind === "removed" ? "" : row.afterLine ?? ""}</code>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
