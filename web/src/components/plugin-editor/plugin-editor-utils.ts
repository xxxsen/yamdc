import type React from "react";
import type {
  PluginEditorDraft,
  PluginEditorTransform,
} from "@/lib/api";

import {
  DEFAULT_FIELD,
  DEFAULT_REQUEST_FORM_STATE,
  FIELD_META,
  FIELD_OPTIONS,
} from "./plugin-editor-constants";
import type {
  EditorState,
  FieldForm,
  FieldMeta,
  KVPairForm,
  RequestBodyDraft,
  RequestFormState,
  TransformForm,
} from "./plugin-editor-types";

// ---------------------------------------------------------------------------
// Default state
// ---------------------------------------------------------------------------

export function defaultState(): EditorState {
  return {
    name: "fixture",
    type: "one-step",
    hostsText: "https://example.com",
    number: "ABC-123",
    precheckPatternsText: "",
    precheckVariables: [],
    request: {
      ...DEFAULT_REQUEST_FORM_STATE,
      path: "/search/${number}",
      acceptStatusText: "200",
      notFoundStatusText: "404",
    },
    scrapeFormat: "html",
    fields: [DEFAULT_FIELD],
    multiRequestEnabled: false,
    multiCandidatesText: "",
    multiUnique: true,
    multiRequest: { ...DEFAULT_REQUEST_FORM_STATE },
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
    workflowNextRequest: { ...DEFAULT_REQUEST_FORM_STATE },
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

// ---------------------------------------------------------------------------
// Primitive helpers
// ---------------------------------------------------------------------------

export function splitLines(value: string): string[] {
  return value
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean);
}

export function parseIntegerList(value: string): number[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => Number.parseInt(item, 10))
    .filter((item) => Number.isFinite(item));
}

