"use client";

import { Plus } from "lucide-react";

import { Button } from "@/components/ui/button";

import { FieldCard } from "../field-card";
import type { EditorState, FieldForm, TransformForm } from "../plugin-editor-types";

// ScrapeSection: 左侧表单 "Fields" 选项卡. 内容就是一串 FieldCard + 底部
// "新增字段" 按钮. canAddField 由外层算 (依赖 FIELD_OPTIONS 上限), 通过
// prop 传入避免本模块再依赖常量.

export interface ScrapeSectionProps {
  state: EditorState;
  canAddField: boolean;
  onPatchField: (id: string, updater: (field: FieldForm) => FieldForm) => void;
  onUpdateFieldName: (id: string, nextName: string) => void;
  onRemoveField: (id: string) => void;
  onAddField: () => void;
  onAddTransform: (fieldID: string, afterTransformID?: string) => void;
  onRemoveTransform: (fieldID: string, transformID: string) => void;
  onPatchTransform: (fieldID: string, transformID: string, updater: (t: TransformForm) => TransformForm) => void;
}

export function ScrapeSection({
  state,
  canAddField,
  onPatchField,
  onUpdateFieldName,
  onRemoveField,
  onAddField,
  onAddTransform,
  onRemoveTransform,
  onPatchTransform,
}: ScrapeSectionProps) {
  return (
    <article id="plugin-editor-section-scrape" className="plugin-editor-panel-fragment">
      <div className="plugin-editor-fields">
        {state.fields.map((field) => (
          <FieldCard
            key={field.id}
            field={field}
            allFields={state.fields}
            onPatchField={(updater) => onPatchField(field.id, updater)}
            onUpdateName={(nextName) => onUpdateFieldName(field.id, nextName)}
            onRemove={() => onRemoveField(field.id)}
            onAddTransform={(afterID) => onAddTransform(field.id, afterID)}
            onRemoveTransform={(transformID) => onRemoveTransform(field.id, transformID)}
            onPatchTransform={(transformID, updater) => onPatchTransform(field.id, transformID, updater)}
          />
        ))}
      </div>
      <div className="plugin-editor-inline-actions">
        <Button
          className="plugin-editor-transform-action"
          aria-label="新增字段"
          title="新增字段"
          onClick={onAddField}
          disabled={!canAddField}
        >
          <Plus size={14} />
        </Button>
      </div>
    </article>
  );
}
