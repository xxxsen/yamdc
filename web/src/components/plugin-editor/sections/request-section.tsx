"use client";

import { Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";

import { WorkflowItemVariablesEditor } from "../kv-pair-editor";
import { handleEditorTextareaKeyDown } from "../plugin-editor-utils";
import { RequestForm } from "../request-form";
import type {
  EditorState,
  KVPairForm,
  RequestFormState,
  WorkflowSelectorForm,
} from "../plugin-editor-types";

// RequestSection: 左侧表单 "Request" 选项卡. 这是整个 shell 里最大的一块,
// 190+ 行 JSX, 包含 3 层折叠区:
//
//   1. Request (RequestForm 主请求)
//   2. Multiple Candidates (开关 + candidates/success_mode/success_conditions)
//   3. Workflow (开关 + data selector / item vars / matcher / next request)
//
// 由于 workflow 子段里又嵌入 WorkflowItemVariablesEditor 和第二个 RequestForm
// (workflowNextRequest), 回调手数较多. 这里 props 列出全部所需 updater,
// 上游 shell 直接把 hook 产的 handler 穿进来.

type WorkflowKVKey = "workflowItemVariables";

export interface RequestSectionProps {
  state: EditorState;
  onPatch: <K extends keyof EditorState>(key: K, value: EditorState[K]) => void;
  onPatchRequest: (key: "request" | "multiRequest" | "workflowNextRequest", updater: (prev: RequestFormState) => RequestFormState) => void;
  onPatchWorkflowSelector: (id: string, updater: (s: WorkflowSelectorForm) => WorkflowSelectorForm) => void;
  onAddWorkflowSelector: () => void;
  onRemoveWorkflowSelector: (id: string) => void;
  onAddKVPair: (key: WorkflowKVKey) => void;
  onRemoveKVPair: (key: WorkflowKVKey, id: string) => void;
  onPatchKVPair: (key: WorkflowKVKey, id: string, updater: (item: KVPairForm) => KVPairForm) => void;
}

export function RequestSection({
  state,
  onPatch,
  onPatchRequest,
  onPatchWorkflowSelector,
  onAddWorkflowSelector,
  onRemoveWorkflowSelector,
  onAddKVPair,
  onRemoveKVPair,
  onPatchKVPair,
}: RequestSectionProps) {
  return (
    <article id="plugin-editor-section-request" className="plugin-editor-panel-fragment plugin-editor-request-shell">
      <div className="plugin-editor-subcard">
        <div className="plugin-editor-subcard-head">
          <strong>Request</strong>
          <span>配置首次请求及其响应判定规则。</span>
        </div>
        <RequestForm
          state={state.request}
          onChange={(updater) => onPatchRequest("request", updater)}
          nextRequestLayout
          compactJSONBlocks
          expandAdvanced
          fetchType={state.fetchType}
        />
      </div>

      <div className="plugin-editor-switch-row">
        <label className="searcher-debug-switch" title="使用多个 candidate 基于当前 Request 重复请求，并按成功条件命中。">
          <input
            type="checkbox"
            checked={state.multiRequestEnabled}
            onChange={(event) => onPatch("multiRequestEnabled", event.target.checked)}
          />
          <span>Multiple Candidates</span>
        </label>
      </div>
      {state.multiRequestEnabled ? (
        <div className="plugin-editor-fields">
          <div className="plugin-editor-subcard">
            <div className="plugin-editor-subcard-head">
              <strong>Multiple Candidates</strong>
              <span>基于当前 request，用多个 candidate 重复请求并按条件命中。</span>
            </div>
            <div className="plugin-editor-form-grid">
              <label className="plugin-editor-field plugin-editor-field-wide">
                <span>Candidates</span>
                <textarea
                  className="input plugin-editor-textarea plugin-editor-textarea-compact"
                  value={state.multiCandidatesText}
                  onChange={(event) => onPatch("multiCandidatesText", event.target.value)}
                  onKeyDown={handleEditorTextareaKeyDown}
                  placeholder={'每行一个 candidate 模板，例如：\n${number}\n${to_upper(${number})}\n${replace(${number}, "-", "_")}\n${replace(${number}, "_", "")}'}
                />
              </label>
              <label className="plugin-editor-field">
                <span>Success Mode</span>
                <select className="input" value={state.multiSuccessMode} onChange={(event) => onPatch("multiSuccessMode", event.target.value)}>
                  <option value="and">and</option>
                  <option value="or">or</option>
                </select>
              </label>
              <label className="plugin-editor-field plugin-editor-field-wide">
                <span>Success Conditions</span>
                <textarea
                  className="input plugin-editor-textarea plugin-editor-textarea-compact"
                  value={state.multiSuccessConditionsText}
                  onChange={(event) => onPatch("multiSuccessConditionsText", event.target.value)}
                  onKeyDown={handleEditorTextareaKeyDown}
                  placeholder={'每行一个条件，例如：\ncontains("${body}", "片名")'}
                />
              </label>
            </div>
          </div>
        </div>
      ) : null}
      <div className="plugin-editor-switch-row">
        <label className="searcher-debug-switch" title="启用 search_select，从首次请求结果中选择目标数据并可进入下一跳请求。">
          <input type="checkbox" checked={state.workflowEnabled} onChange={(event) => onPatch("workflowEnabled", event.target.checked)} />
          <span>Workflow</span>
        </label>
      </div>
      {state.workflowEnabled ? (
        <div className="plugin-editor-workflow-shell">
          <div className="plugin-editor-workflow-scroll">
            <div className="plugin-editor-fields">
              <div className="plugin-editor-subcard">
                <div className="plugin-editor-subcard-head">
                  <strong>Data Selector</strong>
                  <span>从首次请求结果中提取候选数据，供后续匹配使用。</span>
                </div>
                <div className="plugin-editor-fields">
                  {state.workflowSelectors.map((selector) => (
                    <div key={selector.id} className="plugin-editor-transform-card plugin-editor-selector-card">
                      <div className="plugin-editor-transform-actions">
                        <Button
                          className="plugin-editor-transform-action"
                          aria-label="新增 selector"
                          title="新增 selector"
                          onClick={onAddWorkflowSelector}
                        >
                          <Plus size={14} />
                        </Button>
                        <Button
                          className="plugin-editor-transform-action"
                          aria-label="删除 selector"
                          title="删除 selector"
                          onClick={() => onRemoveWorkflowSelector(selector.id)}
                        >
                          <Trash2 size={14} />
                        </Button>
                      </div>
                      <label className="plugin-editor-transform-inline-field plugin-editor-selector-inline-field-name">
                        <span>Name</span>
                        <input className="input" value={selector.name} onChange={(event) => onPatchWorkflowSelector(selector.id, (prev) => ({ ...prev, name: event.target.value }))} />
                      </label>
                      <label className="plugin-editor-transform-inline-field plugin-editor-selector-inline-field-kind">
                        <span>Kind</span>
                        <select className="input" value={selector.kind} onChange={(event) => onPatchWorkflowSelector(selector.id, (prev) => ({ ...prev, kind: event.target.value }))}>
                          <option value="xpath">xpath</option>
                          <option value="jsonpath">jsonpath</option>
                        </select>
                      </label>
                      <label className="plugin-editor-transform-inline-field plugin-editor-selector-inline-field-expr">
                        <span>Expr</span>
                        <input className="input" value={selector.expr} onChange={(event) => onPatchWorkflowSelector(selector.id, (prev) => ({ ...prev, expr: event.target.value }))} />
                      </label>
                    </div>
                  ))}
                </div>
              </div>

              <div className="plugin-editor-subcard">
                <div className="plugin-editor-subcard-head">
                  <strong>Item Variables</strong>
                  <span>定义每个选中项的派生变量。</span>
                </div>
                <WorkflowItemVariablesEditor
                  items={state.workflowItemVariables}
                  onAdd={() => onAddKVPair("workflowItemVariables")}
                  onRemove={(id) => onRemoveKVPair("workflowItemVariables", id)}
                  onChange={(id, updater) => onPatchKVPair("workflowItemVariables", id, updater)}
                />
              </div>

              <div className="plugin-editor-subcard">
                <div className="plugin-editor-subcard-head">
                  <strong>Data Matcher</strong>
                  <span>配置候选数据的匹配方式、数量约束和返回模板。</span>
                </div>
                <div className="plugin-editor-request-inline-row plugin-editor-workflow-inline-row">
                  <label className="plugin-editor-field-inline plugin-editor-workflow-inline-field-sm">
                    <span>Match Mode</span>
                    <select className="input" value={state.workflowMatchMode} onChange={(event) => onPatch("workflowMatchMode", event.target.value)}>
                      <option value="and">and</option>
                      <option value="or">or</option>
                    </select>
                  </label>
                  <label className="plugin-editor-field-inline plugin-editor-workflow-inline-field-sm">
                    <span>Expect Count</span>
                    <input className="input" value={state.workflowExpectCountText} onChange={(event) => onPatch("workflowExpectCountText", event.target.value)} placeholder="可选，例如 1" />
                  </label>
                  <label className="plugin-editor-field-inline plugin-editor-workflow-inline-field-lg">
                    <span>Return Template</span>
                    <input className="input" value={state.workflowReturn} onChange={(event) => onPatch("workflowReturn", event.target.value)} placeholder="${item.read_link}" />
                  </label>
                </div>
                <div className="plugin-editor-form-grid">
                  <label className="plugin-editor-field plugin-editor-field-wide">
                    <span>Match Conditions</span>
                    <textarea
                      className="input plugin-editor-textarea plugin-editor-textarea-compact"
                      value={state.workflowMatchConditionsText}
                      onChange={(event) => onPatch("workflowMatchConditionsText", event.target.value)}
                      onKeyDown={handleEditorTextareaKeyDown}
                      placeholder={'每行一个条件，例如：\ncontains("${item.read_title}", "${number}")'}
                    />
                  </label>
                </div>
              </div>
            </div>
          </div>
          <div className="plugin-editor-subcard">
            <div className="plugin-editor-subcard-head">
              <strong>Next Request</strong>
              <span>配置命中后进入下一跳详情页的请求。</span>
            </div>
            <RequestForm
              state={state.workflowNextRequest}
              onChange={(updater) => onPatchRequest("workflowNextRequest", updater)}
              expandAdvanced
              compactJSONBlocks
              nextRequestLayout
              fetchType={state.fetchType}
            />
          </div>
        </div>
      ) : null}
    </article>
  );
}
