"use client";

// HeaderList: key-value 列表, 用于渲染 HTTP headers / form-urlencoded body.
// 之前在 output-panels.tsx 里, 同时被 BodyPanel / RequestDetailBlock /
// ResponseDetailBlock 三处引用, 拆单文件避免循环依赖 (body-panel ↔
// request-response 互相引用时只要都依赖这个叶子就 OK).

export function HeaderList({ headers }: { headers: Record<string, string> }) {
  const entries = Object.entries(headers);
  if (entries.length === 0) {
    return <div className="ruleset-debug-empty">Header 为空。</div>;
  }
  return (
    <div className="plugin-editor-header-list">
      {entries.map(([key, value]) => (
        <div key={key} className="plugin-editor-header-row">
          <span>{key}</span>
          <strong>{value || "-"}</strong>
        </div>
      ))}
    </div>
  );
}
