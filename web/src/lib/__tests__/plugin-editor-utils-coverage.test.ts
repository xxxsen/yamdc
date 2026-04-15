// @vitest-environment jsdom

import type React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { DEFAULT_FIELD } from "@/components/plugin-editor/plugin-editor-constants";
import {
  applyFieldMeta,
  buildRequestFromFormState,
  defaultState,
  getFieldMeta,
  handleEditorTextareaKeyDown,
  insertTransform,
  makeDefaultTransform,
  normalizeEditorState,
  parseJSON,
  runResponseExpr,
  showParserLayout,
} from "@/components/plugin-editor/plugin-editor-utils";
import type { EditorState, FieldForm, KVPairForm } from "@/components/plugin-editor/plugin-editor-types";

const hasDOMParser = typeof DOMParser !== "undefined";

function baseField(overrides: Partial<FieldForm> = {}): FieldForm {
  return {
    id: "f1",
    name: "title",
    selectorKind: "xpath",
    selectorExpr: "//title",
    selectorMulti: false,
    parserKind: "string",
    parserLayout: "",
    required: false,
    transforms: [],
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// insertTransform (missing branch in main test file)
// ---------------------------------------------------------------------------
describe("insertTransform", () => {
  it("appends when afterTransformID is not found", () => {
    const a = makeDefaultTransform("a");
    const out = insertTransform([a], "no-such-transform");
    expect(out).toHaveLength(2);
    expect(out[0].id).toBe(a.id);
  });
});

// ---------------------------------------------------------------------------
// parseJSON
// ---------------------------------------------------------------------------
describe("parseJSON", () => {
  it("parses valid JSON object", () => {
    expect(parseJSON('{"a":1}', "body")).toEqual({ a: 1 });
  });

  it("throws with label for invalid JSON", () => {
    expect(() => parseJSON("{broken", "myLabel")).toThrow("myLabel 不是有效的 JSON。");
  });

  it("parses null", () => {
    expect(parseJSON("null", "x")).toBeNull();
  });

  it("parses array JSON", () => {
    expect(parseJSON("[]", "x")).toEqual([]);
  });

  it("parses empty string JSON", () => {
    expect(parseJSON('""', "x")).toBe("");
  });
});

// ---------------------------------------------------------------------------
// getFieldMeta
// ---------------------------------------------------------------------------
describe("getFieldMeta", () => {
  it("returns meta with label for known title", () => {
    expect(getFieldMeta("title")).toMatchObject({ label: "title", fixedParser: "string" });
  });

  it("returns string_list meta for actors", () => {
    expect(getFieldMeta("actors")).toMatchObject({ label: "actors", fixedParser: "string_list", fixedMulti: true });
  });

  it("returns fallback meta for unknown custom field", () => {
    expect(getFieldMeta("custom_xyz")).toEqual({
      label: "custom_xyz",
      fixedParser: "string",
      fixedMulti: false,
    });
  });
});

// ---------------------------------------------------------------------------
// showParserLayout
// ---------------------------------------------------------------------------
describe("showParserLayout", () => {
  it("returns true for time_format", () => {
    expect(showParserLayout("time_format")).toBe(true);
  });

  it("returns true for date_layout_soft", () => {
    expect(showParserLayout("date_layout_soft")).toBe(true);
  });

  it("returns false for string", () => {
    expect(showParserLayout("string")).toBe(false);
  });

  it("returns false for empty string", () => {
    expect(showParserLayout("")).toBe(false);
  });

  it("returns false for json", () => {
    expect(showParserLayout("json")).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// applyFieldMeta
// ---------------------------------------------------------------------------
describe("applyFieldMeta", () => {
  it("applies fixedParser for known field title", () => {
    const field = baseField({ name: "title", parserKind: "json" });
    expect(applyFieldMeta(field).parserKind).toBe("string");
  });

  it("overrides selectorMulti when fixedMulti is set", () => {
    const field = baseField({ name: "actors", selectorMulti: false });
    expect(applyFieldMeta(field).selectorMulti).toBe(true);
  });

  it("clears parserLayout when parserKind does not need layout", () => {
    const field = baseField({ name: "title", parserKind: "string", parserLayout: "  something  " });
    expect(applyFieldMeta(field).parserLayout).toBe("");
  });

  it("preserves parserLayout for release_date with time_format", () => {
    const field: FieldForm = {
      ...baseField(),
      name: "release_date",
      parserKind: "time_format",
      parserLayout: "2006-01-02",
    };
    const next = applyFieldMeta(field);
    expect(next.parserKind).toBe("time_format");
    expect(showParserLayout(next.parserKind)).toBe(true);
    expect(next.parserLayout).toBe("2006-01-02");
  });

  it("uses defaultParser when field has no parserKind (release_date)", () => {
    const field: FieldForm = {
      ...baseField(),
      name: "release_date",
      parserKind: "",
      parserLayout: "",
    };
    const next = applyFieldMeta(field);
    expect(next.parserKind).toBe("date_layout_soft");
  });

  it("applies string parser for unknown custom name via fallback meta", () => {
    const field = baseField({ name: "custom_xyz", parserKind: "json" });
    expect(applyFieldMeta(field).parserKind).toBe("string");
  });
});

// ---------------------------------------------------------------------------
// handleEditorTextareaKeyDown
// ---------------------------------------------------------------------------
describe("handleEditorTextareaKeyDown", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("does nothing for non-Tab keys", () => {
    const preventDefault = vi.fn();
    handleEditorTextareaKeyDown({
      key: "Enter",
      preventDefault,
      currentTarget: {} as HTMLTextAreaElement,
    } as unknown as React.KeyboardEvent<HTMLTextAreaElement>);
    expect(preventDefault).not.toHaveBeenCalled();
  });

  it("inserts a tab, dispatches input, and advances caret on Tab", () => {
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      cb(0);
      return 0;
    });

    const onDispatchEvent = vi.fn();

    class FakeTextarea {
      selectionStart = 2;
      selectionEnd = 4;
      private _val = "abcd";
      dispatchEvent = onDispatchEvent;
    }

    vi.stubGlobal("window", {
      HTMLTextAreaElement: FakeTextarea,
    });

    Object.defineProperty(FakeTextarea.prototype, "value", {
      configurable: true,
      enumerable: true,
      get(this: FakeTextarea) {
        return this._val;
      },
      set(this: FakeTextarea, v: string) {
        this._val = v;
      },
    });

    const textarea = new FakeTextarea() as unknown as HTMLTextAreaElement;
    const preventDefault = vi.fn();
    const event = {
      key: "Tab",
      preventDefault,
      currentTarget: textarea,
    } as unknown as React.KeyboardEvent<HTMLTextAreaElement>;

    handleEditorTextareaKeyDown(event);

    expect(preventDefault).toHaveBeenCalled();
    expect(textarea.value).toBe("ab\t");
    expect(onDispatchEvent).toHaveBeenCalled();
    const inputEvent = onDispatchEvent.mock.calls[0][0] as Event;
    expect(inputEvent.type).toBe("input");
    expect(inputEvent.bubbles).toBe(true);
    expect(textarea.selectionStart).toBe(3);
    expect(textarea.selectionEnd).toBe(3);
  });
});

// ---------------------------------------------------------------------------
// normalizeEditorState — legacy migration
// ---------------------------------------------------------------------------
describe("normalizeEditorState legacy migrations", () => {
  it("migrates legacy multiRequest flat fields", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.multiRequest = undefined;
    state.multiRequestMethod = "POST";
    state.multiRequestPath = "/multi";
    state.multiRequestURL = "";
    state.multiRequestQueryJSON = '{"q":"test"}';
    state.multiRequestHeadersJSON = '{"X-Key":"abc"}';
    state.multiRequestCookiesJSON = "{}";
    state.multiRequestBodyKind = "raw";
    state.multiRequestBodyJSON = "hello body";
    state.multiRequestAcceptStatusText = "200,302";
    state.multiRequestNotFoundStatusText = "404,410";
    state.multiRequestDecodeCharset = "shift_jis";
    const result = normalizeEditorState(state);
    expect(result.multiRequest.method).toBe("POST");
    expect(result.multiRequest.path).toBe("/multi");
    expect(JSON.parse(result.multiRequest.queryJSON)).toEqual({ q: "test" });
    expect(JSON.parse(result.multiRequest.headersJSON)).toEqual({ "X-Key": "abc" });
    expect(result.multiRequest.bodyKind).toBe("raw");
    expect(result.multiRequest.bodyJSON).toBe("hello body");
    expect(result.multiRequest.decodeCharset).toBe("shift_jis");
  });

  it("migrates legacy workflowNext flat fields", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.workflowNextRequest = undefined;
    state.workflowNextMethod = "PATCH";
    state.workflowNextPath = "/next";
    state.workflowNextURL = "";
    state.workflowNextQueryJSON = "{}";
    state.workflowNextHeadersJSON = "{}";
    state.workflowNextCookiesJSON = "{}";
    state.workflowNextBodyKind = "json";
    state.workflowNextBodyJSON = "null";
    state.workflowNextAcceptStatusText = "200";
    state.workflowNextNotFoundStatusText = "404";
    state.workflowNextDecodeCharset = "";
    const result = normalizeEditorState(state);
    expect(result.workflowNextRequest.method).toBe("PATCH");
    expect(result.workflowNextRequest.path).toBe("/next");
  });

  it("migrates legacy request flat fields when nested request lacks method key", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.request = {} as EditorState["request"];
    state.requestMethod = "PUT";
    state.requestPath = "/legacy";
    state.requestURL = "";
    state.requestQueryJSON = '{"a":1}';
    state.requestHeadersJSON = "{}";
    state.requestCookiesJSON = "{}";
    state.requestBodyKind = "json";
    state.requestBodyJSON = "";
    state.requestAcceptStatusText = "201";
    state.requestNotFoundStatusText = "410";
    state.requestDecodeCharset = "utf-8";
    const result = normalizeEditorState(state as EditorState);
    expect(result.request.method).toBe("PUT");
    expect(result.request.path).toBe("/legacy");
    expect(result.request.bodyJSON).toBe("null");
    expect(result.request.decodeCharset).toBe("utf-8");
    expect(result.request.browserWaitSelector).toBe("");
    expect(result.request.browserWaitTimeout).toBe("");
  });

  it("normalizes nested request bodyJSON for raw kind when empty", () => {
    const state = defaultState();
    state.request.bodyKind = "raw";
    state.request.bodyJSON = "   ";
    const result = normalizeEditorState(state);
    expect(result.request.bodyJSON).toBe("");
  });

  it("normalizes nested request bodyJSON for json kind when empty", () => {
    const state = defaultState();
    state.request.bodyKind = "json";
    state.request.bodyJSON = "";
    const result = normalizeEditorState(state);
    expect(result.request.bodyJSON).toBe("null");
  });

  it("preserves browserWaitSelector and browserWaitTimeout in nested request", () => {
    const state = defaultState();
    state.request.browserWaitSelector = "#content";
    state.request.browserWaitTimeout = "5000";
    const result = normalizeEditorState(state);
    expect(result.request.browserWaitSelector).toBe("#content");
    expect(result.request.browserWaitTimeout).toBe("5000");
  });

  it("defaults fetchType to go-http when empty", () => {
    const state = defaultState();
    state.fetchType = "";
    const result = normalizeEditorState(state);
    expect(result.fetchType).toBe("go-http");
  });

  it("replaces empty workflowSelectors with defaults", () => {
    const state = defaultState();
    state.workflowSelectors = [];
    const result = normalizeEditorState(state);
    expect(result.workflowSelectors.length).toBeGreaterThan(0);
  });

  it("replaces non-array workflowSelectors with defaults", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.workflowSelectors = null as unknown as EditorState["workflowSelectors"];
    const result = normalizeEditorState(state as EditorState);
    expect(result.workflowSelectors.length).toBeGreaterThan(0);
  });

  it("applies nested request fallbacks for empty strings", () => {
    const state = defaultState();
    state.request.method = "";
    state.request.path = "";
    state.request.rawURL = "";
    state.request.acceptStatusText = "";
    state.request.notFoundStatusText = "";
    state.request.bodyKind = "";
    state.request.queryJSON = "";
    const result = normalizeEditorState(state);
    expect(result.request.method).toBe("GET");
    expect(result.request.queryJSON).toBe("{}");
    expect(result.request.acceptStatusText).toBe("200");
    expect(result.request.notFoundStatusText).toBe("404");
    expect(result.request.bodyKind).toBe("json");
  });

  it("passes through undefined browser wait fields from nested request", () => {
    const state = defaultState();
    Reflect.deleteProperty(state.request, "browserWaitSelector");
    Reflect.deleteProperty(state.request, "browserWaitTimeout");
    const result = normalizeEditorState(state);
    expect(result.request.browserWaitSelector).toBeUndefined();
    expect(result.request.browserWaitTimeout).toBeUndefined();
  });

  it("migrates legacy precheckVariablesJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.precheckVariables = undefined as unknown as KVPairForm[];
    state.precheckVariablesJSON = '{"host":"example.com","lang":"ja"}';
    const result = normalizeEditorState(state as EditorState);
    expect(result.precheckVariables).toHaveLength(2);
    expect(result.precheckVariables[0].key).toBe("host");
    expect(result.precheckVariables[0].value).toBe("example.com");
  });

  it("returns [] for invalid precheckVariablesJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.precheckVariables = undefined as unknown as KVPairForm[];
    state.precheckVariablesJSON = "{broken";
    const result = normalizeEditorState(state as EditorState);
    expect(result.precheckVariables).toEqual([]);
  });

  it("returns [] for null precheckVariablesJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.precheckVariables = undefined as unknown as KVPairForm[];
    state.precheckVariablesJSON = "null";
    const result = normalizeEditorState(state as EditorState);
    expect(result.precheckVariables).toEqual([]);
  });

  it("returns [] for array precheckVariablesJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.precheckVariables = undefined as unknown as KVPairForm[];
    state.precheckVariablesJSON = "[1,2,3]";
    const result = normalizeEditorState(state as EditorState);
    expect(result.precheckVariables).toEqual([]);
  });

  it("returns [] for whitespace-only precheckVariablesJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.precheckVariables = undefined as unknown as KVPairForm[];
    state.precheckVariablesJSON = "   \n\t  ";
    const result = normalizeEditorState(state as EditorState);
    expect(result.precheckVariables).toEqual([]);
  });

  it("keeps precheckVariables array when already present (ignores JSON)", () => {
    const state = defaultState();
    (state as EditorState & { precheckVariablesJSON?: string }).precheckVariablesJSON = '{"ignored":true}';
    state.precheckVariables = [{ id: "k1", key: "keep", value: "v" }];
    const result = normalizeEditorState(state);
    expect(result.precheckVariables).toEqual(state.precheckVariables);
  });

  it("migrates legacy workflowItemVariablesJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    (state as { workflowItemVariables?: KVPairForm[] }).workflowItemVariables = undefined;
    state.workflowItemVariablesJSON = '{"read_link":"${item.href}"}';
    const result = normalizeEditorState(state as EditorState);
    expect(result.workflowItemVariables).toHaveLength(1);
    expect(result.workflowItemVariables[0].key).toBe("read_link");
  });

  it("returns [] for invalid workflowItemVariablesJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    (state as { workflowItemVariables?: KVPairForm[] }).workflowItemVariables = undefined;
    state.workflowItemVariablesJSON = "not-json";
    expect(normalizeEditorState(state as EditorState).workflowItemVariables).toEqual([]);
  });

  it("returns [] for array workflowItemVariablesJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    (state as { workflowItemVariables?: KVPairForm[] }).workflowItemVariables = undefined;
    state.workflowItemVariablesJSON = "[]";
    expect(normalizeEditorState(state as EditorState).workflowItemVariables).toEqual([]);
  });

  it("migrates legacy postAssignJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    (state as { postAssign?: KVPairForm[] }).postAssign = undefined;
    state.postAssignJSON = '{"foo":"bar"}';
    const result = normalizeEditorState(state as EditorState);
    expect(result.postAssign).toHaveLength(1);
    expect(result.postAssign[0].key).toBe("foo");
    expect(result.postAssign[0].value).toBe("bar");
  });

  it("returns [] for invalid postAssignJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    (state as { postAssign?: KVPairForm[] }).postAssign = undefined;
    state.postAssignJSON = "{";
    expect(normalizeEditorState(state as EditorState).postAssign).toEqual([]);
  });

  it("returns [] for null postAssignJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    (state as { postAssign?: KVPairForm[] }).postAssign = undefined;
    state.postAssignJSON = "null";
    expect(normalizeEditorState(state as EditorState).postAssign).toEqual([]);
  });

  it("applies legacy postDefaultsJSON when state langs empty", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postTitleLang = "";
    state.postPlotLang = "";
    state.postGenresLang = "";
    state.postActorsLang = "";
    state.postDefaultsJSON = '{"title_lang":"en","plot_lang":"ja","genres_lang":"g","actors_lang":"a"}';
    const result = normalizeEditorState(state as EditorState);
    expect(result.postTitleLang).toBe("en");
    expect(result.postPlotLang).toBe("ja");
    expect(result.postGenresLang).toBe("g");
    expect(result.postActorsLang).toBe("a");
  });

  it("does not override existing langs with legacy postDefaultsJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postTitleLang = "zh-cn";
    state.postDefaultsJSON = '{"title_lang":"en"}';
    const result = normalizeEditorState(state as EditorState);
    expect(result.postTitleLang).toBe("zh-cn");
  });

  it("ignores invalid legacy postDefaultsJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postDefaultsJSON = "{bad";
    const result = normalizeEditorState(state as EditorState);
    expect(result.postTitleLang).toBe("");
  });

  it("treats null string postDefaultsJSON as empty", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postDefaultsJSON = "null";
    const result = normalizeEditorState(state as EditorState);
    expect(result.postTitleLang).toBe("");
  });

  it("treats empty postDefaultsJSON as empty", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postDefaultsJSON = "";
    const result = normalizeEditorState(state as EditorState);
    expect(result.postPlotLang).toBe("");
  });

  it("applies legacy postSwitchJSON flags", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postDisableReleaseDateCheck = false;
    state.postDisableNumberReplace = false;
    state.postSwitchJSON = '{"disable_release_date_check":true,"disable_number_replace":true}';
    const result = normalizeEditorState(state as EditorState);
    expect(result.postDisableReleaseDateCheck).toBe(true);
    expect(result.postDisableNumberReplace).toBe(true);
  });

  it("leaves flags false for null postSwitchJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postSwitchJSON = "null";
    const result = normalizeEditorState(state as EditorState);
    expect(result.postDisableReleaseDateCheck).toBe(false);
    expect(result.postDisableNumberReplace).toBe(false);
  });

  it("leaves flags false for empty postSwitchJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postSwitchJSON = "";
    const result = normalizeEditorState(state as EditorState);
    expect(result.postDisableReleaseDateCheck).toBe(false);
  });

  it("ignores invalid postSwitchJSON", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.postSwitchJSON = "{";
    const result = normalizeEditorState(state as EditorState);
    expect(result.postDisableReleaseDateCheck).toBe(false);
  });

  it("migrates legacy transformsJSON with valid transforms", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.fields = [
      {
        ...DEFAULT_FIELD,
        id: "f1",
        transforms: undefined as unknown as FieldForm["transforms"],
      },
    ];
    (state.fields[0] as FieldForm & { transformsJSON?: string }).transformsJSON =
      '[{"kind":"trim"},{"kind":"to_upper"}]';
    const result = normalizeEditorState(state as EditorState);
    expect(result.fields[0].transforms).toHaveLength(2);
    expect(result.fields[0].transforms[0].kind).toBe("trim");
    expect(result.fields[0].transforms[1].kind).toBe("to_upper");
  });

  it("handles empty legacy transformsJSON → default transform", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.fields = [
      {
        ...DEFAULT_FIELD,
        id: "f1",
        transforms: undefined as unknown as FieldForm["transforms"],
      },
    ];
    (state.fields[0] as FieldForm & { transformsJSON?: string }).transformsJSON = "";
    const result = normalizeEditorState(state as EditorState);
    expect(result.fields[0].transforms).toHaveLength(1);
    expect(result.fields[0].transforms[0].kind).toBe("trim");
  });

  it("handles invalid JSON in legacy transformsJSON → default transform", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.fields = [
      {
        ...DEFAULT_FIELD,
        id: "f1",
        transforms: undefined as unknown as FieldForm["transforms"],
      },
    ];
    (state.fields[0] as FieldForm & { transformsJSON?: string }).transformsJSON = "{broken}";
    const result = normalizeEditorState(state as EditorState);
    expect(result.fields[0].transforms).toHaveLength(1);
    expect(result.fields[0].transforms[0].kind).toBe("trim");
  });

  it("handles empty array in legacy transformsJSON → default transform", () => {
    const state = defaultState() as EditorState & Record<string, unknown>;
    state.fields = [
      {
        ...DEFAULT_FIELD,
        id: "f1",
        transforms: undefined as unknown as FieldForm["transforms"],
      },
    ];
    (state.fields[0] as FieldForm & { transformsJSON?: string }).transformsJSON = "[]";
    const result = normalizeEditorState(state as EditorState);
    expect(result.fields[0].transforms).toHaveLength(1);
    expect(result.fields[0].transforms[0].kind).toBe("trim");
  });
});

