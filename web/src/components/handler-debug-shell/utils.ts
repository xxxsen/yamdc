export const DEFAULT_META = {
  number: "ABC-123",
  title: "Sample Title",
  actors: [],
  genres: [],
  ext_info: {
    scrape_info: {
      source: "debug",
      date_ts: 0,
    },
  },
};

export const HANDLER_DEBUG_META_STORAGE_KEY = "yamdc.debug.handler.meta";
export const HANDLER_DEBUG_CHAIN_STORAGE_KEY = "yamdc.debug.handler.chain";

export type DiffKind = "added" | "removed" | "changed";

export type DiffRow = {
  kind: DiffKind | "unchanged";
  beforeLineNumber?: number;
  beforeLine?: string;
  afterLineNumber?: number;
  afterLine?: string;
};

// buildJSONDiffRows: 经典 LCS (Longest Common Subsequence) 按行对比.
//
// 使用自底向上的 DP 填表 (dp[i][j] = 从 (i,j) 起 before[i..] 与
// after[j..] 的最长公共子序列长度), 然后通过 i / j 同步游走把对齐
// 结果展开成 DiffRow 列表: "两边都能匹配 -> unchanged, 只能推进 before
// -> removed, 只能推进 after -> added". 当两边等长但内容不同时,
// LCS 算法会把它们拆成一对 removed + added 而不是 "changed", 这与
// 常见的 git diff 风格一致.
//
// 时间复杂度 O(m*n), 空间 O(m*n). 当前 handler-debug 的 meta JSON 都是
// 几十行的小文档, 没有必要引 fast-diff 类库; 保持纯函数, 方便单测.
export function buildJSONDiffRows(before: string, after: string): DiffRow[] {
  const beforeLines = before.split("\n");
  const afterLines = after.split("\n");
  const m = beforeLines.length;
  const n = afterLines.length;
  const dp = Array.from({ length: m + 1 }, () => Array<number>(n + 1).fill(0));

  for (let i = m - 1; i >= 0; i -= 1) {
    for (let j = n - 1; j >= 0; j -= 1) {
      if (beforeLines[i] === afterLines[j]) {
        dp[i][j] = dp[i + 1][j + 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1]);
      }
    }
  }

  const rows: DiffRow[] = [];
  let i = 0;
  let j = 0;
  let beforeLineNumber = 1;
  let afterLineNumber = 1;

  while (i < m && j < n) {
    if (beforeLines[i] === afterLines[j]) {
      rows.push({
        kind: "unchanged",
        beforeLineNumber,
        beforeLine: beforeLines[i],
        afterLineNumber,
        afterLine: afterLines[j],
      });
      i += 1;
      j += 1;
      beforeLineNumber += 1;
      afterLineNumber += 1;
      continue;
    }
    if (dp[i + 1][j] >= dp[i][j + 1]) {
      rows.push({
        kind: "removed",
        beforeLineNumber,
        beforeLine: beforeLines[i],
      });
      i += 1;
      beforeLineNumber += 1;
      continue;
    }
    rows.push({
      kind: "added",
      afterLineNumber,
      afterLine: afterLines[j],
    });
    j += 1;
    afterLineNumber += 1;
  }

  while (i < m) {
    rows.push({
      kind: "removed",
      beforeLineNumber,
      beforeLine: beforeLines[i],
    });
    i += 1;
    beforeLineNumber += 1;
  }

  while (j < n) {
    rows.push({
      kind: "added",
      afterLineNumber,
      afterLine: afterLines[j],
    });
    j += 1;
    afterLineNumber += 1;
  }

  return rows;
}
