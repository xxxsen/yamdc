// 10-uiux-regression: 综合 UI/UX 回归用户故事级 E2E. 覆盖:
//   1) 顶层 8 个页面在桌面分辨率 (1280x800) 下首屏渲染稳定, 不出
//      Unhandled Runtime Error / Internal Server Error;
//   2) /processing 页面: 顶层导航/标题骨架可达 + 不会因长标题/空数据撑爆
//      viewport (横向滚动);
//   3) Escape 按键不会让正常页面崩溃 (空 modal 状态时 Escape 应是 no-op);
//   4) 全局 nav (Tab 顺序) 至少能用 Tab 切到第一个 focusable 元素 — 这是
//      a11y 用户故事最低保障, 防回归;
//   5) /library 长 keyword 输入: 200 字符不应让搜索框溢出 viewport.

import { expect, test } from "@playwright/test";

const PAGES = [
  "/processing",
  "/review",
  "/library",
  "/media-library",
  "/debug/searcher",
  "/debug/ruleset",
  "/debug/handler",
  "/debug/plugin-editor",
];

test.describe("uiux 回归用户故事", () => {
  for (const p of PAGES) {
    test(`${p} 桌面 1280x800: 首屏稳定 + 不出运行时错误 + 不出现 NaN/undefined 文案`, async ({ browser }) => {
      const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
      const page = await ctx.newPage();
      await page.goto(p);
      await expect(page.locator("body")).toBeVisible();
      const text = (await page.locator("body").innerText()).trim();
      expect(text.length).toBeGreaterThan(0);
      expect(text).not.toContain("Unhandled Runtime Error");
      expect(text).not.toContain("Internal Server Error");
      expect(text).not.toContain("undefined");
      expect(text).not.toContain("NaN");
      await ctx.close();
    });
  }

  test("/processing: 不会出现横向滚动 (长标题 / 空数据撑爆 viewport 的回归守护)", async ({ browser }) => {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await ctx.newPage();
    await page.goto("/processing");
    await expect(page.locator("body")).toBeVisible();
    // documentElement.scrollWidth <= viewport width: 桌面优先布局核心 UX 约定.
    const overflow = await page.evaluate(() => {
      const doc = document.documentElement;
      // 8px 容忍滚动条宽度 / sub-pixel 误差.
      return doc.scrollWidth - doc.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(8);
    await ctx.close();
  });

  test("Escape 在空状态页面: 不让页面崩溃 (modal 关闭路径不被无意触发)", async ({ page }) => {
    await page.goto("/processing");
    await page.keyboard.press("Escape");
    const text = (await page.locator("body").innerText()).trim();
    expect(text).not.toContain("Unhandled Runtime Error");
    expect(text).not.toContain("Internal Server Error");
  });

  test("a11y: /library 上 Tab 一次, 焦点必落到一个 focusable 元素 (不会卡在 body)", async ({ page }) => {
    await page.goto("/library");
    await page.locator("body").click({ position: { x: 1, y: 1 } });
    await page.keyboard.press("Tab");
    // 至少不能停在 BODY — 那意味着首屏没有任何可达 focusable 元素.
    const tag = await page.evaluate(() => document.activeElement?.tagName ?? "");
    expect(["A", "BUTTON", "INPUT", "SELECT", "TEXTAREA"]).toContain(tag);
  });

  test("/library 长 keyword: 200 字符输入不会撑爆 viewport (横向 overflow 守护)", async ({ browser }) => {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await ctx.newPage();
    await page.goto("/library");
    const search = page.getByPlaceholder("按标题 / 影片 ID / 演员搜索");
    await search.fill("a".repeat(200));
    const overflow = await page.evaluate(() => {
      const doc = document.documentElement;
      return doc.scrollWidth - doc.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(8);
    await ctx.close();
  });
});
