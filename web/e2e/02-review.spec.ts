// 02-review: Review 页面冒烟 + 异常路径协议.
//
// 真实 backend 启动后 review 列表通常为空, 但页面必须能正常渲染 (空态).
// 业务错误路径 (review save 不存在的 job) 必须 HTTP 200 + envelope.code != 0,
// 这是 yamdc 全栈协议契约的关键回归点.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError } from "./helpers/api";

test.describe("review", () => {
  test("review 页面可达 + 空态渲染 (没有 reviewing job 时也不崩)", async ({ page }) => {
    await page.goto("/review");
    await expect(page.locator("body")).toBeVisible();
    const text = (await page.locator("body").innerText()).trim();
    expect(text.length).toBeGreaterThan(0);
    expect(text).not.toContain("Unhandled Runtime Error");
  });

  test("review save 不存在的 job: 必须 HTTP 200 + envelope.code != 0", async () => {
    const env = await apiCallAllowBusinessError("PUT", "/api/review/jobs/999999", {
      meta: {},
    });
    expect(env.code).not.toBe(0);
    expect(env.message.length).toBeGreaterThan(0);
  });

  test("review import 不存在的 job: 必须 HTTP 200 + envelope.code != 0", async () => {
    const env = await apiCallAllowBusinessError(
      "POST",
      "/api/review/jobs/999999/import",
    );
    expect(env.code).not.toBe(0);
  });
});
