"use client";

import { Play } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { ChainPicker } from "@/components/handler-debug-shell/chain-picker";
import { ResultPanel, type ResultTab } from "@/components/handler-debug-shell/result-panel";
import {
  buildJSONDiffRows,
  DEFAULT_META,
  HANDLER_DEBUG_CHAIN_STORAGE_KEY,
  HANDLER_DEBUG_META_STORAGE_KEY,
} from "@/components/handler-debug-shell/utils";
import { Button } from "@/components/ui/button";
import {
  debugHandler,
  getHandlerDebugHandlers,
  type HandlerDebugInstance,
  type HandlerDebugResult,
} from "@/lib/api";

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
  const [activeTab, setActiveTab] = useState<ResultTab>("json");
  const [isRunning, setIsRunning] = useState(false);

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
          <ChainPicker
            selectedChainHandlers={selectedChainHandlers}
            unselectedChainHandlers={unselectedChainHandlers}
            metaJSON={metaJSON}
            onAdd={addChainHandler}
            onRemove={removeChainHandler}
            onMove={moveChainHandler}
            onMetaJSONChange={setMetaJSON}
          />

          {prefillMessage ? <div className="handler-debug-message">{prefillMessage}</div> : null}
          {error ? <div className="ruleset-debug-error">{error}</div> : null}
        </div>
      </section>

      <ResultPanel
        activeTab={activeTab}
        onTabChange={setActiveTab}
        result={result}
        diffRows={diffRows}
        picDiffState={picDiffState}
      />
    </div>
  );
}
