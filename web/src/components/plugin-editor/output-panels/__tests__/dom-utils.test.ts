// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  ancestorSet,
  ATTR_VALUE_MAX,
  AUTO_EXPAND_DEPTH,
  collectTextMatchNodes,
  computeFullXPath,
  computeSmartXPath,
  copyToClipboard,
  elementMatchesText,
  EMPTY_NODE_SET,
  escapeRegex,
  INLINE_TEXT_MAX,
  isHtmlLike,
  visibleChildNodes,
  xpathSiblingIndex,
} from "../dom-utils";

// output-panels/dom-utils: XPath 计算 + DOM 文本匹配/遍历. 算法分支多,
// 单测需要覆盖 正常case (典型 HTML) / 异常case (content-type 空串, 没有
// parent, 多兄弟同 tag) / 边缘case (html/body skip, 没 class 没 id 回退
// 到 sibling index).

function parseHTML(html: string): Document {
  return new DOMParser().parseFromString(html, "text/html");
}

describe("constants", () => {
  it("exports fixed numeric limits", () => {
    expect(INLINE_TEXT_MAX).toBe(80);
    expect(AUTO_EXPAND_DEPTH).toBe(2);
    expect(ATTR_VALUE_MAX).toBe(60);
    expect(EMPTY_NODE_SET.size).toBe(0);
  });
});

describe("isHtmlLike", () => {
  it("matches text/html regardless of case", () => {
    expect(isHtmlLike("text/html")).toBe(true);
    expect(isHtmlLike("TEXT/HTML; charset=utf-8")).toBe(true);
  });

  it("empty string is treated as HTML-like (fallback)", () => {
    expect(isHtmlLike("")).toBe(true);
  });

  it("non-HTML returns false", () => {
    expect(isHtmlLike("application/json")).toBe(false);
    expect(isHtmlLike("text/plain")).toBe(false);
  });
});

describe("escapeRegex", () => {
  it("escapes all regex metacharacters", () => {
    const result = escapeRegex(".*+?^${}()|[]\\");
    // 每个 metachar 都要被前置 \
    expect(result).toBe("\\.\\*\\+\\?\\^\\$\\{\\}\\(\\)\\|\\[\\]\\\\");
  });

  it("leaves plain text alone", () => {
    expect(escapeRegex("hello world")).toBe("hello world");
  });

  it("empty string stays empty", () => {
    expect(escapeRegex("")).toBe("");
  });
});

describe("copyToClipboard", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("uses navigator.clipboard when available", () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    copyToClipboard("hi");
    expect(writeText).toHaveBeenCalledWith("hi");
  });

  it("falls back to execCommand when navigator.clipboard rejects", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("blocked"));
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    // jsdom 不实现 document.execCommand, 直接 defineProperty 塞进去.
    const execFn = vi.fn().mockReturnValue(true);
    Object.defineProperty(document, "execCommand", {
      value: execFn,
      configurable: true,
      writable: true,
    });

    copyToClipboard("fallback-text");
    await Promise.resolve();
    await Promise.resolve();

    expect(execFn).toHaveBeenCalledWith("copy");
  });

  it("swallows execCommand throwing in the fallback", async () => {
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockRejectedValue(new Error("no")) },
      configurable: true,
    });
    Object.defineProperty(document, "execCommand", {
      value: () => {
        throw new Error("also no");
      },
      configurable: true,
      writable: true,
    });
    expect(() => copyToClipboard("x")).not.toThrow();
    await Promise.resolve();
    await Promise.resolve();
  });
});

