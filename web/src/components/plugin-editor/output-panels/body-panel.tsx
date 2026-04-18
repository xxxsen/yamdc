"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  EMPTY_NODE_SET,
  ancestorSet,
  collectTextMatchNodes,
  escapeRegex,
  isHtmlLike,
} from "./dom-utils";
import { HtmlInspectorPanel } from "./dom-tree";
import { HeaderList } from "./header-list";

// body-panel: plugin-editor "Body" 区块的 UI + 搜索 hook.
//
// 对外只暴露 BodyPanel, 其余都是内部细节:
//
//   BodyPanel
//     ├── BodySearchBar                 // 关键字 / 模式切换 / 跳转
//     ├── HighlightedSource             // Source 模式下的高亮 <pre>
//     └── HtmlInspectorPanel (外部)     // Inspector 模式下的 DOM 树
//     └── HeaderList (form-urlencoded)  // x-www-form-urlencoded 走 header 样式
//
//   useBodySearch: 集中管理 search/kind/matchIdx + 三种匹配集合
//     (xpathNodes / inspectorTextNodes / sourceTextCount), 顺便兜住
//     "每次搜索把祖先节点强制展开" 的粘性行为.

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
  const forceExpandSetRaw = useMemo<ReadonlySet<Node>>(() => ancestorSet(inspectorNodes), [inspectorNodes]);

  const [stickyForceExpand, setStickyForceExpand] = useState<ReadonlySet<Node>>(EMPTY_NODE_SET);
  const [prevRaw, setPrevRaw] = useState<ReadonlySet<Node>>(EMPTY_NODE_SET);
  if (forceExpandSetRaw !== prevRaw) {
    setPrevRaw(forceExpandSetRaw);
    if (forceExpandSetRaw.size > 0) setStickyForceExpand(forceExpandSetRaw);
  }
  const forceExpandSet = forceExpandSetRaw.size > 0 ? forceExpandSetRaw : stickyForceExpand;

  const handleSearch = useCallback((s: string) => {
    setSearch(s);
    setMatchIdx(0);
    if (!s.trim()) setStickyForceExpand(EMPTY_NODE_SET);
  }, []);
  const handleKind = useCallback((k: "text" | "xpath") => { setKind(k); setMatchIdx(0); }, []);

  return { search, kind, matchIdx: safeIdx, matchCount, xpathMatchSet, forceExpandSet, currentMatchNode, handleSearch, handleKind, setMatchIdx };
}

export function BodyPanel(props: { body: string; contentType: string; emptyLabel: string }) {
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
