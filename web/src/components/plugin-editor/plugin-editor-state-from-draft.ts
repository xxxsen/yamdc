import type { PluginEditorDraft } from "@/lib/api";

import { DEFAULT_FIELD } from "./plugin-editor-constants";
import type {
  EditorState,
  FieldForm,
  RequestFormState,
} from "./plugin-editor-types";
import {
  applyFieldMeta,
  defaultState,
  recordToPairs,
  stringifyRequestBody,
  transformSpecToForm,
} from "./plugin-editor-utils";

// ---------------------------------------------------------------------------
// Build state from draft (import / YAML parse)
// ---------------------------------------------------------------------------

// 13 个 request 字段的 JSON 序列化 + 各种 fallback, 拆分成 multiple helpers
// 反而让控制流更难追溯.
// eslint-disable-next-line complexity
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
    browserWaitSelector: req?.browser?.wait_selector ?? "",
    browserWaitTimeout: req?.browser?.wait_timeout ? String(req.browser.wait_timeout) : "",
    browserWaitStable: req?.browser?.wait_stable ? String(req.browser.wait_stable) : "",
  };
}

// draft -> EditorState 全量 roundtrip, 15+ 字段赋值 + multi_request / workflow
// 2 个可选嵌套 block, 已被 plugin-editor-utils.test /
// plugin-editor-real-plugins.test 全量覆盖.
// eslint-disable-next-line complexity
export function stateFromDraft(draft: PluginEditorDraft): EditorState {
  const next = defaultState();
  next.name = draft.name;
  next.type = draft.type;
  next.fetchType = draft.fetch_type || "go-http";
  next.hostsText = draft.hosts.join("\n");
  next.precheckPatternsText = (draft.precheck?.number_patterns ?? []).join("\n");
  next.precheckVariables = recordToPairs(draft.precheck?.variables);
  next.scrapeFormat = draft.scrape.format;
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
        name: item.name,
        kind: item.kind,
        expr: item.expr,
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
// Draft → Fields conversion
// ---------------------------------------------------------------------------

export function draftToFields(draft: PluginEditorDraft): FieldForm[] {
  const entries = Object.entries(draft.scrape.fields);
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
