"use client";

import { useEffect, useMemo, useState } from "react";

import {
  buildJSONDiffRows,
  DEFAULT_META,
  type DiffRow,
  HANDLER_DEBUG_CHAIN_STORAGE_KEY,
  HANDLER_DEBUG_META_STORAGE_KEY,
} from "@/components/handler-debug-shell/utils";
import type { PicDiffState } from "@/components/handler-debug-shell/pic-diff-view";
import type { ResultTab } from "@/components/handler-debug-shell/result-panel";
import {
  debugHandler,
  getHandlerDebugHandlers,
  type HandlerDebugInstance,
  type HandlerDebugResult,
} from "@/lib/api";

// HANDLER_DEBUG_META_SESSION_KEY: searcher-debug 页面在跳转前把当前
// meta JSON 塞进 sessionStorage, handler-debug 用 query `?prefill=
// searcher` 识别是否需要接管. 两个页面的契约必须保持同步, 所以常量
// 集中在 hook 里.
const HANDLER_DEBUG_META_SESSION_KEY = "yamdc.debug.handler_meta";

function readInitialChainIDs(): string[] {
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
}

function readInitialMetaJSON(): string {
  if (typeof window === "undefined") {
    return JSON.stringify(DEFAULT_META, null, 2);
  }
  const stored = window.localStorage.getItem(HANDLER_DEBUG_META_STORAGE_KEY);
  return stored && stored.trim() ? stored : JSON.stringify(DEFAULT_META, null, 2);
}

export interface UseHandlerDebugStateResult {
  selectedChainHandlers: HandlerDebugInstance[];
  unselectedChainHandlers: HandlerDebugInstance[];
  metaJSON: string;
  setMetaJSON: (next: string) => void;
  addChainHandler: (handlerID: string) => void;
  removeChainHandler: (handlerID: string) => void;
  moveChainHandler: (sourceID: string, targetID: string) => void;
  activeTab: ResultTab;
  setActiveTab: (next: ResultTab) => void;
  result: HandlerDebugResult | null;
  diffRows: DiffRow[];
  picDiffState: PicDiffState | null;
  isRunning: boolean;
  handleRun: () => void;
  error: string;
  prefillMessage: string;
}

// useHandlerDebugState: 把 HandlerDebugShell 的全部业务状态 (8 个
// useState + 4 个 useEffect + 4 个 useMemo + 4 个 handler) 都装进
// 一个 hook, 组件只剩下 "把 hook 返回值分发给子组件" 的组装工作.
//
// 生命周期梳理:
//
// 1. 首次挂载: getHandlerDebugHandlers() 拿到可用 handler 列表;
//    如果当前 selectedChainHandlerIDs 为空 (第一次用 / localStorage
//    被清), 默认把全部 handler 加入链路, 否则保留用户已有的顺序.
//    这保证 "默认全选" 语义, 也保证 "用户改过之后不会被刷新覆盖".
//
// 2. searcher prefill: 带 ?prefill=searcher 的场景, 从
//    sessionStorage 里取 handler_meta 当作 metaJSON, 清 result +
//    error, 展示一条 "已从 Searcher 调试带入" 的提示, 然后消费掉
//    sessionStorage 里的 key (不要被下一次访问重复读到).
//
// 3. metaJSON 持久化: 每次 metaJSON 变化都写回 localStorage.
//    selectedChainHandlerIDs 同理. 都走独立 effect, 互不阻塞.
//
// 4. handleRun: 先本地校验 (链路非空 + JSON 合法 + number 字段非
//    空), 然后调 debugHandler chain mode; 成功切回 json tab,
//    失败清空 result 并把错误信息挂到 error. finally 里 setIsRunning
//    false, 保证按钮从 loading 状态恢复.
//
// diffRows / picDiffState 的依赖都只有 result, 所以可以放心记忆化 --
// meta 输入变化但没重新 run, 对比结果不会刷. selectedChain-/
// unselectedChainHandlers 要同时跟踪 handlers 和 selectedChain-
// HandlerIDs 两个源, 否则切列表之后显示会滞后.
export function useHandlerDebugState(): UseHandlerDebugStateResult {
  const [handlers, setHandlers] = useState<HandlerDebugInstance[]>([]);
  const [selectedChainHandlerIDs, setSelectedChainHandlerIDs] = useState<string[]>(readInitialChainIDs);
  const [metaJSON, setMetaJSON] = useState<string>(readInitialMetaJSON);
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
    const stored = window.sessionStorage.getItem(HANDLER_DEBUG_META_SESSION_KEY);
    if (!stored) {
      return;
    }
    setMetaJSON(stored);
    setResult(null);
    setError("");
    setPrefillMessage("已从 Searcher 调试带入当前 Meta JSON。");
    window.sessionStorage.removeItem(HANDLER_DEBUG_META_SESSION_KEY);
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

  const diffRows = useMemo<DiffRow[]>(() => {
    if (!result) {
      return [];
    }
    return buildJSONDiffRows(
      JSON.stringify(result.before_meta, null, 2),
      JSON.stringify(result.after_meta, null, 2),
    );
  }, [result]);

  const picDiffState = useMemo<PicDiffState | null>(() => {
    if (!result) {
      return null;
    }
    return {
      coverChanged: JSON.stringify(result.before_meta.cover ?? null) !== JSON.stringify(result.after_meta.cover ?? null),
      posterChanged: JSON.stringify(result.before_meta.poster ?? null) !== JSON.stringify(result.after_meta.poster ?? null),
      sampleChanged:
        JSON.stringify(result.before_meta.sample_images ?? []) !== JSON.stringify(result.after_meta.sample_images ?? []),
    };
  }, [result]);

  const selectedChainHandlers = useMemo(() => {
    const map = new Map(handlers.map((item) => [item.id, item]));
    return selectedChainHandlerIDs
      .map((id) => map.get(id))
      .filter(Boolean) as HandlerDebugInstance[];
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

  return {
    selectedChainHandlers,
    unselectedChainHandlers,
    metaJSON,
    setMetaJSON,
    addChainHandler,
    removeChainHandler,
    moveChainHandler,
    activeTab,
    setActiveTab,
    result,
    diffRows,
    picDiffState,
    isRunning,
    handleRun,
    error,
    prefillMessage,
  };
}
