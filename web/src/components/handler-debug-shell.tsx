"use client";

import { LoaderCircle, Play } from "lucide-react";
import Image from "next/image";
import { useEffect, useMemo, useState } from "react";

import {
  debugHandler,
  getAssetURL,
  getHandlerDebugHandlers,
  type HandlerDebugInstance,
  type HandlerDebugResult,
} from "@/lib/api";

const DEFAULT_META = {
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

type DiffKind = "added" | "removed" | "changed";

type DiffRow = {
  kind: DiffKind | "unchanged";
  beforeLineNumber?: number;
  beforeLine?: string;
  afterLineNumber?: number;
  afterLine?: string;
};

function buildJSONDiffRows(before: string, after: string): DiffRow[] {
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

export function HandlerDebugShell() {
  const [handlers, setHandlers] = useState<HandlerDebugInstance[]>([]);
  const [selectedHandlerID, setSelectedHandlerID] = useState("");
  const [metaJSON, setMetaJSON] = useState(JSON.stringify(DEFAULT_META, null, 2));
  const [result, setResult] = useState<HandlerDebugResult | null>(null);
  const [error, setError] = useState("");
  const [prefillMessage, setPrefillMessage] = useState("");
  const [activeTab, setActiveTab] = useState<"json" | "pic">("json");
  const [isRunning, setIsRunning] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const next = await getHandlerDebugHandlers();
        setHandlers(next);
        setSelectedHandlerID((current) => current || next[0]?.id || "");
      } catch {
        // keep shell visible
      }
    })();
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const params = new URLSearchParams(window.location.search);
    if (params.get("prefill") !== "searcher") {
      return;
    }
    const stored = window.sessionStorage.getItem("yamdc.debug.handler_meta");
    if (!stored) {
      return;
    }
    setMetaJSON(stored);
    setResult(null);
    setError("");
    setPrefillMessage("已从 Searcher 调试带入当前 Meta JSON。");
    window.sessionStorage.removeItem("yamdc.debug.handler_meta");
  }, []);

  const diffRows = useMemo(() => {
    if (!result) {
      return [];
    }
    return buildJSONDiffRows(JSON.stringify(result.before_meta, null, 2), JSON.stringify(result.after_meta, null, 2));
  }, [result]);

  const picDiffState = useMemo(() => {
    if (!result) {
      return null;
    }
    return {
      coverChanged: JSON.stringify(result.before_meta.cover ?? null) !== JSON.stringify(result.after_meta.cover ?? null),
      posterChanged: JSON.stringify(result.before_meta.poster ?? null) !== JSON.stringify(result.after_meta.poster ?? null),
      sampleChanged: JSON.stringify(result.before_meta.sample_images ?? []) !== JSON.stringify(result.after_meta.sample_images ?? []),
    };
  }, [result]);

  const handleRun = () => {
    if (!selectedHandlerID) {
      setError("请选择一个 handler。");
      return;
    }
    let parsedMeta: Record<string, unknown>;
    try {
      parsedMeta = JSON.parse(metaJSON) as Record<string, unknown>;
    } catch {
      setError("Meta JSON 格式无效。");
      return;
    }
    if (typeof parsedMeta.number !== "string" || parsedMeta.number.trim() === "") {
      setError("Meta JSON 里必须包含 number 字段。");
      return;
    }

    setIsRunning(true);
    void (async () => {
      try {
        const next = await debugHandler({
          handler_id: selectedHandlerID,
          meta: parsedMeta as never,
        });
        setResult(next);
        setActiveTab("json");
        setError("");
      } catch (nextError) {
        setResult(null);
        setError(nextError instanceof Error ? nextError.message : "handler 调试失败");
      } finally {
        setIsRunning(false);
      }
    })();
  };

  return (
    <div className="handler-debug-page">
      <section className="panel handler-debug-hero">
        <div className="handler-debug-copy">
          <span className="ruleset-debug-eyebrow">
            <Play size={14} />
            Handler 调试
          </span>
          <h2>单 Handler 测试</h2>
          <p>选择当前系统里的 handler 实例，输入 Meta JSON，单独执行一次处理并查看前后差异。`duration_fixer` 当前不在 Web 调试范围内。</p>
        </div>

        <div className="handler-debug-controls">
          <div className="handler-debug-toolbar">
            <label className="handler-debug-field">
              <span>Handler</span>
              <select className="input handler-debug-select" value={selectedHandlerID} onChange={(event) => setSelectedHandlerID(event.target.value)}>
                {handlers.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
            <button className="btn btn-primary ruleset-debug-run-button handler-debug-run-inline" type="button" onClick={handleRun} disabled={isRunning}>
              {isRunning ? <LoaderCircle size={16} className="ruleset-debug-spinner" /> : <Play size={16} />}
              <span>{isRunning ? "执行中..." : "运行 Handler"}</span>
            </button>
          </div>

          <label className="handler-debug-field">
            <span>Meta JSON</span>
            <textarea className="input handler-debug-textarea" value={metaJSON} onChange={(event) => setMetaJSON(event.target.value)} />
          </label>

          {prefillMessage ? <div className="handler-debug-message">{prefillMessage}</div> : null}
          {error ? <div className="ruleset-debug-error">{error}</div> : null}
        </div>
      </section>

      <div className="handler-debug-results">
        <section className="panel handler-debug-panel">
          <div className="ruleset-debug-panel-head">
            <div className="handler-debug-tabs">
              <button type="button" className={`handler-debug-tab ${activeTab === "json" ? "handler-debug-tab-active" : ""}`} onClick={() => setActiveTab("json")}>
                Json Diff
              </button>
              <button type="button" className={`handler-debug-tab ${activeTab === "pic" ? "handler-debug-tab-active" : ""}`} onClick={() => setActiveTab("pic")}>
                Pic Diff
              </button>
            </div>
            {result?.error ? <span className="ruleset-debug-status ruleset-debug-status-no_match">error</span> : null}
          </div>
          {activeTab === "json" ? (
            result ? (
              diffRows.some((row) => row.kind !== "unchanged") ? (
                <div className="handler-debug-code-diff">
                  <div className="handler-debug-code-diff-head">
                    <div>Before</div>
                    <div>After</div>
                  </div>
                  <div className="handler-debug-code-diff-body">
                    {diffRows.map((row, index) => (
                      <div key={`${row.kind}-${index}`} className={`handler-debug-code-diff-row handler-debug-code-diff-row-${row.kind}`}>
                        <div className="handler-debug-code-diff-side">
                          <span className="handler-debug-code-diff-line">{row.beforeLineNumber ?? ""}</span>
                          <code>{row.kind === "added" ? "" : row.beforeLine ?? ""}</code>
                        </div>
                        <div className="handler-debug-code-diff-side">
                          <span className="handler-debug-code-diff-line">{row.afterLineNumber ?? ""}</span>
                          <code>{row.kind === "removed" ? "" : row.afterLine ?? ""}</code>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              ) : (
                <div className="ruleset-debug-empty">当前 handler 没有改动任何字段。</div>
              )
            ) : (
              <div className="ruleset-debug-empty">运行后会按 JSON 文本展示前后差异。</div>
            )
          ) : result ? (
            picDiffState && (picDiffState.coverChanged || picDiffState.posterChanged || picDiffState.sampleChanged) ? (
              <div className="handler-debug-pic-diff">
                <article className="handler-debug-pic-diff-section">
                  <div className="handler-debug-pic-diff-head">
                    <h4>Cover</h4>
                    <span className={`ruleset-debug-step-badge ${picDiffState.coverChanged ? "ruleset-debug-step-badge-hit" : ""}`}>{picDiffState.coverChanged ? "changed" : "unchanged"}</span>
                  </div>
                  <div className="handler-debug-pic-diff-compare">
                    <div className="handler-debug-pic-slot">
                      {result.before_meta.cover?.key ? <Image src={getAssetURL(result.before_meta.cover.key)} alt="before cover" width={220} height={320} unoptimized /> : <div className="ruleset-debug-empty">No Image</div>}
                    </div>
                    <div className="handler-debug-pic-slot">
                      {result.after_meta.cover?.key ? <Image src={getAssetURL(result.after_meta.cover.key)} alt="after cover" width={220} height={320} unoptimized /> : <div className="ruleset-debug-empty">No Image</div>}
                    </div>
                  </div>
                </article>

                <article className="handler-debug-pic-diff-section">
                  <div className="handler-debug-pic-diff-head">
                    <h4>Poster</h4>
                    <span className={`ruleset-debug-step-badge ${picDiffState.posterChanged ? "ruleset-debug-step-badge-hit" : ""}`}>{picDiffState.posterChanged ? "changed" : "unchanged"}</span>
                  </div>
                  <div className="handler-debug-pic-diff-compare">
                    <div className="handler-debug-pic-slot">
                      {result.before_meta.poster?.key ? <Image src={getAssetURL(result.before_meta.poster.key)} alt="before poster" width={220} height={320} unoptimized /> : <div className="ruleset-debug-empty">No Image</div>}
                    </div>
                    <div className="handler-debug-pic-slot">
                      {result.after_meta.poster?.key ? <Image src={getAssetURL(result.after_meta.poster.key)} alt="after poster" width={220} height={320} unoptimized /> : <div className="ruleset-debug-empty">No Image</div>}
                    </div>
                  </div>
                </article>

                <article className="handler-debug-pic-diff-section">
                  <div className="handler-debug-pic-diff-head">
                    <h4>Sample Images</h4>
                    <span className={`ruleset-debug-step-badge ${picDiffState.sampleChanged ? "ruleset-debug-step-badge-hit" : ""}`}>{picDiffState.sampleChanged ? "changed" : "unchanged"}</span>
                  </div>
                  <div className="handler-debug-pic-diff-compare">
                    <div className="handler-debug-pic-grid">
                      {(result.before_meta.sample_images ?? []).length ? (
                        result.before_meta.sample_images?.map((item) =>
                          item?.key ? <Image key={`before-${item.key}`} src={getAssetURL(item.key)} alt={item.name || "before sample"} width={220} height={140} unoptimized /> : null,
                        )
                      ) : (
                        <div className="ruleset-debug-empty">No Image</div>
                      )}
                    </div>
                    <div className="handler-debug-pic-grid">
                      {(result.after_meta.sample_images ?? []).length ? (
                        result.after_meta.sample_images?.map((item) =>
                          item?.key ? <Image key={`after-${item.key}`} src={getAssetURL(item.key)} alt={item.name || "after sample"} width={220} height={140} unoptimized /> : null,
                        )
                      ) : (
                        <div className="ruleset-debug-empty">No Image</div>
                      )}
                    </div>
                  </div>
                </article>
              </div>
            ) : (
              <div className="ruleset-debug-empty">当前 handler 没有图片资源差异。</div>
            )
          ) : (
            <div className="ruleset-debug-empty">运行后会按 Before / After 展示图片资源差异。</div>
          )}
          {result?.error ? <div className="ruleset-debug-error">{result.error}</div> : null}
        </section>
      </div>
    </div>
  );
}