// ---------------------------------------------------------------------------
// runResponseExpr (jsonpath)
// ---------------------------------------------------------------------------
describe("runResponseExpr jsonpath", () => {
  it("returns prompt for empty expression", () => {
    expect(
      runResponseExpr({ body: "{}", expr: "", kind: "jsonpath", contentType: "application/json" }),
    ).toBe("请输入表达式。");
  });

  it("returns prompt for whitespace-only expression", () => {
    expect(
      runResponseExpr({ body: "{}", expr: "   \n\t  ", kind: "jsonpath", contentType: "application/json" }),
    ).toBe("请输入表达式。");
  });

  it("returns JSON parse error message for invalid body", () => {
    const msg = runResponseExpr({ body: "{", expr: "$", kind: "jsonpath", contentType: "application/json" });
    expect(msg).toMatch(/Unexpected|JSON/u);
  });

  it("returns fallback message for non-Error throws", () => {
    const spy = vi.spyOn(JSON, "parse").mockImplementationOnce(() => {
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- runResponseExpr handles non-Error rejects
      throw "boom";
    });
    expect(
      runResponseExpr({ body: "{}", expr: "$", kind: "jsonpath", contentType: "application/json" }),
    ).toBe("表达式执行失败。");
    spy.mockRestore();
  });

  it("extracts $.name", () => {
    expect(
      runResponseExpr({
        body: JSON.stringify({ name: "test" }),
        expr: "$.name",
        kind: "jsonpath",
        contentType: "application/json",
      }),
    ).toBe("test");
  });

  it("returns full JSON for $ alone", () => {
    const body = JSON.stringify({ a: 1 });
    expect(
      runResponseExpr({ body, expr: "$", kind: "jsonpath", contentType: "application/json" }),
    ).toBe(JSON.stringify(JSON.parse(body), null, 2));
  });

  it("returns full JSON when path normalizes to empty ($.)", () => {
    const body = JSON.stringify({ x: 1 });
    expect(
      runResponseExpr({ body, expr: "$.", kind: "jsonpath", contentType: "application/json" }),
    ).toBe(JSON.stringify(JSON.parse(body), null, 2));
  });

  it("collects $.items[*].name", () => {
    const body = JSON.stringify({
      items: [{ name: "a" }, { name: "b" }],
    });
    const out = runResponseExpr({ body, expr: "$.items[*].name", kind: "jsonpath", contentType: "application/json" });
    expect(JSON.parse(out)).toEqual(["a", "b"]);
  });

  it("returns $.items[0]", () => {
    const body = JSON.stringify({ items: [{ id: 1 }, { id: 2 }] });
    const out = runResponseExpr({ body, expr: "$.items[0]", kind: "jsonpath", contentType: "application/json" });
    expect(JSON.parse(out)).toEqual({ id: 1 });
  });

  it("returns deep dotted path $.a.b.c", () => {
    const body = JSON.stringify({ a: { b: { c: "deep" } } });
    expect(
      runResponseExpr({ body, expr: "$.a.b.c", kind: "jsonpath", contentType: "application/json" }),
    ).toBe("deep");
  });

  it("returns 无匹配结果 for unmatched path", () => {
    expect(
      runResponseExpr({
        body: JSON.stringify({ a: 1 }),
        expr: "$.missing",
        kind: "jsonpath",
        contentType: "application/json",
      }),
    ).toBe("无匹配结果");
  });

  it("returns 无匹配结果 when traversing through non-object", () => {
    expect(
      runResponseExpr({
        body: JSON.stringify({ a: 1 }),
        expr: "$.a.b",
        kind: "jsonpath",
        contentType: "application/json",
      }),
    ).toBe("无匹配结果");
  });

  it("JSON-stringifies numeric single result", () => {
    expect(
      runResponseExpr({
        body: JSON.stringify({ n: 42 }),
        expr: "$.n",
        kind: "jsonpath",
        contentType: "application/json",
      }),
    ).toBe(JSON.stringify(42, null, 2));
  });

  it("JSON-stringifies multiple collected values", () => {
    const body = JSON.stringify({ items: [{ v: 1 }, { v: 2 }] });
    const out = runResponseExpr({ body, expr: "$.items[*].v", kind: "jsonpath", contentType: "application/json" });
    expect(JSON.parse(out)).toEqual([1, 2]);
  });

  it("handles wildcard segment when property is not an array", () => {
    const body = JSON.stringify({ items: "not-array" });
    const out = runResponseExpr({ body, expr: "$.items[*].x", kind: "jsonpath", contentType: "application/json" });
    expect(out).toBe("无匹配结果");
  });

  it("handles index segment when property is not an array", () => {
    const body = JSON.stringify({ items: "nope" });
    const out = runResponseExpr({ body, expr: "$.items[0]", kind: "jsonpath", contentType: "application/json" });
    expect(out).toBe("无匹配结果");
  });

  it("collects primitive array via wildcard token", () => {
    const body = JSON.stringify({ items: [1, 2, 3] });
    const out = runResponseExpr({ body, expr: "$.items[*]", kind: "jsonpath", contentType: "application/json" });
    expect(JSON.parse(out)).toEqual([1, 2, 3]);
  });

  it("returns 无匹配结果 for out-of-range array index", () => {
    const body = JSON.stringify({ items: [1] });
    expect(
      runResponseExpr({ body, expr: "$.items[5]", kind: "jsonpath", contentType: "application/json" }),
    ).toBe("无匹配结果");
  });

  it("reads dotted key after wildcard expansion", () => {
    const body = JSON.stringify({ rows: [{ id: 1 }, { id: 2 }] });
    const out = runResponseExpr({ body, expr: "$.rows[*].id", kind: "jsonpath", contentType: "application/json" });
    expect(JSON.parse(out)).toEqual([1, 2]);
  });

  it("filters undefined when drilling into objects after wildcard", () => {
    const body = JSON.stringify({ rows: [{ id: "a" }, {}, { id: "b" }] });
    const out = runResponseExpr({ body, expr: "$.rows[*].id", kind: "jsonpath", contentType: "application/json" });
    expect(JSON.parse(out)).toEqual(["a", "b"]);
  });

  it("skips primitive items during wildcard expansion (arrayMatch non-object branch)", () => {
    const body = JSON.stringify({ items: [{ sub: [1, 2] }, 42, null, "text"] });
    const out = runResponseExpr({ body, expr: "$.items[*].sub[*]", kind: "jsonpath", contentType: "application/json" });
    expect(JSON.parse(out)).toEqual([1, 2]);
  });

  it("skips primitive items during index access (indexMatch non-object branch)", () => {
    const body = JSON.stringify({ items: [{ arr: [10, 20] }, 99, null] });
    const out = runResponseExpr({ body, expr: "$.items[*].arr[0]", kind: "jsonpath", contentType: "application/json" });
    expect(out).toBe("10");
  });

  it("handles wildcard on non-array property (returns no match)", () => {
    const body = JSON.stringify({ data: { items: "string-not-array" } });
    const out = runResponseExpr({ body, expr: "$.data.items[*]", kind: "jsonpath", contentType: "application/json" });
    expect(out).toBe("无匹配结果");
  });

  it("handles index on non-array property (returns no match)", () => {
    const body = JSON.stringify({ data: { items: 42 } });
    const out = runResponseExpr({ body, expr: "$.data.items[0]", kind: "jsonpath", contentType: "application/json" });
    expect(out).toBe("无匹配结果");
  });
});

