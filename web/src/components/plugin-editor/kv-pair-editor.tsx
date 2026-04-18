"use client";

import { Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";

import { FIELD_OPTIONS } from "./plugin-editor-constants";
import type { KVPairForm } from "./plugin-editor-types";

export function KVPairEditor(props: {
  items: KVPairForm[];
  emptyLabel: string;
  keyLabel: string;
  valueLabel: string;
  keyOptions?: string[];
  valuePlaceholder?: string;
  onAdd: () => void;
  onRemove: (id: string) => void;
  onChange: (id: string, updater: (item: KVPairForm) => KVPairForm) => void;
}) {
  return (
    <div className="plugin-editor-kv-list">
      {props.items.length === 0 ? <div className="ruleset-debug-empty">{props.emptyLabel}</div> : null}
      {props.items.map((item) => {
        const selectableKeys = props.keyOptions
          ? props.keyOptions.filter((option) => option === item.key || !props.items.some((other) => other.id !== item.id && other.key === option))
          : [];
        return (
          <div key={item.id} className="plugin-editor-kv-row plugin-editor-kv-row-compact">
            <div className="plugin-editor-transform-actions plugin-editor-kv-actions">
              <Button className="plugin-editor-transform-action" aria-label="新增项" title="新增项" onClick={props.onAdd}>
                <Plus size={14} />
              </Button>
              <Button className="plugin-editor-transform-action" aria-label="删除项" title="删除项" onClick={() => props.onRemove(item.id)}>
                <span aria-hidden="true">×</span>
              </Button>
            </div>
            <label className="plugin-editor-field-inline plugin-editor-kv-inline-key">
              <span>{props.keyLabel}</span>
              {props.keyOptions ? (
                <select className="input" value={item.key} onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, key: event.target.value }))}>
                  <option value="">Select field</option>
                  {selectableKeys.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                  {item.key && !props.keyOptions.includes(item.key) ? <option value={item.key}>{item.key}</option> : null}
                </select>
              ) : (
                <input className="input" value={item.key} onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, key: event.target.value }))} />
              )}
            </label>
            <label className="plugin-editor-field-inline plugin-editor-kv-inline-value">
              <span>{props.valueLabel}</span>
              <input
                className="input"
                value={item.value}
                placeholder={props.valuePlaceholder}
                onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, value: event.target.value }))}
              />
            </label>
          </div>
        );
      })}
      {props.items.length === 0 ? (
        <div className="plugin-editor-inline-actions">
          <Button className="plugin-editor-transform-action" aria-label="新增项" title="新增项" onClick={props.onAdd}>
            <Plus size={14} />
          </Button>
        </div>
      ) : null}
    </div>
  );
}

export function WorkflowItemVariablesEditor(props: {
  items: KVPairForm[];
  emptyLabel?: string;
  keyLabel?: string;
  valueLabel?: string;
  valuePlaceholder?: string;
  onAdd: () => void;
  onRemove: (id: string) => void;
  onChange: (id: string, updater: (item: KVPairForm) => KVPairForm) => void;
}) {
  return (
    <div className="plugin-editor-kv-list">
      {props.items.length === 0 ? <div className="ruleset-debug-empty">{props.emptyLabel ?? "暂未定义 item_variables。"}</div> : null}
      {props.items.map((item) => (
        <div key={item.id} className="plugin-editor-kv-row plugin-editor-kv-row-compact">
          <label className="plugin-editor-field-inline plugin-editor-kv-inline-key">
            <span>{props.keyLabel ?? "Name"}</span>
            <input className="input" value={item.key} onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, key: event.target.value }))} />
          </label>
          <label className="plugin-editor-field-inline plugin-editor-kv-inline-value">
            <span>{props.valueLabel ?? "Template"}</span>
            <input
              className="input"
              value={item.value}
              placeholder={props.valuePlaceholder}
              onChange={(event) => props.onChange(item.id, (prev) => ({ ...prev, value: event.target.value }))}
            />
          </label>
          <div className="plugin-editor-transform-actions plugin-editor-kv-actions">
            <Button className="plugin-editor-transform-action" aria-label="新增变量" title="新增变量" onClick={props.onAdd}>
              <Plus size={14} />
            </Button>
            <Button className="plugin-editor-transform-action" aria-label="删除变量" title="删除变量" onClick={() => props.onRemove(item.id)}>
              <Trash2 size={14} />
            </Button>
          </div>
        </div>
      ))}
      {props.items.length === 0 ? (
        <div className="plugin-editor-inline-actions">
          <Button className="plugin-editor-transform-action" aria-label="新增变量" title="新增变量" onClick={props.onAdd}>
            <Plus size={14} />
          </Button>
        </div>
      ) : null}
    </div>
  );
}

// Re-export FIELD_OPTIONS for convenience
export { FIELD_OPTIONS };
