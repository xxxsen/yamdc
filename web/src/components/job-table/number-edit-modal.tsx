"use client";

import { useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";
import type { JobItem, NumberVariantDescriptor, NumberVariantSelection } from "@/lib/api";

// NumberEditModal: "文件列表" 页结构化影片 ID 编辑器。
//
// 交互模型:
//   - base: 影片 ID 主体, 用户只在这个输入框里维护影片 ID 本身 (例如 "PXVR-406")。
//   - variants: 每个 flag descriptor 渲染成 chip 按钮, indexed descriptor
//     额外带一个小数字输入框 (例如 CD 选 1/2/3...)。
//   - preview: 下方实时预览拼装后的完整 number, 让用户在 "这条 patch 会被
//     后端写成什么字符串" 这件事上没有认知差。
//
// 设计取舍:
//   - 保留自由文本 base + 结构化 variants 的组合, 而不是把 base 也做成
//     selector: 影片 ID 的发行规则千奇百怪, 再怎么枚举都会漏, 所以 base
//     保持自由输入; 反倒是 variants 形态相对固定, 放 selector 正好。
//   - 预解析 job.number 的工作只尝试 "自后向前剥匹配到的 descriptor suffix",
//     和后端 number.Parse 的行为一致。解析失败 (例如带 '.'、纯中文等奇怪
//     字符) 就把整段扔回 base, 让用户自己决定要不要 / 怎么改。
export interface NumberEditModalProps {
  job: JobItem;
  descriptors: NumberVariantDescriptor[];
  isSubmitting: boolean;
  descriptorsError?: string;
  onClose: () => void;
  onSubmit: (base: string, selections: NumberVariantSelection[]) => void;
}

interface ParsedNumber {
  base: string;
  selections: Map<string, number | true>;
}

interface DescriptorIndex {
  flagByUpperSuffix: Map<string, NumberVariantDescriptor>;
  indexed: NumberVariantDescriptor | undefined;
}

function indexDescriptors(descriptors: NumberVariantDescriptor[]): DescriptorIndex {
  const flagByUpperSuffix = new Map<string, NumberVariantDescriptor>();
  let indexed: NumberVariantDescriptor | undefined;
  for (const d of descriptors) {
    if (d.kind === "flag") {
      flagByUpperSuffix.set(d.suffix.toUpperCase(), d);
    } else if (!indexed) {
      indexed = d;
    }
  }
  return { flagByUpperSuffix, indexed };
}

// tryMatchIndexedSuffix: 如果 tail 以 indexed descriptor 的 suffix 开头
// 且后跟若干位数字, 返回解析到的 index; 否则返回 undefined。单独抽出来
// 是为了降低 parseExistingNumber 的 cyclomatic complexity。
function tryMatchIndexedSuffix(
  tail: string,
  descriptor: NumberVariantDescriptor | undefined,
): number | undefined {
  if (!descriptor) return undefined;
  const suffixUpper = descriptor.suffix.toUpperCase();
  if (!tail.startsWith(suffixUpper)) return undefined;
  const digits = tail.slice(suffixUpper.length);
  if (digits.length === 0 || !/^\d+$/.test(digits)) return undefined;
  return Number.parseInt(digits, 10);
}

// parseExistingNumber 把已有的影片 ID (如 "PXVR-406-4K-CD2") 拆成 base + 每个
// descriptor 命中的 selection。只匹配传入的 descriptors, 不认识的 suffix 会
// 被当成 base 的一部分留下 (这比丢失信息更保守)。匹配时不区分大小写, 因为
// 后端 Parse 会 ToUpper 然后再比对; 我们也按大写标准化后比较。
function parseExistingNumber(raw: string, descriptors: NumberVariantDescriptor[]): ParsedNumber {
  const selections = new Map<string, number | true>();
  let remaining = raw.toUpperCase();
  const { flagByUpperSuffix, indexed } = indexDescriptors(descriptors);

  // 从末尾尝试 "- / _" 后的 token, 能 match 就剥掉继续, 否则停。
  // 这和 number.resolveSuffixInfo 的循环逻辑一致, 保证前端预填和后端拼装
  // 互为逆操作。
  for (;;) {
    const idx = Math.max(remaining.lastIndexOf("-"), remaining.lastIndexOf("_"));
    if (idx <= 0) break;
    const tail = remaining.slice(idx + 1);
    if (tail.length === 0) break;

    const indexedValue = tryMatchIndexedSuffix(tail, indexed);
    if (indexed && indexedValue !== undefined && !selections.has(indexed.id)) {
      selections.set(indexed.id, indexedValue);
      remaining = remaining.slice(0, idx);
      continue;
    }

    const flag = flagByUpperSuffix.get(tail);
    if (flag && !selections.has(flag.id)) {
      selections.set(flag.id, true);
      remaining = remaining.slice(0, idx);
      continue;
    }
    break;
  }

  return { base: remaining, selections };
}

// composePreview 本地拼一次预览, 和后端 ApplyVariantSelections 的顺序保持
// 一致 (按 descriptors 顺序遍历)。纯展示, 不做任何校验; 真正落盘还是由
// 后端来做。
function composePreview(
  base: string,
  selections: Map<string, number | true>,
  descriptors: NumberVariantDescriptor[],
): string {
  const trimmed = base.trim().toUpperCase();
  if (!trimmed) {
    return "";
  }
  let out = trimmed;
  for (const d of descriptors) {
    const value = selections.get(d.id);
    if (value === undefined) continue;
    if (d.kind === "flag") {
      out += `-${d.suffix}`;
    } else if (typeof value === "number") {
      out += `-${d.suffix}${String(value)}`;
    }
  }
  return out;
}

function isIndexValid(descriptor: NumberVariantDescriptor, index: number | true | undefined): boolean {
  if (descriptor.kind !== "indexed") {
    return true;
  }
  if (typeof index !== "number") {
    return false;
  }
  const min = descriptor.min ?? 1;
  const max = descriptor.max ?? Number.MAX_SAFE_INTEGER;
  return Number.isInteger(index) && index >= min && index <= max;
}

export function NumberEditModal({
  job,
  descriptors,
  isSubmitting,
  descriptorsError,
  onClose,
  onSubmit,
}: NumberEditModalProps) {
  const initial = useMemo(() => parseExistingNumber(job.number, descriptors), [job.number, descriptors]);
  const [base, setBase] = useState<string>(initial.base);
  const [selections, setSelections] = useState<Map<string, number | true>>(initial.selections);

  useEffect(() => {
    setBase(initial.base);
    setSelections(initial.selections);
  }, [initial]);

  const toggleFlag = (d: NumberVariantDescriptor) => {
    setSelections((prev) => {
      const next = new Map(prev);
      if (next.has(d.id)) {
        next.delete(d.id);
      } else {
        next.set(d.id, true);
      }
      return next;
    });
  };

  const setIndexedEnabled = (d: NumberVariantDescriptor, enabled: boolean) => {
    setSelections((prev) => {
      const next = new Map(prev);
      if (!enabled) {
        next.delete(d.id);
      } else if (!next.has(d.id)) {
        next.set(d.id, d.min ?? 1);
      }
      return next;
    });
  };

  const setIndexedValue = (d: NumberVariantDescriptor, value: number) => {
    setSelections((prev) => {
      const next = new Map(prev);
      next.set(d.id, value);
      return next;
    });
  };

  const preview = composePreview(base, selections, descriptors);
  const baseTrimmed = base.trim();

  const invalidIndexedIds: string[] = [];
  for (const d of descriptors) {
    if (d.kind !== "indexed") continue;
    if (selections.has(d.id) && !isIndexValid(d, selections.get(d.id))) {
      invalidIndexedIds.push(d.id);
    }
  }
  const canSubmit = baseTrimmed.length > 0 && invalidIndexedIds.length === 0 && !isSubmitting;

  const handleSubmit = () => {
    if (!canSubmit) {
      return;
    }
    const out: NumberVariantSelection[] = [];
    for (const d of descriptors) {
      const value = selections.get(d.id);
      if (value === undefined) continue;
      if (d.kind === "flag") {
        out.push({ id: d.id });
      } else if (typeof value === "number") {
        out.push({ id: d.id, index: value });
      }
    }
    onSubmit(baseTrimmed, out);
  };

  return (
    <Modal
      open
      onClose={onClose}
      title="编辑影片 ID"
      subtitle={`当前值: ${job.number || "(空)"}`}
      ariaLabel="编辑影片 ID"
      actions={
        <>
          <Button onClick={onClose} disabled={isSubmitting}>
            取消
          </Button>
          <Button variant="primary" onClick={handleSubmit} disabled={!canSubmit} loading={isSubmitting}>
            保存
          </Button>
        </>
      }
    >
      <div className="number-edit-modal">
        <label className="number-edit-field">
          <span className="number-edit-field-label">基础影片 ID</span>
          <input
            className="input"
            autoFocus
            value={base}
            placeholder="例如 PXVR-406"
            onChange={(e) => setBase(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSubmit();
              }
            }}
          />
          <span className="number-edit-field-hint">
            只填主影片 ID, 变体 (CD / 4K / 中字 ...) 请用下方的按钮勾选。
          </span>
        </label>

        {descriptorsError ? (
          <div className="number-edit-error" role="alert">
            加载 variant 列表失败: {descriptorsError}
          </div>
        ) : null}

        <div className="number-edit-variants">
          <div className="number-edit-variants-label">变体</div>
          <div className="number-edit-variants-grid">
            {descriptors.map((d) => {
              const active = selections.has(d.id);
              if (d.kind === "flag") {
                return (
                  <button
                    key={d.id}
                    type="button"
                    className="number-edit-variant-chip"
                    data-active={active}
                    onClick={() => toggleFlag(d)}
                    title={d.description}
                  >
                    <span className="number-edit-variant-label">{d.label}</span>
                    <span className="number-edit-variant-suffix">-{d.suffix}</span>
                  </button>
                );
              }
              const indexValue = typeof selections.get(d.id) === "number" ? (selections.get(d.id) as number) : (d.min ?? 1);
              const invalid = active && !isIndexValid(d, selections.get(d.id));
              return (
                <div
                  key={d.id}
                  className="number-edit-variant-chip number-edit-variant-chip-indexed"
                  data-active={active}
                  data-invalid={invalid}
                >
                  <label className="number-edit-variant-toggle">
                    <input
                      type="checkbox"
                      checked={active}
                      onChange={(e) => setIndexedEnabled(d, e.target.checked)}
                    />
                    <span className="number-edit-variant-label">{d.label}</span>
                  </label>
                  <input
                    className="input number-edit-variant-index-input"
                    type="number"
                    min={d.min ?? 1}
                    max={d.max ?? undefined}
                    step={1}
                    disabled={!active}
                    value={indexValue}
                    aria-label={`${d.label} 序号`}
                    title={d.description}
                    onChange={(e) => {
                      const raw = Number.parseInt(e.target.value, 10);
                      if (!Number.isNaN(raw)) {
                        setIndexedValue(d, raw);
                      }
                    }}
                  />
                </div>
              );
            })}
          </div>
          {descriptors.length === 0 && !descriptorsError ? (
            <div className="number-edit-variants-empty">暂无可用变体</div>
          ) : null}
        </div>

        <div className="number-edit-preview">
          <span className="number-edit-preview-label">预览</span>
          <span className="number-edit-preview-value">{preview || "(请填写基础影片 ID)"}</span>
        </div>
      </div>
    </Modal>
  );
}