// ---------------------------------------------------------------------------
// buildRequestFromFormState (covers buildRequestBody + parseStringRecord)
// ---------------------------------------------------------------------------
describe("buildRequestFromFormState edge cases", () => {
  it("builds raw body preserving untrimmed content", () => {
    const req = defaultState().request;
    req.bodyKind = "raw";
    req.bodyJSON = "  raw\r\n  ";
    expect(buildRequestFromFormState(req).body).toEqual({ kind: "raw", content: "  raw\r\n  " });
  });

  it("returns null body for whitespace-only json body", () => {
    const req = defaultState().request;
    req.bodyKind = "json";
    req.bodyJSON = "   ";
    expect(buildRequestFromFormState(req).body).toBeNull();
  });

  it("returns empty values object when json body parses to array", () => {
    const req = defaultState().request;
    req.bodyKind = "json";
    req.bodyJSON = "[]";
    expect(buildRequestFromFormState(req).body).toEqual({ kind: "json", values: {} });
  });

  it("uses json kind fallback when bodyKind is empty for array json body", () => {
    const req = defaultState().request;
    req.bodyKind = "";
    req.bodyJSON = "[]";
    expect(buildRequestFromFormState(req).body).toEqual({ kind: "json", values: {} });
  });

  it("maps mixed json record values for request body", () => {
    const req = defaultState().request;
    req.bodyKind = "json";
    req.bodyJSON = JSON.stringify({
      s: "x",
      n: 3,
      b: false,
      z: null,
      o: { nested: 1 },
    });
    const body = buildRequestFromFormState(req).body as { kind: string; values: Record<string, string> };
    expect(body.kind).toBe("json");
    expect(body.values).toMatchObject({
      s: "x",
      n: "3",
      b: "false",
      z: "",
      o: JSON.stringify({ nested: 1 }),
    });
  });

  it("returns empty query when queryJSON parses to array", () => {
    const req = defaultState().request;
    req.queryJSON = "[]";
    expect(buildRequestFromFormState(req).query).toEqual({});
  });

  it("returns empty headers when headersJSON parses to null", () => {
    const req = defaultState().request;
    req.headersJSON = "null";
    expect(buildRequestFromFormState(req).headers).toEqual({});
  });

  it("includes browser wait with selector only", () => {
    const req = defaultState().request;
    req.browserWaitSelector = " #x ";
    req.browserWaitTimeout = "";
    expect(buildRequestFromFormState(req).browser).toEqual({ wait_selector: "#x", wait_timeout: undefined });
  });

  it("includes browser wait with timeout only", () => {
    const req = defaultState().request;
    req.browserWaitSelector = "";
    req.browserWaitTimeout = "5000";
    expect(buildRequestFromFormState(req).browser).toEqual({ wait_selector: undefined, wait_timeout: 5000 });
  });

  it("omits wait_timeout when timeout is not numeric", () => {
    const req = defaultState().request;
    req.browserWaitSelector = "s";
    req.browserWaitTimeout = "not-a-number";
    expect(buildRequestFromFormState(req).browser?.wait_timeout).toBeUndefined();
  });

  it("throws with custom label for invalid query JSON", () => {
    const req = defaultState().request;
    req.queryJSON = "{";
    expect(() => buildRequestFromFormState(req, "outer")).toThrow("outer query");
  });
});

