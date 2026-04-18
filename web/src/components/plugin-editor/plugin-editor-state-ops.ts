// plugin-editor-state-ops: usePluginEditorState 里的 **纯更新函数**.
//
// 每个 op 的签名统一成 "高阶函数风格": `op(args...) => (prev) => next`,
// 这样 hook 那一侧就可以直接 `setState(ops.addField())` 或
// `setState(ops.patchField(id, updater))`, 不用再写 `setState((prev) => {...})`
// 样板. 还顺便修掉了 "在 setState 外面读 state.fields" 的 stale-closure
// 隐患 (现在一切计算都在 prev 里做).
//
// 所有 op 都是纯函数, 可以直接在单测里 `op(args)(prev)` 断言输出.

import { DEFAULT_FIELD } from "./plugin-editor-constants";
import type {
  EditorState,
  FieldForm,
  KVPairForm,
  RequestFormState,
  TransformForm,
  WorkflowSelectorForm,
} from "./plugin-editor-types";
import {
  applyFieldMeta,
  insertTransform,
  nextUnusedFieldName,
  nextUnusedKVFieldName,
} from "./plugin-editor-utils";

export type KVPairKey = "workflowItemVariables" | "postAssign" | "precheckVariables";

type Updater<T> = (prev: T) => T;
type StateOp = Updater<EditorState>;

export function patch<K extends keyof EditorState>(key: K, value: EditorState[K]): StateOp {
  return (prev) => ({ ...prev, [key]: value });
}

export function patchRequest(
  key: "request" | "multiRequest" | "workflowNextRequest",
  updater: Updater<RequestFormState>,
): StateOp {
  return (prev) => ({ ...prev, [key]: updater(prev[key]) });
}

export function patchField(id: string, updater: Updater<FieldForm>): StateOp {
  return (prev) => ({
    ...prev,
    fields: prev.fields.map((field) => (field.id === id ? updater(field) : field)),
  });
}

export function updateFieldName(id: string, nextName: string): StateOp {
  return (prev) => ({
    ...prev,
    fields: prev.fields.map((field) => (field.id === id ? applyFieldMeta({ ...field, name: nextName }) : field)),
  });
}

export function patchWorkflowSelector(id: string, updater: Updater<WorkflowSelectorForm>): StateOp {
  return (prev) => ({
    ...prev,
    workflowSelectors: prev.workflowSelectors.map((selector) => (selector.id === id ? updater(selector) : selector)),
  });
}

export function patchKVPair(key: KVPairKey, id: string, updater: Updater<KVPairForm>): StateOp {
  return (prev) => ({
    ...prev,
    [key]: prev[key].map((item) => (item.id === id ? updater(item) : item)),
  });
}

export function addKVPair(key: KVPairKey): StateOp {
  return (prev) => {
    const nextKey = key === "postAssign" ? nextUnusedKVFieldName(prev.postAssign) : "";
    return {
      ...prev,
      [key]: [...prev[key], { id: `kv-${Date.now()}`, key: nextKey, value: "" }],
    };
  };
}

export function removeKVPair(key: KVPairKey, id: string): StateOp {
  return (prev) => ({
    ...prev,
    [key]: prev[key].filter((item) => item.id !== id),
  });
}

export function patchTransform(fieldID: string, transformID: string, updater: Updater<TransformForm>): StateOp {
  return (prev) => ({
    ...prev,
    fields: prev.fields.map((field) =>
      field.id !== fieldID
        ? field
        : {
            ...field,
            transforms: field.transforms.map((transform) => (transform.id === transformID ? updater(transform) : transform)),
          },
    ),
  });
}

export function addField(): StateOp {
  return (prev) => {
    const nextName = nextUnusedFieldName(prev.fields);
    if (!nextName) return prev;
    return {
      ...prev,
      fields: [
        ...prev.fields,
        applyFieldMeta({ ...DEFAULT_FIELD, id: `field-${Date.now()}`, name: nextName }),
      ],
    };
  };
}

export function removeField(id: string): StateOp {
  return (prev) => ({
    ...prev,
    fields: prev.fields.length === 1 ? prev.fields : prev.fields.filter((field) => field.id !== id),
  });
}

export function addTransform(fieldID: string, afterTransformID?: string): StateOp {
  return (prev) => ({
    ...prev,
    fields: prev.fields.map((field) =>
      field.id !== fieldID ? field : { ...field, transforms: insertTransform(field.transforms, afterTransformID) },
    ),
  });
}

export function removeTransform(fieldID: string, transformID: string): StateOp {
  return (prev) => ({
    ...prev,
    fields: prev.fields.map((field) =>
      field.id !== fieldID
        ? field
        : {
            ...field,
            transforms:
              field.transforms.length === 1
                ? field.transforms
                : field.transforms.filter((transform) => transform.id !== transformID),
          },
    ),
  });
}

export function addWorkflowSelector(): StateOp {
  return (prev) => ({
    ...prev,
    workflowSelectors: [
      ...prev.workflowSelectors,
      { id: `selector-${Date.now()}`, name: "", kind: "xpath", expr: "" },
    ],
  });
}

export function removeWorkflowSelector(id: string): StateOp {
  return (prev) => ({
    ...prev,
    workflowSelectors:
      prev.workflowSelectors.length === 1
        ? prev.workflowSelectors
        : prev.workflowSelectors.filter((item) => item.id !== id),
  });
}
