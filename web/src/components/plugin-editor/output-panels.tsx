"use client";

import { useState, useMemo, useRef, useEffect, useCallback, type ReactNode } from "react";
import { createPortal } from "react-dom";

import type {
  PluginEditorRequestDebugResult,
  PluginEditorScrapeDebugResult,
  PluginEditorWorkflowDebugResult,
} from "@/lib/api";

import type { XPathMenuState } from "./plugin-editor-types";
import { runResponseExpr } from "./plugin-editor-utils";

export function RequestBasicPanel({ result }: { result: PluginEditorRequestDebugResult | null }) {
  if (!result) {
    return <div className="ruleset-debug-empty">先执行一次运行调试。</div>;
  }
  return (
    <div className="plugin-editor-preview-grid">
      <div className="plugin-editor-preview-card">
        <span>Method</span>
        <strong>{result.request.method}</strong>
      </div>
      <div className="plugin-editor-preview-card">
        <span>Status</span>
        <strong>{result.response?.status_code ?? "-"}</strong>
      </div>
      <div className="plugin-editor-preview-card plugin-editor-preview-card-wide">
        <span>URL</span>
        <strong>{result.request.url}</strong>
      </div>
      {result.attempts?.length ? (
        <div className="plugin-editor-preview-card plugin-editor-preview-card-wide">
          <span>Attempts</span>
          <strong>{result.attempts.map((item) => `${item.candidate || "-"}:${item.matched ? "hit" : item.error || "skip"}`).join(" | ")}</strong>
        </div>
      ) : null}
    </div>
  );
}

export function RequestDetailPanel({ request }: { request?: PluginEditorRequestDebugResult["request"] | null }) {
  return <RequestDetailBlock request={request} emptyLabel="暂无请求数据。" />;
}

export function ResponseDetailPanel({ response }: { response?: PluginEditorRequestDebugResult["response"] | null }) {
  const [expr, setExpr] = useState("");
  const [kind, setKind] = useState<"xpath" | "jsonpath">("xpath");
  const [output, setOutput] = useState("");
  const body = response?.body || response?.body_preview || "";
  const contentType = response?.headers["content-type"]?.[0] || response?.headers["Content-Type"]?.[0] || "";

  function runExpr() {
    setOutput(runResponseExpr({ body, expr, kind, contentType }));
  }

  return (
    <ResponseDetailBlock
      response={response}
      emptyLabel="暂无响应数据。"
      extra={
        <details className="plugin-editor-output-detail-block">
          <summary>Expr Filter</summary>
          <div className="plugin-editor-expr-runner plugin-editor-expr-runner-card">
            <div className="plugin-editor-expr-runner-top">
              <input className="input" value={expr} onChange={(event) => setExpr(event.target.value)} placeholder={kind === "xpath" ? '//title/text()' : '$.result.name'} />
              <select className="input plugin-editor-expr-kind" value={kind} onChange={(event) => setKind(event.target.value as "xpath" | "jsonpath")}>
                <option value="xpath">xpath</option>
                <option value="jsonpath">json</option>
              </select>
              <button className="btn btn-primary" type="button" onClick={runExpr}>
                Run
              </button>
            </div>
            <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-expr-output">{output || "暂无结果"}</pre>
          </div>
        </details>
      }
    />
  );
}

export function WorkflowOutputPanel({ result }: { result: PluginEditorWorkflowDebugResult | null }) {
  if (!result) {
    return <div className="ruleset-debug-empty">暂无 workflow 结果。</div>;
  }
  return (
    <div className="plugin-editor-output-section plugin-editor-output-section-fill">
      {result.error ? <div className="plugin-editor-output-error">{result.error}</div> : null}
      <WorkflowDebugPreview result={result} />
      <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-json-fill">{JSON.stringify(result, null, 2)}</pre>
    </div>
  );
}

