// debug.ts: /api/debug/* 下所有调试接口 (movieid-cleaner / searcher /
// handler / plugin-editor) 的类型与函数。
// 这些接口共享 SearcherDebugMovieMeta / MovieIDCleanerResult 等类型, 放在
// 一个模块里避免跨 debug 子模块的循环引用和重复 re-export。
// 如果某一类未来继续膨胀 (比如 plugin-editor 单独超过 400 行), 再评估
// 拆成 debug/plugin-editor.ts 子模块。

import { apiRequest, type MediaFileRef, type PluginEditorEnvelope } from "./core";

// ---------------------------------------------------------------------------
// MovieIDCleaner
// ---------------------------------------------------------------------------

export interface MovieIDCleanerCandidate {
  number_id: string;
  score: number;
  rule_hits: string[];
  matcher: string;
  start: number;
  end: number;
  category: string;
  category_matched: boolean;
  uncensor: boolean;
  uncensor_matched: boolean;
}

export interface MovieIDCleanerExplainStep {
  stage: string;
  rule: string;
  input: string;
  output: string;
  matched: boolean;
  selected: boolean;
  summary: string;
  values: string[];
  candidate?: MovieIDCleanerCandidate | null;
}

export interface MovieIDCleanerResult {
  raw_input: string;
  input_no_ext: string;
  normalized: string;
  number_id: string;
  suffixes: string[];
  category: string;
  uncensor: boolean;
  category_matched: boolean;
  uncensor_matched: boolean;
  confidence: string;
  status: string;
  rule_hits: string[];
  warnings: string[];
  candidates: MovieIDCleanerCandidate[];
}

export interface MovieIDCleanerExplainResult {
  input: string;
  input_no_ext: string;
  steps: MovieIDCleanerExplainStep[];
  final: MovieIDCleanerResult;
}

export async function explainMovieIDCleaner(input: string, signal?: AbortSignal) {
  const data = await apiRequest<MovieIDCleanerExplainResult>("/api/debug/movieid-cleaner/explain", {
    method: "POST",
    body: { input },
    signal,
  });
  return data.data;
}

// ---------------------------------------------------------------------------
// Searcher
// ---------------------------------------------------------------------------

export interface SearcherDebugPluginCollection {
  available: string[];
  default: string[];
  category: Record<string, string[]>;
}

export interface SearcherDebugStep {
  stage: string;
  ok: boolean;
  message: string;
  url?: string;
  status_code?: number;
  duration_ms?: number;
}

export interface SearcherDebugMovieMeta {
  number?: string;
  title?: string;
  title_lang?: string;
  title_translated?: string;
  release_date?: number;
  duration?: number;
  studio?: string;
  label?: string;
  series?: string;
  director?: string;
  actors?: string[];
  actors_lang?: string;
  genres?: string[];
  genres_lang?: string;
  plot?: string;
  plot_lang?: string;
  plot_translated?: string;
  cover?: MediaFileRef | null;
  poster?: MediaFileRef | null;
  sample_images?: MediaFileRef[];
  ext_info?: {
    scrape_info: {
      source: string;
      date_ts: number;
    };
  };
}

export interface SearcherDebugPluginResult {
  plugin: string;
  found: boolean;
  error?: string;
  meta?: SearcherDebugMovieMeta | null;
  steps: SearcherDebugStep[];
}

export interface SearcherDebugResult {
  input: string;
  number_id: string;
  requested_input: string;
  used_plugins: string[];
  matched_plugin: string;
  found: boolean;
  category: string;
  uncensor: boolean;
  cleaner_result?: MovieIDCleanerResult | null;
  meta?: SearcherDebugMovieMeta | null;
  plugin_results: SearcherDebugPluginResult[];
  available_tools: SearcherDebugPluginCollection;
}

export async function getSearcherDebugPlugins(signal?: AbortSignal) {
  const data = await apiRequest<SearcherDebugPluginCollection>("/api/debug/searcher/plugins", {
    cache: "no-store",
    signal,
  });
  return data.data;
}

