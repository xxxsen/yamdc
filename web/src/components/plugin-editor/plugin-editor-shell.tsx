"use client";

import { X } from "lucide-react";
import dynamic from "next/dynamic";
import Link from "next/link";

import { FloatingMenu } from "./floating-menu";

// ImportModal / ExampleModal 只在用户按 "导入 YAML" / "查看示例" 后才打开,
// 且还挂带了 IMPORT_YAML_EXAMPLE 常量文本. 默认不进首屏 JS. 两者同在
// ./import-modal, webpack 自动 dedupe 到一个 chunk. 详见 §5.2.
const ImportModal = dynamic(
  () => import("./import-modal").then((m) => m.ImportModal),
  { ssr: false },
);
const ExampleModal = dynamic(
  () => import("./import-modal").then((m) => m.ExampleModal),
  { ssr: false },
);

import { OutputShell } from "./output-shell";
import type { EditorSection } from "./plugin-editor-types";
import { BasicSection } from "./sections/basic-section";
import { PostprocessSection } from "./sections/postprocess-section";
import { RequestSection } from "./sections/request-section";
import { ScrapeSection } from "./sections/scrape-section";
import { usePluginEditorState } from "./use-plugin-editor-state";

// PluginEditorShell: plugin-editor 顶层编排. 拆分方案 (参考 td/022 §2.1 A-2):
//
//   use-plugin-editor-state.ts          — 所有 state/refs/effects/updaters/actions
//   floating-menu.tsx                   — 左上角可拖拽的 Plugin Builder 浮动菜单
//   sections/basic-section.tsx          — Basic tab
//   sections/request-section.tsx        — Request + Multi-candidates + Workflow tab
//   sections/scrape-section.tsx         — Fields tab
//   sections/postprocess-section.tsx    — Advanced tab
//   output-shell.tsx                    — 右侧 "调试输出" 面板 (7 个 tab 分支)
//
// 本文件只负责: (1) hook 实例化, (2) 按 activeSection 路由到对应 section,
// (3) 把模态 / toast 放到合适位置. 零本地 state.

const SECTION_ITEMS: Array<{ id: string; section: EditorSection; label: string }> = [
  { id: "plugin-editor-section-basic", section: "basic", label: "Basic" },
  { id: "plugin-editor-section-request", section: "request", label: "Request" },
  { id: "plugin-editor-section-scrape", section: "scrape", label: "Fields" },
  { id: "plugin-editor-section-postprocess", section: "postprocess", label: "Advanced" },
];

