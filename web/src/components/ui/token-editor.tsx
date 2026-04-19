"use client";

import { useState } from "react";
import { X } from "lucide-react";

// TokenEditor: 项目公共 token 编辑原子。原本在 library-shell / review-shell /
// media-library-detail-shell 三处各有一份几乎重复的实现, 本组件把三份收拢:
//
//   - library-shell / review-shell 场景 (可编辑 + 提交副作用):
//       <TokenEditor idPrefix="library-token" onCommit={flush} ... />
//       编辑结束 (回车 / 失焦 / 移除 token) 后会调用 onCommit, 用于触发父组件
//       的持久化逻辑。如果不需要副作用 (纯受控表单) 可不传 onCommit.
//
//   - media-library-detail-shell 场景 (只读):
//       <TokenEditor idPrefix="..." readOnly value={...} onChange={noop} />
//       readOnly 下不渲染 input 和 remove, value 为空时显示 placeholder 文字,
//       保持与旧实现完全一致 (用 .library-inline-muted 弱化)。
//
// 设计注记:
//   - `idPrefix` 必填, 避免三个实例同名冲突 (旧版各 shell 的 id 前缀分别是
//     library-token / token / media-library-token), 合并后显式传递以保持
//     DOM id 不变。
//   - 保留 DOM 外观 (.review-field / .token-editor / .token-chip ...) —
//     §2.1 Tailwind 迁移暂未触达 token 相关 class, 这里先不改 class 名, 避免
//     被全局 .review-field 规则静默影响。后续迁 Tailwind 时 token 相关整体
//     再走 @layer components 或换 utility。
//   - `singleLine` 改 token-editor 容器的布局 (单行不换行). readOnly 与
//     singleLine 可以组合。
//
// 详见 td/022-frontend-optimization-roadmap.md §2.2 B-1。

export interface TokenEditorProps {
  label: string;
  placeholder: string;
  value: string[];
  onChange: (next: string[]) => void;
  onCommit?: () => void;
  idPrefix: string;
  singleLine?: boolean;
  readOnly?: boolean;
}

export function TokenEditor({
  label,
  placeholder,
  value,
  onChange,
  onCommit,
  idPrefix,
  singleLine = false,
  readOnly = false,
}: TokenEditorProps) {
  const [draft, setDraft] = useState("");
  const inputId = `${idPrefix}-${label}`;

  const commitDraft = () => {
    const next = draft.trim();
    if (!next) {
      setDraft("");
      return;
    }
    onChange([...value, next]);
    setDraft("");
  };

  const removeAt = (idx: number) => {
    onChange(value.filter((_, index) => index !== idx));
    onCommit?.();
  };

  const containerClass = `token-editor${singleLine ? " token-editor-single-line" : ""}${readOnly ? " token-editor-readonly" : ""}`;

  return (
    <div className="review-field review-field-tokens">
      <span className="review-label review-label-side">{label}</span>
      <div
        className={containerClass}
        onClick={() => {
          if (readOnly) return;
          document.getElementById(inputId)?.focus();
        }}
      >
        {value.map((item, idx) => (
          <span key={`${item}-${idx}`} className="token-chip">
            {item}
            {!readOnly ? (
              <button
                type="button"
                className="token-chip-remove"
                aria-label={`删除${item}`}
                onClick={() => removeAt(idx)}
              >
                <X size={11} />
              </button>
            ) : null}
          </span>
        ))}
        {!readOnly ? (
          <input
            id={inputId}
            className="token-input"
            placeholder={value.length === 0 ? placeholder : ""}
            value={draft}
            onChange={(e) => {
              const next = e.target.value;
              if (next.includes(",")) {
                const parts = next.split(",");
                const ready = parts
                  .slice(0, -1)
                  .map((item) => item.trim())
                  .filter(Boolean);
                if (ready.length > 0) {
                  onChange([...value, ...ready]);
                }
                setDraft(parts[parts.length - 1] ?? "");
                return;
              }
              setDraft(next);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                commitDraft();
                onCommit?.();
              } else if (e.key === "Backspace" && draft === "" && value.length > 0) {
                onChange(value.slice(0, -1));
              }
            }}
            onBlur={() => {
              commitDraft();
              onCommit?.();
            }}
          />
        ) : value.length === 0 ? (
          <span className="library-inline-muted">{placeholder}</span>
        ) : null}
      </div>
    </div>
  );
}
