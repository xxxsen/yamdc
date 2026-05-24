// 00-api-helper-contract: 验证 helpers/api.ts 的 envelope 解构契约本身.
//
// 守护点: apiGet<T>(path) 返回的是 envelope.data 本身, 而不是
// envelope ({ code, message, data }) 也不是再包一层 { items }. 任何对
// helper 的回归 (例如把整个 envelope 直接返回) 都会让数组类的 API 用例
// 在生产路径下表现成 "data 是 undefined". 这里直接通过 Mock fetch 验证.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet } from "./helpers/api";

const realFetch = globalThis.fetch;

test.beforeEach(() => {
  globalThis.fetch = realFetch;
});

test.afterAll(() => {
  globalThis.fetch = realFetch;
});

test.describe("helpers/api 契约", () => {
  test("apiGet 返回 envelope.data 本身 (数组直返, 不额外包一层)", async () => {
    const payload = [{ rel_path: "a", title: "A" }];
    globalThis.fetch = async () =>
      new Response(JSON.stringify({ code: 0, message: "ok", data: payload }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    const data = await apiGet<typeof payload>("/api/dummy");
    expect(Array.isArray(data)).toBe(true);
    expect(data).toEqual(payload);
  });

  test("apiGet 业务错 (code != 0) 直接抛出, 携带 message", async () => {
    globalThis.fetch = async () =>
      new Response(JSON.stringify({ code: 100, message: "bad", data: null }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    let err: unknown = null;
    try {
      await apiGet<unknown>("/api/dummy");
    } catch (e) {
      err = e;
    }
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).toContain("code=100");
  });

  test("apiCallAllowBusinessError 对 code != 0 不抛, 返回完整 envelope", async () => {
    globalThis.fetch = async () =>
      new Response(JSON.stringify({ code: 42, message: "biz fail", data: null }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    const env = await apiCallAllowBusinessError<unknown>("GET", "/api/dummy");
    expect(env.code).toBe(42);
    expect(env.message).toBe("biz fail");
  });
});