export function parseOptionalInteger(value: string): number | undefined {
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

export function jsonKeyCount(value: string): number {
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

export function parseJSON<T>(value: string, label: string): T {
  try {
    return JSON.parse(value) as T;
  } catch {
    throw new Error(`${label} 不是有效的 JSON。`);
  }
}

export function normalizeJSONObjectText(value: string | undefined): string {
  if (!value || !value.trim()) {
    return "{}";
  }
  return value;
}

function normalizeRequestBodyText(value: string | undefined, kind: string): string {
  if (!value || !value.trim()) {
    return kind === "raw" ? "" : "null";
  }
  return value;
}

// ---------------------------------------------------------------------------
// Textarea Tab key handler
// ---------------------------------------------------------------------------

export function handleEditorTextareaKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
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

// ---------------------------------------------------------------------------
// KV pair helpers
// ---------------------------------------------------------------------------

export function pairsToRecord(items: KVPairForm[]): Record<string, string> {
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

export function recordToPairs(record?: Record<string, string> | null): KVPairForm[] {
  return Object.entries(record ?? {}).map(([key, value], index) => ({
    id: `kv-${index + 1}-${key}`,
    key,
    value,
  }));
}

// ---------------------------------------------------------------------------
// Transform helpers
// ---------------------------------------------------------------------------

export function needsOldNew(kind: string): boolean {
  return kind === "replace";
}

export function needsValue(kind: string): boolean {
  return kind === "trim_prefix" || kind === "trim_suffix" || kind === "regex_extract";
}

export function needsSep(kind: string): boolean {
  return kind === "split" || kind === "split_index";
}

export function needsCutset(kind: string): boolean {
  return kind === "trim_charset";
}

export function needsIndex(kind: string): boolean {
  return kind === "regex_extract" || kind === "split_index";
}

export function valueLabelForKind(kind: string): string {
  if (kind === "regex_extract") {
    return "Pattern";
  }
  return "Value";
}

export function makeDefaultTransform(seed: string): TransformForm {
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

export function insertTransform(items: TransformForm[], afterTransformID?: string): TransformForm[] {
  const next: TransformForm = {
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

export function transformFormToSpec(form: TransformForm): PluginEditorTransform {
  const kind = form.kind.trim();
  return {
    kind,
    old: needsOldNew(kind) ? form.old : undefined,
    new: needsOldNew(kind) ? form.newValue : undefined,
    cutset: needsCutset(kind) ? form.cutset : undefined,
    sep: needsSep(kind) ? form.sep : undefined,
    index: needsIndex(kind) && form.index.trim() ? Number.parseInt(form.index.trim(), 10) : undefined,
    value: needsValue(kind) ? form.value : undefined,
  };
}

export function transformSpecToForm(spec: PluginEditorTransform, index = 0): TransformForm {
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

// ---------------------------------------------------------------------------
// Field meta helpers
// ---------------------------------------------------------------------------

export function getFieldMeta(name: string): FieldMeta {
  return FIELD_META[name] ?? { label: name, fixedParser: "string", fixedMulti: false };
}

export function showParserLayout(parserKind: string): boolean {
  return parserKind === "time_format" || parserKind === "date_layout_soft";
}

export function applyFieldMeta(field: FieldForm): FieldForm {
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

export function nextUnusedFieldName(fields: FieldForm[]): string {
  const used = new Set(fields.map((field) => field.name));
  return FIELD_OPTIONS.find((option) => !used.has(option)) ?? "";
}

export function nextUnusedKVFieldName(items: KVPairForm[]): string {
  const used = new Set(items.map((item) => item.key).filter(Boolean));
  return FIELD_OPTIONS.find((option) => !used.has(option)) ?? "";
}

// ---------------------------------------------------------------------------
// Request body helpers
// ---------------------------------------------------------------------------

export function stringifyRequestBody(body: RequestBodyDraft | null | undefined): string {
  if (!body) {
    return "null";
  }
  if (body.kind === "raw") {
    return body.content ?? "";
  }
  return JSON.stringify(body.values ?? {}, null, 2);
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

function parseStringRecord(value: string, label: string): Record<string, string> {
  const parsed = parseJSON<Record<string, unknown>>(normalizeJSONObjectText(value), label);
  return Object.entries(parsed ?? {}).reduce<Record<string, string>>((acc, [key, item]) => {
    acc[key] = item == null ? "" : String(item);
    return acc;
  }, {});
}

// ---------------------------------------------------------------------------
// Build request from RequestFormState
// ---------------------------------------------------------------------------

export function buildRequestFromFormState(req: RequestFormState, label = "request"): NonNullable<PluginEditorDraft["request"]> {
  return {
    method: req.method.trim() || "GET",
    path: req.path.trim() || undefined,
    url: req.rawURL.trim() || undefined,
    query: parseStringRecord(req.queryJSON, `${label} query`),
    headers: parseStringRecord(req.headersJSON, `${label} headers`),
    cookies: parseStringRecord(req.cookiesJSON, `${label} cookies`),
    body: buildRequestBody(req.bodyKind, req.bodyJSON, `${label} body`),
    accept_status_codes: parseIntegerList(req.acceptStatusText),
    not_found_status_codes: parseIntegerList(req.notFoundStatusText),
    response: req.decodeCharset.trim() ? { decode_charset: req.decodeCharset.trim() } : undefined,
    browser: req.browserEnable
      ? {
          enable: true,
          wait_selector: req.browserWaitSelector.trim() || undefined,
          wait_timeout: parseInt(req.browserWaitTimeout) || undefined,
        }
      : undefined,
  };
}

// ---------------------------------------------------------------------------
// Build full draft from EditorState
// ---------------------------------------------------------------------------

export function buildDraft(state: EditorState): PluginEditorDraft {
  const hosts = splitLines(state.hostsText);
  if (hosts.length === 0) {
    throw new Error("至少需要一个 host。");
  }
  const fields = state.fields.reduce<Record<string, import("@/lib/api").PluginEditorField>>((acc, field) => {
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
    request: buildRequestFromFormState(state.request, "request"),
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
    const baseRequest = buildRequestFromFormState(state.request, "request");
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
        next_request: buildRequestFromFormState(state.workflowNextRequest, "workflow next"),
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

// ---------------------------------------------------------------------------
// Build state from draft (import / YAML parse)
// ---------------------------------------------------------------------------

function requestFormStateFromDraft(req: PluginEditorDraft["request"]): RequestFormState {
  return {
    method: req?.method ?? "GET",
    path: req?.path ?? "",
    rawURL: req?.url ?? "",
    queryJSON: JSON.stringify(req?.query ?? {}, null, 2),
    headersJSON: JSON.stringify(req?.headers ?? {}, null, 2),
    cookiesJSON: JSON.stringify(req?.cookies ?? {}, null, 2),
    bodyKind: req?.body?.kind ?? "json",
    bodyJSON: stringifyRequestBody(req?.body ?? null),
    acceptStatusText: (req?.accept_status_codes ?? []).join(","),
    notFoundStatusText: (req?.not_found_status_codes ?? []).join(","),
    decodeCharset: req?.response?.decode_charset ?? "",
    browserEnable: req?.browser?.enable ?? false,
    browserWaitSelector: req?.browser?.wait_selector ?? "",
    browserWaitTimeout: req?.browser?.wait_timeout ? String(req.browser.wait_timeout) : "",
  };
}

export function stateFromDraft(draft: PluginEditorDraft): EditorState {
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
    next.request = requestFormStateFromDraft(draft.multi_request.request);
    next.multiCandidatesText = (draft.multi_request.candidates ?? []).join("\n");
    next.multiUnique = true;
    next.multiSuccessMode = draft.multi_request.success_when?.mode ?? "and";
    next.multiSuccessConditionsText = (draft.multi_request.success_when?.conditions ?? []).join("\n");
  } else if (draft.request) {
    next.request = requestFormStateFromDraft(draft.request);
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
    next.workflowNextRequest = requestFormStateFromDraft(searchSelect.next_request);
  }
  return next;
}

// ---------------------------------------------------------------------------
// Normalize legacy editor state from localStorage
// ---------------------------------------------------------------------------

type LegacyEditorState = EditorState & {
  // Legacy flat fields from v1 format
  requestMethod?: string;
  requestPath?: string;
  requestURL?: string;
  requestQueryJSON?: string;
  requestHeadersJSON?: string;
  requestCookiesJSON?: string;
  requestBodyKind?: string;
  requestBodyJSON?: string;
  requestAcceptStatusText?: string;
  requestNotFoundStatusText?: string;
  requestDecodeCharset?: string;
  multiRequestMethod?: string;
  multiRequestPath?: string;
  multiRequestURL?: string;
  multiRequestQueryJSON?: string;
  multiRequestHeadersJSON?: string;
  multiRequestCookiesJSON?: string;
  multiRequestBodyKind?: string;
  multiRequestBodyJSON?: string;
  multiRequestAcceptStatusText?: string;
  multiRequestNotFoundStatusText?: string;
  multiRequestDecodeCharset?: string;
  workflowNextMethod?: string;
  workflowNextPath?: string;
  workflowNextURL?: string;
  workflowNextQueryJSON?: string;
  workflowNextHeadersJSON?: string;
  workflowNextCookiesJSON?: string;
  workflowNextBodyKind?: string;
  workflowNextBodyJSON?: string;
  workflowNextAcceptStatusText?: string;
  workflowNextNotFoundStatusText?: string;
  workflowNextDecodeCharset?: string;
  precheckVariablesJSON?: string;
  workflowItemVariablesJSON?: string;
  postAssignJSON?: string;
  postDefaultsJSON?: string;
  postSwitchJSON?: string;
};

function normalizeLegacyRequestForm(
  state: LegacyEditorState,
  prefix: "request" | "multiRequest" | "workflowNext",
): RequestFormState {
  // Check if new nested format already exists
  const nested = state[prefix === "request" ? "request" : prefix === "multiRequest" ? "multiRequest" : "workflowNextRequest"] as RequestFormState | undefined;
  if (nested && typeof nested === "object" && "method" in nested) {
    return {
      method: nested.method || "GET",
      path: nested.path || "",
      rawURL: nested.rawURL || "",
      queryJSON: normalizeJSONObjectText(nested.queryJSON),
      headersJSON: normalizeJSONObjectText(nested.headersJSON),
      cookiesJSON: normalizeJSONObjectText(nested.cookiesJSON),
      bodyKind: nested.bodyKind || "json",
      bodyJSON: normalizeRequestBodyText(nested.bodyJSON, nested.bodyKind || "json"),
      acceptStatusText: nested.acceptStatusText || "200",
      notFoundStatusText: nested.notFoundStatusText || "404",
      decodeCharset: nested.decodeCharset || "",
      browserEnable: nested.browserEnable ?? false,
      browserWaitSelector: nested.browserWaitSelector ?? "",
      browserWaitTimeout: nested.browserWaitTimeout ?? "",
    };
  }
  // Fall back to legacy flat fields
  const m = prefix === "request" ? "request" : prefix === "multiRequest" ? "multiRequest" : "workflowNext";
  const method = (state as Record<string, unknown>)[`${m}Method`] as string | undefined;
  const path = (state as Record<string, unknown>)[`${m}Path`] as string | undefined;
  const rawURL = (state as Record<string, unknown>)[`${m}URL`] as string | undefined;
  const queryJSON = (state as Record<string, unknown>)[`${m}QueryJSON`] as string | undefined;
  const headersJSON = (state as Record<string, unknown>)[`${m}HeadersJSON`] as string | undefined;
  const cookiesJSON = (state as Record<string, unknown>)[`${m}CookiesJSON`] as string | undefined;
  const bodyKind = (state as Record<string, unknown>)[`${m}BodyKind`] as string | undefined;
  const bodyJSON = (state as Record<string, unknown>)[`${m}BodyJSON`] as string | undefined;
  const acceptStatusText = (state as Record<string, unknown>)[`${m}AcceptStatusText`] as string | undefined;
  const notFoundStatusText = (state as Record<string, unknown>)[`${m}NotFoundStatusText`] as string | undefined;
  const decodeCharset = (state as Record<string, unknown>)[`${m}DecodeCharset`] as string | undefined;
  return {
    method: method || "GET",
    path: path || "",
    rawURL: rawURL || "",
    queryJSON: normalizeJSONObjectText(queryJSON),
    headersJSON: normalizeJSONObjectText(headersJSON),
    cookiesJSON: normalizeJSONObjectText(cookiesJSON),
    bodyKind: bodyKind || "json",
    bodyJSON: normalizeRequestBodyText(bodyJSON, bodyKind || "json"),
    acceptStatusText: acceptStatusText || "200",
    notFoundStatusText: notFoundStatusText || "404",
    decodeCharset: decodeCharset || "",
    browserEnable: false,
    browserWaitSelector: "",
    browserWaitTimeout: "",
  };
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

export function normalizeEditorState(state: EditorState): EditorState {
  const legacyState = state as LegacyEditorState;
  const legacyDefaults = parseLegacyObject<NonNullable<NonNullable<PluginEditorDraft["postprocess"]>["defaults"]>>(legacyState.postDefaultsJSON);
  const legacySwitchConfig = parseLegacyObject<NonNullable<NonNullable<PluginEditorDraft["postprocess"]>["switch_config"]>>(legacyState.postSwitchJSON);

  return {
    ...state,
    request: normalizeLegacyRequestForm(legacyState, "request"),
    multiRequest: normalizeLegacyRequestForm(legacyState, "multiRequest"),
    workflowNextRequest: normalizeLegacyRequestForm(legacyState, "workflowNext"),
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

// ---------------------------------------------------------------------------
// Postprocess helpers
// ---------------------------------------------------------------------------

export function buildDefaults(state: EditorState): NonNullable<NonNullable<PluginEditorDraft["postprocess"]>["defaults"]> | null {
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

export function buildSwitchConfig(state: EditorState): NonNullable<NonNullable<PluginEditorDraft["postprocess"]>["switch_config"]> | null {
  if (!state.postDisableReleaseDateCheck && !state.postDisableNumberReplace) {
    return null;
  }
  return {
    disable_release_date_check: state.postDisableReleaseDateCheck,
    disable_number_replace: state.postDisableNumberReplace,
  };
}

// ---------------------------------------------------------------------------
// Draft → Fields conversion
// ---------------------------------------------------------------------------

export function draftToFields(draft: PluginEditorDraft): FieldForm[] {
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

// ---------------------------------------------------------------------------
// Expression runners (client-side xpath / jsonpath)
// ---------------------------------------------------------------------------

export function runResponseExpr(input: { body: string; expr: string; kind: "xpath" | "jsonpath"; contentType: string }): string {
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

function runXPathExpr(body: string, expr: string): string {
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

function runJSONExpr(body: string, expr: string): string {
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
