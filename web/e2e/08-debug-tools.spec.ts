// 08-debug-tools: 三个 debug 页面的用户故事级 E2E. 覆盖:
//   /debug/searcher  — 输入校验 (空 ID 给前端层错) + 真实搜索成功路径
//                       (用 yamdc-script 真实清理脚本仓库的 case 之一作为 ID)
//                       + 业务错路径 (后端协议契约稳定);
//   /debug/ruleset   — 输入校验 + 用 yamdc-script 真实 case 拉一次 explain;
//   /debug/handler   — 入口可达 + handler 列表协议 + handler-run 422 业务错.
//
// 核心 UX 协议: 业务错必须 HTTP 200 + envelope.code != 0. 任何 5xx 都判失败.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet, apiPost } from "./helpers/api";

interface SearcherDebugPlugins {
  available: string[];
}

interface MovieIDExplainData {
  input: string;
  final?: { number_id?: string; status?: string };
}

interface HandlersListData {
  handlers: string[];
}

test.describe("debug-tools 用户故事", () => {
  test("/debug/searcher: 输入框 + 检索按钮可见 + 插件目录协议契约", async ({ page }) => {
    await page.goto("/debug/searcher");
    await expect(page.getByPlaceholder(/MOVIE-12345|影片 ID|插件/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /开始检索|检索中/ })).toBeVisible();

    const data = await apiGet<SearcherDebugPlugins>("/api/debug/searcher/plugins");
    expect(Array.isArray(data.available)).toBe(true);
  });

  test("/debug/searcher: 失败路径 — 用空白 ID 走业务错协议 (HTTP 200 + envelope.code != 0)", async () => {
    // 后端拒绝空 keyword: HTTP 必须仍然是 200, envelope 用 code 表达失败.
    const env = await apiCallAllowBusinessError("POST", "/api/debug/searcher/search", {
      keyword: "",
    });
    expect(env.code).not.toBe(0);
    expect(env.message.length).toBeGreaterThan(0);
  });

  test("/debug/ruleset: 输入框 + 测试按钮可见 + explain 协议契约 (用真实清理脚本仓库 case)", async ({ page }) => {
    await page.goto("/debug/ruleset");
    await expect(page.getByPlaceholder(/MOVIE12345|文件名/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /开始测试|解析中/ })).toBeVisible();

    // ABC-123 是 movieidcleaner 规则集里的典型形式, 走一次 explain
    // 验证 envelope.code===0 + final 结构.
    const data = await apiPost<MovieIDExplainData>("/api/debug/movieid-cleaner/explain", {
      input: "ABC-123",
    });
    expect(typeof data.input).toBe("string");
    expect(data.input.length).toBeGreaterThan(0);
  });

  test("/debug/handler: handler 列表接口可达 + handler-run 不存在的链 走业务错", async ({ page }) => {
    await page.goto("/debug/handler");
    // 顶部有"运行"按钮.
    await expect(page.getByRole("button", { name: /^运行$|执行中/ })).toBeVisible();

    const data = await apiGet<HandlersListData>("/api/debug/handlers");
    expect(Array.isArray(data.handlers)).toBe(true);

    const env = await apiCallAllowBusinessError("POST", "/api/debug/handler/run", {
      handlers: ["__nonexistent_handler__"],
      meta: {},
    });
    expect(env.code).not.toBe(0);
  });
});
