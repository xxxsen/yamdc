import type { PluginEditorDraft } from "@/lib/api";

export type EditorTab = "compile" | "basic" | "request" | "response" | "workflow" | "scrape" | "draft";
export type EditorSection = "basic" | "request" | "scrape" | "postprocess";
export type RunAction = "compile" | "request" | "workflow" | "scrape";
export type ToastState = { message: string; tone: "info" | "danger" } | null;

export type FieldForm = {
  id: string;
  name: string;
  selectorKind: string;
  selectorExpr: string;
  selectorMulti: boolean;
  parserKind: string;
  parserLayout: string;
  required: boolean;
  transforms: TransformForm[];
};

export type WorkflowSelectorForm = {
  id: string;
  name: string;
  kind: string;
  expr: string;
};

export type TransformForm = {
  id: string;
  kind: string;
  old: string;
  newValue: string;
  cutset: string;
  sep: string;
  index: string;
  value: string;
};

export type KVPairForm = {
  id: string;
  key: string;
  value: string;
};

export type RequestBodyDraft = NonNullable<NonNullable<PluginEditorDraft["request"]>["body"]>;

export type FieldMeta = {
  label: string;
  fixedParser?: string;
  parserOptions?: string[];
  defaultParser?: string;
  fixedMulti?: boolean;
};

export type RequestFormState = {
  method: string;
  path: string;
  rawURL: string;
  queryJSON: string;
  headersJSON: string;
  cookiesJSON: string;
  bodyKind: string;
  bodyJSON: string;
  acceptStatusText: string;
  notFoundStatusText: string;
  decodeCharset: string;
};

export type EditorState = {
  name: string;
  type: string;
  hostsText: string;
  number: string;
  precheckPatternsText: string;
  precheckVariables: KVPairForm[];
  request: RequestFormState;
  scrapeFormat: string;
  fields: FieldForm[];
  multiRequestEnabled: boolean;
  multiCandidatesText: string;
  multiUnique: boolean;
  multiRequest: RequestFormState;
  multiSuccessMode: string;
  multiSuccessConditionsText: string;
  workflowEnabled: boolean;
  workflowSelectors: WorkflowSelectorForm[];
  workflowItemVariables: KVPairForm[];
  workflowMatchMode: string;
  workflowMatchConditionsText: string;
  workflowExpectCountText: string;
  workflowReturn: string;
  workflowNextRequest: RequestFormState;
  postAssign: KVPairForm[];
  postTitleLang: string;
  postPlotLang: string;
  postGenresLang: string;
  postActorsLang: string;
  postDisableReleaseDateCheck: boolean;
  postDisableNumberReplace: boolean;
  importYAML: string;
};
