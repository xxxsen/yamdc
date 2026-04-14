import { describe, expect, it } from "vitest";

import {
  buildDraft,
  buildDefaults,
  buildRequestFromFormState,
  buildSwitchConfig,
  defaultState,
  draftToFields,
  insertTransform,
  jsonKeyCount,
  makeDefaultTransform,
  needsCutset,
  needsIndex,
  needsOldNew,
  needsSep,
  needsValue,
  nextUnusedFieldName,
  nextUnusedKVFieldName,
  normalizeEditorState,
  normalizeJSONObjectText,
  pairsToRecord,
  parseIntegerList,
  parseOptionalInteger,
  recordToPairs,
  splitLines,
  stateFromDraft,
  stringifyRequestBody,
  transformFormToSpec,
  transformSpecToForm,
  valueLabelForKind,
} from "@/components/plugin-editor/plugin-editor-utils";
import { DEFAULT_FIELD } from "@/components/plugin-editor/plugin-editor-constants";
import type { PluginEditorDraft } from "@/lib/api";
import type { EditorState, TransformForm } from "@/components/plugin-editor/plugin-editor-types";

// ---------------------------------------------------------------------------
// splitLines
// ---------------------------------------------------------------------------
describe("splitLines", () => {
  it("returns empty array for empty string", () => {
    expect(splitLines("")).toEqual([]);
  });

  it("splits by newlines and trims", () => {
    expect(splitLines("  a \n  b  \nc")).toEqual(["a", "b", "c"]);
  });

  it("filters empty lines", () => {
    expect(splitLines("a\n\n\nb")).toEqual(["a", "b"]);
  });

  it("handles single line", () => {
    expect(splitLines("hello")).toEqual(["hello"]);
  });
});

// ---------------------------------------------------------------------------
// parseIntegerList
// ---------------------------------------------------------------------------
describe("parseIntegerList", () => {
  it("parses comma-separated integers", () => {
    expect(parseIntegerList("200,302,404")).toEqual([200, 302, 404]);
  });

  it("returns empty for empty string", () => {
    expect(parseIntegerList("")).toEqual([]);
  });

  it("skips non-numeric values", () => {
    expect(parseIntegerList("200,abc,404")).toEqual([200, 404]);
  });

  it("trims whitespace", () => {
    expect(parseIntegerList(" 200 , 302 ")).toEqual([200, 302]);
  });
});

// ---------------------------------------------------------------------------
// parseOptionalInteger
// ---------------------------------------------------------------------------
describe("parseOptionalInteger", () => {
  it("returns undefined for empty string", () => {
    expect(parseOptionalInteger("")).toBeUndefined();
  });

  it("parses valid integer", () => {
    expect(parseOptionalInteger("42")).toBe(42);
  });

  it("throws for non-numeric string", () => {
    expect(() => parseOptionalInteger("abc")).toThrow("expect_count 必须是整数。");
  });
});

