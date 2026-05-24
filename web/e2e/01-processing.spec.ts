// 01-processing: Processing 页面冒烟 + 列表交互.
//
// E2E 走真实后端 (yamdc server) + 真实前端 (Next.js dev). 我们故意只
// 做"页面可达 + 关键 UI 元素出现 + 协议契约稳"的最低冒烟, 不耦合具体
// fixture 内容: scan 目录默认空, jobs 列表也可能为空, 不需要预置数据.
//
// 任何 backend 5xx / Unhandled Runtime Error 都会让本 spec 立刻失败,
// 给整套 e2e 一个稳定基线.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet } from "./helpers/api";

const FORBIDDEN_RUNTIME_TOKENS = [
  "Unhandled Runtime Error",
  "Internal Server Error",
];

test.describe("processing", () => {
  test("processing 页面渲染、healthz 协议 envelope.code === 0", async ({ page }) => {
    await page.goto("/processing");
    await expect(page.locator("body")).toBeVisible();
    const bodyText = (await page.locator("body").innerText()).trim();
    for (const token of FORBIDDEN_RUNTIME_TOKENS) {
      expect(bodyText).not.toContain(token);
    }

    // 协议契约: HTTP 2xx + envelope.code === 0
    const health = await apiGet<{ status: string }>("/api/healthz");
    expect(health).toEqual({ status: "ok" });
  });

  test("不存在 job 触发 run: HTTP 200 + envelope.code != 0 (业务错)", async () => {
    const env = await apiCallAllowBusinessError(
      "POST",
      "/api/jobs/999999/run",
    );
    expect(env.code).not.toBe(0);
    expect(typeof env.message).toBe("string");
    expect(env.message.length).toBeGreaterThan(0);
  });
});
