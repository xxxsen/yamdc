import { describe, expect, it } from "vitest";

import { DEFAULT_FIELD, DEFAULT_REQUEST_FORM_STATE } from "../plugin-editor-constants";
import {
  addField,
  addKVPair,
  addTransform,
  addWorkflowSelector,
  patch,
  patchField,
  patchKVPair,
  patchRequest,
  patchTransform,
  patchWorkflowSelector,
  removeField,
  removeKVPair,
  removeTransform,
  removeWorkflowSelector,
  updateFieldName,
} from "../plugin-editor-state-ops";
import type { EditorState, FieldForm, KVPairForm, TransformForm, WorkflowSelectorForm } from "../plugin-editor-types";

// plugin-editor-state-ops: 16 个 StateOp 工厂. 每个 op 签名都是
// `op(args...) => (prev) => next`, 每条分支 (命中 id / 未命中 id / 空列表 /
// 单元素不允许删等) 都要在这里显式覆盖.

function baseField(overrides: Partial<FieldForm> = {}): FieldForm {
  return { ...DEFAULT_FIELD, transforms: [...DEFAULT_FIELD.transforms], ...overrides };
}

function baseTransform(overrides: Partial<TransformForm> = {}): TransformForm {
  return {
    id: "t1",
    kind: "trim",
    old: "",
    newValue: "",
    cutset: "",
    sep: "",
    index: "",
    value: "",
    ...overrides,
  };
}

function baseKV(overrides: Partial<KVPairForm> = {}): KVPairForm {
  return { id: "kv1", key: "", value: "", ...overrides };
}

function baseSelector(overrides: Partial<WorkflowSelectorForm> = {}): WorkflowSelectorForm {
  return { id: "s1", name: "", kind: "xpath", expr: "", ...overrides };
}

function baseState(overrides: Partial<EditorState> = {}): EditorState {
  return {
    name: "",
    type: "",
    fetchType: "",
    hostsText: "",
    number: "",
    precheckPatternsText: "",
    precheckVariables: [],
    request: { ...DEFAULT_REQUEST_FORM_STATE },
    scrapeFormat: "html",
    fields: [baseField({ id: "f1", name: "title" })],
    multiRequestEnabled: false,
    multiCandidatesText: "",
    multiUnique: false,
    multiRequest: { ...DEFAULT_REQUEST_FORM_STATE },
    multiSuccessMode: "",
    multiSuccessConditionsText: "",
    workflowEnabled: false,
    workflowSelectors: [baseSelector()],
    workflowItemVariables: [],
    workflowMatchMode: "",
    workflowMatchConditionsText: "",
    workflowExpectCountText: "",
    workflowReturn: "",
    workflowNextRequest: { ...DEFAULT_REQUEST_FORM_STATE },
    postAssign: [],
    postTitleLang: "",
    postPlotLang: "",
    postGenresLang: "",
    postActorsLang: "",
    postDisableReleaseDateCheck: false,
    postDisableNumberReplace: false,
    importYAML: "",
    ...overrides,
  };
}

describe("patch", () => {
  it("sets a scalar key immutably", () => {
    const prev = baseState({ name: "before" });
    const next = patch("name", "after")(prev);
    expect(next.name).toBe("after");
    expect(prev.name).toBe("before");
  });

  it("works on boolean keys", () => {
    const next = patch("workflowEnabled", true)(baseState());
    expect(next.workflowEnabled).toBe(true);
  });

  it("returns a new object reference (not in-place mutation)", () => {
    const prev = baseState();
    const next = patch("name", "x")(prev);
    expect(next).not.toBe(prev);
  });
});

describe("patchRequest", () => {
  it("applies updater to the named request sub-state", () => {
    const next = patchRequest("request", (req) => ({ ...req, method: "POST" }))(baseState());
    expect(next.request.method).toBe("POST");
  });

  it("covers all 3 request keys", () => {
    const a = patchRequest("multiRequest", (r) => ({ ...r, path: "/a" }))(baseState());
    expect(a.multiRequest.path).toBe("/a");
    const b = patchRequest("workflowNextRequest", (r) => ({ ...r, path: "/b" }))(baseState());
    expect(b.workflowNextRequest.path).toBe("/b");
  });

  it("leaves other request objects untouched", () => {
    const prev = baseState();
    const next = patchRequest("request", (r) => ({ ...r, method: "POST" }))(prev);
    expect(next.multiRequest).toBe(prev.multiRequest);
  });
});

