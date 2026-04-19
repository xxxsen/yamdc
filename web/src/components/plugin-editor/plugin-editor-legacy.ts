import type {
  PluginEditorDraft,
  PluginEditorTransform,
} from "@/lib/api";

import type {
  EditorState,
  FieldForm,
  KVPairForm,
  RequestFormState,
  TransformForm,
} from "./plugin-editor-types";
import {
  applyFieldMeta,
  defaultState,
  makeDefaultTransform,
  normalizeJSONObjectText,
  normalizeRequestBodyText,
  transformSpecToForm,
} from "./plugin-editor-utils";

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

// legacy v1 -> v2 迁移: 13 个字段 × 有/无嵌套双分支, 复杂度无法降低;
// 已被 plugin-editor-utils.test / plugin-editor-utils-coverage.test 覆盖.
// eslint-disable-next-line complexity
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
      browserWaitSelector: nested.browserWaitSelector,
      browserWaitTimeout: nested.browserWaitTimeout,
      browserWaitStable: nested.browserWaitStable,
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
    browserWaitSelector: "",
    browserWaitTimeout: "",
    browserWaitStable: "",
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
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return [];
    }
    return Object.entries(parsed as Record<string, string>).map(([key, value], index) => ({
      id: `kv-${seed}-${index + 1}`,
      key,
      value,
    }));
  } catch {
    return [];
  }
}

function parseLegacyObject(raw: string | undefined): unknown {
  if (!raw || !raw.trim() || raw.trim() === "null") {
    return null;
  }
  try {
    return JSON.parse(raw);
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

// 13+ 个字段的 legacy 迁移合并, 每个字段都是独立 fallback 链
// (state || legacy || default), 拆分反而破坏内聚性.
// eslint-disable-next-line complexity
export function normalizeEditorState(state: EditorState): EditorState {
  const legacyState = state as LegacyEditorState;
  const legacyDefaults = parseLegacyObject(legacyState.postDefaultsJSON) as NonNullable<
    NonNullable<PluginEditorDraft["postprocess"]>["defaults"]
  > | null;
  const legacySwitchConfig = parseLegacyObject(legacyState.postSwitchJSON) as NonNullable<
    NonNullable<PluginEditorDraft["postprocess"]>["switch_config"]
  > | null;

  return {
    ...state,
    fetchType: state.fetchType || "go-http",
    request: normalizeLegacyRequestForm(legacyState, "request"),
    multiRequest: normalizeLegacyRequestForm(legacyState, "multiRequest"),
    workflowNextRequest: normalizeLegacyRequestForm(legacyState, "workflowNext"),
    fields: state.fields.map((field, index) => {
      const legacy = field as FieldForm & { transformsJSON?: string };
      const transforms = Array.isArray(field.transforms)
        ? field.transforms
        : parseLegacyTransforms(legacy.transformsJSON, index);
      return applyFieldMeta({
        ...field,
        transforms,
      });
    }),
    workflowSelectors:
      Array.isArray(state.workflowSelectors) && state.workflowSelectors.length > 0
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
