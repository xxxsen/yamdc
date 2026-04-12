"use client";

import { useState } from "react";

import type {
  PluginEditorRequestDebugResult,
  PluginEditorScrapeDebugResult,
  PluginEditorWorkflowDebugResult,
} from "@/lib/api";

import { runResponseExpr } from "./plugin-editor-utils";

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
  if (!request) {
    return <div className="ruleset-debug-empty">暂无请求数据。</div>;
  }
  const headerCount = Object.keys(request.headers).length;
  return (
    <div className="plugin-editor-output-detail">
      <details className="plugin-editor-output-detail-block">
        <summary className="plugin-editor-output-detail-summary">
          <span>Headers</span>
          <span className={`plugin-editor-request-json-count ${headerCount > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>{headerCount}</span>
        </summary>
        <HeaderList headers={request.headers} />
      </details>
      <BodyPanel body={request.body} contentType={request.headers["Content-Type"] || request.headers["content-type"] || ""} emptyLabel="请求体为空。" />
    </div>
  );
}

export function ResponseDetailPanel({ response }: { response?: PluginEditorRequestDebugResult["response"] | null }) {
  const [expr, setExpr] = useState("");
  const [kind, setKind] = useState<"xpath" | "jsonpath">("xpath");
  const [output, setOutput] = useState("");
  if (!response) {
    return <div className="ruleset-debug-empty">暂无响应数据。</div>;
  }
  const contentType = response.headers["content-type"]?.[0] || response.headers["Content-Type"]?.[0] || "";
  const headerMap = Object.fromEntries(Object.entries(response.headers).map(([key, values]) => [key, values.join(", ")]));
  const headerCount = Object.keys(headerMap).length;
  const body = response.body || response.body_preview;

  function runExpr() {
    setOutput(runResponseExpr({ body, expr, kind, contentType }));
  }

  return (
    <div className="plugin-editor-output-detail">
      <details className="plugin-editor-output-detail-block">
        <summary className="plugin-editor-output-detail-summary">
          <span>Headers</span>
          <span className={`plugin-editor-request-json-count ${headerCount > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>{headerCount}</span>
        </summary>
        <HeaderList headers={headerMap} />
      </details>
      <BodyPanel body={body} contentType={contentType} emptyLabel="响应体为空。" />
      <details className="plugin-editor-output-detail-block">
        <summary>Expr Filter</summary>
        <div className="plugin-editor-expr-runner plugin-editor-expr-runner-card">
          <div className="plugin-editor-expr-runner-top">
            <input className="input" value={expr} onChange={(event) => setExpr(event.target.value)} placeholder={kind === "xpath" ? '//title/text()' : '$.result.name'} />
            <select className="input plugin-editor-expr-kind" value={kind} onChange={(event) => setKind(event.target.value as "xpath" | "jsonpath")}>
              <option value="xpath">xpath</option>
              <option value="jsonpath">json</option>
            </select>
            <button className="btn btn-primary" type="button" onClick={runExpr}>
              Run
            </button>
          </div>
          <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-expr-output">{output || "暂无结果"}</pre>
        </div>
      </details>
    </div>
  );
}

export function WorkflowOutputPanel({ result }: { result: PluginEditorWorkflowDebugResult | null }) {
  if (!result) {
    return <div className="ruleset-debug-empty">暂无 workflow 结果。</div>;
  }
  return (
    <div className="plugin-editor-output-section plugin-editor-output-section-fill">
      <WorkflowDebugPreview result={result} />
      <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-json-fill">{JSON.stringify(result, null, 2)}</pre>
    </div>
  );
}

function WorkflowDebugPreview({ result }: { result: PluginEditorWorkflowDebugResult }) {
  return (
    <div className="plugin-editor-timeline">
      {result.steps.map((step, index) => (
        <article key={`${step.stage}-${index}`} className="plugin-editor-timeline-step">
          <div className="plugin-editor-timeline-head">
            <strong>{step.stage}</strong>
            <span>{step.summary}</span>
          </div>
          {step.candidate ? <div className="plugin-editor-timeline-detail">candidate: {step.candidate}</div> : null}
          {step.selected_value ? <div className="plugin-editor-timeline-detail">selected: {step.selected_value}</div> : null}
          {step.items?.length ? <div className="plugin-editor-timeline-detail">matched items: {step.items.filter((item) => item.matched).length}/{step.items.length}</div> : null}
        </article>
      ))}
      {result.error ? (
        <article className="plugin-editor-timeline-step plugin-editor-timeline-step-error">
          <div className="plugin-editor-timeline-head">
            <strong>error</strong>
            <span>{result.error}</span>
          </div>
        </article>
      ) : null}
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

export function HeaderList({ headers }: { headers: Record<string, string> }) {
  const entries = Object.entries(headers);
  if (entries.length === 0) {
    return <div className="ruleset-debug-empty">Header 为空。</div>;
  }
  return (
    <div className="plugin-editor-header-list">
      {entries.map(([key, value]) => (
        <div key={key} className="plugin-editor-header-row">
          <span>{key}</span>
          <strong>{value || "-"}</strong>
        </div>
      ))}
    </div>
  );
}

function BodyPanel(props: { body: string; contentType: string; emptyLabel: string }) {
  if (!props.body) {
    return <div className="ruleset-debug-empty">{props.emptyLabel}</div>;
  }
  const contentType = props.contentType.toLowerCase();
  if (contentType.includes("application/json")) {
    let formatted = props.body;
    try {
      formatted = JSON.stringify(JSON.parse(props.body), null, 2);
    } catch {
      formatted = props.body;
    }
    return (
      <div className="plugin-editor-body-panel">
        <pre className="searcher-debug-json">{formatted}</pre>
      </div>
    );
  }
  if (contentType.includes("application/x-www-form-urlencoded")) {
    const params = new URLSearchParams(props.body);
    const headers = Object.fromEntries(params.entries());
    return <HeaderList headers={headers} />;
  }
  return (
    <div className="plugin-editor-body-panel">
      <pre className="searcher-debug-json">{props.body}</pre>
    </div>
  );
}
