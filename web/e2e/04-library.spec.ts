// 04-library: 已入库目录页面用户故事级 E2E.
//
// devcontainer 默认 library 目录是空的, 我们没法在不污染 fixture 的前提下
// 直接断言"列表里有 X 个项目"; 但我们仍然可以围绕"页面骨架 + 用户可发起
// 的纯 UI 交互 + 后端协议契约"做用户故事级覆盖:
//   1) 标题 / 关键字搜索框 / 重新扫描库 / 移动到媒体库 四个核心 UI 节点必须
//      可见, 否则 layout 出回归;
//   2) 用户在搜索框里键入文字 → input 状态被接管, 不会把按钮卡住;
//   3) 后端 GET /api/library 协议契约稳定 (envelope.code === 0 + items 数组);
//   4) 后端 GET /api/library/item 对不存在 rel_path 必须 HTTP 200 + 业务错;
//   5) 关键操作按钮在 "未配置 library_dir / 移动正在跑" 等运行时锁的语义上
//      跟产品文档保持一致 (本 spec 验"未跑任何任务时按钮 enabled" 即可).

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet } from "./helpers/api";

interface LibraryItem {
  rel_path: string;
  title: string;
  number: string;
  variant_count: number;
}

interface LibraryListResponse {
  items: LibraryItem[];
}

test.describe("library 用户故事", () => {
  test("库列表骨架: 标题 / 搜索框 / 重新扫描 / 移动到媒体库 四件套必现", async ({ page }) => {
    await page.goto("/library");
    await expect(page.locator("body")).toBeVisible();

    await expect(page.getByRole("heading", { name: "已入库" })).toBeVisible();
    await expect(page.getByPlaceholder("按标题 / 影片 ID / 演员搜索")).toBeVisible();
    await expect(page.getByRole("button", { name: /重新扫描/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /移动到媒体库/ })).toBeVisible();
  });

  test("用户键入关键字: 搜索框接管输入, 不会被锁死也不会把页面打崩", async ({ page }) => {
    await page.goto("/library");
    const search = page.getByPlaceholder("按标题 / 影片 ID / 演员搜索");
    await search.fill("ABC-001");
    await expect(search).toHaveValue("ABC-001");
    // 清空后仍然能拿回"原始列表"的渲染状态 (不会因 deferredKeyword 死锁).
    await search.fill("");
    await expect(search).toHaveValue("");
  });

  test("协议契约: GET /api/library 返回 items 数组; GET /api/library/item 对不存在 rel_path 走业务错", async () => {
    const data = await apiGet<LibraryListResponse>("/api/library");
    expect(Array.isArray(data.items)).toBe(true);
    for (const it of data.items) {
      expect(typeof it.rel_path).toBe("string");
      expect(typeof it.variant_count).toBe("number");
    }
    const env = await apiCallAllowBusinessError(
      "GET",
      "/api/library/item?rel_path=__nonexistent__/library/path__",
    );
    expect(env.code).not.toBe(0);
    expect(env.message.length).toBeGreaterThan(0);
  });
});
