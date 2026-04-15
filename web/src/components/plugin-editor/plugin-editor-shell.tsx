"use client";

import {
  ChevronDown,
  FileCode2,
  GripVertical,
  LoaderCircle,
  Plus,
  ScanSearch,
  Sparkles,
  Trash2,
  X,
} from "lucide-react";
import Link from "next/link";
import { useDeferredValue, useEffect, useMemo, useRef, useState } from "react";

import {
  compilePluginDraft,
  debugPluginDraftRequest,
  debugPluginDraftScrape,
  debugPluginDraftWorkflow,
  importPluginDraftYAML,
  type PluginEditorCompileResult,
  type PluginEditorRequestDebugResult,
  type PluginEditorScrapeDebugResult,
  type PluginEditorWorkflowDebugResult,
} from "@/lib/api";

import {
  DEFAULT_DRAFT_STORAGE_KEY,
  DEFAULT_FIELD,
  FIELD_OPTIONS,
  META_LANG_OPTIONS,
} from "./plugin-editor-constants";
import type {
  EditorSection,
  EditorState,
  EditorTab,
  FieldForm,
  KVPairForm,
  RequestFormState,
  RunAction,
  ToastState,
  TransformForm,
  WorkflowSelectorForm,
} from "./plugin-editor-types";
import {
  applyFieldMeta,
  buildDraft,
  defaultState,
  handleEditorTextareaKeyDown,
  insertTransform,
  nextUnusedFieldName,
  nextUnusedKVFieldName,
  normalizeEditorState,
  stateFromDraft,
} from "./plugin-editor-utils";
import { RequestForm } from "./request-form";
import { FieldCard } from "./field-card";
import { KVPairEditor, WorkflowItemVariablesEditor } from "./kv-pair-editor";
import { ImportModal, ExampleModal } from "./import-modal";
import {
  RequestBasicPanel,
  RequestDetailPanel,
  ResponseDetailPanel,
  WorkflowOutputPanel,
  ScrapeJSONPanel,
} from "./output-panels";

const DEFAULT_NUMBER_STORAGE_KEY = "yamdc.debug.plugin-editor.number";

