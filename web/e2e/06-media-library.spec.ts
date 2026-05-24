// 06-media-library: 媒体库页面用户故事级 E2E.
// devcontainer 已经把 library_dir 指到 .devcontainer-data/library, 所以
// configured=true 是常态; 列表通常为空 → 看到 "当前媒体库里还没有项目".
//
// 覆盖的用户故事:
//   1) 页面顶部搜索框 + 同步入口按钮 (开始同步) 必现, 用户能立刻看到主动作;
//   2) 默认 sort 控件 (年份 / 大小 / 标题 / 入库时间 / 顺序) 都以可见 button
//      形态出现, 用户可以从 UI 上挑筛选维度;
//   3) 空集合时渲染 "当前媒体库里还没有项目" 兜底文案 (空态用户故事);
//   4) GET /api/media-library 协议契约稳定 (envelope.data.items 是数组).

import { expect, test } from "@playwright/test";

import { apiGet } from "./helpers/api";

interface MediaLibraryItem {
  id: number;
  name: string;
  title: string;
  number: string;
}

interface MediaLibraryListResponse {
  items: MediaLibraryItem[];
}

test.describe("media-library 用户故事", () => {
  test("页面顶部: 关键字搜索框 + 开始同步按钮 必现", async ({ page }) => {
    await page.goto("/media-library");
    await expect(page.locator("body")).toBeVisible();
    await expect(page.getByPlaceholder(/搜索|关键字|按标题/i)).toBeVisible();
    // 同步按钮在 filter rail 末端, label 为"开始同步"或"同步中".
    await expect(page.getByRole("button", { name: /开始同步|同步中/ })).toBeVisible();
  });

  test("筛选 / 排序入口: 年份 / 顺序 button 默认可见 (用户可以切排序)", async ({ page }) => {
    await page.goto("/media-library");
    // 这两个 button 是用户故事里"切年份 / 切顺序"的入口, 任何一个不可见
    // 都意味着 filter rail 出回归.
    await expect(page.getByRole("button", { name: "年份" })).toBeVisible();
    await expect(page.getByRole("button", { name: "顺序" })).toBeVisible();
  });

  test("空集合 OR 列表渲染: 用户必看到内容 (空时兜底文案; 非空时第一张卡片可见)", async ({ page }) => {
    await page.goto("/media-library");
    const emptyHint = page.getByText("当前媒体库里还没有项目");
    const firstCard = page.locator(".media-library-card").first();
    // 二选一可见 — 不强求 fixture 注入, 只要任一文案/卡片出现就算页面正常.
    const seenEither = (await emptyHint.isVisible().catch(() => false))
      || (await firstCard.isVisible().catch(() => false));
    expect(seenEither).toBe(true);
  });

  test("协议契约: GET /api/media-library 返回 items 数组", async () => {
    const data = await apiGet<MediaLibraryListResponse>("/api/media-library");
    expect(Array.isArray(data.items)).toBe(true);
    for (const it of data.items) {
      expect(typeof it.id).toBe("number");
      expect(typeof it.name).toBe("string");
    }
  });
});
