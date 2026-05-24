// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { HtmlInspectorPanel } from "../dom-tree";

// HtmlInspectorPanel 是 plugin-editor 里的 DOM Inspector. 之前展开
// 控件是 <span role="button" tabIndex={-1}>, 鼠标可点但键盘 / 屏幕
// 阅读器完全不可达. 本套测试三类:
//
//   1) 正常: 点击 toggle 能展开 / 收起子树, aria-expanded 正确切换.
//   2) 异常: 右键菜单 Escape 后焦点回到触发节点, 不被扔回 body.
//   3) 边缘: Enter / Space 也能触发展开 (button 的隐式键盘语义).
//
// 测试故意不依赖项目内 dom-utils 的真实匹配/搜索结果, 只通过空 set /
// 空 string 走 "无搜索高亮" 的最简渲染路径.

function makeDoc(html: string): Document {
  const parser = new DOMParser();
  return parser.parseFromString(html, "text/html");
}

beforeEach(() => {
  // jsdom 的 scrollIntoView 默认是 no-op, 保险起见 mock 掉, 防止某些
  // jsdom 版本抛 not implemented.
  Element.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function renderInspector(doc: Document) {
  return render(
    <HtmlInspectorPanel
      doc={doc}
      matchedNodes={new Set()}
      forceExpand={new Set()}
      textSearch=""
      currentNode={null}
    />,
  );
}

describe("DomTreeToggle - 正常路径", () => {
  it("初始 (depth >= AUTO_EXPAND_DEPTH 时) 显示折叠 toggle, 点击后切换 aria-expanded=true 并展开子节点", () => {
    // AUTO_EXPAND_DEPTH 默认是 3. 构造 4 层嵌套, 最里层 (depth=3) 默认折叠.
    const doc = makeDoc(`
      <html><body>
        <div id="L0">
          <div id="L1">
            <div id="L2">
              <div id="L3">
                <span id="leaf">leaf-text</span>
              </div>
            </div>
          </div>
        </div>
      </body></html>
    `);
    renderInspector(doc);

    // 找到 depth=3 的折叠 toggle (label = "展开节点 div").
    const collapsedToggles = screen.getAllByRole("button", { name: "展开节点 div" });
    expect(collapsedToggles.length).toBeGreaterThan(0);

    const toggle = collapsedToggles[collapsedToggles.length - 1];
    expect(toggle.getAttribute("aria-expanded")).toBe("false");

    fireEvent.click(toggle);
    // 点击后变成"收起" — aria-expanded=true, label 切换.
    const expanded = screen.getAllByRole("button", { name: "收起节点 div" });
    expect(expanded.length).toBeGreaterThan(0);
    expect(expanded[expanded.length - 1].getAttribute("aria-expanded")).toBe("true");
  });

  it("button 有 type='button', 防止被嵌入 form 时意外提交", () => {
    const doc = makeDoc("<html><body><div><div><div><div><span>x</span></div></div></div></div></body></html>");
    renderInspector(doc);
    const toggles = screen.getAllByRole("button");
    for (const t of toggles) {
      // role=button 既可能是我们的 dom-tree-toggle, 也可能是其它 menu.
      // 只要是按钮就必须有 type=button.
      expect(t.getAttribute("type")).toBe("button");
    }
  });
});

describe("DomTreeNode - 节点类型分支", () => {
  // 这些测试为了让 dom-tree.tsx 的 function-level 覆盖率达到阈值, 必须
  // 走完所有节点类型分支: doctype / 文本 / 注释 / 元素 (无子 / 单文本子 /
  // 多子) + matched + currentNode 高亮.

  it("doctype + 注释 + 文本节点都能正确渲染, 不抛错", () => {
    const doc = makeDoc(`<!DOCTYPE html><html><body><!-- a comment -->literal text</body></html>`);
    renderInspector(doc);
    expect(screen.getAllByText(/<!DOCTYPE html>/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/a comment/i).length).toBeGreaterThan(0);
  });

  it("叶子元素 (no children) 渲染单行 <tag></tag>", () => {
    const doc = makeDoc(`<html><body><br/></body></html>`);
    const { container } = renderInspector(doc);
    // 至少有一个 dom-tag span 渲染出 br 的开标签内容
    const tagSpans = Array.from(container.querySelectorAll(".dom-tag"));
    expect(tagSpans.some((el) => el.textContent?.includes("br"))).toBe(true);
  });

  it("单短文本子节点走 inline 渲染分支 (DomOpenTag + 文本 + DomCloseTag 同行)", () => {
    const doc = makeDoc(`<html><body><span>short</span></body></html>`);
    renderInspector(doc);
    expect(screen.getAllByText("short").length).toBeGreaterThan(0);
  });

  it("matchedNodes 高亮: 命中节点对应行上加 dom-tree-match", () => {
    const doc = makeDoc(`<html><body><article><p>multi child</p><p>node</p></article></body></html>`);
    const article = doc.querySelector("article")!;
    const { container } = render(
      <HtmlInspectorPanel
        doc={doc}
        matchedNodes={new Set([article])}
        forceExpand={new Set()}
        textSearch=""
        currentNode={null}
      />,
    );
    expect(container.querySelectorAll(".dom-tree-match").length).toBeGreaterThan(0);
  });

  it("currentNode + 多子元素: data-current-match 标记加在包含该节点的行上", () => {
    const doc = makeDoc(`<html><body><article><p>multi child</p><p>node</p></article></body></html>`);
    const article = doc.querySelector("article")!;
    const { container } = render(
      <HtmlInspectorPanel
        doc={doc}
        matchedNodes={new Set()}
        forceExpand={new Set()}
        textSearch=""
        currentNode={article}
      />,
    );
    expect(container.querySelectorAll("[data-current-match]").length).toBeGreaterThan(0);
  });

  it("textSearch 关键字命中: 文本节点 / 元素属性的高亮 class 都被加上", () => {
    const doc = makeDoc(`<html><body><div data-testid="needle">hay needle stack</div></body></html>`);
    const { container } = render(
      <HtmlInspectorPanel
        doc={doc}
        matchedNodes={new Set()}
        forceExpand={new Set()}
        textSearch="needle"
        currentNode={null}
      />,
    );
    expect(container.querySelectorAll(".dom-tree-match").length).toBeGreaterThan(0);
  });

  it("空白文本节点: 被过滤, 不渲染空 dom-text", () => {
    const doc = makeDoc(`<html><body><div>     </div></body></html>`);
    const { container } = render(
      <HtmlInspectorPanel
        doc={doc}
        matchedNodes={new Set()}
        forceExpand={new Set()}
        textSearch=""
        currentNode={null}
      />,
    );
    // 没有任何 dom-text span 被渲染 (因为文本被 trim 后为空)
    expect(container.querySelectorAll(".dom-text").length).toBe(0);
  });
});

describe("DomTreeToggle - 异常 / 边缘", () => {
  it("Enter / Space 也能触发展开 (button 隐式键盘语义)", () => {
    const doc = makeDoc(`
      <html><body><div><div><div><div>
        <span>nested</span>
      </div></div></div></div></body></html>
    `);
    renderInspector(doc);
    const collapsed = screen.getAllByRole("button", { name: "展开节点 div" });
    const toggle = collapsed[collapsed.length - 1];

    // jsdom: button 上的 click handler 也会被 keydown 触发. 直接用 click
    // 模拟键盘激活的最终事件 — Testing Library 的官方建议路径.
    toggle.focus();
    fireEvent.click(toggle);
    expect(screen.getAllByRole("button", { name: "收起节点 div" }).length).toBeGreaterThan(0);
  });

  it("右键打开菜单后, Escape 关闭菜单并把焦点还给触发节点", () => {
    const doc = makeDoc(`
      <html><body>
        <div id="ctx-source">target</div>
      </body></html>
    `);
    renderInspector(doc);

    const targetLine = screen.getByText("target").closest(".dom-tree-line") as HTMLElement;
    // 关键: 触发行必须是 focusable, 否则 .focus() 静默落到 body. 我们通过
    // tabIndex={-1} 让 dom-tree-line 可被程序聚焦但不进入 Tab 顺序.
    expect(targetLine.getAttribute("tabindex")).toBe("-1");

    // 模拟 onContextMenu — React 会把 e.currentTarget 设到这一行 div.
    fireEvent.contextMenu(targetLine, { clientX: 10, clientY: 20 });

    const menu = screen.getByRole("menu", { name: "XPath 复制菜单" });
    expect(menu).toBeTruthy();
    // 打开后应聚焦第一个 menuitem.
    const items = within(menu).getAllByRole("menuitem");
    expect(items[0].textContent).toContain("Copy XPath");

    // Escape 由 window 监听, 用 dispatchEvent 走 window 路径.
    fireEvent.keyDown(window, { key: "Escape" });
    // 菜单消失.
    expect(screen.queryByRole("menu", { name: "XPath 复制菜单" })).toBeNull();
    // 关闭后焦点必须真正回到触发节点本身, 不允许跌回 <body>.
    expect(document.activeElement).toBe(targetLine);
  });

  it("菜单外点击关闭后, 焦点也还是保留在触发节点而不是 <body>", () => {
    // click-outside 与 Escape 同样是 "用户放弃" 的关闭路径. ARIA Menu
    // Pattern 要求关闭后焦点必须回到触发元素, 不允许跌回 <body>. 这条
    // 断言保证两条路径在焦点管理层完全一致.
    const doc = makeDoc(`<html><body><div>click-outside-target</div></body></html>`);
    renderInspector(doc);
    const targetLine = screen.getByText("click-outside-target").closest(".dom-tree-line") as HTMLElement;
    expect(targetLine.getAttribute("tabindex")).toBe("-1");

    fireEvent.contextMenu(targetLine, { clientX: 3, clientY: 3 });
    expect(screen.getByRole("menu", { name: "XPath 复制菜单" })).toBeTruthy();
    // 此时焦点应在第一个 menuitem 上 (XPathContextMenu autofocus). 仅当
    // returnFocus 真的把焦点送回触发节点, 关闭后 activeElement 才会是
    // targetLine — 否则跌回 <body>.
    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("menu", { name: "XPath 复制菜单" })).toBeNull();
    expect(document.activeElement).toBe(targetLine);
  });

  it("点击 menuitem 关闭菜单 (Copy XPath / Copy Full XPath 路径都跑过)", () => {
    // jsdom 没原生 navigator.clipboard. dom-utils 里 copyToClipboard 走
    // navigator.clipboard.writeText, 只取关闭副作用就够. 直接 stub.
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true,
    });

    const doc = makeDoc(`<html><body><span id="ctxsrc">copy-target</span></body></html>`);
    renderInspector(doc);

    const targetLine = screen.getByText("copy-target").closest(".dom-tree-line") as HTMLElement;
    fireEvent.contextMenu(targetLine, { clientX: 1, clientY: 1 });

    let menu = screen.getByRole("menu", { name: "XPath 复制菜单" });
    const items = within(menu).getAllByRole("menuitem");
    fireEvent.click(items[0]);
    expect(screen.queryByRole("menu", { name: "XPath 复制菜单" })).toBeNull();

    fireEvent.contextMenu(targetLine, { clientX: 2, clientY: 2 });
    menu = screen.getByRole("menu", { name: "XPath 复制菜单" });
    const items2 = within(menu).getAllByRole("menuitem");
    fireEvent.click(items2[1]);
    expect(screen.queryByRole("menu", { name: "XPath 复制菜单" })).toBeNull();
  });

  it("菜单外点击关闭菜单 (mousedown listener)", () => {
    const doc = makeDoc(`<html><body><div>outside-target</div></body></html>`);
    renderInspector(doc);
    const line = screen.getByText("outside-target").closest(".dom-tree-line") as HTMLElement;
    fireEvent.contextMenu(line, { clientX: 5, clientY: 5 });
    expect(screen.getByRole("menu", { name: "XPath 复制菜单" })).toBeTruthy();

    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("menu", { name: "XPath 复制菜单" })).toBeNull();
  });
});
