// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { HandlerDebugInstance, HandlerDebugResult } from "@/lib/api";

import { useHandlerDebugState } from "../use-handler-debug-state";
import {
  DEFAULT_META,
  HANDLER_DEBUG_CHAIN_STORAGE_KEY,
  HANDLER_DEBUG_META_STORAGE_KEY,
} from "../utils";

// useHandlerDebugState: handler-debug 页面主状态容器. 覆盖:
// - 默认 handler 全选 / 用户顺序保留
// - searcher prefill 从 sessionStorage 读 meta 并消费 key
// - metaJSON / chainIDs 写回 localStorage
// - handleRun 三种校验失败路径 + 成功路径 + API 失败
// - addChainHandler / removeChainHandler / moveChainHandler 边界

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getHandlerDebugHandlers: vi.fn(),
    debugHandler: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockGetHandlers = vi.mocked(api.getHandlerDebugHandlers);
const mockDebugHandler = vi.mocked(api.debugHandler);

function makeResult(overrides: Partial<HandlerDebugResult> = {}): HandlerDebugResult {
  return {
    mode: "chain",
    handler_id: "",
    handler_name: "",
    number_id: "ABC-123",
    category: "",
    unrated: false,
    before_meta: { number: "ABC-123" } as HandlerDebugResult["before_meta"],
    after_meta: { number: "ABC-123", title: "new" } as HandlerDebugResult["after_meta"],
    error: "",
    steps: null,
    ...overrides,
  };
}

async function flushAsync(ticks = 6) {
  await act(async () => {
    for (let i = 0; i < ticks; i += 1) {
      await Promise.resolve();
    }
  });
}

const HANDLERS: HandlerDebugInstance[] = [
  { id: "h1", name: "Handler 1" },
  { id: "h2", name: "Handler 2" },
  { id: "h3", name: "Handler 3" },
];

beforeEach(() => {
  mockGetHandlers.mockReset();
  mockDebugHandler.mockReset();
  window.localStorage.clear();
  window.sessionStorage.clear();
  window.history.replaceState(null, "", "/");
});

afterEach(() => {
  vi.useRealTimers();
});

describe("初始化: handler 默认全选", () => {
  it("空 localStorage 时默认把全部 handler 加入链路", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.selectedChainHandlers.map((h) => h.id)).toEqual(["h1", "h2", "h3"]);
    expect(result.current.unselectedChainHandlers).toEqual([]);
  });

  it("已有 localStorage 顺序会被保留, 不被覆盖", async () => {
    window.localStorage.setItem(HANDLER_DEBUG_CHAIN_STORAGE_KEY, JSON.stringify(["h2", "h1"]));
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.selectedChainHandlers.map((h) => h.id)).toEqual(["h2", "h1"]);
    expect(result.current.unselectedChainHandlers.map((h) => h.id)).toEqual(["h3"]);
  });

  it("localStorage 内容非数组或损坏时回退为空, 按默认全选走", async () => {
    window.localStorage.setItem(HANDLER_DEBUG_CHAIN_STORAGE_KEY, "{not-json");
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.selectedChainHandlers.map((h) => h.id)).toEqual(["h1", "h2", "h3"]);
  });

  it("getHandlerDebugHandlers 抛错时 shell 保持可见, handlers 为空", async () => {
    mockGetHandlers.mockRejectedValue(new Error("network"));
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.selectedChainHandlers).toEqual([]);
    expect(result.current.unselectedChainHandlers).toEqual([]);
  });
});

describe("初始化: metaJSON", () => {
  it("空 localStorage 使用 DEFAULT_META 的 pretty JSON", async () => {
    mockGetHandlers.mockResolvedValue([]);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.metaJSON).toBe(JSON.stringify(DEFAULT_META, null, 2));
  });

  it("已有 metaJSON 会被保留", async () => {
    window.localStorage.setItem(HANDLER_DEBUG_META_STORAGE_KEY, '{"number":"XYZ-777"}');
    mockGetHandlers.mockResolvedValue([]);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.metaJSON).toBe('{"number":"XYZ-777"}');
  });
});

describe("searcher prefill", () => {
  it("URL 带 ?prefill=searcher + sessionStorage 有值时接管 metaJSON", async () => {
    window.history.replaceState(null, "", "/?prefill=searcher");
    window.sessionStorage.setItem("yamdc.debug.handler_meta", '{"number":"PRE-001"}');
    mockGetHandlers.mockResolvedValue([]);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.metaJSON).toBe('{"number":"PRE-001"}');
    expect(result.current.prefillMessage).toContain("已从 Searcher");
    expect(window.sessionStorage.getItem("yamdc.debug.handler_meta")).toBeNull();
  });

  it("URL 没有 prefill 时不接管, 也不消费 sessionStorage", async () => {
    window.sessionStorage.setItem("yamdc.debug.handler_meta", '{"number":"PRE-002"}');
    mockGetHandlers.mockResolvedValue([]);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.metaJSON).toBe(JSON.stringify(DEFAULT_META, null, 2));
    expect(result.current.prefillMessage).toBe("");
    expect(window.sessionStorage.getItem("yamdc.debug.handler_meta")).toBe('{"number":"PRE-002"}');
  });

  it("有 prefill 但 sessionStorage 为空时, 不刷 metaJSON 也不提示", async () => {
    window.history.replaceState(null, "", "/?prefill=searcher");
    mockGetHandlers.mockResolvedValue([]);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    expect(result.current.prefillMessage).toBe("");
    expect(result.current.metaJSON).toBe(JSON.stringify(DEFAULT_META, null, 2));
  });
});

