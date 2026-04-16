import type { FieldForm, FieldMeta, RequestFormState } from "./plugin-editor-types";

export const DEFAULT_DRAFT_STORAGE_KEY = "yamdc.debug.plugin-editor.draft.v2";
export const DEFAULT_NUMBER_STORAGE_KEY = "yamdc.debug.plugin-editor.number";

export const FIELD_META: Record<string, FieldMeta> = {
  number: { label: "number", fixedParser: "string", fixedMulti: false },
  title: { label: "title", fixedParser: "string", fixedMulti: false },
  plot: { label: "plot", fixedParser: "string", fixedMulti: false },
  actors: { label: "actors", fixedParser: "string_list", fixedMulti: true },
  release_date: {
    label: "release_date",
    parserOptions: ["date_only", "date_layout_soft", "time_format"],
    defaultParser: "date_layout_soft",
    fixedMulti: false,
  },
  duration: {
    label: "duration",
    parserOptions: ["duration_default", "duration_hhmmss", "duration_mm", "duration_mmss", "duration_human"],
    defaultParser: "duration_default",
    fixedMulti: false,
  },
  studio: { label: "studio", fixedParser: "string", fixedMulti: false },
  label: { label: "label", fixedParser: "string", fixedMulti: false },
  series: { label: "series", fixedParser: "string", fixedMulti: false },
  director: { label: "director", fixedParser: "string", fixedMulti: false },
  genres: { label: "genres", fixedParser: "string_list", fixedMulti: true },
  cover: { label: "cover", fixedParser: "string", fixedMulti: false },
  poster: { label: "poster", fixedParser: "string", fixedMulti: false },
  sample_images: { label: "sample_images", fixedParser: "string_list", fixedMulti: true },
};

export const FIELD_OPTIONS = Object.keys(FIELD_META);
export const META_LANG_OPTIONS = ["ja", "en", "zh-cn", "zh-tw"] as const;

export const IMPORT_YAML_EXAMPLE = `version: 1
name: fixture
type: one-step
hosts:
  - https://example.com
request:
  method: GET
  path: /search/\${number}
scrape:
  format: html
  fields:
    title:
      selector:
        kind: xpath
        expr: //title/text()
      parser: string
      required: true`;

export const DEFAULT_REQUEST_FORM_STATE: RequestFormState = {
  method: "GET",
  path: "",
  rawURL: "",
  queryJSON: "{}",
  headersJSON: "{}",
  cookiesJSON: "{}",
  bodyKind: "json",
  bodyJSON: "null",
  acceptStatusText: "200",
  notFoundStatusText: "404",
  decodeCharset: "",
  browserWaitSelector: "",
  browserWaitTimeout: "",
  browserWaitStable: "",
};

export const DEFAULT_FIELD: FieldForm = {
  id: "title",
  name: "title",
  selectorKind: "xpath",
  selectorExpr: "//title/text()",
  selectorMulti: false,
  parserKind: "string",
  parserLayout: "",
  required: true,
  transforms: [
    {
      id: "transform-1",
      kind: "trim",
      old: "",
      newValue: "",
      cutset: "",
      sep: "",
      index: "",
      value: "",
    },
  ],
};