export function PluginEditorShell() {
  const [tab, setTab] = useState<EditorTab>("compile");
  const [activeSection, setActiveSection] = useState<EditorSection>("basic");
  const [state, setState] = useState<EditorState>(defaultState);
  const [compileResult, setCompileResult] = useState<PluginEditorCompileResult | null>(null);
  const [requestResult, setRequestResult] = useState<PluginEditorRequestDebugResult | null>(null);
  const [workflowResult, setWorkflowResult] = useState<PluginEditorWorkflowDebugResult | null>(null);
  const [scrapeResult, setScrapeResult] = useState<PluginEditorScrapeDebugResult | null>(null);
  const [error, setError] = useState("");
  const [busyAction, setBusyAction] = useState<RunAction | "import" | "">("");
  const [toast, setToast] = useState<ToastState>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [exampleOpen, setExampleOpen] = useState(false);
  const [compileMenuOpen, setCompileMenuOpen] = useState(false);
  const [floatingMenuPos, setFloatingMenuPos] = useState<{ x: number; y: number } | null>(null);
  const dragStateRef = useRef<{ offsetX: number; offsetY: number; width: number; height: number } | null>(null);
  const pageRef = useRef<HTMLDivElement | null>(null);
  const compileMenuRef = useRef<HTMLDivElement | null>(null);
  const deferredState = useDeferredValue(state);

  // ---------------------------------------------------------------------------
  // Effects
  // ---------------------------------------------------------------------------

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const timer = window.setTimeout(() => {
      window.localStorage.setItem(DEFAULT_DRAFT_STORAGE_KEY, JSON.stringify(state));
      window.localStorage.setItem(DEFAULT_NUMBER_STORAGE_KEY, state.number);
    }, 160);
    return () => window.clearTimeout(timer);
  }, [state]);

  useEffect(() => {
    if (!toast) {
      return;
    }
    const timer = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    if (!compileMenuOpen) {
      return;
    }
    function handlePointerDown(event: PointerEvent) {
      if (!compileMenuRef.current?.contains(event.target as Node)) {
        setCompileMenuOpen(false);
      }
    }
    window.addEventListener("pointerdown", handlePointerDown);
    return () => window.removeEventListener("pointerdown", handlePointerDown);
  }, [compileMenuOpen]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    try {
      const stored = window.localStorage.getItem(DEFAULT_DRAFT_STORAGE_KEY);
      const parsed = stored ? (JSON.parse(stored) as Partial<EditorState>) : null;
      const next = parsed ? normalizeEditorState({ ...defaultState(), ...parsed }) : defaultState();
      const number = window.localStorage.getItem(DEFAULT_NUMBER_STORAGE_KEY);
      if (number) {
        next.number = number;
      }
      setState(next);
    } catch {
      setState(defaultState());
    }
  }, []);

  useEffect(() => {
    function handlePointerMove(event: PointerEvent) {
      const dragState = dragStateRef.current;
      const page = pageRef.current;
      if (!dragState || !page) {
        return;
      }
      const pageRect = page.getBoundingClientRect();
      const menuWidth = dragState.width;
      const menuHeight = dragState.height;
      const maxX = Math.max(12, pageRect.width - menuWidth - 12);
      const maxY = Math.max(12, pageRect.height - menuHeight - 12);
      setFloatingMenuPos({
        x: Math.min(Math.max(event.clientX - pageRect.left - dragState.offsetX, 12), maxX),
        y: Math.min(Math.max(event.clientY - pageRect.top - dragState.offsetY, 12), maxY),
      });
    }

    function handlePointerUp() {
      dragStateRef.current = null;
    }

    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
    };
  }, []);

  // ---------------------------------------------------------------------------
  // Memos
  // ---------------------------------------------------------------------------

  const previewDraft = useMemo(() => {
    if (tab !== "draft") {
      return null;
    }
    try {
      return buildDraft(deferredState);
    } catch {
      return null;
    }
  }, [deferredState, tab]);

  const draftPreview = useMemo(() => (previewDraft ? JSON.stringify(previewDraft, null, 2) : ""), [previewDraft]);
  const canAddField = state.fields.length < FIELD_OPTIONS.length;
  const sectionItems: Array<{ id: string; section: EditorSection; label: string }> = [
    { id: "plugin-editor-section-basic", section: "basic", label: "Basic" },
    { id: "plugin-editor-section-request", section: "request", label: "Request" },
    { id: "plugin-editor-section-scrape", section: "scrape", label: "Fields" },
    { id: "plugin-editor-section-postprocess", section: "postprocess", label: "Advanced" },
  ];

  // ---------------------------------------------------------------------------
  // State updaters
  // ---------------------------------------------------------------------------

  function patch<K extends keyof EditorState>(key: K, value: EditorState[K]) {
    setState((prev) => ({ ...prev, [key]: value }));
  }

  function patchRequest(key: "request" | "multiRequest" | "workflowNextRequest", updater: (prev: RequestFormState) => RequestFormState) {
    setState((prev) => ({ ...prev, [key]: updater(prev[key]) }));
  }

  function patchField(id: string, updater: (field: FieldForm) => FieldForm) {
    setState((prev) => ({
      ...prev,
      fields: prev.fields.map((field) => (field.id === id ? updater(field) : field)),
    }));
  }

  function updateFieldName(id: string, nextName: string) {
    setState((prev) => ({
      ...prev,
      fields: prev.fields.map((field) => (field.id === id ? applyFieldMeta({ ...field, name: nextName }) : field)),
    }));
  }

  function patchWorkflowSelector(id: string, updater: (selector: WorkflowSelectorForm) => WorkflowSelectorForm) {
    setState((prev) => ({
      ...prev,
      workflowSelectors: prev.workflowSelectors.map((selector) => (selector.id === id ? updater(selector) : selector)),
    }));
  }

  function patchKVPair(
    key: "workflowItemVariables" | "postAssign" | "precheckVariables",
    id: string,
    updater: (item: KVPairForm) => KVPairForm,
  ) {
    setState((prev) => ({
      ...prev,
      [key]: prev[key].map((item) => (item.id === id ? updater(item) : item)),
    }));
  }

  function addKVPair(key: "workflowItemVariables" | "postAssign" | "precheckVariables") {
    const nextKey = key === "postAssign" ? nextUnusedKVFieldName(state.postAssign) : "";
    setState((prev) => ({
      ...prev,
      [key]: [...prev[key], { id: `kv-${Date.now()}`, key: nextKey, value: "" }],
    }));
  }

  function removeKVPair(key: "workflowItemVariables" | "postAssign" | "precheckVariables", id: string) {
    setState((prev) => ({
      ...prev,
      [key]: prev[key].filter((item) => item.id !== id),
    }));
  }

  function patchTransform(fieldID: string, transformID: string, updater: (transform: TransformForm) => TransformForm) {
    setState((prev) => ({
      ...prev,
      fields: prev.fields.map((field) =>
        field.id !== fieldID
          ? field
          : {
              ...field,
              transforms: field.transforms.map((transform) => (transform.id === transformID ? updater(transform) : transform)),
            },
      ),
    }));
  }

  function addField() {
    const nextName = nextUnusedFieldName(state.fields);
    if (!nextName) {
      return;
    }
    setState((prev) => ({
      ...prev,
      fields: [
        ...prev.fields,
        applyFieldMeta({
          ...DEFAULT_FIELD,
          id: `field-${Date.now()}`,
          name: nextName,
        }),
      ],
    }));
  }

  function removeField(id: string) {
    setState((prev) => ({
      ...prev,
      fields: prev.fields.length === 1 ? prev.fields : prev.fields.filter((field) => field.id !== id),
    }));
  }

  function addTransform(fieldID: string, afterTransformID?: string) {
    setState((prev) => ({
      ...prev,
      fields: prev.fields.map((field) =>
        field.id !== fieldID
          ? field
          : {
              ...field,
              transforms: insertTransform(field.transforms, afterTransformID),
            },
      ),
    }));
  }

  function removeTransform(fieldID: string, transformID: string) {
    setState((prev) => ({
      ...prev,
      fields: prev.fields.map((field) =>
        field.id !== fieldID
          ? field
          : {
              ...field,
              transforms:
                field.transforms.length === 1
                  ? field.transforms
                  : field.transforms.filter((transform) => transform.id !== transformID),
            },
      ),
    }));
  }

  function addWorkflowSelector() {
    setState((prev) => ({
      ...prev,
      workflowSelectors: [
        ...prev.workflowSelectors,
        { id: `selector-${Date.now()}`, name: "", kind: "xpath", expr: "" },
      ],
    }));
  }

  function removeWorkflowSelector(id: string) {
    setState((prev) => ({
      ...prev,
      workflowSelectors:
        prev.workflowSelectors.length === 1 ? prev.workflowSelectors : prev.workflowSelectors.filter((item) => item.id !== id),
    }));
  }

  // ---------------------------------------------------------------------------
  // Actions
  // ---------------------------------------------------------------------------

  async function run(action: RunAction) {
    try {
      const draft = buildDraft(state);
      setBusyAction(action);
      setError("");
      setToast(null);
      setCompileResult(null);
      setRequestResult(null);
      setWorkflowResult(null);
      setScrapeResult(null);
      if (action === "compile") {
        const result = await compilePluginDraft(draft);
        setCompileResult(result.data);
        setTab("compile");
        return;
      }
      if (action === "request") {
        const result = await debugPluginDraftRequest(draft, state.number.trim());
        setRequestResult(result.data);
        setTab("request");
        return;
      }
      if (action === "workflow") {
        const result = await debugPluginDraftWorkflow(draft, state.number.trim());
        setWorkflowResult(result.data);
        setTab("workflow");
        return;
      }
      if (action === "scrape") {
        if (state.workflowEnabled) {
          const workflowDebug = await debugPluginDraftWorkflow(draft, state.number.trim());
          setWorkflowResult(workflowDebug.data);
          if (workflowDebug.data.error) {
            setTab("workflow");
            return;
          }
        }
        const scrapeDebug = await debugPluginDraftScrape(draft, state.number.trim());
        // When multi_request is enabled, the request debug endpoint returns
        // `attempts` data that the scrape endpoint does not include.
        // Call it separately to preserve that info in the basics panel.
        if (state.multiRequestEnabled) {
          const reqDebug = await debugPluginDraftRequest(draft, state.number.trim());
          setRequestResult(reqDebug.data);
        } else {
          setRequestResult({
            request: scrapeDebug.data.request,
            response: scrapeDebug.data.response,
          });
        }
        if (!state.workflowEnabled) {
          setWorkflowResult(null);
        }
        setScrapeResult(scrapeDebug.data);
        setTab(state.workflowEnabled ? "scrape" : "basic");
        return;
      }
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "插件调试失败");
    } finally {
      setBusyAction("");
    }
  }

  async function handleCopyYAML() {
    const yaml = compileResult?.yaml;
    if (!yaml) {
      return;
    }
    try {
      await navigator.clipboard.writeText(yaml);
      setToast({ message: "YAML 已复制。", tone: "info" });
    } catch {
      setToast({ message: "复制失败，请手动复制。", tone: "danger" });
    }
  }

  async function handleImportYAML() {
    try {
      setBusyAction("import");
      setError("");
      setToast(null);
      const result = await importPluginDraftYAML(state.importYAML);
      setState((prev) => ({
        ...stateFromDraft(result.data.draft),
        importYAML: prev.importYAML,
      }));
      setCompileResult(null);
      setRequestResult(null);
      setWorkflowResult(null);
      setScrapeResult(null);
      setToast({ message: "YAML 已导入并回填到表单。", tone: "info" });
      setImportOpen(false);
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "YAML 导入失败");
    } finally {
      setBusyAction("");
    }
  }

  function handleFloatingMenuPointerDown(event: React.PointerEvent<HTMLDivElement>) {
    const menu = event.currentTarget.parentElement;
    const page = pageRef.current;
    if (!menu || !page) {
      return;
    }
    const rect = menu.getBoundingClientRect();
    const pageRect = page.getBoundingClientRect();
    dragStateRef.current = {
      offsetX: event.clientX - rect.left,
      offsetY: event.clientY - rect.top,
      width: rect.width,
      height: rect.height,
    };
    setFloatingMenuPos({
      x: Math.min(Math.max(rect.left - pageRect.left, 12), Math.max(12, pageRect.width - rect.width - 12)),
      y: Math.min(Math.max(rect.top - pageRect.top, 12), Math.max(12, pageRect.height - rect.height - 12)),
    });
  }

  function handleClearDraft() {
    const next = defaultState();
    if (typeof window !== "undefined") {
      window.localStorage.removeItem(DEFAULT_DRAFT_STORAGE_KEY);
      window.localStorage.removeItem(DEFAULT_NUMBER_STORAGE_KEY);
    }
    setState(next);
    setCompileResult(null);
    setRequestResult(null);
    setWorkflowResult(null);
    setScrapeResult(null);
    setError("");
    setTab("compile");
    setActiveSection("basic");
    setImportOpen(false);
    setExampleOpen(false);
    setCompileMenuOpen(false);
    setToast({ message: "草稿已清空。", tone: "info" });
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div ref={pageRef} className="plugin-editor-page">
      <Link href="/debug/searcher" className="workspace-close-btn plugin-editor-close-btn" aria-label="关闭插件编辑器" title="关闭插件编辑器">
        <X size={18} />
      </Link>

      <div
        className={`panel plugin-editor-floating-menu ${floatingMenuPos ? "" : "plugin-editor-floating-menu-default"}`}
        style={floatingMenuPos ? { left: `${floatingMenuPos.x}px`, top: `${floatingMenuPos.y}px` } : undefined}
      >
        <div className="plugin-editor-floating-menu-handle" onPointerDown={handleFloatingMenuPointerDown}>
          <GripVertical size={14} />
          <span>Plugin Builder</span>
          <Sparkles size={14} />
        </div>
        <div className="plugin-editor-floating-menu-actions">
          <div ref={compileMenuRef} className="plugin-editor-split-action">
            <button className="btn btn-primary plugin-editor-split-action-main" type="button" onClick={() => void run("compile")} disabled={busyAction !== ""}>
              {busyAction === "compile" ? <LoaderCircle size={16} className="ruleset-debug-spinner" /> : <FileCode2 size={16} />}
              <span>编译草稿</span>
            </button>
            <button
              className="btn btn-primary plugin-editor-split-action-toggle"
              type="button"
              aria-label="展开编译草稿菜单"
              title="展开编译草稿菜单"
              aria-expanded={compileMenuOpen}
              onClick={() => setCompileMenuOpen((prev) => !prev)}
              disabled={busyAction !== ""}
            >
              <ChevronDown size={14} />
            </button>
            {compileMenuOpen ? (
              <div className="plugin-editor-split-action-menu">
                <button className="btn btn-primary plugin-editor-split-action-menu-item" type="button" onClick={handleCopyYAML} disabled={!compileResult?.yaml}>
                  复制 YAML
                </button>
                <button
                  className="btn btn-primary plugin-editor-split-action-menu-item"
                  type="button"
                  onClick={() => {
                    setCompileMenuOpen(false);
                    setImportOpen(true);
                  }}
                  disabled={busyAction !== ""}
                >
                  导入 YAML
                </button>
                <button className="btn btn-primary plugin-editor-split-action-menu-item" type="button" onClick={handleClearDraft}>
                  清空草稿
                </button>
              </div>
            ) : null}
          </div>
          <button className="btn btn-primary" type="button" onClick={() => void run("scrape")} disabled={busyAction !== ""}>
            {busyAction === "scrape" ? <LoaderCircle size={16} className="ruleset-debug-spinner" /> : <ScanSearch size={16} />}
            <span>运行调试</span>
          </button>
        </div>
      </div>

      <div className="plugin-editor-workbench">
        <section className="plugin-editor-column plugin-editor-column-form">
          <section className="panel plugin-editor-panel plugin-editor-editor-shell">
            <div className="plugin-editor-panel-head">
              <h3>插件配置</h3>
              <span>{state.name || "未命名插件"}</span>
            </div>
            <div className="plugin-editor-tabs plugin-editor-tabs-editor">
              {sectionItems.map((item) => (
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
                    <input className="input" value={state.name} onChange={(event) => patch("name", event.target.value)} />
                  </label>
                  <label className="plugin-editor-field">
                    <span>Type</span>
                    <select className="input" value={state.type} onChange={(event) => patch("type", event.target.value)}>
                      <option value="one-step">one-step</option>
                      <option value="two-step">two-step</option>
                    </select>
                  </label>
                  <label className="plugin-editor-field">
                    <span>Fetch Type</span>
                    <select className="input" value={state.fetchType} onChange={(event) => patch("fetchType", event.target.value)}>
                      <option value="go-http">go-http</option>
                      <option value="browser">browser</option>
                    </select>
                  </label>
                </div>
                <div className="plugin-editor-form-grid">
                  <label className="plugin-editor-field plugin-editor-field-wide">
                    <span>Hosts</span>
                    <textarea
                      className="input plugin-editor-textarea plugin-editor-textarea-compact"
                      value={state.hostsText}
                      onChange={(event) => patch("hostsText", event.target.value)}
                      onKeyDown={handleEditorTextareaKeyDown}
                      placeholder="每行一个 host"
                    />
                  </label>
                  <label className="plugin-editor-field plugin-editor-field-wide">
                    <span>Precheck Patterns</span>
                    <textarea
                      className="input plugin-editor-textarea plugin-editor-textarea-compact"
                      value={state.precheckPatternsText}
                      onChange={(event) => patch("precheckPatternsText", event.target.value)}
                      onKeyDown={handleEditorTextareaKeyDown}
                      placeholder="每行一个正则"
                    />
                  </label>
                  <label className="plugin-editor-field plugin-editor-field-wide">
                    <span>Test Number</span>
                    <input className="input" value={state.number} onChange={(event) => patch("number", event.target.value)} />
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
                  onAdd={() => addKVPair("precheckVariables")}
                  onRemove={(id) => removeKVPair("precheckVariables", id)}
                  onChange={(id, updater) => patchKVPair("precheckVariables", id, updater)}
                  keyLabel="Name"
                  valueLabel="Expression"
                  valuePlaceholder='${clean_number(${number})}'
                  emptyLabel="暂未定义 precheck variables。"
                />
              </div>
            </div>
          </article>
          ) : null}

          {activeSection === "request" ? (
          <article id="plugin-editor-section-request" className="plugin-editor-panel-fragment plugin-editor-request-shell">
            <div className="plugin-editor-subcard">
              <div className="plugin-editor-subcard-head">
                <strong>Request</strong>
                <span>配置首次请求及其响应判定规则。</span>
              </div>
              <RequestForm
                state={state.request}
                onChange={(updater) => patchRequest("request", updater)}
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
                  onChange={(event) => patch("multiRequestEnabled", event.target.checked)}
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
                        onChange={(event) => patch("multiCandidatesText", event.target.value)}
                        onKeyDown={handleEditorTextareaKeyDown}
                        placeholder={'每行一个 candidate 模板，例如：\n${number}\n${to_upper(${number})}\n${replace(${number}, "-", "_")}\n${replace(${number}, "_", "")}'}
                      />
                    </label>
                    <label className="plugin-editor-field">
                      <span>Success Mode</span>
                      <select className="input" value={state.multiSuccessMode} onChange={(event) => patch("multiSuccessMode", event.target.value)}>
                        <option value="and">and</option>
                        <option value="or">or</option>
                      </select>
                    </label>
                    <label className="plugin-editor-field plugin-editor-field-wide">
                      <span>Success Conditions</span>
                      <textarea
                        className="input plugin-editor-textarea plugin-editor-textarea-compact"
                        value={state.multiSuccessConditionsText}
                        onChange={(event) => patch("multiSuccessConditionsText", event.target.value)}
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
                <input type="checkbox" checked={state.workflowEnabled} onChange={(event) => patch("workflowEnabled", event.target.checked)} />
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
                              <button
                                className="btn btn-secondary plugin-editor-transform-action"
                                type="button"
                                aria-label="新增 selector"
                                title="新增 selector"
                                onClick={addWorkflowSelector}
                              >
                                <Plus size={14} />
                              </button>
                              <button
                                className="btn btn-secondary plugin-editor-transform-action"
                                type="button"
                                aria-label="删除 selector"
                                title="删除 selector"
                                onClick={() => removeWorkflowSelector(selector.id)}
                              >
                                <Trash2 size={14} />
                              </button>
                            </div>
                            <label className="plugin-editor-transform-inline-field plugin-editor-selector-inline-field-name">
                              <span>Name</span>
                              <input className="input" value={selector.name} onChange={(event) => patchWorkflowSelector(selector.id, (prev) => ({ ...prev, name: event.target.value }))} />
                            </label>
                            <label className="plugin-editor-transform-inline-field plugin-editor-selector-inline-field-kind">
                              <span>Kind</span>
                              <select className="input" value={selector.kind} onChange={(event) => patchWorkflowSelector(selector.id, (prev) => ({ ...prev, kind: event.target.value }))}>
                                <option value="xpath">xpath</option>
                                <option value="jsonpath">jsonpath</option>
                              </select>
                            </label>
                            <label className="plugin-editor-transform-inline-field plugin-editor-selector-inline-field-expr">
                              <span>Expr</span>
                              <input className="input" value={selector.expr} onChange={(event) => patchWorkflowSelector(selector.id, (prev) => ({ ...prev, expr: event.target.value }))} />
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
                        onAdd={() => addKVPair("workflowItemVariables")}
                        onRemove={(id) => removeKVPair("workflowItemVariables", id)}
                        onChange={(id, updater) => patchKVPair("workflowItemVariables", id, updater)}
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
                          <select className="input" value={state.workflowMatchMode} onChange={(event) => patch("workflowMatchMode", event.target.value)}>
                            <option value="and">and</option>
                            <option value="or">or</option>
                          </select>
                        </label>
                        <label className="plugin-editor-field-inline plugin-editor-workflow-inline-field-sm">
                          <span>Expect Count</span>
                          <input className="input" value={state.workflowExpectCountText} onChange={(event) => patch("workflowExpectCountText", event.target.value)} placeholder="可选，例如 1" />
                        </label>
                        <label className="plugin-editor-field-inline plugin-editor-workflow-inline-field-lg">
                          <span>Return Template</span>
                          <input className="input" value={state.workflowReturn} onChange={(event) => patch("workflowReturn", event.target.value)} placeholder="${item.read_link}" />
                        </label>
                      </div>
                      <div className="plugin-editor-form-grid">
                        <label className="plugin-editor-field plugin-editor-field-wide">
                          <span>Match Conditions</span>
                          <textarea
                            className="input plugin-editor-textarea plugin-editor-textarea-compact"
                            value={state.workflowMatchConditionsText}
                            onChange={(event) => patch("workflowMatchConditionsText", event.target.value)}
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
                    onChange={(updater) => patchRequest("workflowNextRequest", updater)}
                    expandAdvanced
                    compactJSONBlocks
                    nextRequestLayout
                    fetchType={state.fetchType}
                  />
                </div>
              </div>
            ) : null}
          </article>
          ) : null}

          {activeSection === "scrape" ? (
          <article id="plugin-editor-section-scrape" className="plugin-editor-panel-fragment">
            <div className="plugin-editor-fields">
              {state.fields.map((field) => (
                <FieldCard
                  key={field.id}
                  field={field}
                  allFields={state.fields}
                  onPatchField={(updater) => patchField(field.id, updater)}
                  onUpdateName={(nextName) => updateFieldName(field.id, nextName)}
                  onRemove={() => removeField(field.id)}
                  onAddTransform={(afterID) => addTransform(field.id, afterID)}
                  onRemoveTransform={(transformID) => removeTransform(field.id, transformID)}
                  onPatchTransform={(transformID, updater) => patchTransform(field.id, transformID, updater)}
                />
              ))}
            </div>
            <div className="plugin-editor-inline-actions">
              <button
                className="btn btn-secondary plugin-editor-transform-action"
                type="button"
                aria-label="新增字段"
                title="新增字段"
                onClick={addField}
                disabled={!canAddField}
              >
                <Plus size={14} />
              </button>
            </div>
          </article>
          ) : null}

          {activeSection === "postprocess" ? (
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
                  onAdd={() => addKVPair("postAssign")}
                  onRemove={(id) => removeKVPair("postAssign", id)}
                  onChange={(id, updater) => patchKVPair("postAssign", id, updater)}
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
                        <select className="input" value={state[key]} onChange={(event) => patch(key, event.target.value)}>
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
                      onChange={(event) => patch("postDisableReleaseDateCheck", event.target.checked)}
                    />
                    <span>disable_release_date_check</span>
                  </label>
                  <label className="searcher-debug-switch">
                    <input
                      type="checkbox"
                      checked={state.postDisableNumberReplace}
                      onChange={(event) => patch("postDisableNumberReplace", event.target.checked)}
                    />
                    <span>disable_number_replace</span>
                  </label>
                </div>
              </div>
            </div>
          </article>
          ) : null}
          </section>
        </section>

        <section className="plugin-editor-column plugin-editor-column-output">
          <article className="panel plugin-editor-panel">
            <div className="plugin-editor-panel-head">
              <h3>调试输出</h3>
              <span>{tab}</span>
            </div>
            <div className="plugin-editor-tabs">
              {[
                ["basic", "Basic"],
                ["request", "Request"],
                ["response", "Response"],
                ["workflow", "Workflow"],
                ["scrape", "Scrape"],
                ["compile", "Compile"],
                ["draft", "Draft"],
              ].map(([key, label]) => (
                <button
                  key={key}
                  className={`handler-debug-tab ${tab === key ? "handler-debug-tab-active" : ""}`}
                  type="button"
                  onClick={() => setTab(key as EditorTab)}
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
        <div className="file-list-toast" data-tone={toast.tone === "danger" ? "danger" : undefined}>
          {toast.message}
        </div>
      ) : null}
    </div>
  );
}
