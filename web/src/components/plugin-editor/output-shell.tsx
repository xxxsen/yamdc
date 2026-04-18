"use client";

import type {
  PluginEditorCompileResult,
  PluginEditorRequestDebugResult,
  PluginEditorScrapeDebugResult,
  PluginEditorWorkflowDebugResult,
} from "@/lib/api";

import {
  RequestBasicPanel,
  RequestDetailPanel,
  ResponseDetailPanel,
  ScrapeJSONPanel,
  WorkflowOutputPanel,
} from "./output-panels";
import type { EditorTab } from "./plugin-editor-types";

// OutputShell: plugin-editor 右侧 "调试输出" 面板 — tab 切换 + 7 种 tab
// 的对应渲染分支 (basic / request / response / workflow / scrape / compile /
// draft). 纯展示, 不持有业务态 (tab 切换本身也由上游托管, 便于和 run()
// 完成后的 setTab 联动).

const OUTPUT_TABS: Array<[EditorTab, string]> = [
  ["basic", "Basic"],
  ["request", "Request"],
  ["response", "Response"],
  ["workflow", "Workflow"],
  ["scrape", "Scrape"],
  ["compile", "Compile"],
  ["draft", "Draft"],
];

export interface OutputShellProps {
  tab: EditorTab;
  onTabChange: (tab: EditorTab) => void;
  error: string;
  compileResult: PluginEditorCompileResult | null;
  requestResult: PluginEditorRequestDebugResult | null;
  workflowResult: PluginEditorWorkflowDebugResult | null;
  scrapeResult: PluginEditorScrapeDebugResult | null;
  draftPreview: string;
}

export function OutputShell({
  tab,
  onTabChange,
  error,
  compileResult,
  requestResult,
  workflowResult,
  scrapeResult,
  draftPreview,
}: OutputShellProps) {
  return (
    <article className="panel plugin-editor-panel">
      <div className="plugin-editor-panel-head">
        <h3>调试输出</h3>
        <span>{tab}</span>
      </div>
      <div className="plugin-editor-tabs">
        {OUTPUT_TABS.map(([key, label]) => (
          <button
            key={key}
            className={`handler-debug-tab ${tab === key ? "handler-debug-tab-active" : ""}`}
            type="button"
            onClick={() => onTabChange(key)}
          >
            {label}
          </button>
        ))}
      </div>
      {error ? <div className="ruleset-debug-error">{error}</div> : null}

      {tab === "compile" ? (
        <div className="plugin-editor-output-section plugin-editor-output-section-fill">
          <div className="plugin-editor-summary-grid">
            <div className="plugin-editor-summary-card">
              <span>request</span>
              <strong>{compileResult?.summary.has_request ? "yes" : "no"}</strong>
            </div>
            <div className="plugin-editor-summary-card">
              <span>multi_request</span>
              <strong>{compileResult?.summary.has_multi_request ? "yes" : "no"}</strong>
            </div>
            <div className="plugin-editor-summary-card">
              <span>workflow</span>
              <strong>{compileResult?.summary.has_workflow ? "yes" : "no"}</strong>
            </div>
            <div className="plugin-editor-summary-card">
              <span>fields</span>
              <strong>{compileResult?.summary.field_count ?? 0}</strong>
            </div>
          </div>
          <div className="plugin-editor-output-detail-block plugin-editor-output-fill-block">
            <div className="plugin-editor-output-block-title">YAML 输出</div>
            <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-json-fill">{compileResult?.yaml || "先执行一次编译。"}</pre>
          </div>
        </div>
      ) : null}

      {tab === "basic" ? (
        <div className="plugin-editor-output-section plugin-editor-output-section-fill">
          <RequestBasicPanel result={requestResult} />
        </div>
      ) : null}

      {tab === "request" ? (
        <div className="plugin-editor-output-section plugin-editor-output-section-fill">
          <RequestDetailPanel request={requestResult?.request} />
        </div>
      ) : null}

      {tab === "response" ? (
        <div className="plugin-editor-output-section plugin-editor-output-section-fill">
          <ResponseDetailPanel response={requestResult?.response} />
        </div>
      ) : null}

      {tab === "workflow" ? (
        <div className="plugin-editor-output-section plugin-editor-output-section-fill">
          <WorkflowOutputPanel result={workflowResult} />
        </div>
      ) : null}

      {tab === "scrape" ? (
        <div className="plugin-editor-output-section plugin-editor-output-section-fill">
          <ScrapeJSONPanel result={scrapeResult} />
        </div>
      ) : null}

      {tab === "draft" ? (
        <div className="plugin-editor-output-section plugin-editor-output-section-fill">
          <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-json-fill">{draftPreview || "当前草稿无效。"}</pre>
        </div>
      ) : null}
    </article>
  );
}
