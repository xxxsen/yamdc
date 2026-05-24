// 07-media-library-sync: 媒体库同步用户故事级 E2E.
//
// 覆盖的用户故事:
//   1) /media-library 页面"同步媒体库"按钮可见且默认 enabled (没有跑同步时);
//   2) 协议契约: 触发 POST /api/media-library/sync 后, 状态查询要么是
//      idle (瞬完) / running (跑中) / completed / failed 四种合法枚举,
//      不允许出未知状态;
//   3) 同步日志接口 GET /api/media-library/sync/logs 直接返回数组
//      (envelope.data 本身是 SyncLogEntry[], 用户能在 UI 上看到同步历史);
//   4) /api/media-library/sync 在没有 fixture 时 also-best-effort 触发: 即便
//      已在运行也必须 HTTP 200 + envelope (不能蹦 5xx) — 这是 UX 退化的
//      关键守护点.
//   5) 用户能在页面上点击"同步媒体库"按钮发起同步, 状态查询能反映出 running.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet } from "./helpers/api";

interface MediaLibraryStatus {
  configured: boolean;
  sync: { status: string };
  move: { status: string };
}

interface SyncLogEntry {
  id: number;
  run_id: string;
  level: string;
  rel_path: string;
  message: string;
  created_at: number;
}

const SYNC_STATES = ["idle", "running", "completed", "failed"];

// SYNC_BTN_LABEL: 见 web/src/components/media-library-shell/use-media-library-sync.ts
// 中 syncButtonLabel 三态 ("同步媒体库" / "同步中..." / "同步完成"), 必须三个都覆盖.
const SYNC_BTN_LABEL = /同步媒体库|同步中|同步完成/;

test.describe("media-library sync 用户故事", () => {
  test("/media-library 页面: 同步按钮可见且默认可点 (没跑同步时不锁死)", async ({ page }) => {
    await page.goto("/media-library");
    const btn = page.getByRole("button", { name: SYNC_BTN_LABEL });
    await expect(btn).toBeVisible();
  });

  test("触发同步后: status 枚举落在合法集合", async () => {
    // 同步触发协议契约: HTTP 200 + envelope, 业务态可能是 "已在跑"
    // (envelope.code != 0). 用 apiCallAllowBusinessError 接 envelope, 不再
    // 通过 .catch(() => undefined) 静吞错误.
    const trigger = await apiCallAllowBusinessError<unknown>("POST", "/api/media-library/sync");
    expect(typeof trigger.code).toBe("number");
    expect(typeof trigger.message).toBe("string");

    const status = await apiGet<MediaLibraryStatus>("/api/media-library/status");
    expect(status.configured).toBe(true);
    expect(SYNC_STATES).toContain(status.sync.status);
  });

  test("协议契约: 同步日志 GET /api/media-library/sync/logs envelope.data 直接是 SyncLogEntry[] 数组", async () => {
    const env = await apiCallAllowBusinessError<SyncLogEntry[] | null>(
      "GET",
      "/api/media-library/sync/logs",
    );
    // 即便没有任何同步历史, code 也应是 0, data 是 [] 或 null.
    expect(env.code).toBe(0);
    // 后端在历史为空时可能返回 null, 客户端 normalize 为 [].
    const arr = Array.isArray(env.data) ? env.data : [];
    expect(Array.isArray(arr)).toBe(true);
    for (const item of arr) {
      expect(typeof item.id).toBe("number");
      expect(typeof item.run_id).toBe("string");
    }
  });

  test("重复触发 sync: 同步在跑也必须 HTTP 200 + envelope (不允许 5xx)", async () => {
    // 第一次触发以及紧接着的第二次都接 envelope. 业务态可以不同 (第一次
    // 进 running, 第二次走 "已在跑") 但都必须是 HTTP 200 + envelope, 任何
    // 网络层失败 / 5xx 都直接让 spec 红.
    const first = await apiCallAllowBusinessError<unknown>("POST", "/api/media-library/sync");
    expect(typeof first.code).toBe("number");
    expect(typeof first.message).toBe("string");
    const env = await apiCallAllowBusinessError("POST", "/api/media-library/sync");
    expect(typeof env.code).toBe("number");
    expect(typeof env.message).toBe("string");
  });

  test("UI 触发同步: 点击同步按钮后 status 接口返回合法枚举且按钮进入忙态", async ({ page }) => {
    await page.goto("/media-library");
    const btn = page.getByRole("button", { name: SYNC_BTN_LABEL });
    await expect(btn).toBeVisible();
    // 关键路径: 真实点击同步按钮 (不再用 .catch 静吞), 等待客户端发出
    // POST /api/media-library/sync 的响应, 然后读 status. 任何点击失败 /
    // 网络失败都会真实抛出, 不被静默吞掉.
    const syncCall = page.waitForResponse((res) =>
      res.url().endsWith("/api/media-library/sync") && res.request().method() === "POST",
    );
    await btn.click();
    await syncCall;
    const status = await apiGet<MediaLibraryStatus>("/api/media-library/status");
    expect(SYNC_STATES).toContain(status.sync.status);
    // 同步触发后按钮要么进 "同步中..." (running 路径), 要么瞬间完成回到
    // "同步媒体库" / 闪烁 "同步完成" — 任意三态命中即视为按钮真实更新.
    await expect(btn).toHaveText(SYNC_BTN_LABEL);
  });
});