export async function debugSearcher(input: string, plugin: string, useCleaner: boolean, signal?: AbortSignal) {
  const plugins = plugin ? [plugin] : [];
  const data = await apiRequest<SearcherDebugResult>("/api/debug/searcher/search", {
    method: "POST",
    body: { input, plugins, use_cleaner: useCleaner },
    signal,
  });
  return data.data;
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

export interface HandlerDebugRequest {
  mode?: "single" | "chain";
  handler_id: string;
  handler_ids?: string[];
  meta: SearcherDebugMovieMeta;
}

export interface HandlerDebugInstance {
  id: string;
  name: string;
}

export interface HandlerDebugResult {
  mode: "single" | "chain";
  handler_id: string;
  handler_name: string;
  number_id: string;
  category: string;
  uncensor: boolean;
  before_meta: SearcherDebugMovieMeta;
  after_meta: SearcherDebugMovieMeta;
  error: string;
  steps: Array<{
    handler_id: string;
    handler_name: string;
    before_meta: SearcherDebugMovieMeta;
    after_meta: SearcherDebugMovieMeta;
    error: string;
  }>;
}

export async function getHandlerDebugHandlers(signal?: AbortSignal) {
  const data = await apiRequest<HandlerDebugInstance[]>("/api/debug/handlers", {
    cache: "no-store",
    signal,
  });
  return data.data;
}

export async function debugHandler(payload: HandlerDebugRequest, signal?: AbortSignal) {
  const data = await apiRequest<HandlerDebugResult>("/api/debug/handler/run", {
    method: "POST",
    body: payload,
    signal,
  });
  return data.data;
}

// ---------------------------------------------------------------------------
// Plugin Editor
// ---------------------------------------------------------------------------

export interface PluginEditorParser {
  kind?: string;
  layout?: string;
}

export interface PluginEditorBrowserSpec {
  wait_selector?: string;
  wait_timeout?: number;
  wait_stable?: number;
}

export interface PluginEditorSelector {
  kind: string;
  expr: string;
  multi?: boolean;
}

export interface PluginEditorTransform {
  kind: string;
  old?: string;
  new?: string;
  cutset?: string;
  sep?: string;
  index?: number;
  value?: string;
}

export interface PluginEditorField {
  selector: PluginEditorSelector;
  transforms?: PluginEditorTransform[];
  parser?: string | PluginEditorParser;
  required?: boolean;
}

export interface PluginEditorDraft {
  version: number;
  name: string;
  type: string;
  fetch_type?: string;
  hosts: string[];
  precheck?: {
    number_patterns?: string[];
    variables?: Record<string, string>;
  } | null;
  request?: {
    method?: string;
    path?: string;
    url?: string;
    query?: Record<string, string>;
    headers?: Record<string, string>;
    cookies?: Record<string, string>;
    body?: {
      kind?: string;
      values?: Record<string, string>;
      content?: string;
    } | null;
    accept_status_codes?: number[];
    not_found_status_codes?: number[];
    response?: {
      decode_charset?: string;
    } | null;
    browser?: PluginEditorBrowserSpec | null;
  } | null;
  multi_request?: {
    candidates?: string[];
    unique?: boolean;
    request?: PluginEditorDraft["request"];
    success_when?: {
      mode?: string;
      conditions?: string[];
      expect_count?: number;
    } | null;
  } | null;
  workflow?: {
    search_select?: {
      selectors?: Array<{ name: string; kind: string; expr: string }>;
      item_variables?: Record<string, string>;
      match?: {
        mode?: string;
        conditions?: string[];
        expect_count?: number;
      } | null;
      return?: string;
      next_request?: PluginEditorDraft["request"];
    } | null;
  } | null;
  scrape: {
    format: string;
    fields: Record<string, PluginEditorField>;
  };
  postprocess?: {
    assign?: Record<string, string>;
    defaults?: {
      title_lang?: string;
      plot_lang?: string;
      genres_lang?: string;
      actors_lang?: string;
    } | null;
    switch_config?: {
      disable_release_date_check?: boolean;
      disable_number_replace?: boolean;
    } | null;
  } | null;
}

export interface PluginEditorCompileSummary {
  has_request: boolean;
  has_multi_request: boolean;
  has_workflow: boolean;
  scrape_format: string;
  field_count: number;
}

export interface PluginEditorCompileResult {
  yaml: string;
  summary: PluginEditorCompileSummary;
}

export interface PluginEditorHTTPRequestDebug {
  method: string;
  url: string;
  headers: Record<string, string>;
  body: string;
}

export interface PluginEditorHTTPResponseDebug {
  status_code: number;
  headers: Record<string, string[]>;
  body: string;
  body_preview: string;
}

export interface PluginEditorRequestDebugAttempt {
  candidate?: string;
  request: PluginEditorHTTPRequestDebug;
  response?: PluginEditorHTTPResponseDebug | null;
  matched: boolean;
  error?: string;
}

export interface PluginEditorRequestDebugResult {
  candidate?: string;
  request: PluginEditorHTTPRequestDebug;
  response?: PluginEditorHTTPResponseDebug | null;
  attempts?: PluginEditorRequestDebugAttempt[];
}

export interface PluginEditorTransformStep {
  kind: string;
  input: unknown;
  output: unknown;
}

export interface PluginEditorFieldDebugResult {
  selector_values: string[];
  transform_steps: PluginEditorTransformStep[];
  parser_result?: unknown;
  required: boolean;
  matched: boolean;
}

export interface PluginEditorScrapeDebugResult {
  request: PluginEditorHTTPRequestDebug;
  response?: PluginEditorHTTPResponseDebug | null;
  fields: Record<string, PluginEditorFieldDebugResult>;
  meta?: SearcherDebugMovieMeta | null;
  error?: string;
}

export interface PluginEditorWorkflowMatchDetail {
  condition: string;
  pass: boolean;
}

export interface PluginEditorWorkflowSelectorItem {
  index: number;
  item: Record<string, string>;
  item_variables?: Record<string, string>;
  matched: boolean;
  match_details?: PluginEditorWorkflowMatchDetail[];
}

export interface PluginEditorWorkflowStep {
  stage: string;
  summary: string;
  candidate?: string;
  request?: PluginEditorHTTPRequestDebug | null;
  response?: PluginEditorHTTPResponseDebug | null;
  selectors?: Record<string, string[]>;
  items?: PluginEditorWorkflowSelectorItem[];
  selected_value?: string;
}

export interface PluginEditorWorkflowDebugResult {
  steps: PluginEditorWorkflowStep[];
  error?: string;
}

export interface PluginEditorCaseSpec {
  name: string;
  input: string;
  output: {
    title?: string;
    tag_set?: string[];
    actor_set?: string[];
    status?: string;
  };
}

export interface PluginEditorCaseDebugResult {
  pass: boolean;
  errmsg: string;
  meta?: SearcherDebugMovieMeta | null;
}

export interface PluginEditorImportResult {
  draft: PluginEditorDraft;
}

export async function compilePluginDraft(draft: PluginEditorDraft, signal?: AbortSignal) {
  const data = await apiRequest<PluginEditorEnvelope<PluginEditorCompileResult>>("/api/debug/plugin-editor/compile", {
    method: "POST",
    body: { draft },
    signal,
  });
  return data.data;
}

export async function importPluginDraftYAML(yaml: string, signal?: AbortSignal) {
  const data = await apiRequest<PluginEditorEnvelope<PluginEditorImportResult>>("/api/debug/plugin-editor/import", {
    method: "POST",
    body: { yaml },
    signal,
  });
  return data.data;
}

export async function debugPluginDraftRequest(draft: PluginEditorDraft, number: string, signal?: AbortSignal) {
  const data = await apiRequest<PluginEditorEnvelope<PluginEditorRequestDebugResult>>("/api/debug/plugin-editor/request", {
    method: "POST",
    body: { draft, number },
    signal,
  });
  return data.data;
}

export async function debugPluginDraftWorkflow(draft: PluginEditorDraft, number: string, signal?: AbortSignal) {
  const data = await apiRequest<PluginEditorEnvelope<PluginEditorWorkflowDebugResult>>("/api/debug/plugin-editor/workflow", {
    method: "POST",
    body: { draft, number },
    signal,
  });
  return data.data;
}

export async function debugPluginDraftScrape(draft: PluginEditorDraft, number: string, signal?: AbortSignal) {
  const data = await apiRequest<PluginEditorEnvelope<PluginEditorScrapeDebugResult>>("/api/debug/plugin-editor/scrape", {
    method: "POST",
    body: { draft, number },
    signal,
  });
  return data.data;
}

export async function debugPluginDraftCase(draft: PluginEditorDraft, caseSpec: PluginEditorCaseSpec, signal?: AbortSignal) {
  const data = await apiRequest<PluginEditorEnvelope<{ result: PluginEditorCaseDebugResult }>>("/api/debug/plugin-editor/case", {
    method: "POST",
    body: { draft, case: caseSpec },
    signal,
  });
  return data.data;
}