describe("xpathSiblingIndex", () => {
  it("returns 0 when no parent", () => {
    const el = document.createElement("div");
    expect(xpathSiblingIndex(el)).toBe(0);
  });

  it("returns 0 when parent has only one child of that tag", () => {
    const doc = parseHTML("<html><body><div><p>only</p></div></body></html>");
    const p = doc.querySelector("p")!;
    expect(xpathSiblingIndex(p)).toBe(0);
  });

  it("returns 1-based position among same-tag siblings", () => {
    const doc = parseHTML("<html><body><p>a</p><p>b</p><p>c</p></body></html>");
    const ps = doc.querySelectorAll("p");
    expect(xpathSiblingIndex(ps[0])).toBe(1);
    expect(xpathSiblingIndex(ps[1])).toBe(2);
    expect(xpathSiblingIndex(ps[2])).toBe(3);
  });

  it("only counts siblings with the same tag", () => {
    const doc = parseHTML("<html><body><span>s1</span><p>p1</p><span>s2</span></body></html>");
    const p = doc.querySelector("p")!;
    expect(xpathSiblingIndex(p)).toBe(0);
  });
});

describe("computeFullXPath", () => {
  it("produces an absolute path from html down to target", () => {
    const doc = parseHTML("<html><body><div><span>x</span></div></body></html>");
    const span = doc.querySelector("span")!;
    expect(computeFullXPath(span)).toBe("/html/body/div/span");
  });

  it("includes [n] on any same-tag siblings along the way", () => {
    const doc = parseHTML("<html><body><p>a</p><p><b>B</b></p></body></html>");
    const b = doc.querySelector("b")!;
    expect(computeFullXPath(b)).toBe("/html/body/p[2]/b");
  });
});

describe("computeSmartXPath", () => {
  it("anchors on id when available and stops walking up", () => {
    const doc = parseHTML("<html><body><div id=\"root\"><span>x</span></div></body></html>");
    const span = doc.querySelector("span")!;
    const xpath = computeSmartXPath(span);
    // 遇到 id 会直接 //tag[@id="..."] 作为起点
    expect(xpath).toBe('//div[@id="root"]/span');
  });

  it("uses @class when no id is present", () => {
    const doc = parseHTML("<html><body><div class=\"card\"><span>x</span></div></body></html>");
    const span = doc.querySelector("span")!;
    expect(computeSmartXPath(span)).toBe('//div[@class="card"]/span');
  });

  it("falls back to plain tag when neither id nor class", () => {
    const doc = parseHTML("<html><body><div><span>x</span></div></body></html>");
    const span = doc.querySelector("span")!;
    expect(computeSmartXPath(span)).toBe("//div/span");
  });

  it("appends [n] when there are same-tag siblings sharing a class", () => {
    const doc = parseHTML(
      "<html><body><div class=\"c\">a</div><div class=\"c\"><span>x</span></div></body></html>",
    );
    const span = doc.querySelector("div.c:nth-of-type(2) span")!;
    expect(computeSmartXPath(span)).toBe('//div[@class="c"][2]/span');
  });

  it("skips html/body in the path", () => {
    const doc = parseHTML("<html><body><div>x</div></body></html>");
    const div = doc.querySelector("div")!;
    // 遇到 body / html 都 continue, 没 id 没 class -> 只输出 div
    expect(computeSmartXPath(div)).toBe("//div");
  });

  it("includes [n] for plain tag when sibling count > 1", () => {
    const doc = parseHTML("<html><body><p>a</p><p>b</p></body></html>");
    const p2 = doc.querySelectorAll("p")[1];
    expect(computeSmartXPath(p2)).toBe("//p[2]");
  });
});

describe("visibleChildNodes", () => {
  it("keeps element and comment children", () => {
    const doc = parseHTML("<html><body><div><!--c--><p>x</p></div></body></html>");
    const div = doc.querySelector("div")!;
    const kids = visibleChildNodes(div);
    const kinds = kids.map((c) => c.nodeType);
    expect(kinds).toContain(1); // element
    expect(kinds).toContain(8); // comment
  });

  it("drops whitespace-only text nodes, keeps meaningful ones", () => {
    const doc = parseHTML("<html><body><div>   \n  <span>hello</span>   world</div></body></html>");
    const div = doc.querySelector("div")!;
    const kids = visibleChildNodes(div);
    // 应该保留 <span> 和 "   world" 文本节点, 丢掉起头的纯空白文本.
    const texts = kids.filter((c) => c.nodeType === 3).map((c) => c.textContent?.trim());
    expect(texts.every((t) => !!t)).toBe(true);
    expect(kids.filter((c) => c.nodeType === 1)).toHaveLength(1);
  });

  it("empty element returns empty array", () => {
    const doc = parseHTML("<html><body><div></div></body></html>");
    const div = doc.querySelector("div")!;
    expect(visibleChildNodes(div)).toEqual([]);
  });
});