describe("metaJSON / chainIDs 持久化", () => {
  it("setMetaJSON 会写回 localStorage", async () => {
    mockGetHandlers.mockResolvedValue([]);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.setMetaJSON('{"number":"NEW-999"}');
    });
    expect(window.localStorage.getItem(HANDLER_DEBUG_META_STORAGE_KEY)).toBe('{"number":"NEW-999"}');
  });

  it("addChainHandler / removeChainHandler 会写回 localStorage", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    // 先清空, 然后加一个
    act(() => {
      result.current.removeChainHandler("h1");
      result.current.removeChainHandler("h2");
      result.current.removeChainHandler("h3");
    });
    act(() => {
      result.current.addChainHandler("h2");
    });
    expect(JSON.parse(window.localStorage.getItem(HANDLER_DEBUG_CHAIN_STORAGE_KEY)!)).toEqual(["h2"]);
  });
});

describe("addChainHandler / removeChainHandler / moveChainHandler", () => {
  it("addChainHandler 对已存在 id 幂等", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.addChainHandler("h1");
    });
    expect(result.current.selectedChainHandlers.map((h) => h.id)).toEqual(["h1", "h2", "h3"]);
  });

  it("removeChainHandler 对不存在 id 是 no-op", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.removeChainHandler("not-exist");
    });
    expect(result.current.selectedChainHandlers.map((h) => h.id)).toEqual(["h1", "h2", "h3"]);
  });

  it("moveChainHandler 把 source 挪到 target 位置", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.moveChainHandler("h3", "h1");
    });
    expect(result.current.selectedChainHandlers.map((h) => h.id)).toEqual(["h3", "h1", "h2"]);
  });

  it("moveChainHandler 对空 source / 相同 source 和 target 是 no-op", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.moveChainHandler("", "h1");
      result.current.moveChainHandler("h1", "h1");
    });
    expect(result.current.selectedChainHandlers.map((h) => h.id)).toEqual(["h1", "h2", "h3"]);
  });

  it("moveChainHandler 对未选中的 source/target 保持原状", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.moveChainHandler("ghost", "h1");
    });
    expect(result.current.selectedChainHandlers.map((h) => h.id)).toEqual(["h1", "h2", "h3"]);
  });
});

describe("handleRun 校验路径", () => {
  it("链路为空时报错, 不调 API", async () => {
    mockGetHandlers.mockResolvedValue([]);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.handleRun();
    });
    expect(result.current.error).toBe("请至少选择一个 handler。");
    expect(mockDebugHandler).not.toHaveBeenCalled();
  });

  it("metaJSON 非法 JSON 时报错", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.setMetaJSON("{not json");
    });
    act(() => {
      result.current.handleRun();
    });
    expect(result.current.error).toBe("Meta JSON 格式无效。");
    expect(mockDebugHandler).not.toHaveBeenCalled();
  });

  it("metaJSON 缺 number 字段时报错", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.setMetaJSON('{"title":"only-title"}');
    });
    act(() => {
      result.current.handleRun();
    });
    expect(result.current.error).toBe("Meta JSON 里必须包含 number 字段。");
    expect(mockDebugHandler).not.toHaveBeenCalled();
  });

  it("metaJSON number 为空字符串时报错", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.setMetaJSON('{"number":"   "}');
    });
    act(() => {
      result.current.handleRun();
    });
    expect(result.current.error).toBe("Meta JSON 里必须包含 number 字段。");
  });
});

describe("handleRun 成功路径", () => {
  it("成功: 设置 result + 清空 error + 切回 json tab", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    mockDebugHandler.mockResolvedValue(makeResult());
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.setActiveTab("pic");
    });
    expect(result.current.activeTab).toBe("pic");

    act(() => {
      result.current.handleRun();
    });
    expect(result.current.isRunning).toBe(true);
    await flushAsync();
    expect(result.current.isRunning).toBe(false);
    expect(result.current.result).not.toBeNull();
    expect(result.current.error).toBe("");
    expect(result.current.activeTab).toBe("json");
    expect(mockDebugHandler).toHaveBeenCalledWith(
      expect.objectContaining({
        mode: "chain",
        handler_ids: ["h1", "h2", "h3"],
      }),
    );
  });

  it("成功后 diffRows / picDiffState 会基于 result 派生", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    mockDebugHandler.mockResolvedValue(
      makeResult({
        before_meta: { number: "X", cover: null } as HandlerDebugResult["before_meta"],
        after_meta: {
          number: "X",
          cover: { key: "k", url: "u" },
        } as HandlerDebugResult["after_meta"],
      }),
    );
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.handleRun();
    });
    await flushAsync();
    expect(result.current.diffRows.length).toBeGreaterThan(0);
    expect(result.current.picDiffState?.coverChanged).toBe(true);
    expect(result.current.picDiffState?.posterChanged).toBe(false);
  });
});

describe("handleRun 失败路径", () => {
  it("debugHandler 抛错时清空 result 并把 message 挂到 error", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    mockDebugHandler.mockRejectedValue(new Error("backend down"));
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.handleRun();
    });
    await flushAsync();
    expect(result.current.isRunning).toBe(false);
    expect(result.current.error).toBe("backend down");
    expect(result.current.result).toBeNull();
  });

  it("debugHandler 抛非 Error 对象时使用兜底文案", async () => {
    mockGetHandlers.mockResolvedValue(HANDLERS);
    mockDebugHandler.mockRejectedValue("raw string");
    const { result } = renderHook(() => useHandlerDebugState());
    await flushAsync();
    act(() => {
      result.current.handleRun();
    });
    await flushAsync();
    expect(result.current.error).toBe("Handler 测试失败");
  });
});
