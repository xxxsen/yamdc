// output-panels/dom-utils: plugin-editor body 面板相关的纯工具集.
//
// 这里收拢的是 **不依赖 React** 的逻辑:
//   - content-type 判定
//   - 剪贴板回退
//   - XPath 计算 (computeFullXPath / computeSmartXPath)
//   - DOM 遍历 / 文本匹配 (visibleChildNodes, elementMatchesText,
//     collectTextMatchNodes, ancestorSet)
//
// 这批函数之前散在 output-panels.tsx 内部, 拆出来目的是:
//   1. 让 output-panels.tsx 回到 ≤ 400 行, 只放 UI 编排
//   2. 纯函数可以单测 (后续加测试不会被 React 环境绊住)
//
// 详见 td/022-frontend-optimization-roadmap.md §2.1 A-1.

export const INLINE_TEXT_MAX = 80;
export const AUTO_EXPAND_DEPTH = 2;
export const ATTR_VALUE_MAX = 60;
export const EMPTY_NODE_SET: ReadonlySet<Node> = new Set<Node>();

export function isHtmlLike(contentType: string): boolean {
  const ct = contentType.toLowerCase();
  return ct.includes("text/html") || ct === "";
}

export function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

export function copyToClipboard(text: string): void {
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

export function xpathSiblingIndex(el: Element): number {
  if (!el.parentElement) return 0;
  const siblings = Array.from(el.parentElement.children).filter((c) => c.tagName === el.tagName);
  if (siblings.length <= 1) return 0;
  return siblings.indexOf(el) + 1;
}

export function computeFullXPath(el: Element): string {
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

export function computeSmartXPath(el: Element): string {
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

export function visibleChildNodes(el: Element): ChildNode[] {
  return Array.from(el.childNodes).filter((c) => {
    if (c.nodeType === 1) return true;
    if (c.nodeType === 3) return !!c.textContent?.trim();
    if (c.nodeType === 8) return true;
    return false;
  });
}

export function elementMatchesText(el: Element, q: string): boolean {
  const lower = q.toLowerCase();
  const tag = el.tagName.toLowerCase();
  if (tag.includes(lower)) return true;
  for (let i = 0; i < el.attributes.length; i++) {
    const a = el.attributes[i];
    if (a.name.toLowerCase().includes(lower) || a.value.toLowerCase().includes(lower)) return true;
  }
  return false;
}

export function collectTextMatchNodes(doc: Document, q: string): Node[] {
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

export function ancestorSet(nodes: Node[]): ReadonlySet<Node> {
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
