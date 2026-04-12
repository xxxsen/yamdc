"use client";

import {
  ChevronDown,
  Plus,
  FileCode2,
  GripVertical,
  Import,
  LoaderCircle,
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
  type PluginEditorDraft,
  type PluginEditorField,
  type PluginEditorRequestDebugResult,
  type PluginEditorScrapeDebugResult,
  type PluginEditorTransform,
  type PluginEditorWorkflowDebugResult,
} from "@/lib/api";

type EditorTab = "compile" | "basic" | "request" | "response" | "workflow" | "scrape" | "draft";
type EditorSection = "basic" | "request" | "scrape" | "postprocess";
type RunAction = "compile" | "request" | "workflow" | "scrape";
type ToastState = { message: string; tone: "info" | "danger" } | null;

type FieldForm = {
  id: string;
  name: string;
  selectorKind: string;
  selectorExpr: string;
  selectorMulti: boolean;
  parserKind: string;
  parserLayout: string;
  required: boolean;
  transforms: TransformForm[];
};

type WorkflowSelectorForm = {
  id: string;
  name: string;
  kind: string;
  expr: string;
};

type TransformForm = {
  id: string;
  kind: string;
  old: string;
  newValue: string;
  cutset: string;
  sep: string;
  index: string;
  value: string;
};

type KVPairForm = {
  id: string;
  key: string;
  value: string;
};

type RequestBodyDraft = NonNullable<NonNullable<PluginEditorDraft["request"]>["body"]>;

function handleEditorTextareaKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
  if (event.key !== "Tab") {
    return;
  }
  event.preventDefault();
  const textarea = event.currentTarget;
  const start = textarea.selectionStart ?? 0;
  const end = textarea.selectionEnd ?? 0;
  const nextValue = `${textarea.value.slice(0, start)}\t${textarea.value.slice(end)}`;
  const nativeSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, "value")?.set;
  nativeSetter?.call(textarea, nextValue);
  textarea.dispatchEvent(new Event("input", { bubbles: true }));
  requestAnimationFrame(() => {
    textarea.selectionStart = start + 1;
    textarea.selectionEnd = start + 1;
  });
}

type EditorState = {
  name: string;
  type: string;
  hostsText: string;
  number: string;
  precheckPatternsText: string;
  precheckVariables: KVPairForm[];
  requestMethod: string;
  requestPath: string;
  requestURL: string;
  requestQueryJSON: string;
  requestHeadersJSON: string;
  requestCookiesJSON: string;
  requestBodyKind: string;
  requestBodyJSON: string;
  requestAcceptStatusText: string;
  requestNotFoundStatusText: string;
  requestDecodeCharset: string;
  scrapeFormat: string;
  fields: FieldForm[];
  multiRequestEnabled: boolean;
  multiCandidatesText: string;
  multiUnique: boolean;
  multiRequestMethod: string;
  multiRequestPath: string;
  multiRequestURL: string;
  multiRequestQueryJSON: string;
  multiRequestHeadersJSON: string;
  multiRequestCookiesJSON: string;
  multiRequestBodyKind: string;
  multiRequestBodyJSON: string;
  multiRequestAcceptStatusText: string;
  multiRequestNotFoundStatusText: string;
  multiRequestDecodeCharset: string;
  multiSuccessMode: string;
  multiSuccessConditionsText: string;
  workflowEnabled: boolean;
  workflowSelectors: WorkflowSelectorForm[];
  workflowItemVariables: KVPairForm[];
  workflowMatchMode: string;
  workflowMatchConditionsText: string;
  workflowExpectCountText: string;
  workflowReturn: string;
  workflowNextMethod: string;
  workflowNextPath: string;
  workflowNextURL: string;
  workflowNextQueryJSON: string;
  workflowNextHeadersJSON: string;
  workflowNextCookiesJSON: string;
  workflowNextBodyKind: string;
  workflowNextBodyJSON: string;
  workflowNextAcceptStatusText: string;
  workflowNextNotFoundStatusText: string;
  workflowNextDecodeCharset: string;
  postAssign: KVPairForm[];
  postTitleLang: string;
  postPlotLang: string;
  postGenresLang: string;
  postActorsLang: string;
  postDisableReleaseDateCheck: boolean;
  postDisableNumberReplace: boolean;
  importYAML: string;
};

type FieldMeta = {
  label: string;
  fixedParser?: string;
  parserOptions?: string[];
  defaultParser?: string;
  fixedMulti?: boolean;
};

const DEFAULT_DRAFT_STORAGE_KEY = "yamdc.debug.plugin-editor.draft.v2";
const DEFAULT_NUMBER_STORAGE_KEY = "yamdc.debug.plugin-editor.number";

const FIELD_META: Record<string, FieldMeta> = {
  number: { label: "number", fixedParser: "string", fixedMulti: false },
  title: { label: "title", fixedParser: "string", fixedMulti: false },
  plot: { label: "plot", fixedParser: "string", fixedMulti: false },
  actors: { label: "actors", fixedParser: "string_list", fixedMulti: true },
  release_date: {
    label: "release_date",
    parserOptions: ["date_only", "date_layout_soft", "time_format"],
    defaultParser: "date_layout_soft",
    fixedMulti: false,
  },
  duration: {
    label: "duration",
    parserOptions: ["duration_default", "duration_hhmmss", "duration_mm", "duration_mmss", "duration_human"],
    defaultParser: "duration_default",
    fixedMulti: false,
  },
  studio: { label: "studio", fixedParser: "string", fixedMulti: false },
  label: { label: "label", fixedParser: "string", fixedMulti: false },
  series: { label: "series", fixedParser: "string", fixedMulti: false },
  director: { label: "director", fixedParser: "string", fixedMulti: false },
  genres: { label: "genres", fixedParser: "string_list", fixedMulti: true },
  cover: { label: "cover", fixedParser: "string", fixedMulti: false },
  poster: { label: "poster", fixedParser: "string", fixedMulti: false },
  sample_images: { label: "sample_images", fixedParser: "string_list", fixedMulti: true },
};

const FIELD_OPTIONS = Object.keys(FIELD_META);
const META_LANG_OPTIONS = ["ja", "en", "zh-cn", "zh-tw"] as const;
const IMPORT_YAML_EXAMPLE = `version: 1
name: fixture
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/\${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
      required: true`;

const DEFAULT_FIELD: FieldForm = {
  id: "title",
  name: "title",
  selectorKind: "xpath",
  selectorExpr: "//title/text()",
  selectorMulti: false,
  parserKind: "string",
  parserLayout: "",
  required: true,
  transforms: [
    {
      id: "transform-1",
      kind: "trim",
      old: "",
      newValue: "",
      cutset: "",
      sep: "",
      index: "",
      value: "",
    },
  ],
};

