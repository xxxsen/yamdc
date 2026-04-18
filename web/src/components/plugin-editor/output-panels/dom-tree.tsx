"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import {
  ATTR_VALUE_MAX,
  AUTO_EXPAND_DEPTH,
  INLINE_TEXT_MAX,
  computeFullXPath,
  computeSmartXPath,
  copyToClipboard,
  elementMatchesText,
  visibleChildNodes,
} from "./dom-utils";
import type { XPathMenuState } from "../plugin-editor-types";

// dom-tree: plugin-editor body 面板 "Inspector" 模式下的 DOM 树渲染 +
// 右键 "Copy XPath" 菜单. 纯展示 + 本地交互, 不持有业务状态.
//
// 模块内部组件层次:
//   HtmlInspectorPanel                  // 入口, 持有菜单态
//     └── DomTreeNode (recursive)       // 每个 element/text/comment 一行
//           ├── DomOpenTag              // <tag attr="..">
//           │     └── DomAttrSpan       // 单个属性
//           └── DomCloseTag             // </tag>
//     └── XPathContextMenu (portal)     // 右键浮层
//
// matchedNodes / forceExpand / textSearch / currentNode 都是 useBodySearch
// 的派生态, 这里只消费不生产.

function XPathContextMenu({ menu, onClose }: {
  menu: XPathMenuState;
  onClose: () => void;
}) {
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menu) return;
    function handleKey(e: KeyboardEvent) { if (e.key === "Escape") onClose(); }
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) onClose();
    }
    window.addEventListener("keydown", handleKey);
    window.addEventListener("mousedown", handleClick);
    return () => { window.removeEventListener("keydown", handleKey); window.removeEventListener("mousedown", handleClick); };
  }, [menu, onClose]);

  if (!menu) return null;

  function handleCopy(text: string) {
    copyToClipboard(text);
    onClose();
  }

  return createPortal(
    <div ref={menuRef} className="xpath-context-menu" style={{ left: menu.x, top: menu.y }}>
      <button type="button" className="xpath-context-menu-item" onClick={() => handleCopy(menu.xpath)}>
        Copy XPath
      </button>
      <button type="button" className="xpath-context-menu-item" onClick={() => handleCopy(menu.fullXpath)}>
        Copy Full XPath
      </button>
    </div>,
    document.body,
  );
}

function DomAttrSpan({ name, value }: { name: string; value: string }) {
  const display = value.length > ATTR_VALUE_MAX ? value.slice(0, ATTR_VALUE_MAX - 3) + "…" : value;
  return (
    <>
      {" "}
      <span className="dom-attr-name">{name}</span>
      <span className="dom-tag">=</span>
      <span className="dom-attr-value">&quot;{display}&quot;</span>
    </>
  );
}

function DomOpenTag({ el }: { el: Element }) {
  const tag = el.tagName.toLowerCase();
  const attrs = Array.from(el.attributes);
  return (
    <>
      <span className="dom-tag">&lt;{tag}</span>
      {attrs.map((a) => <DomAttrSpan key={a.name} name={a.name} value={a.value} />)}
      <span className="dom-tag">&gt;</span>
    </>
  );
}

function DomCloseTag({ tag }: { tag: string }) {
  return <span className="dom-tag">&lt;/{tag}&gt;</span>;
}

