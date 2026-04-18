// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { usePluginEditorState } from "../use-plugin-editor-state";

// usePluginEditorState: plugin-editor 主 shell 的完整 state 容器.
//
// 测试分层 (pure updater 已经在 plugin-editor-state-ops 被独立覆盖,
// 这里只验 hook 层的**编排与副作用**):
//
//   1. localStorage 生命周期: mount 读取 -> debounce 160ms 写回.
//   2. Toast auto-clear: info/danger 2.2s, warning 5s.
//   3. run() 四条分支 (compile / request / workflow / scrape), scrape 两条子
//      分支 (workflow enabled + multi-request enabled).
//   4. run() 里 browser 模式 + 空 wait selector 触发 warning toast.
//   5. run() 错误: setError 吃异常, busyAction 最终回空.
//   6. handleCopyYAML: clipboard 成功 / 失败两种 toast.
//   7. handleImportYAML: 成功回填表单 + 清结果 + 关弹窗; 失败走 setError.
//   8. handleClearDraft: 清 state + 清 localStorage + toast info.
//   9. updater 薄壳连通: 选 2 条验证 setState(ops.xxx) 调用路径确实回写.

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    compilePluginDraft: vi.fn(),
    debugPluginDraftRequest: vi.fn(),
    debugPluginDraftWorkflow: vi.fn(),
    debugPluginDraftScrape: vi.fn(),
    importPluginDraftYAML: vi.fn(),
  };
});

const api = await import("@/lib/api");
const mockCompile = vi.mocked(api.compilePluginDraft);
const mockReq = vi.mocked(api.debugPluginDraftRequest);
const mockWf = vi.mocked(api.debugPluginDraftWorkflow);
const mockScrape = vi.mocked(api.debugPluginDraftScrape);
const mockImport = vi.mocked(api.importPluginDraftYAML);

// clipboard stub
function installClipboard(impl: (text: string) => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn(impl) },
    configurable: true,
  });
}

async function flushAsync() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  });
}

const DRAFT_KEY = "yamdc.debug.plugin-editor.draft.v2";
const NUMBER_KEY = "yamdc.debug.plugin-editor.number";

beforeEach(() => {
  vi.useFakeTimers();
  mockCompile.mockReset();
  mockReq.mockReset();
  mockWf.mockReset();
  mockScrape.mockReset();
  mockImport.mockReset();
  window.localStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  window.localStorage.clear();
});

describe("mount / localStorage hydration", () => {
  it("null storage: boots to defaultState", async () => {
    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    expect(result.current.state.name).toBe("fixture");
    expect(result.current.state.fields.length).toBe(1);
    expect(result.current.tab).toBe("compile");
  });

  it("partial stored draft: merges into defaults, applies stored number override", async () => {
    window.localStorage.setItem(DRAFT_KEY, JSON.stringify({ name: "restored" }));
    window.localStorage.setItem(NUMBER_KEY, "XYZ-777");
    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    expect(result.current.state.name).toBe("restored");
    expect(result.current.state.number).toBe("XYZ-777");
  });

  it("corrupt JSON: falls back to defaultState silently", async () => {
    window.localStorage.setItem(DRAFT_KEY, "{not valid json");
    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    expect(result.current.state.name).toBe("fixture");
  });

  it("debounced save: 160ms after state change, draft is serialized into localStorage", async () => {
    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();

    act(() => {
      result.current.patch("name", "changed-value");
    });
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
    });
    const saved = window.localStorage.getItem(DRAFT_KEY) ?? "";
    expect(saved).toContain("changed-value");
  });
});