function defaultState(): EditorState {
  return {
    name: "fixture",
    type: "one-step",
    hostsText: "https://example.com",
    number: "ABC-123",
    precheckPatternsText: "",
    precheckVariables: [],
    requestMethod: "GET",
    requestPath: "/search/${number}",
    requestURL: "",
    requestQueryJSON: "{}",
    requestHeadersJSON: "{}",
    requestCookiesJSON: "{}",
    requestBodyKind: "json",
    requestBodyJSON: "null",
    requestAcceptStatusText: "200",
    requestNotFoundStatusText: "404",
    requestDecodeCharset: "",
    scrapeFormat: "html",
    fields: [DEFAULT_FIELD],
    multiRequestEnabled: false,
    multiCandidatesText: "",
    multiUnique: true,
    multiRequestMethod: "GET",
    multiRequestPath: "",
    multiRequestURL: "",
    multiRequestQueryJSON: "{}",
    multiRequestHeadersJSON: "{}",
    multiRequestCookiesJSON: "{}",
    multiRequestBodyKind: "json",
    multiRequestBodyJSON: "null",
    multiRequestAcceptStatusText: "200",
    multiRequestNotFoundStatusText: "404",
    multiRequestDecodeCharset: "",
    multiSuccessMode: "and",
    multiSuccessConditionsText: "",
    workflowEnabled: false,
    workflowSelectors: [
      {
        id: "selector-1",
        name: "read_link",
        kind: "xpath",
        expr: "//a/@href",
      },
    ],
    workflowItemVariables: [],
    workflowMatchMode: "and",
    workflowMatchConditionsText: "",
    workflowExpectCountText: "",
    workflowReturn: "${item.read_link}",
    workflowNextMethod: "GET",
    workflowNextPath: "",
    workflowNextURL: "",
    workflowNextQueryJSON: "{}",
    workflowNextHeadersJSON: "{}",
    workflowNextCookiesJSON: "{}",
    workflowNextBodyKind: "json",
    workflowNextBodyJSON: "null",
    workflowNextAcceptStatusText: "200",
    workflowNextNotFoundStatusText: "404",
    workflowNextDecodeCharset: "",
    postAssign: [],
    postTitleLang: "",
    postPlotLang: "",
    postGenresLang: "",
    postActorsLang: "",
    postDisableReleaseDateCheck: false,
    postDisableNumberReplace: false,
    importYAML: "",
  };
}

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

  function patch<K extends keyof EditorState>(key: K, value: EditorState[K]) {
    setState((prev) => ({ ...prev, [key]: value }));
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

  async function run(action: RunAction) {
    try {
      const draft = buildDraft(state);
      setBusyAction(action);
      setError("");
      setToast(null);
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
        const [requestDebug, workflowDebug, scrapeDebug] = await Promise.all([
          debugPluginDraftRequest(draft, state.number.trim()),
          state.workflowEnabled ? debugPluginDraftWorkflow(draft, state.number.trim()) : Promise.resolve(null),
          debugPluginDraftScrape(draft, state.number.trim()),
        ]);
        setRequestResult(requestDebug.data);
        setWorkflowResult(workflowDebug?.data ?? null);
        setScrapeResult(scrapeDebug.data);
        setTab("basic");
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
                <div className="plugin-editor-form-grid">
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
                method={state.requestMethod}
                path={state.requestPath}
                rawURL={state.requestURL}
                queryJSON={state.requestQueryJSON}
                headersJSON={state.requestHeadersJSON}
                cookiesJSON={state.requestCookiesJSON}
                bodyKind={state.requestBodyKind}
                bodyJSON={state.requestBodyJSON}
                acceptStatusText={state.requestAcceptStatusText}
                notFoundStatusText={state.requestNotFoundStatusText}
                decodeCharset={state.requestDecodeCharset}
                onChange={(key, value) => patch(key, value)}
                nextRequestLayout
                compactJSONBlocks
                expandAdvanced
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
                    method={state.workflowNextMethod}
                    path={state.workflowNextPath}
                    rawURL={state.workflowNextURL}
                    queryJSON={state.workflowNextQueryJSON}
                    headersJSON={state.workflowNextHeadersJSON}
                    cookiesJSON={state.workflowNextCookiesJSON}
                    bodyKind={state.workflowNextBodyKind}
                    bodyJSON={state.workflowNextBodyJSON}
                    acceptStatusText={state.workflowNextAcceptStatusText}
                    notFoundStatusText={state.workflowNextNotFoundStatusText}
                    decodeCharset={state.workflowNextDecodeCharset}
                    onChange={(key, value) => patch(key, value)}
                    prefix="workflowNext"
                    expandAdvanced
                    compactJSONBlocks
                    nextRequestLayout
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
                <div key={field.id} className="plugin-editor-field-card">
                  {(() => {
                    const fieldMeta = getFieldMeta(field.name);
                    const showParserKind = Boolean(fieldMeta.parserOptions && fieldMeta.parserOptions.length > 0);
                    const showMultiSelector = typeof fieldMeta.fixedMulti !== "boolean";
                    const selectableFields = FIELD_OPTIONS.filter((option) => option === field.name || !state.fields.some((item) => item.id !== field.id && item.name === option));

                    return (
                      <>
                  <div className="plugin-editor-field-card-rows">
                    <div className="plugin-editor-field-inline-row">
                      <label className="plugin-editor-field-inline plugin-editor-field-inline-name">
                        <span>Field</span>
                        <select className="input" value={field.name} onChange={(event) => updateFieldName(field.id, event.target.value)}>
                          {selectableFields.map((option) => (
                            <option key={option} value={option}>
                              {option}
                            </option>
                          ))}
                          {!FIELD_OPTIONS.includes(field.name) ? (
                            <option value={field.name}>{field.name}</option>
                          ) : null}
                        </select>
                      </label>
                      <label className="plugin-editor-field-inline plugin-editor-field-inline-kind">
                        <span>Kind</span>
                        <select className="input" value={field.selectorKind} onChange={(event) => patchField(field.id, (prev) => ({ ...prev, selectorKind: event.target.value }))}>
                          <option value="xpath">xpath</option>
                          <option value="jsonpath">jsonpath</option>
                        </select>
                      </label>
                      <label className="plugin-editor-field-inline plugin-editor-field-inline-expr">
                        <span>Expr</span>
                        <input className="input" value={field.selectorExpr} onChange={(event) => patchField(field.id, (prev) => ({ ...prev, selectorExpr: event.target.value }))} />
                      </label>
                      <label className="searcher-debug-switch plugin-editor-field-inline-required">
                        <input type="checkbox" checked={field.required} onChange={(event) => patchField(field.id, (prev) => ({ ...prev, required: event.target.checked }))} />
                        <span>REQUIRED</span>
                      </label>
                      <button
                        className="btn btn-secondary plugin-editor-field-card-remove"
                        type="button"
                        aria-label="删除字段"
                        title="删除字段"
                        onClick={() => removeField(field.id)}
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>

                    <div className="plugin-editor-field-inline-row">
                      {showParserKind ? (
                        <label className="plugin-editor-field-inline plugin-editor-field-inline-name">
                          <span>Parse As</span>
                          <select className="input" value={field.parserKind} onChange={(event) => patchField(field.id, (prev) => ({ ...prev, parserKind: event.target.value }))}>
                            {(fieldMeta.parserOptions ?? []).map((option) => (
                              <option key={option} value={option}>
                                {option}
                              </option>
                            ))}
                            {field.parserKind && !(fieldMeta.parserOptions ?? []).includes(field.parserKind) ? (
                              <option value={field.parserKind}>{field.parserKind}</option>
                            ) : null}
                          </select>
                        </label>
                      ) : null}
                      {showParserLayout(field.parserKind) ? (
                        <label className="plugin-editor-field-inline plugin-editor-field-inline-kind">
                          <span>Layout</span>
                          <input className="input" value={field.parserLayout} onChange={(event) => patchField(field.id, (prev) => ({ ...prev, parserLayout: event.target.value }))} />
                        </label>
                      ) : null}
                      <div className="plugin-editor-field-inline-switches">
                        {showMultiSelector ? (
                          <label className="searcher-debug-switch">
                            <input type="checkbox" checked={field.selectorMulti} onChange={(event) => patchField(field.id, (prev) => ({ ...prev, selectorMulti: event.target.checked }))} />
                            <span>multi selector</span>
                          </label>
                        ) : null}
                      </div>
                    </div>

                    <label className="plugin-editor-field plugin-editor-field-wide">
                      <span>Transforms</span>
                      <div className="plugin-editor-transform-list">
                        {(field.transforms ?? []).map((transform) => (
                          <div key={transform.id} className="plugin-editor-transform-card">
                            <div className="plugin-editor-transform-actions">
                              <button
                                className="btn btn-secondary plugin-editor-transform-action"
                                type="button"
                                aria-label="新增 transform"
                                title="新增 transform"
                                onClick={() => addTransform(field.id, transform.id)}
                              >
                                <Plus size={14} />
                              </button>
                              <button
                                className="btn btn-secondary plugin-editor-transform-action"
                                type="button"
                                aria-label="删除 transform"
                                title="删除 transform"
                                onClick={() => removeTransform(field.id, transform.id)}
                              >
                                <span aria-hidden="true">×</span>
                              </button>
                            </div>
                            <label className="plugin-editor-transform-inline-field plugin-editor-transform-inline-field-kind">
                              <span>Kind</span>
                              <select
                                className="input"
                                value={transform.kind}
                                onChange={(event) => patchTransform(field.id, transform.id, (prev) => ({ ...prev, kind: event.target.value }))}
                              >
                                <option value="trim">trim</option>
                                <option value="trim_prefix">trim_prefix</option>
                                <option value="trim_suffix">trim_suffix</option>
                                <option value="trim_charset">trim_charset</option>
                                <option value="replace">replace</option>
                                <option value="regex_extract">regex_extract</option>
                                <option value="split_index">split_index</option>
                                <option value="split">split</option>
                                <option value="map_trim">map_trim</option>
                                <option value="remove_empty">remove_empty</option>
                                <option value="dedupe">dedupe</option>
                                <option value="to_upper">to_upper</option>
                                <option value="to_lower">to_lower</option>
                              </select>
                            </label>
                            <TransformParamFields
                              transform={transform}
                              onChange={(updater) => patchTransform(field.id, transform.id, updater)}
                            />
                          </div>
                        ))}
                      </div>
                    </label>
                  </div>
                      </>
                    );
                  })()}
                </div>
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
                  <label className="plugin-editor-field">
                    <span>Title Lang</span>
                    <select className="input" value={state.postTitleLang} onChange={(event) => patch("postTitleLang", event.target.value)}>
                      <option value="">DEFAULT</option>
                      {META_LANG_OPTIONS.map((option) => (
                        <option key={option} value={option}>
                          {option.toUpperCase()}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="plugin-editor-field">
                    <span>Plot Lang</span>
                    <select className="input" value={state.postPlotLang} onChange={(event) => patch("postPlotLang", event.target.value)}>
                      <option value="">DEFAULT</option>
                      {META_LANG_OPTIONS.map((option) => (
                        <option key={option} value={option}>
                          {option.toUpperCase()}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="plugin-editor-field">
                    <span>Genres Lang</span>
                    <select className="input" value={state.postGenresLang} onChange={(event) => patch("postGenresLang", event.target.value)}>
                      <option value="">DEFAULT</option>
                      {META_LANG_OPTIONS.map((option) => (
                        <option key={option} value={option}>
                          {option.toUpperCase()}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="plugin-editor-field">
                    <span>Actors Lang</span>
                    <select className="input" value={state.postActorsLang} onChange={(event) => patch("postActorsLang", event.target.value)}>
                      <option value="">DEFAULT</option>
                      {META_LANG_OPTIONS.map((option) => (
                        <option key={option} value={option}>
                          {option.toUpperCase()}
                        </option>
                      ))}
                    </select>
                  </label>
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
        <div className="plugin-editor-modal-backdrop" role="presentation" onClick={() => setImportOpen(false)}>
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
              <button className="btn btn-secondary plugin-editor-modal-close" type="button" aria-label="关闭导入窗口" title="关闭导入窗口" onClick={() => setImportOpen(false)}>
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
                <button className="btn btn-secondary plugin-editor-modal-example-btn" type="button" onClick={() => setExampleOpen(true)}>
                  查看 YAML 示例
                </button>
              </div>
              <label className="plugin-editor-field plugin-editor-modal-editor">
                <span>Plugin YAML</span>
                <textarea
                  className="input plugin-editor-textarea plugin-editor-textarea-lg plugin-editor-modal-textarea"
                  value={state.importYAML}
                  onChange={(event) => patch("importYAML", event.target.value)}
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
              <button className="btn btn-secondary" type="button" onClick={() => setImportOpen(false)} disabled={busyAction !== ""}>
                取消
              </button>
              <button className="btn btn-primary plugin-editor-modal-submit" type="button" onClick={() => void handleImportYAML()} disabled={busyAction !== ""}>
                {busyAction === "import" ? <LoaderCircle size={16} className="ruleset-debug-spinner" /> : <Import size={16} />}
                <span>导入 YAML</span>
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {exampleOpen ? (
        <div className="plugin-editor-modal-backdrop" role="presentation" onClick={() => setExampleOpen(false)}>
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
              <button className="btn btn-secondary plugin-editor-modal-close" type="button" aria-label="关闭示例窗口" title="关闭示例窗口" onClick={() => setExampleOpen(false)}>
                <X size={16} />
              </button>
            </div>
            <div className="plugin-editor-modal-body">
              <pre className="plugin-editor-modal-example-code plugin-editor-modal-example-code-dialog">{IMPORT_YAML_EXAMPLE}</pre>
            </div>
            <div className="plugin-editor-modal-actions">
              <button className="btn btn-secondary" type="button" onClick={() => setExampleOpen(false)}>
                关闭
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {toast ? (
        <div className="file-list-toast" data-tone={toast.tone === "danger" ? "danger" : undefined}>
          {toast.message}
        </div>
      ) : null}
    </div>
  );
}

function RequestForm(props: {
  method: string;
  path: string;
  rawURL: string;
  queryJSON: string;
  headersJSON: string;
  cookiesJSON: string;
  bodyKind: string;
  bodyJSON: string;
  acceptStatusText: string;
  notFoundStatusText: string;
  decodeCharset: string;
  prefix?: "workflowNext" | "multiRequest";
  expandAdvanced?: boolean;
  compactJSONBlocks?: boolean;
  nextRequestLayout?: boolean;
  onChange: <K extends keyof EditorState>(key: K, value: EditorState[K]) => void;
}) {
  const prefix = props.prefix ?? "request";
  const key = <K extends keyof EditorState>(name: string) => `${prefix}${name}` as K;
  const targetPathKey = key("Path");
  const targetURLKey = key("URL");
  const targetMode = props.rawURL ? "url" : "path";
  const targetValue = targetMode === "url" ? props.rawURL : props.path;

  function handleTargetModeChange(mode: "path" | "url") {
    const current = targetValue;
    if (mode === "url") {
      props.onChange(targetPathKey, "" as EditorState[typeof targetPathKey]);
      props.onChange(targetURLKey, current as EditorState[typeof targetURLKey]);
      return;
    }
    props.onChange(targetPathKey, current as EditorState[typeof targetPathKey]);
    props.onChange(targetURLKey, "" as EditorState[typeof targetURLKey]);
  }

  function handleTargetValueChange(value: string) {
    if (targetMode === "url") {
      props.onChange(targetURLKey, value as EditorState[typeof targetURLKey]);
      return;
    }
    props.onChange(targetPathKey, value as EditorState[typeof targetPathKey]);
  }

  return (
    <>
      {props.nextRequestLayout ? (
        <>
          <div className="plugin-editor-request-inline-row">
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-method plugin-editor-request-inline-field-next-top">
              <span>Method</span>
              <select className="input" value={props.method} onChange={(event) => props.onChange(key("Method"), event.target.value as EditorState[keyof EditorState])}>
                <option value="GET">GET</option>
                <option value="POST">POST</option>
              </select>
            </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xs plugin-editor-request-inline-field-next-top">
            <span>Target Type</span>
            <select className="input" value={targetMode} onChange={(event) => handleTargetModeChange(event.target.value as "path" | "url")}>
              <option value="path">path</option>
              <option value="url">url</option>
            </select>
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-lg plugin-editor-request-inline-field-next-top">
            <span>{targetMode === "url" ? "URL" : "Path"}</span>
            <input
              className="input"
              value={targetValue}
              onChange={(event) => handleTargetValueChange(event.target.value)}
              placeholder={targetMode === "url" ? "例如 https://example.com/${number}" : "例如 /search/${number}"}
            />
          </label>
        </div>
          <div className="plugin-editor-request-inline-row plugin-editor-request-inline-row-next-meta">
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-accept">
              <span>Accept Status</span>
              <input className="input" value={props.acceptStatusText} onChange={(event) => props.onChange(key("AcceptStatusText"), event.target.value as EditorState[keyof EditorState])} placeholder="200,302" />
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-next-not-found">
              <span title="Not Found Status">Not Found Status</span>
              <input className="input" value={props.notFoundStatusText} onChange={(event) => props.onChange(key("NotFoundStatusText"), event.target.value as EditorState[keyof EditorState])} placeholder="404" />
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-content-type">
              <span>Content-Type</span>
              <select className="input" value={props.bodyKind} onChange={(event) => props.onChange(key("BodyKind"), event.target.value as EditorState[keyof EditorState])}>
                <option value="json">json</option>
                <option value="form">form</option>
                <option value="raw">raw</option>
              </select>
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-next-charset">
              <span title="Decode Charset">Decode Charset</span>
              <input className="input" value={props.decodeCharset} onChange={(event) => props.onChange(key("DecodeCharset"), event.target.value as EditorState[keyof EditorState])} placeholder="例如 euc-jp" />
            </label>
          </div>
        </>
      ) : (
        <div className="plugin-editor-request-inline-row">
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-method">
            <span>Method</span>
            <select className="input" value={props.method} onChange={(event) => props.onChange(key("Method"), event.target.value as EditorState[keyof EditorState])}>
              <option value="GET">GET</option>
              <option value="POST">POST</option>
            </select>
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xs">
            <span>Target Type</span>
            <select className="input" value={targetMode} onChange={(event) => handleTargetModeChange(event.target.value as "path" | "url")}>
              <option value="path">path</option>
              <option value="url">url</option>
            </select>
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-lg">
            <span>{targetMode === "url" ? "URL" : "Path"}</span>
            <input
              className="input"
              value={targetValue}
              onChange={(event) => handleTargetValueChange(event.target.value)}
              placeholder={targetMode === "url" ? "例如 https://example.com/${number}" : "例如 /search/${number}"}
            />
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-accept">
            <span>Accept Status</span>
            <input className="input" value={props.acceptStatusText} onChange={(event) => props.onChange(key("AcceptStatusText"), event.target.value as EditorState[keyof EditorState])} placeholder="200,302" />
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-content-type">
            <span>Content-Type</span>
            <select className="input" value={props.bodyKind} onChange={(event) => props.onChange(key("BodyKind"), event.target.value as EditorState[keyof EditorState])}>
              <option value="json">json</option>
              <option value="form">form</option>
              <option value="raw">raw</option>
            </select>
          </label>
        </div>
      )}
      {props.compactJSONBlocks ? (
        <div className="plugin-editor-request-json-stack">
          <details className="plugin-editor-request-json-detail">
            <summary>
              <span>Header JSON</span>
              <span className={`plugin-editor-request-json-count ${jsonKeyCount(props.headersJSON) > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>
                {jsonKeyCount(props.headersJSON) > 0 ? jsonKeyCount(props.headersJSON) : 0}
              </span>
            </summary>
            <textarea className="input plugin-editor-textarea" value={props.headersJSON} onChange={(event) => props.onChange(key("HeadersJSON"), event.target.value as EditorState[keyof EditorState])} onKeyDown={handleEditorTextareaKeyDown} />
          </details>
          <details className="plugin-editor-request-json-detail">
            <summary>
              <span>Cookie JSON</span>
              <span className={`plugin-editor-request-json-count ${jsonKeyCount(props.cookiesJSON) > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>
                {jsonKeyCount(props.cookiesJSON) > 0 ? jsonKeyCount(props.cookiesJSON) : 0}
              </span>
            </summary>
            <textarea className="input plugin-editor-textarea" value={props.cookiesJSON} onChange={(event) => props.onChange(key("CookiesJSON"), event.target.value as EditorState[keyof EditorState])} onKeyDown={handleEditorTextareaKeyDown} />
          </details>
          <details className="plugin-editor-request-json-detail">
            <summary>
              <span>Query JSON</span>
              <span className={`plugin-editor-request-json-count ${jsonKeyCount(props.queryJSON) > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>
                {jsonKeyCount(props.queryJSON) > 0 ? jsonKeyCount(props.queryJSON) : 0}
              </span>
            </summary>
            <textarea className="input plugin-editor-textarea" value={props.queryJSON} onChange={(event) => props.onChange(key("QueryJSON"), event.target.value as EditorState[keyof EditorState])} onKeyDown={handleEditorTextareaKeyDown} />
          </details>
          <details className="plugin-editor-request-json-detail">
            <summary>
              <span>Body</span>
              <span className={`plugin-editor-request-json-count ${jsonKeyCount(props.bodyJSON) > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>
                {jsonKeyCount(props.bodyJSON) > 0 ? jsonKeyCount(props.bodyJSON) : 0}
              </span>
            </summary>
            <textarea className="input plugin-editor-textarea" value={props.bodyJSON} onChange={(event) => props.onChange(key("BodyJSON"), event.target.value as EditorState[keyof EditorState])} onKeyDown={handleEditorTextareaKeyDown} />
          </details>
        </div>
      ) : (
        <div className="plugin-editor-json-grid">
          <label className="plugin-editor-field">
            <span>Query JSON</span>
            <textarea className="input plugin-editor-textarea" value={props.queryJSON} onChange={(event) => props.onChange(key("QueryJSON"), event.target.value as EditorState[keyof EditorState])} onKeyDown={handleEditorTextareaKeyDown} />
          </label>
          <label className="plugin-editor-field">
            <span>Headers JSON</span>
            <textarea className="input plugin-editor-textarea" value={props.headersJSON} onChange={(event) => props.onChange(key("HeadersJSON"), event.target.value as EditorState[keyof EditorState])} onKeyDown={handleEditorTextareaKeyDown} />
          </label>
          <label className="plugin-editor-field">
            <span>Body</span>
            <textarea className="input plugin-editor-textarea" value={props.bodyJSON} onChange={(event) => props.onChange(key("BodyJSON"), event.target.value as EditorState[keyof EditorState])} onKeyDown={handleEditorTextareaKeyDown} />
          </label>
        </div>
      )}
      {props.expandAdvanced && !props.nextRequestLayout ? (
        <div className="plugin-editor-request-advanced-open">
          <div className="plugin-editor-request-inline-row plugin-editor-advanced-grid">
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
              <span>Not Found Status</span>
              <input className="input" value={props.notFoundStatusText} onChange={(event) => props.onChange(key("NotFoundStatusText"), event.target.value as EditorState[keyof EditorState])} placeholder="404" />
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
              <span>Decode Charset</span>
              <input className="input" value={props.decodeCharset} onChange={(event) => props.onChange(key("DecodeCharset"), event.target.value as EditorState[keyof EditorState])} placeholder="例如 euc-jp" />
            </label>
          </div>
        </div>
      ) : !props.expandAdvanced ? (
        <details className="plugin-editor-advanced">
          <summary>高级选项</summary>
          <div className="plugin-editor-request-inline-row plugin-editor-advanced-grid">
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
              <span>Not Found Status</span>
              <input className="input" value={props.notFoundStatusText} onChange={(event) => props.onChange(key("NotFoundStatusText"), event.target.value as EditorState[keyof EditorState])} placeholder="404" />
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
              <span>Decode Charset</span>
              <input className="input" value={props.decodeCharset} onChange={(event) => props.onChange(key("DecodeCharset"), event.target.value as EditorState[keyof EditorState])} placeholder="例如 euc-jp" />
            </label>
          </div>
          <div className="plugin-editor-form-grid plugin-editor-advanced-grid">
            <label className="plugin-editor-field plugin-editor-field-wide">
              <span>Cookies JSON</span>
              <textarea className="input plugin-editor-textarea" value={props.cookiesJSON} onChange={(event) => props.onChange(key("CookiesJSON"), event.target.value as EditorState[keyof EditorState])} onKeyDown={handleEditorTextareaKeyDown} />
            </label>
          </div>
        </details>
      ) : null}
    </>
  );
}

function RequestBasicPanel({ result }: { result: PluginEditorRequestDebugResult | null }) {
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

function RequestDetailPanel({ request }: { request?: PluginEditorRequestDebugResult["request"] | null }) {
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

function ResponseDetailPanel({ response }: { response?: PluginEditorRequestDebugResult["response"] | null }) {
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

function WorkflowOutputPanel({ result }: { result: PluginEditorWorkflowDebugResult | null }) {
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
          {step.items?.length ? <div className="plugin-editor-timeline-detail">matched items: {step.items.filter((item) => item.matched).length}</div> : null}
        </article>
      ))}
    </div>
  );
}

function ScrapeJSONPanel({ result }: { result: PluginEditorScrapeDebugResult | null }) {
  if (!result) {
    return <div className="ruleset-debug-empty">暂无抓取结果。</div>;
  }
  return (
    <div className="plugin-editor-output-section plugin-editor-output-section-fill">
      <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-json-fill">{JSON.stringify(result.meta ?? result.fields, null, 2)}</pre>
    </div>
  );
}

function HeaderList({ headers }: { headers: Record<string, string> }) {
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

function runResponseExpr(input: { body: string; expr: string; kind: "xpath" | "jsonpath"; contentType: string }) {
  const expr = input.expr.trim();
  if (!expr) {
    return "请输入表达式。";
  }
  try {
    if (input.kind === "xpath") {
      return runXPathExpr(input.body, expr);
    }
    return runJSONExpr(input.body, expr);
  } catch (error) {
    return error instanceof Error ? error.message : "表达式执行失败。";
  }
}

function runXPathExpr(body: string, expr: string) {
  const parser = new DOMParser();
  const doc = parser.parseFromString(body, "text/html");
  const result = doc.evaluate(expr, doc, null, XPathResult.ANY_TYPE, null);
  const values: string[] = [];
  switch (result.resultType) {
    case XPathResult.STRING_TYPE:
      return result.stringValue || "";
    case XPathResult.NUMBER_TYPE:
      return String(result.numberValue);
    case XPathResult.BOOLEAN_TYPE:
      return String(result.booleanValue);
    default: {
      let node = result.iterateNext();
      while (node) {
        if ("textContent" in node) {
          values.push(node.textContent ?? "");
        } else {
          values.push(String(node));
        }
        node = result.iterateNext();
      }
      return values.length > 0 ? JSON.stringify(values, null, 2) : "无匹配结果";
    }
  }
}

function runJSONExpr(body: string, expr: string) {
  const data = JSON.parse(body);
  const normalized = expr.replace(/^\$\./, "").replace(/^\$/, "");
  if (!normalized) {
    return JSON.stringify(data, null, 2);
  }
  const tokens = normalized.split(".").filter(Boolean);
  let current: unknown[] = [data];
  for (const token of tokens) {
    const next: unknown[] = [];
    const arrayMatch = token.match(/^([A-Za-z0-9_-]+)\[\*\]$/);
    const indexMatch = token.match(/^([A-Za-z0-9_-]+)\[(\d+)\]$/);
    for (const item of current) {
      if (arrayMatch) {
        const value = item && typeof item === "object" ? (item as Record<string, unknown>)[arrayMatch[1]] : undefined;
        if (Array.isArray(value)) {
          next.push(...value);
        }
        continue;
      }
      if (indexMatch) {
        const value = item && typeof item === "object" ? (item as Record<string, unknown>)[indexMatch[1]] : undefined;
        if (Array.isArray(value)) {
          next.push(value[Number(indexMatch[2])]);
        }
        continue;
      }
      if (item && typeof item === "object") {
        next.push((item as Record<string, unknown>)[token]);
      }
    }
    current = next.filter((item) => item !== undefined);
  }
  if (current.length === 0) {
    return "无匹配结果";
  }
  if (current.length === 1) {
    return typeof current[0] === "string" ? current[0] : JSON.stringify(current[0], null, 2);
  }
  return JSON.stringify(current, null, 2);
}

function KVPairEditor(props: {
  items: KVPairForm[];
  emptyLabel: string;
  keyLabel: string;
  valueLabel: string;
  keyOptions?: string[];
  valuePlaceholder?: string;
  onAdd: () => void;
  onRemove: (id: string) => void;
  onChange: (id: string, updater: (item: KVPairForm) => KVPairForm) => void;
}) {
  return (
    <div className="plugin-editor-kv-list">
      {props.items.length === 0 ? <div className="ruleset-debug-empty">{props.emptyLabel}</div> : null}
      {props.items.map((item) => {
        const selectableKeys = props.keyOptions
          ? props.keyOptions.filter((option) => option === item.key || !props.items.some((other) => other.id !== item.id && other.key === option))
          : [];
        return (
          <div key={item.id} className="plugin-editor-kv-row plugin-editor-kv-row-compact">
            <div className="plugin-editor-transform-actions plugin-editor-kv-actions">
              <button className="btn btn-secondary plugin-editor-transform-action" type="button" aria-label="新增项" title="新增项" onClick={props.onAdd}>
                <Plus size={14} />
              </button>
              <button className="btn btn-secondary plugin-editor-transform-action" type="button" aria-label="删除项" title="删除项" onClick={() => props.onRemove(item.id)}>
                <span aria-hidden="true">×</span>
              </button>
            </div>
            <label className="plugin-editor-field-inline plugin-editor-kv-inline-key">
              <span>{props.keyLabel}</span>
              {props.keyOptions ? (
                <select className="input" value={item.key} onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, key: event.target.value }))}>
                  <option value="">Select field</option>
                  {selectableKeys.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                  {item.key && !props.keyOptions.includes(item.key) ? <option value={item.key}>{item.key}</option> : null}
                </select>
              ) : (
                <input className="input" value={item.key} onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, key: event.target.value }))} />
              )}
            </label>
            <label className="plugin-editor-field-inline plugin-editor-kv-inline-value">
              <span>{props.valueLabel}</span>
              <input
                className="input"
                value={item.value}
                placeholder={props.valuePlaceholder}
                onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, value: event.target.value }))}
              />
            </label>
          </div>
        );
      })}
      {props.items.length === 0 ? (
        <div className="plugin-editor-inline-actions">
          <button className="btn btn-secondary plugin-editor-transform-action" type="button" aria-label="新增项" title="新增项" onClick={props.onAdd}>
            <Plus size={14} />
          </button>
        </div>
      ) : null}
    </div>
  );
}

function WorkflowItemVariablesEditor(props: {
  items: KVPairForm[];
  emptyLabel?: string;
  keyLabel?: string;
  valueLabel?: string;
  valuePlaceholder?: string;
  onAdd: () => void;
  onRemove: (id: string) => void;
  onChange: (id: string, updater: (item: KVPairForm) => KVPairForm) => void;
}) {
  return (
    <div className="plugin-editor-kv-list">
      {props.items.length === 0 ? <div className="ruleset-debug-empty">{props.emptyLabel ?? "暂未定义 item_variables。"}</div> : null}
      {props.items.map((item) => (
        <div key={item.id} className="plugin-editor-kv-row plugin-editor-kv-row-compact">
          <label className="plugin-editor-field-inline plugin-editor-kv-inline-key">
            <span>{props.keyLabel ?? "Name"}</span>
            <input className="input" value={item.key} onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, key: event.target.value }))} />
          </label>
          <label className="plugin-editor-field-inline plugin-editor-kv-inline-value">
            <span>{props.valueLabel ?? "Template"}</span>
            <input
              className="input"
              value={item.value}
              placeholder={props.valuePlaceholder}
              onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, value: event.target.value }))}
            />
          </label>
          <div className="plugin-editor-transform-actions plugin-editor-kv-actions">
            <button className="btn btn-secondary plugin-editor-transform-action" type="button" aria-label="新增变量" title="新增变量" onClick={props.onAdd}>
              <Plus size={14} />
            </button>
            <button className="btn btn-secondary plugin-editor-transform-action" type="button" aria-label="删除变量" title="删除变量" onClick={() => props.onRemove(item.id)}>
              <Trash2 size={14} />
            </button>
          </div>
        </div>
      ))}
      {props.items.length === 0 ? (
        <div className="plugin-editor-inline-actions">
          <button className="btn btn-secondary plugin-editor-transform-action" type="button" aria-label="新增变量" title="新增变量" onClick={props.onAdd}>
            <Plus size={14} />
          </button>
        </div>
      ) : null}
    </div>
  );
}

