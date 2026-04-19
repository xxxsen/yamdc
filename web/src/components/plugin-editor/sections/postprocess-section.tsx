"use client";

import { KVPairEditor } from "../kv-pair-editor";
import { FIELD_OPTIONS, META_LANG_OPTIONS } from "../plugin-editor-constants";
import type { EditorState, KVPairForm } from "../plugin-editor-types";

// PostprocessSection: 左侧表单 "Advanced" 选项卡. 负责:
//   - Postprocess Assign: 后处理赋值
//   - Defaults: title/plot/genres/actors 默认语言选择
//   - Switch Config: 两个开关
//
// `META_LANG_OPTIONS` / `FIELD_OPTIONS` 从 plugin-editor-constants 吃进来,
// 保持和其他 section 独立.

type PostAssignKey = "postAssign";

export interface PostprocessSectionProps {
  state: EditorState;
  onPatch: <K extends keyof EditorState>(key: K, value: EditorState[K]) => void;
  onAddKVPair: (key: PostAssignKey) => void;
  onRemoveKVPair: (key: PostAssignKey, id: string) => void;
  onPatchKVPair: (key: PostAssignKey, id: string, updater: (item: KVPairForm) => KVPairForm) => void;
}

export function PostprocessSection({
  state,
  onPatch,
  onAddKVPair,
  onRemoveKVPair,
  onPatchKVPair,
}: PostprocessSectionProps) {
  return (
    <article id="plugin-editor-section-postprocess" className="plugin-editor-panel-fragment">
      <div className="plugin-editor-fields">
        <div className="plugin-editor-subcard">
          <div className="plugin-editor-subcard-head">
            <strong>Postprocess Assign</strong>
            <span>定义后处理阶段的字段赋值表达式。内置变量可直接使用，抓取字段请通过 `meta.xxx` 引用。</span>
          </div>
          <div className="plugin-editor-doc-note">
            <strong>变量规则</strong>
            <span>内置变量可直接使用，例如 `{"${number}"}`、`{"${host}"}`；已抓取字段请使用 `{"${meta.title}"}`、`{"${meta.number}"}`；预检变量请使用 `{"${vars.xxx}"}`。</span>
          </div>
          <KVPairEditor
            items={state.postAssign}
            emptyLabel="暂未定义 assign。"
            keyLabel="Field"
            valueLabel="Expression"
            keyOptions={FIELD_OPTIONS}
            valuePlaceholder="${meta.title} hello world"
            onAdd={() => onAddKVPair("postAssign")}
            onRemove={(id) => onRemoveKVPair("postAssign", id)}
            onChange={(id, updater) => onPatchKVPair("postAssign", id, updater)}
          />
        </div>

        <div className="plugin-editor-subcard">
          <div className="plugin-editor-subcard-head">
            <strong>Defaults</strong>
            <span>设置标题、简介、类型和演员等默认语言。</span>
          </div>
          <div className="plugin-editor-form-grid">
            {(["postTitleLang", "postPlotLang", "postGenresLang", "postActorsLang"] as const).map((key) => {
              const labels: Record<string, string> = {
                postTitleLang: "Title Lang",
                postPlotLang: "Plot Lang",
                postGenresLang: "Genres Lang",
                postActorsLang: "Actors Lang",
              };
              return (
                <label key={key} className="plugin-editor-field">
                  <span>{labels[key]}</span>
                  <select className="input" value={state[key]} onChange={(event) => onPatch(key, event.target.value)}>
                    <option value="">DEFAULT</option>
                    {META_LANG_OPTIONS.map((option) => (
                      <option key={option} value={option}>
                        {option.toUpperCase()}
                      </option>
                    ))}
                  </select>
                </label>
              );
            })}
          </div>
        </div>

        <div className="plugin-editor-subcard">
          <div className="plugin-editor-subcard-head">
            <strong>Switch Config</strong>
            <span>配置后处理阶段的可选开关。</span>
          </div>
          <div className="plugin-editor-fields">
            <label className="searcher-debug-switch">
              <input
                type="checkbox"
                checked={state.postDisableReleaseDateCheck}
                onChange={(event) => onPatch("postDisableReleaseDateCheck", event.target.checked)}
              />
              <span>disable_release_date_check</span>
            </label>
            <label className="searcher-debug-switch">
              <input
                type="checkbox"
                checked={state.postDisableNumberReplace}
                onChange={(event) => onPatch("postDisableNumberReplace", event.target.checked)}
              />
              <span>disable_number_replace</span>
            </label>
          </div>
        </div>
      </div>
    </article>
  );
}
