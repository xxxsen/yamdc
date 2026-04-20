import { Check, Edit3, X } from "lucide-react";

import type { JobItem } from "@/lib/api";

import { getNumberMeta } from "./helpers";

interface Props {
  job: JobItem;
}

// NumberStatusIcon: 影片 ID 清洗状态在表格中的圆形图标.
// 颜色 / icon 由 getNumberMeta(job).kind 决定, 四种状态:
//   success → 绿勾
//   manual  → 蓝色编辑笔 (用户手动改过)
//   warn    → 橙色 "!" (中等置信度)
//   danger  → 红色 X (清洗失败 / 低置信度)
// title 属性同步给出文字说明, 方便无障碍 / 鼠标悬停诊断.
export function NumberStatusIcon({ job }: Props) {
  const meta = getNumberMeta(job);
  const baseStyle = {
    width: 24,
    height: 24,
    minWidth: 24,
    borderRadius: 999,
    border: `1.5px solid ${meta.tone}`,
    color: meta.tone,
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "center",
    flexShrink: 0,
  } as const;

  if (meta.kind === "success") {
    return (
      <span style={baseStyle} title="清洗成功，高置信度">
        <Check size={14} strokeWidth={2.4} />
      </span>
    );
  }
  if (meta.kind === "manual") {
    return (
      <span style={baseStyle} title={meta.warning}>
        <Edit3 size={13} strokeWidth={2.2} />
      </span>
    );
  }
  if (meta.kind === "warn") {
    return (
      <span style={baseStyle} title="清洗成功，中等置信度">
        <span style={{ fontSize: 14, fontWeight: 700, lineHeight: 1 }}>!</span>
      </span>
    );
  }
  return (
    <span style={baseStyle} title={meta.warning || "清洗失败或低置信度"}>
      <X size={14} strokeWidth={2.4} />
    </span>
  );
}