function TransformParamFields(props: {
  transform: TransformForm;
  onChange: (updater: (prev: TransformForm) => TransformForm) => void;
}) {
  const { transform } = props;
  const paramCount =
    (needsOldNew(transform.kind) ? 2 : 0) +
    (needsValue(transform.kind) ? 1 : 0) +
    (needsSep(transform.kind) ? 1 : 0) +
    (needsCutset(transform.kind) ? 1 : 0) +
    (needsIndex(transform.kind) ? 1 : 0);

  return (
    <>
      {needsOldNew(transform.kind) ? (
        <>
          <label className="plugin-editor-transform-inline-field">
            <span>Old</span>
            <input className="input" value={transform.old} onChange={(event) => props.onChange((prev) => ({ ...prev, old: event.target.value }))} />
          </label>
          <label className="plugin-editor-transform-inline-field">
            <span>New</span>
            <input className="input" value={transform.newValue} onChange={(event) => props.onChange((prev) => ({ ...prev, newValue: event.target.value }))} />
          </label>
        </>
      ) : null}
      {needsValue(transform.kind) ? (
        <label className="plugin-editor-transform-inline-field">
          <span>{valueLabelForKind(transform.kind)}</span>
          <input className="input" value={transform.value} onChange={(event) => props.onChange((prev) => ({ ...prev, value: event.target.value }))} />
        </label>
      ) : null}
      {needsSep(transform.kind) ? (
        <label className="plugin-editor-transform-inline-field">
          <span>Sep</span>
          <input className="input" value={transform.sep} onChange={(event) => props.onChange((prev) => ({ ...prev, sep: event.target.value }))} />
        </label>
      ) : null}
      {needsCutset(transform.kind) ? (
        <label className="plugin-editor-transform-inline-field">
          <span>Cutset</span>
          <input className="input" value={transform.cutset} onChange={(event) => props.onChange((prev) => ({ ...prev, cutset: event.target.value }))} />
        </label>
      ) : null}
      {needsIndex(transform.kind) ? (
        <label className="plugin-editor-transform-inline-field plugin-editor-transform-inline-field-index">
          <span>Index</span>
          <input className="input" value={transform.index} onChange={(event) => props.onChange((prev) => ({ ...prev, index: event.target.value }))} />
        </label>
      ) : null}
      {paramCount === 1 ? <div className="plugin-editor-transform-inline-spacer" aria-hidden="true" /> : null}
    </>
  );
}