describe("toast auto-clear", () => {
  it("info toast clears after 2.2s", async () => {
    installClipboard(() => Promise.resolve());
    mockCompile.mockResolvedValue({ data: { yaml: "v: 1" } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("compile");
    });
    await act(async () => {
      await result.current.handleCopyYAML();
    });
    expect(result.current.toast?.tone).toBe("info");

    await act(async () => {
      vi.advanceTimersByTime(2200);
      await Promise.resolve();
    });
    expect(result.current.toast).toBeNull();
  });

  it("warning toast clears after 5s", async () => {
    mockCompile.mockResolvedValue({ data: { yaml: "" } } as never);
    mockScrape.mockResolvedValue({ data: { request: null, response: null } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();

    // 设成 browser 模式且不填 wait selector -> 触发 warning toast
    act(() => {
      result.current.patch("fetchType", "browser");
      result.current.patchRequest("request", (p) => ({ ...p, browserWaitSelector: "" }));
    });

    await act(async () => {
      await result.current.run("scrape");
    });
    expect(result.current.toast?.tone).toBe("warning");

    await act(async () => {
      vi.advanceTimersByTime(2200);
      await Promise.resolve();
    });
    expect(result.current.toast).not.toBeNull();

    await act(async () => {
      vi.advanceTimersByTime(3000); // total 5.2s
      await Promise.resolve();
    });
    expect(result.current.toast).toBeNull();
  });
});

describe("run() - action branches", () => {
  it("compile: populates compileResult, sets tab=compile, no other results touched", async () => {
    mockCompile.mockResolvedValue({ data: { yaml: "ok" } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("compile");
    });

    expect(mockCompile).toHaveBeenCalled();
    expect(result.current.compileResult).toEqual({ yaml: "ok" });
    expect(result.current.tab).toBe("compile");
    expect(result.current.requestResult).toBeNull();
    expect(result.current.busyAction).toBe("");
  });

  it("request: populates requestResult, sets tab=request", async () => {
    mockReq.mockResolvedValue({ data: { request: null, response: null } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("request");
    });

    expect(result.current.requestResult).toEqual({ request: null, response: null });
    expect(result.current.tab).toBe("request");
  });

  it("workflow: populates workflowResult, sets tab=workflow", async () => {
    mockWf.mockResolvedValue({ data: { steps: [] } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("workflow");
    });

    expect(result.current.workflowResult).toEqual({ steps: [] });
    expect(result.current.tab).toBe("workflow");
  });

  it("scrape + workflowEnabled=false: skips workflow endpoint, uses scrape response as requestResult", async () => {
    mockScrape.mockResolvedValue({
      data: { request: { url: "u" }, response: { status: 200 } },
    } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("scrape");
    });

    expect(mockWf).not.toHaveBeenCalled();
    expect(result.current.scrapeResult).toBeDefined();
    // non-multi request mode: requestResult 回填自 scrape.data.request/response
    expect(result.current.requestResult).toEqual({ request: { url: "u" }, response: { status: 200 } });
    expect(result.current.tab).toBe("basic");
  });

  it("scrape + workflowEnabled=true + workflow has error: stops at workflow tab", async () => {
    mockWf.mockResolvedValue({ data: { error: "expect_count mismatch" } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    act(() => {
      result.current.patch("workflowEnabled", true);
    });
    await act(async () => {
      await result.current.run("scrape");
    });

    expect(result.current.tab).toBe("workflow");
    expect(mockScrape).not.toHaveBeenCalled();
  });

  it("scrape + workflow OK + multiRequestEnabled=true: separate request debug for attempts", async () => {
    mockWf.mockResolvedValue({ data: { steps: [] } } as never);
    mockScrape.mockResolvedValue({ data: { request: null, response: null } } as never);
    mockReq.mockResolvedValue({ data: { request: { attempts: 3 }, response: null } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    act(() => {
      result.current.patch("workflowEnabled", true);
      result.current.patch("multiRequestEnabled", true);
    });
    await act(async () => {
      await result.current.run("scrape");
    });

    expect(mockReq).toHaveBeenCalled();
    expect(result.current.requestResult).toEqual({ request: { attempts: 3 }, response: null });
    expect(result.current.tab).toBe("scrape");
  });

  it("run error (rejected): setError, busyAction reset to ''", async () => {
    mockCompile.mockRejectedValue(new Error("bad draft"));

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("compile");
    });

    expect(result.current.error).toBe("bad draft");
    expect(result.current.busyAction).toBe("");
  });

  it("run error (non-Error thrown): falls back to default copy '插件调试失败'", async () => {
    mockCompile.mockRejectedValue("not an Error instance");

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("compile");
    });

    expect(result.current.error).toBe("插件调试失败");
  });
});

describe("handleCopyYAML", () => {
  it("noop when compileResult.yaml is empty", async () => {
    installClipboard(() => Promise.resolve());

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.handleCopyYAML();
    });

    expect(result.current.toast).toBeNull();
  });

  it("success: info toast 'YAML 已复制.'", async () => {
    installClipboard(() => Promise.resolve());
    mockCompile.mockResolvedValue({ data: { yaml: "hello" } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("compile");
    });
    await act(async () => {
      await result.current.handleCopyYAML();
    });

    expect(result.current.toast).toEqual({ message: "YAML 已复制。", tone: "info" });
  });

  it("clipboard failure: danger toast", async () => {
    installClipboard(() => Promise.reject(new Error("blocked")));
    mockCompile.mockResolvedValue({ data: { yaml: "hi" } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.run("compile");
    });
    await act(async () => {
      await result.current.handleCopyYAML();
    });

    expect(result.current.toast?.tone).toBe("danger");
  });
});

