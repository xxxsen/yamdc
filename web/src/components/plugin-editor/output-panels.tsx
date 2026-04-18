"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import type {
  PluginEditorRequestDebugResult,
  PluginEditorScrapeDebugResult,
  PluginEditorWorkflowDebugResult,
} from "@/lib/api";

import { RequestDetailBlock, ResponseDetailBlock, WorkflowAccordion } from "./output-panels/workflow-blocks";
import { runResponseExpr } from "./plugin-editor-utils";

// output-panels: plugin-editor 右侧 "输出" tab 的公开面板集合.
//
// 本文件原本是个 890 行的 "什么都往里塞" 的文件, 拆分后:
//   output-panels/dom-utils.ts         — 纯 DOM/XPath 工具
//   output-panels/header-list.tsx      — 叶子 UI
//   output-panels/dom-tree.tsx         — DOM Inspector 树 + 右键菜单
//   output-panels/body-panel.tsx       — Body 渲染 + 搜索 hook
//   output-panels/workflow-blocks.tsx  — workflow/ request/ response 详情块
//
// 当前文件只保留 5 个顶层面板 + HeaderList 重导出, 负责把上面这些块组合到
// plugin-editor 的各个 tab 上. 不持有业务状态 (除了 ResponseDetailPanel
// 的本地 expr runner).

export { HeaderList } from "./output-panels/header-list";

export function RequestBasicPanel({ result }: { result: PluginEditorRequestDebugResult | null }) {
  if (!result) {
    return <div className="ruleset-debug-empty">先执行一次运行调试。</div>;
  }
  return (
    <div className="plugin-editor-preview-grid">
      <div className="plugin-editor-preview-card">
        <span>Method</span>
        <strong>{result.request.method}</strong>
      </div>
      <div className="plugin-editor-preview-card">
        <span>Status</span>
        <strong>{result.response?.status_code ?? "-"}</strong>
      </div>
      <div className="plugin-editor-preview-card plugin-editor-preview-card-wide">
        <span>URL</span>
        <strong>{result.request.url}</strong>
      </div>
      {result.attempts?.length ? (
        <div className="plugin-editor-preview-card plugin-editor-preview-card-wide">
          <span>Attempts</span>
          <strong>{result.attempts.map((item) => `${item.candidate || "-"}:${item.matched ? "hit" : item.error || "skip"}`).join(" | ")}</strong>
        </div>
      ) : null}
    </div>
  );
}

export function RequestDetailPanel({ request }: { request?: PluginEditorRequestDebugResult["request"] | null }) {
  return <RequestDetailBlock request={request} emptyLabel="暂无请求数据。" />;
}

export function ResponseDetailPanel({ response }: { response?: PluginEditorRequestDebugResult["response"] | null }) {
  const [expr, setExpr] = useState("");
  const [kind, setKind] = useState<"xpath" | "jsonpath">("xpath");
  const [output, setOutput] = useState("");
  const body = response?.body || response?.body_preview || "";
  const contentType = response?.headers["content-type"]?.[0] || response?.headers["Content-Type"]?.[0] || "";

  function runExpr() {
    setOutput(runResponseExpr({ body, expr, kind, contentType }));
  }

  return (
    <ResponseDetailBlock
      response={response}
      emptyLabel="暂无响应数据。"
      extra={
        <details className="plugin-editor-output-detail-block">
          <summary>Expr Filter</summary>
          <div className="plugin-editor-expr-runner plugin-editor-expr-runner-card">
            <div className="plugin-editor-expr-runner-top">
              <input className="input" value={expr} onChange={(event) => setExpr(event.target.value)} placeholder={kind === "xpath" ? '//title/text()' : '$.result.name'} />
              <select className="input plugin-editor-expr-kind" value={kind} onChange={(event) => setKind(event.target.value as "xpath" | "jsonpath")}>
                <option value="xpath">xpath</option>
                <option value="jsonpath">json</option>
              </select>
              <Button variant="primary" onClick={runExpr}>
                Run
              </Button>
            </div>
            <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-expr-output">{output || "暂无结果"}</pre>
          </div>
        </details>
      }
    />
  );
}

export function WorkflowOutputPanel({ result }: { result: PluginEditorWorkflowDebugResult | null }) {
  if (!result) {
    return <div className="ruleset-debug-empty">暂无 workflow 结果。</div>;
  }
  return (
    <div className="plugin-editor-output-section plugin-editor-output-section-fill">
      {result.error ? <div className="plugin-editor-output-error">{result.error}</div> : null}
      <WorkflowAccordion result={result} />
    </div>
  );
}

export function ScrapeJSONPanel({ result }: { result: PluginEditorScrapeDebugResult | null }) {
  if (!result) {
    return <div className="ruleset-debug-empty">暂无抓取结果。</div>;
  }
  return (
    <div className="plugin-editor-output-section plugin-editor-output-section-fill">
      {result.error ? <div className="plugin-editor-output-error">{result.error}</div> : null}
      <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-json-fill">{JSON.stringify(result.meta ?? result.fields, null, 2)}</pre>
    </div>
  );
}