describe("patchField / updateFieldName", () => {
  it("patchField: only the matched id is replaced", () => {
    const state = baseState({
      fields: [
        baseField({ id: "a", name: "title" }),
        baseField({ id: "b", name: "actors" }),
      ],
    });
    const next = patchField("b", (f) => ({ ...f, required: false }))(state);
    expect(next.fields[0]).toBe(state.fields[0]);
    expect(next.fields[1].required).toBe(false);
  });

  it("patchField: no match is no-op for fields", () => {
    const state = baseState();
    const next = patchField("missing", (f) => ({ ...f, required: false }))(state);
    expect(next.fields).toEqual(state.fields);
  });

  it("updateFieldName: re-runs applyFieldMeta to reset parser when name changes", () => {
    const state = baseState({
      fields: [baseField({ id: "a", name: "title", parserKind: "string" })],
    });
    // 改名到 actors, 因为 actors 的 fixedParser=string_list, applyFieldMeta 会把
    // parserKind 强改掉.
    const next = updateFieldName("a", "actors")(state);
    expect(next.fields[0].name).toBe("actors");
    expect(next.fields[0].parserKind).toBe("string_list");
  });

  it("updateFieldName: missing id is no-op", () => {
    const state = baseState();
    const next = updateFieldName("missing", "actors")(state);
    expect(next.fields).toEqual(state.fields);
  });
});

describe("patchWorkflowSelector", () => {
  it("patches matched selector", () => {
    const state = baseState({
      workflowSelectors: [baseSelector({ id: "s1" }), baseSelector({ id: "s2" })],
    });
    const next = patchWorkflowSelector("s2", (s) => ({ ...s, name: "new" }))(state);
    expect(next.workflowSelectors[1].name).toBe("new");
    expect(next.workflowSelectors[0]).toBe(state.workflowSelectors[0]);
  });

  it("missing id is no-op", () => {
    const state = baseState();
    const next = patchWorkflowSelector("missing", (s) => ({ ...s, name: "x" }))(state);
    expect(next.workflowSelectors).toEqual(state.workflowSelectors);
  });
});

describe("patchKVPair / addKVPair / removeKVPair", () => {
  it("patch matches by id in the target bucket", () => {
    const state = baseState({ postAssign: [baseKV({ id: "a", key: "x" }), baseKV({ id: "b" })] });
    const next = patchKVPair("postAssign", "b", (kv) => ({ ...kv, value: "y" }))(state);
    expect(next.postAssign[1].value).toBe("y");
    expect(next.postAssign[0].key).toBe("x");
  });

  it("add: workflowItemVariables gets empty key", () => {
    const state = baseState();
    const next = addKVPair("workflowItemVariables")(state);
    expect(next.workflowItemVariables).toHaveLength(1);
    expect(next.workflowItemVariables[0].key).toBe("");
  });

  it("add: postAssign pre-fills next unused field name", () => {
    const state = baseState({ postAssign: [baseKV({ id: "a", key: "number" })] });
    const next = addKVPair("postAssign")(state);
    expect(next.postAssign).toHaveLength(2);
    // postAssign 里已经占用 "number", nextUnusedKVFieldName 会挑下一个 FIELD_OPTIONS 里没用过的.
    expect(next.postAssign[1].key).not.toBe("");
    expect(next.postAssign[1].key).not.toBe("number");
  });

  it("add: precheckVariables gets empty key", () => {
    const state = baseState();
    const next = addKVPair("precheckVariables")(state);
    expect(next.precheckVariables[0].key).toBe("");
  });

  it("remove: drops the target; absent id is no-op", () => {
    const state = baseState({ postAssign: [baseKV({ id: "a" }), baseKV({ id: "b" })] });
    const next1 = removeKVPair("postAssign", "a")(state);
    expect(next1.postAssign).toHaveLength(1);
    expect(next1.postAssign[0].id).toBe("b");

    const next2 = removeKVPair("postAssign", "missing")(state);
    expect(next2.postAssign).toEqual(state.postAssign);
  });
});

describe("patchTransform", () => {
  it("only the field.transforms of the target field is touched", () => {
    const state = baseState({
      fields: [
        baseField({ id: "a", transforms: [baseTransform({ id: "t1" })] }),
        baseField({ id: "b", transforms: [baseTransform({ id: "t2" }), baseTransform({ id: "t3" })] }),
      ],
    });
    const next = patchTransform("b", "t3", (t) => ({ ...t, value: "X" }))(state);
    expect(next.fields[1].transforms[1].value).toBe("X");
    expect(next.fields[1].transforms[0]).toBe(state.fields[1].transforms[0]);
    expect(next.fields[0]).toBe(state.fields[0]);
  });

  it("missing field id is a no-op for fields", () => {
    const state = baseState();
    const next = patchTransform("missing", "t1", (t) => ({ ...t, value: "X" }))(state);
    expect(next.fields).toEqual(state.fields);
  });

  it("missing transform id is a no-op for that field's transforms", () => {
    const state = baseState({
      fields: [baseField({ id: "a", transforms: [baseTransform({ id: "t1" })] })],
    });
    const next = patchTransform("a", "missing", (t) => ({ ...t, value: "X" }))(state);
    expect(next.fields[0].transforms).toEqual(state.fields[0].transforms);
  });
});

