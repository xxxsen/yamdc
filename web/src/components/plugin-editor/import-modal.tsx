"use client";

import { FileCode2, Import } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";

import { IMPORT_YAML_EXAMPLE } from "./plugin-editor-constants";
import { handleEditorTextareaKeyDown } from "./plugin-editor-utils";

export function ImportModal(props: {
  importYAML: string;
  onImportYAMLChange: (value: string) => void;
  onImport: () => void;
  onClose: () => void;
  onShowExample: () => void;
  busyAction: string;
}) {
  const busy = props.busyAction !== "";
  return (
    <Modal
      open
      onClose={props.onClose}
      title="导入 YAML"
      subtitle="粘贴已有插件配置，并将内容回填到当前编辑器表单。"
      badge={{ icon: <Import size={16} />, label: "Import" }}
      ariaLabel="导入 YAML"
      closeAriaLabel="关闭导入窗口"
      disableClose={busy}
      actions={
        <>
          <Button onClick={props.onClose} disabled={busy}>
            取消
          </Button>
          <Button
            variant="primary"
            className="plugin-editor-modal-submit"
            onClick={props.onImport}
            disabled={busy}
            loading={props.busyAction === "import"}
            leftIcon={<Import size={16} />}
          >
            <span>导入 YAML</span>
          </Button>
        </>
      }
    >
      <div className="plugin-editor-modal-tip">
        <strong>支持内容</strong>
        <span>支持直接粘贴完整插件 YAML。导入后会覆盖当前表单内容。</span>
      </div>
      <div className="plugin-editor-modal-example">
        <div className="plugin-editor-modal-example-copy">
          <strong>参考结构</strong>
          <span>查看一份最小可用的 YAML 示例，方便直接按结构粘贴或修改。</span>
        </div>
        <Button
          className="plugin-editor-modal-example-btn"
          onClick={props.onShowExample}
        >
          查看 YAML 示例
        </Button>
      </div>
      <label className="plugin-editor-field plugin-editor-modal-editor">
        <span>Plugin YAML</span>
        <textarea
          className="input plugin-editor-textarea plugin-editor-textarea-lg plugin-editor-modal-textarea"
          value={props.importYAML}
          onChange={(event) => props.onImportYAMLChange(event.target.value)}
          onKeyDown={handleEditorTextareaKeyDown}
          placeholder={
            "version: 1\nname: fixture\ntype: one-step\nhosts:\n  - https://example.com"
          }
        />
      </label>
      <div className="plugin-editor-modal-warning">
        <strong>注意</strong>
        <span>导入后会直接替换当前编辑器中的配置内容，未保存的修改将被覆盖。</span>
      </div>
    </Modal>
  );
}

export function ExampleModal(props: { onClose: () => void }) {
  return (
    <Modal
      open
      onClose={props.onClose}
      title="YAML 示例"
      subtitle="这是一份最小可用参考结构，你可以按需复制并修改。"
      badge={{ icon: <FileCode2 size={16} />, label: "Example" }}
      ariaLabel="YAML 示例"
      closeAriaLabel="关闭示例窗口"
      className="plugin-editor-modal-example-dialog"
      actions={<Button onClick={props.onClose}>关闭</Button>}
    >
      <pre className="plugin-editor-modal-example-code plugin-editor-modal-example-code-dialog">
        {IMPORT_YAML_EXAMPLE}
      </pre>
    </Modal>
  );
}
