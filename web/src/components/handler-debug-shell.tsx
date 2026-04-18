"use client";

import { Play } from "lucide-react";
import Image from "next/image";
import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";

import { Button } from "@/components/ui/button";
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
const HANDLER_DEBUG_META_STORAGE_KEY = "yamdc.debug.handler.meta";
const HANDLER_DEBUG_CHAIN_STORAGE_KEY = "yamdc.debug.handler.chain";

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

function ChainName({ name }: { name: string }) {
  const wrapRef = useRef<HTMLSpanElement | null>(null);
  const textRef = useRef<HTMLSpanElement | null>(null);
  const [distance, setDistance] = useState(0);

  useEffect(() => {
    const wrap = wrapRef.current;
    const text = textRef.current;
    if (!wrap || !text) {
      return;
    }
    const nextDistance = Math.max(0, text.scrollWidth - wrap.clientWidth);
    setDistance(nextDistance);
  }, [name]);

  return (
    <span className="handler-debug-chain-name-wrap" ref={wrapRef} title={name}>
      <span
        ref={textRef}
        className={`handler-debug-chain-name ${distance > 0 ? "handler-debug-chain-name-scroll" : ""}`}
        style={distance > 0 ? ({ "--handler-chain-scroll-distance": `${distance}px` } as CSSProperties) : undefined}
      >
        {name}
      </span>
    </span>
  );
}

