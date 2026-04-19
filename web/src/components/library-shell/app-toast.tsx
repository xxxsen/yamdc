// AppToast: library-shell 顶部错误/提示浮条。很薄 — 只做: 空 message
// 不渲染 / 根据 message 自动判断 tone (失败 / error → danger)。
//
// 不放到 components/ui/ 下, 因为这个组件的 tone 推断是 library-shell
// 语义耦合的快捷决策 (按中文文案匹配), 还不到通用 Toast 的抽象级别。
// 若后续 review-shell 也想复用, 可以再上升。
//
// 详见 td/022-frontend-optimization-roadmap.md §2.2 B-2。

export function AppToast({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div
      className="app-toast app-toast-top"
      data-tone={/失败|error/i.test(message) ? "danger" : undefined}
      role="status"
      aria-live="polite"
    >
      {message}
    </div>
  );
}
