// @vitest-environment jsdom

import * as React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";

// 告诉 React 当前运行在 "测试环境" 下, 允许 act(...) 工作 —
// 不设置这个 flag React 会打一行 warning: "The current testing
// environment is not configured to support act(...)"。
// 文件被 test-helpers import 时统一生效, 各 test 文件不需要重复设置。
declare global {
  var IS_REACT_ACT_ENVIRONMENT: boolean | undefined;
}
globalThis.IS_REACT_ACT_ENVIRONMENT = true;

// 共享 mount helper: 不引入 @testing-library/react, 手写最小封装。
// 每个测试自己创建容器节点、挂载、操作完 unmount — 避免全局状态污染。
export function mount(element: React.ReactElement): {
  container: HTMLDivElement;
  root: Root;
  unmount: () => void;
} {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => {
    root.render(element);
  });
  return {
    container,
    root,
    unmount: () => {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
  };
}

export function rerender(root: Root, element: React.ReactElement) {
  act(() => {
    root.render(element);
  });
}
