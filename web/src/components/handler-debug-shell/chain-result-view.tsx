"use client";

import type { HandlerDebugResult } from "@/lib/api";

export interface ChainResultViewProps {
  result: HandlerDebugResult | null;
}

// ChainResultView: "Chain Result" tab, 按步骤展示链式执行的每一个
// handler 结果. 每一步要么 ok 要么 error, 只有 error 时才显示 summary.
//
// 渲染条件: result 存在且 result.steps 非空. result.steps 为空数组
// 不是 "未运行", 而是 "运行了但没有步骤" (例如 handler 链为空), 两
// 种情况都走同一个引导文案. 原组件的 `result?.steps.length` 三元表
// 达式正好涵盖了 null / undefined / 0 / 正数 四种输入.
export function ChainResultView({ result }: ChainResultViewProps) {
  // Go 的 []DebugStep 零值是 nil, 通过 JSON 会变成 null; `?.` 把 null /
  // undefined / 长度 0 统一归成 "引导文案" 分支, 与原注释的语义一致。
  if (!result?.steps?.length) {
    return <div className="ruleset-debug-empty">运行后会展示链式执行的每一步结果。</div>;
  }
  return (
    <div className="handler-debug-step-list">
      {result.steps.map((step, index) => (
        <article
          key={`${step.handler_id}-${index}`}
          className={`handler-debug-step-card ${step.error ? "handler-debug-step-card-error" : ""}`}
        >
          <div className="handler-debug-step-head">
            <strong>{step.handler_name}</strong>
            <span className={`ruleset-debug-step-badge ${step.error ? "" : "ruleset-debug-step-badge-hit"}`}>
              {step.error ? "error" : "ok"}
            </span>
          </div>
          {step.error ? <p className="ruleset-debug-step-summary">{step.error}</p> : null}
        </article>
      ))}
    </div>
  );
}