function buildDraft(state: EditorState): PluginEditorDraft {
  const hosts = splitLines(state.hostsText);
  if (hosts.length === 0) {
    throw new Error("至少需要一个 host。");
  }
  const fields = state.fields.reduce<Record<string, PluginEditorField>>((acc, field) => {
    const name = field.name.trim();
    if (!name) {
      return acc;
    }
    const parser =
      field.parserLayout.trim() === ""
        ? field.parserKind.trim()
        : {
            kind: field.parserKind.trim(),
            layout: field.parserLayout.trim(),
          };
    acc[name] = {
      selector: {
        kind: field.selectorKind.trim(),
        expr: field.selectorExpr.trim(),
        multi: field.selectorMulti,
      },
      transforms: field.transforms.map(transformFormToSpec).filter((item) => item.kind),
      parser,
      required: field.required,
    };
    return acc;
  }, {});
  if (Object.keys(fields).length === 0) {
    throw new Error("至少需要一个 scrape field。");
  }
  const draft: PluginEditorDraft = {
    version: 1,
    name: state.name.trim(),
    type: state.type,
    hosts,
    request: buildRequestFromState(state),
    scrape: {
      format: state.scrapeFormat,
      fields,
    },
  };
  const precheckPatterns = splitLines(state.precheckPatternsText);
  const precheckVariables = pairsToRecord(state.precheckVariables);
  if (precheckPatterns.length > 0 || Object.keys(precheckVariables).length > 0) {
    draft.precheck = {
      number_patterns: precheckPatterns,
      variables: precheckVariables,
    };
  }
  if (state.multiRequestEnabled) {
    const baseRequest = buildRequestFromState(state);
    draft.request = null;
    draft.multi_request = {
      candidates: splitLines(state.multiCandidatesText),
      unique: true,
      request: baseRequest,
      success_when: {
        mode: state.multiSuccessMode,
        conditions: splitLines(state.multiSuccessConditionsText),
      },
    };
  }
  if (state.workflowEnabled) {
    draft.workflow = {
      search_select: {
        selectors: state.workflowSelectors
          .map((item) => ({
            name: item.name.trim(),
            kind: item.kind.trim(),
            expr: item.expr.trim(),
          }))
          .filter((item) => item.name && item.kind && item.expr),
        item_variables: pairsToRecord(state.workflowItemVariables),
        match: {
          mode: state.workflowMatchMode,
          conditions: splitLines(state.workflowMatchConditionsText),
          expect_count: parseOptionalInteger(state.workflowExpectCountText),
        },
        return: state.workflowReturn.trim(),
        next_request: buildWorkflowNextRequestFromState(state),
      },
    };
  }
  const assign = pairsToRecord(state.postAssign);
  const defaults = buildDefaults(state);
  const switchConfig = buildSwitchConfig(state);
  if ((assign && Object.keys(assign).length > 0) || defaults || switchConfig) {
    draft.postprocess = {
      assign,
      defaults,
      switch_config: switchConfig,
    };
  }
  return draft;
}