export function PluginEditorShell() {
  // NOTE: 这里 **必须** 解构成扁平变量, 不能走 `const s = usePluginEditor...()`
  // 然后到处 s.xxx. 原因是 ESLint react-hooks/refs 规则看到 s 里含 RefObject
  // (pageRef / compileMenuRef) 就把 `s.xxx` 全部当作疑似 ref 访问报警.
  const {
    state,
    tab,
    activeSection,
    compileResult,
    requestResult,
    workflowResult,
    scrapeResult,
    error,
    busyAction,
    toast,
    importOpen,
    exampleOpen,
    compileMenuOpen,
    floatingMenuPos,
    pageRef,
    compileMenuRef,
    draftPreview,
    canAddField,
    setTab,
    setActiveSection,
    setImportOpen,
    setExampleOpen,
    setCompileMenuOpen,
    patch,
    patchRequest,
    patchField,
    updateFieldName,
    patchWorkflowSelector,
    patchKVPair,
    addKVPair,
    removeKVPair,
    patchTransform,
    addField,
    removeField,
    addTransform,
    removeTransform,
    addWorkflowSelector,
    removeWorkflowSelector,
    run,
    handleCopyYAML,
    handleImportYAML,
    handleClearDraft,
    handleFloatingMenuPointerDown,
  } = usePluginEditorState();

  return (
    <div ref={pageRef} className="plugin-editor-page">
      <Link href="/debug/searcher" className="workspace-close-btn plugin-editor-close-btn" aria-label="关闭插件编辑器" title="关闭插件编辑器">
        <X size={18} />
      </Link>

      <FloatingMenu
        compileMenuRef={compileMenuRef}
        floatingMenuPos={floatingMenuPos}
        compileMenuOpen={compileMenuOpen}
        busyAction={busyAction}
        compileResult={compileResult}
        onPointerDown={handleFloatingMenuPointerDown}
        onToggleCompileMenu={() => setCompileMenuOpen((prev) => !prev)}
        onRun={(action) => void run(action)}
        onCopyYAML={() => void handleCopyYAML()}
        onOpenImport={() => setImportOpen(true)}
        onClearDraft={handleClearDraft}
        onCloseCompileMenu={() => setCompileMenuOpen(false)}
      />

      <div className="plugin-editor-workbench">
        <section className="plugin-editor-column plugin-editor-column-form">
          <section className="panel plugin-editor-panel plugin-editor-editor-shell">
            <div className="plugin-editor-panel-head">
              <h3>插件配置</h3>
              <span>{state.name || "未命名插件"}</span>
            </div>
            <div className="plugin-editor-tabs plugin-editor-tabs-editor">
              {SECTION_ITEMS.map((item) => (
                <button
                  key={item.id}
                  className={`handler-debug-tab ${activeSection === item.section ? "handler-debug-tab-active" : ""}`}
                  type="button"
                  onClick={() => setActiveSection(item.section)}
                >
                  {item.label}
                </button>
              ))}
            </div>

            {activeSection === "basic" ? (
              <BasicSection
                state={state}
                onPatch={patch}
                onAddKVPair={addKVPair}
                onRemoveKVPair={removeKVPair}
                onPatchKVPair={patchKVPair}
              />
            ) : null}

            {activeSection === "request" ? (
              <RequestSection
                state={state}
                onPatch={patch}
                onPatchRequest={patchRequest}
                onPatchWorkflowSelector={patchWorkflowSelector}
                onAddWorkflowSelector={addWorkflowSelector}
                onRemoveWorkflowSelector={removeWorkflowSelector}
                onAddKVPair={addKVPair}
                onRemoveKVPair={removeKVPair}
                onPatchKVPair={patchKVPair}
              />
            ) : null}

            {activeSection === "scrape" ? (
              <ScrapeSection
                state={state}
                canAddField={canAddField}
                onPatchField={patchField}
                onUpdateFieldName={updateFieldName}
                onRemoveField={removeField}
                onAddField={addField}
                onAddTransform={addTransform}
                onRemoveTransform={removeTransform}
                onPatchTransform={patchTransform}
              />
            ) : null}

            {activeSection === "postprocess" ? (
              <PostprocessSection
                state={state}
                onPatch={patch}
                onAddKVPair={addKVPair}
                onRemoveKVPair={removeKVPair}
                onPatchKVPair={patchKVPair}
              />
            ) : null}
          </section>
        </section>

        <section className="plugin-editor-column plugin-editor-column-output">
          <OutputShell
            tab={tab}
            onTabChange={setTab}
            error={error}
            compileResult={compileResult}
            requestResult={requestResult}
            workflowResult={workflowResult}
            scrapeResult={scrapeResult}
            draftPreview={draftPreview}
          />
        </section>
      </div>

      {importOpen ? (
        <ImportModal
          importYAML={state.importYAML}
          onImportYAMLChange={(value) => patch("importYAML", value)}
          onImport={() => void handleImportYAML()}
          onClose={() => setImportOpen(false)}
          onShowExample={() => setExampleOpen(true)}
          busyAction={busyAction}
        />
      ) : null}

      {exampleOpen ? <ExampleModal onClose={() => setExampleOpen(false)} /> : null}

      {toast ? (
        <div className="file-list-toast" data-tone={toast.tone !== "info" ? toast.tone : undefined}>
          {toast.message}
        </div>
      ) : null}
    </div>
  );
}
