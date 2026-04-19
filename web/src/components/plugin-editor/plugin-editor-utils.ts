import type React from "react";
import type {
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
  TransformForm,
} from "./plugin-editor-types";

// ---------------------------------------------------------------------------
// Default state
// ---------------------------------------------------------------------------

export function defaultState(): EditorState {
  return {
    name: "fixture",
    type: "one-step",
    fetchType: "go-http",
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
    const parsed: unknown = JSON.parse(trimmed);
    if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
      return 0;
    }
    return Object.keys(parsed as Record<string, unknown>).length;
  } catch {
    return 0;
  }
}

export function parseJSON(value: string, label: string): unknown {
  try {
    return JSON.parse(value);
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

export function normalizeRequestBodyText(value: string | undefined, kind: string): string {
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
  const start = textarea.selectionStart;
  const end = textarea.selectionEnd;
  const nextValue = `${textarea.value.slice(0, start)}\t${textarea.value.slice(end)}`;
  Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, "value")?.set?.call(textarea, nextValue);
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
    if (!key) {
      return acc;
    }
    return { ...acc, [key]: item.value.trim() };
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

// ---------------------------------------------------------------------------
// Re-exports: functions that were split out to sibling files for max-lines.
// Keep the "plugin-editor-utils" barrel stable so existing consumers/tests
// don't have to update every import path.
// ---------------------------------------------------------------------------

export {
  buildDefaults,
  buildDraft,
  buildRequestFromFormState,
  buildSwitchConfig,
} from "./plugin-editor-build-draft";

export {
  draftToFields,
  stateFromDraft,
} from "./plugin-editor-state-from-draft";

export { normalizeEditorState } from "./plugin-editor-legacy";

export { runResponseExpr } from "./plugin-editor-response-expr";