function buildRequestFromState(state: EditorState): NonNullable<PluginEditorDraft["request"]> {
  return {
    method: state.requestMethod.trim() || "GET",
    path: state.requestPath.trim() || undefined,
    url: state.requestURL.trim() || undefined,
    query: parseStringRecord(state.requestQueryJSON, "request query"),
    headers: parseStringRecord(state.requestHeadersJSON, "request headers"),
    cookies: parseStringRecord(state.requestCookiesJSON, "request cookies"),
    body: buildRequestBody(state.requestBodyKind, state.requestBodyJSON, "request body"),
    accept_status_codes: parseIntegerList(state.requestAcceptStatusText),
    not_found_status_codes: parseIntegerList(state.requestNotFoundStatusText),
    response: state.requestDecodeCharset.trim() ? { decode_charset: state.requestDecodeCharset.trim() } : undefined,
  };
}

function buildWorkflowNextRequestFromState(state: EditorState): NonNullable<NonNullable<PluginEditorDraft["workflow"]>["search_select"]>["next_request"] {
  return {
    method: state.workflowNextMethod.trim() || "GET",
    path: state.workflowNextPath.trim() || undefined,
    url: state.workflowNextURL.trim() || undefined,
    query: parseStringRecord(state.workflowNextQueryJSON, "workflow next query"),
    headers: parseStringRecord(state.workflowNextHeadersJSON, "workflow next headers"),
    cookies: parseStringRecord(state.workflowNextCookiesJSON, "workflow next cookies"),
    body: buildRequestBody(state.workflowNextBodyKind, state.workflowNextBodyJSON, "workflow next body"),
    accept_status_codes: parseIntegerList(state.workflowNextAcceptStatusText),
    not_found_status_codes: parseIntegerList(state.workflowNextNotFoundStatusText),
    response: state.workflowNextDecodeCharset.trim() ? { decode_charset: state.workflowNextDecodeCharset.trim() } : undefined,
  };
}