// ---------------------------------------------------------------------------
// jsonKeyCount
// ---------------------------------------------------------------------------
describe("jsonKeyCount", () => {
  it("counts keys in valid JSON object", () => {
    expect(jsonKeyCount('{"a":1,"b":2}')).toBe(2);
  });

  it("returns 0 for null", () => {
    expect(jsonKeyCount("null")).toBe(0);
  });

  it("returns 0 for empty string", () => {
    expect(jsonKeyCount("")).toBe(0);
  });

  it("returns 0 for array", () => {
    expect(jsonKeyCount("[1,2]")).toBe(0);
  });

  it("returns 0 for invalid JSON", () => {
    expect(jsonKeyCount("{broken")).toBe(0);
  });

  it("returns 0 for empty object", () => {
    expect(jsonKeyCount("{}")).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// normalizeJSONObjectText
// ---------------------------------------------------------------------------
describe("normalizeJSONObjectText", () => {
  it("returns {} for empty", () => {
    expect(normalizeJSONObjectText("")).toBe("{}");
    expect(normalizeJSONObjectText(undefined)).toBe("{}");
    expect(normalizeJSONObjectText("   ")).toBe("{}");
  });

  it("passes through non-empty", () => {
    expect(normalizeJSONObjectText('{"a":1}')).toBe('{"a":1}');
  });
});

// ---------------------------------------------------------------------------
// pairsToRecord / recordToPairs
// ---------------------------------------------------------------------------
describe("pairsToRecord", () => {
  it("converts KV pairs to record", () => {
    expect(
      pairsToRecord([
        { id: "1", key: "a", value: "1" },
        { id: "2", key: "b", value: "2" },
      ]),
    ).toEqual({ a: "1", b: "2" });
  });

  it("skips empty keys", () => {
    expect(
      pairsToRecord([
        { id: "1", key: "", value: "ignored" },
        { id: "2", key: " ", value: "also ignored" },
        { id: "3", key: "kept", value: "yes" },
      ]),
    ).toEqual({ kept: "yes" });
  });

  it("trims keys and values", () => {
    expect(
      pairsToRecord([{ id: "1", key: " key ", value: " val " }]),
    ).toEqual({ key: "val" });
  });
});

describe("recordToPairs", () => {
  it("converts record to KV pairs", () => {
    const result = recordToPairs({ a: "1", b: "2" });
    expect(result).toHaveLength(2);
    expect(result[0].key).toBe("a");
    expect(result[0].value).toBe("1");
    expect(result[1].key).toBe("b");
    expect(result[1].value).toBe("2");
  });

  it("returns empty array for null/undefined", () => {
    expect(recordToPairs(null)).toEqual([]);
    expect(recordToPairs(undefined)).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// Transform helpers
// ---------------------------------------------------------------------------
describe("transform kind checks", () => {
  it("needsOldNew", () => {
    expect(needsOldNew("replace")).toBe(true);
    expect(needsOldNew("trim")).toBe(false);
  });

  it("needsValue", () => {
    expect(needsValue("trim_prefix")).toBe(true);
    expect(needsValue("trim_suffix")).toBe(true);
    expect(needsValue("regex_extract")).toBe(true);
    expect(needsValue("trim")).toBe(false);
  });

  it("needsSep", () => {
    expect(needsSep("split")).toBe(true);
    expect(needsSep("split_index")).toBe(true);
    expect(needsSep("trim")).toBe(false);
  });

  it("needsCutset", () => {
    expect(needsCutset("trim_charset")).toBe(true);
    expect(needsCutset("trim")).toBe(false);
  });

  it("needsIndex", () => {
    expect(needsIndex("regex_extract")).toBe(true);
    expect(needsIndex("split_index")).toBe(true);
    expect(needsIndex("trim")).toBe(false);
  });

  it("valueLabelForKind", () => {
    expect(valueLabelForKind("regex_extract")).toBe("Pattern");
    expect(valueLabelForKind("replace")).toBe("Value");
  });
});

// ---------------------------------------------------------------------------
// transformFormToSpec / transformSpecToForm roundtrip
// ---------------------------------------------------------------------------
describe("transform form/spec conversion", () => {
  it("roundtrips a replace transform", () => {
    const form: TransformForm = {
      id: "t1",
      kind: "replace",
      old: "foo",
      newValue: "bar",
      cutset: "",
      sep: "",
      index: "",
      value: "",
    };
    const spec = transformFormToSpec(form);
    expect(spec.kind).toBe("replace");
    expect(spec.old).toBe("foo");
    expect(spec.new).toBe("bar");
    expect(spec.cutset).toBeUndefined();

    const back = transformSpecToForm(spec, 0);
    expect(back.kind).toBe("replace");
    expect(back.old).toBe("foo");
    expect(back.newValue).toBe("bar");
  });

  it("handles split_index with index", () => {
    const form: TransformForm = {
      id: "t2",
      kind: "split_index",
      old: "",
      newValue: "",
      cutset: "",
      sep: ",",
      index: "2",
      value: "",
    };
    const spec = transformFormToSpec(form);
    expect(spec.sep).toBe(",");
    expect(spec.index).toBe(2);

    const back = transformSpecToForm(spec, 0);
    expect(back.sep).toBe(",");
    expect(back.index).toBe("2");
  });

  it("handles trim (no params)", () => {
    const form: TransformForm = makeDefaultTransform("test");
    const spec = transformFormToSpec(form);
    expect(spec.kind).toBe("trim");
    expect(spec.old).toBeUndefined();
    expect(spec.new).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// insertTransform
// ---------------------------------------------------------------------------
describe("insertTransform", () => {
  it("appends when no afterID", () => {
    const items = [makeDefaultTransform("a")];
    const result = insertTransform(items);
    expect(result).toHaveLength(2);
    expect(result[0].id).toBe("transform-a");
  });

  it("inserts after target", () => {
    const a = makeDefaultTransform("a");
    const b = makeDefaultTransform("b");
    const result = insertTransform([a, b], a.id);
    expect(result).toHaveLength(3);
    expect(result[0].id).toBe("transform-a");
    expect(result[2].id).toBe("transform-b");
  });
});

// ---------------------------------------------------------------------------
// nextUnusedFieldName / nextUnusedKVFieldName
// ---------------------------------------------------------------------------
describe("nextUnusedFieldName", () => {
  it("returns first available field", () => {
    const name = nextUnusedFieldName([{ ...DEFAULT_FIELD, name: "number" }]);
    expect(name).toBe("title");
  });

  it("returns empty when all used", () => {
    // We have 14 field options — create a field for each
    const fields = [
      "number", "title", "plot", "actors", "release_date", "duration",
      "studio", "label", "series", "director", "genres", "cover", "poster", "sample_images",
    ].map((name) => ({ ...DEFAULT_FIELD, name }));
    expect(nextUnusedFieldName(fields)).toBe("");
  });
});

describe("nextUnusedKVFieldName", () => {
  it("skips used keys", () => {
    const items = [{ id: "1", key: "number", value: "" }];
    expect(nextUnusedKVFieldName(items)).toBe("title");
  });
});

// ---------------------------------------------------------------------------
// stringifyRequestBody
// ---------------------------------------------------------------------------
describe("stringifyRequestBody", () => {
  it("returns null for null body", () => {
    expect(stringifyRequestBody(null)).toBe("null");
  });

  it("returns content for raw body", () => {
    expect(stringifyRequestBody({ kind: "raw", content: "hello" })).toBe("hello");
  });

  it("returns JSON for json body", () => {
    expect(stringifyRequestBody({ kind: "json", values: { a: "1" } })).toBe(
      JSON.stringify({ a: "1" }, null, 2),
    );
  });
});

// ---------------------------------------------------------------------------
// buildRequestFromFormState
// ---------------------------------------------------------------------------
describe("buildRequestFromFormState", () => {
  it("builds a valid request", () => {
    const req = buildRequestFromFormState({
      method: "POST",
      path: "/api/test",
      rawURL: "",
      queryJSON: '{"q":"hello"}',
      headersJSON: '{"X-Token":"abc"}',
      cookiesJSON: "{}",
      bodyKind: "json",
      bodyJSON: '{"key":"val"}',
      acceptStatusText: "200,302",
      notFoundStatusText: "404",
      decodeCharset: "",
      browserEnable: false,
      browserWaitSelector: "",
      browserWaitTimeout: "",
    });
    expect(req.method).toBe("POST");
    expect(req.path).toBe("/api/test");
    expect(req.url).toBeUndefined();
    expect(req.query).toEqual({ q: "hello" });
    expect(req.headers).toEqual({ "X-Token": "abc" });
    expect(req.accept_status_codes).toEqual([200, 302]);
    expect(req.body).toEqual({ kind: "json", values: { key: "val" } });
  });

  it("uses url when path is empty", () => {
    const req = buildRequestFromFormState({
      method: "GET",
      path: "",
      rawURL: "https://example.com/${number}",
      queryJSON: "{}",
      headersJSON: "{}",
      cookiesJSON: "{}",
      bodyKind: "json",
      bodyJSON: "null",
      acceptStatusText: "200",
      notFoundStatusText: "404",
      decodeCharset: "euc-jp",
      browserEnable: false,
      browserWaitSelector: "",
      browserWaitTimeout: "",
    });
    expect(req.path).toBeUndefined();
    expect(req.url).toBe("https://example.com/${number}");
    expect(req.response).toEqual({ decode_charset: "euc-jp" });
  });
});

// ---------------------------------------------------------------------------
// buildDraft
// ---------------------------------------------------------------------------
describe("buildDraft", () => {
  it("builds a minimal draft from default state", () => {
    const state = defaultState();
    const draft = buildDraft(state);
    expect(draft.version).toBe(1);
    expect(draft.name).toBe("fixture");
    expect(draft.type).toBe("one-step");
    expect(draft.hosts).toEqual(["https://example.com"]);
    expect(draft.scrape.format).toBe("html");
    expect(draft.scrape.fields).toHaveProperty("title");
    expect(draft.request?.method).toBe("GET");
    expect(draft.request?.path).toBe("/search/${number}");
    expect(draft.multi_request).toBeUndefined();
    expect(draft.workflow).toBeUndefined();
    expect(draft.postprocess).toBeUndefined();
    expect(draft.precheck).toBeUndefined();
  });

  it("throws when hosts empty", () => {
    const state = defaultState();
    state.hostsText = "";
    expect(() => buildDraft(state)).toThrow("至少需要一个 host。");
  });

  it("throws when no fields", () => {
    const state = defaultState();
    state.fields = [{ ...DEFAULT_FIELD, name: "" }];
    expect(() => buildDraft(state)).toThrow("至少需要一个 scrape field。");
  });

  it("includes multi_request when enabled", () => {
    const state = defaultState();
    state.multiRequestEnabled = true;
    state.multiCandidatesText = "${number}\n${to_upper(${number})}";
    const draft = buildDraft(state);
    expect(draft.request).toBeNull();
    expect(draft.multi_request).toBeDefined();
    expect(draft.multi_request?.candidates).toEqual(["${number}", "${to_upper(${number})}"]);
    expect(draft.multi_request?.request?.method).toBe("GET");
  });

  it("includes workflow when enabled", () => {
    const state = defaultState();
    state.workflowEnabled = true;
    const draft = buildDraft(state);
    expect(draft.workflow).toBeDefined();
    expect(draft.workflow?.search_select?.selectors).toHaveLength(1);
    expect(draft.workflow?.search_select?.next_request?.method).toBe("GET");
  });

  it("includes postprocess when assign exists", () => {
    const state = defaultState();
    state.postAssign = [{ id: "1", key: "title", value: "${meta.title} fixed" }];
    const draft = buildDraft(state);
    expect(draft.postprocess?.assign).toEqual({ title: "${meta.title} fixed" });
  });

  it("includes postprocess defaults", () => {
    const state = defaultState();
    state.postTitleLang = "ja";
    const draft = buildDraft(state);
    expect(draft.postprocess?.defaults?.title_lang).toBe("ja");
  });

  it("includes postprocess switch_config", () => {
    const state = defaultState();
    state.postDisableReleaseDateCheck = true;
    const draft = buildDraft(state);
    expect(draft.postprocess?.switch_config?.disable_release_date_check).toBe(true);
  });

  it("includes precheck when patterns defined", () => {
    const state = defaultState();
    state.precheckPatternsText = "^ABC-\\d+$";
    const draft = buildDraft(state);
    expect(draft.precheck?.number_patterns).toEqual(["^ABC-\\d+$"]);
  });
});

// ---------------------------------------------------------------------------
// stateFromDraft
// ---------------------------------------------------------------------------
describe("stateFromDraft", () => {
  it("parses a minimal draft", () => {
    const draft: PluginEditorDraft = {
      version: 1,
      name: "test-plugin",
      type: "one-step",
      hosts: ["https://example.com"],
      request: {
        method: "GET",
        path: "/search/${number}",
        headers: { "User-Agent": "test" },
      },
      scrape: {
        format: "html",
        fields: {
          title: {
            selector: { kind: "xpath", expr: "//title/text()" },
            parser: "string",
            required: true,
          },
        },
      },
    };
    const state = stateFromDraft(draft);
    expect(state.name).toBe("test-plugin");
    expect(state.type).toBe("one-step");
    expect(state.hostsText).toBe("https://example.com");
    expect(state.request.method).toBe("GET");
    expect(state.request.path).toBe("/search/${number}");
    expect(JSON.parse(state.request.headersJSON)).toEqual({ "User-Agent": "test" });
    expect(state.fields).toHaveLength(1);
    expect(state.fields[0].name).toBe("title");
  });

  it("parses multi_request", () => {
    const draft: PluginEditorDraft = {
      version: 1,
      name: "multi",
      type: "one-step",
      hosts: ["https://example.com"],
      multi_request: {
        candidates: ["${number}", "${to_upper(${number})}"],
        request: { method: "POST", path: "/api" },
        success_when: { mode: "or", conditions: ["contains(\"${body}\", \"ok\")"] },
      },
      scrape: {
        format: "html",
        fields: {
          title: {
            selector: { kind: "xpath", expr: "//title/text()" },
            parser: "string",
          },
        },
      },
    };
    const state = stateFromDraft(draft);
    expect(state.multiRequestEnabled).toBe(true);
    expect(state.request.method).toBe("POST");
    expect(state.request.path).toBe("/api");
    expect(state.multiCandidatesText).toBe("${number}\n${to_upper(${number})}");
    expect(state.multiSuccessMode).toBe("or");
  });

  it("parses workflow", () => {
    const draft: PluginEditorDraft = {
      version: 1,
      name: "two-step",
      type: "two-step",
      hosts: ["https://example.com"],
      request: { method: "GET", path: "/search/${number}" },
      workflow: {
        search_select: {
          selectors: [{ name: "link", kind: "xpath", expr: "//a/@href" }],
          match: { mode: "and", conditions: ["eq(\"${item.link}\", \"yes\")"], expect_count: 1 },
          return: "${item.link}",
          next_request: { method: "GET", url: "${item.link}" },
        },
      },
      scrape: {
        format: "html",
        fields: {
          title: {
            selector: { kind: "xpath", expr: "//title/text()" },
            parser: "string",
          },
        },
      },
    };
    const state = stateFromDraft(draft);
    expect(state.workflowEnabled).toBe(true);
    expect(state.workflowSelectors).toHaveLength(1);
    expect(state.workflowSelectors[0].name).toBe("link");
    expect(state.workflowMatchMode).toBe("and");
    expect(state.workflowExpectCountText).toBe("1");
    expect(state.workflowReturn).toBe("${item.link}");
    expect(state.workflowNextRequest.method).toBe("GET");
    expect(state.workflowNextRequest.rawURL).toBe("${item.link}");
  });
});

// ---------------------------------------------------------------------------
// buildDraft → stateFromDraft roundtrip
// ---------------------------------------------------------------------------
describe("buildDraft → stateFromDraft roundtrip", () => {
  it("roundtrips default state", () => {
    const original = defaultState();
    const draft = buildDraft(original);
    const restored = stateFromDraft(draft);
    // Compare key structural fields (ignoring dynamic IDs, number, importYAML)
    expect(restored.name).toBe(original.name);
    expect(restored.type).toBe(original.type);
    expect(restored.hostsText).toBe(original.hostsText);
    expect(restored.request.method).toBe(original.request.method);
    expect(restored.request.path).toBe(original.request.path);
    expect(restored.scrapeFormat).toBe(original.scrapeFormat);
    expect(restored.fields).toHaveLength(original.fields.length);
    expect(restored.fields[0].name).toBe(original.fields[0].name);
  });

  it("roundtrips with workflow", () => {
    const original = defaultState();
    original.workflowEnabled = true;
    original.workflowMatchConditionsText = 'contains("${item.read_link}", "${number}")';
    original.workflowExpectCountText = "3";
    const draft = buildDraft(original);
    const restored = stateFromDraft(draft);
    expect(restored.workflowEnabled).toBe(true);
    expect(restored.workflowMatchConditionsText).toBe(original.workflowMatchConditionsText);
    expect(restored.workflowExpectCountText).toBe("3");
  });

  it("roundtrips with postprocess", () => {
    const original = defaultState();
    original.postTitleLang = "ja";
    original.postAssign = [{ id: "1", key: "title", value: "${meta.title}!" }];
    original.postDisableReleaseDateCheck = true;
    const draft = buildDraft(original);
    const restored = stateFromDraft(draft);
    expect(restored.postTitleLang).toBe("ja");
    expect(restored.postAssign).toHaveLength(1);
    expect(restored.postAssign[0].key).toBe("title");
    expect(restored.postDisableReleaseDateCheck).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// buildDefaults / buildSwitchConfig
// ---------------------------------------------------------------------------
describe("buildDefaults", () => {
  it("returns null when all empty", () => {
    expect(buildDefaults(defaultState())).toBeNull();
  });

  it("returns object when any lang set", () => {
    const state = defaultState();
    state.postGenresLang = "zh-cn";
    const result = buildDefaults(state);
    expect(result?.genres_lang).toBe("zh-cn");
  });
});

describe("buildSwitchConfig", () => {
  it("returns null when both false", () => {
    expect(buildSwitchConfig(defaultState())).toBeNull();
  });

  it("returns config when set", () => {
    const state = defaultState();
    state.postDisableNumberReplace = true;
    const result = buildSwitchConfig(state);
    expect(result?.disable_number_replace).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// draftToFields
// ---------------------------------------------------------------------------
describe("draftToFields", () => {
  it("returns default field when draft has no fields", () => {
    const draft: PluginEditorDraft = {
      version: 1,
      name: "x",
      type: "one-step",
      hosts: ["https://example.com"],
      scrape: { format: "html", fields: {} },
    };
    const fields = draftToFields(draft);
    expect(fields).toHaveLength(1);
    expect(fields[0].name).toBe("title");
  });

  it("converts fields correctly", () => {
    const draft: PluginEditorDraft = {
      version: 1,
      name: "x",
      type: "one-step",
      hosts: ["https://example.com"],
      scrape: {
        format: "html",
        fields: {
          actors: {
            selector: { kind: "xpath", expr: "//span[@class='actor']/text()", multi: true },
            parser: "string_list",
            required: false,
            transforms: [{ kind: "trim" }, { kind: "to_upper" }],
          },
        },
      },
    };
    const fields = draftToFields(draft);
    expect(fields).toHaveLength(1);
    expect(fields[0].name).toBe("actors");
    expect(fields[0].selectorMulti).toBe(true);
    expect(fields[0].parserKind).toBe("string_list");
    expect(fields[0].transforms).toHaveLength(2);
  });
});

// ---------------------------------------------------------------------------
// normalizeEditorState
// ---------------------------------------------------------------------------
describe("normalizeEditorState", () => {
  it("normalizes a valid state without changes", () => {
    const state = defaultState();
    const result = normalizeEditorState(state);
    expect(result.name).toBe(state.name);
    expect(result.request.method).toBe("GET");
    expect(result.fields).toHaveLength(1);
  });

  it("handles legacy flat request fields", () => {
    const state = {
      ...defaultState(),
      // Simulate legacy flat format stored in localStorage
    } as EditorState & Record<string, unknown>;
    // Remove nested request and add flat legacy fields
    (state as Record<string, unknown>)["request"] = undefined;
    (state as Record<string, unknown>)["requestMethod"] = "POST";
    (state as Record<string, unknown>)["requestPath"] = "/legacy/path";
    const result = normalizeEditorState(state);
    expect(result.request.method).toBe("POST");
    expect(result.request.path).toBe("/legacy/path");
  });

  it("fills default workflow selectors when empty", () => {
    const state = defaultState();
    state.workflowSelectors = [];
    const result = normalizeEditorState(state);
    expect(result.workflowSelectors).toHaveLength(1);
    expect(result.workflowSelectors[0].name).toBe("read_link");
  });
});