function WorkflowDebugPreview({ result }: { result: PluginEditorWorkflowDebugResult }) {
  return (
    <div className="plugin-editor-timeline">
      {result.steps.map((step, index) => (
        <article key={`${step.stage}-${index}`} className="plugin-editor-timeline-step">
          <div className="plugin-editor-timeline-head">
            <strong>{step.stage}</strong>
            <span>{step.summary}</span>
          </div>
          {step.candidate ? <div className="plugin-editor-timeline-detail">candidate: {step.candidate}</div> : null}
          {step.selected_value ? <div className="plugin-editor-timeline-detail">selected: {step.selected_value}</div> : null}
          {step.items?.length ? <div className="plugin-editor-timeline-detail">matched items: {step.items.filter((item) => item.matched).length}/{step.items.length}</div> : null}
          {step.request ? (
            <details className="plugin-editor-output-detail-block">
              <summary>Request</summary>
              <RequestDetailBlock request={step.request} emptyLabel="暂无请求数据。" />
            </details>
          ) : null}
          {step.response ? (
            <details className="plugin-editor-output-detail-block">
              <summary>Response</summary>
              <ResponseDetailBlock response={step.response} emptyLabel="暂无响应数据。" />
            </details>
          ) : null}
          {step.selectors && Object.keys(step.selectors).length ? (
            <details className="plugin-editor-output-detail-block">
              <summary>Selectors</summary>
              <SelectorDebugPanel selectors={step.selectors} />
            </details>
          ) : null}
          {step.items?.length ? (
            <details className="plugin-editor-output-detail-block">
              <summary>Matched Items</summary>
              <WorkflowItemsPanel items={step.items} />
            </details>
          ) : null}
        </article>
      ))}
      {result.error ? (
        <article className="plugin-editor-timeline-step plugin-editor-timeline-step-error">
          <div className="plugin-editor-timeline-head">
            <strong>error</strong>
            <span>{result.error}</span>
          </div>
        </article>
      ) : null}
    </div>
  );
}

export function ScrapeJSONPanel({ result }: { result: PluginEditorScrapeDebugResult | null }) {
  if (!result) {
    return <div className="ruleset-debug-empty">暂无抓取结果。</div>;
  }
  return (
    <div className="plugin-editor-output-section plugin-editor-output-section-fill">
      {result.error ? <div className="plugin-editor-output-error">{result.error}</div> : null}
      <pre className="searcher-debug-json plugin-editor-json-scroll plugin-editor-json-fill">{JSON.stringify(result.meta ?? result.fields, null, 2)}</pre>
    </div>
  );
}

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