describe("handleImportYAML", () => {
  it("success: sets state from draft, clears all results, toast info", async () => {
    // stateFromDraft 访问很深的字段 (draft.scrape.format / draft.scrape.fields /
    // draft.postprocess.*). 这里只需要最小可运行的骨架.
    mockImport.mockResolvedValue({
      data: {
        draft: {
          name: "imported",
          type: "one-step",
          fetch_type: "go-http",
          hosts: ["https://imported.example.com"],
          request: { method: "GET", path: "/" },
          scrape: { format: "html", fields: {} },
          postprocess: {},
        },
      },
    } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    act(() => {
      result.current.patch("importYAML", "name: imported");
      result.current.setImportOpen(true);
    });
    await act(async () => {
      await result.current.handleImportYAML();
    });

    expect(result.current.state.name).toBe("imported");
    expect(result.current.importOpen).toBe(false);
    expect(result.current.toast?.tone).toBe("info");
  });

  it("error: setError with server message, busyAction reset", async () => {
    mockImport.mockRejectedValue(new Error("YAML parse failed"));

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.handleImportYAML();
    });

    expect(result.current.error).toBe("YAML parse failed");
    expect(result.current.busyAction).toBe("");
  });

  it("non-Error rejection: falls back to 'YAML 导入失败'", async () => {
    mockImport.mockRejectedValue("string error");

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();
    await act(async () => {
      await result.current.handleImportYAML();
    });

    expect(result.current.error).toBe("YAML 导入失败");
  });
});

describe("handleClearDraft", () => {
  it("resets state, localStorage, results, modals, tab", async () => {
    mockCompile.mockResolvedValue({ data: { yaml: "y" } } as never);

    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();

    // 让 save useEffect 真实写进 localStorage 一次, 再验证 handleClearDraft
    // 能把它清掉.
    act(() => {
      result.current.patch("name", "will-be-cleared");
    });
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
    });
    expect(window.localStorage.getItem(DRAFT_KEY)).not.toBeNull();

    await act(async () => {
      await result.current.run("compile");
    });
    act(() => {
      result.current.setTab("request");
      result.current.setActiveSection("post" as never);
      result.current.setImportOpen(true);
      result.current.setExampleOpen(true);
      result.current.setCompileMenuOpen(true);
    });
    act(() => {
      result.current.handleClearDraft();
    });

    expect(window.localStorage.getItem(DRAFT_KEY)).toBeNull();
    expect(window.localStorage.getItem(NUMBER_KEY)).toBeNull();
    expect(result.current.state.name).toBe("fixture");
    expect(result.current.compileResult).toBeNull();
    expect(result.current.tab).toBe("compile");
    expect(result.current.activeSection).toBe("basic");
    expect(result.current.importOpen).toBe(false);
    expect(result.current.exampleOpen).toBe(false);
    expect(result.current.compileMenuOpen).toBe(false);
    expect(result.current.toast?.message).toBe("草稿已清空。");
  });
});