describe("addField / removeField", () => {
  it("addField: no-op when all FIELD_OPTIONS are exhausted", () => {
    // 构造 fields 占满 FIELD_OPTIONS 里所有选项.
    const allNames = ["number", "title", "plot", "actors", "release_date", "duration", "studio", "label", "series", "director", "genres", "cover", "poster", "sample_images"];
    const state = baseState({
      fields: allNames.map((name, i) => baseField({ id: `f${i}`, name })),
    });
    const next = addField()(state);
    expect(next).toBe(state);
  });

  it("addField: picks next unused name when capacity remains", () => {
    const state = baseState({ fields: [baseField({ id: "f1", name: "title" })] });
    const next = addField()(state);
    expect(next.fields).toHaveLength(2);
    expect(next.fields[1].name).not.toBe("title");
    // applyFieldMeta 在 name=number 时会把 parserKind 强制为 string.
    expect(next.fields[1].parserKind).toBeTruthy();
  });

  it("removeField: drops target", () => {
    const state = baseState({
      fields: [baseField({ id: "a" }), baseField({ id: "b" })],
    });
    const next = removeField("a")(state);
    expect(next.fields).toHaveLength(1);
    expect(next.fields[0].id).toBe("b");
  });

  it("removeField: refuses to drop the last remaining field", () => {
    const state = baseState({ fields: [baseField({ id: "only" })] });
    const next = removeField("only")(state);
    expect(next.fields).toEqual(state.fields);
  });

  it("removeField: missing id is no-op (but preserves array identity only when length > 1)", () => {
    const state = baseState({ fields: [baseField({ id: "a" }), baseField({ id: "b" })] });
    const next = removeField("missing")(state);
    expect(next.fields).toHaveLength(2);
  });
});

describe("addTransform / removeTransform", () => {
  it("addTransform: appends when afterTransformID is undefined", () => {
    const state = baseState({
      fields: [baseField({ id: "a", transforms: [baseTransform({ id: "t1" })] })],
    });
    const next = addTransform("a")(state);
    expect(next.fields[0].transforms).toHaveLength(2);
    expect(next.fields[0].transforms[0].id).toBe("t1");
  });

  it("addTransform: inserts after the target id", () => {
    const state = baseState({
      fields: [
        baseField({
          id: "a",
          transforms: [baseTransform({ id: "t1" }), baseTransform({ id: "t2" })],
        }),
      ],
    });
    const next = addTransform("a", "t1")(state);
    expect(next.fields[0].transforms).toHaveLength(3);
    expect(next.fields[0].transforms[0].id).toBe("t1");
    expect(next.fields[0].transforms[2].id).toBe("t2");
  });

  it("addTransform: afterTransformID not found -> appends", () => {
    const state = baseState({
      fields: [baseField({ id: "a", transforms: [baseTransform({ id: "t1" })] })],
    });
    const next = addTransform("a", "missing")(state);
    expect(next.fields[0].transforms).toHaveLength(2);
  });

  it("addTransform: missing field id is no-op for fields", () => {
    const state = baseState();
    const next = addTransform("missing")(state);
    expect(next.fields).toEqual(state.fields);
  });

  it("removeTransform: drops target", () => {
    const state = baseState({
      fields: [
        baseField({
          id: "a",
          transforms: [baseTransform({ id: "t1" }), baseTransform({ id: "t2" })],
        }),
      ],
    });
    const next = removeTransform("a", "t1")(state);
    expect(next.fields[0].transforms).toHaveLength(1);
    expect(next.fields[0].transforms[0].id).toBe("t2");
  });

  it("removeTransform: refuses to drop the last transform of a field", () => {
    const state = baseState({
      fields: [baseField({ id: "a", transforms: [baseTransform({ id: "t1" })] })],
    });
    const next = removeTransform("a", "t1")(state);
    expect(next.fields[0].transforms).toEqual(state.fields[0].transforms);
  });

  it("removeTransform: missing field id is no-op for fields", () => {
    const state = baseState();
    const next = removeTransform("missing", "t1")(state);
    expect(next.fields).toEqual(state.fields);
  });
});

describe("addWorkflowSelector / removeWorkflowSelector", () => {
  it("addWorkflowSelector: appends new selector with kind=xpath", () => {
    const state = baseState({ workflowSelectors: [baseSelector({ id: "s1" })] });
    const next = addWorkflowSelector()(state);
    expect(next.workflowSelectors).toHaveLength(2);
    expect(next.workflowSelectors[1].kind).toBe("xpath");
  });

  it("removeWorkflowSelector: drops target", () => {
    const state = baseState({
      workflowSelectors: [baseSelector({ id: "a" }), baseSelector({ id: "b" })],
    });
    const next = removeWorkflowSelector("a")(state);
    expect(next.workflowSelectors).toHaveLength(1);
    expect(next.workflowSelectors[0].id).toBe("b");
  });

  it("removeWorkflowSelector: refuses to drop the last remaining selector", () => {
    const state = baseState({ workflowSelectors: [baseSelector({ id: "only" })] });
    const next = removeWorkflowSelector("only")(state);
    expect(next.workflowSelectors).toEqual(state.workflowSelectors);
  });

  it("removeWorkflowSelector: missing id is no-op", () => {
    const state = baseState({
      workflowSelectors: [baseSelector({ id: "a" }), baseSelector({ id: "b" })],
    });
    const next = removeWorkflowSelector("missing")(state);
    expect(next.workflowSelectors).toHaveLength(2);
  });
});
