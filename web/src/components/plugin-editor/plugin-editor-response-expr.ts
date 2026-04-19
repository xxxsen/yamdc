// ---------------------------------------------------------------------------
// Expression runners (client-side xpath / jsonpath)
// ---------------------------------------------------------------------------

export function runResponseExpr(input: { body: string; expr: string; kind: "xpath" | "jsonpath"; contentType: string }): string {
  const expr = input.expr.trim();
  if (!expr) {
    return "请输入表达式。";
  }
  try {
    if (input.kind === "xpath") {
      return runXPathExpr(input.body, expr);
    }
    return runJSONExpr(input.body, expr);
  } catch (error) {
    return error instanceof Error ? error.message : "表达式执行失败。";
  }
}

function runXPathExpr(body: string, expr: string): string {
  const parser = new DOMParser();
  const doc = parser.parseFromString(body, "text/html");
  const result = doc.evaluate(expr, doc, null, XPathResult.ANY_TYPE, null);
  const values: string[] = [];
  switch (result.resultType) {
    case XPathResult.STRING_TYPE:
      return result.stringValue || "";
    case XPathResult.NUMBER_TYPE:
      return String(result.numberValue);
    case XPathResult.BOOLEAN_TYPE:
      return String(result.booleanValue);
    default: {
      let node = result.iterateNext();
      while (node) {
        if ("textContent" in node) {
          values.push(node.textContent ?? "");
        } else {
          /* v8 ignore next -- all standard DOM nodes have textContent */
          values.push(String(node));
        }
        node = result.iterateNext();
      }
      return values.length > 0 ? JSON.stringify(values, null, 2) : "无匹配结果";
    }
  }
}

// jsonpath 单步解析 + 三种 token 分支 (field/field[*]/field[N]) 本质上就是
// 17 条路径; 拆分会让可读性变差.
// 已被 plugin-editor-utils.test / plugin-editor-utils-coverage.test 覆盖.
// eslint-disable-next-line complexity
function runJSONExpr(body: string, expr: string): string {
  const data = JSON.parse(body);
  const normalized = expr.replace(/^\$\./, "").replace(/^\$/, "");
  if (!normalized) {
    return JSON.stringify(data, null, 2);
  }
  const tokens = normalized.split(".").filter(Boolean);
  let current: unknown[] = [data];
  for (const token of tokens) {
    const next: unknown[] = [];
    const arrayMatch = token.match(/^([A-Za-z0-9_-]+)\[\*\]$/);
    const indexMatch = token.match(/^([A-Za-z0-9_-]+)\[(\d+)\]$/);
    for (const item of current) {
      if (arrayMatch) {
        const value = item && typeof item === "object" ? (item as Record<string, unknown>)[arrayMatch[1]] : undefined;
        if (Array.isArray(value)) {
          next.push(...value);
        }
        continue;
      }
      if (indexMatch) {
        const value = item && typeof item === "object" ? (item as Record<string, unknown>)[indexMatch[1]] : undefined;
        if (Array.isArray(value)) {
          next.push(value[Number(indexMatch[2])]);
        }
        continue;
      }
      if (item && typeof item === "object") {
        next.push((item as Record<string, unknown>)[token]);
      }
    }
    current = next.filter((item) => item !== undefined);
  }
  if (current.length === 0) {
    return "无匹配结果";
  }
  if (current.length === 1) {
    return typeof current[0] === "string" ? current[0] : JSON.stringify(current[0], null, 2);
  }
  return JSON.stringify(current, null, 2);
}
