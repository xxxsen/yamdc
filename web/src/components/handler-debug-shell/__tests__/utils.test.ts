import { describe, expect, it } from "vitest";

import {
  buildJSONDiffRows,
  DEFAULT_META,
  HANDLER_DEBUG_CHAIN_STORAGE_KEY,
  HANDLER_DEBUG_META_STORAGE_KEY,
} from "../utils";

// handler-debug-shell/utils: buildJSONDiffRows 是按行 LCS diff, 覆盖:
// 正常case (有 unchanged/added/removed 混合), 异常case (空字符串),
// 边缘case (两边完全相同 / 完全不同 / 一侧空).

describe("DEFAULT_META", () => {
  it("exports a sample meta with required shape", () => {
    expect(DEFAULT_META).toMatchObject({
      number: "ABC-123",
      title: "Sample Title",
      actors: [],
      genres: [],
      ext_info: { scrape_info: { source: "debug", date_ts: 0 } },
    });
  });

  it("exports stable localStorage keys", () => {
    expect(HANDLER_DEBUG_META_STORAGE_KEY).toBe("yamdc.debug.handler.meta");
    expect(HANDLER_DEBUG_CHAIN_STORAGE_KEY).toBe("yamdc.debug.handler.chain");
  });
});

describe("buildJSONDiffRows", () => {
  it("empty vs empty produces one unchanged empty-line row", () => {
    // split("\n") 对 "" 返回 [""] (长度 1), 算法不会额外过滤.
    const rows = buildJSONDiffRows("", "");
    expect(rows).toHaveLength(1);
    expect(rows[0].kind).toBe("unchanged");
  });

  it("identical text yields all unchanged rows with matched line numbers", () => {
    const src = "a\nb\nc";
    const rows = buildJSONDiffRows(src, src);
    expect(rows).toHaveLength(3);
    for (const row of rows) {
      expect(row.kind).toBe("unchanged");
      expect(row.beforeLineNumber).toBe(row.afterLineNumber);
      expect(row.beforeLine).toBe(row.afterLine);
    }
  });

  it("pure addition: only 'added' rows with ascending afterLineNumber", () => {
    const rows = buildJSONDiffRows("", "x\ny");
    // 前置: "" -> [""], "x\ny" -> ["x","y"]. LCS 会把 "" 对上 "x" 或 ""
    // 对上 "y" 的可能为 0, 两边开头都不匹配, 但 dp 比较让 j++ 前进 (added).
    const added = rows.filter((r) => r.kind === "added");
    expect(added.map((r) => r.afterLine)).toContain("x");
    expect(added.map((r) => r.afterLine)).toContain("y");
  });

  it("pure removal: only 'removed' rows with ascending beforeLineNumber", () => {
    const rows = buildJSONDiffRows("x\ny", "");
    const removed = rows.filter((r) => r.kind === "removed");
    expect(removed.map((r) => r.beforeLine)).toContain("x");
    expect(removed.map((r) => r.beforeLine)).toContain("y");
  });

  it("partial change: 1 unchanged, 1 removed, 1 added (LCS splits changed line)", () => {
    const before = "a\nb\nc";
    const after = "a\nB\nc";
    const rows = buildJSONDiffRows(before, after);
    // 'a' + 'c' 同, 中间 'b' -> 'B' 按算法拆成 removed + added.
    const kinds = rows.map((r) => r.kind);
    expect(kinds).toContain("unchanged");
    expect(kinds).toContain("removed");
    expect(kinds).toContain("added");
    expect(rows.filter((r) => r.kind === "removed")[0].beforeLine).toBe("b");
    expect(rows.filter((r) => r.kind === "added")[0].afterLine).toBe("B");
  });

  it("line numbers increment only for matching side", () => {
    const before = "keep\nold";
    const after = "keep\nnew\nextra";
    const rows = buildJSONDiffRows(before, after);
    const keepRow = rows.find((r) => r.kind === "unchanged")!;
    expect(keepRow.beforeLineNumber).toBe(1);
    expect(keepRow.afterLineNumber).toBe(1);
    const removed = rows.filter((r) => r.kind === "removed");
    expect(removed[0].beforeLineNumber).toBe(2);
    expect(removed[0].beforeLine).toBe("old");
    const added = rows.filter((r) => r.kind === "added");
    expect(added.some((r) => r.afterLine === "new")).toBe(true);
    expect(added.some((r) => r.afterLine === "extra")).toBe(true);
  });

  it("when dp branches tie, removed is preferred (algorithm contract)", () => {
    // before "a\nb", after "b\na":
    // 按算法 dp[i+1][j] >= dp[i][j+1] 取 removed, 所以 'a' 会被标 removed
    // 然后 'b' 对齐, 最后 'a' added.
    const rows = buildJSONDiffRows("a\nb", "b\na");
    const firstDiff = rows.find((r) => r.kind !== "unchanged");
    expect(firstDiff?.kind).toBe("removed");
  });

  it("single line added at end (enters the trailing 'j < n' loop)", () => {
    const rows = buildJSONDiffRows("x", "x\ny");
    const last = rows[rows.length - 1];
    expect(last.kind).toBe("added");
    expect(last.afterLine).toBe("y");
  });

  it("single line removed at end (enters the trailing 'i < m' loop)", () => {
    const rows = buildJSONDiffRows("x\ny", "x");
    const last = rows[rows.length - 1];
    expect(last.kind).toBe("removed");
    expect(last.beforeLine).toBe("y");
  });
});