function buildRequestBody(kind: string, value: string, label: string): NonNullable<PluginEditorDraft["request"]>["body"] {
  const trimmed = value.trim();
  if (!trimmed || trimmed === "null") {
    return null;
  }
  if (kind === "raw") {
    return {
      kind,
      content: value,
    };
  }
  const parsed = parseJSON<Record<string, unknown>>(value, label);
  const values = Object.entries(parsed ?? {}).reduce<Record<string, string>>((acc, [key, item]) => {
    acc[key] = item == null ? "" : String(item);
    return acc;
  }, {});
  return {
    kind: kind || "json",
    values,
  };
}

function parseStringRecord(value: string, label: string) {
  const parsed = parseJSON<Record<string, unknown>>(normalizeJSONObjectText(value), label);
  return Object.entries(parsed ?? {}).reduce<Record<string, string>>((acc, [key, item]) => {
    acc[key] = item == null ? "" : String(item);
    return acc;
  }, {});
}

function stateFromDraft(draft: PluginEditorDraft): EditorState {
  const next = defaultState();
  next.name = draft.name ?? next.name;
  next.type = draft.type ?? next.type;
  next.hostsText = (draft.hosts ?? []).join("\n");
  next.precheckPatternsText = (draft.precheck?.number_patterns ?? []).join("\n");
  next.precheckVariables = recordToPairs(draft.precheck?.variables);
  next.scrapeFormat = draft.scrape?.format ?? next.scrapeFormat;
  next.fields = draftToFields(draft);
  next.postAssign = recordToPairs(draft.postprocess?.assign);
  next.postTitleLang = draft.postprocess?.defaults?.title_lang ?? next.postTitleLang;
  next.postPlotLang = draft.postprocess?.defaults?.plot_lang ?? next.postPlotLang;
  next.postGenresLang = draft.postprocess?.defaults?.genres_lang ?? "";
  next.postActorsLang = draft.postprocess?.defaults?.actors_lang ?? "";
  next.postDisableReleaseDateCheck = Boolean(draft.postprocess?.switch_config?.disable_release_date_check);
  next.postDisableNumberReplace = Boolean(draft.postprocess?.switch_config?.disable_number_replace);

  if (draft.multi_request) {
    next.multiRequestEnabled = true;
    next.requestMethod = draft.multi_request.request?.method ?? next.requestMethod;
    next.requestPath = draft.multi_request.request?.path ?? "";
    next.requestURL = draft.multi_request.request?.url ?? "";
    next.requestQueryJSON = JSON.stringify(draft.multi_request.request?.query ?? {}, null, 2);
    next.requestHeadersJSON = JSON.stringify(draft.multi_request.request?.headers ?? {}, null, 2);
    next.requestCookiesJSON = JSON.stringify(draft.multi_request.request?.cookies ?? {}, null, 2);
    next.requestBodyKind = draft.multi_request.request?.body?.kind ?? "json";
    next.requestBodyJSON = stringifyRequestBody(draft.multi_request.request?.body ?? null);
    next.requestAcceptStatusText = (draft.multi_request.request?.accept_status_codes ?? []).join(",");
    next.requestNotFoundStatusText = (draft.multi_request.request?.not_found_status_codes ?? []).join(",");
    next.requestDecodeCharset = draft.multi_request.request?.response?.decode_charset ?? "";
    next.multiCandidatesText = (draft.multi_request.candidates ?? []).join("\n");
    next.multiUnique = true;
    next.multiSuccessMode = draft.multi_request.success_when?.mode ?? "and";
    next.multiSuccessConditionsText = (draft.multi_request.success_when?.conditions ?? []).join("\n");
  } else if (draft.request) {
    next.requestMethod = draft.request.method ?? next.requestMethod;
    next.requestPath = draft.request.path ?? "";
    next.requestURL = draft.request.url ?? "";
    next.requestQueryJSON = JSON.stringify(draft.request.query ?? {}, null, 2);
    next.requestHeadersJSON = JSON.stringify(draft.request.headers ?? {}, null, 2);
    next.requestCookiesJSON = JSON.stringify(draft.request.cookies ?? {}, null, 2);
    next.requestBodyKind = draft.request.body?.kind ?? "json";
    next.requestBodyJSON = stringifyRequestBody(draft.request.body ?? null);
    next.requestAcceptStatusText = (draft.request.accept_status_codes ?? []).join(",");
    next.requestNotFoundStatusText = (draft.request.not_found_status_codes ?? []).join(",");
    next.requestDecodeCharset = draft.request.response?.decode_charset ?? "";
  }

  const searchSelect = draft.workflow?.search_select;
  if (searchSelect) {
    next.workflowEnabled = true;
    next.workflowSelectors =
      searchSelect.selectors?.map((item, index) => ({
        id: `selector-${index + 1}`,
        name: item.name ?? "",
        kind: item.kind ?? "xpath",
        expr: item.expr ?? "",
      })) ?? next.workflowSelectors;
    next.workflowItemVariables = recordToPairs(searchSelect.item_variables);
    next.workflowMatchMode = searchSelect.match?.mode ?? "and";
    next.workflowMatchConditionsText = (searchSelect.match?.conditions ?? []).join("\n");
    next.workflowExpectCountText =
      typeof searchSelect.match?.expect_count === "number" && searchSelect.match.expect_count > 0
        ? String(searchSelect.match.expect_count)
        : "";
    next.workflowReturn = searchSelect.return ?? "${item.read_link}";
    next.workflowNextMethod = searchSelect.next_request?.method ?? "GET";
    next.workflowNextPath = searchSelect.next_request?.path ?? "";
    next.workflowNextURL = searchSelect.next_request?.url ?? "";
    next.workflowNextQueryJSON = JSON.stringify(searchSelect.next_request?.query ?? {}, null, 2);
    next.workflowNextHeadersJSON = JSON.stringify(searchSelect.next_request?.headers ?? {}, null, 2);
    next.workflowNextCookiesJSON = JSON.stringify(searchSelect.next_request?.cookies ?? {}, null, 2);
    next.workflowNextBodyKind = searchSelect.next_request?.body?.kind ?? "json";
    next.workflowNextBodyJSON = stringifyRequestBody(searchSelect.next_request?.body ?? null);
    next.workflowNextAcceptStatusText = (searchSelect.next_request?.accept_status_codes ?? []).join(",");
    next.workflowNextNotFoundStatusText = (searchSelect.next_request?.not_found_status_codes ?? []).join(",");
    next.workflowNextDecodeCharset = searchSelect.next_request?.response?.decode_charset ?? "";
  }
  return next;
}

