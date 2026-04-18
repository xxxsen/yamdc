"use client";

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

import { DEFAULT_DRAFT_STORAGE_KEY, FIELD_OPTIONS } from "./plugin-editor-constants";
import * as ops from "./plugin-editor-state-ops";
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
  buildDraft,
  defaultState,
  normalizeEditorState,
  stateFromDraft,
} from "./plugin-editor-utils";

// usePluginEditorState: plugin-editor 主 shell 的 **全部** 本地状态 + effect
// + updater + action. 拆出来是把 1095 行的巨型组件压到 ≤ 400 行的前提 —
// 组件那一侧只负责把 hook 返回值分发给子组件. 纯更新逻辑被进一步拆到
// plugin-editor-state-ops, 这里的 updater 只是 setState + op 的薄壳.
//
// 对应 td/022 §2.1 A-2.

const DEFAULT_NUMBER_STORAGE_KEY = "yamdc.debug.plugin-editor.number";

type KVPairKey = ops.KVPairKey;

export function usePluginEditorState() {
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
    if (typeof window === "undefined") return;
    const timer = window.setTimeout(() => {
      window.localStorage.setItem(DEFAULT_DRAFT_STORAGE_KEY, JSON.stringify(state));
      window.localStorage.setItem(DEFAULT_NUMBER_STORAGE_KEY, state.number);
    }, 160);
    return () => window.clearTimeout(timer);
  }, [state]);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), toast.tone === "warning" ? 5000 : 2200);
    return () => window.clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    if (!compileMenuOpen) return;
    function handlePointerDown(event: PointerEvent) {
      if (!compileMenuRef.current?.contains(event.target as Node)) {
        setCompileMenuOpen(false);
      }
    }
    window.addEventListener("pointerdown", handlePointerDown);
    return () => window.removeEventListener("pointerdown", handlePointerDown);
  }, [compileMenuOpen]);

  useEffect(() => {
    if (typeof window === "undefined") return;
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
      if (!dragState || !page) return;
      const pageRect = page.getBoundingClientRect();
      const maxX = Math.max(12, pageRect.width - dragState.width - 12);
      const maxY = Math.max(12, pageRect.height - dragState.height - 12);
      setFloatingMenuPos({
        x: Math.min(Math.max(event.clientX - pageRect.left - dragState.offsetX, 12), maxX),
        y: Math.min(Math.max(event.clientY - pageRect.top - dragState.offsetY, 12), maxY),
      });
    }
    function handlePointerUp() { dragStateRef.current = null; }
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
    };
  }, []);

  const previewDraft = useMemo(() => {
    if (tab !== "draft") return null;
    try { return buildDraft(deferredState); } catch { return null; }
  }, [deferredState, tab]);
  const draftPreview = useMemo(() => (previewDraft ? JSON.stringify(previewDraft, null, 2) : ""), [previewDraft]);
  const canAddField = state.fields.length < FIELD_OPTIONS.length;

  // ---------- updater 薄壳 (pure ops 在 plugin-editor-state-ops.ts) ----------
  const patch = <K extends keyof EditorState>(key: K, value: EditorState[K]) => setState(ops.patch(key, value));
  const patchRequest = (key: "request" | "multiRequest" | "workflowNextRequest", updater: (prev: RequestFormState) => RequestFormState) => setState(ops.patchRequest(key, updater));
  const patchField = (id: string, updater: (f: FieldForm) => FieldForm) => setState(ops.patchField(id, updater));
  const updateFieldName = (id: string, nextName: string) => setState(ops.updateFieldName(id, nextName));
  const patchWorkflowSelector = (id: string, updater: (s: WorkflowSelectorForm) => WorkflowSelectorForm) => setState(ops.patchWorkflowSelector(id, updater));
  const patchKVPair = (key: KVPairKey, id: string, updater: (item: KVPairForm) => KVPairForm) => setState(ops.patchKVPair(key, id, updater));
  const addKVPair = (key: KVPairKey) => setState(ops.addKVPair(key));
  const removeKVPair = (key: KVPairKey, id: string) => setState(ops.removeKVPair(key, id));
  const patchTransform = (fieldID: string, transformID: string, updater: (t: TransformForm) => TransformForm) => setState(ops.patchTransform(fieldID, transformID, updater));
  const addField = () => setState(ops.addField());
  const removeField = (id: string) => setState(ops.removeField(id));
  const addTransform = (fieldID: string, afterTransformID?: string) => setState(ops.addTransform(fieldID, afterTransformID));
  const removeTransform = (fieldID: string, transformID: string) => setState(ops.removeTransform(fieldID, transformID));
  const addWorkflowSelector = () => setState(ops.addWorkflowSelector());
  const removeWorkflowSelector = (id: string) => setState(ops.removeWorkflowSelector(id));

  // ---------- actions ----------
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
      if (state.workflowEnabled) {
        const workflowDebug = await debugPluginDraftWorkflow(draft, state.number.trim());
        setWorkflowResult(workflowDebug.data);
        if (workflowDebug.data.error) {
          setTab("workflow");
          return;
        }
      }
      const scrapeDebug = await debugPluginDraftScrape(draft, state.number.trim());
      // multi_request 走独立的 request debug endpoint 以拿 attempts,
      // scrape endpoint 不带这个字段.
      if (state.multiRequestEnabled) {
        const reqDebug = await debugPluginDraftRequest(draft, state.number.trim());
        setRequestResult(reqDebug.data);
      } else {
        setRequestResult({ request: scrapeDebug.data.request, response: scrapeDebug.data.response });
      }
      if (!state.workflowEnabled) setWorkflowResult(null);
      setScrapeResult(scrapeDebug.data);
      setTab(state.workflowEnabled ? "scrape" : "basic");
      if (state.fetchType === "browser" && !state.request.browserWaitSelector.trim()) {
        setToast({ message: "提示：browser 模式下建议配置 Wait XPath，否则可能因为页面未完全加载导致抓取失败。", tone: "warning" });
      }
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "插件调试失败");
    } finally {
      setBusyAction("");
    }
  }

  async function handleCopyYAML() {
    const yaml = compileResult?.yaml;
    if (!yaml) return;
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
      setState((prev) => ({ ...stateFromDraft(result.data.draft), importYAML: prev.importYAML }));
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
    if (!menu || !page) return;
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
    if (typeof window !== "undefined") {
      window.localStorage.removeItem(DEFAULT_DRAFT_STORAGE_KEY);
      window.localStorage.removeItem(DEFAULT_NUMBER_STORAGE_KEY);
    }
    setState(defaultState());
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

  return {
    state, tab, activeSection,
    compileResult, requestResult, workflowResult, scrapeResult,
    error, busyAction, toast,
    importOpen, exampleOpen, compileMenuOpen, floatingMenuPos,
    pageRef, compileMenuRef,
    draftPreview, canAddField,
    setTab, setActiveSection, setImportOpen, setExampleOpen, setCompileMenuOpen,
    patch, patchRequest, patchField, updateFieldName, patchWorkflowSelector,
    patchKVPair, addKVPair, removeKVPair,
    patchTransform, addField, removeField,
    addTransform, removeTransform,
    addWorkflowSelector, removeWorkflowSelector,
    run, handleCopyYAML, handleImportYAML, handleClearDraft, handleFloatingMenuPointerDown,
  };
}
