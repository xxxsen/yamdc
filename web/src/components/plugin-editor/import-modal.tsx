"use client";

import { FileCode2, Import, LoaderCircle, X } from "lucide-react";

import { handleEditorTextareaKeyDown } from "./plugin-editor-utils";
import { IMPORT_YAML_EXAMPLE } from "./plugin-editor-constants";

export function ImportModal(props: {
  importYAML: string;
  onImportYAMLChange: (value: string) => void;
  onImport: () => void;
  onClose: () => void;
  onShowExample: () => void;
  busyAction: string;
}) {
  return (
    <div className="plugin-editor-modal-backdrop" role="presentation" onClick={props.onClose}>
      <div className="panel plugin-editor-modal" role="dialog" aria-modal="true" aria-label="导入 YAML" onClick={(event) => event.stopPropagation()}>
        <div className="plugin-editor-modal-head">
          <div className="plugin-editor-modal-title-group">
            <div className="plugin-editor-modal-badge">
              <Import size={16} />
              <span>Import</span>
            </div>
            <div className="plugin-editor-modal-title-copy">
              <h3>导入 YAML</h3>
              <span>粘贴已有插件配置，并将内容回填到当前编辑器表单。</span>
            </div>
          </div>
          <button className="btn btn-secondary plugin-editor-modal-close" type="button" aria-label="关闭导入窗口" title="关闭导入窗口" onClick={props.onClose}>
            <X size={16} />
          </button>
        </div>
        <div className="plugin-editor-modal-body">
          <div className="plugin-editor-modal-tip">
            <strong>支持内容</strong>
            <span>支持直接粘贴完整插件 YAML。导入后会覆盖当前表单内容。</span>
          </div>
          <div className="plugin-editor-modal-example">
            <div className="plugin-editor-modal-example-copy">
              <strong>参考结构</strong>
              <span>查看一份最小可用的 YAML 示例，方便直接按结构粘贴或修改。</span>
            </div>
            <button className="btn btn-secondary plugin-editor-modal-example-btn" type="button" onClick={props.onShowExample}>
              查看 YAML 示例
            </button>
          </div>
          <label className="plugin-editor-field plugin-editor-modal-editor">
            <span>Plugin YAML</span>
            <textarea
              className="input plugin-editor-textarea plugin-editor-textarea-lg plugin-editor-modal-textarea"
              value={props.importYAML}
              onChange={(event) => props.onImportYAMLChange(event.target.value)}
              onKeyDown={handleEditorTextareaKeyDown}
              placeholder={"version: 1\nname: fixture\ntype: one-step\nhosts:\n  - https://example.com"}
            />
          </label>
          <div className="plugin-editor-modal-warning">
            <strong>注意</strong>
            <span>导入后会直接替换当前编辑器中的配置内容，未保存的修改将被覆盖。</span>
          </div>
        </div>
        <div className="plugin-editor-modal-actions">
          <button className="btn btn-secondary" type="button" onClick={props.onClose} disabled={props.busyAction !== ""}>
            取消
          </button>
          <button className="btn btn-primary plugin-editor-modal-submit" type="button" onClick={props.onImport} disabled={props.busyAction !== ""}>
            {props.busyAction === "import" ? <LoaderCircle size={16} className="ruleset-debug-spinner" /> : <Import size={16} />}
            <span>导入 YAML</span>
          </button>
        </div>
      </div>
    </div>
  );
}

export function ExampleModal(props: {
  onClose: () => void;
}) {
  return (
    <div className="plugin-editor-modal-backdrop" role="presentation" onClick={props.onClose}>
      <div className="panel plugin-editor-modal plugin-editor-modal-example-dialog" role="dialog" aria-modal="true" aria-label="YAML 示例" onClick={(event) => event.stopPropagation()}>
        <div className="plugin-editor-modal-head">
          <div className="plugin-editor-modal-title-group">
            <div className="plugin-editor-modal-badge">
              <FileCode2 size={16} />
              <span>Example</span>
            </div>
            <div className="plugin-editor-modal-title-copy">
              <h3>YAML 示例</h3>
              <span>这是一份最小可用参考结构，你可以按需复制并修改。</span>
            </div>
          </div>
          <button className="btn btn-secondary plugin-editor-modal-close" type="button" aria-label="关闭示例窗口" title="关闭示例窗口" onClick={props.onClose}>
            <X size={16} />
          </button>
        </div>
        <div className="plugin-editor-modal-body">
          <pre className="plugin-editor-modal-example-code plugin-editor-modal-example-code-dialog">{IMPORT_YAML_EXAMPLE}</pre>
        </div>
        <div className="plugin-editor-modal-actions">
          <button className="btn btn-secondary" type="button" onClick={props.onClose}>
            关闭
          </button>
        </div>
      </div>
    </div>
  );
}