function isHtmlLike(contentType: string): boolean {
  const ct = contentType.toLowerCase();
  return ct.includes("text/html") || ct === "";
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function copyToClipboard(text: string): void {
  void navigator.clipboard.writeText(text).catch(() => {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    // eslint-disable-next-line @typescript-eslint/no-deprecated -- fallback for older browsers
    try { document.execCommand("copy"); } catch { /* best-effort */ }
    document.body.removeChild(ta);
  });
}

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

/* ── XPath computation ── */

function xpathSiblingIndex(el: Element): number {
  if (!el.parentElement) return 0;
  const siblings = Array.from(el.parentElement.children).filter(c => c.tagName === el.tagName);
  if (siblings.length <= 1) return 0;
  return siblings.indexOf(el) + 1;
}

function computeFullXPath(el: Element): string {
  const parts: string[] = [];
  let cur: Element | null = el;
  while (cur) {
    const tag = cur.tagName.toLowerCase();
    const idx = xpathSiblingIndex(cur);
    parts.unshift(idx > 0 ? `${tag}[${idx}]` : tag);
    cur = cur.parentElement;
  }
  return "/" + parts.join("/");
}

function computeSmartXPath(el: Element): string {
  const parts: string[] = [];
  let cur: Element | null = el;
  while (cur) {
    const tag = cur.tagName.toLowerCase();
    if (tag === "html" || tag === "body") { cur = cur.parentElement; continue; }
    if (cur.id) {
      parts.unshift(`//${tag}[@id="${cur.id}"]`);
      return parts.join("/");
    }
    const cls = cur.getAttribute("class")?.trim() ?? "";
    if (cls) {
      const idx = xpathSiblingIndex(cur);
      const expr = `${tag}[@class="${cls}"]`;
      parts.unshift(idx > 0 ? `${expr}[${idx}]` : expr);
    } else {
      const idx = xpathSiblingIndex(cur);
      parts.unshift(idx > 0 ? `${tag}[${idx}]` : tag);
    }
    cur = cur.parentElement;
  }
  return "//" + parts.join("/");
}

/* ── Body search bar ── */

function BodySearchBar({ search, onSearch, kind, onKindChange, matchCount, matchIndex, onNav }: {
  search: string;
  onSearch: (s: string) => void;
  kind: "text" | "xpath";
  onKindChange: (k: "text" | "xpath") => void;
  matchCount: number;
  matchIndex: number;
  onNav: (i: number) => void;
}) {
  return (
    <div className="body-search-bar">
      <input
        className="body-search-input"
        value={search}
        onChange={(e) => onSearch(e.target.value)}
        placeholder={kind === "xpath" ? "XPath expression..." : "Search text..."}
      />
      <select className="body-search-kind" value={kind} onChange={(e) => onKindChange(e.target.value as "text" | "xpath")}>
        <option value="text">Text</option>
        <option value="xpath">XPath</option>
      </select>
      {search.trim() !== "" && (
        <>
          <span className="body-search-count">
            {matchCount > 0 ? `${matchIndex + 1} / ${matchCount}` : "0 results"}
          </span>
          {matchCount > 1 && (
            <>
              <button type="button" className="body-search-nav" onClick={() => onNav((matchIndex - 1 + matchCount) % matchCount)} title="Previous">&#x25B2;</button>
              <button type="button" className="body-search-nav" onClick={() => onNav((matchIndex + 1) % matchCount)} title="Next">&#x25BC;</button>
            </>
          )}
        </>
      )}
    </div>
  );
}

/* ── Highlighted source (text search in Source mode) ── */

function HighlightedSource({ body, search, matchIndex }: { body: string; search: string; matchIndex: number }) {
  const ref = useRef<HTMLPreElement>(null);

  const segments = useMemo(() => {
    if (!search) return null;
    const regex = new RegExp(escapeRegex(search), "gi");
    const parts: { text: string; hit: boolean }[] = [];
    let last = 0;
    let m: RegExpExecArray | null;
    while ((m = regex.exec(body)) !== null) {
      if (m.index > last) parts.push({ text: body.slice(last, m.index), hit: false });
      parts.push({ text: m[0], hit: true });
      last = m.index + m[0].length;
      if (parts.length > 20_000) break;
    }
    if (last < body.length) parts.push({ text: body.slice(last), hit: false });
    return parts;
  }, [body, search]);

  useEffect(() => {
    const el = ref.current?.querySelector("[data-current-match]");
    if (el) el.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [matchIndex, search]);

  if (!segments) {
    return <pre className="searcher-debug-json plugin-editor-json-scroll">{body}</pre>;
  }

  let hitIdx = 0;
  return (
    <pre ref={ref} className="searcher-debug-json plugin-editor-json-scroll">
      {segments.map((seg, i) => {
        if (!seg.hit) return <span key={i}>{seg.text}</span>;
        const cur = hitIdx === matchIndex;
        hitIdx++;
        return <mark key={i} className={`body-search-hit${cur ? " body-search-current" : ""}`} {...(cur ? { "data-current-match": "" } : {})}>{seg.text}</mark>;
      })}
    </pre>
  );
}

/* ── DOM Inspector tree ── */

const INLINE_TEXT_MAX = 80;
const AUTO_EXPAND_DEPTH = 2;
const ATTR_VALUE_MAX = 60;
const EMPTY_NODE_SET: ReadonlySet<Node> = new Set<Node>();

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

function visibleChildNodes(el: Element): ChildNode[] {
  return Array.from(el.childNodes).filter(c => {
    if (c.nodeType === 1) return true;
    if (c.nodeType === 3) return !!c.textContent?.trim();
    if (c.nodeType === 8) return true;
    return false;
  });
}

function elementMatchesText(el: Element, q: string): boolean {
  const lower = q.toLowerCase();
  const tag = el.tagName.toLowerCase();
  if (tag.includes(lower)) return true;
  for (let i = 0; i < el.attributes.length; i++) {
    const a = el.attributes[i];
    if (a.name.toLowerCase().includes(lower) || a.value.toLowerCase().includes(lower)) return true;
  }
  return false;
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

function HtmlInspectorPanel({ doc, matchedNodes, forceExpand, textSearch, currentNode }: {
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

/* ── Body panel ── */

function collectTextMatchNodes(doc: Document, q: string): Node[] {
  const lower = q.toLowerCase();
  const nodes: Node[] = [];
  function walk(n: Node) {
    if (n.nodeType === 1 && elementMatchesText(n as Element, q)) {
      nodes.push(n);
    } else if (n.nodeType === 3) {
      const t = n.textContent?.trim();
      if (t && t.toLowerCase().includes(lower)) nodes.push(n);
    }
    for (let i = 0; i < n.childNodes.length; i++) walk(n.childNodes[i]);
  }
  walk(doc);
  return nodes;
}

function ancestorSet(nodes: Node[]): ReadonlySet<Node> {
  if (nodes.length === 0) return EMPTY_NODE_SET;
  const set = new Set<Node>();
  for (const n of nodes) {
    let cur = n.parentNode;
    while (cur) {
      if (set.has(cur)) break;
      set.add(cur);
      cur = cur.parentNode;
    }
  }
  return set;
}

function useBodySearch(body: string, doc: Document | null, mode: "source" | "inspector") {
  const [search, setSearch] = useState("");
  const [kind, setKind] = useState<"text" | "xpath">("text");
  const [matchIdx, setMatchIdx] = useState(0);

  const xpathNodes = useMemo((): Node[] => {
    const q = search.trim();
    if (!q || kind !== "xpath" || !doc) return [];
    try {
      const r = doc.evaluate(q, doc, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null);
      const arr: Node[] = [];
      for (let i = 0; i < r.snapshotLength; i++) {
        const n = r.snapshotItem(i);
        if (n) arr.push(n);
      }
      return arr;
    } catch { return []; }
  }, [search, kind, doc]);

  const inspectorTextNodes = useMemo((): Node[] => {
    const q = search.trim();
    if (!q || kind !== "text" || !doc) return [];
    return collectTextMatchNodes(doc, q);
  }, [search, kind, doc]);

  const sourceTextCount = useMemo(() => {
    const q = search.trim();
    if (!q || kind !== "text") return 0;
    return (body.match(new RegExp(escapeRegex(q), "gi")) ?? []).length;
  }, [search, kind, body]);

  const inspectorNodes = kind === "xpath" ? xpathNodes : inspectorTextNodes;
  const matchCount = mode === "source" && kind === "text" ? sourceTextCount : inspectorNodes.length;
  const safeIdx = matchCount === 0 ? 0 : Math.min(matchIdx, matchCount - 1);
  const currentMatchNode: Node | null = mode === "inspector" && inspectorNodes.length > 0 ? (inspectorNodes[safeIdx] ?? null) : null;

  const xpathMatchSet = useMemo<ReadonlySet<Node>>(() => (xpathNodes.length > 0 ? new Set(xpathNodes) : EMPTY_NODE_SET), [xpathNodes]);
  const forceExpandSet = useMemo<ReadonlySet<Node>>(() => ancestorSet(inspectorNodes), [inspectorNodes]);

  const handleSearch = useCallback((s: string) => { setSearch(s); setMatchIdx(0); }, []);
  const handleKind = useCallback((k: "text" | "xpath") => { setKind(k); setMatchIdx(0); }, []);

  return { search, kind, matchIdx: safeIdx, matchCount, xpathMatchSet, forceExpandSet, currentMatchNode, handleSearch, handleKind, setMatchIdx };
}

function BodyPanel(props: { body: string; contentType: string; emptyLabel: string }) {
  const [mode, setMode] = useState<"source" | "inspector">("source");
  const htmlMode = isHtmlLike(props.contentType);

  const doc = useMemo(() => {
    if (!htmlMode || !props.body) return null;
    return new DOMParser().parseFromString(props.body, "text/html");
  }, [props.body, htmlMode]);

  const bs = useBodySearch(props.body, doc, mode);

  if (!props.body) {
    return <div className="ruleset-debug-empty">{props.emptyLabel}</div>;
  }
  const contentType = props.contentType.toLowerCase();
  if (contentType.includes("application/json")) {
    let formatted = props.body;
    try {
      formatted = JSON.stringify(JSON.parse(props.body), null, 2);
    } catch {
      formatted = props.body;
    }
    return (
      <details className="plugin-editor-output-detail-block plugin-editor-body-block" open>
        <summary className="plugin-editor-output-detail-summary">
          <span>Body</span>
        </summary>
        <div className="plugin-editor-body-panel">
          <pre className="searcher-debug-json plugin-editor-json-scroll">{formatted}</pre>
        </div>
      </details>
    );
  }
  if (contentType.includes("application/x-www-form-urlencoded")) {
    const params = new URLSearchParams(props.body);
    const headers = Object.fromEntries(params.entries());
    return (
      <details className="plugin-editor-output-detail-block plugin-editor-body-block" open>
        <summary className="plugin-editor-output-detail-summary">
          <span>Body</span>
        </summary>
        <HeaderList headers={headers} />
      </details>
    );
  }

  const activeTextSearch = bs.kind === "text" ? bs.search.trim() : "";

  return (
    <div className="plugin-editor-output-detail-block plugin-editor-body-block">
      <div className="plugin-editor-output-detail-summary">
        <span>Body</span>
        {htmlMode && (
          <span className="body-mode-toggle">
            <button type="button" className={`body-mode-btn${mode === "source" ? " body-mode-btn-active" : ""}`} onClick={() => setMode("source")}>Source</button>
            <button type="button" className={`body-mode-btn${mode === "inspector" ? " body-mode-btn-active" : ""}`} onClick={() => setMode("inspector")}>Inspector</button>
          </span>
        )}
      </div>
      <div className="plugin-editor-body-inner">
        {htmlMode && (
          <BodySearchBar search={bs.search} onSearch={bs.handleSearch} kind={bs.kind} onKindChange={bs.handleKind} matchCount={bs.matchCount} matchIndex={bs.matchIdx} onNav={bs.setMatchIdx} />
        )}
        {mode === "source" || !htmlMode ? (
          <div className="plugin-editor-body-panel">
            {activeTextSearch ? (
              <HighlightedSource body={props.body} search={activeTextSearch} matchIndex={bs.matchIdx} />
            ) : (
              <pre className="searcher-debug-json plugin-editor-json-scroll">{props.body}</pre>
            )}
          </div>
        ) : (
          doc ? <HtmlInspectorPanel doc={doc} matchedNodes={bs.xpathMatchSet} forceExpand={bs.forceExpandSet} textSearch={activeTextSearch} currentNode={bs.currentMatchNode} /> : null
        )}
      </div>
    </div>
  );
}

function RequestDetailBlock({
  request,
  emptyLabel,
}: {
  request?: PluginEditorRequestDebugResult["request"] | null;
  emptyLabel: string;
}) {
  if (!request) {
    return <div className="ruleset-debug-empty">{emptyLabel}</div>;
  }
  const headerCount = Object.keys(request.headers).length;
  return (
    <div className="plugin-editor-output-detail">
      <details className="plugin-editor-output-detail-block" open>
        <summary className="plugin-editor-output-detail-summary">
          <span>Headers</span>
          <span className={`plugin-editor-request-json-count ${headerCount > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>{headerCount}</span>
        </summary>
        <HeaderList headers={request.headers} />
      </details>
      <BodyPanel body={request.body} contentType={request.headers["Content-Type"] || request.headers["content-type"] || ""} emptyLabel="请求体为空。" />
    </div>
  );
}

function ResponseDetailBlock({
  response,
  emptyLabel,
  extra,
}: {
  response?: PluginEditorRequestDebugResult["response"] | null;
  emptyLabel: string;
  extra?: ReactNode;
}) {
  if (!response) {
    return <div className="ruleset-debug-empty">{emptyLabel}</div>;
  }
  const contentType = (
    Object.entries(response.headers).find(([k]) => k.toLowerCase() === "content-type")?.[1]?.[0]
  ) ?? "";
  const headerMap = Object.fromEntries(Object.entries(response.headers).map(([key, values]) => [key, values.join(", ")]));
  const headerCount = Object.keys(headerMap).length;
  const body = response.body || response.body_preview;

  return (
    <div className="plugin-editor-output-detail">
      <details className="plugin-editor-output-detail-block" open>
        <summary className="plugin-editor-output-detail-summary">
          <span>Headers</span>
          <span className={`plugin-editor-request-json-count ${headerCount > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>{headerCount}</span>
        </summary>
        <HeaderList headers={headerMap} />
      </details>
      <BodyPanel body={body} contentType={contentType} emptyLabel="响应体为空。" />
      {extra}
    </div>
  );
}

function SelectorDebugPanel({ selectors }: { selectors: Record<string, string[]> }) {
  return (
    <div className="plugin-editor-workflow-selector-list">
      {Object.entries(selectors).map(([name, values]) => (
        <div key={name} className="plugin-editor-workflow-debug-card">
          <div className="plugin-editor-workflow-debug-head">
            <strong>{name}</strong>
            <span>{values.length}</span>
          </div>
          <pre className="searcher-debug-json plugin-editor-json-scroll">{JSON.stringify(values, null, 2)}</pre>
        </div>
      ))}
    </div>
  );
}

function WorkflowItemsPanel({ items }: { items: PluginEditorWorkflowDebugResult["steps"][number]["items"] }) {
  return (
    <div className="plugin-editor-workflow-item-list">
      {items?.map((item) => (
        <article key={item.index} className="plugin-editor-workflow-debug-card">
          <div className="plugin-editor-workflow-debug-head">
            <strong>Item #{item.index + 1}</strong>
            <span className={`plugin-editor-workflow-match-chip ${item.matched ? "plugin-editor-workflow-match-chip-hit" : "plugin-editor-workflow-match-chip-miss"}`}>
              {item.matched ? "matched" : "skipped"}
            </span>
          </div>
          <pre className="searcher-debug-json plugin-editor-json-scroll">{JSON.stringify(item.item, null, 2)}</pre>
          {item.item_variables && Object.keys(item.item_variables).length ? (
            <pre className="searcher-debug-json plugin-editor-json-scroll">{JSON.stringify({ item_variables: item.item_variables }, null, 2)}</pre>
          ) : null}
          {item.match_details?.length ? (
            <div className="plugin-editor-workflow-condition-list">
              {item.match_details.map((detail, index) => (
                <div key={`${detail.condition}-${index}`} className="plugin-editor-workflow-condition-row">
                  <span>{detail.condition}</span>
                  <strong>{detail.pass ? "pass" : "fail"}</strong>
                </div>
              ))}
            </div>
          ) : null}
        </article>
      ))}
    </div>
  );
}
