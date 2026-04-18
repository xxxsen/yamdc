"use client";

import { useState, type ReactNode } from "react";

import type {
  PluginEditorRequestDebugResult,
  PluginEditorWorkflowDebugResult,
} from "@/lib/api";

import { BodyPanel } from "./body-panel";
import { HeaderList } from "./header-list";

// workflow-blocks: plugin-editor workflow / request / response 详情的复用
// 展示块.
//
//   RequestDetailBlock / ResponseDetailBlock: Request/Response "Headers +
//     Body" 两件套, 独立调试场景 (RequestDetailPanel / ResponseDetailPanel)
//     和 workflow 单步调试里都会用.
//   WorkflowAccordion: workflow 每一步 (stage) 的可折叠列表, 内部会再用到
//     RequestDetailBlock / ResponseDetailBlock / SelectorDebugPanel /
//     WorkflowItemsPanel.
//   SelectorDebugPanel / WorkflowItemsPanel: workflow step 的 "命中选择器"
//     和 "解析出的 Item" 明细.
//
// 拆出来的目的是让 output-panels.tsx 只负责暴露 5 个顶层 Panel, 不被这批
// 上百行的内部细节淹没.

export function RequestDetailBlock({
  request,
  emptyLabel,
  defaultHeadersOpen = true,
}: {
  request?: PluginEditorRequestDebugResult["request"] | null;
  emptyLabel: string;
  defaultHeadersOpen?: boolean;
}) {
  if (!request) {
    return <div className="ruleset-debug-empty">{emptyLabel}</div>;
  }
  const headerCount = Object.keys(request.headers).length;
  return (
    <div className="plugin-editor-output-detail">
      <details className="plugin-editor-output-detail-block" {...(defaultHeadersOpen ? { open: true } : {})}>
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

export function ResponseDetailBlock({
  response,
  emptyLabel,
  extra,
  defaultHeadersOpen = true,
}: {
  response?: PluginEditorRequestDebugResult["response"] | null;
  emptyLabel: string;
  extra?: ReactNode;
  defaultHeadersOpen?: boolean;
}) {
  if (!response) {
    return <div className="ruleset-debug-empty">{emptyLabel}</div>;
  }
  const contentType = (
    Object.entries(response.headers).find(([k]) => k.toLowerCase() === "content-type")?.[1]?.[0]
  ) ?? "";
  const headerMap = Object.fromEntries(Object.entries(response.headers).map(([key, values]) => [key, values.join(", ")]));
  const headerCount = Object.keys(headerMap).length;
  const body = response.body || response.body_preview;

  return (
    <div className="plugin-editor-output-detail">
      <details className="plugin-editor-output-detail-block" {...(defaultHeadersOpen ? { open: true } : {})}>
        <summary className="plugin-editor-output-detail-summary">
          <span>Headers</span>
          <span className={`plugin-editor-request-json-count ${headerCount > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>{headerCount}</span>
        </summary>
        <HeaderList headers={headerMap} />
      </details>
      <BodyPanel body={body} contentType={contentType} emptyLabel="响应体为空。" />
      {extra}
    </div>
  );
}

function SelectorDebugPanel({ selectors }: { selectors: Record<string, string[]> }) {
  return (
    <div className="plugin-editor-workflow-selector-list">
      {Object.entries(selectors).map(([name, values]) => (
        <div key={name} className="plugin-editor-workflow-debug-card">
          <div className="plugin-editor-workflow-debug-head">
            <strong>{name}</strong>
            <span>{values.length}</span>
          </div>
          <pre className="searcher-debug-json plugin-editor-json-scroll">{JSON.stringify(values, null, 2)}</pre>
        </div>
      ))}
    </div>
  );
}

function WorkflowItemsPanel({ items }: { items: PluginEditorWorkflowDebugResult["steps"][number]["items"] }) {
  return (
    <div className="plugin-editor-workflow-item-list">
      {items?.map((item) => (
        <article key={item.index} className="plugin-editor-workflow-debug-card">
          <div className="plugin-editor-workflow-debug-head">
            <strong>Item #{item.index + 1}</strong>
            <span className={`plugin-editor-workflow-match-chip ${item.matched ? "plugin-editor-workflow-match-chip-hit" : "plugin-editor-workflow-match-chip-miss"}`}>
              {item.matched ? "matched" : "skipped"}
            </span>
          </div>
          <pre className="searcher-debug-json plugin-editor-json-scroll">{JSON.stringify(item.item, null, 2)}</pre>
          {item.item_variables && Object.keys(item.item_variables).length ? (
            <pre className="searcher-debug-json plugin-editor-json-scroll">{JSON.stringify({ item_variables: item.item_variables }, null, 2)}</pre>
          ) : null}
          {item.match_details?.length ? (
            <div className="plugin-editor-workflow-condition-list">
              {item.match_details.map((detail, index) => (
                <div key={`${detail.condition}-${index}`} className="plugin-editor-workflow-condition-row">
                  <span>{detail.condition}</span>
                  <strong>{detail.pass ? "pass" : "fail"}</strong>
                </div>
              ))}
            </div>
          ) : null}
        </article>
      ))}
    </div>
  );
}

export function WorkflowAccordion({ result }: { result: PluginEditorWorkflowDebugResult }) {
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  const toggle = (key: string) => setExpandedKey((prev) => (prev === key ? null : key));

  const [detailOpen, setDetailOpen] = useState<Record<string, Set<string>>>({});
  const isOpen = (stepKey: string, name: string) => {
    const s = detailOpen[stepKey] as Set<string> | undefined;
    return s ? s.has(name) : false;
  };
  const onToggle = (stepKey: string, name: string, open: boolean) => {
    setDetailOpen((prev) => {
      const s = new Set(prev[stepKey]);
      if (open) s.add(name);
      else s.delete(name);
      return { ...prev, [stepKey]: s };
    });
  };

  const stepsJsonKey = "__steps_json__";

  return (
    <div className="wf-accordion">
      {result.steps.map((step, index) => {
        const key = `${step.stage}-${index}`;
        const isExpanded = expandedKey === key;
        return (
          <section key={key} className={`wf-accordion-section${isExpanded ? " wf-accordion-section-expanded" : ""}`}>
            <button type="button" className="wf-accordion-head" onClick={() => toggle(key)}>
              <span className="wf-accordion-arrow">{isExpanded ? "\u25BC" : "\u25B6"}</span>
              <strong>{step.stage}</strong>
              <span className="wf-accordion-summary">{step.summary}</span>
            </button>
            {isExpanded && (
              <div className="wf-accordion-body">
                {step.candidate ? <div className="plugin-editor-timeline-detail">candidate: {step.candidate}</div> : null}
                {step.selected_value ? <div className="plugin-editor-timeline-detail">selected: {step.selected_value}</div> : null}
                {step.items?.length ? <div className="plugin-editor-timeline-detail">matched items: {step.items.filter((item) => item.matched).length}/{step.items.length}</div> : null}
                {step.request ? (
                  <details className="plugin-editor-output-detail-block" open={isOpen(key, "request")} onToggle={(e) => onToggle(key, "request", (e.currentTarget as HTMLDetailsElement).open)}>
                    <summary>Request</summary>
                    <RequestDetailBlock request={step.request} emptyLabel="暂无请求数据。" defaultHeadersOpen={false} />
                  </details>
                ) : null}
                {step.response ? (
                  <details className="plugin-editor-output-detail-block" open={isOpen(key, "response")} onToggle={(e) => onToggle(key, "response", (e.currentTarget as HTMLDetailsElement).open)}>
                    <summary>Response</summary>
                    <ResponseDetailBlock response={step.response} emptyLabel="暂无响应数据。" defaultHeadersOpen={false} />
                  </details>
                ) : null}
                {step.selectors && Object.keys(step.selectors).length ? (
                  <details className="plugin-editor-output-detail-block" open={isOpen(key, "selectors")} onToggle={(e) => onToggle(key, "selectors", (e.currentTarget as HTMLDetailsElement).open)}>
                    <summary>Selectors</summary>
                    <SelectorDebugPanel selectors={step.selectors} />
                  </details>
                ) : null}
                {step.items?.length ? (
                  <details className="plugin-editor-output-detail-block" open={isOpen(key, "items")} onToggle={(e) => onToggle(key, "items", (e.currentTarget as HTMLDetailsElement).open)}>
                    <summary>Matched Items</summary>
                    <WorkflowItemsPanel items={step.items} />
                  </details>
                ) : null}
              </div>
            )}
          </section>
        );
      })}
      {result.error ? (
        <section className="wf-accordion-section wf-accordion-section-error">
          <div className="wf-accordion-head wf-accordion-head-static">
            <strong>error</strong>
            <span className="wf-accordion-summary">{result.error}</span>
          </div>
        </section>
      ) : null}
      <section className={`wf-accordion-section${expandedKey === stepsJsonKey ? " wf-accordion-section-expanded" : ""}`}>
        <button type="button" className="wf-accordion-head" onClick={() => toggle(stepsJsonKey)}>
          <span className="wf-accordion-arrow">{expandedKey === stepsJsonKey ? "\u25BC" : "\u25B6"}</span>
          <strong>Steps JSON</strong>
          <span className="wf-accordion-summary">{result.steps.length} steps</span>
        </button>
        {expandedKey === stepsJsonKey && (
          <div className="wf-accordion-body">
            <pre className="searcher-debug-json plugin-editor-json-scroll">{JSON.stringify(result, null, 2)}</pre>
          </div>
        )}
      </section>
    </div>
  );
}