function pairsToRecord(items: KVPairForm[]) {
  return items.reduce<Record<string, string>>((acc, item) => {
    const key = item.key.trim();
    const value = item.value.trim();
    if (!key) {
      return acc;
    }
    acc[key] = value;
    return acc;
  }, {});
}

function recordToPairs(record?: Record<string, string> | null): KVPairForm[] {
  return Object.entries(record ?? {}).map(([key, value], index) => ({
    id: `kv-${index + 1}-${key}`,
    key,
    value,
  }));
}

function buildDefaults(state: EditorState): NonNullable<NonNullable<PluginEditorDraft["postprocess"]>["defaults"]> | null {
  const defaults = {
    title_lang: state.postTitleLang.trim(),
    plot_lang: state.postPlotLang.trim(),
    genres_lang: state.postGenresLang.trim(),
    actors_lang: state.postActorsLang.trim(),
  };
  if (!defaults.title_lang && !defaults.plot_lang && !defaults.genres_lang && !defaults.actors_lang) {
    return null;
  }
  return defaults;
}

function buildSwitchConfig(state: EditorState): NonNullable<NonNullable<PluginEditorDraft["postprocess"]>["switch_config"]> | null {
  if (!state.postDisableReleaseDateCheck && !state.postDisableNumberReplace) {
    return null;
  }
  return {
    disable_release_date_check: state.postDisableReleaseDateCheck,
    disable_number_replace: state.postDisableNumberReplace,
  };
}

function draftToFields(draft: PluginEditorDraft): FieldForm[] {
  const entries = Object.entries(draft.scrape?.fields ?? {});
  if (entries.length === 0) {
    return [DEFAULT_FIELD];
  }
  return entries.map(([name, field], index) =>
    applyFieldMeta({
      id: `field-${index + 1}`,
      name,
      selectorKind: field.selector.kind,
      selectorExpr: field.selector.expr,
      selectorMulti: Boolean(field.selector.multi),
      parserKind: typeof field.parser === "string" ? field.parser : field.parser?.kind ?? "string",
      parserLayout: typeof field.parser === "string" ? "" : field.parser?.layout ?? "",
      required: Boolean(field.required),
      transforms: (field.transforms ?? []).map(transformSpecToForm),
    }),
  );
}

function splitLines(value: string) {
  return value
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseJSON<T>(value: string, label: string): T {
  try {
    return JSON.parse(value) as T;
  } catch {
    throw new Error(`${label} 不是有效的 JSON。`);
  }
}

function stringifyRequestBody(body: RequestBodyDraft | null | undefined) {
  if (!body) {
    return "null";
  }
  if (body.kind === "raw") {
    return body.content ?? "";
  }
  return JSON.stringify(body.values ?? {}, null, 2);
}

function jsonKeyCount(value: string) {
  const trimmed = value.trim();
  if (!trimmed || trimmed === "null") {
    return 0;
  }
  try {
    const parsed = JSON.parse(trimmed) as Record<string, unknown>;
    if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
      return 0;
    }
    return Object.keys(parsed).length;
  } catch {
    return 0;
  }
}

function parseIntegerList(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => Number.parseInt(item, 10))
    .filter((item) => Number.isFinite(item));
}

function parseOptionalInteger(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const next = Number.parseInt(trimmed, 10);
  if (!Number.isFinite(next)) {
    throw new Error("expect_count 必须是整数。");
  }
  return next;
}