describe("elementMatchesText", () => {
  it("matches by tag name (case-insensitive)", () => {
    const el = document.createElement("span");
    expect(elementMatchesText(el, "SPA")).toBe(true);
  });

  it("matches by attribute name or value", () => {
    const doc = parseHTML("<html><body><div data-id=\"foo\">x</div></body></html>");
    const div = doc.querySelector("div")!;
    expect(elementMatchesText(div, "data-id")).toBe(true);
    expect(elementMatchesText(div, "foo")).toBe(true);
    expect(elementMatchesText(div, "FOO")).toBe(true);
  });

  it("returns false when nothing matches", () => {
    const el = document.createElement("p");
    expect(elementMatchesText(el, "nomatch")).toBe(false);
  });
});

describe("collectTextMatchNodes", () => {
  it("returns elements matching by tag/attribute and text nodes matching body", () => {
    const doc = parseHTML(
      "<html><body><div id=\"hello\"><span>world</span></div><p>goodbye hello</p></body></html>",
    );
    const matches = collectTextMatchNodes(doc, "hello");
    // id="hello" 的 div (element match) + <p> 里的文本 "goodbye hello"
    expect(matches.length).toBeGreaterThanOrEqual(2);
    expect(matches.some((n) => n.nodeType === 1 && (n as Element).tagName === "DIV")).toBe(true);
    expect(matches.some((n) => n.nodeType === 3)).toBe(true);
  });

  it("returns empty when nothing matches", () => {
    const doc = parseHTML("<html><body><p>abc</p></body></html>");
    expect(collectTextMatchNodes(doc, "zzz")).toEqual([]);
  });

  it("is case-insensitive on text", () => {
    const doc = parseHTML("<html><body><p>Hello</p></body></html>");
    const matches = collectTextMatchNodes(doc, "hello");
    expect(matches.some((n) => n.nodeType === 3)).toBe(true);
  });

  it("ignores whitespace-only text nodes", () => {
    const doc = parseHTML("<html><body><p>   </p></body></html>");
    expect(collectTextMatchNodes(doc, " ")).toEqual([]);
  });
});

describe("ancestorSet", () => {
  it("returns EMPTY_NODE_SET for empty input", () => {
    expect(ancestorSet([])).toBe(EMPTY_NODE_SET);
  });

  it("collects unique ancestors of all inputs", () => {
    const doc = parseHTML("<html><body><div><p><span>x</span></p><p><span>y</span></p></div></body></html>");
    const spans = Array.from(doc.querySelectorAll("span"));
    const set = ancestorSet(spans);
    // ancestors are p, div, body, html, document — at minimum the two p's + div are in there
    const ps = doc.querySelectorAll("p");
    const div = doc.querySelector("div")!;
    expect(set.has(ps[0])).toBe(true);
    expect(set.has(ps[1])).toBe(true);
    expect(set.has(div)).toBe(true);
  });

  it("breaks early on second traversal when ancestor already seen (sanity: still correct size)", () => {
    const doc = parseHTML("<html><body><div><p>a</p><p>b</p></div></body></html>");
    const ps = Array.from(doc.querySelectorAll("p"));
    const set = ancestorSet(ps);
    const div = doc.querySelector("div")!;
    expect(set.has(div)).toBe(true);
    // break 后不会再把 div 重复加, Set 语义本来就保证唯一.
    expect(set.size).toBeGreaterThan(0);
  });
});