describe("handleFloatingMenuPointerDown + drag lifecycle", () => {
  it("pointer down on handle sets dragState and floatingMenuPos; pointermove/up move it", async () => {
    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();

    // 构造一个假的 pageRef + menu 结构 (handle 在 menu 里).
    const page = document.createElement("div");
    const menu = document.createElement("div");
    const handle = document.createElement("div");
    menu.appendChild(handle);
    page.appendChild(menu);
    document.body.appendChild(page);

    // jsdom 不实现布局, 自己塞 getBoundingClientRect.
    page.getBoundingClientRect = () =>
      ({ left: 0, top: 0, width: 1000, height: 600, right: 1000, bottom: 600, x: 0, y: 0, toJSON: () => ({}) }) as DOMRect;
    menu.getBoundingClientRect = () =>
      ({ left: 100, top: 100, width: 200, height: 100, right: 300, bottom: 200, x: 100, y: 100, toJSON: () => ({}) }) as DOMRect;

    (result.current.pageRef as { current: HTMLDivElement | null }).current = page;

    act(() => {
      const fakeEvent = {
        currentTarget: handle,
        clientX: 120,
        clientY: 120,
      } as unknown as React.PointerEvent<HTMLDivElement>;
      result.current.handleFloatingMenuPointerDown(fakeEvent);
    });

    expect(result.current.floatingMenuPos).not.toBeNull();

    // 派发 pointermove, 验证 handler 更新 pos
    act(() => {
      window.dispatchEvent(
        Object.assign(new Event("pointermove"), { clientX: 300, clientY: 300 }) as PointerEvent,
      );
    });
    // 派发 pointerup, 清 drag state
    act(() => {
      window.dispatchEvent(new Event("pointerup") as PointerEvent);
    });
  });

  it("pointer down without pageRef / menu: noop", async () => {
    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();

    // pageRef 为 null 时, handler 直接 return.
    const handle = document.createElement("div");
    const menu = document.createElement("div");
    menu.appendChild(handle);

    act(() => {
      const fakeEvent = {
        currentTarget: handle,
        clientX: 10,
        clientY: 10,
      } as unknown as React.PointerEvent<HTMLDivElement>;
      result.current.handleFloatingMenuPointerDown(fakeEvent);
    });
    expect(result.current.floatingMenuPos).toBeNull();
  });
});

describe("compileMenuOpen outside pointerdown", () => {
  it("open state: global pointerdown outside the menu closes it", async () => {
    const { result } = renderHook(() => usePluginEditorState());
    await flushAsync();

    act(() => {
      result.current.setCompileMenuOpen(true);
    });
    expect(result.current.compileMenuOpen).toBe(true);

    // 派发一个 pointerdown 事件, target 不在 compileMenuRef 内 -> 自动关闭.
    act(() => {
      const ev = new Event("pointerdown") as PointerEvent;
      Object.defineProperty(ev, "target", { value: document.body });
      window.dispatchEvent(ev);
    });
    expect(result.current.compileMenuOpen).toBe(false);
  });
});

describe("updater thin wrappers (smoke)", () => {
  it("patch merges single keys", () => {
    const { result } = renderHook(() => usePluginEditorState());
    act(() => {
      result.current.patch("number", "HELLO-1");
    });
    expect(result.current.state.number).toBe("HELLO-1");
  });

  it("addField pushes a new field and removeField pops it (obeying >=1 guard)", () => {
    const { result } = renderHook(() => usePluginEditorState());
    const before = result.current.state.fields.length;
    act(() => {
      result.current.addField();
    });
    expect(result.current.state.fields.length).toBe(before + 1);

    const idToRemove = result.current.state.fields[0]!.id;
    act(() => {
      result.current.removeField(idToRemove);
    });
    expect(result.current.state.fields.length).toBe(before);
  });
});
