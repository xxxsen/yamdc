"use client";

import { WorkflowItemVariablesEditor } from "../kv-pair-editor";
import { handleEditorTextareaKeyDown } from "../plugin-editor-utils";
import type { EditorState, KVPairForm } from "../plugin-editor-types";

// BasicSection: plugin-editor 左侧表单 "Basic" 选项卡的 article.
//
// 拆出动机: 主 shell 单纯"粘 JSX"就吃掉 75 行, 归类到本文件后主 shell
// 只负责路由 (activeSection 切换). 这里只读 state + 回调上游 updater,
// 不持有本地状态.

type PrecheckKVKey = "precheckVariables";

export interface BasicSectionProps {
  state: EditorState;
  onPatch: <K extends keyof EditorState>(key: K, value: EditorState[K]) => void;
  onAddKVPair: (key: PrecheckKVKey) => void;
  onRemoveKVPair: (key: PrecheckKVKey, id: string) => void;
  onPatchKVPair: (key: PrecheckKVKey, id: string, updater: (item: KVPairForm) => KVPairForm) => void;
}

export function BasicSection({ state, onPatch, onAddKVPair, onRemoveKVPair, onPatchKVPair }: BasicSectionProps) {
  return (
    <article id="plugin-editor-section-basic" className="plugin-editor-panel-fragment">
      <div className="plugin-editor-fields">
        <div className="plugin-editor-subcard">
          <div className="plugin-editor-subcard-head">
            <strong>Plugin</strong>
            <span>配置插件基础信息、Host 和预检规则。</span>
          </div>
          <div className="plugin-editor-form-grid plugin-editor-form-grid-compact">
            <label className="plugin-editor-field">
              <span>Plugin Name</span>
              <input className="input" value={state.name} onChange={(event) => onPatch("name", event.target.value)} />
            </label>
            <label className="plugin-editor-field">
              <span>Type</span>
              <select className="input" value={state.type} onChange={(event) => onPatch("type", event.target.value)}>
                <option value="one-step">one-step</option>
                <option value="two-step">two-step</option>
              </select>
            </label>
            <label className="plugin-editor-field">
              <span>Fetch Type</span>
              <select className="input" value={state.fetchType} onChange={(event) => onPatch("fetchType", event.target.value)}>
                <option value="go-http">go-http</option>
                <option value="browser">browser</option>
                <option value="flaresolverr">flaresolverr</option>
              </select>
            </label>
          </div>
          <div className="plugin-editor-form-grid">
            <label className="plugin-editor-field plugin-editor-field-wide">
              <span>Hosts</span>
              <textarea
                className="input plugin-editor-textarea plugin-editor-textarea-compact"
                value={state.hostsText}
                onChange={(event) => onPatch("hostsText", event.target.value)}
                onKeyDown={handleEditorTextareaKeyDown}
                placeholder="每行一个 host"
              />
            </label>
            <label className="plugin-editor-field plugin-editor-field-wide">
              <span>Precheck Patterns</span>
              <textarea
                className="input plugin-editor-textarea plugin-editor-textarea-compact"
                value={state.precheckPatternsText}
                onChange={(event) => onPatch("precheckPatternsText", event.target.value)}
                onKeyDown={handleEditorTextareaKeyDown}
                placeholder="每行一个正则"
              />
            </label>
            <label className="plugin-editor-field plugin-editor-field-wide">
              <span>Test Number</span>
              <input className="input" value={state.number} onChange={(event) => onPatch("number", event.target.value)} />
            </label>
          </div>
        </div>
        <div className="plugin-editor-subcard">
          <div className="plugin-editor-subcard-head">
            <strong>Precheck Variables</strong>
            <span>定义预检阶段可复用的变量，后续可通过 `vars.xxx` 引用。</span>
          </div>
          <WorkflowItemVariablesEditor
            items={state.precheckVariables}
            onAdd={() => onAddKVPair("precheckVariables")}
            onRemove={(id) => onRemoveKVPair("precheckVariables", id)}
            onChange={(id, updater) => onPatchKVPair("precheckVariables", id, updater)}
            keyLabel="Name"
            valueLabel="Expression"
            valuePlaceholder='${clean_number(${number})}'
            emptyLabel="暂未定义 precheck variables。"
          />
        </div>
      </div>
    </article>
  );
}
