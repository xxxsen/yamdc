import type {
  PluginEditorDraft,
  PluginEditorField,
} from "@/lib/api";

import type {
  EditorState,
  RequestFormState,
} from "./plugin-editor-types";
import {
  normalizeJSONObjectText,
  pairsToRecord,
  parseIntegerList,
  parseJSON,
  parseOptionalInteger,
  splitLines,
  transformFormToSpec,
} from "./plugin-editor-utils";

// ---------------------------------------------------------------------------
// Build request body + string-record helpers
// ---------------------------------------------------------------------------

function jsonRecordValueToString(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
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
  const parsed = parseJSON(value, label);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return { kind: kind || "json", values: {} };
  }
  const values = Object.fromEntries(
    Object.entries(parsed as Record<string, unknown>).map(([key, item]) => [key, jsonRecordValueToString(item)]),
  );
  return {
    kind: kind || "json",
    values,
  };
}

function parseStringRecord(value: string, label: string): Record<string, string> {
  const parsed = parseJSON(normalizeJSONObjectText(value), label);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return {};
  }
  return Object.fromEntries(
    Object.entries(parsed as Record<string, unknown>).map(([key, item]) => [key, jsonRecordValueToString(item)]),
  );
}

// ---------------------------------------------------------------------------
// Build request from RequestFormState
// ---------------------------------------------------------------------------

export function buildRequestFromFormState(req: RequestFormState, label = "request"): NonNullable<PluginEditorDraft["request"]> {
  const hasWaitParams = req.browserWaitSelector.trim() || req.browserWaitTimeout.trim() || req.browserWaitStable.trim();
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
    browser: hasWaitParams
      ? {
          wait_selector: req.browserWaitSelector.trim() || undefined,
          wait_timeout: parseInt(req.browserWaitTimeout) || undefined,
          wait_stable: parseInt(req.browserWaitStable) || undefined,
        }
      : undefined,
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
// Build full draft from EditorState
// ---------------------------------------------------------------------------

export function buildDraft(state: EditorState): PluginEditorDraft {
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
    return {
      ...acc,
      [name]: {
        selector: {
          kind: field.selectorKind.trim(),
          expr: field.selectorExpr.trim(),
          multi: field.selectorMulti,
        },
        transforms: field.transforms.map(transformFormToSpec).filter((item) => item.kind),
        parser,
        required: field.required,
      },
    };
  }, {});
  if (Object.keys(fields).length === 0) {
    throw new Error("至少需要一个 scrape field。");
  }
  const draft: PluginEditorDraft = {
    version: 1,
    name: state.name.trim(),
    type: state.type,
    fetch_type: state.fetchType !== "go-http" ? state.fetchType : undefined,
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
  if (Object.keys(assign).length > 0 || defaults || switchConfig) {
    draft.postprocess = {
      assign,
      defaults,
      switch_config: switchConfig,
    };
  }
  return draft;
}