function normalizeEditorState(state: EditorState): EditorState {
  const legacyState = state as EditorState & {
    precheckVariablesJSON?: string;
    workflowItemVariablesJSON?: string;
    postAssignJSON?: string;
    postDefaultsJSON?: string;
    postSwitchJSON?: string;
  };
  const legacyDefaults = parseLegacyObject<NonNullable<NonNullable<PluginEditorDraft["postprocess"]>["defaults"]>>(legacyState.postDefaultsJSON);
  const legacySwitchConfig = parseLegacyObject<NonNullable<NonNullable<PluginEditorDraft["postprocess"]>["switch_config"]>>(legacyState.postSwitchJSON);
  return {
    ...state,
    requestBodyKind: state.requestBodyKind || "json",
    multiRequestBodyKind: state.multiRequestBodyKind || "json",
    workflowNextBodyKind: state.workflowNextBodyKind || "json",
    requestQueryJSON: normalizeJSONObjectText(state.requestQueryJSON),
    requestHeadersJSON: normalizeJSONObjectText(state.requestHeadersJSON),
    requestCookiesJSON: normalizeJSONObjectText(state.requestCookiesJSON),
    requestBodyJSON: normalizeRequestBodyText(state.requestBodyJSON, state.requestBodyKind || "json"),
    multiRequestQueryJSON: normalizeJSONObjectText(state.multiRequestQueryJSON),
    multiRequestHeadersJSON: normalizeJSONObjectText(state.multiRequestHeadersJSON),
    multiRequestCookiesJSON: normalizeJSONObjectText(state.multiRequestCookiesJSON),
    multiRequestBodyJSON: normalizeRequestBodyText(state.multiRequestBodyJSON, state.multiRequestBodyKind || "json"),
    workflowNextQueryJSON: normalizeJSONObjectText(state.workflowNextQueryJSON),
    workflowNextHeadersJSON: normalizeJSONObjectText(state.workflowNextHeadersJSON),
    workflowNextCookiesJSON: normalizeJSONObjectText(state.workflowNextCookiesJSON),
    workflowNextBodyJSON: normalizeRequestBodyText(state.workflowNextBodyJSON, state.workflowNextBodyKind || "json"),
    fields: (state.fields ?? []).map((field, index) => {
      const legacy = field as FieldForm & { transformsJSON?: string };
      const transforms =
        field.transforms && Array.isArray(field.transforms)
          ? field.transforms
          : parseLegacyTransforms(legacy.transformsJSON, index);
      return applyFieldMeta({
        ...field,
        transforms,
      });
    }),
    workflowSelectors:
      state.workflowSelectors && Array.isArray(state.workflowSelectors) && state.workflowSelectors.length > 0
        ? state.workflowSelectors
        : defaultState().workflowSelectors,
    precheckVariables: normalizeKVSource(state.precheckVariables, legacyState.precheckVariablesJSON, "precheck-variable"),
    workflowItemVariables: normalizeKVSource(state.workflowItemVariables, legacyState.workflowItemVariablesJSON, "workflow-item"),
    postAssign: normalizeKVSource(state.postAssign, legacyState.postAssignJSON, "post-assign"),
    postTitleLang: state.postTitleLang || legacyDefaults?.title_lang || defaultState().postTitleLang,
    postPlotLang: state.postPlotLang || legacyDefaults?.plot_lang || defaultState().postPlotLang,
    postGenresLang: state.postGenresLang || legacyDefaults?.genres_lang || "",
    postActorsLang: state.postActorsLang || legacyDefaults?.actors_lang || "",
    postDisableReleaseDateCheck: state.postDisableReleaseDateCheck || Boolean(legacySwitchConfig?.disable_release_date_check),
    postDisableNumberReplace: state.postDisableNumberReplace || Boolean(legacySwitchConfig?.disable_number_replace),
  };
}

function normalizeJSONObjectText(value: string | undefined) {
  if (!value || !value.trim()) {
    return "{}";
  }
  return value;
}

function normalizeRequestBodyText(value: string | undefined, kind: string) {
  if (!value || !value.trim()) {
    return kind === "raw" ? "" : "null";
  }
  return value;
}

function normalizeKVSource(items: KVPairForm[] | undefined, raw: string | undefined, seed: string): KVPairForm[] {
  if (Array.isArray(items)) {
    return items;
  }
  if (!raw || !raw.trim()) {
    return [];
  }
  try {
    const parsed = JSON.parse(raw) as Record<string, string>;
    return Object.entries(parsed ?? {}).map(([key, value], index) => ({
      id: `kv-${seed}-${index + 1}`,
      key,
      value: String(value ?? ""),
    }));
  } catch {
    return [];
  }
}

function parseLegacyObject<T>(raw: string | undefined): T | null {
  if (!raw || !raw.trim() || raw.trim() === "null") {
    return null;
  }
  try {
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

function parseLegacyTransforms(raw: string | undefined, fieldIndex: number): TransformForm[] {
  if (!raw || !raw.trim()) {
    return [makeDefaultTransform(`legacy-${fieldIndex}`)];
  }
  try {
    const items = JSON.parse(raw) as PluginEditorTransform[];
    if (!Array.isArray(items) || items.length === 0) {
      return [makeDefaultTransform(`legacy-${fieldIndex}`)];
    }
    return items.map((item, index) => transformSpecToForm(item, index));
  } catch {
    return [makeDefaultTransform(`legacy-${fieldIndex}`)];
  }
}

function needsOldNew(kind: string) {
  return kind === "replace";
}

function needsValue(kind: string) {
  return kind === "trim_prefix" || kind === "trim_suffix" || kind === "regex_extract";
}

function needsSep(kind: string) {
  return kind === "split" || kind === "split_index";
}

function needsCutset(kind: string) {
  return kind === "trim_charset";
}

function needsIndex(kind: string) {
  return kind === "regex_extract" || kind === "split_index";
}

function valueLabelForKind(kind: string) {
  if (kind === "regex_extract") {
    return "Pattern";
  }
  return "Value";
}

function makeDefaultTransform(seed: string): TransformForm {
  return {
    id: `transform-${seed}`,
    kind: "trim",
    old: "",
    newValue: "",
    cutset: "",
    sep: "",
    index: "",
    value: "",
  };
}

function insertTransform(items: TransformForm[], afterTransformID?: string): TransformForm[] {
  const next = {
    id: `transform-${Date.now()}`,
    kind: "trim",
    old: "",
    newValue: "",
    cutset: "",
    sep: "",
    index: "",
    value: "",
  };
  if (!afterTransformID) {
    return [...items, next];
  }
  const index = items.findIndex((item) => item.id === afterTransformID);
  if (index < 0) {
    return [...items, next];
  }
  return [...items.slice(0, index + 1), next, ...items.slice(index + 1)];
}

function getFieldMeta(name: string): FieldMeta {
  return FIELD_META[name] ?? { label: name, fixedParser: "string", fixedMulti: false };
}

function nextUnusedFieldName(fields: FieldForm[]) {
  const used = new Set(fields.map((field) => field.name));
  return FIELD_OPTIONS.find((option) => !used.has(option)) ?? "";
}

function nextUnusedKVFieldName(items: KVPairForm[]) {
  const used = new Set(items.map((item) => item.key).filter(Boolean));
  return FIELD_OPTIONS.find((option) => !used.has(option)) ?? "";
}

function applyFieldMeta(field: FieldForm): FieldForm {
  const meta = getFieldMeta(field.name);
  const next = { ...field };
  if (meta.fixedParser) {
    next.parserKind = meta.fixedParser;
  } else if (meta.defaultParser && !next.parserKind) {
    next.parserKind = meta.defaultParser;
  }
  if (!showParserLayout(next.parserKind)) {
    next.parserLayout = "";
  }
  if (typeof meta.fixedMulti === "boolean") {
    next.selectorMulti = meta.fixedMulti;
  }
  return next;
}

function showParserLayout(parserKind: string) {
  return parserKind === "time_format" || parserKind === "date_layout_soft";
}

function transformFormToSpec(form: TransformForm): PluginEditorTransform {
  return {
    kind: form.kind.trim(),
    old: form.old || undefined,
    new: form.newValue || undefined,
    cutset: form.cutset || undefined,
    sep: form.sep || undefined,
    index: form.index.trim() ? Number.parseInt(form.index.trim(), 10) : undefined,
    value: form.value || undefined,
  };
}

function transformSpecToForm(spec: PluginEditorTransform, index = 0): TransformForm {
  return {
    id: `transform-${index + 1}-${spec.kind}`,
    kind: spec.kind,
    old: spec.old ?? "",
    newValue: spec.new ?? "",
    cutset: spec.cutset ?? "",
    sep: spec.sep ?? "",
    index: typeof spec.index === "number" ? String(spec.index) : "",
    value: spec.value ?? "",
  };
}
