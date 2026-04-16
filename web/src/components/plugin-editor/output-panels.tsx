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

/* ── DOM Inspector tree ── */

const INLINE_TEXT_MAX = 80;
const AUTO_EXPAND_DEPTH = 2;
const ATTR_VALUE_MAX = 60;

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

function DomTreeNode({ node, depth, onCtxMenu }: {
  node: ChildNode;
  depth: number;
  onCtxMenu: (e: React.MouseEvent, el: Element) => void;
}) {
  const [expanded, setExpanded] = useState(depth < AUTO_EXPAND_DEPTH);
  const indent = { paddingLeft: depth * 16 };

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
    return (
      <div className="dom-tree-line" style={indent}>
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
  function handleCtx(e: React.MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    onCtxMenu(e, el);
  }

  if (!hasChildren) {
    return (
      <div className="dom-tree-line" style={indent} onContextMenu={handleCtx}>
        <span className="dom-tree-spacer" />
        <DomOpenTag el={el} /><DomCloseTag tag={tag} />
      </div>
    );
  }

  if (isSingleShortText) {
    return (
      <div className="dom-tree-line" style={indent} onContextMenu={handleCtx}>
        <span className="dom-tree-spacer" />
        <DomOpenTag el={el} />
        <span className="dom-text">{children[0].textContent?.trim()}</span>
        <DomCloseTag tag={tag} />
      </div>
    );
  }

  if (!expanded) {
    return (
      <div className="dom-tree-line" style={indent} onContextMenu={handleCtx}>
        <span className="dom-tree-toggle" onClick={() => setExpanded(true)} role="button" tabIndex={-1}>&#x25B6;</span>
        <DomOpenTag el={el} />
        <span className="dom-ellipsis">…</span>
        <DomCloseTag tag={tag} />
      </div>
    );
  }

  return (
    <>
      <div className="dom-tree-line" style={indent} onContextMenu={handleCtx}>
        <span className="dom-tree-toggle" onClick={() => setExpanded(false)} role="button" tabIndex={-1}>&#x25BC;</span>
        <DomOpenTag el={el} />
      </div>
      {children.map((child, i) => (
        <DomTreeNode key={i} node={child} depth={depth + 1} onCtxMenu={onCtxMenu} />
      ))}
      <div className="dom-tree-line" style={indent} onContextMenu={handleCtx}>
        <span className="dom-tree-spacer" />
        <DomCloseTag tag={tag} />
      </div>
    </>
  );
}

function HtmlInspectorPanel({ body }: { body: string }) {
  const [menu, setMenu] = useState<XPathMenuState>(null);
  const closeMenu = useCallback(() => setMenu(null), []);

  const doc = useMemo(() => new DOMParser().parseFromString(body, "text/html"), [body]);

  const handleCtxMenu = useCallback((e: React.MouseEvent, el: Element) => {
    setMenu({ x: e.clientX, y: e.clientY, xpath: computeSmartXPath(el), fullXpath: computeFullXPath(el) });
  }, []);

  const preventDefault = useCallback((e: React.MouseEvent) => e.preventDefault(), []);

  return (
    <div className="html-inspector-panel">
      <div className="html-inspector-tree" onContextMenu={preventDefault}>
        {Array.from(doc.childNodes).map((child, i) => (
          <DomTreeNode key={i} node={child} depth={0} onCtxMenu={handleCtxMenu} />
        ))}
      </div>
      <XPathContextMenu menu={menu} onClose={closeMenu} />
    </div>
  );
}

function BodyPanel(props: { body: string; contentType: string; emptyLabel: string }) {
  const [mode, setMode] = useState<"source" | "inspector">("source");

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

  const htmlMode = isHtmlLike(props.contentType);

  return (
    <details className="plugin-editor-output-detail-block plugin-editor-body-block" open>
      <summary className="plugin-editor-output-detail-summary">
        <span>Body</span>
        {htmlMode && (
          <span className="body-mode-toggle">
            <button type="button" className={`body-mode-btn${mode === "source" ? " body-mode-btn-active" : ""}`} onClick={() => setMode("source")}>Source</button>
            <button type="button" className={`body-mode-btn${mode === "inspector" ? " body-mode-btn-active" : ""}`} onClick={() => setMode("inspector")}>Inspector</button>
          </span>
        )}
      </summary>
      {mode === "source" || !htmlMode ? (
        <div className="plugin-editor-body-panel">
          <pre className="searcher-debug-json plugin-editor-json-scroll">{props.body}</pre>
        </div>
      ) : (
        <HtmlInspectorPanel body={props.body} />
      )}
    </details>
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
