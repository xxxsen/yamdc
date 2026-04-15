import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { describeActiveElement, logUploadDebug } from "@/lib/upload-debug";

type DebugWindow = Window & {
  __yamdcUploadDebug?: Array<{ ts: string; scope: string; event: string; data?: unknown }>;
};

describe("upload-debug", () => {
  afterEach(() => {
    if (typeof globalThis.window !== "undefined") {
      delete (globalThis.window as DebugWindow).__yamdcUploadDebug;
    }
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  describe("describeActiveElement", () => {
    it("returns lowercase tag for basic element without id or classes", () => {
      vi.stubGlobal("document", {
        activeElement: { tagName: "INPUT", id: "", className: "" },
      });
      expect(describeActiveElement()).toBe("input");
    });

    it("includes id as #myid", () => {
      vi.stubGlobal("document", {
        activeElement: { tagName: "INPUT", id: "myid", className: "" },
      });
      expect(describeActiveElement()).toBe("input#myid");
    });

    it("includes multiple classes joined with dots", () => {
      vi.stubGlobal("document", {
        activeElement: { tagName: "BUTTON", id: "", className: "cls1 cls2" },
      });
      expect(describeActiveElement()).toBe("button.cls1.cls2");
    });

    it("includes id and className together", () => {
      vi.stubGlobal("document", {
        activeElement: { tagName: "SPAN", id: "myid", className: "cls1" },
      });
      expect(describeActiveElement()).toBe("span#myid.cls1");
    });

    it("returns document-unavailable when document is undefined", () => {
      const origDoc = globalThis.document;
      // @ts-expect-error - intentionally removing document
      delete globalThis.document;
      try {
        expect(describeActiveElement()).toBe("document-unavailable");
      } finally {
        globalThis.document = origDoc;
      }
    });

    it("returns none when activeElement is null", () => {
      vi.stubGlobal("document", { activeElement: null });
      expect(describeActiveElement()).toBe("none");
    });

    it("omits class suffix when className is empty string", () => {
      vi.stubGlobal("document", {
        activeElement: { tagName: "P", id: "", className: "" },
      });
      expect(describeActiveElement()).toBe("p");
    });

    it("omits class suffix when className is whitespace only", () => {
      vi.stubGlobal("document", {
        activeElement: { tagName: "P", id: "", className: "   \t\n" },
      });
      expect(describeActiveElement()).toBe("p");
    });

    it("omits class suffix when className is non-string", () => {
      vi.stubGlobal("document", {
        activeElement: {
          tagName: "SVG",
          id: "",
          className: { baseVal: "should-not-appear" },
        },
      });
      expect(describeActiveElement()).toBe("svg");
    });

    it("normalizes multiple space-separated classes", () => {
      vi.stubGlobal("document", {
        activeElement: { tagName: "A", id: "", className: "  cls1   cls2  cls3 " },
      });
      expect(describeActiveElement()).toBe("a.cls1.cls2.cls3");
    });

    it("matches stubbed example from upload form debugging", () => {
      vi.stubGlobal("document", {
        activeElement: { tagName: "INPUT", id: "name", className: "form-control large" },
      });
      expect(describeActiveElement()).toBe("input#name.form-control.large");
    });
  });

  describe("logUploadDebug", () => {
    let debugSpy: ReturnType<typeof vi.spyOn<typeof console, "debug">>;

    beforeEach(() => {
      vi.stubGlobal("window", {} as DebugWindow);
      debugSpy = vi.spyOn(console, "debug").mockImplementation(() => {});
    });

    it("pushes entry to window.__yamdcUploadDebug with correct fields", () => {
      vi.spyOn(Date.prototype, "toISOString").mockReturnValue("2024-01-02T03:04:05.000Z");
      delete (globalThis.window as DebugWindow).__yamdcUploadDebug;

      logUploadDebug("scope-a", "started", { x: 1 });

      const store = (globalThis.window as DebugWindow).__yamdcUploadDebug;
      expect(store).toBeDefined();
      expect(store).toHaveLength(1);
      const s = store ?? [];
      expect(s[0]).toEqual({
        ts: "2024-01-02T03:04:05.000Z",
        scope: "scope-a",
        event: "started",
        data: { x: 1 },
      });
    });

    it("calls console.debug with formatted message and passes data through", () => {
      logUploadDebug("my-scope", "my-event", { key: "value" });

      expect(debugSpy).toHaveBeenCalledWith("[yamdc-upload][my-scope] my-event", { key: "value" });
    });

    it("returns early when window is undefined without throwing", () => {
      const origWin = globalThis.window;
      // @ts-expect-error - intentionally removing window
      delete globalThis.window;
      try {
        expect(() => logUploadDebug("s", "e", { z: 3 })).not.toThrow();
        expect(debugSpy).not.toHaveBeenCalled();
      } finally {
        globalThis.window = origWin;
      }
    });

    it("omits data on entry when called without data argument", () => {
      vi.spyOn(Date.prototype, "toISOString").mockReturnValue("t-fixed");
      delete (globalThis.window as DebugWindow).__yamdcUploadDebug;

      logUploadDebug("only-scope", "only-event");

      const store = (globalThis.window as DebugWindow).__yamdcUploadDebug ?? [];
      expect(store[0].data).toBeUndefined();
      expect(debugSpy).toHaveBeenCalledWith("[yamdc-upload][only-scope] only-event", undefined);
    });

    it("caps store at 400 entries by shifting oldest", () => {
      const store = ((globalThis.window as DebugWindow).__yamdcUploadDebug = []);
      for (let i = 0; i < 400; i++) {
        store.push({ ts: "t", scope: "s", event: `e${i}` });
      }
      expect(store).toHaveLength(400);
      logUploadDebug("new", "overflow");
      expect(store).toHaveLength(400);
      expect(store[0].event).toBe("e1");
      expect(store[399].event).toBe("overflow");
    });

    it("stores first entry correctly when store initially empty", () => {
      delete (globalThis.window as DebugWindow).__yamdcUploadDebug;
      vi.spyOn(Date.prototype, "toISOString").mockReturnValue("ts0");

      logUploadDebug("first-scope", "first-event", 42);

      const store = (globalThis.window as DebugWindow).__yamdcUploadDebug ?? [];
      expect(store).toHaveLength(1);
      expect(store[0]).toEqual({ ts: "ts0", scope: "first-scope", event: "first-event", data: 42 });
    });

    it("accumulates multiple calls in order", () => {
      delete (globalThis.window as DebugWindow).__yamdcUploadDebug;
      let n = 0;
      vi.spyOn(Date.prototype, "toISOString").mockImplementation(() => `ts-${n++}`);

      logUploadDebug("a", "one");
      logUploadDebug("b", "two");
      logUploadDebug("c", "three");

      const store = (globalThis.window as DebugWindow).__yamdcUploadDebug ?? [];
      expect(store.map((e) => e.event)).toEqual(["one", "two", "three"]);
      expect(store.map((e) => e.scope)).toEqual(["a", "b", "c"]);
    });

    it("reuses existing __yamdcUploadDebug store without replacing it", () => {
      const existing: DebugWindow["__yamdcUploadDebug"] = [{ ts: "old", scope: "x", event: "y" }];
      (globalThis.window as DebugWindow).__yamdcUploadDebug = existing;

      logUploadDebug("a", "b");

      expect((globalThis.window as DebugWindow).__yamdcUploadDebug).toBe(existing);
      expect(existing).toHaveLength(2);
      expect(existing[0].event).toBe("y");
      expect(existing[1].event).toBe("b");
    });
  });
});
