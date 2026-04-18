"use client";

import { Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";

import { FIELD_OPTIONS } from "./plugin-editor-constants";
import type { FieldForm, TransformForm } from "./plugin-editor-types";
import {
  getFieldMeta,
  needsCutset,
  needsIndex,
  needsOldNew,
  needsSep,
  needsValue,
  showParserLayout,
  valueLabelForKind,
} from "./plugin-editor-utils";

export function FieldCard(props: {
  field: FieldForm;
  allFields: FieldForm[];
  onPatchField: (updater: (field: FieldForm) => FieldForm) => void;
  onUpdateName: (nextName: string) => void;
  onRemove: () => void;
  onAddTransform: (afterTransformID?: string) => void;
  onRemoveTransform: (transformID: string) => void;
  onPatchTransform: (transformID: string, updater: (transform: TransformForm) => TransformForm) => void;
}) {
  const { field } = props;
  const fieldMeta = getFieldMeta(field.name);
  const showParserKind = Boolean(fieldMeta.parserOptions && fieldMeta.parserOptions.length > 0);
  const showMultiSelector = typeof fieldMeta.fixedMulti !== "boolean";
  const selectableFields = FIELD_OPTIONS.filter((option) => option === field.name || !props.allFields.some((item) => item.id !== field.id && item.name === option));

  return (
    <div className="plugin-editor-field-card">
      <div className="plugin-editor-field-card-rows">
        <div className="plugin-editor-field-inline-row">
          <label className="plugin-editor-field-inline plugin-editor-field-inline-name">
            <span>Field</span>
            <select className="input" value={field.name} onChange={(event) => props.onUpdateName(event.target.value)}>
              {selectableFields.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
              {!FIELD_OPTIONS.includes(field.name) ? (
                <option value={field.name}>{field.name}</option>
              ) : null}
            </select>
          </label>
          <label className="plugin-editor-field-inline plugin-editor-field-inline-kind">
            <span>Kind</span>
            <select className="input" value={field.selectorKind} onChange={(event) => props.onPatchField((prev) => ({ ...prev, selectorKind: event.target.value }))}>
              <option value="xpath">xpath</option>
              <option value="jsonpath">jsonpath</option>
            </select>
          </label>
          <label className="plugin-editor-field-inline plugin-editor-field-inline-expr">
            <span>Expr</span>
            <input className="input" value={field.selectorExpr} onChange={(event) => props.onPatchField((prev) => ({ ...prev, selectorExpr: event.target.value }))} />
          </label>
          <label className="searcher-debug-switch plugin-editor-field-inline-required">
            <input type="checkbox" checked={field.required} onChange={(event) => props.onPatchField((prev) => ({ ...prev, required: event.target.checked }))} />
            <span>REQUIRED</span>
          </label>
          <Button
            className="plugin-editor-field-card-remove"
            aria-label="删除字段"
            title="删除字段"
            onClick={props.onRemove}
          >
            <Trash2 size={16} />
          </Button>
        </div>

        <div className="plugin-editor-field-inline-row">
          {showParserKind ? (
            <label className="plugin-editor-field-inline plugin-editor-field-inline-name">
              <span>Parse As</span>
              <select className="input" value={field.parserKind} onChange={(event) => props.onPatchField((prev) => ({ ...prev, parserKind: event.target.value }))}>
                {(fieldMeta.parserOptions ?? []).map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
                {field.parserKind && !(fieldMeta.parserOptions ?? []).includes(field.parserKind) ? (
                  <option value={field.parserKind}>{field.parserKind}</option>
                ) : null}
              </select>
            </label>
          ) : null}
          {showParserLayout(field.parserKind) ? (
            <label className="plugin-editor-field-inline plugin-editor-field-inline-kind">
              <span>Layout</span>
              <input className="input" value={field.parserLayout} onChange={(event) => props.onPatchField((prev) => ({ ...prev, parserLayout: event.target.value }))} />
            </label>
          ) : null}
          <div className="plugin-editor-field-inline-switches">
            {showMultiSelector ? (
              <label className="searcher-debug-switch">
                <input type="checkbox" checked={field.selectorMulti} onChange={(event) => props.onPatchField((prev) => ({ ...prev, selectorMulti: event.target.checked }))} />
                <span>multi selector</span>
              </label>
            ) : null}
          </div>
        </div>

        <div className="plugin-editor-field plugin-editor-field-wide">
          <span>Transforms</span>
          <div className="plugin-editor-transform-list">
            {field.transforms.map((transform) => (
              <div key={transform.id} className="plugin-editor-transform-card">
                <div className="plugin-editor-transform-actions">
                  <Button
                    className="plugin-editor-transform-action"
                    aria-label="新增 transform"
                    title="新增 transform"
                    onClick={() => props.onAddTransform(transform.id)}
                  >
                    <Plus size={14} />
                  </Button>
                  <Button
                    className="plugin-editor-transform-action"
                    aria-label="删除 transform"
                    title="删除 transform"
                    onClick={() => props.onRemoveTransform(transform.id)}
                  >
                    <span aria-hidden="true">×</span>
                  </Button>
                </div>
                <label className="plugin-editor-transform-inline-field plugin-editor-transform-inline-field-kind">
                  <span>Kind</span>
                  <select
                    className="input"
                    value={transform.kind}
                    onChange={(event) => props.onPatchTransform(transform.id, (prev) => ({ ...prev, kind: event.target.value }))}
                  >
                    <option value="trim">trim</option>
                    <option value="trim_prefix">trim_prefix</option>
                    <option value="trim_suffix">trim_suffix</option>
                    <option value="trim_charset">trim_charset</option>
                    <option value="replace">replace</option>
                    <option value="regex_extract">regex_extract</option>
                    <option value="split_index">split_index</option>
                    <option value="split">split</option>
                    <option value="map_trim">map_trim</option>
                    <option value="remove_empty">remove_empty</option>
                    <option value="dedupe">dedupe</option>
                    <option value="to_upper">to_upper</option>
                    <option value="to_lower">to_lower</option>
                  </select>
                </label>
                <TransformParamFields
                  transform={transform}
                  onChange={(updater) => props.onPatchTransform(transform.id, updater)}
                />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function TransformParamFields(props: {
  transform: TransformForm;
  onChange: (updater: (prev: TransformForm) => TransformForm) => void;
}) {
  const { transform } = props;
  const paramCount =
    (needsOldNew(transform.kind) ? 2 : 0) +
    (needsValue(transform.kind) ? 1 : 0) +
    (needsSep(transform.kind) ? 1 : 0) +
    (needsCutset(transform.kind) ? 1 : 0) +
    (needsIndex(transform.kind) ? 1 : 0);

  return (
    <>
      {needsOldNew(transform.kind) ? (
        <>
          <label className="plugin-editor-transform-inline-field">
            <span>Old</span>
            <input className="input" value={transform.old} onChange={(event) => props.onChange((prev) => ({ ...prev, old: event.target.value }))} />
          </label>
          <label className="plugin-editor-transform-inline-field">
            <span>New</span>
            <input className="input" value={transform.newValue} onChange={(event) => props.onChange((prev) => ({ ...prev, newValue: event.target.value }))} />
          </label>
        </>
      ) : null}
      {needsValue(transform.kind) ? (
        <label className="plugin-editor-transform-inline-field">
          <span>{valueLabelForKind(transform.kind)}</span>
          <input className="input" value={transform.value} onChange={(event) => props.onChange((prev) => ({ ...prev, value: event.target.value }))} />
        </label>
      ) : null}
      {needsSep(transform.kind) ? (
        <label className="plugin-editor-transform-inline-field">
          <span>Sep</span>
          <input className="input" value={transform.sep} onChange={(event) => props.onChange((prev) => ({ ...prev, sep: event.target.value }))} />
        </label>
      ) : null}
      {needsCutset(transform.kind) ? (
        <label className="plugin-editor-transform-inline-field">
          <span>Cutset</span>
          <input className="input" value={transform.cutset} onChange={(event) => props.onChange((prev) => ({ ...prev, cutset: event.target.value }))} />
        </label>
      ) : null}
      {needsIndex(transform.kind) ? (
        <label className="plugin-editor-transform-inline-field plugin-editor-transform-inline-field-index">
          <span>Index</span>
          <input className="input" value={transform.index} onChange={(event) => props.onChange((prev) => ({ ...prev, index: event.target.value }))} />
        </label>
      ) : null}
      {paramCount === 1 ? <div className="plugin-editor-transform-inline-spacer" aria-hidden="true" /> : null}
    </>
  );
}
