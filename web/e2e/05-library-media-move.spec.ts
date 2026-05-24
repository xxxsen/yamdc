// 05-library-media-move: 移动到媒体库 / 重新扫描库的协议契约 + UI 状态.
// devcontainer 起的是干净环境, library 通常没有可移动条目, 这条 spec 覆盖
// 四个用户故事:
//   1) /library 页面渲染时, "移动到媒体库" 按钮带可读 label, 没有任务跑时
//      enabled (不是常态 disabled);
//   2) /api/media-library/status 协议契约稳定: 返回 envelope.code === 0
//      + sync/move 任务有规范的 status 枚举;
//   3) POST /api/media-library/move 在没有可迁移 review-imported 条目时仍然
//      保持 HTTP 200 + envelope (不是 4xx/5xx); 同步状态再读一遍仍然落在
//      合法枚举里;
//   4) 触发一次 move 后短时间内重复触发: 后端必须用业务错协议把 "已在跑"
//      表达出来 (HTTP 200 + envelope.code !== 0), 不允许 5xx.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet } from "./helpers/api";

interface MediaLibraryStatus {
  configured: boolean;
  sync: { task_key: string; status: string };
  move: { task_key: string; status: string };
}

const TASK_STATES = ["idle", "running", "completed", "failed"];

test.describe("library media move 用户故事", () => {
  test("/library 页面: 移动到媒体库按钮存在且默认可点 (无任务跑时不锁死)", async ({ page }) => {
    await page.goto("/library");
    const moveBtn = page.getByRole("button", { name: /移动到媒体库/ });
    await expect(moveBtn).toBeVisible();
    // disabled 取决于 configured / running / refreshBusy, 默认 devcontainer
    // configured=true, 任务都 idle, 应该 enabled. 这条断言守护"按钮不会
    // 默认就是 disabled" 这种 UX 退化.
    await expect(moveBtn).toBeEnabled();
  });

  test("协议契约: /api/media-library/status 返回 configured + sync/move 状态枚举", async () => {
    const data = await apiGet<MediaLibraryStatus>("/api/media-library/status");
    expect(typeof data.configured).toBe("boolean");
    expect(TASK_STATES).toContain(data.sync.status);
    expect(TASK_STATES).toContain(data.move.status);
  });

  test("POST /api/media-library/move 在无可迁移项时: HTTP 200 + envelope; 状态查询仍然落在合法枚举", async () => {
    const env = await apiCallAllowBusinessError("POST", "/api/media-library/move");
    expect(typeof env.code).toBe("number");
    expect(typeof env.message).toBe("string");

    const status = await apiGet<MediaLibraryStatus>("/api/media-library/status");
    expect(TASK_STATES).toContain(status.move.status);
  });

  test("重复触发 move (本来就在跑或刚完成): 仍然走 envelope 协议, 不会蹦 5xx", async () => {
    // 第一次 move + 第二次紧跟着再来一发. yamdc 后端对 "已在跑" 用
    // envelope.code != 0 表达, 不允许直接 502/503. 两次都走
    // apiCallAllowBusinessError 接 envelope, 任意 envelope.code 都合法,
    // 但 HTTP 必须 2xx + JSON envelope 形态; 网络层挂 / 5xx 都立刻让
    // 测试失败 — 不再 .catch(() => undefined) 静默吞.
    const first = await apiCallAllowBusinessError("POST", "/api/media-library/move");
    expect(typeof first.code).toBe("number");
    expect(typeof first.message).toBe("string");
    const env = await apiCallAllowBusinessError("POST", "/api/media-library/move");
    expect(typeof env.code).toBe("number");
    expect(typeof env.message).toBe("string");
  });
});
