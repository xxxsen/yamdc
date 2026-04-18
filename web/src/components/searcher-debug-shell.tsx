"use client";

import { Search, WandSparkles, Copy, ArrowRight } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  debugSearcher,
  getSearcherDebugPlugins,
  type SearcherDebugPluginCollection,
  type SearcherDebugResult,
} from "@/lib/api";

const DEFAULT_INPUT = "MOVIE-12345";
const SEARCHER_DEBUG_INPUT_STORAGE_KEY = "yamdc.debug.searcher.input";
const SEARCHER_DEBUG_PLUGIN_STORAGE_KEY = "yamdc.debug.searcher.plugin";
const SEARCHER_DEBUG_USE_CLEANER_STORAGE_KEY = "yamdc.debug.searcher.use_cleaner";

export function SearcherDebugShell() {
  const router = useRouter();
  const [input, setInput] = useState(() => {
    if (typeof window === "undefined") {
      return DEFAULT_INPUT;
    }
    const stored = window.localStorage.getItem(SEARCHER_DEBUG_INPUT_STORAGE_KEY);
    return stored && stored.trim() ? stored : DEFAULT_INPUT;
  });
  const [selectedPlugin, setSelectedPlugin] = useState(() => {
    if (typeof window === "undefined") {
      return "";
    }
    return window.localStorage.getItem(SEARCHER_DEBUG_PLUGIN_STORAGE_KEY) ?? "";
  });
  const [useCleaner, setUseCleaner] = useState(() => {
    if (typeof window === "undefined") {
      return true;
    }
    const stored = window.localStorage.getItem(SEARCHER_DEBUG_USE_CLEANER_STORAGE_KEY);
    return stored === "true" || stored === "false" ? stored === "true" : true;
  });
  const [pluginCatalog, setPluginCatalog] = useState<SearcherDebugPluginCollection | null>(null);
  const [result, setResult] = useState<SearcherDebugResult | null>(null);
  const [error, setError] = useState("");
  const [metaActionMessage, setMetaActionMessage] = useState("");
  const [isRunning, setIsRunning] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const next = await getSearcherDebugPlugins();
        setPluginCatalog(next);
      } catch {
        // keep shell usable even if catalog request fails
      }
    })();
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(SEARCHER_DEBUG_INPUT_STORAGE_KEY, input);
  }, [input]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(SEARCHER_DEBUG_PLUGIN_STORAGE_KEY, selectedPlugin);
  }, [selectedPlugin]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(SEARCHER_DEBUG_USE_CLEANER_STORAGE_KEY, String(useCleaner));
  }, [useCleaner]);

  const resultMetaJSON = useMemo(() => (result?.meta ? JSON.stringify(result.meta, null, 2) : ""), [result]);

  useEffect(() => {
    if (!pluginCatalog?.available.length) {
      return;
    }
    if (selectedPlugin && !pluginCatalog.available.includes(selectedPlugin)) {
      setSelectedPlugin("");
    }
  }, [pluginCatalog, selectedPlugin]);

  const handleRun = () => {
    const nextInput = input.trim();
    if (!nextInput) {
      setError("请输入待检索的影片 ID。");
      setResult(null);
      return;
    }
    setIsRunning(true);
    void (async () => {
      try {
        const next = await debugSearcher(nextInput, selectedPlugin, useCleaner);
        setResult(next);
        setError("");
      } catch (nextError) {
        setResult(null);
        setError(nextError instanceof Error ? nextError.message : "插件检索测试失败");
      } finally {
        setIsRunning(false);
      }
    })();
  };

  const handleCopyMeta = async () => {
    if (!resultMetaJSON) {
      return;
    }
    try {
      await navigator.clipboard.writeText(resultMetaJSON);
      setMetaActionMessage("Meta JSON 已复制。");
    } catch {
      setMetaActionMessage("复制失败，请手动展开下方 JSON。");
    }
  };

  const handleOpenHandlerDebug = () => {
    if (!resultMetaJSON) {
      return;
    }
    if (typeof window !== "undefined") {
      window.sessionStorage.setItem("yamdc.debug.handler_meta", resultMetaJSON);
    }
    router.push("/debug/handler?prefill=searcher");
  };

  return (
    <div className="searcher-debug-page">
      <section className="panel searcher-debug-hero">
        <div className="searcher-debug-hero-copy">
          <span className="ruleset-debug-eyebrow">
            <WandSparkles size={14} />
            Searcher 调试
          </span>
          <h2>插件检索测试</h2>
          <p>输入一个影片 ID，查看它经过 `movieidcleaner` 后使用了哪些插件，以及每个插件在哪个阶段返回结果或失败。</p>
        </div>

        <div className="searcher-debug-controls">
          <label className="ruleset-debug-label" htmlFor="searcher-debug-input">
            待检索影片 ID
          </label>
          <div className="ruleset-debug-input-row">
            <input
              id="searcher-debug-input"
              className="input ruleset-debug-input"
              value={input}
              onChange={(event) => setInput(event.target.value)}
              placeholder="例如：MOVIE-12345"
            />
            <Button
              variant="primary"
              className="ruleset-debug-run-button"
              onClick={handleRun}
              disabled={isRunning}
              loading={isRunning}
              leftIcon={<Search size={16} />}
            >
              <span>{isRunning ? "检索中..." : "开始检索"}</span>
            </Button>
          </div>

          <label className="ruleset-debug-label" htmlFor="searcher-debug-plugin">
            指定插件
          </label>
          <select
            id="searcher-debug-plugin"
            className="input ruleset-debug-input"
            value={selectedPlugin}
            onChange={(event) => setSelectedPlugin(event.target.value)}
          >
            <option value="">使用当前配置链</option>
            {pluginCatalog?.available.map((plugin) => (
              <option key={plugin} value={plugin}>
                {plugin}
              </option>
            ))}
          </select>

          <label className="searcher-debug-switch">
            <input type="checkbox" checked={useCleaner} onChange={(event) => setUseCleaner(event.target.checked)} />
            <span>启用影片 ID 清洗</span>
          </label>

          {error ? <div className="ruleset-debug-error">{error}</div> : null}
        </div>
      </section>

      <div className="searcher-debug-grid">
        <section className="panel ruleset-debug-summary-panel">
          <div className="ruleset-debug-panel-head">
            <h3>检索摘要</h3>
            {result ? <span className={`ruleset-debug-status ${result.found ? "" : "ruleset-debug-status-no_match"}`}>{result.found ? "found" : "not_found"}</span> : null}
          </div>
          {result ? (
            <div className="ruleset-debug-summary">
              <div className="searcher-debug-summary-body">
                <div className="ruleset-debug-summary-row">
                  <span>最终影片 ID</span>
                  <strong>{result.number_id || "-"}</strong>
                </div>
                <div className="ruleset-debug-summary-row">
                  <span>清洗结果</span>
                  <strong>{result.cleaner_result?.normalized || "-"}</strong>
                </div>
                <div className="ruleset-debug-summary-row">
                  <span>插件链</span>
                  <strong>{result.used_plugins?.length ? result.used_plugins.join(", ") : "-"}</strong>
                </div>
                <div className="ruleset-debug-summary-row">
                  <span>命中插件</span>
                  <strong>{result.matched_plugin || "-"}</strong>
                </div>
                <div className="ruleset-debug-summary-row">
                  <span>分类 / 附加标记</span>
                  <strong>
                    {result.category || "-"} / {result.uncensor ? "true" : "false"}
                  </strong>
                </div>
                <div className="ruleset-debug-summary-row">
                  <span>标题</span>
                  <strong>{result.meta?.title || "-"}</strong>
                </div>
                <div className="ruleset-debug-summary-row">
                  <span>来源</span>
                  <strong>{result.meta?.ext_info?.scrape_info.source || "-"}</strong>
                </div>
              </div>
              {result.meta ? (
                <div className="searcher-debug-summary-footer">
                  <div className="searcher-debug-summary-actions">
                    <Button
                      variant="primary"
                      onClick={() => void handleCopyMeta()}
                      leftIcon={<Copy size={16} />}
                    >
                      <span>复制 Meta JSON</span>
                    </Button>
                    <Button
                      variant="primary"
                      onClick={handleOpenHandlerDebug}
                      leftIcon={<ArrowRight size={16} />}
                    >
                      <span>发送到 Handler 测试</span>
                    </Button>
                  </div>
                  {metaActionMessage ? <div className="handler-debug-message">{metaActionMessage}</div> : null}
                </div>
              ) : null}
            </div>
          ) : (
            <div className="ruleset-debug-empty">运行后会展示 cleaner 结果、插件链顺序、最终命中插件和抓到的标题。</div>
          )}
        </section>

        <section className="panel searcher-debug-results-panel">
          <div className="ruleset-debug-panel-head">
            <h3>插件执行链路</h3>
            <span>{result ? `${result.plugin_results?.length ?? 0} plugins` : "等待运行"}</span>
          </div>
          {result ? (
            <div className="searcher-debug-plugin-results">
              {(result.plugin_results ?? []).map((pluginResult) => (
                <article key={pluginResult.plugin} className={`searcher-debug-plugin-card ${pluginResult.found ? "searcher-debug-plugin-card-hit" : ""}`}>
                  <div className="searcher-debug-plugin-head">
                    <div>
                      <strong>{pluginResult.plugin}</strong>
                      <span>{pluginResult.found ? "命中结果" : pluginResult.error || "未命中"}</span>
                    </div>
                    <span className={`ruleset-debug-step-badge ${pluginResult.found ? "ruleset-debug-step-badge-hit" : ""}`}>
                      {pluginResult.found ? "hit" : "miss"}
                    </span>
                  </div>

                  {pluginResult.meta ? (
                    <>
                      <div className="searcher-debug-meta">
                        <div className="searcher-debug-meta-row">
                          <span>标题</span>
                          <strong>{pluginResult.meta.title || "-"}</strong>
                        </div>
                        <div className="searcher-debug-meta-row">
                          <span>影片 ID</span>
                          <strong>{pluginResult.meta.number || "-"}</strong>
                        </div>
                      </div>
                      <details className="searcher-debug-json-block">
                        <summary>查看原始 Meta JSON</summary>
                        <pre className="searcher-debug-json">{JSON.stringify(pluginResult.meta, null, 2)}</pre>
                      </details>
                    </>
                  ) : null}

                  <div className="searcher-debug-step-list">
                    {(pluginResult.steps ?? []).map((step, index) => (
                      <div key={`${pluginResult.plugin}-${step.stage}-${index}`} className="searcher-debug-step">
                        <div className="searcher-debug-step-head">
                          <span className="ruleset-debug-step-stage">{step.stage}</span>
                          <span className={`ruleset-debug-step-badge ${step.ok ? "ruleset-debug-step-badge-hit" : ""}`}>{step.ok ? "ok" : "fail"}</span>
                        </div>
                        <p className="ruleset-debug-step-summary">{step.message}</p>
                        {step.url ? <code className="searcher-debug-code">{step.url}</code> : null}
                        {step.status_code ? <p className="searcher-debug-inline-meta">HTTP {step.status_code}</p> : null}
                      </div>
                    ))}
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <div className="ruleset-debug-empty">这里会按插件分组展示 precheck、构造请求、请求、响应预检、解码、元数据校验等阶段。</div>
          )}
        </section>
      </div>
    </div>
  );
}