function DomTreeNode({ node, depth, onCtxMenu, matchedNodes, forceExpand, textSearch, currentNode }: {
  node: ChildNode;
  depth: number;
  onCtxMenu: (e: React.MouseEvent, el: Element) => void;
  matchedNodes: ReadonlySet<Node>;
  forceExpand: ReadonlySet<Node>;
  textSearch: string;
  currentNode: Node | null;
}) {
  const [userExpanded, setUserExpanded] = useState<boolean | null>(null);
  const autoExpand = depth < AUTO_EXPAND_DEPTH || forceExpand.has(node);
  const expanded = userExpanded ?? autoExpand;
  const indent = { paddingLeft: depth * 16 };
  const isCurrent = node === currentNode;
  const currentAttr = isCurrent ? { "data-current-match": "" } : {};

  if (node.nodeType === 10) {
    return (
      <div className="dom-tree-line" style={indent}>
        <span className="dom-tree-spacer" />
        <span className="dom-comment">&lt;!DOCTYPE {(node as DocumentType).name}&gt;</span>
      </div>
    );
  }

  if (node.nodeType === 3) {
    const text = node.textContent?.trim();
    if (!text) return null;
    const hit = textSearch ? text.toLowerCase().includes(textSearch.toLowerCase()) : false;
    const cls = `dom-tree-line${hit ? " dom-tree-match" : ""}${isCurrent ? " dom-tree-match-current" : ""}`;
    return (
      <div className={cls} style={indent} {...currentAttr}>
        <span className="dom-tree-spacer" />
        <span className="dom-text">&quot;{text}&quot;</span>
      </div>
    );
  }

  if (node.nodeType === 8) {
    return (
      <div className="dom-tree-line" style={indent}>
        <span className="dom-tree-spacer" />
        <span className="dom-comment">&lt;!-- {node.textContent} --&gt;</span>
      </div>
    );
  }

  if (node.nodeType !== 1) return null;

  const el = node as Element;
  const tag = el.tagName.toLowerCase();
  const children = visibleChildNodes(el);
  const hasChildren = children.length > 0;
  const isSingleShortText = children.length === 1
    && children[0].nodeType === 3
    && (children[0].textContent?.trim().length ?? 0) <= INLINE_TEXT_MAX;

  const isMatch = matchedNodes.has(node) || (textSearch ? elementMatchesText(el, textSearch) : false);
  const lineClass = `dom-tree-line${isMatch ? " dom-tree-match" : ""}${isCurrent ? " dom-tree-match-current" : ""}`;

  function handleCtx(e: React.MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    onCtxMenu(e, el);
  }

  if (!hasChildren) {
    return (
      <div className={lineClass} style={indent} onContextMenu={handleCtx} {...currentAttr}>
        <span className="dom-tree-spacer" />
        <DomOpenTag el={el} /><DomCloseTag tag={tag} />
      </div>
    );
  }

  if (isSingleShortText) {
    const childHit = textSearch ? (children[0].textContent?.trim() ?? "").toLowerCase().includes(textSearch.toLowerCase()) : false;
    const childIsCurrent = children[0] === currentNode;
    const inlineMatch = isMatch || childHit;
    const inlineCurrent = isCurrent || childIsCurrent;
    const inlineClass = `dom-tree-line${inlineMatch ? " dom-tree-match" : ""}${inlineCurrent ? " dom-tree-match-current" : ""}`;
    const inlineAttr = inlineCurrent ? { "data-current-match": "" } : {};
    return (
      <div className={inlineClass} style={indent} onContextMenu={handleCtx} {...inlineAttr}>
        <span className="dom-tree-spacer" />
        <DomOpenTag el={el} />
        <span className="dom-text">{children[0].textContent?.trim()}</span>
        <DomCloseTag tag={tag} />
      </div>
    );
  }

  if (!expanded) {
    return (
      <div className={lineClass} style={indent} onContextMenu={handleCtx} {...currentAttr}>
        <span className="dom-tree-toggle" onClick={() => setUserExpanded(true)} role="button" tabIndex={-1}>&#x25B6;</span>
        <DomOpenTag el={el} />
        <span className="dom-ellipsis">…</span>
        <DomCloseTag tag={tag} />
      </div>
    );
  }

  return (
    <>
      <div className={lineClass} style={indent} onContextMenu={handleCtx} {...currentAttr}>
        <span className="dom-tree-toggle" onClick={() => setUserExpanded(false)} role="button" tabIndex={-1}>&#x25BC;</span>
        <DomOpenTag el={el} />
      </div>
      {children.map((child, i) => (
        <DomTreeNode key={i} node={child} depth={depth + 1} onCtxMenu={onCtxMenu} matchedNodes={matchedNodes} forceExpand={forceExpand} textSearch={textSearch} currentNode={currentNode} />
      ))}
      <div className={lineClass} style={indent} onContextMenu={handleCtx}>
        <span className="dom-tree-spacer" />
        <DomCloseTag tag={tag} />
      </div>
    </>
  );
}

export function HtmlInspectorPanel({ doc, matchedNodes, forceExpand, textSearch, currentNode }: {
  doc: Document;
  matchedNodes: ReadonlySet<Node>;
  forceExpand: ReadonlySet<Node>;
  textSearch: string;
  currentNode: Node | null;
}) {
  const treeRef = useRef<HTMLDivElement>(null);
  const [menu, setMenu] = useState<XPathMenuState>(null);
  const closeMenu = useCallback(() => setMenu(null), []);

  const handleCtxMenu = useCallback((e: React.MouseEvent, el: Element) => {
    setMenu({ x: e.clientX, y: e.clientY, xpath: computeSmartXPath(el), fullXpath: computeFullXPath(el) });
  }, []);

  const preventDefault = useCallback((e: React.MouseEvent) => e.preventDefault(), []);

  useEffect(() => {
    if (!currentNode) return;
    const el = treeRef.current?.querySelector("[data-current-match]");
    if (el) el.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [currentNode]);

  return (
    <div className="html-inspector-panel">
      <div ref={treeRef} className="html-inspector-tree" onContextMenu={preventDefault}>
        {Array.from(doc.childNodes).map((child, i) => (
          <DomTreeNode key={i} node={child} depth={0} onCtxMenu={handleCtxMenu} matchedNodes={matchedNodes} forceExpand={forceExpand} textSearch={textSearch} currentNode={currentNode} />
        ))}
      </div>
      <XPathContextMenu menu={menu} onClose={closeMenu} />
    </div>
  );
}