// ---------------------------------------------------------------------------
// runResponseExpr (xpath) — DOM only
// ---------------------------------------------------------------------------
(hasDOMParser ? describe : describe.skip)("runResponseExpr xpath", () => {
  it("returns empty string for string() on missing nodes", () => {
    const result = runResponseExpr({
      body: "<html><body></body></html>",
      expr: "string(//missing)",
      kind: "xpath",
      contentType: "text/html",
    });
    expect(result).toBe("");
  });

  it("extracts text from HTML", () => {
    const html = "<html><head><title>Hello</title></head></html>";
    const result = runResponseExpr({ body: html, expr: "//title/text()", kind: "xpath", contentType: "text/html" });
    expect(result).toContain("Hello");
  });

  it("returns no-match for unmatched xpath", () => {
    const result = runResponseExpr({
      body: "<html></html>",
      expr: "//nonexistent",
      kind: "xpath",
      contentType: "text/html",
    });
    expect(result).toBe("无匹配结果");
  });

  it("handles string() xpath function", () => {
    const html = "<html><body><p>text</p></body></html>";
    const result = runResponseExpr({ body: html, expr: "string(//p)", kind: "xpath", contentType: "text/html" });
    expect(result).toBe("text");
  });

  it("handles count() xpath returning number", () => {
    const html = "<html><body><p>1</p><p>2</p></body></html>";
    const result = runResponseExpr({ body: html, expr: "count(//p)", kind: "xpath", contentType: "text/html" });
    expect(result).toBe("2");
  });

  it("handles boolean xpath", () => {
    const html = "<html><body><p>1</p></body></html>";
    const result = runResponseExpr({ body: html, expr: "boolean(//p)", kind: "xpath", contentType: "text/html" });
    expect(result).toBe("true");
  });

  it("returns JSON array for multiple node iterator results", () => {
    const html = "<html><body><p>a</p><p>b</p></body></html>";
    const result = runResponseExpr({ body: html, expr: "//p", kind: "xpath", contentType: "text/html" });
    expect(JSON.parse(result)).toEqual(["a", "b"]);
  });

});