export function HandlerDebugShell() {
  const [handlers, setHandlers] = useState<HandlerDebugInstance[]>([]);
  const [selectedChainHandlerIDs, setSelectedChainHandlerIDs] = useState<string[]>(() => {
    if (typeof window === "undefined") {
      return [];
    }
    const stored = window.localStorage.getItem(HANDLER_DEBUG_CHAIN_STORAGE_KEY);
    if (!stored) {
      return [];
    }
    try {
      const next = JSON.parse(stored) as string[];
      return Array.isArray(next) ? next.filter((item) => typeof item === "string" && item.trim() !== "") : [];
    } catch {
      return [];
    }
  });
  const [metaJSON, setMetaJSON] = useState(() => {
    if (typeof window === "undefined") {
      return JSON.stringify(DEFAULT_META, null, 2);
    }
    const stored = window.localStorage.getItem(HANDLER_DEBUG_META_STORAGE_KEY);
    return stored && stored.trim() ? stored : JSON.stringify(DEFAULT_META, null, 2);
  });
  const [result, setResult] = useState<HandlerDebugResult | null>(null);
  const [error, setError] = useState("");
  const [prefillMessage, setPrefillMessage] = useState("");
  const [activeTab, setActiveTab] = useState<"json" | "pic" | "chain">("json");
  const [isRunning, setIsRunning] = useState(false);
  const [draggingHandlerID, setDraggingHandlerID] = useState<string | null>(null);

  useEffect(() => {
    void (async () => {
      try {
        const next = await getHandlerDebugHandlers();
        setHandlers(next);
        setSelectedChainHandlerIDs((current) => (current.length ? current : next.map((item) => item.id)));
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

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(HANDLER_DEBUG_META_STORAGE_KEY, metaJSON);
  }, [metaJSON]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(HANDLER_DEBUG_CHAIN_STORAGE_KEY, JSON.stringify(selectedChainHandlerIDs));
  }, [selectedChainHandlerIDs]);

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

  const selectedChainHandlers = useMemo(() => {
    const map = new Map(handlers.map((item) => [item.id, item]));
    return selectedChainHandlerIDs.map((id) => map.get(id)).filter(Boolean) as HandlerDebugInstance[];
  }, [handlers, selectedChainHandlerIDs]);

  const unselectedChainHandlers = useMemo(() => {
    const selected = new Set(selectedChainHandlerIDs);
    return handlers.filter((item) => !selected.has(item.id));
  }, [handlers, selectedChainHandlerIDs]);

  const handleRun = () => {
    if (selectedChainHandlerIDs.length === 0) {
      setError("请至少选择一个 handler。");
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
          mode: "chain",
          handler_id: "",
          handler_ids: selectedChainHandlerIDs,
          meta: parsedMeta as never,
        });
        setResult(next);
        setActiveTab("json");
        setError("");
      } catch (nextError) {
        setResult(null);
        setError(nextError instanceof Error ? nextError.message : "Handler 测试失败");
      } finally {
        setIsRunning(false);
      }
    })();
  };

  const addChainHandler = (handlerID: string) => {
    setSelectedChainHandlerIDs((current) => (current.includes(handlerID) ? current : [...current, handlerID]));
  };

  const removeChainHandler = (handlerID: string) => {
    setSelectedChainHandlerIDs((current) => current.filter((item) => item !== handlerID));
  };

  const moveChainHandler = (sourceID: string, targetID: string) => {
    if (!sourceID || sourceID === targetID) {
      return;
    }
    setSelectedChainHandlerIDs((current) => {
      const next = [...current];
      const sourceIndex = next.indexOf(sourceID);
      const targetIndex = next.indexOf(targetID);
      if (sourceIndex === -1 || targetIndex === -1) {
        return current;
      }
      const [item] = next.splice(sourceIndex, 1);
      next.splice(targetIndex, 0, item);
      return next;
    });
  };

  return (
    <div className="handler-debug-page">
      <section className="panel handler-debug-hero">
        <div className="handler-debug-copy">
          <span className="ruleset-debug-eyebrow">
            <Play size={14} />
            Handler 测试
          </span>
          <div className="handler-debug-title-row">
            <h2>Handler 链测试</h2>
            <Button
              variant="primary"
              className="ruleset-debug-run-button handler-debug-run-inline"
              onClick={handleRun}
              disabled={isRunning}
              loading={isRunning}
              leftIcon={<Play size={16} />}
            >
              <span>{isRunning ? "执行中..." : "运行"}</span>
            </Button>
          </div>
        </div>

        <div className="handler-debug-controls">
          <div className="handler-debug-chain-top">
            <div className="handler-debug-chain-workspace">
              <div className="handler-debug-chain-column">
                <div className="handler-debug-chain-head">
                  <strong>已选 Handler</strong>
                  <span className="handler-debug-chain-count">{selectedChainHandlers.length}</span>
                </div>
                <div className="handler-debug-chain-list">
                  {selectedChainHandlers.map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      className="handler-debug-chain-card handler-debug-chain-card-selected"
                      onClick={() => removeChainHandler(item.id)}
                      draggable
                      onDragStart={() => setDraggingHandlerID(item.id)}
                      onDragEnd={() => setDraggingHandlerID(null)}
                      onDragOver={(event) => event.preventDefault()}
                      onDrop={(event) => {
                        event.preventDefault();
                        if (draggingHandlerID) {
                          moveChainHandler(draggingHandlerID, item.id);
                        }
                        setDraggingHandlerID(null);
                      }}
                    >
                      <span className="handler-debug-chain-grip">::</span>
                      <ChainName name={item.name} />
                    </button>
                  ))}
                  {selectedChainHandlers.length === 0 ? <div className="ruleset-debug-empty">点击右侧未选中的 handler 加入链路。</div> : null}
                </div>
              </div>
              <div className="handler-debug-chain-column">
                <div className="handler-debug-chain-head">
                  <strong>未选 Handler</strong>
                  <span className="handler-debug-chain-count">{unselectedChainHandlers.length}</span>
                </div>
                <div className="handler-debug-chain-list">
                  {unselectedChainHandlers.map((item) => (
                    <button key={item.id} type="button" className="handler-debug-chain-card" onClick={() => addChainHandler(item.id)}>
                      <ChainName name={item.name} />
                    </button>
                  ))}
                  {unselectedChainHandlers.length === 0 ? <div className="ruleset-debug-empty">当前全部 handler 都已加入链路。</div> : null}
                </div>
              </div>
            </div>
            <div className="handler-debug-chain-meta">
              <div className="handler-debug-chain-head">
                <strong>Meta JSON</strong>
              </div>
              <textarea className="input handler-debug-textarea handler-debug-textarea-compact" value={metaJSON} onChange={(event) => setMetaJSON(event.target.value)} />
            </div>
          </div>

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
              <button type="button" className={`handler-debug-tab ${activeTab === "chain" ? "handler-debug-tab-active" : ""}`} onClick={() => setActiveTab("chain")}>
                Chain Result
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
          ) : activeTab === "pic" ? (
            result ? (
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
                          item.key ? <Image key={`before-${item.key}`} src={getAssetURL(item.key)} alt={item.name || "before sample"} width={220} height={140} unoptimized /> : null,
                        )
                      ) : (
                        <div className="ruleset-debug-empty">No Image</div>
                      )}
                    </div>
                    <div className="handler-debug-pic-grid">
                      {(result.after_meta.sample_images ?? []).length ? (
                        result.after_meta.sample_images?.map((item) =>
                          item.key ? <Image key={`after-${item.key}`} src={getAssetURL(item.key)} alt={item.name || "after sample"} width={220} height={140} unoptimized /> : null,
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
            )
          ) : result?.steps.length ? (
            <div className="handler-debug-step-list">
              {result.steps.map((step, index) => (
                <article key={`${step.handler_id}-${index}`} className={`handler-debug-step-card ${step.error ? "handler-debug-step-card-error" : ""}`}>
                  <div className="handler-debug-step-head">
                    <strong>{step.handler_name}</strong>
                    <span className={`ruleset-debug-step-badge ${step.error ? "" : "ruleset-debug-step-badge-hit"}`}>{step.error ? "error" : "ok"}</span>
                  </div>
                  {step.error ? <p className="ruleset-debug-step-summary">{step.error}</p> : null}
                </article>
              ))}
            </div>
          ) : (
            <div className="ruleset-debug-empty">运行后会展示链式执行的每一步结果。</div>
          )}
        </section>
      </div>
    </div>
  );
}
